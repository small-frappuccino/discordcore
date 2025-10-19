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

func NewPingCommand() *PingCommand {
	return &PingCommand{}
}

func (c *PingCommand) Name() string {
	return "ping"
}

func (c *PingCommand) Description() string {
	return "Check if the bot is responding"
}

func (c *PingCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil // Simple command without options
}

func (c *PingCommand) RequiresGuild() bool {
	return false // Can be used in DM
}

func (c *PingCommand) RequiresPermissions() bool {
	return false // Everyone can use
}

func (c *PingCommand) Handle(ctx *Context) error {
	return NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "🏓 Pong!")
}

// ======================
// Example 2: Command with Options
// ======================

// EchoCommand demonstrates how to use options and data extraction
type EchoCommand struct{}

func NewEchoCommand() *EchoCommand {
	return &EchoCommand{}
}

func (c *EchoCommand) Name() string {
	return "echo"
}

func (c *EchoCommand) Description() string {
	return "Echo back a message"
}

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

func (c *EchoCommand) RequiresGuild() bool {
	return false
}

func (c *EchoCommand) RequiresPermissions() bool {
	return false
}

func (c *EchoCommand) Handle(ctx *Context) error {
	// Extract command options
	extractor := NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)

	message, err := extractor.StringRequired("message")
	if err != nil {
		return err
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

func NewUserInfoSubCommand() *UserInfoSubCommand {
	return &UserInfoSubCommand{}
}

func (c *UserInfoSubCommand) Name() string {
	return "info"
}

func (c *UserInfoSubCommand) Description() string {
	return "Get information about a user"
}

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

func (c *UserInfoSubCommand) RequiresGuild() bool {
	return true // Requires a server to access member information
}

func (c *UserInfoSubCommand) RequiresPermissions() bool {
	return false
}

func (c *UserInfoSubCommand) Handle(ctx *Context) error {
	extractor := NewOptionExtractor(GetSubCommandOptions(ctx.Interaction))

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

func NewConfigGroupCommand(session *discordgo.Session, configManager *files.ConfigManager) *ConfigGroupCommand {
	checker := NewPermissionChecker(session, configManager)

	group := NewGroupCommand("config", "Manage server configuration", checker)

	// Add subcommands
	group.AddSubCommand(NewConfigSetSubCommand(configManager))
	group.AddSubCommand(NewConfigGetSubCommand(configManager))
	group.AddSubCommand(NewConfigListSubCommand(configManager))

	return &ConfigGroupCommand{GroupCommand: group}
}

// ConfigSetSubCommand - subcommand to set configuration values
type ConfigSetSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigSetSubCommand(configManager *files.ConfigManager) *ConfigSetSubCommand {
	return &ConfigSetSubCommand{configManager: configManager}
}

func (c *ConfigSetSubCommand) Name() string {
	return "set"
}

func (c *ConfigSetSubCommand) Description() string {
	return "Set a configuration value"
}

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

func (c *ConfigSetSubCommand) RequiresGuild() bool {
	return true
}

func (c *ConfigSetSubCommand) RequiresPermissions() bool {
	return true // Only users with permission can change configuration
}

func (c *ConfigSetSubCommand) Handle(ctx *Context) error {
	extractor := NewOptionExtractor(GetSubCommandOptions(ctx.Interaction))

	key, err := extractor.StringRequired("key")
	if err != nil {
		return err
	}

	value, err := extractor.StringRequired("value")
	if err != nil {
		return err
	}

	// Use SafeGuildAccess for safe configuration manipulation
	err = SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		switch key {
		case "command_channel":
			guildConfig.CommandChannelID = value
		case "log_channel":
			guildConfig.UserLogChannelID = value
		case "automod_channel":
			guildConfig.AutomodLogChannelID = value
		default:
			return NewValidationError("key", "Invalid configuration key")
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Persist configuration
	persister := NewConfigPersister(c.configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save config: %v", err)
		return NewCommandError("Failed to save configuration", true)
	}

	return NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Configuration `%s` set to `%s`", key, value))
}

// ConfigGetSubCommand - subcommand to get configurations
type ConfigGetSubCommand struct {
	configManager *files.ConfigManager
}

func NewConfigGetSubCommand(configManager *files.ConfigManager) *ConfigGetSubCommand {
	return &ConfigGetSubCommand{configManager: configManager}
}

func (c *ConfigGetSubCommand) Name() string {
	return "get"
}

func (c *ConfigGetSubCommand) Description() string {
	return "Get current configuration values"
}

func (c *ConfigGetSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

func (c *ConfigGetSubCommand) RequiresGuild() bool {
	return true
}

func (c *ConfigGetSubCommand) RequiresPermissions() bool {
	return true
}

func (c *ConfigGetSubCommand) Handle(ctx *Context) error {
	if err := RequiresGuildConfig(ctx); err != nil {
		return err
	}

	var config strings.Builder
	config.WriteString("**Server Configuration:**\n")
	config.WriteString(fmt.Sprintf("Command Channel: %s\n", ctx.GuildConfig.CommandChannelID))
	config.WriteString(fmt.Sprintf("Log Channel: %s\n", ctx.GuildConfig.UserLogChannelID))
	config.WriteString(fmt.Sprintf("Automod Channel: %s\n", ctx.GuildConfig.AutomodLogChannelID))
	config.WriteString(fmt.Sprintf("Allowed Roles: %d configured\n", len(ctx.GuildConfig.AllowedRoles)))

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

func NewConfigListSubCommand(configManager *files.ConfigManager) *ConfigListSubCommand {
	return &ConfigListSubCommand{configManager: configManager}
}

func (c *ConfigListSubCommand) Name() string {
	return "list"
}

func (c *ConfigListSubCommand) Description() string {
	return "List all available configuration options"
}

func (c *ConfigListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

func (c *ConfigListSubCommand) RequiresGuild() bool {
	return true
}

func (c *ConfigListSubCommand) RequiresPermissions() bool {
	return true
}

func (c *ConfigListSubCommand) Handle(ctx *Context) error {
	options := []string{
		"**Available Configuration Options:**",
		"`command_channel` - Channel for bot commands",
		"`log_channel` - Channel for user logs",
		"`automod_channel` - Channel for automod logs",
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

func NewConfigAutocompleteHandler(configManager *files.ConfigManager) *ConfigAutocompleteHandler {
	return &ConfigAutocompleteHandler{configManager: configManager}
}

func (h *ConfigAutocompleteHandler) HandleAutocomplete(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	switch focusedOption {
	case "key":
		return []*discordgo.ApplicationCommandOptionChoice{
			{Name: "Command Channel", Value: "command_channel"},
			{Name: "Log Channel", Value: "log_channel"},
			{Name: "Automod Channel", Value: "automod_channel"},
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
	router.RegisterCommand(NewPingCommand())
	router.RegisterCommand(NewEchoCommand())

	// Register group command
	configCmd := NewConfigGroupCommand(session, configManager)
	router.RegisterCommand(configCmd)

	// Register autocomplete
	router.RegisterAutocomplete("config", NewConfigAutocompleteHandler(configManager))

	// Sync commands with Discord
	return manager.SetupCommands()
}

// ======================
// Example 7: Advanced Error Handling
// ======================

// AdvancedCommand demonstrates robust error handling
type AdvancedCommand struct{}

func NewAdvancedCommand() *AdvancedCommand {
	return &AdvancedCommand{}
}

func (c *AdvancedCommand) Name() string {
	return "advanced"
}

func (c *AdvancedCommand) Description() string {
	return "Demonstrates advanced error handling"
}

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

func (c *AdvancedCommand) RequiresGuild() bool {
	return true
}

func (c *AdvancedCommand) RequiresPermissions() bool {
	return false
}

func (c *AdvancedCommand) Handle(ctx *Context) error {
	extractor := NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)

	input, err := extractor.StringRequired("input")
	if err != nil {
		return err // Validation error will be handled automatically
	}

	// Custom validations
	stringUtils := StringUtils{}
	if err := stringUtils.ValidateStringLength(input, 1, 100, "input"); err != nil {
		return err
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
