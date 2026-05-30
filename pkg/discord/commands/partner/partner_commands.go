package partner

import (
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	optionName        = "name"
	optionCurrentName = "current_name"
	optionFandom      = "fandom"
	optionLink        = "link"
	optionWebhookURL  = "webhook_url"
	optionMessageID   = "message_id"
	optionURL         = "url"
)

type PartnerCommands struct {
	configManager *files.ConfigManager
	syncer        *partnerPostingSyncer
}

func NewPartnerCommands(configManager *files.ConfigManager) *PartnerCommands {
	return &PartnerCommands{
		configManager: configManager,
		syncer:        newPartnerPostingSyncer(configManager),
	}
}

func (pc *PartnerCommands) RegisterCommands(router *core.CommandRouter) {
	if router == nil || pc == nil || pc.configManager == nil {
		return
	}

	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	group := core.NewGroupCommand(
		"partner",
		"Manage partner board records",
		checker,
	)

	group.AddSubCommand(newPartnerAddSubCommand(pc.configManager, pc.syncer))
	group.AddSubCommand(newPartnerRemoveSubCommand(pc.configManager, pc.syncer))
	group.AddSubCommand(newPartnerLinkSubCommand(pc.configManager, pc.syncer))
	group.AddSubCommand(newPartnerRenameSubCommand(pc.configManager, pc.syncer))
	group.AddSubCommand(newPartnerListSubCommand(pc.configManager))
	group.AddSubCommand(newPartnerPostSubCommand(pc.configManager))
	group.AddSubCommand(newPartnerUnpostSubCommand(pc.configManager))
	group.AddSubCommand(newPartnerRefreshSubCommand(pc.configManager, pc.syncer))
	group.AddSubCommand(newPartnerImportTemplateSubCommand(pc.configManager))
	group.AddSubCommand(newPartnerExportTemplateSubCommand(pc.configManager))

	router.RegisterSlashCommand(group)
}

func parseWebhookURL(url string) (string, string, bool) {
	if url == "" {
		return "", "", false
	}
	parts := strings.Split(url, "/api/webhooks/")
	if len(parts) != 2 {
		return "", "", false
	}

	pathOnly := parts[1]
	if idx := strings.IndexAny(pathOnly, "?#"); idx != -1 {
		pathOnly = pathOnly[:idx]
	}

	creds := strings.Split(strings.TrimRight(pathOnly, "/"), "/")
	if len(creds) != 2 {
		return "", "", false
	}

	return creds[0], creds[1], true
}
