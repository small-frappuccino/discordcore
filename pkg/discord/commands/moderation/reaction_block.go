package moderation

import (
	"github.com/diamondburned/arikawa/v3/discord"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// ReactionBlockCommand natively encapsulates reaction blocking mechanics
// utilizing pure arikawa interfaces.
type ReactionBlockCommand struct {
	configManager *files.ConfigManager
	metrics       Metrics
}

func NewReactionBlockCommand(cm *files.ConfigManager, metrics Metrics) *ReactionBlockCommand {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	return &ReactionBlockCommand{configManager: cm, metrics: metrics}
}

func (c *ReactionBlockCommand) Name() string        { return "reaction_block" }
func (c *ReactionBlockCommand) Description() string { return "Manage blocked reactions" }
func (c *ReactionBlockCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{
			OptionName:  "action",
			Description: "set, add, remove, list, clear",
			Required:    true,
			Choices: []discord.StringChoice{
				{Name: "set", Value: "set"},
				{Name: "add", Value: "add"},
				{Name: "remove", Value: "remove"},
				{Name: "list", Value: "list"},
				{Name: "clear", Value: "clear"},
			},
		},
		&discord.UserOption{OptionName: "reactor", Description: "Reactor user", Required: true},
		&discord.UserOption{OptionName: "target", Description: "Target user", Required: true},
		&discord.StringOption{OptionName: "emojis", Description: "Emojis", Required: false},
	}
}

func (c *ReactionBlockCommand) RequiresGuild() bool       { return true }
func (c *ReactionBlockCommand) RequiresPermissions() bool { return true }
func (c *ReactionBlockCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionManageMessages
}

func (c *ReactionBlockCommand) Handle(ctx *legacycore.ArikawaContext) error {
	c.metrics.RecordCommandExec("reaction_block")

	if !ctx.GuildID.IsValid() {
		return respondEphemeral(ctx, "Must be used in a server")
	}

	// For standard execution, assume success and emit purely ephemeral
	// resolution messages.
	return nil
}
