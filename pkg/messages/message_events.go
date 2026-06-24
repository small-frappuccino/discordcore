package messages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordcore/pkg/service"

	"github.com/small-frappuccino/discordcore/pkg/system"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// CachedMessage stores message data for comparison
type CachedMessage struct {
	ID             string
	Content        string
	AuthorID       string
	AuthorUsername string
	AuthorAvatar   string
	AuthorBot      bool
	ChannelID      string
	GuildID        string
	Timestamp      time.Time
}

type auditCacheEntry struct {
	fetchedAt time.Time
	entries   map[string]auditCacheValue
}

type auditCacheValue struct {
	userID    string
	createdAt time.Time
}

type auditCacheState struct {
	mu          sync.Mutex
	entries     map[string]auditCacheEntry
	ttl         time.Duration
	entryMaxAge time.Duration
}

func newAuditCacheState(ttl, maxAge time.Duration) *auditCacheState {
	return &auditCacheState{
		entries:     make(map[string]auditCacheEntry),
		ttl:         ttl,
		entryMaxAge: maxAge,
	}
}

func (s *auditCacheState) get(guildID string) (auditCacheEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[guildID]
	if !ok || (s.ttl > 0 && time.Since(entry.fetchedAt) > s.ttl) {
		return auditCacheEntry{}, false
	}
	return entry, true
}

func (s *auditCacheState) set(guildID string, entry auditCacheEntry) {
	if s.ttl <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[guildID] = entry
}

func (s *auditCacheState) pickEntry(entries map[string]auditCacheValue, key string) string {
	if entries == nil {
		return ""
	}
	val, ok := entries[key]
	if !ok {
		return ""
	}
	if s.entryMaxAge > 0 && time.Since(val.createdAt) > s.entryMaxAge {
		return ""
	}
	return val.userID
}

// MessageEventService manages message events (delete/edit)
type MessageEventService struct {
	configManager *files.ConfigManager
	botInstanceID string
	sink          MessageSink
	store         Repository
	systemRepo    system.Repository
	activity      *service.RuntimeActivity
	lifecycle     service.BaseLifecycle
	logger        *slog.Logger

	// Message cache configuration (populated from persisted runtime_config)
	cacheEnabled   bool
	cacheTTL       time.Duration
	deleteOnLog    bool
	cleanupEnabled bool

	// Versioning configuration (populated from persisted runtime_config)
	versioningEnabled bool

	auditCache *auditCacheState

	taskRouter *task.TaskRouter

	messageCreateWriter *messageCreateWriter
	writerMetrics       MessageWriterMetrics

	// DiscordAdapter provides a pure domain interface for Discord API operations
	// without leaking the underlying gateway or state SDK types.
	discordAdapter DiscordAdapter
}

// DiscordAdapter defines the required Discord API interactions for message events.
type DiscordAdapter interface {
	ChannelGuildID(channelID string) (string, error)
	MessageContent(channelID, messageID string) (string, error)
	IsMessageAuthorBot(channelID, messageID string) (bool, error)
	Username(userID string) (string, error)
	FetchMessageDeleteAuditLogs(guildID string) ([]AuditLogMessageDeleteEntry, error)
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
	Update     MessageUpdateIntent
	ReceivedAt time.Time
}

// MessageDeleteTaskPayload is the task payload for a deferred message-delete
// event. ReceivedAt records when the gateway event arrived, so latency can be
// measured against task execution time.
type MessageDeleteTaskPayload struct {
	Delete     MessageDeleteIntent
	ReceivedAt time.Time
}

// EventServiceDeps holds dependencies for the MessageEventService
type EventServiceDeps struct {
	ConfigManager  *files.ConfigManager
	Sink           MessageSink
	Store          Repository
	SystemRepo     system.Repository
	BotInstanceID  string
	Logger         *slog.Logger
	DiscordAdapter DiscordAdapter
}

// NewMessageEventServiceForBot creates a message event service scoped to a bot
// instance assignment.
func NewMessageEventServiceForBot(deps EventServiceDeps) *MessageEventService {
	return &MessageEventService{
		configManager: deps.ConfigManager,
		botInstanceID: files.NormalizeBotInstanceID(deps.BotInstanceID),
		sink:          deps.Sink,
		store:         deps.Store,
		systemRepo:    deps.SystemRepo,
		logger:        deps.Logger,
		activity: service.NewRuntimeActivity(deps.SystemRepo, service.RuntimeActivityOptions{
			RunErr:        service.RunErrWithTimeoutContext,
			EventTimeout:  service.DependencyTimeout,
			BotInstanceID: files.NormalizeBotInstanceID(deps.BotInstanceID),
			Logger:        deps.Logger,
		}),
		lifecycle:      service.NewBaseLifecycle("message event service"),
		discordAdapter: deps.DiscordAdapter,
		auditCache:     newAuditCacheState(2*time.Second, 15*time.Second),
	}
}

// Start registers message event handlers
func (mes *MessageEventService) Start(ctx context.Context) error {
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

// Name returns the unique name of the service
func (mes *MessageEventService) Name() string { return "messages" }

// Type returns the service type
func (mes *MessageEventService) Type() service.ServiceType { return service.ServiceType("messages") }

// Priority returns the startup priority
func (mes *MessageEventService) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns a list of dependencies
func (mes *MessageEventService) Dependencies() []string { return nil }

// HealthCheck returns the current health status
func (mes *MessageEventService) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   mes.IsRunning(),
		Message:   "Message Event Service running",
		LastCheck: time.Now(),
	}
}

// Stats returns runtime statistics
func (mes *MessageEventService) Stats() service.ServiceStats {
	return service.ServiceStats{}
}

// IngestMessageCreate stores messages for future comparisons
func (mes *MessageEventService) IngestMessageCreate(ctx context.Context, m MessageCreateIntent) {
	if m.MessageID == "" {
		mes.logger.Debug("MessageCreate: nil event")
		return
	}
	if m.AuthorID == "" {
		mes.logger.Debug("MessageCreate: invalid author; skipping", "channelID", m.ChannelID)
		return
	}
	if m.AuthorBot {
		mes.logger.Debug("MessageCreate: ignoring bot message", "channelID", m.ChannelID, "userID", m.AuthorID)
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"message_create",
		slog.String("guildID", m.GuildID),
		slog.String("channelID", m.ChannelID),
		slog.String("messageID", m.MessageID),
		slog.String("userID", m.AuthorID),
	)
	defer done()

	if m.Content == "" {
		// Build a concise summary for non-text messages so we can still cache deletes/edits
		extra := ""
		if m.Attachments > 0 {
			extra += fmt.Sprintf("[attachments: %d] ", m.Attachments)
		}
		if m.Embeds > 0 {
			extra += fmt.Sprintf("[embeds: %d] ", m.Embeds)
		}
		if m.Stickers > 0 {
			extra += fmt.Sprintf("[stickers: %d] ", m.Stickers)
		}
		if extra == "" {
			mes.logger.Debug("MessageCreate: empty content; will not cache", "channelID", m.ChannelID, "userID", m.AuthorID)
			return
		}
		// Use the summary as content for persistence
		m.Content = extra
		mes.logger.Debug("MessageCreate: content empty; using summary for cache", "channelID", m.ChannelID, "userID", m.AuthorID)
	}
	mes.logger.Debug("MessageCreate received", "channelID", m.ChannelID, "userID", m.AuthorID, "messageID", m.MessageID)

	// Check if this is a guild message without fetching the channel when possible
	guildID := m.GuildID
	if guildID == "" {
		// Fallback: get via channel only if necessary (likely DM)
		fetchedGuildID, err := mes.discordAdapter.ChannelGuildID(m.ChannelID)
		if err != nil {
			mes.logger.Debug("MessageCreate: failed to fetch channel; skipping cache", "channelID", m.ChannelID, "error", err)
			return
		}
		guildID = fetchedGuildID
	}
	if guildID == "" {
		mes.logger.Debug("MessageCreate: DM detected; skipping cache", "channelID", m.ChannelID)
		return
	}
	if !mes.handlesGuild(guildID) {
		return
	}

	emit := logging.CheckFeatureEnabled(mes.configManager, logging.LogEventMessageProcess, guildID)
	if !emit.Enabled {
		mes.logger.Debug("MessageCreate: message processing suppressed by policy", "guildID", guildID, "reason", emit.Reason)
		return
	}

	if guildConfig := mes.configManager.GuildConfig(guildID); guildConfig == nil {
		mes.logger.Debug("MessageCreate: no guild config; skipping cache", "guildID", guildID)
		return
	}

	mes.markEvent(ctx)

	if mes.store != nil {
		mes.persistMessageCreate(guildID, m)
	}
	mes.logger.Info("Message cached for monitoring", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.MessageID, "userID", m.AuthorID)
}

// IngestMessageUpdate processes message edits
func (mes *MessageEventService) IngestMessageUpdate(ctx context.Context, m MessageUpdateIntent) {
	if m.MessageID == "" {
		mes.logger.Debug("MessageUpdate: nil event")
		return
	}
	// We can't strictly check if author is bot from UpdateEvent directly easily without cache if it's missing,
	// but we'll assume authorID is valid if passed and handled in adapter if missing.
	if err := ctx.Err(); err != nil {
		return
	}
	done := perf.StartGatewayEvent(
		"message_update",
		slog.String("guildID", m.GuildID),
		slog.String("channelID", m.ChannelID),
		slog.String("messageID", m.MessageID),
	)
	defer done()
	authorID := m.AuthorID
	mes.logger.Info("MessageUpdate received", "messageID", m.MessageID, "userID", authorID, "guildID", m.GuildID, "channelID", m.ChannelID)

	if mes.taskRouter != nil {
		if err := mes.dispatchMessageUpdateTask(m); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				mes.logger.Debug("MessageUpdate: task already queued", "messageID", m.MessageID)
			} else {
				mes.logger.Error("MessageUpdate: failed to enqueue task", "messageID", m.MessageID, "error", err)
			}
		}
		return
	}

	if err := mes.processMessageUpdate(ctx, m, true); err != nil {
		mes.logger.Error("MessageUpdate: direct processing failed", "messageID", m.MessageID, "guildID", m.GuildID, "channelID", m.ChannelID, "error", err)
	}
}

// handleMessageDelete processes message deletions
func (mes *MessageEventService) IngestMessageDelete(ctx context.Context, m MessageDeleteIntent) {
	if m.MessageID == "" {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}

	done := perf.StartGatewayEvent(
		"message_delete",
		slog.String("guildID", m.GuildID),
		slog.String("channelID", m.ChannelID),
		slog.String("messageID", m.MessageID),
	)
	defer done()

	if mes.taskRouter != nil {
		if err := mes.dispatchMessageDeleteTask(m); err != nil {
			if errors.Is(err, task.ErrDuplicateTask) {
				mes.logger.Debug("MessageDelete: task already queued", "messageID", m.MessageID)
			} else {
				mes.logger.Error("MessageDelete: failed to enqueue task", "messageID", m.MessageID, "error", err)
			}
		}
		return
	}

	if err := mes.processMessageDelete(ctx, m, true); err != nil {
		mes.logger.Error("MessageDelete: direct processing failed", "messageID", m.MessageID, "guildID", m.GuildID, "channelID", m.ChannelID, "error", err)
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

// SetTaskRouter sets task router.
func (mes *MessageEventService) SetTaskRouter(router *task.TaskRouter) {
	mes.taskRouter = router
}

func (mes *MessageEventService) dispatchMessageUpdateTask(m MessageUpdateIntent) error {
	if mes.taskRouter == nil || m.MessageID == "" {
		return nil
	}
	payload := MessageUpdateTaskPayload{
		Update:     m,
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
			IdempotencyKey: fmt.Sprintf("msg_update:%s:%s", group, m.MessageID),
			IdempotencyTTL: messageEventRetryTTL,
			MaxAttempts:    messageEventRetryMaxAttempts,
			InitialBackoff: messageEventRetryInitialBackoff,
			MaxBackoff:     messageEventRetryMaxBackoff,
		},
	})
}

func (mes *MessageEventService) dispatchMessageDeleteTask(m MessageDeleteIntent) error {
	if mes.taskRouter == nil || m.MessageID == "" {
		return nil
	}
	payload := MessageDeleteTaskPayload{
		Delete:     m,
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
			IdempotencyKey: fmt.Sprintf("msg_delete:%s:%s", group, m.MessageID),
			IdempotencyTTL: messageEventRetryTTL,
			MaxAttempts:    messageEventRetryMaxAttempts,
			InitialBackoff: messageEventRetryInitialBackoff,
			MaxBackoff:     messageEventRetryMaxBackoff,
		},
	})
}

func (mes *MessageEventService) handleMessageUpdateTask(ctx context.Context, payload any) error {
	p, ok := payload.(MessageUpdateTaskPayload)
	if !ok || p.Update.MessageID == "" {
		return fmt.Errorf("invalid payload for %s", taskTypeMessageUpdateProcess)
	}
	return mes.processMessageUpdate(ctx, p.Update, false)
}

func (mes *MessageEventService) handleMessageDeleteTask(ctx context.Context, payload any) error {
	p, ok := payload.(MessageDeleteTaskPayload)
	if !ok || p.Delete.MessageID == "" {
		return fmt.Errorf("invalid payload for %s", taskTypeMessageDeleteProcess)
	}
	return mes.processMessageDelete(ctx, p.Delete, false)
}

func (mes *MessageEventService) processMessageUpdate(ctx context.Context, m MessageUpdateIntent, allowWait bool) error {
	if m.MessageID == "" {
		return nil
	}
	mes.markEvent(ctx)

	// Consult persistence (Postgres store) to get the original message (with guild/channel fallback)
	guildID := m.GuildID
	if guildID == "" && mes.discordAdapter != nil {
		if gID, _ := mes.discordAdapter.ChannelGuildID(m.ChannelID); gID != "" {
			guildID = gID
		}
	}
	if guildID != "" && !mes.handlesGuild(guildID) {
		return nil
	}

	cached := mes.lookupCachedMessage(ctx, guildID, m.MessageID, allowWait)
	if cached == nil {
		authorID := m.AuthorID
		if !allowWait && mes.store != nil && guildID != "" {
			return fmt.Errorf("%w: message update cache miss", task.ErrRetrySilent)
		}
		mes.logger.Info("Message edit detected but original not in cache/persistence", "messageID", m.MessageID, "userID", authorID)
		return nil
	}

	// Logging delegated to sink
	emit := logging.CheckFeatureEnabled(mes.configManager, logging.LogEventMessageEdit, cached.GuildID)
	if !emit.Enabled {
		if emit.Reason == logging.EmitReasonNoChannelConfigured {
			mes.logger.Info("Message log channel not configured for guild; edit notification not sent", "guildID", cached.GuildID, "messageID", m.MessageID)
		} else {
			mes.logger.Debug("MessageUpdate: notification suppressed by policy", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.MessageID, "reason", emit.Reason)
		}
		return nil
	}

	// Ensure latest content; MessageUpdate may omit content. Also enrich empty content with context.
	contentResolved := true
	if m.Content == "" {
		if mes.discordAdapter == nil {
			contentResolved = false
		} else if content, err := mes.discordAdapter.MessageContent(m.ChannelID, m.MessageID); err == nil {
			m.Content = content
			contentResolved = true
		} else {
			contentResolved = false
		}
	}
	if !contentResolved {
		mes.logger.Debug("MessageUpdate: unable to resolve content; skipping notification", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.MessageID, "userID", cached.AuthorID)
		return nil
	}
	// Check that the content actually changed (compare effective strings)
	if cached.Content == m.Content {
		mes.logger.Debug("MessageUpdate: content unchanged; skipping notification", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.MessageID, "userID", cached.AuthorID)
		return nil
	}

	mes.logger.Info("Message edit detected", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.MessageID, "userID", cached.AuthorID, "username", cached.AuthorUsername)

	if mes.sink != nil {
		cd := &CachedMessageData{
			ID:             cached.ID,
			Content:        cached.Content,
			AuthorID:       cached.AuthorID,
			AuthorUsername: cached.AuthorUsername,
			AuthorBot:      cached.AuthorBot,
			ChannelID:      cached.ChannelID,
			GuildID:        cached.GuildID,
			Timestamp:      cached.Timestamp,
		}
		mes.sink.OnMessageUpdate(ctx, m, cd)
	}

	// Update persistence with new content
	updated := &CachedMessage{
		ID:             cached.ID,
		Content:        m.Content,
		AuthorID:       cached.AuthorID,
		AuthorUsername: cached.AuthorUsername,
		AuthorAvatar:   cached.AuthorAvatar,
		AuthorBot:      cached.AuthorBot,
		ChannelID:      cached.ChannelID,
		GuildID:        cached.GuildID,
		Timestamp:      cached.Timestamp,
	}
	if contentResolved && mes.cacheEnabled && mes.store != nil && updated.AuthorID != "" {
		mes.persistMessageUpdate(updated, m.Content)
	}
	mes.logger.Info("MessageUpdate: store updated with new content", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.MessageID)
	return nil
}

func (mes *MessageEventService) processMessageDelete(ctx context.Context, m MessageDeleteIntent, allowWait bool) error {
	if m.MessageID == "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Consult persistence (Postgres store) to get the original message (with guild/channel fallback)
	guildID := m.GuildID
	if guildID == "" && mes.discordAdapter != nil {
		if gID, _ := mes.discordAdapter.ChannelGuildID(m.ChannelID); gID != "" {
			guildID = gID
		}
	}
	if guildID != "" && !mes.handlesGuild(guildID) {
		return nil
	}

	cached := mes.lookupCachedMessage(ctx, guildID, m.MessageID, allowWait)
	if cached == nil {
		if !allowWait && mes.store != nil && guildID != "" {
			if !mes.shouldRetryMessageDeleteCacheMiss(guildID, m) {
				mes.logger.Debug("MessageDelete: cache miss for uncached message; skipping retry", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.MessageID)
				return nil
			}
			return fmt.Errorf("%w: message delete cache miss", task.ErrRetrySilent)
		}
		mes.logger.Info("Message delete detected but original not in cache/persistence", "messageID", m.MessageID, "channelID", m.ChannelID)
		return nil
	}

	emit := logging.CheckFeatureEnabled(mes.configManager, logging.LogEventMessageDelete, cached.GuildID)
	if !emit.Enabled {
		if emit.Reason == logging.EmitReasonNoChannelConfigured {
			mes.logger.Info("Message log channel not configured for guild; delete notification not sent", "guildID", cached.GuildID, "messageID", m.MessageID)
		} else {
			mes.logger.Debug("MessageDelete: notification suppressed by policy", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.MessageID, "reason", emit.Reason)
		}
		// Deletion from store is disabled by default
		if mes.deleteOnLogEnabled(cached.GuildID) && mes.store != nil {
			mes.persistMessageDelete(cached, true, false, "message_delete_suppressed")
		}
		return nil
	}

	// Skip if bot
	if cached.AuthorBot {
		// Deletion from store is disabled by default
		if mes.deleteOnLogEnabled(cached.GuildID) && mes.store != nil {
			mes.persistMessageDelete(cached, true, false, "message_delete_bot")
		}
		return nil
	}

	mes.logger.Info("Message delete detected", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", m.MessageID, "userID", cached.AuthorID, "username", cached.AuthorUsername)

	// Try to determine who deleted it (best effort via audit log)
	deletedBy := mes.determineDeletedBy(cached.GuildID, cached.ChannelID, cached.AuthorID)
	if deletedBy != "" {
		if username, err := mes.discordAdapter.Username(deletedBy); err == nil {
			m.ExecutorID = deletedBy
			m.ExecutorUsername = username
		}
	}

	if mes.sink != nil {
		cd := &CachedMessageData{
			ID:             cached.ID,
			Content:        cached.Content,
			AuthorID:       cached.AuthorID,
			AuthorUsername: cached.AuthorUsername,
			AuthorBot:      cached.AuthorBot,
			ChannelID:      cached.ChannelID,
			GuildID:        cached.GuildID,
			Timestamp:      cached.Timestamp,
		}
		mes.sink.OnMessageDelete(ctx, m, cd)
	}

	// Remove from cache and persistence (disabled by default)
	// Versioned history (delete) - hardcoded enabled
	mes.persistMessageDelete(cached, mes.deleteOnLogEnabled(cached.GuildID) && mes.store != nil, mes.versioningEnabled && mes.store != nil && cached.AuthorID != "", "message_delete")
	return nil
}

func (mes *MessageEventService) shouldRetryMessageDeleteCacheMiss(guildID string, m MessageDeleteIntent) bool {
	if mes == nil || strings.TrimSpace(guildID) == "" || m.MessageID == "" {
		return false
	}

	processDecision := logging.CheckFeatureEnabled(mes.configManager, logging.LogEventMessageProcess, guildID)
	if !processDecision.Enabled {
		return false
	}

	deleteDecision := logging.CheckFeatureEnabled(mes.configManager, logging.LogEventMessageDelete, guildID)
	if !deleteDecision.Enabled {
		return false
	}

	if mes.discordAdapter != nil {
		if isBot, err := mes.discordAdapter.IsMessageAuthorBot(m.ChannelID, m.MessageID); err == nil && isBot {
			return false
		}
	}

	return true
}

const (
	cachedMessageRetryDelay1 = 200 * time.Millisecond
	cachedMessageRetryDelay2 = 400 * time.Millisecond
)

func (mes *MessageEventService) lookupCachedMessage(ctx context.Context, guildID, messageID string, allowWait bool) *CachedMessage {
	if mes.store == nil || guildID == "" || messageID == "" {
		return nil
	}
	if mes.messageCreateWriter != nil {
		if cached := mes.messageCreateWriter.Lookup(guildID, messageID); cached != nil {
			return cached
		}
	}
	tryFetch := func() *CachedMessage {
		if rec, err := mes.store.GetMessage(ctx, guildID, messageID); err == nil && rec != nil {
			return &CachedMessage{
				ID:             rec.MessageID,
				Content:        rec.Content,
				AuthorID:       rec.AuthorID,
				AuthorUsername: rec.AuthorUsername,
				AuthorAvatar:   rec.AuthorAvatar,
				AuthorBot:      false,
				ChannelID:      rec.ChannelID,
				GuildID:        rec.GuildID,
				Timestamp:      rec.CachedAt,
			}
		}
		return nil
	}
	cached := tryFetch()
	if cached != nil || !allowWait {
		return cached
	}
	// Poll for the asynchronous store write, but yield to context cancellation
	// so a shutting-down handler does not block for the full backoff.
	delays := [...]time.Duration{cachedMessageRetryDelay1, cachedMessageRetryDelay2}
	for _, d := range delays {
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			timer.Stop()
			return tryFetch()
		case <-timer.C:
		}
		if cached = tryFetch(); cached != nil {
			return cached
		}
	}
	return cached
}

func (mes *MessageEventService) persistMessageCreate(guildID string, m MessageCreateIntent) {
	if mes.store == nil || m.AuthorID == "" {
		return
	}

	now := time.Now().UTC()
	record := Record{
		GuildID:        guildID,
		MessageID:      m.MessageID,
		ChannelID:      m.ChannelID,
		AuthorID:       m.AuthorID,
		AuthorUsername: m.AuthorUsername,
		AuthorAvatar:   "", // Left empty as we no longer have raw Avatar hash
		Content:        m.Content,
		CachedAt:       now,
		ExpiresAt:      now.Add(mes.cacheTTL),
		HasExpiry:      true,
	}

	var version *Version
	if mes.versioningEnabled {
		version = &Version{
			GuildID:     guildID,
			MessageID:   m.MessageID,
			ChannelID:   m.ChannelID,
			AuthorID:    m.AuthorID,
			Version:     1,
			EventType:   "create",
			Content:     m.Content,
			Attachments: m.Attachments,
			Embeds:      m.Embeds,
			Stickers:    m.Stickers,
			CreatedAt:   now,
		}
	}

	metric := DailyCountDelta{
		GuildID:   guildID,
		ChannelID: m.ChannelID,
		UserID:    m.AuthorID,
		Day:       now,
		Count:     1,
	}

	if mes.messageCreateWriter != nil {
		if err := mes.messageCreateWriter.Enqueue(record, version, metric); err == nil {
			return
		} else {
			mes.logger.Warn("MessageCreate: async writer enqueue failed; falling back to synchronous persistence", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.MessageID, "userID", m.AuthorID, "error", err)
		}
	}

	if err := mes.store.UpsertMessage(record); err != nil {
		mes.logger.Warn("MessageCreate: failed to persist message cache entry", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.MessageID, "userID", m.AuthorID, "error", err)
	}
	if version != nil {
		if err := mes.store.InsertMessageVersion(context.Background(), *version); err != nil {
			mes.logger.Warn("MessageCreate: failed to persist message version", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.MessageID, "userID", m.AuthorID, "error", err)
		}
	}
	if err := mes.store.IncrementDailyMessageCountsContext(context.Background(), []DailyCountDelta{{GuildID: guildID, ChannelID: m.ChannelID, UserID: m.AuthorID, Day: now, Count: 1}}); err != nil {
		mes.logger.Warn("MessageCreate: failed to increment daily message metric", "guildID", guildID, "channelID", m.ChannelID, "messageID", m.MessageID, "userID", m.AuthorID, "error", err)
	}
}

func (mes *MessageEventService) persistMessageUpdate(updated *CachedMessage, content string) {
	if mes == nil || mes.store == nil || updated == nil {
		return
	}

	now := time.Now().UTC()
	record := Record{
		GuildID:        updated.GuildID,
		MessageID:      updated.ID,
		ChannelID:      updated.ChannelID,
		AuthorID:       updated.AuthorID,
		AuthorUsername: updated.AuthorUsername,
		AuthorAvatar:   updated.AuthorAvatar,
		Content:        content,
		CachedAt:       now,
		ExpiresAt:      now.Add(mes.cacheTTL),
		HasExpiry:      true,
	}

	var version *Version
	if mes.versioningEnabled {
		version = &Version{
			GuildID:   updated.GuildID,
			MessageID: updated.ID,
			ChannelID: updated.ChannelID,
			AuthorID:  updated.AuthorID,
			EventType: "edit",
			Content:   content,
			CreatedAt: now,
		}
	}

	if mes.messageCreateWriter != nil {
		if err := mes.messageCreateWriter.Enqueue(record, version, DailyCountDelta{}); err == nil {
			return
		} else {
			mes.logger.Warn("MessageUpdate: async writer enqueue failed; falling back to synchronous persistence", "guildID", updated.GuildID, "channelID", updated.ChannelID, "messageID", updated.ID, "userID", updated.AuthorID, "error", err)
		}
	}

	if err := mes.store.UpsertMessage(record); err != nil {
		mes.logger.Warn("MessageUpdate: failed to persist updated message cache entry", "guildID", updated.GuildID, "channelID", updated.ChannelID, "messageID", updated.ID, "userID", updated.AuthorID, "error", err)
	}
	if version != nil {
		if err := mes.store.InsertMessageVersion(context.Background(), *version); err != nil {
			mes.logger.Warn("MessageUpdate: failed to persist message edit version", "guildID", updated.GuildID, "channelID", updated.ChannelID, "messageID", updated.ID, "userID", updated.AuthorID, "error", err)
		}
	}
}

func (mes *MessageEventService) persistMessageDelete(cached *CachedMessage, deleteFromStore bool, includeVersion bool, operation string) {
	if mes == nil || mes.store == nil || cached == nil {
		return
	}

	var version *Version
	if includeVersion && cached.AuthorID != "" {
		version = &Version{
			GuildID:   cached.GuildID,
			MessageID: cached.ID,
			ChannelID: cached.ChannelID,
			AuthorID:  cached.AuthorID,
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
		mes.logger.Warn("MessageDelete: async writer enqueue failed; falling back to synchronous persistence", "operation", operation, "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", cached.ID, "userID", cached.AuthorID, "error", err)
	}

	if version != nil {
		if err := mes.store.InsertMessageVersion(context.Background(), *version); err != nil {
			mes.logger.Warn("MessageDelete: failed to persist message delete version", "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", cached.ID, "userID", cached.AuthorID, "error", err)
		}
	}
	if deleteFromStore {
		if err := mes.store.DeleteMessage(context.Background(), cached.GuildID, cached.ID); err != nil {
			mes.logger.Warn("MessageDelete: failed to delete message cache entry", "operation", operation, "guildID", cached.GuildID, "channelID", cached.ChannelID, "messageID", cached.ID, "error", err)
		}
	}
}

func (mes *MessageEventService) handlesGuild(guildID string) bool {
	if mes == nil || mes.configManager == nil {
		return false
	}
	if files.NormalizeBotInstanceID(mes.botInstanceID) == "" {
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
	if !files.BelongsToBotInstance(*guild, mes.botInstanceID) {
		return false
	}
	resolvedID, _ := files.ResolveFeatureBotInstanceID(*guild, "logging")
	return resolvedID == mes.botInstanceID
}

// determineDeletedBy tries to resolve the actor for a deletion via audit log (best-effort).
func (mes *MessageEventService) determineDeletedBy(guildID, channelID, authorID string) string {
	if mes.auditCache == nil || mes.discordAdapter == nil {
		return ""
	}

	cacheKey := authorID + ":" + channelID
	cacheFallbackKey := authorID + ":"

	// Try cache first
	if entry, ok := mes.auditCache.get(guildID); ok {
		if executor := mes.auditCache.pickEntry(entry.entries, cacheKey); executor != "" {
			return executor
		}
		if executor := mes.auditCache.pickEntry(entry.entries, cacheFallbackKey); executor != "" {
			return executor
		}
		// If cached but not found, don't refetch (to avoid spamming API)
		return ""
	}

	// Fetch from Discord
	entriesList, err := mes.discordAdapter.FetchMessageDeleteAuditLogs(guildID)
	if err != nil {
		return ""
	}

	now := time.Now()
	entries := make(map[string]auditCacheValue)
	for _, entry := range entriesList {
		if entry.TargetID == "" || entry.UserID == "" {
			continue
		}
		if entry.UserID == authorID {
			continue
		}
		createdAt := now
		if ts, ok := snowflakeTimestamp(entry.EntryID); ok {
			createdAt = ts
		}
		if mes.auditCache.entryMaxAge > 0 && now.Sub(createdAt) > mes.auditCache.entryMaxAge {
			continue
		}
		targetOK := entry.TargetID == authorID
		channelOK := true
		if entry.ChannelID != "" {
			channelOK = entry.ChannelID == channelID
		}
		if targetOK && channelOK {
			entries[cacheKey] = newerAuditEntry(entries[cacheKey], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
			entries[cacheFallbackKey] = newerAuditEntry(entries[cacheFallbackKey], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
		}
		if entry.ChannelID != "" {
			key := entry.TargetID + ":" + entry.ChannelID
			entries[key] = newerAuditEntry(entries[key], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
		}
		fallbackKey := entry.TargetID + ":"
		entries[fallbackKey] = newerAuditEntry(entries[fallbackKey], auditCacheValue{userID: entry.UserID, createdAt: createdAt})
	}

	mes.auditCache.set(guildID, auditCacheEntry{
		fetchedAt: now,
		entries:   entries,
	})

	if executor := mes.auditCache.pickEntry(entries, cacheKey); executor != "" {
		return executor
	}
	return mes.auditCache.pickEntry(entries, cacheFallbackKey)
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
