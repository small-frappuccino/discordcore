package config

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// ConfigCommands wires the real config slash command group into the CommandRouter.
// It also optionally registers simple ping/echo commands to validate the routing pipeline.
type ConfigCommands struct {
	configManager *files.ConfigManager
}

// NewConfigCommands creates a new config commands registrar.
func NewConfigCommands(configManager *files.ConfigManager) *ConfigCommands {
	return &ConfigCommands{configManager: configManager}
}

// RegisterCommands registers the /config command group and optional simple commands in the provided router.
func (cc *ConfigCommands) RegisterCommands(router *core.CommandRouter) {
	// Build /config group with permission checks

	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	group := core.NewGroupCommand("config", "Manage server configuration", checker)

	// Attach subcommands
	group.AddSubCommand(NewConfigSetSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigGetSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigListSubCommand(cc.configManager))

	// Register the group
	router.RegisterCommand(group)

	// Optionally register simple commands (useful for quick health checks of the routing stack)
	router.RegisterCommand(NewPingCommand())
	router.RegisterCommand(NewEchoCommand())
}

// ------------------------------
// Simple Commands: ping / echo
// ------------------------------

type pingCommand struct{}

func NewPingCommand() *pingCommand { return &pingCommand{} }

func (c *pingCommand) Name() string        { return "ping" }
func (c *pingCommand) Description() string { return "Check if the bot is responding" }
func (c *pingCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}
func (c *pingCommand) RequiresGuild() bool       { return false }
func (c *pingCommand) RequiresPermissions() bool { return false }
func (c *pingCommand) Handle(ctx *core.Context) error {
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "🏓 Pong!")
}

type echoCommand struct{}

func NewEchoCommand() *echoCommand { return &echoCommand{} }

func (c *echoCommand) Name() string        { return "echo" }
func (c *echoCommand) Description() string { return "Echo back a message" }
func (c *echoCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "message",
			Description: "Message to echo back",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "ephemeral",
			Description: "Send as ephemeral message",
			Required:    false,
		},
	}
}
func (c *echoCommand) RequiresGuild() bool       { return false }
func (c *echoCommand) RequiresPermissions() bool { return false }
func (c *echoCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)

	message, err := extractor.StringRequired("message")
	if err != nil {
		return err
	}
	ephemeral := extractor.Bool("ephemeral")

	builder := core.NewResponseBuilder(ctx.Session)
	if ephemeral {
		builder = builder.Ephemeral()
	}
	return builder.Info(ctx.Interaction, fmt.Sprintf("Echo: %s", message))
}

// -----------------------------------------
// Config Group SubCommands: set / get / list
// -----------------------------------------

// ConfigSetSubCommand - subcommand to set configuration values
type ConfigSetSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigSetSubCommand(configManager *files.ConfigManager) *ConfigSetSubCommand {
	return &ConfigSetSubCommand{configManager: configManager}
}

func (c *ConfigSetSubCommand) Name() string        { return "set" }
func (c *ConfigSetSubCommand) Description() string { return "Set a configuration value" }
func (c *ConfigSetSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "key",
			Description: "Configuration key",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "command_channel", Value: "command_channel"},
				{Name: "log_channel", Value: "log_channel"},
				{Name: "entry_leave_channel", Value: "entry_leave_channel"},
				{Name: "welcome_backlog_channel", Value: "welcome_backlog_channel"},
				{Name: "message_log_channel", Value: "message_log_channel"},
				{Name: "automod_channel", Value: "automod_channel"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "value",
			Description: "Configuration value",
			Required:    true,
		},
	}
}
func (c *ConfigSetSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigSetSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigSetSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	key, err := extractor.StringRequired("key")
	if err != nil {
		return err
	}
	value, err := extractor.StringRequired("value")
	if err != nil {
		return err
	}

	// Safely mutate guild config
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		switch key {
		case "command_channel":
			guildConfig.CommandChannelID = value
		case "log_channel":
			guildConfig.UserLogChannelID = value // also used by user join/leave/avatar logs
		case "entry_leave_channel":
			guildConfig.UserEntryLeaveChannelID = value
		case "welcome_backlog_channel":
			guildConfig.WelcomeBacklogChannelID = value
		case "message_log_channel":
			guildConfig.MessageLogChannelID = value
		case "automod_channel":
			guildConfig.AutomodLogChannelID = value
		default:
			return core.NewValidationError("key", "Invalid configuration key")
		}
		return nil
	}); err != nil {
		return err
	}

	// Persist changes
	persister := core.NewConfigPersister(c.configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save config: %v", err)
		return core.NewCommandError("Failed to save configuration", true)
	}

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Configuration `%s` set to `%s`", key, value))
}

// ConfigGetSubCommand - subcommand to get configuration values
type ConfigGetSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigGetSubCommand(configManager *files.ConfigManager) *ConfigGetSubCommand {
	return &ConfigGetSubCommand{configManager: configManager}
}

func (c *ConfigGetSubCommand) Name() string        { return "get" }
func (c *ConfigGetSubCommand) Description() string { return "Get current configuration values" }
func (c *ConfigGetSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}
func (c *ConfigGetSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigGetSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigGetSubCommand) Handle(ctx *core.Context) error {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("**Server Configuration:**\n")
	b.WriteString(fmt.Sprintf("Command Channel: %s\n", emptyToDash(ctx.GuildConfig.CommandChannelID)))
	b.WriteString(fmt.Sprintf("Log Channel: %s\n", emptyToDash(ctx.GuildConfig.UserLogChannelID)))
	b.WriteString(fmt.Sprintf("Entry/Leave Channel: %s\n", emptyToDash(ctx.GuildConfig.UserEntryLeaveChannelID)))
	b.WriteString(fmt.Sprintf("Welcome Backlog Channel: %s\n", emptyToDash(ctx.GuildConfig.WelcomeBacklogChannelID)))
	b.WriteString(fmt.Sprintf("Message Log Channel: %s\n", emptyToDash(ctx.GuildConfig.MessageLogChannelID)))
	b.WriteString(fmt.Sprintf("Automod Channel: %s\n", emptyToDash(ctx.GuildConfig.AutomodLogChannelID)))
	b.WriteString(fmt.Sprintf("Allowed Roles: %d configured\n", len(ctx.GuildConfig.AllowedRoles)))

	builder := core.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Server Configuration").
		WithColor(theme.Info())

	return builder.Info(ctx.Interaction, b.String())
}

// ConfigListSubCommand - subcommand to list available configuration options
type ConfigListSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigListSubCommand(configManager *files.ConfigManager) *ConfigListSubCommand {
	return &ConfigListSubCommand{configManager: configManager}
}

func (c *ConfigListSubCommand) Name() string { return "list" }
func (c *ConfigListSubCommand) Description() string {
	return "List all available configuration options"
}
func (c *ConfigListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}
func (c *ConfigListSubCommand) RequiresGuild() bool       { return true }
func (c *ConfigListSubCommand) RequiresPermissions() bool { return true }
func (c *ConfigListSubCommand) Handle(ctx *core.Context) error {
	options := []string{
		"**Available Configuration Options:**",
		"`command_channel` - Channel for bot commands",
		"`log_channel` - Channel for user logs (join/leave/avatar)",
		"`entry_leave_channel` - Channel for entry/leave logs (moderators)",
		"`welcome_backlog_channel` - Public welcome/goodbye channel used for backlog/backfill (e.g., Mimu)",
		"`message_log_channel` - Channel for edited/deleted message logs",
		"`automod_channel` - Channel for automod logs",
		"",
		"Use `/config set <key> <value>` to modify these settings.",
	}

	builder := core.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Configuration Options").
		Ephemeral()

	return builder.Info(ctx.Interaction, strings.Join(options, "\n"))
}

// Helpers

func emptyToDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
