package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func updateMaxConcurrent(max *int32, current int32) {
	for {
		existing := atomic.LoadInt32(max)
		if current <= existing {
			return
		}
		if atomic.CompareAndSwapInt32(max, existing, current) {
			return
		}
	}
}

func waitForAtomicAtLeast(t *testing.T, counter *int32, want int32, timeout time.Duration, description string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(counter) >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := atomic.LoadInt32(counter); got < want {
		t.Fatalf("timed out waiting for %s: got=%d want>=%d", description, got, want)
	}
}

func TestResolveRuntimeStartupParallelism(t *testing.T) {
	tests := []struct {
		runtimeCount int
		want         int
	}{
		{runtimeCount: 0, want: 1},
		{runtimeCount: 1, want: 1},
		{runtimeCount: 2, want: 2},
		{runtimeCount: 4, want: 3},
	}

	for _, tt := range tests {
		if got := resolveRuntimeStartupParallelism(tt.runtimeCount); got != tt.want {
			t.Fatalf("resolveRuntimeStartupParallelism(%d)=%d want %d", tt.runtimeCount, got, tt.want)
		}
	}
}

func TestResolveRuntimeBackgroundParallelism(t *testing.T) {
	tests := []struct {
		runtimeCount int
		want         int
	}{
		{runtimeCount: 0, want: 1},
		{runtimeCount: 1, want: 1},
		{runtimeCount: 2, want: 2},
		{runtimeCount: 4, want: 2},
	}

	for _, tt := range tests {
		if got := resolveRuntimeBackgroundParallelism(tt.runtimeCount); got != tt.want {
			t.Fatalf("resolveRuntimeBackgroundParallelism(%d)=%d want %d", tt.runtimeCount, got, tt.want)
		}
	}
}

func TestResolveStartupLightParallelism(t *testing.T) {
	tests := []struct {
		runtimeCount int
		want         int
	}{
		{runtimeCount: 0, want: 2},
		{runtimeCount: 1, want: 2},
		{runtimeCount: 2, want: 3},
		{runtimeCount: 4, want: 4},
	}

	for _, tt := range tests {
		if got := resolveStartupLightParallelism(tt.runtimeCount); got != tt.want {
			t.Fatalf("resolveStartupLightParallelism(%d)=%d want %d", tt.runtimeCount, got, tt.want)
		}
	}
}

func TestOpenBotRuntimesAppliesParallelLimit(t *testing.T) {
	origOpenBotRuntime := openBotRuntimeFn
	t.Cleanup(func() {
		openBotRuntimeFn = origOpenBotRuntime
	})

	release := make(chan struct{})
	var closeRelease sync.Once
	t.Cleanup(func() {
		closeRelease.Do(func() { close(release) })
	})

	var current int32
	var maxConcurrent int32
	openBotRuntimeFn = func(instance resolvedBotInstance, capabilities botRuntimeCapabilities) (*botRuntime, error) {
		n := atomic.AddInt32(&current, 1)
		updateMaxConcurrent(&maxConcurrent, n)
		<-release
		atomic.AddInt32(&current, -1)
		return &botRuntime{instanceID: instance.ID, capabilities: capabilities}, nil
	}

	botInstances := []resolvedBotInstance{
		{ID: "bot-1", TokenEnv: "TOKEN_1", Token: "token-1"},
		{ID: "bot-2", TokenEnv: "TOKEN_2", Token: "token-2"},
		{ID: "bot-3", TokenEnv: "TOKEN_3", Token: "token-3"},
		{ID: "bot-4", TokenEnv: "TOKEN_4", Token: "token-4"},
	}
	capabilities := map[string]botRuntimeCapabilities{
		"bot-1": {},
		"bot-2": {},
		"bot-3": {},
		"bot-4": {},
	}

	type result struct {
		runtimes map[string]*botRuntime
		order    []*botRuntime
		err      error
	}
	done := make(chan result, 1)
	go func() {
		runtimes, order, err := openBotRuntimes(botInstances, capabilities)
		done <- result{runtimes: runtimes, order: order, err: err}
	}()

	waitForAtomicAtLeast(t, &maxConcurrent, 3, time.Second, "runtime opens to reach the configured limit")
	closeRelease.Do(func() { close(release) })

	res := <-done
	if res.err != nil {
		t.Fatalf("openBotRuntimes returned error: %v", res.err)
	}
	if got := atomic.LoadInt32(&maxConcurrent); got != 3 {
		t.Fatalf("expected open phase to cap at 3 concurrent runtimes, got %d", got)
	}
	if len(res.runtimes) != 4 {
		t.Fatalf("expected 4 opened runtimes, got %d", len(res.runtimes))
	}
	if len(res.order) != 4 {
		t.Fatalf("expected 4 runtimes in startup order, got %d", len(res.order))
	}
}

func TestInitializeBotRuntimesAppliesParallelLimit(t *testing.T) {
	origInitializeBotRuntime := initializeBotRuntimeFn
	t.Cleanup(func() {
		initializeBotRuntimeFn = origInitializeBotRuntime
	})

	release := make(chan struct{})
	var closeRelease sync.Once
	t.Cleanup(func() {
		closeRelease.Do(func() { close(release) })
	})

	var current int32
	var maxConcurrent int32
	initializeBotRuntimeFn = func(runtime *botRuntime, opts botRuntimeOptions) error {
		n := atomic.AddInt32(&current, 1)
		updateMaxConcurrent(&maxConcurrent, n)
		<-release
		atomic.AddInt32(&current, -1)
		return nil
	}

	runtimeOrder := []*botRuntime{
		{instanceID: "bot-1"},
		{instanceID: "bot-2"},
		{instanceID: "bot-3"},
		{instanceID: "bot-4"},
	}

	done := make(chan error, 1)
	go func() {
		done <- initializeBotRuntimes(runtimeOrder, botRuntimeOptions{})
	}()

	waitForAtomicAtLeast(t, &maxConcurrent, 3, time.Second, "runtime initialization to reach the configured limit")
	closeRelease.Do(func() { close(release) })

	if err := <-done; err != nil {
		t.Fatalf("initializeBotRuntimes returned error: %v", err)
	}
	if got := atomic.LoadInt32(&maxConcurrent); got != 3 {
		t.Fatalf("expected initialize phase to cap at 3 concurrent runtimes, got %d", got)
	}
}

func TestRuntimeStartupBackgroundWorkerAppliesParallelLimit(t *testing.T) {
	worker := newRuntimeStartupBackgroundWorker(4)

	release := make(chan struct{})
	var current int32
	var maxConcurrent int32
	for i := 0; i < 4; i++ {
		worker.Go(func(context.Context) error {
			n := atomic.AddInt32(&current, 1)
			updateMaxConcurrent(&maxConcurrent, n)
			<-release
			atomic.AddInt32(&current, -1)
			return nil
		})
	}

	waitForAtomicAtLeast(t, &maxConcurrent, 2, time.Second, "background startup tasks to reach the configured limit")
	close(release)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown background worker: %v", err)
	}
	if got := atomic.LoadInt32(&maxConcurrent); got != 2 {
		t.Fatalf("expected background tasks to cap at 2 concurrent jobs, got %d", got)
	}
}
