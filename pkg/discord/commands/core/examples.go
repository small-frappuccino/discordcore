package core

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// This file contains examples of how to use the core infrastructure
// to create Discord commands in a modular and reusable way.

// ======================
// Example 1: Simple Command
// ======================

// PingCommand is a simple command example
type PingCommand struct{}

// Name names.
func (c *PingCommand) Name() string {
	return "ping"
}

// Description descriptions.
func (c *PingCommand) Description() string {
	return "Check if the bot is responding"
}

// Options options.
func (c *PingCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil // Simple command without options
}

// RequiresGuild requires guild.
func (c *PingCommand) RequiresGuild() bool {
	return false // Can be used in DM
}

// RequiresPermissions requires permissions.
func (c *PingCommand) RequiresPermissions() bool {
	return false // Everyone can use
}

// Handle handles.
func (c *PingCommand) Handle(ctx *Context) error {
	return NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "🏓 Pong!")
}

// ======================
// Example 2: Command with Options
// ======================

// EchoCommand demonstrates how to use options and data extraction
type EchoCommand struct{}

// Name names.
func (c *EchoCommand) Name() string {
	return "echo"
}

// Description descriptions.
func (c *EchoCommand) Description() string {
	return "Echo back a message"
}

// Options options.
func (c *EchoCommand) Options() []*discordgo.ApplicationCommandOption {
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

// RequiresGuild requires guild.
func (c *EchoCommand) RequiresGuild() bool {
	return false
}

// RequiresPermissions requires permissions.
func (c *EchoCommand) RequiresPermissions() bool {
	return false
}

// Handle handles.
func (c *EchoCommand) Handle(ctx *Context) error {
	// Extract command options
	extractor := OptionList(ctx.Interaction.ApplicationCommandData().Options)

	message, err := extractor.StringRequired("message")
	if err != nil {
		return fmt.Errorf("EchoCommand.Handle: %w", err)
	}

	ephemeral := extractor.Bool("ephemeral")

	// Use ResponseBuilder for a more flexible response
	builder := NewResponseBuilder(ctx.Session)
	if ephemeral {
		builder = builder.Ephemeral()
	}

	return builder.Info(ctx.Interaction, fmt.Sprintf("Echo: %s", message))
}

// ======================
// Example 3: Subcommand
// ======================

// UserInfoSubCommand demonstrates a subcommand implementation
type UserInfoSubCommand struct{}

// Name names.
func (c *UserInfoSubCommand) Name() string {
	return "info"
}

// Description descriptions.
func (c *UserInfoSubCommand) Description() string {
	return "Get information about a user"
}

// Options options.
func (c *UserInfoSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        "user",
			Description: "User to get info about",
			Required:    false,
		},
	}
}

// RequiresGuild requires guild.
func (c *UserInfoSubCommand) RequiresGuild() bool {
	return true // Requires a server to access member information
}

// RequiresPermissions requires permissions.
func (c *UserInfoSubCommand) RequiresPermissions() bool {
	return false
}

// Handle handles.
func (c *UserInfoSubCommand) Handle(ctx *Context) error {
	extractor := OptionList(GetSubCommandOptions(ctx.Interaction))

	// If no user is specified, use the command author
	var targetUser *discordgo.User
	if extractor.HasOption("user") {
		// Logic to extract the user from the option
		targetUser = ctx.Interaction.Member.User
	} else {
		targetUser = ctx.Interaction.Member.User
	}

	// Create an embed with user information
	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("User Information").
		WithTimestamp()

	message := fmt.Sprintf("**Username:** %s\n**ID:** %s", targetUser.Username, targetUser.ID)

	return builder.Info(ctx.Interaction, message)
}

// ======================
// Example 4: Group Command with Multiple Subcommands
// ======================

// ConfigGroupCommand demonstrates how to create a command with multiple subcommands
type ConfigGroupCommand struct {
	*GroupCommand
}

// NewConfigGroupCommand news config group command.
func NewConfigGroupCommand(session *discordgo.Session, configManager *files.ConfigManager) *ConfigGroupCommand {
	checker := NewPermissionChecker(session, configManager)

	group := NewGroupCommand("config", "Manage server configuration", checker)

	// Add subcommands
	group.AddSubCommand(&ConfigSetSubCommand{configManager: configManager})
	group.AddSubCommand(&ConfigGetSubCommand{configManager: configManager})
	group.AddSubCommand(&ConfigListSubCommand{configManager: configManager})

	return &ConfigGroupCommand{GroupCommand: group}
}

// ConfigSetSubCommand - subcommand to set configuration values
type ConfigSetSubCommand struct {
	configManager *files.ConfigManager
}

// Name names.
func (c *ConfigSetSubCommand) Name() string {
	return "set"
}

// Description descriptions.
func (c *ConfigSetSubCommand) Description() string {
	return "Set a configuration value"
}

// Options options.
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
				{Name: "channels.automod_action", Value: "channels.automod_action"},
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

// RequiresGuild requires guild.
func (c *ConfigSetSubCommand) RequiresGuild() bool {
	return true
}

// RequiresPermissions requires permissions.
func (c *ConfigSetSubCommand) RequiresPermissions() bool {
	return true // Only users with permission can change configuration
}

// Handle handles.
func (c *ConfigSetSubCommand) Handle(ctx *Context) error {
	extractor := OptionList(GetSubCommandOptions(ctx.Interaction))

	key, err := extractor.StringRequired("key")
	if err != nil {
		return fmt.Errorf("ConfigSetSubCommand.Handle: %w", err)
	}

	value, err := extractor.StringRequired("value")
	if err != nil {
		return fmt.Errorf("ConfigSetSubCommand.Handle: %w", err)
	}

	// Use SafeGuildAccess for safe configuration manipulation
	err = SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		switch key {
		case "channels.commands":
			guildConfig.Channels.Commands = value
		case "channels.avatar_logging":
			guildConfig.Channels.AvatarLogging = value
		case "channels.automod_action":
			guildConfig.Channels.AutomodAction = value
		default:
			return &ValidationError{Field: "key", Message: "Invalid configuration key"}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("ConfigSetSubCommand.Handle: %w", err)
	}

	// Persist configuration
	if err := c.configManager.SaveGuildConfig(*ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save config: %v", err)
		return NewCommandError("Failed to save configuration", true)
	}

	return NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Configuration `%s` set to `%s`", key, value))
}

// ConfigGetSubCommand - subcommand to get configurations
type ConfigGetSubCommand struct {
	configManager *files.ConfigManager
}

// Name names.
func (c *ConfigGetSubCommand) Name() string {
	return "get"
}

// Description descriptions.
func (c *ConfigGetSubCommand) Description() string {
	return "Get current configuration values"
}

// Options options.
func (c *ConfigGetSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

// RequiresGuild requires guild.
func (c *ConfigGetSubCommand) RequiresGuild() bool {
	return true
}

// RequiresPermissions requires permissions.
func (c *ConfigGetSubCommand) RequiresPermissions() bool {
	return true
}

// Handle handles.
func (c *ConfigGetSubCommand) Handle(ctx *Context) error {
	if err := RequiresGuildConfig(ctx); err != nil {
		return fmt.Errorf("ConfigGetSubCommand.Handle: %w", err)
	}

	var config strings.Builder
	config.WriteString("**Server Configuration:**\n")
	config.WriteString(fmt.Sprintf("Command Channel: %s\n", ctx.GuildConfig.Channels.Commands))
	config.WriteString(fmt.Sprintf("Avatar Logging: %s\n", ctx.GuildConfig.Channels.AvatarLogging))
	config.WriteString(fmt.Sprintf("Automod Action: %s\n", ctx.GuildConfig.Channels.AutomodAction))
	config.WriteString(fmt.Sprintf("Allowed Roles: %d configured\n", len(ctx.GuildConfig.Roles.Allowed)))

	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Server Configuration").
		WithColor(theme.Info())

	return builder.Info(ctx.Interaction, config.String())
}

// ConfigListSubCommand - subcommand to list all configurations
type ConfigListSubCommand struct {
	configManager *files.ConfigManager
}

// Name names.
func (c *ConfigListSubCommand) Name() string {
	return "list"
}

// Description descriptions.
func (c *ConfigListSubCommand) Description() string {
	return "List all available configuration options"
}

// Options options.
func (c *ConfigListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

// RequiresGuild requires guild.
func (c *ConfigListSubCommand) RequiresGuild() bool {
	return true
}

// RequiresPermissions requires permissions.
func (c *ConfigListSubCommand) RequiresPermissions() bool {
	return true
}

// Handle handles.
func (c *ConfigListSubCommand) Handle(ctx *Context) error {
	options := []string{
		"**Available Configuration Options:**",
		"`channels.commands` - Channel for bot commands",
		"`channels.avatar_logging` - Channel for avatar logs",
		"`channels.automod_action` - Channel for automod logs",
		"",
		"Use `/config set <key> <value>` to modify these settings.",
	}

	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Configuration Options").
		Ephemeral()

	return builder.Info(ctx.Interaction, strings.Join(options, "\n"))
}

// ======================
// Example 5: Autocomplete Handler
// ======================

// ConfigAutocompleteHandler demonstrates an autocomplete implementation
type ConfigAutocompleteHandler struct {
	configManager *files.ConfigManager
}

// HandleAutocomplete handles autocomplete.
func (h *ConfigAutocompleteHandler) HandleAutocomplete(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	switch focusedOption {
	case "key":
		return []*discordgo.ApplicationCommandOptionChoice{
			{Name: "Command Channel", Value: "channels.commands"},
			{Name: "Avatar Log", Value: "channels.avatar_logging"},
			{Name: "Automod Channel", Value: "channels.automod_action"},
		}, nil

	case "value":
		// Autocomplete based on the selected key
		// This would require additional logic to detect the key's value
		return h.getValueChoices(ctx)

	default:
		return []*discordgo.ApplicationCommandOptionChoice{}, nil
	}
}

func (h *ConfigAutocompleteHandler) getValueChoices(ctx *Context) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	// Example: suggest server channels
	channels, err := ctx.Session.GuildChannels(ctx.GuildID)
	if err != nil {
		return []*discordgo.ApplicationCommandOptionChoice{}, nil
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			choice := &discordgo.ApplicationCommandOptionChoice{
				Name:  "#" + channel.Name,
				Value: channel.ID,
			}
			choices = append(choices, choice)
		}
	}

	return choices, nil
}

// ======================
// Example 6: How to Register Everything
// ======================

// ExampleCommandSetup demonstrates how to set up all commands
func ExampleCommandSetup(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) error {
	// Create the command manager
	manager := NewCommandManager(session, configManager)
	router := manager.GetRouter()

	// Register simple commands
	router.RegisterCommand(&PingCommand{})
	router.RegisterCommand(&EchoCommand{})

	// Register group command
	configCmd := NewConfigGroupCommand(session, configManager)
	router.RegisterCommand(configCmd)

	// Register autocomplete
	router.RegisterAutocomplete("config", &ConfigAutocompleteHandler{configManager: configManager})

	// Sync commands with Discord
	return manager.SetupCommands()
}

// ======================
// Example 7: Advanced Error Handling
// ======================

// AdvancedCommand demonstrates robust error handling
type AdvancedCommand struct{}

// Name names.
func (c *AdvancedCommand) Name() string {
	return "advanced"
}

// Description descriptions.
func (c *AdvancedCommand) Description() string {
	return "Demonstrates advanced error handling"
}

// Options options.
func (c *AdvancedCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "input",
			Description: "Some input to validate",
			Required:    true,
		},
	}
}

// RequiresGuild requires guild.
func (c *AdvancedCommand) RequiresGuild() bool {
	return true
}

// RequiresPermissions requires permissions.
func (c *AdvancedCommand) RequiresPermissions() bool {
	return false
}

// Handle handles.
func (c *AdvancedCommand) Handle(ctx *Context) error {
	extractor := OptionList(ctx.Interaction.ApplicationCommandData().Options)

	input, err := extractor.StringRequired("input")
	if err != nil {
		return fmt.Errorf("AdvancedCommand.Handle: %w", err) // Validation error will be handled automatically
	}

	// Custom validations
	if err := ValidateStringLength(input, 1, 100, "input"); err != nil {
		return fmt.Errorf("AdvancedCommand.Handle: %w", err)
	}

	// Operation that may fail
	result, err := c.processInput(input)
	if err != nil {
		// Log the error
		ctx.Logger.Error().Errorf("Failed to process input: %v", err)

		// Return a user-friendly error
		return NewCommandError("Failed to process your input. Please try again.", true)
	}

	// Success response
	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Processing Complete").
		WithTimestamp()

	return builder.Success(ctx.Interaction, fmt.Sprintf("Result: %s", result))
}

func (c *AdvancedCommand) processInput(input string) (string, error) {
	// Simulate processing that may fail
	if strings.Contains(input, "error") {
		return "", fmt.Errorf("input contains forbidden word")
	}
	return strings.ToUpper(input), nil
}
