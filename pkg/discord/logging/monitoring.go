package logging

import (
	"context"
	"fmt"
	"strconv"
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

const (
	heartbeatInterval = time.Minute
	downtimeThreshold = 30 * time.Minute
)

// UserWatcher contém a lógica específica de processamento de mudanças de usuário.
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

// MonitoringService coordena handlers multi-guild e delega tarefas específicas (ex.: usuário).
type MonitoringService struct {
	session             *discordgo.Session
	configManager       *files.ConfigManager
	store               *storage.Store
	notifier            *NotificationSender
	adapters            *task.NotificationAdapters
	router              *task.TaskRouter
	userWatcher         *UserWatcher
	memberEventService  *MemberEventService  // Serviço para eventos de membros
	messageEventService *MessageEventService // Serviço para eventos de mensagens
	isRunning           bool
	stopChan            chan struct{}
	stopOnce            sync.Once
	runMu               sync.Mutex
	recentChanges       map[string]time.Time // Debounce para evitar duplicidade
	changesMutex        sync.RWMutex
	cronCancel          func()

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
		log.Error().Errorf("Monitoring service is already running")
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

	// Iniciar novos serviços
	if err := ms.memberEventService.Start(); err != nil {
		ms.isRunning = false
		return fmt.Errorf("failed to start member event service: %w", err)
	}
	if err := ms.messageEventService.Start(); err != nil {
		ms.isRunning = false
		// Parar o serviço de membros se falhou
		ms.memberEventService.Stop()
		return fmt.Errorf("failed to start message event service: %w", err)
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
				log.Error().Errorf("Error refreshing roles for guild %s: %v", gcfg.GuildID, err)
				continue
			}
			for _, member := range members {
				if len(member.Roles) == 0 {
					continue
				}
				if err := ms.store.UpsertMemberRoles(gcfg.GuildID, member.User.ID, member.Roles, time.Now()); err != nil {
					log.Warn().Applicationf("Failed to upsert roles for user %s in guild %s: %v", member.User.ID, gcfg.GuildID, err)
					continue
				}
				ms.cacheRolesSet(gcfg.GuildID, member.User.ID, member.Roles)
				totalUpdates++
			}
		}
		log.Info().Applicationf("✅ Roles DB refresh completed: %d members updated in %s", totalUpdates, time.Since(start).Round(time.Second))
		return nil
	})

	// Using TaskRouter scheduler helpers for daily scheduling
	// Schedule periodic jobs
	ms.cronCancel = ms.router.ScheduleEvery(30*time.Minute, task.Task{Type: "monitor.scan_avatars"})
	// Schedule daily roles refresh at 03:00 UTC
	ms.router.ScheduleDailyAtUTC(3, 0, task.Task{Type: "monitor.refresh_roles"})

	// Trigger one-time roles refresh on startup (non-blocking)
	go func() {
		_ = ms.router.Dispatch(context.Background(), task.Task{Type: "monitor.refresh_roles"})
	}()

	log.Info().Applicationf("All monitoring services started successfully")
	return nil
}

// Stop stops the monitoring service. Returns error if not running.
func (ms *MonitoringService) Stop() error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	if !ms.isRunning {
		log.Error().Errorf("Monitoring service is not running")
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
		log.Info().Applicationf("💾 Persisting cache to storage...")
		if err := ms.unifiedCache.Persist(); err != nil {
			log.Error().Errorf("Failed to persist cache (continuing): %v", err)
		} else {
			stats := ms.unifiedCache.GetStats()
			total := stats.MemberEntries + stats.GuildEntries + stats.RolesEntries + stats.ChannelEntries
			log.Info().Applicationf("✅ Cache persisted: %d entries saved", total)
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

	// Parar novos serviços
	if err := ms.memberEventService.Stop(); err != nil {
		log.Error().Errorf("Error stopping member event service: %v", err)
	}
	if err := ms.messageEventService.Stop(); err != nil {
		log.Error().Errorf("Error stopping message event service: %v", err)
	}

	// Cancel cron before closing router
	if ms.cronCancel != nil {
		ms.cronCancel()
	}

	if ms.router != nil {
		ms.router.Close()
	}
	log.Info().Applicationf("Monitoring service stopped")
	return nil
}

// initializeCache carrega os usuários atuais dos membros em todos os guilds configurados.
func (ms *MonitoringService) initializeCache() {
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.Info().Applicationf("No guild configured for monitoring")
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

// initializeGuildCache inicializa os avatares atuais dos membros em um guild específico.
func (ms *MonitoringService) initializeGuildCache(guildID string) {
	if ms.store == nil {
		log.Warn().Applicationf("Store is nil; skipping cache initialization for guild: %s", guildID)
		return
	}

	// Use unified cache for guild fetch
	guild, err := ms.getGuild(guildID)
	if err != nil {
		log.Error().Errorf("Error getting guild %s: %v", guildID, err)
		return
	}
	log.Info().Applicationf("Initializing cache for guild: %s (ID: %s)", guild.Name, guild.ID)
	_ = ms.store.SetGuildOwnerID(guildID, guild.OwnerID)

	// Set bot join time if missing
	if _, ok, _ := ms.store.GetBotSince(guildID); !ok {
		botID := ms.session.State.User.ID
		var botMember *discordgo.Member
		// Preferir cache do state para evitar chamada REST
		if ms.session != nil && ms.session.State != nil {
			if m, _ := ms.session.State.Member(guildID, botID); m != nil {
				botMember = m
			}
		}
		// Fallback para REST somente se necessário
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
		log.Error().Errorf("Error getting members for guild %s: %v", guildID, err)
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

// setupEventHandlers registra handlers do Discord.
func (ms *MonitoringService) setupEventHandlers() {
	// Store handler references for later removal
	ms.eventHandlers = append(ms.eventHandlers,
		ms.session.AddHandler(ms.handlePresenceUpdate),
		ms.session.AddHandler(ms.handleMemberUpdate),
		ms.session.AddHandler(ms.handleUserUpdate),
		ms.session.AddHandler(ms.handleGuildCreate),
		ms.session.AddHandler(ms.handleGuildUpdate),
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

// ensureGuildsListed adiciona entradas mínimas de guild no discordcore.json
// para todas as guilds presentes na sessão mas ausentes na configuração.
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
				log.Error().Errorf("Error adding minimal guild entry for guild %s: %v", g.ID, err)
				continue
			}
			if err := ms.configManager.SaveConfig(); err != nil {
				log.Error().Errorf("Error saving config after minimal guild add for guild %s: %v", g.ID, err)
			} else {
				log.Info().Applicationf("📘 Guild listed in config (minimal entry) for guild %s", g.ID)
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
		// Guild nova: adicionar no config e inicializar cache
		if err := ms.configManager.RegisterGuild(s, guildID); err != nil {
			log.Error().Errorf("Falling back to minimal guild entry for guild %s: %v", guildID, err)
			if err2 := ms.configManager.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err2 != nil {
				log.Error().Errorf("Error adding minimal guild entry for guild %s: %v", guildID, err2)
				return
			}
		}
		if err := ms.configManager.SaveConfig(); err != nil {
			log.Error().Errorf("Error saving config after guild add for guild %s: %v", guildID, err)
		}
		log.Info().Applicationf("🆕 New guild listed in config for guild %s", guildID)
		ms.initializeGuildCache(guildID)
		// No-op: avatars persisted per change in SQLite store
	}
}

// handleGuildUpdate atualiza o cache do OwnerID quando houver mudança de propriedade do servidor.
func (ms *MonitoringService) handleGuildUpdate(s *discordgo.Session, e *discordgo.GuildUpdate) {
	if e == nil || e.Guild == nil || e.Guild.ID == "" {
		return
	}
	if ms.store != nil {
		if prev, ok, _ := ms.store.GetGuildOwnerID(e.Guild.ID); ok && prev != e.Guild.OwnerID {
			log.Info().Applicationf("Guild owner changed: guildID=%s, from=%s, to=%s", e.Guild.ID, prev, e.Guild.OwnerID)
		}
		_ = ms.store.SetGuildOwnerID(e.Guild.ID, e.Guild.OwnerID)
	}
}

// handlePresenceUpdate processa updates de presença (inclui avatar).
func (ms *MonitoringService) handlePresenceUpdate(s *discordgo.Session, m *discordgo.PresenceUpdate) {
	if m.User == nil {
		return
	}
	if ms.configManager.GuildConfig(m.GuildID) == nil {
		return
	}
	if m.User.Username == "" {
		log.Info().Applicationf("PresenceUpdate ignored (empty username) for user %s in guild %s", m.User.ID, m.GuildID)
		return
	}
	ms.markEvent()
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)
}

// handleMemberUpdate processa updates de membro.
func (ms *MonitoringService) handleMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.User == nil {
		return
	}
	gcfg := ms.configManager.GuildConfig(m.GuildID)
	if gcfg == nil {
		return
	}

	// Avatar change logging (já existente)
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)

	// Role update logging (via Audit Log)
	channelID := gcfg.UserLogChannelID
	if channelID == "" {
		channelID = gcfg.CommandChannelID
	}
	if channelID == "" {
		return
	}

	// Buscar audit log de atualização de cargos usando constante e com retentativa curta
	actionType := int(discordgo.AuditLogActionMemberRoleUpdate)

	// Helper para obter diff verificado entre o snapshot local (memória/SQLite) e o estado atual no Discord.
	// Retorna também os roles atuais considerados para atualização de snapshot.
	computeVerifiedDiff := func(guildID, userID string, proposed []string) (cur []string, added []string, removed []string) {
		// 1) determinar estado atual a partir do proposto (event) ou do Discord
		cur = proposed
		if len(cur) == 0 {
			if member, err := ms.getGuildMember(guildID, userID); err == nil && member != nil {
				cur = member.Roles
			}
		}
		if len(cur) == 0 {
			return cur, nil, nil
		}

		// 2) obter estado anterior (preferir cache em memória com TTL; fallback SQLite)
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

		// 3) calcular diffs
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
			log.Warn().Applicationf("Failed to fetch audit logs for role update: guildID=%s, userID=%s, error=%v", m.GuildID, m.User.ID, err)
			return false
		}

		for _, entry := range audit.AuditLogEntries {
			if entry == nil || entry.ActionType == nil || *entry.ActionType != discordgo.AuditLogActionMemberRoleUpdate || entry.TargetID != m.User.ID {
				continue
			}
			actorID := entry.UserID

			// Verificação de recência da entry (via snowflake ID -> timestamp)
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
					// considerar NewValue e OldValue por robustez
					added = append(added, extractRoles(ch.NewValue)...)
					added = append(added, extractRoles(ch.OldValue)...)
				case discordgo.AuditLogChangeKeyRoleRemove:
					removed = append(removed, extractRoles(ch.NewValue)...)
					removed = append(removed, extractRoles(ch.OldValue)...)
				}
			}

			if len(added) == 0 && len(removed) == 0 {
				// Sem mudanças relevantes detectadas nessa entrada; continuar varrendo
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

			// Verificar com o Discord + DB quais mudanças realmente foram aplicadas
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

			// Filtrar apenas os cargos que realmente foram adicionados/removidos segundo o estado atual
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

			// Se nada restou após verificação, não enviar embed
			if len(filteredAdded) == 0 && len(filteredRemoved) == 0 {
				// Atualizar snapshot mesmo assim para manter o DB consistente
				if ms.store != nil && len(curRoles) > 0 {
					_ = ms.store.UpsertMemberRoles(m.GuildID, m.User.ID, curRoles, time.Now())
					ms.cacheRolesSet(m.GuildID, m.User.ID, curRoles)
				}
				// Continuar varrendo outras entries possíveis
				continue
			}

			desc := fmt.Sprintf("<@%s> updated roles for **%s** (<@%s>)", actorID, m.User.Username, m.User.ID)
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
				log.Error().Errorf("Failed to send role update notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, channelID, sendErr)
			} else {
				log.Info().Applicationf("Role update notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, channelID)
				// Atualizar snapshot para refletir o estado após a mudança
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
	// Fallback por diff de roles quando audit log não produziu resultado
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
				desc := fmt.Sprintf("Role changes detected for **%s** (<@%s>)", m.User.Username, m.User.ID)
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
					log.Error().Errorf("Failed to send fallback role update notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, channelID, sendErr)
				} else {
					log.Info().Applicationf("Fallback role update notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, channelID)
					// Atualiza snapshot de roles após o envio
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

// handleUserUpdate processa updates de usuário em todos os guilds configurados.
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
			log.Info().Applicationf("Avatar change ignored (debounce) for user %s in guild %s", userID, guildID)
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

// ProcessChange executa a lógica específica de avatar: notificação e persistência.
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
	log.Info().Applicationf("Avatar change detected for user %s in guild %s. Old avatar: %s, new avatar: %s", userID, guildID, oldAvatar, currentAvatar)
	guildConfig := aw.configManager.GuildConfig(guildID)
	if guildConfig != nil {
		channelID := guildConfig.UserLogChannelID // Renamed from AvatarLogChannelID
		if channelID == "" {
			log.Error().Errorf("UserLogChannelID not configured for guild %s. Notification not sent.", guildID)
		} else {
			if err := aw.notifier.SendAvatarChangeNotification(channelID, change); err != nil {
				log.Error().Errorf("Error sending notification to channel %s for user %s in guild %s: %v", channelID, userID, guildID, err)
			} else {
				log.Info().Applicationf("Avatar notification sent successfully to channel %s for user %s in guild %s", channelID, userID, guildID)
			}
		}
	}
	if _, _, err := aw.store.UpsertAvatar(guildID, userID, currentAvatar, time.Now()); err != nil {
		log.Error().Errorf("Error saving avatar in store for guild %s: %v", guildID, err)
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
		log.Info().Applicationf("Error getting member for username for user %s in guild %s: %v - using ID", userID, guildID, err)
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
		log.Info().Applicationf("Cleaned up %d expired roles cache entries", len(toDelete))
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
		stats["unifiedCache"] = ms.unifiedCache.StatsGeneric()
		// Keep specific stats for backward compatibility
		stats["unifiedCacheSpecific"] = map[string]interface{}{
			"memberEntries":  ucStats.MemberEntries,
			"guildEntries":   ucStats.GuildEntries,
			"rolesEntries":   ucStats.RolesEntries,
			"channelEntries": ucStats.ChannelEntries,
			"memberHits":     ucStats.MemberHits,
			"memberMisses":   ucStats.MemberMisses,
			"guildHits":      ucStats.GuildHits,
			"guildMisses":    ucStats.GuildMisses,
			"rolesHits":      ucStats.RolesHits,
			"rolesMisses":    ucStats.RolesMisses,
			"channelHits":    ucStats.ChannelHits,
			"channelMisses":  ucStats.ChannelMisses,
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
		log.Error().Errorf("Failed to read last heartbeat; skipping downtime check: %v", err)
	} else {
		if !okHB || time.Since(lastHB) > downtimeThreshold {
			log.Info().Applicationf("⏱️ Detected downtime > threshold; performing silent avatar refresh before enabling notifications")
			cfg := ms.configManager.Config()
			if cfg == nil || len(cfg.Guilds) == 0 {
				log.Info().Applicationf("No configured guilds for startup silent refresh")
				return
			}
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
			log.Info().Applicationf("✅ Silent avatar refresh completed")
			return
		}
	}
	log.Info().Applicationf("No significant downtime detected; skipping heavy avatar refresh")
}

// fetchAllGuildMembers paginates through all guild members in batches up to 1000 until exhaustion.
func (ms *MonitoringService) fetchAllGuildMembers(guildID string) ([]*discordgo.Member, error) {
	var all []*discordgo.Member
	after := ""
	for {
		members, err := ms.session.GuildMembers(guildID, after, 1000)
		if err != nil {
			log.Error().Errorf("Failed to paginate guild members: guildID=%s, after=%s, fetched_so_far=%d, error=%v", guildID, after, len(all), err)
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
	log.Info().Applicationf("Pagination completed successfully: guildID=%s, total_members_fetched=%d", guildID, len(all))
	return all, nil
}

func (ms *MonitoringService) performPeriodicCheck() {
	log.Info().Applicationf("Running periodic avatar check...")
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.Info().Applicationf("No configured guilds for periodic check")
		return
	}
	for _, gcfg := range cfg.Guilds {
		members, err := ms.fetchAllGuildMembers(gcfg.GuildID)
		if err != nil {
			log.Error().Errorf("Error getting members for guild %s: %v", gcfg.GuildID, err)
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
