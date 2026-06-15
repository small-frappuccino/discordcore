package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/log"
	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
)

// dualSDKPublisher wraps the Arikawa publisher and dynamically injects the appropriate
// Discord client token based on the target guild ID, enabling an active dual-SDK rollout.
type dualSDKPublisher struct {
	resolver *botRuntimeResolver
	mu       sync.Mutex
	clients  map[string]*state.State // botInstanceID -> Arikawa State
}

// newDualSDKPublisher creates a new dualSDKPublisher for the QOTD service.
func newDualSDKPublisher(resolver *botRuntimeResolver) *dualSDKPublisher {
	slog.Info("Architectural state transition: Allocating dual-SDK publisher orchestrator")
	return &dualSDKPublisher{
		resolver: resolver,
		clients:  make(map[string]*state.State),
	}
}

// getArikawaPublisher resolves the guild's bot instance and returns a cached Arikawa publisher.
func (p *dualSDKPublisher) getArikawaPublisher(guildID string) (domain.Publisher, error) {
	_, botInstanceID, err := p.resolver.runtimeForGuild(guildID, "qotd")
	if err != nil {
		errWrap := fmt.Errorf("resolve bot instance for guild %s: %w", guildID, err)
		log.EmitBlockingError("Blocking structural failure: Orchestrator failed to resolve QOTD runtime capability for target guild", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if st, ok := p.clients[botInstanceID]; ok {
		slog.Debug("Tracking complex conditional branch: Arikawa state publisher resolved from memory cache",
			slog.String("bot_instance_id", botInstanceID),
			slog.String("guild_id", guildID),
		)
		return qotd.NewArikawaPublisher(st), nil
	}

	session, err := p.resolver.sessionForGuild(guildID, "qotd")
	if err != nil {
		errWrap := fmt.Errorf("resolve discord session for guild %s: %w", guildID, err)
		log.EmitBlockingError("Blocking structural failure: Orchestrator failed to resolve Discord session pointer for target guild", errWrap, log.GenerateRequestID())
		return nil, errWrap
	}

	token := session.Token
	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}

	slog.Info("Architectural state transition: Materializing new Arikawa state publisher instance",
		slog.String("bot_instance_id", botInstanceID),
	)

	st := state.New("Bot " + strings.TrimPrefix(token, "Bot "))
	p.clients[botInstanceID] = st

	return qotd.NewArikawaPublisher(st), nil
}

// PublishOfficialPost implements domain.Publisher by routing the call to Arikawa.
func (p *dualSDKPublisher) PublishOfficialPost(ctx context.Context, params domain.PublishOfficialPostParams) (*domain.PublishedOfficialPost, error) {
	slog.Debug("Granular inspection: Routing PublishOfficialPost payload through dual-SDK gateway",
		slog.String("guild_id", params.GuildID),
	)

	pub, err := p.getArikawaPublisher(params.GuildID)
	if err != nil {
		return nil, err
	}
	return pub.PublishOfficialPost(ctx, params)
}

// DeleteOfficialPost implements domain.Publisher by routing the call to Arikawa.
func (p *dualSDKPublisher) DeleteOfficialPost(ctx context.Context, params domain.DeleteOfficialPostParams) error {
	slog.Debug("Granular inspection: Routing DeleteOfficialPost payload through dual-SDK gateway",
		slog.String("guild_id", params.GuildID),
	)

	pub, err := p.getArikawaPublisher(params.GuildID)
	if err != nil {
		return err
	}
	return pub.DeleteOfficialPost(ctx, params)
}
