package cache

import (
	"github.com/bwmarrin/discordgo"
)

// CachedSession wraps a discordgo.Session and provides automatic caching for frequently accessed data.
// It maintains cache consistency by invalidating entries on relevant events.
type CachedSession struct {
	session *discordgo.Session
	cache   *UnifiedCache
}

// NewCachedSession creates a new cached session wrapper
func NewCachedSession(session *discordgo.Session, cache *UnifiedCache) *CachedSession {
	cs := &CachedSession{
		session: session,
		cache:   cache,
	}

	// Register event handlers to invalidate cache on updates
	cs.registerInvalidationHandlers()

	return cs
}

// Session returns the underlying discordgo.Session for direct access
func (cs *CachedSession) Session() *discordgo.Session {
	return cs.session
}

// Cache returns the underlying UnifiedCache for direct access
func (cs *CachedSession) Cache() *UnifiedCache {
	return cs.cache
}

// GuildMember retrieves a member from cache or API, updating cache on miss
func (cs *CachedSession) GuildMember(guildID, userID string) (*discordgo.Member, error) {
	// Try cache first
	if member, ok := cs.cache.GetMember(guildID, userID); ok {
		return member, nil
	}

	// Try session state cache
	if cs.session.State != nil {
		if member, err := cs.session.State.Member(guildID, userID); err == nil && member != nil {
			cs.cache.SetMember(guildID, userID, member)
			return member, nil
		}
	}

	// Fallback to API
	member, err := cs.session.GuildMember(guildID, userID)
	if err != nil {
		return nil, err
	}

	cs.cache.SetMember(guildID, userID, member)
	return member, nil
}

// Guild retrieves a guild from cache or API, updating cache on miss
func (cs *CachedSession) Guild(guildID string) (*discordgo.Guild, error) {
	// Try cache first
	if guild, ok := cs.cache.GetGuild(guildID); ok {
		return guild, nil
	}

	// Try session state cache
	if cs.session.State != nil {
		if guild, err := cs.session.State.Guild(guildID); err == nil && guild != nil {
			cs.cache.SetGuild(guildID, guild)
			return guild, nil
		}
	}

	// Fallback to API
	guild, err := cs.session.Guild(guildID)
	if err != nil {
		return nil, err
	}

	cs.cache.SetGuild(guildID, guild)
	return guild, nil
}

// GuildRoles retrieves guild roles from cache or API, updating cache on miss
func (cs *CachedSession) GuildRoles(guildID string) ([]*discordgo.Role, error) {
	// Try cache first
	if roles, ok := cs.cache.GetRoles(guildID); ok {
		return roles, nil
	}

	// Fallback to API
	roles, err := cs.session.GuildRoles(guildID)
	if err != nil {
		return nil, err
	}

	cs.cache.SetRoles(guildID, roles)
	return roles, nil
}

// Channel retrieves a channel from cache or API, updating cache on miss
func (cs *CachedSession) Channel(channelID string) (*discordgo.Channel, error) {
	// Try cache first
	if channel, ok := cs.cache.GetChannel(channelID); ok {
		return channel, nil
	}

	// Try session state cache
	if cs.session.State != nil {
		if channel, err := cs.session.State.Channel(channelID); err == nil && channel != nil {
			cs.cache.SetChannel(channelID, channel)
			return channel, nil
		}
	}

	// Fallback to API
	channel, err := cs.session.Channel(channelID)
	if err != nil {
		return nil, err
	}

	cs.cache.SetChannel(channelID, channel)
	return channel, nil
}

// registerInvalidationHandlers sets up event handlers to keep cache consistent
func (cs *CachedSession) registerInvalidationHandlers() {
	// Invalidate member cache on updates
	cs.session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
		if m.User != nil {
			cs.cache.InvalidateMember(m.GuildID, m.User.ID)
		}
	})

	// Invalidate member cache on removal
	cs.session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
		if m.User != nil {
			cs.cache.InvalidateMember(m.GuildID, m.User.ID)
		}
	})

	// Invalidate guild cache on updates
	cs.session.AddHandler(func(s *discordgo.Session, g *discordgo.GuildUpdate) {
		cs.cache.InvalidateGuild(g.ID)
	})

	// Invalidate roles cache on role updates
	cs.session.AddHandler(func(s *discordgo.Session, r *discordgo.GuildRoleCreate) {
		cs.cache.InvalidateRoles(r.GuildID)
	})

	cs.session.AddHandler(func(s *discordgo.Session, r *discordgo.GuildRoleUpdate) {
		cs.cache.InvalidateRoles(r.GuildID)
	})

	cs.session.AddHandler(func(s *discordgo.Session, r *discordgo.GuildRoleDelete) {
		cs.cache.InvalidateRoles(r.GuildID)
	})

	// Invalidate channel cache on updates
	cs.session.AddHandler(func(s *discordgo.Session, c *discordgo.ChannelUpdate) {
		cs.cache.InvalidateChannel(c.ID)
	})

	cs.session.AddHandler(func(s *discordgo.Session, c *discordgo.ChannelDelete) {
		cs.cache.InvalidateChannel(c.ID)
	})
}
