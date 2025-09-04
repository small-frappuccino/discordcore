package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	dcache "github.com/alice-bnuy/discordcore/v2/pkg/cache"
	"github.com/alice-bnuy/discordcore/v2/pkg/discord/commands"
	"github.com/alice-bnuy/discordcore/v2/pkg/discord/commands/admin"
	"github.com/alice-bnuy/discordcore/v2/pkg/discord/logging"
	"github.com/alice-bnuy/discordcore/v2/pkg/discord/session"
	"github.com/alice-bnuy/discordcore/v2/pkg/errors"
	"github.com/alice-bnuy/discordcore/v2/pkg/files"
	"github.com/alice-bnuy/discordcore/v2/pkg/service"
	"github.com/alice-bnuy/discordcore/v2/pkg/task"
	"github.com/alice-bnuy/discordcore/v2/pkg/util"
	"github.com/alice-bnuy/errutil"
	"github.com/alice-bnuy/logutil"
	"github.com/joho/godotenv"
)

// main is the entry point of the Discord bot.
func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Arquivo .env não encontrado ou erro ao carregar")
	}

	// Initialize global logger
	if err := logutil.SetupLogger(); err != nil {
		fmt.Printf("failed to configure logger: %v\n", err)
		os.Exit(1)
	}
	// Ensure logger is closed on exit
	defer func() {
		if err := logutil.CloseGlobalLogger(); err != nil {
			fmt.Fprintf(os.Stderr, "error closing logger: %v\n", err)
		}
	}()

	// Initialize global error handler
	if err := errutil.InitializeGlobalErrorHandler(logutil.GlobalLogger); err != nil {
		fmt.Fprintln(os.Stderr, "failed to initialize error handler:", err)
		os.Exit(1)
	}

	// Initialize unified error handler
	errorHandler := errors.NewErrorHandler()

	// Log bot startup
	logutil.Info("🚀 Starting Alice Bot...")

	// Ensure config and cache files exist
	if err := files.EnsureConfigFiles(); err != nil {
		logutil.ErrorWithErr("Error checking config files", err)
		logutil.Fatal("❌ Error checking config files")
	}

	// Fetch token
	token := util.GetDiscordBotToken("ALICE_BOT")

	// Initialize config manager
	configManager := files.NewConfigManager()
	// Load existing settings from disk before starting services
	if err := configManager.LoadConfig(); err != nil {
		logutil.ErrorWithErr("Failed to load settings file", err)
	}

	// Add detailed logging for Discord authentication
	logutil.Info("🔑 Attempting to authenticate with Discord API...")
	logutil.Debugf("Using bot token: %s", token)

	// Create Discord session and ensure safe shutdown
	discordSession, err := session.NewDiscordSession(token)
	if err != nil {
		logutil.ErrorWithErr("❌ Authentication failed with Discord API", err)
		logutil.Fatalf("❌ Error creating Discord session: %v", err)
	}
	logutil.Infof("✅ Successfully authenticated with Discord API as %s#%s", discordSession.State.User.Username, discordSession.State.User.Discriminator)

	// Initialize avatar/config cache
	cache := dcache.NewDefaultAvatarCacheManager()
	if err := cache.Load(); err != nil {
		logutil.ErrorWithErr("Error loading cache", err)
	}

	// Log summary of configured guilds
	if err := files.LogConfiguredGuilds(configManager, discordSession); err != nil {
		logutil.ErrorWithErr("Some configured guilds could not be accessed", err)
	}

	// Initialize Service Manager
	serviceManager := service.NewServiceManager(errorHandler)

	// Create service wrappers for existing services
	logutil.Info("🔧 Creating service wrappers...")

	// Wrap MonitoringService
	monitoringService, err := logging.NewMonitoringService(discordSession, configManager, cache)
	if err != nil {
		logutil.ErrorWithErr("Failed to create monitoring service", err)
		logutil.Fatal("❌ Failed to create monitoring service")
	}

	monitoringWrapper := service.NewServiceWrapper(
		"monitoring",
		service.TypeMonitoring,
		service.PriorityHigh,
		[]string{}, // No dependencies
		func() error { return monitoringService.Start() },
		func() error { return monitoringService.Stop() },
		func() bool { return true }, // Simple health check
	)

	// Wrap AutomodService
	automodService := logging.NewAutomodService(discordSession, configManager)
	// Wire Automod with TaskRouter via NotificationAdapters (uses same notifier/config/cache)
	automodRouter := task.NewRouter(task.Defaults())
	automodAdapters := task.NewNotificationAdapters(automodRouter, discordSession, configManager, cache, monitoringService.Notifier())
	automodService.SetAdapters(automodAdapters)
	automodWrapper := service.NewServiceWrapper(
		"automod",
		service.TypeAutomod,
		service.PriorityNormal,
		[]string{}, // No dependencies
		func() error { automodService.Start(); return nil },
		func() error { automodService.Stop(); return nil },
		func() bool { return true }, // Simple health check
	)

	// Register services with the manager
	if err := serviceManager.Register(monitoringWrapper); err != nil {
		logutil.ErrorWithErr("Failed to register monitoring service", err)
		logutil.Fatal("❌ Failed to register monitoring service")
	}

	if err := serviceManager.Register(automodWrapper); err != nil {
		logutil.ErrorWithErr("Failed to register automod service", err)
		logutil.Fatal("❌ Failed to register automod service")
	}

	// Start all services
	logutil.Info("🚀 Starting all services...")
	if err := serviceManager.StartAll(); err != nil {
		logutil.ErrorWithErr("Failed to start services", err)
		logutil.Fatal("❌ Failed to start services")
	}

	// Initialize and register bot commands
	commandHandler := commands.NewCommandHandler(discordSession, configManager)
	if err := commandHandler.SetupCommands(); err != nil {
		logutil.ErrorWithErr("Error configuring slash commands", err)
		logutil.Fatal("❌ Error configuring slash commands")
	}

	// Register admin commands
	compositeCache := dcache.NewCompositeCache("bot_cache", cache, monitoringService.MessageEvents().GetCache())
	adminCommands := admin.NewAdminCommands(serviceManager, compositeCache)
	adminCommands.RegisterCommands(commandHandler.GetCommandManager().GetRouter())

	// Ensure safe shutdown of all services
	defer func() {
		logutil.Info("🛑 Shutting down services...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := serviceManager.StopAll(); err != nil {
			logutil.ErrorWithErr("Some services failed to stop cleanly", err)
		}
		_ = shutdownCtx // Avoid unused variable warning
	}()

	// Log successful initialization and wait for shutdown
	logutil.Info("🔗 Slash commands sync completed")
	logutil.Info("🎯 Bot initialized successfully!")
	logutil.Info("🤖 Bot running. Monitoring all configured guilds. Press Ctrl+C to stop...")

	util.WaitForInterrupt()
	logutil.Info("🛑 Stopping bot...")
}
