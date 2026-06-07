package commands

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/admin"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/analytics"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/partner"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/roles"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
)

// CommandCatalogCapabilities captures runtime capabilities that can gate
// catalog registration.
type CommandCatalogCapabilities struct {
	Admin bool
}

// CommandCatalogRegistrar applies one domain-scoped command catalog fragment to
// a command router.
type CommandCatalogRegistrar struct {
	RequiredCapabilities CommandCatalogCapabilities
	Register             func(*CommandHandler, *core.CommandRouter)
}

// DefaultCommandCatalogRegistrars preserves the legacy all-catalog behavior for
// callers that do not inject a profile-specific registrar set.
func DefaultCommandCatalogRegistrars() []CommandCatalogRegistrar {
	return []CommandCatalogRegistrar{
		BaseCommandCatalogRegistrar(),
		QOTDCommandCatalogRegistrar(),
	}
}

// BaseCommandCatalogRegistrar registers the default-domain slash command
// surfaces.
func BaseCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Register: func(ch *CommandHandler, router *core.CommandRouter) {
			runtime.NewRuntimeConfigCommands(ch.configManager).RegisterCommands(router)
			analytics.RegisterAnalyticsCommands(router)
			partner.NewPartnerCommands(ch.configManager).RegisterCommands(router)
			moderation.RegisterModerationCommandsWithMetrics(router, ch.moderationMetrics)
			roles.NewRolePanelCommands(ch.configManager).RegisterCommands(router)
			embeds.NewEmbedCommands(ch.configManager).RegisterCommands(router)
		},
	}
}

// QOTDCommandCatalogRegistrar registers the QOTD domain slash command surfaces.
func QOTDCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Register: func(ch *CommandHandler, router *core.CommandRouter) {
			qotdcmd.NewCommands(ch.qotdService).RegisterCommands(router)
		},
	}
}

// AdminCommandCatalogRegistrar registers the default-domain admin command
// surface when the runtime exposes admin capability.
func AdminCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		RequiredCapabilities: CommandCatalogCapabilities{
			Admin: true,
		},
		Register: func(ch *CommandHandler, router *core.CommandRouter) {
			if ch.adminServiceManager == nil {
				return
			}
			admin.NewAdminCommands(ch.adminServiceManager).RegisterCommands(router)
		},
	}
}
