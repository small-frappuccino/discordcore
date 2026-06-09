package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

// CommandHandler manages bot command setup and handling
type CommandHandler struct {
	session              *discordgo.Session
	configManager        *files.ConfigManager
	botInstanceID        string
	defaultBotInstanceID string
	catalogCapabilities  CommandCatalogCapabilities
	catalogRegistrars    []CommandCatalogRegistrar
	commandManager       *core.CommandManager
	qotdService          qotdcmd.QuestionCatalogService
	moderationMetrics    moderation.Metrics
	adminServiceManager  *service.ServiceManager
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
		catalogRegistrars:    DefaultCommandCatalogRegistrars(),
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

	// Register configuration and feature command catalogs.
	if err := ch.registerCommandCatalog(); err != nil {
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

// SetQOTDService injects the QOTD application service for interactive QOTD commands.
func (ch *CommandHandler) SetQOTDService(service qotdcmd.QuestionCatalogService) {
	ch.qotdService = service
}

// SetModerationMetrics injects the moderation observability sink so the
// /clean command records attempts, outcomes, and per-message delete failures.
// Nil falls back to NopMetrics inside the moderation registrar.
func (ch *CommandHandler) SetModerationMetrics(metrics moderation.Metrics) {
	if ch == nil {
		return
	}
	ch.moderationMetrics = metrics
}

// SetAdminCommandServices injects runtime services consumed by the admin
// command catalog. Cache and persistent-store observability moved to
// /v1/health/cache, so this seam now carries only the service manager used by
// /admin status/list/restart/health.
func (ch *CommandHandler) SetAdminCommandServices(serviceManager *service.ServiceManager) {
	if ch == nil {
		return
	}
	ch.adminServiceManager = serviceManager
}

// SetCommandCatalogRegistrars overrides the slash command catalogs registered by
// this handler.
func (ch *CommandHandler) SetCommandCatalogRegistrars(registrars ...CommandCatalogRegistrar) {
	if ch == nil {
		return
	}
	ch.catalogRegistrars = append([]CommandCatalogRegistrar(nil), registrars...)
}

// SetCommandCatalogCapabilities sets runtime capabilities used to filter
// capability-gated command registrars.
func (ch *CommandHandler) SetCommandCatalogCapabilities(capabilities CommandCatalogCapabilities) {
	if ch == nil {
		return
	}
	ch.catalogCapabilities = capabilities
}

func (ch *CommandHandler) registerCommandCatalog() error {
	router := ch.commandManager.GetRouter()
	for _, registrar := range ch.commandCatalogRegistrarsForSetup() {
		if registrar.Register == nil {
			continue
		}
		registrar.Register(ch, router)
	}

	log.ApplicationLogger().Info("Command catalog fragments registered successfully")
	return nil
}

func (ch *CommandHandler) commandCatalogRegistrarsForSetup() []CommandCatalogRegistrar {
	filtered := make([]CommandCatalogRegistrar, 0, len(ch.catalogRegistrars))
	for _, registrar := range ch.catalogRegistrars {
		if ch.supportsCatalogCapabilities(registrar.RequiredCapabilities) {
			filtered = append(filtered, registrar)
		}
	}
	return filtered
}

func (ch *CommandHandler) supportsCatalogCapabilities(required CommandCatalogCapabilities) bool {
	if required.Admin && !ch.catalogCapabilities.Admin {
		return false
	}
	return true
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
	feature := "commands"
	if strings.HasPrefix(routeKey.Path, "qotd") {
		feature = "qotd"
	}
	if !ch.matchesGuildBotInstance(guildID, feature) {
		return false
	}
	cfg := ch.configManager.Config()
	if cfg == nil {
		return false
	}
	if cfg.ResolveFeatures(strings.TrimSpace(guildID)).Services.Commands {
		return true
	}
	return false
}

func (ch *CommandHandler) matchesGuildBotInstance(guildID string, feature string) bool {
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

	botInstanceID := ch.botInstanceID
	if botInstanceID == "" {
		botInstanceID = ch.defaultBotInstanceID
	}
	if botInstanceID == "" {
		return true
	}
	if len(guild.BotInstanceTokens) == 0 {
		return botInstanceID == ch.defaultBotInstanceID
	}

	resolvedID, fallback := guild.ResolveFeatureBotInstanceID(feature, ch.defaultBotInstanceID)
	if fallback && resolvedID == ch.defaultBotInstanceID {
		log.ApplicationLogger().Warn(
			"Command routing degraded to default bot instance due to missing or invalid token for designated route",
			"guildID", guildID,
			"feature", feature,
			"fallbackBotInstanceID", resolvedID,
		)
	}

	return botInstanceID == resolvedID
}
