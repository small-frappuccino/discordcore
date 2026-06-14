package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"strings"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

type controlServerHolder struct {
	mu     sync.Mutex
	server *control.Server
}

// Set sets.
func (h *controlServerHolder) Set(server *control.Server) {
	if h == nil || server == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.server = server
}

// Stop stops.
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
	return server.Stop(ctx)
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
			log.ErrorLoggerRaw().Error(
				"Some configured guilds could not be accessed",
				"botInstanceID", runtime.instanceID,
				"err", err,
			)
		}
		return nil
	}

	if startupTasks == nil {
		if err := run(context.Background()); err != nil {
			log.ApplicationLogger().Warn("Failed to log configured guilds", "err", err)
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
				continue
			}

			if err := webhook.PatchMessageEmbed(sess, webhook.MessageEmbedPatch{
				MessageID:  item.update.MessageID,
				WebhookURL: item.update.WebhookURL,
				Embed:      item.update.Embed,
			}); err != nil {
				log.ApplicationLogger().Warn(
					"Webhook embed patch failed",
					"operation", operation,
					"scope", item.scope,
					"messageID", strings.TrimSpace(item.update.MessageID),
					"error", err,
				)
			} else {
				log.ApplicationLogger().Info(
					"Webhook embed patch applied",
					"operation", operation,
					"scope", item.scope,
					"messageID", strings.TrimSpace(item.update.MessageID),
				)
			}
		}
		return nil
	}

	if startupTasks == nil {
		if err := run(context.Background()); err != nil {
			log.ApplicationLogger().Warn("Failed to schedule startup webhook embed updates", "err", err)
		}
		return
	}

	startupTasks.GoLight("startup_webhook_embed_updates", run)
}

func scheduleControlServerStartup(startupTasks *StartupTaskOrchestrator, opts controlStartupTaskOptions) {
	if opts.runOptions.DisableControl {
		log.ApplicationLogger().Info("Control server startup skipped; disabled by run options")
		return
	}

	run := func(ctx context.Context) error {
		return startControlServerStartupTask(ctx, opts)
	}

	if startupTasks == nil {
		if err := run(context.Background()); err != nil {
			log.ApplicationLogger().Warn("Control server startup failed", "err", err)
		}
		return
	}

	startupTasks.GoLight("control_server", run)
}

func startControlServerStartupTask(ctx context.Context, opts controlStartupTaskOptions) error {
	controlRuntime, err := resolveControlRuntime(ctx, opts.runOptions)
	if err != nil && stdErrors.Is(err, errControlLocalTLSUnavailable) {
		log.ApplicationLogger().Warn(
			"Embedded local control HTTPS is unavailable; continuing without control server",
			"err", err,
		)
		return nil
	} else if err != nil {
		return fmt.Errorf("resolve control runtime: %w", err)
	}

	controlServer := control.NewServer(controlRuntime.bindAddr, opts.configManager, opts.runtimeApplier)
	if controlServer == nil {
		log.ApplicationLogger().Warn("Control server disabled (invalid parameters)")
		return nil
	}

	if opts.controlBearerToken == "" && controlRuntime.oauthConfig == nil {
		log.ApplicationLogger().Info(
			"Control server authentication is not configured",
			"addr", controlRuntime.bindAddr,
			"dashboard_only", true,
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
		return fmt.Errorf("configure control public origin: %w", err)
	}
	if controlRuntime.tlsCertFile != "" || controlRuntime.tlsKeyFile != "" {
		if err := controlServer.SetTLSCertificates(controlRuntime.tlsCertFile, controlRuntime.tlsKeyFile); err != nil {
			return fmt.Errorf("configure control tls certificates: %w", err)
		}
	}
	if controlRuntime.oauthConfig != nil {
		if err := controlServer.SetDiscordOAuthConfig(*controlRuntime.oauthConfig); err != nil {
			return fmt.Errorf("configure control discord oauth: %w", err)
		}
		log.ApplicationLogger().Info(
			"Control server Discord OAuth enabled",
			"scopes", strings.Join(control.DiscordOAuthScopes(controlRuntime.oauthConfig.IncludeGuildsMembersRead), " "),
		)
		if controlRuntime.tlsCertFile == "" || controlRuntime.tlsKeyFile == "" {
			log.ApplicationLogger().Warn("Discord OAuth is enabled but control TLS certificate/key are not configured; ensure HTTPS termination in front of control server so Secure cookies persist")
		}
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	if err := controlServer.Start(); err != nil {
		if stdErrors.Is(err, control.ErrControlServerBind) {
			log.ApplicationLogger().Warn(
				"Control server unavailable; continuing without dashboard listener",
				"addr", controlRuntime.bindAddr,
				"err", err,
			)
			return nil
		}
		return fmt.Errorf("start control server: %w", err)
	}

	if err := ctx.Err(); err != nil {
		stopCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if stopErr := controlServer.Stop(stopCtx); stopErr != nil {
			log.ApplicationLogger().Warn("Control server stop failed during startup cancellation", "err", stopErr)
		}
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	if opts.controlServerRegistry != nil {
		opts.controlServerRegistry.Set(controlServer)
	}
	return nil
}
