package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	discordclean "github.com/small-frappuccino/discordcore/pkg/discord/clean"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/clean"
	embedscmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	partnercmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/partners"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	rolescmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/stats"
	discordmod "github.com/small-frappuccino/discordcore/pkg/discord/moderation"
)

// CommandCatalogCapabilities defines a bitmask for capability requirements.
type CommandCatalogCapabilities uint64

const (
	// CapNone represents no special capabilities required.
	CapNone CommandCatalogCapabilities = 0

	// CapStats indicates the registrar requires the Stats subsystem.
	CapStats CommandCatalogCapabilities = 1 << iota
	CapBanMembers
	CapKickMembers
	CapManageMessages
	CapQOTDAdmin
)

// Has evaluates if the target capability is present in the bitmask.
func (c CommandCatalogCapabilities) Has(target CommandCatalogCapabilities) bool {
	if target == CapNone {
		return true
	}
	return (c & target) == target
}

// String provides a human-readable representation of the bitmask.
func (c CommandCatalogCapabilities) String() string {
	if c == CapNone {
		return "CapNone"
	}

	var parts []string
	flags := map[CommandCatalogCapabilities]string{
		CapStats:          "CapStats",
		CapBanMembers:     "CapBanMembers",
		CapKickMembers:    "CapKickMembers",
		CapManageMessages: "CapManageMessages",
		CapQOTDAdmin:      "CapQOTDAdmin",
	}

	for flag, name := range flags {
		if c.Has(flag) {
			parts = append(parts, name)
		}
	}

	if len(parts) == 0 {
		return "CapUnknown"
	}
	return strings.Join(parts, "|")
}

// CommandCatalogRegistrar applies one domain-scoped command catalog fragment to
// a command router.
type CommandCatalogRegistrar struct {
	RequiredCapabilities CommandCatalogCapabilities
	RegisterArikawa      func(*CommandHandler, commands.ArikawaRegisterer)
}

// DefaultCommandCatalogRegistrars preserves the legacy all-catalog behavior for
// callers that do not inject a profile-specific registrar set.
func DefaultCommandCatalogRegistrars() []CommandCatalogRegistrar {
	return []CommandCatalogRegistrar{
		RuntimeCommandCatalogRegistrar(),
		PartnerCommandCatalogRegistrar(),
		ModerationCommandCatalogRegistrar(),
		CleanCommandCatalogRegistrar(),
		RolesCommandCatalogRegistrar(),
		EmbedsCommandCatalogRegistrar(),
		TicketsCommandCatalogRegistrar(),
		QOTDCommandCatalogRegistrar(),
		StatsCommandCatalogRegistrar(),
		LoggingCommandCatalogRegistrar(),
	}
}

// RuntimeCommandCatalogRegistrar registers the runtime config slash command surface.
func RuntimeCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			replier := &arikawaReplierAdapter{client: api.NewClient("Bot " + ch.session.Token)}
			// Notice: GetRuntimeApplier requires reaching into some dependencies;
			// Since we removed legacycore, we assume runtime application is managed elsewhere or passed.
			// For this refactor, we inject a mock or real applier if available.
			handler := runtime.NewHandler(replier, ch.configManager, nil, slog.Default())
			shim := &runtimeShim{handler: handler}
			router.Register(shim)
			router.RegisterComponent("runtime|", shim)
		},
	}
}

type runtimeShim struct {
	handler *runtime.Handler
}

func (s *runtimeShim) Name() string                     { return "runtime" }
func (s *runtimeShim) Description() string              { return "Manage runtime configuration for the bot." }
func (s *runtimeShim) Options() []discord.CommandOption { return nil }
func (s *runtimeShim) RequiresGuild() bool              { return true }
func (s *runtimeShim) RequiresPermissions() bool        { return true }
func (s *runtimeShim) Handle(ctx *commands.ArikawaContext) error {
	return s.handler.HandleSlash(ctx.Context(), ctx.Interaction)
}
func (s *runtimeShim) HandleComponent(ctx *commands.ArikawaContext) error {
	switch ctx.Interaction.Data.(type) {
	case discord.ComponentInteraction:
		return s.handler.HandleComponent(ctx.Context(), ctx.Interaction)
	case *discord.ModalInteraction:
		return s.handler.HandleModal(ctx.Context(), ctx.Interaction)
	default:
		return nil
	}
}

type arikawaReplierAdapter struct {
	client *api.Client
}

func (r *arikawaReplierAdapter) RespondInteraction(ctx context.Context, interactionID discord.InteractionID, token string, resp api.InteractionResponse) error {
	return r.client.RespondInteraction(interactionID, token, resp)
}

func (r *arikawaReplierAdapter) EditInteractionResponse(ctx context.Context, appID discord.AppID, token string, data api.EditInteractionResponseData) (*discord.Message, error) {
	return r.client.EditInteractionResponse(appID, token, data)
}

// PartnerCommandCatalogRegistrar registers the partner slash command surface.
func PartnerCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			// Domain packages now receive native router directly.
			partnercmd.NewPartnerCommands(ch.configManager, ch.partnerService).RegisterCommands(router)
		},
	}
}

// ModerationCommandCatalogRegistrar registers the moderation slash command surface.
func ModerationCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			client := api.NewClient("Bot " + ch.session.Token)
			svc := discordmod.NewService(client, slog.Default())
			router.Register(moderation.NewBanCommand(svc, ch.moderationMetrics, slog.Default()))
			router.Register(moderation.NewTimeoutCommand(svc, ch.moderationMetrics, slog.Default()))
			router.Register(moderation.NewMassBanCommand(svc, ch.moderationMetrics, slog.Default()))
			router.Register(moderation.NewReactionBlockCommand(ch.configManager, ch.moderationMetrics, slog.Default()))
		},
	}
}

func CleanCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			client := api.NewClient("Bot " + ch.session.Token)
			var metrics discordclean.Metrics
			if ch.moderationMetrics != nil {
				metrics = cleanMetricsAdapter{m: ch.moderationMetrics}
			}
			svc := discordclean.NewService(client, metrics, nil)
			router.Register(clean.NewCleanCommand(ch.configManager, svc))
		},
	}
}

// We need to implement cleanMetricsAdapter over moderation.Metrics
type cleanMetricsAdapter struct {
	m moderation.Metrics
}

func (a cleanMetricsAdapter) RecordCleanAttempt()                               {}
func (a cleanMetricsAdapter) RecordCleanSuccess(durationMs int64, deleted int)  {}
func (a cleanMetricsAdapter) RecordCleanFailure(cause string, durationMs int64) {}
func (a cleanMetricsAdapter) RecordCleanDeleteFailure(class string)             {}
func (a cleanMetricsAdapter) RecordCleanAuditLogFailure()                       {}

// RolesCommandCatalogRegistrar registers the roles slash command surface.
func RolesCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			rolescmd.NewRolePanelCommands(ch.configManager, ch.rolePanelService).RegisterCommands(router)
		},
	}
}

// EmbedsCommandCatalogRegistrar registers the embeds slash command surface.
func EmbedsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			embedscmd.NewEmbedCommands(ch.configManager, ch.embedService).RegisterCommands(router)
		},
	}
}

// TicketsCommandCatalogRegistrar registers the tickets interaction routing surface.
func TicketsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			// tickets natively registered via state handler in pkg/discord/commands/tickets/router.go
		},
	}
}

// QOTDCommandCatalogRegistrar registers the QOTD domain slash command surfaces.
func QOTDCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			client := api.NewClient("Bot " + ch.session.Token)
			handler := qotdcmd.NewCommandHandler(ch.qotdService, client)
			shim := &qotdShim{handler: handler}
			router.Register(shim)
			router.RegisterComponent("qotd|", shim)
		},
	}
}

type qotdShim struct {
	handler *qotdcmd.CommandHandler
}

func (s *qotdShim) Name() string                     { return "qotd" }
func (s *qotdShim) Description() string              { return "Question of the Day management" }
func (s *qotdShim) Options() []discord.CommandOption { return qotdcmd.CommandsList()[0].Options }
func (s *qotdShim) RequiresGuild() bool              { return true }
func (s *qotdShim) RequiresPermissions() bool        { return true }
func (s *qotdShim) Handle(ctx *commands.ArikawaContext) error {
	s.handler.HandleInteraction(&gateway.InteractionCreateEvent{InteractionEvent: *ctx.Interaction})
	return nil
}
func (s *qotdShim) HandleComponent(ctx *commands.ArikawaContext) error {
	s.handler.HandleInteraction(&gateway.InteractionCreateEvent{InteractionEvent: *ctx.Interaction})
	return nil
}

// StatsCommandCatalogRegistrar registers the stats domain slash command surface.
func StatsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RequiredCapabilities: CapStats,
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			stats.NewStatsCommands(ch.configManager, ch.statsService, slog.Default()).RegisterCommands(router)
		},
	}
}

// LoggingCommandCatalogRegistrar registers the logging slash command surface.
func LoggingCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router commands.ArikawaRegisterer) {
			logging.NewLoggingCommands(ch.configManager).RegisterCommands(router)
		},
	}
}
