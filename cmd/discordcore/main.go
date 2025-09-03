package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alice-bnuy/discordcore/v2/internal/discord/commands"
	"github.com/alice-bnuy/discordcore/v2/internal/discord/logging"
	"github.com/alice-bnuy/discordcore/v2/internal/discord/session"
	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/alice-bnuy/discordcore/v2/internal/util"
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

	// Log bot startup
	logutil.Info("🚀 Starting Alice Bot...")

	// Ensure config and cache files exist
	if err := files.EnsureConfigFiles(); err != nil {
		logutil.ErrorWithErr("Error checking config files", err)
		logutil.Fatal("❌ Error checking config files")
	}

	// Fetch token
	token := files.GetDiscordBotToken("ALICE_BOT")

	// Initialize config manager
	configManager := files.NewConfigManager()

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
	cache := files.NewAvatarCacheManager()
	if err := cache.Load(); err != nil {
		logutil.ErrorWithErr("Error loading cache", err)
	}

	// Log summary of configured guilds
	if err := files.LogConfiguredGuilds(configManager, discordSession); err != nil {
		logutil.ErrorWithErr("Some configured guilds could not be accessed", err)
	}

	// Initialize and start multi-guild monitoring service
	monitorService, err := logging.NewMonitoringService(discordSession, configManager, cache)
	if err != nil {
		logutil.ErrorWithErr("Failed to initialize monitoring service", err)
		logutil.Fatal("❌ Failed to initialize monitoring service")
	}
	if err := monitorService.Start(); err != nil {
		logutil.ErrorWithErr("Failed to start monitoring service", err)
		logutil.Fatal("❌ Failed to start monitoring service")
	}
	// Ensure safe shutdown of monitoring service
	defer func() {
		if err := monitorService.Stop(); err != nil {
			logutil.ErrorWithErr("Failed to stop monitoring service", err)
		}
	}()

	// Initialize and start AutoMod service (keyword-based moderation)
	automodService := logging.NewAutomodService(discordSession, configManager)
	automodService.Start()
	defer automodService.Stop()

	// Initialize and register bot commands
	commandHandler := commands.NewCommandHandler(discordSession, configManager, cache, monitorService, automodService)
	if err := commandHandler.SetupCommands(); err != nil {
		logutil.ErrorWithErr("Error configuring slash commands", err)
		logutil.Fatal("❌ Error configuring slash commands")
	}

	// Log successful initialization and wait for shutdown
	logutil.Info("🔗 Slash commands sync completed")
	logutil.Info("🎯 Bot initialized successfully!")
	logutil.Info("🤖 Bot running. Monitoring all configured guilds. Press Ctrl+C to stop...")

	util.WaitForInterrupt()
	logutil.Info("🛑 Stopping bot...")
}
