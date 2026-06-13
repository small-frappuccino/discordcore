package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	discordembeds "github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	discordpartners "github.com/small-frappuccino/discordcore/pkg/discord/partners"
	discordroles "github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
	"github.com/small-frappuccino/discordcore/pkg/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordcore/pkg/roles"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

// CommandHandler manages bot command setup and handling
type CommandHandler struct {
	session             *discordgo.Session
	configManager       *files.ConfigManager
	botInstanceID       string
	catalogCapabilities CommandCatalogCapabilities
	catalogRegistrars   []CommandCatalogRegistrar
	commandManager      *core.CommandManager
	qotdService         qotdcmd.QuestionCatalogService
	statsService        *stats.StatsService
	moderationMetrics   moderation.Metrics
	adminServiceManager *service.ServiceManager
	ticketService       *tickets.TicketService
	embedService        *embeds.EmbedService
	rolePanelPublisher  roles.PanelPublisher
	partnerService      *partners.PartnerService

	mu           sync.RWMutex
	running      bool
	startTime    time.Time
	dependencies []string
}

// NewCommandHandler creates a new CommandHandler instance
func NewCommandHandler(
	session *discordgo.Session,
	configManager *files.ConfigManager,
) *CommandHandler {
	return NewCommandHandlerForBot(session, configManager, "")
}

// NewCommandHandlerForBot creates a command handler scoped to a bot-instance guild assignment.
func NewCommandHandlerForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	botInstanceID string,
) *CommandHandler {
	return &CommandHandler{
		session:           session,
		configManager:     configManager,
		botInstanceID:     files.NormalizeBotInstanceID(botInstanceID),
		catalogRegistrars: DefaultCommandCatalogRegistrars(),
	}
}

// SetDependencies allows the orchestrator to inject dynamic dependencies.
func (ch *CommandHandler) SetDependencies(deps []string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	ch.dependencies = append([]string(nil), deps...)
}

// Name returns the service name.
func (ch *CommandHandler) Name() string { return "command-handler" }

// Type returns the service type.
func (ch *CommandHandler) Type() service.ServiceType { return service.TypeCommands }

// Priority returns the service startup priority.
func (ch *CommandHandler) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns the dependencies.
func (ch *CommandHandler) Dependencies() []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return append([]string(nil), ch.dependencies...)
}

// IsRunning reports whether the service is running.
func (ch *CommandHandler) IsRunning() bool {
	ch.mu.RLock()
	defer ch.mu.RUnlock()
	return ch.running
}

// HealthCheck returns the current health status.
func (ch *CommandHandler) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   true,
		Message:   "Command Handler is active",
		LastCheck: time.Now(),
	}
}

// Stats returns runtime statistics.
func (ch *CommandHandler) Stats() service.ServiceStats {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	var uptime time.Duration
	if ch.running {
		uptime = time.Since(ch.startTime)
	}

	return service.ServiceStats{
		StartTime: ch.startTime,
		Uptime:    uptime,
		Metrics: []service.ServiceMetric{
			{Label: "Status", Value: "Running"},
		},
	}
}

// Start implements the service.Service interface.
func (ch *CommandHandler) Start(ctx context.Context) error {
	ch.mu.Lock()
	if ch.running {
		ch.mu.Unlock()
		return nil
	}
	ch.running = true
	ch.startTime = time.Now()
	ch.mu.Unlock()

	err := ch.SetupCommands()
	if err != nil {
		log.ApplicationLogger().Warn("Failed to sync commands during startup; continuing without updated commands", "err", err)
	}
	return nil
}

// Stop implements the service.Service interface.
func (ch *CommandHandler) Stop(ctx context.Context) error {
	ch.mu.Lock()
	if !ch.running {
		ch.mu.Unlock()
		return nil
	}
	ch.running = false
	ch.mu.Unlock()

	return ch.Shutdown()
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
	ch.embedService = embeds.NewEmbedService(ch.configManager, &discordembeds.Adapter{Session: ch.session})
	ch.rolePanelPublisher = discordroles.NewPublisher(ch.session, ch.configManager)
	ch.partnerService = partners.NewPartnerService(ch.configManager, discordpartners.NewDiscordgoBoardPublisher(ch.session))

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

// SetStatsService injects the StatsService for immediate channel updates from the /stats command tree.
func (ch *CommandHandler) SetStatsService(service *stats.StatsService) {
	ch.statsService = service
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

// SetTicketService injects the ticket management service.
func (ch *CommandHandler) SetTicketService(service *tickets.TicketService) {
	if ch == nil {
		return
	}
	ch.ticketService = service
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
	if required.Stats && !ch.catalogCapabilities.Stats {
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
	feature := core.ResolveFeatureForCommandPath(routeKey.Path)
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
	guildID = strings.TrimSpace(guildID)
	if guildID == "" || ch.configManager == nil {
		return false
	}
	guild := ch.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}

	// Commands feature is now universally available to all active bots in the guild.
	if feature == "commands" {
		return guild.BelongsToBotInstance(ch.botInstanceID)
	}

	resolvedID, _ := guild.ResolveFeatureBotInstanceID(feature, "")
	return ch.botInstanceID == resolvedID
}
