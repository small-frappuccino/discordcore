package core

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// Command represents a Discord command
type Command interface {
	Name() string
	Description() string
	Options() []*discordgo.ApplicationCommandOption
	Handle(ctx *Context) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// SubCommand represents a subcommand within a larger command
type SubCommand interface {
	Name() string
	Description() string
	Options() []*discordgo.ApplicationCommandOption
	Handle(ctx *Context) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// Context provides a unified context for command execution
type Context struct {
	Session     *discordgo.Session
	Interaction *discordgo.InteractionCreate
	Config      *files.ConfigManager
	Logger      *log.Logger
	GuildID     string
	UserID      string
	IsOwner     bool
	GuildConfig *files.GuildConfig
	router      *CommandRouter
}

// SetRouter sets the router in the context
func (ctx *Context) SetRouter(router *CommandRouter) {
	ctx.router = router
}

// Router returns the command router from the context
func (ctx *Context) Router() *CommandRouter {
	return ctx.router
}

// Response standardizes command responses
type Response struct {
	Content   string
	Ephemeral bool
	Success   bool
}

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
}

func NewBaseHandler(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *BaseHandler {
	return &BaseHandler{
		session:       session,
		configManager: configManager,
	}
}

// GetSession returns the Discord session
func (bh *BaseHandler) GetSession() *discordgo.Session {
	return bh.session
}

// GetConfigManager returns the configuration manager
func (bh *BaseHandler) GetConfigManager() *files.ConfigManager {
	return bh.configManager
}

// GetAvatarCacheManager retorna o gerenciador de cache de avatar

// CommandRegistry manages command registration and execution
type CommandRegistry struct {
	commands    map[string]Command
	subcommands map[string]map[string]SubCommand // [commandName][subcommandName]
}

func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands:    make(map[string]Command),
		subcommands: make(map[string]map[string]SubCommand),
	}
}

// Register registers a command in the registry
func (r *CommandRegistry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

// RegisterSubCommand registers a subcommand in the registry
func (r *CommandRegistry) RegisterSubCommand(parentName string, subcmd SubCommand) {
	if r.subcommands[parentName] == nil {
		r.subcommands[parentName] = make(map[string]SubCommand)
	}
	r.subcommands[parentName][subcmd.Name()] = subcmd
}

// GetCommand returns a command by name
func (r *CommandRegistry) GetCommand(name string) (Command, bool) {
	cmd, exists := r.commands[name]
	return cmd, exists
}

// GetSubCommand returns a subcommand by its parent command and subcommand name
func (r *CommandRegistry) GetSubCommand(parentName, subName string) (SubCommand, bool) {
	if subs, exists := r.subcommands[parentName]; exists {
		if sub, exists := subs[subName]; exists {
			return sub, true
		}
	}
	return nil, false
}

// GetAllCommands returns all registered commands
func (r *CommandRegistry) GetAllCommands() map[string]Command {
	return r.commands
}

// GetAllSubCommands returns all subcommands for a given command
func (r *CommandRegistry) GetAllSubCommands(parentName string) map[string]SubCommand {
	if subs, exists := r.subcommands[parentName]; exists {
		return subs
	}
	return make(map[string]SubCommand)
}

// CommandMeta defines metadata for building commands
type CommandMeta struct {
	Name        string
	Description string
	Options     []*discordgo.ApplicationCommandOption
}

// SubCommandMeta defines metadata for building subcommands
type SubCommandMeta struct {
	Name        string
	Description string
	Options     []*discordgo.ApplicationCommandOption
}

// AutocompleteHandler defines a handler for autocomplete
type AutocompleteHandler interface {
	HandleAutocomplete(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error)
}

// PermissionLevel defines permission levels for commands
type PermissionLevel int

const (
	PermissionNone PermissionLevel = iota
	PermissionUser
	PermissionModerator
	PermissionAdmin
	PermissionOwner
)

// CommandError represents command-specific errors
type CommandError struct {
	Message   string
	Ephemeral bool
	Code      string
}

func (e *CommandError) Error() string {
	return e.Message
}

// NewCommandError creates a new command error
func NewCommandError(message string, ephemeral bool) *CommandError {
	return &CommandError{
		Message:   message,
		Ephemeral: ephemeral,
	}
}

// ValidationError represents validation errors
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
