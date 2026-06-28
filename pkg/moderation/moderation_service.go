package moderation

import (
	"context"
	"strconv"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/discord"
	"golang.org/x/sync/errgroup"
)

// Service is the asynchronous moderation router based on the Actor Model.
type Service struct {
	discord   DiscordGateway
	queueSize int
	router    *Router
	actorsMu  sync.RWMutex
	actors    map[uint64]*GuildActor
	eg        *errgroup.Group
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewService(discord DiscordGateway, queueSize int, router *Router) *Service {
	return &Service{
		discord:   discord,
		queueSize: queueSize,
		router:    router,
		actors:    make(map[uint64]*GuildActor),
	}
}

func (s *Service) Start(ctx context.Context, numWorkers int) {
	// numWorkers is ignored as we dynamically spawn GuildActors.
	ctx, s.cancel = context.WithCancel(ctx)
	s.eg, s.ctx = errgroup.WithContext(ctx)
}

func (s *Service) Wait() error {
	if s.eg == nil {
		return nil
	}
	return s.eg.Wait()
}

func (s *Service) Route(guildID uint64) discord.ActorInbox {
	if s.ctx == nil || s.eg == nil {
		return nil
	}

	s.actorsMu.RLock()
	actor, ok := s.actors[guildID]
	s.actorsMu.RUnlock()
	if ok {
		return actor
	}

	s.actorsMu.Lock()
	defer s.actorsMu.Unlock()

	// Double-check under write lock to avoid races
	actor, ok = s.actors[guildID]
	if ok {
		return actor
	}

	guildIDStr := strconv.FormatUint(guildID, 10)
	actor = newGuildActor(s.ctx, s.eg, guildIDStr, s.discord, s.queueSize, s.router)
	s.actors[guildID] = actor
	return actor
}

func (s *Service) SystemRoute() discord.ActorInbox {
	// Not implemented for this refactor scope.
	return nil
}
