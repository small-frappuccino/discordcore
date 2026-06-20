package partners

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	discordErrUnknownChannel = 10003
	discordErrUnknownMessage = 10008
)

type partnerSyncFailure struct {
	Posting files.CustomEmbedPostingConfig
	Err     error
}

type partnerSyncResult struct {
	Edited  int
	Dropped []files.CustomEmbedPostingConfig
	Failed  []partnerSyncFailure
}

// HasIssues has issues.
func (r partnerSyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

type partnerPostingSyncer struct {
	configManager      *files.ConfigManager
	editMessage        func(c *api.Client, channelID discord.ChannelID, messageID discord.MessageID, edit api.EditMessageData) error
	editWebhookMessage func(c *api.Client, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID, edit api.EditMessageData) error
	dropPostings       func(cm *files.ConfigManager, guildID string, messageIDs []string) error
}

func newPartnerPostingSyncer(cm *files.ConfigManager) *partnerPostingSyncer {
	return &partnerPostingSyncer{
		configManager:      cm,
		editMessage:        defaultPartnerEditMessage,
		editWebhookMessage: defaultPartnerEditWebhookMessage,
		dropPostings:       defaultPartnerDropPostings,
	}
}

func (s *partnerPostingSyncer) Sync(
	client *api.Client,
	guildID string,
	postings []files.CustomEmbedPostingConfig,
	embeds []discord.Embed,
) partnerSyncResult {
	var result partnerSyncResult
	if len(postings) == 0 {
		return result
	}

	if embeds == nil {
		embeds = []discord.Embed{}
	}

	for _, posting := range postings {
		chID, _ := discord.ParseSnowflake(posting.ChannelID)
		msgID, _ := discord.ParseSnowflake(posting.MessageID)

		edit := api.EditMessageData{
			Embeds: &embeds,
		}
		var err error
		if posting.WebhookID != "" && posting.WebhookToken != "" {
			wID, _ := discord.ParseSnowflake(posting.WebhookID)
			err = s.editWebhookMessage(client, discord.WebhookID(wID), posting.WebhookToken, discord.MessageID(msgID), edit)
		} else {
			err = s.editMessage(client, discord.ChannelID(chID), discord.MessageID(msgID), edit)
		}
		if err == nil {
			result.Edited++
			continue
		}

		if isPartnerPostingMissingError(err) {
			result.Dropped = append(result.Dropped, posting)
			continue
		}

		result.Failed = append(result.Failed, partnerSyncFailure{Posting: posting, Err: err})
	}

	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		if dropErr := s.dropPostings(s.configManager, guildID, ids); dropErr != nil {
			slog.Warn("Partner board posting cleanup failed",
				"guildID", guildID,
				"err", dropErr,
			)
		}
	}

	return result
}

func (s *partnerPostingSyncer) SyncConfig(guildID string, client *api.Client) error {
	cfg := s.configManager.GuildConfig(guildID)
	if cfg == nil {
		return errors.New("guild config not found")
	}

	boardCfg := cfg.PartnerBoard
	var partners []PartnerRecord
	for _, p := range boardCfg.Partners {
		partners = append(partners, PartnerRecord{
			Fandom: p.Fandom,
			Name:   p.Name,
			Link:   p.Link,
		})
	}

	template := PartnerBoardTemplate{
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

	renderer := NewBoardRenderer()
	embeds, err := renderer.Render(template, partners)
	if err != nil {
		return fmt.Errorf("partnerPostingSyncer.SyncConfig: %w", err)
	}

	s.Sync(client, guildID, boardCfg.Postings, embeds)
	return nil
}

func formatPartnerSyncSummary(result partnerSyncResult, action string) string {
	if !result.HasIssues() && result.Edited == 0 {
		return ""
	}
	var lines []string
	if result.Edited > 0 {
		lines = append(lines, fmt.Sprintf("%s %d posting(s).", action, result.Edited))
	}
	if len(result.Dropped) > 0 {
		ids := make([]string, 0, len(result.Dropped))
		for _, p := range result.Dropped {
			ids = append(ids, p.MessageID)
		}
		lines = append(lines, fmt.Sprintf("Dropped %d orphaned posting(s) (message gone): %s.", len(result.Dropped), strings.Join(ids, ", ")))
	}
	if len(result.Failed) > 0 {
		details := make([]string, 0, len(result.Failed))
		for _, f := range result.Failed {
			details = append(details, fmt.Sprintf("message_id=%s (%v)", f.Posting.MessageID, f.Err))
		}
		lines = append(lines, fmt.Sprintf("Could not reconcile %d posting(s); these are kept on file for retry: %s.", len(result.Failed), strings.Join(details, "; ")))
	}
	return strings.Join(lines, "\n")
}

func isPartnerPostingMissingError(err error) bool {
	var httpErr *httputil.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code == discordErrUnknownChannel || httpErr.Code == discordErrUnknownMessage
	}
	return false
}

func defaultPartnerEditMessage(c *api.Client, channelID discord.ChannelID, messageID discord.MessageID, edit api.EditMessageData) error {
	if c == nil {
		return errors.New("discord client is nil")
	}
	_, err := c.EditMessageComplex(channelID, messageID, edit)
	return err
}

func defaultPartnerEditWebhookMessage(c *api.Client, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID, edit api.EditMessageData) error {
	if c == nil {
		return errors.New("discord client is nil")
	}
	whClient := webhook.New(webhookID, webhookToken)
	_, err := whClient.EditMessage(messageID, webhook.EditMessageData{
		Embeds: edit.Embeds,
	})
	return err
}

func defaultPartnerDropPostings(cm *files.ConfigManager, guildID string, messageIDs []string) error {
	if cm == nil {
		return errors.New("config manager is nil")
	}
	return cm.RemovePartnerBoardPostings(guildID, messageIDs)
}
