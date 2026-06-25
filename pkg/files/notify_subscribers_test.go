package files

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// mockSubscriber implements ConfigMutationSubscriber
type mockSubscriber struct {
	fn func(ctx context.Context, snapshot ConfigSnapshot)
}

func (m *mockSubscriber) OnConfigMutated(ctx context.Context, snapshot ConfigSnapshot) {
	if m.fn != nil {
		m.fn(ctx, snapshot)
	}
}

// Concurrency stress validation: Non-blocking fan-out
func TestDispatchConfigMutation_ConcurrencyStress(t *testing.T) {
	t.Parallel()

	registry := NewFeatureRegistry()

	const numSubscribers = 100
	var successCount int32
	var wg sync.WaitGroup
	wg.Add(numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		id := string(rune(i))
		registry.Subscribe(id, &mockSubscriber{
			fn: func(ctx context.Context, snapshot ConfigSnapshot) {
				defer wg.Done()
				atomic.AddInt32(&successCount, 1)
				// Simulate some computational work
				time.Sleep(2 * time.Millisecond)
			},
		})
	}

	cfg := &BotConfig{}

	start := time.Now()
	// This must not block the primary thread waiting for the 100 goroutines
	DispatchConfigMutation(registry, cfg)
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("DispatchConfigMutation blocked caller, took %v", elapsed)
	}

	wg.Wait()

	if atomic.LoadInt32(&successCount) != numSubscribers {
		t.Errorf("expected %d successes, got %d", numSubscribers, successCount)
	}
}

// Panic recovery validation
func TestDispatchConfigMutation_PanicRecovery(t *testing.T) {
	t.Parallel()

	registry := NewFeatureRegistry()
	var successCount int32
	var wg sync.WaitGroup
	wg.Add(2) // We only expect 2 to successfully complete their work

	registry.Subscribe("sub1", &mockSubscriber{
		fn: func(ctx context.Context, snapshot ConfigSnapshot) {
			defer wg.Done()
			atomic.AddInt32(&successCount, 1)
		},
	})

	registry.Subscribe("sub2_panic", &mockSubscriber{
		fn: func(ctx context.Context, snapshot ConfigSnapshot) {
			panic("simulated subscriber panic")
		},
	})

	registry.Subscribe("sub3", &mockSubscriber{
		fn: func(ctx context.Context, snapshot ConfigSnapshot) {
			defer wg.Done()
			atomic.AddInt32(&successCount, 1)
		},
	})

	cfg := &BotConfig{}

	// This should not crash the test suite
	DispatchConfigMutation(registry, cfg)

	wg.Wait()

	if atomic.LoadInt32(&successCount) != 2 {
		t.Errorf("expected 2 successful subscribers despite panic, got %d", successCount)
	}
}

// Context bounded execution validation
func TestDispatchConfigMutation_ContextBounded(t *testing.T) {
	t.Parallel()

	registry := NewFeatureRegistry()

	var ctxErr error
	var wg sync.WaitGroup
	wg.Add(1)

	registry.Subscribe("sub_timeout", &mockSubscriber{
		fn: func(ctx context.Context, snapshot ConfigSnapshot) {
			defer wg.Done()
			// Validate that the context is actually injected and has a strict deadline
			_, ok := ctx.Deadline()
			if !ok {
				ctxErr = context.DeadlineExceeded
			}
		},
	})

	cfg := &BotConfig{}
	DispatchConfigMutation(registry, cfg)

	wg.Wait()

	if ctxErr != nil {
		t.Errorf("expected context to have a strict deadline boundary")
	}
}
