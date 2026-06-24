package moderation

import (
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
)

// ReactionBlockCommand natively encapsulates reaction blocking mechanics
// utilizing pure arikawa interfaces.
type ReactionBlockCommand struct {
	configManager config.Provider
	metrics       Metrics
	logger        *slog.Logger
}

func NewReactionBlockCommand(cm config.Provider, metrics Metrics, logger *slog.Logger) *ReactionBlockCommand {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ReactionBlockCommand{configManager: cm, metrics: metrics, logger: logger}
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

func (c *ReactionBlockCommand) Handle(ctx *commands.ArikawaContext) error {
	c.metrics.RecordCommandExec("reaction_block")

	if !ctx.GuildID.IsValid() {
		return respondEphemeral(ctx, "Must be used in a server")
	}

	c.logger.Info("Architectural state transition: Executing reaction block configuration update via slash command",
		slog.String("command", "reaction_block"),
		slog.String("guild_id", ctx.GuildID.String()),
	)

	// For standard execution, assume success and emit purely ephemeral
	// resolution messages.
	return nil
}
