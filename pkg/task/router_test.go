package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func newTestConfig() RouterConfig {
	return RouterConfig{
		DefaultMaxAttempts: 3,
		InitialBackoff:     5 * time.Millisecond,
		MaxBackoff:         10 * time.Millisecond,
		IdempotencyTTL:     100 * time.Millisecond,
		GroupBuffer:        8,
		GroupIdleTTL:       200 * time.Millisecond,
		CleanupInterval:    20 * time.Millisecond,
		GlobalMaxWorkers:   0,
		GroupMaxParallel:   1,
	}
}

func TestDispatchExecutesHandler(t *testing.T) {
	router := NewRouter(newTestConfig())
	t.Cleanup(router.Close)

	done := make(chan string, 1)
	router.RegisterHandler("ping", func(ctx context.Context, payload any) error {
		done <- payload.(string)
		return nil
	})

	if err := router.Dispatch(context.Background(), Task{Type: "ping", Payload: "ok"}); err != nil {
		t.Fatalf("dispatch returned error: %v", err)
	}

	select {
	case val := <-done:
		if val != "ok" {
			t.Fatalf("unexpected payload: %s", val)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("handler did not run in time")
	}
}

func TestDispatchIdempotency(t *testing.T) {
	router := NewRouter(newTestConfig())
	t.Cleanup(router.Close)

	var calls int32
	ready := make(chan struct{}, 1)
	router.RegisterHandler("once", func(ctx context.Context, payload any) error {
		atomic.AddInt32(&calls, 1)
		ready <- struct{}{}
		return nil
	})

	task := Task{Type: "once", Options: TaskOptions{IdempotencyKey: "dup", IdempotencyTTL: 500 * time.Millisecond}}
	if err := router.Dispatch(context.Background(), task); err != nil {
		t.Fatalf("first dispatch failed: %v", err)
	}
	if err := router.Dispatch(context.Background(), task); !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("expected duplicate error, got: %v", err)
	}

	select {
	case <-ready:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("handler did not run for first dispatch")
	}

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected handler called once, got %d", calls)
	}
}

func TestDispatchRetriesOnError(t *testing.T) {
	cfg := newTestConfig()
	cfg.InitialBackoff = 5 * time.Millisecond
	cfg.MaxBackoff = 5 * time.Millisecond
	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	var attempts int32
	done := make(chan struct{})
	router.RegisterHandler("flaky", func(ctx context.Context, payload any) error {
		n := atomic.AddInt32(&attempts, 1)
		if n < 2 {
			return errors.New("fail")
		}
		close(done)
		return nil
	})

	if err := router.Dispatch(context.Background(), Task{Type: "flaky"}); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("handler did not succeed after retries")
	}

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestScheduleEveryRunsAndCancels(t *testing.T) {
	cfg := newTestConfig()
	cfg.CleanupInterval = 10 * time.Millisecond
	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	var count int32
	router.RegisterHandler("cron", func(ctx context.Context, payload any) error {
		atomic.AddInt32(&count, 1)
		return nil
	})

	cancel := router.ScheduleEvery(15*time.Millisecond, Task{Type: "cron"})
	time.Sleep(60 * time.Millisecond)
	cancel()
	afterCancel := atomic.LoadInt32(&count)
	time.Sleep(30 * time.Millisecond)

	if afterCancel == 0 {
		t.Fatalf("expected scheduled task to run at least once")
	}
	if atomic.LoadInt32(&count) > afterCancel+1 {
		t.Fatalf("scheduled task continued running after cancel")
	}
}

func TestSendToGroupClosedChannelDoesNotPanic(t *testing.T) {
	router := NewRouter(newTestConfig())
	t.Cleanup(router.Close)

	gw := &groupWorker{
		key: "g1",
		ch:  make(chan *enqueuedTask, 1),
	}
	close(gw.ch)

	ok := router.sendToGroup(gw, &enqueuedTask{task: Task{Type: "noop"}})
	if ok {
		t.Fatalf("expected sendToGroup to fail when channel is closed")
	}
}

func TestEnqueueRetryRecoversFromClosedGroupChannel(t *testing.T) {
	router := NewRouter(newTestConfig())
	t.Cleanup(router.Close)

	done := make(chan struct{}, 1)
	router.RegisterHandler("retry", func(ctx context.Context, payload any) error {
		done <- struct{}{}
		return nil
	})

	stale := &groupWorker{
		key:        "g1",
		ch:         make(chan *enqueuedTask, 1),
		lastActive: time.Now(),
	}
	close(stale.ch)

	router.mu.Lock()
	router.groups["g1"] = stale
	router.mu.Unlock()

	ok := router.enqueueRetry("g1", &enqueuedTask{
		task: Task{
			Type: "retry",
			Options: TaskOptions{
				GroupKey: "g1",
			},
		},
		attempt: 2,
	})
	if !ok {
		t.Fatalf("expected enqueueRetry to recover by recreating the group")
	}

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("re-enqueued task did not execute in time")
	}
}

func TestRunCronOnce_UpdatesLastRunEvenWhenDispatchFails(t *testing.T) {
	cfg := newTestConfig()
	cfg.CleanupInterval = time.Hour
	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	job := &cronJob{
		Interval: time.Millisecond,
		Task: Task{
			Type: "missing-handler",
		},
	}

	router.cronMu.Lock()
	router.cronJobs = append(router.cronJobs, job)
	router.cronMu.Unlock()

	if !job.lastRun.IsZero() {
		t.Fatalf("expected zero lastRun before cron execution")
	}

	router.runCronOnce()

	if job.lastRun.IsZero() {
		t.Fatalf("expected cron job lastRun to be updated even when dispatch fails")
	}
}
