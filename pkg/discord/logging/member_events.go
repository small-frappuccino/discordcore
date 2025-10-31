package logging

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// Hardcoded IDs for automatic role assignment

// MemberEventService manages member join/leave events
type MemberEventService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	notifier      *NotificationSender
	adapters      *task.NotificationAdapters
	isRunning     bool

	// Cache para tempos de entrada (membro e bot)

	joinTimes map[string]time.Time // chave: guildID:userID
	joinMu    sync.RWMutex

	// Persistência complementar (SQLite)
	store *storage.Store

	// Cleanup control
	cleanupStop chan struct{}
}

// NewMemberEventService cria uma nova instância do serviço de eventos de membros
func NewMemberEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender, store *storage.Store) *MemberEventService {
	return &MemberEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		store:         store,
		isRunning:     false,
		cleanupStop:   make(chan struct{}),
	}
}

func (mes *MemberEventService) SetAdapters(adapters *task.NotificationAdapters) {
	mes.adapters = adapters
}

// Start registers member event handlers
func (mes *MemberEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("member event service is already running")
	}
	mes.isRunning = true

	// Ensure join map is initialized
	if mes.joinTimes == nil {
		mes.joinTimes = make(map[string]time.Time)
	}

	// Store should be injected and already initialized
	if mes.store != nil {
		if err := mes.store.Init(); err != nil {
			slog.Warn(fmt.Sprintf("Member event service: failed to initialize SQLite store (continuing): %v", err))
		}
	}

	mes.session.AddHandler(mes.handleGuildMemberAdd)
	mes.session.AddHandler(mes.handleGuildMemberUpdate)
	mes.session.AddHandler(mes.handleGuildMemberRemove)

	// Start periodic cleanup of old joinTimes entries
	mes.cleanupStop = make(chan struct{})
	go mes.cleanupLoop()

	slog.Info("Member event service started")
	return nil
}

// Stop the service
func (mes *MemberEventService) Stop() error {
	if !mes.isRunning {
		return fmt.Errorf("member event service is not running")
	}
	mes.isRunning = false

	// Stop cleanup goroutine
	if mes.cleanupStop != nil {
		close(mes.cleanupStop)
		mes.cleanupStop = nil
	}

	slog.Info("Member event service stopped")
	return nil
}

// IsRunning returns whether the service is running
func (mes *MemberEventService) IsRunning() bool {
	return mes.isRunning
}

// handleGuildMemberAdd processes when a user joins the server
func (mes *MemberEventService) handleGuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}

	mes.markEvent()
	guildConfig := mes.configManager.GuildConfig(m.GuildID)
	if guildConfig == nil {
		return
	}

	// Prefer dedicated entry/leave channel; fallback to general user log channel
	logChannelID := guildConfig.UserEntryLeaveChannelID
	if logChannelID == "" {
		logChannelID = guildConfig.UserLogChannelID
	}
	if logChannelID == "" {
		slog.Info(fmt.Sprintf("User entry/leave channel not configured for guild, member join notification not sent: guildID=%s, userID=%s", m.GuildID, m.User.ID))
		return
	}

	// Calculate how long the account has existed
	accountAge := mes.calculateAccountAge(m.User.ID)

	// Also persist to SQLite (best effort)
	if mes.store != nil {
		if member, err := mes.session.GuildMember(m.GuildID, m.User.ID); err == nil && !member.JoinedAt.IsZero() {
			_ = mes.store.UpsertMemberJoin(m.GuildID, m.User.ID, member.JoinedAt)
		}
	}

	// NEW: Register precise member join timestamp in memory
	if member, err := mes.session.GuildMember(m.GuildID, m.User.ID); err == nil && !member.JoinedAt.IsZero() {
		mes.joinMu.Lock()
		if mes.joinTimes == nil {
			mes.joinTimes = make(map[string]time.Time)
		}
		mes.joinTimes[m.GuildID+":"+m.User.ID] = member.JoinedAt
		mes.joinMu.Unlock()
	}

	slog.Info(fmt.Sprintf("Member joined guild: guildID=%s, userID=%s, username=%s, accountAge=%s", m.GuildID, m.User.ID, m.User.Username, accountAge.String()))

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberJoin(logChannelID, m, accountAge); err != nil {
			slog.Error(fmt.Sprintf("Failed to send member join notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
		} else {
			slog.Info(fmt.Sprintf("Member join notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
		}
	} else if err := mes.notifier.SendMemberJoinNotification(logChannelID, m, accountAge); err != nil {
		slog.Error(fmt.Sprintf("Failed to send member join notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
	} else {
		slog.Info(fmt.Sprintf("Member join notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
	}

	// Composite automatic role assignment for the specified guild
	// Atribuição automática de cargo composta (config por guild)
	if guildConfig.AutoRoleAssignmentEnabled {
		targetRoleID := guildConfig.AutoRoleTargetRoleID
		roleA := guildConfig.AutoRolePrereqRoleA
		roleB := guildConfig.AutoRolePrereqRoleB
		if targetRoleID != "" && roleA != "" && roleB != "" {
			if member, err := mes.session.GuildMember(m.GuildID, m.User.ID); err == nil && member != nil {
				roles := member.Roles
				if hasRoleID(roles, roleA) && hasRoleID(roles, roleB) && !hasRoleID(roles, targetRoleID) {
					if err := mes.session.GuildMemberRoleAdd(m.GuildID, m.User.ID, targetRoleID); err != nil {
						slog.Error(fmt.Sprintf("Failed to grant target role on join: guildID=%s, userID=%s, roleID=%s, error=%v", m.GuildID, m.User.ID, targetRoleID, err))
					} else {
						slog.Info(fmt.Sprintf("Granted target role on join: guildID=%s, userID=%s, roleID=%s", m.GuildID, m.User.ID, targetRoleID))
					}
				}
			}
		}
	}
}

// handleGuildMemberRemove processes when a user leaves the server
func (mes *MemberEventService) handleGuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}

	mes.markEvent()
	guildConfig := mes.configManager.GuildConfig(m.GuildID)
	if guildConfig == nil {
		return
	}

	// Prefer dedicated entry/leave channel; fallback to general user log channel
	logChannelID := guildConfig.UserEntryLeaveChannelID
	if logChannelID == "" {
		logChannelID = guildConfig.UserLogChannelID
	}
	if logChannelID == "" {
		slog.Info(fmt.Sprintf("User entry/leave channel not configured for guild, member leave notification not sent: guildID=%s, userID=%s", m.GuildID, m.User.ID))
		return
	}

	// Calculate how long they were in the server
	var serverTime time.Duration
	var t time.Time
	var ok bool
	mes.joinMu.RLock()
	t, ok = mes.joinTimes[m.GuildID+":"+m.User.ID]
	mes.joinMu.RUnlock()
	if ok && !t.IsZero() {
		serverTime = time.Since(t)
	} else {
		serverTime = mes.calculateServerTime(m.GuildID, m.User.ID)
	}

	botTime := mes.getBotTimeOnServer(m.GuildID)

	slog.Info(fmt.Sprintf("Member left guild: guildID=%s, userID=%s, username=%s, serverTime=%s, botTime=%s", m.GuildID, m.User.ID, m.User.Username, serverTime.String(), botTime.String()))

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberLeave(logChannelID, m, serverTime, botTime); err != nil {
			slog.Error(fmt.Sprintf("Failed to send member leave notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
		} else {
			slog.Info(fmt.Sprintf("Member leave notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
		}
	} else if err := mes.notifier.SendMemberLeaveNotification(logChannelID, m, serverTime, botTime); err != nil {
		slog.Error(fmt.Sprintf("Failed to send member leave notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err))
	} else {
		slog.Info(fmt.Sprintf("Member leave notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID))
	}
}

// Utility function to check if the user has a specific role
func hasRoleID(roles []string, roleID string) bool {
	for _, r := range roles {
		if r == roleID {
			return true
		}
	}
	return false
}

// handleGuildMemberUpdate maintains the role relationship:
// - If the user loses role A, remove the target role.
// - If the user has both A and B, grant the target role (if not already present).
func (mes *MemberEventService) handleGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}
	guildConfig := mes.configManager.GuildConfig(m.GuildID)
	if guildConfig == nil || !guildConfig.AutoRoleAssignmentEnabled {
		return
	}

	targetRoleID := guildConfig.AutoRoleTargetRoleID
	roleA := guildConfig.AutoRolePrereqRoleA
	roleB := guildConfig.AutoRolePrereqRoleB
	if targetRoleID == "" || roleA == "" || roleB == "" {
		return
	}
	hasTarget := hasRoleID(m.Roles, targetRoleID)
	hasA := hasRoleID(m.Roles, roleA)
	hasB := hasRoleID(m.Roles, roleB)

	// Se perdeu o cargo A e ainda tem o cargo alvo, remover o cargo alvo
	if hasTarget && !hasA {
		if err := mes.session.GuildMemberRoleRemove(m.GuildID, m.User.ID, targetRoleID); err != nil {
			slog.Error(fmt.Sprintf("Failed to remove target role on update: guildID=%s, userID=%s, roleID=%s, error=%v", m.GuildID, m.User.ID, targetRoleID, err))
		} else {
			slog.Info(fmt.Sprintf("Removed target role on update: guildID=%s, userID=%s, roleID=%s", m.GuildID, m.User.ID, targetRoleID))
		}
		return
	}

	// Se possui ambos os cargos pré-requisito e ainda não tem o cargo alvo, concede o cargo alvo
	if !hasTarget && hasA && hasB {
		if err := mes.session.GuildMemberRoleAdd(m.GuildID, m.User.ID, targetRoleID); err != nil {
			slog.Error(fmt.Sprintf("Failed to grant target role on update: guildID=%s, userID=%s, roleID=%s, error=%v", m.GuildID, m.User.ID, targetRoleID, err))
		} else {
			slog.Info(fmt.Sprintf("Granted target role on update: guildID=%s, userID=%s, roleID=%s", m.GuildID, m.User.ID, targetRoleID))
		}
	}
}

// calculateAccountAge calcula há quanto tempo a conta do Discord existe baseado no Snowflake ID
func (mes *MemberEventService) calculateAccountAge(userID string) time.Duration {
	// Discord Snowflake: (timestamp_ms - DISCORD_EPOCH) << 22
	const DISCORD_EPOCH = 1420070400000 // 01/01/2015 00:00:00 UTC em millisegundos

	// Converter string ID para uint64
	snowflake, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		slog.Warn(fmt.Sprintf("Failed to parse user ID for account age calculation: userID=%s, error=%v", userID, err))
		return 0
	}

	// Extrair timestamp do snowflake
	timestamp := (snowflake >> 22) + DISCORD_EPOCH
	accountCreated := time.Unix(int64(timestamp/1000), int64((timestamp%1000)*1000000))

	return time.Since(accountCreated)
}

// calculateServerTime tenta calcular há quanto tempo o usuário estava no servidor
// Agora usa múltiplas fontes em ordem: memória -> SQLite
func (mes *MemberEventService) calculateServerTime(guildID, userID string) time.Duration {
	// 1) memória (mais preciso no runtime)
	mes.joinMu.RLock()
	t, ok := mes.joinTimes[guildID+":"+userID]
	mes.joinMu.RUnlock()
	if ok && !t.IsZero() {
		return time.Since(t)
	}

	// 3) SQLite (novo repositório)
	if mes.store != nil {
		if t, ok, err := mes.store.GetMemberJoin(guildID, userID); err == nil && ok && !t.IsZero() {
			return time.Since(t)
		}
	}
	return 0
}

// cleanupLoop periodically removes old entries from joinTimes map
func (mes *MemberEventService) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mes.cleanupJoinTimes()
		case <-mes.cleanupStop:
			return
		}
	}
}

// cleanupJoinTimes removes entries older than 7 days from joinTimes map
func (mes *MemberEventService) cleanupJoinTimes() {
	if mes.joinTimes == nil {
		return
	}

	now := time.Now()
	threshold := 7 * 24 * time.Hour
	var toDelete []string

	// Collect keys to delete (can't delete while iterating)
	mes.joinMu.RLock()
	for key, joinTime := range mes.joinTimes {
		if now.Sub(joinTime) > threshold {
			toDelete = append(toDelete, key)
		}
	}
	mes.joinMu.RUnlock()

	// Delete old entries
	if len(toDelete) > 0 {
		mes.joinMu.Lock()
		for _, key := range toDelete {
			delete(mes.joinTimes, key)
		}
		mes.joinMu.Unlock()
	}

	if len(toDelete) > 0 {
		log.ApplicationLogger().Info("Cleaned up old join time entries from memory", "count", len(toDelete))
	}
}

func (mes *MemberEventService) markEvent() {
	if mes.store != nil {
		_ = mes.store.SetLastEvent(time.Now())
	}
}

// NEW: calcula há quanto tempo o bot está na guild (consulta Discord em tempo real)
func (mes *MemberEventService) getBotTimeOnServer(guildID string) time.Duration {
	if mes.session == nil || mes.session.State == nil || mes.session.State.User == nil {
		return 0
	}
	botID := mes.session.State.User.ID
	member, err := mes.session.GuildMember(guildID, botID)
	if err != nil || member == nil || member.JoinedAt.IsZero() {
		return 0
	}
	return time.Since(member.JoinedAt)
}
