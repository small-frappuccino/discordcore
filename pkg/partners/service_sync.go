package partners

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type PartnerSyncFailure struct {
	Posting files.CustomEmbedPostingConfig
	Err     error
}

type PartnerSyncResult struct {
	Edited  int
	Dropped []files.CustomEmbedPostingConfig
	Failed  []PartnerSyncFailure
}

func (r PartnerSyncResult) HasIssues() bool {
	return len(r.Dropped) > 0 || len(r.Failed) > 0
}

type partnerPostingSyncer struct {
	configManager *files.ConfigManager
	publisher     BoardPublisher
	dropPostings  func(cm *files.ConfigManager, guildID string, messageIDs []string) error
}

func newPartnerPostingSyncer(cm *files.ConfigManager, publisher BoardPublisher) *partnerPostingSyncer {
	return &partnerPostingSyncer{
		configManager: cm,
		publisher:     publisher,
		dropPostings:  defaultPartnerDropPostings,
	}
}

func (s *partnerPostingSyncer) Sync(
	guildID string,
	postings []files.CustomEmbedPostingConfig,
	embeds []BoardEmbed,
) PartnerSyncResult {
	if len(postings) == 0 {
		return PartnerSyncResult{}
	}

	result := s.publisher.Publish(guildID, postings, embeds)

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

func (s *partnerPostingSyncer) SyncConfig(guildID string) error {
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

	s.Sync(guildID, boardCfg.Postings, embeds)
	return nil
}

func formatPartnerSyncSummary(result PartnerSyncResult, action string) string {
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

func defaultPartnerDropPostings(cm *files.ConfigManager, guildID string, messageIDs []string) error {
	if cm == nil {
		return errors.New("config manager is nil")
	}
	return cm.RemovePartnerBoardPostings(guildID, messageIDs)
}

