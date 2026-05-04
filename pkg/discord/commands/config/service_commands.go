package config

import (
	"fmt"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	commandsEnabledSubCommandName = "commands_enabled"
	commandChannelSubCommandName  = "command_channel"
	allowedRoleAddSubCommandName    = "allowed_role_add"
	allowedRoleRemoveSubCommandName = "allowed_role_remove"
	allowedRoleListSubCommandName   = "allowed_role_list"
	commandEnabledOptionName      = "enabled"
	commandChannelOptionName      = "channel"
	allowedRoleOptionName         = "role"
)

type CommandsEnabledSubCommand struct {
	configManager *files.ConfigManager
}

func NewCommandsEnabledSubCommand(configManager *files.ConfigManager) *CommandsEnabledSubCommand {
	return &CommandsEnabledSubCommand{configManager: configManager}
}

func (c *CommandsEnabledSubCommand) Name() string { return commandsEnabledSubCommandName }
func (c *CommandsEnabledSubCommand) Description() string {
	return "Enable or disable slash command handling for this guild"
}
func (c *CommandsEnabledSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionBoolean,
		Name:        commandEnabledOptionName,
		Description: "Whether slash commands should be handled for this guild",
		Required:    true,
	}}
}
func (c *CommandsEnabledSubCommand) RequiresGuild() bool       { return true }
func (c *CommandsEnabledSubCommand) RequiresPermissions() bool { return true }
func (c *CommandsEnabledSubCommand) Handle(ctx *core.Context) error {
	enabled := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction)).Bool(commandEnabledOptionName)
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		guildConfig.Features.Services.Commands = boolPtr(enabled)
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	return serviceConfigShortConfirmationResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Slash commands are now %s in this server.", state))
}

type CommandChannelSubCommand struct {
	configManager *files.ConfigManager
}

func NewCommandChannelSubCommand(configManager *files.ConfigManager) *CommandChannelSubCommand {
	return &CommandChannelSubCommand{configManager: configManager}
}

func (c *CommandChannelSubCommand) Name() string { return commandChannelSubCommandName }
func (c *CommandChannelSubCommand) Description() string {
	return "Set the command channel for this guild"
}
func (c *CommandChannelSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:         discordgo.ApplicationCommandOptionChannel,
		Name:         commandChannelOptionName,
		Description:  "Existing text channel used for command references",
		Required:     true,
		ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildText},
	}}
}
func (c *CommandChannelSubCommand) RequiresGuild() bool       { return true }
func (c *CommandChannelSubCommand) RequiresPermissions() bool { return true }
func (c *CommandChannelSubCommand) Handle(ctx *core.Context) error {
	channelID := channelOptionID(ctx.Session, core.GetSubCommandOptions(ctx.Interaction), commandChannelOptionName)
	if channelID == "" {
		return serviceConfigDetailedCommandError("This change needs a channel before it can be applied, so this reply stays private.")
	}
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		guildConfig.Channels.Commands = channelID
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	return serviceConfigShortConfirmationResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Command references will now point people to <#%s>.", channelID))
}

type AllowedRoleAddSubCommand struct {
	configManager *files.ConfigManager
}

func NewAllowedRoleAddSubCommand(configManager *files.ConfigManager) *AllowedRoleAddSubCommand {
	return &AllowedRoleAddSubCommand{configManager: configManager}
}

func (c *AllowedRoleAddSubCommand) Name() string { return allowedRoleAddSubCommandName }
func (c *AllowedRoleAddSubCommand) Description() string {
	return "Allow one role to use admin-level slash commands"
}
func (c *AllowedRoleAddSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionRole,
		Name:        allowedRoleOptionName,
		Description: "Role to allow for admin-level slash commands",
		Required:    true,
	}}
}
func (c *AllowedRoleAddSubCommand) RequiresGuild() bool       { return true }
func (c *AllowedRoleAddSubCommand) RequiresPermissions() bool { return true }
func (c *AllowedRoleAddSubCommand) Handle(ctx *core.Context) error {
	roleID := roleOptionID(core.GetSubCommandOptions(ctx.Interaction), allowedRoleOptionName)
	if roleID == "" {
		return serviceConfigDetailedCommandError("This change needs a role before it can be applied, so this reply stays private.")
	}
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		if slices.Contains(guildConfig.Roles.Allowed, roleID) {
			return nil
		}
		guildConfig.Roles.Allowed = append(guildConfig.Roles.Allowed, roleID)
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	return serviceConfigShortConfirmationResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("<@&%s> can now use the admin slash commands.", roleID))
}

type AllowedRoleRemoveSubCommand struct {
	configManager *files.ConfigManager
}

func NewAllowedRoleRemoveSubCommand(configManager *files.ConfigManager) *AllowedRoleRemoveSubCommand {
	return &AllowedRoleRemoveSubCommand{configManager: configManager}
}

func (c *AllowedRoleRemoveSubCommand) Name() string { return allowedRoleRemoveSubCommandName }
func (c *AllowedRoleRemoveSubCommand) Description() string {
	return "Remove one allowed admin role"
}
func (c *AllowedRoleRemoveSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionRole,
		Name:        allowedRoleOptionName,
		Description: "Allowed role to remove",
		Required:    true,
	}}
}
func (c *AllowedRoleRemoveSubCommand) RequiresGuild() bool       { return true }
func (c *AllowedRoleRemoveSubCommand) RequiresPermissions() bool { return true }
func (c *AllowedRoleRemoveSubCommand) Handle(ctx *core.Context) error {
	roleID := roleOptionID(core.GetSubCommandOptions(ctx.Interaction), allowedRoleOptionName)
	if roleID == "" {
		return serviceConfigDetailedCommandError("This change needs a role before it can be applied, so this reply stays private.")
	}
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		guildConfig.Roles.Allowed = removeString(guildConfig.Roles.Allowed, roleID)
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	return serviceConfigShortConfirmationResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("<@&%s> can no longer use the admin slash commands.", roleID))
}

type AllowedRoleListSubCommand struct {
	configManager *files.ConfigManager
}

func NewAllowedRoleListSubCommand(configManager *files.ConfigManager) *AllowedRoleListSubCommand {
	return &AllowedRoleListSubCommand{configManager: configManager}
}

func (c *AllowedRoleListSubCommand) Name() string { return allowedRoleListSubCommandName }
func (c *AllowedRoleListSubCommand) Description() string {
	return "List the roles allowed to use admin-level slash commands"
}
func (c *AllowedRoleListSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }
func (c *AllowedRoleListSubCommand) RequiresGuild() bool       { return true }
func (c *AllowedRoleListSubCommand) RequiresPermissions() bool { return true }
func (c *AllowedRoleListSubCommand) Handle(ctx *core.Context) error {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return err
	}
	if len(ctx.GuildConfig.Roles.Allowed) == 0 {
		return serviceConfigSetupStateResponseBuilder(ctx.Session).Info(ctx.Interaction, "No roles have admin slash command access yet. This reply stays private because it reflects the current server setup.")
	}
	roles := make([]string, 0, len(ctx.GuildConfig.Roles.Allowed))
	for _, roleID := range ctx.GuildConfig.Roles.Allowed {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		roles = append(roles, fmt.Sprintf("- <@&%s>", roleID))
	}
	if len(roles) == 0 {
		return serviceConfigSetupStateResponseBuilder(ctx.Session).Info(ctx.Interaction, "No roles have admin slash command access yet. This reply stays private because it reflects the current server setup.")
	}
	message := "These roles can use the admin slash commands. This reply stays private because it reflects the current server setup:\n" + strings.Join(roles, "\n")
	return serviceConfigSetupStateResponseBuilder(ctx.Session).Info(ctx.Interaction, message)
}

func persistGuildConfig(ctx *core.Context, configManager *files.ConfigManager) error {
	persister := core.NewConfigPersister(configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save config: %v", err)
		return serviceConfigDetailedCommandError("That change couldn't be saved. This reply stays private so it can be adjusted and retried without extra channel noise.")
	}
	return nil
}

func roleOptionID(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option == nil || option.Name != name {
			continue
		}
		if value, ok := option.Value.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func removeString(values []string, target string) []string {
	target = strings.TrimSpace(target)
	filtered := values[:0]
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			continue
		}
		filtered = append(filtered, value)
	}
	return filtered
}

func boolPtr(value bool) *bool {
	return &value
}