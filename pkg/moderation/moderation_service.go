package moderation

import (
	"context"
	"errors"
	"golang.org/x/sync/errgroup"
	"sync"
)

// Service is the asynchronous moderation router based on the Actor Model.
type Service struct {
	discord   DiscordGateway
	queueSize int
	actors    sync.Map // string (GuildID) -> *GuildActor
	eg        *errgroup.Group
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewService(discord DiscordGateway, queueSize int) *Service {
	return &Service{
		discord:   discord,
		queueSize: queueSize,
	}
}

func (s *Service) Start(ctx context.Context, numWorkers int) {
	// numWorkers is ignored as we dynamically spawn GuildActors.
	s.eg, s.ctx = errgroup.WithContext(ctx)
}

func (s *Service) Wait() error {
	if s.eg == nil {
		return nil
	}
	return s.eg.Wait()
}

func (s *Service) EnqueueTask(job ModerationJob) error {
	if s.ctx == nil || s.eg == nil {
		return errors.New("service not started")
	}

	var actor *GuildActor
	val, ok := s.actors.Load(job.Bot.GuildID)
	if !ok {
		actor = newGuildActor(s.ctx, s.eg, job.Bot.GuildID, s.discord, s.queueSize)
		actual, loaded := s.actors.LoadOrStore(job.Bot.GuildID, actor)
		if loaded {
			actor.cancel()
		}
		actor = actual.(*GuildActor)
	} else {
		actor = val.(*GuildActor)
	}

	return actor.enqueue(job)
}
