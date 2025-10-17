package logging

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// MemberEventService gerencia eventos de entrada e saída de usuários
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

// Start registra os handlers de eventos de membros
func (mes *MemberEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("member event service is already running")
	}
	mes.isRunning = true

	// NEW: garantir mapa de joins
	if mes.joinTimes == nil {
		mes.joinTimes = make(map[string]time.Time)
	}

	// Store should be injected and already initialized
	if mes.store != nil {
		if err := mes.store.Init(); err != nil {
			log.Warn().Applicationf("Member event service: failed to initialize SQLite store (continuing): %v", err)
		}
	}

	mes.session.AddHandler(mes.handleGuildMemberAdd)
	mes.session.AddHandler(mes.handleGuildMemberRemove)

	// Start periodic cleanup of old joinTimes entries
	mes.cleanupStop = make(chan struct{})
	go mes.cleanupLoop()

	log.Info().Applicationf("Member event service started")
	return nil
}

// Stop para o serviço
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

	log.Info().Applicationf("Member event service stopped")
	return nil
}

// IsRunning retorna se o serviço está rodando
func (mes *MemberEventService) IsRunning() bool {
	return mes.isRunning
}

// handleGuildMemberAdd processa quando um usuário entra no servidor
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
		log.Info().Applicationf("User entry/leave channel not configured for guild, member join notification not sent: guildID=%s, userID=%s", m.GuildID, m.User.ID)
		return
	}

	// Calcular há quanto tempo a conta existe
	accountAge := mes.calculateAccountAge(m.User.ID)

	// Persistir também em SQLite (melhor esforço)
	if mes.store != nil {
		if member, err := mes.session.GuildMember(m.GuildID, m.User.ID); err == nil && !member.JoinedAt.IsZero() {
			_ = mes.store.UpsertMemberJoin(m.GuildID, m.User.ID, member.JoinedAt)
		}
	}

	// NEW: Registrar horário de entrada do membro (preciso) em memória
	if member, err := mes.session.GuildMember(m.GuildID, m.User.ID); err == nil && !member.JoinedAt.IsZero() {
		mes.joinMu.Lock()
		if mes.joinTimes == nil {
			mes.joinTimes = make(map[string]time.Time)
		}
		mes.joinTimes[m.GuildID+":"+m.User.ID] = member.JoinedAt
		mes.joinMu.Unlock()
	}

	log.Info().Applicationf("Member joined guild: guildID=%s, userID=%s, username=%s, accountAge=%s", m.GuildID, m.User.ID, m.User.Username, accountAge.String())

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberJoin(logChannelID, m, accountAge); err != nil {
			log.Error().Errorf("Failed to send member join notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err)
		} else {
			log.Info().Applicationf("Member join notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID)
		}
	} else if err := mes.notifier.SendMemberJoinNotification(logChannelID, m, accountAge); err != nil {
		log.Error().Errorf("Failed to send member join notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err)
	} else {
		log.Info().Applicationf("Member join notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID)
	}
}

// handleGuildMemberRemove processa quando um usuário sai do servidor
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
		log.Info().Applicationf("User entry/leave channel not configured for guild, member leave notification not sent: guildID=%s, userID=%s", m.GuildID, m.User.ID)
		return
	}

	// Calcular há quanto tempo estava no servidor
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

	log.Info().Applicationf("Member left guild: guildID=%s, userID=%s, username=%s, serverTime=%s, botTime=%s", m.GuildID, m.User.ID, m.User.Username, serverTime.String(), botTime.String())

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberLeave(logChannelID, m, serverTime, botTime); err != nil {
			log.Error().Errorf("Failed to send member leave notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err)
		} else {
			log.Info().Applicationf("Member leave notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID)
		}
	} else if err := mes.notifier.SendMemberLeaveNotification(logChannelID, m, serverTime, botTime); err != nil {
		log.Error().Errorf("Failed to send member leave notification: guildID=%s, userID=%s, channelID=%s, error=%v", m.GuildID, m.User.ID, logChannelID, err)
	} else {
		log.Info().Applicationf("Member leave notification sent successfully: guildID=%s, userID=%s, channelID=%s", m.GuildID, m.User.ID, logChannelID)
	}
}

// calculateAccountAge calcula há quanto tempo a conta do Discord existe baseado no Snowflake ID
func (mes *MemberEventService) calculateAccountAge(userID string) time.Duration {
	// Discord Snowflake: (timestamp_ms - DISCORD_EPOCH) << 22
	const DISCORD_EPOCH = 1420070400000 // 01/01/2015 00:00:00 UTC em millisegundos

	// Converter string ID para uint64
	snowflake, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		log.Warn().Applicationf("Failed to parse user ID for account age calculation: userID=%s, error=%v", userID, err)
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
		log.Info().Applicationf("Cleaned up %d old join time entries from memory", len(toDelete))
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
