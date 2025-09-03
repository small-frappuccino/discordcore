package core

import (
	"fmt"
	"strings"

	"github.com/alice-bnuy/discordcore/v2/internal/cache"
	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/bwmarrin/discordgo"
)

// Este arquivo contém exemplos de como usar a infraestrutura core
// para criar comandos Discord de forma modular e reutilizável.

// ======================
// Exemplo 1: Comando Simples
// ======================

// PingCommand é um exemplo de comando simples
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
	return nil // Comando simples sem opções
}

func (c *PingCommand) RequiresGuild() bool {
	return false // Pode ser usado em DM
}

func (c *PingCommand) RequiresPermissions() bool {
	return false // Todos podem usar
}

func (c *PingCommand) Handle(ctx *Context) error {
	responder := NewResponder(ctx.Session)
	return responder.Success(ctx.Interaction, "🏓 Pong!")
}

// ======================
// Exemplo 2: Comando com Opções
// ======================

// EchoCommand demonstra como usar opções e extração de dados
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
	// Extrair opções do comando
	extractor := NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)

	message, err := extractor.StringRequired("message")
	if err != nil {
		return err
	}

	ephemeral := extractor.Bool("ephemeral")

	// Usar ResponseBuilder para resposta mais flexível
	builder := NewResponseBuilder(ctx.Session)
	if ephemeral {
		builder = builder.Ephemeral()
	}

	return builder.Info(ctx.Interaction, fmt.Sprintf("Echo: %s", message))
}

// ======================
// Exemplo 3: SubComando
// ======================

// UserInfoSubCommand demonstra implementação de subcomando
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
	return true // Requer servidor para acessar informações do membro
}

func (c *UserInfoSubCommand) RequiresPermissions() bool {
	return false
}

func (c *UserInfoSubCommand) Handle(ctx *Context) error {
	extractor := NewOptionExtractor(GetSubCommandOptions(ctx.Interaction))

	// Se não especificar usuário, usar o autor do comando
	var targetUser *discordgo.User
	if extractor.HasOption("user") {
		// Lógica para extrair usuário da opção
		targetUser = ctx.Interaction.Member.User
	} else {
		targetUser = ctx.Interaction.Member.User
	}

	// Criar embed com informações do usuário
	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("User Information").
		WithTimestamp()

	message := fmt.Sprintf("**Username:** %s\n**ID:** %s", targetUser.Username, targetUser.ID)

	return builder.Info(ctx.Interaction, message)
}

// ======================
// Exemplo 4: Comando de Grupo com Múltiplos SubComandos
// ======================

// ConfigGroupCommand demonstra como criar um comando com múltiplos subcomandos
type ConfigGroupCommand struct {
	*GroupCommand
}

func NewConfigGroupCommand(session *discordgo.Session, configManager *files.ConfigManager) *ConfigGroupCommand {
	responder := NewResponder(session)
	checker := NewPermissionChecker(session, configManager)

	group := NewGroupCommand("config", "Manage server configuration", responder, checker)

	// Adicionar subcomandos
	group.AddSubCommand(NewConfigSetSubCommand(configManager))
	group.AddSubCommand(NewConfigGetSubCommand(configManager))
	group.AddSubCommand(NewConfigListSubCommand(configManager))

	return &ConfigGroupCommand{GroupCommand: group}
}

// ConfigSetSubCommand - subcomando para definir configurações
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
	return true // Apenas usuários com permissão podem alterar config
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

	// Usar SafeGuildAccess para manipulação segura da config
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

	// Persistir configuração
	persister := NewConfigPersister(c.configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.WithField("error", err).Error("Failed to save config")
		return NewCommandError("Failed to save configuration", true)
	}

	responder := NewResponder(ctx.Session)
	return responder.Success(ctx.Interaction, fmt.Sprintf("Configuration `%s` set to `%s`", key, value))
}

// ConfigGetSubCommand - subcomando para obter configurações
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
		WithColor(0x0099FF)

	return builder.Info(ctx.Interaction, config.String())
}

// ConfigListSubCommand - subcomando para listar todas as configurações
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
// Exemplo 5: Autocomplete Handler
// ======================

// ConfigAutocompleteHandler demonstra implementação de autocomplete
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
		// Autocomplete baseado no key selecionado
		// Isso requereria lógica adicional para detectar o valor do key
		return h.getValueChoices(ctx)

	default:
		return []*discordgo.ApplicationCommandOptionChoice{}, nil
	}
}

func (h *ConfigAutocompleteHandler) getValueChoices(ctx *Context) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	// Exemplo: sugerir canais do servidor
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
// Exemplo 6: Como Registrar Tudo
// ======================

// ExampleCommandSetup demonstra como configurar todos os comandos
func ExampleCommandSetup(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	avatarCacheManager *cache.AvatarCacheManager,
) error {
	// Criar o gerenciador de comandos
	manager := NewCommandManager(session, configManager, avatarCacheManager)
	router := manager.GetRouter()

	// Registrar comandos simples
	router.RegisterCommand(NewPingCommand())
	router.RegisterCommand(NewEchoCommand())

	// Registrar comando de grupo
	configCmd := NewConfigGroupCommand(session, configManager)
	router.RegisterCommand(configCmd)

	// Registrar autocomplete
	router.RegisterAutocomplete("config", NewConfigAutocompleteHandler(configManager))

	// Sincronizar comandos com Discord
	return manager.SetupCommands()
}

// ======================
// Exemplo 7: Tratamento de Erros Avançado
// ======================

// AdvancedCommand demonstra tratamento de erros robusto
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
		return err // Erro de validação será tratado automaticamente
	}

	// Validações customizadas
	stringUtils := StringUtils{}
	if err := stringUtils.ValidateStringLength(input, 1, 100, "input"); err != nil {
		return err
	}

	// Operação que pode falhar
	result, err := c.processInput(input)
	if err != nil {
		// Log do erro
		ctx.Logger.WithFields(CreateLogFields(ctx, map[string]any{
			"input": input,
			"error": err,
		})).Error("Failed to process input")

		// Retornar erro amigável para o usuário
		return NewCommandError("Failed to process your input. Please try again.", true)
	}

	// Resposta de sucesso
	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Processing Complete").
		WithTimestamp()

	return builder.Success(ctx.Interaction, fmt.Sprintf("Result: %s", result))
}

func (c *AdvancedCommand) processInput(input string) (string, error) {
	// Simular processamento que pode falhar
	if strings.Contains(input, "error") {
		return "", fmt.Errorf("input contains forbidden word")
	}
	return strings.ToUpper(input), nil
}
