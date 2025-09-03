package commands

import (
	"github.com/bwmarrin/discordgo"
)

// CommandArgMeta define metadados para argumentos de subcomando
type CommandArgMeta struct {
	Type         discordgo.ApplicationCommandOptionType
	Name         string
	Description  string
	Required     bool
	Autocomplete bool
	Choices      []*discordgo.ApplicationCommandOptionChoice
}

// SubcommandMeta define metadados para subcomandos
type SubcommandMeta struct {
	Name        string
	Description string
	Args        []CommandArgMeta
}

// automodSubcommands centraliza a definição dos subcomandos e seus argumentos
var automodSubcommands = []SubcommandMeta{
	{
		Name:        "nativeruleregister",
		Description: "Register a native Discord AutoMod rule.",
		Args: []CommandArgMeta{
			{Type: discordgo.ApplicationCommandOptionString, Name: "rule", Description: "Native rule ID", Required: true, Autocomplete: false},
		},
	},
	{
		Name:        "lists",
		Description: "List all lists configured in this server.",
	},
	{
		Name:        "listcreate",
		Description: "Create a list.",
		Args: []CommandArgMeta{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "List type (keyword, native, website, serverlink)",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "keyword", Value: "keyword"},
					{Name: "native", Value: "native"},
					{Name: "website", Value: "website"},
					{Name: "serverlink", Value: "serverlink"},
				},
			},
			{Type: discordgo.ApplicationCommandOptionString, Name: "list", Description: "List of the words", Required: true, Autocomplete: false},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mode",
				Description: "Mode (denylist or allowlist)",
				Required:    false,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "denylist", Value: "denylist"},
					{Name: "allowlist", Value: "allowlist"},
				},
			},
		},
	},
	{
		Name:        "listdelete",
		Description: "Delete a list.",
		Args: []CommandArgMeta{
			{Type: discordgo.ApplicationCommandOptionString, Name: "list", Description: "Select a list to delete", Required: true, Autocomplete: true},
		},
	},
	{
		Name:        "listrename",
		Description: "Rename a list.",
		Args: []CommandArgMeta{
			{Type: discordgo.ApplicationCommandOptionString, Name: "list", Description: "Select a list to rename", Required: true, Autocomplete: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "New name for the list", Required: true},
		},
	},
	{
		Name:        "rulesets",
		Description: "List all rulesets configured in this server.",
	},
	{
		Name:        "rulesetcreate",
		Description: "Create a new ruleset for this server.",
		Args: []CommandArgMeta{
			{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "Name for the new ruleset", Required: true},
		},
	},
	{
		Name:        "rulesetdelete",
		Description: "Delete a ruleset.",
		Args: []CommandArgMeta{
			{Type: discordgo.ApplicationCommandOptionString, Name: "ruleset", Description: "Select a ruleset to delete", Required: true, Autocomplete: true},
		},
	},
	{
		Name:        "rulesetrename",
		Description: "Rename a ruleset.",
		Args: []CommandArgMeta{
			{Type: discordgo.ApplicationCommandOptionString, Name: "ruleset", Description: "Select a ruleset to rename", Required: true, Autocomplete: true},
			{Type: discordgo.ApplicationCommandOptionString, Name: "name", Description: "New name for the ruleset", Required: true},
		},
	},
	{
		Name:        "rulesettoggle",
		Description: "Toggle a ruleset.",
		Args: []CommandArgMeta{
			{Type: discordgo.ApplicationCommandOptionString, Name: "ruleset", Description: "Select a ruleset to toggle", Required: true, Autocomplete: true},
		},
	},
}

// Helper para converter metadados em ApplicationCommandOption
func buildAutomodOptions() []*discordgo.ApplicationCommandOption {
	var opts []*discordgo.ApplicationCommandOption
	for _, sub := range automodSubcommands {
		var subOpts []*discordgo.ApplicationCommandOption
		for _, arg := range sub.Args {
			subOpts = append(subOpts, &discordgo.ApplicationCommandOption{
				Type:         arg.Type,
				Name:         arg.Name,
				Description:  arg.Description,
				Required:     arg.Required,
				Autocomplete: arg.Autocomplete,
				Choices:      arg.Choices,
			})
		}
		opts = append(opts, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        sub.Name,
			Description: sub.Description,
			Options:     subOpts,
		})
	}
	return opts
}

// buildAutomodCommand constructs the /automod command with its subcommands (modular)
func (ch *CommandHandler) buildAutomodCommand() SlashCommand {
	return SlashCommand{
		Name:        "automod",
		Description: "Manage native Discord AutoMod keyword rules.",
		Handler:     ch.handleAutomodCommand,
		Options:     buildAutomodOptions(),
	}
}
