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

// CommandRouter gerencia o roteamento e a execução de comandos
type CommandRouter struct {
	registry       *CommandRegistry
	routeRegistry  *interactionRouteRegistry
	contextBuilder *ContextBuilder
	middlewares    []InteractionMiddleware

	permChecker      *PermissionChecker
	store            *storage.Store
	guildFilter      func(string) bool
	guildRouteFilter func(string, InteractionRouteKey) bool

	// runtimeApplier consiste em um gerenciador opcional de aplicação a quente compartilhado (tema + alternadores ALICE_DISABLE_*).
	// Configurado pelo executor da aplicação, permite que manipuladores de interação apliquem alterações
	// imediatamente após a persistência de mudanças de configuração em tempo de execução.
	runtimeApplier *runtimeapply.Manager

	// taskRouter consiste em um roteador de tarefas compartilhado opcional (preenchimento retroativo, notificações assíncronas).
	taskRouter *task.TaskRouter
}

// NewCommandRouter aloca e inicializa um novo roteador de comandos
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

		permChecker: permChecker,
	}
	router.UseMiddleware(defaultInteractionMiddlewares(router)...)

	slog.Info("Transição de estado arquitetural: inicialização de rotinas primárias", slog.String("component", "CommandRouter"))

	return router
}

// RegisterSlashCommand registra uma árvore de comandos de barra simultaneamente no registro
// de sincronização e no registro de rotas de barra.
func (cr *CommandRouter) RegisterSlashCommand(cmd Command) {
	cr.RegisterSlashCommandForDomain("", cmd)
}

// RegisterSlashCommandForDomain registra uma árvore de comandos de barra simultaneamente no
// registro de sincronização e no registro de rotas de barra sob o domínio solicitado.
func (cr *CommandRouter) RegisterSlashCommandForDomain(domain string, cmd Command) {
	if cr == nil || cmd == nil {
		return
	}
	cr.registry.Register(cmd)
	cr.registerSlashCommandRoutesForDomain(domain, cmd)
}

// RegisterCommand consiste na API de compatibilidade; priorize RegisterSlashCommand para novas árvores de barra.
func (cr *CommandRouter) RegisterCommand(cmd Command) {
	cr.RegisterSlashCommand(cmd)
}

// RegisterSlashSubCommand registra um subcomando de barra simultaneamente no registro
// de sincronização e no registro de rotas de barra.
func (cr *CommandRouter) RegisterSlashSubCommand(parentName string, subcmd Command) {
	cr.RegisterSlashSubCommandForDomain("", parentName, subcmd)
}

// RegisterSlashSubCommandForDomain registra um subcomando de barra simultaneamente no
// registro de sincronização e no registro de rotas de barra sob o domínio solicitado.
func (cr *CommandRouter) RegisterSlashSubCommandForDomain(domain, parentName string, subcmd Command) {
	if cr == nil || subcmd == nil {
		return
	}
	cr.registry.RegisterSubCommand(parentName, subcmd)
	cr.registerSlashSubCommandRoutesForDomain(domain, parentName, subcmd)
}

// RegisterSubCommand consiste na API de compatibilidade; priorize RegisterSlashSubCommand para novas árvores de barra.
func (cr *CommandRouter) RegisterSubCommand(parentName string, subcmd Command) {
	cr.RegisterSlashSubCommand(parentName, subcmd)
}

// RegisterAutocomplete acopla um manipulador de autocompletar via caminho de rota canônico.
// Consiste na API de compatibilidade; priorize um AutocompleteRouteProvider na
// árvore de barra ou RegisterInteractionRoute para código novo.
func (cr *CommandRouter) RegisterAutocomplete(routePath string, handler AutocompleteHandler) {
	cr.RegisterAutocompleteRoute(routePath, handler)
}

// RegisterComponentHandler acopla um manipulador de componentes para um ID de rota de componente exato.
// Consiste na API de compatibilidade; priorize RegisterInteractionRoute para código novo.
func (cr *CommandRouter) RegisterComponentHandler(routeID string, handler ComponentHandler) {
	cr.RegisterComponentRoute(routeID, handler)
}

// RegisterModalHandler acopla um manipulador de modal para um ID de rota de modal exato.
// Consiste na API de compatibilidade; priorize RegisterInteractionRoute para código novo.
func (cr *CommandRouter) RegisterModalHandler(routeID string, handler ModalHandler) {
	cr.RegisterModalRoute(routeID, handler)
}

// SetGuildFilter restringe o processamento de interações a guildas validadas pelo predicado fornecido.
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

// SetGuildRouteFilter restringe o processamento de interações a pares guilda/rota
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

// CommandManager orquestra o ciclo de vida dos comandos na infraestrutura do Discord
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

// NewCommandManager aloca e inicializa um novo gerenciador de comandos
func NewCommandManager(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandManager {
	slog.Info("Transição de estado arquitetural: inicialização de rotinas primárias", slog.String("component", "CommandManager"))

	return &CommandManager{
		session:       session,
		router:        NewCommandRouter(session, configManager),
		arikawaRouter: NewArikawaCommandRouter(session.Token, configManager),
		logger:        log.GlobalLogger,
	}
}

// GetArikawaRouter expõe o roteador de comandos Arikawa
func (cm *CommandManager) GetArikawaRouter() *ArikawaCommandRouter {
	return cm.arikawaRouter
}

// GetRouter expõe o roteador de comandos primário
func (cm *CommandManager) GetRouter() *CommandRouter {
	return cm.router
}

// SetupCommands configura e alinha o estado local de comandos com a API do Discord
func (cm *CommandManager) SetupCommands() error {
	// Valida a integridade do estado da sessão
	if cm.session == nil || cm.session.State == nil || cm.session.State.User == nil {
		err := fmt.Errorf("estado da sessão não inicializado adequadamente")
		slog.Error("Falha estrutural bloqueante restrita ao escopo da operação",
			slog.String("req_id", "sys-init"),
			slog.String("stack_trace", string(debug.Stack())),
			slog.Int("fail_id", 500),
			slog.String("error", err.Error()),
		)
		return err
	}

	// Previne a duplicação de processamento de interações em ciclos de reinicialização/recarga a quente.
	if cm.interactionHandlerCancel != nil {
		cm.interactionHandlerCancel()
		cm.interactionHandlerCancel = nil
	}
	if cm.rawEventHandlerCancel != nil {
		cm.rawEventHandlerCancel()
		cm.rawEventHandlerCancel = nil
	}
	cm.interactionHandlerCancel = cm.session.AddHandler(cm.router.HandleInteraction)
	cm.rawEventHandlerCancel = cm.session.AddHandler(cm.arikawaRouter.HandleRawEvent)

	slog.Info("Transição de estado arquitetural: acoplamento de manipuladores de eventos assíncronos", slog.String("component", "CommandManager"))

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

	// Despacha tarefa de varredura em segundo plano estritamente após a conclusão do sinal READY
	scheduleOrphanCleanupTask(cm.router.GetTaskRouter(), cm.session)
	slog.Info("Transição de estado arquitetural: inicialização de rotinas primárias assíncronas", slog.String("task", "orphan_cleanup"))

	return nil
}

// Shutdown desvincula os manipuladores de interação de comandos.
func (cm *CommandManager) Shutdown() error {
	slog.Info("Transição de estado arquitetural: encerramento planejado de instâncias principais", slog.String("component", "CommandManager"))

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

// SyncGuildCommands dispara uma sincronização cirúrgica na API do Discord para uma guilda única.
// Retorna nulo se o gerenciador estiver configurado para sincronização global.
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

	slog.Info("Transição de estado arquitetural: conclusão de sincronização de escopo de guilda",
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

	// Extrai comandos Arikawa (ausência de um construtor de grupos de Guilda complexo nesta versão)
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

// commandDefaultMemberPermissions extrai a base de permissões do Discord para
// embutir no descritor de nível superior para o cmd, ou nulo quando o cmd não
// declara nenhuma. O Discord exige um ponteiro; nulo preserva o comportamento anterior
// focado apenas em "permissionGateMiddleware".
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
		errWrap := fmt.Errorf("falha ao buscar comandos registrados para o escopo %s: %w", commandSyncScopeLabel(guildID), err)
		slog.Error("Falha estrutural bloqueante restrita ao escopo da operação",
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
				slog.Debug("Inspeção granular de estado transiente: comando inalterado",
					slog.String("scope", commandSyncScopeLabel(guildID)),
					slog.String("command", name),
					slog.String("options", formatOptions(desiredCommand.Options)),
				)
				summary.unchanged++
				continue
			}

			slog.Debug("Inspeção granular de estado transiente: comando atualizado",
				slog.String("scope", commandSyncScopeLabel(guildID)),
				slog.String("command", name),
				slog.String("options", formatOptions(desiredCommand.Options)),
			)
			summary.updated++
			needsSync = true
			continue
		}

		slog.Debug("Inspeção granular de estado transiente: comando criado",
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
		slog.Debug("Inspeção granular de estado transiente: comando órfão removido",
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
			errWrap := fmt.Errorf("erro ao sobrescrever comandos em massa no escopo %s: %w", commandSyncScopeLabel(guildID), err)
			slog.Error("Falha estrutural bloqueante restrita ao escopo da operação",
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

// GroupCommand encapsula um comando que contém subcomandos
type GroupCommand struct {
	name        string
	description string
	subcommands map[string]Command
	checker     *PermissionChecker
}

// NewGroupCommand aloca um novo comando de grupo
func NewGroupCommand(name, description string, checker *PermissionChecker) *GroupCommand {
	return &GroupCommand{
		name:        name,
		description: description,
		subcommands: make(map[string]Command),
		checker:     checker,
	}
}

// AddSubCommand anexa um subcomando à hierarquia do grupo
func (gc *GroupCommand) AddSubCommand(subcmd Command) {
	gc.subcommands[subcmd.Name()] = subcmd
}

// Name expõe a nomenclatura do comando
func (gc *GroupCommand) Name() string {
	return gc.name
}

// Description expõe a descrição funcional do comando
func (gc *GroupCommand) Description() string {
	return gc.description
}

// Options constrói as opções do comando com base na árvore de subcomandos
func (gc *GroupCommand) Options() []*discordgo.ApplicationCommandOption {
	options := make([]*discordgo.ApplicationCommandOption, 0, len(gc.subcommands))

	for _, subcmd := range gc.subcommands {
		// Avaliação de tipo: se o subcomando em si possui opções que também são subcomandos,
		// esta entrada obrigatoriamente torna-se um SubCommandGroup (Tipo 2).
		// Caso contrário, classifica-se como um SubCommand regular (Tipo 1).
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

// RequiresGuild valida a dependência de infraestrutura de servidor nos subcomandos filhos
func (gc *GroupCommand) RequiresGuild() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresGuild() {
			return true
		}
	}
	return false
}

// RequiresPermissions valida a presença de restrições de permissões nos subcomandos filhos
func (gc *GroupCommand) RequiresPermissions() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresPermissions() {
			return true
		}
	}
	return false
}

// Handle delega o fluxo de controle para o subcomando qualificado
func (gc *GroupCommand) Handle(ctx *Context) error {
	subCommandName := GetSubCommandName(ctx.Interaction)
	if subCommandName == "" {
		slog.Warn("Degradação de serviço interceptada e mitigada",
			slog.String("reason", "subcomando ausente em grupo restrito"),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "Este comando requer um subcomando antes de prosseguir, portanto esta resposta permanece privada.", Ephemeral: true}
	}

	subcmd, exists := gc.subcommands[subCommandName]
	if !exists {
		slog.Warn("Degradação de serviço interceptada e mitigada",
			slog.String("reason", "subcomando solicitado inexistente"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "O subcomando não pôde ser correspondido, portanto esta resposta permanece privada.", Ephemeral: true}
	}

	// Validação de permissões específicas de escopo do subcomando
	if subcmd.RequiresGuild() && ctx.GuildID == "" {
		slog.Warn("Degradação de serviço interceptada e mitigada",
			slog.String("reason", "execução fora de guilda em subcomando dependente de estado de servidor"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "Este subcomando funciona apenas dentro de um servidor, portanto esta falha permanece privada.", Ephemeral: true}
	}

	if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		slog.Warn("Degradação de serviço interceptada e mitigada",
			slog.String("reason", "violação mitigada de controle de acesso de guilda"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "Acesso negado a este subcomando, portanto esta resposta permanece privada.", Ephemeral: true}
	}

	if subcmd.RequiresPermissions() && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		slog.Warn("Degradação de serviço interceptada e mitigada",
			slog.String("reason", "violação mitigada de permissões estritas em tempo de execução"),
			slog.String("subcommand", subCommandName),
			slog.String("user_id", ctx.UserID),
		)
		return &CommandError{Message: "Acesso negado a este subcomando, portanto esta resposta permanece privada.", Ephemeral: true}
	}

	return subcmd.Handle(ctx)
}

// SimpleCommand implementa a interface Command para instruções atômicas
type SimpleCommand struct {
	name                string
	description         string
	options             []*discordgo.ApplicationCommandOption
	handler             func(ctx *Context) error
	autocompleteHandler AutocompleteHandler
	requiresGuild       bool
	requiresPermissions bool
}

// NewSimpleCommand aloca e inicializa um comando atômico
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

// WithAutocomplete acopla um manipulador de autocompletar ao caminho de rota do comando.
func (sc *SimpleCommand) WithAutocomplete(handler AutocompleteHandler) *SimpleCommand {
	if sc == nil {
		return nil
	}
	sc.autocompleteHandler = handler
	return sc
}

// Name expõe a nomenclatura de rota.
func (sc *SimpleCommand) Name() string { return sc.name }

// Description expõe a descrição da entidade.
func (sc *SimpleCommand) Description() string { return sc.description }

// Options expõe os parâmetros estruturais da entidade.
func (sc *SimpleCommand) Options() []*discordgo.ApplicationCommandOption {
	return sc.options
}

// Handle invoca o manipulador primário alocado.
func (sc *SimpleCommand) Handle(ctx *Context) error { return sc.handler(ctx) }

// AutocompleteRouteHandler expõe o autocompletar da rota de manipulador.
func (sc *SimpleCommand) AutocompleteRouteHandler() AutocompleteHandler {
	return sc.autocompleteHandler
}

// RequiresGuild sinaliza dependência infraestrutural de guilda.
func (sc *SimpleCommand) RequiresGuild() bool { return sc.requiresGuild }

// RequiresPermissions sinaliza a imposição estrita de permissões.
func (sc *SimpleCommand) RequiresPermissions() bool { return sc.requiresPermissions }

// GetSession extrai a sessão primária do Discord contida no construtor de contexto
func (cr *CommandRouter) GetSession() *discordgo.Session {
	return cr.contextBuilder.session
}

// GetConfigManager extrai o gerenciador de estado de configuração contido no construtor de contexto
func (cr *CommandRouter) GetConfigManager() *files.ConfigManager {
	return cr.contextBuilder.configManager
}

// formatOptions aplica serialização formatada nas opções de comando para injetar na telemetria
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

// GetRegistry expõe o registro ativo de comandos
func (cr *CommandRouter) GetRegistry() *CommandRegistry {
	return cr.registry
}

// GetPermissionChecker expõe o validador de permissões de escopo global
func (cr *CommandRouter) GetPermissionChecker() *PermissionChecker {
	return cr.permChecker
}

// SetStore injeta o provedor de armazenamento para o validador de permissões habilitar cachê local de OwnerID.
func (cr *CommandRouter) SetStore(store *storage.Store) {
	cr.store = store
	if cr.permChecker != nil {
		cr.permChecker.SetStore(store)
	}
}

// GetStore expõe o provedor de armazenamento acoplado, caso esteja definido.
func (cr *CommandRouter) GetStore() *storage.Store {
	return cr.store
}

// SetCache sobrepõe o cache unificado no validador de permissões para atenuar chamadas de API externas.
func (cr *CommandRouter) SetCache(unifiedCache *cache.UnifiedCache) {
	if cr.permChecker != nil {
		cr.permChecker.SetCache(unifiedCache)
	}
}

// SetRuntimeApplier injeta o gerenciador de aplicação a quente de tempo de execução.
// Classificado como opcional; omiti-lo inativa os recarregamentos dinâmicos a quente.
func (cr *CommandRouter) SetRuntimeApplier(applier *runtimeapply.Manager) {
	cr.runtimeApplier = applier
}

// GetRuntimeApplier expõe a instância alocada do gerenciador de aplicação a quente de tempo de execução.
func (cr *CommandRouter) GetRuntimeApplier() *runtimeapply.Manager {
	return cr.runtimeApplier
}

// SetTaskRouter acopla o roteador estrutural de tarefas assíncronas
func (cr *CommandRouter) SetTaskRouter(router *task.TaskRouter) {
	cr.taskRouter = router
	if cr.taskRouter != nil {
		RegisterOrphanCleanupTask(cr.taskRouter, cr.GetSession(), cr.GetConfigManager())
	}
}

// GetTaskRouter expõe o roteador de tarefas configurado
func (cr *CommandRouter) GetTaskRouter() *task.TaskRouter {
	return cr.taskRouter
}
