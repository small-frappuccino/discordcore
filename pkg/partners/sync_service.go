package partners

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/messageupdate"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// Renderer abstracts partner board rendering for the sync service.
type Renderer interface {
	Render(template PartnerBoardTemplate, partners []PartnerRecord) ([]*discordgo.MessageEmbed, error)
}

// BoardSyncService publishes rendered partner boards to configured update targets.
type BoardSyncService struct {
	configManager *files.ConfigManager
	renderer      Renderer
	updater       messageupdate.EmbedUpdater
}

// NewBoardSyncService creates a sync service with default renderer and updater.
func NewBoardSyncService(configManager *files.ConfigManager) *BoardSyncService {
	return NewBoardSyncServiceWithDependencies(
		configManager,
		NewBoardRenderer(),
		messageupdate.NewDefaultEmbedUpdater(),
	)
}

// NewBoardSyncServiceWithDependencies creates a sync service with explicit dependencies.
func NewBoardSyncServiceWithDependencies(
	configManager *files.ConfigManager,
	renderer Renderer,
	updater messageupdate.EmbedUpdater,
) *BoardSyncService {
	return &BoardSyncService{
		configManager: configManager,
		renderer:      renderer,
		updater:       updater,
	}
}

// SyncGuild renders and publishes the partner board for one guild.
func (s *BoardSyncService) SyncGuild(ctx context.Context, session *discordgo.Session, guildID string) error {
	if s == nil {
		return fmt.Errorf("sync partner board: service is nil")
	}
	if s.configManager == nil {
		return fmt.Errorf("sync partner board: config manager is nil")
	}
	if s.renderer == nil {
		return fmt.Errorf("sync partner board: renderer is nil")
	}
	if s.updater == nil {
		return fmt.Errorf("sync partner board: updater is nil")
	}
	if session == nil {
		return fmt.Errorf("sync partner board: discord session is nil")
	}

	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return fmt.Errorf("sync partner board: guild_id is required")
	}

	if err := contextErr(ctx); err != nil {
		return fmt.Errorf("sync partner board guild_id=%s: %w", guildID, err)
	}

	log.ApplicationLogger().Info(
		"Starting partner board sync",
		"guild_id", guildID,
	)

	board, err := s.configManager.GetPartnerBoard(guildID)
	if err != nil {
		wrapped := fmt.Errorf("sync partner board guild_id=%s: load board config: %w", guildID, err)
		log.ApplicationLogger().Error("Partner board sync failed", "guild_id", guildID, "err", wrapped)
		return wrapped
	}

	target, err := toMessageUpdateTarget(board.Target)
	if err != nil {
		wrapped := fmt.Errorf("sync partner board guild_id=%s: resolve target: %w", guildID, err)
		log.ApplicationLogger().Error("Partner board sync failed", "guild_id", guildID, "err", wrapped)
		return wrapped
	}

	template := toPartnerTemplate(board.Template)
	records := toPartnerRecords(board.Partners)

	embeds, err := s.renderer.Render(template, records)
	if err != nil {
		wrapped := fmt.Errorf("sync partner board guild_id=%s: render embeds: %w", guildID, err)
		log.ApplicationLogger().Error(
			"Partner board sync failed",
			"guild_id", guildID,
			"target_type", target.Type,
			"message_id", target.MessageID,
			"err", wrapped,
		)
		return wrapped
	}

	if err := contextErr(ctx); err != nil {
		return fmt.Errorf("sync partner board guild_id=%s: %w", guildID, err)
	}

	if err := s.updater.UpdateEmbeds(session, target, embeds); err != nil {
		wrapped := fmt.Errorf("sync partner board guild_id=%s: publish embeds: %w", guildID, err)
		log.ApplicationLogger().Error(
			"Partner board sync failed",
			"guild_id", guildID,
			"target_type", target.Type,
			"message_id", target.MessageID,
			"channel_id", target.ChannelID,
			"err", wrapped,
		)
		return wrapped
	}

	log.ApplicationLogger().Info(
		"Partner board sync completed",
		"guild_id", guildID,
		"target_type", target.Type,
		"message_id", target.MessageID,
		"channel_id", target.ChannelID,
		"embed_count", len(embeds),
		"partner_count", len(records),
	)
	return nil
}

func toMessageUpdateTarget(target files.EmbedUpdateTargetConfig) (messageupdate.EmbedUpdateTarget, error) {
	resolved, err := (messageupdate.EmbedUpdateTarget{
		Type:       strings.TrimSpace(target.Type),
		MessageID:  strings.TrimSpace(target.MessageID),
		ChannelID:  strings.TrimSpace(target.ChannelID),
		WebhookURL: strings.TrimSpace(target.WebhookURL),
	}).Normalize()
	if err != nil {
		return messageupdate.EmbedUpdateTarget{}, err
	}
	return resolved, nil
}

func toPartnerTemplate(in files.PartnerBoardTemplateConfig) PartnerBoardTemplate {
	return PartnerBoardTemplate{
		Title:                      strings.TrimSpace(in.Title),
		ContinuationTitle:          strings.TrimSpace(in.ContinuationTitle),
		Intro:                      strings.TrimSpace(in.Intro),
		SectionHeaderTemplate:      strings.TrimSpace(in.SectionHeaderTemplate),
		SectionContinuationSuffix:  strings.TrimSpace(in.SectionContinuationSuffix),
		SectionContinuationPattern: strings.TrimSpace(in.SectionContinuationPattern),
		LineTemplate:               strings.TrimSpace(in.LineTemplate),
		EmptyStateText:             strings.TrimSpace(in.EmptyStateText),
		FooterTemplate:             strings.TrimSpace(in.FooterTemplate),
		OtherFandomLabel:           strings.TrimSpace(in.OtherFandomLabel),
		Color:                      in.Color,
		DisableFandomSorting:       in.DisableFandomSorting,
		DisablePartnerSorting:      in.DisablePartnerSorting,
	}
}

func toPartnerRecords(items []files.PartnerEntryConfig) []PartnerRecord {
	if len(items) == 0 {
		return nil
	}
	out := make([]PartnerRecord, 0, len(items))
	for _, item := range items {
		out = append(out, PartnerRecord{
			Fandom: strings.TrimSpace(item.Fandom),
			Name:   strings.TrimSpace(item.Name),
			Link:   strings.TrimSpace(item.Link),
		})
	}
	return out
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("context done: %w", err)
	}
	return nil
}
