package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/admin"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/errors"
	"github.com/small-frappuccino/discordcore/pkg/errutil"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// Run bootstraps the bot with a unified flow and blocks until shutdown.
// appName affects config/cache/log paths; tokenEnv is the environment variable containing the bot token.
// Run bootstraps the bot with a unified flow and blocks until shutdown.
// Environment: the tokenEnv is read from the current process environment first; if empty,
// a fallback $HOME/.local/bin/.env file will be loaded and the variable re-checked.
// Persistent cache: guild-level cleanup uses explicit (type + key prefix) deletion to safely
// remove rows for members (prefix guildID:), guilds (key guildID), and roles (key guildID).
func Run(appName, tokenEnv string) error {
	started := time.Now()

	// App name first (affects paths)
	util.SetAppName(appName)

	// Load env (with $HOME/.local/bin fallback)
	token, loadErr := util.LoadEnvWithLocalBinFallback(tokenEnv)
	if loadErr != nil {
		log.ApplicationLogger().Warn(fmt.Sprintf("Warning: %v", loadErr))
	}

	// Logger first so subsequent steps can log meaningfully
	if err := log.SetupLogger(); err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	// Ensure logs are flushed on exit
	defer log.GlobalLogger.Sync()

	// Theme configuration
	if err := util.ConfigureThemeFromEnv(); err != nil {
		log.ApplicationLogger().Warn(fmt.Sprintf("Failed to set theme from %s: %v", "ALICE_BOT_THEME", err))
	}
	if os.Getenv("ALICE_BOT_THEME") == "" {
		if err := util.SetTheme(""); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to apply default theme: %v", err))
		} else {
			log.ApplicationLogger().Info("🌈 Default theme applied")
		}
	}

	// Global error handler
	if err := errutil.InitializeGlobalErrorHandler(log.GlobalLogger); err != nil {
		return fmt.Errorf("initialize global error handler: %w", err)
	}

	// Error handler for service manager
	errorHandler := errors.NewErrorHandler()

	log.ApplicationLogger().Info(fmt.Sprintf("🚀 Starting %s...", appName))

	// Token must be present
	if token == "" {
		return fmt.Errorf("%s not set in environment or .env file", tokenEnv)
	}

	// Discord session
	log.DiscordLogger().Info("🔑 Attempting to authenticate with Discord API...")
	log.DiscordLogger().Info("Using bot token (value redacted)")
	discordSession, err := session.NewDiscordSession(token)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}
	if discordSession.State == nil || discordSession.State.User == nil {
		return fmt.Errorf("discord session state not properly initialized")
	}
	log.DiscordLogger().Info(fmt.Sprintf("✅ Authenticated as %s#%s", discordSession.State.User.Username, discordSession.State.User.Discriminator))

	// Minimal on-disk structure
	if err := util.EnsureCacheInitialized(); err != nil {
		log.ApplicationLogger().Warn(fmt.Sprintf("Failed to initialize cache structure: %v", err))
	}
	if err := util.EnsureCacheDirs(); err != nil {
		return fmt.Errorf("create cache directories: %w", err)
	}
	if err := files.EnsureConfigFiles(); err != nil {
		return fmt.Errorf("ensure config files: %w", err)
	}

	// Config manager
	configManager := files.NewConfigManager()
	if err := configManager.LoadConfig(); err != nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("Failed to load settings file: %v", err))
	}

	// SQLite store (support ALICE_MESSAGE_DB_PATH override)
	dbPath := util.GetMessageDBPath()
	if v := os.Getenv("ALICE_MESSAGE_DB_PATH"); v != "" {
		dbPath = v
	}
	store := storage.NewStore(dbPath)
	if err := store.Init(); err != nil {
		return fmt.Errorf("initialize SQLite store: %w", err)
	}

	// Log configured guilds
	if err := files.LogConfiguredGuilds(configManager, discordSession); err != nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("Some configured guilds could not be accessed: %v", err))
	}

	// Periodic cleanup (every 6 hours)
	cleanupStop := cache.SchedulePeriodicCleanup(store, 6*time.Hour)
	defer func() {
		if cleanupStop != nil {
			close(cleanupStop)
		}
	}()

	// Service manager
	serviceManager := service.NewServiceManager(errorHandler)

	// Monitoring service (central orchestration + unified cache)
	monitoringService, err := logging.NewMonitoringService(discordSession, configManager, store)
	if err != nil {
		return fmt.Errorf("create monitoring service: %w", err)
	}

	// Cache warmup (persisted + fetch missing)
	// NOTE: Warmup responsibility is consolidated in the app runner.
	// MonitoringService does not perform its own warmup to avoid duplicate work during startup.
	unifiedCache := monitoringService.GetUnifiedCache()
	if unifiedCache != nil && unifiedCache.WasWarmedUpRecently(10*time.Minute) {
		log.ApplicationLogger().Info("Skipping cache warmup (recently warmed up)")
	} else {
		warmupConfig := cache.DefaultWarmupConfig()
		warmupConfig.MaxMembersPerGuild = 500 // mitigate initial load
		if err := cache.IntelligentWarmup(discordSession, unifiedCache, store, warmupConfig); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Intelligent warmup failed (continuing): %v", err))
		}
	}

	// Periodic cache persistence (configurable via ALICE_UNIFIED_CACHE_PERSIST_INTERVAL; default 1h)
	persistInterval := time.Hour
	if v := os.Getenv("ALICE_UNIFIED_CACHE_PERSIST_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			persistInterval = d
		} else {
			log.ApplicationLogger().Warn(fmt.Sprintf("Invalid ALICE_UNIFIED_CACHE_PERSIST_INTERVAL=%q: %v; using default %v", v, err, persistInterval))
		}
	}
	persistStop := unifiedCache.SetPersistInterval(persistInterval)
	defer func() {
		if persistStop != nil {
			close(persistStop)
		}
	}()

	// Wrap monitoring
	monitoringWrapper := service.NewServiceWrapper(
		"monitoring",
		service.TypeMonitoring,
		service.PriorityHigh,
		[]string{},
		func() error { return monitoringService.Start() },
		func() error { return monitoringService.Stop() },
		func() bool { return true },
	)

	// Automod service with TaskRouter adapters (gated by ALICE_DISABLE_AUTOMOD_LOGS)
	disableAutomod := util.EnvBool("ALICE_DISABLE_AUTOMOD_LOGS")
	var automodWrapper *service.ServiceWrapper
	if disableAutomod {
		log.ApplicationLogger().Info("🛑 Automod logs disabled by ALICE_DISABLE_AUTOMOD_LOGS; AutomodService will not start")
	} else {
		automodService := logging.NewAutomodService(discordSession, configManager)
		automodRouter := task.NewRouter(task.Defaults())
		defer automodRouter.Close()
		automodAdapters := task.NewNotificationAdapters(automodRouter, discordSession, configManager, store, monitoringService.Notifier())
		automodService.SetAdapters(automodAdapters)

		automodWrapper = service.NewServiceWrapper(
			"automod",
			service.TypeAutomod,
			service.PriorityNormal,
			[]string{},
			func() error { automodService.Start(); return nil },
			func() error { automodService.Stop(); return nil },
			func() bool { return true },
		)
	}

	// Register services
	if err := serviceManager.Register(monitoringWrapper); err != nil {
		return fmt.Errorf("register monitoring service: %w", err)
	}
	if automodWrapper != nil {
		if err := serviceManager.Register(automodWrapper); err != nil {
			return fmt.Errorf("register automod service: %w", err)
		}
	}

	// Start services
	log.ApplicationLogger().Info("🚀 Starting all services...")
	if err := serviceManager.StartAll(); err != nil {
		return fmt.Errorf("start services: %w", err)
	}

	// Commands
	commandHandler := commands.NewCommandHandler(discordSession, configManager)
	if err := commandHandler.SetupCommands(); err != nil {
		return fmt.Errorf("configure slash commands: %w", err)
	}

	// Inject store and unified cache into command router
	if cm := commandHandler.GetCommandManager(); cm != nil {
		if router := cm.GetRouter(); router != nil {
			router.SetStore(store)
			if monitoringService != nil {
				router.SetCache(monitoringService.GetUnifiedCache())
			}
		}
	}

	// Admin commands
	adminCommands := admin.NewAdminCommands(serviceManager, unifiedCache)
	adminCommands.RegisterCommands(commandHandler.GetCommandManager().GetRouter())

	log.ApplicationLogger().Info("🔗 Slash commands sync completed")
	log.ApplicationLogger().Info(fmt.Sprintf("🎯 %s initialized successfully in %s", appName, time.Since(started).Round(time.Millisecond)))
	log.ApplicationLogger().Info(fmt.Sprintf("🤖 %s running. Press Ctrl+C to stop...", appName))

	// Wait for shutdown signal
	util.WaitForInterrupt()
	log.ApplicationLogger().Info(fmt.Sprintf("🛑 Stopping %s...", appName))
	log.GlobalLogger.Sync()

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeoutCause(context.Background(), 30*time.Second, fmt.Errorf("application shutdown"))
	defer shutdownCancel()

	if err := serviceManager.StopAll(); err != nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("Some services failed to stop cleanly: %v", err))
	}

	// Allow services to finish final writes before closing store
	time.Sleep(100 * time.Millisecond)

	if store != nil {
		_ = store.Close()
	}

	if discordSession != nil {
		_ = discordSession.Close()
	}

	_ = shutdownCtx
	return nil
}
