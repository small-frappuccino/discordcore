// Package observability provides the shared low-level primitives that
// in-memory metrics implementations across the bot use. Each domain
// package (pkg/qotd, pkg/discord/commands/moderation, ...) still owns its
// own Metrics interface, MetricsSnapshot shape, and InMemoryMetrics
// struct; they just stop copying the underlying primitives.
//
// Designed so a future Prometheus / OpenTelemetry / quantile migration is
// one change here rather than N parallel changes across domain packages.
package observability

import (
	"sync"
	"sync/atomic"
	"time"
)

// SummarySnapshot is the count/sum/max shape that mirrors a Prometheus
// summary minus quantiles. Operators get average via sum/count and tail
// behavior via max. Designed so a Prometheus migration is one transform
// per field, not a redesign.
type SummarySnapshot struct {
	Count      int64   `json:"count"`
	SumSeconds float64 `json:"sum_seconds"`
	MaxSeconds float64 `json:"max_seconds"`
}

// Summary is the count + sum + max tracker behind SummarySnapshot. Three
// atomics — Observe is lock-free, Snapshot is one atomic load each.
//
// Goroutine safety: every method is safe to call concurrently. The zero
// value is ready to use; no constructor is required.
type Summary struct {
	count    atomic.Int64
	sumNanos atomic.Int64
	maxNanos atomic.Int64
}

// Observe records one duration sample. Negative durations are clamped to
// zero so a clock skew or accidental subtraction never poisons the sum or
// max. The max update uses CAS to remain lock-free under contention.
func (s *Summary) Observe(d time.Duration) {
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

// Snapshot returns a JSON-friendly view of the current tracker state. A
// nil receiver returns the zero SummarySnapshot so domain code that
// snapshots an absent per-label Summary does not need a nil check.
func (s *Summary) Snapshot() SummarySnapshot {
	if s == nil {
		return SummarySnapshot{}
	}
	return SummarySnapshot{
		Count:      s.count.Load(),
		SumSeconds: time.Duration(s.sumNanos.Load()).Seconds(),
		MaxSeconds: time.Duration(s.maxNanos.Load()).Seconds(),
	}
}

// GetOrCreateLabeledCounter is the lazy-init helper used by in-memory
// metrics implementations that hold labeled atomic counters
// (failure-by-cause, per-mode publish totals, etc.). The double-checked
// pattern keeps the hot path on a read lock plus a map lookup; only the
// first observer of a new key pays for the write lock.
//
// Key type is generic so callers can use either a plain string label or a
// domain newtype (e.g. qotd.PublishMode) without losing type safety at
// the call site.
func GetOrCreateLabeledCounter[K comparable](mu *sync.Mutex, m *map[K]*atomic.Int64, key K) *atomic.Int64 {
	mu.Lock()
	defer mu.Unlock()
	if *m == nil {
		*m = make(map[K]*atomic.Int64)
	}
	if c, ok := (*m)[key]; ok {
		return c
	}
	c := &atomic.Int64{}
	(*m)[key] = c
	return c
}

// GetOrCreateLabeledSummary is the lazy-init helper used by in-memory
// metrics implementations that hold labeled summaries (e.g. per-task latencies).
// The double-checked pattern keeps the hot path on a read lock plus a map lookup;
// only the first observer of a new key pays for the write lock.
func GetOrCreateLabeledSummary[K comparable](mu *sync.Mutex, m *map[K]*Summary, key K) *Summary {
	mu.Lock()
	defer mu.Unlock()
	if *m == nil {
		*m = make(map[K]*Summary)
	}
	if s, ok := (*m)[key]; ok {
		return s
	}
	s := &Summary{}
	(*m)[key] = s
	return s
}
