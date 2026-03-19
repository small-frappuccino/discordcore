package logging

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const (
	messageCreateWriterQueueSize     = 2048
	messageCreateWriterFlushInterval = 250 * time.Millisecond
	messageCreateWriterMaxBatch      = 128
)

var errMessageCreateWriterStopped = errors.New("message create writer is stopped")

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

type messageWriteStats struct {
	enqueuedUpserts      uint64
	enqueuedDeletes      uint64
	enqueuedVersions     uint64
	enqueuedMetrics      uint64
	queueFullErrors      uint64
	stoppedErrors        uint64
	flushCount           uint64
	flushedRequests      uint64
	flushedUpserts       uint64
	flushedDeletes       uint64
	flushedVersions      uint64
	flushedMetricBuckets uint64
	fallbackUpserts      uint64
	fallbackDeletes      uint64
	fallbackVersions     uint64
	fallbackMetrics      uint64
	maxQueueDepth        uint64
	lastBatchSize        uint64
	lastFlushDurationNs  int64
}

type messageCreateWriter struct {
	store         *storage.Store
	queue         chan messageWriteRequest
	done          chan struct{}
	flushInterval time.Duration
	maxBatch      int

	mu        sync.RWMutex
	nextToken uint64
	pending   map[string]pendingMessageState
	stopOnce  sync.Once
	stats     messageWriteStats
}

func newMessageCreateWriter(store *storage.Store) *messageCreateWriter {
	return &messageCreateWriter{
		store:         store,
		queue:         make(chan messageWriteRequest, messageCreateWriterQueueSize),
		done:          make(chan struct{}),
		flushInterval: messageCreateWriterFlushInterval,
		maxBatch:      messageCreateWriterMaxBatch,
		pending:       make(map[string]pendingMessageState),
	}
}

func (w *messageCreateWriter) Start() {
	if w == nil {
		return
	}
	go w.run()
}

func (w *messageCreateWriter) Stop(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.stopOnce.Do(func() {
		close(w.queue)
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
		return err
	}
	return nil
}

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
		return err
	}
	return nil
}

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
		atomic.AddUint64(&w.stats.stoppedErrors, 1)
		return err
	}
	if !sent {
		atomic.AddUint64(&w.stats.queueFullErrors, 1)
		return fmt.Errorf("message create writer queue is full")
	}
	w.recordEnqueue(req)
	return nil
}

func (w *messageCreateWriter) trySendRequest(req messageWriteRequest) (sent bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errMessageCreateWriterStopped
		}
	}()
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
		atomic.AddUint64(&w.stats.enqueuedUpserts, 1)
	case messageWriteRecordOpDelete:
		atomic.AddUint64(&w.stats.enqueuedDeletes, 1)
	}
	if req.version != nil {
		atomic.AddUint64(&w.stats.enqueuedVersions, 1)
	}
	if req.metric.Count != 0 {
		atomic.AddUint64(&w.stats.enqueuedMetrics, 1)
	}
	w.observeQueueDepth(uint64(len(w.queue)))
}

func (w *messageCreateWriter) observeQueueDepth(depth uint64) {
	for {
		current := atomic.LoadUint64(&w.stats.maxQueueDepth)
		if depth <= current {
			return
		}
		if atomic.CompareAndSwapUint64(&w.stats.maxQueueDepth, current, depth) {
			return
		}
	}
}

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

func (w *messageCreateWriter) Stats() map[string]any {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	pendingCount := len(w.pending)
	w.mu.RUnlock()
	return map[string]any{
		"queueDepth":           len(w.queue),
		"pendingCount":         pendingCount,
		"enqueuedUpserts":      atomic.LoadUint64(&w.stats.enqueuedUpserts),
		"enqueuedDeletes":      atomic.LoadUint64(&w.stats.enqueuedDeletes),
		"enqueuedVersions":     atomic.LoadUint64(&w.stats.enqueuedVersions),
		"enqueuedMetrics":      atomic.LoadUint64(&w.stats.enqueuedMetrics),
		"queueFullErrors":      atomic.LoadUint64(&w.stats.queueFullErrors),
		"stoppedErrors":        atomic.LoadUint64(&w.stats.stoppedErrors),
		"flushCount":           atomic.LoadUint64(&w.stats.flushCount),
		"flushedRequests":      atomic.LoadUint64(&w.stats.flushedRequests),
		"flushedUpserts":       atomic.LoadUint64(&w.stats.flushedUpserts),
		"flushedDeletes":       atomic.LoadUint64(&w.stats.flushedDeletes),
		"flushedVersions":      atomic.LoadUint64(&w.stats.flushedVersions),
		"flushedMetricBuckets": atomic.LoadUint64(&w.stats.flushedMetricBuckets),
		"fallbackUpserts":      atomic.LoadUint64(&w.stats.fallbackUpserts),
		"fallbackDeletes":      atomic.LoadUint64(&w.stats.fallbackDeletes),
		"fallbackVersions":     atomic.LoadUint64(&w.stats.fallbackVersions),
		"fallbackMetrics":      atomic.LoadUint64(&w.stats.fallbackMetrics),
		"maxQueueDepth":        atomic.LoadUint64(&w.stats.maxQueueDepth),
		"lastBatchSize":        atomic.LoadUint64(&w.stats.lastBatchSize),
		"lastFlushDurationMs":  atomic.LoadInt64(&w.stats.lastFlushDurationNs) / int64(time.Millisecond),
	}
}

func (w *messageCreateWriter) run() {
	defer close(w.done)

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
		case req, ok := <-w.queue:
			if !ok {
				flush()
				return
			}
			batch = append(batch, req)
			if len(batch) >= w.maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (w *messageCreateWriter) flushBatch(batch []messageWriteRequest) {
	if w == nil || w.store == nil || len(batch) == 0 {
		return
	}

	start := time.Now()
	defer func() {
		atomic.AddUint64(&w.stats.flushCount, 1)
		atomic.AddUint64(&w.stats.flushedRequests, uint64(len(batch)))
		atomic.StoreUint64(&w.stats.lastBatchSize, uint64(len(batch)))
		atomic.StoreInt64(&w.stats.lastFlushDurationNs, time.Since(start).Nanoseconds())
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
			atomic.AddUint64(&w.stats.fallbackUpserts, uint64(len(upserts)))
			slog.Warn("MessageCreate writer: batch message upsert failed; falling back to sequential writes", "operation", "message_create_writer.flush_messages", "messages", len(upserts), "error", err)
			w.flushMessagesSequentially(upserts, upsertTokens)
		} else {
			atomic.AddUint64(&w.stats.flushedUpserts, uint64(len(upserts)))
			w.clearPendingTokens(upsertTokens)
		}
	}

	if len(deletes) > 0 {
		if err := w.store.DeleteMessagesContext(context.Background(), deletes); err != nil {
			atomic.AddUint64(&w.stats.fallbackDeletes, uint64(len(deletes)))
			slog.Warn("MessageCreate writer: batch message delete failed; falling back to sequential deletes", "operation", "message_create_writer.flush_deletes", "messages", len(deletes), "error", err)
			w.flushDeletesSequentially(deletes, deleteTokens)
		} else {
			atomic.AddUint64(&w.stats.flushedDeletes, uint64(len(deletes)))
			w.clearPendingTokens(deleteTokens)
		}
	}

	if len(versions) > 0 {
		if err := w.store.InsertMessageVersionsMixedBatchContext(context.Background(), versions); err != nil {
			atomic.AddUint64(&w.stats.fallbackVersions, uint64(len(versions)))
			slog.Warn("MessageCreate writer: batch history insert failed; falling back to sequential writes", "operation", "message_create_writer.flush_versions", "versions", len(versions), "error", err)
			w.flushVersionsSequentially(versions, "message_create_writer.flush_versions_fallback")
		} else {
			atomic.AddUint64(&w.stats.flushedVersions, uint64(len(versions)))
		}
	}

	if len(deltasByKey) > 0 {
		deltas := make([]storage.DailyMessageCountDelta, 0, len(deltasByKey))
		for _, delta := range deltasByKey {
			deltas = append(deltas, delta)
		}
		if err := w.store.IncrementDailyMessageCountsContext(context.Background(), deltas); err != nil {
			atomic.AddUint64(&w.stats.fallbackMetrics, uint64(len(deltas)))
			slog.Warn("MessageCreate writer: batch daily metric flush failed; falling back to sequential increments", "operation", "message_create_writer.flush_metrics", "buckets", len(deltas), "error", err)
			for _, delta := range deltas {
				if err := w.store.IncrementDailyMessageCountsContext(context.Background(), []storage.DailyMessageCountDelta{delta}); err != nil {
					slog.Warn("MessageCreate writer: sequential daily metric increment failed", "operation", "message_create_writer.flush_metrics_fallback", "guildID", delta.GuildID, "channelID", delta.ChannelID, "userID", delta.UserID, "error", err)
				} else {
					atomic.AddUint64(&w.stats.flushedMetricBuckets, 1)
				}
			}
		} else {
			atomic.AddUint64(&w.stats.flushedMetricBuckets, uint64(len(deltas)))
		}
	}
}

func (w *messageCreateWriter) flushMessagesSequentially(records []storage.MessageRecord, tokens []pendingMessageToken) {
	for i, record := range records {
		if err := w.store.UpsertMessage(record); err != nil {
			slog.Warn("MessageCreate writer: sequential message upsert failed", "operation", "message_create_writer.flush_messages_fallback", "guildID", record.GuildID, "channelID", record.ChannelID, "messageID", record.MessageID, "userID", record.AuthorID, "error", err)
			continue
		}
		atomic.AddUint64(&w.stats.flushedUpserts, 1)
		if i < len(tokens) {
			w.clearPendingToken(tokens[i].key, tokens[i].token)
		}
	}
}

func (w *messageCreateWriter) flushDeletesSequentially(keys []storage.MessageDeleteKey, tokens []pendingMessageToken) {
	for i, key := range keys {
		if err := w.store.DeleteMessage(key.GuildID, key.MessageID); err != nil {
			slog.Warn("MessageCreate writer: sequential message delete failed", "operation", "message_create_writer.flush_deletes_fallback", "guildID", key.GuildID, "messageID", key.MessageID, "error", err)
			continue
		}
		atomic.AddUint64(&w.stats.flushedDeletes, 1)
		if i < len(tokens) {
			w.clearPendingToken(tokens[i].key, tokens[i].token)
		}
	}
}

func (w *messageCreateWriter) flushVersionsSequentially(versions []storage.MessageVersion, operation string) {
	for _, version := range versions {
		if err := w.store.InsertMessageVersion(version); err != nil {
			slog.Warn("MessageCreate writer: sequential history insert failed", "operation", operation, "guildID", version.GuildID, "channelID", version.ChannelID, "messageID", version.MessageID, "userID", version.AuthorID, "eventType", version.EventType, "error", err)
			continue
		}
		atomic.AddUint64(&w.stats.flushedVersions, 1)
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
