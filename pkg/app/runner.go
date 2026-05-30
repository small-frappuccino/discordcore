package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
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
	waitForInterrupt             = WaitForInterrupt
	shutdownDelay                = time.Sleep
)

const (
	defaultControlAddr                            = "127.0.0.1:8376"
	defaultControlDiscordOAuthClientID            = "1396606252506681395"
	controlBearerTokenEnv                         = "ALICE_CONTROL_BEARER_TOKEN"
	controlDiscordOAuthClientIDEnv                = "ALICE_CONTROL_DISCORD_OAUTH_CLIENT_ID"
	controlDiscordOAuthClientSecretEnv            = "ALICE_CONTROL_DISCORD_OAUTH_CLIENT_SECRET"
	controlDiscordOAuthRedirectURIEnv             = "ALICE_CONTROL_DISCORD_OAUTH_REDIRECT_URI"
	controlDiscordOAuthIncludeGuildMembersReadEnv = "ALICE_CONTROL_DISCORD_OAUTH_INCLUDE_GUILDS_MEMBERS_READ"
	controlDiscordOAuthSessionStorePathEnv        = "ALICE_CONTROL_DISCORD_OAUTH_SESSION_STORE_PATH"
	controlTLSCertFileEnv                         = "ALICE_CONTROL_TLS_CERT_FILE"
	controlTLSKeyFileEnv                          = "ALICE_CONTROL_TLS_KEY_FILE"
)

// Run bootstraps the bot with a unified flow and blocks until shutdown.
// appName affects config/cache/log paths; tokenEnv is the environment variable containing the bot token.
// Run bootstraps the bot with a unified flow and blocks until shutdown.
// Environment: the tokenEnv is read from the current process environment first; if empty,
// a fallback D:\Users\smallfrappuccino\.local\bin\.env file will be loaded and the variable re-checked.
// Persistent cache: guild-level cleanup uses explicit (type + key prefix) deletion to safely
// remove rows for members (prefix guildID:), guilds (key guildID), and roles (key guildID).
func Run(appName, tokenEnv string) error {
	return RunWithOptions(appName, tokenEnv, RunOptions{})
}

// RunWithOptions bootstraps the bot and allows hosts to override control-plane wiring.
func RunWithOptions(appName, tokenEnv string, opts RunOptions) error {
	started := time.Now()

	// App name first (affects paths)
	files.SetAppName(appName)

	// Logger first so subsequent steps can log meaningfully
	if err := log.SetupLogger(files.EffectiveBotName(), files.GetLogFilePath()); err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	// Ensure logs are flushed on exit
	defer log.GlobalLogger.Sync()

	// Best-effort startup notification on the configured lifecycle
	// webhook. Fires once before any work happens so operators see a
	// fresh "back online" chat message after every supervisor restart.
	// notifyLifecycleEvent is a no-op when ALICE_LIFECYCLE_WEBHOOK_URL is unset.
	notifyLifecycleEvent("starting", "")
	// Paired with the startup notification: this defer emits a "stopping"
	// (or "fatal") line right before the process exits, so the operator
	// has a chat-channel record of every clean shutdown. Crashes via
	// runtime.throw bypass deferred handlers — that gap is covered by
	// the external /v1/health/live poller documented in OPS.md.
	defer func() {
		if r := recover(); r != nil {
			notifyLifecycleEvent("fatal", fmt.Sprintf("%v", r))
			panic(r)
		}
		notifyLifecycleEvent("stopping", "")
	}()

	// Theme configuration now comes from persisted runtime_config.
	// IMPORTANT: configManager is created later (after the config store is ready).
	// We cannot read runtime_config here without risking an undefined variable / nil config.
	// Theme will be applied right after loading the config store (see below).

	// Runtime hot-apply manager (theme + ALICE_DISABLE_* toggles)
	// NOTE: The /config runtime panel triggers Apply() after persisting config changes.
	var runtimeApplier *runtimeapply.Manager

	msg := formatStartupMessage(appName, AppVersion(), Version)
	log.ApplicationLogger().Info(msg)

	botInstances, defaultBotInstanceID, err := resolveBotInstances(tokenEnv, opts)
	if err != nil {
		return err
	}

	databaseBootstrap, err := resolveDatabaseBootstrap()
	if err != nil {
		return err
	}
	log.ApplicationLogger().Info(
		"Resolved postgres bootstrap configuration",
		"operation", "startup.database.bootstrap",
		"source", databaseBootstrap.Source,
	)

	// Minimal on-disk structure
	if err := files.EnsureCacheInitialized(); err != nil {
		log.ApplicationLogger().Warn(fmt.Sprintf("Failed to initialize cache structure: %v", err))
	}
	if err := files.EnsureCacheDirs(); err != nil {
		return fmt.Errorf("create cache directories: %w", err)
	}
	// PostgreSQL bootstrap comes from environment variables. The resolved value is
	// mirrored into runtime_config after the config store is loaded.
	store, configManager, err := setupStorage(databaseBootstrap)
	if err != nil {
		return err
	}
	closeStoreOnReturn := true
	defer func() {
		if closeStoreOnReturn && store != nil {
			if err := closeStore(store); err != nil {
				log.ErrorLoggerRaw().Error("Store close failed during startup cleanup", "err", err)
			}
		}
	}()

	// Theme configuration (from persisted runtime_config)
	{
		cfg := configManager.Config()
		themeName := ""
		if cfg != nil {
			themeName = cfg.RuntimeConfig.BotTheme
		}

		if err := files.ConfigureThemeFromConfig(themeName); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to set theme from runtime config %s: %v", "bot_theme", err))
		}
		if themeName == "" {
			if err := files.SetTheme(""); err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to apply default theme: %v", err))
			} else {
				log.ApplicationLogger().Info("🌈 Default theme applied")
			}
		}
	}

	// Periodic cleanup (every 6 hours), can be disabled via runtime config
	var cleanupStop chan struct{}
	disableCleanup := false
	features := (&files.BotConfig{}).ResolveFeatures("")
	if cfg := configManager.Config(); cfg != nil {
		features = cfg.ResolveFeatures("")
		disableCleanup = cfg.RuntimeConfig.DisableDBCleanup
	}
	cleanupEnabled := features.Maintenance.DBCleanup
	if cleanupEnabled && !disableCleanup {
		cleanupStop = cache.SchedulePeriodicCleanup(store, 6*time.Hour)
	} else {
		if !cleanupEnabled {
			log.ApplicationLogger().Info("🛑 DB cleanup disabled by features.maintenance.db_cleanup")
		} else {
			log.ApplicationLogger().Info("🛑 DB cleanup disabled by runtime config disable_db_cleanup")
		}
	}
	defer func() {
		if cleanupStop != nil {
			close(cleanupStop)
		}
	}()

	// Create runtime hot-apply manager and set initial baseline from current config.
	// This lets the runtime config panel apply environment-like toggles without a full restart.
	runtimeApplier = runtimeapply.New(nil, nil)
	if cfg := configManager.Config(); cfg != nil {
		runtimeApplier.SetInitial(cfg.RuntimeConfig)
	}

	runtimes := make(map[string]*botRuntime, len(botInstances))
	runtimeCapabilities := make(map[string]botRuntimeCapabilities, len(botInstances))
	runtimeOrder := make([]*botRuntime, 0, len(botInstances))
	controlServerRegistry := &controlServerHolder{}
	domainSupport := newRuntimeDomainSupport(opts.SupportedDomains)
	cleanupRuntimesOnReturn := true
	defer func() {
		if !cleanupRuntimesOnReturn {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for i := len(runtimeOrder) - 1; i >= 0; i-- {
			runtime := runtimeOrder[i]
			for _, err := range shutdownBotRuntimeFn(runtime, ctx) {
				log.ErrorLoggerRaw().Error("Bot runtime cleanup failed during startup rollback", "botInstanceID", runtime.instanceID, "err", err)
			}
		}
	}()
	closeDiscordSessionsOnReturn := true
	defer func() {
		if !closeDiscordSessionsOnReturn {
			return
		}
		for i := len(runtimeOrder) - 1; i >= 0; i-- {
			runtime := runtimeOrder[i]
			if runtime == nil || runtime.session == nil {
				continue
			}
			if err := closeDiscordSession(runtime.session); err != nil {
				log.ErrorLoggerRaw().Error("Discord session close failed during startup cleanup", "botInstanceID", runtime.instanceID, "err", err)
			}
		}
	}()
	startupTasks := newStartupTaskOrchestrator(len(botInstances))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := startupTasks.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
			log.ApplicationLogger().Warn("Startup background tasks did not finish cleanly", "err", err)
		}
		if err := controlServerRegistry.Stop(ctx); err != nil {
			log.ErrorLoggerRaw().Error("Failed to stop control server cleanly", "err", err)
		}
	}()

	configSnapshot := configManager.Config()
	for _, instance := range botInstances {
		runtimeCapabilities[instance.ID] = resolveBotRuntimeCapabilitiesForDomains(
			configSnapshot,
			instance.ID,
			defaultBotInstanceID,
			domainSupport,
		)
	}

	var openErr error
	runtimes, runtimeOrder, openErr = openBotRuntimes(botInstances, runtimeCapabilities)
	if openErr != nil {
		return openErr
	}

	if err := validateConfiguredBotInstances(
		configManager.Config(),
		knownBotInstanceCatalog(runtimes, opts.KnownBotInstanceIDs),
		defaultBotInstanceID,
	); err != nil {
		return fmt.Errorf("validate configured bot instances: %w", err)
	}

	runtimeResolver := newBotRuntimeResolver(configManager, runtimes, defaultBotInstanceID)
	var defaultSession *discordgo.Session
	if domainSupport.supportsDefaultDomain() {
		defaultSession, err = runtimeResolver.sessionForGuild("")
		if err != nil {
			return fmt.Errorf("resolve default discord session: %w", err)
		}
	}

	// Wire the in-memory metrics sink so /v1/health/qotd has counters to
	// expose. NopMetrics is a valid fallback (the service still works) but
	// without a SnapshotProvider the route returns 503; production startup
	// always uses the in-memory implementation.
	qotdMetrics := qotd.NewInMemoryMetrics()
	qotdService := qotd.NewServiceWithMetrics(configManager, store, nil, qotdMetrics)
	// Mirror QOTD: wire the in-memory moderation metrics so /v1/health/moderation
	// has counters to expose. NopMetrics is a valid fallback for tests, but
	// production startup always uses the in-memory implementation.
	moderationMetrics := moderation.NewInMemoryMetrics()

	if err := initializeBotRuntimes(runtimeOrder, botRuntimeOptions{
		defaultBotInstanceID:     defaultBotInstanceID,
		runtimeCount:             len(runtimeOrder),
		supportedDomains:         opts.SupportedDomains,
		configManager:            configManager,
		store:                    store,
		commandCatalogRegistrars: opts.CommandCatalogRegistrars,
		runtimeApplier:           runtimeApplier,
		qotdCommandService:       qotdService,
		qotdLifecycleService:     qotdService,
		moderationMetrics:        moderationMetrics,
		startupTasks:             startupTasks,
	}); err != nil {
		return err
	}

	controlBearerToken := strings.TrimSpace(files.EnvString(controlBearerTokenEnv, ""))
	scheduleStartupWebhookEmbedUpdates(startupTasks, configManager.Config(), defaultSession)
	scheduleControlServerStartup(startupTasks, controlStartupTaskOptions{
		runOptions:            opts,
		configManager:         configManager,
		runtimeApplier:        runtimeApplier,
		controlBearerToken:    controlBearerToken,
		defaultBotInstanceID:  defaultBotInstanceID,
		runtimeResolver:       runtimeResolver,
		store:                 store,
		qotdService:           qotdService,
		moderationMetrics:     moderationMetrics,
		controlServerRegistry: controlServerRegistry,
	})

	log.ApplicationLogger().Info("🔗 Slash commands sync completed")
	log.ApplicationLogger().Info(fmt.Sprintf("🎯 %s initialized successfully in %s", appName, time.Since(started).Round(time.Millisecond)))
	log.ApplicationLogger().Info(fmt.Sprintf("🤖 %s running. Press Ctrl+C to stop...", appName))

	// Wait for shutdown signal
	waitForInterrupt()
	log.ApplicationLogger().Info(fmt.Sprintf("🛑 Stopping %s...", appName))
	log.GlobalLogger.Sync()

	backgroundShutdownCtx, backgroundShutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := startupTasks.Shutdown(backgroundShutdownCtx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
		log.ApplicationLogger().Warn("Startup background tasks did not finish before shutdown", "err", err)
	}
	if err := controlServerRegistry.Stop(backgroundShutdownCtx); err != nil {
		log.ErrorLoggerRaw().Error("Failed to stop control server cleanly", "err", err)
	}
	backgroundShutdownCancel()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeoutCause(context.Background(), 30*time.Second, fmt.Errorf("application shutdown"))
	defer shutdownCancel()
	var shutdownErrs []error

	for i := len(runtimeOrder) - 1; i >= 0; i-- {
		runtime := runtimeOrder[i]
		for _, err := range shutdownBotRuntimeFn(runtime, shutdownCtx) {
			log.ErrorLoggerRaw().Error("Bot runtime shutdown failed", "botInstanceID", runtime.instanceID, "err", err)
			shutdownErrs = append(shutdownErrs, err)
		}
	}

	// Allow services to finish final writes before closing store
	shutdownDelay(100 * time.Millisecond)

	closeStoreOnReturn = false
	if store != nil {
		if err := closeStore(store); err != nil {
			log.ErrorLoggerRaw().Error("Store close failed during shutdown", "err", err)
			shutdownErrs = append(shutdownErrs, fmt.Errorf("close store: %w", err))
		}
	}

	closeDiscordSessionsOnReturn = false
	cleanupRuntimesOnReturn = false
	for i := len(runtimeOrder) - 1; i >= 0; i-- {
		runtime := runtimeOrder[i]
		if runtime == nil || runtime.session == nil {
			continue
		}
		if err := closeDiscordSession(runtime.session); err != nil {
			log.ErrorLoggerRaw().Error("Discord session close failed during shutdown", "botInstanceID", runtime.instanceID, "err", err)
			shutdownErrs = append(shutdownErrs, fmt.Errorf("close discord session for %s: %w", runtime.instanceID, err))
		}
	}

	_ = shutdownCtx
	if len(shutdownErrs) > 0 {
		return fmt.Errorf("shutdown: %w", stdErrors.Join(shutdownErrs...))
	}
	return nil
}

func loadControlDiscordOAuthConfigFromEnv(publicOrigin string) (*control.DiscordOAuthConfig, error) {
	clientID := strings.TrimSpace(files.EnvString(controlDiscordOAuthClientIDEnv, defaultControlDiscordOAuthClientID))
	clientSecret := strings.TrimSpace(files.EnvString(controlDiscordOAuthClientSecretEnv, ""))
	redirectURI := strings.TrimSpace(files.EnvString(controlDiscordOAuthRedirectURIEnv, ""))
	includeGuildMembersRead := files.EnvBool(controlDiscordOAuthIncludeGuildMembersReadEnv)
	sessionStorePath := strings.TrimSpace(files.EnvString(controlDiscordOAuthSessionStorePathEnv, ""))

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

// formatStartupMessage builds the startup log line.
// Rules:
// - If appVersion is empty: omit it.
// - If coreVersion is empty: omit it.
// - If appVersion equals coreVersion: omit the "(discordcore ...)" suffix to avoid redundant output.
func formatStartupMessage(appName, appVersion, coreVersion string) string {
	appName = strings.TrimSpace(appName)
	appVersion = strings.TrimSpace(appVersion)
	coreVersion = strings.TrimSpace(coreVersion)

	msg := fmt.Sprintf("🚀 Starting %s", appName)
	if appVersion != "" {
		msg += fmt.Sprintf(" %s", appVersion)
	}

	// Avoid duplicated versions like: "discordmain v0.146.0 (discordcore v0.146.0)"
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
		return nil, nil, fmt.Errorf("open postgres database: %w", err)
	}
	log.ApplicationLogger().Info("Database connection opened", "operation", "startup.database.open", "driver", "postgres")

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := persistence.Ping(pingCtx, db); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("postgres readiness check failed: %w", err)
	}
	log.ApplicationLogger().Info("Database readiness check passed", "operation", "startup.database.ping", "driver", "postgres")

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer migrateCancel()
	migrator := persistence.NewPostgresMigrator(db)
	if err := migrator.Up(migrateCtx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("apply postgres migrations: %w", err)
	}
	log.ApplicationLogger().Info("Database migrations applied", "operation", "startup.database.migrate", "driver", "postgres")

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("initialize postgres store: %w", err)
	}
	log.ApplicationLogger().Info("Storage layer initialized", "operation", "startup.database.store_init", "driver", "postgres")

	configStore := files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey)
	configManager := files.NewConfigManagerWithStore(configStore)
	if err := configManager.LoadConfig(); err != nil {
		return nil, nil, fmt.Errorf("load config from postgres: %w", err)
	}
	if err := syncBootstrapDatabaseConfig(configManager, dbCfg); err != nil {
		return nil, nil, fmt.Errorf("sync runtime database bootstrap config: %w", err)
	}

	return store, configManager, nil
}
