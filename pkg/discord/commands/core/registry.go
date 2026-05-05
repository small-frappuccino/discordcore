package core

import (
	"fmt"
	"log/slog"
	"maps"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// CommandRouter manages routing and execution of commands
type CommandRouter struct {
	registry       *CommandRegistry
	routeRegistry  *interactionRouteRegistry
	contextBuilder *ContextBuilder
	middlewares    []InteractionMiddleware

	permChecker *PermissionChecker
	store       *storage.Store
	guildFilter func(string) bool
	guildRouteFilter func(string, InteractionRouteKey) bool

	// runtimeApplier is an optional shared hot-apply manager (theme + ALICE_DISABLE_* toggles).
	// It is set by the app runner and can be used by interaction handlers to apply changes
	// immediately after persisting runtime config changes.
	runtimeApplier *runtimeapply.Manager

	// taskRouter is an optional shared task router (backfill, async notifications).
	taskRouter *task.TaskRouter
}

// NewCommandRouter creates a new command router
func NewCommandRouter(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandRouter {
	registry := NewCommandRegistry()

	permChecker := NewPermissionChecker(session, configManager)
	contextBuilder := NewContextBuilder(session, configManager, permChecker)

	router := &CommandRouter{
		registry:       registry,
		routeRegistry:  newInteractionRouteRegistry(),
		contextBuilder: contextBuilder,

		permChecker: permChecker,
	}
	router.UseMiddleware(defaultInteractionMiddlewares(router)...)
	return router
}

// RegisterSlashCommand registers a slash command tree in both the sync registry
// and the slash route registry.
func (cr *CommandRouter) RegisterSlashCommand(cmd Command) {
	cr.RegisterSlashCommandForDomain("", cmd)
}

// RegisterSlashCommandForDomain registers a slash command tree in both the
// sync registry and the slash route registry under the requested domain.
func (cr *CommandRouter) RegisterSlashCommandForDomain(domain string, cmd Command) {
	if cr == nil || cmd == nil {
		return
	}
	cr.registry.Register(cmd)
	cr.registerSlashCommandRoutesForDomain(domain, cmd)
}

// RegisterCommand is the compatibility API; prefer RegisterSlashCommand for new slash trees.
func (cr *CommandRouter) RegisterCommand(cmd Command) {
	cr.RegisterSlashCommand(cmd)
}

// RegisterSlashSubCommand registers a slash subcommand in both the sync registry
// and the slash route registry.
func (cr *CommandRouter) RegisterSlashSubCommand(parentName string, subcmd SubCommand) {
	cr.RegisterSlashSubCommandForDomain("", parentName, subcmd)
}

// RegisterSlashSubCommandForDomain registers a slash subcommand in both the
// sync registry and the slash route registry under the requested domain.
func (cr *CommandRouter) RegisterSlashSubCommandForDomain(domain, parentName string, subcmd SubCommand) {
	if cr == nil || subcmd == nil {
		return
	}
	cr.registry.RegisterSubCommand(parentName, subcmd)
	cr.registerSlashSubCommandRoutesForDomain(domain, parentName, subcmd)
}

// RegisterSubCommand is the compatibility API; prefer RegisterSlashSubCommand for new slash trees.
func (cr *CommandRouter) RegisterSubCommand(parentName string, subcmd SubCommand) {
	cr.RegisterSlashSubCommand(parentName, subcmd)
}

// RegisterAutocomplete registers an autocomplete handler by canonical route path.
// This is the compatibility API; prefer an AutocompleteRouteProvider on the
// slash tree or RegisterInteractionRoute for new code.
func (cr *CommandRouter) RegisterAutocomplete(routePath string, handler AutocompleteHandler) {
	cr.RegisterAutocompleteRoute(routePath, handler)
}

// RegisterComponentHandler registers a component handler for an exact component route ID.
// This is the compatibility API; prefer RegisterInteractionRoute for new code.
func (cr *CommandRouter) RegisterComponentHandler(routeID string, handler ComponentHandler) {
	cr.RegisterComponentRoute(routeID, handler)
}

// RegisterModalHandler registers a modal handler for an exact modal route ID.
// This is the compatibility API; prefer RegisterInteractionRoute for new code.
func (cr *CommandRouter) RegisterModalHandler(routeID string, handler ModalHandler) {
	cr.RegisterModalRoute(routeID, handler)
}

// SetGuildFilter restricts interaction handling to guilds accepted by the provided predicate.
func (cr *CommandRouter) SetGuildFilter(filter func(string) bool) {
	if cr == nil {
		return
	}
	cr.guildFilter = filter
	if filter == nil {
		cr.guildRouteFilter = nil
		return
	}
	cr.guildRouteFilter = func(guildID string, _ InteractionRouteKey) bool {
		return filter(guildID)
	}
}

// SetGuildRouteFilter restricts interaction handling to guild/route pairs
// accepted by the provided predicate.
func (cr *CommandRouter) SetGuildRouteFilter(filter func(string, InteractionRouteKey) bool) {
	if cr == nil {
		return
	}
	cr.guildRouteFilter = filter
	if filter == nil {
		cr.guildFilter = nil
	}
}

func (cr *CommandRouter) shouldHandleInteraction(guildID string, routeKey InteractionRouteKey) bool {
	if cr == nil || guildID == "" {
		return true
	}
	if cr.guildRouteFilter != nil {
		return cr.guildRouteFilter(guildID, routeKey)
	}
	if cr.guildFilter == nil {
		return true
	}
	return cr.guildFilter(guildID)
}

// CommandManager manages the lifecycle of commands on Discord
type CommandManager struct {
	session                  *discordgo.Session
	router                   *CommandRouter
	logger                   *log.Logger
	interactionHandlerCancel func()
}

// NewCommandManager creates a new command manager
func NewCommandManager(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandManager {
	return &CommandManager{
		session: session,
		router:  NewCommandRouter(session, configManager),
		logger:  log.GlobalLogger,
	}
}

// GetRouter returns the command router
func (cm *CommandManager) GetRouter() *CommandRouter {
	return cm.router
}

// SetupCommands configures and synchronizes commands with Discord
func (cm *CommandManager) SetupCommands() error {
	// Verify session state is properly initialized
	if cm.session == nil || cm.session.State == nil || cm.session.State.User == nil {
		return fmt.Errorf("session not properly initialized")
	}

	// Prevent duplicated interaction handling in reinit/hot-reload paths.
	if cm.interactionHandlerCancel != nil {
		cm.interactionHandlerCancel()
		cm.interactionHandlerCancel = nil
	}
	cm.interactionHandlerCancel = cm.session.AddHandler(cm.router.HandleInteraction)
	rollback := func(err error) error {
		if cm.interactionHandlerCancel != nil {
			cm.interactionHandlerCancel()
			cm.interactionHandlerCancel = nil
		}
		return err
	}

	// Fetch commands already registered on Discord
	registered, err := cm.session.ApplicationCommands(cm.session.State.User.ID, "")
	if err != nil {
		return rollback(fmt.Errorf("failed to fetch registered commands: %w", err))
	}

	// Build map of registered commands
	regByName := make(map[string]*discordgo.ApplicationCommand, len(registered))
	for _, rc := range registered {
		regByName[rc.Name] = rc
	}

	// Build map of code-defined commands
	codeCommands := cm.router.registry.GetAllCommands()
	codeByName := maps.Clone(codeCommands)

	// Create/Update commands as needed
	created, updated, unchanged := 0, 0, 0
	commandIDs := make(map[string]string, len(codeCommands))
	for name, cmd := range codeCommands {
		desired := &discordgo.ApplicationCommand{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			Options:     normalizeCommandOptions(cmd.Options()),
		}
		if existing, ok := regByName[name]; ok {
			// Command already exists, check if it needs updating
			if CompareCommands(existing, desired) {
				slog.Info(fmt.Sprintf("Command unchanged: /%s %s - %s", name, FormatOptions(cmd.Options()), cmd.Description()))
				unchanged++
				commandIDs[name] = existing.ID
				continue
			}

			// Update command
			updatedCmd, err := cm.session.ApplicationCommandEdit(cm.session.State.User.ID, "", existing.ID, desired)
			if err != nil {
				return rollback(fmt.Errorf("error updating command '%s': %w", name, err))
			}
			if updatedCmd != nil {
				commandIDs[name] = updatedCmd.ID
			} else {
				commandIDs[name] = existing.ID
			}
			slog.Info(fmt.Sprintf("Command updated: /%s %s - %s", name, FormatOptions(cmd.Options()), cmd.Description()))
			updated++
		} else {
			// Create new command
			createdCmd, err := cm.session.ApplicationCommandCreate(cm.session.State.User.ID, "", desired)
			if err != nil {
				return rollback(fmt.Errorf("error creating command '%s': %w", name, err))
			}
			if createdCmd != nil {
				commandIDs[name] = createdCmd.ID
			}
			slog.Info(fmt.Sprintf("Command created: /%s %s - %s", name, FormatOptions(cmd.Options()), cmd.Description()))
			created++
		}
	}

	// Remove orphaned commands (present on Discord but not in code)
	deleted := 0
	for _, rc := range registered {
		if _, exists := codeByName[rc.Name]; !exists {
			if err := cm.session.ApplicationCommandDelete(cm.session.State.User.ID, "", rc.ID); err != nil {
				slog.Warn(fmt.Sprintf("Error removing orphan command: %s, error: %v", rc.Name, err))
				continue
			}
			slog.Info(fmt.Sprintf("Orphan command removed: /%s %s - %s", rc.Name, FormatOptions(rc.Options), rc.Description))
			deleted++
		}
	}
	// Log do resumo
	slog.Info(fmt.Sprintf("Command synchronization completed: created=%d, updated=%d, deleted=%d, unchanged=%d, total=%d, mode=incremental", created, updated, deleted, unchanged, len(codeCommands)))

	return nil
}

// Shutdown unregisters command interaction handlers.
func (cm *CommandManager) Shutdown() error {
	if cm.interactionHandlerCancel != nil {
		cm.interactionHandlerCancel()
		cm.interactionHandlerCancel = nil
	}
	return nil
}

// GroupCommand represents a command that contains subcommands
type GroupCommand struct {
	name        string
	description string
	subcommands map[string]SubCommand
	checker     *PermissionChecker
}

// NewGroupCommand creates a new group command
func NewGroupCommand(name, description string, checker *PermissionChecker) *GroupCommand {
	return &GroupCommand{
		name:        name,
		description: description,
		subcommands: make(map[string]SubCommand),
		checker:     checker,
	}
}

// AddSubCommand adds a subcommand to the group
func (gc *GroupCommand) AddSubCommand(subcmd SubCommand) {
	gc.subcommands[subcmd.Name()] = subcmd
}

// Name returns the command name
func (gc *GroupCommand) Name() string {
	return gc.name
}

// Description returns the command description
func (gc *GroupCommand) Description() string {
	return gc.description
}

// Options builds the command options based on subcommands
func (gc *GroupCommand) Options() []*discordgo.ApplicationCommandOption {
	options := make([]*discordgo.ApplicationCommandOption, 0, len(gc.subcommands))

	for _, subcmd := range gc.subcommands {
		// Determine type: if the subcommand itself has options that are also subcommands,
		// then this entry must be a SubCommandGroup (Type 2).
		// Otherwise, it's a regular SubCommand (Type 1).
		optType := discordgo.ApplicationCommandOptionSubCommand
		subOpts := subcmd.Options()
		for _, so := range subOpts {
			if so.Type == discordgo.ApplicationCommandOptionSubCommand || so.Type == discordgo.ApplicationCommandOptionSubCommandGroup {
				optType = discordgo.ApplicationCommandOptionSubCommandGroup
				break
			}
		}

		option := &discordgo.ApplicationCommandOption{
			Type:        optType,
			Name:        subcmd.Name(),
			Description: subcmd.Description(),
			Options:     subOpts,
		}
		options = append(options, option)
	}

	return options
}

// RequiresGuild checks if any subcommand requires a server
func (gc *GroupCommand) RequiresGuild() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresGuild() {
			return true
		}
	}
	return false
}

// RequiresPermissions checks if any subcommand requires permissions
func (gc *GroupCommand) RequiresPermissions() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresPermissions() {
			return true
		}
	}
	return false
}

// Handle routes to the appropriate subcommand
func (gc *GroupCommand) Handle(ctx *Context) error {
	subCommandName := GetSubCommandName(ctx.Interaction)
	if subCommandName == "" {
		return NewCommandError("This command needs a subcommand before it can continue, so this reply stays private.", true)
	}

	subcmd, exists := gc.subcommands[subCommandName]
	if !exists {
		return NewCommandError("That subcommand couldn't be matched, so this reply stays private.", true)
	}

	// Check subcommand-specific permissions
	if subcmd.RequiresGuild() && ctx.GuildID == "" {
		return NewCommandError("This subcommand only works inside a server, so this failure stays private.", true)
	}

	if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		return NewCommandError("You don't have access to this subcommand, so this reply stays private.", true)
	}

	if subcmd.RequiresPermissions() && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		return NewCommandError("You don't have access to this subcommand, so this reply stays private.", true)
	}

	return subcmd.Handle(ctx)
}

// SimpleCommand implementa Command para comandos simples
type SimpleCommand struct {
	name                string
	description         string
	options             []*discordgo.ApplicationCommandOption
	handler             func(ctx *Context) error
	autocompleteHandler AutocompleteHandler
	requiresGuild       bool
	requiresPermissions bool
}

// NewSimpleCommand cria um comando simples
func NewSimpleCommand(
	name, description string,
	options []*discordgo.ApplicationCommandOption,
	handler func(ctx *Context) error,
	requiresGuild, requiresPermissions bool,
) *SimpleCommand {
	return &SimpleCommand{
		name:                name,
		description:         description,
		options:             options,
		handler:             handler,
		requiresGuild:       requiresGuild,
		requiresPermissions: requiresPermissions,
	}
}

// WithAutocomplete binds an autocomplete handler to the command's route path.
func (sc *SimpleCommand) WithAutocomplete(handler AutocompleteHandler) *SimpleCommand {
	if sc == nil {
		return nil
	}
	sc.autocompleteHandler = handler
	return sc
}

func (sc *SimpleCommand) Name() string        { return sc.name }
func (sc *SimpleCommand) Description() string { return sc.description }
func (sc *SimpleCommand) Options() []*discordgo.ApplicationCommandOption {
	return sc.options
}
func (sc *SimpleCommand) Handle(ctx *Context) error { return sc.handler(ctx) }
func (sc *SimpleCommand) AutocompleteRouteHandler() AutocompleteHandler {
	return sc.autocompleteHandler
}
func (sc *SimpleCommand) RequiresGuild() bool       { return sc.requiresGuild }
func (sc *SimpleCommand) RequiresPermissions() bool { return sc.requiresPermissions }

// GetSession returns the Discord session from the context builder
func (cr *CommandRouter) GetSession() *discordgo.Session {
	return cr.contextBuilder.session
}

// GetConfigManager returns the config manager from the context builder
func (cr *CommandRouter) GetConfigManager() *files.ConfigManager {
	return cr.contextBuilder.configManager
}

// GetRegistry returns the command registry
func (cr *CommandRouter) GetRegistry() *CommandRegistry {
	return cr.registry
}

// GetPermissionChecker returns the permission checker
func (cr *CommandRouter) GetPermissionChecker() *PermissionChecker {
	return cr.permChecker
}

// SetStore sets the shared store for the permission checker to enable local OwnerID cache usage.
func (cr *CommandRouter) SetStore(store *storage.Store) {
	cr.store = store
	if cr.permChecker != nil {
		cr.permChecker.SetStore(store)
	}
}

// GetStore returns the shared store used by the router, if any.
func (cr *CommandRouter) GetStore() *storage.Store {
	return cr.store
}

// SetCache sets the unified cache for the permission checker to reduce API calls.
func (cr *CommandRouter) SetCache(unifiedCache *cache.UnifiedCache) {
	if cr.permChecker != nil {
		cr.permChecker.SetCache(unifiedCache)
	}
}

// SetRuntimeApplier sets the shared runtime hot-apply manager.
// This is optional; if unset, hot-apply is simply not performed.
func (cr *CommandRouter) SetRuntimeApplier(applier *runtimeapply.Manager) {
	cr.runtimeApplier = applier
}

// GetRuntimeApplier returns the shared runtime hot-apply manager (if any).
func (cr *CommandRouter) GetRuntimeApplier() *runtimeapply.Manager {
	return cr.runtimeApplier
}

// SetTaskRouter sets the task router
func (cr *CommandRouter) SetTaskRouter(router *task.TaskRouter) {
	cr.taskRouter = router
}

// GetTaskRouter returns the task router
func (cr *CommandRouter) GetTaskRouter() *task.TaskRouter {
	return cr.taskRouter
}
