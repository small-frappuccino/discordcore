package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/small-frappuccino/discordgo"
	"golang.org/x/sync/errgroup"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

var (
	newDiscordSession            = session.NewDiscordSession
	newDiscordSessionWithIntents = session.NewDiscordSessionWithIntents
	newCommandHandler            = commands.NewCommandHandler
	newCommandHandlerForBot      = commands.NewCommandHandlerForBot
	setupCommandHandler          = func(ch *commands.CommandHandler) error { return ch.SetupCommands() }
	shutdownCommandHandler       = func(ch *commands.CommandHandler) error { return ch.Shutdown() }
	closeStore                   = func(c interface{ Close() error }) error { return c.Close() }
	closeDiscordSession          = func(c interface{ Close() error }) error { return c.Close() }
	openDiscordSession           = func(s interface{ Open() error }) error { return s.Open() }
	shutdownDelay                = time.Sleep
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

// Run bootstraps the bot with a unified flow and blocks until shutdown.
func Run(appName string) error {
	return RunWithOptions(appName, RunOptions{})
}

// RunWithOptions bootstraps the bot and allows hosts to override control-plane wiring.
func RunWithOptions(appName string, opts RunOptions) (err error) {
	defer func() {
		if r := recover(); r != nil {
			errWrap := fmt.Errorf("panic recovered during runtime: %v", r)
			log.EmitBlockingError("Critical pipeline failure: Unhandled panic intercepted", errWrap, log.GenerateRequestID())
			notifyLifecycleEvent("fatal", errWrap.Error())
			err = errWrap
		} else if err != nil {
			log.EmitBlockingError("Critical pipeline failure: Primary routine aborted", err, log.GenerateRequestID())
			notifyLifecycleEvent("fatal", fmt.Sprintf("startup or runtime error: %v", err))
		} else {
			notifyLifecycleEvent("stopping", "")
		}
	}()
	return runWithOptions(appName, opts)
}

func runWithOptions(appName string, opts RunOptions) error {
	started := time.Now()

	files.SetAppName(appName)

	if err := idgen.Init(1); err != nil {
		errWrap := fmt.Errorf("initialize idgen: %w", err)
		log.EmitBlockingError("Structural dependency failure: ID generator initialization aborted", errWrap, log.GenerateRequestID())
		return errWrap
	}

	if err := log.SetupLogger(files.EffectiveBotName(), files.GetLogFilePath()); err != nil {
		errWrap := fmt.Errorf("configure logger: %w", err)
		log.EmitBlockingError("Structural dependency failure: Core logging infrastructure aborted", errWrap, log.GenerateRequestID())
		return errWrap
	}
	defer log.GlobalLogger.Sync()

	notifyLifecycleEvent("starting", "")

	var runtimeApplier *runtimeapply.Manager

	msg := formatStartupMessage(appName, AppVersion(), Version)
	slog.Info("Architectural state transition: Executing application binary",
		slog.String("version_info", msg),
	)

	databaseBootstrap, err := resolveDatabaseBootstrap()
	if err != nil {
		errWrap := fmt.Errorf("RunWithOptions resolveDatabaseBootstrap: %w", err)
		log.EmitBlockingError("Structural dependency failure: Database manifest resolution aborted", errWrap, log.GenerateRequestID())
		return errWrap
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
		errWrap := fmt.Errorf("create cache directories: %w", err)
		log.EmitBlockingError("Structural dependency failure: Block storage provisioning aborted", errWrap, log.GenerateRequestID())
		return errWrap
	}

	store, configManager, err := setupStorage(databaseBootstrap)
	if err != nil {
		errWrap := fmt.Errorf("RunWithOptions setupStorage: %w", err)
		log.EmitBlockingError("Structural dependency failure: Storage manifold orchestration aborted", errWrap, log.GenerateRequestID())
		return errWrap
	}
	closeStoreOnReturn := true
	defer func() { rollbackStoreClose(closeStoreOnReturn, store) }()

	applyConfiguredTheme(configManager)

	cleanupStop := scheduleDBCleanup(store, configManager)
	defer func() {
		if cleanupStop != nil {
			close(cleanupStop)
		}
	}()

	runtimeApplier = runtimeapply.New(nil, nil)
	if cfg := configManager.Config(); cfg != nil {
		runtimeApplier.SetInitial(cfg.RuntimeConfig)
	}

	knownInstances := make(map[string]struct{})
	if cfg := configManager.Config(); cfg != nil {
		for _, guild := range cfg.Guilds {
			for instanceID, token := range guild.BotInstanceTokens {
				if string(token) != "" {
					knownInstances[instanceID] = struct{}{}
				}
			}
		}
	}
	runtimeCount := len(knownInstances)
	if runtimeCount == 0 {
		runtimeCount = 1
	}

	controlServerRegistry := &controlServerHolder{}
	startupTasks := NewStartupTaskOrchestrator(runtimeCount)
	defer shutdownStartupServices(startupTasks, controlServerRegistry, "Startup background tasks did not finish cleanly")

	qotdMetrics := &qotd.InMemoryMetrics{}
	qotdService := qotd.NewServiceWithMetrics(configManager, store, nil, qotdMetrics)

	appClock := clock.NewHTTPClock("https://discord.com")
	qotdService.SetClock(appClock)

	moderationMetrics := &moderation.InMemoryMetrics{}
	appServiceManager := service.NewServiceManager(slog.Default())

	storeService := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "postgres-store",
		Type:     service.TypeCache,
		Priority: service.PriorityHigh,
		Start:    func(context.Context) error { return nil },
		Stop: func(context.Context) error {
			shutdownDelay(100 * time.Millisecond)
			return closeStore(store)
		},
		Logger: slog.Default(),
	})
	if err := appServiceManager.Register(storeService); err != nil {
		errWrap := fmt.Errorf("register store service: %w", err)
		log.EmitBlockingError("Structural dependency failure: Subgraph registration aborted", errWrap, log.GenerateRequestID())
		return errWrap
	}

	botOpts := botRuntimeOptions{
		runtimeCount:             runtimeCount,
		configManager:            configManager,
		store:                    store,
		commandCatalogRegistrars: opts.CommandCatalogRegistrars,
		runtimeApplier:           runtimeApplier,
		qotdCommandService:       qotdService,
		qotdLifecycleService:     qotdService,
		moderationMetrics:        moderationMetrics,
		startupTasks:             startupTasks,
		profile:                  opts.Profile,
		appClock:                 appClock,
		controlServerRegistry:    controlServerRegistry,
	}

	botSupervisor := NewBotSupervisor(configManager, botOpts)
	qotdService.SetPublisher(newDualSDKPublisher(botSupervisor.GetResolver()))
	configManager.AddSubscriber(botSupervisor.onConfigChanged)

	botSupervisor.SetFatalCallback(func(err error) {
		appServiceManager.Fatal(err)
	})

	botSupervisorService := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:     "bot-supervisor",
		Type:     service.TypeMonitoring,
		Priority: service.PriorityNormal,
		Start: func(context.Context) error {
			return botSupervisor.Start()
		},
		Stop: func(ctx context.Context) error {
			return botSupervisor.Stop(ctx)
		},
		Logger: slog.Default(),
	})

	if err := appServiceManager.Register(botSupervisorService); err != nil {
		errWrap := fmt.Errorf("register bot supervisor service: %w", err)
		log.EmitBlockingError("Structural dependency failure: Supervisor allocation aborted", errWrap, log.GenerateRequestID())
		return errWrap
	}

	runtimeResolver := botSupervisor.GetResolver()

	attachCtx, attachCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer attachCancel()
	if err := qotdMetrics.Attach(attachCtx); err != nil {
		errWrap := fmt.Errorf("fatal abort: qotd metrics pipeline failed to attach: %w", err)
		log.EmitBlockingError("Structural dependency failure: Core metrics pipeline desync", errWrap, log.GenerateRequestID())
		return errWrap
	}
	if err := moderationMetrics.Attach(attachCtx); err != nil {
		errWrap := fmt.Errorf("fatal abort: moderation metrics pipeline failed to attach: %w", err)
		log.EmitBlockingError("Structural dependency failure: Moderation metrics pipeline desync", errWrap, log.GenerateRequestID())
		return errWrap
	}

	eg, egCtx := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		return appServiceManager.StartAll()
	})

	eg.Go(func() error {
		if err := runtimeResolver.waitForReady(egCtx); err != nil {
			return err
		}

		controlBearerToken := strings.TrimSpace(files.EnvString(controlBearerTokenEnv, ""))
		scheduleStartupWebhookEmbedUpdates(startupTasks, configManager.Config(), func(guildID string) *discordgo.Session {
			sess, _ := runtimeResolver.sessionForGuild(guildID, "")
			return sess
		})
		scheduleControlServerStartup(startupTasks, controlStartupTaskOptions{
			runOptions:            opts,
			configManager:         configManager,
			runtimeApplier:        runtimeApplier,
			controlBearerToken:    controlBearerToken,
			runtimeResolver:       runtimeResolver,
			store:                 store,
			qotdService:           qotdService,
			moderationMetrics:     moderationMetrics,
			controlServerRegistry: controlServerRegistry,
		})
		slog.Info("Architectural state transition: Command tree sync complete")
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	slog.Info("Architectural state transition: Main process operational matrix finalized",
		slog.String("app_name", appName),
		slog.Duration("boot_time", time.Since(started).Round(time.Millisecond)),
	)

	rootCtx, stopRoot := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopRoot()

	sigHupCh := make(chan os.Signal, 1)
	signal.Notify(sigHupCh, syscall.SIGHUP)

	fatalErrCh := make(chan error, 1)
	go func() {
		fatalErrCh <- appServiceManager.Wait()
	}()

	var runErr error
	for {
		select {
		case err := <-fatalErrCh:
			if err != nil {
				log.EmitBlockingError("Critical pipeline failure: Daemon cluster collapsed", err, log.GenerateRequestID())
				runErr = err
				goto shutdown
			}
			goto shutdown

		case <-rootCtx.Done():
			slog.Info("Architectural state transition: Process termination signal acknowledged. Initiating graceful teardown.")
			stopRoot()
			goto shutdown

		case <-sigHupCh:
			slog.Debug("Dynamic instruction intercepted: Evaluating configuration layer reload via SIGHUP")
			newCfg, needsSave, err := configManager.LoadConfigFromStore()
			if err != nil {
				slog.Warn("Mitigated service degradation: Live configuration mutation failed; enforcing active baseline",
					slog.String("error", err.Error()),
				)
				continue
			}

			dupCount := configManager.ApplyConfig(newCfg)

			if dupCount > 0 || needsSave {
				if saveErr := configManager.SaveConfig(); saveErr != nil {
					log.EmitBlockingError("Structural state failure: Volatile configuration drift blocks persistence flush", saveErr, log.GenerateRequestID())
				} else {
					slog.Info("Architectural state transition: Configuration topology updated and indexes rebuilt",
						slog.Int("duplicates_purged", dupCount),
					)
				}
			} else {
				slog.Info("Architectural state transition: Configuration topology refreshed directly from disk")
			}
		}
	}

shutdown:
	slog.Info("Architectural state transition: Commencing teardown sequence across local orchestrators",
		slog.String("app_name", appName),
	)
	log.GlobalLogger.Sync()

	shutdownStartupServices(startupTasks, controlServerRegistry, "Startup background tasks did not finish before shutdown")

	closeStoreOnReturn = false
	if err := appServiceManager.StopAll(context.Background()); err != nil {
		errWrap := fmt.Errorf("shutdown: %w", err)
		log.EmitBlockingError("Structural teardown failure: Zombie sub-processes detected during stop iteration", errWrap, log.GenerateRequestID())
		if runErr != nil {
			return stdErrors.Join(runErr, errWrap)
		}
		return errWrap
	}
	return runErr
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

func scheduleDBCleanup(store *storage.Store, configManager *files.ConfigManager) chan struct{} {
	disableCleanup := false
	features := (&files.BotConfig{}).ResolveFeatures("")
	if cfg := configManager.Config(); cfg != nil {
		features = cfg.ResolveFeatures("")
		disableCleanup = cfg.RuntimeConfig.DisableDBCleanup
	}
	cleanupEnabled := features.Maintenance.DBCleanup

	slog.Debug("Evaluating temporal garbage collection routines",
		slog.Bool("cleanup_enabled", cleanupEnabled),
		slog.Bool("disable_cleanup_flag", disableCleanup),
	)

	if cleanupEnabled && !disableCleanup {
		return cache.SchedulePeriodicCleanup(store, 6*time.Hour)
	}

	if !cleanupEnabled {
		slog.Info("Architectural state override: Database garbage collection suppressed explicitly by node definition",
			slog.String("flag", "features.maintenance.db_cleanup"),
		)
	} else {
		slog.Info("Architectural state override: Database garbage collection suppressed globally by configuration override",
			slog.String("flag", "disable_db_cleanup"),
		)
	}
	return nil
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

func rollbackStoreClose(enabled bool, store *storage.Store) {
	if !enabled || store == nil {
		return
	}
	if err := closeStore(store); err != nil {
		log.EmitBlockingError("Structural rollback failure: Persistence mechanism locked during compensatory teardown", err, log.GenerateRequestID())
	}
}

func shutdownStartupServices(startupTasks *StartupTaskOrchestrator, controlServerRegistry *controlServerHolder, tasksWarn string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := startupTasks.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
		slog.Warn("Mitigated shutdown degradation: Async orchestrator missed synchronization lock",
			slog.String("warning_context", tasksWarn),
			slog.String("error", err.Error()),
		)
	}
	if err := controlServerRegistry.Stop(ctx); err != nil {
		log.EmitBlockingError("Structural teardown failure: Interface server socket release hung", err, log.GenerateRequestID())
	}
}

func loadControlDiscordOAuthConfigFromEnv(publicOrigin string) (*control.DiscordOAuthConfig, error) {
	clientID := strings.TrimSpace(files.EnvString(controlDiscordOAuthClientIDEnv, defaultControlDiscordOAuthClientID))
	clientSecret := strings.TrimSpace(files.EnvString(controlDiscordOAuthClientSecretEnv, ""))
	redirectURI := strings.TrimSpace(files.EnvString(controlDiscordOAuthRedirectURIEnv, ""))
	includeGuildMembersRead := files.EnvBool(controlDiscordOAuthIncludeGuildMembersReadEnv)
	sessionStorePath := strings.TrimSpace(files.EnvString(controlDiscordOAuthSessionStorePathEnv, ""))

	slog.Debug("Inspecting environment map for dynamic OAuth injections",
		slog.String("client_id", clientID),
	)

	if clientSecret == "" && redirectURI == "" {
		if includeGuildMembersRead {
			return nil, fmt.Errorf(
				"%s=true requires %s and %s",
				controlDiscordOAuthIncludeGuildMembersReadEnv,
				controlDiscordOAuthClientSecretEnv,
				controlDiscordOAuthRedirectURIEnv,
			)
		}
		return nil, nil
	}
	if clientSecret != "" && redirectURI == "" {
		redirectURI = strings.TrimSpace(publicOrigin)
		if redirectURI != "" {
			redirectURI = strings.TrimRight(redirectURI, "/") + "/auth/discord/callback"
		}
	}

	missing := make([]string, 0, 2)
	if clientSecret == "" {
		missing = append(missing, controlDiscordOAuthClientSecretEnv)
	}
	if redirectURI == "" {
		missing = append(missing, controlDiscordOAuthRedirectURIEnv)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("incomplete Discord OAuth configuration: missing %s", strings.Join(missing, ", "))
	}
	if sessionStorePath == "" {
		sessionStorePath = filepath.Join(files.ApplicationCachesPath, "control", "oauth_sessions.json")
	}

	return &control.DiscordOAuthConfig{
		ClientID:                 clientID,
		ClientSecret:             clientSecret,
		RedirectURI:              redirectURI,
		IncludeGuildsMembersRead: includeGuildMembersRead,
		SessionStorePath:         sessionStorePath,
	}, nil
}

func loadControlTLSFilesFromEnv() (certFile string, keyFile string, err error) {
	certFile = strings.TrimSpace(files.EnvString(controlTLSCertFileEnv, ""))
	keyFile = strings.TrimSpace(files.EnvString(controlTLSKeyFileEnv, ""))
	if certFile == "" && keyFile == "" {
		return "", "", nil
	}

	missing := make([]string, 0, 2)
	if certFile == "" {
		missing = append(missing, controlTLSCertFileEnv)
	}
	if keyFile == "" {
		missing = append(missing, controlTLSKeyFileEnv)
	}
	if len(missing) > 0 {
		return "", "", fmt.Errorf("incomplete control TLS configuration: missing %s", strings.Join(missing, ", "))
	}

	return certFile, keyFile, nil
}

func listBotGuildIDsFromSessionState(session *discordgo.Session) ([]string, error) {
	if session == nil || session.State == nil {
		return nil, fmt.Errorf("discord session state is unavailable")
	}

	seen := make(map[string]struct{}, len(session.State.Guilds))
	ids := make([]string, 0, len(session.State.Guilds))
	for _, guild := range session.State.Guilds {
		if guild == nil {
			continue
		}
		guildID := strings.TrimSpace(guild.ID)
		if guildID == "" {
			continue
		}
		if _, ok := seen[guildID]; ok {
			continue
		}
		seen[guildID] = struct{}{}
		ids = append(ids, guildID)
	}

	return ids, nil
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

type startupWebhookEmbedUpdate struct {
	scope  string
	index  int
	update files.WebhookEmbedUpdateConfig
}

func collectStartupWebhookEmbedUpdates(cfg *files.BotConfig) []startupWebhookEmbedUpdate {
	if cfg == nil {
		return nil
	}

	var out []startupWebhookEmbedUpdate

	for idx, update := range cfg.RuntimeConfig.NormalizedWebhookEmbedUpdates() {
		out = append(out, startupWebhookEmbedUpdate{
			scope:  "global",
			index:  idx,
			update: update,
		})
	}

	for _, guild := range cfg.Guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" {
			continue
		}
		for idx, update := range guild.RuntimeConfig.NormalizedWebhookEmbedUpdates() {
			out = append(out, startupWebhookEmbedUpdate{
				scope:  "guild:" + guildID,
				index:  idx,
				update: update,
			})
		}
	}

	return out
}

func setupStorage(dbb resolvedDatabaseBootstrap) (*storage.Store, *files.ConfigManager, error) {
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
		errWrap := fmt.Errorf("open postgres database: %w", err)
		log.EmitBlockingError("Structural dependency failure: Core socket driver rejected host", errWrap, log.GenerateRequestID())
		return nil, nil, errWrap
	}
	slog.Info("Architectural state transition: Remote persistence pipeline materialized",
		slog.String("operation", "startup.database.open"),
		slog.String("driver", "postgres"),
	)

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := persistence.Ping(pingCtx, db); err != nil {
		db.Close()
		errWrap := fmt.Errorf("postgres readiness check failed: %w", err)
		log.EmitBlockingError("Structural dependency failure: Remote persistence pipeline failed readiness probe", errWrap, log.GenerateRequestID())
		return nil, nil, errWrap
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
		errWrap := fmt.Errorf("apply postgres migrations: %w", err)
		log.EmitBlockingError("Structural schema failure: Matrix migration scripts stalled", errWrap, log.GenerateRequestID())
		return nil, nil, errWrap
	}
	slog.Info("Architectural state transition: Schema schema deltas propagated successfully",
		slog.String("operation", "startup.database.migrate"),
		slog.String("driver", "postgres"),
	)

	store, err := storage.NewStore(db, slog.Default())
	if err != nil {
		db.Close()
		errWrap := fmt.Errorf("create postgres store: %w", err)
		log.EmitBlockingError("Structural dependency failure: Allocation of I/O buffer blocks aborted", errWrap, log.GenerateRequestID())
		return nil, nil, errWrap
	}
	if err := store.Init(); err != nil {
		db.Close()
		errWrap := fmt.Errorf("initialize postgres store: %w", err)
		log.EmitBlockingError("Structural dependency failure: Internal store map lock aborted", errWrap, log.GenerateRequestID())
		return nil, nil, errWrap
	}
	slog.Info("Architectural state transition: Virtual storage layers active",
		slog.String("operation", "startup.database.store_init"),
		slog.String("driver", "postgres"),
	)

	configStore := files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey, slog.Default())
	configManager := files.NewConfigManagerWithStore(configStore, slog.Default())

	slog.Debug("Executing cross-boundary extraction for master configuration tree")
	if err := configManager.LoadConfig(); err != nil {
		errWrap := fmt.Errorf("load config from postgres: %w", err)
		log.EmitBlockingError("Structural dependency failure: Configuration load blocked", errWrap, log.GenerateRequestID())
		return nil, nil, errWrap
	}
	if err := syncBootstrapDatabaseConfig(configManager, dbCfg); err != nil {
		errWrap := fmt.Errorf("sync runtime database bootstrap config: %w", err)
		log.EmitBlockingError("Structural dependency failure: Node manifest drift detected", errWrap, log.GenerateRequestID())
		return nil, nil, errWrap
	}

	return store, configManager, nil
}
