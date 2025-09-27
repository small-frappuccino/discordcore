package logging

import (
	"fmt"
	"time"

	"github.com/alice-bnuy/discordcore/pkg/files"
	"github.com/alice-bnuy/discordcore/pkg/log"
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
			log.Warn().Applicationf("Message event service: failed to initialize SQLite store (continuing without persistence): %v", err)
		} else {
			_ = mes.store.CleanupExpiredMessages()
		}
	}

	mes.session.AddHandler(mes.handleMessageCreate)
	mes.session.AddHandler(mes.handleMessageUpdate)
	mes.session.AddHandler(mes.handleMessageDelete)

	// TTL cache handles cleanup internally

	log.Info().Applicationf("Message event service started")
	return nil
}

// Stop para o serviço
func (mes *MessageEventService) Stop() error {
	if !mes.isRunning {
		return fmt.Errorf("message event service is not running")
	}
	mes.isRunning = false

	log.Info().Applicationf("Message event service stopped")
	return nil
}

// IsRunning retorna se o serviço está rodando
func (mes *MessageEventService) IsRunning() bool {
	return mes.isRunning
}

// handleMessageCreate armazena mensagens no cache para futuras comparações
func (mes *MessageEventService) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil {
		log.Info().Applicationf("DEBUG: MessageCreate: nil event")
		return
	}
	if m.Author == nil {
		log.Info().Applicationf("MessageCreate: nil author; skipping: channelID=%s", m.ChannelID)
		return
	}
	if m.Author.Bot {
		log.Info().Applicationf("MessageCreate: ignoring bot message: channelID=%s, userID=%s", m.ChannelID, m.Author.ID)
		return
	}
	if m.Content == "" {
		log.Info().Applicationf("MessageCreate: empty content; will not cache: channelID=%s, userID=%s", m.ChannelID, m.Author.ID)
		return
	}
	log.Info().Applicationf("MessageCreate received: channelID=%s, userID=%s, messageID=%s", m.ChannelID, m.Author.ID, m.ID)

	// Verificar se é uma mensagem de guild
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		log.Info().Applicationf("MessageCreate: failed to fetch channel; skipping cache: channelID=%s, error=%v", m.ChannelID, err)
		return
	}
	if channel.GuildID == "" {
		log.Info().Applicationf("MessageCreate: DM detected; skipping cache: channelID=%s", m.ChannelID)
		return
	}

	// Verificar se o guild está configurado
	guildConfig := mes.configManager.GuildConfig(channel.GuildID)
	if guildConfig == nil {
		log.Info().Applicationf("MessageCreate: no guild config; skipping cache: guildID=%s", channel.GuildID)
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

	log.Info().Applicationf("Message cached for monitoring: guildID=%s, channelID=%s, messageID=%s, userID=%s", channel.GuildID, m.ChannelID, m.ID, m.Author.ID)
}

// handleMessageUpdate processa edições de mensagens
func (mes *MessageEventService) handleMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m == nil {
		log.Info().Applicationf("DEBUG: MessageUpdate: nil event")
		return
	}
	if m.Author == nil {
		log.Info().Applicationf("MessageUpdate: nil author; skipping: messageID=%s, channelID=%s", m.ID, m.ChannelID)
		return
	}
	if m.Author.Bot {
		log.Info().Applicationf("MessageUpdate: ignoring bot edit: messageID=%s, userID=%s, channelID=%s", m.ID, m.Author.ID, m.ChannelID)
		return
	}
	log.Info().Applicationf("MessageUpdate received: messageID=%s, userID=%s, guildID=%s, channelID=%s", m.ID, m.Author.ID, m.GuildID, m.ChannelID)

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
		log.Info().Applicationf("Message edit detected but original not in cache/persistence: messageID=%s, userID=%s", m.ID, m.Author.ID)
		return
	}

	// Verificar se realmente mudou o conteúdo
	if cached.Content == m.Content {
		log.Info().Applicationf("MessageUpdate: content unchanged; skipping notification: guildID=%s, channelID=%s, messageID=%s, userID=%s", cached.GuildID, cached.ChannelID, m.ID, cached.Author.ID)
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		log.Info().Applicationf("MessageUpdate: no guild config; skipping notification: guildID=%s, messageID=%s", cached.GuildID, m.ID)
		return
	}

	logChannelID := guildConfig.MessageLogChannelID
	if logChannelID == "" {
		log.Info().Applicationf("MessageLogChannelID not configured for guild, message edit notification not sent: guildID=%s, messageID=%s", cached.GuildID, m.ID)
		return
	}

	log.Info().Applicationf("Message edit detected: guildID=%s, channelID=%s, messageID=%s, userID=%s, username=%s", cached.GuildID, cached.ChannelID, m.ID, cached.Author.ID, cached.Author.Username)

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
			log.Error().Errorf("Failed to send message edit notification: guildID=%s, messageID=%s, channelID=%s, error=%v", cached.GuildID, m.ID, logChannelID, err)
		} else {
			log.Info().Applicationf("Message edit notification sent successfully: guildID=%s, messageID=%s, channelID=%s", cached.GuildID, m.ID, logChannelID)
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
			log.Error().Errorf("Failed to send message edit notification: guildID=%s, messageID=%s, channelID=%s, error=%v", cached.GuildID, m.ID, logChannelID, err)
		} else {
			log.Info().Applicationf("Message edit notification sent successfully: guildID=%s, messageID=%s, channelID=%s", cached.GuildID, m.ID, logChannelID)
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
	log.Info().Applicationf("MessageUpdate: store updated with new content: guildID=%s, channelID=%s, messageID=%s", cached.GuildID, cached.ChannelID, m.ID)
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
		log.Info().Applicationf("Message delete detected but original not in cache/persistence: messageID=%s, channelID=%s", m.ID, m.ChannelID)
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
		log.Info().Applicationf("MessageLogChannelID not configured for guild, message edit notification not sent: guildID=%s, messageID=%s", cached.GuildID, m.ID)
		// no-op: cache removed; using SQLite only
		if mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return
	}

	log.Info().Applicationf("Message delete detected: guildID=%s, channelID=%s, messageID=%s, userID=%s, username=%s", cached.GuildID, cached.ChannelID, m.ID, cached.Author.ID, cached.Author.Username)

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
			log.Error().Errorf("Failed to send message delete notification: guildID=%s, messageID=%s, channelID=%s, error=%v", cached.GuildID, m.ID, logChannelID, err)
		} else {
			log.Info().Applicationf("Message delete notification sent successfully: guildID=%s, messageID=%s, channelID=%s", cached.GuildID, m.ID, logChannelID)
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
			log.Error().Errorf("Failed to send message delete notification: guildID=%s, messageID=%s, channelID=%s, error=%v", cached.GuildID, m.ID, logChannelID, err)
		} else {
			log.Info().Applicationf("Message delete notification sent successfully: guildID=%s, messageID=%s, channelID=%s", cached.GuildID, m.ID, logChannelID)
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
