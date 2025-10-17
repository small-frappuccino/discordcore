package main

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

// main is the entry point of the Discord bot.
func main() {
	// Hard-set app name before any initialization (affects config/cache/log paths)
	util.SetAppName("discordcore")

	// Load environment with fallback search under $HOME/.local/bin
	token, loadErr := util.LoadEnvWithLocalBinFallback("ALICE_BOT_DEVELOPMENT_TOKEN")
	if loadErr != nil {
		fmt.Printf("Warning: %v\n", loadErr)
	}

	// Initialize global logger (internal discordcore logger)
	if err := log.SetupLogger(); err != nil {
		fmt.Printf("failed to configure logger: %v\n", err)
		os.Exit(1)
	}

	// Configure theme from environment if set (ALICE_BOT_THEME)
	if err := util.ConfigureThemeFromEnv(); err != nil {
		log.Warn().Applicationf("Failed to set theme from ALICE_BOT_THEME: %v", err)
	}
	// Default: if ALICE_BOT_THEME is not set, ensure default theme is active
	if os.Getenv("ALICE_BOT_THEME") == "" {
		if err := util.SetTheme(""); err != nil {
			log.Warn().Applicationf("Failed to apply default theme: %v", err)
		} else {
			log.Info().Applicationf("🌈 Default theme applied")
		}
	}

	// Initialize global error handler (uses internal logger)
	if err := errutil.InitializeGlobalErrorHandler(log.GlobalLogger); err != nil {
		fmt.Fprintln(os.Stderr, "failed to initialize error handler:", err)
		os.Exit(1)
	}

	// Initialize unified error handler
	errorHandler := errors.NewErrorHandler()

	// Log bot startup
	log.Info().Applicationf("🚀 Starting bot...")

	// Ensure token present
	if token == "" {
		log.Error().Fatalf("❌ ALICE_BOT_DEVELOPMENT_TOKEN not set in environment or .env file")
	}

	// Add detailed logging for Discord authentication
	log.Info().Discordf("🔑 Attempting to authenticate with Discord API...")
	log.Info().Discordf("Using bot token (value redacted)")

	// Create Discord session and ensure safe shutdown
	discordSession, err := session.NewDiscordSession(token)
	if err != nil {
		log.Error().Errorf("❌ Authentication failed with Discord API: %v", err)
		log.Error().Fatalf("❌ Error creating Discord session: %v", err)
	}
	// Verify session state is properly initialized
	if discordSession.State == nil || discordSession.State.User == nil {
		log.Error().Fatalf("❌ Discord session state not properly initialized")
	}
	log.Info().Discordf("✅ Successfully authenticated with Discord API as %s#%s", discordSession.State.User.Username, discordSession.State.User.Discriminator)

	// Initialize minimal cache structure (idempotent)
	if err := util.EnsureCacheInitialized(); err != nil {
		log.Warn().Applicationf("Failed to initialize cache structure: %v", err)
	}

	// Ensure cache directories exist for new caches root
	if err := util.EnsureCacheDirs(); err != nil {
		log.Error().Errorf("Failed to create cache directories: %v", err)
		log.Error().Fatalf("❌ Failed to create cache directories")
	}

	// Ensure config and cache files exist
	if err := files.EnsureConfigFiles(); err != nil {
		log.Error().Errorf("Error checking config files: %v", err)
		log.Error().Fatalf("❌ Error checking config files")
	}

	// Initialize config manager
	configManager := files.NewConfigManager()
	// Load existing settings from disk before starting services
	if err := configManager.LoadConfig(); err != nil {
		log.Error().Errorf("Failed to load settings file: %v", err)
	}

	// Initialize SQLite store (messages, avatars, joins)
	store := storage.NewStore(util.GetMessageDBPath())
	if err := store.Init(); err != nil {
		log.Error().Errorf("Failed to initialize SQLite store: %v", err)
		log.Error().Fatalf("❌ Failed to initialize SQLite store")
	}

	// Log summary of configured guilds
	if err := files.LogConfiguredGuilds(configManager, discordSession); err != nil {
		log.Error().Errorf("Some configured guilds could not be accessed: %v", err)
	}

	// Downtime-aware silent avatar refresh before starting services/notifications
	if store != nil {
		if lastHB, ok, err := store.GetHeartbeat(); err == nil {
			if !ok || time.Since(lastHB) > 30*time.Minute {
				log.Info().Applicationf("⏱️ Detected downtime > 30m; performing silent avatar refresh before enabling notifications")
				if cfg := configManager.Config(); cfg != nil {
					for _, gcfg := range cfg.Guilds {
						after := ""
						for {
							members, err := discordSession.GuildMembers(gcfg.GuildID, after, 1000)
							if err != nil {
								log.Error().Errorf("Failed to list members for silent refresh for guild %s: %v", gcfg.GuildID, err)
								break
							}
							if len(members) == 0 {
								break
							}
							for _, member := range members {
								if member == nil || member.User == nil {
									continue
								}
								avatarHash := member.User.Avatar
								if avatarHash == "" {
									avatarHash = "default"
								}
								_, _, _ = store.UpsertAvatar(gcfg.GuildID, member.User.ID, avatarHash, time.Now())
							}
							last := members[len(members)-1]
							if last == nil || last.User == nil {
								break
							}
							after = last.User.ID
							if len(members) < 1000 {
								break
							}
						}
					}
				}
				log.Info().Applicationf("✅ Silent avatar refresh completed")
			} else {
				log.Info().Applicationf("No significant downtime detected; skipping silent avatar refresh")
			}
		} else {
			log.Error().Errorf("Failed to read last heartbeat; skipping downtime check: %v", err)
		}
		_ = store.SetHeartbeat(time.Now())
	}

	// Schedule periodic cleanup of obsolete data (every 6 hours)
	cleanupStop := cache.SchedulePeriodicCleanup(store, 6*time.Hour)
	defer func() {
		if cleanupStop != nil {
			close(cleanupStop)
		}
	}()

	// Initialize Service Manager
	serviceManager := service.NewServiceManager(errorHandler)

	// Create service wrappers for existing services
	log.Info().Applicationf("🔧 Creating service wrappers...")

	// Wrap MonitoringService
	monitoringService, err := logging.NewMonitoringService(discordSession, configManager, store)
	if err != nil {
		log.Error().Errorf("Failed to create monitoring service: %v", err)
		log.Error().Fatalf("❌ Failed to create monitoring service")
	}

	// Get unified cache from monitoring service for intelligent warmup
	unifiedCache := monitoringService.GetUnifiedCache()

	// Perform intelligent cache warmup after monitoring service is created
	// This will load persisted cache and fetch missing data from Discord
	warmupConfig := cache.DefaultWarmupConfig()
	warmupConfig.MaxMembersPerGuild = 500 // Limit initial member fetch
	if err := cache.IntelligentWarmup(discordSession, unifiedCache, store, warmupConfig); err != nil {
		log.Warn().Applicationf("Intelligent warmup failed (continuing): %v", err)
	}

	// Enable automatic cache persistence every 30 minutes
	persistStop := unifiedCache.SetPersistInterval(30 * time.Minute)
	defer func() {
		if persistStop != nil {
			close(persistStop)
		}
	}()

	monitoringWrapper := service.NewServiceWrapper(
		"monitoring",
		service.TypeMonitoring,
		service.PriorityHigh,
		[]string{},
		func() error { return monitoringService.Start() },
		func() error { return monitoringService.Stop() },
		func() bool { return true },
	)

	// Wrap AutomodService
	automodService := logging.NewAutomodService(discordSession, configManager)
	automodRouter := task.NewRouter(task.Defaults())
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

	// Register services with the manager
	if err := serviceManager.Register(monitoringWrapper); err != nil {
		log.Error().Errorf("Failed to register monitoring service: %v", err)
		log.Error().Fatalf("❌ Failed to register monitoring service")
	}

	if err := serviceManager.Register(automodWrapper); err != nil {
		log.Error().Errorf("Failed to register automod service: %v", err)
		log.Error().Fatalf("❌ Failed to register automod service")
	}

	// Start all services
	log.Info().Applicationf("🚀 Starting all services...")
	if err := serviceManager.StartAll(); err != nil {
		log.Error().Errorf("Failed to start services: %v", err)
		log.Error().Fatalf("❌ Failed to start services")
	}

	// Initialize and register bot commands
	commandHandler := commands.NewCommandHandler(discordSession, configManager)
	if err := commandHandler.SetupCommands(); err != nil {
		log.Error().Errorf("Error configuring slash commands: %v", err)
		log.Error().Fatalf("❌ Error configuring slash commands")
	}

	// Inject store and unified cache into command router for proper dependency injection
	if cm := commandHandler.GetCommandManager(); cm != nil {
		if router := cm.GetRouter(); router != nil {
			router.SetStore(store)
			// Inject unified cache into permission checker
			if monitoringService != nil {
				router.SetCache(monitoringService.GetUnifiedCache())
			}
		}
	}

	// Register admin commands
	adminCommands := admin.NewAdminCommands(serviceManager)
	adminCommands.RegisterCommands(commandHandler.GetCommandManager().GetRouter())

	// Ensure safe shutdown of all services
	defer func() {
		log.Info().Applicationf("🛑 Shutting down services...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := serviceManager.StopAll(); err != nil {
			log.Error().Errorf("Some services failed to stop cleanly: %v", err)
		}

		// Close task routers to stop background goroutines
		if automodRouter != nil {
			automodRouter.Close()
		}

		// Allow services to finish final writes before closing store
		time.Sleep(100 * time.Millisecond)

		if store != nil {
			_ = store.Close()
		}

		// Close Discord session
		if discordSession != nil {
			_ = discordSession.Close()
		}

		_ = shutdownCtx
	}()

	// Log successful initialization and wait for shutdown
	log.Info().Applicationf("🔗 Slash commands sync completed")
	log.Info().Applicationf("🎯 Bot initialized successfully!")
	log.Info().Applicationf("🤖 Bot running. Monitoring all configured guilds. Press Ctrl+C to stop...")

	util.WaitForInterrupt()
	log.Info().Applicationf("🛑 Stopping bot...")
}
