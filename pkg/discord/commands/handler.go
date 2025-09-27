package commands

import (
	"fmt"

	"github.com/alice-bnuy/discordcore/pkg/discord/commands/config"
	"github.com/alice-bnuy/discordcore/pkg/discord/commands/core"
	"github.com/alice-bnuy/discordcore/pkg/files"
	"github.com/alice-bnuy/discordcore/pkg/log"
	"github.com/bwmarrin/discordgo"
)

// CommandHandler é o handler principal que coordena todos os comandos do bot
type CommandHandler struct {
	session        *discordgo.Session
	configManager  *files.ConfigManager
	commandManager *core.CommandManager
}

// NewCommandHandler cria uma nova instância do command handler
func NewCommandHandler(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandHandler {
	return &CommandHandler{
		session:       session,
		configManager: configManager,
	}
}

// SetupCommands inicializa e registra todos os comandos do bot
func (ch *CommandHandler) SetupCommands() error {
	log.Info(log.Application, "Setting up bot commands...")

	// Criar o gerenciador de comandos
	ch.commandManager = core.NewCommandManager(ch.session, ch.configManager)

	// Registrar comandos de configuração
	if err := ch.registerConfigCommands(); err != nil {
		return fmt.Errorf("failed to register config commands: %w", err)
	}

	// Configurar os comandos no Discord
	if err := ch.commandManager.SetupCommands(); err != nil {
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	log.Info(log.Application, "Bot commands setup completed successfully")
	return nil
}

// registerConfigCommands registra os comandos de configuração
func (ch *CommandHandler) registerConfigCommands() error {
	router := ch.commandManager.GetRouter()

	// Registrar o grupo /config e comandos simples (ping/echo)
	config.NewConfigCommands(ch.configManager).RegisterCommands(router)

	log.Info(log.Application, "Config commands registered successfully")
	return nil
}

// Shutdown realiza limpeza dos recursos do command handler
func (ch *CommandHandler) Shutdown() error {
	log.Info(log.Application, "Shutting down command handler...")

	// Aqui você pode adicionar lógica de limpeza se necessário
	// Por exemplo, salvar configurações, limpar caches, etc.

	return nil
}

// GetCommandManager retorna o gerenciador de comandos (para uso em testes ou extensões)
func (ch *CommandHandler) GetCommandManager() *core.CommandManager {
	return ch.commandManager
}

// GetConfigManager retorna o gerenciador de configurações
func (ch *CommandHandler) GetConfigManager() *files.ConfigManager {
	return ch.configManager
}
