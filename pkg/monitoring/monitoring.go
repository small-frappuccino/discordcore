package monitoring

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/eventlog"
	"github.com/small-frappuccino/discordcore/pkg/files"
	svc "github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/notifications"
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
	if err := RunErrWithTimeout(ctx, DependencyTimeout, stopFn); err != nil {
		slog.Error("Monitoring sub-service stop failed", slog.String("operation", operation), slog.String("service", serviceName), slog.Any("err", err))
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func startMonitoringSubService(ctx context.Context, operation, serviceName string, startFn func() error) error {
	if startFn == nil {
		return nil
	}
	if err := RunErrWithTimeout(ctx, DependencyTimeout, startFn); err != nil {
		return fmt.Errorf("%s (%s): %w", operation, serviceName, err)
	}
	return nil
}

const (
	heartbeatInterval = 5 * time.Minute
	downtimeThreshold = 30 * time.Minute

	DependencyTimeout              = 15 * time.Second
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
	session       *discordgo.Session
	arikawaState  *state.State
	eventLogger   *eventlog.Logger
	configManager *files.ConfigManager
	botInstanceID string
	store         *storage.Store
	activity      *RuntimeActivity
	notifier      *notifications.NotificationSender
	adapters      *task.NotificationAdapters
	router        *task.TaskRouter
	routerConfig  task.RouterConfig
	userWatcher   *UserWatcher
	controlCh     chan func()
	runState      atomic.Pointer[monitoringRunState]

	// Control loop exclusive fields
	cancel                 context.CancelFunc
	stopChan               chan struct{}
	stopOnce               sync.Once
	wg                     sync.WaitGroup
	cronCancel             func()
	rolesRefreshCronCancel func()
	persistStop            chan struct{}
	changeDebounce         changeDebouncer
	logger                 *slog.Logger

	// Unified cache for Discord API data (members, guilds, roles, channels)
	unifiedCache *cache.UnifiedCache

	// Sub-services for domain separation
	rolesCacheService *RolesCacheService

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

func (ms *MonitoringService) serveControl() {
	for fn := range ms.controlCh {
		fn()
	}
}

func (ms *MonitoringService) doControl(fn func() error) error {
	errCh := make(chan error, 1)
	ms.controlCh <- func() {
		errCh <- fn()
	}
	return <-errCh
}

// Name names.
func (ms *MonitoringService) Name() string {
	return "monitoring"
}

// Type types.
func (ms *MonitoringService) Type() svc.ServiceType {
	return svc.TypeMonitoring
}

// Priority prioritys.
func (ms *MonitoringService) Priority() svc.ServicePriority {
	return svc.PriorityHigh
}

// Dependencies dependencies.
func (ms *MonitoringService) Dependencies() []string {
	return nil
}

// IsRunning is running.
func (ms *MonitoringService) IsRunning() bool {
	if state := ms.runState.Load(); state != nil {
		return state.running
	}
	return false
}

// currentRunCtx returns a snapshot of ms.run.ctx taken under runMu. It returns
// nil after Stop has cleared the lifecycle, so hot-path callers can skip work
// that must not outlive the running monitoring service.
func (ms *MonitoringService) currentRunCtx() context.Context {
	if ms == nil {
		return nil
	}
	if state := ms.runState.Load(); state != nil {
		return state.ctx
	}
	return nil
}

// HealthCheck healths check.
func (ms *MonitoringService) HealthCheck(ctx context.Context) svc.HealthStatus {
	state := ms.runState.Load()
	if state == nil {
		state = &monitoringRunState{}
	}
	isRunning := state.running
	runCtx := state.ctx

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

// Stats stats.
func (ms *MonitoringService) Stats() svc.ServiceStats {
	state := ms.runState.Load()
	if state == nil {
		state = &monitoringRunState{}
	}
	startTime := state.startTime
	stopTime := state.stopTime
	restartCount := state.restartCount
	errorCount := state.errorCount
	lastErrorAt := state.lastErrorAt

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
	ms.wg.Add(1)
	go func() {
		defer ms.wg.Done()
		fn(ctx)
	}()
}

func (ms *MonitoringService) waitForOwnedWorkers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ms.wg.Wait()
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
	if state := ms.runState.Load(); state != nil {
		newState := *state
		newState.errorCount++
		newState.lastErrorAt = &now
		ms.runState.Store(&newState)
	}

}

// NewMonitoringService creates the multi-guild monitoring service. Returns error if any dependency is nil.
func NewMonitoringService(
	session *discordgo.Session,
	arikawaState *state.State,
	eventLogger *eventlog.Logger,
	configManager *files.ConfigManager,
	store *storage.Store,
	logger *slog.Logger,
) (*MonitoringService, error) {
	return NewMonitoringServiceForBot(session, arikawaState, eventLogger, configManager, store, "", logger)
}

// NewMonitoringServiceForBot creates a monitoring service scoped to the
// provided bot instance ID. For the primary runtime with metrics, callers
// should use NewMonitoringServiceForBotWithMetrics instead, or invoke
// SetMetrics immediately after creation.
func NewMonitoringServiceForBot(
	session *discordgo.Session,
	arikawaState *state.State,
	eventLogger *eventlog.Logger,
	configManager *files.ConfigManager,
	store *storage.Store,
	botInstanceID string,
	logger *slog.Logger,
) (*MonitoringService, error) {
	return NewMonitoringServiceForBotWithMetrics(session, arikawaState, eventLogger, configManager, store, botInstanceID, nil, logger)
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
	arikawaState *state.State,
	eventLogger *eventlog.Logger,
	configManager *files.ConfigManager,
	store *storage.Store,
	botInstanceID string,
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
	n := notifications.NewNotificationSender(session, logger)

	// Create unified cache with persistence enabled
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Store = store
	cacheConfig.PersistEnabled = true
	cacheConfig.Session = session
	unifiedCache := cache.NewUnifiedCache(cacheConfig)

	ms := &MonitoringService{
		session:           session,
		configManager:     configManager,
		botInstanceID:     files.NormalizeBotInstanceID(botInstanceID),
		store:             store,
		activity:          NewMonitoringRuntimeActivity(store, logger, files.NormalizeBotInstanceID(botInstanceID)),
		notifier:          n,
		unifiedCache:      unifiedCache,
		changeDebounce:    changeDebouncer{},
		userWatcher:       NewUserWatcher(session, arikawaState, configManager, store, n, unifiedCache, logger),
		controlCh:         make(chan func()),
		stopChan:          make(chan struct{}),
		rolesCacheService: NewRolesCacheService(configManager),
		eventHandlers:     make([]func(), 0),
		metrics:           metrics,
		logger:            logger,
		arikawaState:      arikawaState,
		eventLogger:       eventLogger,
	}
	ms.runState.Store(&monitoringRunState{})
	go ms.serveControl()
	ms.rebuildTaskPipeline()
	return ms, nil
}

func (ms *MonitoringService) rebuildTaskPipeline() {
	if ms.router != nil {
		ms.router.Close()
	}

	router := task.NewRouter(ms.routerConfig)
	adapters := &task.NotificationAdapters{
		Router:   router,
		Session:  ms.session,
		Config:   ms.configManager,
		Store:    ms.store,
		Notifier: ms.notifier,
	}
	adapters.RegisterHandlers()
	if ms.userWatcher != nil {
		adapters.SetAvatarProcessor(ms.userWatcher)
	}

	ms.router = router
	ms.adapters = adapters
}

// SetTaskRouterConfig sets task router config.
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

	return ms.doControl(func() error {
		state := ms.runState.Load()
		if state != nil && state.running {
			ms.logger.LogAttrs(context.Background(), slog.LevelError, "Monitoring service is already running")
			return fmt.Errorf("monitoring service is already running")
		}

		lifecycleCtx, cancelLifecycle := ms.startLifecycle(ctx)
		ms.stopChan = make(chan struct{})
		ms.stopOnce = sync.Once{}
		if ms.router == nil {
			ms.rebuildTaskPipeline()
		}

		if err := ms.handleStartupDowntimeAndMaybeRefresh(ctx); err != nil {
			cancelLifecycle()
			ms.recordLifecycleErrorLocked()
			return fmt.Errorf("handle startup downtime: %w", err)
		}
		ms.setupEventHandlers()

		globalRC := files.RuntimeConfig{}
		if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
			globalRC = scopedCfg.RuntimeConfig
		}
		globalFeatures := (&files.BotConfig{}).ResolveFeatures("")
		if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
			globalFeatures = scopedCfg.ResolveFeatures("")
		}
		workload := ms.workloadState(globalRC)

		if !workload.memberEventService {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "🛑 Entry/exit logs and auto-role assignment are disabled; MemberEventService will not start")
		}
		if globalRC.DisableAutomodLogs || !globalFeatures.Logging.AutomodAction {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "🛑 Automod logs disabled by runtime config/features")
		}
		if !workload.messageEventService {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "🛑 Message logging disabled by runtime config/features; MessageEventService will not start")
		}
		if !workload.reactionEventService {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "🛑 Reaction event handling disabled by runtime config/features; ReactionEventService will not start")
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

		serviceCtx := lifecycleCtx

		ms.registerStartupWarmupHandler(serviceCtx)
		ms.syncSchedulesLocked(serviceCtx, workload)

		ms.registerBackfillHandlers(serviceCtx, workload)
		ms.registerAutomodHandlers()

		now := time.Now()
		newState := monitoringRunState{}
		if state != nil {
			newState = *state
		}
		if newState.startTime != nil {
			newState.restartCount++
		}
		newState.ctx = serviceCtx
		ms.cancel = cancelLifecycle
		newState.running = true
		newState.startTime = &now
		newState.stopTime = nil

		if ms.unifiedCache != nil {
			ms.persistStop = ms.unifiedCache.SetPersistInterval(time.Hour)
		}

		ms.runState.Store(&newState)

		ms.scheduleEnsureGuildsListed(serviceCtx)
		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "All monitoring services started successfully")
		return nil
	})
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
	scopedGuilds := cfg.GuildsForBotInstanceFeature(ms.botInstanceID, "monitoring")
	if len(scopedGuilds) == len(cfg.Guilds) {
		return cfg
	}
	scoped := *cfg
	scoped.Guilds = scopedGuilds
	return &scoped
}

func (ms *MonitoringService) handlesGuild(guildID string) bool {
	if ms == nil || ms.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(ms.botInstanceID) == "" {
		return true
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}
	guild := ms.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}
	if !guild.BelongsToBotInstance(ms.botInstanceID) {
		return false
	}
	rolesResolvedID, _ := guild.ResolveFeatureBotInstanceID("roles")
	modResolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation")
	loggingResolvedID, _ := guild.ResolveFeatureBotInstanceID("logging")
	return rolesResolvedID == ms.botInstanceID || modResolvedID == ms.botInstanceID || loggingResolvedID == ms.botInstanceID
}

func (ms *MonitoringService) isFeatureBot(guildID string, feature string) bool {
	if ms == nil || ms.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(ms.botInstanceID) == "" {
		return true
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}
	guild := ms.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}
	if !guild.BelongsToBotInstance(ms.botInstanceID) {
		return false
	}
	resolvedID, _ := guild.ResolveFeatureBotInstanceID(feature)
	return resolvedID == ms.botInstanceID
}

func (ms *MonitoringService) getLastEvent(ctx context.Context) (time.Time, bool, error) {
	if ms == nil || ms.store == nil {
		return time.Time{}, false, fmt.Errorf("store unavailable")
	}
	if ts, ok, err := ms.store.LastEventForBot(ctx, ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" {
		return ts, ok, err
	}
	return ms.store.LastEvent(ctx)
}

func (ms *MonitoringService) getHeartbeat(ctx context.Context) (time.Time, bool, error) {
	if ms == nil || ms.store == nil {
		return time.Time{}, false, fmt.Errorf("store unavailable")
	}
	if ts, ok, err := ms.store.HeartbeatForBot(ctx, ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" {
		return ts, ok, err
	}
	return ms.store.Heartbeat(ctx)
}

// Stop stops the monitoring service. Returns error if not running.
func (ms *MonitoringService) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	return ms.doControl(func() error {
		state := ms.runState.Load()
		if state == nil || !state.running {
			ms.logger.LogAttrs(context.Background(), slog.LevelError, "Monitoring service is not running")
			return fmt.Errorf("monitoring service is not running")
		}

		cancelLifecycle := ms.cancel
		ms.cancel = nil

		newState := *state
		newState.ctx = nil
		newState.running = false
		ms.runState.Store(&newState)

		ms.stopOnce.Do(func() {
			close(ms.stopChan)
		})
		cronCancel := ms.cronCancel
		ms.cronCancel = nil
		rolesRefreshCronCancel := ms.rolesRefreshCronCancel
		ms.rolesRefreshCronCancel = nil
		persistStop := ms.persistStop
		ms.persistStop = nil
		router := ms.router
		ms.router = nil
		ms.adapters = nil

		var stopErrs []error

		if router != nil {
			if err := RunErrWithTimeout(ctx, monitoringRouterCloseTimeout, func() error {
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
		if rolesRefreshCronCancel != nil {
			rolesRefreshCronCancel()
		}
		if persistStop != nil {
			close(persistStop)
		}

		ms.removeEventHandlers()

		if err := ms.waitForOwnedWorkers(ctx); err != nil {
			stopErrs = append(stopErrs, fmt.Errorf("wait for monitoring workers: %w", err))
		}

		if ms.unifiedCache != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "💾 Persisting cache to storage...")
			if err := RunErrWithTimeout(ctx, monitoringPersistenceTimeout, ms.unifiedCache.Persist); err != nil {
				ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to persist cache", slog.Any("err", err))
				stopErrs = append(stopErrs, fmt.Errorf("persist unified cache: %w", err))
			} else {
				members := ms.unifiedCache.MemberCount()
				guilds := ms.unifiedCache.GuildCount()
				roles := ms.unifiedCache.RolesCount()
				channels := ms.unifiedCache.ChannelCount()
				total := members + guilds + roles + channels
				ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "✅ Cache persisted", slog.Int("entries_saved", total))
			}
			ms.unifiedCache.Stop()
		}

		now := time.Now()
		finalState := ms.runState.Load()
		if finalState != nil {
			fs := *finalState
			fs.stopTime = &now
			ms.runState.Store(&fs)
		}

		if len(stopErrs) > 0 {
			if state := ms.runState.Load(); state != nil {
				fs := *state
				fs.errorCount++
				fs.lastErrorAt = &now
				ms.runState.Store(&fs)
			}
			return errors.Join(stopErrs...)
		}

		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "Monitoring service cleanly stopped")
		return nil
	})
}

// initializeGuildCache initializes the current avatars of members in a specific guild.
func (ms *MonitoringService) initializeGuildCache(guildID string) {
	_ = ms.initializeGuildCacheContext(context.Background(), guildID)
}

func compareSnowflakes(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

func (ms *MonitoringService) initializeGuildCacheContext(ctx context.Context, guildID string) error {
	if ms.store == nil {
		ms.logger.LogAttrs(ctx, slog.LevelWarn, "Store is nil; skipping cache initialization for guild", slog.String("guildID", guildID))
		return nil
	}

	// Use unified cache for guild fetch
	guild, err := ms.getGuildContext(ctx, guildID)
	if err != nil {
		ms.logger.LogAttrs(ctx, slog.LevelError, "Error getting guild", slog.String("guildID", guildID), slog.Any("err", err))
		return fmt.Errorf("MonitoringService.initializeGuildCacheContext: %w", err)
	}
	ms.logger.LogAttrs(ctx, slog.LevelInfo, "Initializing cache for guild", slog.String("guildName", guild.Name), slog.String("guildID", guild.ID))
	if err := ms.store.SetGuildOwnerID(guildID, guild.OwnerID); err != nil {
		ms.logger.LogAttrs(ctx, slog.LevelWarn, "Failed to persist guild owner ID during cache initialization", slog.String("guildID", guildID), slog.String("ownerID", guild.OwnerID), slog.Any("err", err))
	}

	// Set bot join time if missing
	_, hasBotSince, err := ms.store.BotSince(ctx, guildID)
	if err != nil {
		slog.Error(
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
				ms.logger.LogAttrs(ctx, slog.LevelWarn, "Failed to persist bot join timestamp", slog.String("guildID", guildID), slog.Time("joinedAt", botMember.JoinedAt), slog.Any("err", err))
			}
		} else {
			now := time.Now()
			if err := ms.store.SetBotSince(ctx, guildID, now); err != nil {
				ms.logger.LogAttrs(ctx, slog.LevelWarn, "Failed to persist fallback bot join timestamp", slog.String("guildID", guildID), slog.Time("joinedAt", now), slog.Any("err", err))
			}
		}
	}
	totalMembers := 0
	snapshotAt := time.Now().UTC()
	var snapshots []storage.GuildMemberSnapshot

	flush := func() error {
		if len(snapshots) == 0 {
			return nil
		}
		if err := ms.store.UpsertGuildMemberSnapshotsContext(ctx, guildID, snapshots, snapshotAt); err != nil {
			slog.Warn(
				"Failed to persist guild member snapshot page",
				"operation", "monitoring.initialize_guild_cache.persist_page",
				"guildID", guildID,
				"members", len(snapshots),
				"err", err,
			)
			return err
		}
		for _, snapshot := range snapshots {
			ms.rolesCacheService.CacheRolesSet(guildID, snapshot.UserID, snapshot.Roles)
		}
		snapshots = snapshots[:0]
		return nil
	}

	nextDiscord, stopDiscord := iter.Pull2(ms.StreamGuildMembersContext(ctx, guildID))
	defer stopDiscord()

	nextDB, stopDB := iter.Pull2(ms.store.GetActiveGuildMemberStatesContext(ctx, guildID))
	defer stopDB()

	discordMember, errDiscord, okDiscord := nextDiscord()
	dbMember, errDB, okDB := nextDB()

	for okDiscord || okDB {
		if err := ctx.Err(); err != nil {
			ms.logger.LogAttrs(ctx, slog.LevelError, "Context canceled during cache initialization", slog.String("guildID", guildID), slog.Any("err", err))
			return fmt.Errorf("MonitoringService.initializeGuildCacheContext: %w", err)
		}

		var cmp int
		if !okDiscord {
			if errDB != nil {
				ms.logger.LogAttrs(ctx, slog.LevelError, "Error reading member state from DB", slog.String("guildID", guildID), slog.Any("err", errDB))
				return fmt.Errorf("MonitoringService.initializeGuildCacheContext (DB error): %w", errDB)
			}
			cmp = 1 // only DB has members left (DB is smaller, meaning we must advance DB)
		} else if !okDB {
			if errDiscord != nil {
				ms.logger.LogAttrs(ctx, slog.LevelError, "Error reading member from Discord", slog.String("guildID", guildID), slog.Any("err", errDiscord))
				return fmt.Errorf("MonitoringService.initializeGuildCacheContext (Discord error): %w", errDiscord)
			}
			cmp = -1 // only Discord has members left (Discord is smaller, advance Discord)
		} else {
			if errDiscord != nil {
				return fmt.Errorf("MonitoringService.initializeGuildCacheContext (Discord error): %w", errDiscord)
			}
			if errDB != nil {
				return fmt.Errorf("MonitoringService.initializeGuildCacheContext (DB error): %w", errDB)
			}
			cmp = compareSnowflakes(discordMember.User.ID, dbMember.UserID)
		}

		if cmp < 0 {
			// Discord ID is smaller (or DB is empty) -> New member missing from DB or just updating
			totalMembers++
			if discordMember != nil && discordMember.User != nil {
				avatarHash := discordMember.User.Avatar
				if avatarHash == "" {
					avatarHash = "default"
				}
				snapshots = append(snapshots, storage.GuildMemberSnapshot{
					UserID:     discordMember.User.ID,
					AvatarHash: avatarHash,
					HasAvatar:  true,
					Roles:      discordMember.Roles,
					HasRoles:   true,
					JoinedAt:   discordMember.JoinedAt,
					IsBot:      discordMember.User.Bot,
					HasBot:     true,
				})

				if len(snapshots) >= monitoringGuildMembersPageSize {
					_ = flush()
				}
			}
			discordMember, errDiscord, okDiscord = nextDiscord()
		} else if cmp > 0 {
			// DB ID is smaller (or Discord is empty) -> Member left the server
			if err := ms.store.MarkMemberLeftContext(ctx, guildID, dbMember.UserID, snapshotAt); err != nil {
				slog.Warn(
					"Failed to mark member as left during reconciliation",
					"guildID", guildID,
					"userID", dbMember.UserID,
					"err", err,
				)
			}
			dbMember, errDB, okDB = nextDB()
		} else {
			// Match! Member exists in both. Update DB snapshot from Discord data.
			totalMembers++
			if discordMember != nil && discordMember.User != nil {
				avatarHash := discordMember.User.Avatar
				if avatarHash == "" {
					avatarHash = "default"
				}
				snapshots = append(snapshots, storage.GuildMemberSnapshot{
					UserID:     discordMember.User.ID,
					AvatarHash: avatarHash,
					HasAvatar:  true,
					Roles:      discordMember.Roles,
					HasRoles:   true,
					JoinedAt:   discordMember.JoinedAt,
					IsBot:      discordMember.User.Bot,
					HasBot:     true,
				})

				if len(snapshots) >= monitoringGuildMembersPageSize {
					_ = flush()
				}
			}
			discordMember, errDiscord, okDiscord = nextDiscord()
			dbMember, errDB, okDB = nextDB()
		}
	}

	_ = flush()
	ms.logger.LogAttrs(ctx, slog.LevelInfo, "Guild cache initialization member scan completed", slog.String("guildID", guildID), slog.Int("members", totalMembers))
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
	return ms.doControl(func() error {
		state := ms.runState.Load()
		if state == nil || !state.running {
			return nil
		}

		workload := ms.workloadState(rc)
		var stopErrs []error

		// User logs -> re-register handlers (presence/member/user updates)
		ms.removeEventHandlers()
		ms.setupEventHandlersFromRuntimeConfig(rc)
		ms.syncSchedulesLocked(state.ctx, workload)

		if len(stopErrs) > 0 {
			return fmt.Errorf("apply runtime toggles: %w", errors.Join(stopErrs...))
		}
		return nil
	})
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
			config.FetchMissingMembers = true
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
		if err := RunErrWithTimeout(ctx, monitoringPersistenceTimeout, func() error {
			ms.ensureGuildsListed()
			return nil
		}); err != nil && ctx.Err() == nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Ensure guilds listed task failed", slog.Any("err", err))
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
			ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to dispatch startup monitor task", slog.String("taskType", string(dispatchTask.Type)), slog.Any("err", err))
		}
	})
	return true
}

// ScheduleStartupMemberWarmup schedules startup member warmup.
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
		snapshotAt := time.Now().UTC()
		var snapshots []storage.GuildMemberSnapshot

		flush := func() {
			if len(snapshots) == 0 {
				return
			}
			if err := ms.store.UpsertGuildMemberSnapshotsContext(runCtx, gcfg.GuildID, snapshots, snapshotAt); err != nil {
				slog.Warn(
					"Failed to persist guild role snapshot page",
					"operation", "monitoring.refresh_roles.persist_page",
					"guildID", gcfg.GuildID,
					"members", len(snapshots),
					"err", err,
				)
			} else {
				for _, snapshot := range snapshots {
					ms.rolesCacheService.CacheRolesSet(gcfg.GuildID, snapshot.UserID, snapshot.Roles)
				}
				guildUpdates += len(snapshots)
				totalUpdates += len(snapshots)
			}
			snapshots = snapshots[:0]
		}

		for member, err := range ms.StreamGuildMembersContext(runCtx, gcfg.GuildID) {
			if err != nil {
				ms.logger.LogAttrs(context.Background(), slog.LevelError, "Error refreshing roles for guild", slog.String("guildID", gcfg.GuildID), slog.Any("err", err))
				break
			}
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
			if len(snapshots) >= monitoringGuildMembersPageSize {
				flush()
			}
		}
		flush()
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
			memberRolesStream, err := ms.store.StreamAllGuildMemberRoles(gcfg.GuildID)
			if err != nil {
				ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Failed to load member roles from DB for reconciliation", slog.String("guildID", gcfg.GuildID), slog.Any("err", err))
				continue
			}
			botUsers := botUsersByGuild[gcfg.GuildID]
			for userID, roles := range memberRolesStream {
				if _, isBot := botUsers[userID]; isBot {
					continue
				}
				switch evaluateAutoRoleDecision(roles, targetRoleID, requiredRoles) {
				case autoRoleAddTarget:
					if err := ms.session.GuildMemberRoleAdd(gcfg.GuildID, userID, targetRoleID); err != nil {
						ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Failed to grant target role during reconciliation", slog.String("guildID", gcfg.GuildID), slog.String("userID", userID), slog.String("roleID", targetRoleID), slog.Any("err", err))
					} else {
						reconciledAdds++
					}
				case autoRoleRemoveTarget:
					if err := ms.session.GuildMemberRoleRemove(gcfg.GuildID, userID, targetRoleID); err != nil {
						ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Failed to remove target role during reconciliation", slog.String("guildID", gcfg.GuildID), slog.String("userID", userID), slog.String("roleID", targetRoleID), slog.Any("err", err))
					} else {
						reconciledRemoves++
					}
				}
			}
		}
	}

	ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "✅ Roles DB refresh completed", slog.Int("members_updated", totalUpdates), slog.Duration("duration", time.Since(start).Round(time.Second)), slog.Int("reconciled_adds", reconciledAdds), slog.Int("reconciled_removes", reconciledRemoves))
	return nil
}

type autoRoleDecision int

const (
	autoRoleNoop autoRoleDecision = iota
	autoRoleAddTarget
	autoRoleRemoveTarget
)

func evaluateAutoRoleDecision(memberRoles []string, targetRoleID string, requiredRoles []string) autoRoleDecision {
	if targetRoleID == "" || len(requiredRoles) < 2 {
		return autoRoleNoop
	}
	roleA := requiredRoles[0]
	roleB := requiredRoles[1]

	hasTarget := false
	hasA := false
	hasB := false
	for _, rid := range memberRoles {
		if rid == targetRoleID {
			hasTarget = true
		}
		if rid == roleA {
			hasA = true
		}
		if rid == roleB {
			hasB = true
		}
	}

	if hasTarget && !hasA {
		return autoRoleRemoveTarget
	}
	if !hasTarget && hasA && hasB {
		return autoRoleAddTarget
	}
	return autoRoleNoop
}
