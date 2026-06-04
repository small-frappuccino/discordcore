package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	svc "github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

const (
	monitoringGuildMembersPageSize   = 1000
	monitoringMaxConcurrentGuildScan = 4
	taskTypeStartupWarmupMembers     = "monitor.startup_warmup_members"
)

var monitoringWarmupTaskFn = cache.IntelligentWarmupContext

func stopMonitoringSubService(ctx context.Context, operation, serviceName string, stopFn func() error) error {
	if stopFn == nil {
		return nil
	}
	if err := monitoringRunErrWithTimeout(ctx, monitoringDependencyTimeout, stopFn); err != nil {
		log.ErrorLoggerRaw().Error(
			"Monitoring sub-service stop failed",
			"operation", operation,
			"service", serviceName,
			"err", err,
		)
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func startMonitoringSubService(ctx context.Context, operation, serviceName string, startFn func() error) error {
	if startFn == nil {
		return nil
	}
	if err := monitoringRunErrWithTimeout(ctx, monitoringDependencyTimeout, startFn); err != nil {
		return fmt.Errorf("%s (%s): %w", operation, serviceName, err)
	}
	return nil
}

const (
	heartbeatInterval = 5 * time.Minute
	downtimeThreshold = 30 * time.Minute

	monitoringDependencyTimeout    = 15 * time.Second
	monitoringPersistenceTimeout   = 10 * time.Second
	monitoringRouterCloseTimeout   = 10 * time.Second
	monitoringStartupDispatchLimit = 5 * time.Second
	monitoringRoleAuditCacheTTL    = 2 * time.Second
	monitoringRoleAuditDebounceTTL = 1 * time.Second
	monitoringRoleAuditRetryDelay  = 300 * time.Millisecond
	monitoringRoleAuditEntryMaxAge = 2 * time.Minute
)

var heartbeatTickInterval = heartbeatInterval

// MonitoringService coordinates multi-guild handlers and delegates specific tasks (e.g., user).
type MonitoringService struct {
	session              *discordgo.Session
	configManager        *files.ConfigManager
	botInstanceID        string
	defaultBotInstanceID string
	store                *storage.Store
	activity             *runtimeActivity
	notifier             *NotificationSender
	adapters             *task.NotificationAdapters
	router               *task.TaskRouter
	routerConfig         task.RouterConfig
	userWatcher          *UserWatcher
	memberEventService   *MemberEventService   // Service for member events
	messageEventService  *MessageEventService  // Service for message events
	reactionEventService *ReactionEventService // Service for reaction event handling
	runMu                sync.RWMutex          // serializes Start/Stop and guards run
	run                  monitoringRunState
	changeDebounce       changeDebouncer
	logger               *slog.Logger

	// Unified cache for Discord API data (members, guilds, roles, channels)
	unifiedCache *cache.UnifiedCache

	// Sub-services for domain separation
	rolesCacheService *RolesCacheService
	statsService      *StatsService

	// Event handler references for cleanup

	eventHandlers []func()
	presence      presenceWatcher

	// Observability sink. When nil, observability() returns NopMetrics
	// so call-sites can issue Record* without nil checks. This mirrors
	// the QOTD/moderation pattern: write-only on the hot path, read via
	// type-asserting SnapshotProvider on the /v1/health/monitoring route.
	metrics Metrics
}

// Metrics exposes the observability sink for read-only access by the
// control server (/v1/health/monitoring uses a type assertion to find the
// SnapshotProvider implementation). Returns NopMetrics when the service
// was constructed without a metrics value.
func (ms *MonitoringService) Metrics() Metrics {
	return ms.observability()
}

// SetMetrics swaps the observability sink. Useful in tests; production
// startup wires metrics via NewMonitoringServiceForBotWithMetrics. nil is
// treated as NopMetrics via observability().
func (ms *MonitoringService) SetMetrics(metrics Metrics) {
	if ms == nil {
		return
	}
	ms.metrics = metrics
}

// observability is the nil-safe accessor every internal Record* call site
// uses. Hot path is one nil compare; the only branch operators take is
// "metrics wired" vs. "default NopMetrics" — write-only on this side.
func (ms *MonitoringService) observability() Metrics {
	if ms == nil || ms.metrics == nil {
		return NopMetrics{}
	}
	return ms.metrics
}

func (ms *MonitoringService) Name() string {
	return "monitoring"
}

func (ms *MonitoringService) Type() svc.ServiceType {
	return svc.TypeMonitoring
}

func (ms *MonitoringService) Priority() svc.ServicePriority {
	return svc.PriorityHigh
}

func (ms *MonitoringService) Dependencies() []string {
	return nil
}

func (ms *MonitoringService) IsRunning() bool {
	ms.runMu.RLock()
	defer ms.runMu.RUnlock()
	return ms.run.running
}

// currentRunCtx returns a snapshot of ms.run.ctx taken under runMu. It returns
// nil after Stop has cleared the lifecycle, so hot-path callers can skip work
// that must not outlive the running monitoring service.
func (ms *MonitoringService) currentRunCtx() context.Context {
	if ms == nil {
		return nil
	}
	ms.runMu.RLock()
	defer ms.runMu.RUnlock()
	return ms.run.ctx
}

func (ms *MonitoringService) HealthCheck(ctx context.Context) svc.HealthStatus {
	ms.runMu.RLock()
	isRunning := ms.run.running
	runCtx := ms.run.ctx
	ms.runMu.RUnlock()

	message := "Monitoring service is stopped"
	if isRunning {
		message = "Monitoring service is running"
	}
	if runCtx != nil && runCtx.Err() != nil {
		message = fmt.Sprintf("Monitoring service lifecycle canceled: %v", runCtx.Err())
	}

	return svc.HealthStatus{
		Healthy:   isRunning && runCtx != nil && runCtx.Err() == nil,
		Message:   message,
		LastCheck: time.Now(),
		Details: map[string]string{
			"router_ready": strconv.FormatBool(ms.TaskRouter() != nil),
		},
	}
}

func (ms *MonitoringService) Stats() svc.ServiceStats {
	ms.runMu.RLock()
	startTime := ms.run.startTime
	stopTime := ms.run.stopTime
	restartCount := ms.run.restartCount
	errorCount := ms.run.errorCount
	lastErrorAt := ms.run.lastErrorAt
	ms.runMu.RUnlock()

	stats := svc.ServiceStats{
		RestartCount: restartCount,
		ErrorCount:   errorCount,
		Metrics:      ms.metricsRows(),
	}
	if startTime != nil {
		stats.StartTime = *startTime
		if stopTime != nil {
			stats.Uptime = stopTime.Sub(*startTime)
		} else {
			stats.Uptime = time.Since(*startTime)
		}
	}
	if lastErrorAt != nil {
		errAt := *lastErrorAt
		stats.LastError = &errAt
	}
	return stats
}

func (ms *MonitoringService) startLifecycle(ctx context.Context) (context.Context, context.CancelFunc) {
	base := context.Background()
	if ctx != nil {
		base = context.WithoutCancel(ctx)
	}
	return context.WithCancel(base)
}

func (ms *MonitoringService) startOwnedWorker(ctx context.Context, fn func(context.Context)) {
	ms.run.wg.Add(1)
	go func() {
		defer ms.run.wg.Done()
		fn(ctx)
	}()
}

func (ms *MonitoringService) waitForOwnedWorkers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ms.run.wg.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (ms *MonitoringService) recordLifecycleErrorLocked() {
	now := time.Now()
	ms.run.errorCount++
	ms.run.lastErrorAt = &now
}

func monitoringRunWithTimeout[T any](ctx context.Context, timeout time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	type result struct {
		value T
		err   error
	}
	resultCh := make(chan result, 1)
	go func() {
		value, err := fn()
		resultCh <- result{value: value, err: err}
	}()

	select {
	case res := <-resultCh:
		return res.value, res.err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func monitoringRunErrWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	_, err := monitoringRunWithTimeout(ctx, timeout, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func monitoringRunWithTimeoutContext[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if fn == nil {
		return zero, nil
	}
	return fn(ctx)
}

func monitoringRunErrWithTimeoutContext(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	_, err := monitoringRunWithTimeoutContext(ctx, timeout, func(runCtx context.Context) (struct{}, error) {
		if fn == nil {
			return struct{}{}, nil
		}
		return struct{}{}, fn(runCtx)
	})
	return err
}

// NewMonitoringService creates the multi-guild monitoring service. Returns error if any dependency is nil.
func NewMonitoringService(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store, logger *slog.Logger) (*MonitoringService, error) {
	return NewMonitoringServiceForBot(session, configManager, store, "", "", logger)
}

// NewMonitoringServiceForBot creates a monitoring service scoped to the
// guilds assigned to a specific bot instance. The service is constructed
// with NopMetrics; callers that want /v1/health/monitoring telemetry
// should use NewMonitoringServiceForBotWithMetrics instead, or invoke
// SetMetrics on the returned service before Start.
func NewMonitoringServiceForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	store *storage.Store,
	botInstanceID string,
	defaultBotInstanceID string,
	logger *slog.Logger,
) (*MonitoringService, error) {
	return NewMonitoringServiceForBotWithMetrics(session, configManager, store, botInstanceID, defaultBotInstanceID, nil, logger)
}

// NewMonitoringServiceForBotWithMetrics is the constructor production startup
// uses to attach the in-memory Metrics implementation. Passing nil falls
// back to NopMetrics, matching NewMonitoringServiceForBot. Mirrors
// qotd.NewServiceWithMetrics — callers wire one metrics value, expose it
// via MonitoringService.Metrics() to the control server, and forget about
// it; every Record* call routes through ms.observability() which falls
// back to NopMetrics when nil.
func NewMonitoringServiceForBotWithMetrics(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	store *storage.Store,
	botInstanceID string,
	defaultBotInstanceID string,
	metrics Metrics,
	logger *slog.Logger,
) (*MonitoringService, error) {
	if session == nil {
		return nil, fmt.Errorf("discord session is nil")
	}
	if configManager == nil {
		return nil, fmt.Errorf("config manager is nil")
	}
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	n := NewNotificationSender(session, logger)

	// Create unified cache with persistence enabled
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Store = store
	cacheConfig.PersistEnabled = true
	unifiedCache := cache.NewUnifiedCache(cacheConfig)

	ms := &MonitoringService{
		session:              session,
		configManager:        configManager,
		botInstanceID:        files.NormalizeBotInstanceID(botInstanceID),
		defaultBotInstanceID: files.NormalizeBotInstanceID(defaultBotInstanceID),
		store:                store,
		activity:             newMonitoringRuntimeActivity(store, files.NormalizeBotInstanceID(botInstanceID)),
		notifier:             n,
		unifiedCache:         unifiedCache,
		userWatcher:          NewUserWatcher(session, configManager, store, n, unifiedCache),
		memberEventService:   NewMemberEventServiceForBot(eventServiceDeps{Session: session, ConfigManager: configManager, Notifier: n, Store: store, BotInstanceID: botInstanceID, DefaultBotInstanceID: defaultBotInstanceID, Logger: logger}),
		messageEventService:  NewMessageEventServiceForBot(eventServiceDeps{Session: session, ConfigManager: configManager, Notifier: n, Store: store, BotInstanceID: botInstanceID, DefaultBotInstanceID: defaultBotInstanceID, Logger: logger}),
		run:                  monitoringRunState{stopChan: make(chan struct{})},
		rolesCacheService:    NewRolesCacheService(configManager),
		eventHandlers:        make([]func(), 0),
		statsService:         NewStatsService(session, configManager, store, logger, botInstanceID, defaultBotInstanceID, nil, nil, nil),
		metrics:              metrics,
		logger:               logger,
	}
	ms.statsService.currentRunCtx = ms.currentRunCtx
	ms.statsService.getHeartbeat = ms.getHeartbeat
	ms.statsService.fetchMembers = ms.forEachGuildMemberPageContext
	ms.rebuildTaskPipeline()
	return ms, nil
}

func (ms *MonitoringService) rebuildTaskPipeline() {
	if ms.router != nil {
		ms.router.Close()
	}

	router := task.NewRouter(ms.routerConfig)
	adapters := task.NewNotificationAdapters(router, ms.session, ms.configManager, ms.store, ms.notifier)
	if ms.userWatcher != nil {
		adapters.SetAvatarProcessor(ms.userWatcher)
	}

	ms.router = router
	ms.adapters = adapters

	if ms.memberEventService != nil {
		ms.memberEventService.SetAdapters(adapters)
	}
	if ms.messageEventService != nil {
		ms.messageEventService.SetAdapters(adapters)
		ms.messageEventService.SetTaskRouter(router)
	}
}

func (ms *MonitoringService) SetTaskRouterConfig(cfg task.RouterConfig) {
	if ms == nil {
		return
	}
	ms.routerConfig = cfg
	ms.rebuildTaskPipeline()
}

// Start starts the monitoring service. Returns error if already running.
func (ms *MonitoringService) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	if ms.run.running {
		log.ErrorLoggerRaw().Error("Monitoring service is already running")
		return fmt.Errorf("monitoring service is already running")
	}

	lifecycleCtx, cancelLifecycle := ms.startLifecycle(ctx)
	ms.run.stopChan = make(chan struct{})
	ms.run.stopOnce = sync.Once{}
	if ms.router == nil {
		ms.rebuildTaskPipeline()
	}

	if err := ms.handleStartupDowntimeAndMaybeRefresh(ctx); err != nil {
		cancelLifecycle()
		ms.recordLifecycleErrorLocked()
		return fmt.Errorf("handle startup downtime: %w", err)
	}
	ms.setupEventHandlers()

	// Global check for services
	globalRC := files.RuntimeConfig{}
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		globalRC = scopedCfg.RuntimeConfig
	}
	globalFeatures := (&files.BotConfig{}).ResolveFeatures("")
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		globalFeatures = scopedCfg.ResolveFeatures("")
	}
	workload := ms.workloadState(globalRC)

	// Start member/message services (member events are needed for entry/exit logs and auto-role assignment).
	// Note: these services are currently global, so we use global config for startup.
	// Per-guild toggles would need these services to be guild-aware or filtered.
	if !workload.memberEventService {
		log.ApplicationLogger().Info("🛑 Entry/exit logs and auto-role assignment are disabled; MemberEventService will not start")
	} else {
		if err := startMonitoringSubService(ctx, "monitoring.start.member", "member_event_service", func() error {
			return ms.memberEventService.Start(ctx)
		}); err != nil {
			cancelLifecycle()
			ms.removeEventHandlers()
			ms.recordLifecycleErrorLocked()
			return fmt.Errorf("failed to start member event service: %w", err)
		}
	}
	// Optionally honor DisableAutomodLogs here (Automod service is started elsewhere)
	if globalRC.DisableAutomodLogs || !globalFeatures.Logging.AutomodAction {
		log.ApplicationLogger().Info("🛑 Automod logs disabled by runtime config/features")
	}

	// Gate message logging behind runtime config
	if !workload.messageEventService {
		log.ApplicationLogger().Info("🛑 Message logging disabled by runtime config/features; MessageEventService will not start")
	} else {
		if err := startMonitoringSubService(ctx, "monitoring.start.message", "message_event_service", func() error {
			return ms.messageEventService.Start(ctx)
		}); err != nil {
			startErrs := []error{err}
			// Stop the member event service if start failed
			if ms.memberEventService != nil && ms.memberEventService.IsRunning() {
				if stopErr := stopMonitoringSubService(
					ctx,
					"monitoring.start.cleanup.stop_member_after_message_start_failure",
					"member_event_service",
					func() error { return ms.memberEventService.Stop(ctx) },
				); stopErr != nil {
					startErrs = append(startErrs, stopErr)
				}
			}
			cancelLifecycle()
			ms.removeEventHandlers()
			ms.recordLifecycleErrorLocked()
			if len(startErrs) > 1 {
				return fmt.Errorf("failed to start message event service: %w", errors.Join(startErrs...))
			}
			return fmt.Errorf("failed to start message event service: %w", err)
		}
	}

	// Gate reaction event handling behind runtime config and guild needs.
	if !workload.reactionEventService {
		log.ApplicationLogger().Info("🛑 Reaction event handling disabled by runtime config/features; ReactionEventService will not start")
	} else {
		// Lazily initialize service if not yet created
		if ms.reactionEventService == nil {
			ms.reactionEventService = NewReactionEventServiceForBot(ms.session, ms.configManager, ms.store, ms.botInstanceID, ms.defaultBotInstanceID, ms.logger)
		}
		if err := startMonitoringSubService(ctx, "monitoring.start.reaction", "reaction_event_service", func() error {
			return ms.reactionEventService.Start(ctx)
		}); err != nil {
			startErrs := []error{err}
			// Stop previously started services on failure
			if ms.messageEventService != nil && ms.messageEventService.IsRunning() {
				if stopErr := stopMonitoringSubService(
					ctx,
					"monitoring.start.cleanup.stop_message_after_reaction_start_failure",
					"message_event_service",
					func() error { return ms.messageEventService.Stop(ctx) },
				); stopErr != nil {
					startErrs = append(startErrs, stopErr)
				}
			}
			if ms.memberEventService != nil && ms.memberEventService.IsRunning() {
				if stopErr := stopMonitoringSubService(
					ctx,
					"monitoring.start.cleanup.stop_member_after_reaction_start_failure",
					"member_event_service",
					func() error { return ms.memberEventService.Stop(ctx) },
				); stopErr != nil {
					startErrs = append(startErrs, stopErr)
				}
			}
			cancelLifecycle()
			ms.removeEventHandlers()
			ms.recordLifecycleErrorLocked()
			if len(startErrs) > 1 {
				return fmt.Errorf("failed to start reaction event service: %w", errors.Join(startErrs...))
			}
			return fmt.Errorf("failed to start reaction event service: %w", err)
		}
	}

	ms.startHeartbeat(lifecycleCtx)
	if err := startMonitoringSubService(lifecycleCtx, "monitoring.start.roles_cache", "roles_cache_service", func() error {
		return ms.rolesCacheService.Start(lifecycleCtx)
	}); err != nil {
		cancelLifecycle()
		ms.removeEventHandlers()
		ms.recordLifecycleErrorLocked()
		return fmt.Errorf("failed to start roles cache service: %w", err)
	}

	if err := startMonitoringSubService(lifecycleCtx, "monitoring.start.stats", "stats_service", func() error {
		return ms.statsService.Start(lifecycleCtx)
	}); err != nil {
		cancelLifecycle()
		ms.removeEventHandlers()
		ms.recordLifecycleErrorLocked()
		return fmt.Errorf("failed to start stats service: %w", err)
	}

	serviceCtx := lifecycleCtx

	ms.registerStartupWarmupHandler(serviceCtx)
	ms.syncSchedulesLocked(serviceCtx, workload)

	ms.registerBackfillHandlers(serviceCtx, workload)

	now := time.Now()
	if ms.run.startTime != nil {
		ms.run.restartCount++
	}
	ms.run.ctx = serviceCtx
	ms.run.cancel = cancelLifecycle
	ms.run.running = true
	ms.run.startTime = &now
	ms.run.stopTime = nil

	if ms.unifiedCache != nil {
		ms.run.persistStop = ms.unifiedCache.SetPersistInterval(time.Hour)
	}

	ms.scheduleEnsureGuildsListed(serviceCtx)
	log.ApplicationLogger().Info("All monitoring services started successfully")
	return nil
}

func shouldRunMemberEventService(cfg *files.BotConfig, globalRC files.RuntimeConfig) bool {
	if cfg == nil {
		return false
	}

	// Global/default behavior still matters for guilds that only inherit config.
	globalFeatures := cfg.ResolveFeatures("")
	if globalFeatures.Services.Monitoring && !globalRC.DisableEntryExitLogs && (globalFeatures.Logging.MemberJoin || globalFeatures.Logging.MemberLeave) {
		return true
	}

	for _, guildCfg := range cfg.Guilds {
		features := cfg.ResolveFeatures(guildCfg.GuildID)
		if !features.Services.Monitoring {
			continue
		}
		guildDisableEntryExit := globalRC.DisableEntryExitLogs || guildCfg.RuntimeConfig.DisableEntryExitLogs
		if !guildDisableEntryExit && (features.Logging.MemberJoin || features.Logging.MemberLeave) {
			return true
		}
		if features.AutoRoleAssign && guildCfg.Roles.AutoAssignment.Enabled {
			return true
		}
	}

	return false
}

func (ms *MonitoringService) scopedConfig() *files.BotConfig {
	if ms == nil || ms.configManager == nil {
		return nil
	}
	cfg := ms.configManager.Config()
	if cfg == nil {
		return nil
	}
	scopedGuilds := cfg.GuildsForBotInstance(ms.botInstanceID, ms.defaultBotInstanceID)
	if len(scopedGuilds) == len(cfg.Guilds) {
		return cfg
	}
	scoped := *cfg
	scoped.Guilds = scopedGuilds
	return &scoped
}

func (ms *MonitoringService) handlesGuild(guildID string) bool {
	if ms == nil {
		return false
	}
	if files.NormalizeBotInstanceID(ms.botInstanceID) == "" && files.NormalizeBotInstanceID(ms.defaultBotInstanceID) == "" {
		return true
	}
	if ms.configManager == nil {
		return false
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}
	guild := ms.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}
	return guild.EffectiveBotInstanceID(ms.defaultBotInstanceID) == files.NormalizeBotInstanceID(ms.botInstanceID)
}

func (ms *MonitoringService) getLastEvent(ctx context.Context) (time.Time, bool, error) {
	if ms == nil || ms.store == nil {
		return time.Time{}, false, fmt.Errorf("store unavailable")
	}
	if ts, ok, err := ms.store.LastEventForBot(ctx, ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" || ms.botInstanceID != ms.defaultBotInstanceID {
		return ts, ok, err
	}
	return ms.store.LastEvent(ctx)
}

func (ms *MonitoringService) getHeartbeat(ctx context.Context) (time.Time, bool, error) {
	if ms == nil || ms.store == nil {
		return time.Time{}, false, fmt.Errorf("store unavailable")
	}
	if ts, ok, err := ms.store.HeartbeatForBot(ctx, ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" || ms.botInstanceID != ms.defaultBotInstanceID {
		return ts, ok, err
	}
	return ms.store.Heartbeat(ctx)
}

// Stop stops the monitoring service. Returns error if not running.
func (ms *MonitoringService) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ms.runMu.Lock()
	if !ms.run.running {
		ms.runMu.Unlock()
		log.ErrorLoggerRaw().Error("Monitoring service is not running")
		return fmt.Errorf("monitoring service is not running")
	}

	cancelLifecycle := ms.run.cancel
	ms.run.cancel = nil
	ms.run.ctx = nil
	ms.run.running = false
	ms.run.stopOnce.Do(func() {
		close(ms.run.stopChan)
	})
	cronCancel := ms.run.cronCancel
	ms.run.cronCancel = nil
	statsCronCancel := ms.run.statsCronCancel
	ms.run.statsCronCancel = nil
	rolesRefreshCronCancel := ms.run.rolesRefreshCronCancel
	ms.run.rolesRefreshCronCancel = nil
	persistStop := ms.run.persistStop
	ms.run.persistStop = nil
	router := ms.router
	ms.router = nil
	ms.adapters = nil
	ms.runMu.Unlock()

	var stopErrs []error

	// Drain the task router before canceling the lifecycle context. In-flight
	// handlers depend on lifecycle-scoped collaborators — notably the stats
	// actor goroutine, which serves request/reply round-trips. Canceling the
	// lifecycle first would stop that actor while a handler is mid-round-trip,
	// stranding it until the close timeout; draining first lets handlers finish
	// cleanly. Close also cancels the router lifecycle context so handlers that
	// honor it abort promptly.
	if router != nil {
		if err := monitoringRunErrWithTimeout(ctx, monitoringRouterCloseTimeout, func() error {
			router.Close()
			return nil
		}); err != nil {
			stopErrs = append(stopErrs, fmt.Errorf("close task router: %w", err))
		}
	}

	if cancelLifecycle != nil {
		cancelLifecycle()
	}
	if err := ms.stopHeartbeat(ctx); err != nil {
		stopErrs = append(stopErrs, fmt.Errorf("stop heartbeat: %w", err))
	}
	if cronCancel != nil {
		cronCancel()
	}
	if statsCronCancel != nil {
		statsCronCancel()
	}
	if rolesRefreshCronCancel != nil {
		rolesRefreshCronCancel()
	}
	if persistStop != nil {
		close(persistStop)
	}

	ms.removeEventHandlers()
	if ms.memberEventService != nil && ms.memberEventService.IsRunning() {
		if err := stopMonitoringSubService(ctx, "monitoring.stop.member", "member_event_service", func() error {
			return ms.memberEventService.Stop(ctx)
		}); err != nil {
			stopErrs = append(stopErrs, err)
		}
	}
	if ms.messageEventService != nil && ms.messageEventService.IsRunning() {
		if err := stopMonitoringSubService(ctx, "monitoring.stop.message", "message_event_service", func() error {
			return ms.messageEventService.Stop(ctx)
		}); err != nil {
			stopErrs = append(stopErrs, err)
		}
	}
	if ms.reactionEventService != nil && ms.reactionEventService.IsRunning() {
		if err := stopMonitoringSubService(ctx, "monitoring.stop.reaction", "reaction_event_service", func() error {
			return ms.reactionEventService.Stop(ctx)
		}); err != nil {
			stopErrs = append(stopErrs, err)
		}
	}

	if err := ms.waitForOwnedWorkers(ctx); err != nil {
		stopErrs = append(stopErrs, fmt.Errorf("wait for monitoring workers: %w", err))
	}

	if ms.unifiedCache != nil {
		log.ApplicationLogger().Info("💾 Persisting cache to storage...")
		if err := monitoringRunErrWithTimeout(ctx, monitoringPersistenceTimeout, ms.unifiedCache.Persist); err != nil {
			log.ErrorLoggerRaw().Error("Failed to persist cache", "err", err)
			stopErrs = append(stopErrs, fmt.Errorf("persist unified cache: %w", err))
		} else {
			members := ms.unifiedCache.MemberCount()
			guilds := ms.unifiedCache.GuildCount()
			roles := ms.unifiedCache.RolesCount()
			channels := ms.unifiedCache.ChannelCount()
			total := members + guilds + roles + channels
			log.ApplicationLogger().Info("✅ Cache persisted", "entries_saved", total)
		}
		ms.unifiedCache.Stop()
	}

	if ms.messageEventService != nil {
		ms.messageEventService.SetTaskRouter(nil)
		ms.messageEventService.SetAdapters(nil)
	}
	if ms.memberEventService != nil {
		ms.memberEventService.SetAdapters(nil)
	}

	ms.runMu.Lock()
	now := time.Now()
	ms.run.stopTime = &now
	if len(stopErrs) > 0 {
		ms.recordLifecycleErrorLocked()
		ms.runMu.Unlock()
		return errors.Join(stopErrs...)
	}
	ms.runMu.Unlock()

	log.ApplicationLogger().Info("Monitoring service stopped")
	return nil
}

// initializeGuildCache initializes the current avatars of members in a specific guild.
func (ms *MonitoringService) initializeGuildCache(guildID string) {
	_ = ms.initializeGuildCacheContext(context.Background(), guildID)
}

func (ms *MonitoringService) initializeGuildCacheContext(ctx context.Context, guildID string) error {
	if ms.store == nil {
		log.ApplicationLogger().Warn("Store is nil; skipping cache initialization for guild", "guildID", guildID)
		return nil
	}

	// Use unified cache for guild fetch
	guild, err := ms.getGuildContext(ctx, guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Error getting guild", "guildID", guildID, "err", err)
		return fmt.Errorf("MonitoringService.initializeGuildCacheContext: %w", err)
	}
	log.ApplicationLogger().Info("Initializing cache for guild", "guildName", guild.Name, "guildID", guild.ID)
	if err := ms.store.SetGuildOwnerID(guildID, guild.OwnerID); err != nil {
		log.ApplicationLogger().Warn("Failed to persist guild owner ID during cache initialization", "guildID", guildID, "ownerID", guild.OwnerID, "err", err)
	}

	// Set bot join time if missing
	_, hasBotSince, err := ms.store.BotSince(ctx, guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to read bot join timestamp during cache initialization",
			"operation", "monitoring.initialize_guild_cache.get_bot_since",
			"guildID", guildID,
			"err", err,
		)
	} else if !hasBotSince {
		botID := ms.session.State.User.ID
		var botMember *discordgo.Member
		// Prefer state cache to avoid a REST call
		if ms.session != nil && ms.session.State != nil {
			if m, _ := ms.session.State.Member(guildID, botID); m != nil {
				botMember = m
			}
		}
		// Fallback to REST only if necessary
		if botMember == nil {
			if m, err := ms.getGuildMemberContext(ctx, guildID, botID); err == nil {
				botMember = m
			}
		}
		if botMember != nil && !botMember.JoinedAt.IsZero() {
			if err := ms.store.SetBotSince(ctx, guildID, botMember.JoinedAt); err != nil {
				log.ApplicationLogger().Warn("Failed to persist bot join timestamp", "guildID", guildID, "joinedAt", botMember.JoinedAt, "err", err)
			}
		} else {
			now := time.Now()
			if err := ms.store.SetBotSince(ctx, guildID, now); err != nil {
				log.ApplicationLogger().Warn("Failed to persist fallback bot join timestamp", "guildID", guildID, "joinedAt", now, "err", err)
			}
		}
	}
	totalMembers, err := ms.forEachGuildMemberPageContext(ctx, guildID, func(members []*discordgo.Member) error {
		snapshotAt := time.Now().UTC()
		snapshots := make([]storage.GuildMemberSnapshot, 0, len(members))
		for _, member := range members {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("MonitoringService.initializeGuildCacheContext: %w", err)
			}
			if member == nil || member.User == nil {
				continue
			}
			avatarHash := member.User.Avatar
			if avatarHash == "" {
				avatarHash = "default"
			}
			snapshots = append(snapshots, storage.GuildMemberSnapshot{
				UserID:     member.User.ID,
				AvatarHash: avatarHash,
				HasAvatar:  true,
				Roles:      member.Roles,
				HasRoles:   true,
				JoinedAt:   member.JoinedAt,
				IsBot:      member.User.Bot,
				HasBot:     true,
			})
		}
		if len(snapshots) == 0 {
			return nil
		}
		if err := ms.store.UpsertGuildMemberSnapshotsContext(ctx, guildID, snapshots, snapshotAt); err != nil {
			log.ApplicationLogger().Warn(
				"Failed to persist guild member snapshot page",
				"operation", "monitoring.initialize_guild_cache.persist_page",
				"guildID", guildID,
				"members", len(snapshots),
				"err", err,
			)
			return nil
		}
		for _, snapshot := range snapshots {
			ms.rolesCacheService.CacheRolesSet(guildID, snapshot.UserID, snapshot.Roles)
		}
		return nil
	})
	if err != nil {
		log.ErrorLoggerRaw().Error("Error getting members for guild", "guildID", guildID, "err", err)
		return fmt.Errorf("MonitoringService.initializeGuildCacheContext: %w", err)
	}
	log.ApplicationLogger().Info("Guild cache initialization member scan completed", "guildID", guildID, "members", totalMembers)
	return nil
}

// ApplyRuntimeToggles hot-applies a subset of runtime_config toggles without restarting the process.
//
// Scope:
//   - ALICE_DISABLE_ENTRY_EXIT_LOGS: start/stop MemberEventService
//   - ALICE_DISABLE_MESSAGE_LOGS: start/stop MessageEventService
//   - ALICE_DISABLE_REACTION_LOGS: enable/disable reaction metrics; the service
//     still stays up when guild reaction blocks require reaction handling
//   - ALICE_DISABLE_USER_LOGS: re-register user-related handlers (presence/member/user updates)
//   - ALICE_DISABLE_BOT_ROLE_PERM_MIRROR / ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID: no-op here (checked at event time)
//
// Notes:
// - Backfill settings are intentionally not handled here.
// - This is safe to call even if MonitoringService is not running; it will no-op.
func (ms *MonitoringService) ApplyRuntimeToggles(ctx context.Context, rc files.RuntimeConfig) error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()

	if !ms.run.running {
		return nil
	}

	workload := ms.workloadState(rc)
	var stopErrs []error

	// Entry/Exit logs and auto-role assignment -> MemberEventService
	if !workload.memberEventService {
		if ms.memberEventService != nil && ms.memberEventService.IsRunning() {
			if err := stopMonitoringSubService(
				ctx,
				"monitoring.apply_runtime_toggles.stop_member",
				"member_event_service",
				func() error { return ms.memberEventService.Stop(ctx) },
			); err != nil {
				stopErrs = append(stopErrs, err)
			}
		}
	} else {
		if ms.memberEventService != nil && !ms.memberEventService.IsRunning() {
			if err := startMonitoringSubService(ctx, "monitoring.apply_runtime_toggles.start_member", "member_event_service", func() error {
				return ms.memberEventService.Start(ctx)
			}); err != nil {
				return fmt.Errorf("start MemberEventService: %w", err)
			}
		}
	}

	// Message logs -> MessageEventService
	if !workload.messageEventService {
		if ms.messageEventService != nil && ms.messageEventService.IsRunning() {
			if err := stopMonitoringSubService(
				ctx,
				"monitoring.apply_runtime_toggles.stop_message",
				"message_event_service",
				func() error { return ms.messageEventService.Stop(ctx) },
			); err != nil {
				stopErrs = append(stopErrs, err)
			}
		}
	} else {
		if ms.messageEventService != nil && !ms.messageEventService.IsRunning() {
			if err := startMonitoringSubService(ctx, "monitoring.apply_runtime_toggles.start_message", "message_event_service", func() error {
				return ms.messageEventService.Start(ctx)
			}); err != nil {
				return fmt.Errorf("start MessageEventService: %w", err)
			}
		}
	}

	// Reaction event handling -> ReactionEventService
	if !workload.reactionEventService {
		if ms.reactionEventService != nil && ms.reactionEventService.IsRunning() {
			if err := stopMonitoringSubService(
				ctx,
				"monitoring.apply_runtime_toggles.stop_reaction",
				"reaction_event_service",
				func() error { return ms.reactionEventService.Stop(ctx) },
			); err != nil {
				stopErrs = append(stopErrs, err)
			}
		}
	} else {
		if ms.reactionEventService == nil {
			ms.reactionEventService = NewReactionEventServiceForBot(ms.session, ms.configManager, ms.store, ms.botInstanceID, ms.defaultBotInstanceID, ms.logger)
		}
		if !ms.reactionEventService.IsRunning() {
			if err := startMonitoringSubService(ctx, "monitoring.apply_runtime_toggles.start_reaction", "reaction_event_service", func() error {
				return ms.reactionEventService.Start(ctx)
			}); err != nil {
				return fmt.Errorf("start ReactionEventService: %w", err)
			}
		}
	}

	// User logs -> re-register handlers (presence/member/user updates)
	ms.removeEventHandlers()
	ms.setupEventHandlersFromRuntimeConfig(rc)
	ms.syncSchedulesLocked(ms.run.ctx, workload)

	if len(stopErrs) > 0 {
		return fmt.Errorf("apply runtime toggles: %w", errors.Join(stopErrs...))
	}
	return nil
}

func (ms *MonitoringService) registerStartupWarmupHandler(runCtx context.Context) {
	if ms == nil || ms.router == nil || runCtx == nil {
		return
	}

	ms.router.RegisterHandler(taskTypeStartupWarmupMembers, func(ctx context.Context, payload any) error {
		if err := runCtx.Err(); err != nil {
			return fmt.Errorf("MonitoringService.registerStartupWarmupHandler: %w", err)
		}

		config, ok := payload.(cache.WarmupConfig)
		if !ok {
			config = cache.DefaultWarmupConfig()
			config.FetchMissingGuilds = false
			config.FetchMissingRoles = false
			config.FetchMissingChannels = false
			config.MaxMembersPerGuild = 500
		}
		return monitoringWarmupTaskFn(runCtx, ms.session, ms.unifiedCache, ms.store, config)
	})
}

func (ms *MonitoringService) scheduleEnsureGuildsListed(runCtx context.Context) {
	if ms == nil || runCtx == nil {
		return
	}

	ms.startOwnedWorker(runCtx, func(ctx context.Context) {
		if err := monitoringRunErrWithTimeout(ctx, monitoringPersistenceTimeout, func() error {
			ms.ensureGuildsListed()
			return nil
		}); err != nil && ctx.Err() == nil {
			log.ApplicationLogger().Warn("Ensure guilds listed task failed", "err", err)
		}
	})
}

func (ms *MonitoringService) dispatchMonitorTaskLocked(runCtx context.Context, taskType string) {
	ms.dispatchMonitorTaskWithPayloadLocked(runCtx, task.Task{Type: taskType, Payload: task.EmptyPayload{}})
}

func (ms *MonitoringService) dispatchMonitorTaskWithPayloadLocked(runCtx context.Context, dispatchTask task.Task) bool {
	if ms.router == nil || runCtx == nil || strings.TrimSpace(dispatchTask.Type) == "" {
		return false
	}

	router := ms.router
	ms.startOwnedWorker(runCtx, func(workerCtx context.Context) {
		dispatchCtx, cancel := context.WithTimeout(workerCtx, monitoringStartupDispatchLimit)
		defer cancel()
		if err := router.Dispatch(dispatchCtx, dispatchTask); err != nil {
			log.ErrorLoggerRaw().Error("Failed to dispatch startup monitor task", "taskType", dispatchTask.Type, "err", err)
		}
	})
	return true
}

func (ms *MonitoringService) ScheduleStartupMemberWarmup(config cache.WarmupConfig) bool {
	if ms == nil {
		return false
	}

	return ms.dispatchMonitorTaskWithPayloadLocked(ms.currentRunCtx(), task.Task{
		Type:    taskTypeStartupWarmupMembers,
		Payload: config,
	})
}

func (ms *MonitoringService) runAvatarScanTask(runCtx context.Context) error {
	if runCtx == nil {
		return nil
	}
	if err := runCtx.Err(); err != nil {
		return fmt.Errorf("MonitoringService.runAvatarScanTask: %w", err)
	}
	return ms.performPeriodicCheck(runCtx)
}

func (ms *MonitoringService) runStatsUpdateTask(runCtx context.Context) error {
	if runCtx == nil {
		return nil
	}
	if err := runCtx.Err(); err != nil {
		return fmt.Errorf("MonitoringService.runStatsUpdateTask: %w", err)
	}
	return ms.statsService.UpdateStatsChannels(runCtx)
}

func (ms *MonitoringService) runRolesRefreshTask(runCtx context.Context) error {
	if runCtx == nil {
		return nil
	}
	if err := runCtx.Err(); err != nil {
		return fmt.Errorf("MonitoringService.runRolesRefreshTask: %w", err)
	}
	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 || ms.store == nil {
		return nil
	}

	start := time.Now()
	totalUpdates := 0
	botUsersByGuild := make(map[string]map[string]struct{}, len(cfg.Guilds))
	for _, gcfg := range cfg.Guilds {
		if err := runCtx.Err(); err != nil {
			return fmt.Errorf("MonitoringService.runRolesRefreshTask: %w", err)
		}
		botUsers := make(map[string]struct{})
		guildUpdates := 0
		_, err := ms.forEachGuildMemberPageContext(runCtx, gcfg.GuildID, func(members []*discordgo.Member) error {
			snapshotAt := time.Now().UTC()
			snapshots := make([]storage.GuildMemberSnapshot, 0, len(members))
			for _, member := range members {
				if member == nil || member.User == nil {
					continue
				}
				if member.User.Bot {
					botUsers[member.User.ID] = struct{}{}
				}
				snapshots = append(snapshots, storage.GuildMemberSnapshot{
					UserID:   member.User.ID,
					Roles:    member.Roles,
					HasRoles: true,
					JoinedAt: member.JoinedAt,
					IsBot:    member.User.Bot,
					HasBot:   true,
				})
			}
			if len(snapshots) == 0 {
				return nil
			}
			if err := ms.store.UpsertGuildMemberSnapshotsContext(runCtx, gcfg.GuildID, snapshots, snapshotAt); err != nil {
				log.ApplicationLogger().Warn(
					"Failed to persist guild role snapshot page",
					"operation", "monitoring.refresh_roles.persist_page",
					"guildID", gcfg.GuildID,
					"members", len(snapshots),
					"err", err,
				)
				return nil
			}
			for _, snapshot := range snapshots {
				ms.rolesCacheService.CacheRolesSet(gcfg.GuildID, snapshot.UserID, snapshot.Roles)
			}
			guildUpdates += len(snapshots)
			return nil
		})
		if err != nil {
			log.ErrorLoggerRaw().Error("Error refreshing roles for guild", "guildID", gcfg.GuildID, "err", err)
			continue
		}
		totalUpdates += guildUpdates
		botUsersByGuild[gcfg.GuildID] = botUsers
	}

	reconciledAdds := 0
	reconciledRemoves := 0
	if ms.session != nil {
		for _, gcfg := range cfg.Guilds {
			features := cfg.ResolveFeatures(gcfg.GuildID)
			if !features.AutoRoleAssign || !gcfg.Roles.AutoAssignment.Enabled || gcfg.Roles.AutoAssignment.TargetRoleID == "" || len(gcfg.Roles.AutoAssignment.RequiredRoles) < 2 {
				continue
			}
			targetRoleID := gcfg.Roles.AutoAssignment.TargetRoleID
			requiredRoles := gcfg.Roles.AutoAssignment.RequiredRoles
			memberRoles, err := ms.store.GetAllGuildMemberRoles(gcfg.GuildID)
			if err != nil {
				log.ApplicationLogger().Warn("Failed to load member roles from DB for reconciliation", "guildID", gcfg.GuildID, "err", err)
				continue
			}
			botUsers := botUsersByGuild[gcfg.GuildID]
			for userID, roles := range memberRoles {
				if _, isBot := botUsers[userID]; isBot {
					continue
				}
				switch evaluateAutoRoleDecision(roles, targetRoleID, requiredRoles) {
				case autoRoleAddTarget:
					if err := ms.session.GuildMemberRoleAdd(gcfg.GuildID, userID, targetRoleID); err != nil {
						log.ApplicationLogger().Warn("Failed to grant target role during reconciliation", "guildID", gcfg.GuildID, "userID", userID, "roleID", targetRoleID, "err", err)
					} else {
						reconciledAdds++
					}
				case autoRoleRemoveTarget:
					if err := ms.session.GuildMemberRoleRemove(gcfg.GuildID, userID, targetRoleID); err != nil {
						log.ApplicationLogger().Warn("Failed to remove target role during reconciliation", "guildID", gcfg.GuildID, "userID", userID, "roleID", targetRoleID, "err", err)
					} else {
						reconciledRemoves++
					}
				}
			}
		}
	}

	log.ApplicationLogger().Info("✅ Roles DB refresh completed", "members_updated", totalUpdates, "duration", time.Since(start).Round(time.Second), "reconciled_adds", reconciledAdds, "reconciled_removes", reconciledRemoves)
	return nil
}
