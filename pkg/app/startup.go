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
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleRuntimeConfiguredGuildLogging")
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

			operation := fmt.Sprintf("runtime_config.webhook_embed_updates[%s:%d]", item.scope, item.index)
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
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleStartupWebhookEmbedUpdates")
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
		slog.Error("Blocking structural failure: startupTasks orchestrator is nil")
		panic("hardware-aligned validation failure: startupTasks cannot be nil during scheduleControlServerStartup")
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

func ResolveRuntimeStartupParallelism(runtimeCount int) int {
	if runtimeCount <= 1 {
		return 1
	} else if runtimeCount == 2 {
		return 2
	} else {
		return 3
	}
}

func ResolveRuntimeBackgroundParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 1
	default:
		return 2
	}
}

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

func ResolveStartupLightQueueSize(runtimeCount int) int {
	if runtimeCount <= 1 {
		return 4
	} else if runtimeCount == 2 {
		return 6
	} else {
		return runtimeCount * 2
	}
}

// RuntimeStartupBackgroundWorker implementa um pool estático com alocação O(1).
type RuntimeStartupBackgroundWorker struct {
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	queue        chan func(context.Context) error
	shutdownOnce sync.Once
	closed       atomic.Bool
}

func NewRuntimeStartupBackgroundWorker(runtimeCount int) *RuntimeStartupBackgroundWorker {
	return NewRuntimeStartupBackgroundWorkerWithLimits(
		ResolveRuntimeBackgroundParallelism(runtimeCount),
		runtimeCount,
	)
}

func NewRuntimeStartupBackgroundWorkerWithLimits(parallelism, queueSize int) *RuntimeStartupBackgroundWorker {
	ctx, cancel := context.WithCancel(context.Background())
	if parallelism <= 0 {
		parallelism = 1
	}
	if queueSize <= 0 {
		queueSize = 1
	}

	worker := &RuntimeStartupBackgroundWorker{
		ctx:    ctx,
		cancel: cancel,
		queue:  make(chan func(context.Context) error, queueSize),
	}

	slog.Info("Architectural state transition: Background worker pool initialized",
		slog.Int("parallelism_limit", parallelism),
		slog.Int("queue_capacity", queueSize),
	)

	// Inicialização estática determinística do pool de execução fixa
	worker.wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go worker.worker()
	}

	return worker
}

type StartupTaskOrchestrator struct {
	light *RuntimeStartupBackgroundWorker
	heavy *RuntimeStartupBackgroundWorker
}

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

func (o *StartupTaskOrchestrator) GoLight(name string, fn func(context.Context) error) {
	if o == nil {
		return
	}
	o.goTask(o.light, name, "light", fn)
}

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

func (o *StartupTaskOrchestrator) Shutdown(ctx context.Context) error {
	if o == nil {
		return nil
	}

	slog.Info("Architectural state transition: Halting startup orchestrator and draining worker pools")

	var errs []error
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

// Go insere workloads no canal de barramento de forma limpa.
func (w *RuntimeStartupBackgroundWorker) Go(fn func(context.Context) error) {
	if w == nil || fn == nil {
		return
	}

	// Verificação atômica rápida para evitar escritas em canais em fase final de teardown
	if w.closed.Load() {
		return
	}

	select {
	case <-w.ctx.Done():
		return
	case w.queue <- fn:
	}
}

// Shutdown drena o pool garantindo zero panics por fechamento assíncrono.
func (w *RuntimeStartupBackgroundWorker) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	w.shutdownOnce.Do(func() {
		slog.Debug("Tracking complex conditional branch: Initiating worker pool graceful isolation sequence")
		// 1. Bloqueia novas entradas a nível de API atômica imediatamente
		w.closed.Store(true)

		// 2. Sinaliza cancelamento para as tarefas em execução cortarem seu ciclo
		if w.cancel != nil {
			w.cancel()
		}

		// NOTA DE SEGURANÇA CONCORRENTE: Não fechamos o canal 'w.queue' aqui.
		// Como as goroutines produtoras podem estar presas no operador 'select' do método Go,
		// fechar o canal aqui induziria panics estocásticos por escrita em canal fechado.
		// Deixamos o canal aberto; os consumidores estáticos da goroutine sairão via sinalização
		// de canal ou expiração de contexto de qualquer forma, eliminando a corrida.
	})

	slog.Info("Architectural state transition: Draining worker pool via explicit synchronization barrier")
	w.wg.Wait() // O protocolo de Shutdown deve ser obrigatoriamente bloqueado

	return nil
}

func (w *RuntimeStartupBackgroundWorker) worker() {
	defer w.wg.Done()

	for {
		select {
		case <-w.ctx.Done():
			// Contexto global cancelado, encerra imediatamente o worker estático
			return
		case fn, ok := <-w.queue:
			if !ok {
				return
			}
			if fn != nil {
				// Executa a tarefa injetada usando o contexto encapsulado do worker pool
				if err := fn(w.ctx); err != nil {
					if w.ctx.Err() == nil {
						slog.Warn("Mitigated service degradation: Background startup task encountered an error",
							slog.String("error", err.Error()),
						)
					}
				}
			}
		}
	}
}
