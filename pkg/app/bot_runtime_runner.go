package app

import (
	"context"
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/admin"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/maintenance"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	coreerrors "github.com/small-frappuccino/discordcore/pkg/errors"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

type botRuntimeOptions struct {
	defaultBotInstanceID string
	runtimeCount         int
	configManager        *files.ConfigManager
	store                *storage.Store
	errorHandler         *coreerrors.ErrorHandler
	runtimeApplier       *runtimeapply.Manager
	partnerBoardService  partners.BoardService
	partnerSyncExecutor  partners.GuildSyncExecutor
	qotdCommandService   *applicationqotd.Service
	qotdLifecycleService discordqotd.GuildLifecycleService
	startupTasks         *startupTaskOrchestrator
}

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

func initializeBotRuntime(runtime *botRuntime, opts botRuntimeOptions) error {
	if runtime == nil || runtime.session == nil {
		return fmt.Errorf("bot runtime is unavailable")
	}
	if err := enforceRuntimeGuildAllowlist(runtime); err != nil {
		return fmt.Errorf("enforce runtime guild allowlist for %s: %w", runtime.instanceID, err)
	}
	registerRuntimeGuildAllowlistHandler(runtime)

	cfg := opts.configManager.Config()
	runtimeConfig := files.RuntimeConfig{}
	if cfg != nil {
		runtimeConfig = cfg.RuntimeConfig
	}
	routerConfig := newRuntimeTaskRouterConfig(cfg, runtime.instanceID, opts.defaultBotInstanceID, opts.runtimeCount)
	log.ApplicationLogger().Info(
		"Configured runtime task router budget",
		"botInstanceID", runtime.instanceID,
		"globalMaxWorkers", routerConfig.GlobalMaxWorkers,
		"sharedLimiter", routerConfig.ExecutionLimiter != nil,
	)

	runtime.serviceManager = service.NewServiceManager(opts.errorHandler)
	var monitoringService *logging.MonitoringService
	var unifiedCache *cache.UnifiedCache
	if runtime.capabilities.monitoring {
		var err error
		monitoringService, err = logging.NewMonitoringServiceForBot(
			runtime.session,
			opts.configManager,
			opts.store,
			runtime.instanceID,
			opts.defaultBotInstanceID,
		)
		if err != nil {
			return fmt.Errorf("create monitoring service for %s: %w", runtime.instanceID, err)
		}
		monitoringService.SetTaskRouterConfig(routerConfig)
		runtime.monitoringService = monitoringService
		unifiedCache = monitoringService.GetUnifiedCache()
		if unifiedCache != nil {
			runtime.persistStop = unifiedCache.SetPersistInterval(time.Hour)
		}
	} else {
		log.ApplicationLogger().Info("Monitoring runtime skipped; no effective monitoring workload is enabled", "botInstanceID", runtime.instanceID)
	}
	if opts.runtimeApplier != nil {
		opts.runtimeApplier.AddRuntime(runtime.serviceManager, monitoringService)
	}

	disableAutomod := runtimeConfig.DisableAutomodLogs
	var automodWrapper *service.ServiceWrapper
	if !runtime.capabilities.automod {
		log.ApplicationLogger().Info("Automod service skipped; no effective automod logging workload is enabled", "botInstanceID", runtime.instanceID)
	} else if disableAutomod {
		log.ApplicationLogger().Info("Automod logs disabled by runtime config disable_automod_logs; AutomodService will not start", "botInstanceID", runtime.instanceID)
	} else {
		automodService := logging.NewAutomodService(runtime.session, opts.configManager)
		automodRouter := task.NewRouter(routerConfig)
		notifier := logging.NewNotificationSender(runtime.session)
		if monitoringService != nil {
			notifier = monitoringService.Notifier()
		}
		automodAdapters := task.NewNotificationAdapters(automodRouter, runtime.session, opts.configManager, opts.store, notifier)
		automodService.SetAdapters(automodAdapters)

		automodWrapper = service.NewServiceWrapper(
			"automod",
			service.TypeAutomod,
			service.PriorityNormal,
			[]string{},
			func(context.Context) error { automodService.Start(); return nil },
			func(context.Context) error {
				automodRouter.Close()
				automodService.Stop()
				return nil
			},
			func() bool { return true },
		)
	}

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

	if runtime.capabilities.userPrune {
		userPruneService := maintenance.NewUserPruneServiceForBot(runtime.session, opts.configManager, opts.store, runtime.instanceID, opts.defaultBotInstanceID)
		userPruneDependencies := []string{}
		if monitoringService != nil {
			userPruneDependencies = []string{"monitoring"}
		}
		userPruneWrapper := service.NewServiceWrapper(
			"user-prune",
			service.TypeMonitoring,
			service.PriorityNormal,
			userPruneDependencies,
			func(context.Context) error { userPruneService.Start(); return nil },
			func(context.Context) error { userPruneService.Stop(); return nil },
			func() bool { return userPruneService.IsRunning() },
		)
		if err := runtime.serviceManager.Register(userPruneWrapper); err != nil {
			return fmt.Errorf("register user prune service for %s: %w", runtime.instanceID, err)
		}
		log.ApplicationLogger().Info("User prune enabled (Discord native prune: day 28, 30 days)", "botInstanceID", runtime.instanceID)
	}

	if runtime.capabilities.qotd && opts.qotdLifecycleService != nil {
		qotdRuntimeService := discordqotd.NewRuntimeServiceForBot(
			runtime.session,
			opts.configManager,
			opts.qotdLifecycleService,
			runtime.instanceID,
			opts.defaultBotInstanceID,
		)
		qotdWrapper := service.NewServiceWrapper(
			"qotd",
			service.TypeMonitoring,
			service.PriorityNormal,
			[]string{},
			func(context.Context) error { qotdRuntimeService.Start(); return nil },
			func(context.Context) error { qotdRuntimeService.Stop(); return nil },
			func() bool { return qotdRuntimeService.IsRunning() },
		)
		if err := runtime.serviceManager.Register(qotdWrapper); err != nil {
			return fmt.Errorf("register qotd runtime service for %s: %w", runtime.instanceID, err)
		}
		log.ApplicationLogger().Info("QOTD runtime enabled", "botInstanceID", runtime.instanceID)
	}

	log.ApplicationLogger().Info("Starting runtime services", "botInstanceID", runtime.instanceID)
	if err := runtime.serviceManager.StartAll(); err != nil {
		return fmt.Errorf("start services for %s: %w", runtime.instanceID, err)
	}

	if runtime.capabilities.commands {
		commandHandler := newCommandHandlerForBot(runtime.session, opts.configManager, runtime.instanceID, opts.defaultBotInstanceID)
		commandHandler.SetPartnerBoardService(opts.partnerBoardService)
		commandHandler.SetPartnerBoardSyncExecutor(opts.partnerSyncExecutor)
		commandHandler.SetQOTDService(opts.qotdCommandService)
		if err := setupCommandHandler(commandHandler); err != nil {
			return fmt.Errorf("configure slash commands for %s: %w", runtime.instanceID, err)
		}
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
		if runtime.capabilities.admin {
			adminCommands := admin.NewAdminCommands(runtime.serviceManager, unifiedCache, opts.store)
			adminCommands.RegisterCommands(commandHandler.GetCommandManager().GetRouter())
		}
		runtime.commandHandler = commandHandler
	} else {
		log.ApplicationLogger().Info("Commands skipped; no guild bound to this runtime has commands enabled", "botInstanceID", runtime.instanceID)
	}

	scheduleRuntimeConfiguredGuildLogging(runtime, opts.configManager, opts.defaultBotInstanceID, opts.startupTasks)
	scheduleRuntimeWarmup(runtime, opts.store, opts.startupTasks)
	return nil
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

func scheduleRuntimeWarmup(runtime *botRuntime, store *storage.Store, startupTasks *startupTaskOrchestrator) {
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
		if err := runWarmup(context.Background(), baseWarmupConfig); err != nil {
			log.ApplicationLogger().Warn(
				fmt.Sprintf("Cache warmup base phase failed (continuing): %v", err),
				"botInstanceID", runtime.instanceID,
			)
			return
		}
		if err := runWarmup(context.Background(), memberWarmupConfig); err != nil {
			log.ApplicationLogger().Warn(
				fmt.Sprintf("Cache warmup member phase failed (continuing): %v", err),
				"botInstanceID", runtime.instanceID,
			)
		}
		return
	}

	log.ApplicationLogger().Info("Scheduling cache warmup base phase in background", "botInstanceID", runtime.instanceID)
	startupTasks.GoHeavy("cache_warmup_base:"+runtime.instanceID, func(ctx context.Context) error {
		if err := runWarmup(ctx, baseWarmupConfig); err != nil {
			if ctx.Err() != nil {
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

		if err := runWarmup(ctx, memberWarmupConfig); err != nil {
			if ctx.Err() != nil {
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
	if runtime.commandHandler != nil {
		if err := shutdownCommandHandler(runtime.commandHandler); err != nil {
			errs = append(errs, fmt.Errorf("shutdown command handler for %s: %w", runtime.instanceID, err))
		}
	}
	if runtime.persistStop != nil {
		close(runtime.persistStop)
		runtime.persistStop = nil
	}
	if runtime.cleanupStop != nil {
		close(runtime.cleanupStop)
		runtime.cleanupStop = nil
	}
	_ = ctx
	return errs
}
