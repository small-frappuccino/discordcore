package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/small-frappuccino/discordgo"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/clock"
	discord_automod "github.com/small-frappuccino/discordcore/pkg/discord/automod"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/eventlog"
	"github.com/small-frappuccino/discordcore/pkg/discord/maintenance"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	discordstats "github.com/small-frappuccino/discordcore/pkg/discord/stats"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/stats"
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
	appClock                 clock.Clock
	controlServerRegistry    *controlServerHolder
	logger                   *slog.Logger
}

var openBotDiscordSession = session.OpenSession

func openBotRuntime(instance resolvedBotInstance, capabilities botRuntimeCapabilities) (*botRuntime, error) {
	slog.Info("Architectural state transition: Initializing primary Discord API routine",
		slog.String("botInstanceID", instance.ID),
	)

	slog.Debug("Injecting runtime configuration payload",
		slog.String("botInstanceID", instance.ID),
		slog.Int("intents", int(capabilities.intents)),
	)

	discordSession, err := newDiscordSessionWithIntents(instance.Token, capabilities.intents)
	if err != nil {
		errWrap := fmt.Errorf("create discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure during session initialization", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	if instance.DiscordStatus != "" {
		slog.Debug("Applying dynamic gateway status update",
			slog.String("status", instance.DiscordStatus),
		)
		discordSession.Identify.Presence = discordgo.GatewayStatusUpdate{
			Status: instance.DiscordStatus,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := openBotDiscordSession(ctx, discordSession); err != nil {
		errWrap := fmt.Errorf("open discord session for %s: %w", instance.ID, err)
		log.EmitBlockingError("Blocking structural failure during socket bind and handshake", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	if discordSession.State == nil || discordSession.State.User == nil {
		errState := fmt.Errorf("discord session state not properly initialized for %s", instance.ID)
		log.EmitBlockingError("Blocking structural failure: Gateway payload yielded nil state", errState, log.GenerateRequestID())
		return nil, errState
	}

	slog.Info("Architectural state transition: Socket bound and API authenticated",
		slog.String("botInstanceID", instance.ID),
		slog.String("botUser", fmt.Sprintf("%s#%s", discordSession.State.User.Username, discordSession.State.User.Discriminator)),
	)

	return &botRuntime{
		instanceID:   instance.ID,
		capabilities: capabilities,
		session:      discordSession,
	}, nil
}

func initializeBotRuntime(ctx context.Context, runtime *botRuntime, opts botRuntimeOptions) error {
	if runtime == nil || runtime.session == nil {
		err := fmt.Errorf("bot runtime is unavailable")
		log.EmitBlockingError("Blocking structural failure: Runtime pointer resolves to nil", err, log.GenerateRequestID())
		return err
	}

	cfg := opts.configManager.Config()
	runtimeConfig := files.RuntimeConfig{}
	if cfg != nil {
		runtimeConfig = cfg.RuntimeConfig
	}

	if opts.controlServerRegistry != nil {
		slog.Debug("Binding control server registry event handlers")
		runtime.session.AddHandler(func(s *discordgo.Session, e *discordgo.GuildCreate) {
			opts.controlServerRegistry.BroadcastGuildEvent(e.Guild.ID, true)
		})
		runtime.session.AddHandler(func(s *discordgo.Session, e *discordgo.GuildDelete) {
			opts.controlServerRegistry.BroadcastGuildEvent(e.Guild.ID, false)
		})
	}

	routerConfig := newRuntimeTaskRouterConfig(cfg, runtime.instanceID, opts.runtimeCount)
	slog.Info("Architectural state transition: Configured runtime task router budget",
		slog.String("botInstanceID", runtime.instanceID),
		slog.Int("globalMaxWorkers", routerConfig.GlobalMaxWorkers),
		slog.Bool("sharedLimiter", routerConfig.ExecutionLimiter != nil),
	)

	runtime.serviceManager = service.NewServiceManager(slog.Default())

	monitoringService, err := setupMonitoringService(runtime, opts, routerConfig)
	if err != nil {
		return err
	}
	if opts.runtimeApplier != nil {
		opts.runtimeApplier.AddRuntime(runtime.serviceManager, monitoringService)
	}

	if monitoringService != nil {
		if err := runtime.serviceManager.Register(monitoringService); err != nil {
			errWrap := fmt.Errorf("register monitoring service for %s: %w", runtime.instanceID, err)
			log.EmitBlockingError("Blocking structural failure during service registry update", errWrap, log.GenerateRequestID())
			return errWrap
		}
	}
	if automodService := buildAutomodService(runtime, opts, routerConfig, runtimeConfig, monitoringService); automodService != nil {
		if err := runtime.serviceManager.Register(automodService); err != nil {
			errWrap := fmt.Errorf("register automod service for %s: %w", runtime.instanceID, err)
			log.EmitBlockingError("Blocking structural failure during service registry update", errWrap, log.GenerateRequestID())
			return errWrap
		}
	}

	if err := registerUserPruneService(runtime, opts, monitoringService); err != nil {
		return err
	}
	if err := registerQOTDRuntimeService(runtime, opts); err != nil {
		return err
	}

	token := runtime.session.Token
	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}
	arikawaState := state.New(token)
	runtime.arikawaState = arikawaState
	statsGateway := discordstats.NewArikawaGateway(arikawaState, slog.Default())
	statsService := stats.NewStatsService(statsGateway, opts.configManager, opts.store, slog.Default(), runtime.instanceID)
	discordstats.RegisterDiscordGoEventHandlers(runtime.session, statsService, slog.Default())

	if err := runtime.serviceManager.Register(statsService); err != nil {
		errWrap := fmt.Errorf("register stats service for %s: %w", runtime.instanceID, err)
		log.EmitBlockingError("Blocking structural failure during service registry update", errWrap, log.GenerateRequestID())
		return errWrap
	}

	if commandHandler := setupRuntimeCommandHandler(runtime, opts, cfg, monitoringService, statsService); commandHandler != nil {
		if err := runtime.serviceManager.Register(commandHandler); err != nil {
			errWrap := fmt.Errorf("register command handler service for %s: %w", runtime.instanceID, err)
			log.EmitBlockingError("Blocking structural failure during service registry update", errWrap, log.GenerateRequestID())
			return errWrap
		}
	}

	slog.Info("Architectural state transition: Executing StartAll across service manager instances",
		slog.String("botInstanceID", runtime.instanceID),
	)
	if err := runtime.serviceManager.StartAll(); err != nil {
		errWrap := fmt.Errorf("start services for %s: %w", runtime.instanceID, err)
		log.EmitBlockingError("Blocking structural failure: Service manager execution sequence aborted", errWrap, log.GenerateRequestID())
		return errWrap
	}

	scheduleRuntimeConfiguredGuildLogging(runtime, opts.configManager, opts.startupTasks)
	scheduleRuntimeWarmup(ctx, runtime, opts.store, opts.startupTasks)
	return nil
}

func setupMonitoringService(runtime *botRuntime, opts botRuntimeOptions, routerConfig task.RouterConfig) (*monitoring.MonitoringService, error) {
	if !runtime.capabilities.monitoring {
		slog.Info("Architectural state bypass: Monitoring runtime skipped due to explicit capability flags",
			slog.String("botInstanceID", runtime.instanceID),
		)
		return nil, nil
	}

	var eventLogger *eventlog.Logger
	if runtime.arikawaState != nil && runtime.arikawaState.Session != nil {
		eventLogger = eventlog.NewLogger(runtime.arikawaState.Session.Client, opts.configManager, runtime.session, slog.Default())
	}

	monitoringService, err := monitoring.NewMonitoringServiceForBotWithMetrics(
		runtime.session,
		runtime.arikawaState,
		eventLogger,
		opts.configManager,
		opts.store,
		runtime.instanceID,
		&monitoring.InMemoryMetrics{},
		slog.Default(),
	)
	if err != nil {
		errWrap := fmt.Errorf("create monitoring service for %s: %w", runtime.instanceID, err)
		log.EmitBlockingError("Blocking structural failure during monitoring instantiation", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}
	monitoringService.SetTaskRouterConfig(routerConfig)
	runtime.monitoringService = monitoringService
	return monitoringService, nil
}

func buildAutomodService(runtime *botRuntime, opts botRuntimeOptions, routerConfig task.RouterConfig, runtimeConfig files.RuntimeConfig, monitoringService *monitoring.MonitoringService) service.Service {
	if !runtime.capabilities.automod {
		slog.Info("Architectural state bypass: Automod service skipped due to explicit capability flags",
			slog.String("botInstanceID", runtime.instanceID),
		)
		return nil
	}
	if runtimeConfig.DisableAutomodLogs {
		slog.Info("Architectural state bypass: Automod logs strictly disabled via configuration manifest",
			slog.String("botInstanceID", runtime.instanceID),
		)
		return nil
	}

	automodService := discord_automod.NewArikawaAdapter(runtime.arikawaState, monitoringService, opts.logger)

	return automodService
}

func registerUserPruneService(runtime *botRuntime, opts botRuntimeOptions, monitoringService *monitoring.MonitoringService) error {
	if !runtime.capabilities.userPrune {
		return nil
	}
	userPruneService := maintenance.NewUserPruneService(runtime.session, opts.configManager, opts.store, runtime.instanceID)
	userPruneDependencies := []string{}
	if monitoringService != nil {
		userPruneDependencies = []string{"monitoring"}
	}
	userPruneService.SetDependencies(userPruneDependencies)

	if err := runtime.serviceManager.Register(userPruneService); err != nil {
		errWrap := fmt.Errorf("register user prune service for %s: %w", runtime.instanceID, err)
		log.EmitBlockingError("Blocking structural failure during user prune registration", errWrap, log.GenerateRequestID())
		return errWrap
	}
	slog.Info("Architectural state transition: User prune operational routine initialized",
		slog.String("botInstanceID", runtime.instanceID),
	)
	return nil
}

func registerQOTDRuntimeService(runtime *botRuntime, opts botRuntimeOptions) error {
	if !runtime.capabilities.qotdRuntime || opts.qotdLifecycleService == nil {
		return nil
	}
	qotdRuntimeService := discordqotd.NewRuntimeServiceForBot(
		runtime.session,
		opts.configManager,
		opts.qotdLifecycleService,
		runtime.instanceID,
	)
	if opts.appClock != nil {
		qotdRuntimeService.SetClock(opts.appClock)
	}
	if err := runtime.serviceManager.Register(qotdRuntimeService); err != nil {
		errWrap := fmt.Errorf("register qotd runtime service for %s: %w", runtime.instanceID, err)
		log.EmitBlockingError("Blocking structural failure during QOTD runtime registration", errWrap, log.GenerateRequestID())
		return errWrap
	}
	slog.Info("Architectural state transition: QOTD runtime initialized",
		slog.String("botInstanceID", runtime.instanceID),
	)
	return nil
}

func setupRuntimeCommandHandler(runtime *botRuntime, opts botRuntimeOptions, cfg *files.BotConfig, monitoringService *monitoring.MonitoringService, statsService *stats.StatsService) service.Service {
	if !runtime.capabilities.HasCommands() {
		logRuntimeCommandsSkipped(runtime, opts, cfg)

		if runtime.session != nil && runtime.session.Token != "" {
			commandHandler := newCommandHandlerForBot(runtime.session, opts.configManager, runtime.instanceID)
			return commandHandler
		}
		return nil
	}

	commandHandler := newCommandHandlerForBot(runtime.session, opts.configManager, runtime.instanceID)
	if len(opts.commandCatalogRegistrars) > 0 {
		commandHandler.SetCommandCatalogRegistrars(opts.commandCatalogRegistrars...)
	}
	commandHandler.SetCommandCatalogCapabilities(commands.CommandCatalogCapabilities{
		Stats: runtime.capabilities.stats,
	})
	commandHandler.SetQOTDService(opts.qotdCommandService)
	commandHandler.SetModerationMetrics(opts.moderationMetrics)
	commandHandler.SetStatsService(statsService)

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
	commandHandler.SetDependencies(deps)

	return commandHandler
}

func logRuntimeCommandsSkipped(runtime *botRuntime, opts botRuntimeOptions, cfg *files.BotConfig) {
	slog.Info("Architectural state bypass: Commands skipped due to empty guild bindings",
		slog.String("botInstanceID", runtime.instanceID),
	)
}

var intelligentWarmupFn = cache.IntelligentWarmupContext
var monitoringUnifiedCacheFn = func(ms *monitoring.MonitoringService) *cache.UnifiedCache {
	if ms == nil {
		return nil
	}
	return ms.GetUnifiedCache()
}
var scheduleStartupMemberWarmupFn = func(ms *monitoring.MonitoringService, config cache.WarmupConfig) bool {
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
		slog.Info("Architectural state bypass: Suppressing cache warmup sequence due to valid temporal TTL",
			slog.String("botInstanceID", runtime.instanceID),
		)
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
				slog.Warn("Mitigated service degradation: Cache warmup base phase failed, executing compensatory bypass",
					slog.String("botInstanceID", runtime.instanceID),
					slog.String("error", err.Error()),
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
				slog.Warn("Mitigated service degradation: Cache warmup member phase failed, executing compensatory bypass",
					slog.String("botInstanceID", runtime.instanceID),
					slog.String("error", err.Error()),
				)
			}
		}()
		return
	}

	slog.Debug("Delegating cache warmup base phase to orchestrator scheduling queue",
		slog.String("botInstanceID", runtime.instanceID),
	)
	startupTasks.GoHeavy("cache_warmup_base:"+runtime.instanceID, func(taskCtx context.Context) error {
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
			slog.Warn("Mitigated service degradation: Orchestrated cache warmup base phase failed, pipeline resumes",
				slog.String("botInstanceID", runtime.instanceID),
				slog.String("error", err.Error()),
			)
			return nil
		}

		if scheduleStartupMemberWarmupFn(runtime.monitoringService, memberWarmupConfig) {
			slog.Debug("Prioritized member phase execution behind startup tasks lock",
				slog.String("botInstanceID", runtime.instanceID),
			)
			return nil
		}

		if err := runWarmup(localCtx, memberWarmupConfig); err != nil {
			if localCtx.Err() != nil {
				return nil
			}
			slog.Warn("Mitigated service degradation: Orchestrated cache warmup member phase failed, pipeline resumes",
				slog.String("botInstanceID", runtime.instanceID),
				slog.String("error", err.Error()),
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

	slog.Info("Architectural state transition: Executing planned shutdown across main runtime instances",
		slog.String("botInstanceID", runtime.instanceID),
	)

	var errs []error
	if runtime.serviceManager != nil {
		if err := runtime.serviceManager.StopAll(ctx); err != nil {
			errWrap := fmt.Errorf("stop services for %s: %w", runtime.instanceID, err)
			log.EmitBlockingError("Blocking structural failure during scheduled teardown sequence", errWrap, log.GenerateRequestID())
			errs = append(errs, errWrap)
		}
	}
	return errs
}
