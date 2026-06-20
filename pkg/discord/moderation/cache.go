package moderation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
)

// CacheFallbackResolver defines a mechanism to attempt memory-only reads
// and conditionally fall back to synchronous REST calls.
type CacheFallbackResolver interface {
	Member(guildID discord.GuildID, userID discord.UserID) (*discord.Member, error)
	MemberFromAPI(guildID discord.GuildID, userID discord.UserID) (*discord.Member, error)
}

// FallbackCache wraps Arikawa cache/state mechanisms to ensure robust member resolution.
type FallbackCache struct {
	state  CacheFallbackResolver
	logger *slog.Logger
}

// NewFallbackCache constructs a fallback wrapper over an arikawa state.
func NewFallbackCache(state CacheFallbackResolver, logger *slog.Logger) *FallbackCache {
	if logger == nil {
		logger = slog.Default()
	}
	return &FallbackCache{state: state, logger: logger}
}

// ResolveMember attempts to read the target from in-memory caches.
// If absent, it immediately triggers a secondary REST call, blocking until resolution.
func (c *FallbackCache) ResolveMember(ctx context.Context, guildID discord.GuildID, userID discord.UserID) (*discord.Member, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	member, err := c.state.Member(guildID, userID)
	if err == nil && member != nil {
		return member, nil
	}

	// Fallback to API if cache misses
	c.logger.Warn("Mitigated service degradation: Target absent from local cache; executing REST fallback",
		slog.String("guild_id", guildID.String()),
		slog.String("target_id", userID.String()),
	)

	member, err = c.state.MemberFromAPI(guildID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed resolving member from REST API: %w", err)
	}

	return member, nil
}
