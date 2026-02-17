package core

import (
	"fmt"
	"log/slog"
	"maps"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// CommandRouter manages routing and execution of commands
type CommandRouter struct {
	registry       *CommandRegistry
	contextBuilder *ContextBuilder

	permChecker     *PermissionChecker
	autocompleteMap map[string]AutocompleteHandler
	store           *storage.Store

	// runtimeApplier is an optional shared hot-apply manager (theme + ALICE_DISABLE_* toggles).
	// It is set by the app runner and can be used by interaction handlers to apply changes
	// immediately after persisting settings.json.
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

	return &CommandRouter{
		registry:       registry,
		contextBuilder: contextBuilder,

		permChecker:     permChecker,
		autocompleteMap: make(map[string]AutocompleteHandler),
	}
}

// RegisterCommand registers a simple command
func (cr *CommandRouter) RegisterCommand(cmd Command) {
	cr.registry.Register(cmd)
}

// RegisterSubCommand registers a subcommand
func (cr *CommandRouter) RegisterSubCommand(parentName string, subcmd SubCommand) {
	cr.registry.RegisterSubCommand(parentName, subcmd)
}

// RegisterAutocomplete registers an autocomplete handler
func (cr *CommandRouter) RegisterAutocomplete(commandName string, handler AutocompleteHandler) {
	cr.autocompleteMap[commandName] = handler
}

// HandleInteraction routes interactions to the appropriate handlers
func (cr *CommandRouter) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil {
		return
	}

	done := perf.StartGatewayEvent(
		"interaction_create",
		slog.Int("interactionType", int(i.Type)),
		slog.String("guildID", i.GuildID),
		slog.String("userID", extractUserID(i)),
	)
	defer done()

	if IsAutocompleteInteraction(i) {
		cr.handleAutocomplete(i)
		return
	}

	if !IsSlashCommandInteraction(i) {
		return
	}

	cr.handleSlashCommand(i)
}

// handleSlashCommand processes slash commands
func (cr *CommandRouter) handleSlashCommand(i *discordgo.InteractionCreate) {
	ctx := cr.contextBuilder.BuildContext(i)
	ctx.SetRouter(cr)
	commandName := i.ApplicationCommandData().Name

	slog.Info("Processing slash command")

	// Check if the command exists
	cmd, exists := cr.registry.GetCommand(commandName)
	if !exists {
		slog.Error("Command not found")
		NewResponseBuilder(ctx.Session).Ephemeral().Error(i, "Command not found")
		return
	}

	// Check if the command requires a guild
	if cmd.RequiresGuild() && ctx.GuildID == "" {
		slog.Warn("Command used outside of guild")
		NewResponseBuilder(ctx.Session).Ephemeral().Error(i, "This command can only be used in a server")
		return
	}

	// Check permissions
	if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !cr.permChecker.HasPermission(ctx.GuildID, ctx.UserID) {
		slog.Warn("User without allowed role tried to use command")
		NewResponseBuilder(ctx.Session).Ephemeral().Error(i, "You do not have permission to use this command")
		return
	}

	if cmd.RequiresPermissions() && !cr.permChecker.HasPermission(ctx.GuildID, ctx.UserID) {
		slog.Warn("User without permission tried to use command")
		NewResponseBuilder(ctx.Session).Ephemeral().Error(i, "You do not have permission to use this command")
		return
	}

	// Execute command
	slog.Info("Executing command")
	if err := cmd.Handle(ctx); err != nil {
		slog.Error("Command execution failed", "err", err)

		// Check if it's a command-specific error
		if cmdErr, ok := err.(*CommandError); ok {
			if cmdErr.Ephemeral {
				NewResponseBuilder(ctx.Session).Ephemeral().Error(i, cmdErr.Message)
			} else {
				NewResponseBuilder(ctx.Session).Error(i, cmdErr.Message)
			}
		} else {
			NewResponseBuilder(ctx.Session).Ephemeral().Error(i, "An error occurred while executing the command")
		}
	}
}

// handleAutocomplete processes autocomplete interactions
func (cr *CommandRouter) handleAutocomplete(i *discordgo.InteractionCreate) {
	ctx := cr.contextBuilder.BuildContext(i)
	commandName := i.ApplicationCommandData().Name

	// Find autocomplete handler
	handler, exists := cr.autocompleteMap[commandName]
	if !exists {
		NewResponseBuilder(ctx.Session).Build().Autocomplete(i, []*discordgo.ApplicationCommandOptionChoice{})
		return
	}

	// Find the focused option
	focusedOpt, hasFocus := HasFocusedOption(i.ApplicationCommandData().Options)
	if !hasFocus {
		NewResponseBuilder(ctx.Session).Build().Autocomplete(i, []*discordgo.ApplicationCommandOptionChoice{})
		return
	}

	// Executar autocomplete
	choices, err := handler.HandleAutocomplete(ctx, focusedOpt.Name)
	if err != nil {
		slog.Error("Autocomplete handler failed", "err", err)
		choices = []*discordgo.ApplicationCommandOptionChoice{}
	}

	NewResponseBuilder(ctx.Session).Build().Autocomplete(i, choices)
}

// CommandManager manages the lifecycle of commands on Discord
type CommandManager struct {
	session *discordgo.Session
	router  *CommandRouter
	logger  *log.Logger
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
	// Register interaction handler
	cm.session.AddHandler(cm.router.HandleInteraction)

	// Verify session state is properly initialized
	if cm.session == nil || cm.session.State == nil || cm.session.State.User == nil {
		return fmt.Errorf("session not properly initialized")
	}

	// Fetch commands already registered on Discord
	registered, err := cm.session.ApplicationCommands(cm.session.State.User.ID, "")
	if err != nil {
		return fmt.Errorf("failed to fetch registered commands: %w", err)
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
			Options:     cmd.Options(),
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
				return fmt.Errorf("error updating command '%s': %w", name, err)
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
				return fmt.Errorf("error creating command '%s': %w", name, err)
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
		return NewCommandError("Subcommand is required", true)
	}

	subcmd, exists := gc.subcommands[subCommandName]
	if !exists {
		return NewCommandError("Unknown subcommand", true)
	}

	// Check subcommand-specific permissions
	if subcmd.RequiresGuild() && ctx.GuildID == "" {
		return NewCommandError("This subcommand can only be used in a server", true)
	}

	if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		return NewCommandError("You do not have permission to use this subcommand", true)
	}

	if subcmd.RequiresPermissions() && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		return NewCommandError("You don't have permission to use this subcommand", true)
	}

	return subcmd.Handle(ctx)
}

// SimpleCommand implementa Command para comandos simples
type SimpleCommand struct {
	name                string
	description         string
	options             []*discordgo.ApplicationCommandOption
	handler             func(ctx *Context) error
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

func (sc *SimpleCommand) Name() string        { return sc.name }
func (sc *SimpleCommand) Description() string { return sc.description }
func (sc *SimpleCommand) Options() []*discordgo.ApplicationCommandOption {
	return sc.options
}
func (sc *SimpleCommand) Handle(ctx *Context) error { return sc.handler(ctx) }
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
