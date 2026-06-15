package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

// generateRequestID creates a transient unique identifier for error correlation.
func generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(bytes)
}

// emitBlockingError encapsulates the emission of structural failures with mandatory metadata.
func emitBlockingError(msg string, err error, requestID string) {
	slog.Error(msg,
		slog.String("request_id", requestID),
		slog.String("synthetic_code", "500"),
		slog.String("stack_trace", string(debug.Stack())),
		slog.Any("error", err),
	)
}

type controlServerHolder struct {
	mu     sync.Mutex
	server *control.Server
}

// Set updates the held control server reference safely.
func (h *controlServerHolder) Set(server *control.Server) {
	if h == nil || server == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	slog.Debug("Updating control server reference in memory holder")
	h.server = server
}

// Stop safely shuts down the held control server if one is active.
func (h *controlServerHolder) Stop(ctx context.Context) error {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	server := h.server
	h.server = nil
	h.mu.Unlock()

	if server == nil {
		return nil
	}

	slog.Info("Planned shutdown of control server instance initiated")

	if err := server.Stop(ctx); err != nil {
		emitBlockingError("Blocking failure during control server shutdown", err, generateRequestID())
		return err
	}

	slog.Info("Planned shutdown of control server instance completed successfully")
	return nil
}

func (h *controlServerHolder) BroadcastGuildEvent(guildID string, botPresent bool) {
	if h == nil {
		return
	}
	h.mu.Lock()
	server := h.server
	h.mu.Unlock()

	if server == nil {
		return
	}

	slog.Debug("Broadcasting guild presence transition event",
		slog.String("guild_id", guildID),
		slog.Bool("bot_present", botPresent),
	)
	server.BroadcastGuildEvent(guildID, botPresent)
}

type controlStartupTaskOptions struct {
	runOptions            RunOptions
	configManager         *files.ConfigManager
	runtimeApplier        *runtimeapply.Manager
	controlBearerToken    string
	runtimeResolver       *botRuntimeResolver
	store                 *storage.Store
	qotdService           *qotd.Service
	moderationMetrics     moderation.Metrics
	controlServerRegistry *controlServerHolder
}

func scheduleRuntimeConfiguredGuildLogging(
	runtime *botRuntime,
	configManager *files.ConfigManager,
	startupTasks *StartupTaskOrchestrator,
) {
	if runtime == nil || runtime.session == nil || configManager == nil {
		return
	}

	run := func(context.Context) error {
		err := files.LogConfiguredGuildsForBot(configManager, runtime.session, runtime.instanceID)
		if err != nil {
			slog.Warn("Mitigated degradation: Some configured guilds could not be accessed during runtime logging",
				slog.String("botInstanceID", runtime.instanceID),
				slog.String("error", err.Error()),
			)
		}
		return nil
	}

	if startupTasks == nil {
		if err := run(context.Background()); err != nil {
			slog.Warn("Failed to execute synchronous guild logging task",
				slog.String("error", err.Error()),
			)
		}
		return
	}

	startupTasks.GoLight("log_configured_guilds:"+runtime.instanceID, run)
}

func scheduleStartupWebhookEmbedUpdates(
	startupTasks *StartupTaskOrchestrator,
	cfg *files.BotConfig,
	sessionResolver func(guildID string) *discordgo.Session,
) {
	if cfg == nil || sessionResolver == nil {
		return
	}

	run := func(ctx context.Context) error {
		for _, item := range collectStartupWebhookEmbedUpdates(cfg) {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("scheduleStartupWebhookEmbedUpdates: %w", err)
			}

			operation := fmt.Sprintf(
				"runtime_config.webhook_embed_updates[%s:%d]",
				item.scope,
				item.index,
			)
			sess := sessionResolver(item.scope)
			if sess == nil {
				slog.Debug("Session resolution missed for webhook patch target; skipping",
					slog.String("operation", operation),
					slog.String("scope", item.scope),
				)
				continue
			}

			if err := webhook.PatchMessageEmbed(sess, webhook.MessageEmbedPatch{
				MessageID:  item.update.MessageID,
				WebhookURL: item.update.WebhookURL,
				Embed:      item.update.Embed,
			}); err != nil {
				slog.Warn("Compensatory action required: Webhook embed patch payload rejected",
					slog.String("operation", operation),
					slog.String("scope", item.scope),
					slog.String("message_id", strings.TrimSpace(item.update.MessageID)),
					slog.String("error", err.Error()),
				)
			} else {
				slog.Debug("Webhook embed patch applied successfully to target",
					slog.String("operation", operation),
					slog.String("scope", item.scope),
					slog.String("message_id", strings.TrimSpace(item.update.MessageID)),
				)
			}
		}
		return nil
	}

	if startupTasks == nil {
		if err := run(context.Background()); err != nil {
			slog.Warn("Failed to execute synchronous webhook embed update schedule",
				slog.String("error", err.Error()),
			)
		}
		return
	}

	startupTasks.GoLight("startup_webhook_embed_updates", run)
}

func scheduleControlServerStartup(startupTasks *StartupTaskOrchestrator, opts controlStartupTaskOptions) {
	if opts.runOptions.DisableControl {
		slog.Info("Architectural transition: Control server startup bypassed via explicit run options")
		return
	}

	run := func(ctx context.Context) error {
		return startControlServerStartupTask(ctx, opts)
	}

	if startupTasks == nil {
		if err := run(context.Background()); err != nil {
			emitBlockingError("Synchronous execution of control server startup failed completely", err, generateRequestID())
		}
		return
	}

	startupTasks.GoLight("control_server", run)
}

func startControlServerStartupTask(ctx context.Context, opts controlStartupTaskOptions) error {
	controlRuntime, err := resolveControlRuntime(ctx, opts.runOptions)
	if err != nil && stdErrors.Is(err, errControlLocalTLSUnavailable) {
		slog.Warn("Local TLS parameters unavailable; fallback to insecure local execution state activated",
			slog.String("error", err.Error()),
		)
		return nil
	} else if err != nil {
		errWrap := fmt.Errorf("resolve control runtime: %w", err)
		emitBlockingError("Blocking failure during control runtime resolution", errWrap, generateRequestID())
		return errWrap
	}

	controlServer := control.NewServer(controlRuntime.bindAddr, opts.configManager, opts.runtimeApplier)
	if controlServer == nil {
		slog.Warn("Control server allocation yielded nil structure; execution branching aborted")
		return nil
	}

	if opts.controlBearerToken == "" && controlRuntime.oauthConfig == nil {
		slog.Info("Architectural transition: Control server initializing without authentication middleware",
			slog.String("addr", controlRuntime.bindAddr),
			slog.Bool("dashboard_only", true),
		)
	}
	if opts.controlBearerToken != "" {
		controlServer.SetBearerToken(opts.controlBearerToken)
	}
	if opts.runtimeResolver != nil {
		controlServer.SetKnownBotInstanceIDs(
			knownBotInstanceCatalogSlice(
				knownBotInstanceCatalog(opts.runtimeResolver.getRuntimes(), nil),
			),
		)
	}
	controlServer.SetQOTDService(opts.qotdService)
	controlServer.SetModerationMetrics(opts.moderationMetrics)
	controlServer.SetCacheObservability(func() *cache.UnifiedCache {
		if opts.runtimeResolver == nil {
			return nil
		}
		caches := opts.runtimeResolver.aggregateUnifiedCaches()
		if len(caches) == 0 {
			return nil
		}
		// For dashboard simplicity, just return the first one found until UI handles aggregates.
		for _, c := range caches {
			return c
		}
		return nil
	}, opts.store)
	controlServer.SetMonitoringMetricsResolver(func() monitoring.Metrics {
		if opts.runtimeResolver == nil {
			return nil
		}
		metrics := opts.runtimeResolver.aggregateMonitoringMetrics()
		if len(metrics) == 0 {
			return nil
		}
		for _, m := range metrics {
			return m
		}
		return nil
	})
	controlServer.SetDiscordSessionResolver(func(guildID string) (*discordgo.Session, error) {
		return opts.runtimeResolver.sessionForGuild(guildID, "dashboard")
	})
	controlServer.SetBotGuildBindingsProvider(func(ctx context.Context) ([]control.BotGuildBinding, error) {
		return opts.runtimeResolver.guildBindings(ctx)
	})
	controlServer.SetGuildRegistrationResolver(func(ctx context.Context, guildID string) error {
		return opts.runtimeResolver.registerGuild(ctx, guildID)
	})
	if err := controlServer.SetPublicOrigin(controlRuntime.publicOrigin); err != nil {
		errWrap := fmt.Errorf("configure control public origin: %w", err)
		emitBlockingError("Failed to lock public origin for control server", errWrap, generateRequestID())
		return errWrap
	}
	if controlRuntime.tlsCertFile != "" || controlRuntime.tlsKeyFile != "" {
		if err := controlServer.SetTLSCertificates(controlRuntime.tlsCertFile, controlRuntime.tlsKeyFile); err != nil {
			errWrap := fmt.Errorf("configure control tls certificates: %w", err)
			emitBlockingError("Failed to bind TLS material to control server listener", errWrap, generateRequestID())
			return errWrap
		}
	}
	if controlRuntime.oauthConfig != nil {
		if err := controlServer.SetDiscordOAuthConfig(*controlRuntime.oauthConfig); err != nil {
			errWrap := fmt.Errorf("configure control discord oauth: %w", err)
			emitBlockingError("Failed to inject OAuth configuration into control server", errWrap, generateRequestID())
			return errWrap
		}
		slog.Info("Architectural transition: Discord OAuth constraints applied to control interface",
			slog.String("scopes", strings.Join(control.DiscordOAuthScopes(controlRuntime.oauthConfig.IncludeGuildsMembersRead), " ")),
		)
		if controlRuntime.tlsCertFile == "" || controlRuntime.tlsKeyFile == "" {
			slog.Warn("Misconfigured deployment topology: OAuth enforced without local TLS termination; secure cookies risk clearance drop")
		}
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	slog.Info("Architectural transition: Binding control server socket",
		slog.String("address", controlRuntime.bindAddr),
	)

	if err := controlServer.Start(); err != nil {
		if stdErrors.Is(err, control.ErrControlServerBind) {
			slog.Warn("Port allocation collision detected; control server bypassing listener initialization",
				slog.String("addr", controlRuntime.bindAddr),
				slog.String("error", err.Error()),
			)
			return nil
		}
		errWrap := fmt.Errorf("start control server: %w", err)
		emitBlockingError("Critical failure during control server socket bind operation", errWrap, generateRequestID())
		return errWrap
	}

	if err := ctx.Err(); err != nil {
		slog.Warn("Startup context invalidated; executing compensatory teardown of control server")
		stopCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if stopErr := controlServer.Stop(stopCtx); stopErr != nil {
			emitBlockingError("Teardown failure during aborted startup sequence", stopErr, generateRequestID())
		}
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	if opts.controlServerRegistry != nil {
		opts.controlServerRegistry.Set(controlServer)
	}
	return nil
}
