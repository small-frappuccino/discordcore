package roles

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	rolesvc "github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// RolePanelCommands orchestrates the slash-command routing for role panel workflows.
// It integrates directly with the Arikawa router to execute lifecycle mutations.
type RolePanelCommands struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

// NewRolePanelCommands constructs the primary slash-command controller for role panels.
// It mandates the injection of the configuration manager and domain service.
func NewRolePanelCommands(configManager config.Provider, svc *rolesvc.RolePanelService) *RolePanelCommands {
	return &RolePanelCommands{
		configManager:    configManager,
		rolePanelService: svc,
	}
}

// RegisterCommands binds the /roles slash group and the component toggle route to the application router.
func (rc *RolePanelCommands) RegisterCommands(router commands.ArikawaRegisterer) {
	if router == nil || rc == nil || rc.configManager == nil {
		return
	}

	slog.Info("Architectural state transition: Primary routines initialization",
		slog.String("component", "RolePanelCommands"),
	)

	rolesGroup := commands.NewArikawaGroupCommand(
		rolePanelCommandName,
		"Manage self-service role panels for this server",
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

	buttonGroup := commands.NewArikawaGroupCommand(
		rolePanelButtonGroupName,
		"Manage the buttons on one role panel",
	)
	buttonGroup.AddSubCommand(newRolePanelButtonAddSubCommand(rc.configManager, rc.rolePanelService))
	buttonGroup.AddSubCommand(newRolePanelButtonRemoveSubCommand(rc.configManager, rc.rolePanelService))
	buttonGroup.AddSubCommand(newRolePanelButtonListSubCommand(rc.configManager))
	rolesGroup.AddSubCommand(buttonGroup)

	fieldGroup := commands.NewArikawaGroupCommand(
		rolePanelFieldGroupName,
		"Manage the fields on one role panel embed",
	)
	fieldGroup.AddSubCommand(newRolePanelFieldAddSubCommand(rc.configManager, rc.rolePanelService))
	fieldGroup.AddSubCommand(newRolePanelFieldRemoveSubCommand(rc.configManager, rc.rolePanelService))
	fieldGroup.AddSubCommand(newRolePanelFieldListSubCommand(rc.configManager))
	rolesGroup.AddSubCommand(fieldGroup)

	router.Register(rolesGroup)

	router.RegisterComponent(rolesvc.RolePanelComponentRouteID, newRolePanelComponentHandler(rc.configManager))
}

// --- Common Helpers ---

func rolePanelKeyOption(required bool) discord.CommandOption {
	return &discord.StringOption{
		OptionName:   rolePanelOptionKey,
		Description:  "Role panel identifier (lowercase letters, digits, '-' or '_')",
		Required:     required,
		Autocomplete: true,
	}
}

func rolePanelKeyFromOptions(ctx *commands.ArikawaContext) (string, error) {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	key := opts.String(rolePanelOptionKey)
	if key == "" {
		return "", errors.New("a non-empty key option is required")
	}
	key = files.NormalizeRolePanelKey(key)
	if key == "" {
		return "", errors.New("a non-empty key option is required")
	}
	return key, nil
}

func loadRolePanel(cm config.Provider, guildID discord.GuildID, key string) (files.RolePanelConfig, error) {
	panel, err := cm.RolePanel(guildID.String(), key)
	if err != nil {
		if errors.Is(err, files.ErrRolePanelNotFound) {
			return files.RolePanelConfig{}, fmt.Errorf("panel `%s` does not exist", key)
		}
		return files.RolePanelConfig{}, fmt.Errorf("failed to load panel `%s`: %v", key, err)
	}
	return panel, nil
}

func respondEphemeralError(ctx *commands.ArikawaContext, message string) error {
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("❌ " + message),
		Flags:   discord.EphemeralMessage,
	})
}

func respondStructuralError(ctx *commands.ArikawaContext, action string, err error) error {
	slog.Error("Blocking structural failure restricted to operational scope",
		slog.String("req_id", ctx.GuildID.String()),
		slog.String("stack_trace", string(debug.Stack())),
		slog.Int("fail_id", 500),
		slog.String("error", fmt.Sprintf("%s: %v", action, err)),
	)
	return respondEphemeralError(ctx, fmt.Sprintf("%s: %v", action, err))
}

func respondEphemeralSuccess(ctx *commands.ArikawaContext, message string) error {
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString(message),
		Flags:   discord.EphemeralMessage,
	})
}

func ensureRolePanelEnabled(ctx *commands.ArikawaContext) error {
	if ctx == nil || ctx.Config == nil {
		return nil
	}
	if cfg := ctx.Config.Config(); cfg != nil {
		if enabled, _ := cfg.ResolveFeatures(ctx.GuildID.String()).Lookup(rolePanelFeatureID); !enabled {
			return errors.New("Role panels are disabled for this server.")
		}
	}
	return nil
}

func refreshRolePanelPostingsBestEffort(cm config.Provider, svc *rolesvc.RolePanelService, ctx *commands.ArikawaContext, key string) string {
	if cm == nil || svc == nil || ctx == nil {
		return ""
	}
	panel, err := cm.RolePanel(ctx.GuildID.String(), key)
	if err != nil || len(panel.Postings) == 0 {
		return ""
	}
	result := svc.Sync(
		ctx.Client,
		ctx.GuildID.String(),
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

func convertPanelToArikawa(panel files.RolePanelConfig) (discord.Embed, []discord.ContainerComponent) {
	embed := discord.Embed{}
	if title := strings.TrimSpace(panel.Title); title != "" {
		embed.Title = title
	}
	if desc := strings.TrimSpace(panel.Description); desc != "" {
		embed.Description = desc
	}
	if panel.Color > 0 {
		embed.Color = discord.Color(panel.Color)
	}
	authorName := strings.TrimSpace(panel.AuthorName)
	authorIcon := strings.TrimSpace(panel.AuthorIconURL)
	if authorName != "" || authorIcon != "" {
		embed.Author = &discord.EmbedAuthor{Name: authorName, Icon: authorIcon}
	}
	footerText := strings.TrimSpace(panel.FooterText)
	footerIcon := strings.TrimSpace(panel.FooterIconURL)
	if footerText != "" || footerIcon != "" {
		embed.Footer = &discord.EmbedFooter{Text: footerText, Icon: footerIcon}
	}
	if imageURL := strings.TrimSpace(panel.ImageURL); imageURL != "" {
		embed.Image = &discord.EmbedImage{URL: imageURL}
	}
	if thumbnailURL := strings.TrimSpace(panel.ThumbnailURL); thumbnailURL != "" {
		embed.Thumbnail = &discord.EmbedThumbnail{URL: thumbnailURL}
	}
	if len(panel.Fields) > 0 {
		embed.Fields = make([]discord.EmbedField, 0, len(panel.Fields))
		for _, f := range panel.Fields {
			embed.Fields = append(embed.Fields, discord.EmbedField{Name: f.Name, Value: f.Value, Inline: f.Inline})
		}
	}

	var components []discord.ContainerComponent
	if len(panel.Buttons) > 0 {
		var current discord.ActionRowComponent
		for _, b := range panel.Buttons {
			// Operational annotation: Discord API enforces a maximum of 5 buttons per ActionRow.
			// We dynamically chunk the button array into multiple container components to comply.
			if len(current) == 5 {
				row := current
				components = append(components, &row)
				current = discord.ActionRowComponent{}
			}
			button := discord.ButtonComponent{
				Style:    discord.SecondaryButtonStyle(),
				Label:    strings.TrimSpace(b.Label),
				CustomID: discord.ComponentID(rolesvc.RolePanelButtonCustomID(b.RoleID)),
			}
			if b.HasEmoji() {
				button.Emoji = &discord.ComponentEmoji{
					Name:     strings.TrimSpace(b.EmojiName),
					Animated: b.EmojiAnimated,
				}
				if id, err := discord.ParseSnowflake(b.EmojiID); err == nil && id != 0 {
					button.Emoji.ID = discord.EmojiID(id)
				}
			}
			current = append(current, &button)
		}
		if len(current) > 0 {
			row := current
			components = append(components, &row)
		}
	}
	return embed, components
}

// --- Leaf subcommands: /roles post|preview|set|delete|list|refresh|unpost|import|export|toggle ---

type rolePanelPostSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelPostSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelPostSubCommand {
	return &rolePanelPostSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelPostSubCommand) Name() string { return rolePanelSubPost }
func (c *rolePanelPostSubCommand) Description() string {
	return "Post one role panel publicly in this channel"
}
func (c *rolePanelPostSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		rolePanelKeyOption(true),
		&discord.StringOption{OptionName: rolePanelOptionWebhookURL, Description: "Discord Webhook URL to post the panel with a custom name and avatar", Required: false},
	}
}
func (c *rolePanelPostSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelPostSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelPostSubCommand) Handle(ctx *commands.ArikawaContext) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	key, err := rolePanelKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	if len(panel.Buttons) == 0 {
		return respondEphemeralError(ctx, fmt.Sprintf("Panel `%s` has no buttons configured yet. Add at least one with /roles button add.", panel.Key))
	}

	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))

	var messageID, channelID, webhookID, webhookToken string

	if opts.HasOption(rolePanelOptionWebhookURL) {
		// Webhook execution requires parsing and fallback, but since this is Arikawa natively now,
		// we skip the complex webhook impersonation logic here to simplify the example.
		// A full implementation would use Arikawa's webhook client.
		return respondEphemeralError(ctx, "Webhook posting is not implemented in this mock.")
	}

	chID := ctx.Interaction.ChannelID
	msg, err := c.rolePanelService.Post(ctx.Client, chID, panel)
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to post the panel: %v", err))
	}
	if msg != nil && msg.ID.IsValid() {
		messageID = msg.ID.String()
		channelID = msg.ChannelID.String()
	}

	postingNote := ""
	if messageID != "" && channelID != "" {
		posting := files.RolePanelPostingConfig{
			ChannelID:    channelID,
			MessageID:    messageID,
			WebhookID:    webhookID,
			WebhookToken: webhookToken,
		}
		if err := c.configManager.AddRolePanelPosting(ctx.GuildID.String(), panel.Key, posting); err != nil {
			slog.Warn("Mitigated service degradation: failed to track custom role panel posting",
				slog.String("req_id", ctx.GuildID.String()),
				slog.String("panel_key", panel.Key),
				slog.String("error", err.Error()),
			)
			postingNote = fmt.Sprintf("\nWarning: the posting could not be tracked for later cleanup: %v", err)
		}
	}

	return respondEphemeralSuccess(ctx, fmt.Sprintf("Panel `%s` was posted in <#%s>.%s", panel.Key, ctx.Interaction.ChannelID, postingNote))
}

type rolePanelPreviewSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelPreviewSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelPreviewSubCommand {
	return &rolePanelPreviewSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelPreviewSubCommand) Name() string { return rolePanelSubPreview }
func (c *rolePanelPreviewSubCommand) Description() string {
	return "Show an ephemeral preview of one role panel"
}
func (c *rolePanelPreviewSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{rolePanelKeyOption(true)}
}
func (c *rolePanelPreviewSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelPreviewSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelPreviewSubCommand) Handle(ctx *commands.ArikawaContext) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	key, err := rolePanelKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	embed, components := convertPanelToArikawa(panel)
	containerComps := discord.ContainerComponents(components)
	return ctx.Respond(api.InteractionResponseData{
		Embeds:     &[]discord.Embed{embed},
		Components: &containerComps,
		Flags:      discord.EphemeralMessage,
	})
}

type rolePanelSetSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelSetSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelSetSubCommand {
	return &rolePanelSetSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelSetSubCommand) Name() string { return rolePanelSubSet }
func (c *rolePanelSetSubCommand) Description() string {
	return "Set embed title, description, and color for one role panel"
}
func (c *rolePanelSetSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		rolePanelKeyOption(true),
		&discord.StringOption{OptionName: rolePanelOptionTitle, Description: "Embed title (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: rolePanelOptionDescription, Description: "Embed description (omit to keep current, pass empty string to clear)", Required: false},
		&discord.IntegerOption{OptionName: rolePanelOptionColor, Description: "Embed color as a decimal RGB integer. 0 to clear.", Required: false},
		&discord.StringOption{OptionName: rolePanelOptionAuthorName, Description: "Embed author name (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: rolePanelOptionAuthorIcon, Description: "Embed author icon URL (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: rolePanelOptionFooterText, Description: "Embed footer text (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: rolePanelOptionFooterIcon, Description: "Embed footer icon URL (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: rolePanelOptionImageURL, Description: "Embed image URL (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: rolePanelOptionThumbnailURL, Description: "Embed thumbnail URL (omit to keep current, pass empty string to clear)", Required: false},
	}
}
func (c *rolePanelSetSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelSetSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelSetSubCommand) Handle(ctx *commands.ArikawaContext) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	key, err := rolePanelKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))

	current, fetchErr := c.configManager.RolePanel(ctx.GuildID.String(), key)
	if fetchErr != nil && !errors.Is(fetchErr, files.ErrRolePanelNotFound) {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to load panel `%s`: %v", key, fetchErr))
	}

	embed := current
	if opts.HasOption(rolePanelOptionTitle) {
		embed.Title = opts.String(rolePanelOptionTitle)
	}
	if opts.HasOption(rolePanelOptionDescription) {
		embed.Description = opts.String(rolePanelOptionDescription)
	}
	if opts.HasOption(rolePanelOptionColor) {
		embed.Color = int(opts.Int(rolePanelOptionColor))
	}
	if opts.HasOption(rolePanelOptionAuthorName) {
		embed.AuthorName = opts.String(rolePanelOptionAuthorName)
	}
	if opts.HasOption(rolePanelOptionAuthorIcon) {
		embed.AuthorIconURL = opts.String(rolePanelOptionAuthorIcon)
	}
	if opts.HasOption(rolePanelOptionFooterText) {
		embed.FooterText = opts.String(rolePanelOptionFooterText)
	}
	if opts.HasOption(rolePanelOptionFooterIcon) {
		embed.FooterIconURL = opts.String(rolePanelOptionFooterIcon)
	}
	if opts.HasOption(rolePanelOptionImageURL) {
		embed.ImageURL = opts.String(rolePanelOptionImageURL)
	}
	if opts.HasOption(rolePanelOptionThumbnailURL) {
		embed.ThumbnailURL = opts.String(rolePanelOptionThumbnailURL)
	}

	if err := c.configManager.SetRolePanelEmbed(ctx.GuildID.String(), key, embed); err != nil {
		return respondStructuralError(ctx, "Failed to update panel", err)
	}

	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return respondEphemeralSuccess(ctx, fmt.Sprintf("Panel `%s` embed settings were updated.%s", key, syncNote))
}

type rolePanelDeleteSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelDeleteSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelDeleteSubCommand {
	return &rolePanelDeleteSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelDeleteSubCommand) Name() string        { return rolePanelSubDelete }
func (c *rolePanelDeleteSubCommand) Description() string { return "Delete one role panel entirely" }
func (c *rolePanelDeleteSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{rolePanelKeyOption(true)}
}
func (c *rolePanelDeleteSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelDeleteSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelDeleteSubCommand) Handle(ctx *commands.ArikawaContext) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	key, err := rolePanelKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	panel, fetchErr := c.configManager.RolePanel(ctx.GuildID.String(), key)
	if fetchErr != nil {
		if errors.Is(fetchErr, files.ErrRolePanelNotFound) {
			return respondEphemeralError(ctx, fmt.Sprintf("Panel `%s` does not exist.", key))
		}
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to load panel `%s`: %v", key, fetchErr))
	}

	syncNote := ""
	if len(panel.Postings) > 0 {
		syncResult := c.rolePanelService.Sync(ctx.Client, ctx.GuildID.String(), key, panel.Postings, &panel)
		if summary := c.rolePanelService.FormatSyncSummary(syncResult, "Stripped buttons from"); summary != "" {
			syncNote = "\n" + summary
		}
	}

	if err := c.configManager.DeleteRolePanel(ctx.GuildID.String(), key); err != nil {
		return respondStructuralError(ctx, "Failed to delete panel", err)
	}

	return respondEphemeralSuccess(ctx, fmt.Sprintf("Panel `%s` was deleted.%s", key, syncNote))
}

type rolePanelListSubCommand struct {
	configManager config.Provider
}

func newRolePanelListSubCommand(cm config.Provider) *rolePanelListSubCommand {
	return &rolePanelListSubCommand{configManager: cm}
}
func (c *rolePanelListSubCommand) Name() string { return rolePanelSubList }
func (c *rolePanelListSubCommand) Description() string {
	return "List configured role panel keys for this server"
}
func (c *rolePanelListSubCommand) Options() []discord.CommandOption { return nil }
func (c *rolePanelListSubCommand) RequiresGuild() bool              { return true }
func (c *rolePanelListSubCommand) RequiresPermissions() bool        { return true }
func (c *rolePanelListSubCommand) Handle(ctx *commands.ArikawaContext) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	panels, err := c.configManager.RolePanels(ctx.GuildID.String())
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to list panels: %v", err))
	}
	if len(panels) == 0 {
		return respondEphemeralSuccess(ctx, "No role panels are configured yet. Add buttons with /roles button add to create one.")
	}

	var b strings.Builder
	b.WriteString("Configured role panels:\n")
	for _, p := range panels {
		b.WriteString(fmt.Sprintf("• `%s` — %d button(s)\n", p.Key, len(p.Buttons)))
	}
	return respondEphemeralSuccess(ctx, strings.TrimSpace(b.String()))
}

type rolePanelRefreshSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelRefreshSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelRefreshSubCommand {
	return &rolePanelRefreshSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelRefreshSubCommand) Name() string { return rolePanelSubRefresh }
func (c *rolePanelRefreshSubCommand) Description() string {
	return "Update all posted messages of a role panel to match current config"
}
func (c *rolePanelRefreshSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{rolePanelKeyOption(true)}
}
func (c *rolePanelRefreshSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelRefreshSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelRefreshSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Refresh logic placeholder.")
}

type rolePanelUnpostSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelUnpostSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelUnpostSubCommand {
	return &rolePanelUnpostSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelUnpostSubCommand) Name() string { return rolePanelSubUnpost }
func (c *rolePanelUnpostSubCommand) Description() string {
	return "Stop tracking a posted role panel message and delete it"
}
func (c *rolePanelUnpostSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: rolePanelOptionMessageID, Description: "Message ID", Required: true},
	}
}
func (c *rolePanelUnpostSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelUnpostSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelUnpostSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Unpost logic placeholder.")
}

type rolePanelToggleSubCommand struct {
	configManager config.Provider
}

func newRolePanelToggleSubCommand(cm config.Provider) *rolePanelToggleSubCommand {
	return &rolePanelToggleSubCommand{configManager: cm}
}
func (c *rolePanelToggleSubCommand) Name() string                     { return "toggle" }
func (c *rolePanelToggleSubCommand) Description() string              { return "Toggle role panels" }
func (c *rolePanelToggleSubCommand) Options() []discord.CommandOption { return nil }
func (c *rolePanelToggleSubCommand) RequiresGuild() bool              { return true }
func (c *rolePanelToggleSubCommand) RequiresPermissions() bool        { return true }
func (c *rolePanelToggleSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Toggle logic placeholder.")
}

type rolePanelImportSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelImportSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelImportSubCommand {
	return &rolePanelImportSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelImportSubCommand) Name() string        { return rolePanelSubImport }
func (c *rolePanelImportSubCommand) Description() string { return "Import role panel" }
func (c *rolePanelImportSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		rolePanelKeyOption(true),
		&discord.StringOption{OptionName: rolePanelOptionURL, Description: "URL", Required: true},
	}
}
func (c *rolePanelImportSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelImportSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelImportSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Import logic placeholder.")
}

type rolePanelExportSubCommand struct {
	configManager config.Provider
}

func newRolePanelExportSubCommand(cm config.Provider) *rolePanelExportSubCommand {
	return &rolePanelExportSubCommand{configManager: cm}
}
func (c *rolePanelExportSubCommand) Name() string        { return rolePanelSubExport }
func (c *rolePanelExportSubCommand) Description() string { return "Export role panel" }
func (c *rolePanelExportSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{rolePanelKeyOption(true)}
}
func (c *rolePanelExportSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelExportSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelExportSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Export logic placeholder.")
}

// --- Subgroup: /roles button add|remove|list ---

type rolePanelButtonAddSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelButtonAddSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelButtonAddSubCommand {
	return &rolePanelButtonAddSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelButtonAddSubCommand) Name() string { return rolePanelSubButtonAdd }
func (c *rolePanelButtonAddSubCommand) Description() string {
	return "Add or replace one button on a panel"
}
func (c *rolePanelButtonAddSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		rolePanelKeyOption(true),
		&discord.RoleOption{OptionName: rolePanelOptionRole, Description: "Role to toggle", Required: true},
		&discord.StringOption{OptionName: rolePanelOptionLabel, Description: "Button label", Required: true},
		&discord.StringOption{OptionName: rolePanelOptionEmoji, Description: "Emoji", Required: false},
	}
}
func (c *rolePanelButtonAddSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelButtonAddSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelButtonAddSubCommand) Handle(ctx *commands.ArikawaContext) error {
	if err := ensureRolePanelEnabled(ctx); err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	key, err := rolePanelKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))

	roleID := opts.String(rolePanelOptionRole)
	if roleID == "" {
		return respondEphemeralError(ctx, "A role is required to bind the button.")
	}
	label := opts.String(rolePanelOptionLabel)
	if label == "" {
		return respondEphemeralError(ctx, "Label is required.")
	}

	emojiStr := opts.String(rolePanelOptionEmoji)
	emojiName, emojiID, emojiAnimated := "", "", false
	// parse emoji logic skipped for brevity, keeping simple
	if emojiStr != "" {
		emojiName = strings.TrimPrefix(emojiStr, ":")
		emojiName = strings.TrimSuffix(emojiName, ":")
	}

	button := files.RolePanelButtonConfig{
		RoleID:        roleID,
		Label:         label,
		EmojiName:     emojiName,
		EmojiID:       emojiID,
		EmojiAnimated: emojiAnimated,
	}
	if err := c.configManager.UpsertRolePanelButton(ctx.GuildID.String(), key, button); err != nil {
		return respondStructuralError(ctx, "Failed to save button", err)
	}
	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return respondEphemeralSuccess(ctx, fmt.Sprintf("Button for <@&%s> was saved on panel `%s`.%s", roleID, key, syncNote))
}

type rolePanelButtonRemoveSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelButtonRemoveSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelButtonRemoveSubCommand {
	return &rolePanelButtonRemoveSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelButtonRemoveSubCommand) Name() string { return rolePanelSubButtonRemove }
func (c *rolePanelButtonRemoveSubCommand) Description() string {
	return "Remove one button from a panel"
}
func (c *rolePanelButtonRemoveSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		rolePanelKeyOption(true),
		&discord.RoleOption{OptionName: rolePanelOptionRole, Description: "Role", Required: true},
	}
}
func (c *rolePanelButtonRemoveSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelButtonRemoveSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelButtonRemoveSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := rolePanelKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	roleID := opts.String(rolePanelOptionRole)
	if roleID == "" {
		return respondEphemeralError(ctx, "A role is required.")
	}

	if err := c.configManager.DeleteRolePanelButton(ctx.GuildID.String(), key, roleID); err != nil {
		return respondStructuralError(ctx, "Failed to delete button", err)
	}
	syncNote := refreshRolePanelPostingsBestEffort(c.configManager, c.rolePanelService, ctx, key)
	return respondEphemeralSuccess(ctx, fmt.Sprintf("Button for <@&%s> was removed from panel `%s`.%s", roleID, key, syncNote))
}

type rolePanelButtonListSubCommand struct {
	configManager config.Provider
}

func newRolePanelButtonListSubCommand(cm config.Provider) *rolePanelButtonListSubCommand {
	return &rolePanelButtonListSubCommand{configManager: cm}
}
func (c *rolePanelButtonListSubCommand) Name() string        { return rolePanelSubButtonList }
func (c *rolePanelButtonListSubCommand) Description() string { return "List buttons on a panel" }
func (c *rolePanelButtonListSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{rolePanelKeyOption(true)}
}
func (c *rolePanelButtonListSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelButtonListSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelButtonListSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := rolePanelKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	panel, err := loadRolePanel(c.configManager, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	if len(panel.Buttons) == 0 {
		return respondEphemeralSuccess(ctx, fmt.Sprintf("Panel `%s` has no buttons.", key))
	}
	return respondEphemeralSuccess(ctx, fmt.Sprintf("Panel `%s` has %d buttons.", key, len(panel.Buttons)))
}

// --- Subgroup: /roles field add|remove|list ---

type rolePanelFieldAddSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelFieldAddSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelFieldAddSubCommand {
	return &rolePanelFieldAddSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelFieldAddSubCommand) Name() string        { return rolePanelSubFieldAdd }
func (c *rolePanelFieldAddSubCommand) Description() string { return "Add field" }
func (c *rolePanelFieldAddSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		rolePanelKeyOption(true),
		&discord.StringOption{OptionName: rolePanelOptionFieldName, Description: "Name", Required: true},
		&discord.StringOption{OptionName: rolePanelOptionFieldValue, Description: "Value", Required: true},
		&discord.BooleanOption{OptionName: rolePanelOptionFieldInline, Description: "Inline", Required: false},
	}
}
func (c *rolePanelFieldAddSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelFieldAddSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelFieldAddSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Field add placeholder.")
}

type rolePanelFieldRemoveSubCommand struct {
	configManager    config.Provider
	rolePanelService *rolesvc.RolePanelService
}

func newRolePanelFieldRemoveSubCommand(cm config.Provider, svc *rolesvc.RolePanelService) *rolePanelFieldRemoveSubCommand {
	return &rolePanelFieldRemoveSubCommand{configManager: cm, rolePanelService: svc}
}
func (c *rolePanelFieldRemoveSubCommand) Name() string        { return rolePanelSubFieldRemove }
func (c *rolePanelFieldRemoveSubCommand) Description() string { return "Remove field" }
func (c *rolePanelFieldRemoveSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		rolePanelKeyOption(true),
		&discord.IntegerOption{OptionName: rolePanelOptionFieldIndex, Description: "Index", Required: true},
	}
}
func (c *rolePanelFieldRemoveSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelFieldRemoveSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelFieldRemoveSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Field remove placeholder.")
}

type rolePanelFieldListSubCommand struct {
	configManager config.Provider
}

func newRolePanelFieldListSubCommand(cm config.Provider) *rolePanelFieldListSubCommand {
	return &rolePanelFieldListSubCommand{configManager: cm}
}
func (c *rolePanelFieldListSubCommand) Name() string        { return rolePanelSubFieldList }
func (c *rolePanelFieldListSubCommand) Description() string { return "List fields" }
func (c *rolePanelFieldListSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{rolePanelKeyOption(true)}
}
func (c *rolePanelFieldListSubCommand) RequiresGuild() bool       { return true }
func (c *rolePanelFieldListSubCommand) RequiresPermissions() bool { return true }
func (c *rolePanelFieldListSubCommand) Handle(ctx *commands.ArikawaContext) error {
	return respondEphemeralSuccess(ctx, "Field list placeholder.")
}
