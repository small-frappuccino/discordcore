package partners

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncRecorder struct {
	mu      sync.Mutex
	calls   []string
	started chan string
	block   map[string]chan struct{}
}

func newSyncRecorder() *syncRecorder {
	return &syncRecorder{
		started: make(chan string, 32),
		block:   map[string]chan struct{}{},
	}
}

func (r *syncRecorder) SyncGuild(ctx context.Context, guildID string) error {
	r.mu.Lock()
	r.calls = append(r.calls, guildID)
	blockCh := r.block[guildID]
	r.mu.Unlock()

	select {
	case r.started <- guildID:
	default:
	}

	if blockCh != nil {
		select {
		case <-blockCh:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (r *syncRecorder) count(guildID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	total := 0
	for _, call := range r.calls {
		if guildID == "" || call == guildID {
			total++
		}
	}
	return total
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestAutoSyncCoordinatorDebounceSingleGuild(t *testing.T) {
	t.Parallel()

	rec := newSyncRecorder()
	coordinator := NewAutoSyncCoordinator(rec, AutoSyncCoordinatorOptions{
		Debounce:    40 * time.Millisecond,
		Tick:        10 * time.Millisecond,
		SyncTimeout: 2 * time.Second,
	})
	if err := coordinator.Start(); err != nil {
		t.Fatalf("start coordinator: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = coordinator.Stop(ctx)
	})

	for i := 0; i < 3; i++ {
		if err := coordinator.Notify("g1"); err != nil {
			t.Fatalf("notify g1 (iteration %d): %v", i, err)
		}
	}

	waitForCondition(t, 500*time.Millisecond, func() bool {
		return rec.count("g1") >= 1
	})

	time.Sleep(120 * time.Millisecond)
	if got := rec.count("g1"); got != 1 {
		t.Fatalf("expected exactly one sync call for debounced guild, got %d", got)
	}
}

func TestAutoSyncCoordinatorQueuesMultipleGuilds(t *testing.T) {
	t.Parallel()

	rec := newSyncRecorder()
	coordinator := NewAutoSyncCoordinator(rec, AutoSyncCoordinatorOptions{
		Debounce:    20 * time.Millisecond,
		Tick:        10 * time.Millisecond,
		SyncTimeout: 2 * time.Second,
	})
	if err := coordinator.Start(); err != nil {
		t.Fatalf("start coordinator: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = coordinator.Stop(ctx)
	})

	if err := coordinator.Notify("g1"); err != nil {
		t.Fatalf("notify g1: %v", err)
	}
	if err := coordinator.Notify("g2"); err != nil {
		t.Fatalf("notify g2: %v", err)
	}

	waitForCondition(t, 700*time.Millisecond, func() bool {
		return rec.count("g1") >= 1 && rec.count("g2") >= 1
	})
}

func TestAutoSyncCoordinatorNotifyWhileProcessingSchedulesNextRun(t *testing.T) {
	t.Parallel()

	rec := newSyncRecorder()
	block := make(chan struct{})
	rec.block["g1"] = block

	coordinator := NewAutoSyncCoordinator(rec, AutoSyncCoordinatorOptions{
		Debounce:    20 * time.Millisecond,
		Tick:        10 * time.Millisecond,
		SyncTimeout: 2 * time.Second,
	})
	if err := coordinator.Start(); err != nil {
		t.Fatalf("start coordinator: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = coordinator.Stop(ctx)
	})

	if err := coordinator.Notify("g1"); err != nil {
		t.Fatalf("notify first g1: %v", err)
	}

	select {
	case guildID := <-rec.started:
		if guildID != "g1" {
			t.Fatalf("expected first started guild to be g1, got %q", guildID)
		}
	case <-time.After(700 * time.Millisecond):
		t.Fatal("timed out waiting first started sync")
	}

	if err := coordinator.Notify("g1"); err != nil {
		t.Fatalf("notify second g1: %v", err)
	}

	time.Sleep(40 * time.Millisecond)
	close(block)
	delete(rec.block, "g1")

	waitForCondition(t, 700*time.Millisecond, func() bool {
		return rec.count("g1") >= 2
	})
}

func TestAutoSyncCoordinatorStartStopAndNotifyValidation(t *testing.T) {
	t.Parallel()

	coordinator := NewAutoSyncCoordinator(nil, AutoSyncCoordinatorOptions{})
	if err := coordinator.Start(); err == nil {
		t.Fatal("expected start failure when executor is nil")
	}

	rec := newSyncRecorder()
	coordinator = NewAutoSyncCoordinator(rec, AutoSyncCoordinatorOptions{
		Debounce: 10 * time.Millisecond,
		Tick:     5 * time.Millisecond,
	})
	if err := coordinator.Start(); err != nil {
		t.Fatalf("start coordinator: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := coordinator.Stop(ctx); err != nil {
		t.Fatalf("stop coordinator: %v", err)
	}

	if err := coordinator.Notify(""); err == nil {
		t.Fatal("expected notify validation error for empty guild")
	}
	if err := coordinator.Notify("g1"); err == nil {
		t.Fatal("expected notify error when coordinator is not running")
	} else if !strings.Contains(err.Error(), "not running") {
		t.Fatalf("expected not-running error, got %v", err)
	}
}
