package core

import (
	"context"

	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// ArikawaCommand represents a Discord slash command fully native to Arikawa.
type ArikawaCommand interface {
	Name() string
	Description() string
	Options() []discord.CommandOption
	Handle(ctx *ArikawaContext) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// ArikawaDefaultMemberPermissionsProvider allows Arikawa commands to specify a default permission floor.
type ArikawaDefaultMemberPermissionsProvider interface {
	DefaultMemberPermissions() discord.Permissions
}

// ArikawaContext represents the execution context for an Arikawa command.
type ArikawaContext struct {
	Client      *api.Client
	Interaction *discord.InteractionEvent
	Config      *files.ConfigManager
	Logger      *slog.Logger
	GuildID     discord.GuildID
	UserID      discord.UserID
	GuildConfig *files.GuildConfig
	ctx         context.Context
}

// Context returns the standard library context.
func (c *ArikawaContext) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// Respond responds to the interaction with the given message data.
func (c *ArikawaContext) Respond(data api.InteractionResponseData) error {
	return c.Client.RespondInteraction(c.Interaction.ID, c.Interaction.Token, api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &data,
	})
}
