package files

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// Injeção de $N$ assinantes de bloqueio simultâneo onde $N$ supera o limite de errgroup.SetLimit.
func TestNotifySubscribers_ConcurrencyLimitExceeded(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfgManager := NewConfigManagerWithStore(nil, nil)

	const numSubscribers = 25 // Exceeds errgroup limit of 10
	var activeWorkers int32
	var maxWorkers int32
	var wg sync.WaitGroup

	wg.Add(numSubscribers)

	// Barrier to hold workers until all are ready
	startBarrier := make(chan struct{})

	for i := 0; i < numSubscribers; i++ {
		cfgManager.AddSubscriber(func(ctx context.Context, oldConfig, newConfig *BotConfig) error {
			defer wg.Done()

			current := atomic.AddInt32(&activeWorkers, 1)

			for {
				max := atomic.LoadInt32(&maxWorkers)
				if current > max {
					if atomic.CompareAndSwapInt32(&maxWorkers, max, current) {
						break
					}
				} else {
					break
				}
			}

			<-startBarrier
			atomic.AddInt32(&activeWorkers, -1)
			return nil
		})
	}

	cfg := &BotConfig{}

	// Launch notifySubscribers in background so we can unblock the barrier
	notifyDone := make(chan struct{})
	go func() {
		_ = cfgManager.notifySubscribers(context.Background(), cfg, cfg)
		close(notifyDone)
	}()

	// Give enough time for errgroup to spawn up to its limit
	time.Sleep(100 * time.Millisecond)

	// Unblock all workers
	close(startBarrier)

	// Wait for notifications to finish
	<-notifyDone
	wg.Wait()

	limit := atomic.LoadInt32(&maxWorkers)
	if limit > 10 {
		t.Errorf("expected max concurrency <= 10, got %d", limit)
	}
}

// Disparo de exceções forçadas via panic("simulated") no interior das rotinas de processamento de múltiplos assinantes.
func TestNotifySubscribers_PanicRecovery(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfgManager := NewConfigManagerWithStore(nil, nil)

	var successCount int32

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		atomic.AddInt32(&successCount, 1)
		return nil
	})

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		panic("simulated")
	})

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		atomic.AddInt32(&successCount, 1)
		return nil
	})

	cfg := &BotConfig{}
	err := cfgManager.notifySubscribers(context.Background(), cfg, cfg)

	if err == nil || !strings.Contains(err.Error(), "simulated") {
		t.Fatalf("expected panic error, got %v", err)
	}

	// We can't guarantee how many succeeded due to errgroup short-circuiting on first error
	// But it shouldn't crash the test.
}

// Injeção de time.Sleep ou laços infinitos em assinantes combinada com uma janela milissegunda restrita via context.WithTimeout.
func TestNotifySubscribers_ContextTimeoutPreemption(t *testing.T) {
	defer goleak.VerifyNone(t)

	cfgManager := NewConfigManagerWithStore(nil, nil)

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := &BotConfig{}
	err := cfgManager.notifySubscribers(ctx, cfg, cfg)

	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}
