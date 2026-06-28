package moderation

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord"
	"golang.org/x/sync/errgroup"
)

var (
	ErrModerationQueueFull = errors.New("moderation queue full for this guild (load shedding)")
	ErrActorClosed         = errors.New("actor closed")
)

// GuildActor encapsulates the serial state mutation for a single guild.
type GuildActor struct {
	guildID string
	inbox   chan *discord.GatewayEvent
	client  DiscordGateway
	router  *Router
	ctx     context.Context
	cancel  context.CancelFunc
	timer   *time.Timer // timer pre-allocated para zero allocs em backoffs de rate limit
}

func newGuildActor(ctx context.Context, eg *errgroup.Group, guildID string, client DiscordGateway, queueSize int, router *Router) *GuildActor {
	actorCtx, cancel := context.WithCancel(ctx)

	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	actor := &GuildActor{
		guildID: guildID,
		inbox:   make(chan *discord.GatewayEvent, queueSize),
		client:  client,
		router:  router,
		ctx:     actorCtx,
		cancel:  cancel,
		timer:   timer,
	}

	eg.Go(func() error {
		return actor.run()
	})

	return actor
}

// EnqueueEvent implements discord.ActorInbox.
func (a *GuildActor) EnqueueEvent(evt *discord.GatewayEvent) error {
	if err := a.ctx.Err(); err != nil {
		return ErrActorClosed
	}
	select {
	case <-a.ctx.Done():
		return ErrActorClosed
	case a.inbox <- evt:
		return nil
	default:
		return ErrModerationQueueFull
	}
}

func (a *GuildActor) run() error {
	for {
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case evt, ok := <-a.inbox:
			if !ok {
				return nil
			}

			if evt.Type == "INTERACTION_CREATE" {
				// Parse and route inside the Actor
				a.processInteraction(evt.Data)
			}

			evt.Release()
		}
	}
}

func (a *GuildActor) processInteraction(data []byte) {
	it, err := a.router.ParseInteraction(a.ctx, data)
	if err != nil {
		slog.Error("Falha no parse da interação", "guild_id", a.guildID, "error", err)
		return
	}

	it.All(func(job ModerationJob) bool {
		var execErr error
		switch job.Action {
		case ActionBan:
			execErr = a.client.ExecuteBan(a.ctx, job.Bot, job.TargetUserID, job.Reason, job.DeleteDays*86400)
		case ActionKick:
			execErr = a.client.ExecuteKick(a.ctx, job.Bot, job.TargetUserID, job.Reason)
		}

		if execErr != nil {
			var rlErr *discord.RateLimitError
			if errors.As(execErr, &rlErr) {
				// Sleep blocking the actor. This exerts backpressure on the inbox channel.
				a.timer.Reset(rlErr.RetryAfter)
				select {
				case <-a.ctx.Done():
					a.timer.Stop()
					return false
				case <-a.timer.C:
					// Retry immediately inline.
				}

				// Re-execute
				switch job.Action {
				case ActionBan:
					_ = a.client.ExecuteBan(a.ctx, job.Bot, job.TargetUserID, job.Reason, job.DeleteDays*86400)
				case ActionKick:
					_ = a.client.ExecuteKick(a.ctx, job.Bot, job.TargetUserID, job.Reason)
				}
			} else {
				slog.Error("Falha na tarefa de moderação",
					"action", job.Action,
					"guild_id", a.guildID,
					"error", execErr)
			}
		}
		return true
	})
}
