package logging

import (
	"context"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type runtimeActivityRunner func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error

type runtimeActivityOptions struct {
	RunErr           runtimeActivityRunner
	EventTimeout     time.Duration
	HeartbeatTimeout time.Duration
	Warn             func(string, ...any)
	Now              func() time.Time
}

type runtimeActivity struct {
	store            *storage.Store
	runErr           runtimeActivityRunner
	eventTimeout     time.Duration
	heartbeatTimeout time.Duration
	warn             func(string, ...any)
	now              func() time.Time

	mu       sync.Mutex
	hbCancel context.CancelFunc
	hbDone   chan struct{}
}

func newRuntimeActivity(store *storage.Store, opts runtimeActivityOptions) *runtimeActivity {
	runErr := opts.RunErr
	if runErr == nil {
		runErr = runErrWithTimeoutContext
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	return &runtimeActivity{
		store:            store,
		runErr:           runErr,
		eventTimeout:     opts.EventTimeout,
		heartbeatTimeout: opts.HeartbeatTimeout,
		warn:             opts.Warn,
		now:              now,
	}
}

func newMonitoringRuntimeActivity(store *storage.Store) *runtimeActivity {
	return newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           monitoringRunErrWithTimeoutContext,
		EventTimeout:     monitoringPersistenceTimeout,
		HeartbeatTimeout: monitoringPersistenceTimeout,
		Warn:             log.ApplicationLogger().Warn,
	})
}

func (ra *runtimeActivity) MarkEvent(ctx context.Context, source string) {
	if ra == nil || ra.store == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ra.runErr(ctx, ra.eventTimeout, func(runCtx context.Context) error {
		return ra.store.SetLastEventContext(runCtx, ra.now())
	}); err != nil && ra.warn != nil {
		ra.warn("Failed to persist last event timestamp", "source", source, "error", err)
	}
}

func (ra *runtimeActivity) StartHeartbeat(ctx context.Context, interval time.Duration) {
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

	if err := ra.runErr(hbCtx, ra.heartbeatTimeout, func(runCtx context.Context) error {
		return ra.store.SetHeartbeatContext(runCtx, ra.now())
	}); err != nil && ra.warn != nil {
		ra.warn("Failed to persist startup heartbeat", "error", err)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		defer close(done)

		for {
			select {
			case <-ticker.C:
				if err := ra.runErr(hbCtx, ra.heartbeatTimeout, func(runCtx context.Context) error {
					return ra.store.SetHeartbeatContext(runCtx, ra.now())
				}); err != nil && ra.warn != nil {
					ra.warn("Failed to persist heartbeat", "error", err)
				}
			case <-hbCtx.Done():
				return
			}
		}
	}()
}

func (ra *runtimeActivity) StopHeartbeat(ctx context.Context) error {
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
