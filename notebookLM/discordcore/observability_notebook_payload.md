# Domain Architecture: observability

## Layout Topology
```text
observability/
├── primitives.go
└── primitives_test.go
```

## Source Stream Aggregation

// === FILE: pkg/observability/primitives.go ===
```go
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

```

// === FILE: pkg/observability/primitives_test.go ===
```go
package observability

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestSummaryBasic(t *testing.T) {
	t.Parallel()
	// Zero value ready to use
	var s Summary

	// nil receiver test
	var nilS *Summary
	nilSnap := nilS.Snapshot()
	if nilSnap.Count != 0 || nilSnap.SumSeconds != 0 || nilSnap.MaxSeconds != 0 {
		t.Errorf("expected zero snapshot for nil Summary")
	}

	// Positive duration
	s.Observe(100 * time.Millisecond)
	snap := s.Snapshot()
	if snap.Count != 1 {
		t.Errorf("expected count 1, got %d", snap.Count)
	}
	if snap.SumSeconds != 0.1 {
		t.Errorf("expected sum 0.1, got %f", snap.SumSeconds)
	}
	if snap.MaxSeconds != 0.1 {
		t.Errorf("expected max 0.1, got %f", snap.MaxSeconds)
	}

	// Negative duration (clamps to 0)
	s.Observe(-50 * time.Millisecond)
	snap = s.Snapshot()
	if snap.Count != 2 {
		t.Errorf("expected count 2, got %d", snap.Count)
	}
	if snap.SumSeconds != 0.1 {
		t.Errorf("expected sum 0.1 (not incremented), got %f", snap.SumSeconds)
	}
	if snap.MaxSeconds != 0.1 {
		t.Errorf("expected max 0.1, got %f", snap.MaxSeconds)
	}

	// Lower duration than max
	s.Observe(50 * time.Millisecond)
	snap = s.Snapshot()
	if snap.Count != 3 {
		t.Errorf("expected count 3, got %d", snap.Count)
	}
	if snap.SumSeconds != 0.15 {
		t.Errorf("expected sum 0.15, got %f", snap.SumSeconds)
	}
	if snap.MaxSeconds != 0.1 {
		t.Errorf("expected max 0.1, got %f", snap.MaxSeconds)
	}

	// Higher duration than max
	s.Observe(200 * time.Millisecond)
	snap = s.Snapshot()
	if snap.Count != 4 {
		t.Errorf("expected count 4, got %d", snap.Count)
	}
	if snap.SumSeconds != 0.35 {
		t.Errorf("expected sum 0.35, got %f", snap.SumSeconds)
	}
	if snap.MaxSeconds != 0.2 {
		t.Errorf("expected max 0.2, got %f", snap.MaxSeconds)
	}
}

func TestSummaryConcurrency(t *testing.T) {
	t.Parallel()
	var s Summary
	eg, ctx := errgroup.WithContext(context.Background())
	workers := 20
	iterations := 1000

	for i := 0; i < workers; i++ {
		workerID := i
		eg.Go(func() error {
			for j := 0; j < iterations; j++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				// Alternating durations to stress the CAS loop
				dur := time.Duration(j+workerID) * time.Microsecond
				s.Observe(dur)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent Summary observation failed: %v", err)
	}
	snap := s.Snapshot()
	expectedCount := int64(workers * iterations)
	if snap.Count != expectedCount {
		t.Errorf("expected count %d, got %d", expectedCount, snap.Count)
	}
	if snap.MaxSeconds <= 0 {
		t.Errorf("expected positive max seconds")
	}
}

type customKey string

func TestGetOrCreateLabeledCounter(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var m map[customKey]*atomic.Int64

	// First call on nil map
	c1 := GetOrCreateLabeledCounter(&mu, &m, customKey("foo"))
	if c1 == nil {
		t.Fatalf("expected non-nil counter")
	}
	c1.Add(5)

	// Second call (existing key - fast path)
	c2 := GetOrCreateLabeledCounter(&mu, &m, customKey("foo"))
	if c2 != c1 {
		t.Errorf("expected same counter pointer")
	}
	if c2.Load() != 5 {
		t.Errorf("expected counter value to persist")
	}

	// Third call (new key - slow path on initialized map)
	c3 := GetOrCreateLabeledCounter(&mu, &m, customKey("bar"))
	if c3 == c1 {
		t.Errorf("expected different counter pointer for different key")
	}
	c3.Add(10)

	// Verify values
	if m[customKey("foo")].Load() != 5 {
		t.Errorf("unexpected value for foo")
	}
	if m[customKey("bar")].Load() != 10 {
		t.Errorf("unexpected value for bar")
	}
}

func TestGetOrCreateLabeledSummary(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var m map[string]*Summary

	// First call on nil map
	s1 := GetOrCreateLabeledSummary(&mu, &m, "latencies")
	if s1 == nil {
		t.Fatalf("expected non-nil summary")
	}
	s1.Observe(150 * time.Millisecond)

	// Second call (existing key - fast path)
	s2 := GetOrCreateLabeledSummary(&mu, &m, "latencies")
	if s2 != s1 {
		t.Errorf("expected same summary pointer")
	}
	snap := s2.Snapshot()
	if snap.Count != 1 || snap.SumSeconds != 0.15 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}

	// Third call (new key - slow path on initialized map)
	s3 := GetOrCreateLabeledSummary(&mu, &m, "other_latencies")
	if s3 == s1 {
		t.Errorf("expected different summary pointer for different key")
	}
	s3.Observe(300 * time.Millisecond)

	// Verify values
	if m["latencies"].Snapshot().Count != 1 {
		t.Errorf("unexpected count for latencies")
	}
	if m["other_latencies"].Snapshot().Count != 1 {
		t.Errorf("unexpected count for other_latencies")
	}
}

```

