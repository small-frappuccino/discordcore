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

func newTestGroupWorker(router *TaskRouter, key string, buffer int) *groupWorker {
	nowNs := int64(1)
	if router != nil {
		nowNs = router.nowNs()
	}
	return newGroupWorker(key, buffer, nowNs)
}

func stopTestGroup(gw *groupWorker) {
	if gw == nil {
		return
	}
	gw.beginStop()
	gw.finishStop()
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

func TestRetryLoopReschedulesWhenGroupIsTemporarilyFull(t *testing.T) {
	cfg := newTestConfig()
	cfg.CleanupInterval = time.Hour
	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	done := make(chan struct{}, 1)
	router.RegisterHandler("retry-full", func(ctx context.Context, payload any) error {
		done <- struct{}{}
		return nil
	})

	blocked := newTestGroupWorker(router, "busy", 1)
	blocked.ch <- &enqueuedTask{task: Task{Type: "retry-full"}}

	router.mu.Lock()
	router.groups["busy"] = blocked
	router.mu.Unlock()

	router.scheduleRetry("busy", &enqueuedTask{
		task: Task{
			Type: "retry-full",
			Options: TaskOptions{
				GroupKey: "busy",
			},
		},
		attempt: 2,
	}, 5*time.Millisecond)

	time.Sleep(20 * time.Millisecond)

	router.mu.Lock()
	delete(router.groups, "busy")
	router.mu.Unlock()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatalf("scheduled retry did not run after blocked group was released")
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
	gw = newTestGroupWorker(router, "g1", 1)
	stopTestGroup(gw)

	ok := router.sendToGroup(gw, &enqueuedTask{task: Task{Type: "noop"}})
	if ok {
		t.Fatalf("expected sendToGroup to fail when channel is closed")
	}
}

func TestDispatchRecoversFromClosedGroupChannel(t *testing.T) {
	router := NewRouter(newTestConfig())
	t.Cleanup(router.Close)

	done := make(chan struct{}, 1)
	router.RegisterHandler("dispatch-recover", func(ctx context.Context, payload any) error {
		done <- struct{}{}
		return nil
	})

	stale := newTestGroupWorker(router, "g1", 1)
	stopTestGroup(stale)

	router.mu.Lock()
	router.groups["g1"] = stale
	router.mu.Unlock()

	if err := router.Dispatch(context.Background(), Task{
		Type: "dispatch-recover",
		Options: TaskOptions{
			GroupKey: "g1",
		},
	}); err != nil {
		t.Fatalf("expected dispatch to recover from closed group channel, got %v", err)
	}

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("recovered dispatch did not execute in time")
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

	stale := newTestGroupWorker(router, "g1", 1)
	stopTestGroup(stale)

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

func TestSharedExecutionLimiterBoundsMultipleRouters(t *testing.T) {
	cfg := newTestConfig()
	limiter := NewExecutionLimiter(1)
	cfg.ExecutionLimiter = limiter
	cfg.GlobalMaxWorkers = limiter.Capacity()

	routerA := NewRouter(cfg)
	t.Cleanup(routerA.Close)
	routerB := NewRouter(cfg)
	t.Cleanup(routerB.Close)

	var active int32
	var maxActive int32
	release := make(chan struct{})
	started := make(chan struct{}, 2)

	handler := func(ctx context.Context, payload any) error {
		current := atomic.AddInt32(&active, 1)
		for {
			observed := atomic.LoadInt32(&maxActive)
			if current <= observed || atomic.CompareAndSwapInt32(&maxActive, observed, current) {
				break
			}
		}
		started <- struct{}{}
		<-release
		atomic.AddInt32(&active, -1)
		return nil
	}

	routerA.RegisterHandler("work", handler)
	routerB.RegisterHandler("work", handler)

	if err := routerA.Dispatch(context.Background(), Task{Type: "work", Options: TaskOptions{GroupKey: "a"}}); err != nil {
		t.Fatalf("dispatch on routerA failed: %v", err)
	}
	if err := routerB.Dispatch(context.Background(), Task{Type: "work", Options: TaskOptions{GroupKey: "b"}}); err != nil {
		t.Fatalf("dispatch on routerB failed: %v", err)
	}

	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected first handler to start")
	}

	select {
	case <-started:
		t.Fatalf("second handler started before limiter slot was released")
	case <-time.After(40 * time.Millisecond):
	}

	close(release)

	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected second handler to start after limiter release")
	}

	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("expected max concurrent handlers to be 1, got %d", got)
	}
}

func TestDispatchDoesNotSerializeUnrelatedGroupsWhenOneGroupBufferIsFull(t *testing.T) {
	cfg := newTestConfig()
	cfg.GroupBuffer = 1
	cfg.CleanupInterval = time.Hour

	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	coolDone := make(chan struct{}, 1)
	router.RegisterHandler("work", func(ctx context.Context, payload any) error {
		if s, _ := payload.(string); s == "cool" {
			coolDone <- struct{}{}
		}
		return nil
	})

	hotGroup := newTestGroupWorker(router, "hot", 1)
	hotGroup.ch <- &enqueuedTask{task: Task{Type: "work", Payload: "prefill"}}

	router.mu.Lock()
	router.groups["hot"] = hotGroup

	hotStarted := make(chan struct{})
	hotResult := make(chan error, 1)
	go func() {
		close(hotStarted)
		hotCtx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		hotResult <- router.Dispatch(hotCtx, Task{
			Type:    "work",
			Payload: "hot",
			Options: TaskOptions{
				GroupKey: "hot",
			},
		})
	}()
	<-hotStarted
	router.mu.Unlock()

	time.Sleep(20 * time.Millisecond)

	coolCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := router.Dispatch(coolCtx, Task{
		Type:    "work",
		Payload: "cool",
		Options: TaskOptions{
			GroupKey: "cool",
		},
	}); err != nil {
		t.Fatalf("expected unrelated group dispatch to proceed while hot group is blocked, got %v", err)
	}

	select {
	case <-coolDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("unrelated group handler did not execute in time")
	}

	if err := <-hotResult; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected hot group dispatch to time out while blocked, got %v", err)
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

	stats := router.Stats()
	if stats.CronDispatchAttempts != 1 {
		t.Fatalf("expected one cron dispatch attempt, got %d", stats.CronDispatchAttempts)
	}
	if stats.CronDispatchSuccess != 0 {
		t.Fatalf("expected zero successful cron dispatches, got %d", stats.CronDispatchSuccess)
	}
	if stats.CronDispatchFailures != 1 {
		t.Fatalf("expected one failed cron dispatch, got %d", stats.CronDispatchFailures)
	}
}

func TestRunCronOnce_TracksDispatchSuccessMetrics(t *testing.T) {
	cfg := newTestConfig()
	cfg.CleanupInterval = time.Hour
	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	var calls int32
	done := make(chan struct{}, 1)
	router.RegisterHandler("cron-ok", func(ctx context.Context, payload any) error {
		atomic.AddInt32(&calls, 1)
		done <- struct{}{}
		return nil
	})

	job := &cronJob{
		Interval: time.Millisecond,
		Task: Task{
			Type: "cron-ok",
		},
	}

	router.cronMu.Lock()
	router.cronJobs = append(router.cronJobs, job)
	router.cronMu.Unlock()

	router.runCronOnce()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected cron handler to run once")
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected cron handler call count to be one, got %d", got)
	}

	stats := router.Stats()
	if stats.CronDispatchAttempts != 1 {
		t.Fatalf("expected one cron dispatch attempt, got %d", stats.CronDispatchAttempts)
	}
	if stats.CronDispatchSuccess != 1 {
		t.Fatalf("expected one successful cron dispatch, got %d", stats.CronDispatchSuccess)
	}
	if stats.CronDispatchFailures != 0 {
		t.Fatalf("expected zero failed cron dispatches, got %d", stats.CronDispatchFailures)
	}
}

func TestCleanupOnceKeepsGroupWithActiveHandler(t *testing.T) {
	cfg := newTestConfig()
	cfg.CleanupInterval = time.Hour
	cfg.GroupIdleTTL = 10 * time.Millisecond

	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	done := make(chan struct{}, 1)

	router.RegisterHandler("busy", func(ctx context.Context, payload any) error {
		started <- struct{}{}
		<-release
		done <- struct{}{}
		return nil
	})

	if err := router.Dispatch(context.Background(), Task{
		Type: "busy",
		Options: TaskOptions{
			GroupKey: "busy",
		},
	}); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("handler did not start in time")
	}

	time.Sleep(2 * cfg.GroupIdleTTL)
	router.cleanupOnce()

	router.mu.RLock()
	gw, ok := router.groups["busy"]
	router.mu.RUnlock()
	if !ok || gw == nil {
		t.Fatalf("expected active group to remain registered during cleanup")
	}
	if got := gw.queueState(); got != queueStateOpen {
		t.Fatalf("expected active group to remain open during cleanup, got %v", got)
	}

	close(release)

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("handler did not finish in time")
	}
}

func TestCleanupOnceClosesIdleGroupAfterHandlerCompletes(t *testing.T) {
	cfg := newTestConfig()
	cfg.CleanupInterval = time.Hour
	cfg.GroupIdleTTL = 10 * time.Millisecond

	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	done := make(chan struct{}, 1)

	router.RegisterHandler("idle", func(ctx context.Context, payload any) error {
		started <- struct{}{}
		<-release
		done <- struct{}{}
		return nil
	})

	if err := router.Dispatch(context.Background(), Task{
		Type: "idle",
		Options: TaskOptions{
			GroupKey: "idle",
		},
	}); err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	select {
	case <-started:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("handler did not start in time")
	}

	close(release)

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("handler did not finish in time")
	}

	router.cleanupOnce()

	router.mu.RLock()
	_, ok := router.groups["idle"]
	router.mu.RUnlock()
	if !ok {
		t.Fatalf("expected group to remain before idle TTL elapses")
	}

	time.Sleep(2 * cfg.GroupIdleTTL)
	router.cleanupOnce()

	router.mu.RLock()
	_, ok = router.groups["idle"]
	router.mu.RUnlock()
	if ok {
		t.Fatalf("expected idle group to be removed after cleanup")
	}
}

func TestCloseWaitsForInFlightSenderWithoutPanicking(t *testing.T) {
	cfg := newTestConfig()
	cfg.CleanupInterval = time.Hour

	router := NewRouter(cfg)

	gw := newTestGroupWorker(router, "close", 1)
	router.mu.Lock()
	router.groups["close"] = gw
	router.mu.Unlock()

	if !gw.tryAcquireSender() {
		t.Fatalf("expected to acquire sender for test group")
	}

	closeDone := make(chan struct{})
	go func() {
		router.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
		t.Fatalf("router close returned before in-flight sender released")
	case <-time.After(20 * time.Millisecond):
	}

	gw.releaseSender()

	select {
	case <-closeDone:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("router close did not finish after sender release")
	}

	if got := gw.queueState(); got != queueStateClosed {
		t.Fatalf("expected group queue to be closed after router shutdown, got %v", got)
	}
}
