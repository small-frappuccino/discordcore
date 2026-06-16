package monitoring

import (
	"context"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type RuntimeActivityRunner func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error

type RuntimeActivityOptions struct {
	RunErr           RuntimeActivityRunner
	EventTimeout     time.Duration
	HeartbeatTimeout time.Duration
	BotInstanceID    string
	Logger           *slog.Logger
	Now              func() time.Time
	// OnHeartbeatTick fires after every heartbeat persistence attempt
	// (the startup attempt and each ticker firing), with the error
	// returned by RunErr. Test-only seam — production callers leave
	// it nil so the heartbeat loop adds zero work per tick.
	//
	// The callback runs synchronously inside the inner attempt
	// goroutine spawned by runCancellableHeartbeat. A callback that
	// blocks indefinitely no longer deadlocks StopHeartbeat (the
	// outer goroutine still exits via hbCtx.Done()), but it does
	// leak the inner attempt goroutine until the callback returns.
	// Tests that observe ticks should pass tickRecorder.Hook so the
	// dedicated drainer absorbs ticks the test is not asserting on
	// and releases in-flight sends via context cancel during cleanup.
	OnHeartbeatTick func(err error)
}

type RuntimeActivity struct {
	store            *storage.Store
	runErr           RuntimeActivityRunner
	eventTimeout     time.Duration
	heartbeatTimeout time.Duration
	botInstanceID    string
	logger           *slog.Logger
	now              func() time.Time
	onHeartbeatTick  func(err error)

	mu       sync.Mutex
	hbCancel context.CancelFunc
	hbDone   chan struct{}
	hbWg     sync.WaitGroup
}

func NewRuntimeActivity(store *storage.Store, opts RuntimeActivityOptions) *RuntimeActivity {
	runErr := opts.RunErr
	if runErr == nil {
		runErr = RunErrWithTimeoutContext
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	return &RuntimeActivity{
		store:            store,
		runErr:           runErr,
		eventTimeout:     opts.EventTimeout,
		heartbeatTimeout: opts.HeartbeatTimeout,
		botInstanceID:    strings.TrimSpace(opts.BotInstanceID),
		logger:           opts.Logger,
		now:              now,
		onHeartbeatTick:  opts.OnHeartbeatTick,
	}
}

func NewMonitoringRuntimeActivity(store *storage.Store, logger *slog.Logger, botInstanceID ...string) *RuntimeActivity {
	scopedBotInstanceID := ""
	if len(botInstanceID) > 0 {
		scopedBotInstanceID = botInstanceID[0]
	}
	return NewRuntimeActivity(store, RuntimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		EventTimeout:     monitoringPersistenceTimeout,
		HeartbeatTimeout: monitoringPersistenceTimeout,
		BotInstanceID:    scopedBotInstanceID,
		Logger:           logger,
	})
}

// MarkEvent marks event.
func (ra *RuntimeActivity) MarkEvent(ctx context.Context, source string) {
	if ra == nil || ra.store == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ra.runErr(ctx, ra.eventTimeout, func(runCtx context.Context) error {
		return ra.store.SetLastEventForBot(runCtx, ra.botInstanceID, ra.now())
	}); err != nil && ra.logger != nil {
		ra.logger.LogAttrs(ctx, slog.LevelWarn, "Failed to persist last event timestamp", slog.String("source", source), slog.Any("error", err))
	}
}

// StartHeartbeat starts heartbeat.
func (ra *RuntimeActivity) StartHeartbeat(ctx context.Context, interval time.Duration) {
	if ra == nil || ra.store == nil || interval <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ra.mu.Lock()
	if ra.hbCancel != nil {
		ra.mu.Unlock()
		return
	}

	hbCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	ra.hbCancel = cancel
	ra.hbDone = done
	ra.mu.Unlock()

	// Both the startup persistence and each ticker firing are dispatched
	// through runCancellableHeartbeat so the outer goroutine can always
	// exit via hbCtx.Done(): a RunErr or OnHeartbeatTick that ignores ctx
	// only wedges the inner attempt goroutine (which is left to leak until
	// its blocking call returns), never close(done) or StopHeartbeat.
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(done)

		if !ra.runCancellableHeartbeat(hbCtx, "Failed to persist startup heartbeat") {
			return
		}

		for {
			select {
			case <-ticker.C:
				if !ra.runCancellableHeartbeat(hbCtx, "Failed to persist heartbeat") {
					return
				}
			case <-hbCtx.Done():
				return
			}
		}
	}()
}

func (ra *RuntimeActivity) attemptHeartbeat(ctx context.Context, failureMessage string) {
	err := ra.runErr(ctx, ra.heartbeatTimeout, func(runCtx context.Context) error {
		return ra.store.SetHeartbeatForBot(runCtx, ra.botInstanceID, ra.now())
	})
	if err != nil && ra.logger != nil {
		ra.logger.LogAttrs(ctx, slog.LevelWarn, failureMessage, slog.Any("error", err))
	}
	if ra.onHeartbeatTick != nil {
		ra.onHeartbeatTick(err)
	}
}

// runCancellableHeartbeat runs a single attemptHeartbeat in a child
// goroutine and returns true when the attempt completes, false when ctx is
// canceled first. On cancellation the child goroutine is left running and
// exits when its underlying call eventually returns. The leak is the price
// for keeping close(done) and StopHeartbeat reachable when RunErr (or any
// callback it invokes) ignores ctx; in production the call respects ctx
// and the child returns promptly, so the leak is transient.
func (ra *RuntimeActivity) runCancellableHeartbeat(ctx context.Context, failureMessage string) bool {
	attemptDone := make(chan struct{})
	go func() {
		defer close(attemptDone)
		ra.attemptHeartbeat(ctx, failureMessage)
	}()
	select {
	case <-attemptDone:
		return true
	case <-ctx.Done():
		return false
	}
}

// StopHeartbeat stops heartbeat.
func (ra *RuntimeActivity) StopHeartbeat(ctx context.Context) error {
	if ra == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ra.mu.Lock()
	cancel := ra.hbCancel
	done := ra.hbDone
	ra.hbCancel = nil
	ra.hbDone = nil
	ra.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
