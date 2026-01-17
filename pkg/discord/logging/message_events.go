package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
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

type auditCacheEntry struct {
	fetchedAt time.Time
	entries   map[string]auditCacheValue
}

type auditCacheValue struct {
	userID    string
	createdAt time.Time
}

// MessageEventService manages message events (delete/edit)
type MessageEventService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	notifier      *NotificationSender
	adapters      *task.NotificationAdapters
	store         *storage.Store
	isRunning     bool
	verifyStop    chan struct{}
	verifyWG      sync.WaitGroup

	// Message cache configuration (populated from settings.json runtime_config)
	cacheEnabled   bool
	cacheTTL       time.Duration
	deleteOnLog    bool
	cleanupEnabled bool

	// Versioning configuration (populated from settings.json runtime_config)
	versioningEnabled bool

	auditCacheMu  sync.Mutex
	auditCache    map[string]auditCacheEntry
	auditCacheTTL time.Duration
	auditEntryMax time.Duration

	verifyMu      sync.Mutex
	verifyPending map[string]time.Time

	taskRouter *task.TaskRouter
}

const (
	verificationCleanupInterval     = 5 * time.Minute
	verificationInactivityThreshold = 30 * time.Minute
	verificationMimuEmbedMessageID  = "1375847102344593482"
	verificationPendingWindow       = 30 * time.Minute
)

const (
	messageEventRetryInitialBackoff = 300 * time.Millisecond
	messageEventRetryMaxBackoff     = 1200 * time.Millisecond
	messageEventRetryMaxAttempts    = 4
	messageEventRetryTTL            = 5 * time.Second

	taskTypeMessageUpdateProcess = "message_event.process_update"
	taskTypeMessageDeleteProcess = "message_event.process_delete"
)

type messageUpdateTaskPayload struct {
	Update     *discordgo.MessageUpdate
	ReceivedAt time.Time
}

type messageDeleteTaskPayload struct {
	Delete     *discordgo.MessageDelete
	ReceivedAt time.Time
}

// NewMessageEventService creates a new instance of the message events service
func NewMessageEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender, store *storage.Store) *MessageEventService {
	return &MessageEventService{
		session:       session,
		configManager: configManager,
		notifier:      notifier,
		store:         store,
		isRunning:     false,
		auditCache:    make(map[string]auditCacheEntry),
		auditCacheTTL: 2 * time.Second,
		auditEntryMax: 15 * time.Second,
		verifyStop:    make(chan struct{}),
		verifyPending: make(map[string]time.Time),
	}
}

// Start registers message event handlers
func (mes *MessageEventService) Start() error {
	if mes.isRunning {
		return fmt.Errorf("message event service is already running")
	}
	mes.isRunning = true

	// Load message cache configuration from settings.json runtime_config,
	// but keep cache + versioning hardcoded enabled.
	{
		rc := files.RuntimeConfig{}
		if mes.configManager != nil && mes.configManager.Config() != nil {
			rc = mes.configManager.Config().RuntimeConfig
		}

		// Hardcoded enabled
		mes.cacheEnabled = true

		ttlHours := 72
		if rc.MessageCacheTTLHours > 0 {
			ttlHours = rc.MessageCacheTTLHours
		}
		mes.cacheTTL = time.Duration(ttlHours) * time.Hour

		mes.deleteOnLog = rc.MessageDeleteOnLog
		mes.cleanupEnabled = rc.MessageCacheCleanup

		// Hardcoded enabled
		mes.versioningEnabled = true
	}

	// Store should be injected and already initialized
	// Cleanup is gated by env and disabled by default (do not delete by default)
	if mes.store != nil && mes.cleanupEnabled {
		_ = mes.store.CleanupExpiredMessages()
	}

	mes.session.AddHandler(mes.handleMessageCreate)
	mes.session.AddHandler(mes.handleMessageUpdate)
	mes.session.AddHandler(mes.handleMessageDelete)
	mes.session.AddHandler(mes.handleGuildMemberUpdate)

	if mes.taskRouter != nil {
		mes.taskRouter.RegisterHandler(taskTypeMessageUpdateProcess, mes.handleMessageUpdateTask)
		mes.taskRouter.RegisterHandler(taskTypeMessageDeleteProcess, mes.handleMessageDeleteTask)
	}

	if mes.verifyStop == nil {
		mes.verifyStop = make(chan struct{})
	}
	mes.verifyWG.Add(1)
	go mes.verificationCleanupLoop()

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

	if mes.verifyStop != nil {
		close(mes.verifyStop)
		mes.verifyStop = nil
	}
	mes.verifyWG.Wait()

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

	done := perf.StartGatewayEvent(
		"message_create",
		slog.String("guildID", m.GuildID),
		slog.String("channelID", m.ChannelID),
		slog.String("messageID", m.ID),
		slog.String("userID", m.Author.ID),
	)
	defer done()

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
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}
	rc := cfg.ResolveRuntimeConfig(guildID)
	if rc.DisableMessageLogs {
		slog.Debug("MessageCreate: message logs disabled for guild; skipping cache", "guildID", guildID)
		return
	}

	guildConfig := mes.configManager.GuildConfig(guildID)
	if guildConfig == nil {
		slog.Debug("MessageCreate: no guild config; skipping cache", "guildID", guildID)
		return
	}

	if strings.TrimSpace(guildConfig.Channels.VerificationChat) == m.ChannelID {
		mes.cleanupPreviousVerificationMessages(guildID, m.ChannelID, m.Author.ID, m.ID)
		mes.markVerificationPendingIfUnverified(s, guildConfig, m)
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

		// Versioned history (v1) - hardcoded enabled
		if mes.versioningEnabled {
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
	done := perf.StartGatewayEvent(
		"message_update",
		slog.String("guildID", m.GuildID),
		slog.String("channelID", m.ChannelID),
		slog.String("messageID", m.ID),
	)
	defer done()
	authorID := ""
	if m.Author != nil {
		authorID = m.Author.ID
	}
	slog.Info("MessageUpdate received", "messageID", m.ID, "userID", authorID, "guildID", m.GuildID, "channelID", m.ChannelID)

	if mes.taskRouter != nil {
		if err := mes.dispatchMessageUpdateTask(m); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				slog.Debug("MessageUpdate: task already queued", "messageID", m.ID)
			} else {
				slog.Error("MessageUpdate: failed to enqueue task", "messageID", m.ID, "error", err)
			}
		}
		return
	}

	_ = mes.processMessageUpdate(s, m, true)
}

// handleMessageDelete processes message deletions
func (mes *MessageEventService) handleMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	if m == nil {
		return
	}

	done := perf.StartGatewayEvent(
		"message_delete",
		slog.String("guildID", m.GuildID),
		slog.String("channelID", m.ChannelID),
		slog.String("messageID", m.ID),
	)
	defer done()

	if mes.taskRouter != nil {
		if err := mes.dispatchMessageDeleteTask(m); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				slog.Debug("MessageDelete: task already queued", "messageID", m.ID)
			} else {
				slog.Error("MessageDelete: failed to enqueue task", "messageID", m.ID, "error", err)
			}
		}
		return
	}

	_ = mes.processMessageDelete(s, m, true)
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

func (mes *MessageEventService) SetTaskRouter(router *task.TaskRouter) {
	mes.taskRouter = router
}

func (mes *MessageEventService) dispatchMessageUpdateTask(m *discordgo.MessageUpdate) error {
	if mes.taskRouter == nil || m == nil || m.ID == "" {
		return nil
	}
	payload := messageUpdateTaskPayload{
		Update:     cloneMessageUpdate(m),
		ReceivedAt: time.Now().UTC(),
	}
	group := m.GuildID
	if group == "" {
		group = m.ChannelID
	}
	if group == "" {
		group = "message_update"
	}
	return mes.taskRouter.Dispatch(context.Background(), task.Task{
		Type:    taskTypeMessageUpdateProcess,
		Payload: payload,
		Options: task.TaskOptions{
			GroupKey:       group,
			IdempotencyKey: fmt.Sprintf("msg_update:%s:%s", group, m.ID),
			IdempotencyTTL: messageEventRetryTTL,
			MaxAttempts:    messageEventRetryMaxAttempts,
			InitialBackoff: messageEventRetryInitialBackoff,
			MaxBackoff:     messageEventRetryMaxBackoff,
		},
	})
}

func (mes *MessageEventService) dispatchMessageDeleteTask(m *discordgo.MessageDelete) error {
	if mes.taskRouter == nil || m == nil || m.ID == "" {
		return nil
	}
	payload := messageDeleteTaskPayload{
		Delete:     cloneMessageDelete(m),
		ReceivedAt: time.Now().UTC(),
	}
	group := m.GuildID
	if group == "" {
		group = m.ChannelID
	}
	if group == "" {
		group = "message_delete"
	}
	return mes.taskRouter.Dispatch(context.Background(), task.Task{
		Type:    taskTypeMessageDeleteProcess,
		Payload: payload,
		Options: task.TaskOptions{
			GroupKey:       group,
			IdempotencyKey: fmt.Sprintf("msg_delete:%s:%s", group, m.ID),
			IdempotencyTTL: messageEventRetryTTL,
			MaxAttempts:    messageEventRetryMaxAttempts,
			InitialBackoff: messageEventRetryInitialBackoff,
			MaxBackoff:     messageEventRetryMaxBackoff,
		},
	})
}

func (mes *MessageEventService) handleMessageUpdateTask(_ context.Context, payload any) error {
	p, ok := payload.(messageUpdateTaskPayload)
	if !ok || p.Update == nil {
		return fmt.Errorf("invalid payload for %s", taskTypeMessageUpdateProcess)
	}
	return mes.processMessageUpdate(mes.session, p.Update, false)
}

func (mes *MessageEventService) handleMessageDeleteTask(_ context.Context, payload any) error {
	p, ok := payload.(messageDeleteTaskPayload)
	if !ok || p.Delete == nil {
		return fmt.Errorf("invalid payload for %s", taskTypeMessageDeleteProcess)
	}
	return mes.processMessageDelete(mes.session, p.Delete, false)
}

func (mes *MessageEventService) processMessageUpdate(s *discordgo.Session, m *discordgo.MessageUpdate, allowWait bool) error {
	if m == nil {
		return nil
	}
	if m.Author != nil && m.Author.Bot {
		return nil
	}

	mes.markEvent()

	// Consult persistence (SQLite) to get the original message (with guild/channel fallback)
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

	cached := mes.lookupCachedMessage(guildID, m.ID, allowWait)
	if cached == nil {
		authorID := ""
		if m.Author != nil {
			authorID = m.Author.ID
		}
		if !allowWait && mes.store != nil && guildID != "" {
			return fmt.Errorf("%w: message update cache miss", task.ErrRetrySilent)
		}
		slog.Info("Message edit detected but original not in cache/persistence", "messageID", m.ID, "userID", authorID)
		return nil
	}

	cfg := mes.configManager.Config()
	if cfg == nil {
		return nil
	}
	rc := cfg.ResolveRuntimeConfig(cached.GuildID)
	if rc.DisableMessageLogs {
		return nil
	}

	// Ensure latest content; MessageUpdate may omit content. Also enrich empty content with context.
	contentResolved := true
	if m.Content == "" {
		if s == nil {
			contentResolved = false
		} else if msg, err := s.ChannelMessage(m.ChannelID, m.ID); err == nil && msg != nil {
			m.Content = msg.Content
			// Enrich only when original content is empty (e.g., attachments-only messages)
			m.Content = mes.summarizeMessageContent(msg, m.Content)
		} else {
			contentResolved = false
		}
	}
	if !contentResolved {
		slog.Debug("MessageUpdate: unable to resolve content; skipping notification", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID)
		return nil
	}
	// Check that the content actually changed (compare effective strings)
	if cached.Content == m.Content {
		slog.Debug("MessageUpdate: content unchanged; skipping notification", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID)
		return nil
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		slog.Debug("MessageUpdate: no guild config; skipping notification", "guildID", cached.GuildID, "messageID", m.ID)
		return nil
	}

	logChannelID := mes.fallbackMessageLogChannel(guildConfig)
	if logChannelID == "" {
		slog.Info("Message log channel not configured for guild; edit notification not sent", "guildID", cached.GuildID, "messageID", m.ID)
		return nil
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
	if contentResolved && mes.cacheEnabled && mes.store != nil && updated.Author != nil {
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

		// Versioned history (edit) - hardcoded enabled
		if mes.versioningEnabled {
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
	return nil
}

func (mes *MessageEventService) processMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete, allowWait bool) error {
	if m == nil {
		return nil
	}

	mes.markEvent()

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

	cached := mes.lookupCachedMessage(guildID, m.ID, allowWait)
	if cached == nil {
		if !allowWait && mes.store != nil && guildID != "" {
			return fmt.Errorf("%w: message delete cache miss", task.ErrRetrySilent)
		}
		slog.Info("Message delete detected but original not in cache/persistence", "messageID", m.ID, "channelID", m.ChannelID)
		return nil
	}

	cfg := mes.configManager.Config()
	if cfg == nil {
		return nil
	}
	rc := cfg.ResolveRuntimeConfig(cached.GuildID)
	if rc.DisableMessageLogs {
		return nil
	}

	// Skip if bot
	if cached.Author.Bot {
		// Deletion from store is disabled by default
		if mes.deleteOnLog && mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return nil
	}

	guildConfig := mes.configManager.GuildConfig(cached.GuildID)
	if guildConfig == nil {
		// Deletion from store is disabled by default
		if mes.deleteOnLog && mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return nil
	}

	logChannelID := mes.fallbackMessageLogChannel(guildConfig)
	if logChannelID == "" {
		slog.Info("Message log channel not configured for guild; delete notification not sent", "guildID", cached.GuildID, "messageID", m.ID)
		// Deletion from store is disabled by default
		if mes.deleteOnLog && mes.store != nil {
			_ = mes.store.DeleteMessage(m.GuildID, m.ID)
		}
		return nil
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
	// Versioned history (delete) - hardcoded enabled
	if mes.versioningEnabled && mes.store != nil && cached.Author != nil {
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
	return nil
}

func (mes *MessageEventService) lookupCachedMessage(guildID, messageID string, allowWait bool) *CachedMessage {
	if mes.store == nil || guildID == "" || messageID == "" {
		return nil
	}
	tryFetch := func() *CachedMessage {
		if rec, err := mes.store.GetMessage(guildID, messageID); err == nil && rec != nil {
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
	cached := tryFetch()
	if cached != nil || !allowWait {
		return cached
	}
	time.Sleep(200 * time.Millisecond)
	cached = tryFetch()
	if cached != nil {
		return cached
	}
	time.Sleep(400 * time.Millisecond)
	return tryFetch()
}

func cloneMessageUpdate(m *discordgo.MessageUpdate) *discordgo.MessageUpdate {
	if m == nil {
		return nil
	}
	copy := *m
	return &copy
}

func cloneMessageDelete(m *discordgo.MessageDelete) *discordgo.MessageDelete {
	if m == nil {
		return nil
	}
	copy := *m
	return &copy
}

// fallbackMessageLogChannel chooses the best available channel for message logs.
func (mes *MessageEventService) fallbackMessageLogChannel(g *files.GuildConfig) string {
	if g == nil {
		return ""
	}
	if g.Channels.MessageAuditLog != "" {
		return g.Channels.MessageAuditLog
	}
	if g.Channels.UserActivityLog != "" {
		return g.Channels.UserActivityLog
	}
	if g.Channels.Commands != "" {
		return g.Channels.Commands
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

func (mes *MessageEventService) verificationCleanupLoop() {
	defer mes.verifyWG.Done()

	if verificationCleanupInterval <= 0 {
		return
	}

	ticker := time.NewTicker(verificationCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mes.cleanupVerificationChannels()
		case <-mes.verifyStop:
			return
		}
	}
}

func (mes *MessageEventService) cleanupVerificationChannels() {
	if mes.session == nil || mes.configManager == nil {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return
	}

	for _, gcfg := range cfg.Guilds {
		channelID := strings.TrimSpace(gcfg.Channels.VerificationChat)
		if channelID == "" {
			continue
		}
		mes.cleanupIdleVerificationChannel(gcfg.GuildID, channelID)
	}
}

func (mes *MessageEventService) cleanupIdleVerificationChannel(guildID, channelID string) {
	if mes.session == nil || channelID == "" {
		return
	}

	msgs, err := mes.session.ChannelMessages(channelID, 100, "", "", "")
	if err != nil {
		slog.Warn("Verification cleanup: failed to fetch channel messages", "guildID", guildID, "channelID", channelID, "error", err)
		return
	}

	var newest time.Time
	toDelete := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil || msg.ID == verificationMimuEmbedMessageID {
			continue
		}
		toDelete = append(toDelete, msg.ID)
		if msg.Timestamp.After(newest) {
			newest = msg.Timestamp
		}
	}

	if len(toDelete) == 0 || newest.IsZero() {
		return
	}
	if time.Since(newest) < verificationInactivityThreshold {
		return
	}

	removed := 0
	for _, msgID := range toDelete {
		if err := mes.session.ChannelMessageDelete(channelID, msgID); err != nil {
			slog.Warn("Verification cleanup: failed to delete stale message", "guildID", guildID, "channelID", channelID, "messageID", msgID, "error", err)
			continue
		}
		removed++
	}

	if removed > 0 {
		slog.Info("Verification cleanup: removed stale messages", "guildID", guildID, "channelID", channelID, "removed", removed)
	}
}

func (mes *MessageEventService) cleanupPreviousVerificationMessages(guildID, channelID, userID, currentMessageID string) {
	if mes.session == nil || channelID == "" || userID == "" {
		return
	}

	msgs, err := mes.session.ChannelMessages(channelID, 100, "", "", "")
	if err != nil {
		slog.Warn("Verification cleanup: failed to fetch channel messages", "guildID", guildID, "channelID", channelID, "error", err)
		return
	}

	removed := 0
	for _, msg := range msgs {
		if msg == nil || msg.Author == nil {
			continue
		}
		if msg.ID == currentMessageID || msg.ID == verificationMimuEmbedMessageID {
			continue
		}
		if msg.Author.ID != userID {
			continue
		}
		if err := mes.session.ChannelMessageDelete(channelID, msg.ID); err != nil {
			slog.Warn("Verification cleanup: failed to delete previous message", "guildID", guildID, "channelID", channelID, "userID", userID, "messageID", msg.ID, "error", err)
			continue
		}
		removed++
	}

	if removed > 0 {
		slog.Info("Verification cleanup: removed previous messages", "guildID", guildID, "channelID", channelID, "userID", userID, "removed", removed)
	}
}

func (mes *MessageEventService) cleanupAllVerificationMessagesForUser(guildID, channelID, userID string) {
	if mes.session == nil || channelID == "" || userID == "" {
		return
	}

	msgs, err := mes.session.ChannelMessages(channelID, 100, "", "", "")
	if err != nil {
		slog.Warn("Verification cleanup: failed to fetch channel messages", "guildID", guildID, "channelID", channelID, "error", err)
		return
	}

	removed := 0
	for _, msg := range msgs {
		if msg == nil || msg.Author == nil {
			continue
		}
		if msg.ID == verificationMimuEmbedMessageID {
			continue
		}
		if msg.Author.ID != userID {
			continue
		}
		if err := mes.session.ChannelMessageDelete(channelID, msg.ID); err != nil {
			slog.Warn("Verification cleanup: failed to delete verified user message", "guildID", guildID, "channelID", channelID, "userID", userID, "messageID", msg.ID, "error", err)
			continue
		}
		removed++
	}

	if removed > 0 {
		slog.Info("Verification cleanup: removed verified user messages", "guildID", guildID, "channelID", channelID, "userID", userID, "removed", removed)
	}
}

func (mes *MessageEventService) markVerificationPendingIfUnverified(s *discordgo.Session, gcfg *files.GuildConfig, m *discordgo.MessageCreate) {
	if gcfg == nil || m == nil || m.Author == nil || m.Author.Bot {
		return
	}
	verifiedRoleID := strings.TrimSpace(gcfg.Roles.VerificationRole)
	if verifiedRoleID == "" {
		return
	}

	roles := []string{}
	if m.Member != nil {
		roles = m.Member.Roles
	} else if s != nil && s.State != nil && m.GuildID != "" {
		if member, err := s.State.Member(m.GuildID, m.Author.ID); err == nil && member != nil {
			roles = member.Roles
		}
	}
	if hasRoleID(roles, verifiedRoleID) {
		return
	}

	key := m.GuildID + ":" + m.Author.ID
	mes.verifyMu.Lock()
	if mes.verifyPending == nil {
		mes.verifyPending = make(map[string]time.Time)
	}
	mes.verifyPending[key] = time.Now().UTC()
	mes.verifyMu.Unlock()
}

func (mes *MessageEventService) handleGuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m == nil || m.User == nil || m.User.Bot {
		return
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return
	}
	guildConfig := mes.configManager.GuildConfig(m.GuildID)
	if guildConfig == nil {
		return
	}
	channelID := strings.TrimSpace(guildConfig.Channels.VerificationChat)
	if channelID == "" {
		return
	}
	verifiedRoleID := strings.TrimSpace(guildConfig.Roles.VerificationRole)
	if verifiedRoleID == "" {
		return
	}

	key := m.GuildID + ":" + m.User.ID
	mes.verifyMu.Lock()
	pendingAt, ok := mes.verifyPending[key]
	if !ok {
		mes.verifyMu.Unlock()
		return
	}
	if verificationPendingWindow > 0 && time.Since(pendingAt) > verificationPendingWindow {
		delete(mes.verifyPending, key)
		mes.verifyMu.Unlock()
		return
	}
	mes.verifyMu.Unlock()

	if !hasRoleID(m.Roles, verifiedRoleID) {
		return
	}

	mes.cleanupAllVerificationMessagesForUser(m.GuildID, channelID, m.User.ID)

	mes.verifyMu.Lock()
	delete(mes.verifyPending, key)
	mes.verifyMu.Unlock()
}

// determineDeletedBy tries to resolve the actor for a deletion via audit log (best-effort).
func (mes *MessageEventService) determineDeletedBy(s *discordgo.Session, guildID, channelID, authorID string) string {
	if s == nil || guildID == "" || authorID == "" {
		return ""
	}

	cacheKey := authorID + ":" + channelID
	cacheFallbackKey := authorID + ":"
	if mes.auditCacheTTL > 0 {
		mes.auditCacheMu.Lock()
		if entry, ok := mes.auditCache[guildID]; ok && time.Since(entry.fetchedAt) < mes.auditCacheTTL {
			if userID := mes.pickAuditEntry(entry.entries, cacheKey); userID != "" {
				mes.auditCacheMu.Unlock()
				return userID
			}
			if userID := mes.pickAuditEntry(entry.entries, cacheFallbackKey); userID != "" {
				mes.auditCacheMu.Unlock()
				return userID
			}
			mes.auditCacheMu.Unlock()
			return ""
		}
		mes.auditCacheMu.Unlock()
	}

	al, err := s.GuildAuditLog(guildID, "", "", int(discordgo.AuditLogActionMessageDelete), 50)
	if err != nil || al == nil {
		return ""
	}

	now := time.Now()
	entries := make(map[string]auditCacheValue)
	for _, entry := range al.AuditLogEntries {
		if entry == nil || entry.ActionType == nil || *entry.ActionType != discordgo.AuditLogActionMessageDelete {
			continue
		}
		if entry.TargetID == "" || entry.UserID == "" {
			continue
		}
		if entry.UserID == authorID {
			continue
		}
		createdAt := now
		if ts, ok := snowflakeTimestamp(entry.ID); ok {
			createdAt = ts
		}
		if mes.auditEntryMax > 0 && now.Sub(createdAt) > mes.auditEntryMax {
			continue
		}
		targetOK := entry.TargetID == authorID
		channelOK := true
		if entry.Options != nil && entry.Options.ChannelID != "" {
			channelOK = entry.Options.ChannelID == channelID
		}
		if targetOK && channelOK {
			entries[cacheKey] = newerAuditEntry(entries[cacheKey], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
			entries[cacheFallbackKey] = newerAuditEntry(entries[cacheFallbackKey], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
		}
		if entry.Options != nil && entry.Options.ChannelID != "" {
			key := entry.TargetID + ":" + entry.Options.ChannelID
			entries[key] = newerAuditEntry(entries[key], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
		}
		// Also store fallback without channel when available
		key := entry.TargetID + ":"
		entries[key] = newerAuditEntry(entries[key], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
	}

	if mes.auditCacheTTL > 0 {
		mes.auditCacheMu.Lock()
		mes.auditCache[guildID] = auditCacheEntry{
			fetchedAt: now,
			entries:   entries,
		}
		mes.auditCacheMu.Unlock()
		if userID := mes.pickAuditEntry(entries, cacheKey); userID != "" {
			return userID
		}
		if userID := mes.pickAuditEntry(entries, cacheFallbackKey); userID != "" {
			return userID
		}
	}
	return ""
}

func (mes *MessageEventService) pickAuditEntry(entries map[string]auditCacheValue, key string) string {
	if entries == nil {
		return ""
	}
	val, ok := entries[key]
	if !ok {
		return ""
	}
	if mes.auditEntryMax > 0 && time.Since(val.createdAt) > mes.auditEntryMax {
		return ""
	}
	return val.userID
}

func newerAuditEntry(current, candidate auditCacheValue) auditCacheValue {
	if candidate.userID == "" {
		return current
	}
	if current.userID == "" || candidate.createdAt.After(current.createdAt) {
		return candidate
	}
	return current
}

func snowflakeTimestamp(id string) (time.Time, bool) {
	if id == "" {
		return time.Time{}, false
	}
	raw, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	const discordEpochMS = 1420070400000
	ms := int64(raw>>22) + discordEpochMS
	return time.Unix(0, ms*int64(time.Millisecond)), true
}
