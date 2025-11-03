package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// CachedMessage stores message data for comparison
type CachedMessage struct {
	ID        string
	Content   string
	Author    *discordgo.User
	ChannelID string
	GuildID   string
	Timestamp time.Time
}

// MessageEventService manages message events (delete/edit)
type MessageEventService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	notifier      *NotificationSender
	adapters      *task.NotificationAdapters
	store         *storage.Store
	isRunning     bool

	// Message cache configuration (env-controlled)
	cacheEnabled   bool
	cacheTTL       time.Duration
	deleteOnLog    bool
	cleanupEnabled bool
}

// NewMessageEventService creates a new instance of the message events service
func NewMessageEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender, store *storage.Store) *MessageEventService {
	return &MessageEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		store:         store,
		isRunning:     false,
	}
}

// Start registers message event handlers
func (mes *MessageEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("message event service is already running")
	}
	mes.isRunning = true

	// Load message cache configuration from environment
	// ALICE_MESSAGE_CACHE_ENABLED: "1/true/on/yes" enables write-through caching (default: disabled)
	// ALICE_MESSAGE_CACHE_TTL_HOURS: TTL for cached messages (default: 72 when enabled)
	// ALICE_MESSAGE_DELETE_ON_LOG: delete message rows after logging deletions (default: disabled)
	// ALICE_MESSAGE_CACHE_CLEANUP: run periodic cleanup of expired messages on start (default: disabled)
	{
		mes.cacheEnabled = util.EnvBool("ALICE_MESSAGE_CACHE_ENABLED")

		ttlHours := 72
		if vv := strings.TrimSpace(os.Getenv("ALICE_MESSAGE_CACHE_TTL_HOURS")); vv != "" {
			if n, err := strconv.Atoi(vv); err == nil && n > 0 {
				ttlHours = n
			}
		}
		mes.cacheTTL = time.Duration(ttlHours) * time.Hour

		mes.deleteOnLog = util.EnvBool("ALICE_MESSAGE_DELETE_ON_LOG")

		mes.cleanupEnabled = util.EnvBool("ALICE_MESSAGE_CACHE_CLEANUP")
	}

	// Store should be injected and already initialized
	// Cleanup is gated by env and disabled by default (do not delete by default)
	if mes.store != nil && mes.cleanupEnabled {
		_ = mes.store.CleanupExpiredMessages()
	}

	mes.session.AddHandler(mes.handleMessageCreate)
	mes.session.AddHandler(mes.handleMessageUpdate)
	mes.session.AddHandler(mes.handleMessageDelete)

	// TTL cache handles cleanup internally

	slog.Info("Message event service started")
	return nil
}

// Stop stops the service
func (mes *MessageEventService) Stop() error {
	if !mes.isRunning {
		return fmt.Errorf("message event service is not running")
	}
	mes.isRunning = false

	slog.Info("Message event service stopped")
	return nil
}

// IsRunning returns whether the service is running
func (mes *MessageEventService) IsRunning() bool {
	return mes.isRunning
}

// handleMessageCreate stores messages for future comparisons
func (mes *MessageEventService) handleMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil {
		slog.Debug("MessageCreate: nil event")
		return
	}
	if m.Author == nil {
		slog.Debug("MessageCreate: nil author; skipping", "channelID", m.ChannelID)
		return
	}
	if m.Author.Bot {
		slog.Debug("MessageCreate: ignoring bot message", "channelID", m.ChannelID, "userID", m.Author.ID)
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
			slog.Debug("MessageCreate: empty content; will not cache", "channelID", m.ChannelID, "userID", m.Author.ID)
			return
		}
		// Use the summary as content for persistence
		m.Content = extra
		slog.Debug("MessageCreate: content empty; using summary for cache", "channelID", m.ChannelID, "userID", m.Author.ID)
	}
	slog.Debug("MessageCreate received", "channelID", m.ChannelID, "userID", m.Author.ID, "messageID", m.ID)

	// Check if this is a guild message without fetching the channel when possible
	guildID := m.GuildID
	if guildID == "" {
		// Fallback: get via channel only if necessary (likely DM)
		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			slog.Debug("MessageCreate: failed to fetch channel; skipping cache", "channelID", m.ChannelID, "error", err)
			return
		}
		guildID = channel.GuildID
	}
	if guildID == "" {
		slog.Debug("MessageCreate: DM detected; skipping cache", "channelID", m.ChannelID)
		return
	}

	// Check if the guild is configured
	guildConfig := mes.configManager.GuildConfig(guildID)
	if guildConfig == nil {
		slog.Debug("MessageCreate: no guild config; skipping cache", "guildID", guildID)
		return
	}

	mes.markEvent()

	// Persist to SQLite (write-through; best effort)
	if mes.cacheEnabled && mes.store != nil && m.Author != nil {
		_ = mes.store.UpsertMessage(storage.MessageRecord{
			GuildID:        guildID,
			MessageID:      m.ID,
			ChannelID:      m.ChannelID,
			AuthorID:       m.Author.ID,
			AuthorUsername: m.Author.Username,
			AuthorAvatar:   m.Author.Avatar,
			Content:        m.Content,
			CachedAt:       time.Now().UTC(),
			ExpiresAt:      time.Now().UTC().Add(mes.cacheTTL),
			HasExpiry:      true,
		})

		// Versioned history (v1) - gated by ALICE_MESSAGE_VERSIONING_ENABLED
		if util.EnvBool("ALICE_MESSAGE_VERSIONING_ENABLED") {
			_ = mes.store.InsertMessageVersion(storage.MessageVersion{
				GuildID:     guildID,
				MessageID:   m.ID,
				ChannelID:   m.ChannelID,
				AuthorID:    m.Author.ID,
				Version:     1,
				EventType:   "create",
				Content:     m.Content,
				Attachments: len(m.Attachments),
				Embeds:      len(m.Embeds),
				Stickers:    len(m.StickerItems),
				CreatedAt:   time.Now().UTC(),
			})
		}
	}

	if mes.store != nil && m.Author != nil {
		_ = mes.store.IncrementDailyMessageCount(guildID, m.ChannelID, m.Author.ID, time.Now().UTC())
	}
	slog.Info("Message cached for monitoring", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.ID, "userID", m.Author.ID)
}

// handleMessageUpdate processes message edits
func (mes *MessageEventService) handleMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m == nil {
		slog.Debug("MessageUpdate: nil event")
		return
	}
	if m.Author != nil && m.Author.Bot {
		slog.Debug("MessageUpdate: ignoring bot edit", "messageID", m.ID, "userID", m.Author.ID, "channelID", m.ChannelID)
		return
	}
	slog.Info("MessageUpdate received", "messageID", m.ID, "userID", m.Author.ID, "guildID", m.GuildID, "channelID", m.ChannelID)

	mes.markEvent()

	// Consult persistence (SQLite) to get the original message (with guild/channel fallback + short retry)
	var cached *CachedMessage
	guildID := m.GuildID
	if guildID == "" && s != nil && s.State != nil {
		if ch, _ := s.State.Channel(m.ChannelID); ch != nil {
			guildID = ch.GuildID
		}
	}
	if guildID == "" && s != nil {
		if ch, _ := s.Channel(m.ChannelID); ch != nil {
			guildID = ch.GuildID
		}
	}
	if mes.store != nil && guildID != "" {
		tryFetch := func() *CachedMessage {
			if rec, err := mes.store.GetMessage(guildID, m.ID); err == nil && rec != nil {
				return &CachedMessage{
					ID:        rec.MessageID,
					Content:   rec.Content,
					Author:    &discordgo.User{ID: rec.AuthorID, Username: rec.AuthorUsername, Avatar: rec.AuthorAvatar},
					ChannelID: rec.ChannelID,
					GuildID:   rec.GuildID,
					Timestamp: rec.CachedAt,
				}
			}
			return nil
		}
		cached = tryFetch()
		if cached == nil {
			time.Sleep(200 * time.Millisecond)
			cached = tryFetch()
		}
		if cached == nil {
			time.Sleep(400 * time.Millisecond)
			cached = tryFetch()
		}
	}

	if cached == nil {
		slog.Info("Message edit detected but original not in cache/persistence", "messageID", m.ID, "userID", m.Author.ID)
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
	// Check that the content actually changed (compare effective strings)
	if cached.Content == m.Content {
		slog.Debug("MessageUpdate: content unchanged; skipping notification", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID)
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		slog.Debug("MessageUpdate: no guild config; skipping notification", "guildID", cached.GuildID, "messageID", m.ID)
		return
	}

	logChannelID := mes.fallbackMessageLogChannel(guildConfig)
	if logChannelID == "" {
		slog.Info("Message log channel not configured for guild; edit notification not sent", "guildID", cached.GuildID, "messageID", m.ID)
		return
	}

	slog.Info("Message edit detected", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID, "username", cached.Author.Username)

	// Send edit notification
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
			slog.Error("Failed to send message edit notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			slog.Info("Message edit notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
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
			slog.Error("Failed to send message edit notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			slog.Info("Message edit notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
		}
	}

	// Update persistence with new content
	updated := &CachedMessage{
		ID:        cached.ID,
		Content:   m.Content,
		Author:    cached.Author,
		ChannelID: cached.ChannelID,
		GuildID:   cached.GuildID,
		Timestamp: cached.Timestamp,
	}
	if mes.cacheEnabled && mes.store != nil && updated.Author != nil {
		_ = mes.store.UpsertMessage(storage.MessageRecord{
			GuildID:        updated.GuildID,
			MessageID:      updated.ID,
			ChannelID:      updated.ChannelID,
			AuthorID:       updated.Author.ID,
			AuthorUsername: updated.Author.Username,
			AuthorAvatar:   updated.Author.Avatar,
			Content:        updated.Content,
			CachedAt:       time.Now().UTC(),
			ExpiresAt:      time.Now().UTC().Add(mes.cacheTTL),
			HasExpiry:      true,
		})

		// Versioned history (edit) - gated by ALICE_MESSAGE_VERSIONING_ENABLED
		if util.EnvBool("ALICE_MESSAGE_VERSIONING_ENABLED") {
			_ = mes.store.InsertMessageVersion(storage.MessageVersion{
				GuildID:   updated.GuildID,
				MessageID: updated.ID,
				ChannelID: updated.ChannelID,
				AuthorID:  updated.Author.ID,
				EventType: "edit",
				Content:   m.Content,
				CreatedAt: time.Now().UTC(),
			})
		}
	}
	slog.Info("MessageUpdate: store updated with new content", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID)
}

// handleMessageDelete processes message deletions
func (mes *MessageEventService) handleMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	if m == nil {
		return
	}

	var cached *CachedMessage

	mes.markEvent()

	if mes.store != nil {
		guildID := m.GuildID
		if guildID == "" && s != nil && s.State != nil {
			if ch, _ := s.State.Channel(m.ChannelID); ch != nil {
				guildID = ch.GuildID
			}
		}
		if guildID == "" && s != nil {
			if ch, _ := s.Channel(m.ChannelID); ch != nil {
				guildID = ch.GuildID
			}
		}
		if guildID != "" {
			tryFetch := func() *CachedMessage {
				if rec, err := mes.store.GetMessage(guildID, m.ID); err == nil && rec != nil {
					return &CachedMessage{
						ID:        rec.MessageID,
						Content:   rec.Content,
						Author:    &discordgo.User{ID: rec.AuthorID, Username: rec.AuthorUsername, Avatar: rec.AuthorAvatar},
						ChannelID: rec.ChannelID,
						GuildID:   rec.GuildID,
						Timestamp: rec.CachedAt,
					}
				}
				return nil
			}
			cached = tryFetch()
			if cached == nil {
				time.Sleep(200 * time.Millisecond)
				cached = tryFetch()
			}
			if cached == nil {
				time.Sleep(400 * time.Millisecond)
				cached = tryFetch()
			}
		}
	}

	if cached == nil {
		slog.Info("Message delete detected but original not in cache/persistence", "messageID", m.ID, "channelID", m.ChannelID)
		return
	}

	// Skip if bot
	if cached.Author.Bot {
		// Deletion from store is disabled by default
		if mes.deleteOnLog && mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		// Deletion from store is disabled by default
		if mes.deleteOnLog && mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return
	}

	logChannelID := mes.fallbackMessageLogChannel(guildConfig)
	if logChannelID == "" {
		slog.Info("Message log channel not configured for guild; delete notification not sent", "guildID", cached.GuildID, "messageID", m.ID)
		// Deletion from store is disabled by default
		if mes.deleteOnLog && mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return
	}

	slog.Info("Message delete detected", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID, "username", cached.Author.Username)

	// Try to determine who deleted it (best effort via audit log)
	deletedBy := mes.determineDeletedBy(s, cached.GuildID, cached.ChannelID, cached.Author.ID)

	// Send deletion notification
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
			slog.Error("Failed to send message delete notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			slog.Info("Message delete notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
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
			slog.Error("Failed to send message delete notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			slog.Info("Message delete notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
		}
	}

	// Remove from cache and persistence (disabled by default)
	// Versioned history (delete) - gated by ALICE_MESSAGE_VERSIONING_ENABLED
	if util.EnvBool("ALICE_MESSAGE_VERSIONING_ENABLED") && mes.store != nil && cached.Author != nil {
		_ = mes.store.InsertMessageVersion(storage.MessageVersion{
			GuildID:   cached.GuildID,
			MessageID: cached.ID,
			ChannelID: cached.ChannelID,
			AuthorID:  cached.Author.ID,
			EventType: "delete",
			Content:   cached.Content,
			CreatedAt: time.Now().UTC(),
		})
	}
	if mes.deleteOnLog && mes.store != nil {
		_ = mes.store.DeleteMessage(m.GuildID, m.ID)
	}
}

// Persistent storage (SQLite) handles expiration and cleanup

// markEvent stores the last event timestamp (best effort)
func (mes *MessageEventService) markEvent() {
	if mes.store != nil {
		_ = mes.store.SetLastEvent(time.Now())
	}
}

func (mes *MessageEventService) GetCacheStats() map[string]any {
	return map[string]any{
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
		return "User"
	}
	al, err := s.GuildAuditLog(guildID, "", "", int(discordgo.AuditLogActionMessageDelete), 50)
	if err != nil || al == nil {
		return "User"
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
	return "User"
}
