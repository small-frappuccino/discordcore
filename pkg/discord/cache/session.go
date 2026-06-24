package cache

import (
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"golang.org/x/sync/singleflight"
)

// CachedSession acts as a transparent, caching proxy layer wrapping an underlying Arikawa Discord API client.
type CachedSession struct {
	client *api.Client
	cache  *UnifiedCache
	sf     singleflight.Group
}

// NewCachedSession initializes a resilient API client wrapper equipped with singleflight request deduplication.
func NewCachedSession(client *api.Client, cache *UnifiedCache) *CachedSession {
	slog.Info("Architectural state transition: Initializing CachedSession wrapper")
	return &CachedSession{
		client: client,
		cache:  cache,
	}
}

// GuildMember attempts to resolve a user within a guild, preferring the local cache before executing a network request.
// It leverages singleflight to coalesce concurrent fetches for the same member, preventing thundering herd exhaustion.
func (cs *CachedSession) GuildMember(guildID, userID string) (*discord.Member, error) {
	if member, ok := cs.cache.GetMember(guildID, userID); ok {
		return member, nil
	}

	key := guildID + ":" + userID
	v, err, shared := cs.sf.Do(key, func() (any, error) {
		slog.Debug("Granular transient state inspection: Cache miss, executing singleflight fetch",
			slog.String("guildID", guildID),
			slog.String("userID", userID),
		)
		gid, _ := discord.ParseSnowflake(guildID)
		uid, _ := discord.ParseSnowflake(userID)
		return cs.client.Member(discord.GuildID(gid), discord.UserID(uid))
	})
	if err != nil {
		slog.Error("Blocking structural failure: Singleflight REST fetch failed for member",
			slog.String("request_id", "fetch_member_"+key),
			slog.String("error", err.Error()),
			slog.Int("status_code", 500),
		)
		return nil, fmt.Errorf("CachedSession.GuildMember: %w", err)
	}

	if shared {
		slog.Debug("Granular transient state inspection: Singleflight shared identical fetch",
			slog.String("guildID", guildID),
			slog.String("userID", userID),
		)
	}

	member := v.(*discord.Member)
	cs.cache.SetMember(guildID, userID, member)
	return member, nil
}

// Guild resolves a Discord guild structure, checking the local cache prior to a fallback REST call.
// It prevents redundant concurrent API requests for the identical resource using singleflight deduplication.
func (cs *CachedSession) Guild(guildID string) (*discord.Guild, error) {
	if guild, ok := cs.cache.GetGuild(guildID); ok {
		return guild, nil
	}

	key := "guild:" + guildID
	v, err, shared := cs.sf.Do(key, func() (any, error) {
		slog.Debug("Granular transient state inspection: Cache miss, executing singleflight fetch",
			slog.String("guildID", guildID),
		)
		gid, _ := discord.ParseSnowflake(guildID)
		return cs.client.Guild(discord.GuildID(gid))
	})
	if err != nil {
		slog.Error("Blocking structural failure: Singleflight REST fetch failed for guild",
			slog.String("request_id", "fetch_"+key),
			slog.String("error", err.Error()),
			slog.Int("status_code", 500),
		)
		return nil, fmt.Errorf("CachedSession.Guild: %w", err)
	}

	if shared {
		slog.Debug("Granular transient state inspection: Singleflight shared identical fetch",
			slog.String("guildID", guildID),
		)
	}

	guild := v.(*discord.Guild)
	cs.cache.SetGuild(guildID, guild)
	return guild, nil
}

// HandleGuildMemberUpdate processes gateway synchronization payloads by explicitly evicting stale member cache lines.
func (cs *CachedSession) HandleGuildMemberUpdate(e *gateway.GuildMemberUpdateEvent) {
	slog.Info("Architectural state transition: Invalidation via Gateway", slog.String("event", "GuildMemberUpdate"))
	cs.cache.InvalidateMember(e.GuildID.String(), e.User.ID.String())
}

// HandleGuildRoleDelete processes gateway synchronization payloads by iterating and purging the targeted role from the cached slice.
// This implements a partial-update strategy to avoid entirely invalidating the guild's role aggregate.
func (cs *CachedSession) HandleGuildRoleDelete(e *gateway.GuildRoleDeleteEvent) {
	slog.Info("Architectural state transition: Partial Invalidation via Gateway", slog.String("event", "GuildRoleDelete"))
	if roles, ok := cs.cache.GetRoles(e.GuildID.String()); ok {
		newRoles := make([]discord.Role, 0, len(*roles))
		for _, r := range *roles {
			if r.ID != e.RoleID {
				newRoles = append(newRoles, r)
			}
		}
		cs.cache.SetRoles(e.GuildID.String(), &newRoles)
	}
}
