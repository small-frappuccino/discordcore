package logging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	svc "github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/theme"
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
					_, hasProgress, err := ms.store.GetMetadata("backfill_progress:" + cid)
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
	if ts, ok, err := ms.store.GetLastEventForBot(ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" || ms.botInstanceID != ms.defaultBotInstanceID {
		return ts, ok, err
	}
	return ms.store.GetLastEvent()
}

func (ms *MonitoringService) getHeartbeat() (time.Time, bool, error) {
	if ms == nil || ms.store == nil {
		return time.Time{}, false, fmt.Errorf("store unavailable")
	}
	if ts, ok, err := ms.store.GetHeartbeatForBot(ms.botInstanceID); err != nil || ok || strings.TrimSpace(ms.botInstanceID) == "" || ms.botInstanceID != ms.defaultBotInstanceID {
		return ts, ok, err
	}
	return ms.store.GetHeartbeat()
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

// setupEventHandlers registra handlers do Discord.
func (ms *MonitoringService) setupEventHandlers() {
	// Delegate to config-driven version (keeps behavior in one spot).
	rc := files.RuntimeConfig{}
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		rc = scopedCfg.RuntimeConfig
	}
	ms.setupEventHandlersFromRuntimeConfig(rc)
}

// setupEventHandlersFromRuntimeConfig registers handlers based on the provided runtime config.
// This is used both at startup and for hot-apply.
func (ms *MonitoringService) setupEventHandlersFromRuntimeConfig(rc files.RuntimeConfig) {
	state := ms.workloadState(rc)

	if state.presenceHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.session.AddHandler(ms.handlePresenceUpdate))
	}
	if state.memberUpdateHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.session.AddHandler(ms.handleMemberUpdate))
	}
	if state.statsMemberHandlers {
		ms.eventHandlers = append(ms.eventHandlers,
			ms.session.AddHandler(ms.handleStatsMemberAdd),
			ms.session.AddHandler(ms.handleStatsMemberRemove),
		)
	}
	if state.userUpdateHandler {
		ms.eventHandlers = append(ms.eventHandlers, ms.session.AddHandler(ms.handleUserUpdate))
	}
	ms.eventHandlers = append(ms.eventHandlers,
		ms.session.AddHandler(ms.handleGuildCreate),
		ms.session.AddHandler(ms.handleGuildUpdate),
	)
	if !state.presenceHandler && !state.memberUpdateHandler && !state.userUpdateHandler {
		log.ApplicationLogger().Info("🛑 User and presence handlers are disabled by effective runtime/features")
	}
	if state.botPermMirrorHandlers {
		ms.eventHandlers = append(ms.eventHandlers,
			ms.session.AddHandler(ms.handleRoleUpdateForBotPermMirroring),
			ms.session.AddHandler(ms.handleRoleCreateForBotPermMirroring),
		)
	}
}

// removeEventHandlers removes all registered event handlers
// Note: discordgo returns an unsubscribe function from AddHandler; we capture those when registering and call them here
// Handlers are explicitly removed; any remaining handlers will be dropped when the session is closed on shutdown
func (ms *MonitoringService) removeEventHandlers() {
	// Call unsubscriber functions returned by AddHandler to deregister callbacks
	for _, h := range ms.eventHandlers {
		if h == nil {
			continue
		}
		if fn, ok := h.(func()); ok {
			fn()
		}
	}
	ms.eventHandlers = nil
}

// ensureGuildsListed adds minimal guild entries to discordcore.json
// for all guilds present in the session but missing from the configuration.
func (ms *MonitoringService) ensureGuildsListed() {
	if ms.session == nil || ms.session.State == nil {
		return
	}

	for _, g := range ms.session.State.Guilds {
		if g == nil || g.ID == "" {
			continue
		}
		if ms.configManager.GuildConfig(g.ID) == nil {
			if err := ms.configManager.EnsureMinimalGuildConfigForBot(g.ID, ms.botInstanceID); err != nil {
				log.ErrorLoggerRaw().Error("Error adding minimal dormant guild entry", "guildID", g.ID, "err", err)
			} else {
				log.ApplicationLogger().Info("📘 Guild listed in config with disabled defaults", "guildID", g.ID)
			}
		}
	}
}

func (ms *MonitoringService) handleGuildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	if e == nil {
		return
	}
	guildID := e.ID
	if guildID == "" {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_create",
		slog.String("guildID", guildID),
	)
	defer done()

	if ms.configManager.GuildConfig(guildID) == nil {
		if err := ms.configManager.EnsureMinimalGuildConfigForBot(guildID, ms.botInstanceID); err != nil {
			log.ErrorLoggerRaw().Error("Error adding dormant guild entry for new guild", "guildID", guildID, "err", err)
			return
		}
		log.ApplicationLogger().Info("🆕 New guild listed in config with disabled defaults", "guildID", guildID)
		ms.initializeGuildCache(guildID)
		// No-op: avatars persisted per change in Postgres store
	}
}

// handleGuildUpdate updates the OwnerID cache when the server ownership changes.
func (ms *MonitoringService) handleGuildUpdate(s *discordgo.Session, e *discordgo.GuildUpdate) {
	if e == nil || e.Guild == nil || e.Guild.ID == "" {
		return
	}
	if !ms.handlesGuild(e.Guild.ID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_update",
		slog.String("guildID", e.Guild.ID),
	)
	defer done()

	if ms.store != nil {
		prev, ok, err := ms.store.GetGuildOwnerID(e.Guild.ID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to read guild owner cache during guild update",
				"operation", "monitoring.handle_guild_update.get_owner",
				"guildID", e.Guild.ID,
				"err", err,
			)
		} else if ok && prev != e.Guild.OwnerID {
			log.ApplicationLogger().Info("Guild owner changed", "guildID", e.Guild.ID, "from", prev, "to", e.Guild.OwnerID)
		}
		if err := ms.store.SetGuildOwnerID(e.Guild.ID, e.Guild.OwnerID); err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to persist guild owner cache during guild update",
				"operation", "monitoring.handle_guild_update.set_owner",
				"guildID", e.Guild.ID,
				"ownerID", e.Guild.OwnerID,
				"err", err,
			)
		}
	}
}

// handlePresenceUpdate processes presence updates (includes avatar).
func (ms *MonitoringService) handlePresenceUpdate(s *discordgo.Session, m *discordgo.PresenceUpdate) {
	if m.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}
	if m.User.Username == "" {
		log.ApplicationLogger().Debug("PresenceUpdate ignored (empty username)", "userID", m.User.ID, "guildID", m.GuildID)
		ms.handlePresenceWatch(m)
		return
	}

	done := perf.StartGatewayEvent(
		"presence_update",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.User.ID),
	)
	defer done()

	ms.markEvent(nil)
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)
	ms.handlePresenceWatch(m)
}

func (ms *MonitoringService) handlePresenceWatch(m *discordgo.PresenceUpdate) {
	if m == nil || m.User == nil || ms.configManager == nil {
		return
	}
	cfg := ms.scopedConfig()
	if cfg == nil {
		return
	}
	rc := cfg.ResolveRuntimeConfig(m.GuildID)
	features := cfg.ResolveFeatures(m.GuildID)
	watchUserID := strings.TrimSpace(rc.PresenceWatchUserID)
	watchBot := rc.PresenceWatchBot
	if !features.PresenceWatch.User {
		watchUserID = ""
	}
	if !features.PresenceWatch.Bot {
		watchBot = false
	}
	if watchUserID == "" && !watchBot {
		return
	}

	userID := strings.TrimSpace(m.User.ID)
	if userID == "" {
		return
	}

	botID := ""
	if ms.session != nil && ms.session.State != nil && ms.session.State.User != nil {
		botID = ms.session.State.User.ID
	}
	isBotTarget := watchBot && botID != "" && userID == botID
	isUserTarget := watchUserID != "" && userID == watchUserID
	if !isBotTarget && !isUserTarget {
		return
	}

	snap := presenceSnapshot{
		Status:       normalizeStatus(m.Status),
		ClientStatus: normalizeClientStatus(m.ClientStatus),
	}

	ms.presenceWatchMu.Lock()
	prev, hasPrev := ms.presenceWatch[userID]
	if hasPrev && presenceSnapshotEqual(prev, snap) {
		ms.presenceWatchMu.Unlock()
		return
	}
	ms.presenceWatch[userID] = snap
	ms.presenceWatchMu.Unlock()

	statusChange := ""
	if hasPrev {
		if normalizeStatus(prev.Status) != normalizeStatus(snap.Status) {
			statusChange = fmt.Sprintf("%s -> %s", statusDisplay(prev.Status), statusDisplay(snap.Status))
		}
	} else {
		statusChange = statusDisplay(snap.Status)
	}

	deviceChanges := deviceStatusChanges(prev.ClientStatus, snap.ClientStatus)

	username := strings.TrimSpace(m.User.Username)
	if username == "" {
		username = userID
	}

	target := "user"
	if isBotTarget {
		target = "bot"
	}

	fields := []any{
		"target", target,
		"userID", userID,
		"username", username,
		"status", presenceStatusLabel(snap.Status, snap.ClientStatus),
		"devices", clientStatusSummary(snap.ClientStatus),
	}
	if m.GuildID != "" {
		fields = append(fields, "guildID", m.GuildID)
	}
	if statusChange != "" {
		fields = append(fields, "status_change", statusChange)
	}
	if len(deviceChanges) > 0 {
		fields = append(fields, "device_changes", strings.Join(deviceChanges, "; "))
	}

	log.ApplicationLogger().Info("Presence watch update", fields...)
}

func presenceSnapshotEqual(a, b presenceSnapshot) bool {
	if normalizeStatus(a.Status) != normalizeStatus(b.Status) {
		return false
	}
	return clientStatusEqual(a.ClientStatus, b.ClientStatus)
}

func normalizeStatus(status discordgo.Status) discordgo.Status {
	if strings.TrimSpace(string(status)) == "" {
		return discordgo.StatusOffline
	}
	return status
}

func normalizeClientStatus(cs discordgo.ClientStatus) discordgo.ClientStatus {
	cs.Desktop = normalizeStatus(cs.Desktop)
	cs.Mobile = normalizeStatus(cs.Mobile)
	cs.Web = normalizeStatus(cs.Web)
	return cs
}

func clientStatusEqual(a, b discordgo.ClientStatus) bool {
	a = normalizeClientStatus(a)
	b = normalizeClientStatus(b)
	return a.Desktop == b.Desktop && a.Mobile == b.Mobile && a.Web == b.Web
}

func isActiveStatus(status discordgo.Status) bool {
	switch normalizeStatus(status) {
	case discordgo.StatusOnline, discordgo.StatusIdle, discordgo.StatusDoNotDisturb:
		return true
	default:
		return false
	}
}

func statusDisplay(status discordgo.Status) string {
	switch normalizeStatus(status) {
	case discordgo.StatusOnline:
		return "online"
	case discordgo.StatusIdle:
		return "idle (away)"
	case discordgo.StatusDoNotDisturb:
		return "dnd"
	case discordgo.StatusInvisible:
		return "invisible"
	case discordgo.StatusOffline:
		return "offline"
	default:
		return string(status)
	}
}

func presenceStatusLabel(status discordgo.Status, client discordgo.ClientStatus) string {
	label := statusDisplay(status)
	if isActiveStatus(client.Mobile) {
		label += " (mobile)"
	}
	return label
}

func clientStatusSummary(cs discordgo.ClientStatus) string {
	cs = normalizeClientStatus(cs)
	return fmt.Sprintf("desktop=%s mobile=%s web=%s", statusDisplay(cs.Desktop), statusDisplay(cs.Mobile), statusDisplay(cs.Web))
}

func deviceStatusChanges(prev, cur discordgo.ClientStatus) []string {
	prev = normalizeClientStatus(prev)
	cur = normalizeClientStatus(cur)
	changes := []string{}
	addChange := func(label string, prevStatus, curStatus discordgo.Status) {
		prevActive := isActiveStatus(prevStatus)
		curActive := isActiveStatus(curStatus)
		if prevActive != curActive {
			if curActive {
				changes = append(changes, fmt.Sprintf("%s entered (%s)", label, statusDisplay(curStatus)))
			} else {
				changes = append(changes, fmt.Sprintf("%s left", label))
			}
			return
		}
		if prevStatus != curStatus {
			changes = append(changes, fmt.Sprintf("%s status %s -> %s", label, statusDisplay(prevStatus), statusDisplay(curStatus)))
		}
	}

	addChange("desktop", prev.Desktop, cur.Desktop)
	addChange("mobile", prev.Mobile, cur.Mobile)
	addChange("web", prev.Web, cur.Web)
	return changes
}

type auditRolePartial struct {
	ID   string
	Name string
}

func diffStringIDs(prev, cur []string) (added []string, removed []string) {
	curSet := make(map[string]struct{}, len(cur))
	for _, roleID := range cur {
		if roleID != "" {
			curSet[roleID] = struct{}{}
		}
	}
	prevSet := make(map[string]struct{}, len(prev))
	for _, roleID := range prev {
		if roleID != "" {
			prevSet[roleID] = struct{}{}
		}
	}
	for roleID := range curSet {
		if _, ok := prevSet[roleID]; !ok {
			added = append(added, roleID)
		}
	}
	for roleID := range prevSet {
		if _, ok := curSet[roleID]; !ok {
			removed = append(removed, roleID)
		}
	}
	return added, removed
}

func (ms *MonitoringService) computeMemberRoleDiff(guildID, userID string, proposed []string) (cur []string, added []string, removed []string, known bool) {
	switch {
	case proposed != nil:
		cur = append([]string(nil), proposed...)
		known = true
	default:
		member, err := ms.getGuildMember(guildID, userID)
		if err == nil && member != nil {
			cur = append([]string(nil), member.Roles...)
			known = true
		}
	}
	if !known {
		return nil, nil, nil, false
	}

	var prev []string
	if p, ok := ms.cacheRolesGet(guildID, userID); ok {
		atomic.AddUint64(&ms.cacheRolesMemoryHits, 1)
		prev = p
	} else if ms.store != nil {
		if r, err := ms.store.GetMemberRoles(guildID, userID); err == nil {
			atomic.AddUint64(&ms.cacheRolesStoreHits, 1)
			prev = r
		}
	}

	added, removed = diffStringIDs(prev, cur)
	return cur, added, removed, true
}

func (ms *MonitoringService) persistMemberRoleSnapshot(guildID, userID string, roles []string) error {
	if ms.store != nil {
		if err := ms.store.UpsertMemberRoles(guildID, userID, roles, time.Now()); err != nil {
			return err
		}
	}
	ms.cacheRolesSet(guildID, userID, roles)
	return nil
}

func (ms *MonitoringService) getRoleUpdateAuditEntries(guildID string, forceRefresh bool) ([]*discordgo.AuditLogEntry, bool, error) {
	now := time.Now()

	ms.roleUpdateAuditMu.Lock()
	if !forceRefresh {
		if entry, ok := ms.roleUpdateAuditCache[guildID]; ok && now.Sub(entry.fetchedAt) < monitoringRoleAuditCacheTTL {
			entries := append([]*discordgo.AuditLogEntry(nil), entry.entries...)
			atomic.AddUint64(&ms.cacheRoleAuditHits, 1)
			ms.roleUpdateAuditMu.Unlock()
			return entries, true, nil
		}
	}
	ms.roleUpdateAuditMu.Unlock()

	audit, err := ms.session.GuildAuditLog(guildID, "", "", int(discordgo.AuditLogActionMemberRoleUpdate), 10)
	atomic.AddUint64(&ms.apiAuditLogCalls, 1)
	if err != nil {
		return nil, false, err
	}
	if audit == nil {
		return nil, false, nil
	}

	entries := make([]*discordgo.AuditLogEntry, 0, len(audit.AuditLogEntries))
	for _, entry := range audit.AuditLogEntries {
		if entry == nil || entry.ActionType == nil || *entry.ActionType != discordgo.AuditLogActionMemberRoleUpdate {
			continue
		}
		entries = append(entries, entry)
	}

	ms.roleUpdateAuditMu.Lock()
	ms.roleUpdateAuditCache[guildID] = cachedRoleUpdateAudit{
		fetchedAt: now,
		entries:   append([]*discordgo.AuditLogEntry(nil), entries...),
	}
	if len(ms.roleUpdateAuditCache) > 100 {
		for key, entry := range ms.roleUpdateAuditCache {
			if now.Sub(entry.fetchedAt) > 5*time.Minute {
				delete(ms.roleUpdateAuditCache, key)
			}
		}
	}
	ms.roleUpdateAuditMu.Unlock()

	return entries, false, nil
}

func (ms *MonitoringService) shouldDebounceRoleUpdateAuditRefresh(guildID, userID string) bool {
	now := time.Now()
	key := guildID + ":" + userID

	ms.roleUpdateAuditMu.Lock()
	defer ms.roleUpdateAuditMu.Unlock()

	if last, ok := ms.roleUpdateAuditDebounce[key]; ok && now.Sub(last) < monitoringRoleAuditDebounceTTL {
		return true
	}
	ms.roleUpdateAuditDebounce[key] = now
	if len(ms.roleUpdateAuditDebounce) > 200 {
		for debounceKey, last := range ms.roleUpdateAuditDebounce {
			if now.Sub(last) > 5*time.Minute {
				delete(ms.roleUpdateAuditDebounce, debounceKey)
			}
		}
	}
	return false
}

func isRecentRoleUpdateAuditEntry(entry *discordgo.AuditLogEntry) bool {
	if entry == nil || entry.ID == "" {
		return true
	}
	entryTime, ok := snowflakeTimestamp(entry.ID)
	if !ok {
		return true
	}
	return time.Since(entryTime) <= monitoringRoleAuditEntryMaxAge
}

func extractAuditRolePartials(v interface{}) []auditRolePartial {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]auditRolePartial, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		role := auditRolePartial{}
		if value, ok := obj["id"].(string); ok {
			role.ID = value
		}
		if value, ok := obj["name"].(string); ok {
			role.Name = value
		}
		if role.ID != "" || role.Name != "" {
			out = append(out, role)
		}
	}
	return out
}

func extractAuditRoleDelta(entry *discordgo.AuditLogEntry) (added []auditRolePartial, removed []auditRolePartial) {
	if entry == nil {
		return nil, nil
	}
	for _, change := range entry.Changes {
		if change == nil || change.Key == nil {
			continue
		}
		switch *change.Key {
		case discordgo.AuditLogChangeKeyRoleAdd:
			added = append(added, extractAuditRolePartials(change.NewValue)...)
			added = append(added, extractAuditRolePartials(change.OldValue)...)
		case discordgo.AuditLogChangeKeyRoleRemove:
			removed = append(removed, extractAuditRolePartials(change.NewValue)...)
			removed = append(removed, extractAuditRolePartials(change.OldValue)...)
		}
	}
	return added, removed
}

func toIDSet(ids []string) map[string]struct{} {
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			set[id] = struct{}{}
		}
	}
	return set
}

func buildAuditRoleList(list []auditRolePartial) string {
	if len(list) == 0 {
		return "None"
	}
	out := ""
	for i, role := range list {
		display := ""
		if role.ID != "" {
			display = "<@&" + role.ID + ">"
		}
		if display == "" && role.Name != "" {
			display = "`" + role.Name + "`"
		}
		if display == "" && role.ID != "" {
			display = "`" + role.ID + "`"
		}
		if i > 0 {
			out += ", "
		}
		out += display
	}
	return out
}

func buildRoleIDList(list []string) string {
	if len(list) == 0 {
		return "None"
	}
	out := ""
	for i, id := range list {
		if i > 0 {
			out += ", "
		}
		out += "<@&" + id + ">"
	}
	return out
}

func (ms *MonitoringService) sendRoleUpdateNotification(channelID string, user *discordgo.User, actorID string, added string, removed string, source string) error {
	if user == nil {
		return fmt.Errorf("role update user is nil")
	}
	targetLabel := formatUserLabel(user.Username, user.ID)
	actorLabel := formatUserRef(actorID)
	desc := fmt.Sprintf("**Target:** %s\n**Actor:** %s", targetLabel, actorLabel)
	embed := &discordgo.MessageEmbed{
		Title:       "Roles Updated",
		Color:       theme.MemberRoleUpdate(),
		Description: desc,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Added",
				Value:  added,
				Inline: true,
			},
			{
				Name:   "Removed",
				Value:  removed,
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: source,
		},
	}

	atomic.AddUint64(&ms.apiMessagesSent, 1)
	_, err := ms.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// handleMemberUpdate processes member updates.
func (ms *MonitoringService) handleMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_member_update.monitoring",
		slog.String("guildID", m.GuildID),
		slog.String("userID", m.User.ID),
	)
	defer done()

	gcfg := ms.configManager.GuildConfig(m.GuildID)
	if gcfg == nil {
		return
	}

	ms.applyStatsMemberUpdate(m.GuildID, m.User.ID, m.User.Bot, m.Roles)

	// Avatar change logging (already in place)
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)

	emit := ShouldEmitLogEvent(ms.session, ms.configManager, LogEventRoleChange, m.GuildID)
	if !emit.Enabled {
		log.ApplicationLogger().Debug("Role update notification suppressed by policy", "guildID", m.GuildID, "userID", m.User.ID, "reason", emit.Reason)
		return
	}
	channelID := emit.ChannelID

	curRoles, verifiedAdded, verifiedRemoved, known := ms.computeMemberRoleDiff(m.GuildID, m.User.ID, m.Roles)
	if !known {
		log.ApplicationLogger().Debug(
			"Role update skipped because current role state could not be resolved",
			"guildID", m.GuildID,
			"userID", m.User.ID,
		)
		return
	}

	if len(verifiedAdded) == 0 && len(verifiedRemoved) == 0 {
		if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
			log.ApplicationLogger().Warn(
				"Failed to persist role snapshot after empty local diff",
				"guildID", m.GuildID,
				"userID", m.User.ID,
				"roleCount", len(curRoles),
				"err", err,
			)
		}
		return
	}

	tryNotifyFromEntries := func(entries []*discordgo.AuditLogEntry) bool {
		verifiedAddedSet := toIDSet(verifiedAdded)
		verifiedRemovedSet := toIDSet(verifiedRemoved)

		for _, entry := range entries {
			if entry == nil || entry.TargetID != m.User.ID || !isRecentRoleUpdateAuditEntry(entry) {
				continue
			}

			auditAdded, auditRemoved := extractAuditRoleDelta(entry)
			if len(auditAdded) == 0 && len(auditRemoved) == 0 {
				continue
			}

			filteredAdded := make([]auditRolePartial, 0, len(auditAdded))
			for _, role := range auditAdded {
				if _, ok := verifiedAddedSet[role.ID]; ok {
					filteredAdded = append(filteredAdded, role)
				}
			}
			filteredRemoved := make([]auditRolePartial, 0, len(auditRemoved))
			for _, role := range auditRemoved {
				if _, ok := verifiedRemovedSet[role.ID]; ok {
					filteredRemoved = append(filteredRemoved, role)
				}
			}

			if len(filteredAdded) == 0 && len(filteredRemoved) == 0 {
				log.ApplicationLogger().Debug(
					"Role update skipped after verification produced empty delta",
					"guildID", m.GuildID,
					"userID", m.User.ID,
					"auditAddedCount", len(auditAdded),
					"auditRemovedCount", len(auditRemoved),
					"verifiedAddedCount", len(verifiedAdded),
					"verifiedRemovedCount", len(verifiedRemoved),
				)
				if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
					log.ApplicationLogger().Warn(
						"Failed to persist role snapshot after verification skip",
						"guildID", m.GuildID,
						"userID", m.User.ID,
						"roleCount", len(curRoles),
						"err", err,
					)
				}
				return true
			}

			if err := ms.sendRoleUpdateNotification(
				channelID,
				m.User,
				entry.UserID,
				buildAuditRoleList(filteredAdded),
				buildAuditRoleList(filteredRemoved),
				"Source: Audit Log",
			); err != nil {
				log.ErrorLoggerRaw().Error("Failed to send role update notification", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID, "err", err)
			} else {
				log.ApplicationLogger().Info("Role update notification sent successfully", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID)
				if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
					log.ApplicationLogger().Warn(
						"Failed to persist role snapshot after role update notification",
						"guildID", m.GuildID,
						"userID", m.User.ID,
						"roleCount", len(curRoles),
						"err", err,
					)
				}
			}
			return true
		}
		return false
	}

	auditLookupDebounced := ms.shouldDebounceRoleUpdateAuditRefresh(m.GuildID, m.User.ID)
	entries, fromCache, err := ms.getRoleUpdateAuditEntries(m.GuildID, false)
	if err != nil {
		log.ApplicationLogger().Warn("Failed to fetch audit logs for role update", "guildID", m.GuildID, "userID", m.User.ID, "err", err)
	} else if tryNotifyFromEntries(entries) {
		return
	}

	if fromCache && !auditLookupDebounced {
		time.Sleep(monitoringRoleAuditRetryDelay)
		refreshedEntries, _, refreshErr := ms.getRoleUpdateAuditEntries(m.GuildID, true)
		if refreshErr != nil {
			log.ApplicationLogger().Warn("Failed to refresh audit logs for role update", "guildID", m.GuildID, "userID", m.User.ID, "err", refreshErr)
		} else if tryNotifyFromEntries(refreshedEntries) {
			return
		}
	}

	if err := ms.sendRoleUpdateNotification(
		channelID,
		m.User,
		"",
		buildRoleIDList(verifiedAdded),
		buildRoleIDList(verifiedRemoved),
		"Source: Role Diff",
	); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send fallback role update notification", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID, "err", err)
		return
	}

	log.ApplicationLogger().Info("Fallback role update notification sent successfully", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID)
	if err := ms.persistMemberRoleSnapshot(m.GuildID, m.User.ID, curRoles); err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist role snapshot after fallback role update notification",
			"guildID", m.GuildID,
			"userID", m.User.ID,
			"roleCount", len(curRoles),
			"err", err,
		)
	}

}

// handleUserUpdate processes user updates across all configured guilds.
func (ms *MonitoringService) handleUserUpdate(s *discordgo.Session, m *discordgo.UserUpdate) {
	if m == nil || m.User == nil {
		return
	}

	done := perf.StartGatewayEvent(
		"user_update",
		slog.String("userID", m.User.ID),
	)
	defer done()

	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}
	for _, gcfg := range cfg.Guilds {
		var member *discordgo.Member
		// Use unified cache
		if m2, err := ms.getGuildMember(gcfg.GuildID, m.User.ID); err == nil {
			member = m2
		}
		if member == nil || member.User == nil {
			continue
		}
		ms.checkAvatarChange(gcfg.GuildID, member.User.ID, member.User.Avatar, member.User.Username)
	}
}

// checkAvatarChange aplica debounce e delega processamento ao UserWatcher.
func (ms *MonitoringService) checkAvatarChange(guildID, userID, currentAvatar, username string) {
	if currentAvatar == "" {
		currentAvatar = "default"
	}
	changeKey := fmt.Sprintf("%s:%s:%s", guildID, userID, currentAvatar)
	ms.changesMutex.RLock()
	if lastChange, exists := ms.recentChanges[changeKey]; exists {
		if time.Since(lastChange) < 65*time.Second {
			ms.changesMutex.RUnlock()
			return
		}
	}
	ms.changesMutex.RUnlock()

	// Initial check using cache to avoid unnecessary enqueuing
	changed := true
	if ms.unifiedCache != nil {
		if member, ok := ms.unifiedCache.GetMember(guildID, userID); ok {
			if member.User != nil && member.User.Avatar == currentAvatar {
				// No change according to cache; skip unless it's a known stale case
				changed = false
			}
		}
	}

	if changed {
		ms.changesMutex.Lock()
		ms.recentChanges[changeKey] = time.Now()
		// Clean up only occasionally to avoid CPU overhead on every event
		if len(ms.recentChanges) > 100 {
			for key, timestamp := range ms.recentChanges {
				if time.Since(timestamp) > 5*time.Minute {
					delete(ms.recentChanges, key)
				}
			}
		}
		ms.changesMutex.Unlock()

		if ms.adapters != nil {
			if err := ms.adapters.EnqueueProcessAvatarChange(guildID, userID, username, currentAvatar); err != nil {
				if err.Error() == "duplicate task (idempotency key present)" {
					log.ApplicationLogger().Info("Avatar change task already enqueued (idempotency)", "guildID", guildID, "userID", userID)
				} else {
					log.ErrorLoggerRaw().Error("Failed to enqueue avatar change task; falling back to synchronous processing", "guildID", guildID, "userID", userID, "err", err)
					ms.userWatcher.ProcessChange(guildID, userID, currentAvatar, username)
				}
			}
		} else {
			ms.userWatcher.ProcessChange(guildID, userID, currentAvatar, username)
		}
	}
}

// ProcessChange performs avatar-specific logic: notification and persistence.
// It also verifies if the change is actual by comparing with the database to avoid redundant notifications.
func (aw *UserWatcher) ProcessChange(guildID, userID, currentAvatar, username string) {
	// Synchronous DB check to verify if the change is actual and fetch old avatar
	oldAvatar, _, ok, err := aw.store.GetAvatar(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to fetch current avatar from store", "guildID", guildID, "userID", userID, "err", err)
		// We continue anyway; if it's a real change, UpsertAvatar will handle it or fail later.
	}

	if ok && oldAvatar == currentAvatar {
		// Change was already processed or is redundant
		return
	}

	finalUsername := username
	if finalUsername == "" {
		finalUsername = aw.getUsernameForNotification(guildID, userID)
	}

	change := files.AvatarChange{
		UserID:    userID,
		Username:  finalUsername,
		OldAvatar: oldAvatar,
		NewAvatar: currentAvatar,
		Timestamp: time.Now(),
	}

	log.ApplicationLogger().Info("Avatar change detected and processing", "userID", userID, "guildID", guildID, "old_avatar", oldAvatar, "new_avatar", currentAvatar)

	emit := ShouldEmitLogEvent(aw.session, aw.configManager, LogEventAvatarChange, guildID)
	if !emit.Enabled {
		if emit.Reason == EmitReasonNoChannelConfigured {
			log.ErrorLoggerRaw().Error("User activity log channel not configured; notification not sent", "guildID", guildID)
		} else {
			log.ApplicationLogger().Debug("Avatar notification suppressed by policy", "guildID", guildID, "userID", userID, "reason", emit.Reason)
		}
	} else {
		if err := aw.notifier.SendAvatarChangeNotification(emit.ChannelID, change); err != nil {
			log.ErrorLoggerRaw().Error("Error sending notification", "channelID", emit.ChannelID, "userID", userID, "guildID", guildID, "err", err)
		} else {
			log.ApplicationLogger().Info("Avatar notification sent successfully", "channelID", emit.ChannelID, "userID", userID, "guildID", guildID)
		}
	}

	if _, _, err := aw.store.UpsertAvatar(guildID, userID, currentAvatar, time.Now()); err != nil {
		log.ErrorLoggerRaw().Error("Error saving avatar in store for guild", "guildID", guildID, "err", err)
	}
}

func (aw *UserWatcher) getUsernameForNotification(guildID, userID string) string {
	// Try unified cache first
	if aw.cache != nil {
		if member, ok := aw.cache.GetMember(guildID, userID); ok {
			if member.Nick != "" {
				return member.Nick
			}
			if member.User != nil && member.User.Username != "" {
				return member.User.Username
			}
		}
	}

	// Prefer session state cache to avoid REST calls
	if aw.session != nil && aw.session.State != nil {
		if m, _ := aw.session.State.Member(guildID, userID); m != nil {
			if aw.cache != nil {
				aw.cache.SetMember(guildID, userID, m)
			}
			if m.Nick != "" {
				return m.Nick
			}
			if m.User != nil && m.User.Username != "" {
				return m.User.Username
			}
		}
	}

	// Fallback: REST fetch
	member, err := aw.session.GuildMember(guildID, userID)
	if err != nil || member == nil {
		log.ApplicationLogger().Info("Error getting member for username; using ID", "userID", userID, "guildID", guildID, "err", err)
		return userID
	}

	if aw.cache != nil {
		aw.cache.SetMember(guildID, userID, member)
	}

	if member.Nick != "" {
		return member.Nick
	}
	if member.User != nil && member.User.Username != "" {
		return member.User.Username
	}
	return userID
}

func (ms *MonitoringService) markEvent(ctx context.Context) {
	if ms.activity == nil {
		return
	}
	ms.activity.MarkEvent(ctx, "monitoring_service")
}

func (ms *MonitoringService) startHeartbeat(ctx context.Context) {
	if ms.activity == nil {
		return
	}
	ms.activity.StartHeartbeat(ctx, heartbeatTickInterval)
}

func (ms *MonitoringService) stopHeartbeat(ctx context.Context) error {
	if ms.activity == nil {
		return nil
	}
	return ms.activity.StopHeartbeat(ctx)
}

// rolesCacheCleanupLoop periodically removes expired entries from rolesCache
func (ms *MonitoringService) rolesCacheCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.cleanupRolesCache()
		case <-ctx.Done():
			return
		case <-ms.rolesCacheCleanup:
			return
		}
	}
}

// cleanupRolesCache removes expired entries from rolesCache map
func (ms *MonitoringService) cleanupRolesCache() {
	now := time.Now()
	var toDelete []string

	ms.rolesCacheMu.RLock()
	for key, entry := range ms.rolesCache {
		if now.After(entry.expiresAt) {
			toDelete = append(toDelete, key)
		}
	}
	ms.rolesCacheMu.RUnlock()

	if len(toDelete) > 0 {
		ms.rolesCacheMu.Lock()
		for _, key := range toDelete {
			delete(ms.rolesCache, key)
		}
		ms.rolesCacheMu.Unlock()
		log.ApplicationLogger().Info("Cleaned up expired roles cache entries", "count", len(toDelete))
	}
}

func (ms *MonitoringService) cacheRolesSet(guildID, userID string, roles []string) {
	key := guildID + ":" + userID
	if len(roles) == 0 {
		// Empty snapshot means "no tracked roles"; drop any stale cache entry.
		ms.rolesCacheMu.Lock()
		delete(ms.rolesCache, key)
		ms.rolesCacheMu.Unlock()
		return
	}
	// TTL: prefer guild-configured value, fallback to service default (5m)
	ttl := ms.rolesTTL
	if ms.configManager != nil {
		if gcfg := ms.configManager.GuildConfig(guildID); gcfg != nil {
			if d := gcfg.RolesCacheTTLDuration(); d > 0 {
				ttl = d
			}
		}
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	ms.rolesCacheMu.Lock()
	ms.rolesCache[key] = cachedRoles{
		roles:     append([]string(nil), roles...),
		expiresAt: time.Now().Add(ttl),
	}
	ms.rolesCacheMu.Unlock()
}

func (ms *MonitoringService) cacheRolesGet(guildID, userID string) ([]string, bool) {
	key := guildID + ":" + userID
	ms.rolesCacheMu.RLock()
	entry, ok := ms.rolesCache[key]
	ms.rolesCacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			ms.rolesCacheMu.Lock()
			delete(ms.rolesCache, key)
			ms.rolesCacheMu.Unlock()
		}
		return nil, false
	}
	out := append([]string(nil), entry.roles...)
	return out, true
}

func (ms *MonitoringService) GetCacheStats() map[string]interface{} {
	ms.rolesCacheMu.RLock()
	size := len(ms.rolesCache)
	ms.rolesCacheMu.RUnlock()
	ms.roleUpdateAuditMu.Lock()
	roleAuditCacheSize := len(ms.roleUpdateAuditCache)
	roleAuditDebounceSize := len(ms.roleUpdateAuditDebounce)
	ms.roleUpdateAuditMu.Unlock()
	ttl := ms.rolesTTL
	isRunning := ms.IsRunning()

	stats := map[string]interface{}{
		"isRunning":                   isRunning,
		"rolesCacheSize":              size,
		"rolesCacheTTLSeconds":        int(ttl.Seconds()),
		"roleUpdateAuditCacheSize":    roleAuditCacheSize,
		"roleUpdateAuditDebounceSize": roleAuditDebounceSize,
		"apiAuditLogCalls":            atomic.LoadUint64(&ms.apiAuditLogCalls),
		"apiGuildMemberCalls":         atomic.LoadUint64(&ms.apiGuildMemberCalls),
		"apiMessagesSent":             atomic.LoadUint64(&ms.apiMessagesSent),
		"cacheStateMemberHits":        atomic.LoadUint64(&ms.cacheStateMemberHits),
		"cacheRolesMemoryHits":        atomic.LoadUint64(&ms.cacheRolesMemoryHits),
		"cacheRolesStoreHits":         atomic.LoadUint64(&ms.cacheRolesStoreHits),
		"cacheRoleAuditHits":          atomic.LoadUint64(&ms.cacheRoleAuditHits),
	}

	// Add unified cache stats
	if ms.unifiedCache != nil {
		ucStats := ms.unifiedCache.GetStats()
		// Prefer generic unified cache stats (primary)
		stats["unifiedCache"] = ucStats

		// Keep specific stats for backward compatibility using CustomMetrics
		var memberEntries, guildEntries, rolesEntries, channelEntries int
		var memberHits, memberMisses, guildHits, guildMisses, rolesHits, rolesMisses, channelHits, channelMisses uint64

		if ucStats.CustomMetrics != nil {
			if v, ok := ucStats.CustomMetrics["memberEntries"]; ok {
				switch t := v.(type) {
				case int:
					memberEntries = t
				case int64:
					memberEntries = int(t)
				case float64:
					memberEntries = int(t)
				}
			}
			if v, ok := ucStats.CustomMetrics["guildEntries"]; ok {
				switch t := v.(type) {
				case int:
					guildEntries = t
				case int64:
					guildEntries = int(t)
				case float64:
					guildEntries = int(t)
				}
			}
			if v, ok := ucStats.CustomMetrics["rolesEntries"]; ok {
				switch t := v.(type) {
				case int:
					rolesEntries = t
				case int64:
					rolesEntries = int(t)
				case float64:
					rolesEntries = int(t)
				}
			}
			if v, ok := ucStats.CustomMetrics["channelEntries"]; ok {
				switch t := v.(type) {
				case int:
					channelEntries = t
				case int64:
					channelEntries = int(t)
				case float64:
					channelEntries = int(t)
				}
			}

			if v, ok := ucStats.CustomMetrics["memberHits"]; ok {
				switch t := v.(type) {
				case uint64:
					memberHits = t
				case int:
					if t >= 0 {
						memberHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						memberHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						memberHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["memberMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					memberMisses = t
				case int:
					if t >= 0 {
						memberMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						memberMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						memberMisses = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["guildHits"]; ok {
				switch t := v.(type) {
				case uint64:
					guildHits = t
				case int:
					if t >= 0 {
						guildHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						guildHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						guildHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["guildMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					guildMisses = t
				case int:
					if t >= 0 {
						guildMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						guildMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						guildMisses = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["rolesHits"]; ok {
				switch t := v.(type) {
				case uint64:
					rolesHits = t
				case int:
					if t >= 0 {
						rolesHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						rolesHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						rolesHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["rolesMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					rolesMisses = t
				case int:
					if t >= 0 {
						rolesMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						rolesMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						rolesMisses = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["channelHits"]; ok {
				switch t := v.(type) {
				case uint64:
					channelHits = t
				case int:
					if t >= 0 {
						channelHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						channelHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						channelHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["channelMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					channelMisses = t
				case int:
					if t >= 0 {
						channelMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						channelMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						channelMisses = uint64(t)
					}
				}
			}
		}

		stats["unifiedCacheSpecific"] = map[string]interface{}{
			"memberEntries":  memberEntries,
			"guildEntries":   guildEntries,
			"rolesEntries":   rolesEntries,
			"channelEntries": channelEntries,
			"memberHits":     memberHits,
			"memberMisses":   memberMisses,
			"guildHits":      guildHits,
			"guildMisses":    guildMisses,
			"rolesHits":      rolesHits,
			"rolesMisses":    rolesMisses,
			"channelHits":    channelHits,
			"channelMisses":  channelMisses,
		}
	}

	return stats
}
func (ms *MonitoringService) handleStartupDowntimeAndMaybeRefresh(ctx context.Context) error {
	if ms.store == nil {
		return nil
	}
	type heartbeatState struct {
		at time.Time
		ok bool
	}
	hb, err := monitoringRunWithTimeout(ctx, monitoringPersistenceTimeout, func() (heartbeatState, error) {
		at, ok, err := ms.getHeartbeat()
		return heartbeatState{at: at, ok: ok}, err
	})
	lastHB := hb.at
	okHB := hb.ok
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to read last heartbeat; skipping downtime check", "err", err)
	} else {
		if !okHB || time.Since(lastHB) > downtimeThreshold {
			downtimeDuration := "unknown"
			if okHB {
				downtimeDuration = time.Since(lastHB).Round(time.Second).String()
			}
			log.ApplicationLogger().Info("⏱️ Detected downtime; performing silent avatar refresh before enabling notifications", "downtime", downtimeDuration, "threshold", downtimeThreshold.String())
			cfg := ms.scopedConfig()
			if cfg == nil || len(cfg.Guilds) == 0 {
				log.ApplicationLogger().Info("No configured guilds for startup silent refresh")
				return nil
			}
			startTime := time.Now()
			guildIDs := make([]string, 0, len(cfg.Guilds))
			for _, gcfg := range cfg.Guilds {
				if gid := strings.TrimSpace(gcfg.GuildID); gid != "" {
					guildIDs = append(guildIDs, gid)
				}
			}
			if err := runGuildTasksWithLimit(ctx, guildIDs, monitoringMaxConcurrentGuildScan, func(runCtx context.Context, guildID string) error {
				return ms.initializeGuildCacheContext(runCtx, guildID)
			}); err != nil {
				return err
			}
			log.ApplicationLogger().Info("✅ Silent avatar refresh completed", "duration", time.Since(startTime).Round(time.Millisecond))
			return nil
		}
	}
	log.ApplicationLogger().Info("No significant downtime detected; skipping heavy avatar refresh")
	return nil
}

type guildMemberPageFetcher func(ctx context.Context, guildID, after string, limit int) ([]*discordgo.Member, error)

func paginateGuildMembersContext(
	ctx context.Context,
	guildID string,
	pageSize int,
	fetch guildMemberPageFetcher,
	handle func([]*discordgo.Member) error,
) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if pageSize <= 0 {
		pageSize = monitoringGuildMembersPageSize
	}
	if fetch == nil {
		return 0, fmt.Errorf("guild member fetcher is nil")
	}

	total := 0
	after := ""
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		members, err := fetch(ctx, guildID, after, pageSize)
		if err != nil {
			return total, err
		}
		if len(members) == 0 {
			return total, nil
		}
		if handle != nil {
			if err := handle(members); err != nil {
				return total, err
			}
		}
		total += len(members)
		if len(members) < pageSize {
			return total, nil
		}
		last := members[len(members)-1]
		if last == nil || last.User == nil || strings.TrimSpace(last.User.ID) == "" {
			return total, fmt.Errorf("paginate guild members: invalid page tail for guild %s", guildID)
		}
		after = last.User.ID
	}
}

func (ms *MonitoringService) fetchGuildMemberPageContext(ctx context.Context, guildID, after string, limit int) ([]*discordgo.Member, error) {
	if ms == nil || ms.session == nil {
		return nil, fmt.Errorf("discord session is unavailable")
	}
	if limit <= 0 {
		limit = monitoringGuildMembersPageSize
	}
	return monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() ([]*discordgo.Member, error) {
		return ms.session.GuildMembers(guildID, after, limit)
	})
}

func (ms *MonitoringService) forEachGuildMemberPageContext(ctx context.Context, guildID string, handle func([]*discordgo.Member) error) (int, error) {
	total, err := paginateGuildMembersContext(ctx, guildID, monitoringGuildMembersPageSize, ms.fetchGuildMemberPageContext, handle)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to paginate guild members", "guildID", guildID, "fetched_so_far", total, "err", err)
		return total, err
	}
	log.ApplicationLogger().Info("Pagination completed successfully", "guildID", guildID, "total_members_fetched", total)
	return total, nil
}

// fetchAllGuildMembers paginates through all guild members until exhaustion and materializes them in memory.
func (ms *MonitoringService) fetchAllGuildMembers(guildID string) ([]*discordgo.Member, error) {
	return ms.fetchAllGuildMembersContext(context.Background(), guildID)
}

func (ms *MonitoringService) fetchAllGuildMembersContext(ctx context.Context, guildID string) ([]*discordgo.Member, error) {
	all := make([]*discordgo.Member, 0)
	_, err := ms.forEachGuildMemberPageContext(ctx, guildID, func(members []*discordgo.Member) error {
		all = append(all, members...)
		return nil
	})
	if err != nil {
		return all, err
	}
	return all, nil
}

func (ms *MonitoringService) performPeriodicCheck(ctx context.Context) error {
	log.ApplicationLogger().Info("Running periodic avatar check...")
	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.ApplicationLogger().Info("No configured guilds for periodic check")
		return nil
	}
	for _, gcfg := range cfg.Guilds {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := ms.forEachGuildMemberPageContext(ctx, gcfg.GuildID, func(members []*discordgo.Member) error {
			joinSnapshots := make([]storage.GuildMemberSnapshot, 0, len(members))
			for _, member := range members {
				if err := ctx.Err(); err != nil {
					return err
				}
				if member == nil || member.User == nil {
					continue
				}
				if ms.store != nil && !member.JoinedAt.IsZero() {
					joinSnapshots = append(joinSnapshots, storage.GuildMemberSnapshot{
						UserID:   member.User.ID,
						JoinedAt: member.JoinedAt,
						IsBot:    member.User.Bot,
						HasBot:   true,
					})
				}
			}
			if ms.store != nil && len(joinSnapshots) > 0 {
				if err := ms.store.UpsertGuildMemberSnapshotsContext(ctx, gcfg.GuildID, joinSnapshots, time.Now().UTC()); err != nil {
					log.ApplicationLogger().Warn(
						"Periodic check: failed to backfill member join page",
						"operation", "monitoring.periodic_check.persist_joins_page",
						"guildID", gcfg.GuildID,
						"members", len(joinSnapshots),
						"err", err,
					)
				}
			}

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
				ms.checkAvatarChange(gcfg.GuildID, member.User.ID, avatarHash, member.User.Username)
			}
			return nil
		})
		if err != nil {
			log.ErrorLoggerRaw().Error("Error getting members for guild", "guildID", gcfg.GuildID, "err", err)
			continue
		}
	}
	return nil
}

func runGuildTasksWithLimit(ctx context.Context, guildIDs []string, limit int, fn func(context.Context, string) error) error {
	if fn == nil || len(guildIDs) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 1
	}

	sem := make(chan struct{}, limit)
	errCh := make(chan error, len(guildIDs))
	var wg sync.WaitGroup

	for _, guildID := range guildIDs {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(gid string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(ctx, gid); err != nil {
				errCh <- err
			}
		}(guildID)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return ctx.Err()
}

// MemberEvents exposes the member event sub-service.
func (ms *MonitoringService) MemberEvents() *MemberEventService {
	return ms.memberEventService
}

// MessageEvents exposes the message event sub-service.
func (ms *MonitoringService) MessageEvents() *MessageEventService {
	return ms.messageEventService
}

// Notifier exposes the notification sender used by monitoring.
func (ms *MonitoringService) Notifier() *NotificationSender {
	return ms.notifier
}

// CacheManager exposes the avatar cache manager used by monitoring.
func (ms *MonitoringService) Store() *storage.Store {
	return ms.store
}

// GetUnifiedCache exposes the unified cache for use by other components
func (ms *MonitoringService) GetUnifiedCache() *cache.UnifiedCache {
	return ms.unifiedCache
}

func (ms *MonitoringService) TaskRouter() *task.TaskRouter {
	return ms.router
}

func (ms *MonitoringService) botRolePermSnapshotKey(guildID, roleID string) string {
	if guildID == "" || roleID == "" {
		return ""
	}
	return persistentCacheKeyPrefixBotRolePermSnapshot + guildID + ":" + roleID
}

func (ms *MonitoringService) botPermMirrorEnabled(guildID string) bool {
	// Enabled by default (safety feature).
	// Previously gated via ALICE_DISABLE_BOT_ROLE_PERM_MIRROR env var; now read from persisted runtime_config.
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		cfg := scopedCfg
		rc := cfg.ResolveRuntimeConfig(guildID)
		features := cfg.ResolveFeatures(guildID)
		if !features.Safety.BotRolePermMirror {
			return false
		}
		return !rc.DisableBotRolePermMirror
	}
	return true
}

func (ms *MonitoringService) botPermMirrorActorRoleID(guildID string) string {
	// Previously overridable via ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID env var; now read from persisted runtime_config.
	if scopedCfg := ms.scopedConfig(); scopedCfg != nil {
		rc := scopedCfg.ResolveRuntimeConfig(guildID)
		v := strings.TrimSpace(rc.BotRolePermMirrorActorRoleID)
		if v != "" {
			return v
		}
	}
	return defaultBotPermMirrorActorRoleID
}

func (ms *MonitoringService) findGuildRole(guildID string, match func(*discordgo.Role) bool) (*discordgo.Role, bool) {
	if guildID == "" || ms.session == nil || match == nil {
		return nil, false
	}
	roles, err := ms.session.GuildRoles(guildID)
	if err != nil {
		return nil, false
	}
	for _, role := range roles {
		if role == nil {
			continue
		}
		if match(role) {
			return role, true
		}
	}
	return nil, false
}

func (ms *MonitoringService) isBotManagedRole(guildID, roleID string) bool {
	if roleID == "" {
		return false
	}
	role, ok := ms.findGuildRole(guildID, func(r *discordgo.Role) bool {
		return r.ID == roleID
	})
	return ok && role.Managed
}

func (ms *MonitoringService) getRoleByID(guildID, roleID string) (*discordgo.Role, bool) {
	if roleID == "" {
		return nil, false
	}
	return ms.findGuildRole(guildID, func(r *discordgo.Role) bool {
		return r.ID == roleID
	})
}

func (ms *MonitoringService) findBotManagedRole(guildID string) (*discordgo.Role, bool) {
	return ms.findGuildRole(guildID, func(r *discordgo.Role) bool {
		return r.Managed
	})
}

func (ms *MonitoringService) saveBotRolePermSnapshot(guildID, roleID string, prevPerm int64, actorUserID string) {
	if ms.store == nil || guildID == "" || roleID == "" {
		return
	}
	snap := botRolePermSnapshot{
		GuildID:         guildID,
		RoleID:          roleID,
		PrevPermissions: prevPerm,
		SavedAt:         time.Now().UTC(),
		SavedByUserID:   actorUserID,
	}
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	// Keep snapshot for a long time; it is safe and small.
	expiresAt := time.Now().UTC().Add(365 * 24 * time.Hour)
	_ = ms.store.UpsertCacheEntry(ms.botRolePermSnapshotKey(guildID, roleID), persistentCacheTypeBotRolePermSnapshot, string(b), expiresAt)
}

func (ms *MonitoringService) getBotRolePermSnapshot(guildID, roleID string) (*botRolePermSnapshot, bool) {
	if ms.store == nil {
		return nil, false
	}
	key := ms.botRolePermSnapshotKey(guildID, roleID)
	if key == "" {
		return nil, false
	}
	tp, data, _, ok, err := ms.store.GetCacheEntry(key)
	if err != nil || !ok || tp != persistentCacheTypeBotRolePermSnapshot || strings.TrimSpace(data) == "" {
		return nil, false
	}
	var snap botRolePermSnapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return nil, false
	}
	if snap.GuildID == "" || snap.RoleID == "" {
		return nil, false
	}
	return &snap, true
}

func (ms *MonitoringService) maybeRestoreBotRolePermissions(guildID, roleID string, newPerm int64) {
	// If we have a stored snapshot and the role seems to have been reset, restore.
	snap, ok := ms.getBotRolePermSnapshot(guildID, roleID)
	if !ok || snap == nil {
		return
	}
	// If current perms already match the snapshot, nothing to do.
	if newPerm == snap.PrevPermissions {
		return
	}

	// Restore only if this is a likely reset scenario:
	// - The role is managed (bot/integration role)
	// - Current perms are "smaller" than snapshot (common after reset)
	if !ms.isBotManagedRole(guildID, roleID) {
		return
	}
	if newPerm > snap.PrevPermissions {
		// don't "downgrade" if somehow perms increased
		return
	}

	if ms.session == nil {
		return
	}
	perm := snap.PrevPermissions
	if _, err := ms.session.GuildRoleEdit(guildID, roleID, &discordgo.RoleParams{
		Permissions: &perm,
	}); err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to restore bot managed role permissions from snapshot",
			"guildID", guildID,
			"roleID", roleID,
			"targetPermissions", perm,
			"err", err,
		)
	}
}

func (ms *MonitoringService) handleRoleCreateForBotPermMirroring(s *discordgo.Session, e *discordgo.GuildRoleCreate) {
	if e == nil || e.Role == nil || e.GuildID == "" {
		return
	}
	if !ms.handlesGuild(e.GuildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_role_create",
		slog.String("guildID", e.GuildID),
		slog.String("roleID", e.Role.ID),
		slog.Bool("managed", e.Role.Managed),
	)
	defer done()

	if !ms.botPermMirrorEnabled(e.GuildID) {
		return
	}
	// When a managed role is (re)created (common after bot add/re-add), try to restore.
	if !e.Role.Managed {
		return
	}
	ms.maybeRestoreBotRolePermissions(e.GuildID, e.Role.ID, e.Role.Permissions)
}

func (ms *MonitoringService) handleRoleUpdateForBotPermMirroring(s *discordgo.Session, e *discordgo.GuildRoleUpdate) {
	if e == nil || e.Role == nil || e.GuildID == "" {
		return
	}
	if !ms.handlesGuild(e.GuildID) {
		return
	}

	done := perf.StartGatewayEvent(
		"guild_role_update",
		slog.String("guildID", e.GuildID),
		slog.String("roleID", e.Role.ID),
		slog.Bool("managed", e.Role.Managed),
	)
	defer done()

	if !ms.botPermMirrorEnabled(e.GuildID) {
		return
	}

	// Only care about managed roles (bot/integration roles)
	if !e.Role.Managed {
		return
	}

	// Try to locate an audit log entry to understand who did it and snapshot previous perms
	// when the actor has the privileged role.
	actionType := int(discordgo.AuditLogActionRoleUpdate)
	audit, err := ms.session.GuildAuditLog(e.GuildID, "", "", actionType, 10)
	atomic.AddUint64(&ms.apiAuditLogCalls, 1)
	if err == nil && audit != nil {
		for _, entry := range audit.AuditLogEntries {
			if entry == nil || entry.ActionType == nil {
				continue
			}
			if *entry.ActionType != discordgo.AuditLogActionRoleUpdate {
				continue
			}
			// Role update entries target the role id
			if entry.TargetID != e.Role.ID {
				continue
			}

			actorID := entry.UserID
			if strings.TrimSpace(actorID) == "" {
				break
			}

			// If actor lacks the mirroring role, do not snapshot; still allow restore path below.
			actor, err := ms.getGuildMember(e.GuildID, actorID)
			if err != nil || actor == nil {
				break
			}
			hasActorRole := false
			requiredRoleID := ms.botPermMirrorActorRoleID(e.GuildID)
			if requiredRoleID != "" {
				for _, rid := range actor.Roles {
					if rid == requiredRoleID {
						hasActorRole = true
						break
					}
				}
			}
			if !hasActorRole {
				break
			}

			// Find the previous permissions from the audit log "changes".
			var oldPerm *int64
			for _, ch := range entry.Changes {
				if ch == nil || ch.Key == nil {
					continue
				}
				if *ch.Key != "permissions" {
					continue
				}
				// OldValue/NewValue can be string or float64 depending on decoding.
				switch v := ch.OldValue.(type) {
				case string:
					if p, err := strconv.ParseInt(v, 10, 64); err == nil {
						oldPerm = &p
					}
				case float64:
					p := int64(v)
					oldPerm = &p
				case int64:
					p := v
					oldPerm = &p
				case int:
					p := int64(v)
					oldPerm = &p
				}
			}
			if oldPerm != nil {
				ms.saveBotRolePermSnapshot(e.GuildID, e.Role.ID, *oldPerm, actorID)
			}
			break
		}
	}

	// If bot role permissions were reset (e.g., bot kicked/re-added), restore from snapshot.
	ms.maybeRestoreBotRolePermissions(e.GuildID, e.Role.ID, e.Role.Permissions)
}

// Helper methods for cached API calls

// getGuildMember retrieves a member using unified cache -> state -> API fallback
func (ms *MonitoringService) getGuildMember(guildID, userID string) (*discordgo.Member, error) {
	return ms.getGuildMemberContext(context.Background(), guildID, userID)
}

func (ms *MonitoringService) getGuildMemberContext(ctx context.Context, guildID, userID string) (*discordgo.Member, error) {
	// Try unified cache first
	if ms.unifiedCache != nil {
		if member, ok := ms.unifiedCache.GetMember(guildID, userID); ok {
			return member, nil
		}
	}

	// Try state cache
	if ms.session != nil && ms.session.State != nil {
		if member, err := ms.session.State.Member(guildID, userID); err == nil && member != nil {
			atomic.AddUint64(&ms.cacheStateMemberHits, 1)
			if ms.unifiedCache != nil {
				ms.unifiedCache.SetMember(guildID, userID, member)
			}
			return member, nil
		}
	}

	// Fallback to API
	atomic.AddUint64(&ms.apiGuildMemberCalls, 1)
	member, err := monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Member, error) {
		return ms.session.GuildMember(guildID, userID)
	})
	if err != nil {
		return nil, err
	}

	if ms.unifiedCache != nil {
		ms.unifiedCache.SetMember(guildID, userID, member)
	}
	return member, nil
}

// getGuild retrieves a guild using unified cache -> state -> API fallback
func (ms *MonitoringService) getGuild(guildID string) (*discordgo.Guild, error) {
	return ms.getGuildContext(context.Background(), guildID)
}

func (ms *MonitoringService) getGuildContext(ctx context.Context, guildID string) (*discordgo.Guild, error) {
	// Try unified cache first
	if ms.unifiedCache != nil {
		if guild, ok := ms.unifiedCache.GetGuild(guildID); ok {
			return guild, nil
		}
	}

	// Try state cache
	if ms.session != nil && ms.session.State != nil {
		if guild, err := ms.session.State.Guild(guildID); err == nil && guild != nil {
			if ms.unifiedCache != nil {
				ms.unifiedCache.SetGuild(guildID, guild)
			}
			return guild, nil
		}
	}

	// Fallback to API
	guild, err := monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Guild, error) {
		return ms.session.Guild(guildID)
	})
	if err != nil {
		return nil, err
	}

	if ms.unifiedCache != nil {
		ms.unifiedCache.SetGuild(guildID, guild)
	}
	return guild, nil
}
