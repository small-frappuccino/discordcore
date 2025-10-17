package core

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// CommandRouter gerencia o roteamento e execução de comandos
type CommandRouter struct {
	registry        *CommandRegistry
	contextBuilder  *ContextBuilder
	responder       *Responder
	permChecker     *PermissionChecker
	autocompleteMap map[string]AutocompleteHandler
}

// NewCommandRouter cria um novo roteador de comandos
func NewCommandRouter(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandRouter {
	registry := NewCommandRegistry()
	responder := NewResponder(session)
	permChecker := NewPermissionChecker(session, configManager)
	contextBuilder := NewContextBuilder(session, configManager, permChecker)

	return &CommandRouter{
		registry:        registry,
		contextBuilder:  contextBuilder,
		responder:       responder,
		permChecker:     permChecker,
		autocompleteMap: make(map[string]AutocompleteHandler),
	}
}

// RegisterCommand registra um comando simples
func (cr *CommandRouter) RegisterCommand(cmd Command) {
	cr.registry.Register(cmd)
}

// RegisterSubCommand registra um subcomando
func (cr *CommandRouter) RegisterSubCommand(parentName string, subcmd SubCommand) {
	cr.registry.RegisterSubCommand(parentName, subcmd)
}

// RegisterAutocomplete registra um handler de autocomplete
func (cr *CommandRouter) RegisterAutocomplete(commandName string, handler AutocompleteHandler) {
	cr.autocompleteMap[commandName] = handler
}

// HandleInteraction roteia interações para os handlers apropriados
func (cr *CommandRouter) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if IsAutocompleteInteraction(i) {
		cr.handleAutocomplete(i)
		return
	}

	if !IsSlashCommandInteraction(i) {
		return
	}

	cr.handleSlashCommand(i)
}

// handleSlashCommand processa comandos slash
func (cr *CommandRouter) handleSlashCommand(i *discordgo.InteractionCreate) {
	ctx := cr.contextBuilder.BuildContext(i)
	commandName := i.ApplicationCommandData().Name

	ctx.Logger.Info().Applicationf("Processing slash command")

	// Verificar se o comando existe
	cmd, exists := cr.registry.GetCommand(commandName)
	if !exists {
		ctx.Logger.Error().Errorf("Command not found")
		cr.responder.Error(i, "Command not found")
		return
	}

	// Verificar se requer servidor
	if cmd.RequiresGuild() && ctx.GuildID == "" {
		ctx.Logger.Warn().Applicationf("Command used outside of guild")
		cr.responder.Error(i, "This command can only be used in a server")
		return
	}

	// Verificar permissões
	if cmd.RequiresPermissions() && !cr.permChecker.HasPermission(ctx.GuildID, ctx.UserID) {
		ctx.Logger.Warn().Applicationf("User without permission tried to use command")
		cr.responder.Error(i, "You do not have permission to use this command")
		return
	}

	// Executar comando
	ctx.Logger.Info().Applicationf("Executing command")
	if err := cmd.Handle(ctx); err != nil {
		ctx.Logger.Error().Errorf("Command execution failed: %v", err)

		// Verificar se é um erro específico de comando
		if cmdErr, ok := err.(*CommandError); ok {
			if cmdErr.Ephemeral {
				cr.responder.Ephemeral(i, cmdErr.Message)
			} else {
				cr.responder.Error(i, cmdErr.Message)
			}
		} else {
			cr.responder.Error(i, "An error occurred while executing the command")
		}
	}
}

// handleAutocomplete processa interações de autocomplete
func (cr *CommandRouter) handleAutocomplete(i *discordgo.InteractionCreate) {
	ctx := cr.contextBuilder.BuildContext(i)
	commandName := i.ApplicationCommandData().Name

	// Buscar handler de autocomplete
	handler, exists := cr.autocompleteMap[commandName]
	if !exists {
		cr.responder.Autocomplete(i, []*discordgo.ApplicationCommandOptionChoice{})
		return
	}

	// Encontrar a opção com foco
	focusedOpt, hasFocus := HasFocusedOption(i.ApplicationCommandData().Options)
	if !hasFocus {
		cr.responder.Autocomplete(i, []*discordgo.ApplicationCommandOptionChoice{})
		return
	}

	// Executar autocomplete
	choices, err := handler.HandleAutocomplete(ctx, focusedOpt.Name)
	if err != nil {
		ctx.Logger.Error().Errorf("Autocomplete handler failed: %v", err)
		choices = []*discordgo.ApplicationCommandOptionChoice{}
	}

	cr.responder.Autocomplete(i, choices)
}

// CommandManager gerencia o ciclo de vida dos comandos no Discord
type CommandManager struct {
	session *discordgo.Session
	router  *CommandRouter
	logger  *log.Logger
}

// NewCommandManager cria um novo gerenciador de comandos
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

// GetRouter retorna o roteador de comandos
func (cm *CommandManager) GetRouter() *CommandRouter {
	return cm.router
}

// SetupCommands configura e sincroniza comandos com o Discord
func (cm *CommandManager) SetupCommands() error {
	// Registrar handler de interações
	cm.session.AddHandler(cm.router.HandleInteraction)

	// Verify session state is properly initialized
	if cm.session == nil || cm.session.State == nil || cm.session.State.User == nil {
		return fmt.Errorf("session not properly initialized")
	}

	// Obter comandos já registrados no Discord
	registered, err := cm.session.ApplicationCommands(cm.session.State.User.ID, "")
	if err != nil {
		return fmt.Errorf("failed to fetch registered commands: %w", err)
	}

	// Criar mapa de comandos registrados
	regByName := make(map[string]*discordgo.ApplicationCommand, len(registered))
	for _, rc := range registered {
		regByName[rc.Name] = rc
	}

	// Criar mapa de comandos do código
	codeCommands := cm.router.registry.GetAllCommands()
	codeByName := make(map[string]Command, len(codeCommands))
	for name, cmd := range codeCommands {
		codeByName[name] = cmd
	}

	// Criar/Atualizar comandos conforme necessário
	created, updated, unchanged := 0, 0, 0
	for name, cmd := range codeCommands {
		desired := &discordgo.ApplicationCommand{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			Options:     cmd.Options(),
		}

		if existing, ok := regByName[name]; ok {
			// Comando já existe, verificar se precisa atualizar
			if CompareCommands(existing, desired) {
				cm.logger.Info().Applicationf("Command unchanged, skipping: %s", name)
				unchanged++
				continue
			}

			// Atualizar comando
			if _, err := cm.session.ApplicationCommandEdit(cm.session.State.User.ID, "", existing.ID, desired); err != nil {
				return fmt.Errorf("error updating command '%s': %w", name, err)
			}
			cm.logger.Info().Applicationf("Command updated: %s", name)
			updated++
		} else {
			// Criar novo comando
			if _, err := cm.session.ApplicationCommandCreate(cm.session.State.User.ID, "", desired); err != nil {
				return fmt.Errorf("error creating command '%s': %w", name, err)
			}
			cm.logger.Info().Applicationf("Command created: %s", name)
			created++
		}
	}

	// Remover comandos órfãos (existem no Discord mas não no código)
	deleted := 0
	for _, rc := range registered {
		if _, exists := codeByName[rc.Name]; !exists {
			if err := cm.session.ApplicationCommandDelete(cm.session.State.User.ID, "", rc.ID); err != nil {
				cm.logger.Warn().Applicationf("Error removing orphan command: %s, error: %v", rc.Name, err)
				continue
			}
			cm.logger.Info().Applicationf("Orphan command removed: %s", rc.Name)
			deleted++
		}
	}
	// Log do resumo
	cm.logger.Info().Applicationf("Command synchronization completed: created=%d, updated=%d, deleted=%d, unchanged=%d, total=%d, mode=incremental", created, updated, deleted, unchanged, len(codeCommands))

	return nil
}

// GroupCommand representa um comando que contém subcomandos
type GroupCommand struct {
	name        string
	description string
	subcommands map[string]SubCommand
	responder   *Responder
	checker     *PermissionChecker
}

// NewGroupCommand cria um novo comando de grupo
func NewGroupCommand(name, description string, responder *Responder, checker *PermissionChecker) *GroupCommand {
	return &GroupCommand{
		name:        name,
		description: description,
		subcommands: make(map[string]SubCommand),
		responder:   responder,
		checker:     checker,
	}
}

// AddSubCommand adiciona um subcomando ao grupo
func (gc *GroupCommand) AddSubCommand(subcmd SubCommand) {
	gc.subcommands[subcmd.Name()] = subcmd
}

// Name retorna o nome do comando
func (gc *GroupCommand) Name() string {
	return gc.name
}

// Description retorna a descrição do comando
func (gc *GroupCommand) Description() string {
	return gc.description
}

// Options constrói as opções do comando baseadas nos subcomandos
func (gc *GroupCommand) Options() []*discordgo.ApplicationCommandOption {
	options := make([]*discordgo.ApplicationCommandOption, 0, len(gc.subcommands))

	for _, subcmd := range gc.subcommands {
		option := &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        subcmd.Name(),
			Description: subcmd.Description(),
			Options:     subcmd.Options(),
		}
		options = append(options, option)
	}

	return options
}

// RequiresGuild verifica se algum subcomando requer servidor
func (gc *GroupCommand) RequiresGuild() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresGuild() {
			return true
		}
	}
	return false
}

// RequiresPermissions verifica se algum subcomando requer permissões
func (gc *GroupCommand) RequiresPermissions() bool {
	for _, subcmd := range gc.subcommands {
		if subcmd.RequiresPermissions() {
			return true
		}
	}
	return false
}

// Handle roteia para o subcomando apropriado
func (gc *GroupCommand) Handle(ctx *Context) error {
	subCommandName := GetSubCommandName(ctx.Interaction)
	if subCommandName == "" {
		return NewCommandError("No subcommand specified", true)
	}

	subcmd, exists := gc.subcommands[subCommandName]
	if !exists {
		return NewCommandError("Unknown subcommand", true)
	}

	// Verificar permissões específicas do subcomando
	if subcmd.RequiresGuild() && ctx.GuildID == "" {
		return NewCommandError("This subcommand can only be used in a server", true)
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

// GetResponder returns the responder
func (cr *CommandRouter) GetResponder() *Responder {
	return cr.responder
}

// GetPermissionChecker returns the permission checker
func (cr *CommandRouter) GetPermissionChecker() *PermissionChecker {
	return cr.permChecker
}

// SetStore sets the shared store for the permission checker to enable local OwnerID cache usage.
func (cr *CommandRouter) SetStore(store *storage.Store) {
	if cr.permChecker != nil {
		cr.permChecker.SetStore(store)
	}
}

// SetCache sets the unified cache for the permission checker to reduce API calls.
func (cr *CommandRouter) SetCache(unifiedCache *cache.UnifiedCache) {
	if cr.permChecker != nil {
		cr.permChecker.SetCache(unifiedCache)
	}
}
