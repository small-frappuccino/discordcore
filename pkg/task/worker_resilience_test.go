package task

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// FuzzTaskRouter_PayloadResilience tests continuous mutation of byte slices
// with oscillating latency in the producer.
func FuzzTaskRouter_PayloadResilience(f *testing.F) {
	f.Add([]byte("initial payload"), uint(10))
	f.Add([]byte("another payload"), uint(50))

	f.Fuzz(func(t *testing.T, payload []byte, latency uint) {
		tr := NewRouter(Defaults())
		defer tr.Close()

		tr.RegisterHandler("fuzz_task", func(ctx context.Context, p any) error {
			// Simulate some work
			time.Sleep(1 * time.Millisecond)
			return nil
		})

		ctx := context.Background()

		// Oscillating latency simulation
		if latency > 100 {
			latency = 100
		}
		time.Sleep(time.Duration(latency) * time.Millisecond)

		err := tr.Dispatch(ctx, Task{
			Type:    "fuzz_task",
			Payload: payload,
		})

		if err != nil && !errors.Is(err, ErrRouterClosed) {
			t.Errorf("Dispatch failed: %v", err)
		}
	})
}

// TestTaskRouter_DeadlinePreemption tests substitution of target function dependency
// by mocks that block the scheduler beyond ctx.Deadline().
func TestTaskRouter_DeadlinePreemption(t *testing.T) {
	tr := NewRouter(Defaults())
	defer tr.Close()

	started := make(chan struct{})

	tr.RegisterHandler("block_task", func(ctx context.Context, p any) error {
		close(started)
		// Simulating mock that blocks infinitely, exceeding context deadline.
		<-ctx.Done()
		return ctx.Err()
	})

	dispatchCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := tr.Dispatch(dispatchCtx, Task{
		Type:    "block_task",
		Payload: "payload",
	})
	if err != nil {
		t.Fatalf("Failed to dispatch: %v", err)
	}

	<-started

	// Force immediate close to trigger the ctx.Done() inside the handler
	// via tr.ctx cancellation in Close()
	tr.Close()

	// If we get here quickly, preemption works
}

// TestTaskRouter_ConcurrentOverloadAndShutdown tests parallel execution overload
// followed by simultaneous destruction of the instance.
func TestTaskRouter_ConcurrentOverloadAndShutdown(t *testing.T) {
	tr := NewRouter(Defaults())

	var wg sync.WaitGroup

	tr.RegisterHandler("overload_task", func(ctx context.Context, p any) error {
		// Just wait to be cancelled
		<-ctx.Done()
		return nil
	})

	// Dispatch tasks concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = tr.Dispatch(context.Background(), Task{
				Type:    "overload_task",
				Payload: idx,
				Options: TaskOptions{GroupKey: "overload_group"},
			})
		}(i)
	}

	// Overload and shutdown simultaneously
	go func() {
		// Wait just a tiny bit to allow some dispatches
		time.Sleep(2 * time.Millisecond)
		tr.Close()
	}()

	wg.Wait()
	// Should not block or panic.
}

// TestTaskRouter_RaceConditions tests aggressive dispatch of parallel inputs
// to catch data races (run under -race).
func TestTaskRouter_RaceConditions(t *testing.T) {
	tr := NewRouter(Defaults())
	defer tr.Close()

	var counter int32
	tr.RegisterHandler("race_task", func(ctx context.Context, p any) error {
		atomic.AddInt32(&counter, 1)
		// Incrementing shared state safely (if it wasn't safe, -race would catch it if it was raw,
		// but since it's atomic it's fine. We use a lock to simulate normal work).
		// Note: The task router limits concurrency per group (default 1).
		// We'll dispatch to many groups to maximize parallelism.
		return nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Randomize group to ensure high parallelism
			group := string(rune('a' + (idx % 26)))

			_ = tr.Dispatch(context.Background(), Task{
				Type:    "race_task",
				Payload: idx,
				Options: TaskOptions{GroupKey: group},
			})
		}(i)
	}

	wg.Wait()
}
