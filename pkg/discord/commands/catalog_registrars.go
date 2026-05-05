package commands

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/admin"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/metrics"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/partner"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/runtime"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/partners"
)

// CommandCatalogCapabilities captures runtime capabilities that can gate
// catalog registration.
type CommandCatalogCapabilities struct {
	Admin bool
}

// CommandCatalogRegistrar applies one domain-scoped command catalog fragment to
// a command router.
type CommandCatalogRegistrar struct {
	Domain               string
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
		Domain: "",
		Register: func(ch *CommandHandler, router *core.CommandRouter) {
			configCommands := config.NewConfigCommands(ch.configManager)
			configCommands.RegisterBaseCommands(router)
			runtime.NewRuntimeConfigCommands(ch.configManager).RegisterCommands(router)
			metrics.RegisterMetricsCommands(router)
			if ch.partnerBoardService != nil || ch.partnerSyncExecutor != nil {
				boardService := ch.partnerBoardService
				if boardService == nil {
					boardService = partners.NewBoardApplicationService(ch.configManager, nil)
				}
				partner.NewPartnerCommandsWithServices(boardService, ch.partnerSyncExecutor).RegisterCommands(router)
			} else {
				partner.NewPartnerCommands(ch.configManager).RegisterCommands(router)
			}
			moderation.RegisterModerationCommands(router)
		},
	}
}

// QOTDCommandCatalogRegistrar registers the QOTD domain slash command surfaces.
func QOTDCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Domain: files.BotDomainQOTD,
		Register: func(ch *CommandHandler, router *core.CommandRouter) {
			configCommands := config.NewConfigCommands(ch.configManager)
			configCommands.RegisterQOTDCommands(router)
			qotdcmd.NewCommands(ch.qotdService).RegisterCommands(router)
		},
	}
}

// AdminCommandCatalogRegistrar registers the default-domain admin command
// surface when the runtime exposes admin capability.
func AdminCommandCatalogRegistrar() CommandCatalogRegistrar {
	return CommandCatalogRegistrar{
		Domain: "",
		RequiredCapabilities: CommandCatalogCapabilities{
			Admin: true,
		},
		Register: func(ch *CommandHandler, router *core.CommandRouter) {
			if ch.adminServiceManager == nil {
				return
			}
			admin.NewAdminCommands(ch.adminServiceManager, ch.adminUnifiedCache, ch.adminStore).RegisterCommands(router)
		},
	}
}