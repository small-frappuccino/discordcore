package logging

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
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
func NewMessageEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender, store *storage.Store) *MessageEventService {
	return &MessageEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		store:         store,
		isRunning:     false,
	}
}

// Start registra os handlers de eventos de mensagens
func (mes *MessageEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("message event service is already running")
	}
	mes.isRunning = true

	// Store should be injected and already initialized
	// Clean up expired messages (best effort)
	if mes.store != nil {
		_ = mes.store.CleanupExpiredMessages()
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
		// Build a concise summary for non-text messages so we can still cache deletes/edits
		extra := ""
		if len(m.Attachments) > 0 {
			extra += fmt.Sprintf("[attachments: %d] ", len(m.Attachments))
		}
		if len(m.Embeds) > 0 {
			extra += fmt.Sprintf("[embeds: %d] ", len(m.Embeds))
		}
		if len(m.StickerItems) > 0 {
			extra += fmt.Sprintf("[stickers: %d] ", len(m.StickerItems))
		}
		if extra == "" {
			log.Info().Applicationf("MessageCreate: empty content; will not cache: channelID=%s, userID=%s", m.ChannelID, m.Author.ID)
			return
		}
		// Use the summary as content for persistence
		m.Content = extra
		log.Info().Applicationf("MessageCreate: content empty; using summary for cache: channelID=%s, userID=%s", m.ChannelID, m.Author.ID)
	}
	log.Info().Applicationf("MessageCreate received: channelID=%s, userID=%s, messageID=%s", m.ChannelID, m.Author.ID, m.ID)

	// Verificar se é uma mensagem de guild sem buscar o canal quando possível
	guildID := m.GuildID
	if guildID == "" {
		// Fallback: obter via canal apenas se necessário (provável DM)
		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			log.Info().Applicationf("MessageCreate: failed to fetch channel; skipping cache: channelID=%s, error=%v", m.ChannelID, err)
			return
		}
		guildID = channel.GuildID
	}
	if guildID == "" {
		log.Info().Applicationf("MessageCreate: DM detected; skipping cache: channelID=%s", m.ChannelID)
		return
	}

	// Verificar se o guild está configurado
	guildConfig := mes.configManager.GuildConfig(guildID)
	if guildConfig == nil {
		log.Info().Applicationf("MessageCreate: no guild config; skipping cache: guildID=%s", guildID)
		return
	}

	mes.markEvent()

	// Persistir em SQLite (write-through; melhor esforço)
	if mes.store != nil && m.Author != nil {
		_ = mes.store.UpsertMessage(storage.MessageRecord{
			GuildID:        guildID,
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

	log.Info().Applicationf("Message cached for monitoring: guildID=%s, channelID=%s, messageID=%s, userID=%s", guildID, m.ChannelID, m.ID, m.Author.ID)
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

	// Ensure latest content; MessageUpdate may omit content. Also enrich empty content with context.
	if m.Content == "" {
		if msg, err := s.ChannelMessage(m.ChannelID, m.ID); err == nil && msg != nil {
			m.Content = msg.Content
			// Enrich only when original content is empty (e.g., attachments-only messages)
			m.Content = mes.summarizeMessageContent(msg, m.Content)
		}
	}
	// Verificar se realmente mudou o conteúdo (compare effective strings)
	if cached.Content == m.Content {
		log.Info().Applicationf("MessageUpdate: content unchanged; skipping notification: guildID=%s, channelID=%s, messageID=%s, userID=%s", cached.GuildID, cached.ChannelID, m.ID, cached.Author.ID)
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		log.Info().Applicationf("MessageUpdate: no guild config; skipping notification: guildID=%s, messageID=%s", cached.GuildID, m.ID)
		return
	}

	logChannelID := mes.fallbackMessageLogChannel(guildConfig)
	if logChannelID == "" {
		log.Info().Applicationf("Message log channel not configured for guild; edit notification not sent: guildID=%s, messageID=%s", cached.GuildID, m.ID)
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

	logChannelID := mes.fallbackMessageLogChannel(guildConfig)
	if logChannelID == "" {
		log.Info().Applicationf("Message log channel not configured for guild; delete notification not sent: guildID=%s, messageID=%s", cached.GuildID, m.ID)
		// no-op: cache removed; using SQLite only
		if mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return
	}

	log.Info().Applicationf("Message delete detected: guildID=%s, channelID=%s, messageID=%s, userID=%s, username=%s", cached.GuildID, cached.ChannelID, m.ID, cached.Author.ID, cached.Author.Username)

	// Tentar determinar quem deletou (melhor esforço via audit log)
	deletedBy := mes.determineDeletedBy(s, cached.GuildID, cached.ChannelID, cached.Author.ID)

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

// fallbackMessageLogChannel chooses the best available channel for message logs.
func (mes *MessageEventService) fallbackMessageLogChannel(g *files.GuildConfig) string {
	if g == nil {
		return ""
	}
	if g.MessageLogChannelID != "" {
		return g.MessageLogChannelID
	}
	if g.UserLogChannelID != "" {
		return g.UserLogChannelID
	}
	if g.CommandChannelID != "" {
		return g.CommandChannelID
	}
	return ""
}

// summarizeMessageContent enriches content with a concise summary when the message has
// non-textual elements and content is otherwise empty.
func (mes *MessageEventService) summarizeMessageContent(msg *discordgo.Message, base string) string {
	if msg == nil {
		return base
	}
	extra := ""
	if len(msg.Attachments) > 0 {
		extra += fmt.Sprintf("[attachments: %d] ", len(msg.Attachments))
	}
	if len(msg.Embeds) > 0 {
		extra += fmt.Sprintf("[embeds: %d] ", len(msg.Embeds))
	}
	if len(msg.StickerItems) > 0 {
		extra += fmt.Sprintf("[stickers: %d] ", len(msg.StickerItems))
	}
	if extra == "" {
		return base
	}
	if base == "" {
		return strings.TrimSpace(extra)
	}
	return base + "\n" + strings.TrimSpace(extra)
}

// determineDeletedBy tries to resolve the actor for a deletion via audit log (best-effort).
func (mes *MessageEventService) determineDeletedBy(s *discordgo.Session, guildID, channelID, authorID string) string {
	if s == nil || guildID == "" {
		return "Usuário"
	}
	al, err := s.GuildAuditLog(guildID, "", "", int(discordgo.AuditLogActionMessageDelete), 50)
	if err != nil || al == nil {
		return "Usuário"
	}
	for _, entry := range al.AuditLogEntries {
		if entry == nil || entry.ActionType == nil || *entry.ActionType != discordgo.AuditLogActionMessageDelete {
			continue
		}
		targetOK := entry.TargetID == authorID
		channelOK := true
		if entry.Options != nil && entry.Options.ChannelID != "" {
			channelOK = entry.Options.ChannelID == channelID
		}
		if targetOK && channelOK && entry.UserID != "" {
			return "<@" + entry.UserID + ">"
		}
	}
	return "Usuário"
}
