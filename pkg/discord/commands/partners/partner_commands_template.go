package partners

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	localdiscord "github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

type partnerImportTemplateSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerImportTemplateSubCommand(cm *files.ConfigManager) *partnerImportTemplateSubCommand {
	return &partnerImportTemplateSubCommand{configManager: cm}
}

// Name names.
func (c *partnerImportTemplateSubCommand) Name() string { return "import_template" }

// Description descriptions.
func (c *partnerImportTemplateSubCommand) Description() string {
	return "Import partner template embed properties from a Pastebin URL"
}

// Options options.
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

// RequiresGuild requires guild.
func (c *partnerImportTemplateSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerImportTemplateSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *partnerImportTemplateSubCommand) HandleAutocomplete(ctx *legacycore.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return nil, nil
}

// Handle handles.
func (c *partnerImportTemplateSubCommand) Handle(ctx *legacycore.Context) error {
	builder := legacycore.NewResponseBuilder(ctx.Session).Ephemeral()
	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return fmt.Errorf("partnerImportTemplateSubCommand.Handle: %w", err)
	}
	ctx.Acknowledged = true
	guildID := ctx.GuildID

	var pasteURL string
	opts := legacycore.GetSubCommandOptions(ctx.Interaction)
	for _, opt := range opts {
		if opt.Name == optionURL {
			pasteURL = strings.TrimSpace(fmt.Sprint(opt.Value))
		}
	}

	data, err := localdiscord.FetchPastebinContent(context.Background(), pasteURL)
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

// Name names.
func (c *partnerExportTemplateSubCommand) Name() string { return "export_template" }

// Description descriptions.
func (c *partnerExportTemplateSubCommand) Description() string {
	return "Export partner template embed properties to a Pastebin provider"
}

// Options options.
func (c *partnerExportTemplateSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

// RequiresGuild requires guild.
func (c *partnerExportTemplateSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerExportTemplateSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *partnerExportTemplateSubCommand) HandleAutocomplete(ctx *legacycore.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return nil, nil
}

// Handle handles.
func (c *partnerExportTemplateSubCommand) Handle(ctx *legacycore.Context) error {
	builder := legacycore.NewResponseBuilder(ctx.Session).Ephemeral()
	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return fmt.Errorf("partnerExportTemplateSubCommand.Handle: %w", err)
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

	url, err := localdiscord.UploadExportedContent(context.Background(), ctx.Interaction.Member, ownerID, c.configManager, data)
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to upload: %v", err))
	}

	return builder.WithContext(ctx).Success(ctx.Interaction, fmt.Sprintf("Partner template successfully exported: <%s>", url))
}
