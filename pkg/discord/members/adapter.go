package members

import (
	"context"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

// ArikawaAdapter implements the domain members.DiscordAdapter interface
// using the Arikawa SDK state.
type ArikawaAdapter struct {
	state *state.State
}

// NewArikawaAdapter creates a new ArikawaAdapter.
func NewArikawaAdapter(s *state.State) *ArikawaAdapter {
	return &ArikawaAdapter{state: s}
}

func (a *ArikawaAdapter) Me() (string, error) {
	u, err := a.state.Me()
	if err != nil {
		return "", err
	}
	return u.ID.String(), nil
}

func (a *ArikawaAdapter) MemberJoinedAt(ctx context.Context, guildID, userID string) (time.Time, error) {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return time.Time{}, err
	}
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return time.Time{}, err
	}
	mem, err := a.state.Member(discord.GuildID(gID), discord.UserID(uID))
	if err != nil {
		return time.Time{}, err
	}
	return mem.Joined.Time(), nil
}

func (a *ArikawaAdapter) AddRole(ctx context.Context, guildID, userID, roleID string) error {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return err
	}
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return err
	}
	rID, err := discord.ParseSnowflake(roleID)
	if err != nil {
		return err
	}
	return a.state.AddRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), api.AddRoleData{})
}

func (a *ArikawaAdapter) RemoveRole(ctx context.Context, guildID, userID, roleID string) error {
	gID, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return err
	}
	uID, err := discord.ParseSnowflake(userID)
	if err != nil {
		return err
	}
	rID, err := discord.ParseSnowflake(roleID)
	if err != nil {
		return err
	}
	return a.state.RemoveRole(discord.GuildID(gID), discord.UserID(uID), discord.RoleID(rID), "automated role removal")
}
