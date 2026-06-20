package partners

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	partnersvc "github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/theme"
	"github.com/small-frappuccino/discordgo"
)

// --- Add ---
type partnerAddSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerAddSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerAddSubCommand {
	return &partnerAddSubCommand{configManager: cm, partnerService: s}
}

// Name names.
func (c *partnerAddSubCommand) Name() string { return "add" }

// Description descriptions.
func (c *partnerAddSubCommand) Description() string {
	return "Add a new partner to the board"
}

// Options options.
func (c *partnerAddSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: optionFandom, Description: "Partner fandom category", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: optionName, Description: "Partner name", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: optionLink, Description: "Partner Discord invite link", Required: true},
	}
}

// RequiresGuild requires guild.
func (c *partnerAddSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerAddSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *partnerAddSubCommand) Handle(ctx *legacycore.Context) error {
	extractor := legacycore.OptionList(legacycore.GetSubCommandOptions(ctx.Interaction))
	fandom, _ := extractor.StringRequired(optionFandom)
	name, _ := extractor.StringRequired(optionName)
	link, _ := extractor.StringRequired(optionLink)

	name = strings.TrimSpace(name)
	fandom = strings.TrimSpace(fandom)
	if name == "" || fandom == "" {
		return partnerDetailedCommandError("Name and fandom must not be empty.")
	}

	cfg := c.configManager.GuildConfig(ctx.GuildID)
	if cfg == nil {
		return partnerDetailedCommandError("Guild config not found.")
	}
	for _, p := range cfg.PartnerBoard.Partners {
		if strings.EqualFold(p.Name, name) {
			return partnerDetailedCommandError("A partner with this name already exists.")
		}
	}

	entry := files.PartnerEntryConfig{Name: name, Fandom: fandom, Link: link}
	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for i := range cfg.Guilds {
			if cfg.Guilds[i].GuildID == ctx.GuildID {
				cfg.Guilds[i].PartnerBoard.Partners = append(cfg.Guilds[i].PartnerBoard.Partners, entry)
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerDetailedCommandError(fmt.Sprintf("Failed to add partner: %v", err))
	}

	c.partnerService.SyncConfig(ctx.GuildID, api.NewClient(ctx.Session.Token))
	return legacycore.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "Partner added successfully.")
}

// --- Remove ---
type partnerRemoveSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerRemoveSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerRemoveSubCommand {
	return &partnerRemoveSubCommand{configManager: cm, partnerService: s}
}

// Name names.
func (c *partnerRemoveSubCommand) Name() string { return "remove" }

// Description descriptions.
func (c *partnerRemoveSubCommand) Description() string {
	return "Remove a partner from the board"
}

// Options options.
func (c *partnerRemoveSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: optionName, Description: "Partner name", Required: true, Autocomplete: true},
	}
}

// RequiresGuild requires guild.
func (c *partnerRemoveSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerRemoveSubCommand) RequiresPermissions() bool { return true }

// Autocomplete autocompletes.
func (c *partnerRemoveSubCommand) Autocomplete(ctx *legacycore.Context) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return autocompletePartnerName(ctx, c.configManager)
}

// Handle handles.
func (c *partnerRemoveSubCommand) Handle(ctx *legacycore.Context) error {
	extractor := legacycore.OptionList(legacycore.GetSubCommandOptions(ctx.Interaction))
	name, _ := extractor.StringRequired(optionName)

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID {
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
					return errors.New("partner not found")
				}
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerDetailedCommandError(fmt.Sprintf("Failed to remove partner: %v", err))
	}

	c.partnerService.SyncConfig(ctx.GuildID, api.NewClient(ctx.Session.Token))
	return legacycore.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "Partner removed successfully.")
}

// --- Link ---
type partnerLinkSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerLinkSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerLinkSubCommand {
	return &partnerLinkSubCommand{configManager: cm, partnerService: s}
}

// Name names.
func (c *partnerLinkSubCommand) Name() string { return "link" }

// Description descriptions.
func (c *partnerLinkSubCommand) Description() string {
	return "Update a partner's Discord invite link"
}

// Options options.
func (c *partnerLinkSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: optionName, Description: "Partner name", Required: true, Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: optionLink, Description: "New partner Discord invite link", Required: true},
	}
}

// RequiresGuild requires guild.
func (c *partnerLinkSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerLinkSubCommand) RequiresPermissions() bool { return true }

// Autocomplete autocompletes.
func (c *partnerLinkSubCommand) Autocomplete(ctx *legacycore.Context) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return autocompletePartnerName(ctx, c.configManager)
}

// Handle handles.
func (c *partnerLinkSubCommand) Handle(ctx *legacycore.Context) error {
	extractor := legacycore.OptionList(legacycore.GetSubCommandOptions(ctx.Interaction))
	name, _ := extractor.StringRequired(optionName)
	link, _ := extractor.StringRequired(optionLink)

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID {
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
					return errors.New("partner not found")
				}
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerDetailedCommandError(fmt.Sprintf("Failed to update partner link: %v", err))
	}

	c.partnerService.SyncConfig(ctx.GuildID, api.NewClient(ctx.Session.Token))
	return legacycore.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "Partner link updated successfully.")
}

// --- Rename ---
type partnerRenameSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerRenameSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerRenameSubCommand {
	return &partnerRenameSubCommand{configManager: cm, partnerService: s}
}

// Name names.
func (c *partnerRenameSubCommand) Name() string { return "rename" }

// Description descriptions.
func (c *partnerRenameSubCommand) Description() string {
	return "Rename a partner and/or move them to a different fandom"
}

// Options options.
func (c *partnerRenameSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: optionCurrentName, Description: "Current partner name", Required: true, Autocomplete: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: optionName, Description: "New partner name", Required: true},
		{Type: discordgo.ApplicationCommandOptionString, Name: optionFandom, Description: "New partner fandom category", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *partnerRenameSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerRenameSubCommand) RequiresPermissions() bool { return true }

// Autocomplete autocompletes.
func (c *partnerRenameSubCommand) Autocomplete(ctx *legacycore.Context) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	opts := legacycore.GetSubCommandOptions(ctx.Interaction)
	focused, found := legacycore.HasFocusedOption(opts)
	if found && focused.Name == optionCurrentName {
		return autocompletePartnerNameFocused(ctx, c.configManager, optionCurrentName)
	}
	return nil, nil
}

// Handle handles.
func (c *partnerRenameSubCommand) Handle(ctx *legacycore.Context) error {
	extractor := legacycore.OptionList(legacycore.GetSubCommandOptions(ctx.Interaction))
	currentName, _ := extractor.StringRequired(optionCurrentName)
	newName, _ := extractor.StringRequired(optionName)
	fandom := extractor.String(optionFandom)

	newName = strings.TrimSpace(newName)
	fandom = strings.TrimSpace(fandom)
	if newName == "" {
		return partnerDetailedCommandError("New name must not be empty.")
	}

	if _, err := c.configManager.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID {
				bc := &cfg.Guilds[idx].PartnerBoard

				for _, p := range bc.Partners {
					if strings.EqualFold(p.Name, newName) && !strings.EqualFold(currentName, newName) {
						return errors.New("a partner with the new name already exists")
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
					return errors.New("partner not found")
				}
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerDetailedCommandError(fmt.Sprintf("Failed to update partner: %v", err))
	}

	c.partnerService.SyncConfig(ctx.GuildID, api.NewClient(ctx.Session.Token))
	return legacycore.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "Partner renamed successfully.")
}

// --- List ---
type partnerListSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerListSubCommand(cm *files.ConfigManager) *partnerListSubCommand {
	return &partnerListSubCommand{configManager: cm}
}

// Name names.
func (c *partnerListSubCommand) Name() string { return "list" }

// Description descriptions.
func (c *partnerListSubCommand) Description() string { return "List all partners on the board" }

// Options options.
func (c *partnerListSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }

// RequiresGuild requires guild.
func (c *partnerListSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerListSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *partnerListSubCommand) Handle(ctx *legacycore.Context) error {
	cfg := c.configManager.GuildConfig(ctx.GuildID)
	if cfg == nil {
		return partnerDetailedCommandError("Guild config not found.")
	}

	boardCfg := cfg.PartnerBoard
	if len(boardCfg.Partners) == 0 {
		return legacycore.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, "There are no partners configured for this server.")
	}

	var b strings.Builder
	for i, p := range boardCfg.Partners {
		b.WriteString(fmt.Sprintf("%d. `%s` | `%s` | %s\n", i+1, p.Name, p.Fandom, p.Link))
	}

	return legacycore.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Partner List").
		WithColor(theme.Info()).
		Info(ctx.Interaction, b.String())
}

// --- Refresh ---
type partnerRefreshSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerRefreshSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerRefreshSubCommand {
	return &partnerRefreshSubCommand{configManager: cm, partnerService: s}
}

// Name names.
func (c *partnerRefreshSubCommand) Name() string { return "refresh" }

// Description descriptions.
func (c *partnerRefreshSubCommand) Description() string { return "Refresh all active partner postings" }

// Options options.
func (c *partnerRefreshSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }

// RequiresGuild requires guild.
func (c *partnerRefreshSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerRefreshSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *partnerRefreshSubCommand) Handle(ctx *legacycore.Context) error {
	builder := legacycore.NewResponseBuilder(ctx.Session)
	if err := builder.Build().DeferResponse(ctx.Interaction, true); err != nil {
		return fmt.Errorf("partnerRefreshSubCommand.Handle: %w", err)
	}
	ctx.Acknowledged = true

	if err := c.partnerService.SyncConfig(ctx.GuildID, api.NewClient(ctx.Session.Token)); err != nil {
		return builder.WithContext(ctx).Error(ctx.Interaction, fmt.Sprintf("Failed to sync partner board: %v", err))
	}
	return builder.WithContext(ctx).Success(ctx.Interaction, "Partner board refreshed successfully.")
}

func autocompletePartnerNameFocused(ctx *legacycore.Context, cm *files.ConfigManager, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	extractor := legacycore.OptionList(legacycore.GetSubCommandOptions(ctx.Interaction))
	query := extractor.String(focusedOption)

	cfg := cm.GuildConfig(ctx.GuildID)
	if cfg == nil {
		return nil, nil
	}
	bc := cfg.PartnerBoard

	var choices []*discordgo.ApplicationCommandOptionChoice
	queryLower := strings.ToLower(query)

	for _, p := range bc.Partners {
		if queryLower == "" || strings.Contains(strings.ToLower(p.Name), queryLower) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
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

func autocompletePartnerName(ctx *legacycore.Context, cm *files.ConfigManager) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	return autocompletePartnerNameFocused(ctx, cm, optionName)
}
