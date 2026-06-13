package partner

import (
	"errors"
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	partnersvc "github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordgo"
)

// --- Post ---
type partnerPostSubCommand struct {
	configManager  *files.ConfigManager
	partnerService *partnersvc.PartnerService
}

func newPartnerPostSubCommand(cm *files.ConfigManager, s *partnersvc.PartnerService) *partnerPostSubCommand {
	return &partnerPostSubCommand{configManager: cm, partnerService: s}
}

// Name names.
func (c *partnerPostSubCommand) Name() string { return "post" }

// Description descriptions.
func (c *partnerPostSubCommand) Description() string {
	return "Publish the partner board to a channel or webhook"
}

// Options options.
func (c *partnerPostSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionChannel, Name: "channel", Description: "Target channel", Required: false},
		{Type: discordgo.ApplicationCommandOptionString, Name: optionWebhookURL, Description: "Target webhook URL", Required: false},
	}
}

// RequiresGuild requires guild.
func (c *partnerPostSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerPostSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *partnerPostSubCommand) Handle(ctx *core.Context) error {
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	channelID := ""
	for _, opt := range core.GetSubCommandOptions(ctx.Interaction) {
		if opt.Name == "channel" {
			if chVal, ok := opt.Value.(string); ok && strings.TrimSpace(chVal) != "" {
				channelID = strings.TrimSpace(chVal)
			}
		}
	}
	webhookURL := extractor.String(optionWebhookURL)

	if channelID == "" && webhookURL == "" {
		channelID = ctx.Interaction.ChannelID
	}

	cfg := c.configManager.GuildConfig(ctx.GuildID)
	if cfg == nil {
		return partnerDetailedCommandError("Guild config not found.")
	}

	boardCfg := cfg.PartnerBoard
	var partners []partnersvc.PartnerRecord
	for _, p := range boardCfg.Partners {
		partners = append(partners, partnersvc.PartnerRecord{
			Fandom: p.Fandom,
			Name:   p.Name,
			Link:   p.Link,
		})
	}

	template := partnersvc.PartnerBoardTemplate{
		Title:                      boardCfg.Template.Title,
		ContinuationTitle:          boardCfg.Template.ContinuationTitle,
		Intro:                      boardCfg.Template.Intro,
		SectionHeaderTemplate:      boardCfg.Template.SectionHeaderTemplate,
		SectionContinuationSuffix:  boardCfg.Template.SectionContinuationSuffix,
		SectionContinuationPattern: boardCfg.Template.SectionContinuationPattern,
		LineTemplate:               boardCfg.Template.LineTemplate,
		EmptyStateText:             boardCfg.Template.EmptyStateText,
		FooterTemplate:             boardCfg.Template.FooterTemplate,
		OtherFandomLabel:           boardCfg.Template.OtherFandomLabel,
		Color:                      boardCfg.Template.Color,
		DisableFandomSorting:       boardCfg.Template.DisableFandomSorting,
		DisablePartnerSorting:      boardCfg.Template.DisablePartnerSorting,
	}

	embeds, err := c.partnerService.Render(template, partners)
	if err != nil || len(embeds) == 0 {
		return partnerDetailedCommandError(fmt.Sprintf("Failed to render partner board: %v", err))
	}

	var discordEmbeds []*discordgo.MessageEmbed
	for _, e := range embeds {
		discordEmbeds = append(discordEmbeds, &discordgo.MessageEmbed{
			Title:       e.Title,
			Description: e.Description,
			Color:       e.Color,
			Footer:      &discordgo.MessageEmbedFooter{Text: e.FooterText},
		})
	}

	var posting files.CustomEmbedPostingConfig
	postingNote := ""

	if webhookURL != "" {
		wID, wToken, ok := parseWebhookURL(webhookURL)
		if !ok {
			return partnerDetailedCommandError("Invalid webhook URL.")
		}
		message, err := ctx.Session.WebhookExecute(wID, wToken, true, &discordgo.WebhookParams{
			Embeds: discordEmbeds,
		})
		if err != nil {
			return partnerDetailedCommandError(fmt.Sprintf("Failed to post the embed via webhook: %v", err))
		}
		if message != nil && message.ID != "" {
			posting = files.CustomEmbedPostingConfig{
				ChannelID:    message.ChannelID,
				MessageID:    message.ID,
				WebhookID:    wID,
				WebhookToken: wToken,
			}
		}
	} else {
		message, err := ctx.Session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Embeds: discordEmbeds,
		})
		if err != nil {
			return partnerDetailedCommandError(fmt.Sprintf("Failed to post the embed: %v", err))
		}
		if message != nil && message.ID != "" {
			posting = files.CustomEmbedPostingConfig{
				ChannelID: channelID,
				MessageID: message.ID,
			}
		}
	}

	if !posting.IsZero() {
		if _, err := c.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
			for i := range cfg.Guilds {
				if cfg.Guilds[i].GuildID == ctx.GuildID {
					cfg.Guilds[i].PartnerBoard.Postings = append(cfg.Guilds[i].PartnerBoard.Postings, posting)
					return nil
				}
			}
			return errors.New("guild not found in config")
		}); err != nil {
			postingNote = fmt.Sprintf("\nWarning: the posting could not be tracked for later updates: %v", err)
		}
	}

	var successMessage string
	if webhookURL != "" {
		successMessage = fmt.Sprintf("Partner board was posted via webhook.%s", postingNote)
	} else {
		successMessage = fmt.Sprintf("Partner board was posted in <#%s>.%s", channelID, postingNote)
	}

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, successMessage)
}

// --- Unpost ---
type partnerUnpostSubCommand struct {
	configManager *files.ConfigManager
}

func newPartnerUnpostSubCommand(cm *files.ConfigManager) *partnerUnpostSubCommand {
	return &partnerUnpostSubCommand{configManager: cm}
}

// Name names.
func (c *partnerUnpostSubCommand) Name() string { return "unpost" }

// Description descriptions.
func (c *partnerUnpostSubCommand) Description() string {
	return "Stop tracking and remove a partner board posting"
}

// Options options.
func (c *partnerUnpostSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{Type: discordgo.ApplicationCommandOptionString, Name: optionMessageID, Description: "Message ID of the posting to remove", Required: true},
	}
}

// RequiresGuild requires guild.
func (c *partnerUnpostSubCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *partnerUnpostSubCommand) RequiresPermissions() bool { return true }

// Handle handles.
func (c *partnerUnpostSubCommand) Handle(ctx *core.Context) error {
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))
	messageID, _ := extractor.StringRequired(optionMessageID)

	cfg := c.configManager.GuildConfig(ctx.GuildID)
	if cfg == nil {
		return partnerDetailedCommandError("Guild config not found.")
	}

	foundPosting := false
	var targetPosting files.CustomEmbedPostingConfig

	if _, err := c.configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID == ctx.GuildID {
				bc := &cfg.Guilds[idx].PartnerBoard
				for i, posting := range bc.Postings {
					if posting.MessageID == messageID {
						targetPosting = posting
						foundPosting = true
						copy(bc.Postings[i:], bc.Postings[i+1:])
						bc.Postings[len(bc.Postings)-1] = files.CustomEmbedPostingConfig{}
						bc.Postings = bc.Postings[:len(bc.Postings)-1]
						break
					}
				}
				if !foundPosting {
					return errors.New("posting not found in tracking list")
				}
				return nil
			}
		}
		return errors.New("guild not found in config")
	}); err != nil {
		return partnerDetailedCommandError(fmt.Sprintf("Failed to remove posting from tracking list: %v", err))
	}

	deleteErr := ""
	if targetPosting.WebhookID != "" && targetPosting.WebhookToken != "" {
		if err := ctx.Session.WebhookMessageDelete(targetPosting.WebhookID, targetPosting.WebhookToken, targetPosting.MessageID); err != nil {
			deleteErr = fmt.Sprintf("\nNote: Discord deletion via webhook failed (%v). You may need to delete it manually.", err)
		}
	} else if targetPosting.ChannelID != "" {
		if err := ctx.Session.ChannelMessageDelete(targetPosting.ChannelID, targetPosting.MessageID); err != nil {
			deleteErr = fmt.Sprintf("\nNote: Discord deletion failed (%v). You may need to delete it manually.", err)
		}
	}

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("The partner board posting has been removed from tracking.%s", deleteErr))
}
