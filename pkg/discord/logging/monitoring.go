package logging

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/alice-bnuy/discordcore/pkg/files"
	logutil "github.com/alice-bnuy/discordcore/pkg/logging"
	"github.com/alice-bnuy/discordcore/pkg/storage"
	"github.com/alice-bnuy/discordcore/pkg/task"
	"github.com/bwmarrin/discordgo"
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
}

func NewUserWatcher(session *discordgo.Session, configManager *files.ConfigManager, store *storage.Store, notifier *NotificationSender) *UserWatcher {
	return &UserWatcher{
		session:       session,
		configManager: configManager,
		store:         store,
		notifier:      notifier,
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
	memberEventService  *MemberEventService  // Serviço para eventos de entrada/saída
	messageEventService *MessageEventService // Serviço para eventos de mensagens
	isRunning           bool
	stopChan            chan struct{}
	runMu               sync.Mutex
	recentChanges       map[string]time.Time // Debounce para evitar duplicidade
	changesMutex        sync.RWMutex
	cronCancel          func()

	// Heartbeat runtime tracking
	heartbeatTicker *time.Ticker
	heartbeatStop   chan struct{}
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
	ms := &MonitoringService{
		session:             session,
		configManager:       configManager,
		store:               store,
		notifier:            n,
		userWatcher:         NewUserWatcher(session, configManager, store, n),
		memberEventService:  NewMemberEventService(session, configManager, n),
		messageEventService: NewMessageEventService(session, configManager, n),
		adapters:            adapters,
		router:              router,
		stopChan:            make(chan struct{}),
		recentChanges:       make(map[string]time.Time),
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
		logutil.Warn("Monitoring service is already running")
		return fmt.Errorf("monitoring service is already running")
	}
	ms.isRunning = true
	ms.stopChan = make(chan struct{})
	ms.ensureGuildsListed()
	// Detect downtime and refresh avatars silently before wiring handlers (no notifications)
	ms.handleStartupDowntimeAndMaybeRefresh()
	ms.setupEventHandlers()
	// Start periodic heartbeat tracker (persisted)
	ms.startHeartbeat()

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
	ms.cronCancel = ms.router.ScheduleEvery(30*time.Minute, task.Task{Type: "monitor.scan_avatars"})
	logutil.Info("All monitoring services started successfully")
	return nil
}

// Stop stops the monitoring service. Returns error if not running.
func (ms *MonitoringService) Stop() error {
	ms.runMu.Lock()
	defer ms.runMu.Unlock()
	if !ms.isRunning {
		logutil.Warn("Monitoring service is not running")
		return fmt.Errorf("monitoring service is not running")
	}
	ms.isRunning = false
	close(ms.stopChan)
	ms.stopHeartbeat()

	// Parar novos serviços
	if err := ms.memberEventService.Stop(); err != nil {
		logutil.WithError(err).Warn("Error stopping member event service")
	}
	if err := ms.messageEventService.Stop(); err != nil {
		logutil.WithError(err).Warn("Error stopping message event service")
	}

	// Cancel cron before closing router
	if ms.cronCancel != nil {
		ms.cronCancel()
	}

	if ms.router != nil {
		ms.router.Close()
	}
	logutil.Info("Monitoring service stopped")
	return nil
}

// initializeCache carrega os usuários atuais dos membros em todos os guilds configurados.
func (ms *MonitoringService) initializeCache() {
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.Println("No guild configured for monitoring")
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
	guild, err := ms.session.Guild(guildID)
	if err != nil {
		log.Printf("Error getting guild %s: %v", guildID, err)
		return
	}
	log.Printf("Initializing cache for guild: %s (ID: %s)", guild.Name, guild.ID)

	// Set bot join time if missing
	if _, ok, _ := ms.store.GetBotSince(guildID); !ok {
		botID := ms.session.State.User.ID
		if botMember, err := ms.session.GuildMember(guildID, botID); err == nil && !botMember.JoinedAt.IsZero() {
			_ = ms.store.SetBotSince(guildID, botMember.JoinedAt)
		} else {
			_ = ms.store.SetBotSince(guildID, time.Now())
		}
	}
	members, err := ms.session.GuildMembers(guildID, "", 1000)
	if err != nil {
		log.Printf("Error getting members for guild %s: %v", guildID, err)
		return
	}
	for _, member := range members {
		avatarHash := member.User.Avatar
		if avatarHash == "" {
			avatarHash = "default"
		}
		_, _, _ = ms.store.UpsertAvatar(guildID, member.User.ID, avatarHash, time.Now())
	}
}

// setupEventHandlers registra handlers do Discord.
func (ms *MonitoringService) setupEventHandlers() {
	ms.session.AddHandler(ms.handlePresenceUpdate)
	ms.session.AddHandler(ms.handleMemberUpdate)
	ms.session.AddHandler(ms.handleUserUpdate)
	ms.session.AddHandler(ms.handleGuildCreate)
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
				logutil.WithField("guildID", g.ID).ErrorWithErr("Error adding minimal guild entry", err)
				continue
			}
			if err := ms.configManager.SaveConfig(); err != nil {
				logutil.WithField("guildID", g.ID).ErrorWithErr("Error saving config after minimal guild add", err)
			} else {
				logutil.WithField("guildID", g.ID).Info("📘 Guild listed in config (minimal entry)")
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
			logutil.WithField("guildID", guildID).Warnf("Falling back to minimal guild entry: %v", err)
			if err2 := ms.configManager.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err2 != nil {
				logutil.WithField("guildID", guildID).ErrorWithErr("Error adding minimal guild entry", err2)
				return
			}
		}
		if err := ms.configManager.SaveConfig(); err != nil {
			logutil.WithField("guildID", guildID).ErrorWithErr("Error saving config after guild add", err)
		}
		logutil.WithField("guildID", guildID).Info("🆕 New guild listed in config")
		ms.initializeGuildCache(guildID)
		// No-op: avatars persisted per change in SQLite store
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
		logutil.WithFields(map[string]interface{}{"userID": m.User.ID, "guildID": m.GuildID}).Debug("PresenceUpdate ignored (empty username)")
		return
	}
	ms.markEvent()
	ms.markEvent()
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)
}

// handleMemberUpdate processa updates de membro.
func (ms *MonitoringService) handleMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m.User == nil {
		return
	}
	if ms.configManager.GuildConfig(m.GuildID) == nil {
		return
	}
	ms.checkAvatarChange(m.GuildID, m.User.ID, m.User.Avatar, m.User.Username)
}

// handleUserUpdate processa updates de usuário em todos os guilds configurados.
func (ms *MonitoringService) handleUserUpdate(s *discordgo.Session, m *discordgo.UserUpdate) {
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}
	for _, gcfg := range cfg.Guilds {
		member, err := s.GuildMember(gcfg.GuildID, m.User.ID)
		if err != nil || member == nil || member.User == nil {
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
			logutil.WithFields(map[string]interface{}{"userID": userID, "guildID": guildID, "username": username, "avatar": currentAvatar}).Debug("Avatar change ignored (debounce)")
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
	logutil.WithFields(map[string]interface{}{"userID": userID, "guildID": guildID, "username": finalUsername, "oldAvatar": oldAvatar, "newAvatar": currentAvatar}).Info("Avatar change detected")
	guildConfig := aw.configManager.GuildConfig(guildID)
	if guildConfig != nil {
		channelID := guildConfig.UserLogChannelID // Renamed from AvatarLogChannelID
		if channelID == "" {
			logutil.WithFields(map[string]interface{}{"guildID": guildID}).Warn("UserLogChannelID not configured for guild. Notification not sent.")
		} else {
			if err := aw.notifier.SendAvatarChangeNotification(channelID, change); err != nil {
				logutil.WithFields(map[string]interface{}{"channelID": channelID, "userID": userID, "guildID": guildID}).ErrorWithErr("Error sending notification", err)
			} else {
				logutil.WithFields(map[string]interface{}{"channelID": channelID, "userID": userID, "guildID": guildID}).Info("Avatar notification sent successfully")
			}
		}
	}
	if _, _, err := aw.store.UpsertAvatar(guildID, userID, currentAvatar, time.Now()); err != nil {
		logutil.WithField("guildID", guildID).ErrorWithErr("Error saving avatar in store", err)
	}
}

func (aw *UserWatcher) getUsernameForNotification(guildID, userID string) string {
	member, err := aw.session.GuildMember(guildID, userID)
	if err != nil {
		logutil.WithFields(map[string]interface{}{"userID": userID, "guildID": guildID, "error": err.Error()}).Debug("Error getting member for username - using ID")
		return userID
	}
	if member.User != nil && member.User.Username != "" {
		return member.User.Username
	}
	if member.Nick != "" {
		return member.Nick
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

func (ms *MonitoringService) handleStartupDowntimeAndMaybeRefresh() {
	if ms.store == nil {
		return
	}
	lastHB, okHB, err := ms.store.GetHeartbeat()
	if err != nil {
		logutil.WithError(err).Warn("Failed to read last heartbeat; skipping downtime check")
	} else {
		if !okHB || time.Since(lastHB) > downtimeThreshold {
			logutil.Info("⏱️ Detected downtime > threshold; performing silent avatar refresh before enabling notifications")
			cfg := ms.configManager.Config()
			if cfg == nil || len(cfg.Guilds) == 0 {
				log.Println("No configured guilds for startup silent refresh")
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
			logutil.Info("✅ Silent avatar refresh completed")
			return
		}
	}
	logutil.Debug("No significant downtime detected; skipping heavy avatar refresh")
}

func (ms *MonitoringService) performPeriodicCheck() {
	log.Println("Running periodic avatar check...")
	cfg := ms.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.Println("No configured guilds for periodic check")
		return
	}
	for _, gcfg := range cfg.Guilds {
		members, err := ms.session.GuildMembers(gcfg.GuildID, "", 1000)
		if err != nil {
			log.Printf("Error getting members for guild %s: %v", gcfg.GuildID, err)
			continue
		}
		for _, member := range members {
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
