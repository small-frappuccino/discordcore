package moderation

import (
	"context"
	"errors"
	"log/slog"
	"time"
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
}

func newGuildActor(ctx context.Context, guildID string, discord DiscordGateway, queueSize int) *GuildActor {
	actorCtx, cancel := context.WithCancel(ctx)
	actor := &GuildActor{
		guildID: guildID,
		inbox:   make(chan ModerationJob, queueSize),
		discord: discord,
		ctx:     actorCtx,
		cancel:  cancel,
	}
	go actor.run()
	return actor
}

func (a *GuildActor) run() {
	for {
		select {
		case <-a.ctx.Done():
			return
		case job, ok := <-a.inbox:
			if !ok {
				return
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
					go func(j ModerationJob, delay time.Duration) {
						timer := time.NewTimer(delay)
						defer timer.Stop()
						select {
						case <-a.ctx.Done():
							return
						case <-timer.C:
							a.enqueue(j)
						}
					}(job, rlErr.RetryAfter)
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
