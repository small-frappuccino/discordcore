package core

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// InteractionKind identifies the normalized interaction surface being routed.
type InteractionKind int

// InteractionKindSlash defines interaction kind slash.
// InteractionKindAutocomplete defines interaction kind autocomplete.
// InteractionKindComponent defines interaction kind component.
// InteractionKindModal defines interaction kind modal.
// InteractionKindUnknown defines interaction kind unknown.
const (
	InteractionKindUnknown InteractionKind = iota
	InteractionKindSlash
	InteractionKindAutocomplete
	InteractionKindComponent
	InteractionKindModal
)

// String strings.
func (kind InteractionKind) String() string {
	switch kind {
	case InteractionKindSlash:
		return "slash"
	case InteractionKindAutocomplete:
		return "autocomplete"
	case InteractionKindComponent:
		return "component"
	case InteractionKindModal:
		return "modal"
	default:
		return "unknown"
	}
}

// InteractionRouteKey is the normalized dispatch key used by the unified
// interaction router.
//
// Path is the canonical route path for the interaction kind:
// - slash/autocomplete: full command path, including subcommand groups
// - component/modal: stable route ID extracted from the custom ID
//
// CustomID keeps the raw encoded custom ID when the interaction uses one.
type InteractionRouteKey struct {
	Kind          InteractionKind
	Path          string
	FocusedOption string
	CustomID      string
}

// InteractionAckMode defines how the router should acknowledge an interaction
// before handing execution to the route handler.
type InteractionAckMode int

// InteractionAckModeNone defines interaction ack mode none.
// InteractionAckModeDefer defines interaction ack mode defer.
const (
	InteractionAckModeNone InteractionAckMode = iota
	InteractionAckModeDefer
)

// InteractionAckPolicy declares whether the router should acknowledge the
// interaction before the handler runs.
//
// Mode=Defer means:
// - slash: deferred channel message response
// - component/modal: deferred message update
//
// Handlers that need to open a modal or otherwise control the first response
// should leave the policy as Mode=None.
type InteractionAckPolicy struct {
	Mode      InteractionAckMode
	Ephemeral bool
}

func (policy InteractionAckPolicy) requiresAck() bool {
	return policy.Mode != InteractionAckModeNone
}

// Command represents a Discord command
type Command interface {
	Name() string
	Description() string
	Options() []*discordgo.ApplicationCommandOption
	Handle(ctx *Context) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// SubCommand type removed; use Command directly

// DefaultMemberPermissionsProvider is an optional opt-in for top-level
// commands that want Discord to enforce a permission floor before the
// interaction reaches the bot. Discord only honors this field on the
// top-level descriptor — subcommand entries cannot declare their own
// floor in the protocol. The router applies the returned bits via a
// type assertion so commands that don't implement this interface keep
// the previous "no floor declared" behavior.
type DefaultMemberPermissionsProvider interface {
	DefaultMemberPermissions() int64
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
	RouteKey    InteractionRouteKey
	// Acknowledged is true once the router has sent an ack response for this
	// interaction (e.g. a deferred channel message). Follow-up writes must use
	// InteractionResponseEdit / FollowupMessageCreate instead of a fresh
	// InteractionRespond, otherwise Discord returns 40060 ("already
	// acknowledged"). Response helpers honor this flag when given the Context.
	Acknowledged bool
	// AckEphemeral records whether the deferred ack was ephemeral so that
	// follow-up writes can preserve the visibility chosen at defer time.
	AckEphemeral bool
	router       *CommandRouter
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

// NewBaseHandler news base handler.
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
	subcommands map[string]map[string]Command // [commandName][subcommandName]
}

// Register registers a command in the registry
func (r *CommandRegistry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

// RegisterSubCommand registers a subcommand in the registry
func (r *CommandRegistry) RegisterSubCommand(parentName string, subcmd Command) {
	if r.subcommands[parentName] == nil {
		r.subcommands[parentName] = make(map[string]Command)
	}
	r.subcommands[parentName][subcmd.Name()] = subcmd
}

// GetCommand returns a command by name
func (r *CommandRegistry) GetCommand(name string) (Command, bool) {
	cmd, exists := r.commands[name]
	return cmd, exists
}

// GetSubCommand returns a subcommand by its parent command and subcommand name
func (r *CommandRegistry) GetSubCommand(parentName, subName string) (Command, bool) {
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
func (r *CommandRegistry) GetAllSubCommands(parentName string) map[string]Command {
	if subs, exists := r.subcommands[parentName]; exists {
		return subs
	}
	return make(map[string]Command)
}

// CommandMeta defines metadata for building commands
type CommandMeta struct {
	Name        string
	Description string
	Options     []*discordgo.ApplicationCommandOption
}

// CommandMeta removed; use CommandMeta directly

// AutocompleteHandler defines a handler for autocomplete
type AutocompleteHandler interface {
	HandleAutocomplete(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error)
}

// AutocompleteRouteProvider allows a slash command route to expose an
// autocomplete handler for the same canonical route path.
type AutocompleteRouteProvider interface {
	AutocompleteRouteHandler() AutocompleteHandler
}

// InteractionAckPolicyProvider allows a slash command route to expose an ack
// policy for the same canonical route path derived into the interaction router.
type InteractionAckPolicyProvider interface {
	InteractionAckPolicy() InteractionAckPolicy
}

// AutocompleteHandlerFunc adapts a function into an AutocompleteHandler.
type AutocompleteHandlerFunc func(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error)

// HandleAutocomplete handles autocomplete.
func (fn AutocompleteHandlerFunc) HandleAutocomplete(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return fn(ctx, focusedOption)
}

// SlashHandler defines the minimal execution contract for a slash route.
type SlashHandler interface {
	Handle(ctx *Context) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// ComponentHandler defines a handler for message component interactions.
type ComponentHandler interface {
	HandleComponent(ctx *Context) error
}

// ComponentHandlerFunc adapts a function into a ComponentHandler.
type ComponentHandlerFunc func(ctx *Context) error

// HandleComponent handles component.
func (fn ComponentHandlerFunc) HandleComponent(ctx *Context) error {
	return fn(ctx)
}

// ModalHandler defines a handler for modal submit interactions.
type ModalHandler interface {
	HandleModal(ctx *Context) error
}

// ModalHandlerFunc adapts a function into a ModalHandler.
type ModalHandlerFunc func(ctx *Context) error

// HandleModal handles modal.
func (fn ModalHandlerFunc) HandleModal(ctx *Context) error {
	return fn(ctx)
}

// InteractionRouteBinding declares the handlers bound to the same normalized
// interaction route path or stable route ID.
type InteractionRouteBinding struct {
	Path         string
	Domain       string
	Slash        SlashHandler
	Autocomplete AutocompleteHandler
	Component    ComponentHandler
	Modal        ModalHandler
	AckPolicy    InteractionAckPolicy
}

func (binding InteractionRouteBinding) hasHandlers() bool {
	return binding.Slash != nil || binding.Autocomplete != nil || binding.Component != nil || binding.Modal != nil
}

// PermissionLevel defines permission levels for commands
type PermissionLevel int

// PermissionNone defines permission none.
// PermissionUser defines permission user.
// PermissionModerator defines permission moderator.
// PermissionAdmin defines permission admin.
// PermissionOwner defines permission owner.
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

// CommandErrorCode commands error code.
func (e *CommandError) CommandErrorCode() string {
	if e == nil {
		return ""
	}
	return e.Code
}

// Error errors.
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

// ValidationField validations field.
func (e *ValidationError) ValidationField() string {
	if e == nil {
		return ""
	}
	return e.Field
}

// Error errors.
func (e *ValidationError) Error() string {
	return e.Message
}
