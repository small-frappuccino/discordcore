package logging

import (
	"fmt"
	"strconv"
	"time"

	"github.com/alice-bnuy/discordcore/v2/pkg/cache"
	"github.com/alice-bnuy/discordcore/v2/pkg/files"
	"github.com/alice-bnuy/discordcore/v2/pkg/task"
	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

// MemberEventService gerencia eventos de entrada e saída de usuários
type MemberEventService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	notifier      *NotificationSender
	adapters      *task.NotificationAdapters
	isRunning     bool

	// Cache para tempos de entrada (membro e bot)
	cacheManager *cache.AvatarCacheManager
	joinTimes    map[string]time.Time // chave: guildID:userID
}

// NewMemberEventService cria uma nova instância do serviço de eventos de membros
func NewMemberEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender) *MemberEventService {
	return &MemberEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		isRunning:     false,
	}

}

func (mes *MemberEventService) SetAdapters(adapters *task.NotificationAdapters) {
	mes.adapters = adapters
	// NEW: injetar cache manager a partir dos adapters
	if adapters != nil {
		mes.cacheManager = adapters.Cache
	}
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

	mes.session.AddHandler(mes.handleGuildMemberAdd)
	mes.session.AddHandler(mes.handleGuildMemberRemove)

	logutil.Info("Member event service started")
	return nil
}

// Stop para o serviço
func (mes *MemberEventService) Stop() error {
	if !mes.isRunning {
		return fmt.Errorf("member event service is not running")
	}
	mes.isRunning = false
	logutil.Info("Member event service stopped")
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
		logutil.WithFields(map[string]interface{}{
			"guildID": m.GuildID,
			"userID":  m.User.ID,
		}).Debug("User entry/leave channel not configured for guild, member join notification not sent")
		return
	}

	// Calcular há quanto tempo a conta existe
	accountAge := mes.calculateAccountAge(m.User.ID)
	// Registrar horário preciso de entrada do membro no cache (se possível)
	if mes.cacheManager != nil {
		if member, err := mes.session.GuildMember(m.GuildID, m.User.ID); err == nil && !member.JoinedAt.IsZero() {
			mes.cacheManager.RecordMemberJoin(m.GuildID, m.User.ID, member.JoinedAt)
		}
	}

	// NEW: Registrar horário de entrada do membro (preciso) em memória
	if member, err := mes.session.GuildMember(m.GuildID, m.User.ID); err == nil && !member.JoinedAt.IsZero() {
		if mes.joinTimes == nil {
			mes.joinTimes = make(map[string]time.Time)
		}
		mes.joinTimes[m.GuildID+":"+m.User.ID] = member.JoinedAt
	}

	logutil.WithFields(map[string]interface{}{
		"guildID":    m.GuildID,
		"userID":     m.User.ID,
		"username":   m.User.Username,
		"accountAge": accountAge.String(),
	}).Info("Member joined guild")

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberJoin(logChannelID, m, accountAge); err != nil {
			logutil.WithFields(map[string]interface{}{
				"guildID":   m.GuildID,
				"userID":    m.User.ID,
				"channelID": logChannelID,
				"error":     err,
			}).Error("Failed to send member join notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   m.GuildID,
				"userID":    m.User.ID,
				"channelID": logChannelID,
			}).Info("Member join notification sent successfully")
		}
	} else if err := mes.notifier.SendMemberJoinNotification(logChannelID, m, accountAge); err != nil {
		logutil.WithFields(map[string]interface{}{
			"guildID":   m.GuildID,
			"userID":    m.User.ID,
			"channelID": logChannelID,
			"error":     err,
		}).Error("Failed to send member join notification")
	} else {
		logutil.WithFields(map[string]interface{}{
			"guildID":   m.GuildID,
			"userID":    m.User.ID,
			"channelID": logChannelID,
		}).Info("Member join notification sent successfully")
	}
}

// handleGuildMemberRemove processa quando um usuário sai do servidor
func (mes *MemberEventService) handleGuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}

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
		logutil.WithFields(map[string]interface{}{
			"guildID": m.GuildID,
			"userID":  m.User.ID,
		}).Debug("User entry/leave channel not configured for guild, member leave notification not sent")
		return
	}

	// Calcular há quanto tempo estava no servidor
	var serverTime time.Duration
	if t, ok := mes.joinTimes[m.GuildID+":"+m.User.ID]; ok && !t.IsZero() {
		serverTime = time.Since(t)
	} else {
		serverTime = mes.calculateServerTime(m.GuildID, m.User.ID)
	}

	botTime := mes.getBotTimeOnServer(m.GuildID)

	logutil.WithFields(map[string]interface{}{
		"guildID":    m.GuildID,
		"userID":     m.User.ID,
		"username":   m.User.Username,
		"serverTime": serverTime.String(),
		"botTime":    botTime.String(),
	}).Info("Member left guild")

	if mes.adapters != nil {
		if err := mes.adapters.EnqueueMemberLeave(logChannelID, m, serverTime, botTime); err != nil {
			logutil.WithFields(map[string]interface{}{
				"guildID":   m.GuildID,
				"userID":    m.User.ID,
				"channelID": logChannelID,
				"error":     err,
			}).Error("Failed to send member leave notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   m.GuildID,
				"userID":    m.User.ID,
				"channelID": logChannelID,
			}).Info("Member leave notification sent successfully")
		}
	} else if err := mes.notifier.SendMemberLeaveNotification(logChannelID, m, serverTime, botTime); err != nil {
		logutil.WithFields(map[string]interface{}{
			"guildID":   m.GuildID,
			"userID":    m.User.ID,
			"channelID": logChannelID,
			"error":     err,
		}).Error("Failed to send member leave notification")
	} else {
		logutil.WithFields(map[string]interface{}{
			"guildID":   m.GuildID,
			"userID":    m.User.ID,
			"channelID": logChannelID,
		}).Info("Member leave notification sent successfully")
	}
}

// calculateAccountAge calcula há quanto tempo a conta do Discord existe baseado no Snowflake ID
func (mes *MemberEventService) calculateAccountAge(userID string) time.Duration {
	// Discord Snowflake: (timestamp_ms - DISCORD_EPOCH) << 22
	const DISCORD_EPOCH = 1420070400000 // 01/01/2015 00:00:00 UTC em millisegundos

	// Converter string ID para uint64
	snowflake, err := strconv.ParseUint(userID, 10, 64)
	if err != nil {
		logutil.WithFields(map[string]interface{}{
			"userID": userID,
			"error":  err,
		}).Warn("Failed to parse user ID for account age calculation")
		return 0
	}

	// Extrair timestamp do snowflake
	timestamp := (snowflake >> 22) + DISCORD_EPOCH
	accountCreated := time.Unix(int64(timestamp/1000), int64((timestamp%1000)*1000000))

	return time.Since(accountCreated)
}

// calculateServerTime tenta calcular há quanto tempo o usuário estava no servidor
// Nota: isso é limitado pois não temos histórico persistente, então retorna 0 (tempo desconhecido)
// Em uma implementação mais avançada, você poderia salvar dados de join em um banco de dados
func (mes *MemberEventService) calculateServerTime(guildID, userID string) time.Duration {
	// Como não temos dados históricos persistentes do tempo de entrada no servidor,
	// retornamos 0 para indicar que não sabemos
	// TODO: Implementar persistência de dados de entrada para cálculo preciso
	return 0
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
