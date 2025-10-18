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
func Run(appName, tokenEnv string) error {
	started := time.Now()

	// App name first (affects paths)
	util.SetAppName(appName)

	// Load env (with $HOME/.local/bin fallback)
	token, loadErr := util.LoadEnvWithLocalBinFallback(tokenEnv)
	if loadErr != nil {
		fmt.Printf("Warning: %v\n", loadErr)
	}

	// Logger first so subsequent steps can log meaningfully
	if err := log.SetupLogger(); err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}

	// Theme configuration
	if err := util.ConfigureThemeFromEnv(); err != nil {
		log.Warn().Applicationf("Failed to set theme from %s: %v", "ALICE_BOT_THEME", err)
	}
	if os.Getenv("ALICE_BOT_THEME") == "" {
		if err := util.SetTheme(""); err != nil {
			log.Warn().Applicationf("Failed to apply default theme: %v", err)
		} else {
			log.Info().Applicationf("🌈 Default theme applied")
		}
	}

	// Global error handler
	if err := errutil.InitializeGlobalErrorHandler(log.GlobalLogger); err != nil {
		return fmt.Errorf("initialize global error handler: %w", err)
	}

	// Error handler for service manager
	errorHandler := errors.NewErrorHandler()

	log.Info().Applicationf("🚀 Starting %s...", appName)

	// Token must be present
	if token == "" {
		return fmt.Errorf("%s not set in environment or .env file", tokenEnv)
	}

	// Discord session
	log.Info().Discordf("🔑 Attempting to authenticate with Discord API...")
	log.Info().Discordf("Using bot token (value redacted)")
	discordSession, err := session.NewDiscordSession(token)
	if err != nil {
		return fmt.Errorf("create discord session: %w", err)
	}
	if discordSession.State == nil || discordSession.State.User == nil {
		return fmt.Errorf("discord session state not properly initialized")
	}
	log.Info().Discordf("✅ Authenticated as %s#%s", discordSession.State.User.Username, discordSession.State.User.Discriminator)

	// Minimal on-disk structure
	if err := util.EnsureCacheInitialized(); err != nil {
		log.Warn().Applicationf("Failed to initialize cache structure: %v", err)
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
		log.Error().Errorf("Failed to load settings file: %v", err)
	}

	// SQLite store
	store := storage.NewStore(util.GetMessageDBPath())
	if err := store.Init(); err != nil {
		return fmt.Errorf("initialize SQLite store: %w", err)
	}

	// Log configured guilds
	if err := files.LogConfiguredGuilds(configManager, discordSession); err != nil {
		log.Error().Errorf("Some configured guilds could not be accessed: %v", err)
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
		log.Info().Applicationf("Skipping cache warmup (recently warmed up)")
	} else {
		warmupConfig := cache.DefaultWarmupConfig()
		warmupConfig.MaxMembersPerGuild = 500 // mitigate initial load
		if err := cache.IntelligentWarmup(discordSession, unifiedCache, store, warmupConfig); err != nil {
			log.Warn().Applicationf("Intelligent warmup failed (continuing): %v", err)
		}
	}

	// Periodic cache persistence
	persistStop := unifiedCache.SetPersistInterval(30 * time.Minute)
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

	// Automod service with TaskRouter adapters
	automodService := logging.NewAutomodService(discordSession, configManager)
	automodRouter := task.NewRouter(task.Defaults())
	defer automodRouter.Close()
	automodAdapters := task.NewNotificationAdapters(automodRouter, discordSession, configManager, store, monitoringService.Notifier())
	automodService.SetAdapters(automodAdapters)

	automodWrapper := service.NewServiceWrapper(
		"automod",
		service.TypeAutomod,
		service.PriorityNormal,
		[]string{},
		func() error { automodService.Start(); return nil },
		func() error { automodService.Stop(); return nil },
		func() bool { return true },
	)

	// Register services
	if err := serviceManager.Register(monitoringWrapper); err != nil {
		return fmt.Errorf("register monitoring service: %w", err)
	}
	if err := serviceManager.Register(automodWrapper); err != nil {
		return fmt.Errorf("register automod service: %w", err)
	}

	// Start services
	log.Info().Applicationf("🚀 Starting all services...")
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
	adminCommands := admin.NewAdminCommands(serviceManager)
	adminCommands.RegisterCommands(commandHandler.GetCommandManager().GetRouter())

	log.Info().Applicationf("🔗 Slash commands sync completed")
	log.Info().Applicationf("🎯 %s initialized successfully in %s", appName, time.Since(started).Round(time.Millisecond))
	log.Info().Applicationf("🤖 %s running. Press Ctrl+C to stop...", appName)

	// Wait for shutdown signal
	util.WaitForInterrupt()
	log.Info().Applicationf("🛑 Stopping %s...", appName)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := serviceManager.StopAll(); err != nil {
		log.Error().Errorf("Some services failed to stop cleanly: %v", err)
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
