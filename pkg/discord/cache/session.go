package cache

import (
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"golang.org/x/sync/singleflight"
)

type CachedSession struct {
	client *api.Client
	cache  *UnifiedCache
	sf     singleflight.Group
}

func NewCachedSession(client *api.Client, cache *UnifiedCache) *CachedSession {
	return &CachedSession{
		client: client,
		cache:  cache,
	}
}

func (cs *CachedSession) GuildMember(guildID, userID string) (*discord.Member, error) {
	if member, ok := cs.cache.GetMember(guildID, userID); ok {
		return member, nil
	}

	v, err, _ := cs.sf.Do(fmt.Sprintf("member:%s:%s", guildID, userID), func() (any, error) {
		gid, _ := discord.ParseSnowflake(guildID)
		uid, _ := discord.ParseSnowflake(userID)
		return cs.client.Member(discord.GuildID(gid), discord.UserID(uid))
	})
	if err != nil {
		return nil, fmt.Errorf("CachedSession.GuildMember: %w", err)
	}

	member := v.(*discord.Member)
	cs.cache.SetMember(guildID, userID, member)
	return member, nil
}

func (cs *CachedSession) Guild(guildID string) (*discord.Guild, error) {
	if guild, ok := cs.cache.GetGuild(guildID); ok {
		return guild, nil
	}

	v, err, _ := cs.sf.Do(fmt.Sprintf("guild:%s", guildID), func() (any, error) {
		gid, _ := discord.ParseSnowflake(guildID)
		return cs.client.Guild(discord.GuildID(gid))
	})
	if err != nil {
		return nil, fmt.Errorf("CachedSession.Guild: %w", err)
	}

	guild := v.(*discord.Guild)
	cs.cache.SetGuild(guildID, guild)
	return guild, nil
}

func (cs *CachedSession) HandleGuildMemberUpdate(e *gateway.GuildMemberUpdateEvent) {
	slog.Info("Architectural state transition: Invalidation via Gateway", slog.String("event", "GuildMemberUpdate"))
	cs.cache.InvalidateMember(e.GuildID.String(), e.User.ID.String())
}

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
