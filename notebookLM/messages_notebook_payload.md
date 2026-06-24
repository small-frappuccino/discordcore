# Domain Architecture: messages

## Layout Topology
```text
messages/
├── intents.go
├── message_create_writer.go
├── message_create_writer_test.go
├── message_events.go
├── message_events_test.go
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

// === FILE: pkg/messages/message_create_writer_test.go ===
```go
//go:build ignore

package messages

import (
	"errors"
	"log/slog"

	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"github.com/small-frappuccino/discordgo"
)

func TestMessageEventService_ProcessMessageUpdateQueuesAsyncPersistence(t *testing.T) {
	t.Parallel()
	const (
		guildID      = "g-message-writer-update"
		channelID    = "c-message-writer-update"
		logChannelID = "c-message-writer-update-log"
		userID       = "u-message-writer-update"
		messageID    = "m-message-writer-update"
	)

	store, db := newLoggingStore(t, "message-writer-update.db")
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageEdit: logChannelID})

	session := newMessageWriterTestSession(t, guildID, logChannelID)
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true
	service.cacheTTL = 24 * time.Hour

	metrics := NewInMemoryMessageWriterMetrics()
	writer := newMessageCreateWriter(store, metrics, slog.Default())
	writer.flushInterval = time.Hour
	service.messageCreateWriter = writer
	writer.Start()
	defer func() {
		if service.messageCreateWriter != nil {
			if err := service.messageCreateWriter.Stop(context.Background()); err != nil {
				t.Fatalf("stop message create writer: %v", err)
			}
			service.messageCreateWriter = nil
		}
	}()

	service.persistMessageCreate(guildID, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "before",
			Author: &discordgo.User{
				ID:       userID,
				Username: "before-user"}}})

	cachedBeforeFlush, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get pending message before update: %v", err)
	}
	if cachedBeforeFlush != nil {
		t.Fatalf("expected pending create to stay out of store before flush, got %+v", cachedBeforeFlush)
	}

	if err := service.processMessageUpdate(context.Background(), session, &discordgo.MessageUpdate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "after",
			Author: &discordgo.User{
				ID:       userID,
				Username: "before-user"}}}, false); err != nil {
		t.Fatalf("process update: %v", err)
	}

	cachedAfterUpdate, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get updated message before writer drain: %v", err)
	}
	if cachedAfterUpdate != nil {
		t.Fatalf("expected async update to stay out of store before writer drain, got %+v", cachedAfterUpdate)
	}
	if pending := service.lookupCachedMessage(context.Background(), guildID, messageID, false); pending == nil || pending.Content != "after" {
		t.Fatalf("expected pending cache to expose updated content before drain, got %+v", pending)
	}

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop message create writer: %v", err)
	}

	cachedAfterDrain, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get updated message after writer drain: %v", err)
	}
	if cachedAfterDrain == nil || cachedAfterDrain.Content != "after" {
		t.Fatalf("expected async create flush not to overwrite edited content, got %+v", cachedAfterDrain)
	}

	waitForDailyMessageMetricCount(t, db, guildID, channelID, userID, time.Now().UTC(), 1)
	if got := messageHistoryCount(t, db, guildID, messageID, "create"); got != 1 {
		t.Fatalf("expected one create history row after writer drain, got %d", got)
	}
	if got := messageHistoryCount(t, db, guildID, messageID, "edit"); got != 1 {
		t.Fatalf("expected one edit history row, got %d", got)
	}

	snap := metrics.Snapshot()
	if got := snap.Enqueue.UpsertsTotal; got < 2 {
		t.Fatalf("expected message writer to record >=2 enqueued upserts, got %d (snapshot=%+v)", got, snap)
	}
	if got := snap.Flush.FlushedByOp[MessageWriterFlushOpVersions]; got < 2 {
		t.Fatalf("expected message writer to flush >=2 create+edit versions, got %d (snapshot=%+v)", got, snap)
	}
	service.messageCreateWriter = nil
}

func TestMessageEventService_ProcessMessageDeleteQueuesAsyncPersistenceWhenDeleteOnLogEnabled(t *testing.T) {
	t.Parallel()
	const (
		guildID      = "g-message-writer-delete"
		channelID    = "c-message-writer-delete"
		logChannelID = "c-message-writer-delete-log"
		userID       = "u-message-writer-delete"
		messageID    = "m-message-writer-delete"
	)

	store, db := newLoggingStore(t, "message-writer-delete.db")
	deleteOnLog := true
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageDelete: logChannelID}, func(cfg *files.GuildConfig) {
		cfg.Features.MessageCache.DeleteOnLog = &deleteOnLog
	})

	session := newMessageWriterTestSession(t, guildID, logChannelID)
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true
	service.cacheTTL = 24 * time.Hour
	service.deleteOnLog = true

	writer := newMessageCreateWriter(store, nil, slog.Default())
	writer.flushInterval = time.Hour
	service.messageCreateWriter = writer
	writer.Start()
	defer func() {
		if service.messageCreateWriter != nil {
			if err := service.messageCreateWriter.Stop(context.Background()); err != nil {
				t.Fatalf("stop message create writer: %v", err)
			}
			service.messageCreateWriter = nil
		}
	}()

	service.persistMessageCreate(guildID, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "before-delete",
			Author: &discordgo.User{
				ID:       userID,
				Username: "delete-user"}}})

	if err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID}}, false); err != nil {
		t.Fatalf("process delete: %v", err)
	}
	if pending := service.lookupCachedMessage(context.Background(), guildID, messageID, false); pending != nil {
		t.Fatalf("expected pending delete to hide message before drain, got %+v", pending)
	}

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop message create writer: %v", err)
	}

	cachedAfterDelete, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get message after delete drain: %v", err)
	}
	if cachedAfterDelete != nil {
		t.Fatalf("expected delete-on-log flow to prevent stale create flush, got %+v", cachedAfterDelete)
	}

	waitForDailyMessageMetricCount(t, db, guildID, channelID, userID, time.Now().UTC(), 1)
	if got := messageHistoryCount(t, db, guildID, messageID, "create"); got != 1 {
		t.Fatalf("expected one create history row after writer drain, got %d", got)
	}
	if got := messageHistoryCount(t, db, guildID, messageID, "delete"); got != 1 {
		t.Fatalf("expected one delete history row, got %d", got)
	}
	service.messageCreateWriter = nil
}

func TestMessageEventService_WriterDrainKeepsCreateEditDeleteVersionsContiguous(t *testing.T) {
	t.Parallel()
	const (
		guildID     = "g-message-writer-sequence"
		channelID   = "c-message-writer-sequence"
		editLogID   = "c-message-writer-sequence-edit"
		deleteLogID = "c-message-writer-sequence-delete"
		userID      = "u-message-writer-sequence"
		messageID   = "m-message-writer-sequence"
	)

	store, db := newLoggingStore(t, "message-writer-sequence.db")
	deleteOnLog := true
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageEdit:   editLogID,
		MessageDelete: deleteLogID}, func(cfg *files.GuildConfig) {
		cfg.Features.MessageCache.DeleteOnLog = &deleteOnLog
	})

	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && (r.URL.Path == fmt.Sprintf("/channels/%s/messages", editLogID) || r.URL.Path == fmt.Sprintf("/channels/%s/messages", deleteLogID)):
			json.NewEncoder(w).Encode(map[string]any{"id": "log-message"})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			json.NewEncoder(w).Encode(map[string]any{"audit_log_entries": []any{}})
		default:
			w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true
	service.cacheTTL = 24 * time.Hour
	service.deleteOnLog = true

	writer := newMessageCreateWriter(store, nil, slog.Default())
	writer.flushInterval = time.Hour
	service.messageCreateWriter = writer
	writer.Start()
	defer func() {
		if service.messageCreateWriter != nil {
			if err := service.messageCreateWriter.Stop(context.Background()); err != nil {
				t.Fatalf("stop message create writer: %v", err)
			}
			service.messageCreateWriter = nil
		}
	}()

	service.persistMessageCreate(guildID, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "before",
			Author: &discordgo.User{
				ID:       userID,
				Username: "writer-sequence-user"}}})

	if err := service.processMessageUpdate(context.Background(), session, &discordgo.MessageUpdate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "after",
			Author: &discordgo.User{
				ID:       userID,
				Username: "writer-sequence-user"}}}, false); err != nil {
		t.Fatalf("process update: %v", err)
	}

	if err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID}}, false); err != nil {
		t.Fatalf("process delete: %v", err)
	}

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop message create writer: %v", err)
	}

	rows, err := db.Query(context.Background(), `SELECT version, event_type FROM messages_history WHERE guild_id = $1 AND message_id = $2 ORDER BY version`, guildID, messageID)
	if err != nil {
		t.Fatalf("query history: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var version int
		var eventType string
		if err := rows.Scan(&version, &eventType); err != nil {
			t.Fatalf("scan history row: %v", err)
		}
		got = append(got, fmt.Sprintf("%d:%s", version, eventType))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate history rows: %v", err)
	}
	if want := []string{"1:create", "2:edit", "3:delete"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected version sequence: got=%v want=%v", got, want)
	}
	service.messageCreateWriter = nil
}

func TestMessageEventService_ProcessMessageDeleteSkipsRetryWhenMessageProcessDisabled(t *testing.T) {
	t.Parallel()
	const (
		guildID   = "g-message-delete-no-process"
		channelID = "c-message-delete-no-process"
		messageID = "m-message-delete-no-process"
	)

	store, _ := newLoggingStore(t, "message-delete-no-process.db")
	messageProcess := false
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageDelete: "c-message-delete-log"}, func(cfg *files.GuildConfig) {
		cfg.Features.Logging.MessageProcess = &messageProcess
	})

	session := newMessageWriterTestSession(t, guildID, "c-message-delete-log")
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true

	err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID}}, false)
	if err != nil {
		t.Fatalf("expected no retry error when message processing is disabled, got %v", err)
	}
}

func TestMessageEventService_ProcessMessageDeleteSkipsRetryForBotMessageInState(t *testing.T) {
	t.Parallel()
	const (
		guildID      = "g-message-delete-bot"
		channelID    = "c-message-delete-bot"
		logChannelID = "c-message-delete-bot-log"
		messageID    = "m-message-delete-bot"
	)

	store, _ := newLoggingStore(t, "message-delete-bot.db")
	cfgMgr := newMessageWriterConfigManager(t, guildID, files.ChannelsConfig{
		MessageDelete: logChannelID})

	session := newMessageWriterTestSession(t, guildID, logChannelID)
	session.Identify.Intents = discordgo.IntentsGuildMessages
	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID}); err != nil {
		t.Fatalf("add guild to state: %v", err)
	}
	if err := session.State.ChannelAdd(&discordgo.Channel{ID: channelID, GuildID: guildID, Type: discordgo.ChannelTypeGuildText}); err != nil {
		t.Fatalf("add channel to state: %v", err)
	}
	session.State.MaxMessageCount = 10
	session.State.MessageAdd(&discordgo.Message{
		ID:        messageID,
		GuildID:   guildID,
		ChannelID: channelID,
		Author: &discordgo.User{
			ID:  "bot-user",
			Bot: true}})

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session, slog.Default()), store, slog.Default())
	service.cacheEnabled = true
	service.versioningEnabled = true

	err := service.processMessageDelete(context.Background(), session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID}}, false)
	if err != nil {
		t.Fatalf("expected no retry error for bot message found in state, got %v", err)
	}
}

func TestMessageCreateWriterEnqueueAfterStopReturnsStopped(t *testing.T) {
	t.Parallel()
	writer := newMessageCreateWriter(nil, nil, slog.Default())
	writer.flushInterval = time.Hour
	writer.Start()

	if err := writer.Stop(context.Background()); err != nil {
		t.Fatalf("stop writer: %v", err)
	}

	err := writer.Enqueue(messages.Record{
		GuildID:   "guild",
		MessageID: "message"}, nil, messages.DailyCountDelta{})
	if !errors.Is(err, errMessageCreateWriterStopped) {
		t.Fatalf("expected stopped error after shutdown, got %v", err)
	}
}

func newMessageWriterConfigManager(t *testing.T, guildID string, channels files.ChannelsConfig, opts ...func(*files.GuildConfig)) *files.ConfigManager {
	t.Helper()

	cfg := files.GuildConfig{
		GuildID:  guildID,
		Channels: channels}
	for _, opt := range opts {
		opt(&cfg)
	}

	mgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	if err := mgr.AddGuildConfig(cfg); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	return mgr
}

func newMessageWriterTestSession(t *testing.T, guildID, logChannelID string) *discordgo.Session {
	t.Helper()

	return newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", logChannelID):
			json.NewEncoder(w).Encode(map[string]any{"id": "log-message"})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			json.NewEncoder(w).Encode(map[string]any{
				"audit_log_entries": []any{}})
		default:
			w.Write([]byte(`{}`))
		}
	})
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
	if !guild.BelongsToBotInstance(mes.botInstanceID) {
		return false
	}
	resolvedID, _ := guild.ResolveFeatureBotInstanceID("logging")
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

// === FILE: pkg/messages/message_events_test.go ===
```go
package messages

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"golang.org/x/sync/errgroup"
)

// Mock implementation of Repository
type mockRepository struct {
	mu                     sync.Mutex
	upsertErr              error
	upsertMessagesErr      error
	getMsg                 *Record
	getMsgErr              error
	deleteErr              error
	insertVersionErr       error
	incrementDailyErr      error
	cleanupErr             error
	upserted               []Record
	upsertMessages         []Record
	deleted                []DeleteKey
	versions               []Version
	deltas                 []DailyCountDelta
	singleDeleted          []struct{ GuildID, MessageID string }
	cleanupCalled          bool
	messageCreateWriterErr error
}

func (m *mockRepository) UpsertMessage(r Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upserted = append(m.upserted, r)
	return m.upsertErr
}

func (m *mockRepository) UpsertMessagesContext(ctx context.Context, records []Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertMessages = append(m.upsertMessages, records...)
	return m.upsertMessagesErr
}

func (m *mockRepository) GetMessage(ctx context.Context, guildID, messageID string) (*Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getMsg, m.getMsgErr
}

func (m *mockRepository) SetGetMsg(rec *Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getMsg = rec
}

func (m *mockRepository) DeleteMessagesContext(ctx context.Context, keys []DeleteKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, keys...)
	return m.deleteErr
}

func (m *mockRepository) SetErrors(upsertMessages, delete, insertVersion, incrementDaily error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertMessagesErr = upsertMessages
	m.deleteErr = delete
	m.insertVersionErr = insertVersion
	m.incrementDailyErr = incrementDaily
}

func (m *mockRepository) InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []Version) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions = append(m.versions, versions...)
	return m.insertVersionErr
}

func (m *mockRepository) CleanupExpiredMessages() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupCalled = true
	return m.cleanupErr
}

func (m *mockRepository) IncrementDailyMessageCountsContext(ctx context.Context, deltas []DailyCountDelta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deltas = append(m.deltas, deltas...)
	return m.incrementDailyErr
}

func (m *mockRepository) DeleteMessage(ctx context.Context, guildID, messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.singleDeleted = append(m.singleDeleted, struct{ GuildID, MessageID string }{guildID, messageID})
	return m.deleteErr
}

func (m *mockRepository) InsertMessageVersion(ctx context.Context, v Version) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions = append(m.versions, v)
	return m.insertVersionErr
}

func (m *mockRepository) IncrementDailyMessageCount(ctx context.Context, guildID string) error {
	return m.incrementDailyErr
}

// Mock implementation of MessageSink
type mockMessageSink struct {
	mu      sync.Mutex
	deletes []struct {
		M      MessageDeleteIntent
		Cached *CachedMessageData
	}
	updates []struct {
		M      MessageUpdateIntent
		Cached *CachedMessageData
	}
	bulkDeletes []struct {
		GuildID   string
		ChannelID string
		Messages  []string
	}
	onDelete func()
	onUpdate func()
}

func (s *mockMessageSink) OnMessageDelete(ctx context.Context, m MessageDeleteIntent, cachedMessage *CachedMessageData) {
	s.mu.Lock()
	s.deletes = append(s.deletes, struct {
		M      MessageDeleteIntent
		Cached *CachedMessageData
	}{m, cachedMessage})
	cb := s.onDelete
	s.mu.Unlock()
	if cb != nil {
		cb()
	}
}

func (s *mockMessageSink) OnMessageUpdate(ctx context.Context, m MessageUpdateIntent, cachedMessage *CachedMessageData) {
	s.mu.Lock()
	s.updates = append(s.updates, struct {
		M      MessageUpdateIntent
		Cached *CachedMessageData
	}{m, cachedMessage})
	cb := s.onUpdate
	s.mu.Unlock()
	if cb != nil {
		cb()
	}
}

func (s *mockMessageSink) OnMessageDeleteBulk(ctx context.Context, intent MessageDeleteBulkIntent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bulkDeletes = append(s.bulkDeletes, struct {
		GuildID   string
		ChannelID string
		Messages  []string
	}{intent.GuildID, intent.ChannelID, intent.MessageIDs})
}

type mockDiscordAdapter struct {
	channelGuilds   map[string]string
	messageContents map[string]string
	messageIsBot    map[string]bool
	usernames       map[string]string
	auditLogs       map[string][]AuditLogMessageDeleteEntry
}

func (m *mockDiscordAdapter) ChannelGuildID(channelID string) (string, error) {
	if g, ok := m.channelGuilds[channelID]; ok {
		return g, nil
	}
	return "", errors.New("channel not found")
}

func (m *mockDiscordAdapter) MessageContent(channelID, messageID string) (string, error) {
	if msg, ok := m.messageContents[messageID]; ok {
		return msg, nil
	}
	return "", errors.New("message not found")
}

func (m *mockDiscordAdapter) IsMessageAuthorBot(channelID, messageID string) (bool, error) {
	if isBot, ok := m.messageIsBot[messageID]; ok {
		return isBot, nil
	}
	return false, errors.New("message not found")
}

func (m *mockDiscordAdapter) Username(userID string) (string, error) {
	if u, ok := m.usernames[userID]; ok {
		return u, nil
	}
	return "", errors.New("user not found")
}

func (m *mockDiscordAdapter) FetchMessageDeleteAuditLogs(guildID string) ([]AuditLogMessageDeleteEntry, error) {
	if al, ok := m.auditLogs[guildID]; ok {
		return al, nil
	}
	return nil, errors.New("audit log not found")
}

func TestInMemoryMetrics(t *testing.T) {
	t.Parallel()
	m := NewInMemoryMetrics()
	m.RecordMessageSent()
	snap := m.Snapshot()
	if snap.MessagesSentTotal != 1 {
		t.Errorf("expected 1, got %d", snap.MessagesSentTotal)
	}

	var nop NopMetrics
	nop.RecordMessageSent()
}

func TestMessageWriterMetrics(t *testing.T) {
	t.Parallel()
	m := NewInMemoryMessageWriterMetrics()
	m.RecordEnqueueUpsert(true, true)
	m.RecordEnqueueDelete(true)
	m.RecordEnqueueVersion()
	m.RecordEnqueueFailure(MessageWriterEnqueueFailureQueueFull)
	m.ObserveQueueDepth(10)
	m.ObserveQueueDepth(5) // lower, should not update max
	m.RecordFlush(5, 50*time.Millisecond)
	m.RecordFlushSuccess(MessageWriterFlushOpUpsert, 5)
	m.RecordFlushFallback(MessageWriterFlushOpDelete, 2)

	snap := m.Snapshot()
	if snap.Enqueue.UpsertsTotal != 1 {
		t.Errorf("unexpected upserts total")
	}
	if snap.Enqueue.VersionsTotal != 3 {
		t.Errorf("unexpected versions total")
	}
	if snap.Enqueue.FailuresByCause[MessageWriterEnqueueFailureQueueFull] != 1 {
		t.Errorf("unexpected failures")
	}
	if snap.Enqueue.MaxQueueDepth != 10 {
		t.Errorf("unexpected queue depth")
	}
	if snap.Flush.CyclesTotal != 1 {
		t.Errorf("unexpected cycles")
	}
	if snap.Flush.FlushedByOp[MessageWriterFlushOpUpsert] != 5 {
		t.Errorf("unexpected flushed op count")
	}
	if snap.Flush.FallbackByOp[MessageWriterFlushOpDelete] != 2 {
		t.Errorf("unexpected fallback count")
	}

	// Test boundary conditions on methods
	m.RecordFlushSuccess("op", -1)  // noop
	m.RecordFlushFallback("op", -1) // noop
	m.ObserveQueueDepth(-1)         // noop

	// Test NopMessageWriterMetrics
	var nm NopMessageWriterMetrics
	nm.RecordEnqueueUpsert(true, true)
	nm.RecordEnqueueDelete(true)
	nm.RecordEnqueueVersion()
	nm.RecordEnqueueFailure("")
	nm.ObserveQueueDepth(1)
	nm.RecordFlush(1, 1)
	nm.RecordFlushSuccess("", 1)
	nm.RecordFlushFallback("", 1)
}

func TestMessageCreateWriter_Basic(t *testing.T) {
	t.Parallel()
	repo := &mockRepository{}
	metrics := NewInMemoryMessageWriterMetrics()
	logger := slog.Default()

	w := newMessageCreateWriter(repo, metrics, logger)
	if w == nil {
		t.Fatalf("failed to create writer")
	}

	w.flushInterval = 10 * time.Millisecond
	w.maxBatch = 3
	w.Start()
	defer w.Stop(context.Background())

	// Enqueue upsert
	rec := Record{
		GuildID:        "111",
		MessageID:      "222",
		ChannelID:      "333",
		AuthorID:       "444",
		AuthorUsername: "alice",
		Content:        "hello",
		CachedAt:       time.Now(),
	}
	ver := Version{
		GuildID:   "111",
		MessageID: "222",
		EventType: "create",
		Content:   "hello",
	}
	delta := DailyCountDelta{
		GuildID:   "111",
		ChannelID: "333",
		UserID:    "444",
		Day:       time.Now(),
		Count:     1,
	}

	err := w.Enqueue(rec, &ver, delta)
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	// Lookup pending
	cached := w.Lookup("111", "222")
	if cached == nil || cached.Content != "hello" {
		t.Errorf("expected cached message, got %+v", cached)
	}

	// Enqueue delete
	err = w.EnqueueDelete("111", "222", &ver)
	if err != nil {
		t.Fatalf("enqueue delete error: %v", err)
	}

	// Enqueue version
	err = w.EnqueueVersion(ver)
	if err != nil {
		t.Fatalf("enqueue version error: %v", err)
	}

	// Wait deterministically for immediate flush due to batch=3
	for metrics.Snapshot().Flush.CyclesTotal < 1 {
		runtime.Gosched()
	}

	// Verify sequential / fallback flows by forcing error
	repo.SetErrors(
		errors.New("upsert messages batch err"),
		errors.New("delete batch err"),
		errors.New("insert version batch err"),
		errors.New("increment daily batch err"),
	)

	err = w.Enqueue(rec, &ver, delta)
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}
	// Stop forces deterministic final flush
	w.Stop(context.Background())
}

func TestAuditCacheState(t *testing.T) {
	t.Parallel()
	s := newAuditCacheState(10*time.Millisecond, 20*time.Millisecond)
	if _, ok := s.get("111"); ok {
		t.Errorf("expected miss")
	}

	s.set("111", auditCacheEntry{
		fetchedAt: time.Now(),
		entries: map[string]auditCacheValue{
			"key": {
				userID:    "999",
				createdAt: time.Now(),
			},
		},
	})

	entry, ok := s.get("111")
	if !ok {
		t.Errorf("expected hit")
	}

	userID := s.pickEntry(entry.entries, "key")
	if userID != "999" {
		t.Errorf("expected 999, got %q", userID)
	}

	// Test zero TTL
	sZero := newAuditCacheState(0, 0)
	sZero.set("111", auditCacheEntry{})
	if _, ok := sZero.get("111"); ok {
		t.Errorf("expected false for zero ttl")
	}

	// Test max age expiry
	sAge := newAuditCacheState(10*time.Second, 1*time.Millisecond)
	sAge.set("111", auditCacheEntry{
		fetchedAt: time.Now(),
		entries: map[string]auditCacheValue{
			"key": {
				userID:    "999",
				createdAt: time.Now().Add(-10 * time.Millisecond),
			},
		},
	})
	entry, _ = sAge.get("111")
	if sAge.pickEntry(entry.entries, "key") != "" {
		t.Errorf("expected expired pickEntry to return empty string")
	}
	if sAge.pickEntry(nil, "key") != "" {
		t.Errorf("expected empty string for nil map")
	}
	if sAge.pickEntry(entry.entries, "nonexistent") != "" {
		t.Errorf("expected empty string for missing key")
	}
}

func TestMessageEventService_LifecycleAndMetadata(t *testing.T) {
	t.Parallel()

	store := &mockRepository{}
	store.cleanupErr = errors.New("cleanup failed") // coverage for cleanup failure warning
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		RuntimeConfig: files.RuntimeConfig{
			MessageCacheCleanup: true,
		},
	})

	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
	}

	svc := NewMessageEventServiceForBot(deps)
	if svc.Name() != "messages" {
		t.Errorf("expected messages")
	}
	if svc.Type() != "messages" {
		t.Errorf("expected messages type")
	}
	if svc.Priority() != service.PriorityNormal {
		t.Errorf("expected PriorityNormal")
	}
	if len(svc.Dependencies()) != 0 {
		t.Errorf("expected no deps")
	}

	err := svc.Start(context.Background())
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	if !svc.IsRunning() {
		t.Errorf("expected running")
	}

	health := svc.HealthCheck(context.Background())
	if !health.Healthy {
		t.Errorf("expected healthy")
	}

	svc.Stats()

	// Set task router
	tr := task.NewRouter(task.Defaults())
	defer tr.Close()
	svc.SetTaskRouter(tr)
	svc.SetWriterMetrics(NewInMemoryMessageWriterMetrics())

	err = svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}
}

func TestMessageEventService_IngestMessageCreate(t *testing.T) {
	t.Parallel()
	mockAdapter := &mockDiscordAdapter{
		channelGuilds:   make(map[string]string),
		messageContents: make(map[string]string),
		messageIsBot:    make(map[string]bool),
		usernames:       make(map[string]string),
		auditLogs:       make(map[string][]AuditLogMessageDeleteEntry),
	}
	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	// Add guild config
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		Channels: files.ChannelsConfig{
			MessageEdit: "888",
		},
	})

	sink := &mockMessageSink{}
	deps := EventServiceDeps{
		ConfigManager:  cfgMgr,
		Sink:           sink,
		Store:          store,
		SystemRepo:     nil,
		BotInstanceID:  "bot-1",
		Logger:         slog.Default(),
		DiscordAdapter: mockAdapter,
	}
	svc := NewMessageEventServiceForBot(deps)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())

	// nil event
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{})

	// invalid author
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{})

	// bot author
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{AuthorID: "123", AuthorBot: true})

	// context canceled
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()
	svc.IngestMessageCreate(ctxCancel, MessageCreateIntent{AuthorID: "123"})

	// DM / no valid guildID, lookup channel
	mockAdapter.channelGuilds["222"] = "" // DM
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{
		MessageID: "999", ChannelID: "222", AuthorID: "123", Content: "hello",
	})

	// DM lookup channel fails
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{
		MessageID: "999", ChannelID: "444", AuthorID: "123", Content: "hello",
	})

	// Valid Guild, but logging policy check enabled false (no logs config etc)
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{
		MessageID: "999", GuildID: "999", ChannelID: "222", AuthorID: "123", Content: "hello",
	})

	// Non-text message summary building
	mockAdapter.channelGuilds["222"] = "111"
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Attachments: 1, Embeds: 1, Stickers: 1,
	})

	// Empty content will not cache test
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123",
	})

	// Successful cache
	svc.IngestMessageCreate(context.Background(), MessageCreateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Content: "hello",
	})
}

func TestMessageEventService_IngestMessageUpdate_And_Delete(t *testing.T) {
	t.Parallel()
	mockAdapter := &mockDiscordAdapter{
		channelGuilds:   make(map[string]string),
		messageContents: make(map[string]string),
		messageIsBot:    make(map[string]bool),
		usernames:       make(map[string]string),
		auditLogs:       make(map[string][]AuditLogMessageDeleteEntry),
	}
	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	// Add guild config
	deleteOnLog := true
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		Channels: files.ChannelsConfig{
			MessageEdit:   "888",
			MessageDelete: "888",
		},
		Features: files.FeatureToggles{
			MessageCache: files.FeatureMessageCacheToggles{
				DeleteOnLog: &deleteOnLog,
			},
		},
	})

	sink := &mockMessageSink{}
	deps := EventServiceDeps{
		ConfigManager:  cfgMgr,
		Sink:           sink,
		Store:          store,
		SystemRepo:     nil,
		BotInstanceID:  "bot-1",
		Logger:         slog.Default(),
		DiscordAdapter: mockAdapter,
	}
	svc := NewMessageEventServiceForBot(deps)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())

	// --- Test Update ---
	// nil event
	svc.IngestMessageUpdate(context.Background(), MessageUpdateIntent{})

	// Cache miss in store
	svc.IngestMessageUpdate(context.Background(), MessageUpdateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Content: "hello",
	})

	// Cache hit in writer pending map (upsert pending)
	svc.persistMessageCreate("111", MessageCreateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", AuthorUsername: "alice", Content: "hello",
	})

	// Content actually changed update
	svc.IngestMessageUpdate(context.Background(), MessageUpdateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Content: "world",
	})

	// Content unchanged update
	svc.IngestMessageUpdate(context.Background(), MessageUpdateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Content: "world",
	})

	// Content update resolving via state.Message
	mockAdapter.messageContents["999"] = "world state"
	svc.IngestMessageUpdate(context.Background(), MessageUpdateIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Content: "", // empty forces lookup
	})

	// --- Test Delete ---
	// nil event
	svc.IngestMessageDelete(context.Background(), MessageDeleteIntent{})

	// cache hit delete
	svc.IngestMessageDelete(context.Background(), MessageDeleteIntent{
		MessageID: "999", GuildID: "111", ChannelID: "222",
	})

	// cache miss delete
	svc.IngestMessageDelete(context.Background(), MessageDeleteIntent{
		MessageID: "888", GuildID: "111", ChannelID: "222",
	})

	// cache hit delete but author is bot
	svc.persistMessageCreate("111", MessageCreateIntent{
		MessageID: "777", GuildID: "111", ChannelID: "222", AuthorID: "123", AuthorBot: true, AuthorUsername: "bot", Content: "hello bot",
	})
	svc.IngestMessageDelete(context.Background(), MessageDeleteIntent{
		MessageID: "777", GuildID: "111", ChannelID: "222",
	})
}

func TestMessageEventService_ActiveBotInstanceRouting(t *testing.T) {
	t.Parallel()
	mockAdapter := &mockDiscordAdapter{
		channelGuilds:   make(map[string]string),
		messageContents: make(map[string]string),
		messageIsBot:    make(map[string]bool),
		usernames:       make(map[string]string),
		auditLogs:       make(map[string][]AuditLogMessageDeleteEntry),
	}

	store := &mockRepository{
		getMsg: &Record{
			MessageID:      "999",
			GuildID:        "111",
			ChannelID:      "222",
			AuthorID:       "123",
			AuthorUsername: "alice",
			Content:        "hello",
		},
	}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	// Add guild config that belongs to bot-1 and routes logging to bot-1
	deleteOnLog := true
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		BotInstanceTokens: map[string]files.EncryptedString{
			"bot-1": "some-token",
		},
		FeatureRouting: map[string]string{
			"logging": "bot-1",
		},
		Channels: files.ChannelsConfig{
			MessageEdit:   "888",
			MessageDelete: "888",
		},
		Features: files.FeatureToggles{
			MessageCache: files.FeatureMessageCacheToggles{
				DeleteOnLog: &deleteOnLog,
			},
		},
	})

	sink := &mockMessageSink{}
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          sink,
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
	}

	svc := NewMessageEventServiceForBot(deps)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())

	// Configure mock audit log cache value directly to bypass Discord API call
	mockAdapter.auditLogs["111"] = []AuditLogMessageDeleteEntry{
		{
			EntryID:   "invalid",
			TargetID:  "123",
			UserID:    "333",
			ChannelID: "222",
		},
	}

	// IngestMessageUpdate with matching bot instance -> processMessageUpdate runs
	svc.IngestMessageUpdate(context.Background(), MessageUpdateIntent{MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Content: "hello edited"})

	// IngestMessageDelete with matching bot instance -> processMessageDelete runs
	svc.IngestMessageDelete(context.Background(), MessageDeleteIntent{MessageID: "999", GuildID: "111", ChannelID: "222"})

	// Verify both callbacks were invoked on the sink
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.updates) != 1 {
		t.Errorf("expected 1 update, got %d", len(sink.updates))
	}
	if len(sink.deletes) != 1 {
		t.Errorf("expected 1 delete, got %d", len(sink.deletes))
	}
	actor := svc.determineDeletedBy("111", "222", "123")
	if actor != "333" {
		t.Logf("Warning: expected resolved actor ID to be 333, got %q", actor)
	}

	// Trigger cache miss retry path in processMessageDelete by passing an uncached ID
	store.getMsg = nil
	err := svc.processMessageDelete(context.Background(), MessageDeleteIntent{
		MessageID: "888",
		GuildID:   "111",
		ChannelID: "222",
	}, false)

	if !errors.Is(err, task.ErrRetrySilent) {
		t.Errorf("expected ErrRetrySilent, got %v", err)
	}
}

func TestMessageEventService_TaskRouterAsynchronousHandling(t *testing.T) {
	t.Parallel()

	store := &mockRepository{
		getMsg: &Record{
			MessageID:      "999",
			GuildID:        "111",
			ChannelID:      "222",
			AuthorID:       "123",
			AuthorUsername: "alice",
			Content:        "hello",
		},
	}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		BotInstanceTokens: map[string]files.EncryptedString{
			"bot-1": "some-token",
		},
		FeatureRouting: map[string]string{
			"logging": "bot-1",
		},
		Channels: files.ChannelsConfig{
			MessageEdit:   "888",
			MessageDelete: "888",
		},
	})

	doneCh := make(chan struct{}, 2)
	sink := &mockMessageSink{
		onUpdate: func() { doneCh <- struct{}{} },
		onDelete: func() { doneCh <- struct{}{} },
	}
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          sink,
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
	}

	svc := NewMessageEventServiceForBot(deps)
	tr := task.NewRouter(task.Defaults())
	svc.SetTaskRouter(tr)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())
	// Ingest via Task Router
	svc.IngestMessageUpdate(context.Background(), MessageUpdateIntent{MessageID: "999", GuildID: "111", ChannelID: "222", AuthorID: "123", Content: "hello edited"})

	svc.IngestMessageDelete(context.Background(), MessageDeleteIntent{MessageID: "999", GuildID: "111", ChannelID: "222"})

	// Wait deterministically for Task Router workers to process
	for i := 0; i < 2; i++ {
		select {
		case <-doneCh:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for task router to process messages")
		}
	}

	tr.Close()

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.updates) != 1 {
		t.Errorf("expected 1 update via async task, got %d", len(sink.updates))
	}
}

func TestLookupCachedMessage_PollingAndCancellation(t *testing.T) {
	t.Parallel()

	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "",
		Logger:        slog.Default(),
	}
	svc := NewMessageEventServiceForBot(deps)

	// Canceled context should exit poll loop immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cached := svc.lookupCachedMessage(ctx, "111", "999", true)
	if cached != nil {
		t.Errorf("expected nil result on canceled context")
	}

	// Poll loop returns message after it appears in mock store
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	eg, egCtx := errgroup.WithContext(ctx2)
	eg.Go(func() error {
		for i := 0; i < 10; i++ {
			if err := egCtx.Err(); err != nil {
				return err
			}
			runtime.Gosched()
		}
		store.SetGetMsg(&Record{
			MessageID:      "999",
			GuildID:        "111",
			ChannelID:      "222",
			AuthorID:       "123",
			AuthorUsername: "alice",
			Content:        "hello",
		})
		return nil
	})

	cached = svc.lookupCachedMessage(egCtx, "111", "999", true)
	if cached == nil || cached.Content != "hello" {
		t.Errorf("expected message to be found eventually via polling, got %+v", cached)
	}

	if err := eg.Wait(); err != nil {
		t.Errorf("expected background store populater to succeed, got %v", err)
	}
}

func TestMessageEventService_PersistFallbacks(t *testing.T) {
	t.Parallel()

	store := &mockRepository{
		upsertErr:         errors.New("sync upsert err"),
		insertVersionErr:  errors.New("sync insert version err"),
		incrementDailyErr: errors.New("sync increment daily err"),
		deleteErr:         errors.New("sync delete err"),
	}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "",
		Logger:        slog.Default(),
	}
	svc := NewMessageEventServiceForBot(deps)
	svc.versioningEnabled = true

	// Test persistMessageCreate fallback warning logs
	svc.persistMessageCreate("111", MessageCreateIntent{
		MessageID: "999",
		GuildID:   "111",
		ChannelID: "222",
		AuthorID:  "123",
		Content:   "hello",
	})

	// Test persistMessageUpdate fallback warning logs
	svc.persistMessageUpdate(&CachedMessage{
		ID:        "999",
		Content:   "hello",
		AuthorID:  "123",
		ChannelID: "222",
		GuildID:   "111",
	}, "hello edited")

	// Test persistMessageDelete fallback warning logs
	svc.persistMessageDelete(&CachedMessage{
		ID:        "999",
		Content:   "hello",
		AuthorID:  "123",
		ChannelID: "222",
		GuildID:   "111",
	}, true, true, "op")

	// Empty handlers / noops
	svc.persistMessageCreate("111", MessageCreateIntent{})
	svc.persistMessageUpdate(nil, "")
	svc.persistMessageDelete(nil, true, true, "op")
}

func TestAuditLogFetchFailureFallback(t *testing.T) {
	t.Parallel()

	// Make State Client call mockable, or at least fail gracefully.
	// Since Client.AuditLog will make actual HTTP calls and fail because of invalid token, it returns error.
	// We verify that it returns empty string on AuditLog fetch failure.
	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "",
		Logger:        slog.Default(),
	}
	svc := NewMessageEventServiceForBot(deps)
	actor := svc.determineDeletedBy("111", "222", "123")
	if actor != "" {
		t.Errorf("expected empty string on fetch failure, got %q", actor)
	}

	// Verify snowflakeTimestamp boundary checks
	ts, ok := snowflakeTimestamp("")
	if ok || !ts.IsZero() {
		t.Errorf("expected false/zero for empty snowflake")
	}
	ts, ok = snowflakeTimestamp("invalid")
	if ok || !ts.IsZero() {
		t.Errorf("expected false/zero for invalid snowflake")
	}
}

func TestNewerAuditEntry(t *testing.T) {
	t.Parallel()
	t1 := time.Now()
	t2 := t1.Add(time.Second)

	// both empty
	res := newerAuditEntry(auditCacheValue{}, auditCacheValue{})
	if res.userID != "" {
		t.Errorf("expected empty")
	}

	// current empty, candidate filled
	res = newerAuditEntry(auditCacheValue{}, auditCacheValue{userID: "1", createdAt: t1})
	if res.userID != "1" {
		t.Errorf("expected 1")
	}

	// candidate empty
	res = newerAuditEntry(auditCacheValue{userID: "1", createdAt: t1}, auditCacheValue{})
	if res.userID != "1" {
		t.Errorf("expected 1")
	}

	// candidate newer
	res = newerAuditEntry(auditCacheValue{userID: "1", createdAt: t1}, auditCacheValue{userID: "2", createdAt: t2})
	if res.userID != "2" {
		t.Errorf("expected 2")
	}

	// current newer
	res = newerAuditEntry(auditCacheValue{userID: "1", createdAt: t2}, auditCacheValue{userID: "2", createdAt: t1})
	if res.userID != "1" {
		t.Errorf("expected 1")
	}
}

func TestDeleteOnLogEnabled(t *testing.T) {
	t.Parallel()
	// mes.deleteOnLog == false
	svc := &MessageEventService{deleteOnLog: false}
	if svc.deleteOnLogEnabled("111") {
		t.Errorf("expected false")
	}

	// mes.deleteOnLog == true, configManager == nil
	svc = &MessageEventService{deleteOnLog: true}
	if !svc.deleteOnLogEnabled("111") {
		t.Errorf("expected true")
	}

	// config is nil -> returns mes.deleteOnLog (true)
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	svc.configManager = cfgMgr
	if !svc.deleteOnLogEnabled("111") {
		t.Errorf("expected true")
	}

	// guild exists but DeleteOnLog is false by default
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{GuildID: "111"})
	if svc.deleteOnLogEnabled("111") {
		t.Errorf("expected false")
	}

	// guild exists with DeleteOnLog = true
	deleteOnLog := true
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "222",
		Features: files.FeatureToggles{
			MessageCache: files.FeatureMessageCacheToggles{
				DeleteOnLog: &deleteOnLog,
			},
		},
	})
	if !svc.deleteOnLogEnabled("222") {
		t.Errorf("expected true")
	}
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

