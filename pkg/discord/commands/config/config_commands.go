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
	if router == nil {
		return
	}

	// Build /config group with permission checks (or reuse existing group).
	var group *core.GroupCommand
	if existing, ok := router.GetRegistry().GetCommand("config"); ok {
		if g, ok := existing.(*core.GroupCommand); ok {
			group = g
		}
	}
	if group == nil {
		checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
		group = core.NewGroupCommand("config", "Manage server configuration", checker)
	}

	// Attach subcommands
	group.AddSubCommand(NewConfigSetSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigGetSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigListSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigWebhookEmbedCreateSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigWebhookEmbedReadSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigWebhookEmbedUpdateSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigWebhookEmbedDeleteSubCommand(cc.configManager))
	group.AddSubCommand(NewConfigWebhookEmbedListSubCommand(cc.configManager))

	// Register (or refresh) the group.
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
				{Name: "channels.commands", Value: "channels.commands"},
				{Name: "channels.avatar_logging", Value: "channels.avatar_logging"},
				{Name: "channels.role_update", Value: "channels.role_update"},
				{Name: "channels.member_join", Value: "channels.member_join"},
				{Name: "channels.member_leave", Value: "channels.member_leave"},
				{Name: "channels.message_edit", Value: "channels.message_edit"},
				{Name: "channels.message_delete", Value: "channels.message_delete"},
				{Name: "channels.automod_action", Value: "channels.automod_action"},
				{Name: "channels.moderation_case", Value: "channels.moderation_case"},
				{Name: "channels.clean_action", Value: "channels.clean_action"},
				{Name: "channels.entry_backfill", Value: "channels.entry_backfill"},
				{Name: "channels.verification_cleanup", Value: "channels.verification_cleanup"},
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
		case "channels.commands":
			guildConfig.Channels.Commands = value
		case "channels.avatar_logging":
			guildConfig.Channels.AvatarLogging = value
		case "channels.role_update":
			guildConfig.Channels.RoleUpdate = value
		case "channels.member_join":
			guildConfig.Channels.MemberJoin = value
		case "channels.member_leave":
			guildConfig.Channels.MemberLeave = value
		case "channels.message_edit":
			guildConfig.Channels.MessageEdit = value
		case "channels.message_delete":
			guildConfig.Channels.MessageDelete = value
		case "channels.automod_action":
			guildConfig.Channels.AutomodAction = value
		case "channels.moderation_case":
			guildConfig.Channels.ModerationCase = value
		case "channels.clean_action":
			guildConfig.Channels.CleanAction = value
		case "channels.entry_backfill":
			guildConfig.Channels.EntryBackfill = value
		case "channels.verification_cleanup":
			guildConfig.Channels.VerificationCleanup = value
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
	b.WriteString(fmt.Sprintf("Command Channel: %s\n", emptyToDash(ctx.GuildConfig.Channels.Commands)))
	b.WriteString(fmt.Sprintf("Avatar Logging: %s\n", emptyToDash(ctx.GuildConfig.Channels.AvatarLogging)))
	b.WriteString(fmt.Sprintf("Role Update: %s\n", emptyToDash(ctx.GuildConfig.Channels.RoleUpdate)))
	b.WriteString(fmt.Sprintf("Member Join: %s\n", emptyToDash(ctx.GuildConfig.Channels.MemberJoin)))
	b.WriteString(fmt.Sprintf("Member Leave: %s\n", emptyToDash(ctx.GuildConfig.Channels.MemberLeave)))
	b.WriteString(fmt.Sprintf("Message Edit: %s\n", emptyToDash(ctx.GuildConfig.Channels.MessageEdit)))
	b.WriteString(fmt.Sprintf("Message Delete: %s\n", emptyToDash(ctx.GuildConfig.Channels.MessageDelete)))
	b.WriteString(fmt.Sprintf("Automod Action: %s\n", emptyToDash(ctx.GuildConfig.Channels.AutomodAction)))
	b.WriteString(fmt.Sprintf("Moderation Case: %s\n", emptyToDash(ctx.GuildConfig.Channels.ModerationCase)))
	b.WriteString(fmt.Sprintf("Clean Action: %s\n", emptyToDash(ctx.GuildConfig.Channels.CleanAction)))
	b.WriteString(fmt.Sprintf("Entry Backfill: %s\n", emptyToDash(ctx.GuildConfig.Channels.EntryBackfill)))
	b.WriteString(fmt.Sprintf("Verification Cleanup: %s\n", emptyToDash(ctx.GuildConfig.Channels.VerificationCleanup)))
	b.WriteString(fmt.Sprintf("Allowed Roles: %d configured\n", len(ctx.GuildConfig.Roles.Allowed)))

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
		"`channels.commands` - Channel for bot commands",
		"`channels.avatar_logging` - Channel for avatar change logs",
		"`channels.role_update` - Channel for role update logs",
		"`channels.member_join` - Channel for member join logs",
		"`channels.member_leave` - Channel for member leave logs",
		"`channels.message_edit` - Channel for message edit logs",
		"`channels.message_delete` - Channel for message delete logs",
		"`channels.automod_action` - Channel for automod action logs",
		"`channels.moderation_case` - Dedicated channel for moderation case logs",
		"`channels.clean_action` - Channel for /clean action logs",
		"`channels.entry_backfill` - Channel used by entry/leave backfill",
		"`channels.verification_cleanup` - Channel used for verification cleanup routines",
		"",
		"Use `/config set <key> <value>` to modify these settings.",
		"",
		"`/config webhook_embed_create` - Add webhook embed patch entry",
		"`/config webhook_embed_read` - Show one webhook embed patch entry",
		"`/config webhook_embed_update` - Update existing webhook embed patch entry",
		"`/config webhook_embed_delete` - Delete webhook embed patch entry",
		"`/config webhook_embed_list` - List webhook embed patch entries",
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
