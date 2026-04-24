package logging

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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

var mentionRe = regexp.MustCompile(`<@!?(\d+)>`)

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

// parseEntryExitBackfillMessage extracts (eventType, userID) from messages in a welcome/entry-leave channel.
// It supports:
// - Alice embeds (sent by our bot) with title "Member Joined" / "Member Left".
// - Mimu-like plain text messages containing a user mention and keywords "welcome" / "goodbye".
func parseEntryExitBackfillMessage(m *discordgo.Message, botID string) (string, string, bool) {
	if m == nil {
		return "", "", false
	}

	// 1) Our own embed format (legacy backfill)
	if m.Author != nil && botID != "" && m.Author.ID == botID && len(m.Embeds) > 0 {
		for _, em := range m.Embeds {
			if em == nil || em.Title == "" || em.Description == "" {
				continue
			}
			title := strings.ToLower(strings.TrimSpace(em.Title))
			if title != "member joined" && title != "member left" {
				continue
			}

			// Extract user ID from description: "**username** (<@id>, `id`)"
			desc := em.Description
			userID := ""
			if i := strings.Index(desc, "`"); i >= 0 {
				if j := strings.Index(desc[i+1:], "`"); j >= 0 {
					userID = desc[i+1 : i+1+j]
				}
			}
			if userID == "" {
				continue
			}
			if title == "member joined" {
				return "join", userID, true
			}
			return "leave", userID, true
		}
	}

	// 2) Mimu-like plain text
	content := strings.TrimSpace(m.Content)
	if content == "" {
		return "", "", false
	}

	lc := strings.ToLower(content)

	// New formats:
	// "welcome to alice mains! @username"
	// "@username has left the server... :("
	if strings.HasPrefix(lc, "welcome to alice mains!") {
		mm := mentionRe.FindStringSubmatch(content)
		if len(mm) >= 2 {
			return "join", mm[1], true
		}
	}
	if strings.HasSuffix(lc, "has left the server... :(") {
		mm := mentionRe.FindStringSubmatch(content)
		if len(mm) >= 2 {
			return "leave", mm[1], true
		}
	}

	mm := mentionRe.FindStringSubmatch(content)
	if len(mm) < 2 {
		return "", "", false
	}
	userID := mm[1]
	if userID == "" {
		return "", "", false
	}

	// Keep it intentionally permissive; this is best-effort reconstruction.
	if strings.Contains(lc, "goodbye") {
		return "leave", userID, true
	}
	if strings.Contains(lc, "welcome") {
		return "join", userID, true
	}
	return "", "", false
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

const (
	// Defaults (can be overridden by env).
	defaultBotPermMirrorActorRoleID = "1376361448942342164"

	// persistent_cache types
	persistentCacheTypeBotRolePermSnapshot = "bot_role_perm_snapshot"

	// persistent_cache key prefix
	persistentCacheKeyPrefixBotRolePermSnapshot = "bot_role_perm_snapshot:"
)

type botRolePermSnapshot struct {
	GuildID         string    `json:"guild_id"`
	RoleID          string    `json:"role_id"`
	PrevPermissions int64     `json:"prev_permissions"`
	SavedAt         time.Time `json:"saved_at"`
	SavedByUserID   string    `json:"saved_by_user_id"`
}

// UserWatcher contains the specific logic for processing user changes.
type UserWatcher struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	store         *storage.Store
	notifier      *NotificationSender
	cache         *cache.UnifiedCache
}

func NewUserWatcher(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store, notifier *NotificationSender, unifiedCache *cache.UnifiedCache) *UserWatcher {
	return &UserWatcher{
		session:       session,
		configManager: configManager,
		store:         store,
		notifier:      notifier,
		cache:         unifiedCache,
	}
}

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
	reactionEventService *ReactionEventService // Service for reaction metrics
	isRunning            bool
	startTime            *time.Time
	stopTime             *time.Time
	restartCount         int
	errorCount           int
	lastErrorAt          *time.Time
	stopChan             chan struct{}
	stopOnce             sync.Once
	runMu                sync.RWMutex
	runCtx               context.Context
	cancelRun            context.CancelFunc
	runWG                sync.WaitGroup
	recentChanges        map[string]time.Time // Debounce to avoid duplicates
	changesMutex         sync.RWMutex
	cronCancel           func()

	// Unified cache for Discord API data (members, guilds, roles, channels)
	unifiedCache *cache.UnifiedCache

	// In-memory roles cache with TTL to reduce REST/DB lookups
	rolesCache        map[string]cachedRoles
	rolesCacheMu      sync.RWMutex
	rolesTTL          time.Duration
	rolesCacheCleanup chan struct{}

	// Short-lived audit cache for member role updates.
	roleUpdateAuditMu       sync.Mutex
	roleUpdateAuditCache    map[string]cachedRoleUpdateAudit
	roleUpdateAuditDebounce map[string]time.Time

	// Event handler references for cleanup
	eventHandlers []interface{}

	// Presence watch tracking for targeted logs
	presenceWatchMu sync.Mutex
	presenceWatch   map[string]presenceSnapshot

	// Stats channel updates
	statsCronCancel        func()
	rolesRefreshCronCancel func()
	statsLastRun           map[string]time.Time
	statsGuilds            map[string]*statsGuildState
	statsMu                sync.Mutex

	// Metrics counters
	apiAuditLogCalls     uint64
	apiGuildMemberCalls  uint64
	apiMessagesSent      uint64
	cacheStateMemberHits uint64
	cacheRolesMemoryHits uint64
	cacheRolesStoreHits  uint64
	cacheRoleAuditHits   uint64
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
	return ms.isRunning
}

func (ms *MonitoringService) HealthCheck(ctx context.Context) svc.HealthStatus {
	ms.runMu.RLock()
	isRunning := ms.isRunning
	runCtx := ms.runCtx
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
		Details: map[string]interface{}{
			"router_ready": ms.TaskRouter() != nil,
		},
	}
}

func (ms *MonitoringService) Stats() svc.ServiceStats {
	ms.runMu.RLock()
	startTime := ms.startTime
	stopTime := ms.stopTime
	restartCount := ms.restartCount
	errorCount := ms.errorCount
	lastErrorAt := ms.lastErrorAt
	ms.runMu.RUnlock()

	stats := svc.ServiceStats{
		RestartCount:  restartCount,
		ErrorCount:    errorCount,
		CustomMetrics: ms.GetCacheStats(),
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
	ms.runWG.Add(1)
	go func() {
		defer ms.runWG.Done()
		fn(ctx)
	}()
}

func (ms *MonitoringService) waitForOwnedWorkers(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ms.runWG.Wait()
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
	ms.errorCount++
	ms.lastErrorAt = &now
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

type cachedRoles struct {
	roles     []string
	expiresAt time.Time
}

type cachedRoleUpdateAudit struct {
	fetchedAt time.Time
	entries   []*discordgo.AuditLogEntry
}

type presenceSnapshot struct {
	Status       discordgo.Status
	ClientStatus discordgo.ClientStatus
}

// NewMonitoringService creates the multi-guild monitoring service. Returns error if any dependency is nil.
func NewMonitoringService(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store) (*MonitoringService, error) {
	return NewMonitoringServiceForBot(session, configManager, store, "", "")
}

// NewMonitoringServiceForBot creates a monitoring service scoped to the guilds
// assigned to a specific bot instance.
func NewMonitoringServiceForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	store *storage.Store,
	botInstanceID string,
	defaultBotInstanceID string,
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
	n := NewNotificationSender(session)

	// Create unified cache with persistence enabled
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Store = store
	cacheConfig.PersistEnabled = true
	unifiedCache := cache.NewUnifiedCache(cacheConfig)

	ms := &MonitoringService{
		session:                 session,
		configManager:           configManager,
		botInstanceID:           files.NormalizeBotInstanceID(botInstanceID),
		defaultBotInstanceID:    files.NormalizeBotInstanceID(defaultBotInstanceID),
		store:                   store,
		activity:                newMonitoringRuntimeActivity(store, files.NormalizeBotInstanceID(botInstanceID)),
		notifier:                n,
		unifiedCache:            unifiedCache,
		userWatcher:             NewUserWatcher(session, configManager, store, n, unifiedCache),
		memberEventService:      NewMemberEventServiceForBot(session, configManager, n, store, botInstanceID, defaultBotInstanceID),
		messageEventService:     NewMessageEventServiceForBot(session, configManager, n, store, botInstanceID, defaultBotInstanceID),
		stopChan:                make(chan struct{}),
		recentChanges:           make(map[string]time.Time),
		rolesCache:              make(map[string]cachedRoles),
		rolesTTL:                5 * time.Minute,
		rolesCacheCleanup:       make(chan struct{}),
		roleUpdateAuditCache:    make(map[string]cachedRoleUpdateAudit),
		roleUpdateAuditDebounce: make(map[string]time.Time),
		eventHandlers:           make([]interface{}, 0),
		presenceWatch:           make(map[string]presenceSnapshot),
		statsLastRun:            make(map[string]time.Time),
		statsGuilds:             make(map[string]*statsGuildState),
	}
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
	if ms.isRunning {
		log.ErrorLoggerRaw().Error("Monitoring service is already running")
		return fmt.Errorf("monitoring service is already running")
	}

	lifecycleCtx, cancelLifecycle := ms.startLifecycle(ctx)
	ms.stopChan = make(chan struct{})
	ms.stopOnce = sync.Once{}
	ms.rolesCacheCleanup = make(chan struct{})
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

	// Gate reaction logging behind runtime config
	if !workload.reactionEventService {
		log.ApplicationLogger().Info("🛑 Reaction logging disabled by runtime config/features; ReactionEventService will not start")
	} else {
		// Lazily initialize service if not yet created
		if ms.reactionEventService == nil {
			ms.reactionEventService = NewReactionEventServiceForBot(ms.session, ms.configManager, ms.store, ms.botInstanceID, ms.defaultBotInstanceID)
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
	ms.startOwnedWorker(lifecycleCtx, ms.rolesCacheCleanupLoop)
	serviceCtx := lifecycleCtx

	ms.registerStartupWarmupHandler(serviceCtx)
	ms.syncSchedulesLocked(serviceCtx, workload)

	if workload.backfill {
		// Register one-shot entry/exit backfill handler (Option A).
		ms.router.RegisterHandler("monitor.backfill_entry_exit_day", func(ctx context.Context, payload any) error {
			// Payload is expected to be: struct{ ChannelID, Day string }
			// Day format: YYYY-MM-DD (UTC)
			type pld struct {
				ChannelID string
				Day       string
			}
			p, _ := payload.(pld)
			channelID := strings.TrimSpace(p.ChannelID)
			day := strings.TrimSpace(p.Day)
			if channelID == "" {
				return nil
			}
			if day == "" {
				day = time.Now().UTC().Format("2006-01-02")
			}

			start, err := time.Parse("2006-01-02", day)
			if err != nil {
				return nil
			}
			end := start.Add(24 * time.Hour)

			// Resolve guild ID from channel
			var guildID string
			if ms.session != nil && ms.session.State != nil {
				if ch, _ := ms.session.State.Channel(channelID); ch != nil {
					guildID = ch.GuildID
				}
			}
			if guildID == "" && ms.session != nil {
				if ch, err := ms.session.Channel(channelID); err == nil && ch != nil {
					guildID = ch.GuildID
				}
			}
			if guildID == "" {
				return nil
			}

			log.ApplicationLogger().Info("📥 Starting entry/exit backfill (day)", "channelID", channelID, "guildID", guildID, "day", day)

			botID := ""
			if ms.session != nil && ms.session.State != nil && ms.session.State.User != nil {
				botID = ms.session.State.User.ID
			}

			var before string
			processedCount := 0
			eventsFound := 0
			startTime := time.Now()

			for {
				if err := serviceCtx.Err(); err != nil {
					return err
				}
				msgs, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() ([]*discordgo.Message, error) {
					return ms.session.ChannelMessages(channelID, 100, before, "", "")
				})
				if err != nil {
					log.ErrorLoggerRaw().Error("Failed to fetch channel messages for backfill", "channelID", channelID, "err", err)
					break
				}
				if len(msgs) == 0 {
					break
				}

				// Messages come newest -> oldest
				stop := false
				for _, m := range msgs {
					if err := serviceCtx.Err(); err != nil {
						return err
					}
					t := m.Timestamp.UTC()
					// Stop if we've paged past the target day
					if t.Before(start) {
						stop = true
						break
					}
					// Only consider messages within the day
					if t.Before(end) && !t.Before(start) {
						evt, userID, ok := parseEntryExitBackfillMessage(m, botID)
						if ok && ms.store != nil {
							eventsFound++
							if evt == "join" {
								if err := ms.store.UpsertMemberJoin(guildID, userID, t); err != nil {
									log.ApplicationLogger().Warn("Backfill(day): failed to persist member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
								}
								if err := ms.store.IncrementDailyMemberJoin(guildID, userID, t); err != nil {
									log.ApplicationLogger().Warn("Backfill(day): failed to increment daily member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
								}
							} else if evt == "leave" {
								// If name was not in message, check if still in server via code
								stillInServer := false
								if ms.session != nil {
									mem, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() (*discordgo.Member, error) {
										return ms.session.GuildMember(guildID, userID)
									})
									if err == nil && mem != nil {
										stillInServer = true
									}
								}

								if !stillInServer {
									if err := ms.store.IncrementDailyMemberLeave(guildID, userID, t); err != nil {
										log.ApplicationLogger().Warn("Backfill(day): failed to increment daily member leave", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
									}
								}
							}
							// Record the oldest processed timestamp for this channel
							if err := ms.store.SetMetadata("backfill_progress:"+channelID, t); err != nil {
								log.ApplicationLogger().Warn("Backfill(day): failed to persist progress metadata", "guildID", guildID, "channelID", channelID, "at", t, "err", err)
							}
						}
					}
					processedCount++
				}

				if processedCount%500 == 0 || processedCount < 500 && processedCount%100 == 0 {
					log.ApplicationLogger().Info("⏳ Backfill in progress (day)...", "channelID", channelID, "processed", processedCount, "events_found", eventsFound)
				}

				// Prepare next page or stop
				before = msgs[len(msgs)-1].ID
				if stop {
					break
				}
			}

			log.ApplicationLogger().Info("✅ Backfill completed (day)", "channelID", channelID, "processed", processedCount, "events_found", eventsFound, "duration", time.Since(startTime).Round(time.Millisecond))
			return nil
		})

		// Register range-based entry/exit backfill handler (used for downtime recovery and historical scans)
		ms.router.RegisterHandler("monitor.backfill_entry_exit_range", func(ctx context.Context, payload any) error {
			p, ok := payload.(struct {
				ChannelID string
				Start     string
				End       string
			})
			if !ok {
				// Try to handle map[string]interface{} as well, which is common if coming from JSON or some routers
				if m, ok := payload.(map[string]any); ok {
					p.ChannelID, _ = m["ChannelID"].(string)
					p.Start, _ = m["Start"].(string)
					p.End, _ = m["End"].(string)
				} else {
					// Try the other struct type just in case
					type pld struct {
						ChannelID string
						Start     string
						End       string
					}
					p2, ok2 := payload.(pld)
					if ok2 {
						p.ChannelID = p2.ChannelID
						p.Start = p2.Start
						p.End = p2.End
					} else {
						log.ErrorLoggerRaw().Error("Invalid payload type for monitor.backfill_entry_exit_range", "type", fmt.Sprintf("%T", payload))
						return nil
					}
				}
			}

			channelID := strings.TrimSpace(p.ChannelID)
			startRaw := strings.TrimSpace(p.Start)
			endRaw := strings.TrimSpace(p.End)
			if channelID == "" || startRaw == "" || endRaw == "" {
				log.ErrorLoggerRaw().Warn("Missing required fields for backfill range", "channelID", channelID, "start", startRaw, "end", endRaw)
				return nil
			}

			start, err := time.Parse(time.RFC3339, startRaw)
			if err != nil {
				log.ErrorLoggerRaw().Error("Failed to parse start date for backfill range", "err", err, "start", startRaw)
				return nil
			}
			end, err := time.Parse(time.RFC3339, endRaw)
			if err != nil {
				log.ErrorLoggerRaw().Error("Failed to parse end date for backfill range", "err", err, "end", endRaw)
				return nil
			}
			start = start.UTC()
			end = end.UTC()
			if !end.After(start) {
				log.ErrorLoggerRaw().Warn("End date must be after start date for backfill range", "start", start, "end", end)
				return nil
			}

			// Resolve guild ID from channel
			var guildID string
			if ms.session != nil && ms.session.State != nil {
				if ch, _ := ms.session.State.Channel(channelID); ch != nil {
					guildID = ch.GuildID
				}
			}
			if guildID == "" && ms.session != nil {
				if ch, err := ms.session.Channel(channelID); err == nil && ch != nil {
					guildID = ch.GuildID
				}
			}
			if guildID == "" {
				log.ErrorLoggerRaw().Warn("Could not resolve guild ID for channel during backfill", "channelID", channelID)
				return nil
			}

			log.ApplicationLogger().Info("📥 Starting entry/exit backfill (range)", "channelID", channelID, "guildID", guildID, "start", start.Format(time.RFC3339), "end", end.Format(time.RFC3339))

			botID := ""
			if ms.session != nil && ms.session.State != nil && ms.session.State.User != nil {
				botID = ms.session.State.User.ID
			}

			var before string
			processedCount := 0
			eventsFound := 0
			startTime := time.Now()

			for {
				if err := serviceCtx.Err(); err != nil {
					return err
				}
				msgs, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() ([]*discordgo.Message, error) {
					return ms.session.ChannelMessages(channelID, 100, before, "", "")
				})
				if err != nil {
					log.ErrorLoggerRaw().Error("Failed to fetch channel messages for backfill range", "channelID", channelID, "err", err)
					break
				}
				if len(msgs) == 0 {
					break
				}

				// Messages come newest -> oldest
				stop := false
				for _, m := range msgs {
					if err := serviceCtx.Err(); err != nil {
						return err
					}
					t := m.Timestamp.UTC()
					// Stop if we've paged past the target window
					if t.Before(start) {
						stop = true
						break
					}
					// Only consider messages within the window
					if t.Before(end) && !t.Before(start) {
						evt, userID, ok := parseEntryExitBackfillMessage(m, botID)
						if ok && ms.store != nil {
							eventsFound++
							if evt == "join" {
								if err := ms.store.UpsertMemberJoin(guildID, userID, t); err != nil {
									log.ApplicationLogger().Warn("Backfill(range): failed to persist member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
								}
								if err := ms.store.IncrementDailyMemberJoin(guildID, userID, t); err != nil {
									log.ApplicationLogger().Warn("Backfill(range): failed to increment daily member join", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
								}
							} else if evt == "leave" {
								// If name was not in message, check if still in server via code
								stillInServer := false
								if ms.session != nil {
									mem, err := monitoringRunWithTimeout(serviceCtx, monitoringDependencyTimeout, func() (*discordgo.Member, error) {
										return ms.session.GuildMember(guildID, userID)
									})
									if err == nil && mem != nil {
										stillInServer = true
									}
								}

								if !stillInServer {
									if err := ms.store.IncrementDailyMemberLeave(guildID, userID, t); err != nil {
										log.ApplicationLogger().Warn("Backfill(range): failed to increment daily member leave", "guildID", guildID, "channelID", channelID, "userID", userID, "at", t, "err", err)
									}
								}
							}
							// Record the oldest processed timestamp for this channel
							if err := ms.store.SetMetadata("backfill_progress:"+channelID, t); err != nil {
								log.ApplicationLogger().Warn("Backfill(range): failed to persist progress metadata", "guildID", guildID, "channelID", channelID, "at", t, "err", err)
							}
						}
					}
					processedCount++
				}

				if processedCount%500 == 0 || processedCount < 500 && processedCount%100 == 0 {
					log.ApplicationLogger().Info("⏳ Backfill in progress (range)...", "channelID", channelID, "processed", processedCount, "events_found", eventsFound)
				}

				before = msgs[len(msgs)-1].ID
				if stop {
					break
				}
			}

			log.ApplicationLogger().Info("✅ Backfill completed (range)", "channelID", channelID, "processed", processedCount, "events_found", eventsFound, "duration", time.Since(startTime).Round(time.Millisecond))
			return nil
		})

		// Optionally auto-dispatch backfill tasks right after startup based on runtime config.
		//
		// Behavior:
		// - If `BackfillStartDay` is set: run day-based scan.
		// - Otherwise: if downtime is detected via `store.GetLastEvent()` and exceeds threshold, run a range scan to recover.
		//
		// New Condition: Backfill only runs if a channel is configured AND an initial start date is provided in config.
		if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
			cfg := scopedCfg
			globalRC := cfg.RuntimeConfig

			// Get all potential channels and their resolved configs
			type backfillTarget struct {
				ChannelID      string
				RC             files.RuntimeConfig
				FeatureEnabled bool
			}
			targets := make([]backfillTarget, 0)

			// Global target if configured
			if globalRC.BackfillChannelID != "" {
				targets = append(targets, backfillTarget{
					ChannelID:      strings.TrimSpace(globalRC.BackfillChannelID),
					RC:             globalRC,
					FeatureEnabled: cfg.ResolveFeatures("").Backfill.Enabled,
				})
			}

			// Guild targets
			for _, g := range cfg.Guilds {
				cid := g.Channels.BackfillChannelID()
				if cid != "" {
					featureEnabled := cfg.ResolveFeatures(g.GuildID).Backfill.Enabled
					targets = append(targets, backfillTarget{
						ChannelID:      cid,
						RC:             cfg.ResolveRuntimeConfig(g.GuildID),
						FeatureEnabled: featureEnabled,
					})
				}
			}

			if len(targets) == 0 {
				log.ApplicationLogger().Debug("No target channels for backfill check")
			} else {
				lastEvent, hasLastEvent, err := ms.getLastEvent()
				if err != nil {
					lastEvent = time.Time{}
					hasLastEvent = false
					log.ErrorLoggerRaw().Error(
						"Failed to read last event for backfill recovery; downtime recovery disabled for this startup",
						"operation", "monitoring.start.backfill.get_last_event",
						"err", err,
					)
				}
				now := time.Now().UTC()

				for _, target := range targets {
					cid := target.ChannelID
					rc := target.RC
					if !target.FeatureEnabled {
						log.ApplicationLogger().Debug("Backfill disabled by features.backfill.enabled", "channelID", cid)
						continue
					}
					day := strings.TrimSpace(rc.BackfillStartDay)
					initialDate := strings.TrimSpace(rc.BackfillInitialDate)

					if day != "" {
						dispatchCtx, cancel := context.WithTimeout(serviceCtx, monitoringStartupDispatchLimit)
						err := ms.router.Dispatch(dispatchCtx, task.Task{
							Type:    "monitor.backfill_entry_exit_day",
							Payload: struct{ ChannelID, Day string }{ChannelID: cid, Day: day},
							Options: task.TaskOptions{GroupKey: "backfill:" + cid},
						})
						cancel()
						if err != nil {
							log.ErrorLoggerRaw().Error("Failed to dispatch entry/exit backfill task (day)", "channelID", cid, "day", day, "err", err)
						} else {
							log.ApplicationLogger().Info("▶️ Dispatched entry/exit backfill task (day)", "channelID", cid, "day", day)
						}
						continue
					}

					// If no specific day, check for initial scan or recovery
					if initialDate == "" {
						log.ApplicationLogger().Debug("Backfill skip for channel: no day set and initial_date is empty", "channelID", cid)
						continue
					}

					// Check progress for this channel
					_, hasProgress, err := ms.store.Metadata("backfill_progress:" + cid)
					if err != nil {
						log.ErrorLoggerRaw().Error(
							"Failed to read backfill progress; skipping backfill dispatch for channel",
							"operation", "monitoring.start.backfill.get_progress",
							"channelID", cid,
							"err", err,
						)
						continue
					}

					if !hasProgress {
						// Use initialDate to calculate start date
						parsedDate, err := time.Parse("2006-01-02", initialDate)
						if err != nil {
							log.ApplicationLogger().Error("Failed to parse backfill_initial_date", "date", initialDate, "err", err)
							continue
						}
						start := parsedDate.Format(time.RFC3339)
						end := now.Format(time.RFC3339)
						dispatchCtx, cancel := context.WithTimeout(serviceCtx, monitoringStartupDispatchLimit)
						err = ms.router.Dispatch(dispatchCtx, task.Task{
							Type:    "monitor.backfill_entry_exit_range",
							Payload: struct{ ChannelID, Start, End string }{ChannelID: cid, Start: start, End: end},
							Options: task.TaskOptions{GroupKey: "backfill:" + cid},
						})
						cancel()
						if err != nil {
							log.ErrorLoggerRaw().Error("Failed to dispatch initial entry/exit backfill (range)", "channelID", cid, "start", start, "end", end, "err", err)
						} else {
							log.ApplicationLogger().Info("▶️ Dispatched initial entry/exit backfill (range)", "channelID", cid, "start", start)
						}
						continue
					}

					// If we have progress, check if we need downtime recovery
					if hasLastEvent {
						downtime := now.Sub(lastEvent)
						if downtime > downtimeThreshold {
							start := lastEvent.UTC().Format(time.RFC3339)
							end := now.Format(time.RFC3339)
							dispatchCtx, cancel := context.WithTimeout(serviceCtx, monitoringStartupDispatchLimit)
							err := ms.router.Dispatch(dispatchCtx, task.Task{
								Type:    "monitor.backfill_entry_exit_range",
								Payload: struct{ ChannelID, Start, End string }{ChannelID: cid, Start: start, End: end},
								Options: task.TaskOptions{GroupKey: "backfill:" + cid},
							})
							cancel()
							if err != nil {
								log.ErrorLoggerRaw().Error("Failed to dispatch entry/exit backfill recovery (range)", "channelID", cid, "start", start, "end", end, "err", err)
							} else {
								log.ApplicationLogger().Info("▶️ Dispatched entry/exit backfill recovery (range)", "channelID", cid, "start", start, "end", end)
							}
						} else {
							log.ApplicationLogger().Debug("Downtime below threshold, skipping recovery", "channelID", cid, "downtime", downtime)
						}
					} else {
						log.ApplicationLogger().Debug("No last event recorded, skipping downtime recovery", "channelID", cid)
					}
				}
			}
		} else {
			log.ApplicationLogger().Info("Backfill skip: config manager or config is nil")
		}
	}

	now := time.Now()
	if ms.startTime != nil {
		ms.restartCount++
	}
	ms.runCtx = serviceCtx
	ms.cancelRun = cancelLifecycle
	ms.isRunning = true
	ms.startTime = &now
	ms.stopTime = nil

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

type monitoringWorkloadState struct {
	memberEventService    bool
	messageEventService   bool
	reactionEventService  bool
	presenceHandler       bool
	memberUpdateHandler   bool
	statsMemberHandlers   bool
	userUpdateHandler     bool
	botPermMirrorHandlers bool
	avatarScan            bool
	statsUpdates          bool
	rolesRefresh          bool
	backfill              bool
}

func resolveMonitoringWorkloadState(cfg *files.BotConfig) monitoringWorkloadState {
	state := monitoringWorkloadState{}
	if cfg == nil {
		return state
	}

	state.memberEventService = shouldRunMemberEventService(cfg, cfg.RuntimeConfig)
	for _, guildCfg := range cfg.Guilds {
		features := cfg.ResolveFeatures(guildCfg.GuildID)
		if !features.Services.Monitoring {
			continue
		}
		rc := cfg.ResolveRuntimeConfig(guildCfg.GuildID)
		statsEnabledForGuild := features.StatsChannels && statsEnabled(guildCfg.Stats)

		avatarEnabled := !rc.DisableUserLogs && features.Logging.AvatarLogging
		roleEnabled := !rc.DisableUserLogs && features.Logging.RoleUpdate
		presenceWatchEnabled := (features.PresenceWatch.User && strings.TrimSpace(rc.PresenceWatchUserID) != "") ||
			(features.PresenceWatch.Bot && rc.PresenceWatchBot)

		if avatarEnabled || presenceWatchEnabled {
			state.presenceHandler = true
		}
		if avatarEnabled || roleEnabled || statsEnabledForGuild {
			state.memberUpdateHandler = true
		}
		if avatarEnabled {
			state.userUpdateHandler = true
			state.avatarScan = true
		}
		if !rc.DisableMessageLogs && (features.Logging.MessageProcess || features.Logging.MessageEdit || features.Logging.MessageDelete) {
			state.messageEventService = true
		}
		if !rc.DisableReactionLogs && features.Logging.ReactionMetric {
			state.reactionEventService = true
		}
		if statsEnabledForGuild {
			state.statsUpdates = true
			state.statsMemberHandlers = true
		}
		if features.Backfill.Enabled && strings.TrimSpace(rc.BackfillChannelID) != "" {
			state.backfill = true
		}
		if features.Safety.BotRolePermMirror && !rc.DisableBotRolePermMirror {
			state.botPermMirrorHandlers = true
		}
		if roleEnabled || (features.AutoRoleAssign && guildCfg.Roles.AutoAssignment.Enabled) {
			state.rolesRefresh = true
		}
	}
	if state.memberEventService {
		state.rolesRefresh = true
	}
	return state
}

func (ms *MonitoringService) shouldRunMemberEventService(globalRC files.RuntimeConfig) bool {
	if ms.configManager == nil {
		return false
	}
	return shouldRunMemberEventService(ms.scopedConfig(), globalRC)
}

func (ms *MonitoringService) workloadState(globalRC files.RuntimeConfig) monitoringWorkloadState {
	cfg := ms.scopedConfig()
	if cfg == nil {
		return monitoringWorkloadState{}
	}
	scoped := *cfg
	scoped.RuntimeConfig = globalRC
	return resolveMonitoringWorkloadState(&scoped)
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

func (ms *MonitoringService) configuredGuilds() []files.GuildConfig {
	if cfg := ms.scopedConfig(); cfg != nil {
		return cfg.Guilds
	}
	return nil
}

func (ms *MonitoringService) handlesGuild(guildID string) bool {
	if ms == nil || ms.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(ms.botInstanceID) == "" && files.NormalizeBotInstanceID(ms.defaultBotInstanceID) == "" {
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
	return guild.EffectiveBotInstanceID(ms.defaultBotInstanceID) == files.NormalizeBotInstanceID(ms.botInstanceID)
}

func (ms *MonitoringService) getLastEvent() (time.Time, bool, error) {
	if ms == nil || ms.store == nil {
		return time.Time{}, false, fmt.Errorf("store unavailable")
	}
	if ts, ok, err := ms.store.LastEventForBot(ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" || ms.botInstanceID != ms.defaultBotInstanceID {
		return ts, ok, err
	}
	return ms.store.LastEvent()
}

func (ms *MonitoringService) getHeartbeat() (time.Time, bool, error) {
	if ms == nil || ms.store == nil {
		return time.Time{}, false, fmt.Errorf("store unavailable")
	}
	if ts, ok, err := ms.store.HeartbeatForBot(ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" || ms.botInstanceID != ms.defaultBotInstanceID {
		return ts, ok, err
	}
	return ms.store.Heartbeat()
}

// Stop stops the monitoring service. Returns error if not running.
func (ms *MonitoringService) Stop(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	ms.runMu.Lock()
	if !ms.isRunning {
		ms.runMu.Unlock()
		log.ErrorLoggerRaw().Error("Monitoring service is not running")
		return fmt.Errorf("monitoring service is not running")
	}

	cancelLifecycle := ms.cancelRun
	ms.cancelRun = nil
	ms.runCtx = nil
	ms.isRunning = false
	ms.stopOnce.Do(func() {
		close(ms.stopChan)
	})
	if ms.rolesCacheCleanup != nil {
		close(ms.rolesCacheCleanup)
		ms.rolesCacheCleanup = nil
	}
	cronCancel := ms.cronCancel
	ms.cronCancel = nil
	statsCronCancel := ms.statsCronCancel
	ms.statsCronCancel = nil
	rolesRefreshCronCancel := ms.rolesRefreshCronCancel
	ms.rolesRefreshCronCancel = nil
	router := ms.router
	ms.router = nil
	ms.adapters = nil
	ms.runMu.Unlock()

	if cancelLifecycle != nil {
		cancelLifecycle()
	}
	var stopErrs []error
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
			members, _, _, _ := ms.unifiedCache.MemberMetrics()
			guilds, _, _, _ := ms.unifiedCache.GuildMetrics()
			roles, _, _, _ := ms.unifiedCache.RolesMetrics()
			channels, _, _, _ := ms.unifiedCache.ChannelMetrics()
			total := members + guilds + roles + channels
			log.ApplicationLogger().Info("✅ Cache persisted", "entries_saved", total)
		}
		ms.unifiedCache.Stop()
	}

	if router != nil {
		if err := monitoringRunErrWithTimeout(ctx, monitoringRouterCloseTimeout, func() error {
			router.Close()
			return nil
		}); err != nil {
			stopErrs = append(stopErrs, fmt.Errorf("close task router: %w", err))
		}
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
	ms.stopTime = &now
	if len(stopErrs) > 0 {
		ms.recordLifecycleErrorLocked()
		ms.runMu.Unlock()
		return errors.Join(stopErrs...)
	}
	ms.runMu.Unlock()

	log.ApplicationLogger().Info("Monitoring service stopped")
	return nil
}

// initializeCache loads the current member users for all configured guilds.
func (ms *MonitoringService) initializeCache() {
	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.ApplicationLogger().Info("No guild configured for monitoring")
		return
	}
	ms.markEvent(nil)
	guildIDs := make([]string, 0, len(cfg.Guilds))
	for _, gcfg := range cfg.Guilds {
		if gid := strings.TrimSpace(gcfg.GuildID); gid != "" {
			guildIDs = append(guildIDs, gid)
		}
	}
	if err := runGuildTasksWithLimit(context.Background(), guildIDs, monitoringMaxConcurrentGuildScan, func(runCtx context.Context, guildID string) error {
		return ms.initializeGuildCacheContext(runCtx, guildID)
	}); err != nil {
		log.ApplicationLogger().Warn("Some guild cache initializations failed", "err", err)
	}
	// No-op: avatars are persisted per change in the Postgres store
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
		return err
	}
	log.ApplicationLogger().Info("Initializing cache for guild", "guildName", guild.Name, "guildID", guild.ID)
	if err := ms.store.SetGuildOwnerID(guildID, guild.OwnerID); err != nil {
		log.ApplicationLogger().Warn("Failed to persist guild owner ID during cache initialization", "guildID", guildID, "ownerID", guild.OwnerID, "err", err)
	}

	// Set bot join time if missing
	_, hasBotSince, err := ms.store.GetBotSince(guildID)
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
			if err := ms.store.SetBotSince(guildID, botMember.JoinedAt); err != nil {
				log.ApplicationLogger().Warn("Failed to persist bot join timestamp", "guildID", guildID, "joinedAt", botMember.JoinedAt, "err", err)
			}
		} else {
			now := time.Now()
			if err := ms.store.SetBotSince(guildID, now); err != nil {
				log.ApplicationLogger().Warn("Failed to persist fallback bot join timestamp", "guildID", guildID, "joinedAt", now, "err", err)
			}
		}
	}
	totalMembers, err := ms.forEachGuildMemberPageContext(ctx, guildID, func(members []*discordgo.Member) error {
		snapshotAt := time.Now().UTC()
		snapshots := make([]storage.GuildMemberSnapshot, 0, len(members))
		for _, member := range members {
			if err := ctx.Err(); err != nil {
				return err
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
			ms.cacheRolesSet(guildID, snapshot.UserID, snapshot.Roles)
		}
		return nil
	})
	if err != nil {
		log.ErrorLoggerRaw().Error("Error getting members for guild", "guildID", guildID, "err", err)
		return err
	}
	log.ApplicationLogger().Info("Guild cache initialization member scan completed", "guildID", guildID, "members", totalMembers)
	return nil
}

// ApplyRuntimeToggles hot-applies a subset of runtime_config toggles without restarting the process.
//
// Scope:
// - ALICE_DISABLE_ENTRY_EXIT_LOGS: start/stop MemberEventService
// - ALICE_DISABLE_MESSAGE_LOGS: start/stop MessageEventService
// - ALICE_DISABLE_REACTION_LOGS: start/stop ReactionEventService
// - ALICE_DISABLE_USER_LOGS: re-register user-related handlers (presence/member/user updates)
// - ALICE_DISABLE_BOT_ROLE_PERM_MIRROR / ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID: no-op here (checked at event time)
//
// Notes:
// - Backfill settings are intentionally not handled here.
// - This is safe to call even if MonitoringService is not running; it will no-op.
func (ms *MonitoringService) ApplyRuntimeToggles(ctx context.Context, rc files.RuntimeConfig) error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()

	if !ms.isRunning {
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

	// Reaction logs -> ReactionEventService
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
			ms.reactionEventService = NewReactionEventServiceForBot(ms.session, ms.configManager, ms.store, ms.botInstanceID, ms.defaultBotInstanceID)
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
	ms.syncSchedulesLocked(ms.runCtx, workload)

	if len(stopErrs) > 0 {
		return fmt.Errorf("apply runtime toggles: %w", errors.Join(stopErrs...))
	}
	return nil
}

func (ms *MonitoringService) syncSchedulesLocked(runCtx context.Context, state monitoringWorkloadState) {
	if !state.avatarScan && ms.cronCancel != nil {
		ms.cronCancel()
		ms.cronCancel = nil
	}
	if !state.statsUpdates && ms.statsCronCancel != nil {
		ms.statsCronCancel()
		ms.statsCronCancel = nil
	}
	if !state.rolesRefresh && ms.rolesRefreshCronCancel != nil {
		ms.rolesRefreshCronCancel()
		ms.rolesRefreshCronCancel = nil
	}

	if ms.router == nil || runCtx == nil {
		return
	}

	if state.avatarScan {
		ms.router.RegisterHandler("monitor.scan_avatars", func(ctx context.Context, _ any) error {
			return ms.runAvatarScanTask(runCtx)
		})
		if ms.cronCancel == nil {
			ms.cronCancel = ms.router.ScheduleEvery(2*time.Hour, task.Task{Type: "monitor.scan_avatars"})
		}
	}

	if state.statsUpdates {
		ms.router.RegisterHandler("monitor.update_stats_channels", func(ctx context.Context, _ any) error {
			return ms.runStatsUpdateTask(runCtx)
		})
		if ms.statsCronCancel == nil {
			ms.statsCronCancel = ms.router.ScheduleEvery(5*time.Minute, task.Task{Type: "monitor.update_stats_channels"})
			ms.dispatchMonitorTaskLocked(runCtx, "monitor.update_stats_channels")
		}
	}

	if state.rolesRefresh {
		ms.router.RegisterHandler("monitor.refresh_roles", func(ctx context.Context, _ any) error {
			return ms.runRolesRefreshTask(runCtx)
		})
		if ms.rolesRefreshCronCancel == nil {
			ms.rolesRefreshCronCancel = ms.router.ScheduleDailyAtUTC(3, 0, task.Task{Type: "monitor.refresh_roles"})
			ms.dispatchMonitorTaskLocked(runCtx, "monitor.refresh_roles")
		}
	}
}

func (ms *MonitoringService) registerStartupWarmupHandler(runCtx context.Context) {
	if ms == nil || ms.router == nil || runCtx == nil {
		return
	}

	ms.router.RegisterHandler(taskTypeStartupWarmupMembers, func(ctx context.Context, payload any) error {
		if err := runCtx.Err(); err != nil {
			return err
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
	ms.dispatchMonitorTaskWithPayloadLocked(runCtx, task.Task{Type: taskType})
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

	ms.runMu.RLock()
	runCtx := ms.runCtx
	ms.runMu.RUnlock()

	return ms.dispatchMonitorTaskWithPayloadLocked(runCtx, task.Task{
		Type:    taskTypeStartupWarmupMembers,
		Payload: config,
	})
}

func (ms *MonitoringService) runAvatarScanTask(runCtx context.Context) error {
	if runCtx == nil {
		return nil
	}
	if err := runCtx.Err(); err != nil {
		return err
	}
	return ms.performPeriodicCheck(runCtx)
}

func (ms *MonitoringService) runStatsUpdateTask(runCtx context.Context) error {
	if runCtx == nil {
		return nil
	}
	if err := runCtx.Err(); err != nil {
		return err
	}
	return ms.updateStatsChannels(runCtx)
}

func (ms *MonitoringService) runRolesRefreshTask(runCtx context.Context) error {
	if runCtx == nil {
		return nil
	}
	if err := runCtx.Err(); err != nil {
		return err
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
			return err
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
				ms.cacheRolesSet(gcfg.GuildID, snapshot.UserID, snapshot.Roles)
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
