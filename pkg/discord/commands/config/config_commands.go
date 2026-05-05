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

const (
	configGroupName        = "config"
	configGroupDescription = "Manage server configuration"
)

// NewConfigCommands creates a new config commands registrar.
func NewConfigCommands(configManager *files.ConfigManager) *ConfigCommands {
	return &ConfigCommands{configManager: configManager}
}

// RegisterCommands registers the /config command group and optional simple commands in the provided router.
func (cc *ConfigCommands) RegisterCommands(router *core.CommandRouter) {
	cc.RegisterBaseCommands(router)
	cc.RegisterQOTDCommands(router)
}

// RegisterBaseCommands registers the default-domain /config surface and the
// simple root commands used for bootstrap checks.
func (cc *ConfigCommands) RegisterBaseCommands(router *core.CommandRouter) {
	if router == nil {
		return
	}

	setCmd := NewConfigSetSubCommand(cc.configManager)
	getCmd := NewConfigGetSubCommand(cc.configManager)
	listCmd := NewConfigListSubCommand(cc.configManager)
	smokeTestCmd := NewSmokeTestSubCommand(cc.configManager)
	commandsEnabledCmd := NewCommandsEnabledSubCommand(cc.configManager)
	commandChannelCmd := NewCommandChannelSubCommand(cc.configManager)
	allowedRoleAddCmd := NewAllowedRoleAddSubCommand(cc.configManager)
	allowedRoleRemoveCmd := NewAllowedRoleRemoveSubCommand(cc.configManager)
	allowedRoleListCmd := NewAllowedRoleListSubCommand(cc.configManager)
	webhookCatalog := newWebhookEmbedInteractionCatalog(cc.configManager)
	pingCmd := NewPingCommand()
	echoCmd := NewEchoCommand()

	cc.registerConfigSubcommands(router, "", append([]core.SubCommand{
		setCmd,
		getCmd,
		listCmd,
		smokeTestCmd,
		commandsEnabledCmd,
		commandChannelCmd,
		allowedRoleAddCmd,
		allowedRoleRemoveCmd,
		allowedRoleListCmd,
	}, webhookCatalog.subcommands()...)...)

	// Optionally register simple commands (useful for quick health checks of the routing stack)
	router.RegisterSlashCommand(pingCmd)
	router.RegisterSlashCommand(echoCmd)
}

// RegisterQOTDCommands registers the qotd-scoped /config subcommands.
func (cc *ConfigCommands) RegisterQOTDCommands(router *core.CommandRouter) {
	if router == nil {
		return
	}

	cc.registerConfigSubcommands(router, files.BotDomainQOTD,
		NewQOTDGetSubCommand(cc.configManager),
		NewQOTDEnabledSubCommand(cc.configManager),
		NewQOTDChannelSubCommand(cc.configManager),
		NewQOTDScheduleSubCommand(cc.configManager),
	)
}

func (cc *ConfigCommands) registerConfigSubcommands(router *core.CommandRouter, domain string, subcommands ...core.SubCommand) {
	if router == nil || len(subcommands) == 0 {
		return
	}

	var group *core.GroupCommand
	if existing, ok := router.GetRegistry().GetCommand(configGroupName); ok {
		if g, ok := existing.(*core.GroupCommand); ok {
			group = g
		}
	}

	if group == nil {
		checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
		group = core.NewGroupCommand(configGroupName, configGroupDescription, checker)
		for _, subcommand := range subcommands {
			if subcommand == nil {
				continue
			}
			group.AddSubCommand(subcommand)
		}
		router.RegisterSlashCommandForDomain(domain, group)
		return
	}

	for _, subcommand := range subcommands {
		if subcommand == nil {
			continue
		}
		group.AddSubCommand(subcommand)
		router.RegisterSlashSubCommandForDomain(domain, configGroupName, subcommand)
	}
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
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "Bot is responding normally.")
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
		return builder.Ephemeral().Info(
			ctx.Interaction,
			fmt.Sprintf("Here is the requested echo text. This reply stays private because echo output is usually only useful to the person who ran the command: %s", message),
		)
	}
	return builder.Info(ctx.Interaction, fmt.Sprintf("Echo from this command: %s", message))
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
		return configCommandDetailedCommandError("That change couldn't be saved. This reply stays private so it can be adjusted and retried without extra channel noise.")
	}

	return configCommandShortConfirmationResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Configuration `%s` is now set to `%s`.", key, value))
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
	commandsEnabled := false
	if snapshot := ctx.Config.Config(); snapshot != nil {
		commandsEnabled = snapshot.ResolveFeatures(ctx.GuildID).Services.Commands
	}
	b.WriteString(fmt.Sprintf("Commands Enabled: %t\n", commandsEnabled))
	b.WriteString(fmt.Sprintf("Command Channel: %s\n", emptyToDash(ctx.GuildConfig.Channels.Commands)))
	b.WriteString(fmt.Sprintf("Avatar Logging: %s\n", emptyToDash(ctx.GuildConfig.Channels.AvatarLogging)))
	b.WriteString(fmt.Sprintf("Role Update: %s\n", emptyToDash(ctx.GuildConfig.Channels.RoleUpdate)))
	b.WriteString(fmt.Sprintf("Member Join: %s\n", emptyToDash(ctx.GuildConfig.Channels.MemberJoin)))
	b.WriteString(fmt.Sprintf("Member Leave: %s\n", emptyToDash(ctx.GuildConfig.Channels.MemberLeave)))
	b.WriteString(fmt.Sprintf("Message Edit: %s\n", emptyToDash(ctx.GuildConfig.Channels.MessageEdit)))
	b.WriteString(fmt.Sprintf("Message Delete: %s\n", emptyToDash(ctx.GuildConfig.Channels.MessageDelete)))
	b.WriteString(fmt.Sprintf("Automod Action: %s\n", emptyToDash(ctx.GuildConfig.Channels.AutomodAction)))
	b.WriteString(fmt.Sprintf("Moderation Case: %s\n", emptyToDash(ctx.GuildConfig.Channels.ModerationCase)))
	b.WriteString(fmt.Sprintf("Entry Backfill: %s\n", emptyToDash(ctx.GuildConfig.Channels.EntryBackfill)))
	b.WriteString(fmt.Sprintf("Verification Cleanup: %s\n", emptyToDash(ctx.GuildConfig.Channels.VerificationCleanup)))
	qotdSettings := files.DashboardQOTDConfig(ctx.GuildConfig.QOTD)
	qotdDeck, _ := qotdSettings.ActiveDeck()
	qotdEnabled := false
	qotdChannel := ""
	if qotdDeck.ID != "" {
		qotdEnabled = qotdDeck.Enabled
		qotdChannel = qotdDeck.ChannelID
	}
	b.WriteString(fmt.Sprintf("QOTD Enabled: %t\n", qotdEnabled))
	b.WriteString(fmt.Sprintf("QOTD Channel: %s\n", emptyToDash(qotdChannel)))
	b.WriteString(fmt.Sprintf("QOTD Schedule (UTC): %s\n", formatQOTDSchedule(qotdSettings.Schedule)))
	b.WriteString(fmt.Sprintf("Allowed Roles: %d configured\n", len(ctx.GuildConfig.Roles.Allowed)))

	builder := configCommandCurrentStateResponseBuilder(ctx.Session).
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
		"`/config smoke_test` - Show bootstrap readiness for general config and QOTD",
		"`/config commands_enabled <enabled>` - Enable or disable slash command handling for this guild",
		"`/config command_channel <channel>` - Set the channel used for command routing or references",
		"`/config allowed_role_add <role>` - Allow one role to use admin-level slash commands",
		"`/config allowed_role_remove <role>` - Remove one allowed admin role",
		"`/config allowed_role_list` - Show the current allowed admin roles",
		"",
		"`channels.commands` - Channel for bot commands",
		"`channels.avatar_logging` - Channel for avatar change logs",
		"`channels.role_update` - Channel for role update logs",
		"`channels.member_join` - Channel for member join logs",
		"`channels.member_leave` - Channel for member leave logs",
		"`channels.message_edit` - Channel for message edit logs",
		"`channels.message_delete` - Channel for message delete logs",
		"`channels.automod_action` - Channel for automod action logs",
		"`channels.moderation_case` - Dedicated channel for moderation case logs",
		"`channels.entry_backfill` - Channel used by entry/leave backfill",
		"`channels.verification_cleanup` - Channel used for verification cleanup routines",
		"",
		"Use `/config set <key> <value>` to modify these settings.",
		"",
		"`/config qotd_schedule <hour> <minute>` - Set the QOTD publish schedule in UTC",
		"`/config qotd_get` - Show the current reduced QOTD configuration",
		"`/config qotd_enabled <enabled>` - Enable or disable QOTD publishing for the active deck",
		"`/config qotd_channel <channel>` - Set the QOTD delivery channel for the active deck",
		"",
		"`/config webhook_embed_create` - Add webhook embed patch entry",
		"`/config webhook_embed_read` - Show one webhook embed patch entry",
		"`/config webhook_embed_update` - Update existing webhook embed patch entry",
		"`/config webhook_embed_delete` - Delete webhook embed patch entry",
		"`/config webhook_embed_list` - List webhook embed patch entries",
	}

	builder := configCommandAvailableOptionsResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Configuration Options")

	return builder.Info(ctx.Interaction, strings.Join(options, "\n"))
}

// Helpers

func emptyToDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func formatQOTDSchedule(schedule files.QOTDPublishScheduleConfig) string {
	hourUTC, minuteUTC, ok := schedule.Values()
	if ok {
		return fmt.Sprintf("%02d:%02d", hourUTC, minuteUTC)
	}
	hour := "--"
	minute := "--"
	if schedule.HourUTC != nil {
		hour = fmt.Sprintf("%02d", *schedule.HourUTC)
	}
	if schedule.MinuteUTC != nil {
		minute = fmt.Sprintf("%02d", *schedule.MinuteUTC)
	}
	if schedule.IsZero() {
		return "—"
	}
	return hour + ":" + minute
}
