package partner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type partnerImportTemplateSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerImportTemplateSubCommand(cm *files.ConfigManager) *partnerImportTemplateSubCommand {
	return &partnerImportTemplateSubCommand{configManager: cm}
}

func (c *partnerImportTemplateSubCommand) Name() string { return "import_template" }

func (c *partnerImportTemplateSubCommand) Description() string {
	return "Import partner template embed properties from a Pastebin URL"
}

func (c *partnerImportTemplateSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        optionURL,
			Description: "The URL of the Pastebin/Discohook JSON",
			Required:    true,
		},
	}
}

func (c *partnerImportTemplateSubCommand) RequiresGuild() bool       { return true }
func (c *partnerImportTemplateSubCommand) RequiresPermissions() bool { return true }
func (c *partnerImportTemplateSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return nil, nil
}

func (c *partnerImportTemplateSubCommand) Handle(ctx *core.Context) error {
	builder := core.NewResponseBuilder(ctx.Session).Ephemeral()
	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return err
	}
	ctx.Acknowledged = true
	guildID := ctx.GuildID

	var pasteURL string
	opts := core.GetSubCommandOptions(ctx.Interaction)
	for _, opt := range opts {
		if opt.Name == optionURL {
			pasteURL = strings.TrimSpace(fmt.Sprint(opt.Value))
		}
	}

	data, err := discord.FetchPastebinContent(context.Background(), pasteURL)
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to fetch from pastebin: %v", err))
	}

	discohookEmbed, err := files.ParseAndValidateDiscohookJSON(data)
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Invalid embed JSON: %v", err))
	}

	var currentTemplate files.PartnerBoardTemplateConfig
	boardCfg, _ := c.configManager.PartnerBoard(guildID)
	currentTemplate = boardCfg.Template

	newTemplate := files.ToPartnerBoardTemplate(discohookEmbed, currentTemplate)
	if err := c.configManager.SetPartnerBoardTemplate(guildID, newTemplate); err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to save imported partner board template: %v", err))
	}

	return builder.WithContext(ctx).Success(ctx.Interaction, "Successfully imported JSON into partner board template.")
}

type partnerExportTemplateSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerExportTemplateSubCommand(cm *files.ConfigManager) *partnerExportTemplateSubCommand {
	return &partnerExportTemplateSubCommand{configManager: cm}
}

func (c *partnerExportTemplateSubCommand) Name() string { return "export_template" }

func (c *partnerExportTemplateSubCommand) Description() string {
	return "Export partner template embed properties to a Pastebin provider"
}

func (c *partnerExportTemplateSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

func (c *partnerExportTemplateSubCommand) RequiresGuild() bool       { return true }
func (c *partnerExportTemplateSubCommand) RequiresPermissions() bool { return true }
func (c *partnerExportTemplateSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return nil, nil
}

func (c *partnerExportTemplateSubCommand) Handle(ctx *core.Context) error {
	builder := core.NewResponseBuilder(ctx.Session).Ephemeral()
	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return err
	}
	ctx.Acknowledged = true
	guildID := ctx.GuildID

	boardCfg, err := c.configManager.PartnerBoard(guildID)
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, "No partner board configuration found for this guild.")
	}

	discohookJSON := files.FromPartnerBoardTemplate(boardCfg.Template)
	data, err := json.MarshalIndent(discohookJSON, "", "  ")
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to format JSON: %v", err))
	}

	ownerID := ""
	if g, err := ctx.Session.State.Guild(ctx.GuildID); err == nil {
		ownerID = g.OwnerID
	}

	url, err := discord.UploadExportedContent(context.Background(), ctx.Interaction.Member, ownerID, c.configManager, data)
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to upload: %v", err))
	}

	return builder.WithContext(ctx).Success(ctx.Interaction, fmt.Sprintf("Partner template successfully exported: <%s>", url))
}
