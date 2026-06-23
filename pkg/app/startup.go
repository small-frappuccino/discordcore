package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
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
	// Removido o log.EmitBlockingError e o debug.Stack()
	return resolvedDatabaseBootstrap{}, fmt.Errorf("postgres bootstrap config unavailable: set %s before startup", databaseURLEnv)
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
		// Retorna o erro puramente envelopado
		return fmt.Errorf("persist runtime database config: %w", err)
	}

	slog.Info("Architectural state transition: Database bootstrap configuration synchronized successfully")
	return nil
}

type controlServerHolder struct {
	server atomic.Pointer[control.Server]
}

// Set updates the held control server reference safely.
func (h *controlServerHolder) Set(server *control.Server) {
	if h == nil || server == nil {
		return
	}
	slog.Debug("Updating control server reference in memory holder")
	h.server.Store(server)
}

// Stop safely shuts down the held control server if one is active.
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
	runtime *botRuntime,
	configManager *files.ConfigManager,
	startupTasks *StartupTaskOrchestrator,
) {
	if runtime == nil || runtime.legacySession == nil || configManager == nil {
		return
	}

	run := func(context.Context) error {
		err := files.LogConfiguredGuildsForBot(configManager, runtime.legacySession, runtime.instanceID)
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
	sessionResolver func(guildID string) *session.LegacySession,
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

			if err := webhook.PatchMessageEmbed(ctx, &webhook.ArikawaAPI{}, webhook.MessageEmbedPatch{
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
			log.EmitBlockingError("Synchronous execution of control server startup failed completely", err, log.GenerateRequestID())
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

var (
	openBotRuntimeFn       = openBotRuntime
	initializeBotRuntimeFn = initializeBotRuntime
	shutdownBotRuntimeFn   = shutdownBotRuntime
)

// ResolveRuntimeStartupParallelism calculates the optimal concurrency limit.
func ResolveRuntimeStartupParallelism(runtimeCount int) int {
	if runtimeCount <= 1 {
		return 1
	} else if runtimeCount == 2 {
		return 2
	} else {
		return 3
	}
}

// ResolveRuntimeBackgroundParallelism calculates the optimal concurrency limit for generic background tasks.
func ResolveRuntimeBackgroundParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 1
	default:
		return 2
	}
}

// ResolveStartupLightParallelism computes the concurrency limit for non-blocking I/O tasks during startup.
func ResolveStartupLightParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 2
	case runtimeCount == 2:
		return 3
	default:
		return 4
	}
}

// ResolveStartupLightQueueSize determines the maximum queued capacity for light startup operations.
func ResolveStartupLightQueueSize(runtimeCount int) int {
	if runtimeCount <= 1 {
		return 4
	} else if runtimeCount == 2 {
		return 6
	} else {
		return runtimeCount * 2
	}
}

// RuntimeStartupBackgroundWorker manages a bounded worker pool for executing asynchronous initialization routines.
type RuntimeStartupBackgroundWorker struct {
	ctx          context.Context
	cancel       context.CancelFunc
	group        *errgroup.Group
	queue        chan func(context.Context) error
	dispatchDone chan struct{}
	shutdownOnce sync.Once
}

// NewRuntimeStartupBackgroundWorker initializes a generic worker pool based on the detected runtime count.
func NewRuntimeStartupBackgroundWorker(runtimeCount int) *RuntimeStartupBackgroundWorker {
	return NewRuntimeStartupBackgroundWorkerWithLimits(
		ResolveRuntimeBackgroundParallelism(runtimeCount),
		runtimeCount,
	)
}

// NewRuntimeStartupBackgroundWorkerWithLimits creates a custom background worker with explicit concurrency and queue limits.
func NewRuntimeStartupBackgroundWorkerWithLimits(parallelism, queueSize int) *RuntimeStartupBackgroundWorker {
	ctx, cancel := context.WithCancel(context.Background())
	group, groupCtx := errgroup.WithContext(ctx)
	if parallelism <= 0 {
		parallelism = 1
	}
	group.SetLimit(parallelism)

	if queueSize <= 0 {
		queueSize = 1
	}

	worker := &RuntimeStartupBackgroundWorker{
		ctx:          groupCtx,
		cancel:       cancel,
		group:        group,
		queue:        make(chan func(context.Context) error, queueSize),
		dispatchDone: make(chan struct{}),
	}

	slog.Info("Architectural state transition: Background worker pool initialized",
		slog.Int("parallelism_limit", parallelism),
		slog.Int("queue_capacity", queueSize),
	)

	go worker.dispatch()
	return worker
}

// StartupTaskOrchestrator coordinates multi-tier asynchronous execution pipelines during the boot phase.
type StartupTaskOrchestrator struct {
	light *RuntimeStartupBackgroundWorker
	heavy *RuntimeStartupBackgroundWorker
}

// NewStartupTaskOrchestrator allocates a tiered worker pool system to manage concurrent startup sequences.
func NewStartupTaskOrchestrator(runtimeCount int) *StartupTaskOrchestrator {
	slog.Info("Architectural state transition: Startup task orchestrator instantiated",
		slog.Int("runtime_count_heuristic", runtimeCount),
	)

	return &StartupTaskOrchestrator{
		light: NewRuntimeStartupBackgroundWorkerWithLimits(
			ResolveStartupLightParallelism(runtimeCount),
			ResolveStartupLightQueueSize(runtimeCount),
		),
		heavy: NewRuntimeStartupBackgroundWorker(runtimeCount),
	}
}

// GoLight enqueues a non-blocking asynchronous routine into the high-throughput, low-latency execution tier.
func (o *StartupTaskOrchestrator) GoLight(name string, fn func(context.Context) error) {
	if o == nil {
		return
	}
	o.goTask(o.light, name, "light", fn)
}

// GoHeavy enqueues an intensive asynchronous routine into the constrained, high-latency execution tier.
func (o *StartupTaskOrchestrator) GoHeavy(name string, fn func(context.Context) error) {
	if o == nil {
		return
	}
	o.goTask(o.heavy, name, "heavy", fn)
}

func (o *StartupTaskOrchestrator) goTask(worker *RuntimeStartupBackgroundWorker, name, kind string, fn func(context.Context) error) {
	if o == nil || worker == nil || fn == nil {
		return
	}

	slog.Debug("Tracking complex conditional branch: Injecting closure into orchestrator queue",
		slog.String("task_name", name),
		slog.String("queue_tier", kind),
	)

	worker.Go(func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			if ctx.Err() != nil {
				slog.Debug("Tracking complex conditional branch: Task execution halted via context cancellation",
					slog.String("task_name", name),
				)
				return nil
			}
			slog.Warn("Mitigated service degradation: Background startup task encountered an error and aborted",
				slog.String("task", name),
				slog.String("kind", kind),
				slog.String("error", err.Error()),
			)
		}
		return nil
	})
}

// Shutdown triggers a graceful teardown of all managed execution pipelines, blocking until completion or context expiration.
func (o *StartupTaskOrchestrator) Shutdown(ctx context.Context) error {
	if o == nil {
		return nil
	}

	slog.Info("Architectural state transition: Halting startup orchestrator and draining worker pools")

	var errs []error
	// Error Bubbling Puro: Delegamos o tratamento do erro de encerramento para o caller.
	if o.light != nil {
		if err := o.light.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown light startup tasks: %w", err))
		}
	}
	if o.heavy != nil {
		if err := o.heavy.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown heavy startup tasks: %w", err))
		}
	}

	return stdErrors.Join(errs...)
}

// Go submits an asynchronous workload into the worker pool for execution.
func (w *RuntimeStartupBackgroundWorker) Go(fn func(context.Context) error) {
	if w == nil || fn == nil {
		return
	}
	select {
	case <-w.ctx.Done():
		slog.Debug("Tracking complex conditional branch: Task rejected, worker pool context already finalized")
		return
	case w.queue <- fn:
	}
}

// Shutdown signals the worker pool to reject new tasks and blocks until the active queue is fully drained.
func (w *RuntimeStartupBackgroundWorker) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	w.shutdownOnce.Do(func() {
		slog.Debug("Tracking complex conditional branch: Broadcasting cancellation signal across worker goroutines")
		if w.cancel != nil {
			w.cancel()
		}
	})

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		select {
		case <-w.dispatchDone:
		case <-egCtx.Done():
			return egCtx.Err()
		}

		waitCh := make(chan error, 1)
		go func() { waitCh <- w.group.Wait() }()
		select {
		case err := <-waitCh:
			return err
		case <-egCtx.Done():
			return egCtx.Err()
		}
	})

	err := eg.Wait()

	// Filtra eventos normais de ciclo de vida (Canceled/DeadlineExceeded) de anomalias estruturais.
	if err != nil && !stdErrors.Is(err, context.Canceled) && !stdErrors.Is(err, context.DeadlineExceeded) {
		slog.Warn("Mitigated service degradation: Anomalous failure during worker pool drain",
			slog.String("error", err.Error()),
		)
	}

	return err
}

func (w *RuntimeStartupBackgroundWorker) dispatch() {
	defer close(w.dispatchDone)

	for {
		select {
		case <-w.ctx.Done():
			slog.Debug("Tracking complex conditional branch: Dispatcher loop terminating via context closure")
			for {
				select {
				case fn := <-w.queue:
					if fn != nil {
						_ = fn(w.ctx)
					}
				default:
					return
				}
			}
		case fn := <-w.queue:
			if fn == nil {
				continue
			}
			w.group.Go(func() error {
				if err := w.ctx.Err(); err != nil {
					return nil
				}
				if err := fn(w.ctx); err != nil {
					if w.ctx.Err() != nil {
						return nil
					}
					return fmt.Errorf("RuntimeStartupBackgroundWorker.dispatch: %w", err)
				}
				return nil
			})
		}
	}
}
