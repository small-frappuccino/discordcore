package core

import (
	"github.com/alice-bnuy/discordcore/pkg/files"
	logutil "github.com/alice-bnuy/discordcore/pkg/logging"
	"github.com/bwmarrin/discordgo"
)

// ContextBuilder cria contextos para execução de comandos
type ContextBuilder struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	checker       *PermissionChecker
}

// NewContextBuilder cria um novo construtor de contexto
func NewContextBuilder(session *discordgo.Session, configManager *files.ConfigManager, checker *PermissionChecker) *ContextBuilder {
	return &ContextBuilder{
		session:       session,
		configManager: configManager,
		checker:       checker,
	}
}

// BuildContext cria um contexto completo para execução de comando
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

	logger := logutil.WithFields(map[string]any{
		"command": i.ApplicationCommandData().Name,
		"userID":  userID,
		"guildID": guildID,
	})

	return &Context{
		Session:     cb.session,
		Interaction: i,
		Config:      cb.configManager,
		Logger:      logger,
		GuildID:     guildID,
		UserID:      userID,
		IsOwner:     isOwner,
		GuildConfig: guildConfig,
	}
}

// isGuildOwner verifica se o usuário é o dono do servidor
func (cb *ContextBuilder) isGuildOwner(guildID, userID string) bool {
	guild, err := cb.session.Guild(guildID)
	if err != nil {
		return false
	}
	return guild.OwnerID == userID
}

// extractUserID extrai o ID do usuário da interação
func extractUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	} else if i.User != nil {
		return i.User.ID
	}
	return ""
}

// GetSubCommandName extrai o nome do subcomando da interação
func GetSubCommandName(i *discordgo.InteractionCreate) string {
	options := i.ApplicationCommandData().Options
	if len(options) > 0 && options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return options[0].Name
	}
	return ""
}

// GetSubCommandOptions extrai as opções do subcomando da interação
func GetSubCommandOptions(i *discordgo.InteractionCreate) []*discordgo.ApplicationCommandInteractionDataOption {
	options := i.ApplicationCommandData().Options
	if len(options) > 0 && options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return options[0].Options
	}
	return options // Retorna as opções diretas se não for subcomando
}

// CommandLogEntry cria uma entrada de log padronizada para comandos
func CommandLogEntry(i *discordgo.InteractionCreate, command string, userID string) *logutil.Logger {
	return logutil.WithFields(map[string]any{
		"command": command,
		"guildID": i.GuildID,
		"userID":  userID,
	})
}

// ValidateGuildContext valida se o contexto tem as informações necessárias do servidor
func ValidateGuildContext(ctx *Context) error {
	if ctx.GuildID == "" {
		return NewCommandError("This command can only be used in a server", true)
	}

	if ctx.GuildConfig == nil {
		return NewCommandError("Server configuration not found", true)
	}

	return nil
}

// ValidateUserContext valida se o contexto tem as informações necessárias do usuário
func ValidateUserContext(ctx *Context) error {
	if ctx.UserID == "" {
		return NewCommandError("Unable to identify user", true)
	}

	return nil
}

// HasFocusedOption verifica se há uma opção com foco (para autocomplete)
func HasFocusedOption(options []*discordgo.ApplicationCommandInteractionDataOption) (*discordgo.ApplicationCommandInteractionDataOption, bool) {
	for _, opt := range options {
		if opt.Focused {
			return opt, true
		}
		// Verifica recursivamente em subcomandos
		if opt.Type == discordgo.ApplicationCommandOptionSubCommand && len(opt.Options) > 0 {
			if focused, found := HasFocusedOption(opt.Options); found {
				return focused, true
			}
		}
	}
	return nil, false
}

// GetCommandPath retorna o caminho completo do comando (comando + subcomando se houver)
func GetCommandPath(i *discordgo.InteractionCreate) string {
	path := i.ApplicationCommandData().Name

	subCmd := GetSubCommandName(i)
	if subCmd != "" {
		path += " " + subCmd
	}

	return path
}

// IsAutocompleteInteraction verifica se a interação é de autocomplete
func IsAutocompleteInteraction(i *discordgo.InteractionCreate) bool {
	return i.Type == discordgo.InteractionApplicationCommandAutocomplete
}

// IsSlashCommandInteraction verifica se a interação é de comando slash
func IsSlashCommandInteraction(i *discordgo.InteractionCreate) bool {
	return i.Type == discordgo.InteractionApplicationCommand
}

// CreateLogFields cria campos de log padronizados
func CreateLogFields(ctx *Context, additionalFields map[string]any) map[string]any {
	fields := map[string]any{
		"command": GetCommandPath(ctx.Interaction),
		"guildID": ctx.GuildID,
		"userID":  ctx.UserID,
	}

	// Adiciona campos adicionais
	for k, v := range additionalFields {
		fields[k] = v
	}

	return fields
}

// RequiresGuildConfig verifica se o comando requer configuração de servidor
func RequiresGuildConfig(ctx *Context) error {
	if err := ValidateGuildContext(ctx); err != nil {
		return err
	}

	if ctx.GuildConfig == nil {
		return NewCommandError("Server configuration is required for this command", true)
	}

	return nil
}

// SafeGuildAccess fornece acesso seguro às informações do servidor
func SafeGuildAccess(ctx *Context, fn func(*files.GuildConfig) error) error {
	if err := RequiresGuildConfig(ctx); err != nil {
		return err
	}

	return fn(ctx.GuildConfig)
}
