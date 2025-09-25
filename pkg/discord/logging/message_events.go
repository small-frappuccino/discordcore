package logging

import (
	"fmt"
	"time"

	"github.com/alice-bnuy/discordcore/pkg/files"
	logutil "github.com/alice-bnuy/discordcore/pkg/logging"
	"github.com/alice-bnuy/discordcore/pkg/storage"
	"github.com/alice-bnuy/discordcore/pkg/task"
	"github.com/alice-bnuy/discordcore/pkg/util"
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
	store         *storage.Store
	isRunning     bool
}

// NewMessageEventService cria uma nova instância do serviço de eventos de mensagens
func NewMessageEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender) *MessageEventService {
	return &MessageEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		store:         storage.NewStore(util.GetMessageDBPath()),
		isRunning:     false,
	}
}

// Start registra os handlers de eventos de mensagens
func (mes *MessageEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("message event service is already running")
	}
	mes.isRunning = true

	// Inicializa a persistência (SQLite) e limpa expirados (melhor esforço)
	if mes.store != nil {
		if err := mes.store.Init(); err != nil {
			logutil.WithError(err).Warn("Message event service: failed to initialize SQLite store (continuing without persistence)")
		} else {
			_ = mes.store.CleanupExpiredMessages()
		}
	}

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

	logutil.Info("Message event service stopped")
	return nil
}

// IsRunning retorna se o serviço está rodando
func (mes *MessageEventService) IsRunning() bool {
	return mes.isRunning
}

// handleMessageCreate armazena mensagens no cache para futuras comparações
func (mes *MessageEventService) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil {
		logutil.Debug("MessageCreate: nil event")
		return
	}
	if m.Author == nil {
		logutil.WithFields(map[string]interface{}{
			"channelID": m.ChannelID,
		}).Debug("MessageCreate: nil author; skipping")
		return
	}
	if m.Author.Bot {
		logutil.WithFields(map[string]interface{}{
			"channelID": m.ChannelID,
			"userID":    m.Author.ID,
		}).Debug("MessageCreate: ignoring bot message")
		return
	}
	if m.Content == "" {
		logutil.WithFields(map[string]interface{}{
			"channelID": m.ChannelID,
			"userID":    m.Author.ID,
		}).Debug("MessageCreate: empty content; will not cache")
		return
	}
	logutil.WithFields(map[string]interface{}{
		"channelID": m.ChannelID,
		"userID":    m.Author.ID,
		"messageID": m.ID,
	}).Debug("MessageCreate received")

	// Verificar se é uma mensagem de guild
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		logutil.WithFields(map[string]interface{}{
			"channelID": m.ChannelID,
			"error":     err.Error(),
		}).Debug("MessageCreate: failed to fetch channel; skipping cache")
		return
	}
	if channel.GuildID == "" {
		logutil.WithFields(map[string]interface{}{
			"channelID": m.ChannelID,
		}).Debug("MessageCreate: DM detected; skipping cache")
		return
	}

	// Verificar se o guild está configurado
	guildConfig := mes.configManager.GuildConfig(channel.GuildID)
	if guildConfig == nil {
		logutil.WithFields(map[string]interface{}{
			"guildID": channel.GuildID,
		}).Debug("MessageCreate: no guild config; skipping cache")
		return
	}

	mes.markEvent()

	// Persistir em SQLite (write-through; melhor esforço)
	if mes.store != nil && m.Author != nil {
		_ = mes.store.UpsertMessage(storage.MessageRecord{
			GuildID:        channel.GuildID,
			MessageID:      m.ID,
			ChannelID:      m.ChannelID,
			AuthorID:       m.Author.ID,
			AuthorUsername: m.Author.Username,
			AuthorAvatar:   m.Author.Avatar,
			Content:        m.Content,
			CachedAt:       time.Now(),
			ExpiresAt:      time.Now().Add(24 * time.Hour),
			HasExpiry:      true,
		})
	}

	logutil.WithFields(map[string]interface{}{
		"guildID":   channel.GuildID,
		"channelID": m.ChannelID,
		"messageID": m.ID,
		"userID":    m.Author.ID,
	}).Debug("Message cached for monitoring")
}

// handleMessageUpdate processa edições de mensagens
func (mes *MessageEventService) handleMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m == nil {
		logutil.Debug("MessageUpdate: nil event")
		return
	}
	if m.Author == nil {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"channelID": m.ChannelID,
		}).Debug("MessageUpdate: nil author; skipping")
		return
	}
	if m.Author.Bot {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"userID":    m.Author.ID,
			"channelID": m.ChannelID,
		}).Debug("MessageUpdate: ignoring bot edit")
		return
	}
	logutil.WithFields(map[string]interface{}{
		"messageID": m.ID,
		"userID":    m.Author.ID,
		"guildID":   m.GuildID,
		"channelID": m.ChannelID,
	}).Debug("MessageUpdate received")

	mes.markEvent()

	// Consultar persistência (SQLite) para obter a mensagem original
	var cached *CachedMessage
	if mes.store != nil && m.GuildID != "" {
		if rec, err := mes.store.GetMessage(m.GuildID, m.ID); err == nil && rec != nil {
			cached = &CachedMessage{
				ID:        rec.MessageID,
				Content:   rec.Content,
				Author:    &discordgo.User{ID: rec.AuthorID, Username: rec.AuthorUsername, Avatar: rec.AuthorAvatar},
				ChannelID: rec.ChannelID,
				GuildID:   rec.GuildID,
				Timestamp: rec.CachedAt,
			}
		}
	}

	if cached == nil {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"userID":    m.Author.ID,
		}).Debug("Message edit detected but original not in cache/persistence")
		return
	}

	// Verificar se realmente mudou o conteúdo
	if cached.Content == m.Content {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"channelID": cached.ChannelID,
			"messageID": m.ID,
			"userID":    cached.Author.ID,
		}).Debug("MessageUpdate: content unchanged; skipping notification")
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
		}).Debug("MessageUpdate: no guild config; skipping notification")
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
				"error":     err.Error(),
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
				"error":     err.Error(),
			}).Error("Failed to send message edit notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
			}).Info("Message edit notification sent successfully")
		}
	}

	// Atualizar persistência com novo conteúdo
	updated := &CachedMessage{
		ID:        cached.ID,
		Content:   m.Content,
		Author:    cached.Author,
		ChannelID: cached.ChannelID,
		GuildID:   cached.GuildID,
		Timestamp: cached.Timestamp,
	}
	if mes.store != nil && updated.Author != nil {
		_ = mes.store.UpsertMessage(storage.MessageRecord{
			GuildID:        updated.GuildID,
			MessageID:      updated.ID,
			ChannelID:      updated.ChannelID,
			AuthorID:       updated.Author.ID,
			AuthorUsername: updated.Author.Username,
			AuthorAvatar:   updated.Author.Avatar,
			Content:        updated.Content,
			CachedAt:       time.Now(),
			ExpiresAt:      time.Now().Add(24 * time.Hour),
			HasExpiry:      true,
		})
	}
	logutil.WithFields(map[string]interface{}{
		"guildID":   cached.GuildID,
		"channelID": cached.ChannelID,
		"messageID": m.ID,
	}).Debug("MessageUpdate: store updated with new content")
}

// handleMessageDelete processa deleções de mensagens
func (mes *MessageEventService) handleMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	if m == nil {
		return
	}

	var cached *CachedMessage

	mes.markEvent()

	if mes.store != nil && m.GuildID != "" {
		if rec, err := mes.store.GetMessage(m.GuildID, m.ID); err == nil && rec != nil {
			cached = &CachedMessage{
				ID:        rec.MessageID,
				Content:   rec.Content,
				Author:    &discordgo.User{ID: rec.AuthorID, Username: rec.AuthorUsername, Avatar: rec.AuthorAvatar},
				ChannelID: rec.ChannelID,
				GuildID:   rec.GuildID,
				Timestamp: rec.CachedAt,
			}
		}
	}

	if cached == nil {
		logutil.WithFields(map[string]interface{}{
			"messageID": m.ID,
			"channelID": m.ChannelID,
		}).Debug("Message delete detected but original not in cache/persistence")
		return
	}

	// Pular se for bot
	if cached.Author.Bot {
		// no-op: cache removed; using SQLite only
		if mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		// no-op: cache removed; using SQLite only
		if mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return
	}

	logChannelID := guildConfig.MessageLogChannelID
	if logChannelID == "" {
		logutil.WithFields(map[string]interface{}{
			"guildID":   cached.GuildID,
			"messageID": m.ID,
		}).Debug("MessageLogChannelID not configured for guild, message delete notification not sent")
		// no-op: cache removed; using SQLite only
		if mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
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
				"error":     err.Error(),
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
				"error":     err.Error(),
			}).Error("Failed to send message delete notification")
		} else {
			logutil.WithFields(map[string]interface{}{
				"guildID":   cached.GuildID,
				"messageID": m.ID,
				"channelID": logChannelID,
			}).Info("Message delete notification sent successfully")
		}
	}

	// Remover do cache e persistência
	// no-op: cache removed; using SQLite only
	if mes.store != nil {
		_ = mes.store.DeleteMessage(m.GuildID, m.ID)
	}
}

// Persistent storage (SQLite) handles expiration and cleanup

// GetCacheStats retorna estatísticas do cache para debugging
func (mes *MessageEventService) markEvent() {
	if mes.store != nil {
		_ = mes.store.SetLastEvent(time.Now())
	}
}

func (mes *MessageEventService) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"isRunning": mes.isRunning,
		"backend":   "sqlite",
	}
}

func (mes *MessageEventService) SetAdapters(adapters *task.NotificationAdapters) {
	mes.adapters = adapters
}
