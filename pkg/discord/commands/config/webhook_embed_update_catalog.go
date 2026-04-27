package config

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// webhookEmbedInteractionCatalog keeps the webhook-embed update workflow local
// to the feature so /config registration does not need to know each individual
// subcommand constructor.
type webhookEmbedInteractionCatalog struct {
	configManager *files.ConfigManager
}

func newWebhookEmbedInteractionCatalog(configManager *files.ConfigManager) webhookEmbedInteractionCatalog {
	return webhookEmbedInteractionCatalog{configManager: configManager}
}

func (catalog webhookEmbedInteractionCatalog) appendToGroup(group *core.GroupCommand) {
	if group == nil {
		return
	}
	for _, subcommand := range catalog.subcommands() {
		group.AddSubCommand(subcommand)
	}
}

func (catalog webhookEmbedInteractionCatalog) subcommands() []core.SubCommand {
	return []core.SubCommand{
		NewConfigWebhookEmbedCreateSubCommand(catalog.configManager),
		NewConfigWebhookEmbedReadSubCommand(catalog.configManager),
		NewConfigWebhookEmbedUpdateSubCommand(catalog.configManager),
		NewConfigWebhookEmbedDeleteSubCommand(catalog.configManager),
		NewConfigWebhookEmbedListSubCommand(catalog.configManager),
	}
}