// Package roles owns the slash-command surface and component handler for
// guild-configured self-service role panels. The published panel is a
// regular Discord message authored by the bot with one button per role;
// clicking a button toggles that role on the invoking member.
//
// The persisted shape lives on files.GuildConfig.RolePanels and is
// edited through ConfigManager methods (UpsertRolePanelButton,
// SetRolePanelEmbed, etc.) so dashboards, future migrations and tests
// share the same canonical entry points.
package roles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	rolesvc "github.com/small-frappuccino/discordcore/pkg/roles"
	"github.com/small-frappuccino/discordgo"
)

const (
	rolePanelFeatureID = "role_panels"

	rolePanelCommandName     = "roles"
	rolePanelButtonGroupName = "button"

	rolePanelSubPost         = "post"
	rolePanelSubPreview      = "preview"
	rolePanelSubSet          = "set"
	rolePanelSubDelete       = "delete"
	rolePanelSubList         = "list"
	rolePanelSubRefresh      = "refresh"
	rolePanelSubUnpost       = "unpost"
	rolePanelSubImport       = "import"
	rolePanelSubExport       = "export"
	rolePanelSubButtonAdd    = "add"
	rolePanelSubButtonRemove = "remove"
	rolePanelSubButtonList   = "list"

	rolePanelOptionKey         = "key"
	rolePanelOptionWebhookURL  = "webhook_url"
	rolePanelOptionTitle       = "title"
	rolePanelOptionDescription = "description"
	rolePanelOptionColor       = "color"
	rolePanelOptionRole        = "role"
	rolePanelOptionLabel       = "label"
	rolePanelOptionEmoji       = "emoji"
	rolePanelOptionMessageID   = "message_id"
	rolePanelOptionURL         = "url"

	rolePanelOptionAuthorName   = "author_name"
	rolePanelOptionAuthorIcon   = "author_icon_url"
	rolePanelOptionFooterText   = "footer_text"
	rolePanelOptionFooterIcon   = "footer_icon_url"
	rolePanelOptionImageURL     = "image_url"
	rolePanelOptionThumbnailURL = "thumbnail_url"
	rolePanelOptionFieldName    = "name"
	rolePanelOptionFieldValue   = "value"
	rolePanelOptionFieldInline  = "inline"
	rolePanelOptionFieldIndex   = "index"

	rolePanelFieldGroupName = "field"
	rolePanelSubFieldAdd    = "add"
	rolePanelSubFieldRemove = "remove"
	rolePanelSubFieldList   = "list"
)

// RolePanelCommands wires the /roles command tree into the router.
type RolePanelCommands struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

// NewRolePanelCommands builds the command bundle.
func NewRolePanelCommands(configManager *files.ConfigManager, svc *rolesvc.RolePanelService) *RolePanelCommands {
	return &RolePanelCommands{
		configManager:    configManager,
		rolePanelService: svc,
	}
}

// RegisterCommands registers the slash group and the component route
// shared by every panel button on the supplied router. The component
// route uses an ephemeral defer ack so the click responds with a small
// confirmation visible only to the clicker.
func (rc *RolePanelCommands) RegisterCommands(router *core.CommandRouter) {
	if router == nil || rc == nil || rc.configManager == nil {
		return
	}

	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())

	rolesGroup := core.NewGroupCommand(
		rolePanelCommandName,
		"Manage self-service role panels for this server",
		checker,
	)
	rolesGroup.AddSubCommand(newRolePanelPostSubCommand(rc.configManager, rc.rolePanelService))
	rolesGroup.AddSubCommand(newRolePanelPreviewSubCommand(rc.configManager, rc.rolePanelService))
	rolesGroup.AddSubCommand(newRolePanelSetSubCommand(rc.configManager, rc.rolePanelService))
	rolesGroup.AddSubCommand(newRolePanelDeleteSubCommand(rc.configManager, rc.rolePanelService))
	rolesGroup.AddSubCommand(newRolePanelListSubCommand(rc.configManager))
	rolesGroup.AddSubCommand(newRolePanelRefreshSubCommand(rc.configManager, rc.rolePanelService))
	rolesGroup.AddSubCommand(newRolePanelUnpostSubCommand(rc.configManager, rc.rolePanelService))
	rolesGroup.AddSubCommand(newRolePanelToggleSubCommand(rc.configManager))
	rolesGroup.AddSubCommand(newRolePanelImportSubCommand(rc.configManager, rc.rolePanelService))
	rolesGroup.AddSubCommand(newRolePanelExportSubCommand(rc.configManager))

	buttonGroup := core.NewGroupCommand(
		rolePanelButtonGroupName,
		"Manage the buttons on one role panel",
		checker,
	)
	buttonGroup.AddSubCommand(newRolePanelButtonAddSubCommand(rc.configManager, rc.rolePanelService))
	buttonGroup.AddSubCommand(newRolePanelButtonRemoveSubCommand(rc.configManager, rc.rolePanelService))
	buttonGroup.AddSubCommand(newRolePanelButtonListSubCommand(rc.configManager))
	rolesGroup.AddSubCommand(buttonGroup)

	fieldGroup := core.NewGroupCommand(
		rolePanelFieldGroupName,
		"Manage the fields on one role panel embed",
		checker,
	)
	fieldGroup.AddSubCommand(newRolePanelFieldAddSubCommand(rc.configManager, rc.rolePanelService))
	fieldGroup.AddSubCommand(newRolePanelFieldRemoveSubCommand(rc.configManager, rc.rolePanelService))
	fieldGroup.AddSubCommand(newRolePanelFieldListSubCommand(rc.configManager))
	rolesGroup.AddSubCommand(fieldGroup)

	router.RegisterSlashCommand(rolesGroup)

	router.RegisterInteractionRoutes(core.InteractionRouteBinding{
		Path:      rolesvc.RolePanelComponentRouteID,
		Component: newRolePanelComponentHandler(rc.configManager),
		AckPolicy: core.InteractionAckPolicy{
			Mode:      core.InteractionAckModeNone,
			Ephemeral: true,
		},
	})
}

// --- Leaf subcommands: /roles post|preview|set|delete|list ---

type rolePanelPostSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelPostSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelPostSubCommand {
	return &rolePanelPostSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelPostSubCommand) Name() string { return rolePanelSubPost }

// Description descriptions.
func (c *rolePanelPostSubCommand) Description() string {
	return "Post one role panel publicly in this channel"
}

// Options options.
func (c *rolePanelPostSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		rolePanelKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionWebhookURL, Description: "Discord Webhook URL to post the panel with a custom name and avatar", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelPostSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelPostSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelPostSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelPostSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelPostSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelPostSubCommand.Handle: %w", err)
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("rolePanelPostSubCommand.Handle: %w", err)
	}
	if len(panel.Buttons) == 0 {
		return rolePanelDetailedCommandError(fmt.Sprintf("Panel `%s` has no buttons configured yet. Add at least one with /roles button add.", panel.Key))
	}

	embed := c.rolePanelService.RenderEmbed(&panel)
	components := c.rolePanelService.RenderComponents(&panel)

	var messageID, channelID, webhookID, webhookToken string
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	if extractor.HasOption(rolePanelOptionWebhookURL) {
		webhookURL := extractor.String(rolePanelOptionWebhookURL)
		wID, wToken, parseErr := parseRolePanelWebhookURL(webhookURL)
		if parseErr != nil {
			return rolePanelDetailedCommandError(parseErr.Error())
		}

		targetWebhook, err := ctx.Session.WebhookWithToken(wID, wToken)
		if err != nil {
			return rolePanelDetailedCommandError(fmt.Sprintf("Failed to fetch the provided webhook: %v", err))
		}

		executionWebhookID := wID
		executionWebhookToken := wToken
		var overrideUsername, overrideAvatarURL string

		// Discord strips components (like buttons) from webhooks that are not owned by the bot application.
		// To fix this, if the provided webhook is user-owned, we find or create an application-owned webhook
		// in the same channel and use it to impersonate the target webhook by overriding the username and avatar.
		if targetWebhook.ApplicationID != ctx.Session.State.User.ID {
			appWebhooks, err := ctx.Session.ChannelWebhooks(targetWebhook.ChannelID)
			if err != nil {
				return rolePanelDetailedCommandError(fmt.Sprintf("Failed to list channel webhooks (requires Manage Webhooks permission to preserve buttons): %v", err))
			}

			var appHook *discordgo.Webhook
			for _, hw := range appWebhooks {
				if hw.ApplicationID == ctx.Session.State.User.ID {
					appHook = hw
					break
				}
			}

			if appHook == nil {
				appHook, err = ctx.Session.WebhookCreate(targetWebhook.ChannelID, "Role Panel Webhook", "")
				if err != nil {
					return rolePanelDetailedCommandError(fmt.Sprintf("Failed to create bot-owned webhook to preserve buttons: %v", err))
				}
			}

			executionWebhookID = appHook.ID
			executionWebhookToken = appHook.Token
			overrideUsername = targetWebhook.Name
			if targetWebhook.Avatar != "" {
				overrideAvatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", targetWebhook.ID, targetWebhook.Avatar)
			}
		}

		msg, err := ctx.Session.WebhookExecute(executionWebhookID, executionWebhookToken, true, &discordgo.WebhookParams{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
			Username:   overrideUsername,
			AvatarURL:  overrideAvatarURL,
		})
		if err != nil {
			return rolePanelDetailedCommandError(fmt.Sprintf("Failed to post the panel via webhook: %v", err))
		}
		if msg != nil {
			messageID = msg.ID
			channelID = msg.ChannelID
			webhookID = executionWebhookID
			webhookToken = executionWebhookToken
		}
	} else {
		msg, err := ctx.Session.ChannelMessageSendComplex(ctx.Interaction.ChannelID, &discordgo.MessageSend{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		})
		if err != nil {
			return rolePanelDetailedCommandError(fmt.Sprintf("Failed to post the panel: %v", err))
		}
		if msg != nil {
			messageID = msg.ID
			channelID = msg.ChannelID
		}
	}

	postingNote := ""
	if messageID != "" && channelID != "" {
		posting := files.RolePanelPostingConfig{
			ChannelID:    channelID,
			MessageID:    messageID,
			WebhookID:    webhookID,
			WebhookToken: webhookToken,
		}
		if err := c.configManager.AddRolePanelPosting(ctx.GuildID, panel.Key, posting); err != nil {
			postingNote = fmt.Sprintf("\nWarning: the posting could not be tracked for later cleanup: %v", err)
		}
	}

	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Panel `%s` was posted in <#%s>.%s", panel.Key, ctx.Interaction.ChannelID, postingNote),
	)
}

type rolePanelPreviewSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelPreviewSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelPreviewSubCommand {
	return &rolePanelPreviewSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelPreviewSubCommand) Name() string { return rolePanelSubPreview }

// Description descriptions.
func (c *rolePanelPreviewSubCommand) Description() string {
	return "Show an ephemeral preview of one role panel"
}

// Options options.
func (c *rolePanelPreviewSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{rolePanelKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *rolePanelPreviewSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelPreviewSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelPreviewSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelPreviewSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelPreviewSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelPreviewSubCommand.Handle: %w", err)
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("rolePanelPreviewSubCommand.Handle: %w", err)
	}

	embed := c.rolePanelService.RenderEmbed(&panel)
	components := c.rolePanelService.RenderComponents(&panel)

	rm := rolePanelPreviewResponseBuilder(ctx.Session).WithComponents(components...).Build()
	return rm.Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}

type rolePanelSetSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelSetSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelSetSubCommand {
	return &rolePanelSetSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelSetSubCommand) Name() string { return rolePanelSubSet }

// Description descriptions.
func (c *rolePanelSetSubCommand) Description() string {
	return "Set embed title, description, and color for one role panel"
}

// Options options.
func (c *rolePanelSetSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		rolePanelKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionTitle, Description: "Embed title (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionDescription, Description: "Embed description (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: rolePanelOptionColor, Description: "Embed color as a decimal RGB integer (e.g. 16753104). 0 to clear.", Required: false, MinValue: new(float64(0)), MaxValue: float64(files.RolePanelColorMax)},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionAuthorName, Description: "Embed author name (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionAuthorIcon, Description: "Embed author icon URL (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionFooterText, Description: "Embed footer text (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionFooterIcon, Description: "Embed footer icon URL (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionImageURL, Description: "Embed image URL (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionThumbnailURL, Description: "Embed thumbnail URL (omit to keep current, pass empty string to clear)", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelSetSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelSetSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelSetSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelSetSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelSetSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelSetSubCommand.Handle: %w", err)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	current, fetchErr := c.configManager.RolePanel(ctx.GuildID, key)
	if fetchErr != nil && !errors.Is(fetchErr, files.ErrRolePanelNotFound) {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to load panel `%s`: %v", key, fetchErr))
	}

	embed := current
	if extractor.HasOption(rolePanelOptionTitle) {
		embed.Title = extractor.String(rolePanelOptionTitle)
	}
	if extractor.HasOption(rolePanelOptionDescription) {
		embed.Description = extractor.String(rolePanelOptionDescription)
	}
	if extractor.HasOption(rolePanelOptionColor) {
		embed.Color = int(extractor.Int(rolePanelOptionColor))
	}
	if extractor.HasOption(rolePanelOptionAuthorName) {
		embed.AuthorName = extractor.String(rolePanelOptionAuthorName)
	}
	if extractor.HasOption(rolePanelOptionAuthorIcon) {
		embed.AuthorIconURL = extractor.String(rolePanelOptionAuthorIcon)
	}
	if extractor.HasOption(rolePanelOptionFooterText) {
		embed.FooterText = extractor.String(rolePanelOptionFooterText)
	}
	if extractor.HasOption(rolePanelOptionFooterIcon) {
		embed.FooterIconURL = extractor.String(rolePanelOptionFooterIcon)
	}
	if extractor.HasOption(rolePanelOptionImageURL) {
		embed.ImageURL = extractor.String(rolePanelOptionImageURL)
	}
	if extractor.HasOption(rolePanelOptionThumbnailURL) {
		embed.ThumbnailURL = extractor.String(rolePanelOptionThumbnailURL)
	}

	if err := c.configManager.SetRolePanelEmbed(ctx.GuildID, key, embed); err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to update panel `%s`: %v", key, err))
	}

	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Panel `%s` embed settings were updated.%s", key, syncNote),
	)
}

type rolePanelDeleteSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelDeleteSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelDeleteSubCommand {
	return &rolePanelDeleteSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelDeleteSubCommand) Name() string { return rolePanelSubDelete }

// Description descriptions.
func (c *rolePanelDeleteSubCommand) Description() string {
	return "Delete one role panel entirely"
}

// Options options.
func (c *rolePanelDeleteSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{rolePanelKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *rolePanelDeleteSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelDeleteSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelDeleteSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelDeleteSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelDeleteSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelDeleteSubCommand.Handle: %w", err)
	}

	panel, fetchErr := c.configManager.RolePanel(ctx.GuildID, key)
	if fetchErr != nil {
		if errors.Is(fetchErr, files.ErrRolePanelNotFound) {
			return rolePanelDetailedCommandError(fmt.Sprintf("Panel `%s` does not exist.", key))
		}
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to load panel `%s`: %v", key, fetchErr))
	}

	syncNote := ""
	if len(panel.Postings) > 0 {
		syncResult := c.rolePanelService.Sync(ctx.Session, ctx.GuildID, key, panel.Postings, &panel)
		if summary := c.rolePanelService.FormatSyncSummary(syncResult, "Stripped buttons from"); summary != "" {
			syncNote = "\n" + summary
		}
	}

	if err := c.configManager.DeleteRolePanel(ctx.GuildID, key); err != nil {
		if errors.Is(err, files.ErrRolePanelNotFound) {
			return rolePanelDetailedCommandError(fmt.Sprintf("Panel `%s` does not exist.", key))
		}
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to delete panel `%s`: %v", key, err))
	}

	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Panel `%s` was deleted.%s", key, syncNote),
	)
}

type rolePanelListSubCommand struct {
	configManager *files.ConfigManager
}

func newRolePanelListSubCommand(cm *files.ConfigManager) *rolePanelListSubCommand {
	return &rolePanelListSubCommand{configManager: cm}
}

// Name names.
func (c *rolePanelListSubCommand) Name() string { return rolePanelSubList }

// Description descriptions.
func (c *rolePanelListSubCommand) Description() string {
	return "List configured role panel keys for this server"
}

// Options options.
func (c *rolePanelListSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }

// RequiresGuild requires guild.
func (c *rolePanelListSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelListSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *rolePanelListSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelListSubCommand.Handle: %w", err)
	}
	panels, err := c.configManager.RolePanels(ctx.GuildID)
	if err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to list panels: %v", err))
	}
	if len(panels) == 0 {
		return rolePanelConfigurationResponseBuilder(ctx.Session).Info(
			ctx.Interaction,
			"No role panels are configured yet. Add buttons with /roles button add to create one.",
		)
	}

	var b strings.Builder
	b.WriteString("Configured role panels:\n")
	for _, p := range panels {
		b.WriteString(fmt.Sprintf("• `%s` — %d button(s)\n", p.Key, len(p.Buttons)))
	}
	return rolePanelConfigurationResponseBuilder(ctx.Session).Info(ctx.Interaction, strings.TrimSpace(b.String()))
}

// --- Subgroup: /roles button add|remove|list ---

type rolePanelButtonAddSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelButtonAddSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelButtonAddSubCommand {
	return &rolePanelButtonAddSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelButtonAddSubCommand) Name() string { return rolePanelSubButtonAdd }

// Description descriptions.
func (c *rolePanelButtonAddSubCommand) Description() string {
	return "Add or replace one button on a panel"
}

// Options options.
func (c *rolePanelButtonAddSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		rolePanelKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionRole, Name: rolePanelOptionRole, Description: "Role to toggle when the button is pressed", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionLabel, Description: "Button label shown in Discord", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionEmoji, Description: "Custom emoji like <:name:id> or a unicode glyph (optional)", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelButtonAddSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelButtonAddSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelButtonAddSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelButtonAddSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelButtonAddSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelButtonAddSubCommand.Handle: %w", err)
	}
	opts := core.GetSubCommandOptions(ctx.Interaction)
	extractor := core.OptionList(opts)

	roleID := strings.TrimSpace(roleOptionID(opts, rolePanelOptionRole))
	if roleID == "" {
		return rolePanelDetailedCommandError("A role is required to bind the button.")
	}
	label, err := extractor.StringRequired(rolePanelOptionLabel)
	if err != nil {
		return rolePanelDetailedCommandError(err.Error())
	}

	emojiName, emojiID, emojiAnimated, err := parseRolePanelButtonEmoji(extractor.String(rolePanelOptionEmoji))
	if err != nil {
		return rolePanelDetailedCommandError(err.Error())
	}

	button := files.RolePanelButtonConfig{
		RoleID:        roleID,
		Label:         label,
		EmojiName:     emojiName,
		EmojiID:       emojiID,
		EmojiAnimated: emojiAnimated,
	}
	if err := c.configManager.UpsertRolePanelButton(ctx.GuildID, key, button); err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to save button: %v", err))
	}
	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Button for <@&%s> was saved on panel `%s`.%s", roleID, key, syncNote),
	)
}

type rolePanelButtonRemoveSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelButtonRemoveSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelButtonRemoveSubCommand {
	return &rolePanelButtonRemoveSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelButtonRemoveSubCommand) Name() string { return rolePanelSubButtonRemove }

// Description descriptions.
func (c *rolePanelButtonRemoveSubCommand) Description() string {
	return "Remove one button from a panel"
}

// Options options.
func (c *rolePanelButtonRemoveSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		rolePanelKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionRole, Name: rolePanelOptionRole, Description: "Role whose button should be removed", Required: true},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelButtonRemoveSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelButtonRemoveSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelButtonRemoveSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelButtonRemoveSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelButtonRemoveSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelButtonRemoveSubCommand.Handle: %w", err)
	}
	roleID := strings.TrimSpace(roleOptionID(core.GetSubCommandOptions(ctx.Interaction), rolePanelOptionRole))
	if roleID == "" {
		return rolePanelDetailedCommandError("A role is required to identify the button.")
	}
	if err := c.configManager.DeleteRolePanelButton(ctx.GuildID, key, roleID); err != nil {
		switch {
		case errors.Is(err, files.ErrRolePanelNotFound):
			return rolePanelDetailedCommandError(fmt.Sprintf("Panel `%s` does not exist.", key))
		case errors.Is(err, files.ErrRolePanelButtonNotFound):
			return rolePanelDetailedCommandError(fmt.Sprintf("No button is bound to <@&%s> on panel `%s`.", roleID, key))
		default:
			return rolePanelDetailedCommandError(fmt.Sprintf("Failed to remove button: %v", err))
		}
	}
	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Button for <@&%s> was removed from panel `%s`.%s", roleID, key, syncNote),
	)
}

type rolePanelButtonListSubCommand struct {
	configManager *files.ConfigManager
}

func newRolePanelButtonListSubCommand(cm *files.ConfigManager) *rolePanelButtonListSubCommand {
	return &rolePanelButtonListSubCommand{configManager: cm}
}

// Name names.
func (c *rolePanelButtonListSubCommand) Name() string { return rolePanelSubButtonList }

// Description descriptions.
func (c *rolePanelButtonListSubCommand) Description() string {
	return "List the buttons configured on one panel"
}

// Options options.
func (c *rolePanelButtonListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{rolePanelKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *rolePanelButtonListSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelButtonListSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelButtonListSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelButtonListSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelButtonListSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelButtonListSubCommand.Handle: %w", err)
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("rolePanelButtonListSubCommand.Handle: %w", err)
	}
	if len(panel.Buttons) == 0 {
		return rolePanelConfigurationResponseBuilder(ctx.Session).Info(
			ctx.Interaction,
			fmt.Sprintf("Panel `%s` has no buttons configured yet.", panel.Key),
		)
	}

	var b strings.Builder
	var buttons []string
	b.WriteString(fmt.Sprintf("Buttons on panel `%s`:\n", panel.Key))
	for i, btn := range panel.Buttons {
		buttons = append(buttons, rolesvc.FormatRolePanelButtonForList(btn))
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, buttons[len(buttons)-1]))
	}
	return rolePanelConfigurationResponseBuilder(ctx.Session).Info(ctx.Interaction, strings.TrimSpace(b.String()))
}

// --- Subgroup: /roles field add|remove|list ---

type rolePanelFieldAddSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelFieldAddSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelFieldAddSubCommand {
	return &rolePanelFieldAddSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelFieldAddSubCommand) Name() string { return rolePanelSubFieldAdd }

// Description descriptions.
func (c *rolePanelFieldAddSubCommand) Description() string {
	return "Add a field to the embed of a role panel"
}

// Options options.
func (c *rolePanelFieldAddSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		rolePanelKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionFieldName, Description: "Field name/title", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: rolePanelOptionFieldValue, Description: "Field value/content", Required: true},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: rolePanelOptionFieldInline, Description: "Whether the field is inline (default: false)", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelFieldAddSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelFieldAddSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelFieldAddSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelFieldAddSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelFieldAddSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelFieldAddSubCommand.Handle: %w", err)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(rolePanelOptionFieldName)
	if err != nil {
		return rolePanelDetailedCommandError(err.Error())
	}
	value, err := extractor.StringRequired(rolePanelOptionFieldValue)
	if err != nil {
		return rolePanelDetailedCommandError(err.Error())
	}
	inline := false
	if extractor.HasOption(rolePanelOptionFieldInline) {
		inline = extractor.Bool(rolePanelOptionFieldInline)
	}

	field := files.RolePanelEmbedFieldConfig{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
	if err := c.configManager.AddRolePanelField(ctx.GuildID, key, field); err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to add field: %v", err))
	}
	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Field `%s` was added to panel `%s`.%s", name, key, syncNote),
	)
}

type rolePanelFieldRemoveSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelFieldRemoveSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelFieldRemoveSubCommand {
	return &rolePanelFieldRemoveSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelFieldRemoveSubCommand) Name() string { return rolePanelSubFieldRemove }

// Description descriptions.
func (c *rolePanelFieldRemoveSubCommand) Description() string {
	return "Remove a field from the embed of a role panel by its index"
}

// Options options.
func (c *rolePanelFieldRemoveSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		rolePanelKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionInteger, Name: rolePanelOptionFieldIndex, Description: "1-based index of the field to remove (use /roles field list to see indexes)", Required: true},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelFieldRemoveSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelFieldRemoveSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelFieldRemoveSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelFieldRemoveSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelFieldRemoveSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelFieldRemoveSubCommand.Handle: %w", err)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))
	if !extractor.HasOption(rolePanelOptionFieldIndex) {
		return rolePanelDetailedCommandError("A field index is required.")
	}
	// The user passes a 1-based index, but our API uses 0-based
	index := int(extractor.Int(rolePanelOptionFieldIndex)) - 1

	if err := c.configManager.RemoveRolePanelField(ctx.GuildID, key, index); err != nil {
		switch {
		case errors.Is(err, files.ErrRolePanelNotFound):
			return rolePanelDetailedCommandError(fmt.Sprintf("Panel `%s` does not exist.", key))
		default:
			return rolePanelDetailedCommandError(fmt.Sprintf("Failed to remove field: %v", err))
		}
	}
	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Field %d was removed from panel `%s`.%s", index+1, key, syncNote),
	)
}

type rolePanelFieldListSubCommand struct {
	configManager *files.ConfigManager
}

func newRolePanelFieldListSubCommand(cm *files.ConfigManager) *rolePanelFieldListSubCommand {
	return &rolePanelFieldListSubCommand{configManager: cm}
}

// Name names.
func (c *rolePanelFieldListSubCommand) Name() string { return rolePanelSubFieldList }

// Description descriptions.
func (c *rolePanelFieldListSubCommand) Description() string {
	return "List the fields configured on one panel"
}

// Options options.
func (c *rolePanelFieldListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{rolePanelKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *rolePanelFieldListSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelFieldListSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelFieldListSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelFieldListSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelFieldListSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelFieldListSubCommand.Handle: %w", err)
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("rolePanelFieldListSubCommand.Handle: %w", err)
	}
	if len(panel.Fields) == 0 {
		return rolePanelConfigurationResponseBuilder(ctx.Session).Info(
			ctx.Interaction,
			fmt.Sprintf("Panel `%s` has no fields configured.", panel.Key),
		)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Fields on panel `%s`:\n", panel.Key))
	for i, f := range panel.Fields {
		inlineStr := ""
		if f.Inline {
			inlineStr = " (inline)"
		}
		b.WriteString(fmt.Sprintf("%d. **%s**%s\n`%s`\n", i+1, f.Name, inlineStr, f.Value))
	}
	return rolePanelConfigurationResponseBuilder(ctx.Session).Info(ctx.Interaction, strings.TrimSpace(b.String()))
}

// --- /roles refresh, /roles unpost ---

type rolePanelRefreshSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelRefreshSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelRefreshSubCommand {
	return &rolePanelRefreshSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelRefreshSubCommand) Name() string { return rolePanelSubRefresh }

// Description descriptions.
func (c *rolePanelRefreshSubCommand) Description() string {
	return "Re-render all live postings of one panel to match the current configuration"
}

// Options options.
func (c *rolePanelRefreshSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{rolePanelKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *rolePanelRefreshSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelRefreshSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelRefreshSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelRefreshSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelRefreshSubCommand.Handle: %w", err)
	}
	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelRefreshSubCommand.Handle: %w", err)
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("rolePanelRefreshSubCommand.Handle: %w", err)
	}
	if len(panel.Postings) == 0 {
		return rolePanelConfigurationResponseBuilder(ctx.Session).Info(
			ctx.Interaction,
			fmt.Sprintf("Panel `%s` has no tracked postings yet. Use /roles post to publish it.", panel.Key),
		)
	}

	result := c.rolePanelService.Sync(
		ctx.Session,
		ctx.GuildID,
		panel.Key,
		panel.Postings,
		&panel,
	)
	summary := c.rolePanelService.FormatSyncSummary(result, "Refreshed")
	if summary == "" {
		summary = "No postings needed updating."
	}
	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(ctx.Interaction, summary)
}

type rolePanelUnpostSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelUnpostSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelUnpostSubCommand {
	return &rolePanelUnpostSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelUnpostSubCommand) Name() string { return rolePanelSubUnpost }

// Description descriptions.
func (c *rolePanelUnpostSubCommand) Description() string {
	return "Stop tracking one posted panel message and strip its buttons"
}

// Options options.
func (c *rolePanelUnpostSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        rolePanelOptionMessageID,
		Description: "Discord message ID of the posting to retire (right-click message → Copy ID)",
		Required:    true,
	}}
}

// RequiresGuild requires guild.
func (c *rolePanelUnpostSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelUnpostSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelUnpostSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelUnpostSubCommand) Handle(ctx *core.Context) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return fmt.Errorf("rolePanelUnpostSubCommand.Handle: %w", err)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))
	messageID, err := extractor.StringRequired(rolePanelOptionMessageID)
	if err != nil {
		return rolePanelDetailedCommandError("A message ID is required.")
	}

	panelKey, posting, lookupErr := c.configManager.FindRolePanelPosting(ctx.GuildID, messageID)
	if lookupErr != nil {
		if errors.Is(lookupErr, files.ErrRolePanelPostingNotFound) {
			return rolePanelDetailedCommandError(fmt.Sprintf("No tracked posting for message_id `%s`.", strings.TrimSpace(messageID)))
		}
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to look up posting: %v", lookupErr))
	}

	panel, fetchErr := c.configManager.RolePanel(ctx.GuildID, panelKey)
	if fetchErr != nil && !errors.Is(fetchErr, files.ErrRolePanelNotFound) {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to load panel `%s`: %v", panelKey, fetchErr))
	}

	emptyPanel := panel
	emptyPanel.Buttons = nil // Strip buttons

	result := c.rolePanelService.Sync(
		ctx.Session,
		ctx.GuildID,
		panelKey,
		[]files.RolePanelPostingConfig{posting},
		&emptyPanel,
	)

	// Sync's drop-on-missing path already removed the posting from
	// config when Discord returned 10003/10008. Otherwise, the
	// operator asked for the posting to be retired, so drop it now
	// regardless of whether the edit succeeded.
	if len(result.Dropped) == 0 {
		if err := c.configManager.RemoveRolePanelPosting(ctx.GuildID, panelKey, posting.MessageID); err != nil && !errors.Is(err, files.ErrRolePanelPostingNotFound) {
			return rolePanelDetailedCommandError(fmt.Sprintf("Failed to drop posting from config: %v", err))
		}
	}

	syncSummary := ""
	if summary := c.rolePanelService.FormatSyncSummary(result, "Stripped buttons from"); summary != "" {
		syncSummary = "\n" + summary
	}
	return rolePanelConfigurationResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Posting `%s` on panel `%s` was retired.%s", posting.MessageID, panelKey, syncSummary),
	)
}

type rolePanelToggleSubCommand struct {
	configManager *files.ConfigManager
}

func newRolePanelToggleSubCommand(cm *files.ConfigManager) *rolePanelToggleSubCommand {
	return &rolePanelToggleSubCommand{configManager: cm}
}

// Name names.
func (c *rolePanelToggleSubCommand) Name() string { return "toggle" }

// Description descriptions.
func (c *rolePanelToggleSubCommand) Description() string {
	return "Toggle interactive ephemeral messages for this server"
}

// Options options.
func (c *rolePanelToggleSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

// RequiresGuild requires guild.
func (c *rolePanelToggleSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelToggleSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *rolePanelToggleSubCommand) Handle(ctx *core.Context) error {
	var newValue bool
	_, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for i := range cfg.Guilds {
			if cfg.Guilds[i].GuildID == ctx.GuildID {
				cfg.Guilds[i].RuntimeConfig.DisableInteractiveEphemeral = !cfg.Guilds[i].RuntimeConfig.DisableInteractiveEphemeral
				newValue = cfg.Guilds[i].RuntimeConfig.DisableInteractiveEphemeral
				return nil
			}
		}
		return fmt.Errorf("guild config for %s not found in memory during save", ctx.GuildID)
	})

	if err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to update configuration: %v", err))
	}

	state := "enabled"
	if newValue {
		state = "disabled"
	}

	return core.NewResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Interactive ephemeral messages have been %s for this server.", state),
	)
}

// --- Helpers ---

// refreshRolePanelPostingsBestEffort re-renders the panel and applies
// the result to every tracked posting. Returns a formatted suffix
// (starting with a newline) when there is something to report;
// returns an empty string when the panel has no postings or the
// refresh was a quiet success.
func refreshRolePanelPostingsBestEffort(cm *files.ConfigManager, svc *rolesvc.RolePanelService, ctx *core.Context, key string) string {
	if cm == nil || svc == nil || ctx == nil {
		return ""
	}
	panel, err := cm.RolePanel(ctx.GuildID, key)
	if err != nil {
		return ""
	}
	if len(panel.Postings) == 0 {
		return ""
	}
	result := svc.Sync(
		ctx.Session,
		ctx.GuildID,
		panel.Key,
		panel.Postings,
		&panel,
	)
	if !result.HasIssues() && result.Edited == 0 {
		return ""
	}
	summary := svc.FormatSyncSummary(result, "Refreshed")
	if summary == "" {
		return ""
	}
	return "\n" + summary
}

func rolePanelKeyOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         rolePanelOptionKey,
		Description:  "Panel identifier (lowercase letters, digits, '-' or '_'); used to bind buttons together",
		Required:     required,
		Autocomplete: true,
	}
}

func handleRolePanelKeyAutocomplete(cm *files.ConfigManager, ctx *core.Context) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if ctx.GuildID == "" {
		return nil, nil
	}
	panels, err := cm.RolePanels(ctx.GuildID)
	if err != nil || len(panels) == 0 {
		return nil, nil
	}

	opts := core.GetSubCommandOptions(ctx.Interaction)
	focused, found := core.HasFocusedOption(opts)
	if !found {
		return nil, nil
	}

	input := strings.ToLower(fmt.Sprintf("%v", focused.Value))

	var choices []*discordgo.ApplicationCommandOptionChoice
	for _, p := range panels {
		if input == "" || strings.HasPrefix(p.Key, input) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  p.Key,
				Value: p.Key,
			})
		}
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices, nil
}

func rolePanelKeyFromOptions(i *discordgo.InteractionCreate) (string, error) {
	extractor := core.OptionList(core.GetSubCommandOptions(i))
	raw, err := extractor.StringRequired(rolePanelOptionKey)
	if err != nil {
		return "", rolePanelDetailedCommandError("A panel key is required.")
	}
	return raw, nil
}

func loadRolePanel(cm *files.ConfigManager, guildID, key string) (files.RolePanelConfig, error) {
	panel, err := cm.RolePanel(guildID, key)
	if err != nil {
		if errors.Is(err, files.ErrRolePanelNotFound) {
			return files.RolePanelConfig{}, rolePanelDetailedCommandError(fmt.Sprintf("Panel `%s` does not exist yet. Add a button with /roles button add to create it.", strings.TrimSpace(key)))
		}
		if errors.Is(err, files.ErrInvalidRolePanelInput) {
			return files.RolePanelConfig{}, rolePanelDetailedCommandError(err.Error())
		}
		return files.RolePanelConfig{}, rolePanelDetailedCommandError(fmt.Sprintf("Failed to load panel `%s`: %v", strings.TrimSpace(key), err))
	}
	return panel, nil
}

func roleOptionID(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option == nil || option.Name != name {
			continue
		}
		if value, ok := option.Value.(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func ensureRolePanelEnabled(ctx *core.Context) error {
	if ctx == nil || ctx.Config == nil {
		return rolePanelDetailedCommandError("Configuration is not available right now.")
	}
	cfg := ctx.Config.Config()
	if cfg == nil {
		return rolePanelDetailedCommandError("Configuration is not available right now.")
	}

	enabled, _ := cfg.ResolveFeatures(ctx.GuildID).Lookup(rolePanelFeatureID)
	log.DiscordLogger().Debug("Transient state inspection: Evaluated feature enablement for Role Panels",
		slog.Bool("toggle_enabled", enabled),
	)

	if !enabled {
		return core.NewMissingConfigError(ctx.GuildID, "Role Panels", "/roles")
	}
	return nil
}

func parseRolePanelWebhookURL(rawURL string) (webhookID, webhookToken string, err error) {
	rawURL = strings.TrimSpace(rawURL)
	matches := regexp.MustCompile(`(?:discordapp\.com|discord\.com)/api/webhooks/(\d+)/([a-zA-Z0-9_-]+)`).FindStringSubmatch(rawURL)
	if len(matches) != 3 {
		return "", "", errors.New("invalid Discord webhook URL format")
	}
	return matches[1], matches[2], nil
}

type rolePanelImportSubCommand struct {
	configManager    *files.ConfigManager
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelImportSubCommand(cm *files.ConfigManager, svc *rolesvc.RolePanelService) *rolePanelImportSubCommand {
	return &rolePanelImportSubCommand{configManager: cm, rolePanelService: svc}
}

// Name names.
func (c *rolePanelImportSubCommand) Name() string { return rolePanelSubImport }

// Description descriptions.
func (c *rolePanelImportSubCommand) Description() string {
	return "Import a JSON embed from a Pastebin URL"
}

// Options options.
func (c *rolePanelImportSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        rolePanelOptionKey,
			Description: "The unique key of the role panel to update",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        rolePanelOptionURL,
			Description: "The URL of the Pastebin/Discohook JSON",
			Required:    true,
		},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelImportSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelImportSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelImportSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelImportSubCommand) Handle(ctx *core.Context) error {
	builder := rolePanelConfigurationResponseBuilder(ctx.Session)
	guildID := ctx.GuildID

	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelImportSubCommand.Handle: %w", err)
	}

	var pasteURL string
	opts := core.GetSubCommandOptions(ctx.Interaction)
	for _, opt := range opts {
		if opt.Name == rolePanelOptionURL {
			pasteURL = strings.TrimSpace(fmt.Sprint(opt.Value))
		}
	}

	data, err := discord.FetchPastebinContent(context.Background(), pasteURL)
	if err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to fetch from pastebin: %v", err))
	}

	discohookEmbed, err := files.ParseAndValidateDiscohookJSON(data)
	if err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Invalid embed JSON: %v", err))
	}

	newRP := files.ToRolePanelConfig(discohookEmbed, key)
	if err := c.configManager.SetRolePanelEmbed(guildID, key, newRP); err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to save imported role panel embed: %v", err))
	}

	return builder.Success(ctx.Interaction, fmt.Sprintf("Successfully imported JSON into role panel `%s`.", key))
}

type rolePanelExportSubCommand struct {
	configManager *files.ConfigManager
}

func newRolePanelExportSubCommand(cm *files.ConfigManager) *rolePanelExportSubCommand {
	return &rolePanelExportSubCommand{configManager: cm}
}

// Name names.
func (c *rolePanelExportSubCommand) Name() string { return rolePanelSubExport }

// Description descriptions.
func (c *rolePanelExportSubCommand) Description() string {
	return "Export a role panel embed to a Pastebin provider"
}

// Options options.
func (c *rolePanelExportSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        rolePanelOptionKey,
			Description: "The unique key of the role panel to export",
			Required:    true,
		},
	}
}

// RequiresGuild requires guild.
func (c *rolePanelExportSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *rolePanelExportSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *rolePanelExportSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == rolePanelOptionKey {
		return handleRolePanelKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *rolePanelExportSubCommand) Handle(ctx *core.Context) error {
	builder := rolePanelConfigurationResponseBuilder(ctx.Session)
	guildID := ctx.GuildID

	key, err := rolePanelKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("rolePanelExportSubCommand.Handle: %w", err)
	}

	rp, err := loadRolePanel(c.configManager, guildID, key)
	if err != nil {
		return fmt.Errorf("rolePanelExportSubCommand.Handle: %w", err)
	}

	discohookJSON := files.FromRolePanelConfig(rp)
	data, err := json.MarshalIndent(discohookJSON, "", "  ")
	if err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to format JSON: %v", err))
	}

	ownerID := ""
	if g, err := ctx.Session.State.Guild(ctx.GuildID); err == nil {
		ownerID = g.OwnerID
	}

	url, err := discord.UploadExportedContent(context.Background(), ctx.Interaction.Member, ownerID, c.configManager, data)
	if err != nil {
		return rolePanelDetailedCommandError(fmt.Sprintf("Failed to upload: %v", err))
	}

	return builder.Success(ctx.Interaction, fmt.Sprintf("Role panel `%s` successfully exported: <%s>", key, url))
}
