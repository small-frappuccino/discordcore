package qotd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
)

// ErrSessionUnavailable indicates that a bot session is not available.
var ErrSessionUnavailable = errors.New("discord session is unavailable")

// ClientResolver abstracts how the QOTD publisher obtains an Arikawa client for a guild.
type ClientResolver interface {
	ArikawaClientForGuild(guildID string) (*api.Client, error)
}

// PublisherRouter routes domain publishing requests directly to the active Arikawa gateway state,
// eliminating dual-SDK translation locks and local caching.
type PublisherRouter struct {
	resolver ClientResolver
}

// NewPublisherRouter instantiates a purely stateless publisher router.
func NewPublisherRouter(resolver ClientResolver) *PublisherRouter {
	slog.Info("Architectural state transition: Allocating stateless native Arikawa publisher orchestrator")
	return &PublisherRouter{
		resolver: resolver,
	}
}

func (p *PublisherRouter) PublishOfficialPost(ctx context.Context, params domain.PublishOfficialPostParams) (*domain.PublishedOfficialPost, error) {
	client, err := p.resolver.ArikawaClientForGuild(params.GuildID)
	if err != nil {
		if errors.Is(err, ErrSessionUnavailable) {
			slog.Debug("QOTD publish execution dropped: explicitly disabled for guild", slog.String("guildID", params.GuildID))
			return nil, nil
		}
		return nil, err
	}
	pub := NewArikawaPublisher(client)
	return pub.PublishOfficialPost(ctx, params)
}

func (p *PublisherRouter) DeleteOfficialPost(ctx context.Context, params domain.DeleteOfficialPostParams) error {
	client, err := p.resolver.ArikawaClientForGuild(params.GuildID)
	if err != nil {
		if errors.Is(err, ErrSessionUnavailable) {
			slog.Debug("QOTD delete execution dropped: explicitly disabled for guild", slog.String("guildID", params.GuildID))
			return nil
		}
		return err
	}
	pub := NewArikawaPublisher(client)
	return pub.DeleteOfficialPost(ctx, params)
}
