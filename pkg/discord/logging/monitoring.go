package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

var mentionRe = regexp.MustCompile(`<@!?(\d+)>`)

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
)

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
	store                *storage.Store
	notifier             *NotificationSender
	adapters             *task.NotificationAdapters
	router               *task.TaskRouter
	userWatcher          *UserWatcher
	memberEventService   *MemberEventService   // Service for member events
	messageEventService  *MessageEventService  // Service for message events
	reactionEventService *ReactionEventService // Service for reaction metrics
	isRunning            bool
	stopChan             chan struct{}
	stopOnce             sync.Once
	runMu                sync.Mutex
	recentChanges        map[string]time.Time // Debounce to avoid duplicates
	changesMutex         sync.RWMutex
	cronCancel           func()

	// Heartbeat runtime tracking
	heartbeatTicker *time.Ticker
	heartbeatStop   chan struct{}

	// Unified cache for Discord API data (members, guilds, roles, channels)
	unifiedCache *cache.UnifiedCache

	// In-memory roles cache with TTL to reduce REST/DB lookups
	rolesCache        map[string]cachedRoles
	rolesCacheMu      sync.RWMutex
	rolesTTL          time.Duration
	rolesCacheCleanup chan struct{}

	// Event handler references for cleanup
	eventHandlers []interface{}

	// Metrics counters
	apiAuditLogCalls     uint64
	apiGuildMemberCalls  uint64
	apiMessagesSent      uint64
	cacheStateMemberHits uint64
	cacheRolesMemoryHits uint64
	cacheRolesStoreHits  uint64
}

type cachedRoles struct {
	roles     []string
	expiresAt time.Time
}

// NewMonitoringService creates the multi-guild monitoring service. Returns error if any dependency is nil.
func NewMonitoringService(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store) (*MonitoringService, error) {
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
	router := task.NewRouter(task.Defaults())
	adapters := task.NewNotificationAdapters(router, session, configManager, nil, n)

	// Create unified cache with persistence enabled
	cacheConfig := cache.DefaultCacheConfig()
	cacheConfig.Store = store
	cacheConfig.PersistEnabled = true
	unifiedCache := cache.NewUnifiedCache(cacheConfig)

	ms := &MonitoringService{
		session:             session,
		configManager:       configManager,
		store:               store,
		notifier:            n,
		unifiedCache:        unifiedCache,
		userWatcher:         NewUserWatcher(session, configManager, store, n, unifiedCache),
		memberEventService:  NewMemberEventService(session, configManager, n, store),
		messageEventService: NewMessageEventService(session, configManager, n, store),
		adapters:            adapters,
		router:              router,
		stopChan:            make(chan struct{}),
		recentChanges:       make(map[string]time.Time),
		rolesCache:          make(map[string]cachedRoles),
		rolesTTL:            5 * time.Minute,
		rolesCacheCleanup:   make(chan struct{}),
		eventHandlers:       make([]interface{}, 0),
	}
	// Wire task adapters into sub-services
	ms.memberEventService.SetAdapters(adapters)
	ms.messageEventService.SetAdapters(adapters)
	return ms, nil
}

// Start starts the monitoring service. Returns error if already running.
func (ms *MonitoringService) Start() error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	if ms.isRunning {
		log.ErrorLoggerRaw().Error("Monitoring service is already running")
		return fmt.Errorf("monitoring service is already running")
	}
	ms.isRunning = true
	// Recreate stopChan and reset stopOnce for restart
	ms.stopChan = make(chan struct{})
	ms.stopOnce = sync.Once{}

	// Unified cache warmup is performed in app runner; skipping here to prevent duplicate work

	ms.ensureGuildsListed()
	// Detect downtime and refresh avatars silently before wiring handlers (no notifications)
	ms.handleStartupDowntimeAndMaybeRefresh()
	ms.setupEventHandlers()
	// Start periodic heartbeat tracker (persisted)
	ms.startHeartbeat()
	// Start periodic roles cache cleanup
	ms.rolesCacheCleanup = make(chan struct{})
	go ms.rolesCacheCleanupLoop()

	// Global check for services
	globalRC := files.RuntimeConfig{}
	if ms.configManager != nil && ms.configManager.Config() != nil {
		globalRC = ms.configManager.Config().RuntimeConfig
	}

	// Start member/message services (gate entry/exit logs via runtime config)
	// Note: these services are currently global, so we use global config for startup.
	// Per-guild toggles would need these services to be guild-aware or filtered.
	disableEntryExit := globalRC.DisableEntryExitLogs
	if disableEntryExit {
		log.ApplicationLogger().Info("🛑 Entry/exit logs disabled by global runtime config; MemberEventService will not start")
	} else {
		if err := ms.memberEventService.Start(); err != nil {
			ms.isRunning = false
			return fmt.Errorf("failed to start member event service: %w", err)
		}
	}
	// Optionally honor DisableAutomodLogs here (Automod service is started elsewhere)
	if globalRC.DisableAutomodLogs {
		log.ApplicationLogger().Info("🛑 Automod logs disabled by global runtime config")
	}

	// Gate message logging behind runtime config
	if globalRC.DisableMessageLogs {
		log.ApplicationLogger().Info("🛑 Message logging disabled by global runtime config; MessageEventService will not start")
	} else {
		if err := ms.messageEventService.Start(); err != nil {
			ms.isRunning = false
			// Stop the member event service if start failed
			ms.memberEventService.Stop()
			return fmt.Errorf("failed to start message event service: %w", err)
		}
	}

	// Gate reaction logging behind runtime config
	if globalRC.DisableReactionLogs {
		log.ApplicationLogger().Info("🛑 Reaction logging disabled by global runtime config; ReactionEventService will not start")
	} else {
		// Lazily initialize service if not yet created
		if ms.reactionEventService == nil {
			ms.reactionEventService = NewReactionEventService(ms.session, ms.configManager, ms.store)
		}
		if err := ms.reactionEventService.Start(); err != nil {
			ms.isRunning = false
			// Stop previously started services on failure
			if ms.messageEventService != nil && ms.messageEventService.IsRunning() {
				_ = ms.messageEventService.Stop()
			}
			if ms.memberEventService != nil && ms.memberEventService.IsRunning() {
				_ = ms.memberEventService.Stop()
			}
			return fmt.Errorf("failed to start reaction event service: %w", err)
		}
	}

	// Schedule periodic avatar scan via router cron instead of local goroutine
	ms.router.RegisterHandler("monitor.scan_avatars", func(ctx context.Context, _ any) error {
		ms.performPeriodicCheck()
		return nil
	})

	// Register a daily roles DB refresh task and run once at startup
	ms.router.RegisterHandler("monitor.refresh_roles", func(ctx context.Context, _ any) error {
		cfg := ms.configManager.Config()
		if cfg == nil || len(cfg.Guilds) == 0 || ms.store == nil {
			return nil
		}
		start := time.Now()
		totalUpdates := 0
		for _, gcfg := range cfg.Guilds {
			members, err := ms.fetchAllGuildMembers(gcfg.GuildID)
			if err != nil {
				log.ErrorLoggerRaw().Error("Error refreshing roles for guild", "guildID", gcfg.GuildID, "err", err)
				continue
			}
			for _, member := range members {
				if len(member.Roles) == 0 {
					continue
				}
				if err := ms.store.UpsertMemberRoles(gcfg.GuildID, member.User.ID, member.Roles, time.Now()); err != nil {
					log.ApplicationLogger().Warn("Failed to upsert roles for user in guild", "userID", member.User.ID, "guildID", gcfg.GuildID, "err", err)
					continue
				}
				ms.cacheRolesSet(gcfg.GuildID, member.User.ID, member.Roles)
				totalUpdates++
			}
		}
		// Reconcile target role based on local DB data after the refresh
		reconciledAdds := 0
		reconciledRemoves := 0
		if ms.store != nil && ms.session != nil {
			for _, gcfg := range cfg.Guilds {
				// Skip guilds without auto-role assignment enabled or missing config
				if !gcfg.AutoRoleAssignmentEnabled || gcfg.AutoRoleTargetRoleID == "" || gcfg.AutoRolePrereqRoleA == "" || gcfg.AutoRolePrereqRoleB == "" {
					continue
				}
				memberRoles, err := ms.store.GetAllGuildMemberRoles(gcfg.GuildID)
				if err != nil {
					log.ApplicationLogger().Warn("Failed to load member roles from DB for reconciliation", "guildID", gcfg.GuildID, "err", err)
					continue
				}
				for userID, roles := range memberRoles {
					hasA, hasB, hasTarget := false, false, false
					for _, r := range roles {
						if r == gcfg.AutoRolePrereqRoleA {
							hasA = true
						} else if r == gcfg.AutoRolePrereqRoleB {
							hasB = true
						} else if r == gcfg.AutoRoleTargetRoleID {
							hasTarget = true
						}
					}
					// Grant target role if both prerequisites are present and target is missing
					if hasA && hasB && !hasTarget {
						if err := ms.session.GuildMemberRoleAdd(gcfg.GuildID, userID, gcfg.AutoRoleTargetRoleID); err != nil {
							log.ApplicationLogger().Warn("Failed to grant target role during reconciliation", "guildID", gcfg.GuildID, "userID", userID, "roleID", gcfg.AutoRoleTargetRoleID, "err", err)
						} else {
							reconciledAdds++
						}
					}
					// Remove target role if prerequisite A is missing
					if hasTarget && !hasA {
						if err := ms.session.GuildMemberRoleRemove(gcfg.GuildID, userID, gcfg.AutoRoleTargetRoleID); err != nil {
							log.ApplicationLogger().Warn("Failed to remove target role during reconciliation", "guildID", gcfg.GuildID, "userID", userID, "roleID", gcfg.AutoRoleTargetRoleID, "err", err)
						} else {
							reconciledRemoves++
						}
					}
				}
			}
		}
		log.ApplicationLogger().Info("✅ Roles DB refresh completed", "members_updated", totalUpdates, "duration", time.Since(start).Round(time.Second), "reconciled_adds", reconciledAdds, "reconciled_removes", reconciledRemoves)
		return nil
	})

	// Using TaskRouter scheduler helpers for daily scheduling
	// Schedule periodic jobs
	ms.cronCancel = ms.router.ScheduleEvery(2*time.Hour, task.Task{Type: "monitor.scan_avatars"})
	// Schedule daily roles refresh at 03:00 UTC
	ms.router.ScheduleDailyAtUTC(3, 0, task.Task{Type: "monitor.refresh_roles"})

	// Trigger one-time roles refresh on startup (non-blocking)
	go func() {
		_ = ms.router.Dispatch(context.Background(), task.Task{Type: "monitor.refresh_roles"})
	}()

	// Register one-shot entry/exit backfill handler (Option A)
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
			msgs, err := ms.session.ChannelMessages(channelID, 100, before, "", "")
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
							_ = ms.store.UpsertMemberJoin(guildID, userID, t)
							_ = ms.store.IncrementDailyMemberJoin(guildID, userID, t)
						} else if evt == "leave" {
							// If name was not in message, check if still in server via code
							stillInServer := false
							if ms.session != nil {
								mem, err := ms.session.GuildMember(guildID, userID)
								if err == nil && mem != nil {
									stillInServer = true
								}
							}

							if !stillInServer {
								_ = ms.store.IncrementDailyMemberLeave(guildID, userID, t)
							}
						}
						// Record the oldest processed timestamp for this channel
						_ = ms.store.SetMetadata("backfill_progress:"+channelID, t)
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
		// Payload is expected to be: struct{ ChannelID, Start, End string }
		// Start/End format: RFC3339 (UTC recommended)
		type pld struct {
			ChannelID string
			Start     string
			End       string
		}
		p, _ := payload.(pld)
		channelID := strings.TrimSpace(p.ChannelID)
		startRaw := strings.TrimSpace(p.Start)
		endRaw := strings.TrimSpace(p.End)
		if channelID == "" || startRaw == "" || endRaw == "" {
			return nil
		}

		start, err := time.Parse(time.RFC3339, startRaw)
		if err != nil {
			return nil
		}
		end, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return nil
		}
		start = start.UTC()
		end = end.UTC()
		if !end.After(start) {
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
			msgs, err := ms.session.ChannelMessages(channelID, 100, before, "", "")
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
							_ = ms.store.UpsertMemberJoin(guildID, userID, t)
							_ = ms.store.IncrementDailyMemberJoin(guildID, userID, t)
						} else if evt == "leave" {
							// If name was not in message, check if still in server via code
							stillInServer := false
							if ms.session != nil {
								mem, err := ms.session.GuildMember(guildID, userID)
								if err == nil && mem != nil {
									stillInServer = true
								}
							}

							if !stillInServer {
								_ = ms.store.IncrementDailyMemberLeave(guildID, userID, t)
							}
						}
						// Record the oldest processed timestamp for this channel
						_ = ms.store.SetMetadata("backfill_progress:"+channelID, t)
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
	if ms.configManager != nil && ms.configManager.Config() != nil {
		cfg := ms.configManager.Config()
		globalRC := cfg.RuntimeConfig
		initialDate := strings.TrimSpace(globalRC.BackfillInitialDate)

		// Get all potential channels and their resolved configs
		type backfillTarget struct {
			ChannelID string
			RC        files.RuntimeConfig
		}
		targets := make([]backfillTarget, 0)

		// Global target if configured
		if globalRC.BackfillChannelID != "" {
			targets = append(targets, backfillTarget{
				ChannelID: strings.TrimSpace(globalRC.BackfillChannelID),
				RC:        globalRC,
			})
		}

		// Guild targets
		for _, g := range cfg.Guilds {
			cid := strings.TrimSpace(g.WelcomeBacklogChannelID)
			if cid == "" {
				cid = strings.TrimSpace(g.UserEntryLeaveChannelID)
			}
			if cid != "" {
				targets = append(targets, backfillTarget{
					ChannelID: cid,
					RC:        cfg.ResolveRuntimeConfig(g.GuildID),
				})
			}
		}

		if len(targets) == 0 {
			log.ApplicationLogger().Debug("No target channels for backfill check")
		} else {
			lastEvent, hasLastEvent, _ := ms.store.GetLastEvent()
			now := time.Now().UTC()

			for _, target := range targets {
				cid := target.ChannelID
				rc := target.RC
				day := strings.TrimSpace(rc.BackfillStartDay)

				if day != "" {
					_ = ms.router.Dispatch(context.Background(), task.Task{
						Type:    "monitor.backfill_entry_exit_day",
						Payload: struct{ ChannelID, Day string }{ChannelID: cid, Day: day},
						Options: task.TaskOptions{GroupKey: "backfill:" + cid},
					})
					log.ApplicationLogger().Info("▶️ Dispatched entry/exit backfill task (day)", "channelID", cid, "day", day)
					continue
				}

				// If no specific day, check for initial scan or recovery
				// initialDate is GLOBAL ONLY per requirements
				if initialDate == "" {
					log.ApplicationLogger().Debug("Backfill skip for channel: no day set and global initial_date is empty", "channelID", cid)
					continue
				}

				// Check progress for this channel
				_, hasProgress, _ := ms.store.GetMetadata("backfill_progress:" + cid)

				if !hasProgress {
					// Use initialDate to calculate start date
					parsedDate, err := time.Parse("2006-01-02", initialDate)
					if err != nil {
						log.ApplicationLogger().Error("Failed to parse backfill_initial_date", "date", initialDate, "err", err)
						continue
					}
					start := parsedDate.Format(time.RFC3339)
					end := now.Format(time.RFC3339)
					_ = ms.router.Dispatch(context.Background(), task.Task{
						Type:    "monitor.backfill_entry_exit_range",
						Payload: struct{ ChannelID, Start, End string }{ChannelID: cid, Start: start, End: end},
						Options: task.TaskOptions{GroupKey: "backfill:" + cid},
					})
					log.ApplicationLogger().Info("▶️ Dispatched initial entry/exit backfill (range)", "channelID", cid, "start", start)
					continue
				}

				// If we have progress, check if we need downtime recovery
				if hasLastEvent {
					downtime := now.Sub(lastEvent)
					if downtime > downtimeThreshold {
						start := lastEvent.UTC().Format(time.RFC3339)
						end := now.Format(time.RFC3339)
						_ = ms.router.Dispatch(context.Background(), task.Task{
							Type:    "monitor.backfill_entry_exit_range",
							Payload: struct{ ChannelID, Start, End string }{ChannelID: cid, Start: start, End: end},
							Options: task.TaskOptions{GroupKey: "backfill:" + cid},
						})
						log.ApplicationLogger().Info("▶️ Dispatched entry/exit backfill recovery (range)", "channelID", cid, "start", start, "end", end)
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

	log.ApplicationLogger().Info("All monitoring services started successfully")
	return nil
}

// Stop stops the monitoring service. Returns error if not running.
func (ms *MonitoringService) Stop() error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	if !ms.isRunning {
		log.ErrorLoggerRaw().Error("Monitoring service is not running")
		return fmt.Errorf("monitoring service is not running")
	}
	ms.isRunning = false
	// Use sync.Once to prevent double-closing stopChan
	ms.stopOnce.Do(func() {
		close(ms.stopChan)
	})
	ms.stopHeartbeat()

	// Persist cache before shutdown
	if ms.unifiedCache != nil {
		log.ApplicationLogger().Info("💾 Persisting cache to storage...")
		if err := ms.unifiedCache.Persist(); err != nil {
			log.ErrorLoggerRaw().Error("Failed to persist cache (continuing)", "err", err)
		} else {
			members, _, _, _ := ms.unifiedCache.MemberMetrics()
			guilds, _, _, _ := ms.unifiedCache.GuildMetrics()
			roles, _, _, _ := ms.unifiedCache.RolesMetrics()
			channels, _, _, _ := ms.unifiedCache.ChannelMetrics()
			total := members + guilds + roles + channels
			log.ApplicationLogger().Info("✅ Cache persisted", "entries_saved", total)
		}
		// Stop cache cleanup goroutine
		ms.unifiedCache.Stop()
	}

	// Stop roles cache cleanup
	if ms.rolesCacheCleanup != nil {
		close(ms.rolesCacheCleanup)
		ms.rolesCacheCleanup = nil
	}

	// Remove event handlers
	ms.removeEventHandlers()

	// Stop services
	if err := ms.memberEventService.Stop(); err != nil {
		log.ErrorLoggerRaw().Error("Error stopping member event service", "err", err)
	}
	if ms.messageEventService != nil && ms.messageEventService.IsRunning() {
		if err := ms.messageEventService.Stop(); err != nil {
			log.ErrorLoggerRaw().Error("Error stopping message event service", "err", err)
		}
	}
	if ms.reactionEventService != nil && ms.reactionEventService.IsRunning() {
		if err := ms.reactionEventService.Stop(); err != nil {
			log.ErrorLoggerRaw().Error("Error stopping reaction event service", "err", err)
		}
	}

	// Cancel cron before closing router
	if ms.cronCancel != nil {
		ms.cronCancel()
	}

	if ms.router != nil {
		ms.router.Close()
	}
	log.ApplicationLogger().Info("Monitoring service stopped")
	return nil
}

// initializeCache loads the current member users for all configured guilds.
func (ms *MonitoringService) initializeCache() {
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.ApplicationLogger().Info("No guild configured for monitoring")
		return
	}
	var wg sync.WaitGroup
	ms.markEvent()
	for _, gcfg := range cfg.Guilds {
		gid := gcfg.GuildID
		wg.Add(1)
		go func(guildID string) {
			defer wg.Done()
			ms.initializeGuildCache(guildID)
		}(gid)
	}
	wg.Wait()
	// No-op: avatars are persisted per change in the SQLite store
}

// initializeGuildCache initializes the current avatars of members in a specific guild.
func (ms *MonitoringService) initializeGuildCache(guildID string) {
	if ms.store == nil {
		log.ApplicationLogger().Warn("Store is nil; skipping cache initialization for guild", "guildID", guildID)
		return
	}

	// Use unified cache for guild fetch
	guild, err := ms.getGuild(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Error getting guild", "guildID", guildID, "err", err)
		return
	}
	log.ApplicationLogger().Info("Initializing cache for guild", "guildName", guild.Name, "guildID", guild.ID)
	_ = ms.store.SetGuildOwnerID(guildID, guild.OwnerID)

	// Set bot join time if missing
	if _, ok, _ := ms.store.GetBotSince(guildID); !ok {
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
			if m, err := ms.getGuildMember(guildID, botID); err == nil {
				botMember = m
			}
		}
		if botMember != nil && !botMember.JoinedAt.IsZero() {
			_ = ms.store.SetBotSince(guildID, botMember.JoinedAt)
		} else {
			_ = ms.store.SetBotSince(guildID, time.Now())
		}
	}
	members, err := ms.fetchAllGuildMembers(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Error getting members for guild", "guildID", guildID, "err", err)
		return
	}
	for _, member := range members {
		avatarHash := member.User.Avatar
		if avatarHash == "" {
			avatarHash = "default"
		}
		_, _, _ = ms.store.UpsertAvatar(guildID, member.User.ID, avatarHash, time.Now())
		// Persist roles snapshot for the member to enable efficient role diffing later
		if ms.store != nil && len(member.Roles) > 0 {
			_ = ms.store.UpsertMemberRoles(guildID, member.User.ID, member.Roles, time.Now())
			ms.cacheRolesSet(guildID, member.User.ID, member.Roles)
		}

		// Backfill missing member join date using Discord data
		if ms.store != nil && !member.JoinedAt.IsZero() {
			if _, ok, _ := ms.store.GetMemberJoin(guildID, member.User.ID); !ok {
				_ = ms.store.UpsertMemberJoin(guildID, member.User.ID, member.JoinedAt)
			}
		}
	}
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

	// Entry/Exit logs -> MemberEventService
	if rc.DisableEntryExitLogs {
		if ms.memberEventService != nil && ms.memberEventService.IsRunning() {
			_ = ms.memberEventService.Stop()
		}
	} else {
		if ms.memberEventService != nil && !ms.memberEventService.IsRunning() {
			if err := ms.memberEventService.Start(); err != nil {
				return fmt.Errorf("start MemberEventService: %w", err)
			}
		}
	}

	// Message logs -> MessageEventService
	if rc.DisableMessageLogs {
		if ms.messageEventService != nil && ms.messageEventService.IsRunning() {
			_ = ms.messageEventService.Stop()
		}
	} else {
		if ms.messageEventService != nil && !ms.messageEventService.IsRunning() {
			if err := ms.messageEventService.Start(); err != nil {
				return fmt.Errorf("start MessageEventService: %w", err)
			}
		}
	}

	// Reaction logs -> ReactionEventService
	if rc.DisableReactionLogs {
		if ms.reactionEventService != nil && ms.reactionEventService.IsRunning() {
			_ = ms.reactionEventService.Stop()
		}
	} else {
		if ms.reactionEventService == nil {
			ms.reactionEventService = NewReactionEventService(ms.session, ms.configManager, ms.store)
		}
		if !ms.reactionEventService.IsRunning() {
			if err := ms.reactionEventService.Start(); err != nil {
				return fmt.Errorf("start ReactionEventService: %w", err)
			}
		}
	}

	// User logs -> re-register handlers (presence/member/user updates)
	ms.removeEventHandlers()
	ms.setupEventHandlersFromRuntimeConfig(rc)

	_ = ctx
	return nil
}

// setupEventHandlers registra handlers do Discord.
func (ms *MonitoringService) setupEventHandlers() {
	// Delegate to config-driven version (keeps behavior in one spot).
	rc := files.RuntimeConfig{}
	if ms.configManager != nil && ms.configManager.Config() != nil {
		rc = ms.configManager.Config().RuntimeConfig
	}
	ms.setupEventHandlersFromRuntimeConfig(rc)
}

// setupEventHandlersFromRuntimeConfig registers handlers based on the provided runtime config.
// This is used both at startup and for hot-apply.
func (ms *MonitoringService) setupEventHandlersFromRuntimeConfig(rc files.RuntimeConfig) {
	// Store handler references for later removal
	// Gate user logs (avatars and roles) via runtime config
	disableUser := rc.DisableUserLogs

	if disableUser {
		// Register only non-user handlers
		ms.eventHandlers = append(ms.eventHandlers,
			ms.session.AddHandler(ms.handleGuildCreate),
			ms.session.AddHandler(ms.handleGuildUpdate),
		)
		log.ApplicationLogger().Info("🛑 User logs disabled by runtime config ALICE_DISABLE_USER_LOGS; avatar/role handlers not registered")
	} else {
		ms.eventHandlers = append(ms.eventHandlers,
			ms.session.AddHandler(ms.handlePresenceUpdate),
			ms.session.AddHandler(ms.handleMemberUpdate),
			ms.session.AddHandler(ms.handleUserUpdate),
			ms.session.AddHandler(ms.handleGuildCreate),
			ms.session.AddHandler(ms.handleGuildUpdate),
		)
	}

	// Always keep an eye on role updates so bot-role permission resets can be detected.
	// This is independent from ALICE_DISABLE_USER_LOGS (it can be a safety mechanism).
	ms.eventHandlers = append(ms.eventHandlers,
		ms.session.AddHandler(ms.handleRoleUpdateForBotPermMirroring),
		ms.session.AddHandler(ms.handleRoleCreateForBotPermMirroring),
	)
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
			if err := ms.configManager.AddGuildConfig(files.GuildConfig{GuildID: g.ID}); err != nil {
				log.ErrorLoggerRaw().Error("Error adding minimal guild entry for guild", "guildID", g.ID, "err", err)
				continue
			}
			if err := ms.configManager.SaveConfig(); err != nil {
				log.ErrorLoggerRaw().Error("Error saving config after minimal guild add for guild", "guildID", g.ID, "err", err)
			} else {
				log.ApplicationLogger().Info("📘 Guild listed in config (minimal entry) for guild", "guildID", g.ID)
			}
		}
	}
}

func (ms *MonitoringService) handleGuildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	guildID := e.ID
	if guildID == "" {
		return
	}

	if ms.configManager.GuildConfig(guildID) == nil {
		// New guild: add to config and initialize cache
		if err := ms.configManager.RegisterGuild(s, guildID); err != nil {
			log.ErrorLoggerRaw().Error("Falling back to minimal guild entry for guild", "guildID", guildID, "err", err)
			if err2 := ms.configManager.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err2 != nil {
				log.ErrorLoggerRaw().Error("Error adding minimal guild entry for guild", "guildID", guildID, "err", err2)
				return
			}
		}
		if err := ms.configManager.SaveConfig(); err != nil {
			log.ErrorLoggerRaw().Error("Error saving config after guild add for guild", "guildID", guildID, "err", err)
		}
		log.ApplicationLogger().Info("🆕 New guild listed in config for guild", "guildID", guildID)
		ms.initializeGuildCache(guildID)
		// No-op: avatars persisted per change in SQLite store
	}
}

// handleGuildUpdate updates the OwnerID cache when the server ownership changes.
func (ms *MonitoringService) handleGuildUpdate(s *discordgo.Session, e *discordgo.GuildUpdate) {
	if e == nil || e.Guild == nil || e.Guild.ID == "" {
		return
	}
	if ms.store != nil {
		if prev, ok, _ := ms.store.GetGuildOwnerID(e.Guild.ID); ok && prev != e.Guild.OwnerID {
			log.ApplicationLogger().Info("Guild owner changed", "guildID", e.Guild.ID, "from", prev, "to", e.Guild.OwnerID)
		}
		_ = ms.store.SetGuildOwnerID(e.Guild.ID, e.Guild.OwnerID)
	}
}

// handlePresenceUpdate processes presence updates (includes avatar).
func (ms *MonitoringService) handlePresenceUpdate(s *discordgo.Session, m *discordgo.PresenceUpdate) {
	if m.User == nil {
		return
	}
	if ms.configManager.GuildConfig(m.GuildID) == nil {
		return
	}
	if m.User.Username == "" {
		log.ApplicationLogger().Debug("PresenceUpdate ignored (empty username)", "userID", m.User.ID, "guildID", m.GuildID)
		return
	}
	ms.markEvent()
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)
}

// handleMemberUpdate processes member updates.
func (ms *MonitoringService) handleMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.User == nil {
		return
	}
	gcfg := ms.configManager.GuildConfig(m.GuildID)
	if gcfg == nil {
		return
	}

	// Avatar change logging (already in place)
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)

	// Role update logging (via Audit Log)
	channelID := gcfg.UserLogChannelID
	if channelID == "" {
		channelID = gcfg.CommandChannelID
	}
	if channelID == "" {
		return
	}

	// Fetch role update audit log using constant with a short retry
	actionType := int(discordgo.AuditLogActionMemberRoleUpdate)

	// Helper to compute a verified diff between the local snapshot (memory/SQLite) and the current Discord state.
	// Also returns the current roles considered for snapshot update.
	computeVerifiedDiff := func(guildID, userID string, proposed []string) (cur []string, added []string, removed []string) {
		// 1) determine current state from the proposed (event) or from Discord
		cur = proposed
		if len(cur) == 0 {
			if member, err := ms.getGuildMember(guildID, userID); err == nil && member != nil {
				cur = member.Roles
			}
		}
		if len(cur) == 0 {
			return cur, nil, nil
		}

		// 2) get previous state (prefer in-memory TTL cache; fallback SQLite)
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

		// 3) compute diffs
		curSet := make(map[string]struct{}, len(cur))
		for _, r := range cur {
			if r != "" {
				curSet[r] = struct{}{}
			}
		}
		prevSet := make(map[string]struct{}, len(prev))
		for _, r := range prev {
			if r != "" {
				prevSet[r] = struct{}{}
			}
		}
		for r := range curSet {
			if _, ok := prevSet[r]; !ok {
				added = append(added, r)
			}
		}
		for r := range prevSet {
			if _, ok := curSet[r]; !ok {
				removed = append(removed, r)
			}
		}
		return cur, added, removed
	}

	tryFetchAndNotify := func() (sent bool) {
		audit, err := ms.session.GuildAuditLog(m.GuildID, "", "", actionType, 10)
		atomic.AddUint64(&ms.apiAuditLogCalls, 1)
		if err != nil || audit == nil {
			log.ApplicationLogger().Warn("Failed to fetch audit logs for role update", "guildID", m.GuildID, "userID", m.User.ID, "err", err)
			return false
		}

		for _, entry := range audit.AuditLogEntries {
			if entry == nil || entry.ActionType == nil || *entry.ActionType != discordgo.AuditLogActionMemberRoleUpdate || entry.TargetID != m.User.ID {
				continue
			}
			actorID := entry.UserID

			// Recency check of the entry (via Snowflake ID -> timestamp)
			recentThreshold := 2 * time.Minute
			if entry.ID != "" {
				if sid, err := strconv.ParseUint(entry.ID, 10, 64); err == nil {
					const discordEpoch = int64(1420070400000) // 2015-01-01 UTC em ms
					tsMillis := int64(sid>>22) + discordEpoch
					entryTime := time.Unix(0, tsMillis*int64(time.Millisecond))
					if time.Since(entryTime) > recentThreshold {
						continue
					}
				}
			}

			type rolePartial struct {
				ID   string
				Name string
			}
			extractRoles := func(v interface{}) []rolePartial {
				arr, ok := v.([]interface{})
				if !ok {
					return nil
				}
				out := make([]rolePartial, 0, len(arr))
				for _, it := range arr {
					if obj, ok := it.(map[string]interface{}); ok {
						r := rolePartial{}
						if vv, ok := obj["id"].(string); ok {
							r.ID = vv
						}
						if vv, ok := obj["name"].(string); ok {
							r.Name = vv
						}
						if r.ID != "" || r.Name != "" {
							out = append(out, r)
						}
					}
				}
				return out
			}

			added := []rolePartial{}
			removed := []rolePartial{}

			for _, ch := range entry.Changes {
				if ch == nil || ch.Key == nil {
					continue
				}
				switch *ch.Key {
				case discordgo.AuditLogChangeKeyRoleAdd:
					// consider NewValue and OldValue for robustness
					added = append(added, extractRoles(ch.NewValue)...)
					added = append(added, extractRoles(ch.OldValue)...)
				case discordgo.AuditLogChangeKeyRoleRemove:
					removed = append(removed, extractRoles(ch.NewValue)...)
					removed = append(removed, extractRoles(ch.OldValue)...)
				}
			}

			if len(added) == 0 && len(removed) == 0 {
				// No relevant changes detected in this entry; continue scanning
				continue
			}

			buildList := func(list []rolePartial) string {
				if len(list) == 0 {
					return "None"
				}
				out := ""
				for i, r := range list {
					display := ""
					if r.ID != "" {
						display = "<@&" + r.ID + ">"
					}
					if display == "" && r.Name != "" {
						display = "`" + r.Name + "`"
					}
					if display == "" && r.ID != "" {
						display = "`" + r.ID + "`"
					}
					if i > 0 {
						out += ", "
					}
					out += display
				}
				return out
			}

			// Verify with Discord + DB which changes were actually applied
			curRoles, verifiedAdded, verifiedRemoved := computeVerifiedDiff(m.GuildID, m.User.ID, m.Roles)

			toSet := func(ids []string) map[string]struct{} {
				s := make(map[string]struct{}, len(ids))
				for _, id := range ids {
					if id != "" {
						s[id] = struct{}{}
					}
				}
				return s
			}
			verifiedAddedSet := toSet(verifiedAdded)
			verifiedRemovedSet := toSet(verifiedRemoved)

			// Filter only the roles that were actually added/removed according to the current state
			filteredAdded := make([]rolePartial, 0, len(added))
			for _, r := range added {
				if r.ID != "" {
					if _, ok := verifiedAddedSet[r.ID]; ok {
						filteredAdded = append(filteredAdded, r)
					}
				}
			}
			filteredRemoved := make([]rolePartial, 0, len(removed))
			for _, r := range removed {
				if r.ID != "" {
					if _, ok := verifiedRemovedSet[r.ID]; ok {
						filteredRemoved = append(filteredRemoved, r)
					}
				}
			}

			// If nothing remains after verification, do not send an embed
			if len(filteredAdded) == 0 && len(filteredRemoved) == 0 {
				// Update the snapshot anyway to keep the DB consistent
				if ms.store != nil && len(curRoles) > 0 {
					_ = ms.store.UpsertMemberRoles(m.GuildID, m.User.ID, curRoles, time.Now())
					ms.cacheRolesSet(m.GuildID, m.User.ID, curRoles)
				}
				// Continue scanning other possible entries
				continue
			}

			desc := fmt.Sprintf("<@%s> updated roles for **%s** (<@%s>, `%s`)", actorID, m.User.Username, m.User.ID, m.User.ID)
			embed := &discordgo.MessageEmbed{
				Title:       "Roles updated",
				Color:       0x3498db,
				Description: desc,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "Added",
						Value:  buildList(filteredAdded),
						Inline: true,
					},
					{
						Name:   "Removed",
						Value:  buildList(filteredRemoved),
						Inline: true,
					},
				},
				Timestamp: time.Now().Format(time.RFC3339),
			}

			atomic.AddUint64(&ms.apiMessagesSent, 1)
			if _, sendErr := ms.session.ChannelMessageSendEmbed(channelID, embed); sendErr != nil {
				log.ErrorLoggerRaw().Error("Failed to send role update notification", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID, "err", sendErr)
			} else {
				log.ApplicationLogger().Info("Role update notification sent successfully", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID)
				// Update the snapshot to reflect the state after the change
				if ms.store != nil && len(curRoles) > 0 {
					_ = ms.store.UpsertMemberRoles(m.GuildID, m.User.ID, curRoles, time.Now())
					ms.cacheRolesSet(m.GuildID, m.User.ID, curRoles)
				}
			}

			// Consider only the latest relevant entry
			return true
		}
		return false
	}

	// Primeira tentativa
	if tryFetchAndNotify() {
		return
	}
	// Retentativa curta
	time.Sleep(300 * time.Millisecond)
	if tryFetchAndNotify() {
		return
	}
	// Fallback by role diff when the audit log produced no result
	if ms.store != nil {
		curRoles := m.Roles
		if len(curRoles) == 0 {
			if member, err := ms.getGuildMember(m.GuildID, m.User.ID); err == nil && member != nil {
				curRoles = member.Roles
			}
		}
		if len(curRoles) > 0 {
			var addedIDs, removedIDs []string
			_, addedIDs, removedIDs = computeVerifiedDiff(m.GuildID, m.User.ID, curRoles)

			if len(addedIDs) > 0 || len(removedIDs) > 0 {
				buildListIDs := func(list []string) string {
					if len(list) == 0 {
						return "None"
					}
					out := ""
					for i, id := range list {
						display := ""
						if id != "" {
							display = "<@&" + id + ">"
						}
						if i > 0 {
							out += ", "
						}
						out += display
					}
					return out
				}
				desc := fmt.Sprintf("Role changes detected for **%s** (<@%s>, `%s`)", m.User.Username, m.User.ID, m.User.ID)
				embed := &discordgo.MessageEmbed{
					Title:       "Roles updated (fallback)",
					Color:       0x3498db,
					Description: desc,
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Added",
							Value:  buildListIDs(addedIDs),
							Inline: true,
						},
						{
							Name:   "Removed",
							Value:  buildListIDs(removedIDs),
							Inline: true,
						},
					},
					Timestamp: time.Now().Format(time.RFC3339),
				}
				if _, sendErr := ms.session.ChannelMessageSendEmbed(channelID, embed); sendErr != nil {
					log.ErrorLoggerRaw().Error("Failed to send fallback role update notification", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID, "err", sendErr)
				} else {
					log.ApplicationLogger().Info("Fallback role update notification sent successfully", "guildID", m.GuildID, "userID", m.User.ID, "channelID", channelID)
					// Update roles snapshot after sending
					if ms.store != nil {
						_ = ms.store.UpsertMemberRoles(m.GuildID, m.User.ID, curRoles, time.Now())
					}
					// update in-memory cache

					ms.cacheRolesSet(m.GuildID, m.User.ID, curRoles)
				}

			}
		}
	}

}

// handleUserUpdate processes user updates across all configured guilds.
func (ms *MonitoringService) handleUserUpdate(s *discordgo.Session, m *discordgo.UserUpdate) {
	cfg := ms.configManager.Config()
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
	changeKey := fmt.Sprintf("%s:%s:%s", guildID, userID, currentAvatar)
	ms.changesMutex.RLock()
	if lastChange, exists := ms.recentChanges[changeKey]; exists {
		if time.Since(lastChange) < 5*time.Second {
			ms.changesMutex.RUnlock()
			log.ApplicationLogger().Info("Avatar change ignored (debounce)", "userID", userID, "guildID", guildID)
			return
		}
	}
	ms.changesMutex.RUnlock()

	oldHash, _, ok, _ := ms.store.GetAvatar(guildID, userID)
	changed := true
	if ok {
		changed = oldHash != currentAvatar
	} else {
		changed = currentAvatar != ""
	}
	if changed {
		ms.changesMutex.Lock()
		ms.recentChanges[changeKey] = time.Now()
		for key, timestamp := range ms.recentChanges {
			if time.Since(timestamp) > time.Minute {
				delete(ms.recentChanges, key)
			}
		}
		ms.changesMutex.Unlock()

		ms.userWatcher.ProcessChange(guildID, userID, currentAvatar, username)
	}
}

// ProcessChange performs avatar-specific logic: notification and persistence.
func (aw *UserWatcher) ProcessChange(guildID, userID, currentAvatar, username string) {
	finalUsername := username
	if finalUsername == "" {
		finalUsername = aw.getUsernameForNotification(guildID, userID)
	}
	var oldAvatar string
	if h, _, ok, _ := aw.store.GetAvatar(guildID, userID); ok {
		oldAvatar = h
	}
	change := files.AvatarChange{
		UserID:    userID,
		Username:  finalUsername,
		OldAvatar: oldAvatar,
		NewAvatar: currentAvatar,
		Timestamp: time.Now(),
	}
	log.ApplicationLogger().Info("Avatar change detected", "userID", userID, "guildID", guildID, "old_avatar", oldAvatar, "new_avatar", currentAvatar)
	guildConfig := aw.configManager.GuildConfig(guildID)
	if guildConfig != nil {
		channelID := guildConfig.UserLogChannelID // Renamed from AvatarLogChannelID
		if channelID == "" {
			log.ErrorLoggerRaw().Error("UserLogChannelID not configured; notification not sent", "guildID", guildID)
		} else {
			if err := aw.notifier.SendAvatarChangeNotification(channelID, change); err != nil {
				log.ErrorLoggerRaw().Error("Error sending notification", "channelID", channelID, "userID", userID, "guildID", guildID, "err", err)
			} else {
				log.ApplicationLogger().Info("Avatar notification sent successfully", "channelID", channelID, "userID", userID, "guildID", guildID)
			}
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

func (ms *MonitoringService) markEvent() {
	if ms.store != nil {
		_ = ms.store.SetLastEvent(time.Now())
	}
}

func (ms *MonitoringService) startHeartbeat() {
	if ms.store == nil || ms.heartbeatTicker != nil {
		return
	}
	ms.heartbeatTicker = time.NewTicker(heartbeatInterval)
	ms.heartbeatStop = make(chan struct{})
	// Set immediately on startup
	_ = ms.store.SetHeartbeat(time.Now())
	go func() {
		for {
			select {
			case <-ms.heartbeatTicker.C:
				_ = ms.store.SetHeartbeat(time.Now())
			case <-ms.heartbeatStop:
				return
			case <-ms.stopChan:
				return
			}
		}
	}()
}

func (ms *MonitoringService) stopHeartbeat() {
	if ms.heartbeatTicker != nil {
		ms.heartbeatTicker.Stop()
		ms.heartbeatTicker = nil
	}
	if ms.heartbeatStop != nil {
		close(ms.heartbeatStop)
		ms.heartbeatStop = nil
	}
}

// rolesCacheCleanupLoop periodically removes expired entries from rolesCache
func (ms *MonitoringService) rolesCacheCleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.cleanupRolesCache()
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
	if len(roles) == 0 {
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
	key := guildID + ":" + userID
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
	ttl := ms.rolesTTL

	stats := map[string]interface{}{
		"isRunning":            ms.isRunning,
		"rolesCacheSize":       size,
		"rolesCacheTTLSeconds": int(ttl.Seconds()),
		"apiAuditLogCalls":     atomic.LoadUint64(&ms.apiAuditLogCalls),
		"apiGuildMemberCalls":  atomic.LoadUint64(&ms.apiGuildMemberCalls),
		"apiMessagesSent":      atomic.LoadUint64(&ms.apiMessagesSent),
		"cacheStateMemberHits": atomic.LoadUint64(&ms.cacheStateMemberHits),
		"cacheRolesMemoryHits": atomic.LoadUint64(&ms.cacheRolesMemoryHits),
		"cacheRolesStoreHits":  atomic.LoadUint64(&ms.cacheRolesStoreHits),
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
func (ms *MonitoringService) handleStartupDowntimeAndMaybeRefresh() {
	if ms.store == nil {
		return
	}
	lastHB, okHB, err := ms.store.GetHeartbeat()
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to read last heartbeat; skipping downtime check", "err", err)
	} else {
		if !okHB || time.Since(lastHB) > downtimeThreshold {
			downtimeDuration := "unknown"
			if okHB {
				downtimeDuration = time.Since(lastHB).Round(time.Second).String()
			}
			log.ApplicationLogger().Info("⏱️ Detected downtime; performing silent avatar refresh before enabling notifications", "downtime", downtimeDuration, "threshold", downtimeThreshold.String())
			cfg := ms.configManager.Config()
			if cfg == nil || len(cfg.Guilds) == 0 {
				log.ApplicationLogger().Info("No configured guilds for startup silent refresh")
				return
			}
			startTime := time.Now()
			var wg sync.WaitGroup
			for _, gcfg := range cfg.Guilds {
				gid := gcfg.GuildID
				wg.Add(1)
				go func(guildID string) {
					defer wg.Done()
					ms.initializeGuildCache(guildID) // Upserts avatars without sending notifications
				}(gid)
			}
			wg.Wait()
			log.ApplicationLogger().Info("✅ Silent avatar refresh completed", "duration", time.Since(startTime).Round(time.Millisecond))
			return
		}
	}
	log.ApplicationLogger().Info("No significant downtime detected; skipping heavy avatar refresh")
}

// fetchAllGuildMembers paginates through all guild members in batches up to 1000 until exhaustion.
func (ms *MonitoringService) fetchAllGuildMembers(guildID string) ([]*discordgo.Member, error) {
	var all []*discordgo.Member
	after := ""
	for {
		members, err := ms.session.GuildMembers(guildID, after, 1000)
		if err != nil {
			log.ErrorLoggerRaw().Error("Failed to paginate guild members", "guildID", guildID, "after", after, "fetched_so_far", len(all), "err", err)
			return all, err
		}
		if len(members) == 0 {
			break
		}
		all = append(all, members...)
		if len(members) < 1000 {
			break
		}
		after = members[len(members)-1].User.ID
	}
	log.ApplicationLogger().Info("Pagination completed successfully", "guildID", guildID, "total_members_fetched", len(all))
	return all, nil
}

func (ms *MonitoringService) performPeriodicCheck() {
	log.ApplicationLogger().Info("Running periodic avatar check...")
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.ApplicationLogger().Info("No configured guilds for periodic check")
		return
	}
	for _, gcfg := range cfg.Guilds {
		members, err := ms.fetchAllGuildMembers(gcfg.GuildID)
		if err != nil {
			log.ErrorLoggerRaw().Error("Error getting members for guild", "guildID", gcfg.GuildID, "err", err)
			continue
		}
		for _, member := range members {
			// Backfill missing member join date using Discord data
			if ms.store != nil && !member.JoinedAt.IsZero() {
				if _, ok, _ := ms.store.GetMemberJoin(gcfg.GuildID, member.User.ID); !ok {
					_ = ms.store.UpsertMemberJoin(gcfg.GuildID, member.User.ID, member.JoinedAt)
				}
			}

			avatarHash := member.User.Avatar
			if avatarHash == "" {
				continue
			}
			ms.checkAvatarChange(gcfg.GuildID, member.User.ID, avatarHash, member.User.Username)
		}
	}
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
	// Previously gated via ALICE_DISABLE_BOT_ROLE_PERM_MIRROR env var; now read from runtime_config in settings.json.
	if ms.configManager != nil && ms.configManager.Config() != nil {
		rc := ms.configManager.Config().ResolveRuntimeConfig(guildID)
		return !rc.DisableBotRolePermMirror
	}
	return true
}

func (ms *MonitoringService) botPermMirrorActorRoleID(guildID string) string {
	// Previously overridable via ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID env var; now read from runtime_config in settings.json.
	if ms.configManager != nil && ms.configManager.Config() != nil {
		rc := ms.configManager.Config().ResolveRuntimeConfig(guildID)
		v := strings.TrimSpace(rc.BotRolePermMirrorActorRoleID)
		if v != "" {
			return v
		}
	}
	return defaultBotPermMirrorActorRoleID
}

func (ms *MonitoringService) isBotManagedRole(guildID, roleID string) bool {
	if guildID == "" || roleID == "" || ms.session == nil {
		return false
	}
	roles, err := ms.session.GuildRoles(guildID)
	if err != nil {
		return false
	}
	for _, r := range roles {
		if r == nil || r.ID != roleID {
			continue
		}
		return r.Managed
	}
	return false
}

func (ms *MonitoringService) getRoleByID(guildID, roleID string) (*discordgo.Role, bool) {
	if guildID == "" || roleID == "" || ms.session == nil {
		return nil, false
	}
	roles, err := ms.session.GuildRoles(guildID)
	if err != nil {
		return nil, false
	}
	for _, r := range roles {
		if r != nil && r.ID == roleID {
			return r, true
		}
	}
	return nil, false
}

func (ms *MonitoringService) findBotManagedRole(guildID string) (*discordgo.Role, bool) {
	if guildID == "" || ms.session == nil {
		return nil, false
	}
	roles, err := ms.session.GuildRoles(guildID)
	if err != nil {
		return nil, false
	}
	for _, r := range roles {
		if r == nil {
			continue
		}
		if r.Managed {
			return r, true
		}
	}
	return nil, false
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
	_, _ = ms.session.GuildRoleEdit(guildID, roleID, &discordgo.RoleParams{
		Permissions: &perm,
	})
}

func (ms *MonitoringService) handleRoleCreateForBotPermMirroring(s *discordgo.Session, e *discordgo.GuildRoleCreate) {
	if e == nil || e.Role == nil || e.GuildID == "" {
		return
	}
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
	member, err := ms.session.GuildMember(guildID, userID)
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
	guild, err := ms.session.Guild(guildID)
	if err != nil {
		return nil, err
	}

	if ms.unifiedCache != nil {
		ms.unifiedCache.SetGuild(guildID, guild)
	}
	return guild, nil
}
