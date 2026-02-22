package partners

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

const (
	defaultAutoSyncDebounce    = 750 * time.Millisecond
	defaultAutoSyncTick        = 200 * time.Millisecond
	defaultAutoSyncNotifyQueue = 256
	defaultAutoSyncWorkQueue   = 128
	defaultAutoSyncTimeout     = 15 * time.Second
)

// GuildSyncExecutor executes one partner board sync for a guild.
type GuildSyncExecutor interface {
	SyncGuild(ctx context.Context, guildID string) error
}

// AutoSyncCoordinatorOptions controls queue/debounce behavior.
type AutoSyncCoordinatorOptions struct {
	Debounce    time.Duration
	Tick        time.Duration
	NotifyQueue int
	WorkQueue   int
	SyncTimeout time.Duration
}

func (o AutoSyncCoordinatorOptions) normalized() AutoSyncCoordinatorOptions {
	out := o
	if out.Debounce <= 0 {
		out.Debounce = defaultAutoSyncDebounce
	}
	if out.Tick <= 0 {
		out.Tick = defaultAutoSyncTick
	}
	if out.NotifyQueue <= 0 {
		out.NotifyQueue = defaultAutoSyncNotifyQueue
	}
	if out.WorkQueue <= 0 {
		out.WorkQueue = defaultAutoSyncWorkQueue
	}
	if out.SyncTimeout <= 0 {
		out.SyncTimeout = defaultAutoSyncTimeout
	}
	return out
}

// AutoSyncCoordinator coordinates async partner board syncs with debounce per guild.
type AutoSyncCoordinator struct {
	executor GuildSyncExecutor
	options  AutoSyncCoordinatorOptions

	notifyQueue chan string
	workQueue   chan string
	doneQueue   chan string

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewAutoSyncCoordinator creates a coordinator for auto sync orchestration.
func NewAutoSyncCoordinator(executor GuildSyncExecutor, options AutoSyncCoordinatorOptions) *AutoSyncCoordinator {
	return &AutoSyncCoordinator{
		executor: executor,
		options:  options.normalized(),
	}
}

// NewSessionBoundBoardSyncExecutor adapts BoardSyncService + Discord session into a GuildSyncExecutor.
func NewSessionBoundBoardSyncExecutor(service *BoardSyncService, session *discordgo.Session) GuildSyncExecutor {
	return GuildSyncExecutorFunc(func(ctx context.Context, guildID string) error {
		if service == nil {
			return fmt.Errorf("session-bound board sync executor: service is nil")
		}
		if session == nil {
			return fmt.Errorf("session-bound board sync executor: discord session is nil")
		}
		return service.SyncGuild(ctx, session, guildID)
	})
}

// GuildSyncExecutorFunc adapts a function to GuildSyncExecutor.
type GuildSyncExecutorFunc func(ctx context.Context, guildID string) error

// SyncGuild executes one guild sync.
func (fn GuildSyncExecutorFunc) SyncGuild(ctx context.Context, guildID string) error {
	return fn(ctx, guildID)
}

// Start starts scheduler/worker loops.
func (c *AutoSyncCoordinator) Start() error {
	if c == nil {
		return fmt.Errorf("start auto-sync coordinator: coordinator is nil")
	}
	if c.executor == nil {
		return fmt.Errorf("start auto-sync coordinator: executor is nil")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return nil
	}

	c.notifyQueue = make(chan string, c.options.NotifyQueue)
	c.workQueue = make(chan string, c.options.WorkQueue)
	c.doneQueue = make(chan string, c.options.WorkQueue)

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.running = true

	c.wg.Add(2)
	go c.schedulerLoop(ctx)
	go c.workerLoop(ctx)

	log.ApplicationLogger().Info(
		"Partner auto-sync coordinator started",
		"debounce_ms", c.options.Debounce.Milliseconds(),
		"tick_ms", c.options.Tick.Milliseconds(),
		"sync_timeout_ms", c.options.SyncTimeout.Milliseconds(),
	)
	return nil
}

// Stop stops coordinator loops and waits for completion.
func (c *AutoSyncCoordinator) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return nil
	}
	cancel := c.cancel
	c.running = false
	c.cancel = nil
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	if ctx == nil {
		<-done
		log.ApplicationLogger().Info("Partner auto-sync coordinator stopped")
		return nil
	}

	select {
	case <-done:
		log.ApplicationLogger().Info("Partner auto-sync coordinator stopped")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop auto-sync coordinator: %w", ctx.Err())
	}
}

// Notify schedules a guild sync with debounce.
func (c *AutoSyncCoordinator) Notify(guildID string) error {
	if c == nil {
		return fmt.Errorf("notify auto-sync coordinator: coordinator is nil")
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return fmt.Errorf("notify auto-sync coordinator: guild_id is required")
	}

	c.mu.Lock()
	running := c.running
	notifyQueue := c.notifyQueue
	c.mu.Unlock()

	if !running || notifyQueue == nil {
		return fmt.Errorf("notify auto-sync coordinator: coordinator is not running")
	}

	select {
	case notifyQueue <- guildID:
		return nil
	default:
		return fmt.Errorf("notify auto-sync coordinator: notify queue is full")
	}
}

func (c *AutoSyncCoordinator) schedulerLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.options.Tick)
	defer ticker.Stop()

	pendingDue := map[string]time.Time{}
	queued := map[string]bool{}

	for {
		select {
		case <-ctx.Done():
			return
		case guildID := <-c.notifyQueue:
			pendingDue[guildID] = time.Now().Add(c.options.Debounce)
		case guildID := <-c.doneQueue:
			delete(queued, guildID)
		case <-ticker.C:
			now := time.Now()
			for guildID, due := range pendingDue {
				if queued[guildID] {
					continue
				}
				if due.After(now) {
					continue
				}

				select {
				case c.workQueue <- guildID:
					queued[guildID] = true
					delete(pendingDue, guildID)
				default:
					// Keep pending entry; next tick retries enqueue.
				}
			}
		}
	}
}

func (c *AutoSyncCoordinator) workerLoop(ctx context.Context) {
	defer c.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case guildID := <-c.workQueue:
			c.runSync(ctx, guildID)
			select {
			case c.doneQueue <- guildID:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (c *AutoSyncCoordinator) runSync(parent context.Context, guildID string) {
	syncCtx, cancel := context.WithTimeout(parent, c.options.SyncTimeout)
	defer cancel()

	if err := c.executor.SyncGuild(syncCtx, guildID); err != nil {
		log.ApplicationLogger().Error(
			"Partner auto-sync execution failed",
			"guild_id", guildID,
			"err", err,
		)
		return
	}

	log.ApplicationLogger().Info(
		"Partner auto-sync execution completed",
		"guild_id", guildID,
	)
}
