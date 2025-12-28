package core

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// ContextBuilder creates contexts for command execution
type ContextBuilder struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	checker       *PermissionChecker
}

// NewContextBuilder creates a new context builder
func NewContextBuilder(session *discordgo.Session, configManager *files.ConfigManager, checker *PermissionChecker) *ContextBuilder {
	return &ContextBuilder{
		session:       session,
		configManager: configManager,
		checker:       checker,
	}
}

// BuildContext creates a complete context for command execution
func (cb *ContextBuilder) BuildContext(i *discordgo.InteractionCreate) *Context {
	userID := extractUserID(i)
	guildID := i.GuildID

	var guildConfig *files.GuildConfig
	if guildID != "" {
		guildConfig = cb.configManager.GuildConfig(guildID)
	}

	isOwner := false
	if guildID != "" {
		isOwner = cb.isGuildOwner(guildID, userID)
	}

	logger := log.GlobalLogger

	ctx := &Context{
		Session:     cb.session,
		Interaction: i,
		Config:      cb.configManager,
		Logger:      logger,
		GuildID:     guildID,
		UserID:      userID,
		IsOwner:     isOwner,
		GuildConfig: guildConfig,
	}

	return ctx
}

// isGuildOwner checks if the user is the server owner
func (cb *ContextBuilder) isGuildOwner(guildID, userID string) bool {
	// Prefer state cache to avoid REST calls when possible
	if cb.session != nil && cb.session.State != nil {
		if g, _ := cb.session.State.Guild(guildID); g != nil {
			return g.OwnerID == userID
		}
	}
	// Fallback to REST only if necessary
	guild, err := cb.session.Guild(guildID)
	if err != nil || guild == nil {
		return false
	}
	return guild.OwnerID == userID
}

// extractUserID extracts the user ID from the interaction
func extractUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	} else if i.User != nil {
		return i.User.ID
	}
	return ""
}

// GetSubCommandName extracts the subcommand name from the interaction
func GetSubCommandName(i *discordgo.InteractionCreate) string {
	options := i.ApplicationCommandData().Options
	if len(options) > 0 && options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return options[0].Name
	}
	return ""
}

// GetSubCommandOptions extracts the subcommand options from the interaction
func GetSubCommandOptions(i *discordgo.InteractionCreate) []*discordgo.ApplicationCommandInteractionDataOption {
	options := i.ApplicationCommandData().Options
	if len(options) > 0 && options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return options[0].Options
	}
	return options // Returns direct options if not a subcommand
}

// CommandLogEntry creates a standardized log entry for commands
func CommandLogEntry(i *discordgo.InteractionCreate, command string, userID string) *log.Logger {
	return log.GlobalLogger
}

// ValidateGuildContext validates if the context has the required server information
func ValidateGuildContext(ctx *Context) error {
	if ctx.GuildID == "" {
		return NewCommandError("This command can only be used in a server", true)
	}

	if ctx.GuildConfig == nil {
		return NewCommandError("Server configuration not found", true)
	}

	return nil
}

// ValidateUserContext validates if the context has the required user information
func ValidateUserContext(ctx *Context) error {
	if ctx.UserID == "" {
		return NewCommandError("Unable to identify user", true)
	}

	return nil
}

// HasFocusedOption checks if there is a focused option (for autocomplete)
func HasFocusedOption(options []*discordgo.ApplicationCommandInteractionDataOption) (*discordgo.ApplicationCommandInteractionDataOption, bool) {
	for _, opt := range options {
		if opt.Focused {
			return opt, true
		}
		// Checks recursively in subcommands
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand && len(opt.Options) > 0 {
			if focused, found := HasFocusedOption(opt.Options); found {
				return focused, true
			}
		}
	}
	return nil, false
}

// GetCommandPath returns the full command path (command + subcommand if present)
func GetCommandPath(i *discordgo.InteractionCreate) string {
	path := i.ApplicationCommandData().Name

	subCmd := GetSubCommandName(i)
	if subCmd != "" {
		path += " " + subCmd
	}

	return path
}

// IsAutocompleteInteraction checks if the interaction is for autocomplete
func IsAutocompleteInteraction(i *discordgo.InteractionCreate) bool {
	return i.Type == discordgo.InteractionApplicationCommandAutocomplete
}

// IsSlashCommandInteraction checks if the interaction is a slash command
func IsSlashCommandInteraction(i *discordgo.InteractionCreate) bool {
	return i.Type == discordgo.InteractionApplicationCommand
}

// CreateLogFields creates standardized log fields
func CreateLogFields(ctx *Context, additionalFields map[string]any) map[string]any {
	fields := map[string]any{
		"command": GetCommandPath(ctx.Interaction),
		"guildID": ctx.GuildID,
		"userID":  ctx.UserID,
	}

	// Adds additional fields
	for k, v := range additionalFields {
		fields[k] = v
	}

	return fields
}

// RequiresGuildConfig checks if the command requires server configuration
func RequiresGuildConfig(ctx *Context) error {
	if err := ValidateGuildContext(ctx); err != nil {
		return err
	}

	if ctx.GuildConfig == nil {
		return NewCommandError("Server configuration is required for this command", true)
	}

	return nil
}

// SafeGuildAccess provides safe access to server information
func SafeGuildAccess(ctx *Context, fn func(*files.GuildConfig) error) error {
	if err := RequiresGuildConfig(ctx); err != nil {
		return err
	}

	return fn(ctx.GuildConfig)
}
