package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// CommandHandler manages bot command setup and handling
type CommandHandler struct {
	session              *discordgo.Session
	configManager        *files.ConfigManager
	botInstanceID        string
	defaultBotInstanceID string
	supportedDomains     map[string]struct{}
	catalogCapabilities  CommandCatalogCapabilities
	catalogRegistrars    []CommandCatalogRegistrar
	commandManager       *core.CommandManager
	partnerBoardService  partners.BoardService
	partnerSyncExecutor  partners.GuildSyncExecutor
	qotdService          qotdcmd.QuestionCatalogService
	adminServiceManager  *service.ServiceManager
	adminUnifiedCache    *cache.UnifiedCache
	adminStore           *storage.Store
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

// SetPartnerBoardService injects partner board application service for /partner commands.
func (ch *CommandHandler) SetPartnerBoardService(service partners.BoardService) {
	ch.partnerBoardService = service
}

// SetPartnerBoardSyncExecutor injects a sync executor used by /partner sync.
func (ch *CommandHandler) SetPartnerBoardSyncExecutor(executor partners.GuildSyncExecutor) {
	ch.partnerSyncExecutor = executor
}

// SetQOTDService injects the QOTD application service for interactive QOTD commands.
func (ch *CommandHandler) SetQOTDService(service qotdcmd.QuestionCatalogService) {
	ch.qotdService = service
}

// SetAdminCommandServices injects runtime services consumed by the admin
// command catalog.
func (ch *CommandHandler) SetAdminCommandServices(serviceManager *service.ServiceManager, unifiedCache *cache.UnifiedCache, store *storage.Store) {
	if ch == nil {
		return
	}
	ch.adminServiceManager = serviceManager
	ch.adminUnifiedCache = unifiedCache
	ch.adminStore = store
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

// SetSupportedDomains limits catalog registration to the provided domains. If
// not called, the handler preserves the legacy behavior and registers every
// known fragment.
func (ch *CommandHandler) SetSupportedDomains(domains ...string) {
	if ch == nil {
		return
	}
	if len(domains) == 0 {
		ch.supportedDomains = nil
		return
	}
	ch.supportedDomains = make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		ch.supportedDomains[files.NormalizeBotDomain(domain)] = struct{}{}
	}
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
	if ch.supportedDomains == nil {
		filtered := make([]CommandCatalogRegistrar, 0, len(ch.catalogRegistrars))
		for _, registrar := range ch.catalogRegistrars {
			if ch.supportsCatalogCapabilities(registrar.RequiredCapabilities) {
				filtered = append(filtered, registrar)
			}
		}
		return filtered
	}

	filtered := make([]CommandCatalogRegistrar, 0, len(ch.catalogRegistrars))
	for _, registrar := range ch.catalogRegistrars {
		if ch.supportsDomain(registrar.Domain) && ch.supportsCatalogCapabilities(registrar.RequiredCapabilities) {
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

func (ch *CommandHandler) supportsDomain(domain string) bool {
	if ch == nil || ch.supportedDomains == nil {
		return true
	}
	_, ok := ch.supportedDomains[files.NormalizeBotDomain(domain)]
	return ok
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
	domain := ch.routeDomain(routeKey)
	if !ch.supportsDomain(domain) {
		return false
	}
	if !ch.matchesGuildBotInstanceForDomain(guildID, domain) {
		return false
	}
	cfg := ch.configManager.Config()
	if cfg == nil {
		return false
	}
	if cfg.ResolveFeatures(strings.TrimSpace(guildID)).Services.Commands {
		return true
	}
	return config.AllowsDormantGuildBootstrapRouteForDomain(domain, routeKey)
}

func (ch *CommandHandler) matchesGuildBotInstance(guildID string) bool {
	return ch.matchesGuildBotInstanceForDomain(guildID, "")
}

func (ch *CommandHandler) matchesGuildBotInstanceForDomain(guildID, domain string) bool {
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
	if guild.EffectiveBotInstanceIDForDomain(domain, ch.defaultBotInstanceID) != files.NormalizeBotInstanceID(ch.botInstanceID) {
		return false
	}
	return true
}

func (ch *CommandHandler) routeDomain(routeKey core.InteractionRouteKey) string {
	if ch != nil && ch.commandManager != nil {
		if router := ch.commandManager.GetRouter(); router != nil {
			if domain := router.InteractionRouteDomain(routeKey); domain != "" {
				return domain
			}
		}
	}

	if domain, ok := config.DormantGuildBootstrapRouteDomain(routeKey); ok {
		return domain
	}
	return ""
}
