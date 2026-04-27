package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/metrics"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/partner"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/partners"
)

// CommandHandler manages bot command setup and handling
type CommandHandler struct {
	session              *discordgo.Session
	configManager        *files.ConfigManager
	botInstanceID        string
	defaultBotInstanceID string
	commandManager       *core.CommandManager
	partnerBoardService  partners.BoardService
	partnerSyncExecutor  partners.GuildSyncExecutor
}

// NewCommandHandler creates a new CommandHandler instance
func NewCommandHandler(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandHandler {
	return NewCommandHandlerForBot(session, configManager, "", "")
}

// NewCommandHandlerForBot creates a command handler scoped to a bot-instance guild assignment.
func NewCommandHandlerForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	botInstanceID string,
	defaultBotInstanceID string,
) *CommandHandler {
	return &CommandHandler{
		session:              session,
		configManager:        configManager,
		botInstanceID:        files.NormalizeBotInstanceID(botInstanceID),
		defaultBotInstanceID: files.NormalizeBotInstanceID(defaultBotInstanceID),
	}
}

// SetupCommands initializes and registers all bot commands
func (ch *CommandHandler) SetupCommands() error {
	log.ApplicationLogger().Info("Setting up bot commands...")

	// Re-init safety: avoid duplicated handlers if setup is called more than once.
	if ch.commandManager != nil {
		log.ApplicationLogger().Warn("Command setup called with active handlers; cleaning previous registrations first")
		if err := ch.Shutdown(); err != nil {
			return fmt.Errorf("cleanup previous command handlers: %w", err)
		}
	}

	// Create the command manager
	ch.commandManager = core.NewCommandManager(ch.session, ch.configManager)
	if router := ch.commandManager.GetRouter(); router != nil {
		router.SetGuildRouteFilter(ch.handlesGuildRoute)
	}

	// Register configuration commands
	if err := ch.registerConfigCommands(); err != nil {
		return fmt.Errorf("failed to register config commands: %w", err)
	}

	// Configure commands on Discord
	if err := ch.commandManager.SetupCommands(); err != nil {
		if ch.commandManager != nil {
			if shutdownErr := ch.commandManager.Shutdown(); shutdownErr != nil {
				log.ErrorLoggerRaw().Error("Failed to rollback command manager handler registration", "err", shutdownErr)
			}
			ch.commandManager = nil
		}
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	log.ApplicationLogger().Info("Bot commands setup completed successfully")
	return nil
}

// GetCommandManager returns the command manager (for tests or extensions)
func (ch *CommandHandler) GetCommandManager() *core.CommandManager {
	return ch.commandManager
}

// SetPartnerBoardService injects partner board application service for /partner commands.
func (ch *CommandHandler) SetPartnerBoardService(service partners.BoardService) {
	ch.partnerBoardService = service
}

// SetPartnerBoardSyncExecutor injects a sync executor used by /partner sync.
func (ch *CommandHandler) SetPartnerBoardSyncExecutor(executor partners.GuildSyncExecutor) {
	ch.partnerSyncExecutor = executor
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
	// Register partner CRUD commands
	if ch.partnerBoardService != nil || ch.partnerSyncExecutor != nil {
		boardService := ch.partnerBoardService
		if boardService == nil {
			boardService = partners.NewBoardApplicationService(ch.configManager, nil)
		}
		partner.NewPartnerCommandsWithServices(boardService, ch.partnerSyncExecutor).RegisterCommands(router)
	} else {
		partner.NewPartnerCommands(ch.configManager).RegisterCommands(router)
	}
	// Register moderation commands
	moderation.RegisterModerationCommands(router)

	log.ApplicationLogger().Info("Config, partner, metrics, and moderation commands registered successfully")
	return nil
}

// Shutdown performs cleanup for the command handler resources
func (ch *CommandHandler) Shutdown() error {
	log.ApplicationLogger().Info("Shutting down command handler...")

	var errs []error
	if ch.commandManager != nil {
		if err := ch.commandManager.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("shutdown command manager: %w", err))
		}
		ch.commandManager = nil
	}

	return errors.Join(errs...)
}

// GetConfigManager returns the configuration manager
func (ch *CommandHandler) GetConfigManager() *files.ConfigManager {
	return ch.configManager
}

func (ch *CommandHandler) handlesGuild(guildID string) bool {
	return ch.handlesGuildRoute(guildID, core.InteractionRouteKey{})
}

func (ch *CommandHandler) handlesGuildRoute(guildID string, routeKey core.InteractionRouteKey) bool {
	if !ch.matchesGuildBotInstance(guildID) {
		return false
	}
	cfg := ch.configManager.Config()
	if cfg == nil {
		return false
	}
	if cfg.ResolveFeatures(strings.TrimSpace(guildID)).Services.Commands {
		return true
	}
	return config.AllowsDormantGuildBootstrapRoute(routeKey)
}

func (ch *CommandHandler) matchesGuildBotInstance(guildID string) bool {
	if ch == nil {
		return false
	}
	if ch.botInstanceID == "" && ch.defaultBotInstanceID == "" {
		return true
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || ch.configManager == nil {
		return false
	}
	guild := ch.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}
	if guild.EffectiveBotInstanceID(ch.defaultBotInstanceID) != files.NormalizeBotInstanceID(ch.botInstanceID) {
		return false
	}
	return true
}
