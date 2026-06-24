package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"golang.org/x/sync/errgroup"
)

const (
	defaultControlAddr                            = "127.0.0.1:8376"
	defaultControlDiscordOAuthClientID            = "1396606252506681395"
	controlBearerTokenEnv                         = "DISCORDCORE_CONTROL_BEARER_TOKEN"
	controlDiscordOAuthClientIDEnv                = "DISCORDCORE_CONTROL_DISCORD_OAUTH_CLIENT_ID"
	controlDiscordOAuthClientSecretEnv            = "DISCORDCORE_CONTROL_DISCORD_OAUTH_CLIENT_SECRET"
	controlDiscordOAuthRedirectURIEnv             = "DISCORDCORE_CONTROL_DISCORD_OAUTH_REDIRECT_URI"
	controlDiscordOAuthIncludeGuildMembersReadEnv = "DISCORDCORE_CONTROL_DISCORD_OAUTH_INCLUDE_GUILDS_MEMBERS_READ"
	controlDiscordOAuthSessionStorePathEnv        = "DISCORDCORE_CONTROL_DISCORD_OAUTH_SESSION_STORE_PATH"
	controlTLSCertFileEnv                         = "DISCORDCORE_CONTROL_TLS_CERT_FILE"
	controlTLSKeyFileEnv                          = "DISCORDCORE_CONTROL_TLS_KEY_FILE"
)

// App encapsulates the state of the initializing application process, providing
// a testable, instance-based context tree instead of procedural global variables.
type App struct {
	appName        string
	opts           RunOptions
	serviceManager *service.ServiceManager
	startupTasks   *StartupTaskOrchestrator
	logger         *slog.Logger

	store                 *postgres.Store
	configManager         *files.ConfigManager
	controlServerRegistry *controlServerHolder
	botSupervisor         *BotSupervisor
	runtimeResolver       *botRuntimeResolver
	runtimeApplier        *runtimeapply.Manager

	qotdService       *qotd.Service
	moderationMetrics *moderation.InMemoryMetrics
	membersMetrics    *members.InMemoryMetrics
	messagesMetrics   *messages.InMemoryMetrics

	cleanupCancel context.CancelFunc
}

// NewApp allocates the initial structural foundations for a bot runtime pipeline.
func NewApp(appName string, opts RunOptions) *App {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.StoreCloseHook == nil {
		opts.StoreCloseHook = func(c interface{ Close() error }) error { return c.Close() }
	}
	if opts.DiscordSessionCloseHook == nil {
		opts.DiscordSessionCloseHook = func(c interface{ Close() error }) error { return c.Close() }
	}
	if opts.ShutdownDelay == 0 {
		opts.ShutdownDelay = 100 * time.Millisecond
	}

	return &App{
		appName:        appName,
		opts:           opts,
		serviceManager: service.NewServiceManager(opts.Logger),
		logger:         opts.Logger,
	}
}

// Run bootstraps the bot with a unified flow and blocks until shutdown.
func Run(appName string) error {
	return RunWithOptions(appName, RunOptions{})
}

func RunWithOptions(appName string, opts RunOptions) (err error) {
	defer func() {
		log.GlobalLogger.Sync()
		log.CloseGlobalLogger()
	}()
	defer func() {
		if r := recover(); r != nil {
			// Unmanaged Panic: Exige interrupção agressiva e dump de memória (Stack Trace)
			errWrap := fmt.Errorf("panic recovered during runtime: %v", r)
			log.EmitBlockingError("Critical pipeline failure: Unhandled panic intercepted", errWrap, log.GenerateRequestID())
			notifyLifecycleEvent("fatal", errWrap.Error())
			err = errWrap
		} else if err != nil {
			// Managed Error: Falha validada e propagada. Usa O(1) structured logging sem stack trace.
			slog.Error("Primary execution routine aborted",
				slog.String("app_name", appName),
				slog.Any("error", err),
			)
			notifyLifecycleEvent("fatal", fmt.Sprintf("startup or runtime error: %v", err))
		} else {
			// Desligamento limpo
			notifyLifecycleEvent("stopping", "")
		}
	}()

	app := NewApp(appName, opts)
	ctx := context.Background()

	if bootErr := app.Boot(ctx); bootErr != nil {
		return app.Teardown(bootErr)
	}

	return app.Teardown(app.RunAndListen(ctx))
}

// Boot executes the application initialization matrix deterministically.
func (a *App) Boot(ctx context.Context) error {
	a.logger.Info("Architectural state transition: Executing application boot sequence")

	if err := a.InitializeIO(ctx); err != nil {
		return err
	}
	if err := a.ConstructServices(ctx); err != nil {
		return err
	}

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return a.serviceManager.StartAll()
	})

	eg.Go(func() error {
		if err := a.runtimeResolver.waitForReady(egCtx); err != nil {
			return err
		}

		controlBearerToken := strings.TrimSpace(files.EnvString(controlBearerTokenEnv, ""))
		scheduleStartupWebhookEmbedUpdates(a.startupTasks, a.configManager.Config(), func(guildID string) *session.LegacySession {
			sess, _ := a.runtimeResolver.sessionForGuild(guildID, "")
			return sess
		})
		scheduleControlServerStartup(a.startupTasks, controlStartupTaskOptions{
			runOptions:            a.opts,
			configManager:         a.configManager,
			runtimeApplier:        a.runtimeApplier,
			controlBearerToken:    controlBearerToken,
			runtimeResolver:       a.runtimeResolver,
			store:                 a.store,
			qotdService:           a.qotdService,
			moderationMetrics:     a.moderationMetrics,
			membersMetrics:        a.membersMetrics,
			messagesMetrics:       a.messagesMetrics,
			controlServerRegistry: a.controlServerRegistry,
		})
		slog.Info("Architectural state transition: Command tree sync complete")
		return nil
	})

	return eg.Wait()
}

// InitializeIO establishes critical state boundaries across filesystems and storage.
func (a *App) InitializeIO(ctx context.Context) error {
	if err := idgen.Init(1); err != nil {
		return fmt.Errorf("initialize idgen: %w", err)
	}

	notifyLifecycleEvent("starting", "")

	msg := formatStartupMessage(a.appName, AppVersion(), Version)
	slog.Info("Architectural state transition: Executing application binary",
		slog.String("version_info", msg),
	)

	databaseBootstrap, err := resolveDatabaseBootstrap()
	if err != nil {
		return fmt.Errorf("InitializeIO resolveDatabaseBootstrap: %w", err)
	}
	slog.Info("Architectural state transition: Database matrix parameters loaded",
		slog.String("operation", "startup.database.bootstrap"),
		slog.String("source", databaseBootstrap.Source),
	)

	if err := files.EnsureCacheInitialized(); err != nil {
		slog.Warn("Mitigated service degradation: Sub-optimal filesystem state detected; executing local cache fallback",
			slog.String("error", err.Error()),
		)
	}
	if err := files.EnsureCacheDirs(); err != nil {
		return fmt.Errorf("create cache directories: %w", err)
	}

	store, configManager, err := setupStorage(databaseBootstrap)
	if err != nil {
		return fmt.Errorf("InitializeIO setupStorage: %w", err)
	}
	a.store = store
	a.configManager = configManager

	applyConfiguredTheme(a.configManager)

	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	scheduleDBCleanup(cleanupCtx, a.store, a.configManager)
	a.cleanupCancel = cleanupCancel

	return nil
}

// ConstructServices assembles the runtime domain logic elements and their dependency graph.
func (a *App) ConstructServices(ctx context.Context) error {
	a.runtimeApplier = runtimeapply.New(nil, nil)
	if cfg := a.configManager.Config(); cfg != nil {
		a.runtimeApplier.SetInitial(cfg.RuntimeConfig)
	}

	// Cômputo achatado e in-line de instâncias ativas
	runtimeCount := 1 // Default estrito
	if cfg := a.configManager.Config(); cfg != nil {
		knownInstances := make(map[string]struct{})
		for _, guild := range cfg.Guilds {
			for instanceID, token := range guild.BotInstanceTokens {
				if string(token) != "" {
					knownInstances[instanceID] = struct{}{}
				}
			}
		}
		if len(knownInstances) > 0 {
			runtimeCount = len(knownInstances)
		}
	}

	a.controlServerRegistry = &controlServerHolder{}
	a.startupTasks = NewStartupTaskOrchestrator(runtimeCount)

	qotdMetrics := qotd.NopMetrics{}
	qotdService := qotd.NewServiceWithMetrics(a.configManager, a.store, nil, qotdMetrics)

	appClock := clock.NewHTTPClock("https://discord.com")
	qotdService.SetClock(appClock)

	a.moderationMetrics = &moderation.InMemoryMetrics{}
	a.membersMetrics = members.NewInMemoryMetrics()
	a.messagesMetrics = messages.NewInMemoryMetrics()
	a.qotdService = qotdService

	storeService := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "postgres-store",
		Type:     service.TypeCache,
		Priority: service.PriorityHigh,
		Start:    func(context.Context) error { return nil },
		Stop: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.opts.ShutdownDelay):
			}
			return a.opts.StoreCloseHook(a.store)
		},
		Logger: a.logger,
	})
	if err := a.serviceManager.Register(storeService); err != nil {
		return fmt.Errorf("register store service: %w", err)
	}

	embedService := embeds.NewEmbedService(a.configManager)
	rolePanelService := roles.NewRolePanelService(a.configManager)
	partnerService := partners.NewPartnerService(a.configManager)

	botOpts := botRuntimeOptions{
		runtimeCount:             runtimeCount,
		configManager:            a.configManager,
		store:                    a.store,
		commandCatalogRegistrars: a.opts.CommandCatalogRegistrars,
		runtimeApplier:           a.runtimeApplier,
		qotdCommandService:       qotdService,
		moderationMetrics:        a.moderationMetrics,
		membersMetrics:           a.membersMetrics,
		messagesMetrics:          a.messagesMetrics,
		startupTasks:             a.startupTasks,
		profile:                  a.opts.Profile,
		appClock:                 appClock,
		controlServerRegistry:    a.controlServerRegistry,
		logger:                   a.logger,
		embedService:             embedService,
		rolePanelService:         rolePanelService,
		partnerService:           partnerService,
		openBotArikawaState:      a.opts.openBotArikawaState,
		fetchBotArikawaMe:        a.opts.fetchBotArikawaMe,
		discordSessionCloseHook:  a.opts.DiscordSessionCloseHook,
		newCommandHandlerForBot:  a.opts.newCommandHandlerForBot,
		newCommandHandler:        a.opts.newCommandHandler,
		setupCommandHandler:      a.opts.setupCommandHandler,
		shutdownCommandHandler:   a.opts.shutdownCommandHandler,
	}

	a.botSupervisor = NewBotSupervisor(a.configManager, botOpts)
	qotdService.SetPublisher(NewArikawaQOTDPublisher(a.botSupervisor.GetResolver()))
	a.configManager.AddSubscriber(a.botSupervisor.onConfigChanged)

	a.botSupervisor.SetFatalCallback(func(err error) {
		a.serviceManager.Fatal(err)
	})

	botSupervisorService := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "bot-supervisor",
		Type:     service.TypeMonitoring,
		Priority: service.PriorityNormal,
		Start: func(context.Context) error {
			return a.botSupervisor.Start()
		},
		Stop: func(ctx context.Context) error {
			return a.botSupervisor.Stop(ctx)
		},
		Logger: a.logger,
	})

	if err := a.serviceManager.Register(botSupervisorService); err != nil {
		return fmt.Errorf("register bot supervisor service: %w", err)
	}

	a.runtimeResolver = a.botSupervisor.GetResolver()

	attachCtx, attachCancel := context.WithTimeout(ctx, 5*time.Second)
	defer attachCancel()
	if err := a.moderationMetrics.Attach(attachCtx); err != nil {
		return fmt.Errorf("fatal abort: moderation metrics pipeline failed to attach: %w", err)
	}

	return nil
}

// RunAndListen hooks context lifecycles to OS events, averting complex goto flow control.
func (a *App) RunAndListen(ctx context.Context) error {
	signalCtx, stopSignal := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	rootCtx, rootCancel := context.WithCancel(signalCtx)
	defer rootCancel()

	sigHupCh := make(chan os.Signal, 1)
	signal.Notify(sigHupCh, syscall.SIGHUP)
	defer signal.Stop(sigHupCh)

	eg, egCtx := errgroup.WithContext(rootCtx)

	// Phase 2: SIGHUP Valve & Serialized Mutation Pipeline
	// Dedicated resident worker executing continuous state routing with O(1) thermodynamic footprint.
	eg.Go(func() error {
		for {
			select {
			case <-egCtx.Done():
				return nil
			case <-sigHupCh:
				a.logger.Debug("Dynamic instruction intercepted: Emitting non-blocking intent trigger for configuration layer reload")

				// Mutação serializada governada por CSP e limitação de timeout (Phase 2)
				mutCtx, mutCancel := context.WithTimeout(egCtx, 30*time.Second)

				newCfg, needsSave, err := a.configManager.LoadConfigFromStore()
				if err != nil {
					slog.Warn("Mitigated service degradation: Live configuration mutation failed; enforcing active baseline",
						slog.String("error", err.Error()),
					)
					mutCancel()
					continue
				}

				if mutCtx.Err() != nil {
					mutCancel()
					continue
				}

				dupCount := a.configManager.ApplyConfig(newCfg)

				if dupCount == 0 && !needsSave {
					slog.Info("Architectural state transition: Configuration topology refreshed directly from disk")
					mutCancel()
					continue
				}

				if saveErr := a.configManager.SaveConfig(); saveErr != nil {
					log.EmitBlockingError("Structural state failure: Volatile configuration drift blocks persistence flush", saveErr, log.GenerateRequestID())
				} else {
					slog.Info("Architectural state transition: Configuration topology updated and indexes rebuilt",
						slog.Int("duplicates_purged", dupCount),
					)
				}
				mutCancel()
			}
		}
	})

	// Phase 1: Anchor asynchronous execution routes to the strict limits of an errgroup.Group
	// to annihilate any incidence of naked goroutines in the execution matrix.
	eg.Go(func() error {
		err := a.serviceManager.Wait()
		if err != nil {
			log.EmitBlockingError("Critical pipeline failure: Daemon cluster collapsed", err, log.GenerateRequestID())
			rootCancel() // Force cascade teardown across the context tree
			return err
		}
		return nil
	})

	// Phase 3: Central router block for deterministic lifecycle observation
	eg.Go(func() error {
		select {
		case <-signalCtx.Done():
			a.logger.Info("Architectural state transition: Process termination signal acknowledged. Initiating graceful teardown.")
			rootCancel()
			// Unblock a.serviceManager.Wait() dynamically by initiating the graceful stop sequence
			_ = a.serviceManager.StopAll(context.Background())
			return nil
		case <-a.opts.TestShutdownCh:
			a.logger.Info("Architectural state transition: Test simulated shutdown initiated")
			rootCancel()
			// Unblock a.serviceManager.Wait() dynamically by initiating the graceful stop sequence
			_ = a.serviceManager.StopAll(context.Background())
			return nil
		case <-egCtx.Done():
			// Natural synchronized drainage due to sibling cancellation
			return nil
		}
	})

	// Asynchronous Teardown & Quiescent Memory Rest
	return eg.Wait()
}

// Teardown safely shuts down orchestrators and the database subsystem.
func (a *App) Teardown(originalErr error) error {
	if a == nil {
		return originalErr
	}

	slog.Info("Architectural state transition: Commencing teardown sequence across local orchestrators",
		slog.String("app_name", a.appName),
	)

	if a.cleanupCancel != nil {
		a.cleanupCancel()
	}

	if a.startupTasks != nil {
		shutdownStartupServices(a.startupTasks, a.controlServerRegistry, "Startup background tasks did not finish before shutdown")
	}

	if a.serviceManager != nil {
		err := a.serviceManager.StopAll(context.Background())
		a.serviceManager.Wait() // Executado incondicionalmente para limpar zumbis

		if err != nil {
			errWrap := fmt.Errorf("shutdown: %w", err)
			log.EmitBlockingError("Structural teardown failure: Zombie sub-processes detected during stop iteration", errWrap, log.GenerateRequestID())
			if originalErr != nil {
				return stdErrors.Join(originalErr, errWrap)
			}
			return errWrap
		}
	}

	return originalErr
}

func applyConfiguredTheme(configManager *files.ConfigManager) {
	cfg := configManager.Config()
	themeName := ""
	if cfg != nil {
		themeName = cfg.RuntimeConfig.BotTheme
	}

	if err := files.ConfigureThemeFromConfig(themeName); err != nil {
		slog.Warn("Mitigated service degradation: Client interface theme rejected context; reverting visual defaults",
			slog.String("theme_name", themeName),
			slog.String("error", err.Error()),
		)
	}
	if themeName == "" {
		if err := files.SetTheme(""); err != nil {
			slog.Warn("Mitigated service degradation: Hard fallback visual theme application failed",
				slog.String("error", err.Error()),
			)
		} else {
			slog.Info("Architectural state transition: Standard UI theme locked")
		}
	}
}

func scheduleDBCleanup(ctx context.Context, store *postgres.Store, configManager *files.ConfigManager) {
	cfg := configManager.Config()
	var features files.ResolvedFeatureToggles
	var disableCleanup bool

	if cfg != nil {
		features = cfg.ResolveFeatures("")
		disableCleanup = cfg.RuntimeConfig.DisableDBCleanup
	} else {
		features = (&files.BotConfig{}).ResolveFeatures("")
	}

	cleanupEnabled := features.Maintenance.DBCleanup

	slog.Debug("Evaluating temporal garbage collection routines",
		slog.Bool("cleanup_enabled", cleanupEnabled),
		slog.Bool("disable_cleanup_flag", disableCleanup),
	)

	// Avaliação Estrita O(1) de Ativação
	if cleanupEnabled && !disableCleanup {
		cache.SchedulePeriodicCleanup(ctx, store, 6*time.Hour)
		return
	}

	// Avaliação de Logs Desacoplada e Clara
	if !cleanupEnabled {
		slog.Info("Architectural state override: Database garbage collection suppressed explicitly by node definition",
			slog.String("flag", "features.maintenance.db_cleanup"),
		)
	} else {
		slog.Info("Architectural state override: Database garbage collection suppressed globally by configuration override",
			slog.String("flag", "disable_db_cleanup"),
		)
	}
}

func resolveRuntimeCapabilities(configSnapshot *files.BotConfig, botInstances []resolvedBotInstance, profile RunProfile) map[string]botRuntimeCapabilities {
	capabilities := make(map[string]botRuntimeCapabilities, len(botInstances))
	for _, instance := range botInstances {
		cap := resolveBotRuntimeCapabilities(
			configSnapshot,
			instance.ID,
		)

		capabilities[instance.ID] = cap
	}
	return capabilities
}

func shutdownStartupServices(startupTasks *StartupTaskOrchestrator, controlServerRegistry *controlServerHolder, tasksWarn string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if controlServerRegistry != nil {
		if err := controlServerRegistry.Stop(ctx); err != nil {
			log.EmitBlockingError("Structural teardown failure: Interface server socket release hung", err, log.GenerateRequestID())
		}
	}
	if startupTasks != nil {
		if err := startupTasks.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
			slog.Warn("Mitigated shutdown degradation: Async orchestrator missed synchronization lock",
				slog.String("warning_context", tasksWarn),
				slog.String("error", err.Error()),
			)
		}
	}
}

func formatStartupMessage(appName, appVersion, coreVersion string) string {
	appName = strings.TrimSpace(appName)
	appVersion = strings.TrimSpace(appVersion)
	coreVersion = strings.TrimSpace(coreVersion)

	msg := fmt.Sprintf("🚀 Starting %s", appName)
	if appVersion != "" {
		msg += fmt.Sprintf(" %s", appVersion)
	}

	if coreVersion == "" || (appVersion != "" && appVersion == coreVersion) {
		return msg + "..."
	}

	return msg + fmt.Sprintf(" (discordcore %s)...", coreVersion)
}

func setupStorage(dbb resolvedDatabaseBootstrap) (*postgres.Store, *files.ConfigManager, error) {
	dbCfg := dbb.Config
	dbc := persistence.Config{
		Driver:              dbCfg.Driver,
		DatabaseURL:         dbCfg.DatabaseURL,
		MaxOpenConns:        dbCfg.MaxOpenConns,
		MaxIdleConns:        dbCfg.MaxIdleConns,
		ConnMaxLifetimeSecs: dbCfg.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: dbCfg.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       dbCfg.PingTimeoutMS,
	}

	openCtx, openCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer openCancel()
	db, err := persistence.Open(openCtx, dbc)
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres database: %w", err)
	}
	slog.Info("Architectural state transition: Remote persistence pipeline materialized",
		slog.String("operation", "startup.database.open"),
		slog.String("driver", "postgres"),
	)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := persistence.Ping(pingCtx, db); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("postgres readiness check failed: %w", err)
	}
	slog.Info("Architectural state transition: I/O payload validation complete",
		slog.String("operation", "startup.database.ping"),
		slog.String("driver", "postgres"),
	)

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer migrateCancel()
	migrator := persistence.NewPostgresMigrator(db)
	if err := migrator.Up(migrateCtx); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("apply postgres migrations: %w", err)
	}
	slog.Info("Architectural state transition: Schema schema deltas propagated successfully",
		slog.String("operation", "startup.database.migrate"),
		slog.String("driver", "postgres"),
	)

	store, err := postgres.NewStore(db, slog.Default())
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("create postgres store: %w", err)
	}
	if err := store.Init(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("initialize postgres store: %w", err)
	}
	slog.Info("Architectural state transition: Virtual storage layers active",
		slog.String("operation", "startup.database.store_init"),
		slog.String("driver", "postgres"),
	)

	configStore := files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey, slog.Default())
	configManager := files.NewConfigManagerWithStore(configStore, slog.Default())

	slog.Debug("Executing cross-boundary extraction for master configuration tree")
	if err := configManager.LoadConfig(); err != nil {
		return nil, nil, fmt.Errorf("load config from postgres: %w", err)
	}
	if err := syncBootstrapDatabaseConfig(configManager, dbCfg); err != nil {
		return nil, nil, fmt.Errorf("sync runtime database bootstrap config: %w", err)
	}

	return store, configManager, nil
}
