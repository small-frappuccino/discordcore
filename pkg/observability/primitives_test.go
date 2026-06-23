package observability

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSummaryBasic(t *testing.T) {
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
	var s Summary
	var wg sync.WaitGroup
	workers := 20
	iterations := 1000

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternating durations to stress the CAS loop
				dur := time.Duration(j+workerID) * time.Microsecond
				s.Observe(dur)
			}
		}(i)
	}

	wg.Wait()
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
	var mu sync.RWMutex
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
	var mu sync.RWMutex
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
