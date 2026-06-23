//go:build !legacy
// +build !legacy

package task

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math/rand"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/observability"
)

// TaskHandler represents a state-less processing unit for a single task payload.
// Handlers must be concurrent-safe and respect context cancellation guarantees.
type TaskHandler func(ctx context.Context, payload any) error

// TaskOptions dictates the operational routing parameters for a scheduled task.
type TaskOptions struct {
	// GroupKey enforces sequential execution guarantees across concurrent dispatchers.
	// Tasks sharing an identical group key will never execute in parallel.
	GroupKey string

	// IdempotencyKey prevents duplicate task queuing within the defined TTL window.
	IdempotencyKey string

	// MaxAttempts defines the upper bound for handler execution retries.
	// If 0, the router defaults to RouterConfig.DefaultMaxAttempts.
	MaxAttempts int

	// InitialBackoff configures the baseline delay for exponential retry scaling.
	InitialBackoff time.Duration

	// MaxBackoff establishes the absolute ceiling for the exponential backoff formula.
	MaxBackoff time.Duration

	// IdempotencyTTL determines the survival duration of the idempotency token in memory.
	IdempotencyTTL time.Duration
}

// EmptyPayload serves as a zero-allocation marker for tasks requiring no dynamic context.
type EmptyPayload struct{}

// Task encapsulates the immutable instructions and metadata required for background dispatch.
type Task struct {
	Type    string
	Payload any
	Options TaskOptions
}

// RouterConfig defines the holistic tuning parameters for the task orchestration layer.
type RouterConfig struct {
	DefaultMaxAttempts int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
	IdempotencyTTL     time.Duration
	GroupBuffer        int
	GroupIdleTTL       time.Duration
	CleanupInterval    time.Duration
	GlobalMaxWorkers   int
	GroupMaxParallel   int

	// ExecutionLimiter enables resource sharing across multiple router topologies.
	ExecutionLimiter *ExecutionLimiter

	// Clock abstracts the time package to enable deterministic execution testing.
	Clock clock.Clock

	// Logger receives the injected application logger for structural reporting.
	Logger *slog.Logger
}

// Defaults provisions a base configuration profile suitable for production operations.
func Defaults() RouterConfig {
	return RouterConfig{
		DefaultMaxAttempts: 3,
		InitialBackoff:     1 * time.Second,
		MaxBackoff:         30 * time.Second,
		IdempotencyTTL:     60 * time.Second,
		GroupBuffer:        128,
		GroupIdleTTL:       2 * time.Minute,
		CleanupInterval:    2 * time.Minute,
		GlobalMaxWorkers:   0,
		GroupMaxParallel:   1,
		Clock:              clock.RealClock{},
		Logger:             slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// ExecutionLimiter bounds the aggregate concurrency ceiling via a counting semaphore.
type ExecutionLimiter struct {
	sem chan struct{}
}

// NewExecutionLimiter allocates a fixed-capacity execution bounded semaphore.
// A capacity of 0 or less returns a nil limiter, which signifies unbounded execution.
func NewExecutionLimiter(maxWorkers int) *ExecutionLimiter {
	if maxWorkers <= 0 {
		return nil
	}
	return &ExecutionLimiter{
		sem: make(chan struct{}, maxWorkers),
	}
}

// Acquire blocks until a concurrency token becomes available within the semaphore.
func (l *ExecutionLimiter) Acquire() {
	if l == nil || l.sem == nil {
		return
	}
	// Blocks until capacity is freed.
	l.sem <- struct{}{}
}

// Release yields a concurrency token back to the semaphore pool.
func (l *ExecutionLimiter) Release() {
	if l == nil || l.sem == nil {
		return
	}
	select {
	case <-l.sem:
	default:
		// Evades panic on over-release, which indicates a severe architectural failure but shouldn't crash the loop.
	}
}

// Capacity interrogates the upper bound of the concurrency semaphore.
func (l *ExecutionLimiter) Capacity() int {
	if l == nil || l.sem == nil {
		return 0
	}
	return cap(l.sem)
}

// Operational errors for routing boundaries.
var (
	ErrRouterClosed    = errors.New("task router is closed")
	ErrUnknownTaskType = errors.New("unknown task type")
	ErrDuplicateTask   = errors.New("duplicate task (idempotency key present)")
	ErrRetrySilent     = errors.New("retryable task error (silent)")
	errTaskEnqueue     = errors.New("task enqueue failed")
)

const (
	globalGroup         = "_global"
	maxEnqueueAttempts  = 3
	retryRescheduleWait = 50 * time.Millisecond
)

type groupSendResult uint8

const (
	groupSendEnqueued groupSendResult = iota
	groupSendClosed
	groupSendContextDone
	groupSendRouterClosed
	groupSendWouldBlock
)

type queueState uint32

const (
	queueStateOpen queueState = iota
	queueStateStopping
	queueStateClosed
)

// TaskRouter orchestrates background execution scheduling, concurrency boundaries, and idempotency states.
type TaskRouter struct {
	mu        sync.Mutex
	handlers  map[string]TaskHandler
	groups    map[string]*groupWorker
	inflight  map[string]time.Time
	closed    bool
	cfg       RouterConfig
	startedAt time.Time
	wg        sync.WaitGroup
	stopOnce  sync.Once
	stopCh    chan struct{}

	// ctx dictates the holistic lifecycle boundary passed into active task handlers.
	ctx       context.Context
	cancel    context.CancelFunc
	randMutex sync.Mutex

	execLimiter *ExecutionLimiter

	retryMu     sync.Mutex
	retryQueue  retryTaskHeap
	retryWakeCh chan struct{}
	retrySeq    uint64

	cronMu   sync.Mutex
	cronJobs []*cronJob

	cronDispatchAttempts int64
	cronDispatchSuccess  int64
	cronDispatchFailures int64

	latencyMu       sync.Mutex
	latenciesByType map[string]*observability.Summary
}

type groupWorker struct {
	key          string
	ch           chan *enqueuedTask
	state        atomic.Uint32
	senders      atomic.Int32
	closeMu      sync.Mutex
	closeCond    *sync.Cond
	lastActiveNs atomic.Int64
	activeCount  atomic.Int32
}

type enqueuedTask struct {
	task    Task
	attempt int
}

type scheduledRetry struct {
	at       time.Time
	groupKey string
	task     *enqueuedTask
	seq      uint64
	index    int
}

type retryTaskHeap []*scheduledRetry

// Len reports the total elements within the retry heap.
func (h retryTaskHeap) Len() int {
	return len(h)
}

// Less ensures chronological priority, falling back to sequence insertion ordering.
func (h retryTaskHeap) Less(i, j int) bool {
	if h[i].at.Equal(h[j].at) {
		return h[i].seq < h[j].seq
	}
	return h[i].at.Before(h[j].at)
}

// Swap transposes two elements within the heap to maintain algorithmic invariants.
func (h retryTaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

// Push allocates a scheduled retry entity onto the trailing tail of the slice.
func (h *retryTaskHeap) Push(x any) {
	item := x.(*scheduledRetry)
	item.index = len(*h)
	*h = append(*h, item)
}

// Pop truncates and returns the lowest priority element from the slice tail.
func (h *retryTaskHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	item.index = -1
	*h = old[:n-1]
	return item
}

type cronJob struct {
	Interval time.Duration
	Task     Task
	lastRun  time.Time
	stopped  bool
}

// NewRouter constructs an isolated task dispatch infrastructure mapping config defaults.
func NewRouter(cfg RouterConfig) *TaskRouter {
	def := Defaults()
	if cfg.DefaultMaxAttempts <= 0 {
		cfg.DefaultMaxAttempts = def.DefaultMaxAttempts
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = def.InitialBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = def.MaxBackoff
	}
	if cfg.IdempotencyTTL <= 0 {
		cfg.IdempotencyTTL = def.IdempotencyTTL
	}
	if cfg.GroupBuffer <= 0 {
		cfg.GroupBuffer = def.GroupBuffer
	}
	if cfg.GroupIdleTTL <= 0 {
		cfg.GroupIdleTTL = def.GroupIdleTTL
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = def.CleanupInterval
	}
	if cfg.GroupMaxParallel <= 0 {
		cfg.GroupMaxParallel = def.GroupMaxParallel
	}
	if cfg.Clock == nil {
		cfg.Clock = def.Clock
	}
	if cfg.Logger == nil {
		cfg.Logger = def.Logger
	}

	tr := &TaskRouter{
		handlers:    make(map[string]TaskHandler),
		groups:      make(map[string]*groupWorker),
		inflight:    make(map[string]time.Time),
		cfg:         cfg,
		startedAt:   cfg.Clock.Now(),
		stopCh:      make(chan struct{}),
		retryWakeCh: make(chan struct{}, 1),
	}
	tr.ctx, tr.cancel = context.WithCancel(context.Background())
	if cfg.ExecutionLimiter != nil {
		tr.execLimiter = cfg.ExecutionLimiter
	} else if cfg.GlobalMaxWorkers > 0 {
		tr.execLimiter = NewExecutionLimiter(cfg.GlobalMaxWorkers)
	}

	tr.wg.Add(1)
	go tr.backgroundLoop()
	tr.wg.Add(1)
	go tr.retryLoop()
	tr.cfg.Logger.Info("TaskRouter initialized")
	return tr
}

// RegisterHandler binds an execution callback directly to a string payload type boundary.
func (tr *TaskRouter) RegisterHandler(taskType string, handler TaskHandler) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.handlers[taskType] = handler
}

// Dispatch queues an arbitrary payload to the routing engine.
// Emits ErrDuplicateTask synchronously if idempotency bounds are violated.
func (tr *TaskRouter) Dispatch(ctx context.Context, t Task) error {
	groupKey, eff, err := tr.prepareDispatch(t)
	if err != nil {
		return fmt.Errorf("TaskRouter.Dispatch: %w", err)
	}

	enq := &enqueuedTask{task: t, attempt: 1}
	for i := 0; i < maxEnqueueAttempts; i++ {
		gw, ok := tr.getOrCreateGroup(groupKey)
		if !ok || gw == nil {
			tr.rollbackIdempotencyReservation(eff)
			return ErrRouterClosed
		}

		switch tr.sendToGroupContext(ctx, gw, enq) {
		case groupSendEnqueued:
			return nil
		case groupSendContextDone:
			tr.rollbackIdempotencyReservation(eff)
			if ctx == nil {
				return context.Canceled
			}
			return ctx.Err()
		case groupSendRouterClosed:
			tr.rollbackIdempotencyReservation(eff)
			return ErrRouterClosed
		case groupSendClosed:
			// Group was shutting down while attempting dispatch, cycle loop to instantiate cleanly.
			tr.dropStaleGroup(groupKey, gw)
		}
	}

	tr.rollbackIdempotencyReservation(eff)
	return errTaskEnqueue
}

func (tr *TaskRouter) prepareDispatch(t Task) (string, TaskOptions, error) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.closed {
		return "", TaskOptions{}, ErrRouterClosed
	}

	handler, ok := tr.handlers[t.Type]
	if !ok || handler == nil {
		return "", TaskOptions{}, ErrUnknownTaskType
	}

	eff := tr.effectiveOptions(t.Options)

	if eff.IdempotencyKey != "" {
		if expiry, exists := tr.inflight[eff.IdempotencyKey]; exists && tr.cfg.Clock.Now().Before(expiry) {
			return "", TaskOptions{}, ErrDuplicateTask
		}
		tr.inflight[eff.IdempotencyKey] = tr.cfg.Clock.Now().Add(eff.IdempotencyTTL)
	}

	groupKey := eff.GroupKey
	if groupKey == "" {
		groupKey = globalGroup
	}

	return groupKey, eff, nil
}

// Close gracefully halts all pending orchestration and unblocks all background workers.
// Any execution routines in-flight will be hard-canceled via context propagation.
func (tr *TaskRouter) Close() {
	tr.stopOnce.Do(func() {
		tr.cfg.Logger.Info("TaskRouter shutting down")
		tr.mu.Lock()
		tr.closed = true
		groups := make([]*groupWorker, 0, len(tr.groups))
		for _, gw := range tr.groups {
			if gw == nil {
				continue
			}
			if gw.queueState() == queueStateOpen {
				gw.beginStop()
			}
			groups = append(groups, gw)
		}
		// Discard memory structures to halt map accesses proactively
		clear(tr.groups)
		clear(tr.inflight)
		clear(tr.handlers)
		tr.mu.Unlock()

		close(tr.stopCh)
		if tr.cancel != nil {
			tr.cancel()
		}
		for _, gw := range groups {
			gw.finishStop()
		}
		tr.wg.Wait()
	})
}

// Stats encapsulates operational metrics intended for telemetry scraping.
type Stats struct {
	GroupsCount          int
	InflightCount        int
	RouterClosed         bool
	RegisteredTypes      int
	CronDispatchAttempts int64
	CronDispatchSuccess  int64
	CronDispatchFailures int64
}

// Stats aggregates immediate internal counters inside a thread-safe read envelope.
func (tr *TaskRouter) Stats() Stats {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return Stats{
		GroupsCount:          len(tr.groups),
		InflightCount:        len(tr.inflight),
		RouterClosed:         tr.closed,
		RegisteredTypes:      len(tr.handlers),
		CronDispatchAttempts: atomic.LoadInt64(&tr.cronDispatchAttempts),
		CronDispatchSuccess:  atomic.LoadInt64(&tr.cronDispatchSuccess),
		CronDispatchFailures: atomic.LoadInt64(&tr.cronDispatchFailures),
	}
}

// ScheduleEvery establishes an interval repetition loop that enqueues the provided task.
func (tr *TaskRouter) ScheduleEvery(interval time.Duration, t Task) func() {
	job := &cronJob{
		Interval: interval,
		Task:     t,
		lastRun:  time.Time{},
		stopped:  false,
	}
	tr.cronMu.Lock()
	tr.cronJobs = append(tr.cronJobs, job)
	idx := len(tr.cronJobs) - 1
	tr.cronMu.Unlock()

	cancel := func() {
		tr.cronMu.Lock()
		if idx >= 0 && idx < len(tr.cronJobs) && tr.cronJobs[idx] == job {
			tr.cronJobs[idx] = nil
		}
		job.stopped = true
		tr.cronMu.Unlock()
	}
	return cancel
}

// Cancel models the tear-down execution routine for scheduled jobs.
type Cancel func()

// ScheduleDailyAtUTC registers a recurring periodic submission locking onto an absolute UTC clock boundary.
func (tr *TaskRouter) ScheduleDailyAtUTC(hour, minute int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(1, hour, minute, 0, t)
}

// ScheduleDailyAtUTCWithSeconds anchors a 24-hour job repeating down to the exact second precision.
func (tr *TaskRouter) ScheduleDailyAtUTCWithSeconds(hour, minute, second int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(1, hour, minute, second, t)
}

// ScheduleEveryNDaysAtUTC schedules across an N day period omitting seconds.
func (tr *TaskRouter) ScheduleEveryNDaysAtUTC(n int, hour, minute int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(n, hour, minute, 0, t)
}

// ScheduleEveryNDaysAtUTCWithSeconds computes a localized UTC offset boundary for execution bridging multiple days.
func (tr *TaskRouter) ScheduleEveryNDaysAtUTCWithSeconds(n int, hour, minute, second int, t Task) Cancel {
	if n <= 0 {
		n = 1
	}
	hour = clampInt(hour, 0, 23)
	minute = clampInt(minute, 0, 59)
	second = clampInt(second, 0, 59)

	interval := time.Duration(n) * 24 * time.Hour

	now := tr.cfg.Clock.Now().UTC()
	target := nextUTCTimestamp(now, hour, minute, second)
	if !now.Before(target) {
		target = target.Add(interval)
	}

	// Pre-date the lastRun so that when 'now' reaches 'target', exactly 'interval' has passed.
	lastRun := target.Add(-interval)

	job := &cronJob{
		Interval: interval,
		Task:     t,
		lastRun:  lastRun,
		stopped:  false,
	}

	tr.cronMu.Lock()
	tr.cronJobs = append(tr.cronJobs, job)
	idx := len(tr.cronJobs) - 1
	tr.cronMu.Unlock()

	cancel := func() {
		tr.cronMu.Lock()
		if idx >= 0 && idx < len(tr.cronJobs) && tr.cronJobs[idx] == job {
			tr.cronJobs[idx] = nil
		}
		job.stopped = true
		tr.cronMu.Unlock()
	}

	return cancel
}

func nextUTCTimestamp(from time.Time, hour, minute, second int) time.Time {
	return time.Date(from.Year(), from.Month(), from.Day(), hour, minute, second, 0, time.UTC)
}

func clampInt(v, lo, hi int) int {
	return max(min(v, hi), lo)
}

func (tr *TaskRouter) effectiveOptions(opt TaskOptions) TaskOptions {
	if opt.MaxAttempts <= 0 {
		opt.MaxAttempts = tr.cfg.DefaultMaxAttempts
	}
	if opt.InitialBackoff <= 0 {
		opt.InitialBackoff = tr.cfg.InitialBackoff
	}
	if opt.MaxBackoff <= 0 {
		opt.MaxBackoff = tr.cfg.MaxBackoff
	}
	if opt.IdempotencyTTL <= 0 {
		opt.IdempotencyTTL = tr.cfg.IdempotencyTTL
	}
	return opt
}

func (tr *TaskRouter) effectiveGroupParallel() int {
	if tr.cfg.GroupMaxParallel <= 0 {
		return 1
	}
	return tr.cfg.GroupMaxParallel
}

func newGroupWorker(key string, buffer int, nowNs int64) *groupWorker {
	gw := &groupWorker{
		key: key,
		ch:  make(chan *enqueuedTask, buffer),
	}
	gw.closeCond = sync.NewCond(&gw.closeMu)
	gw.state.Store(uint32(queueStateOpen))
	gw.markActive(nowNs)
	return gw
}

func (tr *TaskRouter) nowNs() int64 {
	return tr.cfg.Clock.Now().Sub(tr.startedAt).Nanoseconds() + 1
}

func (gw *groupWorker) queueState() queueState {
	if gw == nil {
		return queueStateClosed
	}
	return queueState(gw.state.Load())
}

func (gw *groupWorker) markActive(nowNs int64) {
	gw.lastActiveNs.Store(nowNs)
}

func (gw *groupWorker) beginWork(nowNs int64) {
	gw.markActive(nowNs)
	gw.activeCount.Add(1)
}

func (gw *groupWorker) endWork(nowNs int64) {
	gw.markActive(nowNs)
	gw.activeCount.Add(-1)
}

func (gw *groupWorker) idleFor(nowNs int64) time.Duration {
	lastActiveNs := gw.lastActiveNs.Load()
	if lastActiveNs <= 0 || nowNs <= lastActiveNs {
		return 0
	}
	return time.Duration(nowNs - lastActiveNs)
}

func (gw *groupWorker) tryAcquireSender() bool {
	if gw == nil {
		return false
	}
	for {
		if gw.queueState() != queueStateOpen {
			return false
		}
		current := gw.senders.Load()
		if !gw.senders.CompareAndSwap(current, current+1) {
			continue
		}
		if gw.queueState() == queueStateOpen {
			return true
		}
		gw.releaseSender()
		return false
	}
}

func (gw *groupWorker) releaseSender() {
	if gw == nil {
		return
	}
	if gw.senders.Add(-1) == 0 && gw.queueState() != queueStateOpen {
		gw.closeMu.Lock()
		if gw.closeCond != nil {
			gw.closeCond.Broadcast()
		}
		gw.closeMu.Unlock()
	}
}

func (gw *groupWorker) beginStop() bool {
	if gw == nil {
		return false
	}
	return gw.state.CompareAndSwap(uint32(queueStateOpen), uint32(queueStateStopping))
}

func (gw *groupWorker) finishStop() {
	if gw == nil {
		return
	}
	// Blocks aggressively waiting for all pending atomic senders to drain to 0
	gw.closeMu.Lock()
	for gw.senders.Load() > 0 {
		gw.closeCond.Wait()
	}
	if gw.queueState() == queueStateStopping {
		close(gw.ch)
		gw.state.Store(uint32(queueStateClosed))
	}
	if gw.closeCond != nil {
		gw.closeCond.Broadcast()
	}
	gw.closeMu.Unlock()
}

func (tr *TaskRouter) ensureGroupLocked(key string) *groupWorker {
	if gw, ok := tr.groups[key]; ok && gw != nil {
		return gw
	}
	gw := newGroupWorker(key, tr.cfg.GroupBuffer, tr.nowNs())
	tr.groups[key] = gw
	parallel := tr.effectiveGroupParallel()
	for i := 0; i < parallel; i++ {
		tr.wg.Add(1)
		go tr.groupLoop(gw)
	}
	return gw
}

func (tr *TaskRouter) acquireExecSlot() {
	tr.execLimiter.Acquire()
}

func (tr *TaskRouter) releaseExecSlot() {
	tr.execLimiter.Release()
}

func (tr *TaskRouter) groupLoop(gw *groupWorker) {
	defer tr.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			tr.cfg.Logger.Error("Worker runtime panic pre-empted", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	for {
		select {
		case <-tr.ctx.Done():
			tr.cfg.Logger.Warn("Context cancelled: abandoning unread queue", "group", gw.key)
			return
		case enq, ok := <-gw.ch:
			if !ok {
				return
			}
			gw.beginWork(tr.nowNs())

			tr.mu.Lock()
			handler := tr.handlers[enq.task.Type]
			eff := tr.effectiveOptions(enq.task.Options)
			tr.mu.Unlock()

			if handler == nil {
				gw.endWork(tr.nowNs())
				tr.cfg.Logger.Warn("Task dropped (handler not registered)", "type", enq.task.Type, "group", gw.key)
				continue
			}

			// Implements execution slot limitation across all topological boundaries to prevent host saturation.
			tr.acquireExecSlot()
			startExec := tr.cfg.Clock.Now()
			err := func() error {
				defer tr.releaseExecSlot()
				ctx := tr.ctx
				if ctx == nil {
					ctx = context.Background()
				}
				return handler(ctx, enq.task.Payload)
			}()
			execDuration := tr.cfg.Clock.Now().Sub(startExec)

			summary := observability.GetOrCreateLabeledSummary(&tr.latencyMu, &tr.latenciesByType, enq.task.Type)
			summary.Observe(execDuration)

			if execDuration > 5*time.Second {
				tr.cfg.Logger.Warn("slow background task execution",
					"type", enq.task.Type,
					"duration", execDuration.String(),
					"duration_ms", execDuration.Milliseconds(),
				)
			}
			gw.endWork(tr.nowNs())

			if err != nil {
				silent := errors.Is(err, ErrRetrySilent)
				if enq.attempt < eff.MaxAttempts {
					delay := tr.computeBackoff(eff.InitialBackoff, eff.MaxBackoff, enq.attempt)
					attempt := enq.attempt + 1

					if silent {
						tr.cfg.Logger.Debug("Task failed, scheduling retry",
							"type", enq.task.Type,
							"group", gw.key,
							"attempt", attempt,
							"max_attempts", eff.MaxAttempts,
							"backoff", delay.String(),
							"err", err,
						)
					} else {
						tr.cfg.Logger.Warn("Task failed, scheduling retry",
							"type", enq.task.Type,
							"group", gw.key,
							"attempt", attempt,
							"max_attempts", eff.MaxAttempts,
							"backoff", delay.String(),
							"err", err,
						)
					}

					enq.attempt = attempt
					tr.scheduleRetry(gw.key, enq, delay)
					continue
				}

				if silent {
					tr.cfg.Logger.Info("Task dropped after retry window",
						"type", enq.task.Type,
						"group", gw.key,
						"attempts", enq.attempt,
						"err", err,
					)
				} else {
					tr.cfg.Logger.Error("Task dropped after retry window",
						"type", enq.task.Type,
						"group", gw.key,
						"attempts", enq.attempt,
						"err", err,
					)
				}
			}
		}
	}
}

func (tr *TaskRouter) enqueueRetry(groupKey string, et *enqueuedTask) bool {
	return tr.tryEnqueueRetry(groupKey, et) == groupSendEnqueued
}

func (tr *TaskRouter) scheduleRetry(groupKey string, et *enqueuedTask, delay time.Duration) {
	if groupKey == "" || et == nil {
		return
	}

	item := &scheduledRetry{
		at:       tr.cfg.Clock.Now().Add(delay),
		groupKey: groupKey,
		task:     et,
	}

	tr.retryMu.Lock()
	tr.retrySeq++
	item.seq = tr.retrySeq
	heap.Push(&tr.retryQueue, item)
	tr.retryMu.Unlock()
	tr.signalRetryLoop()
}

func (tr *TaskRouter) signalRetryLoop() {
	select {
	case tr.retryWakeCh <- struct{}{}:
	default:
	}
}

func (tr *TaskRouter) retryLoop() {
	defer tr.wg.Done()

	for {
		delay, ok := tr.nextRetryDelay()
		if !ok {
			select {
			case <-tr.stopCh:
				return
			case <-tr.retryWakeCh:
				continue
			}
		}

		if delay > 0 {
			timer := tr.cfg.Clock.NewTimer(delay)
			select {
			case <-tr.stopCh:
				if !timer.Stop() {
					select {
					case <-timer.C():
					default:
					}
				}
				return
			case <-tr.retryWakeCh:
				if !timer.Stop() {
					select {
					case <-timer.C():
					default:
					}
				}
				continue
			case <-timer.C():
			}
		}

		for _, item := range tr.popDueRetries(tr.cfg.Clock.Now()) {
			switch tr.tryEnqueueRetry(item.groupKey, item.task) {
			case groupSendEnqueued:
				continue
			case groupSendWouldBlock:
				tr.scheduleRetry(item.groupKey, item.task, retryRescheduleWait)
			case groupSendRouterClosed:
				return
			default:
				tr.cfg.Logger.Debug("Task retry dropped while enqueuing",
					"type", item.task.task.Type,
					"group", item.groupKey,
					"attempt", item.task.attempt,
				)
			}
		}
	}
}

func (tr *TaskRouter) nextRetryDelay() (time.Duration, bool) {
	tr.retryMu.Lock()
	defer tr.retryMu.Unlock()

	if len(tr.retryQueue) == 0 {
		return 0, false
	}

	delay := tr.retryQueue[0].at.Sub(tr.cfg.Clock.Now())
	if delay < 0 {
		delay = 0
	}
	return delay, true
}

func (tr *TaskRouter) popDueRetries(now time.Time) []*scheduledRetry {
	tr.retryMu.Lock()
	defer tr.retryMu.Unlock()

	var due []*scheduledRetry
	for len(tr.retryQueue) > 0 {
		next := tr.retryQueue[0]
		if next == nil || next.at.After(now) {
			break
		}
		due = append(due, heap.Pop(&tr.retryQueue).(*scheduledRetry))
	}
	return due
}

func (tr *TaskRouter) tryEnqueueRetry(groupKey string, et *enqueuedTask) groupSendResult {
	if groupKey == "" || et == nil {
		return groupSendClosed
	}

	for i := 0; i < maxEnqueueAttempts; i++ {
		select {
		case <-tr.stopCh:
			return groupSendRouterClosed
		default:
		}

		gw, ok := tr.getOrCreateGroup(groupKey)
		if !ok || gw == nil {
			return groupSendRouterClosed
		}

		switch tr.sendToGroupTry(gw, et) {
		case groupSendEnqueued:
			return groupSendEnqueued
		case groupSendWouldBlock:
			return groupSendWouldBlock
		case groupSendRouterClosed:
			return groupSendRouterClosed
		}

		tr.dropStaleGroup(groupKey, gw)
	}

	return groupSendClosed
}

func (tr *TaskRouter) getOrCreateGroup(groupKey string) (*groupWorker, bool) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.closed {
		return nil, false
	}

	if gw, exists := tr.groups[groupKey]; exists && gw != nil {
		if gw.queueState() != queueStateOpen {
			delete(tr.groups, groupKey)
		} else {
			return gw, true
		}
	}

	return tr.ensureGroupLocked(groupKey), true
}

func (tr *TaskRouter) sendToGroupTry(gw *groupWorker, et *enqueuedTask) (result groupSendResult) {
	if gw == nil || gw.ch == nil || et == nil {
		return groupSendClosed
	}
	if !gw.tryAcquireSender() {
		return groupSendClosed
	}
	defer gw.releaseSender()

	select {
	case gw.ch <- et:
		return groupSendEnqueued
	case <-tr.stopCh:
		return groupSendRouterClosed
	default:
		return groupSendWouldBlock
	}
}

func (tr *TaskRouter) sendToGroupContext(ctx context.Context, gw *groupWorker, et *enqueuedTask) (result groupSendResult) {
	if gw == nil || gw.ch == nil || et == nil {
		return groupSendClosed
	}
	if !gw.tryAcquireSender() {
		return groupSendClosed
	}
	defer gw.releaseSender()

	var ctxDone <-chan struct{}
	if ctx != nil {
		ctxDone = ctx.Done()
	}

	select {
	case gw.ch <- et:
		return groupSendEnqueued
	case <-ctxDone:
		return groupSendContextDone
	case <-tr.stopCh:
		return groupSendRouterClosed
	}
}

func (tr *TaskRouter) dropStaleGroup(groupKey string, gw *groupWorker) {
	tr.mu.Lock()
	if current, exists := tr.groups[groupKey]; exists && current == gw {
		delete(tr.groups, groupKey)
	}
	tr.mu.Unlock()
}

func (tr *TaskRouter) rollbackIdempotencyReservation(eff TaskOptions) {
	if eff.IdempotencyKey == "" {
		return
	}
	tr.mu.Lock()
	delete(tr.inflight, eff.IdempotencyKey)
	tr.mu.Unlock()
}

func (tr *TaskRouter) computeBackoff(initial, max time.Duration, attempt int) time.Duration {
	backoff := initial
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > max {
			backoff = max
			break
		}
	}
	jitter := tr.jitter(backoff, 0.1)
	return clampDuration(backoff+jitter, initial, max)
}

func (tr *TaskRouter) jitter(d time.Duration, ratio float64) time.Duration {
	if ratio <= 0 {
		return 0
	}
	tr.randMutex.Lock()
	defer tr.randMutex.Unlock()
	delta := int64(float64(d) * ratio)
	if delta <= 0 {
		return 0
	}
	n := rand.Int63n(2*delta+1) - delta
	return time.Duration(n)
}

func clampDuration(v, lo, hi time.Duration) time.Duration {
	return max(min(v, hi), lo)
}

func (tr *TaskRouter) backgroundLoop() {
	defer tr.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			tr.cfg.Logger.Error("TaskRouter background loop panic caught", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tr.wg.Add(1)
	go func() {
		defer tr.wg.Done()
		select {
		case <-tr.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	ticker := tr.cfg.Clock.NewTicker(tr.cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C():
			tr.cleanupOnce()
			tr.runCronOnce()
		}
	}
}

func (tr *TaskRouter) cleanupOnce() {
	now := tr.cfg.Clock.Now()
	nowNs := tr.nowNs()
	toClose := make([]*groupWorker, 0)

	tr.mu.Lock()
	maps.DeleteFunc(tr.inflight, func(_ string, expiry time.Time) bool {
		return now.After(expiry)
	})

	for key, gw := range tr.groups {
		if gw == nil {
			delete(tr.groups, key)
			continue
		}
		if gw.queueState() != queueStateOpen {
			delete(tr.groups, key)
			toClose = append(toClose, gw)
			continue
		}
		if gw.activeCount.Load() == 0 && gw.senders.Load() == 0 && len(gw.ch) == 0 && gw.idleFor(nowNs) >= tr.cfg.GroupIdleTTL {
			gw.beginStop()
			delete(tr.groups, key)
			toClose = append(toClose, gw)
		}
	}
	tr.mu.Unlock()
	for _, gw := range toClose {
		gw.finishStop()
	}
}

func (tr *TaskRouter) runCronOnce() {
	now := tr.cfg.Clock.Now()
	tr.cronMu.Lock()
	for _, job := range tr.cronJobs {
		if job == nil || job.stopped {
			continue
		}
		if job.lastRun.IsZero() || now.Sub(job.lastRun) >= job.Interval {
			atomic.AddInt64(&tr.cronDispatchAttempts, 1)
			if err := tr.Dispatch(context.Background(), job.Task); err != nil {
				atomic.AddInt64(&tr.cronDispatchFailures, 1)
				tr.cfg.Logger.Error(
					"Cron task dispatch failed",
					"operation", "task.router.cron.dispatch",
					"taskType", job.Task.Type,
					"interval", job.Interval.String(),
					"group", job.Task.Options.GroupKey,
					"idempotencyKey", job.Task.Options.IdempotencyKey,
					"err", err,
				)
			} else {
				atomic.AddInt64(&tr.cronDispatchSuccess, 1)
			}
			job.lastRun = now
		}
	}
	last := len(tr.cronJobs)
	for last > 0 && tr.cronJobs[last-1] == nil {
		last--
	}
	if last != len(tr.cronJobs) {
		tr.cronJobs = slices.Clip(tr.cronJobs[:last])
	}
	tr.cronMu.Unlock()
}
