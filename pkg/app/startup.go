package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/discord/webhook"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
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

type databaseConfigUpdater struct {
	cfg files.DatabaseRuntimeConfig
}

func (u databaseConfigUpdater) apply(rc *files.RuntimeConfig) error {
	rc.Database = u.cfg
	return nil
}

func syncBootstrapDatabaseConfig(configManager *files.ConfigManager, cfg files.DatabaseRuntimeConfig) error {
	if configManager == nil {
		return errors.New("cannot sync config without configManager")
	}

	current := configManager.SnapshotConfig().RuntimeConfig.Database
	if current == cfg {
		slog.Debug("Tracking complex conditional branch: Database configuration identical to persisted state, bypassing update")
		return nil
	}

	updater := databaseConfigUpdater{cfg: cfg}
	_, err := configManager.UpdateRuntimeConfig(updater.apply)
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

type RuntimeConfiguredGuildLoggingTask struct {
	runtime       *botRuntime
	configManager *files.ConfigManager
}

func (t RuntimeConfiguredGuildLoggingTask) Execute(taskCtx context.Context) error {
	if taskCtx.Err() != nil {
		return nil
	}
	err := files.LogConfiguredGuildsForBot(t.configManager, t.runtime.legacySession, t.runtime.instanceID)
	if err != nil {
		slog.Warn("Mitigated degradation: Some configured guilds could not be accessed during runtime logging",
			slog.String("botInstanceID", t.runtime.instanceID),
			slog.String("error", err.Error()),
		)
	}
	return nil
}

func (t RuntimeConfiguredGuildLoggingTask) Name() string {
	return "log_configured_guilds:" + t.runtime.instanceID
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

	startupTasks.Go(RuntimeConfiguredGuildLoggingTask{
		runtime:       runtime,
		configManager: configManager,
	})
}

type WebhookSessionResolver interface {
	SessionForGuild(guildID string, feature string) (*session.LegacySession, error)
}

type StartupWebhookEmbedUpdatesTask struct {
	cfg             *files.BotConfig
	sessionResolver WebhookSessionResolver
}

func (t StartupWebhookEmbedUpdatesTask) Execute(taskCtx context.Context) error {
	for _, item := range collectStartupWebhookEmbedUpdates(t.cfg) {
		if err := taskCtx.Err(); err != nil {
			return fmt.Errorf("scheduleStartupWebhookEmbedUpdates: %w", err)
		}

		operation := fmt.Sprintf("runtime_config.webhook_embed_updates[%s:%d]", item.scope, item.index)
		sess, err := t.sessionResolver.SessionForGuild(item.scope, "webhook")
		if err != nil || sess == nil {
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
}

func (t StartupWebhookEmbedUpdatesTask) Name() string {
	return "startup_webhook_embed_updates"
}

func scheduleStartupWebhookEmbedUpdates(
	startupTasks *StartupTaskOrchestrator,
	cfg *files.BotConfig,
	sessionResolver WebhookSessionResolver,
) {
	if cfg == nil || sessionResolver == nil {
		return
	}

	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleStartupWebhookEmbedUpdates")
	}

	startupTasks.Go(StartupWebhookEmbedUpdatesTask{
		cfg:             cfg,
		sessionResolver: sessionResolver,
	})
}

type ControlServerStartupTask struct {
	controlRuntime        resolvedControlRuntime
	configManager         *files.ConfigManager
	runtimeApplier        *runtimeapply.Manager
	controlServerRegistry *controlServerHolder
	serverOpts            []control.ServerOption
}

func (t ControlServerStartupTask) Execute(taskCtx context.Context) error {
	return startControlServerStartupTask(taskCtx, t.controlRuntime, t.configManager, t.runtimeApplier, t.controlServerRegistry, t.serverOpts...)
}

func (t ControlServerStartupTask) Name() string {
	return "control_server"
}

func scheduleControlServerStartup(startupTasks *StartupTaskOrchestrator, controlRuntime resolvedControlRuntime, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager, controlServerRegistry *controlServerHolder, serverOpts ...control.ServerOption) {
	if startupTasks == nil {
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleControlServerStartup")
	}

	startupTasks.Go(ControlServerStartupTask{
		controlRuntime:        controlRuntime,
		configManager:         configManager,
		runtimeApplier:        runtimeApplier,
		controlServerRegistry: controlServerRegistry,
		serverOpts:            serverOpts,
	})
}

func startControlServerStartupTask(ctx context.Context, controlRuntime resolvedControlRuntime, configManager *files.ConfigManager, runtimeApplier *runtimeapply.Manager, controlServerRegistry *controlServerHolder, serverOpts ...control.ServerOption) error {
	controlServer, err := control.NewServer(controlRuntime.bindAddr, configManager, runtimeApplier, serverOpts...)
	if err != nil {
		errWrap := fmt.Errorf("create control server: %w", err)
		log.EmitBlockingError("Blocking failure during control server allocation", errWrap, log.GenerateRequestID())
		return errWrap
	}
	if controlServer == nil {
		slog.Warn("Control server allocation yielded nil structure; execution branching aborted")
		return nil
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("startControlServerStartupTask: %w", err)
	}

	slog.Info("Architectural transition: Binding control server socket",
		slog.String("address", controlRuntime.bindAddr),
	)

	if err := controlServer.Start(); err != nil {
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

	if controlServerRegistry != nil {
		controlServerRegistry.Set(controlServer)
	}
	return nil
}

// ResolveRuntimeStartupParallelism determines the optimal parallel execution bound for startup tasks.
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

// NewStartupTaskOrchestrator instantiates a bounded concurrency manager.
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

type BootstrapTask interface {
	Execute(context.Context) error
	Name() string
}

func (o *StartupTaskOrchestrator) Go(task BootstrapTask) {
	if o == nil || o.eg == nil || task == nil {
		return
	}

	name := task.Name()

	if err := o.ctx.Err(); err != nil {
		slog.Warn("Architectural state transition: Startup orchestrator rejecting task due to context cancellation",
			slog.String("task_name", name),
			slog.String("error", err.Error()),
		)
		return
	}

	slog.Debug("Tracking complex conditional branch: Injecting closure into orchestrator",
		slog.String("task_name", name),
	)

	o.eg.Go(func() error {
		if err := task.Execute(o.ctx); err != nil {
			if o.ctx.Err() != nil {
				slog.Debug("Tracking complex conditional branch: Task execution halted via context cancellation",
					slog.String("task_name", name),
				)
				return err
			}
			slog.Warn("Mitigated service degradation: Background startup task encountered an error and aborted",
				slog.String("task", name),
				slog.String("error", err.Error()),
			)
			return err
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
