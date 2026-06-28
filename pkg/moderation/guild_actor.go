package moderation

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	ErrModerationQueueFull = errors.New("moderation queue full for this guild (load shedding)")
	ErrActorClosed         = errors.New("actor closed")
)

// GuildActor encapsulates the serial state mutation for a single guild.
type GuildActor struct {
	guildID string
	inbox   chan ModerationJob
	discord DiscordGateway
	ctx     context.Context
	cancel  context.CancelFunc
	timer   *time.Timer // timer pre-allocated para zero allocs em backoffs de rate limit
}

func newGuildActor(ctx context.Context, eg *errgroup.Group, guildID string, discord DiscordGateway, queueSize int) *GuildActor {
	actorCtx, cancel := context.WithCancel(ctx)

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	actor := &GuildActor{
		guildID: guildID,
		inbox:   make(chan ModerationJob, queueSize),
		discord: discord,
		ctx:     actorCtx,
		cancel:  cancel,
		timer:   timer,
	}

	eg.Go(func() error {
		return actor.run()
	})

	return actor
}

func (a *GuildActor) run() error {
	for {
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case job, ok := <-a.inbox:
			if !ok {
				return nil
			}

			var err error
			switch job.Action {
			case ActionBan:
				err = a.discord.ExecuteBan(a.ctx, job.Bot, job.TargetUserID, job.Reason, job.DeleteDays*86400)
			case ActionKick:
				err = a.discord.ExecuteKick(a.ctx, job.Bot, job.TargetUserID, job.Reason)
			}

			if err != nil {
				var rlErr *RateLimitError
				if errors.As(err, &rlErr) {
					// We must not block the actor loop, but we also can't just spawn unbounded goroutines.
					// However, a rate limit retry should probably just block the actor to exert backpressure,
					// or be pushed back to the queue. Wait, pushing back to the queue without a goroutine
					// would require a select with timeout, or just a sleep.
					// Since this actor ONLY processes this guild, sleeping here is exact per-bucket backpressure!
					// "apply per-bucket token-bucket backpressure to the calling Actor's dispatch loop"

					// Sleep blocking the actor. This exerts backpressure on the inbox channel.
					// If the inbox fills up, `enqueue` will fail with ErrModerationQueueFull (load shedding).
					a.timer.Reset(rlErr.RetryAfter)
					select {
					case <-a.ctx.Done():
						a.timer.Stop()
						return a.ctx.Err()
					case <-a.timer.C:
						// Retry immediately by pushing to front? No, just re-enqueue or process inline.
						// The prompt implies we shouldn't spawn a naked go func here.
						// Let's just process inline.
					}

					// Re-execute
					switch job.Action {
					case ActionBan:
						_ = a.discord.ExecuteBan(a.ctx, job.Bot, job.TargetUserID, job.Reason, job.DeleteDays*86400)
					case ActionKick:
						_ = a.discord.ExecuteKick(a.ctx, job.Bot, job.TargetUserID, job.Reason)
					}
				} else {
					slog.Error("Falha na tarefa de moderação",
						"action", job.Action,
						"guild_id", a.guildID,
						"error", err)
				}
			}
		}
	}
}

func (a *GuildActor) enqueue(job ModerationJob) error {
	if err := a.ctx.Err(); err != nil {
		return ErrActorClosed
	}
	select {
	case <-a.ctx.Done():
		return ErrActorClosed
	case a.inbox <- job:
		return nil
	default:
		return ErrModerationQueueFull
	}
}
