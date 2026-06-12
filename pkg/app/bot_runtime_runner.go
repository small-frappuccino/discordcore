package app

import (
	"context"
	"fmt"
	"time"

	"github.com/small-frappuccino/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/maintenance"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

type botRuntimeOptions struct {
	runtimeCount             int
	configManager            *files.ConfigManager
	store                    *storage.Store
	commandCatalogRegistrars []commands.CommandCatalogRegistrar
	runtimeApplier           *runtimeapply.Manager
	qotdCommandService       *applicationqotd.Service
	qotdLifecycleService     discordqotd.GuildLifecycleService
	moderationMetrics        moderation.Metrics
	startupTasks             *StartupTaskOrchestrator
	profile                  RunProfile
}

var openBotDiscordSession = session.OpenSession

func openBotRuntime(instance resolvedBotInstance, capabilities botRuntimeCapabilities) (*botRuntime, error) {
	log.DiscordLogger().Info(
		"Attempting to authenticate with Discord API...",
		"botInstanceID", instance.ID,
	)
	log.DiscordLogger().Info("Using bot token (value redacted)", "botInstanceID", instance.ID)

	discordSession, err := newDiscordSessionWithIntents(instance.Token, capabilities.intents)
	if err != nil {
		return nil, fmt.Errorf("create discord session for %s: %w", instance.ID, err)
	}

	if instance.DiscordStatus != "" {
		discordSession.Identify.Presence = discordgo.GatewayStatusUpdate{
			Status: instance.DiscordStatus,
		}
	}

	// Estabelecer o handshake com o Discord respeitando o timeout do supervisor (implícito no loop de retry,
	// mas adicionamos timeout explícito para não trancar o bot)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := openBotDiscordSession(ctx, discordSession); err != nil {
		return nil, fmt.Errorf("open discord session for %s: %w", instance.ID, err)
	}

	if discordSession.State == nil || discordSession.State.User == nil {
		return nil, fmt.Errorf("discord session state not properly initialized for %s", instance.ID)
	}

	log.DiscordLogger().Info(
		"Authenticated with Discord",
		"botInstanceID", instance.ID,
		"botUser", fmt.Sprintf("%s#%s", discordSession.State.User.Username, discordSession.State.User.Discriminator),
	)
	return &botRuntime{
		instanceID:   instance.ID,
		capabilities: capabilities,
		session:      discordSession,
	}, nil
}

func initializeBotRuntime(ctx context.Context, runtime *botRuntime, opts botRuntimeOptions) error {
	if runtime == nil || runtime.session == nil {
		return fmt.Errorf("bot runtime is unavailable")
	}

	cfg := opts.configManager.Config()
	runtimeConfig := files.RuntimeConfig{}
	if cfg != nil {
		runtimeConfig = cfg.RuntimeConfig
	}
	routerConfig := newRuntimeTaskRouterConfig(cfg, runtime.instanceID, opts.runtimeCount)
	log.ApplicationLogger().Info(
		"Configured runtime task router budget",
		"botInstanceID", runtime.instanceID,
		"globalMaxWorkers", routerConfig.GlobalMaxWorkers,
		"sharedLimiter", routerConfig.ExecutionLimiter != nil,
	)

	runtime.serviceManager = service.NewServiceManager()

	monitoringService, err := setupMonitoringService(runtime, opts, routerConfig)
	if err != nil {
		return err
	}
	if opts.runtimeApplier != nil {
		opts.runtimeApplier.AddRuntime(runtime.serviceManager, monitoringService)
	}

	automodWrapper := buildAutomodWrapper(runtime, opts, routerConfig, runtimeConfig, monitoringService)

	if monitoringService != nil {
		if err := runtime.serviceManager.Register(monitoringService); err != nil {
			return fmt.Errorf("register monitoring service for %s: %w", runtime.instanceID, err)
		}
	}
	if automodWrapper != nil {
		if err := runtime.serviceManager.Register(automodWrapper); err != nil {
			return fmt.Errorf("register automod service for %s: %w", runtime.instanceID, err)
		}
	}

	if err := registerUserPruneService(runtime, opts, monitoringService); err != nil {
		return err
	}
	if err := registerQOTDRuntimeService(runtime, opts); err != nil {
		return err
	}

	commandWrapper := setupRuntimeCommandHandler(runtime, opts, cfg, monitoringService)
	if commandWrapper != nil {
		if err := runtime.serviceManager.Register(commandWrapper); err != nil {
			return fmt.Errorf("register command handler service for %s: %w", runtime.instanceID, err)
		}
	}

	log.ApplicationLogger().Info("Starting runtime services", "botInstanceID", runtime.instanceID)
	if err := runtime.serviceManager.StartAll(); err != nil {
		return fmt.Errorf("start services for %s: %w", runtime.instanceID, err)
	}

	scheduleRuntimeConfiguredGuildLogging(runtime, opts.configManager, opts.startupTasks)
	scheduleRuntimeWarmup(ctx, runtime, opts.store, opts.startupTasks)
	return nil
}

// setupMonitoringService creates and wires the per-runtime monitoring service when the
// runtime has the monitoring capability, configuring its task-router budget and cache
// persistence interval. It returns (nil, nil) when monitoring is not enabled.
func setupMonitoringService(runtime *botRuntime, opts botRuntimeOptions, routerConfig task.RouterConfig) (*logging.MonitoringService, error) {
	if !runtime.capabilities.monitoring {
		log.ApplicationLogger().Info("Monitoring runtime skipped; no effective monitoring workload is enabled", "botInstanceID", runtime.instanceID)
		return nil, nil
	}

	// Per-runtime metrics: each bot's monitoring service writes to its
	// own InMemoryMetrics. The control plane reads via the default
	// runtime's MonitoringMetricsResolver, mirroring the cache
	// observability resolver pattern.
	monitoringService, err := logging.NewMonitoringServiceForBotWithMetrics(
		runtime.session,
		opts.configManager,
		opts.store,
		runtime.instanceID,
		"",
		&logging.InMemoryMetrics{},
		log.DiscordLogger(),
	)
	if err != nil {
		return nil, fmt.Errorf("create monitoring service for %s: %w", runtime.instanceID, err)
	}
	monitoringService.SetTaskRouterConfig(routerConfig)
	runtime.monitoringService = monitoringService
	return monitoringService, nil
}

// buildAutomodWrapper constructs the automod logging service wrapper when the runtime
// has the automod capability and automod logs are not disabled, sharing the monitoring
// notifier when available. It returns nil when automod should not run.
func buildAutomodWrapper(runtime *botRuntime, opts botRuntimeOptions, routerConfig task.RouterConfig, runtimeConfig files.RuntimeConfig, monitoringService *logging.MonitoringService) *service.LegacyServiceWrapper {
	if !runtime.capabilities.automod {
		log.ApplicationLogger().Info("Automod service skipped; no effective automod logging workload is enabled", "botInstanceID", runtime.instanceID)
		return nil
	}
	if runtimeConfig.DisableAutomodLogs {
		log.ApplicationLogger().Info("Automod logs disabled by runtime config disable_automod_logs; AutomodService will not start", "botInstanceID", runtime.instanceID)
		return nil
	}

	automodService := logging.NewAutomodService(runtime.session, opts.configManager, runtime.instanceID, "")
	automodRouter := task.NewRouter(routerConfig)
	notifier := logging.NewNotificationSender(runtime.session, log.DiscordLogger())
	if monitoringService != nil {
		notifier = monitoringService.Notifier()
	}
	automodAdapters := &task.NotificationAdapters{
		Router:   automodRouter,
		Session:  runtime.session,
		Config:   opts.configManager,
		Store:    opts.store,
		Notifier: notifier,
	}
	automodAdapters.RegisterHandlers()
	automodService.SetAdapters(automodAdapters)

	return service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:         "automod",
		Type:         service.TypeAutomod,
		Priority:     service.PriorityNormal,
		Dependencies: []string{},
		Start:        func(context.Context) error { automodService.Start(); return nil },
		Stop: func(context.Context) error {
			automodRouter.Close()
			automodService.Stop()
			return nil
		},
		Check: func() bool { return true },
	})
}

// registerUserPruneService registers the Discord-native user prune maintenance service
// when the runtime has the userPrune capability.
func registerUserPruneService(runtime *botRuntime, opts botRuntimeOptions, monitoringService *logging.MonitoringService) error {
	if !runtime.capabilities.userPrune {
		return nil
	}
	userPruneService := maintenance.NewUserPruneServiceForBot(runtime.session, opts.configManager, opts.store, runtime.instanceID, "")
	userPruneDependencies := []string{}
	if monitoringService != nil {
		userPruneDependencies = []string{"monitoring"}
	}
	userPruneWrapper := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:         "user-prune",
		Type:         service.TypeMonitoring,
		Priority:     service.PriorityNormal,
		Dependencies: userPruneDependencies,
		Start:        func(context.Context) error { userPruneService.Start(); return nil },
		Stop:         func(context.Context) error { userPruneService.Stop(); return nil },
		Check:        func() bool { return userPruneService.IsRunning() },
	})
	if err := runtime.serviceManager.Register(userPruneWrapper); err != nil {
		return fmt.Errorf("register user prune service for %s: %w", runtime.instanceID, err)
	}
	log.ApplicationLogger().Info("User prune enabled (Discord native prune: day 28, 30 days)", "botInstanceID", runtime.instanceID)
	return nil
}

// registerQOTDRuntimeService registers the QOTD runtime service when the runtime has the
// qotdRuntime capability and a lifecycle service is wired.
func registerQOTDRuntimeService(runtime *botRuntime, opts botRuntimeOptions) error {
	if !runtime.capabilities.qotdRuntime || opts.qotdLifecycleService == nil {
		return nil
	}
	qotdRuntimeService := discordqotd.NewRuntimeServiceForBot(
		runtime.session,
		opts.configManager,
		opts.qotdLifecycleService,
		runtime.instanceID,
		"",
	)
	qotdWrapper := service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:         "qotd",
		Type:         service.TypeMonitoring,
		Priority:     service.PriorityNormal,
		Dependencies: []string{},
		Start:        func(context.Context) error { qotdRuntimeService.Start(); return nil },
		Stop:         func(context.Context) error { qotdRuntimeService.Stop(); return nil },
		Check:        func() bool { return qotdRuntimeService.IsRunning() },
	})
	if err := runtime.serviceManager.Register(qotdWrapper); err != nil {
		return fmt.Errorf("register qotd runtime service for %s: %w", runtime.instanceID, err)
	}
	log.ApplicationLogger().Info("QOTD runtime enabled", "botInstanceID", runtime.instanceID)
	return nil
}

// setupRuntimeCommandHandler builds and registers the slash-command handler for runtimes
// that expose commands; otherwise it logs why commands were skipped.
func setupRuntimeCommandHandler(runtime *botRuntime, opts botRuntimeOptions, cfg *files.BotConfig, monitoringService *logging.MonitoringService) *service.LegacyServiceWrapper {
	if !runtime.capabilities.HasCommands() {
		logRuntimeCommandsSkipped(runtime, opts, cfg)

		// If the bot has a valid token, we must still synchronize an empty command list to Discord.
		// Otherwise, previously registered commands from an earlier capability assignment
		// (or before registry sync existed) will remain perpetually cached in the guild or global scope.
		if runtime.session != nil && runtime.session.Token != "" {
			commandHandler := newCommandHandlerForBot(runtime.session, opts.configManager, runtime.instanceID)
			return service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
				Name:         "commands-clear",
				Type:         service.TypeCommands,
				Priority:     service.PriorityHigh,
				Dependencies: []string{},
				Start: func(context.Context) error {
					return setupCommandHandler(commandHandler)
				},
				Stop: func(context.Context) error {
					return shutdownCommandHandler(commandHandler)
				},
				Check: func() bool { return true },
			})
		}
		return nil
	}

	commandHandler := newCommandHandlerForBot(runtime.session, opts.configManager, runtime.instanceID)
	if len(opts.commandCatalogRegistrars) > 0 {
		commandHandler.SetCommandCatalogRegistrars(opts.commandCatalogRegistrars...)
	}
	commandHandler.SetCommandCatalogCapabilities(commands.CommandCatalogCapabilities{
		Admin: runtime.capabilities.admin,
		Stats: runtime.capabilities.stats,
	})
	commandHandler.SetQOTDService(opts.qotdCommandService)
	commandHandler.SetModerationMetrics(opts.moderationMetrics)
	// Cache observability flows through /v1/health/cache via the control server's
	// runtime resolver, not the admin command catalog.
	commandHandler.SetAdminCommandServices(runtime.serviceManager)

	if cm := commandHandler.GetCommandManager(); cm != nil {
		if router := cm.GetRouter(); router != nil {
			router.SetStore(opts.store)
			if monitoringService != nil {
				router.SetCache(monitoringService.GetUnifiedCache())
				router.SetTaskRouter(monitoringService.TaskRouter())
			}
			router.SetRuntimeApplier(opts.runtimeApplier)
		}
	}
	runtime.commandHandler = commandHandler

	deps := []string{}
	if monitoringService != nil {
		deps = append(deps, "monitoring")
	}

	return service.NewLegacyServiceWrapper(service.LegacyServiceWrapperSpec{
		Name:         "command-handler",
		Type:         service.TypeCommands,
		Priority:     service.PriorityNormal,
		Dependencies: deps,
		Start:        func(context.Context) error { return setupCommandHandler(commandHandler) },
		Stop:         func(context.Context) error { return shutdownCommandHandler(commandHandler) },
		Check:        func() bool { return true },
	})
}

func logRuntimeCommandsSkipped(runtime *botRuntime, opts botRuntimeOptions, cfg *files.BotConfig) {
	log.ApplicationLogger().Info("Commands skipped; no guild bound to this runtime has commands enabled", "botInstanceID", runtime.instanceID)
}

var intelligentWarmupFn = cache.IntelligentWarmupContext
var monitoringUnifiedCacheFn = func(ms *logging.MonitoringService) *cache.UnifiedCache {
	if ms == nil {
		return nil
	}
	return ms.GetUnifiedCache()
}
var scheduleStartupMemberWarmupFn = func(ms *logging.MonitoringService, config cache.WarmupConfig) bool {
	if ms == nil {
		return false
	}
	return ms.ScheduleStartupMemberWarmup(config)
}

func scheduleRuntimeWarmup(ctx context.Context, runtime *botRuntime, store *storage.Store, startupTasks *StartupTaskOrchestrator) {
	if runtime == nil || runtime.session == nil || !runtime.capabilities.warmup || runtime.monitoringService == nil {
		return
	}

	unifiedCache := monitoringUnifiedCacheFn(runtime.monitoringService)
	if unifiedCache == nil {
		return
	}
	if unifiedCache.WasWarmedUpRecently(10 * time.Minute) {
		log.ApplicationLogger().Info("Skipping cache warmup (recently warmed up)", "botInstanceID", runtime.instanceID)
		return
	}

	baseWarmupConfig, memberWarmupConfig := runtimeWarmupPhases()
	runWarmup := func(ctx context.Context, config cache.WarmupConfig) error {
		return intelligentWarmupFn(ctx, runtime.session, unifiedCache, store, config)
	}

	if startupTasks == nil {
		go func() {
			if err := runWarmup(ctx, baseWarmupConfig); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.ApplicationLogger().Warn(
					fmt.Sprintf("Cache warmup base phase failed (continuing): %v", err),
					"botInstanceID", runtime.instanceID,
				)
				return
			}
			if scheduleStartupMemberWarmupFn(runtime.monitoringService, memberWarmupConfig) {
				return
			}
			if err := runWarmup(ctx, memberWarmupConfig); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.ApplicationLogger().Warn(
					fmt.Sprintf("Cache warmup member phase failed (continuing): %v", err),
					"botInstanceID", runtime.instanceID,
				)
			}
		}()
		return
	}

	log.ApplicationLogger().Info("Scheduling cache warmup base phase in background", "botInstanceID", runtime.instanceID)
	startupTasks.GoHeavy("cache_warmup_base:"+runtime.instanceID, func(taskCtx context.Context) error {
		// Respect both the task context and the supervisor context
		localCtx, localCancel := context.WithCancel(taskCtx)
		defer localCancel()
		go func() {
			select {
			case <-ctx.Done():
				localCancel()
			case <-localCtx.Done():
			}
		}()

		if err := runWarmup(localCtx, baseWarmupConfig); err != nil {
			if localCtx.Err() != nil {
				return nil
			}
			log.ApplicationLogger().Warn(
				fmt.Sprintf("Cache warmup base phase failed (continuing): %v", err),
				"botInstanceID", runtime.instanceID,
			)
			return nil
		}

		if scheduleStartupMemberWarmupFn(runtime.monitoringService, memberWarmupConfig) {
			log.ApplicationLogger().Info("Queued cache warmup member phase behind startup monitoring tasks", "botInstanceID", runtime.instanceID)
			return nil
		}

		if err := runWarmup(localCtx, memberWarmupConfig); err != nil {
			if localCtx.Err() != nil {
				return nil
			}
			log.ApplicationLogger().Warn(
				fmt.Sprintf("Cache warmup member phase failed (continuing): %v", err),
				"botInstanceID", runtime.instanceID,
			)
		}
		return nil
	})
}

func runtimeWarmupPhases() (cache.WarmupConfig, cache.WarmupConfig) {
	base := cache.DefaultWarmupConfig()
	base.FetchMissingMembers = false
	base.MaxMembersPerGuild = 500

	members := cache.DefaultWarmupConfig()
	members.FetchMissingGuilds = false
	members.FetchMissingRoles = false
	members.FetchMissingChannels = false
	members.MaxMembersPerGuild = 500

	return base, members
}

func shutdownBotRuntime(runtime *botRuntime, ctx context.Context) []error {
	if runtime == nil {
		return nil
	}

	var errs []error
	if runtime.serviceManager != nil {
		if err := runtime.serviceManager.StopAll(); err != nil {
			errs = append(errs, fmt.Errorf("stop services for %s: %w", runtime.instanceID, err))
		}
	}
	return errs
}
