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
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
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
	session        *discordgo.Session
	configManager  *files.ConfigManager
	botInstanceID  string
	defaultBotID   string
	notifier       *NotificationSender
	adapters       *task.NotificationAdapters
	store          *storage.Store
	activity       *runtimeActivity
	lifecycle      serviceLifecycle
	handlerCancels []func()
	logger         *slog.Logger

	// Message cache configuration (populated from persisted runtime_config)
	cacheEnabled   bool
	cacheTTL       time.Duration
	deleteOnLog    bool
	cleanupEnabled bool

	// Versioning configuration (populated from persisted runtime_config)
	versioningEnabled bool

	auditCacheMu  sync.Mutex
	auditCache    map[string]auditCacheEntry
	auditCacheTTL time.Duration
	auditEntryMax time.Duration

	taskRouter *task.TaskRouter

	messageCreateWriter *messageCreateWriter
	writerMetrics       MessageWriterMetrics
}

const (
	messageEventRetryInitialBackoff = 300 * time.Millisecond
	messageEventRetryMaxBackoff     = 1200 * time.Millisecond
	messageEventRetryMaxAttempts    = 4
	messageEventRetryTTL            = 5 * time.Second

	taskTypeMessageUpdateProcess = "message_event.process_update"
	taskTypeMessageDeleteProcess = "message_event.process_delete"
)

// MessageUpdateTaskPayload is the task payload for a deferred message-edit
// event. ReceivedAt records when the gateway event arrived, so latency can be
// measured against task execution time.
type MessageUpdateTaskPayload struct {
	Update     *discordgo.MessageUpdate
	ReceivedAt time.Time
}

// MessageDeleteTaskPayload is the task payload for a deferred message-delete
// event. ReceivedAt records when the gateway event arrived, so latency can be
// measured against task execution time.
type MessageDeleteTaskPayload struct {
	Delete     *discordgo.MessageDelete
	ReceivedAt time.Time
}

// NewMessageEventService creates a new instance of the message events service
func NewMessageEventService(session *discordgo.Session, configManager *files.ConfigManager, notifier *NotificationSender, store *storage.Store, logger *slog.Logger) *MessageEventService {
	return NewMessageEventServiceForBot(session, configManager, notifier, store, "", "", logger)
}

// NewMessageEventServiceForBot creates a message event service scoped to a bot
// instance assignment.
func NewMessageEventServiceForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	notifier *NotificationSender,
	store *storage.Store,
	botInstanceID string,
	defaultBotInstanceID string,
	logger *slog.Logger,
) *MessageEventService {
	return &MessageEventService{
		session:       session,
		configManager: configManager,
		botInstanceID: files.NormalizeBotInstanceID(botInstanceID),
		defaultBotID:  files.NormalizeBotInstanceID(defaultBotInstanceID),
		notifier:      notifier,
		store:         store,
		logger:        logger,
		activity: newRuntimeActivity(store, runtimeActivityOptions{
			RunErr:        runErrWithTimeoutContext,
			EventTimeout:  loggingDependencyTimeout,
			BotInstanceID: files.NormalizeBotInstanceID(botInstanceID),
			Warn:          slog.Warn,
		}),
		lifecycle:      newServiceLifecycle("message event service"),
		auditCache:     make(map[string]auditCacheEntry),
		auditCacheTTL:  2 * time.Second,
		auditEntryMax:  15 * time.Second,
		handlerCancels: make([]func(), 0, 4),
	}
}

// Start registers message event handlers
func (mes *MessageEventService) Start(ctx context.Context) error {
	if mes.session == nil {
		return fmt.Errorf("message event service discord session is nil")
	}

	if _, err := mes.lifecycle.Start(ctx); err != nil {
		return fmt.Errorf("MessageEventService.Start: %w", err)
	}

	// Load message cache configuration from persisted runtime_config,
	// but keep cache + versioning hardcoded enabled.
	{
		rc := files.RuntimeConfig{}
		features := (&files.BotConfig{}).ResolveFeatures("")
		if mes.configManager != nil && mes.configManager.Config() != nil {
			cfg := mes.configManager.Config()
			rc = cfg.RuntimeConfig
			features = cfg.ResolveFeatures("")
		}

		// Hardcoded enabled
		mes.cacheEnabled = true

		ttlHours := 72
		if rc.MessageCacheTTLHours > 0 {
			ttlHours = rc.MessageCacheTTLHours
		}
		mes.cacheTTL = time.Duration(ttlHours) * time.Hour

		mes.deleteOnLog = rc.MessageDeleteOnLog
		mes.cleanupEnabled = rc.MessageCacheCleanup && features.MessageCache.CleanupOnStartup

		// Hardcoded enabled
		mes.versioningEnabled = true
	}

	// Store should be injected and already initialized
	// Cleanup is gated by env and disabled by default (do not delete by default)
	if mes.store != nil && mes.cleanupEnabled {
		if err := mes.store.CleanupExpiredMessages(); err != nil {
			mes.logger.Warn("MessageEventService: startup cleanup failed", "error", err)
		}
	}
	if mes.store != nil {
		mes.messageCreateWriter = newMessageCreateWriter(mes.store, mes.writerMetrics, mes.logger)
		mes.messageCreateWriter.Start()
	}

	mes.handlerCancels = mes.handlerCancels[:0]
	mes.handlerCancels = append(mes.handlerCancels,
		mes.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
			runCtx, done, ok := mes.lifecycle.Begin()
			if !ok {
				return
			}
			defer done()

			mes.handleMessageCreate(runCtx, s, m)
		}),
		mes.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageUpdate) {
			runCtx, done, ok := mes.lifecycle.Begin()
			if !ok {
				return
			}
			defer done()

			mes.handleMessageUpdate(runCtx, s, m)
		}),
		mes.session.AddHandler(func(s *discordgo.Session, m *discordgo.MessageDelete) {
			runCtx, done, ok := mes.lifecycle.Begin()
			if !ok {
				return
			}
			defer done()

			mes.handleMessageDelete(runCtx, s, m)
		}),
	)

	if mes.taskRouter != nil {
		mes.taskRouter.RegisterHandler(taskTypeMessageUpdateProcess, mes.handleMessageUpdateTask)
		mes.taskRouter.RegisterHandler(taskTypeMessageDeleteProcess, mes.handleMessageDeleteTask)
	}

	// TTL cache handles cleanup internally

	mes.logger.Info("Message event service started")
	return nil
}

// Stop stops the service
func (mes *MessageEventService) Stop(ctx context.Context) error {
	if err := mes.lifecycle.Cancel(); err != nil {
		return fmt.Errorf("MessageEventService.Stop: %w", err)
	}

	for _, cancel := range mes.handlerCancels {
		if cancel != nil {
			cancel()
		}
	}
	mes.handlerCancels = nil

	waitErr := mes.lifecycle.Wait(ctx)
	if mes.messageCreateWriter != nil {
		if err := mes.messageCreateWriter.Stop(ctx); err != nil {
			return fmt.Errorf("MessageEventService.Stop: %w", err)
		}
		mes.messageCreateWriter = nil
	}
	if waitErr != nil {
		return waitErr
	}

	mes.logger.Info("Message event service stopped")
	return nil
}

// IsRunning returns whether the service is running
func (mes *MessageEventService) IsRunning() bool {
	return mes.lifecycle.IsRunning()
}

// handleMessageCreate stores messages for future comparisons
func (mes *MessageEventService) handleMessageCreate(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil {
		mes.logger.Debug("MessageCreate: nil event")
		return
	}
	if m.Author == nil {
		mes.logger.Debug("MessageCreate: nil author; skipping", "channelID", m.ChannelID)
		return
	}
	if m.Author.Bot {
		mes.logger.Debug("MessageCreate: ignoring bot message", "channelID", m.ChannelID, "userID", m.Author.ID)
		return
	}
	if err := ctx.Err(); err != nil {
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
			mes.logger.Debug("MessageCreate: empty content; will not cache", "channelID", m.ChannelID, "userID", m.Author.ID)
			return
		}
		// Use the summary as content for persistence
		m.Content = extra
		mes.logger.Debug("MessageCreate: content empty; using summary for cache", "channelID", m.ChannelID, "userID", m.Author.ID)
	}
	mes.logger.Debug("MessageCreate received", "channelID", m.ChannelID, "userID", m.Author.ID, "messageID", m.ID)

	// Check if this is a guild message without fetching the channel when possible
	guildID := m.GuildID
	if guildID == "" {
		// Fallback: get via channel only if necessary (likely DM)
		channel, err := s.Channel(m.ChannelID)
		if err != nil {
			mes.logger.Debug("MessageCreate: failed to fetch channel; skipping cache", "channelID", m.ChannelID, "error", err)
			return
		}
		guildID = channel.GuildID
	}
	if guildID == "" {
		mes.logger.Debug("MessageCreate: DM detected; skipping cache", "channelID", m.ChannelID)
		return
	}
	if !mes.handlesGuild(guildID) {
		return
	}

	emit := logpolicy.ShouldEmitLogEvent(mes.session, mes.configManager, logpolicy.LogEventMessageProcess, guildID)
	if !emit.Enabled {
		mes.logger.Debug("MessageCreate: message processing suppressed by policy", "guildID", guildID, "reason", emit.Reason)
		return
	}

	if guildConfig := mes.configManager.GuildConfig(guildID); guildConfig == nil {
		mes.logger.Debug("MessageCreate: no guild config; skipping cache", "guildID", guildID)
		return
	}

	mes.markEvent(ctx)

	if mes.store != nil && m.Author != nil {
		mes.persistMessageCreate(guildID, m)
	}
	mes.logger.Info("Message cached for monitoring", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.ID, "userID", m.Author.ID)
}

// handleMessageUpdate processes message edits
func (mes *MessageEventService) handleMessageUpdate(ctx context.Context, s *discordgo.Session, m *discordgo.MessageUpdate) {
	if m == nil {
		mes.logger.Debug("MessageUpdate: nil event")
		return
	}
	if m.Author != nil && m.Author.Bot {
		mes.logger.Debug("MessageUpdate: ignoring bot edit", "messageID", m.ID, "userID", m.Author.ID, "channelID", m.ChannelID)
		return
	}
	if err := ctx.Err(); err != nil {
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
	mes.logger.Info("MessageUpdate received", "messageID", m.ID, "userID", authorID, "guildID", m.GuildID, "channelID", m.ChannelID)

	if mes.taskRouter != nil {
		if err := mes.dispatchMessageUpdateTask(m); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				mes.logger.Debug("MessageUpdate: task already queued", "messageID", m.ID)
			} else {
				mes.logger.Error("MessageUpdate: failed to enqueue task", "messageID", m.ID, "error", err)
			}
		}
		return
	}

	if err := mes.processMessageUpdate(ctx, s, m, true); err != nil {
		mes.logger.Error("MessageUpdate: direct processing failed", "messageID", m.ID, "guildID", m.GuildID, "channelID", m.ChannelID, "error", err)
	}
}

// handleMessageDelete processes message deletions
func (mes *MessageEventService) handleMessageDelete(ctx context.Context, s *discordgo.Session, m *discordgo.MessageDelete) {
	if m == nil {
		return
	}
	if err := ctx.Err(); err != nil {
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
				mes.logger.Debug("MessageDelete: task already queued", "messageID", m.ID)
			} else {
				mes.logger.Error("MessageDelete: failed to enqueue task", "messageID", m.ID, "error", err)
			}
		}
		return
	}

	if err := mes.processMessageDelete(ctx, s, m, true); err != nil {
		mes.logger.Error("MessageDelete: direct processing failed", "messageID", m.ID, "guildID", m.GuildID, "channelID", m.ChannelID, "error", err)
	}
}

// Persistent storage (Postgres) handles expiration and cleanup

// markEvent stores the last event timestamp (best effort)
func (mes *MessageEventService) markEvent(ctx context.Context) {
	if mes.activity == nil {
		return
	}
	mes.activity.MarkEvent(ctx, "message_event_service")
}

func (mes *MessageEventService) deleteOnLogEnabled(guildID string) bool {
	if !mes.deleteOnLog {
		return false
	}
	if mes.configManager == nil {
		return mes.deleteOnLog
	}
	cfg := mes.configManager.Config()
	if cfg == nil {
		return mes.deleteOnLog
	}
	return cfg.ResolveFeatures(guildID).MessageCache.DeleteOnLog
}

// SetWriterMetrics attaches a metrics implementation for the async message
// persistence writer. Must be called before Start; if unset the writer uses
// NopMessageWriterMetrics, matching the qotd/moderation pattern.
func (mes *MessageEventService) SetWriterMetrics(metrics MessageWriterMetrics) {
	mes.writerMetrics = metrics
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
	payload := MessageUpdateTaskPayload{
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
	payload := MessageDeleteTaskPayload{
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

func (mes *MessageEventService) handleMessageUpdateTask(ctx context.Context, payload any) error {
	p, ok := payload.(MessageUpdateTaskPayload)
	if !ok || p.Update == nil {
		return fmt.Errorf("invalid payload for %s", taskTypeMessageUpdateProcess)
	}
	return mes.processMessageUpdate(ctx, mes.session, p.Update, false)
}

func (mes *MessageEventService) handleMessageDeleteTask(ctx context.Context, payload any) error {
	p, ok := payload.(MessageDeleteTaskPayload)
	if !ok || p.Delete == nil {
		return fmt.Errorf("invalid payload for %s", taskTypeMessageDeleteProcess)
	}
	return mes.processMessageDelete(ctx, mes.session, p.Delete, false)
}

func (mes *MessageEventService) processMessageUpdate(ctx context.Context, s *discordgo.Session, m *discordgo.MessageUpdate, allowWait bool) error {
	if m == nil {
		return nil
	}
	if m.Author != nil && m.Author.Bot {
		return nil
	}

	mes.markEvent(ctx)

	// Consult persistence (Postgres store) to get the original message (with guild/channel fallback)
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
	if guildID != "" && !mes.handlesGuild(guildID) {
		return nil
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
		mes.logger.Info("Message edit detected but original not in cache/persistence", "messageID", m.ID, "userID", authorID)
		return nil
	}

	emit := logpolicy.ShouldEmitLogEvent(mes.session, mes.configManager, logpolicy.LogEventMessageEdit, cached.GuildID)
	if !emit.Enabled {
		if emit.Reason == logpolicy.EmitReasonNoChannelConfigured {
			mes.logger.Info("Message log channel not configured for guild; edit notification not sent", "guildID", cached.GuildID, "messageID", m.ID)
		} else {
			mes.logger.Debug("MessageUpdate: notification suppressed by policy", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "reason", emit.Reason)
		}
		return nil
	}
	logChannelID := emit.ChannelID

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
		mes.logger.Debug("MessageUpdate: unable to resolve content; skipping notification", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID)
		return nil
	}
	// Check that the content actually changed (compare effective strings)
	if cached.Content == m.Content {
		mes.logger.Debug("MessageUpdate: content unchanged; skipping notification", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID)
		return nil
	}

	mes.logger.Info("Message edit detected", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID, "username", cached.Author.Username)

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
			mes.logger.Error("Failed to send message edit notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			mes.logger.Info("Message edit notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
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
			mes.logger.Error("Failed to send message edit notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			mes.logger.Info("Message edit notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
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
		mes.persistMessageUpdate(updated, m.Content)
	}
	mes.logger.Info("MessageUpdate: store updated with new content", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID)
	return nil
}

func (mes *MessageEventService) processMessageDelete(ctx context.Context, s *discordgo.Session, m *discordgo.MessageDelete, allowWait bool) error {
	if m == nil {
		return nil
	}

	mes.markEvent(ctx)

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
	if guildID != "" && !mes.handlesGuild(guildID) {
		return nil
	}

	cached := mes.lookupCachedMessage(guildID, m.ID, allowWait)
	if cached == nil {
		if !allowWait && mes.store != nil && guildID != "" {
			if !mes.shouldRetryMessageDeleteCacheMiss(s, guildID, m) {
				mes.logger.Debug("MessageDelete: cache miss for uncached message; skipping retry", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.ID)
				return nil
			}
			return fmt.Errorf("%w: message delete cache miss", task.ErrRetrySilent)
		}
		mes.logger.Info("Message delete detected but original not in cache/persistence", "messageID", m.ID, "channelID", m.ChannelID)
		return nil
	}

	emit := logpolicy.ShouldEmitLogEvent(mes.session, mes.configManager, logpolicy.LogEventMessageDelete, cached.GuildID)
	if !emit.Enabled {
		if emit.Reason == logpolicy.EmitReasonNoChannelConfigured {
			mes.logger.Info("Message log channel not configured for guild; delete notification not sent", "guildID", cached.GuildID, "messageID", m.ID)
		} else {
			mes.logger.Debug("MessageDelete: notification suppressed by policy", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "reason", emit.Reason)
		}
		// Deletion from store is disabled by default
		if mes.deleteOnLogEnabled(cached.GuildID) && mes.store != nil {
			mes.persistMessageDelete(cached, true, false, "message_delete_suppressed")
		}
		return nil
	}
	logChannelID := emit.ChannelID

	// Skip if bot
	if cached.Author.Bot {
		// Deletion from store is disabled by default
		if mes.deleteOnLogEnabled(cached.GuildID) && mes.store != nil {
			mes.persistMessageDelete(cached, true, false, "message_delete_bot")
		}
		return nil
	}

	mes.logger.Info("Message delete detected", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.ID, "userID", cached.Author.ID, "username", cached.Author.Username)

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
			mes.logger.Error("Failed to send message delete notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			mes.logger.Info("Message delete notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
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
			mes.logger.Error("Failed to send message delete notification", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID, "error", err)
		} else {
			mes.logger.Info("Message delete notification sent successfully", "guildID", cached.GuildID, "messageID", m.ID, "channelID", logChannelID)
		}
	}

	// Remove from cache and persistence (disabled by default)
	// Versioned history (delete) - hardcoded enabled
	mes.persistMessageDelete(cached, mes.deleteOnLogEnabled(cached.GuildID) && mes.store != nil, mes.versioningEnabled && mes.store != nil && cached.Author != nil, "message_delete")
	return nil
}

func (mes *MessageEventService) shouldRetryMessageDeleteCacheMiss(s *discordgo.Session, guildID string, m *discordgo.MessageDelete) bool {
	if mes == nil || strings.TrimSpace(guildID) == "" || m == nil {
		return false
	}

	processDecision := logpolicy.ShouldEmitLogEvent(mes.session, mes.configManager, logpolicy.LogEventMessageProcess, guildID)
	if !processDecision.Enabled {
		return false
	}

	deleteDecision := logpolicy.ShouldEmitLogEvent(mes.session, mes.configManager, logpolicy.LogEventMessageDelete, guildID)
	if !deleteDecision.Enabled {
		return false
	}

	if s != nil && s.State != nil {
		if msg, err := s.State.Message(m.ChannelID, m.ID); err == nil && msg != nil && msg.Author != nil && msg.Author.Bot {
			return false
		}
	}

	return true
}

func (mes *MessageEventService) lookupCachedMessage(guildID, messageID string, allowWait bool) *CachedMessage {
	if mes.store == nil || guildID == "" || messageID == "" {
		return nil
	}
	if mes.messageCreateWriter != nil {
		if cached := mes.messageCreateWriter.Lookup(guildID, messageID); cached != nil {
			return cached
		}
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

func (mes *MessageEventService) persistMessageCreate(guildID string, m *discordgo.MessageCreate) {
	if mes.store == nil || m == nil || m.Author == nil {
		return
	}

	now := time.Now().UTC()
	record := storage.MessageRecord{
		GuildID:        guildID,
		MessageID:      m.ID,
		ChannelID:      m.ChannelID,
		AuthorID:       m.Author.ID,
		AuthorUsername: m.Author.Username,
		AuthorAvatar:   m.Author.Avatar,
		Content:        m.Content,
		CachedAt:       now,
		ExpiresAt:      now.Add(mes.cacheTTL),
		HasExpiry:      true,
	}

	var version *storage.MessageVersion
	if mes.versioningEnabled {
		version = &storage.MessageVersion{
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
			CreatedAt:   now,
		}
	}

	metric := storage.DailyMessageCountDelta{
		GuildID:   guildID,
		ChannelID: m.ChannelID,
		UserID:    m.Author.ID,
		Day:       now.Format("2006-01-02"),
		Count:     1,
	}

	if mes.messageCreateWriter != nil {
		if err := mes.messageCreateWriter.Enqueue(record, version, metric); err == nil {
			return
		} else {
			mes.logger.Warn("MessageCreate: async writer enqueue failed; falling back to synchronous persistence", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.ID, "userID", m.Author.ID, "error", err)
		}
	}

	if err := mes.store.UpsertMessage(record); err != nil {
		mes.logger.Warn("MessageCreate: failed to persist message cache entry", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.ID, "userID", m.Author.ID, "error", err)
	}
	if version != nil {
		if err := mes.store.InsertMessageVersion(*version); err != nil {
			mes.logger.Warn("MessageCreate: failed to persist message version", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.ID, "userID", m.Author.ID, "error", err)
		}
	}
	if err := mes.store.IncrementDailyMessageCount(guildID, m.ChannelID, m.Author.ID, now); err != nil {
		mes.logger.Warn("MessageCreate: failed to increment daily message metric", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.ID, "userID", m.Author.ID, "error", err)
	}
}

func (mes *MessageEventService) persistMessageUpdate(updated *CachedMessage, content string) {
	if mes == nil || mes.store == nil || updated == nil || updated.Author == nil {
		return
	}

	now := time.Now().UTC()
	record := storage.MessageRecord{
		GuildID:        updated.GuildID,
		MessageID:      updated.ID,
		ChannelID:      updated.ChannelID,
		AuthorID:       updated.Author.ID,
		AuthorUsername: updated.Author.Username,
		AuthorAvatar:   updated.Author.Avatar,
		Content:        content,
		CachedAt:       now,
		ExpiresAt:      now.Add(mes.cacheTTL),
		HasExpiry:      true,
	}

	var version *storage.MessageVersion
	if mes.versioningEnabled {
		version = &storage.MessageVersion{
			GuildID:   updated.GuildID,
			MessageID: updated.ID,
			ChannelID: updated.ChannelID,
			AuthorID:  updated.Author.ID,
			EventType: "edit",
			Content:   content,
			CreatedAt: now,
		}
	}

	if mes.messageCreateWriter != nil {
		if err := mes.messageCreateWriter.Enqueue(record, version, storage.DailyMessageCountDelta{}); err == nil {
			return
		} else {
			mes.logger.Warn("MessageUpdate: async writer enqueue failed; falling back to synchronous persistence", "guildID", updated.GuildID, "channelID", updated.ChannelID, "messageID", updated.ID, "userID", updated.Author.ID, "error", err)
		}
	}

	if err := mes.store.UpsertMessage(record); err != nil {
		mes.logger.Warn("MessageUpdate: failed to persist updated message cache entry", "guildID", updated.GuildID, "channelID", updated.ChannelID, "messageID", updated.ID, "userID", updated.Author.ID, "error", err)
	}
	if version != nil {
		if err := mes.store.InsertMessageVersion(*version); err != nil {
			mes.logger.Warn("MessageUpdate: failed to persist message edit version", "guildID", updated.GuildID, "channelID", updated.ChannelID, "messageID", updated.ID, "userID", updated.Author.ID, "error", err)
		}
	}
}

func (mes *MessageEventService) persistMessageDelete(cached *CachedMessage, deleteFromStore bool, includeVersion bool, operation string) {
	if mes == nil || mes.store == nil || cached == nil {
		return
	}

	var version *storage.MessageVersion
	if includeVersion && cached.Author != nil {
		version = &storage.MessageVersion{
			GuildID:   cached.GuildID,
			MessageID: cached.ID,
			ChannelID: cached.ChannelID,
			AuthorID:  cached.Author.ID,
			EventType: "delete",
			Content:   cached.Content,
			CreatedAt: time.Now().UTC(),
		}
	}

	if mes.messageCreateWriter != nil {
		var err error
		switch {
		case deleteFromStore:
			err = mes.messageCreateWriter.EnqueueDelete(cached.GuildID, cached.ID, version)
		case version != nil:
			err = mes.messageCreateWriter.EnqueueVersion(*version)
		default:
			return
		}
		if err == nil {
			return
		}
		mes.logger.Warn("MessageDelete: async writer enqueue failed; falling back to synchronous persistence", "operation", operation, "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", cached.ID, "userID", cached.Author.ID, "error", err)
	}

	if version != nil {
		if err := mes.store.InsertMessageVersion(*version); err != nil {
			mes.logger.Warn("MessageDelete: failed to persist message delete version", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", cached.ID, "userID", cached.Author.ID, "error", err)
		}
	}
	if deleteFromStore {
		if err := mes.store.DeleteMessage(cached.GuildID, cached.ID); err != nil {
			mes.logger.Warn("MessageDelete: failed to delete message cache entry", "operation", operation, "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", cached.ID, "error", err)
		}
	}
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

func (mes *MessageEventService) handlesGuild(guildID string) bool {
	if mes == nil || mes.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(mes.botInstanceID) == "" && files.NormalizeBotInstanceID(mes.defaultBotID) == "" {
		return true
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}
	guild := mes.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}
	return guild.EffectiveBotInstanceID(mes.defaultBotID) == files.NormalizeBotInstanceID(mes.botInstanceID)
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
