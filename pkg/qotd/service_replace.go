package qotd

import (
	"context"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

func (s *Service) ReplaceCurrentPublish(ctx context.Context, guildID string, session *discordgo.Session) (*PublishResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrDiscordUnavailable
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	now := s.clock()
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return nil, err
	}
	today := NormalizePublishDateUTC(now)
	post, err := s.loadSlotOfficialPost(ctx, guildID, today)
	if err != nil {
		return nil, err
	}
	if post == nil {
		yesterday := today.AddDate(0, 0, -1)
		post, err = s.loadSlotOfficialPost(ctx, guildID, yesterday)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	if counts.Ready+counts.Draft == 0 {
		return nil, ErrNoQuestionsAvailable
	}

	// Delete from Discord (best effort, errors logged but ignored)
	if post.DiscordThreadID != "" {
		_, err := session.ChannelDelete(post.DiscordThreadID)
		if err != nil {
			log.ApplicationLogger().Warn("qotd skip failed to delete discord thread", "guildID", guildID, "threadID", post.DiscordThreadID, "err", err)
		}
	}
	if post.DiscordStarterMessageID != "" && post.ChannelID != "" {
		err := session.ChannelMessageDelete(post.ChannelID, post.DiscordStarterMessageID)
		if err != nil {
			log.ApplicationLogger().Warn("qotd skip failed to delete starter message", "guildID", guildID, "channelID", post.ChannelID, "messageID", post.DiscordStarterMessageID, "err", err)
		}
	}
	if post.QuestionListEntryMessageID != "" && post.QuestionListThreadID != "" {
		err := session.ChannelMessageDelete(post.QuestionListThreadID, post.QuestionListEntryMessageID)
		if err != nil {
			log.ApplicationLogger().Warn("qotd skip failed to delete question list entry", "guildID", guildID, "threadID", post.QuestionListThreadID, "messageID", post.QuestionListEntryMessageID, "err", err)
		}
	}

	// Delete from DB
	if err := s.store.DeleteQOTDOfficialPostByID(ctx, post.ID); err != nil {
		return nil, err
	}

	// We delete the question. The service layer normally checks for immutability,
	// but calling the store directly bypasses it which is intentional here.
	if err := s.store.DeleteQOTDQuestion(ctx, guildID, post.QuestionID); err != nil {
		return nil, err
	}

	// If this post was consuming the automatic slot, we need to clear the suppression
	// so the new publish can take the slot instead of failing as duplicate.
	if post.ConsumeAutomaticSlot {
		s.clearScheduledPublishSuppressionForDate(guildID, post.PublishDateUTC)
	}

	publishDate := post.PublishDateUTC
	return s.PublishNowWithParams(ctx, guildID, session, PublishNowParams{
		ConsumeAutomaticSlot: &post.ConsumeAutomaticSlot,
		PublishDateOverride:  &publishDate,
		IsReplacement:        true,
	})
}
