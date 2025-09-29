package core

import (
	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// Command representa um comando Discord
type Command interface {
	Name() string
	Description() string
	Options() []*discordgo.ApplicationCommandOption
	Handle(ctx *Context) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// SubCommand representa um subcomando dentro de um comando maior
type SubCommand interface {
	Name() string
	Description() string
	Options() []*discordgo.ApplicationCommandOption
	Handle(ctx *Context) error
	RequiresGuild() bool
	RequiresPermissions() bool
}

// Context fornece contexto unificado para execução de comandos
type Context struct {
	Session     *discordgo.Session
	Interaction *discordgo.InteractionCreate
	Config      *files.ConfigManager
	Logger      *log.Logger
	GuildID     string
	UserID      string
	IsOwner     bool
	GuildConfig *files.GuildConfig
}

// Response padroniza respostas de comandos
type Response struct {
	Content   string
	Ephemeral bool
	Success   bool
}

// BaseHandler fornece funcionalidades comuns para todos os handlers
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

// GetSession retorna a sessão do Discord
func (bh *BaseHandler) GetSession() *discordgo.Session {
	return bh.session
}

// GetConfigManager retorna o gerenciador de configuração
func (bh *BaseHandler) GetConfigManager() *files.ConfigManager {
	return bh.configManager
}

// GetAvatarCacheManager retorna o gerenciador de cache de avatar

// CommandRegistry gerencia registro e execução de comandos
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

// Register registra um comando no registry
func (r *CommandRegistry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

// RegisterSubCommand registra um subcomando no registry
func (r *CommandRegistry) RegisterSubCommand(parentName string, subcmd SubCommand) {
	if r.subcommands[parentName] == nil {
		r.subcommands[parentName] = make(map[string]SubCommand)
	}
	r.subcommands[parentName][subcmd.Name()] = subcmd
}

// GetCommand retorna um comando pelo nome
func (r *CommandRegistry) GetCommand(name string) (Command, bool) {
	cmd, exists := r.commands[name]
	return cmd, exists
}

// GetSubCommand retorna um subcomando pelo nome do comando pai e nome do subcomando
func (r *CommandRegistry) GetSubCommand(parentName, subName string) (SubCommand, bool) {
	if subs, exists := r.subcommands[parentName]; exists {
		if sub, exists := subs[subName]; exists {
			return sub, true
		}
	}
	return nil, false
}

// GetAllCommands retorna todos os comandos registrados
func (r *CommandRegistry) GetAllCommands() map[string]Command {
	return r.commands
}

// GetAllSubCommands retorna todos os subcomandos de um comando
func (r *CommandRegistry) GetAllSubCommands(parentName string) map[string]SubCommand {
	if subs, exists := r.subcommands[parentName]; exists {
		return subs
	}
	return make(map[string]SubCommand)
}

// CommandMeta define metadados para construção de comandos
type CommandMeta struct {
	Name        string
	Description string
	Options     []*discordgo.ApplicationCommandOption
}

// SubCommandMeta define metadados para construção de subcomandos
type SubCommandMeta struct {
	Name        string
	Description string
	Options     []*discordgo.ApplicationCommandOption
}

// AutocompleteHandler define um handler para autocomplete
type AutocompleteHandler interface {
	HandleAutocomplete(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error)
}

// PermissionLevel define níveis de permissão para comandos
type PermissionLevel int

const (
	PermissionNone PermissionLevel = iota
	PermissionUser
	PermissionModerator
	PermissionAdmin
	PermissionOwner
)

// CommandError representa erros específicos de comandos
type CommandError struct {
	Message   string
	Ephemeral bool
	Code      string
}

func (e *CommandError) Error() string {
	return e.Message
}

// NewCommandError cria um novo erro de comando
func NewCommandError(message string, ephemeral bool) *CommandError {
	return &CommandError{
		Message:   message,
		Ephemeral: ephemeral,
	}
}

// ValidationError representa erros de validação
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError cria um novo erro de validação
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
