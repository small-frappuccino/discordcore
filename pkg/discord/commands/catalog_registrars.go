package commands

import (
	"log/slog"

	embedscmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	partnercmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/partners"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	rolescmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/stats"
	tickets_cmds "github.com/small-frappuccino/discordcore/pkg/discord/commands/tickets"
)

// CommandCatalogCapabilities captures runtime capabilities that can gate
// catalog registration.
type CommandCatalogCapabilities struct {
	Stats bool
}

// CommandCatalogRegistrar applies one domain-scoped command catalog fragment to
// a command router.
type CommandCatalogRegistrar struct {
	RequiredCapabilities CommandCatalogCapabilities
	Register             func(*CommandHandler, *legacycore.CommandRouter)
	RegisterArikawa      func(*CommandHandler, *legacycore.ArikawaCommandRouter)
}

// DefaultCommandCatalogRegistrars preserves the legacy all-catalog behavior for
// callers that do not inject a profile-specific registrar set.
func DefaultCommandCatalogRegistrars() []CommandCatalogRegistrar {
	return []CommandCatalogRegistrar{
		RuntimeCommandCatalogRegistrar(),
		PartnerCommandCatalogRegistrar(),
		ModerationCommandCatalogRegistrar(),
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
		Register: func(ch *CommandHandler, router *legacycore.CommandRouter) {
			runtime.NewRuntimeConfigCommands(ch.configManager).RegisterCommands(router)
		},
	}
}

// PartnerCommandCatalogRegistrar registers the partner slash command surface.
func PartnerCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router *legacycore.ArikawaCommandRouter) {
			partnercmd.NewPartnerCommands(ch.configManager, ch.partnerService).RegisterCommands(router)
		},
	}
}

// ModerationCommandCatalogRegistrar registers the moderation slash command surface.
func ModerationCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Register: func(ch *CommandHandler, router *legacycore.CommandRouter) {
			moderation.RegisterModerationCommandsWithMetrics(router, ch.moderationMetrics)
		},
	}
}

// RolesCommandCatalogRegistrar registers the roles slash command surface.
func RolesCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router *legacycore.ArikawaCommandRouter) {
			rolescmd.NewRolePanelCommands(ch.configManager, ch.rolePanelService).RegisterCommands(router)
		},
	}
}

// EmbedsCommandCatalogRegistrar registers the embeds slash command surface.
func EmbedsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RegisterArikawa: func(ch *CommandHandler, router *legacycore.ArikawaCommandRouter) {
			embedscmd.NewEmbedCommands(ch.configManager, ch.embedService).RegisterCommands(router)
		},
	}
}

// TicketsCommandCatalogRegistrar registers the tickets interaction routing surface.
func TicketsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Register: func(ch *CommandHandler, router *legacycore.CommandRouter) {
			if ch.ticketService != nil {
				tickets_cmds.RegisterComponents(router, ch.ticketService)
			}
		},
	}
}

// QOTDCommandCatalogRegistrar registers the QOTD domain slash command surfaces.
func QOTDCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Register: func(ch *CommandHandler, router *legacycore.CommandRouter) {
			qotdcmd.NewCommands(ch.qotdService).RegisterCommands(router)
		},
	}
}

// StatsCommandCatalogRegistrar registers the stats domain slash command surface.
func StatsCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RequiredCapabilities: CommandCatalogCapabilities{
			Stats: true,
		},
		Register: func(ch *CommandHandler, router *legacycore.CommandRouter) {
			stats.NewStatsCommands(ch.configManager, ch.statsService, slog.Default()).RegisterCommands(ch.GetCommandManager().GetArikawaRouter())
		},
	}
}

// LoggingCommandCatalogRegistrar registers the logging slash command surface.
func LoggingCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Register: func(ch *CommandHandler, router *legacycore.CommandRouter) {
			logging.NewLoggingCommands(ch.configManager).RegisterCommands(ch.GetCommandManager().GetArikawaRouter())
		},
	}
}
