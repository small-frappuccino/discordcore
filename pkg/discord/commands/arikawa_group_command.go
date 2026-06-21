package commands

import (
	"fmt"

	"github.com/diamondburned/arikawa/v3/discord"
)

// ArikawaGroupCommand represents a root slash command that acts as a container for subcommands.
type ArikawaGroupCommand struct {
	name        string
	description string

	subcommands map[string]ArikawaCommand
}

// NewArikawaGroupCommand creates a new group command.
func NewArikawaGroupCommand(name, description string) *ArikawaGroupCommand {
	return &ArikawaGroupCommand{
		name:        name,
		description: description,
		subcommands: make(map[string]ArikawaCommand),
	}
}

// AddSubCommand adds a subcommand to the group.
func (c *ArikawaGroupCommand) AddSubCommand(cmd ArikawaCommand) {
	c.subcommands[cmd.Name()] = cmd
}

// Name returns the command name.
func (c *ArikawaGroupCommand) Name() string {
	return c.name
}

// Description returns the command description.
func (c *ArikawaGroupCommand) Description() string {
	return c.description
}

// Options returns the aggregated options from subcommands.
func (c *ArikawaGroupCommand) Options() []discord.CommandOption {
	var opts []discord.CommandOption
	for _, sub := range c.subcommands {
		// Group subcommand
		if group, ok := sub.(*ArikawaGroupCommand); ok {
			opts = append(opts, &discord.SubcommandGroupOption{
				OptionName:  group.Name(),
				Description: group.Description(),
				Subcommands: convertSubcommandsToOptions(group.subcommands),
			})
			continue
		}
		// Regular subcommand
		opts = append(opts, &discord.SubcommandOption{
			OptionName:  sub.Name(),
			Description: sub.Description(),
			Options:     convertCommandOptionsToValues(sub.Options()),
		})
	}
	return opts
}

func convertCommandOptionsToValues(opts []discord.CommandOption) []discord.CommandOptionValue {
	var vals []discord.CommandOptionValue
	for _, opt := range opts {
		if val, ok := opt.(discord.CommandOptionValue); ok {
			vals = append(vals, val)
		}
	}
	return vals
}

func convertSubcommandsToOptions(cmds map[string]ArikawaCommand) []*discord.SubcommandOption {
	var opts []*discord.SubcommandOption
	for _, cmd := range cmds {
		opts = append(opts, &discord.SubcommandOption{
			OptionName:  cmd.Name(),
			Description: cmd.Description(),
			Options:     convertCommandOptionsToValues(cmd.Options()),
		})
	}
	return opts
}

// RequiresGuild returns true.
func (c *ArikawaGroupCommand) RequiresGuild() bool {
	return true
}

// RequiresPermissions returns true.
func (c *ArikawaGroupCommand) RequiresPermissions() bool {
	return true
}

// Handle routes the interaction to the appropriate subcommand.
func (c *ArikawaGroupCommand) Handle(ctx *ArikawaContext) error {
	data, ok := ctx.Interaction.Data.(*discord.CommandInteraction)
	if !ok {
		return fmt.Errorf("invalid interaction data type")
	}

	if len(data.Options) == 0 {
		return fmt.Errorf("no subcommand specified")
	}

	// data.Options[0] could be a subcommand or a subcommand group
	opt := data.Options[0]

	if cmd, exists := c.subcommands[opt.Name]; exists {
		// If it's a group, the next level should be passed or we just delegate to it
		return cmd.Handle(ctx)
	}

	return fmt.Errorf("subcommand %q not found", opt.Name)
}
