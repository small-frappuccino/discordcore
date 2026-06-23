package partners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	localdiscord "github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	partnersvc "github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/theme"
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

var (
	errPartnerNotFound = errors.New("partner not found")
	errPartnerExists   = errors.New("a partner with the new name already exists")
)

// PartnerCommands orchestrates the slash-command routing for partner board workflows.
// It integrates directly with the Arikawa router to execute lifecycle mutations.
type PartnerCommands struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

// NewPartnerCommands constructs the primary slash-command controller for partner boards.
// It mandates the injection of the configuration manager and domain service.
func NewPartnerCommands(configManager *files.ConfigManager, svc *partnersvc.PartnerService) *PartnerCommands {
	return &PartnerCommands{
		configManager:  configManager,
		partnerService: svc,
	}
}

// RegisterCommands binds the /partner slash group to the application router.
func (pc *PartnerCommands) RegisterCommands(router commands.ArikawaRegisterer) {
	if router == nil || pc == nil || pc.configManager == nil {
		return
	}

	slog.Info("Architectural state transition: Primary routines initialization",
		slog.String("component", "PartnerCommands"),
	)

	group := commands.NewArikawaGroupCommand(
		"partner",
		"Manage partner board records",
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

	router.Register(group)
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

func partnerDetailedCommandError(ctx *commands.ArikawaContext, message string) error {
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("❌ " + message),
		Flags:   discord.EphemeralMessage,
	})
}

func partnerStructuralError(ctx *commands.ArikawaContext, action string, err error) error {
	slog.Error("Blocking structural failure restricted to operational scope",
		slog.String("req_id", ctx.GuildID.String()),
		slog.String("stack_trace", string(debug.Stack())),
		slog.Int("fail_id", 500),
		slog.String("error", fmt.Sprintf("%s: %v", action, err)),
	)
	return partnerDetailedCommandError(ctx, fmt.Sprintf("%s: %v", action, err))
}

func partnerSuccess(ctx *commands.ArikawaContext, message string) error {
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("✅ " + message),
		Flags:   discord.EphemeralMessage,
	})
}

// --- Add ---
type partnerAddSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerAddSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerAddSubCommand {
	return &partnerAddSubCommand{configManager: cm, partnerService: s}
}

func (c *partnerAddSubCommand) Name() string { return "add" }

func (c *partnerAddSubCommand) Description() string {
	return "Add a new partner to the board"
}

func (c *partnerAddSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: optionFandom, Description: "Partner fandom category", Required: true},
		&discord.StringOption{OptionName: optionName, Description: "Partner name", Required: true},
		&discord.StringOption{OptionName: optionLink, Description: "Partner Discord invite link", Required: true},
	}
}

func (c *partnerAddSubCommand) RequiresGuild() bool       { return true }
func (c *partnerAddSubCommand) RequiresPermissions() bool { return true }

func (c *partnerAddSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	fandom := strings.TrimSpace(opts.String(optionFandom))
	name := strings.TrimSpace(opts.String(optionName))
	link := strings.TrimSpace(opts.String(optionLink))

	if name == "" || fandom == "" {
		return partnerDetailedCommandError(ctx, "Name and fandom must not be empty.")
	}

	cfg := c.configManager.GuildConfig(ctx.GuildID.String())
	if cfg == nil {
		return partnerDetailedCommandError(ctx, "Guild config not found.")
	}
	for _, p := range cfg.PartnerBoard.Partners {
		if strings.EqualFold(p.Name, name) {
			// Operational annotation: Partner names must remain strictly unique within a guild
			// to guarantee reliable resolution during autocomplete and targeted deletions.
			return partnerDetailedCommandError(ctx, "A partner with this name already exists.")
		}
	}

	entry := files.PartnerEntryConfig{Name: name, Fandom: fandom, Link: link}
	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for i := range cfg.Guilds {
			if cfg.Guilds[i].GuildID == ctx.GuildID.String() {
				cfg.Guilds[i].PartnerBoard.Partners = append(cfg.Guilds[i].PartnerBoard.Partners, entry)
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerStructuralError(ctx, "Failed to add partner", err)
	}

	c.partnerService.SyncConfig(ctx.GuildID.String(), ctx.Client)
	return partnerSuccess(ctx, "Partner added successfully.")
}

func autocompletePartnerNameFocused(ctx *commands.ArikawaContext, cm *files.ConfigManager, focusedOption string) (api.AutocompleteChoices, error) {
	var query string
	if data, ok := ctx.Interaction.Data.(*discord.AutocompleteInteraction); ok {
		var opts []discord.AutocompleteOption
		if len(data.Options) > 0 && data.Options[0].Type == discord.SubcommandOptionType {
			opts = data.Options[0].Options
		} else if len(data.Options) > 0 && data.Options[0].Type == discord.SubcommandGroupOptionType {
			if len(data.Options[0].Options) > 0 {
				opts = data.Options[0].Options[0].Options
			}
		} else {
			opts = data.Options
		}

		for _, opt := range opts {
			if opt.Name == focusedOption {
				query = opt.String()
				break
			}
		}
	}

	cfg := cm.GuildConfig(ctx.GuildID.String())
	if cfg == nil {
		return nil, nil
	}
	bc := cfg.PartnerBoard

	var choices api.AutocompleteStringChoices
	queryLower := strings.ToLower(query)

	for _, p := range bc.Partners {
		if queryLower == "" || strings.Contains(strings.ToLower(p.Name), queryLower) {
			choices = append(choices, discord.StringChoice{
				Name:  p.Name,
				Value: p.Name,
			})
			if len(choices) >= 25 {
				break
			}
		}
	}
	return choices, nil
}

func autocompletePartnerName(ctx *commands.ArikawaContext, cm *files.ConfigManager) (api.AutocompleteChoices, error) {
	return autocompletePartnerNameFocused(ctx, cm, optionName)
}

// --- Remove ---
type partnerRemoveSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerRemoveSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerRemoveSubCommand {
	return &partnerRemoveSubCommand{configManager: cm, partnerService: s}
}

func (c *partnerRemoveSubCommand) Name() string { return "remove" }
func (c *partnerRemoveSubCommand) Description() string {
	return "Remove a partner from the board"
}

func (c *partnerRemoveSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: optionName, Description: "Partner name", Required: true, Autocomplete: true},
	}
}

func (c *partnerRemoveSubCommand) RequiresGuild() bool       { return true }
func (c *partnerRemoveSubCommand) RequiresPermissions() bool { return true }

func (c *partnerRemoveSubCommand) Autocomplete(ctx *commands.ArikawaContext) (api.AutocompleteChoices, error) {
	return autocompletePartnerName(ctx, c.configManager)
}

func (c *partnerRemoveSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	name := strings.TrimSpace(opts.String(optionName))

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID.String() {
				bc := &cfg.Guilds[idx].PartnerBoard
				found := false
				for i, p := range bc.Partners {
					if strings.EqualFold(p.Name, name) {
						copy(bc.Partners[i:], bc.Partners[i+1:])
						bc.Partners[len(bc.Partners)-1] = files.PartnerEntryConfig{}
						bc.Partners = bc.Partners[:len(bc.Partners)-1]
						found = true
						break
					}
				}
				if !found {
					return errPartnerNotFound
				}
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		if errors.Is(err, errPartnerNotFound) {
			return partnerDetailedCommandError(ctx, "Partner not found.")
		}
		return partnerStructuralError(ctx, "Failed to remove partner", err)
	}

	c.partnerService.SyncConfig(ctx.GuildID.String(), ctx.Client)
	return partnerSuccess(ctx, "Partner removed successfully.")
}

// --- Link ---
type partnerLinkSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerLinkSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerLinkSubCommand {
	return &partnerLinkSubCommand{configManager: cm, partnerService: s}
}

func (c *partnerLinkSubCommand) Name() string { return "link" }
func (c *partnerLinkSubCommand) Description() string {
	return "Update a partner's Discord invite link"
}

func (c *partnerLinkSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: optionName, Description: "Partner name", Required: true, Autocomplete: true},
		&discord.StringOption{OptionName: optionLink, Description: "New partner Discord invite link", Required: true},
	}
}

func (c *partnerLinkSubCommand) RequiresGuild() bool       { return true }
func (c *partnerLinkSubCommand) RequiresPermissions() bool { return true }

func (c *partnerLinkSubCommand) Autocomplete(ctx *commands.ArikawaContext) (api.AutocompleteChoices, error) {
	return autocompletePartnerName(ctx, c.configManager)
}

func (c *partnerLinkSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	name := strings.TrimSpace(opts.String(optionName))
	link := strings.TrimSpace(opts.String(optionLink))

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID.String() {
				bc := &cfg.Guilds[idx].PartnerBoard
				found := false
				for i, p := range bc.Partners {
					if strings.EqualFold(p.Name, name) {
						bc.Partners[i].Link = link
						found = true
						break
					}
				}
				if !found {
					return errPartnerNotFound
				}
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		if errors.Is(err, errPartnerNotFound) {
			return partnerDetailedCommandError(ctx, "Partner not found.")
		}
		return partnerStructuralError(ctx, "Failed to update partner link", err)
	}

	c.partnerService.SyncConfig(ctx.GuildID.String(), ctx.Client)
	return partnerSuccess(ctx, "Partner link updated successfully.")
}

// --- Rename ---
type partnerRenameSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerRenameSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerRenameSubCommand {
	return &partnerRenameSubCommand{configManager: cm, partnerService: s}
}

func (c *partnerRenameSubCommand) Name() string { return "rename" }
func (c *partnerRenameSubCommand) Description() string {
	return "Rename a partner and/or move them to a different fandom"
}

func (c *partnerRenameSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: optionCurrentName, Description: "Current partner name", Required: true, Autocomplete: true},
		&discord.StringOption{OptionName: optionName, Description: "New partner name", Required: true},
		&discord.StringOption{OptionName: optionFandom, Description: "New partner fandom category", Required: false},
	}
}

func (c *partnerRenameSubCommand) RequiresGuild() bool       { return true }
func (c *partnerRenameSubCommand) RequiresPermissions() bool { return true }

func (c *partnerRenameSubCommand) Autocomplete(ctx *commands.ArikawaContext) (api.AutocompleteChoices, error) {
	var focusedName string
	if data, ok := ctx.Interaction.Data.(*discord.AutocompleteInteraction); ok {
		var opts []discord.AutocompleteOption
		if len(data.Options) > 0 && data.Options[0].Type == discord.SubcommandOptionType {
			opts = data.Options[0].Options
		} else if len(data.Options) > 0 && data.Options[0].Type == discord.SubcommandGroupOptionType {
			if len(data.Options[0].Options) > 0 {
				opts = data.Options[0].Options[0].Options
			}
		} else {
			opts = data.Options
		}

		for _, opt := range opts {
			if opt.Focused {
				focusedName = opt.Name
				break
			}
		}
	}

	if focusedName == optionCurrentName {
		return autocompletePartnerNameFocused(ctx, c.configManager, optionCurrentName)
	}
	return nil, nil
}

func (c *partnerRenameSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	currentName := strings.TrimSpace(opts.String(optionCurrentName))
	newName := strings.TrimSpace(opts.String(optionName))
	fandom := strings.TrimSpace(opts.String(optionFandom))

	if newName == "" {
		return partnerDetailedCommandError(ctx, "New name must not be empty.")
	}

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID.String() {
				bc := &cfg.Guilds[idx].PartnerBoard

				for _, p := range bc.Partners {
					if strings.EqualFold(p.Name, newName) && !strings.EqualFold(currentName, newName) {
						return errPartnerExists
					}
				}

				found := false
				for i, p := range bc.Partners {
					if strings.EqualFold(p.Name, currentName) {
						bc.Partners[i].Name = newName
						if fandom != "" {
							bc.Partners[i].Fandom = fandom
						}
						found = true
						break
					}
				}
				if !found {
					return errPartnerNotFound
				}
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		if errors.Is(err, errPartnerNotFound) {
			return partnerDetailedCommandError(ctx, "Partner not found.")
		}
		if errors.Is(err, errPartnerExists) {
			return partnerDetailedCommandError(ctx, "A partner with the new name already exists.")
		}
		return partnerStructuralError(ctx, "Failed to rename partner", err)
	}

	c.partnerService.SyncConfig(ctx.GuildID.String(), ctx.Client)
	return partnerSuccess(ctx, "Partner renamed successfully.")
}

// --- List ---
type partnerListSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerListSubCommand(cm *files.ConfigManager) *partnerListSubCommand {
	return &partnerListSubCommand{configManager: cm}
}

func (c *partnerListSubCommand) Name() string                     { return "list" }
func (c *partnerListSubCommand) Description() string              { return "List all partners on the board" }
func (c *partnerListSubCommand) Options() []discord.CommandOption { return nil }

func (c *partnerListSubCommand) RequiresGuild() bool       { return true }
func (c *partnerListSubCommand) RequiresPermissions() bool { return true }

func (c *partnerListSubCommand) Handle(ctx *commands.ArikawaContext) error {
	cfg := c.configManager.GuildConfig(ctx.GuildID.String())
	if cfg == nil {
		return partnerDetailedCommandError(ctx, "Guild config not found.")
	}

	boardCfg := cfg.PartnerBoard
	if len(boardCfg.Partners) == 0 {
		return partnerSuccess(ctx, "There are no partners configured for this server.")
	}

	var b strings.Builder
	for i, p := range boardCfg.Partners {
		b.WriteString(fmt.Sprintf("%d. `%s` | `%s` | %s\n", i+1, p.Name, p.Fandom, p.Link))
	}

	return ctx.Respond(api.InteractionResponseData{
		Embeds: &[]discord.Embed{
			{
				Title:       "Partner List",
				Description: b.String(),
				Color:       discord.Color(theme.Info()),
			},
		},
		Flags: discord.EphemeralMessage,
	})
}

// --- Post ---
type partnerPostSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerPostSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerPostSubCommand {
	return &partnerPostSubCommand{configManager: cm, partnerService: s}
}

func (c *partnerPostSubCommand) Name() string { return "post" }
func (c *partnerPostSubCommand) Description() string {
	return "Add a new posting channel or webhook for the partner board"
}

func (c *partnerPostSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: optionWebhookURL, Description: "Webhook URL to post via (if any)", Required: false},
	}
}

func (c *partnerPostSubCommand) RequiresGuild() bool       { return true }
func (c *partnerPostSubCommand) RequiresPermissions() bool { return true }

func (c *partnerPostSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	webhookURL := strings.TrimSpace(opts.String(optionWebhookURL))

	if webhookURL != "" {
		id, token, ok := parseWebhookURL(webhookURL)
		if !ok {
			return partnerDetailedCommandError(ctx, "Invalid Discord webhook URL.")
		}
		cfg := c.configManager.GuildConfig(ctx.GuildID.String())
		if cfg != nil {
			for _, posting := range cfg.PartnerBoard.Postings {
				if posting.WebhookID == id {
					return partnerDetailedCommandError(ctx, "This webhook is already registered.")
				}
			}
		}

		newPosting := files.CustomEmbedPostingConfig{
			WebhookID:    id,
			WebhookToken: token,
		}

		if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
			for i := range cfg.Guilds {
				if cfg.Guilds[i].GuildID == ctx.GuildID.String() {
					cfg.Guilds[i].PartnerBoard.Postings = append(cfg.Guilds[i].PartnerBoard.Postings, newPosting)
					return nil
				}
			}
			return errors.New("guild not found in config")
		}); err != nil {
			return partnerStructuralError(ctx, "Failed to save webhook", err)
		}
		c.partnerService.SyncConfig(ctx.GuildID.String(), ctx.Client)
		return partnerSuccess(ctx, "Webhook added successfully.")
	}

	channelID := ctx.Interaction.ChannelID
	cfg := c.configManager.GuildConfig(ctx.GuildID.String())
	if cfg != nil {
		for _, posting := range cfg.PartnerBoard.Postings {
			if posting.ChannelID == channelID.String() {
				return partnerDetailedCommandError(ctx, "This channel is already registered as a posting destination.")
			}
		}
	}

	newPosting := files.CustomEmbedPostingConfig{
		ChannelID: channelID.String(),
	}

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for i := range cfg.Guilds {
			if cfg.Guilds[i].GuildID == ctx.GuildID.String() {
				cfg.Guilds[i].PartnerBoard.Postings = append(cfg.Guilds[i].PartnerBoard.Postings, newPosting)
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerStructuralError(ctx, "Failed to save posting channel", err)
	}
	c.partnerService.SyncConfig(ctx.GuildID.String(), ctx.Client)
	return partnerSuccess(ctx, "Channel registered for postings successfully.")
}

// --- Unpost ---
type partnerUnpostSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerUnpostSubCommand(cm *files.ConfigManager) *partnerUnpostSubCommand {
	return &partnerUnpostSubCommand{configManager: cm}
}

func (c *partnerUnpostSubCommand) Name() string { return "unpost" }
func (c *partnerUnpostSubCommand) Description() string {
	return "Stop posting the partner board to a previously configured message or webhook"
}

func (c *partnerUnpostSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: optionMessageID, Description: "Discord message ID (if posted in a channel without webhook)", Required: false},
		&discord.StringOption{OptionName: optionWebhookURL, Description: "Webhook URL (if posted via webhook)", Required: false},
	}
}

func (c *partnerUnpostSubCommand) RequiresGuild() bool       { return true }
func (c *partnerUnpostSubCommand) RequiresPermissions() bool { return true }

func (c *partnerUnpostSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	messageID := strings.TrimSpace(opts.String(optionMessageID))
	webhookURL := strings.TrimSpace(opts.String(optionWebhookURL))

	if messageID == "" && webhookURL == "" {
		return partnerDetailedCommandError(ctx, "You must provide either a message ID or a webhook URL.")
	}

	whID := ""
	if webhookURL != "" {
		id, _, ok := parseWebhookURL(webhookURL)
		if !ok {
			return partnerDetailedCommandError(ctx, "Invalid Discord webhook URL.")
		}
		whID = id
	}

	var found *files.CustomEmbedPostingConfig
	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID.String() {
				postings := cfg.Guilds[idx].PartnerBoard.Postings
				for i, posting := range postings {
					matchMsg := messageID != "" && posting.MessageID == messageID
					matchWh := whID != "" && posting.WebhookID == whID
					if matchMsg || matchWh {
						found = &posting
						copy(postings[i:], postings[i+1:])
						postings[len(postings)-1] = files.CustomEmbedPostingConfig{}
						cfg.Guilds[idx].PartnerBoard.Postings = postings[:len(postings)-1]
						return nil
					}
				}
				return errors.New("posting not found")
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		slog.Warn("Mitigated service degradation: failed to strictly drop posting from config",
			slog.String("req_id", ctx.GuildID.String()),
			slog.String("error", err.Error()),
		)
	}

	if found != nil && found.ChannelID != "" && found.MessageID != "" {
		// Operational annotation: Execution of native message deletion is treated as best-effort.
		// Missing permissions or an already-deleted message will fail silently to prioritize
		// successful configuration state mutation over strict API parity.
		chID, _ := discord.ParseSnowflake(found.ChannelID)
		msgID, _ := discord.ParseSnowflake(found.MessageID)
		if chID != 0 && msgID != 0 {
			ctx.Client.DeleteMessage(discord.ChannelID(chID), discord.MessageID(msgID), "unpost command")
		}
	}
	return partnerSuccess(ctx, "Posting removed successfully.")
}

// --- Refresh ---
type partnerRefreshSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerRefreshSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerRefreshSubCommand {
	return &partnerRefreshSubCommand{configManager: cm, partnerService: s}
}

func (c *partnerRefreshSubCommand) Name() string                     { return "refresh" }
func (c *partnerRefreshSubCommand) Description() string              { return "Refresh all active partner postings" }
func (c *partnerRefreshSubCommand) Options() []discord.CommandOption { return nil }

func (c *partnerRefreshSubCommand) RequiresGuild() bool       { return true }
func (c *partnerRefreshSubCommand) RequiresPermissions() bool { return true }

func (c *partnerRefreshSubCommand) Handle(ctx *commands.ArikawaContext) error {
	ctx.Defer(discord.EphemeralMessage)

	if err := c.partnerService.SyncConfig(ctx.GuildID.String(), ctx.Client); err != nil {
		return partnerDetailedCommandError(ctx, fmt.Sprintf("Failed to sync partner board: %v", err))
	}
	return partnerSuccess(ctx, "Partner board refreshed successfully.")
}

// --- ImportTemplate ---
type partnerImportTemplateSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerImportTemplateSubCommand(cm *files.ConfigManager) *partnerImportTemplateSubCommand {
	return &partnerImportTemplateSubCommand{configManager: cm}
}

func (c *partnerImportTemplateSubCommand) Name() string { return "import_template" }
func (c *partnerImportTemplateSubCommand) Description() string {
	return "Import a template JSON from Pastebin to format the partner board"
}

func (c *partnerImportTemplateSubCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{OptionName: optionURL, Description: "Pastebin URL", Required: true},
	}
}

func (c *partnerImportTemplateSubCommand) RequiresGuild() bool       { return true }
func (c *partnerImportTemplateSubCommand) RequiresPermissions() bool { return true }

func (c *partnerImportTemplateSubCommand) Handle(ctx *commands.ArikawaContext) error {
	opts := commands.ArikawaOptionList(commands.GetArikawaSubCommandOptions(ctx.Interaction))
	pasteURL := strings.TrimSpace(opts.String(optionURL))

	data, err := localdiscord.FetchPastebinContent(context.Background(), pasteURL)
	if err != nil {
		return partnerDetailedCommandError(ctx, fmt.Sprintf("Failed to fetch from pastebin: %v", err))
	}

	discohookEmbed, err := files.ParseAndValidateDiscohookJSON(data)
	if err != nil {
		return partnerDetailedCommandError(ctx, fmt.Sprintf("Invalid embed JSON: %v", err))
	}

	cfg := c.configManager.GuildConfig(ctx.GuildID.String())
	if cfg == nil {
		return partnerDetailedCommandError(ctx, "Guild config not found.")
	}

	template := files.ToPartnerBoardTemplate(discohookEmbed, cfg.PartnerBoard.Template)

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for i := range cfg.Guilds {
			if cfg.Guilds[i].GuildID == ctx.GuildID.String() {
				cfg.Guilds[i].PartnerBoard.Template = template
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerStructuralError(ctx, "Failed to save template", err)
	}

	return partnerSuccess(ctx, "Template successfully imported.")
}

// --- ExportTemplate ---
type partnerExportTemplateSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerExportTemplateSubCommand(cm *files.ConfigManager) *partnerExportTemplateSubCommand {
	return &partnerExportTemplateSubCommand{configManager: cm}
}

func (c *partnerExportTemplateSubCommand) Name() string { return "export_template" }
func (c *partnerExportTemplateSubCommand) Description() string {
	return "Export the current template JSON to a Pastebin provider"
}

func (c *partnerExportTemplateSubCommand) Options() []discord.CommandOption { return nil }

func (c *partnerExportTemplateSubCommand) RequiresGuild() bool       { return true }
func (c *partnerExportTemplateSubCommand) RequiresPermissions() bool { return true }

func (c *partnerExportTemplateSubCommand) Handle(ctx *commands.ArikawaContext) error {
	cfg := c.configManager.GuildConfig(ctx.GuildID.String())
	if cfg == nil {
		return partnerDetailedCommandError(ctx, "Guild config not found.")
	}

	template := cfg.PartnerBoard.Template
	discohookJSON := files.FromPartnerBoardTemplate(template)
	data, err := json.MarshalIndent(discohookJSON, "", "  ")
	if err != nil {
		return partnerDetailedCommandError(ctx, fmt.Sprintf("Failed to format JSON: %v", err))
	}

	url, err := localdiscord.UploadExportedContent(context.Background(), nil, "", c.configManager, data)
	if err != nil {
		return partnerDetailedCommandError(ctx, fmt.Sprintf("Failed to upload: %v", err))
	}

	return partnerSuccess(ctx, fmt.Sprintf("Template successfully exported: <%s>", url))
}
