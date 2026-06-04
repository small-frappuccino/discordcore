package logging

import (
	"sync"
	"sync/atomic"
	"time"
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
	CyclesTotal   int64                        `json:"cycles_total"`
	RequestsTotal int64                        `json:"requests_total"`
	LastBatchSize int64                        `json:"last_batch_size"`
	Duration      MessageWriterSummarySnapshot `json:"duration_seconds"`
	FlushedByOp   map[string]int64             `json:"flushed_by_op,omitempty"`
	FallbackByOp  map[string]int64             `json:"fallback_by_op,omitempty"`
}

// MessageWriterSummarySnapshot is the count/sum/max shape mirroring a
// Prometheus summary minus quantiles. Operators get average via sum/count
// and tail behavior via max; a Prometheus migration is one transform per
// field, not a redesign.
type MessageWriterSummarySnapshot struct {
	Count      int64   `json:"count"`
	SumSeconds float64 `json:"sum_seconds"`
	MaxSeconds float64 `json:"max_seconds"`
}

// NopMessageWriterMetrics is the default implementation when the writer
// is constructed without explicit metrics wiring. Every method is a
// no-op; this lets the writer call w.metrics.RecordX(...) without nil
// checks.
type NopMessageWriterMetrics struct{}

func (NopMessageWriterMetrics) RecordEnqueueUpsert(bool, bool)  {}
func (NopMessageWriterMetrics) RecordEnqueueDelete(bool)        {}
func (NopMessageWriterMetrics) RecordEnqueueVersion()           {}
func (NopMessageWriterMetrics) RecordEnqueueFailure(string)     {}
func (NopMessageWriterMetrics) ObserveQueueDepth(int)           {}
func (NopMessageWriterMetrics) RecordFlush(int, time.Duration)  {}
func (NopMessageWriterMetrics) RecordFlushSuccess(string, int)  {}
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
	mu sync.RWMutex

	enqueueUpserts  atomic.Int64
	enqueueDeletes  atomic.Int64
	enqueueVersions atomic.Int64
	enqueueMetrics  atomic.Int64
	enqueueFailures map[string]*atomic.Int64
	maxQueueDepth   atomic.Int64

	flushCycles   atomic.Int64
	flushRequests atomic.Int64
	lastBatchSize atomic.Int64
	flushDuration messageWriterSummary

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

func (m *InMemoryMessageWriterMetrics) RecordEnqueueUpsert(version bool, metric bool) {
	m.enqueueUpserts.Add(1)
	if version {
		m.enqueueVersions.Add(1)
	}
	if metric {
		m.enqueueMetrics.Add(1)
	}
}

func (m *InMemoryMessageWriterMetrics) RecordEnqueueDelete(version bool) {
	m.enqueueDeletes.Add(1)
	if version {
		m.enqueueVersions.Add(1)
	}
}

func (m *InMemoryMessageWriterMetrics) RecordEnqueueVersion() {
	m.enqueueVersions.Add(1)
}

func (m *InMemoryMessageWriterMetrics) RecordEnqueueFailure(cause string) {
	getOrCreateMessageWriterLabeledCounter(&m.mu, m.enqueueFailures, cause).Add(1)
}

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

func (m *InMemoryMessageWriterMetrics) RecordFlush(batchSize int, duration time.Duration) {
	m.flushCycles.Add(1)
	if batchSize > 0 {
		m.flushRequests.Add(int64(batchSize))
		m.lastBatchSize.Store(int64(batchSize))
	}
	m.flushDuration.observe(duration)
}

func (m *InMemoryMessageWriterMetrics) RecordFlushSuccess(op string, count int) {
	if count <= 0 {
		return
	}
	getOrCreateMessageWriterLabeledCounter(&m.mu, m.flushedByOp, op).Add(int64(count))
}

func (m *InMemoryMessageWriterMetrics) RecordFlushFallback(op string, count int) {
	if count <= 0 {
		return
	}
	getOrCreateMessageWriterLabeledCounter(&m.mu, m.fallbackByOp, op).Add(int64(count))
}

// Snapshot returns a JSON-friendly view of the current counter state. The
// returned MessageWriterMetricsSnapshot is a copy; callers can mutate it
// without affecting the live counters.
func (m *InMemoryMessageWriterMetrics) Snapshot() MessageWriterMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

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
			Duration:      m.flushDuration.snapshot(),
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

func getOrCreateMessageWriterLabeledCounter(mu *sync.RWMutex, m map[string]*atomic.Int64, key string) *atomic.Int64 {
	mu.RLock()
	if c, ok := m[key]; ok {
		mu.RUnlock()
		return c
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if c, ok := m[key]; ok {
		return c
	}
	c := &atomic.Int64{}
	m[key] = c
	return c
}

// messageWriterSummary is the count + sum + max tracker behind
// MessageWriterSummarySnapshot. Three atomics — observe is lock-free,
// snapshot is one load each.
type messageWriterSummary struct {
	count    atomic.Int64
	sumNanos atomic.Int64
	maxNanos atomic.Int64
}

func (s *messageWriterSummary) observe(d time.Duration) {
	if d < 0 {
		d = 0
	}
	s.count.Add(1)
	s.sumNanos.Add(d.Nanoseconds())
	candidate := d.Nanoseconds()
	for {
		cur := s.maxNanos.Load()
		if candidate <= cur {
			return
		}
		if s.maxNanos.CompareAndSwap(cur, candidate) {
			return
		}
	}
}

func (s *messageWriterSummary) snapshot() MessageWriterSummarySnapshot {
	if s == nil {
		return MessageWriterSummarySnapshot{}
	}
	return MessageWriterSummarySnapshot{
		Count:      s.count.Load(),
		SumSeconds: time.Duration(s.sumNanos.Load()).Seconds(),
		MaxSeconds: time.Duration(s.maxNanos.Load()).Seconds(),
	}
}
