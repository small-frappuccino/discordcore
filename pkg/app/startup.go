package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync/atomic"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"golang.org/x/sync/errgroup"
)

const (
	databaseDriverEnv              = "DISCORDCORE_DATABASE_DRIVER"
	databaseURLEnv                 = "DISCORDCORE_DATABASE_URL"
	databaseMaxOpenConnsEnv        = "DISCORDCORE_DATABASE_MAX_OPEN_CONNS"
	databaseMaxIdleConnsEnv        = "DISCORDCORE_DATABASE_MAX_IDLE_CONNS"
	databaseConnMaxLifetimeSecsEnv = "DISCORDCORE_DATABASE_CONN_MAX_LIFETIME_SECS"
	databaseConnMaxIdleTimeSecsEnv = "DISCORDCORE_DATABASE_CONN_MAX_IDLE_TIME_SECS"
	databasePingTimeoutMSEnv       = "DISCORDCORE_DATABASE_PING_TIMEOUT_MS"
)

type resolvedDatabaseBootstrap struct {
	Config files.DatabaseRuntimeConfig
	Source string
}

func resolveDatabaseBootstrap() (resolvedDatabaseBootstrap, error) {
	if cfg, ok := databaseBootstrapFromEnv(); ok {
		return resolvedDatabaseBootstrap{
			Config: cfg,
			Source: "env",
		}, nil
	}
	panic("hardware-aligned validation failure: postgres bootstrap config unavailable: set DISCORDCORE_DATABASE_URL before startup")
}

func databaseBootstrapFromEnv() (files.DatabaseRuntimeConfig, bool) {
	url := files.EnvString(databaseURLEnv, "")
	if url == "" {
		slog.Debug("Granular inspection: Database environment variable absent, skipping payload injection",
			slog.String("env", databaseURLEnv),
		)
		return files.DatabaseRuntimeConfig{}, false
	}

	driver := files.EnvString(databaseDriverEnv, "postgres")
	maxOpen := int(files.EnvInt64(databaseMaxOpenConnsEnv, 20))
	maxIdle := int(files.EnvInt64(databaseMaxIdleConnsEnv, 10))
	connMaxLifetime := int(files.EnvInt64(databaseConnMaxLifetimeSecsEnv, 1800))
	connMaxIdle := int(files.EnvInt64(databaseConnMaxIdleTimeSecsEnv, 300))
	pingTimeout := int(files.EnvInt64(databasePingTimeoutMSEnv, 5000))

	slog.Debug("Granular inspection: Database connection parameters injected via environment",
		slog.String("driver", driver),
		slog.Int("max_open_conns", maxOpen),
		slog.Int("max_idle_conns", maxIdle),
		slog.Int("conn_max_lifetime_secs", connMaxLifetime),
		slog.Int("conn_max_idle_time_secs", connMaxIdle),
		slog.Int("ping_timeout_ms", pingTimeout),
	)

	return files.DatabaseRuntimeConfig{
		Driver:              driver,
		DatabaseURL:         url,
		MaxOpenConns:        maxOpen,
		MaxIdleConns:        maxIdle,
		ConnMaxLifetimeSecs: connMaxLifetime,
		ConnMaxIdleTimeSecs: connMaxIdle,
		PingTimeoutMS:       pingTimeout,
	}, true
}

func syncBootstrapDatabaseConfig(configManager *files.ConfigManager, cfg files.DatabaseRuntimeConfig) error {
	if configManager == nil {
		return fmt.Errorf("config manager is nil")
	}

	current := configManager.SnapshotConfig().RuntimeConfig.Database
	if current == cfg {
		slog.Debug("Tracking complex conditional branch: Database configuration identical to persisted state, bypassing update")
		return nil
	}

	_, err := configManager.UpdateRuntimeConfig(func(rc *files.RuntimeConfig) error {
		rc.Database = cfg
		return nil
	})
	if err != nil {
		return fmt.Errorf("persist runtime database config: %w", err)
	}

	slog.Info("Architectural state transition: Database bootstrap configuration synchronized successfully")
	return nil
}

type controlServerHolder struct {
	server atomic.Pointer[control.Server]
}

func (h *controlServerHolder) Set(server *control.Server) {
	if h == nil || server == nil {
		return
	}
	slog.Debug("Updating control server reference in memory holder")
	h.server.Store(server)
}

func (h *controlServerHolder) Stop(ctx context.Context) error {
	if h == nil {
		return nil
	}

	server := h.server.Swap(nil)
	if server == nil {
		return nil
	}

	slog.Info("Planned shutdown of control server instance initiated")
	if err := server.Stop(ctx); err != nil {
		log.EmitBlockingError("Blocking failure during control server shutdown", err, log.GenerateRequestID())
		return err
	}

	slog.Info("Planned shutdown of control server instance completed successfully")
	return nil
}

func (h *controlServerHolder) BroadcastGuildEvent(guildID string, botPresent bool) {
	if h == nil {
		return
	}
	server := h.server.Load()
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
	store                 *postgres.Store
	qotdService           *qotd.Service
	moderationMetrics     moderation.Metrics
	membersMetrics        members.Metrics
	messagesMetrics       messages.Metrics
	controlServerRegistry *controlServerHolder
}

func scheduleRuntimeConfiguredGuildLogging(
	ctx context.Context,
	runtime *botRuntime,
	configManager *files.ConfigManager,
	startupTasks *StartupTaskOrchestrator,
) {
	if runtime == nil || runtime.legacySession == nil || configManager == nil {
		return
	}

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleRuntimeConfiguredGuildLogging")
	}

	startupTasks.Go("log_configured_guilds:"+runtime.instanceID, func(taskCtx context.Context) error {
		if taskCtx.Err() != nil {
			return nil
		}
		err := files.LogConfiguredGuildsForBot(configManager, runtime.legacySession, runtime.instanceID)
		if err != nil {
			slog.Warn("Mitigated degradation: Some configured guilds could not be accessed during runtime logging",
				slog.String("botInstanceID", runtime.instanceID),
				slog.String("error", err.Error()),
			)
		}
		return nil
	})
}
func scheduleStartupWebhookEmbedUpdates(
	startupTasks *StartupTaskOrchestrator,
	cfg *files.BotConfig,
	sessionResolver func(guildID string) *session.LegacySession,
) {
	if cfg == nil || sessionResolver == nil {
		return
	}

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleStartupWebhookEmbedUpdates")
	}

	startupTasks.Go("startup_webhook_embed_updates", func(taskCtx context.Context) error {
		for _, item := range collectStartupWebhookEmbedUpdates(cfg) {
			if err := taskCtx.Err(); err != nil {
				return fmt.Errorf("scheduleStartupWebhookEmbedUpdates: %w", err)
			}

			operation := fmt.Sprintf("runtime_config.webhook_embed_updates[%s:%d]", item.scope, item.index)
			sess := sessionResolver(item.scope)
			if sess == nil {
				slog.Debug("Session resolution missed for webhook patch target; skipping",
					slog.String("operation", operation),
					slog.String("scope", item.scope),
				)
				continue
			}

			if err := webhook.PatchMessageEmbed(taskCtx, &webhook.ArikawaAPI{}, webhook.MessageEmbedPatch{
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
	})
}
func scheduleControlServerStartup(startupTasks *StartupTaskOrchestrator, opts controlStartupTaskOptions) {
	if opts.runOptions.DisableControl {
		slog.Info("Architectural transition: Control server startup bypassed via explicit run options")
		return
	}

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleControlServerStartup")
	}

	startupTasks.Go("control_server", func(taskCtx context.Context) error {
		return startControlServerStartupTask(taskCtx, opts)
	})
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
		log.EmitBlockingError("Blocking failure during control runtime resolution", errWrap, log.GenerateRequestID())
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
			slices.Collect(knownBotInstanceCatalogSeq(opts.runtimeResolver.getRuntimes(), nil)),
		)
	}
	controlServer.SetQOTDService(opts.qotdService)
	controlServer.SetModerationMetrics(opts.moderationMetrics)
	controlServer.SetMembersMetricsResolver(func() members.Metrics { return opts.membersMetrics })
	controlServer.SetMessagesMetricsResolver(func() messages.Metrics { return opts.messagesMetrics })
	controlServer.SetStorage(opts.store)
	controlServer.SetCacheObservability(func() *cache.UnifiedCache {
		if opts.runtimeResolver == nil {
			return nil
		}
		caches := opts.runtimeResolver.aggregateUnifiedCaches()
		if len(caches) == 0 {
			return nil
		}
		for _, c := range caches {
			return c
		}
		return nil
	}, opts.store)

	controlServer.SetArikawaStateResolver(func(guildID string) (*state.State, error) {
		return opts.runtimeResolver.arikawaStateForGuild(guildID, "dashboard")
	})
	controlServer.SetBotGuildBindingsProvider(func(ctx context.Context) ([]control.BotGuildBinding, error) {
		return opts.runtimeResolver.guildBindings(ctx)
	})
	controlServer.SetGuildRegistrationResolver(func(ctx context.Context, guildID string) error {
		return opts.runtimeResolver.registerGuild(ctx, guildID)
	})
	if err := controlServer.SetPublicOrigin(controlRuntime.publicOrigin); err != nil {
		errWrap := fmt.Errorf("configure control public origin: %w", err)
		log.EmitBlockingError("Failed to lock public origin for control server", errWrap, log.GenerateRequestID())
		return errWrap
	}
	if controlRuntime.tlsCertFile != "" || controlRuntime.tlsKeyFile != "" {
		if err := controlServer.SetTLSCertificates(controlRuntime.tlsCertFile, controlRuntime.tlsKeyFile); err != nil {
			errWrap := fmt.Errorf("configure control tls certificates: %w", err)
			log.EmitBlockingError("Failed to bind TLS material to control server listener", errWrap, log.GenerateRequestID())
			return errWrap
		}
	}
	if controlRuntime.oauthConfig != nil {
		if err := controlServer.SetDiscordOAuthConfig(*controlRuntime.oauthConfig); err != nil {
			errWrap := fmt.Errorf("configure control discord oauth: %w", err)
			log.EmitBlockingError("Failed to inject OAuth configuration into control server", errWrap, log.GenerateRequestID())
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
		log.EmitBlockingError("Critical failure during control server socket bind operation", errWrap, log.GenerateRequestID())
		return errWrap
	}

	if err := ctx.Err(); err != nil {
		slog.Warn("Startup context invalidated; executing compensatory teardown of control server")
		stopCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if stopErr := controlServer.Stop(stopCtx); stopErr != nil {
			log.EmitBlockingError("Teardown failure during aborted startup sequence", stopErr, log.GenerateRequestID())
		}
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	if opts.controlServerRegistry != nil {
		opts.controlServerRegistry.Set(controlServer)
	}
	return nil
}

func ResolveRuntimeStartupParallelism(runtimeCount int) int {
	if runtimeCount <= 1 {
		return 1
	} else if runtimeCount == 2 {
		return 2
	} else {
		return 3
	}
}

// StartupTaskOrchestrator unifies bounded concurrency via a strict errgroup.Group,
// eradicating heuristic routing fragmentation and manual worker pools.
type StartupTaskOrchestrator struct {
	eg  *errgroup.Group
	ctx context.Context
}

func NewStartupTaskOrchestrator(ctx context.Context, runtimeCount int) *StartupTaskOrchestrator {
	eg, egCtx := errgroup.WithContext(ctx)

	parallelism := runtimeCount * 2
	if parallelism <= 0 {
		parallelism = 2
	}
	eg.SetLimit(parallelism)

	slog.Info("Architectural state transition: Startup task orchestrator instantiated",
		slog.Int("concurrency_limit", parallelism),
	)

	return &StartupTaskOrchestrator{
		eg:  eg,
		ctx: egCtx,
	}
}

func (o *StartupTaskOrchestrator) Go(name string, t func(context.Context) error) {
	if o == nil || o.eg == nil || t == nil {
		return
	}

	slog.Debug("Tracking complex conditional branch: Injecting closure into orchestrator",
		slog.String("task_name", name),
	)

	o.eg.Go(func() error {
		if err := t(o.ctx); err != nil {
			if o.ctx.Err() != nil {
				slog.Debug("Tracking complex conditional branch: Task execution halted via context cancellation",
					slog.String("task_name", name),
				)
				return nil
			}
			slog.Warn("Mitigated service degradation: Background startup task encountered an error and aborted",
				slog.String("task", name),
				slog.String("error", err.Error()),
			)
		}
		return nil
	})
}

func (o *StartupTaskOrchestrator) Shutdown(ctx context.Context) error {
	if o == nil || o.eg == nil {
		return nil
	}

	slog.Info("Architectural state transition: Halting startup orchestrator and draining execution ring")

	done := make(chan error, 1)
	go func() {
		done <- o.eg.Wait()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
