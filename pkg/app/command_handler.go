package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

// CommandHandler manages bot command setup and handling.
type CommandHandler struct {
	session             *discordgo.Session
	configManager       *files.ConfigManager
	botInstanceID       string
	catalogCapabilities CommandCatalogCapabilities
	catalogRegistrars   []CommandCatalogRegistrar

	// Atomic pointers enforce memory safety without mutex contention
	router atomic.Pointer[commands.CommandRouter]
	syncer atomic.Pointer[commands.CommandSyncer]

	interactionCancel func()
	qotdService       qotdcmd.Service
	statsService      *stats.StatsService
	moderationMetrics moderation.Metrics
	ticketService     *tickets.Service
	embedService      *embeds.EmbedService
	rolePanelService  *roles.RolePanelService
	partnerService    *partners.PartnerService
	runtimeApplier    *runtimeapply.Manager

	mu           sync.RWMutex
	running      bool
	startTime    time.Time
	dependencies []string
}

// CommandHandlerDeps encapsulates all required invariants for the CommandHandler.
type CommandHandlerDeps struct {
	Session             *discordgo.Session
	ConfigManager       *files.ConfigManager
	BotInstanceID       string
	CatalogCapabilities CommandCatalogCapabilities
	CatalogRegistrars   []CommandCatalogRegistrar
	QotdService         qotdcmd.Service
	StatsService        *stats.StatsService
	ModerationMetrics   moderation.Metrics
	TicketService       *tickets.Service
	RuntimeApplier      *runtimeapply.Manager
	EmbedService        *embeds.EmbedService
	RolePanelService    *roles.RolePanelService
	PartnerService      *partners.PartnerService
}

// NewCommandHandler creates a new CommandHandler instance
func NewCommandHandler(deps CommandHandlerDeps) (*CommandHandler, error) {
	deps.BotInstanceID = ""
	return NewCommandHandlerForBot(deps)
}

// NewCommandHandlerForBot creates a command handler scoped to a bot-instance guild assignment.
// It forces fail-fast initialization: missing invariants halt bootstrapping.
func NewCommandHandlerForBot(deps CommandHandlerDeps) (*CommandHandler, error) {
	if deps.Session == nil {
		return nil, errors.New("initialization failure: Session is strictly required")
	}
	if deps.ConfigManager == nil {
		return nil, errors.New("initialization failure: ConfigManager is strictly required")
	}

	registrars := deps.CatalogRegistrars
	if len(registrars) == 0 {
		registrars = DefaultCommandCatalogRegistrars()
	}

	return &CommandHandler{
		session:             deps.Session,
		configManager:       deps.ConfigManager,
		botInstanceID:       deps.BotInstanceID,
		catalogCapabilities: deps.CatalogCapabilities,
		catalogRegistrars:   registrars,
		qotdService:         deps.QotdService,
		statsService:        deps.StatsService,
		moderationMetrics:   deps.ModerationMetrics,
		ticketService:       deps.TicketService,
		embedService:        deps.EmbedService,
		rolePanelService:    deps.RolePanelService,
		partnerService:      deps.PartnerService,
		runtimeApplier:      deps.RuntimeApplier,
	}, nil
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

// SetupCommands initializes and registers all bot commands.
func (ch *CommandHandler) SetupCommands() error {
	slog.Info("Starting command and route coupling", slog.String("botInstanceID", ch.botInstanceID))

	if ch.router.Load() != nil {
		slog.Warn("overlapping handler registration; invoking cleanup of previous registrations", slog.String("botInstanceID", ch.botInstanceID))
		if err := ch.Shutdown(); err != nil {
			return fmt.Errorf("cleanup previous command handlers: %w", err)
		}
	}

	apiClient := api.NewClient(ch.session.Token)
	newRouter := commands.NewCommandRouter(apiClient, ch.configManager)

	if ch.session == nil || ch.session.State == nil || ch.session.State.User == nil {
		return errors.New("cannot setup commands: session user state is missing")
	}
	appIDInt := ch.session.State.User.ID
	if appIDInt == "" {
		return errors.New("cannot setup commands: bot User ID is empty")
	}
	appID, err := discord.ParseSnowflake(appIDInt)
	if err != nil {
		return fmt.Errorf("invalid bot app ID: %w", err)
	}
	newSyncer := commands.NewCommandSyncer(apiClient, discord.AppID(appID))

	if err := ch.registerCommandCatalog(newRouter); err != nil {
		return fmt.Errorf("failed to register config commands: %w", err)
	}

	ch.router.Store(newRouter)
	ch.syncer.Store(newSyncer)

	// Direct method injection strictly avoids inline closure allocation overhead.
	ch.interactionCancel = ch.session.AddHandler(ch.handleInteractionCreate)

	currentRouter := ch.router.Load()
	currentSyncer := ch.syncer.Load()
	if err := currentSyncer.SyncBulkOverwrite(0, currentRouter.Registry()); err != nil {
		if shutdownErr := ch.Shutdown(); shutdownErr != nil {
			slog.Error("fatal failure during command manager registration rollback",
				slog.String("botInstanceID", ch.botInstanceID),
				slog.String("synthetic_fault_code", "500"),
				slog.String("stack_trace", fmt.Sprintf("%+v", shutdownErr)),
			)
		}
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	slog.Info("Command architecture successfully established natively", slog.String("botInstanceID", ch.botInstanceID))
	return nil
}

// handleInteractionCreate executes isolated runtime processing.
func (ch *CommandHandler) handleInteractionCreate(s *discordgo.Session, rawEvent *discordgo.Event) {
	if rawEvent.Type != "INTERACTION_CREATE" {
		return
	}

	currentRouter := ch.router.Load()
	if currentRouter == nil {
		return
	}

	var arikawaEvent discord.InteractionEvent
	if err := arikawaEvent.UnmarshalJSON(rawEvent.RawData); err != nil {
		slog.Error("Failed to unmarshal INTERACTION_CREATE into Arikawa event", slog.Any("error", err))
		return
	}

	var routePath string
	switch data := arikawaEvent.Data.(type) {
	case *discord.CommandInteraction:
		routePath = data.Name
	case *discord.AutocompleteInteraction:
		routePath = data.Name
	case discord.ComponentInteraction:
		routePath = string(data.ID())
	case *discord.ModalInteraction:
		routePath = string(data.CustomID)
	}

	if routePath != "" && arikawaEvent.GuildID.IsValid() {
		if !ch.handlesGuildRoute(arikawaEvent.GuildID.String(), commands.InteractionRouteKey{Path: routePath}) {
			return
		}
	}

	_ = currentRouter.HandleEvent(&arikawaEvent)
}

// GetRouter returns the command router (for tests or extensions).
func (ch *CommandHandler) GetRouter() *commands.CommandRouter {
	return ch.router.Load()
}

func (ch *CommandHandler) registerCommandCatalog(router *commands.CommandRouter) error {
	for _, registrar := range ch.commandCatalogRegistrarsForSetup() {
		if registrar.RegisterArikawa != nil {
			registrar.RegisterArikawa(ch, router)
		}
	}

	slog.Info("Command catalog fragments coupled to the native Arikawa router")
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
	return ch.catalogCapabilities.Has(required)
}

// Shutdown performs cleanup for the command handler resources.
func (ch *CommandHandler) Shutdown() error {
	slog.Info("Starting connection drain and shutdown of CommandHandler",
		slog.String("botInstanceID", ch.botInstanceID),
	)

	if ch.interactionCancel != nil {
		ch.interactionCancel()
		ch.interactionCancel = nil
	}

	ch.router.Store(nil)
	ch.syncer.Store(nil)

	return nil
}

// GetConfigManager returns the configuration manager.
func (ch *CommandHandler) GetConfigManager() *files.ConfigManager {
	return ch.configManager
}

func (ch *CommandHandler) handlesGuild(guildID string) bool {
	return ch.handlesGuildRoute(guildID, commands.InteractionRouteKey{})
}

func (ch *CommandHandler) handlesGuildRoute(guildID string, routeKey commands.InteractionRouteKey) bool {
	slog.Debug("evaluating route authorization for request",
		slog.String("guildID", guildID),
		slog.String("routeKeyPath", routeKey.Path),
	)

	feature := commands.ResolveFeatureForCommandPath(routeKey.Path)
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

	// Immediate return of the structural boolean flag avoids intermediate states.
	return cfg.ResolveFeatures(strings.TrimSpace(guildID)).Services.Commands
}

// matchesGuildBotInstance enforces strict binary command authorization.
// If ResolveFeatureBotInstanceID yields an unmapped or deactivated target,
// it instantly returns false. It strictly rejects unpredictable generic routing
// and refuses to authorize a command simply because a generic bot happens to be online.
func (ch *CommandHandler) matchesGuildBotInstance(guildID string, feature string) bool {
	if ch == nil || ch.configManager == nil {
		return false
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return false
	}

	guild := ch.configManager.GuildConfig(guildID)
	if guild == nil {
		return false
	}

	resolvedID, _ := files.ResolveFeatureBotInstanceID(*guild, feature)
	if resolvedID == "" {
		return false
	}
	tokenEnc, ok := guild.BotInstanceTokens[resolvedID]

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

func (ch *CommandHandler) GetSyncer() *commands.CommandSyncer {
	return ch.syncer.Load()
}

// --- RegistrarContext Implementation ---
// These methods satisfy the read-only boundary required by the CommandCatalogRegistrars
// without exposing internal synchronization primitives or lifecycle controls.

func (ch *CommandHandler) SessionToken() string {
	if ch.session != nil {
		return ch.session.Token
	}
	return ""
}

func (ch *CommandHandler) ConfigProvider() config.Provider {
	return ch.configManager
}

func (ch *CommandHandler) RuntimeApplier() *runtimeapply.Manager {
	return ch.runtimeApplier
}

func (ch *CommandHandler) PartnerService() *partners.PartnerService {
	return ch.partnerService
}

func (ch *CommandHandler) ModerationMetrics() moderation.Metrics {
	return ch.moderationMetrics
}

func (ch *CommandHandler) RolePanelService() *roles.RolePanelService {
	return ch.rolePanelService
}

func (ch *CommandHandler) EmbedService() *embeds.EmbedService {
	return ch.embedService
}

func (ch *CommandHandler) QOTDService() qotdcmd.Service {
	return ch.qotdService
}

func (ch *CommandHandler) StatsService() *stats.StatsService {
	return ch.statsService
}
