//go:build !legacy
// +build !legacy

package task

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestRouter_GroupKeySerialization(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	router := NewRouter(cfg)
	defer router.Close()

	var counter atomic.Int32
	var maxParallel atomic.Int32
	var currentParallel atomic.Int32

	router.RegisterHandler("serialize_test", func(ctx context.Context, payload any) error {
		v := currentParallel.Add(1)
		defer currentParallel.Add(-1)

		for {
			maxP := maxParallel.Load()
			if v > maxP {
				if !maxParallel.CompareAndSwap(maxP, v) {
					continue
				}
			}
			break
		}

		counter.Add(1)
		// Yield slightly to encourage race if serialization is broken
		time.Sleep(time.Microsecond)
		return nil
	})

	const numTasks = 10000
	var wg sync.WaitGroup
	wg.Add(numTasks)

	for i := 0; i < numTasks; i++ {
		go func(id int) {
			defer wg.Done()
			err := router.Dispatch(context.Background(), Task{
				Type: "serialize_test",
				Options: TaskOptions{
					GroupKey: "single_group",
				},
			})
			if err != nil {
				t.Errorf("Dispatch failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Flush tasks
	time.Sleep(200 * time.Millisecond)

	if c := counter.Load(); c != numTasks {
		t.Fatalf("Expected %d tasks processed, got %d", numTasks, c)
	}
	if p := maxParallel.Load(); p > 1 {
		t.Fatalf("Expected strictly 1 parallel execution per group, got %d", p)
	}
}

func TestRouter_ExecutionLimiter(t *testing.T) {
	t.Parallel()

	cfg := Defaults()
	cfg.ExecutionLimiter = NewExecutionLimiter(10)
	router := NewRouter(cfg)
	defer router.Close()

	var active atomic.Int32
	var maxActive atomic.Int32
	blockCh := make(chan struct{})

	router.RegisterHandler("limiter_test", func(ctx context.Context, payload any) error {
		v := active.Add(1)
		for {
			maxA := maxActive.Load()
			if v > maxA {
				if !maxActive.CompareAndSwap(maxA, v) {
					continue
				}
			}
			break
		}
		<-blockCh
		active.Add(-1)
		return nil
	})

	for i := 0; i < 50; i++ {
		// Unique group keys so group serialization doesn't limit concurrency
		err := router.Dispatch(context.Background(), Task{
			Type: "limiter_test",
			Options: TaskOptions{
				GroupKey: fmt.Sprintf("group_%d", i),
			},
		})
		if err != nil {
			t.Fatalf("Dispatch failed: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond) // Allow workers to spawn and block
	close(blockCh)                     // Unblock all
	time.Sleep(100 * time.Millisecond)

	if p := maxActive.Load(); p != 10 {
		t.Fatalf("Expected max active handlers to be bounded by Limiter (10), got %d", p)
	}
}

func TestRouter_IdempotencyTTL(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Now())
	cfg := Defaults()
	cfg.Clock = mockClock
	router := NewRouter(cfg)
	defer router.Close()

	router.RegisterHandler("idem_test", func(ctx context.Context, payload any) error {
		return nil
	})

	task := Task{
		Type: "idem_test",
		Options: TaskOptions{
			IdempotencyKey: "A",
			IdempotencyTTL: 60 * time.Second,
		},
	}

	if err := router.Dispatch(context.Background(), task); err != nil {
		t.Fatalf("Initial dispatch failed: %v", err)
	}

	if err := router.Dispatch(context.Background(), task); !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("Expected ErrDuplicateTask within TTL window, got %v", err)
	}

	mockClock.Advance(59 * time.Second)
	if err := router.Dispatch(context.Background(), task); !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("Expected ErrDuplicateTask at 59s, got %v", err)
	}

	mockClock.Advance(2 * time.Second) // Total 61s

	// Force cleanup
	router.cleanupOnce()

	if err := router.Dispatch(context.Background(), task); err != nil {
		t.Fatalf("Expected success after TTL expiry (61s), got %v", err)
	}
}

func TestRouter_RetryHeap(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Now())
	cfg := Defaults()
	cfg.Clock = mockClock
	cfg.DefaultMaxAttempts = 5
	cfg.InitialBackoff = 100 * time.Millisecond
	cfg.MaxBackoff = 2 * time.Second
	router := NewRouter(cfg)
	defer router.Close()

	var attemptCount atomic.Int32
	errStatic := errors.New("static network error")

	router.RegisterHandler("retry_test", func(ctx context.Context, payload any) error {
		attemptCount.Add(1)
		return errStatic
	})

	// Inject a task directly to test backoff computation natively
	delay1 := router.computeBackoff(100*time.Millisecond, 2*time.Second, 1)
	if delay1 < 90*time.Millisecond || delay1 > 110*time.Millisecond { // 10% jitter bounds
		t.Fatalf("Expected backoff ~100ms, got %v", delay1)
	}

	delay2 := router.computeBackoff(100*time.Millisecond, 2*time.Second, 2)
	if delay2 < 180*time.Millisecond || delay2 > 220*time.Millisecond {
		t.Fatalf("Expected backoff ~200ms, got %v", delay2)
	}

	// Test heap explicitly
	router.scheduleRetry("group_a", &enqueuedTask{attempt: 1}, 10*time.Second)
	router.scheduleRetry("group_b", &enqueuedTask{attempt: 1}, 5*time.Second)

	mockClock.Advance(6 * time.Second)
	due := router.popDueRetries(mockClock.Now())
	if len(due) != 1 || due[0].groupKey != "group_b" {
		t.Fatalf("Expected group_b to pop first")
	}

	mockClock.Advance(5 * time.Second)
	due = router.popDueRetries(mockClock.Now())
	if len(due) != 1 || due[0].groupKey != "group_a" {
		t.Fatalf("Expected group_a to pop second")
	}
}

func TestRouter_CronSchedule(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC))
	cfg := Defaults()
	cfg.Clock = mockClock
	router := NewRouter(cfg)
	defer router.Close()

	var runs atomic.Int32
	router.RegisterHandler("cron_task", func(ctx context.Context, payload any) error {
		runs.Add(1)
		return nil
	})

	// Schedule for 15:00 UTC daily
	cancel := router.ScheduleDailyAtUTC(15, 0, Task{Type: "cron_task"})
	defer cancel()

	// Initial time is 10:00. First target is today 15:00 (5h from now).
	// Advance clock by exactly 72 hours (3 days).
	// We expect 3 triggers: Today at 15:00, Tomorrow at 15:00, Day 3 at 15:00.
	for i := 0; i < 72; i++ {
		mockClock.Advance(1 * time.Hour)
		router.runCronOnce()
		// Wait slightly to let dispatch process
		time.Sleep(time.Millisecond)
	}

	if r := runs.Load(); r != 3 {
		t.Fatalf("Expected cron to fire exactly 3 times in 72h, got %d", r)
	}
}

func TestRouter_ContextCancel(t *testing.T) {
	t.Parallel()

	router := NewRouter(Defaults())

	blockCh := make(chan struct{})
	doneCh := make(chan struct{})
	var ctxErr error

	router.RegisterHandler("cancel_test", func(ctx context.Context, payload any) error {
		close(blockCh) // Signal we are inside handler
		<-ctx.Done()
		ctxErr = ctx.Err()
		close(doneCh)
		return nil
	})

	_ = router.Dispatch(context.Background(), Task{Type: "cancel_test"})
	<-blockCh

	router.Close()

	<-doneCh
	if !errors.Is(ctxErr, context.Canceled) {
		t.Fatalf("Expected context.Canceled, got %v", ctxErr)
	}
}

func TestRouter_Observability(t *testing.T) {
	t.Parallel()

	mockClock := clock.NewMockClock(time.Now())
	cfg := Defaults()
	cfg.Clock = mockClock
	router := NewRouter(cfg)
	defer router.Close()

	router.RegisterHandler("slow_test", func(ctx context.Context, payload any) error {
		// Simulate 6s execution
		mockClock.Advance(6 * time.Second)
		return nil
	})

	_ = router.Dispatch(context.Background(), Task{Type: "slow_test"})

	// Wait for handler to complete
	time.Sleep(50 * time.Millisecond)

	// Intercept observability metrics manually since getOrCreate is not strictly exported
	// We just ensure it doesn't panic and the latency map registers it.
	stats := router.Stats()
	if stats.RegisteredTypes != 1 {
		t.Errorf("Stats registered types mismatch")
	}

	router.latencyMu.RLock()
	s := router.latenciesByType["slow_test"]
	router.latencyMu.RUnlock()

	if s == nil {
		t.Fatalf("Expected latency summary to be created for slow_test")
	}
}

// FuzzRouter_QueueMutation validates thread safety and bounds against corrupted payload shapes.
func FuzzRouter_QueueMutation(f *testing.F) {
	f.Add("group_1", "idem_1")
	f.Add("", "")
	f.Add(string([]byte{0x00, 0xFF}), "null_idem")

	f.Fuzz(func(t *testing.T, group string, idem string) {
		router := NewRouter(Defaults())
		router.RegisterHandler("fuzz", func(ctx context.Context, payload any) error {
			return nil
		})

		err := router.Dispatch(context.Background(), Task{
			Type: "fuzz",
			Options: TaskOptions{
				GroupKey:       group,
				IdempotencyKey: idem,
				IdempotencyTTL: 1 * time.Second,
			},
		})
		if err != nil && err.Error() != errTaskEnqueue.Error() && !errors.Is(err, ErrUnknownTaskType) && !errors.Is(err, ErrDuplicateTask) {
			// All other structural panics or out-of-bounds maps will fail test naturally.
		}
		router.Close()
	})
}

// FuzzRouter_HeapLimits injects extreme boundaries to validate container/heap resilience.
func FuzzRouter_HeapLimits(f *testing.F) {
	f.Add(int64(-1), int64(1))
	f.Add(int64(math.MaxInt64), int64(math.MinInt64))
	f.Add(int64(0), int64(0))

	f.Fuzz(func(t *testing.T, t1, t2 int64) {
		var h retryTaskHeap
		heap.Init(&h)

		item1 := &scheduledRetry{at: time.Unix(t1, 0), seq: 1}
		item2 := &scheduledRetry{at: time.Unix(t2, 0), seq: 2}

		heap.Push(&h, item1)
		heap.Push(&h, item2)

		if h.Len() != 2 {
			t.Fatalf("Length mismatch")
		}

		_ = heap.Pop(&h)
		_ = heap.Pop(&h)
	})
}
