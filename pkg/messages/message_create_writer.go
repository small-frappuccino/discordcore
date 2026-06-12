package messages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
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
	record   storage.MessageRecord
	version  *storage.MessageVersion
	metric   storage.DailyMessageCountDelta
}

type pendingMessageState struct {
	token   uint64
	deleted bool
	record  storage.MessageRecord
}

type pendingMessageToken struct {
	key   string
	token uint64
}

type messageCreateWriter struct {
	store         *storage.Store
	queue         chan messageWriteRequest
	stopCh        chan struct{}
	done          chan struct{}
	flushInterval time.Duration
	maxBatch      int
	metrics       MessageWriterMetrics

	state atomic.Uint32

	mu        sync.RWMutex
	nextToken uint64
	pending   map[string]pendingMessageState
	stopOnce  sync.Once
	logger    *slog.Logger
}

func newMessageCreateWriter(store *storage.Store, metrics MessageWriterMetrics, logger *slog.Logger) *messageCreateWriter {
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
func (w *messageCreateWriter) Enqueue(record storage.MessageRecord, version *storage.MessageVersion, metric storage.DailyMessageCountDelta) error {
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
func (w *messageCreateWriter) EnqueueDelete(guildID, messageID string, version *storage.MessageVersion) error {
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
func (w *messageCreateWriter) EnqueueVersion(version storage.MessageVersion) error {
	if w == nil {
		return fmt.Errorf("message create writer is nil")
	}
	req := messageWriteRequest{
		key:     messageCreatePendingKey(version.GuildID, version.MessageID),
		version: cloneMessageVersion(&version),
	}
	return w.enqueueRequest(req)
}

func cloneMessageVersion(version *storage.MessageVersion) *storage.MessageVersion {
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
	w.mu.RLock()
	pending, ok := w.pending[key]
	w.mu.RUnlock()
	if !ok || pending.deleted {
		return nil
	}

	record := pending.record
	return &CachedMessage{
		ID:      record.MessageID,
		Content: record.Content,
		Author: &discordgo.User{
			ID:       record.AuthorID,
			Username: record.AuthorUsername,
			Avatar:   record.AuthorAvatar,
		},
		ChannelID: record.ChannelID,
		GuildID:   record.GuildID,
		Timestamp: record.CachedAt,
	}
}

func (w *messageCreateWriter) run() {
	defer func() {
		w.state.Store(uint32(writerStateClosed))
		close(w.done)
	}()

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	batch := make([]messageWriteRequest, 0, w.maxBatch)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		w.flushBatch(batch)
		batch = batch[:0]
	}

	for {
		select {
		case req := <-w.queue:
			batch = append(batch, req)
			if len(batch) >= w.maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
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

	upserts := make([]storage.MessageRecord, 0, len(batch))
	upsertTokens := make([]pendingMessageToken, 0, len(batch))
	deletes := make([]storage.MessageDeleteKey, 0, len(batch))
	deleteTokens := make([]pendingMessageToken, 0, len(batch))
	versions := make([]storage.MessageVersion, 0, len(batch))
	deltasByKey := make(map[string]storage.DailyMessageCountDelta, len(batch))

	for _, req := range batch {
		switch req.recordOp {
		case messageWriteRecordOpUpsert:
			if w.pendingStateMatches(req.key, req.token, false) {
				upserts = append(upserts, req.record)
				upsertTokens = append(upsertTokens, pendingMessageToken{key: req.key, token: req.token})
			}
		case messageWriteRecordOpDelete:
			if w.pendingStateMatches(req.key, req.token, true) {
				deletes = append(deletes, storage.MessageDeleteKey{
					GuildID:   req.record.GuildID,
					MessageID: req.record.MessageID,
				})
				if req.record.GuildID == "" || req.record.MessageID == "" {
					parts := strings.SplitN(req.key, ":", 2)
					if len(parts) == 2 {
						deletes[len(deletes)-1] = storage.MessageDeleteKey{GuildID: parts[0], MessageID: parts[1]}
					}
				}
				deleteTokens = append(deleteTokens, pendingMessageToken{key: req.key, token: req.token})
			}
		}
		if req.version != nil {
			versions = append(versions, *req.version)
		}
		if req.metric.Count != 0 {
			metricKey := strings.Join([]string{req.metric.GuildID, req.metric.ChannelID, req.metric.UserID, req.metric.Day}, ":")
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
		deltas := make([]storage.DailyMessageCountDelta, 0, len(deltasByKey))
		for _, delta := range deltasByKey {
			deltas = append(deltas, delta)
		}
		if err := w.store.IncrementDailyMessageCountsContext(context.Background(), deltas); err != nil {
			w.metrics.RecordFlushFallback(MessageWriterFlushOpMetricBuckets, len(deltas))
			w.logger.Warn("MessageCreate writer: batch daily metric flush failed; falling back to sequential increments", "operation", "message_create_writer.flush_metrics", "buckets", len(deltas), "error", err)
			for _, delta := range deltas {
				if err := w.store.IncrementDailyMessageCountsContext(context.Background(), []storage.DailyMessageCountDelta{delta}); err != nil {
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

func (w *messageCreateWriter) flushMessagesSequentially(records []storage.MessageRecord, tokens []pendingMessageToken) {
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

func (w *messageCreateWriter) flushDeletesSequentially(keys []storage.MessageDeleteKey, tokens []pendingMessageToken) {
	for i, key := range keys {
		if err := w.store.DeleteMessage(key.GuildID, key.MessageID); err != nil {
			w.logger.Warn("MessageCreate writer: sequential message delete failed", "operation", "message_create_writer.flush_deletes_fallback", "guildID", key.GuildID, "messageID", key.MessageID, "error", err)
			continue
		}
		w.metrics.RecordFlushSuccess(MessageWriterFlushOpDelete, 1)
		if i < len(tokens) {
			w.clearPendingToken(tokens[i].key, tokens[i].token)
		}
	}
}

func (w *messageCreateWriter) flushVersionsSequentially(versions []storage.MessageVersion, operation string) {
	for _, version := range versions {
		if err := w.store.InsertMessageVersion(version); err != nil {
			w.logger.Warn("MessageCreate writer: sequential history insert failed", "operation", operation, "guildID", version.GuildID, "channelID", version.ChannelID, "messageID", version.MessageID, "userID", version.AuthorID, "eventType", version.EventType, "error", err)
			continue
		}
		w.metrics.RecordFlushSuccess(MessageWriterFlushOpVersions, 1)
	}
}

func (w *messageCreateWriter) storePendingRecord(key string, record storage.MessageRecord) uint64 {
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
	w.mu.RLock()
	defer w.mu.RUnlock()
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
