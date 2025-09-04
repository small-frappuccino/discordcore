package logging

import (
	"fmt"
	"strings"
	"time"

	"github.com/alice-bnuy/discordcore/v2/internal/cache"
	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/alice-bnuy/discordcore/v2/internal/task"
	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

// CachedMessage armazena dados de mensagens para comparação
type CachedMessage struct {
	ID        string
	Content   string
	Author    *discordgo.User
	ChannelID string
	GuildID   string
	Timestamp time.Time
}

// MessageEventService gerencia eventos de mensagens (deletar/editar)
type MessageEventService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	notifier      *NotificationSender
	adapters      *task.NotificationAdapters
	cache         *cache.TTLMap
	isRunning     bool
}

// NewMessageEventService cria uma nova instância do serviço de eventos de mensagens
func NewMessageEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender) *MessageEventService {
	return &MessageEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		cache:         cache.NewTTLMap("message_events", 24*time.Hour, 1*time.Hour, 0),
		isRunning:     false,
	}
}

// Start registra os handlers de eventos de mensagens
func (mes *MessageEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("message event service is already running")
	}
	mes.isRunning = true

	mes.session.AddHandler(mes.handleMessageCreate)
	mes.session.AddHandler(mes.handleMessageUpdate)
	mes.session.AddHandler(mes.handleMessageDelete)

	// TTL cache handles cleanup internally

	logutil.Info("Message event service started")
	return nil
}

// Stop para o serviço
func (mes *MessageEventService) Stop() error {
	if !mes.isRunning {
		return fmt.Errorf("message event service is not running")
	}
	mes.isRunning = false

	if mes.cache != nil {
		mes.cache.Close()
	}

	logutil.Info("Message event service stopped")
	return nil
}

// IsRunning retorna se o serviço está rodando
func (mes *MessageEventService) IsRunning() bool {
	return mes.isRunning
}

// GetCache expõe o cache TTL usado pelo serviço para integração com agregadores
func (mes *MessageEventService) GetCache() cache.CacheManager {
	return mes.cache
}

// handleMessageCreate armazena mensagens no cache para futuras comparações
func (mes *MessageEventService) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Author == nil || m.Author.Bot || m.Content == "" {
		return
	}

	// Verificar se é uma mensagem de guild
	channel, err := s.Channel(m.ChannelID)
	if err != nil || channel.GuildID == "" {
		return // Ignorar DMs
	}

	// Verificar se o guild está configurado
	guildConfig := mes.configManager.GuildConfig(channel.GuildID)
	if guildConfig == nil {
		return
	}

	// Armazenar no cache (TTL 24h via default)
	key := channel.GuildID + ":" + m.ID
	_ = mes.cache.Set(key, &CachedMessage{
		ID:        m.ID,
		Content:   m.Content,
		Author:    m.Author,
		ChannelID: m.ChannelID,
		GuildID:   channel.GuildID,
		Timestamp: m.Timestamp,
	}, 0)

	logutil.WithFields(map[string]interface{}{
		"guildID":   channel.GuildID,
		"channelID": m.ChannelID,
		"messageID": m.ID,
		"userID":    m.Author.ID,
	}).Debug("Message cached for monitoring")
}

// handleMessageUpdate processa edições de mensagens
func (mes *MessageEventService) handleMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m == nil || m.Author == nil || m.Author.Bot {
		return
	}

	// Verificar se temos a mensagem original no cache
	key := m.GuildID + ":" + m.ID
	v, exists := mes.cache.Get(key)
	var cached *CachedMessage
	if exists {
		cached, _ = v.(*CachedMessage)
	}

	if !exists || cached == nil {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"userID":    m.Author.ID,
		}).Debug("Message edit detected but original not in cache")
		return
	}

	// Verificar se realmente mudou o conteúdo
	if cached.Content == m.Content {
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		return
	}

	logChannelID := guildConfig.MessageLogChannelID
	if logChannelID == "" {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
		}).Debug("MessageLogChannelID not configured for guild, message edit notification not sent")
		return
	}

	logutil.WithFields(map[string]interface{}{
		"guildID":   cached.GuildID,
		"channelID": cached.ChannelID,
		"messageID": m.ID,
		"userID":    cached.Author.ID,
		"username":  cached.Author.Username,
	}).Info("Message edit detected")

	// Enviar notificação de edição
	if mes.adapters != nil {
		tCached := &task.CachedMessage{
			ID:        cached.ID,
			Content:   cached.Content,
			Author:    cached.Author,
			ChannelID: cached.ChannelID,
			GuildID:   cached.GuildID,
			Timestamp: cached.Timestamp,
		}
		if err := mes.adapters.EnqueueMessageEdit(logChannelID, tCached, m); err != nil {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
				"error":     err,
			}).Error("Failed to send message edit notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
			}).Info("Message edit notification sent successfully")
		}
	} else {
		tCached := &task.CachedMessage{
			ID:        cached.ID,
			Content:   cached.Content,
			Author:    cached.Author,
			ChannelID: cached.ChannelID,
			GuildID:   cached.GuildID,
			Timestamp: cached.Timestamp,
		}
		if err := mes.notifier.SendMessageEditNotification(logChannelID, tCached, m); err != nil {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
				"error":     err,
			}).Error("Failed to send message edit notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
			}).Info("Message edit notification sent successfully")
		}
	}

	// Atualizar cache com novo conteúdo
	_ = mes.cache.Set(key, &CachedMessage{
		ID:        cached.ID,
		Content:   m.Content,
		Author:    cached.Author,
		ChannelID: cached.ChannelID,
		GuildID:   cached.GuildID,
		Timestamp: cached.Timestamp,
	}, 0)
}

// handleMessageDelete processa deleções de mensagens
func (mes *MessageEventService) handleMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	if m == nil {
		return
	}

	key := m.GuildID + ":" + m.ID
	v, exists := mes.cache.Get(key)
	var cached *CachedMessage
	if exists {
		cached, _ = v.(*CachedMessage)
	}

	if !exists {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"channelID": m.ChannelID,
		}).Debug("Message delete detected but original not in cache")
		return
	}

	// Pular se for bot
	if cached.Author.Bot {
		_ = mes.cache.Delete(m.ID)
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		_ = mes.cache.Delete(m.ID)
		return
	}

	logChannelID := guildConfig.MessageLogChannelID
	if logChannelID == "" {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
		}).Debug("MessageLogChannelID not configured for guild, message delete notification not sent")
		_ = mes.cache.Delete(m.ID)
		return
	}

	logutil.WithFields(map[string]interface{}{
		"guildID":   cached.GuildID,
		"channelID": cached.ChannelID,
		"messageID": m.ID,
		"userID":    cached.Author.ID,
		"username":  cached.Author.Username,
	}).Info("Message delete detected")

	// Tentar determinar quem deletou (limitado pela API do Discord)
	deletedBy := "Usuário" // Padrão - assumimos que foi o próprio usuário
	// TODO: Implementar auditlog check para detectar se foi um moderador

	// Enviar notificação de deleção
	if mes.adapters != nil {
		tCached := &task.CachedMessage{
			ID:        cached.ID,
			Content:   cached.Content,
			Author:    cached.Author,
			ChannelID: cached.ChannelID,
			GuildID:   cached.GuildID,
			Timestamp: cached.Timestamp,
		}
		if err := mes.adapters.EnqueueMessageDelete(logChannelID, tCached, deletedBy); err != nil {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
				"error":     err,
			}).Error("Failed to send message delete notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
			}).Info("Message delete notification sent successfully")
		}
	} else {
		tCached := &task.CachedMessage{
			ID:        cached.ID,
			Content:   cached.Content,
			Author:    cached.Author,
			ChannelID: cached.ChannelID,
			GuildID:   cached.GuildID,
			Timestamp: cached.Timestamp,
		}
		if err := mes.notifier.SendMessageDeleteNotification(logChannelID, tCached, deletedBy); err != nil {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
				"error":     err,
			}).Error("Failed to send message delete notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
			}).Info("Message delete notification sent successfully")
		}
	}

	// Remover do cache
	_ = mes.cache.Delete(m.ID)
}

// TTL cache handles expiration; explicit cleanup routine removed

// GetCacheStats retorna estatísticas do cache para debugging
func (mes *MessageEventService) GetCacheStats() map[string]interface{} {
	stats := mes.cache.Stats()

	result := map[string]interface{}{
		"totalCached": stats.TotalEntries,
		"isRunning":   mes.isRunning,
		"hitRate":     stats.HitRate,
		"memoryUsage": stats.MemoryUsage,
	}

	// Contar mensagens por guild (melhor esforço)
	guildCounts := make(map[string]int)
	for _, key := range mes.cache.Keys() {
		if idx := strings.IndexByte(key, ':'); idx > 0 {
			guildID := key[:idx]
			guildCounts[guildID]++
		}
	}
	result["perGuild"] = guildCounts

	return result
}

func (mes *MessageEventService) SetAdapters(adapters *task.NotificationAdapters) {
	mes.adapters = adapters
}
