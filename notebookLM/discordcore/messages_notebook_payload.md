# Domain Architecture: messages

## Layout Topology
```text
messages/
├── intents.go
├── message_create_writer.go
├── message_events.go
├── message_writer_observability.go
├── models.go
├── observability.go
├── repository.go
└── sink.go
```

## Source Stream Aggregation

// === FILE: pkg/messages/intents.go ===
```go
package messages

import "time"

// CachedMessageData provides a pure representation of the deleted or edited message state.
type CachedMessageData struct {
	ID             string
	Content        string
	AuthorID       string
	AuthorUsername string
	AuthorBot      bool
	ChannelID      string
	GuildID        string
	Timestamp      time.Time
}

// MessageDeleteIntent represents a message being deleted.
type MessageDeleteIntent struct {
	GuildID          string
	ChannelID        string
	MessageID        string
	ExecutorID       string
	ExecutorUsername string
}

// MessageUpdateIntent represents a message being edited.
type MessageUpdateIntent struct {
	GuildID   string
	ChannelID string
	MessageID string
	Content   string
	AuthorID  string
}

// MessageDeleteBulkIntent represents multiple messages being deleted.
type MessageDeleteBulkIntent struct {
	GuildID    string
	ChannelID  string
	MessageIDs []string
}

// MessageCreateIntent represents a message being created.
type MessageCreateIntent struct {
	GuildID        string
	ChannelID      string
	MessageID      string
	Content        string
	AuthorID       string
	AuthorUsername string
	AuthorBot      bool
	Attachments    int
	Embeds         int
	Stickers       int
	Timestamp      time.Time
}

// AuditLogMessageDeleteEntry represents a cached deletion audit log.
type AuditLogMessageDeleteEntry struct {
	EntryID   string
	TargetID  string
	UserID    string
	ChannelID string
}

```

// === FILE: pkg/messages/message_create_writer.go ===
```go
package messages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	messageCreateWriterQueueSize     = 2048
	messageCreateWriterFlushInterval = 250 * time.Millisecond
	messageCreateWriterMaxBatch      = 128
)

var errMessageCreateWriterStopped = errors.New("message create writer is stopped")

type writerState uint32

const (
	writerStateOpen writerState = iota
	writerStateStopping
	writerStateClosed
)

type messageWriteRecordOp uint8

const (
	messageWriteRecordOpNone messageWriteRecordOp = iota
	messageWriteRecordOpUpsert
	messageWriteRecordOpDelete
)

type messageWriteRequest struct {
	key      string
	token    uint64
	recordOp messageWriteRecordOp
	record   Record
	version  *Version
	metric   DailyCountDelta
}

type pendingMessageState struct {
	token   uint64
	deleted bool
	record  Record
}

type pendingMessageToken struct {
	key   string
	token uint64
}

type messageCreateWriter struct {
	store         Repository
	queue         chan messageWriteRequest
	stopCh        chan struct{}
	done          chan struct{}
	flushInterval time.Duration
	maxBatch      int
	metrics       MessageWriterMetrics

	state atomic.Uint32

	mu        sync.Mutex
	nextToken uint64
	pending   map[string]pendingMessageState
	stopOnce  sync.Once
	logger    *slog.Logger
}

func newMessageCreateWriter(store Repository, metrics MessageWriterMetrics, logger *slog.Logger) *messageCreateWriter {
	if metrics == nil {
		metrics = NopMessageWriterMetrics{}
	}
	writer := &messageCreateWriter{
		store:         store,
		queue:         make(chan messageWriteRequest, messageCreateWriterQueueSize),
		stopCh:        make(chan struct{}),
		done:          make(chan struct{}),
		flushInterval: messageCreateWriterFlushInterval,
		maxBatch:      messageCreateWriterMaxBatch,
		metrics:       metrics,
		pending:       make(map[string]pendingMessageState),
		logger:        logger,
	}
	writer.state.Store(uint32(writerStateOpen))
	return writer
}

// Start starts.
func (w *messageCreateWriter) Start() {
	if w == nil {
		return
	}
	go w.run()
}

// Stop stops.
func (w *messageCreateWriter) Stop(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.stopOnce.Do(func() {
		w.beginStop()
		close(w.stopCh)
	})
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *messageCreateWriter) stateValue() writerState {
	if w == nil {
		return writerStateClosed
	}
	return writerState(w.state.Load())
}

func (w *messageCreateWriter) beginStop() {
	if w == nil {
		return
	}
	w.state.CompareAndSwap(uint32(writerStateOpen), uint32(writerStateStopping))
}

// Enqueue enqueues.
func (w *messageCreateWriter) Enqueue(record Record, version *Version, metric DailyCountDelta) error {
	if w == nil {
		return fmt.Errorf("message create writer is nil")
	}
	key := messageCreatePendingKey(record.GuildID, record.MessageID)
	if key == "" {
		return fmt.Errorf("message create writer key is empty")
	}

	token := w.storePendingRecord(key, record)
	req := messageWriteRequest{
		key:      key,
		token:    token,
		recordOp: messageWriteRecordOpUpsert,
		record:   record,
		version:  cloneMessageVersion(version),
		metric:   metric,
	}
	if err := w.enqueueRequest(req); err != nil {
		w.clearPendingToken(key, token)
		return fmt.Errorf("messageCreateWriter.Enqueue: %w", err)
	}
	return nil
}

// EnqueueDelete enqueues delete.
func (w *messageCreateWriter) EnqueueDelete(guildID, messageID string, version *Version) error {
	if w == nil {
		return fmt.Errorf("message create writer is nil")
	}
	key := messageCreatePendingKey(guildID, messageID)
	if key == "" {
		return fmt.Errorf("message create writer key is empty")
	}

	token := w.storePendingDelete(key)
	req := messageWriteRequest{
		key:      key,
		token:    token,
		recordOp: messageWriteRecordOpDelete,
		version:  cloneMessageVersion(version),
	}
	if err := w.enqueueRequest(req); err != nil {
		w.clearPendingToken(key, token)
		return fmt.Errorf("messageCreateWriter.EnqueueDelete: %w", err)
	}
	return nil
}

// EnqueueVersion enqueues version.
func (w *messageCreateWriter) EnqueueVersion(version Version) error {
	if w == nil {
		return fmt.Errorf("message create writer is nil")
	}
	req := messageWriteRequest{
		key:     messageCreatePendingKey(version.GuildID, version.MessageID),
		version: cloneMessageVersion(&version),
	}
	return w.enqueueRequest(req)
}

func cloneMessageVersion(version *Version) *Version {
	if version == nil {
		return nil
	}
	cloned := *version
	return &cloned
}

func (w *messageCreateWriter) enqueueRequest(req messageWriteRequest) error {
	sent, err := w.trySendRequest(req)
	if err != nil {
		w.metrics.RecordEnqueueFailure(MessageWriterEnqueueFailureStopped)
		return fmt.Errorf("messageCreateWriter.enqueueRequest: %w", err)
	}
	if !sent {
		w.metrics.RecordEnqueueFailure(MessageWriterEnqueueFailureQueueFull)
		return fmt.Errorf("message create writer queue is full")
	}
	w.recordEnqueue(req)
	return nil
}

func (w *messageCreateWriter) trySendRequest(req messageWriteRequest) (sent bool, err error) {
	if w.stateValue() != writerStateOpen {
		return false, errMessageCreateWriterStopped
	}
	select {
	case w.queue <- req:
		return true, nil
	default:
		return false, nil
	}
}

func (w *messageCreateWriter) recordEnqueue(req messageWriteRequest) {
	switch req.recordOp {
	case messageWriteRecordOpUpsert:
		w.metrics.RecordEnqueueUpsert(req.version != nil, req.metric.Count != 0)
	case messageWriteRecordOpDelete:
		w.metrics.RecordEnqueueDelete(req.version != nil)
	default:
		if req.version != nil {
			w.metrics.RecordEnqueueVersion()
		}
	}
	w.metrics.ObserveQueueDepth(len(w.queue))
}

// Lookup lookups.
func (w *messageCreateWriter) Lookup(guildID, messageID string) *CachedMessage {
	if w == nil {
		return nil
	}
	key := messageCreatePendingKey(guildID, messageID)
	if key == "" {
		return nil
	}
	w.mu.Lock()
	pending, ok := w.pending[key]
	w.mu.Unlock()
	if !ok || pending.deleted {
		return nil
	}

	record := pending.record
	return &CachedMessage{
		ID:             record.MessageID,
		Content:        record.Content,
		AuthorID:       record.AuthorID,
		AuthorUsername: record.AuthorUsername,
		AuthorAvatar:   record.AuthorAvatar,
		AuthorBot:      false, // Assume false since bot messages aren't cached normally
		ChannelID:      record.ChannelID,
		GuildID:        record.GuildID,
		Timestamp:      record.CachedAt,
	}
}

func (w *messageCreateWriter) run() {
	defer func() {
		if r := recover(); r != nil {
			w.logger.Error("MessageCreateWriter loop panic caught", "panic", r, "stack", string(debug.Stack()))
		}
		w.state.Store(uint32(writerStateClosed))
		close(w.done)
	}()

	batch := make([]messageWriteRequest, 0, w.maxBatch)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flushBatch(batch)
		batch = batch[:0]
	}

	timer := time.NewTimer(w.flushInterval)
	defer timer.Stop()

	for {
		select {
		case req := <-w.queue:
			batch = append(batch, req)
			if len(batch) >= w.maxBatch {
				flush()
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(w.flushInterval)
			}
		case <-timer.C:
			flush()
			timer.Reset(w.flushInterval)
		case <-w.stopCh:
			for {
				select {
				case req := <-w.queue:
					batch = append(batch, req)
					if len(batch) >= w.maxBatch {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

func (w *messageCreateWriter) flushBatch(batch []messageWriteRequest) {
	if w == nil || w.store == nil || len(batch) == 0 {
		return
	}

	start := time.Now()
	defer func() {
		w.metrics.RecordFlush(len(batch), time.Since(start))
	}()

	upserts := make([]Record, 0, len(batch))
	upsertTokens := make([]pendingMessageToken, 0, len(batch))
	deletes := make([]DeleteKey, 0, len(batch))
	deleteTokens := make([]pendingMessageToken, 0, len(batch))
	versions := make([]Version, 0, len(batch))
	deltasByKey := make(map[string]DailyCountDelta, len(batch))

	for _, req := range batch {
		switch req.recordOp {
		case messageWriteRecordOpUpsert:
			if w.pendingStateMatches(req.key, req.token, false) {
				upserts = append(upserts, req.record)
				upsertTokens = append(upsertTokens, pendingMessageToken{key: req.key, token: req.token})
			}
		case messageWriteRecordOpDelete:
			if w.pendingStateMatches(req.key, req.token, true) {
				deletes = append(deletes, DeleteKey{
					GuildID:   req.record.GuildID,
					MessageID: req.record.MessageID,
				})
				if req.record.GuildID == "" || req.record.MessageID == "" {
					parts := strings.SplitN(req.key, ":", 2)
					if len(parts) == 2 {
						deletes[len(deletes)-1] = DeleteKey{GuildID: parts[0], MessageID: parts[1]}
					}
				}
				deleteTokens = append(deleteTokens, pendingMessageToken{key: req.key, token: req.token})
			}
		}
		if req.version != nil {
			versions = append(versions, *req.version)
		}
		if req.metric.Count != 0 {
			metricKey := strings.Join([]string{req.metric.GuildID, req.metric.ChannelID, req.metric.UserID, req.metric.Day.Format("2006-01-02")}, ":")
			delta := deltasByKey[metricKey]
			if delta.GuildID == "" {
				delta = req.metric
			} else {
				delta.Count += req.metric.Count
			}
			deltasByKey[metricKey] = delta
		}
	}

	if len(upserts) > 0 {
		if err := w.store.UpsertMessagesContext(context.Background(), upserts); err != nil {
			w.metrics.RecordFlushFallback(MessageWriterFlushOpUpsert, len(upserts))
			w.logger.Warn("MessageCreate writer: batch message upsert failed; falling back to sequential writes", "operation", "message_create_writer.flush_messages", "messages", len(upserts), "error", err)
			w.flushMessagesSequentially(upserts, upsertTokens)
		} else {
			w.metrics.RecordFlushSuccess(MessageWriterFlushOpUpsert, len(upserts))
			w.clearPendingTokens(upsertTokens)
		}
	}

	if len(deletes) > 0 {
		if err := w.store.DeleteMessagesContext(context.Background(), deletes); err != nil {
			w.metrics.RecordFlushFallback(MessageWriterFlushOpDelete, len(deletes))
			w.logger.Warn("MessageCreate writer: batch message delete failed; falling back to sequential deletes", "operation", "message_create_writer.flush_deletes", "messages", len(deletes), "error", err)
			w.flushDeletesSequentially(deletes, deleteTokens)
		} else {
			w.metrics.RecordFlushSuccess(MessageWriterFlushOpDelete, len(deletes))
			w.clearPendingTokens(deleteTokens)
		}
	}

	if len(versions) > 0 {
		if err := w.store.InsertMessageVersionsMixedBatchContext(context.Background(), versions); err != nil {
			w.metrics.RecordFlushFallback(MessageWriterFlushOpVersions, len(versions))
			w.logger.Warn("MessageCreate writer: batch history insert failed; falling back to sequential writes", "operation", "message_create_writer.flush_versions", "versions", len(versions), "error", err)
			w.flushVersionsSequentially(versions, "message_create_writer.flush_versions_fallback")
		} else {
			w.metrics.RecordFlushSuccess(MessageWriterFlushOpVersions, len(versions))
		}
	}

	if len(deltasByKey) > 0 {
		deltas := make([]DailyCountDelta, 0, len(deltasByKey))
		for _, delta := range deltasByKey {
			deltas = append(deltas, delta)
		}
		if err := w.store.IncrementDailyMessageCountsContext(context.Background(), deltas); err != nil {
			w.metrics.RecordFlushFallback(MessageWriterFlushOpMetricBuckets, len(deltas))
			w.logger.Warn("MessageCreate writer: batch daily metric flush failed; falling back to sequential increments", "operation", "message_create_writer.flush_metrics", "buckets", len(deltas), "error", err)
			for _, delta := range deltas {
				if err := w.store.IncrementDailyMessageCountsContext(context.Background(), []DailyCountDelta{delta}); err != nil {
					w.logger.Warn("MessageCreate writer: sequential daily metric increment failed", "operation", "message_create_writer.flush_metrics_fallback", "guildID", delta.GuildID, "channelID", delta.ChannelID, "userID", delta.UserID, "error", err)
				} else {
					w.metrics.RecordFlushSuccess(MessageWriterFlushOpMetricBuckets, 1)
				}
			}
		} else {
			w.metrics.RecordFlushSuccess(MessageWriterFlushOpMetricBuckets, len(deltas))
		}
	}
}

func (w *messageCreateWriter) flushMessagesSequentially(records []Record, tokens []pendingMessageToken) {
	for i, record := range records {
		if err := w.store.UpsertMessage(record); err != nil {
			w.logger.Warn("MessageCreate writer: sequential message upsert failed", "operation", "message_create_writer.flush_messages_fallback", "guildID", record.GuildID, "channelID", record.ChannelID, "messageID", record.MessageID, "userID", record.AuthorID, "error", err)
			continue
		}
		w.metrics.RecordFlushSuccess(MessageWriterFlushOpUpsert, 1)
		if i < len(tokens) {
			w.clearPendingToken(tokens[i].key, tokens[i].token)
		}
	}
}

func (w *messageCreateWriter) flushDeletesSequentially(keys []DeleteKey, tokens []pendingMessageToken) {
	for i, key := range keys {
		if err := w.store.DeleteMessage(context.Background(), key.GuildID, key.MessageID); err != nil {
			w.logger.Warn("MessageCreate writer: sequential message delete failed", "operation", "message_create_writer.flush_deletes_fallback", "guildID", key.GuildID, "messageID", key.MessageID, "error", err)
			continue
		}
		w.metrics.RecordFlushSuccess(MessageWriterFlushOpDelete, 1)
		if i < len(tokens) {
			w.clearPendingToken(tokens[i].key, tokens[i].token)
		}
	}
}

func (w *messageCreateWriter) flushVersionsSequentially(versions []Version, operation string) {
	for _, version := range versions {
		if err := w.store.InsertMessageVersion(context.Background(), version); err != nil {
			w.logger.Warn("MessageCreate writer: sequential history insert failed", "operation", operation, "guildID", version.GuildID, "channelID", version.ChannelID, "messageID", version.MessageID, "userID", version.AuthorID, "eventType", version.EventType, "error", err)
			continue
		}
		w.metrics.RecordFlushSuccess(MessageWriterFlushOpVersions, 1)
	}
}

func (w *messageCreateWriter) storePendingRecord(key string, record Record) uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nextToken++
	token := w.nextToken
	w.pending[key] = pendingMessageState{
		token:   token,
		deleted: false,
		record:  record,
	}
	return token
}

func (w *messageCreateWriter) storePendingDelete(key string) uint64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.nextToken++
	token := w.nextToken
	state := pendingMessageState{
		token:   token,
		deleted: true,
	}
	if current, ok := w.pending[key]; ok {
		state.record = current.record
	}
	w.pending[key] = state
	return token
}

func (w *messageCreateWriter) pendingStateMatches(key string, token uint64, deleted bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	current, ok := w.pending[key]
	return ok && current.token == token && current.deleted == deleted
}

func (w *messageCreateWriter) clearPendingTokens(tokens []pendingMessageToken) {
	for _, token := range tokens {
		w.clearPendingToken(token.key, token.token)
	}
}

func (w *messageCreateWriter) clearPendingToken(key string, token uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	current, ok := w.pending[key]
	if !ok || current.token != token {
		return
	}
	delete(w.pending, key)
}

func messageCreatePendingKey(guildID, messageID string) string {
	guildID = strings.TrimSpace(guildID)
	messageID = strings.TrimSpace(messageID)
	if guildID == "" || messageID == "" {
		return ""
	}
	return guildID + ":" + messageID
}

```

// === FILE: pkg/messages/message_events.go ===
```go
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

```

// === FILE: pkg/messages/message_writer_observability.go ===
```go
package messages

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/observability"
)

// MessageWriterMetrics is the narrow observability seam the async message
// persistence writer (messageCreateWriter) writes through. The interface
// intentionally hides whether the counters are in-memory, shipped to
// Prometheus, or thrown away — the writer code stays the same in all three
// worlds. NopMessageWriterMetrics is the default so the writer can run without
// explicit metrics wiring; the in-memory implementation
// (NewInMemoryMessageWriterMetrics) is what a future writer-aware health
// endpoint reads from.
//
// Naming is prefixed with MessageWriter to avoid colliding with the
// monitoring service's Metrics / NopMetrics / InMemoryMetrics types defined
// in observability.go. The two subsystems live in the same package but track
// disjoint concerns.
//
// Method shape is "Record<event>(labels, optional duration)" rather than
// "GetCounter('name').Inc()" because a typed surface catches event-naming
// drift at compile time. Adding a new event is one method on this interface
// plus the corresponding field on the snapshot; ad-hoc string keys are not
// supported on purpose.
// MessageWriterEnqueueMetrics tracks the producer-side telemetry for the message writer.
type MessageWriterEnqueueMetrics interface {
	// RecordEnqueueUpsert is called once per accepted upsert enqueue
	// (Enqueue). version and metric indicate whether the same request
	// also carried a versioned-history insert and/or a daily-metric
	// delta, so the snapshot can attribute side writes back to the
	// upsert call that produced them.
	RecordEnqueueUpsert(version bool, metric bool)

	// RecordEnqueueDelete is called once per accepted delete enqueue
	// (EnqueueDelete). version indicates whether the same request
	// carried a versioned-history insert.
	RecordEnqueueDelete(version bool)

	// RecordEnqueueVersion is called once per pure EnqueueVersion call
	// (no upsert/delete on the same request).
	RecordEnqueueVersion()

	// RecordEnqueueFailure is called when an enqueue attempt was
	// rejected. cause is one of the MessageWriterEnqueueFailure*
	// tokens; operators build alerts against the stable string set.
	RecordEnqueueFailure(cause string)

	// ObserveQueueDepth is called after each accepted enqueue with the
	// channel buffer occupancy. Tracks the max watermark; useful as the
	// backpressure headline number.
	ObserveQueueDepth(depth int)
}

// MessageWriterFlushMetrics tracks the consumer-side batch flush telemetry.
type MessageWriterFlushMetrics interface {
	// RecordFlush is called once per flush cycle, success or failure,
	// with the batch request count and wall-clock duration. Operators
	// read this as the writer's drain rate.
	RecordFlush(batchSize int, duration time.Duration)

	// RecordFlushSuccess is called per successful store write. op is
	// one of the MessageWriterFlushOp* tokens. count is the number of
	// rows that landed (batch path: full batch size; sequential fallback
	// path: 1 per row). Operators read FlushedByOp as "data that
	// actually made it to Postgres", independent of which path it took.
	RecordFlushSuccess(op string, count int)

	// RecordFlushFallback is called once per batched store call that
	// rejected the batch and forced the writer onto the per-row
	// sequential path. count is the batch size that fell back. A
	// non-zero rate is the explicit "Postgres is rejecting bulk writes"
	// signal — operators correlate it against batch-store latency.
	RecordFlushFallback(op string, count int)
}

// MessageWriterMetrics is the union of all observability seams the async message
// persistence writer (messageCreateWriter) writes through.
type MessageWriterMetrics interface {
	MessageWriterEnqueueMetrics
	MessageWriterFlushMetrics
}

// Stable cause tokens recorded by RecordEnqueueFailure. Renames are a
// breaking change for operators with alerts pinned to these strings.
const (
	MessageWriterEnqueueFailureQueueFull = "queue_full"
	MessageWriterEnqueueFailureStopped   = "stopped"
)

// Stable op tokens recorded by RecordFlushSuccess and RecordFlushFallback.
// One token per write side: the message row itself (upsert/delete), the
// versioned-history insert, and the daily-metric increment bucket.
const (
	MessageWriterFlushOpUpsert        = "upsert"
	MessageWriterFlushOpDelete        = "delete"
	MessageWriterFlushOpVersions      = "versions"
	MessageWriterFlushOpMetricBuckets = "metric_buckets"
)

// MessageWriterSnapshotProvider is the optional capability a future
// writer-aware health endpoint looks for. The in-memory implementation
// satisfies it; NopMessageWriterMetrics does not (it has nothing to
// snapshot). A handler would use a type assertion so the metrics
// dependency stays write-only on the hot path.
type MessageWriterSnapshotProvider interface {
	Snapshot() MessageWriterMetricsSnapshot
}

// MessageWriterMetricsSnapshot is the JSON-friendly view of the writer's
// counters. The outer struct splits enqueue-side telemetry from flush-side
// telemetry so a future health endpoint can render two compact tables
// instead of one wide one.
type MessageWriterMetricsSnapshot struct {
	Enqueue MessageWriterEnqueueSnapshot `json:"enqueue"`
	Flush   MessageWriterFlushSnapshot   `json:"flush"`
}

// MessageWriterEnqueueSnapshot tracks the producer-side counters and the
// per-cause rejection breakdown.
type MessageWriterEnqueueSnapshot struct {
	UpsertsTotal    int64            `json:"upserts_total"`
	DeletesTotal    int64            `json:"deletes_total"`
	VersionsTotal   int64            `json:"versions_total"`
	MetricsTotal    int64            `json:"metrics_total"`
	FailuresByCause map[string]int64 `json:"failures_by_cause,omitempty"`
	MaxQueueDepth   int64            `json:"max_queue_depth"`
}

// MessageWriterFlushSnapshot tracks the flush-cycle counters and the
// per-op write breakdown. FlushedByOp totals successful writes across
// batch and sequential paths; FallbackByOp counts rows that the batch
// store call rejected and forced through the sequential fallback.
type MessageWriterFlushSnapshot struct {
	CyclesTotal   int64                         `json:"cycles_total"`
	RequestsTotal int64                         `json:"requests_total"`
	LastBatchSize int64                         `json:"last_batch_size"`
	Duration      observability.SummarySnapshot `json:"duration_seconds"`
	FlushedByOp   map[string]int64              `json:"flushed_by_op,omitempty"`
	FallbackByOp  map[string]int64              `json:"fallback_by_op,omitempty"`
}

// NopMessageWriterMetrics is the default implementation when the writer
// is constructed without explicit metrics wiring. Every method is a
// no-op; this lets the writer call w.metrics.RecordX(...) without nil
// checks.
type NopMessageWriterMetrics struct{}

// RecordEnqueueUpsert records enqueue upsert.
func (NopMessageWriterMetrics) RecordEnqueueUpsert(bool, bool) {}

// RecordEnqueueDelete records enqueue delete.
func (NopMessageWriterMetrics) RecordEnqueueDelete(bool) {}

// RecordEnqueueVersion records enqueue version.
func (NopMessageWriterMetrics) RecordEnqueueVersion() {}

// RecordEnqueueFailure records enqueue failure.
func (NopMessageWriterMetrics) RecordEnqueueFailure(string) {}

// ObserveQueueDepth observes queue depth.
func (NopMessageWriterMetrics) ObserveQueueDepth(int) {}

// RecordFlush records flush.
func (NopMessageWriterMetrics) RecordFlush(int, time.Duration) {}

// RecordFlushSuccess records flush success.
func (NopMessageWriterMetrics) RecordFlushSuccess(string, int) {}

// RecordFlushFallback records flush fallback.
func (NopMessageWriterMetrics) RecordFlushFallback(string, int) {}

// InMemoryMessageWriterMetrics is the lightweight implementation backing
// the writer-aware health endpoint. Atomic int64 counters; the labeled
// maps (failure-by-cause, flushed/fallback by op) sit behind a RWMutex
// because their cardinality is bounded by the source code (two failure
// causes, four op tokens).
//
// Goroutine safety: every method is safe to call concurrently. Snapshot()
// briefly takes a read lock to copy the maps; the atomic loads happen
// without locks.
type InMemoryMessageWriterMetrics struct {
	mu sync.Mutex

	enqueueUpserts  atomic.Int64
	enqueueDeletes  atomic.Int64
	enqueueVersions atomic.Int64
	enqueueMetrics  atomic.Int64
	enqueueFailures map[string]*atomic.Int64
	maxQueueDepth   atomic.Int64

	flushCycles   atomic.Int64
	flushRequests atomic.Int64
	lastBatchSize atomic.Int64
	flushDuration observability.Summary

	flushedByOp  map[string]*atomic.Int64
	fallbackByOp map[string]*atomic.Int64
}

// NewInMemoryMessageWriterMetrics constructs the production metrics
// implementation. Use this in pkg/app wiring and pass into
// MessageEventService.SetWriterMetrics so the future writer-aware health
// endpoint has counters to expose.
func NewInMemoryMessageWriterMetrics() *InMemoryMessageWriterMetrics {
	return &InMemoryMessageWriterMetrics{
		enqueueFailures: make(map[string]*atomic.Int64),
		flushedByOp:     make(map[string]*atomic.Int64),
		fallbackByOp:    make(map[string]*atomic.Int64),
	}
}

// RecordEnqueueUpsert records enqueue upsert.
func (m *InMemoryMessageWriterMetrics) RecordEnqueueUpsert(version bool, metric bool) {
	m.enqueueUpserts.Add(1)
	if version {
		m.enqueueVersions.Add(1)
	}
	if metric {
		m.enqueueMetrics.Add(1)
	}
}

// RecordEnqueueDelete records enqueue delete.
func (m *InMemoryMessageWriterMetrics) RecordEnqueueDelete(version bool) {
	m.enqueueDeletes.Add(1)
	if version {
		m.enqueueVersions.Add(1)
	}
}

// RecordEnqueueVersion records enqueue version.
func (m *InMemoryMessageWriterMetrics) RecordEnqueueVersion() {
	m.enqueueVersions.Add(1)
}

// RecordEnqueueFailure records enqueue failure.
func (m *InMemoryMessageWriterMetrics) RecordEnqueueFailure(cause string) {
	observability.GetOrCreateLabeledCounter(&m.mu, &m.enqueueFailures, cause).Add(1)
}

// ObserveQueueDepth observes queue depth.
func (m *InMemoryMessageWriterMetrics) ObserveQueueDepth(depth int) {
	if depth <= 0 {
		return
	}
	candidate := int64(depth)
	for {
		cur := m.maxQueueDepth.Load()
		if candidate <= cur {
			return
		}
		if m.maxQueueDepth.CompareAndSwap(cur, candidate) {
			return
		}
	}
}

// RecordFlush records flush.
func (m *InMemoryMessageWriterMetrics) RecordFlush(batchSize int, duration time.Duration) {
	m.flushCycles.Add(1)
	if batchSize > 0 {
		m.flushRequests.Add(int64(batchSize))
		m.lastBatchSize.Store(int64(batchSize))
	}
	m.flushDuration.Observe(duration)
}

// RecordFlushSuccess records flush success.
func (m *InMemoryMessageWriterMetrics) RecordFlushSuccess(op string, count int) {
	if count <= 0 {
		return
	}
	observability.GetOrCreateLabeledCounter(&m.mu, &m.flushedByOp, op).Add(int64(count))
}

// RecordFlushFallback records flush fallback.
func (m *InMemoryMessageWriterMetrics) RecordFlushFallback(op string, count int) {
	if count <= 0 {
		return
	}
	observability.GetOrCreateLabeledCounter(&m.mu, &m.fallbackByOp, op).Add(int64(count))
}

// Snapshot returns a JSON-friendly view of the current counter state. The
// returned MessageWriterMetricsSnapshot is a copy; callers can mutate it
// without affecting the live counters.
func (m *InMemoryMessageWriterMetrics) Snapshot() MessageWriterMetricsSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	return MessageWriterMetricsSnapshot{
		Enqueue: MessageWriterEnqueueSnapshot{
			UpsertsTotal:    m.enqueueUpserts.Load(),
			DeletesTotal:    m.enqueueDeletes.Load(),
			VersionsTotal:   m.enqueueVersions.Load(),
			MetricsTotal:    m.enqueueMetrics.Load(),
			FailuresByCause: copyMessageWriterCounterMap(m.enqueueFailures),
			MaxQueueDepth:   m.maxQueueDepth.Load(),
		},
		Flush: MessageWriterFlushSnapshot{
			CyclesTotal:   m.flushCycles.Load(),
			RequestsTotal: m.flushRequests.Load(),
			LastBatchSize: m.lastBatchSize.Load(),
			Duration:      m.flushDuration.Snapshot(),
			FlushedByOp:   copyMessageWriterCounterMap(m.flushedByOp),
			FallbackByOp:  copyMessageWriterCounterMap(m.fallbackByOp),
		},
	}
}

func copyMessageWriterCounterMap(src map[string]*atomic.Int64) map[string]int64 {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]int64, len(src))
	for k, v := range src {
		out[k] = v.Load()
	}
	return out
}

```

// === FILE: pkg/messages/models.go ===
```go
package messages

import "time"

type Record struct {
	GuildID        string
	MessageID      string
	ChannelID      string
	AuthorID       string
	AuthorUsername string
	AuthorAvatar   string
	Content        string
	CachedAt       time.Time
	ExpiresAt      time.Time
	HasExpiry      bool
}

type DeleteKey struct {
	GuildID   string
	MessageID string
}

type Version struct {
	GuildID     string
	MessageID   string
	ChannelID   string
	AuthorID    string
	Version     int
	EventType   string
	Content     string
	Attachments int
	Embeds      int
	Stickers    int
	CreatedAt   time.Time
}

type DailyCountDelta struct {
	GuildID     string
	ChannelID   string
	UserID      string
	Day         time.Time
	MessageType string
	Count       int
}

```

// === FILE: pkg/messages/observability.go ===
```go
package messages

import (
	"sync/atomic"
)

// Metrics is the observability seam the messages service writes through.
type Metrics interface {
	RecordMessageSent()
}

// SnapshotProvider is the optional capability the /v1/health/messages handler looks for.
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is the JSON payload /v1/health/messages returns.
type MetricsSnapshot struct {
	MessagesSentTotal int64 `json:"messages_sent_total"`
}

// NopMetrics is the default implementation when the service is constructed without explicit metrics wiring.
type NopMetrics struct{}

func (NopMetrics) RecordMessageSent() {}

// InMemoryMetrics is the lightweight implementation backing /v1/health/messages.
type InMemoryMetrics struct {
	messagesSent atomic.Int64
}

// NewInMemoryMetrics constructs the production metrics implementation.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{}
}

func (m *InMemoryMetrics) RecordMessageSent() { m.messagesSent.Add(1) }

// Snapshot returns a JSON-friendly view of the current counter state.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		MessagesSentTotal: m.messagesSent.Load(),
	}
}

```

// === FILE: pkg/messages/repository.go ===
```go
package messages

import (
	"context"
)

type Repository interface {
	UpsertMessage(m Record) error
	UpsertMessagesContext(ctx context.Context, records []Record) error
	GetMessage(ctx context.Context, guildID, messageID string) (*Record, error)
	DeleteMessagesContext(ctx context.Context, keys []DeleteKey) error
	InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []Version) error
	CleanupExpiredMessages() error
	IncrementDailyMessageCountsContext(ctx context.Context, deltas []DailyCountDelta) error
	DeleteMessage(ctx context.Context, guildID, messageID string) error
	InsertMessageVersion(ctx context.Context, v Version) error
	IncrementDailyMessageCount(ctx context.Context, guildID string) error
}

```

// === FILE: pkg/messages/sink.go ===
```go
package messages

import (
	"context"
)

// MessageSink receives validated message domain events.
// This interface allows the domain module to be decoupled from logging/notification implementations.
type MessageSink interface {
	OnMessageDelete(ctx context.Context, intent MessageDeleteIntent, cachedMessage *CachedMessageData)
	OnMessageUpdate(ctx context.Context, intent MessageUpdateIntent, cachedMessage *CachedMessageData)
	OnMessageDeleteBulk(ctx context.Context, intent MessageDeleteBulkIntent)
}

```

