package partner

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
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
	editMessage        func(s *discordgo.Session, edit *discordgo.MessageEdit) error
	editWebhookMessage func(s *discordgo.Session, edit *discordgo.MessageEdit, webhookID, webhookToken string) error
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

// Sync syncs.
func (s *partnerPostingSyncer) Sync(
	session *discordgo.Session,
	guildID string,
	postings []files.CustomEmbedPostingConfig,
	embeds []*discordgo.MessageEmbed,
) partnerSyncResult {
	var result partnerSyncResult
	if len(postings) == 0 {
		return result
	}

	if embeds == nil {
		embeds = []*discordgo.MessageEmbed{}
	}

	for _, posting := range postings {
		edit := &discordgo.MessageEdit{
			ID:      strings.TrimSpace(posting.MessageID),
			Channel: strings.TrimSpace(posting.ChannelID),
			Embeds:  &embeds,
		}
		var err error
		if posting.WebhookID != "" && posting.WebhookToken != "" {
			err = s.editWebhookMessage(session, edit, posting.WebhookID, posting.WebhookToken)
		} else {
			err = s.editMessage(session, edit)
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

// SyncConfig syncs config.
func (s *partnerPostingSyncer) SyncConfig(guildID string, session *discordgo.Session) error {
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

	s.Sync(session, guildID, boardCfg.Postings, embeds)
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
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) || rest.Message == nil {
		return false
	}
	switch rest.Message.Code {
	case discordErrUnknownChannel, discordErrUnknownMessage:
		return true
	}
	return false
}

func defaultPartnerEditMessage(s *discordgo.Session, edit *discordgo.MessageEdit) error {
	if s == nil {
		return errors.New("discord session is nil")
	}
	_, err := s.ChannelMessageEditComplex(edit)
	return err
}

func defaultPartnerEditWebhookMessage(s *discordgo.Session, edit *discordgo.MessageEdit, webhookID, webhookToken string) error {
	if s == nil {
		return errors.New("discord session is nil")
	}
	_, err := s.WebhookMessageEdit(webhookID, webhookToken, edit.ID, &discordgo.WebhookEdit{
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
