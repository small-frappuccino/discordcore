package task

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// TaskHandler is a function that processes a task payload.
type TaskHandler func(ctx context.Context, payload any) error

// TaskOptions configures how a task should be dispatched and executed.
type TaskOptions struct {
	// GroupKey ensures serialized execution for tasks that share the same group.
	// Use this to guarantee order per guild/user/etc. If empty, tasks use a global group.
	GroupKey string

	// IdempotencyKey deduplicates tasks enqueued within the IdempotencyTTL window.
	// If a task with the same key is already in-flight or recently enqueued, it will not be enqueued again.
	IdempotencyKey string

	// MaxAttempts controls how many times the task may be retried on handler error.
	// If 0, router uses RouterConfig.DefaultMaxAttempts.
	MaxAttempts int

	// InitialBackoff sets the initial backoff used for retries. If 0, router uses RouterConfig.InitialBackoff.
	InitialBackoff time.Duration

	// MaxBackoff caps the exponential backoff. If 0, router uses RouterConfig.MaxBackoff.
	MaxBackoff time.Duration

	// IdempotencyTTL controls how long the idempotency key is kept for deduplication.
	// If 0, router uses RouterConfig.IdempotencyTTL.
	IdempotencyTTL time.Duration
}

// Task encapsulates the work to be executed by the router.
type Task struct {
	Type    string
	Payload any
	Options TaskOptions
}

// RouterConfig configures the TaskRouter behavior.
type RouterConfig struct {
	// DefaultMaxAttempts applies when TaskOptions.MaxAttempts is 0.
	DefaultMaxAttempts int

	// InitialBackoff applies when TaskOptions.InitialBackoff is 0.
	InitialBackoff time.Duration

	// MaxBackoff applies when TaskOptions.MaxBackoff is 0.
	MaxBackoff time.Duration

	// IdempotencyTTL applies when TaskOptions.IdempotencyTTL is 0.
	IdempotencyTTL time.Duration

	// GroupBuffer controls the buffered channel size for each group worker.
	GroupBuffer int

	// GroupIdleTTL after which an idle group worker will be stopped (when no tasks).
	GroupIdleTTL time.Duration

	// CleanupInterval controls how often background cleanup runs (idle groups, idempotency keys).
	CleanupInterval time.Duration

	// GlobalMaxWorkers limits how many handler executions can run at the same time across all groups.
	// 0 or less means unlimited.
	GlobalMaxWorkers int

	// GroupMaxParallel limits how many workers a single group has.
	// Minimum is 1 (default), which preserves serialized execution per group.
	GroupMaxParallel int
}

// Defaults returns a RouterConfig with sensible defaults.
func Defaults() RouterConfig {
	return RouterConfig{
		DefaultMaxAttempts: 3,
		InitialBackoff:     1 * time.Second,
		MaxBackoff:         30 * time.Second,
		IdempotencyTTL:     60 * time.Second,
		GroupBuffer:        128,
		GroupIdleTTL:       2 * time.Minute,
		CleanupInterval:    2 * time.Minute,
		GlobalMaxWorkers:   0, // unlimited by default
		GroupMaxParallel:   1, // serialized per group by default
	}
}

// Errors returned by the router.
var (
	ErrRouterClosed    = errors.New("task router is closed")
	ErrUnknownTaskType = errors.New("unknown task type")
	ErrDuplicateTask   = errors.New("duplicate task (idempotency key present)")
)

const globalGroup = "_global"

// TaskRouter is a minimal in-memory dispatcher with per-group serialization,
// idempotency (dedupe), and retry with exponential backoff.
type TaskRouter struct {
	mu        sync.RWMutex
	handlers  map[string]TaskHandler
	groups    map[string]*groupWorker
	inflight  map[string]time.Time // idempotencyKey -> expiry
	closed    bool
	cfg       RouterConfig
	wg        sync.WaitGroup
	stopOnce  sync.Once
	stopCh    chan struct{}
	randMutex sync.Mutex // for jitter RNG

	// Global concurrency semaphore; nil when unlimited.
	execSem chan struct{}

	// Simple cron scheduler
	cronMu   sync.Mutex
	cronJobs []*cronJob
}

type groupWorker struct {
	key        string
	ch         chan *enqueuedTask
	lastActive time.Time
	stopping   bool
}

type enqueuedTask struct {
	task    Task
	attempt int
}

type cronJob struct {
	Interval time.Duration
	Task     Task
	lastRun  time.Time
	stopped  bool
}

// NewRouter creates a new TaskRouter with the provided configuration.
func NewRouter(cfg RouterConfig) *TaskRouter {
	// Fill defaults for zero-values
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

	tr := &TaskRouter{
		handlers: make(map[string]TaskHandler),
		groups:   make(map[string]*groupWorker),
		inflight: make(map[string]time.Time),
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}
	// Initialize global semaphore if needed
	if cfg.GlobalMaxWorkers > 0 {
		tr.execSem = make(chan struct{}, cfg.GlobalMaxWorkers)
	}

	tr.wg.Add(1)
	go tr.backgroundLoop()
	return tr
}

// RegisterHandler registers a handler for the given task type.
func (tr *TaskRouter) RegisterHandler(taskType string, handler TaskHandler) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.handlers[taskType] = handler
}

// Dispatch enqueues a task for execution, respecting grouping and idempotency.
// Returns ErrUnknownTaskType if no handler is registered.
// Returns ErrDuplicateTask when a non-expired IdempotencyKey already exists.
func (tr *TaskRouter) Dispatch(ctx context.Context, t Task) error {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if tr.closed {
		return ErrRouterClosed
	}

	handler, ok := tr.handlers[t.Type]
	if !ok || handler == nil {
		return ErrUnknownTaskType
	}

	// Resolve effective options
	eff := tr.effectiveOptions(t.Options)

	// Idempotency: reject duplicates within TTL window
	if eff.IdempotencyKey != "" {
		if expiry, exists := tr.inflight[eff.IdempotencyKey]; exists && time.Now().Before(expiry) {
			return ErrDuplicateTask
		}
		tr.inflight[eff.IdempotencyKey] = time.Now().Add(eff.IdempotencyTTL)
	}

	// Ensure group worker
	groupKey := eff.GroupKey
	if groupKey == "" {
		groupKey = globalGroup
	}
	gw := tr.ensureGroupLocked(groupKey)

	// Enqueue
	enq := &enqueuedTask{task: t, attempt: 1}
	select {
	case gw.ch <- enq:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close gracefully stops the router, waits for background goroutines to exit.
// Enqueued tasks that are not yet picked up may be dropped.
func (tr *TaskRouter) Close() {
	tr.stopOnce.Do(func() {
		tr.mu.Lock()
		tr.closed = true
		for _, gw := range tr.groups {
			if gw != nil && !gw.stopping {
				gw.stopping = true
				close(gw.ch)
			}
		}
		tr.mu.Unlock()
		close(tr.stopCh)
		tr.wg.Wait()
	})
}

// Stats provides a snapshot with counts useful for debugging/monitoring.
type Stats struct {
	GroupsCount     int
	InflightCount   int
	RouterClosed    bool
	RegisteredTypes int
}

func (tr *TaskRouter) Stats() Stats {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return Stats{
		GroupsCount:     len(tr.groups),
		InflightCount:   len(tr.inflight),
		RouterClosed:    tr.closed,
		RegisteredTypes: len(tr.handlers),
	}
}

// ScheduleEvery registers a simple periodic job that dispatches the given task
// at the specified interval. Returns a cancel function.
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

// --- Internals ---

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

func (tr *TaskRouter) ensureGroupLocked(key string) *groupWorker {
	if gw, ok := tr.groups[key]; ok && gw != nil {
		return gw
	}
	gw := &groupWorker{
		key:        key,
		ch:         make(chan *enqueuedTask, tr.cfg.GroupBuffer),
		lastActive: time.Now(),
	}
	tr.groups[key] = gw
	// Spawn up to GroupMaxParallel workers for this group
	parallel := tr.effectiveGroupParallel()
	for range parallel {
		tr.wg.Add(1)
		go tr.groupLoop(gw)
	}
	return gw
}

func (tr *TaskRouter) acquireExecSlot() {
	if tr.execSem != nil {
		tr.execSem <- struct{}{}
	}
}

func (tr *TaskRouter) releaseExecSlot() {
	if tr.execSem != nil {
		select {
		case <-tr.execSem:
		default:
		}
	}
}

func (tr *TaskRouter) groupLoop(gw *groupWorker) {
	defer tr.wg.Done()

	for enq := range gw.ch {
		gw.lastActive = time.Now()

		// Resolve handler and effective options each run (options may be zero).
		tr.mu.RLock()
		handler := tr.handlers[enq.task.Type]
		eff := tr.effectiveOptions(enq.task.Options)
		tr.mu.RUnlock()

		// Safety: ensure handler still registered
		if handler == nil {
			log.ApplicationLogger().Warn("Task dropped (handler not registered)", "type", enq.task.Type, "group", gw.key)
			tr.maybeReleaseIdempotency(enq.task, eff)
			continue
		}

		// Execute with global concurrency control
		tr.acquireExecSlot()
		err := func() error {
			defer tr.releaseExecSlot()
			ctx := context.Background()
			return handler(ctx, enq.task.Payload)
		}()

		if err != nil {
			// Retry if allowed
			if enq.attempt < eff.MaxAttempts {
				delay := tr.computeBackoff(eff.InitialBackoff, eff.MaxBackoff, enq.attempt)
				attempt := enq.attempt + 1

				log.ApplicationLogger().Warn("Task failed, scheduling retry",
					"type", enq.task.Type,
					"group", gw.key,
					"attempt", attempt,
					"max_attempts", eff.MaxAttempts,
					"backoff", delay.String(),
					"err", err,
				)

				// Re-enqueue after backoff (same group)
				tr.wg.Add(1)
				go func(et *enqueuedTask, d time.Duration) {
					defer tr.wg.Done()
					timer := time.NewTimer(d)
					defer timer.Stop()
					select {
					case <-timer.C:
						et.attempt = attempt
						// If group channel is closed (router shutting down), drop.
						tr.mu.RLock()
						g := tr.groups[gw.key]
						tr.mu.RUnlock()
						if g == nil {
							return
						}
						select {
						case g.ch <- et:
						default:
							// Best-effort: if buffer is full, try a blocking send unless router is closing.
							select {
							case g.ch <- et:
							case <-tr.stopCh:
								return
							}
						}
					case <-tr.stopCh:
						return
					}
				}(enq, delay)
				continue
			}

			log.ErrorLoggerRaw().Error("Task failed; max attempts reached",
				"type", enq.task.Type,
				"group", gw.key,
				"attempts", enq.attempt,
				"err", err,
			)
		}

		// Success or final failure: allow idempotency key to naturally expire.
		tr.maybeReleaseIdempotency(enq.task, eff)
	}
}

func (tr *TaskRouter) computeBackoff(initial, max time.Duration, attempt int) time.Duration {
	// Exponential backoff with jitter: initial * 2^(attempt-1)
	backoff := initial
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > max {
			backoff = max
			break
		}
	}
	// Add 10% jitter
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
	// random in [-delta, +delta]
	n := rand.Int63n(2*delta+1) - delta
	return time.Duration(n)
}

func clampDuration(v, lo, hi time.Duration) time.Duration {
	return max(min(v, hi), lo)
}

func (tr *TaskRouter) backgroundLoop() {
	defer tr.wg.Done()
	t := time.NewTicker(tr.cfg.CleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-tr.stopCh:
			return
		case <-t.C:
			tr.cleanupOnce()
			tr.runCronOnce()
		}
	}
}

func (tr *TaskRouter) cleanupOnce() {
	now := time.Now()

	// Clean idempotency map
	tr.mu.Lock()
	for k, expiry := range tr.inflight {
		if now.After(expiry) {
			delete(tr.inflight, k)
		}
	}

	// Stop idle groups
	for key, gw := range tr.groups {
		if gw == nil || gw.stopping {
			continue
		}
		if now.Sub(gw.lastActive) >= tr.cfg.GroupIdleTTL && len(gw.ch) == 0 {
			gw.stopping = true
			close(gw.ch)
			delete(tr.groups, key)
		}
	}
	tr.mu.Unlock()
}

func (tr *TaskRouter) runCronOnce() {
	now := time.Now()
	tr.cronMu.Lock()
	for _, job := range tr.cronJobs {
		if job == nil || job.stopped {
			continue
		}
		if job.lastRun.IsZero() || now.Sub(job.lastRun) >= job.Interval {
			_ = tr.Dispatch(context.Background(), job.Task)
			job.lastRun = now
		}
	}
	tr.cronMu.Unlock()
}

func (tr *TaskRouter) maybeReleaseIdempotency(t Task, eff TaskOptions) {
	// Keep idempotency entry until TTL expires to dedupe follow-up duplicates.
	// Nothing to do here; cleanupOnce will remove expired entries.
	// This function exists to provide a single point to change behavior if needed later.
	_ = eff
}
