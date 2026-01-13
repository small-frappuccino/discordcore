package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/metrics"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
)

// CommandHandler manages bot command setup and handling
type CommandHandler struct {
	session        *discordgo.Session
	configManager  *files.ConfigManager
	commandManager *core.CommandManager
}

// NewCommandHandler creates a new CommandHandler instance
func NewCommandHandler(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandHandler {
	return &CommandHandler{
		session:       session,
		configManager: configManager,
	}
}

// SetupCommands initializes and registers all bot commands
func (ch *CommandHandler) SetupCommands() error {
	log.ApplicationLogger().Info("Setting up bot commands...")

	// Create the command manager
	ch.commandManager = core.NewCommandManager(ch.session, ch.configManager)

	// Register configuration commands
	if err := ch.registerConfigCommands(); err != nil {
		return fmt.Errorf("failed to register config commands: %w", err)
	}

	// Register runtime-config interactive handlers (components + modals)
	// This is required because core.CommandRouter currently routes only slash commands.
	//
	// The runtime config panel needs the hot-apply manager to apply THEME + ALICE_DISABLE_* at runtime.
	// Since these interactions do not go through the slash CommandRouter, we pass the shared applier
	// via closure here.
	var applier *runtimeapply.Manager
	if ch.commandManager != nil {
		if router := ch.commandManager.GetRouter(); router != nil {
			applier = router.GetRuntimeApplier()
		}
	}
	ch.session.AddHandler(runtime.HandleRuntimeConfigInteractions(ch.configManager, applier))

	// Configure commands on Discord
	if err := ch.commandManager.SetupCommands(); err != nil {
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	log.ApplicationLogger().Info("Bot commands setup completed successfully")
	return nil
}

// GetCommandManager returns the command manager (for tests or extensions)
func (ch *CommandHandler) GetCommandManager() *core.CommandManager {
	return ch.commandManager
}

// registerConfigCommands registers configuration-related commands
func (ch *CommandHandler) registerConfigCommands() error {
	router := ch.commandManager.GetRouter()

	// Register the /config group and simple commands (ping/echo)
	config.NewConfigCommands(ch.configManager).RegisterCommands(router)

	// Register the /config runtime panel (replaces env-var operational toggles)
	runtime.NewRuntimeConfigCommands(ch.configManager).RegisterCommands(router)

	// Register metrics commands (activity, members)
	metrics.RegisterMetricsCommands(router)
	// Register moderation commands
	moderation.RegisterModerationCommands(router)

	log.ApplicationLogger().Info("Config, metrics, and moderation commands registered successfully")
	return nil
}

// Shutdown performs cleanup for the command handler resources
func (ch *CommandHandler) Shutdown() error {
	log.ApplicationLogger().Info("Shutting down command handler...")

	// You can add cleanup logic here if needed
	// For example, save settings, clear caches, etc.

	return nil
}

// GetConfigManager returns the configuration manager
func (ch *CommandHandler) GetConfigManager() *files.ConfigManager {
	return ch.configManager
}
