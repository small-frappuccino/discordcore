package qotd

import (
	"context"
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// ReplaceCurrentPublish replaces current publish.
func (s *Service) ReplaceCurrentPublish(ctx context.Context, guildID string) (*PublishResult, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.ReplaceCurrentPublish: %w", err)
	}

	guildID = strings.TrimSpace(guildID)

	val, err := s.ExecuteInGuildActorWithResult(guildID, func() (any, error) {
		now := s.clock()
		cfg, err := s.configManager.QOTDConfig(guildID)
		if err != nil {
			return nil, fmt.Errorf("Service.ReplaceCurrentPublish: %w", err)
		}
		today := NormalizePublishDateUTC(now)
		post, err := s.loadSlotOfficialPost(ctx, guildID, today)
		if err != nil {
			return nil, fmt.Errorf("Service.ReplaceCurrentPublish: %w", err)
		}
		if post == nil {
			yesterday := today.AddDate(0, 0, -1)
			post, err = s.loadSlotOfficialPost(ctx, guildID, yesterday)
			if err != nil {
				return nil, fmt.Errorf("Service.ReplaceCurrentPublish: %w", err)
			}
		}

		if post == nil {
			return nil, ErrNoCurrentPublish
		}

		deck, ok := cfg.DeckByID(post.DeckID)
		if !ok || !deck.Enabled || !canPublishQOTD(deck) {
			return nil, ErrQOTDDisabled
		}

		// Guarantee that we can publish a new question before deleting the current one.
		counts, err := s.deckQuestionCounts(ctx, guildID, deck.ID)
		if err != nil {
			return nil, fmt.Errorf("Service.ReplaceCurrentPublish: %w", err)
		}
		if counts.Ready+counts.Draft == 0 {
			return nil, ErrNoQuestionsAvailable
		}

		// Delete from Discord (best effort, errors logged but ignored)
		err = s.publisher.DeleteOfficialPost(ctx, DeleteOfficialPostParams{
			GuildID:                    guildID,
			DiscordThreadID:            post.DiscordThreadID,
			DiscordStarterMessageID:    post.DiscordStarterMessageID,
			ChannelID:                  post.ChannelID,
			QuestionListThreadID:       post.QuestionListThreadID,
			QuestionListEntryMessageID: post.QuestionListEntryMessageID,
		})
		if err != nil {
			log.ApplicationLogger().Warn("qotd skip failed to delete discord thread/message", "guildID", guildID, "threadID", post.DiscordThreadID, "err", err)
		}

		// Delete from DB
		if err := s.store.DeleteQOTDOfficialPostByID(ctx, post.ID); err != nil {
			return nil, fmt.Errorf("Service.ReplaceCurrentPublish: %w", err)
		}

		// We delete the question. The service layer normally checks for immutability,
		// but calling the store directly bypasses it which is intentional here.
		if err := s.store.DeleteQOTDQuestion(ctx, guildID, post.QuestionID); err != nil {
			return nil, fmt.Errorf("Service.ReplaceCurrentPublish: %w", err)
		}

		// If this post was consuming the automatic slot, we need to clear the suppression
		// so the new publish can take the slot instead of failing as duplicate.
		if post.ConsumeAutomaticSlot {
			s.clearScheduledPublishSuppressionForDate(guildID, post.PublishDateUTC)
		}

		publishDate := post.PublishDateUTC
		return s.PublishNowWithParams(ctx, guildID, PublishNowParams{
			ConsumeAutomaticSlot: &post.ConsumeAutomaticSlot,
			PublishDateOverride:  &publishDate,
			IsReplacement:        true,
		})
	})

	if err != nil {
		return nil, err
	}
	return val.(*PublishResult), nil
}
