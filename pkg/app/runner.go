package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/control"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/admin"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/maintenance"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/errors"
	"github.com/small-frappuccino/discordcore/pkg/errutil"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
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

	// Theme configuration (now from settings.json runtime_config)

	// IMPORTANT: configManager is created later (after config files are ensured).
	// We cannot read runtime_config here without risking an undefined variable / nil config.
	// Theme will be applied right after loading settings.json (see below).

	// Runtime hot-apply manager (theme + ALICE_DISABLE_* toggles)
	// NOTE: The /config runtime panel triggers Apply() after persisting settings.json.
	var runtimeApplier *runtimeapply.Manager

	// Global error handler
	if err := errutil.InitializeGlobalErrorHandler(log.GlobalLogger); err != nil {
		return fmt.Errorf("initialize global error handler: %w", err)
	}

	// Error handler for service manager
	errorHandler := errors.NewErrorHandler()

	msg := formatStartupMessage(appName, AppVersion(), Version)
	log.ApplicationLogger().Info(msg)

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

	features := (&files.BotConfig{}).ResolveFeatures("")
	if cfg := configManager.Config(); cfg != nil {
		features = cfg.ResolveFeatures("")
	}
	monitoringEnabled := features.Services.Monitoring

	// Theme configuration (from settings.json runtime_config)
	{
		cfg := configManager.Config()
		themeName := ""
		if cfg != nil {
			themeName = cfg.RuntimeConfig.BotTheme
		}

		if err := util.ConfigureThemeFromConfig(themeName); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to set theme from runtime config %s: %v", "bot_theme", err))
		}
		if themeName == "" {
			if err := util.SetTheme(""); err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to apply default theme: %v", err))
			} else {
				log.ApplicationLogger().Info("🌈 Default theme applied")
			}
		}
	}

	// SQLite store (hardcoded path; no runtime_config override)
	dbPath := util.GetMessageDBPath()
	store := storage.NewStore(dbPath)
	if err := store.Init(); err != nil {
		return fmt.Errorf("initialize SQLite store: %w", err)
	}

	// Log configured guilds
	if err := files.LogConfiguredGuilds(configManager, discordSession); err != nil {
		log.ErrorLoggerRaw().Error(fmt.Sprintf("Some configured guilds could not be accessed: %v", err))
	}

	// Periodic cleanup (every 6 hours), can be disabled via runtime config
	var cleanupStop chan struct{}
	disableCleanup := false
	cleanupEnabled := features.Maintenance.DBCleanup
	if cfg := configManager.Config(); cfg != nil {
		disableCleanup = cfg.RuntimeConfig.DisableDBCleanup
	}
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

	// Service manager
	serviceManager := service.NewServiceManager(errorHandler)

	// Monitoring service (central orchestration + unified cache)
	monitoringService, err := logging.NewMonitoringService(discordSession, configManager, store)
	if err != nil {
		return fmt.Errorf("create monitoring service: %w", err)
	}

	// Create runtime hot-apply manager and set initial baseline from current config.
	// This lets the runtime config panel apply environment-like toggles without a full restart.
	runtimeApplier = runtimeapply.New(serviceManager, monitoringService)
	if cfg := configManager.Config(); cfg != nil {
		runtimeApplier.SetInitial(cfg.RuntimeConfig)
	}

	controlAddr := strings.TrimSpace(util.EnvString("ALICE_CONTROL_ADDR", ""))
	var controlServer *control.Server
	if controlAddr != "" {
		controlServer = control.NewServer(controlAddr, configManager, runtimeApplier)
		if controlServer == nil {
			log.ApplicationLogger().Warn("Control server disabled (invalid parameters)")
		} else if err := controlServer.Start(); err != nil {
			return fmt.Errorf("start control server: %w", err)
		} else {
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := controlServer.Stop(ctx); err != nil {
					log.ErrorLoggerRaw().Error("Failed to stop control server cleanly", "err", err)
				}
			}()
		}
	}

	// Cache warmup (persisted + fetch missing)
	// NOTE: Warmup responsibility is consolidated in the app runner.
	// MonitoringService does not perform its own warmup to avoid duplicate work during startup.
	unifiedCache := monitoringService.GetUnifiedCache()
	if monitoringEnabled {
		if unifiedCache != nil && unifiedCache.WasWarmedUpRecently(10*time.Minute) {
			log.ApplicationLogger().Info("Skipping cache warmup (recently warmed up)")
		} else {
			warmupConfig := cache.DefaultWarmupConfig()
			warmupConfig.MaxMembersPerGuild = 500 // mitigate initial load
			if err := cache.IntelligentWarmup(discordSession, unifiedCache, store, warmupConfig); err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Intelligent warmup failed (continuing): %v", err))
			}
		}
	}

	// Periodic cache persistence (hardcoded interval)
	persistInterval := time.Hour
	var persistStop chan struct{}
	if monitoringEnabled {
		persistStop = unifiedCache.SetPersistInterval(persistInterval)
	}
	defer func() {
		if persistStop != nil {
			close(persistStop)
		}
	}()

	// Wrap monitoring
	var monitoringWrapper *service.ServiceWrapper
	if monitoringEnabled {
		monitoringWrapper = service.NewServiceWrapper(
			"monitoring",
			service.TypeMonitoring,
			service.PriorityHigh,
			[]string{},
			func() error { return monitoringService.Start() },
			func() error { return monitoringService.Stop() },
			func() bool { return true },
		)
	} else {
		log.ApplicationLogger().Info("🛑 Monitoring service disabled by features.services.monitoring")
	}

	// Automod service with TaskRouter adapters (gated by runtime config)
	disableAutomod := false
	if cfg := configManager.Config(); cfg != nil {
		disableAutomod = cfg.RuntimeConfig.DisableAutomodLogs
	}
	var automodWrapper *service.ServiceWrapper
	if !features.Services.Automod {
		log.ApplicationLogger().Info("🛑 Automod service disabled by features.services.automod")
	} else if !features.Logging.AutomodAction {
		log.ApplicationLogger().Info("🛑 Automod logs disabled by features.logging.automod_action; AutomodService will not start")
	} else if disableAutomod {
		log.ApplicationLogger().Info("🛑 Automod logs disabled by runtime config disable_automod_logs; AutomodService will not start")
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
	if monitoringWrapper != nil {
		if err := serviceManager.Register(monitoringWrapper); err != nil {
			return fmt.Errorf("register monitoring service: %w", err)
		}
	}
	if automodWrapper != nil {
		if err := serviceManager.Register(automodWrapper); err != nil {
			return fmt.Errorf("register automod service: %w", err)
		}
	}

	// User prune service (optional; native Discord prune, day 28, 30-day window)
	{
		cfg := configManager.Config()
		enabled := false
		if cfg != nil {
			for _, g := range cfg.Guilds {
				feature := cfg.ResolveFeatures(g.GuildID)
				if !feature.UserPrune {
					continue
				}
				if g.UserPrune.Enabled {
					enabled = true
					break
				}
			}
		}

		if enabled {
			userPruneService := maintenance.NewUserPruneService(discordSession, configManager, store)
			userPruneWrapper := service.NewServiceWrapper(
				"user-prune",
				service.TypeMonitoring,
				service.PriorityNormal,
				[]string{"monitoring"},
				func() error { userPruneService.Start(); return nil },
				func() error { userPruneService.Stop(); return nil },
				func() bool { return userPruneService.IsRunning() },
			)
			if err := serviceManager.Register(userPruneWrapper); err != nil {
				return fmt.Errorf("register user prune service: %w", err)
			}
			log.ApplicationLogger().Info("✅ User prune enabled (Discord native prune: day 28, 30 days)")
		}
	}

	// Start services
	log.ApplicationLogger().Info("🚀 Starting all services...")
	if err := serviceManager.StartAll(); err != nil {
		return fmt.Errorf("start services: %w", err)
	}

	// Commands
	var commandHandler *commands.CommandHandler
	if features.Services.Commands {
		commandHandler = commands.NewCommandHandler(discordSession, configManager)
		if err := commandHandler.SetupCommands(); err != nil {
			return fmt.Errorf("configure slash commands: %w", err)
		}
	} else {
		log.ApplicationLogger().Info("🛑 Commands disabled by features.services.commands")
	}

	// NOTE:
	// The runtime hot-apply manager is created here and kept alive for the lifetime of the process.
	// The /config runtime panel should call runtimeApplier.Apply(ctx, nextRuntimeConfig) after saving.
	_ = runtimeApplier

	// Inject store and unified cache into command router
	if commandHandler != nil {
		if cm := commandHandler.GetCommandManager(); cm != nil {
			if router := cm.GetRouter(); router != nil {
				router.SetStore(store)
				if monitoringService != nil {
					router.SetCache(monitoringService.GetUnifiedCache())
					router.SetTaskRouter(monitoringService.TaskRouter())
				}
				// Wire runtime hot-apply manager so /config runtime can apply changes immediately.
				router.SetRuntimeApplier(runtimeApplier)
			}
		}
	}

	// Admin commands
	if features.Services.AdminCommands {
		if commandHandler != nil {
			adminCommands := admin.NewAdminCommands(serviceManager, unifiedCache, store)
			adminCommands.RegisterCommands(commandHandler.GetCommandManager().GetRouter())
		} else {
			log.ApplicationLogger().Warn("Admin commands enabled but commands are disabled; skipping admin command registration")
		}
	} else {
		log.ApplicationLogger().Info("🛑 Admin commands disabled by features.services.admin_commands")
	}

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

	// Avoid duplicated versions like: "alicebot v0.146.0 (discordcore v0.146.0)"
	if coreVersion == "" || (appVersion != "" && appVersion == coreVersion) {
		return msg + "..."
	}

	return msg + fmt.Sprintf(" (discordcore %s)...", coreVersion)
}
