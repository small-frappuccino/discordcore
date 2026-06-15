package core

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"
)

// CommandRouter manages routing and execution of commands
type CommandRouter struct {
	registry       *CommandRegistry
	routeRegistry  *interactionRouteRegistry
	contextBuilder *ContextBuilder
	middlewares    []InteractionMiddleware

	permChecker      *PermissionChecker
	store            *storage.Store
	guildFilter      func(string) bool
	guildRouteFilter func(string, InteractionRouteKey) bool

	// runtimeApplier consists of an optional shared hot-apply manager (theme + ALICE_DISABLE_* toggles).
	// Configured by the application runner, it allows interaction handlers to apply changes
	// immediately after persisting runtime configuration changes.
	runtimeApplier *runtimeapply.Manager

	// taskRouter consists of an optional shared task router (backfilling, asynchronous notifications).
	taskRouter *task.TaskRouter

	ignoredArikawaCommands map[string]bool
}

// NewCommandRouter allocates and initializes a new command router
func NewCommandRouter(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandRouter {
	registry := &CommandRegistry{
		commands:    make(map[string]Command),
		subcommands: make(map[string]map[string]Command),
	}

	permChecker := NewPermissionChecker(session, configManager)
	contextBuilder := NewContextBuilder(session, configManager, permChecker)

	router := &CommandRouter{
		registry:       registry,
		routeRegistry:  newInteractionRouteRegistry(),
		contextBuilder: contextBuilder,

		permChecker:            permChecker,
		ignoredArikawaCommands: make(map[string]bool),
	}
	router.UseMiddleware(defaultInteractionMiddlewares(router)...)

	slog.Info("Architectural state transition: Primary routines initialization", slog.String("component", "CommandRouter"))

	return router
}

// RegisterSlashCommand registers a slash command tree simultaneously in the sync registry
// and slash route registry.
func (cr *CommandRouter) RegisterSlashCommand(cmd Command) {
	cr.RegisterSlashCommandForDomain("", cmd)
}

// RegisterSlashCommandForDomain registers a slash command tree simultaneously in the
// sync registry and slash route registry under the requested domain.
func (cr *CommandRouter) RegisterSlashCommandForDomain(domain string, cmd Command) {
	if cr == nil || cmd == nil {
		return
	}
	cr.registry.Register(cmd)
	cr.registerSlashCommandRoutesForDomain(domain, cmd)
}

// RegisterCommand is the compatibility API; prioritize RegisterSlashCommand for new slash trees.
func (cr *CommandRouter) RegisterCommand(cmd Command) {
	cr.RegisterSlashCommand(cmd)
}

// RegisterSlashSubCommand registers a slash subcommand simultaneously in the sync registry
// and slash route registry.
func (cr *CommandRouter) RegisterSlashSubCommand(parentName string, subcmd Command) {
	cr.RegisterSlashSubCommandForDomain("", parentName, subcmd)
}

// RegisterSlashSubCommandForDomain registers a slash subcommand simultaneously in the
// sync registry and slash route registry under the requested domain.
func (cr *CommandRouter) RegisterSlashSubCommandForDomain(domain, parentName string, subcmd Command) {
	if cr == nil || subcmd == nil {
		return
	}
	cr.registry.RegisterSubCommand(parentName, subcmd)
	cr.registerSlashSubCommandRoutesForDomain(domain, parentName, subcmd)
}

// RegisterSubCommand is the compatibility API; prioritize RegisterSlashSubCommand for new slash trees.
func (cr *CommandRouter) RegisterSubCommand(parentName string, subcmd Command) {
	cr.RegisterSlashSubCommand(parentName, subcmd)
}

// RegisterAutocomplete binds an autocomplete handler via canonical route path.
// It is the compatibility API; prioritize an AutocompleteRouteProvider in the
// slash tree or RegisterInteractionRoute for new code.
func (cr *CommandRouter) RegisterAutocomplete(routePath string, handler AutocompleteHandler) {
	cr.RegisterAutocompleteRoute(routePath, handler)
}

// RegisterComponentHandler binds a component handler for an exact component route ID.
// It is the compatibility API; prioritize RegisterInteractionRoute for new code.
func (cr *CommandRouter) RegisterComponentHandler(routeID string, handler ComponentHandler) {
	cr.RegisterComponentRoute(routeID, handler)
}

// RegisterModalHandler binds a modal handler for an exact modal route ID.
// It is the compatibility API; prioritize RegisterInteractionRoute for new code.
func (cr *CommandRouter) RegisterModalHandler(routeID string, handler ModalHandler) {
	cr.RegisterModalRoute(routeID, handler)
}

// IgnoreArikawaCommand tells the router to ignore a root command name because it's handled natively.
func (cr *CommandRouter) IgnoreArikawaCommand(rootName string) {
	if cr != nil && cr.ignoredArikawaCommands != nil {
		cr.ignoredArikawaCommands[rootName] = true
	}
}

// SetGuildFilter restricts interaction processing to guilds validated by the provided predicate.
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

// SetGuildRouteFilter restricts interaction processing to guild/route pairs.
// validados pelo predicado fornecido.
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

// CommandManager orchestrates the command lifecycle in the Discord infrastructure
type CommandManager struct {
	session                  *discordgo.Session
	router                   *CommandRouter
	arikawaRouter            *ArikawaCommandRouter
	logger                   *log.Logger
	interactionHandlerCancel func()
	rawEventHandlerCancel    func()
}

type commandSyncSummary struct {
	created   int
	updated   int
	deleted   int
	unchanged int
	total     int
}

// NewCommandManager allocates and initializes a new command manager
func NewCommandManager(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandManager {
	slog.Info("Architectural state transition: Primary routines initialization", slog.String("component", "CommandManager"))

	return &CommandManager{
		session:       session,
		router:        NewCommandRouter(session, configManager),
		arikawaRouter: NewArikawaCommandRouter(session.Token, configManager),
		logger:        log.GlobalLogger,
	}
}

// GetArikawaRouter exposes the Arikawa command router
func (cm *CommandManager) GetArikawaRouter() *ArikawaCommandRouter {
	return cm.arikawaRouter
}

// GetRouter exposes the primary command router
func (cm *CommandManager) GetRouter() *CommandRouter {
	return cm.router
}

// SetupCommands configures and aligns local command state with the Discord API
func (cm *CommandManager) SetupCommands() error {
	// Validates session state integrity
	if cm.session == nil || cm.session.State == nil || cm.session.State.User == nil {
		err := fmt.Errorf("session state not properly initialized")
		slog.Error("Blocking structural failure restricted to operational scope",
			slog.String("req_id", "sys-init"),
			slog.String("stack_trace", string(debug.Stack())),
			slog.Int("fail_id", 500),
			slog.String("error", err.Error()),
		)
		return err
	}

	// Prevents duplicate interaction processing in restart/hot-reload cycles.
	if cm.interactionHandlerCancel != nil {
		cm.interactionHandlerCancel()
		cm.interactionHandlerCancel = nil
	}
	if cm.rawEventHandlerCancel != nil {
		cm.rawEventHandlerCancel()
		cm.rawEventHandlerCancel = nil
	}

	for name := range cm.arikawaRouter.GetAllCommands() {
		cm.router.IgnoreArikawaCommand(name)
	}

	cm.interactionHandlerCancel = cm.session.AddHandler(cm.router.HandleInteraction)
	cm.rawEventHandlerCancel = cm.session.AddHandler(cm.arikawaRouter.HandleRawEvent)

	slog.Info("Architectural state transition: Asynchronous event handler coupling", slog.String("component", "CommandManager"))

	rollback := func(err error) error {
		if cm.interactionHandlerCancel != nil {
			cm.interactionHandlerCancel()
			cm.interactionHandlerCancel = nil
		}
		if cm.rawEventHandlerCancel != nil {
			cm.rawEventHandlerCancel()
			cm.rawEventHandlerCancel = nil
		}
		return err
	}
	if cm.usesGuildScopedSync() {
		if err := cm.syncGuildScopedCommands(); err != nil {
			return rollback(err)
		}
	} else {
		if _, err := cm.syncCommandScope("", cm.globalDesiredCommands()); err != nil {
			return rollback(err)
		}
	}

	// Dispatches background scan task strictly after READY signal completion
	scheduleOrphanCleanupTask(cm.router.GetTaskRouter(), cm.session)
	slog.Info("Architectural state transition: Asynchronous primary routines initialization", slog.String("task", "orphan_cleanup"))

	return nil
}

// Shutdown unbinds command interaction handlers.
func (cm *CommandManager) Shutdown() error {
	slog.Info("Architectural state transition: Planned shutdown of main instances", slog.String("component", "CommandManager"))

	if cm.interactionHandlerCancel != nil {
		cm.interactionHandlerCancel()
		cm.interactionHandlerCancel = nil
	}
	if cm.rawEventHandlerCancel != nil {
		cm.rawEventHandlerCancel()
		cm.rawEventHandlerCancel = nil
	}
	return nil
}

func (cm *CommandManager) usesGuildScopedSync() bool {
	if cm == nil || cm.router == nil {
		return false
	}
	configManager := cm.router.GetConfigManager()
	if configManager == nil {
		return false
	}
	cfg := configManager.Config()
	if cfg == nil {
		return false
	}
	for _, guild := range cfg.Guilds {
		if len(guild.BotInstanceTokens) > 0 {
			return true
		}
	}
	return false
}

// SyncGuildCommands triggers a surgical sync on the Discord API for a single guild.
// Returns nil if the manager is configured for global synchronization.
func (cm *CommandManager) SyncGuildCommands(guildID string) error {
	if !cm.usesGuildScopedSync() {
		return nil
	}

	desired := cm.guildDesiredCommands(guildID)
	_, err := cm.syncCommandScope(guildID, desired)
	return err
}

func (cm *CommandManager) syncGuildScopedCommands() error {
	if cm == nil || cm.router == nil {
		return nil
	}

	summary := commandSyncSummary{}
	globalSummary, err := cm.syncCommandScope("", map[string]*discordgo.ApplicationCommand{})
	if err != nil {
		return fmt.Errorf("CommandManager.syncGuildScopedCommands: %w", err)
	}
	summary.add(globalSummary)

	for _, guildID := range cm.guildScopedSyncTargets() {
		guildSummary, err := cm.syncCommandScope(guildID, cm.guildDesiredCommands(guildID))
		if err != nil {
			return fmt.Errorf("CommandManager.syncGuildScopedCommands: %w", err)
		}
		summary.add(guildSummary)
	}

	slog.Info("Architectural state transition: Guild scope synchronization completion",
		slog.Int("created", summary.created),
		slog.Int("updated", summary.updated),
		slog.Int("deleted", summary.deleted),
		slog.Int("unchanged", summary.unchanged),
		slog.Int("total", summary.total),
	)
	return nil
}

func (summary *commandSyncSummary) add(other commandSyncSummary) {
	summary.created += other.created
	summary.updated += other.updated
	summary.deleted += other.deleted
	summary.unchanged += other.unchanged
	summary.total += other.total
}

func (cm *CommandManager) globalDesiredCommands() map[string]*discordgo.ApplicationCommand {
	if cm == nil || cm.router == nil || cm.router.registry == nil {
		return nil
	}
	codeCommands := cm.router.registry.GetAllCommands()
	arikawaCommands := cm.arikawaRouter.GetAllCommands()

	desired := make(map[string]*discordgo.ApplicationCommand, len(codeCommands)+len(arikawaCommands))
	for name, cmd := range codeCommands {
		desired[name] = &discordgo.ApplicationCommand{
			Name:                     cmd.Name(),
			Description:              cmd.Description(),
			Options:                  normalizeCommandOptions(cmd.Options()),
			DefaultMemberPermissions: commandDefaultMemberPermissions(cmd),
		}
	}
	for name, cmd := range arikawaCommands {
		desired[name] = &discordgo.ApplicationCommand{
			Name:                     cmd.Name(),
			Description:              cmd.Description(),
			Options:                  normalizeCommandOptions(ConvertArikawaOptions(cmd.Options())),
			DefaultMemberPermissions: arikawaCommandDefaultMemberPermissions(cmd),
		}
	}
	return desired
}

func (cm *CommandManager) guildDesiredCommands(guildID string) map[string]*discordgo.ApplicationCommand {
	if cm == nil || cm.router == nil || cm.router.registry == nil {
		return nil
	}
	codeCommands := cm.router.registry.GetAllCommands()
	arikawaCommands := cm.arikawaRouter.GetAllCommands()

	desired := make(map[string]*discordgo.ApplicationCommand, len(codeCommands)+len(arikawaCommands))
	for _, name := range sortedCommandNames(codeCommands) {
		cmd := codeCommands[name]
		if cmd == nil {
			continue
		}
		built := cm.buildGuildApplicationCommand(guildID, cmd)
		if built == nil {
			continue
		}
		desired[built.Name] = built
	}

	// Extracts Arikawa commands (absence of a complex Guild group builder in this version)
	for name, cmd := range arikawaCommands {
		if !cm.shouldSyncSlashRoute(guildID, strings.TrimSpace(cmd.Name())) {
			continue
		}
		desired[name] = &discordgo.ApplicationCommand{
			Name:                     cmd.Name(),
			Description:              cmd.Description(),
			Options:                  normalizeCommandOptions(ConvertArikawaOptions(cmd.Options())),
			DefaultMemberPermissions: arikawaCommandDefaultMemberPermissions(cmd),
		}
	}
	return desired
}

func (cm *CommandManager) buildGuildApplicationCommand(guildID string, cmd Command) *discordgo.ApplicationCommand {
	if cm == nil || cmd == nil {
		return nil
	}
	if group, ok := cmd.(*GroupCommand); ok {
		options := cm.buildGuildGroupOptions(guildID, strings.TrimSpace(group.Name()), group)
		if len(options) == 0 {
			return nil
		}
		return &discordgo.ApplicationCommand{
			Name:                     group.Name(),
			Description:              group.Description(),
			Options:                  normalizeCommandOptions(options),
			DefaultMemberPermissions: commandDefaultMemberPermissions(group),
		}
	}

	if !cm.shouldSyncSlashRoute(guildID, strings.TrimSpace(cmd.Name())) {
		return nil
	}
	return &discordgo.ApplicationCommand{
		Name:                     cmd.Name(),
		Description:              cmd.Description(),
		Options:                  normalizeCommandOptions(cmd.Options()),
		DefaultMemberPermissions: commandDefaultMemberPermissions(cmd),
	}
}

// commandDefaultMemberPermissions extracts the Discord permission base to
// embed in the top-level descriptor for the cmd, or nil when the cmd
// declares none. Discord requires a pointer; nil preserves the previous
// behavior focused only on "permissionGateMiddleware".
func commandDefaultMemberPermissions(cmd Command) *int64 {
	provider, ok := cmd.(DefaultMemberPermissionsProvider)
	if !ok {
		return nil
	}
	return new(int64(provider.DefaultMemberPermissions()))
}

func arikawaCommandDefaultMemberPermissions(cmd ArikawaCommand) *int64 {
	provider, ok := cmd.(ArikawaDefaultMemberPermissionsProvider)
	if !ok {
		return nil
	}
	return new(int64(provider.DefaultMemberPermissions()))
}

func (cm *CommandManager) buildGuildGroupOptions(guildID, parentPath string, group *GroupCommand) []*discordgo.ApplicationCommandOption {
	if cm == nil || group == nil {
		return nil
	}
	options := make([]*discordgo.ApplicationCommandOption, 0, len(group.subcommands))
	for _, name := range sortedSubCommandNames(group.subcommands) {
		subcmd := group.subcommands[name]
		option, ok := cm.buildGuildSubCommandOption(guildID, JoinRoutePath(parentPath, name), subcmd)
		if ok {
			options = append(options, option)
		}
	}
	return options
}

func (cm *CommandManager) buildGuildSubCommandOption(guildID, routePath string, subcmd Command) (*discordgo.ApplicationCommandOption, bool) {
	if cm == nil || subcmd == nil {
		return nil, false
	}

	if group, ok := subcmd.(*GroupCommand); ok {
		childOptions := cm.buildGuildGroupOptions(guildID, routePath, group)
		if len(childOptions) == 0 {
			return nil, false
		}
		optionType := discordgo.ApplicationCommandOptionSubCommand
		for _, childOption := range childOptions {
			if childOption == nil {
				continue
			}
			if childOption.Type == discordgo.ApplicationCommandOptionSubCommand || childOption.Type == discordgo.ApplicationCommandOptionSubCommandGroup {
				optionType = discordgo.ApplicationCommandOptionSubCommandGroup
				break
			}
		}
		return &discordgo.ApplicationCommandOption{
			Type:        optionType,
			Name:        group.Name(),
			Description: group.Description(),
			Options:     normalizeCommandOptions(childOptions),
		}, true
	}

	if !cm.shouldSyncSlashRoute(guildID, routePath) {
		return nil, false
	}
	optionType := discordgo.ApplicationCommandOptionSubCommand
	subOptions := normalizeCommandOptions(subcmd.Options())
	for _, subOption := range subOptions {
		if subOption == nil {
			continue
		}
		if subOption.Type == discordgo.ApplicationCommandOptionSubCommand || subOption.Type == discordgo.ApplicationCommandOptionSubCommandGroup {
			optionType = discordgo.ApplicationCommandOptionSubCommandGroup
			break
		}
	}
	return &discordgo.ApplicationCommandOption{
		Type:        optionType,
		Name:        subcmd.Name(),
		Description: subcmd.Description(),
		Options:     subOptions,
	}, true
}

func (cm *CommandManager) shouldSyncSlashRoute(guildID, path string) bool {
	if cm == nil || cm.router == nil {
		return false
	}
	return cm.router.shouldHandleInteraction(guildID, InteractionRouteKey{
		Kind: InteractionKindSlash,
		Path: JoinRoutePath(path),
	})
}

func (cm *CommandManager) guildScopedSyncTargets() []string {
	if cm == nil || cm.router == nil {
		return nil
	}
	configManager := cm.router.GetConfigManager()
	if configManager == nil {
		return nil
	}
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}
	sessionGuildIDs := cm.sessionGuildIDSet()
	filterBySession := len(sessionGuildIDs) > 0
	seen := make(map[string]struct{}, len(cfg.Guilds))
	targets := make([]string, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" {
			continue
		}
		if filterBySession {
			if _, ok := sessionGuildIDs[guildID]; !ok {
				continue
			}
		}
		if _, exists := seen[guildID]; exists {
			continue
		}
		seen[guildID] = struct{}{}
		targets = append(targets, guildID)
	}
	sort.Strings(targets)
	return targets
}

func (cm *CommandManager) sessionGuildIDSet() map[string]struct{} {
	if cm == nil || cm.session == nil || cm.session.State == nil || len(cm.session.State.Guilds) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(cm.session.State.Guilds))
	for _, guild := range cm.session.State.Guilds {
		if guild == nil {
			continue
		}
		guildID := strings.TrimSpace(guild.ID)
		if guildID == "" {
			continue
		}
		seen[guildID] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	return seen
}

func (cm *CommandManager) syncCommandScope(guildID string, desired map[string]*discordgo.ApplicationCommand) (commandSyncSummary, error) {
	if desired == nil {
		desired = map[string]*discordgo.ApplicationCommand{}
	}

	registered, err := cm.session.ApplicationCommands(cm.session.State.User.ID, guildID)
	if err != nil {
		reqID := guildID
		if reqID == "" {
			reqID = "global"
		}
		errWrap := fmt.Errorf("failed to fetch registered commands for scope %s: %w", commandSyncScopeLabel(guildID), err)
		slog.Error("Blocking structural failure restricted to operational scope",
			slog.String("req_id", reqID),
			slog.String("stack_trace", string(debug.Stack())),
			slog.Int("fail_id", 500),
			slog.String("error", errWrap.Error()),
		)
		return commandSyncSummary{}, errWrap
	}

	regByName := make(map[string]*discordgo.ApplicationCommand, len(registered))
	for _, rc := range registered {
		regByName[rc.Name] = rc
	}

	summary := commandSyncSummary{total: len(desired)}
	needsSync := false
	for _, name := range sortedDesiredCommandNames(desired) {
		desiredCommand := desired[name]
		if existing, ok := regByName[name]; ok {
			if CompareCommands(existing, desiredCommand) {
				slog.Debug("Granular transient state inspection: command unchanged",
					slog.String("scope", commandSyncScopeLabel(guildID)),
					slog.String("command", name),
					slog.String("options", formatOptions(desiredCommand.Options)),
				)
				summary.unchanged++
				continue
			}

			slog.Debug("Granular transient state inspection: command updated",
				slog.String("scope", commandSyncScopeLabel(guildID)),
				slog.String("command", name),
				slog.String("options", formatOptions(desiredCommand.Options)),
			)
			summary.updated++
			needsSync = true
			continue
		}

		slog.Debug("Granular transient state inspection: command created",
			slog.String("scope", commandSyncScopeLabel(guildID)),
			slog.String("command", name),
			slog.String("options", formatOptions(desiredCommand.Options)),
		)
		summary.created++
		needsSync = true
	}

	for _, rc := range registered {
		if _, exists := desired[rc.Name]; exists {
			continue
		}
		slog.Debug("Granular transient state inspection: orphan command removed",
			slog.String("scope", commandSyncScopeLabel(guildID)),
			slog.String("command", rc.Name),
			slog.String("options", formatOptions(rc.Options)),
		)
		summary.deleted++
		needsSync = true
	}

	if needsSync {
		overwrite := make([]*discordgo.ApplicationCommand, 0, len(desired))
		for _, name := range sortedDesiredCommandNames(desired) {
			overwrite = append(overwrite, desired[name])
		}
		if _, err := cm.session.ApplicationCommandBulkOverwrite(cm.session.State.User.ID, guildID, overwrite); err != nil {
			reqID := guildID
			if reqID == "" {
				reqID = "global"
			}
			errWrap := fmt.Errorf("error bulk overwriting commands in scope %s: %w", commandSyncScopeLabel(guildID), err)
			slog.Error("Blocking structural failure restricted to operational scope",
				slog.String("req_id", reqID),
				slog.String("stack_trace", string(debug.Stack())),
				slog.Int("fail_id", 500),
				slog.String("error", errWrap.Error()),
			)
			return commandSyncSummary{}, errWrap
		}
	}

	return summary, nil
}

func commandSyncScopeLabel(guildID string) string {
	if strings.TrimSpace(guildID) == "" {
		return "global"
	}
	return fmt.Sprintf("guild %s", guildID)
}

func sortedCommandNames(commands map[string]Command) []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedDesiredCommandNames(commands map[string]*discordgo.ApplicationCommand) []string {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSubCommandNames(subcommands map[string]Command) []string {
	names := make([]string, 0, len(subcommands))
	for name := range subcommands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GroupCommand encapsulates a command containing subcommands
type GroupCommand struct {
	name        string
	description string
	subcommands map[string]Command
	checker     *PermissionChecker
}

// NewGroupCommand allocates a new group command
func NewGroupCommand(name, description string, checker *PermissionChecker) *GroupCommand {
	return &GroupCommand{
		name:        name,
		description: description,
		subcommands: make(map[string]Command),
		checker:     checker,
	}
}

// AddSubCommand appends a subcommand to the group hierarchy
func (gc *GroupCommand) AddSubCommand(subcmd Command) {
	gc.subcommands[subcmd.Name()] = subcmd
}

// Name exposes the command name
func (gc *GroupCommand) Name() string {
	return gc.name
}

// Description exposes the functional description of the command
func (gc *GroupCommand) Description() string {
	return gc.description
}

// Options builds command options based on the subcommand tree
func (gc *GroupCommand) Options() []*discordgo.ApplicationCommandOption {
	options := make([]*discordgo.ApplicationCommandOption, 0, len(gc.subcommands))

	for _, subcmd := range gc.subcommands {
		// Type evaluation: if the subcommand itself has options that are also subcommands,
		// this entry must be a SubCommandGroup (Type 2).
		// Otherwise, it is classified as a regular SubCommand (Type 1).
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

// RequiresGuild validates guild infrastructure dependency on child subcommands
func (gc *GroupCommand) RequiresGuild() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresGuild() {
			return true
		}
	}
	return false
}

// RequiresPermissions validates the presence of permission restrictions on child subcommands
func (gc *GroupCommand) RequiresPermissions() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresPermissions() {
			return true
		}
	}
	return false
}

// Handle delegates control flow to the qualified subcommand
func (gc *GroupCommand) Handle(ctx *Context) error {
	subCommandName := GetSubCommandName(ctx.Interaction)
	if subCommandName == "" {
		slog.Warn("Service degradation intercepted and mitigated",
			slog.String("reason", "missing subcommand in restricted group"),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "This command requires a subcommand before proceeding, so this response remains private.", Ephemeral: true}
	}

	subcmd, exists := gc.subcommands[subCommandName]
	if !exists {
		slog.Warn("Service degradation intercepted and mitigated",
			slog.String("reason", "requested subcommand does not exist"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "The subcommand could not be matched, so this response remains private.", Ephemeral: true}
	}

	// Validation of subcommand-specific scope permissions
	if subcmd.RequiresGuild() && ctx.GuildID == "" {
		slog.Warn("Service degradation intercepted and mitigated",
			slog.String("reason", "execution outside guild in subcommand dependent on guild state"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "This subcommand only works inside a server, so this failure remains private.", Ephemeral: true}
	}

	if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		slog.Warn("Service degradation intercepted and mitigated",
			slog.String("reason", "mitigated violation of guild access control"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "Access denied to this subcommand, therefore this response remains private.", Ephemeral: true}
	}

	if subcmd.RequiresPermissions() && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		slog.Warn("Service degradation intercepted and mitigated",
			slog.String("reason", "mitigated violation of strict runtime permissions"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "Access denied to this subcommand, therefore this response remains private.", Ephemeral: true}
	}

	return subcmd.Handle(ctx)
}

// SimpleCommand implements the Command interface for atomic instructions
type SimpleCommand struct {
	name                string
	description         string
	options             []*discordgo.ApplicationCommandOption
	handler             func(ctx *Context) error
	autocompleteHandler AutocompleteHandler
	requiresGuild       bool
	requiresPermissions bool
}

// NewSimpleCommand allocates and initializes an atomic command
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

// WithAutocomplete binds an autocomplete handler to the command route path.
func (sc *SimpleCommand) WithAutocomplete(handler AutocompleteHandler) *SimpleCommand {
	if sc == nil {
		return nil
	}
	sc.autocompleteHandler = handler
	return sc
}

// Name exposes the route name.
func (sc *SimpleCommand) Name() string { return sc.name }

// Description exposes the entity description.
func (sc *SimpleCommand) Description() string { return sc.description }

// Options exposes the structural parameters of the entity.
func (sc *SimpleCommand) Options() []*discordgo.ApplicationCommandOption {
	return sc.options
}

// Handle invokes the allocated primary handler.
func (sc *SimpleCommand) Handle(ctx *Context) error { return sc.handler(ctx) }

// AutocompleteRouteHandler exposes the autocomplete of the handler route.
func (sc *SimpleCommand) AutocompleteRouteHandler() AutocompleteHandler {
	return sc.autocompleteHandler
}

// RequiresGuild signals guild infrastructure dependency.
func (sc *SimpleCommand) RequiresGuild() bool { return sc.requiresGuild }

// RequiresPermissions signals strict permission enforcement.
func (sc *SimpleCommand) RequiresPermissions() bool { return sc.requiresPermissions }

// GetSession extracts the primary Discord session contained in the context builder
func (cr *CommandRouter) GetSession() *discordgo.Session {
	return cr.contextBuilder.session
}

// GetConfigManager extracts the configuration state manager contained in the context builder
func (cr *CommandRouter) GetConfigManager() *files.ConfigManager {
	return cr.contextBuilder.configManager
}

// formatOptions applies formatted serialization to command options to inject into telemetry
func formatOptions(options []*discordgo.ApplicationCommandOption) string {
	if len(options) == 0 {
		return ""
	}
	var parts []string
	for _, opt := range options {
		parts = append(parts, fmt.Sprintf("%s (%s)", opt.Name, opt.Type.String()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// GetRegistry exposes the active command registry
func (cr *CommandRouter) GetRegistry() *CommandRegistry {
	return cr.registry
}

// GetPermissionChecker exposes the global scope permission checker
func (cr *CommandRouter) GetPermissionChecker() *PermissionChecker {
	return cr.permChecker
}

// SetStore injects the storage provider for the permission checker to enable local OwnerID cache.
func (cr *CommandRouter) SetStore(store *storage.Store) {
	cr.store = store
	if cr.permChecker != nil {
		cr.permChecker.SetStore(store)
	}
}

// GetStore exposes the coupled storage provider, if defined.
func (cr *CommandRouter) GetStore() *storage.Store {
	return cr.store
}

// SetCache overlays the unified cache on the permission checker to mitigate external API calls.
func (cr *CommandRouter) SetCache(unifiedCache *cache.UnifiedCache) {
	if cr.permChecker != nil {
		cr.permChecker.SetCache(unifiedCache)
	}
}

// SetRuntimeApplier injects the runtime hot-apply manager.
// Classified as optional; omitting it disables dynamic hot reloads.
func (cr *CommandRouter) SetRuntimeApplier(applier *runtimeapply.Manager) {
	cr.runtimeApplier = applier
}

// GetRuntimeApplier exposes the allocated instance of the runtime hot-apply manager.
func (cr *CommandRouter) GetRuntimeApplier() *runtimeapply.Manager {
	return cr.runtimeApplier
}

// SetTaskRouter couples the structural router for asynchronous tasks
func (cr *CommandRouter) SetTaskRouter(router *task.TaskRouter) {
	cr.taskRouter = router
	if cr.taskRouter != nil {
		RegisterOrphanCleanupTask(cr.taskRouter, cr.GetSession(), cr.GetConfigManager())
	}
}

// GetTaskRouter exposes the configured task router
func (cr *CommandRouter) GetTaskRouter() *task.TaskRouter {
	return cr.taskRouter
}
