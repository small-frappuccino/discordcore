package embeds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
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

// EmbedCommands wires the /embed command tree into the router.
type EmbedCommands struct {
	configManager *files.ConfigManager
	syncer        *customEmbedPostingSyncer
}

// NewEmbedCommands builds the command bundle.
func NewEmbedCommands(configManager *files.ConfigManager) *EmbedCommands {
	return &EmbedCommands{
		configManager: configManager,
		syncer:        newCustomEmbedPostingSyncer(configManager),
	}
}

// RegisterCommands registers the slash group on the supplied router.
func (ec *EmbedCommands) RegisterCommands(router *core.CommandRouter) {
	if router == nil || ec == nil || ec.configManager == nil {
		return
	}

	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())

	embedGroup := core.NewGroupCommand(
		embedCommandName,
		"Manage custom embeds for this server",
		checker,
	)
	embedGroup.AddSubCommand(newEmbedPostSubCommand(ec.configManager))
	embedGroup.AddSubCommand(newEmbedPreviewSubCommand(ec.configManager))
	embedGroup.AddSubCommand(newEmbedSetSubCommand(ec.configManager, ec.syncer))
	embedGroup.AddSubCommand(newEmbedDeleteSubCommand(ec.configManager))
	embedGroup.AddSubCommand(newEmbedListSubCommand(ec.configManager))
	embedGroup.AddSubCommand(newEmbedRefreshSubCommand(ec.configManager, ec.syncer))
	embedGroup.AddSubCommand(newEmbedUnpostSubCommand(ec.configManager))
	embedGroup.AddSubCommand(newEmbedImportSubCommand(ec.configManager))
	embedGroup.AddSubCommand(newEmbedExportSubCommand(ec.configManager))

	fieldGroup := core.NewGroupCommand(
		embedFieldGroupName,
		"Manage the fields on a custom embed",
		checker,
	)
	fieldGroup.AddSubCommand(newEmbedFieldAddSubCommand(ec.configManager, ec.syncer))
	fieldGroup.AddSubCommand(newEmbedFieldRemoveSubCommand(ec.configManager, ec.syncer))
	fieldGroup.AddSubCommand(newEmbedFieldListSubCommand(ec.configManager))
	embedGroup.AddSubCommand(fieldGroup)

	router.RegisterSlashCommand(embedGroup)
}

// --- Leaf subcommands: /embed post|preview|set|delete|list|refresh|unpost ---

type embedPostSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedPostSubCommand(cm *files.ConfigManager) *embedPostSubCommand {
	return &embedPostSubCommand{configManager: cm}
}

// Name names.
func (c *embedPostSubCommand) Name() string { return embedSubPost }

// Description descriptions.
func (c *embedPostSubCommand) Description() string {
	return "Post a custom embed publicly in a channel"
}

// Options options.
func (c *embedPostSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		embedKeyOption(true),
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        embedOptionChannel,
			Description: "Target channel (defaults to current channel)",
			Required:    false,
		},
	}
}

// RequiresGuild requires guild.
func (c *embedPostSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedPostSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedPostSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedPostSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedPostSubCommand.Handle: %w", err)
	}
	ce, err := loadCustomEmbed(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("embedPostSubCommand.Handle: %w", err)
	}

	channelID := ctx.Interaction.ChannelID
	opts := core.GetSubCommandOptions(ctx.Interaction)
	for _, opt := range opts {
		if opt.Name == embedOptionChannel {
			if chVal, ok := opt.Value.(string); ok && strings.TrimSpace(chVal) != "" {
				channelID = strings.TrimSpace(chVal)
			}
		}
	}

	embed := renderCustomEmbed(ce)
	message, err := ctx.Session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
	if err != nil {
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to post the embed: %v", err))
	}

	postingNote := ""
	if message != nil && message.ID != "" {
		posting := files.CustomEmbedPostingConfig{
			ChannelID: channelID,
			MessageID: message.ID,
		}
		if err := c.configManager.AddCustomEmbedPosting(ctx.GuildID, ce.Key, posting); err != nil {
			postingNote = fmt.Sprintf("\nWarning: the posting could not be tracked for later updates: %v", err)
		}
	}

	return customEmbedResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Embed `%s` was posted in <#%s>.%s", ce.Key, channelID, postingNote),
	)
}

type embedPreviewSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedPreviewSubCommand(cm *files.ConfigManager) *embedPreviewSubCommand {
	return &embedPreviewSubCommand{configManager: cm}
}

// Name names.
func (c *embedPreviewSubCommand) Name() string { return embedSubPreview }

// Description descriptions.
func (c *embedPreviewSubCommand) Description() string {
	return "Show an ephemeral preview of a custom embed"
}

// Options options.
func (c *embedPreviewSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{embedKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *embedPreviewSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedPreviewSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedPreviewSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedPreviewSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedPreviewSubCommand.Handle: %w", err)
	}
	ce, err := loadCustomEmbed(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("embedPreviewSubCommand.Handle: %w", err)
	}

	embed := renderCustomEmbed(ce)
	return customEmbedResponseBuilder(ctx.Session).Build().Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}

type embedSetSubCommand struct {
	configManager *files.ConfigManager
	syncer        *customEmbedPostingSyncer
}

func newEmbedSetSubCommand(cm *files.ConfigManager, syncer *customEmbedPostingSyncer) *embedSetSubCommand {
	return &embedSetSubCommand{configManager: cm, syncer: syncer}
}

// Name names.
func (c *embedSetSubCommand) Name() string { return embedSubSet }

// Description descriptions.
func (c *embedSetSubCommand) Description() string {
	return "Set custom embed title, description, color, images, author, and footer"
}

// Options options.
func (c *embedSetSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		embedKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionTitle, Description: "Embed title (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionDescription, Description: "Embed description (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: embedOptionColor, Description: "Embed color as a decimal RGB integer. 0 to clear.", Required: false, MinValue: floatPtr(0), MaxValue: float64(files.CustomEmbedColorMax)},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionAuthorName, Description: "Embed author name (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionAuthorIcon, Description: "Embed author icon URL (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionFooterText, Description: "Embed footer text (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionFooterIcon, Description: "Embed footer icon URL (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionImageURL, Description: "Embed image URL (omit to keep current, pass empty string to clear)", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionThumbnailURL, Description: "Embed thumbnail URL (omit to keep current, pass empty string to clear)", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *embedSetSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedSetSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedSetSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedSetSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedSetSubCommand.Handle: %w", err)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	current, fetchErr := c.configManager.CustomEmbed(ctx.GuildID, key)
	if fetchErr != nil && !errors.Is(fetchErr, files.ErrCustomEmbedNotFound) {
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to load embed `%s`: %v", key, fetchErr))
	}

	embed := current
	if extractor.HasOption(embedOptionTitle) {
		embed.Title = extractor.String(embedOptionTitle)
	}
	if extractor.HasOption(embedOptionDescription) {
		embed.Description = extractor.String(embedOptionDescription)
	}
	if extractor.HasOption(embedOptionColor) {
		embed.Color = int(extractor.Int(embedOptionColor))
	}
	if extractor.HasOption(embedOptionAuthorName) {
		embed.AuthorName = extractor.String(embedOptionAuthorName)
	}
	if extractor.HasOption(embedOptionAuthorIcon) {
		embed.AuthorIconURL = extractor.String(embedOptionAuthorIcon)
	}
	if extractor.HasOption(embedOptionFooterText) {
		embed.FooterText = extractor.String(embedOptionFooterText)
	}
	if extractor.HasOption(embedOptionFooterIcon) {
		embed.FooterIconURL = extractor.String(embedOptionFooterIcon)
	}
	if extractor.HasOption(embedOptionImageURL) {
		embed.ImageURL = extractor.String(embedOptionImageURL)
	}
	if extractor.HasOption(embedOptionThumbnailURL) {
		embed.ThumbnailURL = extractor.String(embedOptionThumbnailURL)
	}

	if err := c.configManager.SetCustomEmbedProperties(ctx.GuildID, key, embed); err != nil {
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to update embed `%s`: %v", key, err))
	}

	syncNote := refreshCustomEmbedPostingsBestEffort(c.configManager, c.syncer, ctx, key)
	return customEmbedResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Embed `%s` settings were updated.%s", key, syncNote),
	)
}

type embedDeleteSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedDeleteSubCommand(cm *files.ConfigManager) *embedDeleteSubCommand {
	return &embedDeleteSubCommand{configManager: cm}
}

// Name names.
func (c *embedDeleteSubCommand) Name() string { return embedSubDelete }

// Description descriptions.
func (c *embedDeleteSubCommand) Description() string {
	return "Delete a custom embed entirely from config"
}

// Options options.
func (c *embedDeleteSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{embedKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *embedDeleteSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedDeleteSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedDeleteSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedDeleteSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedDeleteSubCommand.Handle: %w", err)
	}

	if _, err := c.configManager.DeleteCustomEmbed(ctx.GuildID, key); err != nil {
		if errors.Is(err, files.ErrCustomEmbedNotFound) {
			return customEmbedDetailedCommandError(fmt.Sprintf("Embed `%s` does not exist.", key))
		}
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to delete embed `%s`: %v", key, err))
	}

	return customEmbedResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Embed `%s` was deleted.", key),
	)
}

type embedListSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedListSubCommand(cm *files.ConfigManager) *embedListSubCommand {
	return &embedListSubCommand{configManager: cm}
}

// Name names.
func (c *embedListSubCommand) Name() string { return embedSubList }

// Description descriptions.
func (c *embedListSubCommand) Description() string { return "List configured custom embeds" }

// Options options.
func (c *embedListSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }

// RequiresGuild requires guild.
func (c *embedListSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedListSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *embedListSubCommand) Handle(ctx *core.Context) error {
	embeds, err := c.configManager.CustomEmbeds(ctx.GuildID)
	if err != nil {
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to list embeds: %v", err))
	}
	if len(embeds) == 0 {
		return customEmbedResponseBuilder(ctx.Session).Info(
			ctx.Interaction,
			"No custom embeds are configured yet. Use `/embed set` to create one.",
		)
	}

	var b strings.Builder
	b.WriteString("Configured custom embeds:\n")
	for _, ce := range embeds {
		b.WriteString(fmt.Sprintf("• `%s` — %d field(s)\n", ce.Key, len(ce.Fields)))
	}
	return customEmbedResponseBuilder(ctx.Session).Info(ctx.Interaction, strings.TrimSpace(b.String()))
}

type embedRefreshSubCommand struct {
	configManager *files.ConfigManager
	syncer        *customEmbedPostingSyncer
}

func newEmbedRefreshSubCommand(cm *files.ConfigManager, syncer *customEmbedPostingSyncer) *embedRefreshSubCommand {
	return &embedRefreshSubCommand{configManager: cm, syncer: syncer}
}

// Name names.
func (c *embedRefreshSubCommand) Name() string { return embedSubRefresh }

// Description descriptions.
func (c *embedRefreshSubCommand) Description() string {
	return "Update all posted messages of a custom embed to match current config"
}

// Options options.
func (c *embedRefreshSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{embedKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *embedRefreshSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedRefreshSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedRefreshSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedRefreshSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedRefreshSubCommand.Handle: %w", err)
	}
	ce, err := loadCustomEmbed(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("embedRefreshSubCommand.Handle: %w", err)
	}
	if len(ce.Postings) == 0 {
		return customEmbedResponseBuilder(ctx.Session).Info(
			ctx.Interaction,
			fmt.Sprintf("Embed `%s` has no tracked postings yet. Use `/embed post` to publish it.", ce.Key),
		)
	}

	builder := customEmbedResponseBuilder(ctx.Session)
	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return fmt.Errorf("embedRefreshSubCommand.Handle: %w", err)
	}
	ctx.Acknowledged = true

	result := c.syncer.Sync(
		ctx.Session,
		ctx.GuildID,
		ce.Key,
		ce.Postings,
		renderCustomEmbed(ce),
	)
	summary := formatCustomEmbedSyncSummary(result, "Refreshed")
	if summary == "" {
		summary = "No postings needed updating."
	}
	return builder.WithContext(ctx).Success(ctx.Interaction, summary)
}

type embedUnpostSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedUnpostSubCommand(cm *files.ConfigManager) *embedUnpostSubCommand {
	return &embedUnpostSubCommand{configManager: cm}
}

// Name names.
func (c *embedUnpostSubCommand) Name() string { return embedSubUnpost }

// Description descriptions.
func (c *embedUnpostSubCommand) Description() string {
	return "Stop tracking a posted custom embed message and delete it"
}

// Options options.
func (c *embedUnpostSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        embedOptionMessageID,
		Description: "Discord message ID of the posting to retire",
		Required:    true,
	}}
}

// RequiresGuild requires guild.
func (c *embedUnpostSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedUnpostSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedUnpostSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedUnpostSubCommand) Handle(ctx *core.Context) error {
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))
	messageID, err := extractor.StringRequired(embedOptionMessageID)
	if err != nil {
		return customEmbedDetailedCommandError("A message ID is required.")
	}

	embedKey, posting, lookupErr := c.configManager.FindCustomEmbedPosting(ctx.GuildID, messageID)
	if lookupErr != nil {
		if errors.Is(lookupErr, files.ErrCustomEmbedPostingNotFound) {
			return customEmbedDetailedCommandError(fmt.Sprintf("No tracked posting for message_id `%s`.", strings.TrimSpace(messageID)))
		}
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to look up posting: %v", lookupErr))
	}

	// Delete from Discord (best-effort)
	_ = ctx.Session.ChannelMessageDelete(posting.ChannelID, posting.MessageID)

	// Remove posting track from config
	if err := c.configManager.RemoveCustomEmbedPosting(ctx.GuildID, embedKey, posting.MessageID); err != nil && !errors.Is(err, files.ErrCustomEmbedPostingNotFound) {
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to drop posting from config: %v", err))
	}

	return customEmbedResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Stopped tracking posting `%s` for embed `%s` and deleted message.", messageID, embedKey),
	)
}

// --- Subgroup: /embed field add|remove|list ---

type embedFieldAddSubCommand struct {
	configManager *files.ConfigManager
	syncer        *customEmbedPostingSyncer
}

func newEmbedFieldAddSubCommand(cm *files.ConfigManager, syncer *customEmbedPostingSyncer) *embedFieldAddSubCommand {
	return &embedFieldAddSubCommand{configManager: cm, syncer: syncer}
}

// Name names.
func (c *embedFieldAddSubCommand) Name() string { return embedSubFieldAdd }

// Description descriptions.
func (c *embedFieldAddSubCommand) Description() string {
	return "Add a field to a custom embed"
}

// Options options.
func (c *embedFieldAddSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		embedKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionFieldName, Description: "Field name/title", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: embedOptionFieldValue, Description: "Field value/content", Required: true},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: embedOptionFieldInline, Description: "Whether the field is inline (default: false)", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *embedFieldAddSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedFieldAddSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedFieldAddSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedFieldAddSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedFieldAddSubCommand.Handle: %w", err)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(embedOptionFieldName)
	if err != nil {
		return customEmbedDetailedCommandError(err.Error())
	}
	value, err := extractor.StringRequired(embedOptionFieldValue)
	if err != nil {
		return customEmbedDetailedCommandError(err.Error())
	}
	inline := false
	if extractor.HasOption(embedOptionFieldInline) {
		inline = extractor.Bool(embedOptionFieldInline)
	}

	field := files.CustomEmbedFieldConfig{
		Name:   name,
		Value:  value,
		Inline: inline,
	}
	if err := c.configManager.AddCustomEmbedField(ctx.GuildID, key, field); err != nil {
		return customEmbedDetailedCommandError(fmt.Sprintf("Failed to add field: %v", err))
	}
	syncNote := refreshCustomEmbedPostingsBestEffort(c.configManager, c.syncer, ctx, key)
	return customEmbedResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Field `%s` was added to embed `%s`.%s", name, key, syncNote),
	)
}

type embedFieldRemoveSubCommand struct {
	configManager *files.ConfigManager
	syncer        *customEmbedPostingSyncer
}

func newEmbedFieldRemoveSubCommand(cm *files.ConfigManager, syncer *customEmbedPostingSyncer) *embedFieldRemoveSubCommand {
	return &embedFieldRemoveSubCommand{configManager: cm, syncer: syncer}
}

// Name names.
func (c *embedFieldRemoveSubCommand) Name() string { return embedSubFieldRemove }

// Description descriptions.
func (c *embedFieldRemoveSubCommand) Description() string {
	return "Remove a field from a custom embed by its index"
}

// Options options.
func (c *embedFieldRemoveSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		embedKeyOption(true),
		{Type: discordgo.ApplicationCommandOptionInteger, Name: embedOptionFieldIndex, Description: "1-based index of the field to remove (use /embed field list to see indexes)", Required: true},
	}
}

// RequiresGuild requires guild.
func (c *embedFieldRemoveSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedFieldRemoveSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedFieldRemoveSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedFieldRemoveSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedFieldRemoveSubCommand.Handle: %w", err)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))
	if !extractor.HasOption(embedOptionFieldIndex) {
		return customEmbedDetailedCommandError("A field index is required.")
	}
	// The user passes a 1-based index, but our API uses 0-based
	index := int(extractor.Int(embedOptionFieldIndex)) - 1

	if err := c.configManager.RemoveCustomEmbedField(ctx.GuildID, key, index); err != nil {
		switch {
		case errors.Is(err, files.ErrCustomEmbedNotFound):
			return customEmbedDetailedCommandError(fmt.Sprintf("Embed `%s` does not exist.", key))
		default:
			return customEmbedDetailedCommandError(fmt.Sprintf("Failed to remove field: %v", err))
		}
	}
	syncNote := refreshCustomEmbedPostingsBestEffort(c.configManager, c.syncer, ctx, key)
	return customEmbedResponseBuilder(ctx.Session).Success(
		ctx.Interaction,
		fmt.Sprintf("Field %d was removed from embed `%s`.%s", index+1, key, syncNote),
	)
}

type embedFieldListSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedFieldListSubCommand(cm *files.ConfigManager) *embedFieldListSubCommand {
	return &embedFieldListSubCommand{configManager: cm}
}

// Name names.
func (c *embedFieldListSubCommand) Name() string { return embedSubFieldList }

// Description descriptions.
func (c *embedFieldListSubCommand) Description() string {
	return "List the fields configured on a custom embed"
}

// Options options.
func (c *embedFieldListSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{embedKeyOption(true)}
}

// RequiresGuild requires guild.
func (c *embedFieldListSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedFieldListSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedFieldListSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedFieldListSubCommand) Handle(ctx *core.Context) error {
	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedFieldListSubCommand.Handle: %w", err)
	}
	ce, err := loadCustomEmbed(c.configManager, ctx.GuildID, key)
	if err != nil {
		return fmt.Errorf("embedFieldListSubCommand.Handle: %w", err)
	}
	if len(ce.Fields) == 0 {
		return customEmbedResponseBuilder(ctx.Session).Info(
			ctx.Interaction,
			fmt.Sprintf("Embed `%s` has no fields configured yet. Add one with `/embed field add`.", ce.Key),
		)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Fields on embed `%s`:\n", ce.Key))
	for i, f := range ce.Fields {
		b.WriteString(fmt.Sprintf("%d. `%s` — `%s` (inline: %t)\n", i+1, f.Name, f.Value, f.Inline))
	}
	return customEmbedResponseBuilder(ctx.Session).Info(ctx.Interaction, strings.TrimSpace(b.String()))
}

// --- Helpers ---

func refreshCustomEmbedPostingsBestEffort(cm *files.ConfigManager, syncer *customEmbedPostingSyncer, ctx *core.Context, key string) string {
	if cm == nil || syncer == nil || ctx == nil {
		return ""
	}
	ce, err := cm.CustomEmbed(ctx.GuildID, key)
	if err != nil {
		return ""
	}
	if len(ce.Postings) == 0 {
		return ""
	}
	result := syncer.Sync(
		ctx.Session,
		ctx.GuildID,
		ce.Key,
		ce.Postings,
		renderCustomEmbed(ce),
	)
	if !result.HasIssues() && result.Edited == 0 {
		return ""
	}
	summary := formatCustomEmbedSyncSummary(result, "Refreshed")
	if summary == "" {
		return ""
	}
	return "\n" + summary
}

func embedKeyOption(required bool) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:         discordgo.ApplicationCommandOptionString,
		Name:         embedOptionKey,
		Description:  "Embed identifier (lowercase letters, digits, '-' or '_')",
		Required:     required,
		Autocomplete: true,
	}
}

func handleEmbedKeyAutocomplete(cm *files.ConfigManager, ctx *core.Context) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if ctx.GuildID == "" {
		return nil, nil
	}
	embeds, err := cm.CustomEmbeds(ctx.GuildID)
	if err != nil || len(embeds) == 0 {
		return nil, nil
	}

	opts := core.GetSubCommandOptions(ctx.Interaction)
	focused, found := core.HasFocusedOption(opts)
	if !found {
		return nil, nil
	}

	input := strings.ToLower(fmt.Sprintf("%v", focused.Value))

	var choices []*discordgo.ApplicationCommandOptionChoice
	for _, e := range embeds {
		if input == "" || strings.HasPrefix(e.Key, input) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  e.Key,
				Value: e.Key,
			})
		}
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices, nil
}

func embedKeyFromOptions(interaction *discordgo.InteractionCreate) (string, error) {
	opts := core.GetSubCommandOptions(interaction)
	extractor := core.OptionList(opts)
	key, err := extractor.StringRequired(embedOptionKey)
	if err != nil {
		return "", customEmbedDetailedCommandError(err.Error())
	}
	key = files.NormalizeCustomEmbedKey(key)
	if key == "" {
		return "", customEmbedDetailedCommandError("A non-empty key option is required.")
	}
	return key, nil
}

func loadCustomEmbed(cm *files.ConfigManager, guildID, key string) (files.CustomEmbedConfig, error) {
	ce, err := cm.CustomEmbed(guildID, key)
	if err != nil {
		if errors.Is(err, files.ErrCustomEmbedNotFound) {
			return files.CustomEmbedConfig{}, customEmbedDetailedCommandError(fmt.Sprintf("Embed `%s` does not exist.", key))
		}
		return files.CustomEmbedConfig{}, customEmbedDetailedCommandError(fmt.Sprintf("Failed to load embed `%s`: %v", key, err))
	}
	return ce, nil
}

func customEmbedDetailedCommandError(message string) error {
	return &core.CommandError{Message: message, Ephemeral: true}
}

func customEmbedResponseBuilder(session *discordgo.Session) *core.ResponseBuilder {
	return core.NewResponseBuilder(session).Ephemeral()
}

func floatPtr(v float64) *float64 { return &v }

type embedImportSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedImportSubCommand(cm *files.ConfigManager) *embedImportSubCommand {
	return &embedImportSubCommand{configManager: cm}
}

// Name names.
func (c *embedImportSubCommand) Name() string { return embedSubImport }

// Description descriptions.
func (c *embedImportSubCommand) Description() string {
	return "Import a JSON embed from a Pastebin URL"
}

// Options options.
func (c *embedImportSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        embedOptionKey,
			Description: "The unique key of the embed to update",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        embedOptionURL,
			Description: "The URL of the Pastebin/Discohook JSON",
			Required:    true,
		},
	}
}

// RequiresGuild requires guild.
func (c *embedImportSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedImportSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedImportSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedImportSubCommand) Handle(ctx *core.Context) error {
	builder := customEmbedResponseBuilder(ctx.Session)
	guildID := ctx.GuildID

	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedImportSubCommand.Handle: %w", err)
	}

	var pasteURL string
	opts := core.GetSubCommandOptions(ctx.Interaction)
	for _, opt := range opts {
		if opt.Name == embedOptionURL {
			pasteURL = strings.TrimSpace(fmt.Sprint(opt.Value))
		}
	}

	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return fmt.Errorf("embedImportSubCommand.Handle: %w", err)
	}
	ctx.Acknowledged = true

	data, err := discord.FetchPastebinContent(context.Background(), pasteURL)
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to fetch from pastebin: %v", err))
	}

	discohookEmbed, err := files.ParseAndValidateDiscohookJSON(data)
	if err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Invalid embed JSON: %v", err))
	}

	newEmbed := files.ToCustomEmbedConfig(discohookEmbed, key)
	if err := c.configManager.SetCustomEmbedProperties(guildID, key, newEmbed); err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to save imported embed properties: %v", err))
	}
	if err := c.configManager.SetCustomEmbedFields(guildID, key, newEmbed.Fields); err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to save imported embed fields: %v", err))
	}

	return builder.WithContext(ctx).Success(ctx.Interaction, fmt.Sprintf("Successfully imported JSON into embed `%s`.", key))
}

type embedExportSubCommand struct {
	configManager *files.ConfigManager
}

func newEmbedExportSubCommand(cm *files.ConfigManager) *embedExportSubCommand {
	return &embedExportSubCommand{configManager: cm}
}

// Name names.
func (c *embedExportSubCommand) Name() string { return embedSubExport }

// Description descriptions.
func (c *embedExportSubCommand) Description() string {
	return "Export a JSON embed to a Pastebin provider"
}

// Options options.
func (c *embedExportSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        embedOptionKey,
			Description: "The unique key of the embed to export",
			Required:    true,
		},
	}
}

// RequiresGuild requires guild.
func (c *embedExportSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *embedExportSubCommand) RequiresPermissions() bool { return true }

// HandleAutocomplete handles autocomplete.
func (c *embedExportSubCommand) HandleAutocomplete(ctx *core.Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	if focusedOption == embedOptionKey {
		return handleEmbedKeyAutocomplete(c.configManager, ctx)
	}
	return nil, nil
}

// Handle handles.
func (c *embedExportSubCommand) Handle(ctx *core.Context) error {
	builder := customEmbedResponseBuilder(ctx.Session)
	guildID := ctx.GuildID

	key, err := embedKeyFromOptions(ctx.Interaction)
	if err != nil {
		return fmt.Errorf("embedExportSubCommand.Handle: %w", err)
	}

	ce, err := loadCustomEmbed(c.configManager, guildID, key)
	if err != nil {
		return fmt.Errorf("embedExportSubCommand.Handle: %w", err)
	}

	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return fmt.Errorf("embedExportSubCommand.Handle: %w", err)
	}
	ctx.Acknowledged = true

	discohookJSON := files.FromCustomEmbedConfig(ce)
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

	return builder.WithContext(ctx).Success(ctx.Interaction, fmt.Sprintf("Embed `%s` successfully exported: <%s>", key, url))
}
