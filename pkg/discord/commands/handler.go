package commands

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
	"github.com/small-frappuccino/discordcore/pkg/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
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
	commandManager      *legacycore.CommandManager
	qotdService         qotdcmd.QuestionCatalogService
	statsService        *stats.StatsService
	moderationMetrics   moderation.Metrics
	ticketService       *tickets.TicketService
	embedService        *embeds.EmbedService
	rolePanelService    *roles.RolePanelService
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
		botInstanceID:     botInstanceID,
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

	// Info: Service architectural state transition log (initialization).
	slog.Info("Starting primary routine of CommandHandler",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	err := ch.SetupCommands()
	if err != nil {
		// Warn: Mitigated failure that does not compromise main data flow;
		// the service continues execution ignoring command synchronization.
		slog.Warn("command synchronization failed at initialization; operating in degraded state",
			slog.String("botInstanceID", ch.botInstanceID),
			slog.Any("err", err),
		)
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

	// Info: Planned instance shutdown log.
	slog.Info("Stopping main instances of CommandHandler",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	return ch.Shutdown()
}

// SetupCommands initializes and registers all bot commands
func (ch *CommandHandler) SetupCommands() error {
	slog.Info("Starting command and route coupling",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	// Re-init safety: avoid duplicated handlers if setup is called more than once.
	if ch.commandManager != nil {
		// Warn: Condition mitigated by compensatory repetition of local state cleanup.
		slog.Warn("overlapping handler registration; invoking cleanup of previous registrations",
			slog.String("botInstanceID", ch.botInstanceID),
		)
		if err := ch.Shutdown(); err != nil {
			return fmt.Errorf("cleanup previous command handlers: %w", err)
		}
	}

	// Create the command manager
	ch.commandManager = legacycore.NewCommandManager(ch.session, ch.configManager)
	ch.embedService = embeds.NewEmbedService(ch.configManager)
	ch.rolePanelService = roles.NewRolePanelService(ch.configManager)
	ch.partnerService = partners.NewPartnerService(ch.configManager)

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
				// Error: Blocking structural failure of current operation.
				slog.Error("fatal failure during command manager registration rollback",
					slog.Group("metadata",
						slog.String("botInstanceID", ch.botInstanceID),
						slog.String("synthetic_fault_code", "500"),
						slog.String("stack_trace", fmt.Sprintf("%+v", shutdownErr)),
					),
				)
			}
			ch.commandManager = nil
		}
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	slog.Info("Command architecture successfully established",
		slog.String("botInstanceID", ch.botInstanceID),
	)
	return nil
}

// GetCommandManager returns the command manager (for tests or extensions)
func (ch *CommandHandler) GetCommandManager() *legacycore.CommandManager {
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

	slog.Info("Command catalog fragments coupled to the router")
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
	if required.Stats && !ch.catalogCapabilities.Stats {
		return false
	}
	return true
}

// Shutdown performs cleanup for the command handler resources
func (ch *CommandHandler) Shutdown() error {
	slog.Info("Starting connection drain and shutdown of CommandHandler",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	var errs []error
	if ch.commandManager != nil {
		if err := ch.commandManager.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("shutdown command manager: %w", err))
		}
		ch.commandManager = nil
	}

	if len(errs) > 0 {
		// Error: Blocking structural failure draining dependencies. Triggers aggregation system.
		slog.Error("failures detected during command manager shutdown execution",
			slog.Group("metadata",
				slog.String("botInstanceID", ch.botInstanceID),
				slog.String("synthetic_fault_code", "500"),
				slog.String("stack_trace", fmt.Sprintf("%+v", errors.Join(errs...))),
			),
		)
	}

	return errors.Join(errs...)
}

// GetConfigManager returns the configuration manager
func (ch *CommandHandler) GetConfigManager() *files.ConfigManager {
	return ch.configManager
}

func (ch *CommandHandler) handlesGuild(guildID string) bool {
	return ch.handlesGuildRoute(guildID, legacycore.InteractionRouteKey{})
}

func (ch *CommandHandler) handlesGuildRoute(guildID string, routeKey legacycore.InteractionRouteKey) bool {
	// Debug: Granular tracking of the guild route filter logical flow.
	slog.Debug("evaluating route authorization for request",
		slog.String("guildID", guildID),
		slog.String("routeKeyPath", routeKey.Path),
	)

	feature := legacycore.ResolveFeatureForCommandPath(routeKey.Path)
	if !ch.matchesGuildBotInstance(guildID, feature) {
		slog.Debug("permission denied: mismatch between bot instance and mapped functionality",
			slog.String("feature", feature),
		)
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

	// Commands feature is universally available to all active bots in the guild.
	if feature == "commands" {
		for instanceID, tokenEnc := range guild.BotInstanceTokens {
			if string(tokenEnc) != "" && instanceID == ch.botInstanceID {
				return true
			}
		}
		return false
	}

	resolvedID, _ := guild.ResolveFeatureBotInstanceID(feature)
	tokenEnc, ok := guild.BotInstanceTokens[resolvedID]

	// Debug: Granular inspection of transient state and structural evaluation for context identification.
	slog.Debug("resolution of bot execution scope for specific guild",
		slog.String("resolvedID", resolvedID),
		slog.String("feature", feature),
		slog.Bool("tokenExists", ok),
	)

	if !ok || string(tokenEnc) == "" {
		return false
	}
	return resolvedID == ch.botInstanceID
}
