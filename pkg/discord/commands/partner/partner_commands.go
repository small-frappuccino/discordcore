package partner

import (
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/files"
	partnersvc "github.com/small-frappuccino/discordcore/pkg/partners"
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

// PartnerCommands registers and backs the "/partner" command group, which
// manages partner-board records and syncs them to their posting channel.
type PartnerCommands struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

// NewPartnerCommands news partner commands.
func NewPartnerCommands(configManager *files.ConfigManager, svc *partnersvc.PartnerService) *PartnerCommands {
	return &PartnerCommands{
		configManager:  configManager,
		partnerService: svc,
	}
}

// RegisterCommands registers commands.
func (pc *PartnerCommands) RegisterCommands(router *legacycore.CommandRouter) {
	if router == nil || pc == nil || pc.configManager == nil {
		return
	}

	checker := legacycore.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	group := legacycore.NewGroupCommand(
		"partner",
		"Manage partner board records",
		checker,
	)

	group.AddSubCommand(newPartnerAddSubCommand(pc.configManager, pc.partnerService))
	group.AddSubCommand(newPartnerRemoveSubCommand(pc.configManager, pc.partnerService))
	group.AddSubCommand(newPartnerLinkSubCommand(pc.configManager, pc.partnerService))
	group.AddSubCommand(newPartnerRenameSubCommand(pc.configManager, pc.partnerService))
	group.AddSubCommand(newPartnerListSubCommand(pc.configManager))
	group.AddSubCommand(newPartnerPostSubCommand(pc.configManager, pc.partnerService))
	group.AddSubCommand(newPartnerUnpostSubCommand(pc.configManager))
	group.AddSubCommand(newPartnerRefreshSubCommand(pc.configManager, pc.partnerService))
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
