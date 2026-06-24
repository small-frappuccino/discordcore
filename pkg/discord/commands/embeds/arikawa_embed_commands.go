package embeds

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/config"
	localdiscord "github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	embedsvc "github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	embedCommandName    = "embed"
	embedSubPost        = "post"
	embedSubPreview     = "preview"
	embedSubSet         = "set"
	embedSubDelete      = "delete"
	embedSubList        = "list"
	embedSubRefresh     = "refresh"
	embedSubUnpost      = "unpost"
	embedSubImport      = "import"
	embedSubExport      = "export"
	embedFieldGroupName = "field"
	embedSubFieldAdd    = "add"
	embedSubFieldRemove = "remove"
	embedSubFieldList   = "list"

	embedOptionKey          = "key"
	embedOptionTitle        = "title"
	embedOptionDescription  = "description"
	embedOptionColor        = "color"
	embedOptionMessageID    = "message_id"
	embedOptionAuthorName   = "author_name"
	embedOptionAuthorIcon   = "author_icon_url"
	embedOptionFooterText   = "footer_text"
	embedOptionFooterIcon   = "footer_icon_url"
	embedOptionImageURL     = "image_url"
	embedOptionThumbnailURL = "thumbnail_url"
	embedOptionFieldName    = "name"
	embedOptionFieldValue   = "value"
	embedOptionFieldInline  = "inline"
	embedOptionFieldIndex   = "index"
	embedOptionChannel      = "channel"
	embedOptionURL          = "url"
)

// EmbedCommands orchestrates the slash-command routing for custom embed workflows.
// It integrates directly with the Arikawa router to execute lifecycle mutations.
type EmbedCommands struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

// NewEmbedCommands constructs the primary slash-command controller for embeds.
// It mandates the injection of the configuration manager and domain service.
func NewEmbedCommands(configManager config.Provider, embedService *embedsvc.EmbedService) *EmbedCommands {
	return &EmbedCommands{
		configManager: configManager,
		embedService:  embedService,
	}
}

// RegisterCommands binds the /embed slash group and its nested execution trees to the application router.
func (ec *EmbedCommands) RegisterCommands(router commands.ArikawaRegisterer) {
	if router == nil || ec == nil || ec.configManager == nil {
		return
	}

	slog.Info("Architectural state transition: Primary routines initialization",
		slog.String("component", "EmbedCommands"),
	)

	embedGroup := commands.NewArikawaGroupCommand(
		embedCommandName,
		"Manage custom embeds for this server",
	)
	embedGroup.AddSubCommand(newEmbedPostSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedPreviewSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedSetSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedDeleteSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedListSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedRefreshSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedUnpostSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedImportSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(newEmbedExportSubCommand(ec.configManager, ec.embedService))

	fieldGroup := commands.NewArikawaGroupCommand(
		embedFieldGroupName,
		"Manage the fields on a custom embed",
	)
	fieldGroup.AddSubCommand(newEmbedFieldAddSubCommand(ec.configManager, ec.embedService))
	fieldGroup.AddSubCommand(newEmbedFieldRemoveSubCommand(ec.configManager, ec.embedService))
	fieldGroup.AddSubCommand(newEmbedFieldListSubCommand(ec.configManager, ec.embedService))
	embedGroup.AddSubCommand(fieldGroup)

	router.Register(embedGroup)
}

// --- Common Helpers ---

func embedKeyOption(required bool) discord.CommandOption {
	return &discord.StringOption{
		OptionName:   embedOptionKey,
		Description:  "Embed identifier (lowercase letters, digits, '-' or '_')",
		Required:     required,
		Autocomplete: true,
	}
}

func embedKeyFromOptions(ctx *commands.ArikawaContext) (string, error) {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	key := opts.String(embedOptionKey)
	if key == "" {
		return "", errors.New("a non-empty key option is required")
	}
	key = embedsvc.NormalizeCustomEmbedKey(key)
	if key == "" {
		return "", errors.New("a non-empty key option is required")
	}
	return key, nil
}

func loadCustomEmbed(svc *embedsvc.EmbedService, guildID discord.GuildID, key string) (files.CustomEmbedConfig, error) {
	ce, err := svc.CustomEmbed(guildID.String(), key)
	if err != nil {
		if errors.Is(err, embedsvc.ErrCustomEmbedNotFound) {
			return files.CustomEmbedConfig{}, fmt.Errorf("embed `%s` does not exist", key)
		}
		return files.CustomEmbedConfig{}, fmt.Errorf("failed to load embed `%s`: %v", key, err)
	}
	return ce, nil
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

func refreshCustomEmbedPostingsBestEffort(cm config.Provider, svc *embedsvc.EmbedService, ctx *commands.ArikawaContext, key string) string {
	if cm == nil || svc == nil || ctx == nil {
		return ""
	}
	ce, err := svc.CustomEmbed(ctx.GuildID.String(), key)
	if err != nil || len(ce.Postings) == 0 {
		return ""
	}
	embed := svc.Render(ce)
	// Operational annotation: The following sync relies on a best-effort mitigation.
	// We execute it synchronously during the command response lifecycle, but avoid
	// failing the interaction if the background refresh encounters partial state drops.
	result := svc.Sync(
		ctx.Client,
		ctx.GuildID.String(),
		ce.Key,
		ce.Postings,
		&embed,
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

// --- Subcommands ---

type embedPostSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedPostSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedPostSubCommand {
	return &embedPostSubCommand{configManager: cm, embedService: svc}
}

func (c *embedPostSubCommand) Name() string { return embedSubPost }
func (c *embedPostSubCommand) Description() string {
	return "Post a custom embed publicly in a channel"
}
func (c *embedPostSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		embedKeyOption(true),
		&discord.ChannelOption{
			OptionName:  embedOptionChannel,
			Description: "Target channel (defaults to current channel)",
			Required:    false,
		},
	}
}
func (c *embedPostSubCommand) RequiresGuild() bool       { return true }
func (c *embedPostSubCommand) RequiresPermissions() bool { return true }
func (c *embedPostSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	ce, err := loadCustomEmbed(c.embedService, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	channelID := ctx.Interaction.ChannelID
	if chID := opts.ChannelID(embedOptionChannel); chID != "" {
		cid, _ := discord.ParseSnowflake(chID)
		if cid != 0 {
			channelID = discord.ChannelID(cid)
		}
	}

	message, err := c.embedService.Post(ctx.Client, channelID, ce)
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to post the embed: %v", err))
	}

	postingNote := ""
	if message != nil && message.ID.IsValid() {
		posting := files.CustomEmbedPostingConfig{
			ChannelID: channelID.String(),
			MessageID: message.ID.String(),
		}
		if err := c.embedService.AddCustomEmbedPosting(ctx.GuildID.String(), ce.Key, posting); err != nil {
			slog.Warn("Mitigated service degradation: failed to track custom embed posting",
				slog.String("req_id", ctx.GuildID.String()),
				slog.String("embed_key", ce.Key),
				slog.String("error", err.Error()),
			)
			postingNote = fmt.Sprintf("\nWarning: the posting could not be tracked for later updates: %v", err)
		}
	}

	return respondEphemeralSuccess(ctx, fmt.Sprintf("Embed `%s` was posted in <#%s>.%s", ce.Key, channelID, postingNote))
}

type embedPreviewSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedPreviewSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedPreviewSubCommand {
	return &embedPreviewSubCommand{configManager: cm, embedService: svc}
}

func (c *embedPreviewSubCommand) Name() string { return embedSubPreview }
func (c *embedPreviewSubCommand) Description() string {
	return "Show an ephemeral preview of a custom embed"
}
func (c *embedPreviewSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{embedKeyOption(true)}
}
func (c *embedPreviewSubCommand) RequiresGuild() bool       { return true }
func (c *embedPreviewSubCommand) RequiresPermissions() bool { return true }
func (c *embedPreviewSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	ce, err := loadCustomEmbed(c.embedService, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	embed := c.embedService.Render(ce)

	// Convert embed structure to Arikawa Embed
	b, _ := json.Marshal(embed)
	var arikawaEmbed discord.Embed
	json.Unmarshal(b, &arikawaEmbed)

	return ctx.Respond(api.InteractionResponseData{
		Embeds: &[]discord.Embed{arikawaEmbed},
		Flags:  discord.EphemeralMessage,
	})
}

type embedSetSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedSetSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedSetSubCommand {
	return &embedSetSubCommand{configManager: cm, embedService: svc}
}

func (c *embedSetSubCommand) Name() string { return embedSubSet }
func (c *embedSetSubCommand) Description() string {
	return "Set custom embed title, description, color, images, author, and footer"
}
func (c *embedSetSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		embedKeyOption(true),
		&discord.StringOption{OptionName: embedOptionTitle, Description: "Embed title (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: embedOptionDescription, Description: "Embed description (omit to keep current, pass empty string to clear)", Required: false},
		&discord.IntegerOption{OptionName: embedOptionColor, Description: "Embed color as a decimal RGB integer. 0 to clear.", Required: false},
		&discord.StringOption{OptionName: embedOptionAuthorName, Description: "Embed author name (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: embedOptionAuthorIcon, Description: "Embed author icon URL (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: embedOptionFooterText, Description: "Embed footer text (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: embedOptionFooterIcon, Description: "Embed footer icon URL (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: embedOptionImageURL, Description: "Embed image URL (omit to keep current, pass empty string to clear)", Required: false},
		&discord.StringOption{OptionName: embedOptionThumbnailURL, Description: "Embed thumbnail URL (omit to keep current, pass empty string to clear)", Required: false},
	}
}
func (c *embedSetSubCommand) RequiresGuild() bool       { return true }
func (c *embedSetSubCommand) RequiresPermissions() bool { return true }
func (c *embedSetSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))

	current, fetchErr := c.embedService.CustomEmbed(ctx.GuildID.String(), key)
	if fetchErr != nil && !errors.Is(fetchErr, embedsvc.ErrCustomEmbedNotFound) {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to load embed `%s`: %v", key, fetchErr))
	}

	embed := current
	if opts.HasOption(embedOptionTitle) {
		embed.Title = opts.String(embedOptionTitle)
	}
	if opts.HasOption(embedOptionDescription) {
		embed.Description = opts.String(embedOptionDescription)
	}
	if opts.HasOption(embedOptionColor) {
		embed.Color = int(opts.Int(embedOptionColor))
	}
	if opts.HasOption(embedOptionAuthorName) {
		embed.AuthorName = opts.String(embedOptionAuthorName)
	}
	if opts.HasOption(embedOptionAuthorIcon) {
		embed.AuthorIconURL = opts.String(embedOptionAuthorIcon)
	}
	if opts.HasOption(embedOptionFooterText) {
		embed.FooterText = opts.String(embedOptionFooterText)
	}
	if opts.HasOption(embedOptionFooterIcon) {
		embed.FooterIconURL = opts.String(embedOptionFooterIcon)
	}
	if opts.HasOption(embedOptionImageURL) {
		embed.ImageURL = opts.String(embedOptionImageURL)
	}
	if opts.HasOption(embedOptionThumbnailURL) {
		embed.ThumbnailURL = opts.String(embedOptionThumbnailURL)
	}

	if err := c.embedService.SetCustomEmbedProperties(ctx.GuildID.String(), key, embed); err != nil {
		return respondStructuralError(ctx, "Failed to save changes", err)
	}

	syncNote := refreshCustomEmbedPostingsBestEffort(c.configManager, c.embedService, ctx, key)
	return respondEphemeralSuccess(ctx, fmt.Sprintf("Embed `%s` settings were updated.%s", key, syncNote))
}

type embedDeleteSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedDeleteSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedDeleteSubCommand {
	return &embedDeleteSubCommand{configManager: cm, embedService: svc}
}

func (c *embedDeleteSubCommand) Name() string { return embedSubDelete }
func (c *embedDeleteSubCommand) Description() string {
	return "Delete a custom embed entirely from config"
}
func (c *embedDeleteSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{embedKeyOption(true)}
}
func (c *embedDeleteSubCommand) RequiresGuild() bool       { return true }
func (c *embedDeleteSubCommand) RequiresPermissions() bool { return true }
func (c *embedDeleteSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	if _, err := c.embedService.DeleteCustomEmbed(ctx.GuildID.String(), key); err != nil {
		if errors.Is(err, embedsvc.ErrCustomEmbedNotFound) {
			return respondEphemeralError(ctx, fmt.Sprintf("Embed `%s` does not exist.", key))
		}
		return respondStructuralError(ctx, "Failed to delete embed", err)
	}

	return respondEphemeralSuccess(ctx, fmt.Sprintf("Embed `%s` was deleted.", key))
}

type embedListSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedListSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedListSubCommand {
	return &embedListSubCommand{configManager: cm, embedService: svc}
}

func (c *embedListSubCommand) Name() string                     { return embedSubList }
func (c *embedListSubCommand) Description() string              { return "List configured custom embeds" }
func (c *embedListSubCommand) Options() []discord.CommandOption { return nil }
func (c *embedListSubCommand) RequiresGuild() bool              { return true }
func (c *embedListSubCommand) RequiresPermissions() bool        { return true }
func (c *embedListSubCommand) Handle(ctx *commands.ArikawaContext) error {
	embeds, err := c.embedService.CustomEmbeds(ctx.GuildID.String())
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to list embeds: %v", err))
	}
	if len(embeds) == 0 {
		return respondEphemeralSuccess(ctx, "No custom embeds are configured yet. Use `/embed set` to create one.")
	}

	var b strings.Builder
	b.WriteString("Configured custom embeds:\n")
	for _, ce := range embeds {
		b.WriteString(fmt.Sprintf("• `%s` — %d field(s)\n", ce.Key, len(ce.Fields)))
	}
	return respondEphemeralSuccess(ctx, strings.TrimSpace(b.String()))
}

type embedRefreshSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedRefreshSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedRefreshSubCommand {
	return &embedRefreshSubCommand{configManager: cm, embedService: svc}
}

func (c *embedRefreshSubCommand) Name() string { return embedSubRefresh }
func (c *embedRefreshSubCommand) Description() string {
	return "Update all posted messages of a custom embed to match current config"
}
func (c *embedRefreshSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{embedKeyOption(true)}
}
func (c *embedRefreshSubCommand) RequiresGuild() bool       { return true }
func (c *embedRefreshSubCommand) RequiresPermissions() bool { return true }
func (c *embedRefreshSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	ce, err := loadCustomEmbed(c.embedService, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	if len(ce.Postings) == 0 {
		return respondEphemeralSuccess(ctx, fmt.Sprintf("Embed `%s` has no tracked postings yet. Use `/embed post` to publish it.", ce.Key))
	}

	embed := c.embedService.Render(ce)
	result := c.embedService.Sync(
		ctx.Client,
		ctx.GuildID.String(),
		ce.Key,
		ce.Postings,
		&embed,
	)
	summary := c.embedService.FormatSyncSummary(result, "Refreshed")
	if summary == "" {
		summary = "No postings needed updating."
	}
	return respondEphemeralSuccess(ctx, summary)
}

type embedUnpostSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedUnpostSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedUnpostSubCommand {
	return &embedUnpostSubCommand{configManager: cm, embedService: svc}
}

func (c *embedUnpostSubCommand) Name() string { return embedSubUnpost }
func (c *embedUnpostSubCommand) Description() string {
	return "Stop tracking a posted custom embed message and delete it"
}
func (c *embedUnpostSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{
			OptionName:  embedOptionMessageID,
			Description: "Discord message ID of the posting to retire",
			Required:    true,
		},
	}
}
func (c *embedUnpostSubCommand) RequiresGuild() bool       { return true }
func (c *embedUnpostSubCommand) RequiresPermissions() bool { return true }
func (c *embedUnpostSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	messageID := opts.String(embedOptionMessageID)
	if messageID == "" {
		return respondEphemeralError(ctx, "A message ID is required.")
	}

	embedKey, posting, lookupErr := c.embedService.FindCustomEmbedPosting(ctx.GuildID.String(), messageID)
	if lookupErr != nil {
		if errors.Is(lookupErr, embedsvc.ErrCustomEmbedPostingNotFound) {
			return respondEphemeralError(ctx, fmt.Sprintf("No tracked posting for message_id `%s`.", strings.TrimSpace(messageID)))
		}
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to look up posting: %v", lookupErr))
	}

	// Delete from Discord (best-effort)
	chID, _ := discord.ParseSnowflake(posting.ChannelID)
	msgID, _ := discord.ParseSnowflake(posting.MessageID)
	c.embedService.DeletePosting(ctx.Client, discord.ChannelID(chID), discord.MessageID(msgID))

	// Remove posting track from config
	if err := c.embedService.RemoveCustomEmbedPosting(ctx.GuildID.String(), embedKey, posting.MessageID); err != nil && !errors.Is(err, embedsvc.ErrCustomEmbedPostingNotFound) {
		slog.Warn("Mitigated service degradation: failed to strictly untrack old posting",
			slog.String("req_id", ctx.GuildID.String()),
			slog.String("error", err.Error()),
		)
		// We still consider the command a success because the post was deleted
	}

	return respondEphemeralSuccess(ctx, fmt.Sprintf("Stopped tracking posting `%s` for embed `%s` and deleted message.", messageID, embedKey))
}

type embedFieldAddSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedFieldAddSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedFieldAddSubCommand {
	return &embedFieldAddSubCommand{configManager: cm, embedService: svc}
}

func (c *embedFieldAddSubCommand) Name() string        { return embedSubFieldAdd }
func (c *embedFieldAddSubCommand) Description() string { return "Add a field to a custom embed" }
func (c *embedFieldAddSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		embedKeyOption(true),
		&discord.StringOption{OptionName: embedOptionFieldName, Description: "Field name/title", Required: true},
		&discord.StringOption{OptionName: embedOptionFieldValue, Description: "Field value/content", Required: true},
		&discord.BooleanOption{OptionName: embedOptionFieldInline, Description: "Whether the field is inline (default: false)", Required: false},
	}
}
func (c *embedFieldAddSubCommand) RequiresGuild() bool       { return true }
func (c *embedFieldAddSubCommand) RequiresPermissions() bool { return true }
func (c *embedFieldAddSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))

	name := opts.String(embedOptionFieldName)
	value := opts.String(embedOptionFieldValue)
	inline := opts.Bool(embedOptionFieldInline)

	field := files.CustomEmbedFieldConfig{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
	if err := c.embedService.AddCustomEmbedField(ctx.GuildID.String(), key, field); err != nil {
		return respondStructuralError(ctx, "Failed to add field", err)
	}
	syncNote := refreshCustomEmbedPostingsBestEffort(c.configManager, c.embedService, ctx, key)
	return respondEphemeralSuccess(ctx, fmt.Sprintf("Field `%s` was added to embed `%s`.%s", name, key, syncNote))
}

type embedFieldRemoveSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedFieldRemoveSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedFieldRemoveSubCommand {
	return &embedFieldRemoveSubCommand{configManager: cm, embedService: svc}
}

func (c *embedFieldRemoveSubCommand) Name() string { return embedSubFieldRemove }
func (c *embedFieldRemoveSubCommand) Description() string {
	return "Remove a field from a custom embed by its index"
}
func (c *embedFieldRemoveSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		embedKeyOption(true),
		&discord.IntegerOption{OptionName: embedOptionFieldIndex, Description: "1-based index of the field to remove", Required: true},
	}
}
func (c *embedFieldRemoveSubCommand) RequiresGuild() bool       { return true }
func (c *embedFieldRemoveSubCommand) RequiresPermissions() bool { return true }
func (c *embedFieldRemoveSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	if !opts.HasOption(embedOptionFieldIndex) {
		return respondEphemeralError(ctx, "A field index is required.")
	}
	index := int(opts.Int(embedOptionFieldIndex)) - 1

	if err := c.embedService.RemoveCustomEmbedField(ctx.GuildID.String(), key, index); err != nil {
		return respondStructuralError(ctx, "Failed to remove field", err)
	}
	syncNote := refreshCustomEmbedPostingsBestEffort(c.configManager, c.embedService, ctx, key)
	return respondEphemeralSuccess(ctx, fmt.Sprintf("Field %d was removed from embed `%s`.%s", index+1, key, syncNote))
}

type embedFieldListSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedFieldListSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedFieldListSubCommand {
	return &embedFieldListSubCommand{configManager: cm, embedService: svc}
}

func (c *embedFieldListSubCommand) Name() string { return embedSubFieldList }
func (c *embedFieldListSubCommand) Description() string {
	return "List the fields configured on a custom embed"
}
func (c *embedFieldListSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{embedKeyOption(true)}
}
func (c *embedFieldListSubCommand) RequiresGuild() bool       { return true }
func (c *embedFieldListSubCommand) RequiresPermissions() bool { return true }
func (c *embedFieldListSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	ce, err := loadCustomEmbed(c.embedService, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}
	if len(ce.Fields) == 0 {
		return respondEphemeralSuccess(ctx, fmt.Sprintf("Embed `%s` has no fields configured yet. Add one with `/embed field add`.", ce.Key))
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Fields on embed `%s`:\n", ce.Key))
	for i, f := range ce.Fields {
		b.WriteString(fmt.Sprintf("%d. `%s` — `%s` (inline: %t)\n", i+1, f.Name, f.Value, f.Inline))
	}
	return respondEphemeralSuccess(ctx, strings.TrimSpace(b.String()))
}

type embedImportSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedImportSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedImportSubCommand {
	return &embedImportSubCommand{configManager: cm, embedService: svc}
}

func (c *embedImportSubCommand) Name() string { return embedSubImport }
func (c *embedImportSubCommand) Description() string {
	return "Import a JSON embed from a Pastebin URL"
}
func (c *embedImportSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: embedOptionKey, Description: "The unique key of the embed to update", Required: true},
		&discord.StringOption{OptionName: embedOptionURL, Description: "The URL of the Pastebin/Discohook JSON", Required: true},
	}
}
func (c *embedImportSubCommand) RequiresGuild() bool       { return true }
func (c *embedImportSubCommand) RequiresPermissions() bool { return true }
func (c *embedImportSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	pasteURL := opts.String(embedOptionURL)

	data, err := localdiscord.FetchPastebinContent(ctx.Context(), pasteURL)
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to fetch from pastebin: %v", err))
	}

	discohookEmbed, err := embedsvc.ParseAndValidateDiscohookJSON(data)
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Invalid embed JSON: %v", err))
	}

	newEmbed := embedsvc.ToCustomEmbedConfig(discohookEmbed, key)
	if err := c.embedService.SetCustomEmbedProperties(ctx.GuildID.String(), key, newEmbed); err != nil {
		return respondStructuralError(ctx, "Failed to save imported embed properties", err)
	}
	if err := c.embedService.SetCustomEmbedFields(ctx.GuildID.String(), key, newEmbed.Fields); err != nil {
		return respondStructuralError(ctx, "Failed to save imported embed fields", err)
	}

	return respondEphemeralSuccess(ctx, fmt.Sprintf("Successfully imported JSON into embed `%s`.", key))
}

type embedExportSubCommand struct {
	configManager config.Provider
	embedService  *embedsvc.EmbedService
}

func newEmbedExportSubCommand(cm config.Provider, svc *embedsvc.EmbedService) *embedExportSubCommand {
	return &embedExportSubCommand{configManager: cm, embedService: svc}
}

func (c *embedExportSubCommand) Name() string { return embedSubExport }
func (c *embedExportSubCommand) Description() string {
	return "Export a JSON embed to a Pastebin provider"
}
func (c *embedExportSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: embedOptionKey, Description: "The unique key of the embed to export", Required: true},
	}
}
func (c *embedExportSubCommand) RequiresGuild() bool       { return true }
func (c *embedExportSubCommand) RequiresPermissions() bool { return true }
func (c *embedExportSubCommand) Handle(ctx *commands.ArikawaContext) error {
	key, err := embedKeyFromOptions(ctx)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	ce, err := loadCustomEmbed(c.embedService, ctx.GuildID, key)
	if err != nil {
		return respondEphemeralError(ctx, err.Error())
	}

	discohookJSON := embedsvc.FromCustomEmbedConfig(ce)
	data, err := json.MarshalIndent(discohookJSON, "", "  ")
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to format JSON: %v", err))
	}

	// This invokes localdiscord.UploadExportedContent to handle Pastebin uploads.
	// We pass nil for the authoring member as this package relies on arikawa
	// and the upload helper gracefully handles nil members.
	url, err := localdiscord.UploadExportedContent(ctx.Context(), nil, "", c.configManager, data)
	if err != nil {
		return respondEphemeralError(ctx, fmt.Sprintf("Failed to upload: %v", err))
	}

	return respondEphemeralSuccess(ctx, fmt.Sprintf("Embed `%s` successfully exported: <%s>", key, url))
}
