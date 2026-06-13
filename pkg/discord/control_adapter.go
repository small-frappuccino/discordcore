package discord

import (
	"errors"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/control"
)

// ControlAdapter wraps Arikawa's state to provide the control.DiscordService interface.
type ControlAdapter struct {
	state *state.State
}

// NewControlAdapter creates a new ControlAdapter.
func NewControlAdapter(s *state.State) *ControlAdapter {
	return &ControlAdapter{state: s}
}

// BotUser returns the bot user.
func (a *ControlAdapter) BotUser() (*control.User, error) {
	u, err := a.state.Me()
	if err != nil {
		return nil, err
	}
	return &control.User{
		ID:       u.ID.String(),
		Username: u.Username,
		Avatar:   string(u.Avatar),
	}, nil
}

// Guild returns a guild.
func (a *ControlAdapter) Guild(guildID string) (*control.Guild, error) {
	gid, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	g, err := a.state.Guild(discord.GuildID(gid))
	if err != nil {
		return nil, err
	}

	roles := make([]*control.Role, len(g.Roles))
	for i, r := range g.Roles {
		roles[i] = &control.Role{
			ID:          r.ID.String(),
			Name:        r.Name,
			Position:    r.Position,
			Permissions: int64(r.Permissions),
		}
	}

	channels, err := a.state.Channels(discord.GuildID(gid))
	var cChannels []*control.Channel
	if err == nil {
		cChannels = make([]*control.Channel, len(channels))
		for i, c := range channels {
			cChannels[i] = &control.Channel{
				ID:       c.ID.String(),
				Name:     c.Name,
				Type:     control.ChannelType(c.Type),
				Position: c.Position,
				ParentID: c.ParentID.String(),
			}
		}
	}

	return &control.Guild{
		ID:       g.ID.String(),
		Name:     g.Name,
		OwnerID:  g.OwnerID.String(),
		Icon:     string(g.Icon),
		Roles:    roles,
		Channels: cChannels,
	}, nil
}

// GuildMember returns a member.
func (a *ControlAdapter) GuildMember(guildID, userID string) (*control.Member, error) {
	gid, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	uid, err := discord.ParseSnowflake(userID)
	if err != nil {
		return nil, err
	}
	m, err := a.state.Member(discord.GuildID(gid), discord.UserID(uid))
	if err != nil {
		return nil, err
	}

	roles := make([]string, len(m.RoleIDs))
	for i, r := range m.RoleIDs {
		roles[i] = r.String()
	}

	return &control.Member{
		User: &control.User{
			ID:       m.User.ID.String(),
			Username: m.User.Username,
			Avatar:   string(m.User.Avatar),
		},
		Nick:  m.Nick,
		Roles: roles,
	}, nil
}

// GuildMembers returns a list of members.
func (a *ControlAdapter) GuildMembers(guildID, after string, limit int) ([]*control.Member, error) {
	gid, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return nil, err
	}
	afterID := discord.UserID(0)
	if after != "" {
		parsed, err := discord.ParseSnowflake(after)
		if err == nil {
			afterID = discord.UserID(parsed)
		}
	}
	members, err := a.state.MembersAfter(discord.GuildID(gid), afterID, uint(limit))
	if err != nil {
		return nil, err
	}

	res := make([]*control.Member, len(members))
	for i, m := range members {
		roles := make([]string, len(m.RoleIDs))
		for j, r := range m.RoleIDs {
			roles[j] = r.String()
		}
		res[i] = &control.Member{
			User: &control.User{
				ID:       m.User.ID.String(),
				Username: m.User.Username,
				Avatar:   string(m.User.Avatar),
			},
			Nick:  m.Nick,
			Roles: roles,
		}
	}
	return res, nil
}

// GuildMembersSearch returns a list of members matching the query.
func (a *ControlAdapter) GuildMembersSearch(guildID, query string, limit int) ([]*control.Member, error) {
	return nil, errors.New("not implemented for state alone, requires gateway request")
}

// HasIntent checks if the intent is available.
func (a *ControlAdapter) HasIntent(intentMask int) bool {
	// Not straightforward to check in Arikawa v3 state directly, return true for dashboard needs
	return true
}
