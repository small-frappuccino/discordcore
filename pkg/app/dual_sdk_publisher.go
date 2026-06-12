package app

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/qotd"
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
	return &dualSDKPublisher{
		resolver: resolver,
		clients:  make(map[string]*state.State),
	}
}

// getArikawaPublisher resolves the guild's bot instance and returns a cached Arikawa publisher.
func (p *dualSDKPublisher) getArikawaPublisher(guildID string) (domain.Publisher, error) {
	_, botInstanceID, err := p.resolver.runtimeForGuild(guildID)
	if err != nil {
		return nil, fmt.Errorf("resolve bot instance for guild %s: %w", guildID, err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if st, ok := p.clients[botInstanceID]; ok {
		return qotd.NewArikawaPublisher(st), nil
	}

	session, err := p.resolver.sessionForGuild(guildID)
	if err != nil {
		return nil, fmt.Errorf("resolve discord session for guild %s: %w", guildID, err)
	}

	token := session.Token
	if !strings.HasPrefix(token, "Bot ") {
		token = "Bot " + token
	}

	st := state.New("Bot " + strings.TrimPrefix(token, "Bot "))
	p.clients[botInstanceID] = st

	return qotd.NewArikawaPublisher(st), nil
}

// PublishOfficialPost implements domain.Publisher by routing the call to Arikawa.
func (p *dualSDKPublisher) PublishOfficialPost(ctx context.Context, params domain.PublishOfficialPostParams) (*domain.PublishedOfficialPost, error) {
	pub, err := p.getArikawaPublisher(params.GuildID)
	if err != nil {
		return nil, err
	}
	return pub.PublishOfficialPost(ctx, params)
}

// SetThreadState implements domain.Publisher by routing the call to Arikawa.
func (p *dualSDKPublisher) SetThreadState(ctx context.Context, guildID string, threadID string, state domain.ThreadState) error {
	pub, err := p.getArikawaPublisher(guildID)
	if err != nil {
		return err
	}
	return pub.SetThreadState(ctx, guildID, threadID, state)
}

// DeleteOfficialPost implements domain.Publisher by routing the call to Arikawa.
func (p *dualSDKPublisher) DeleteOfficialPost(ctx context.Context, params domain.DeleteOfficialPostParams) error {
	pub, err := p.getArikawaPublisher(params.GuildID)
	if err != nil {
		return err
	}
	return pub.DeleteOfficialPost(ctx, params)
}
