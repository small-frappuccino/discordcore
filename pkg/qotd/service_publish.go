package qotd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func (s *Service) PublishNow(ctx context.Context, guildID string, session *discordgo.Session) (*PublishResult, error) {
	return s.PublishNowWithParams(ctx, guildID, session, PublishNowParams{})
}

func (s *Service) PublishNowWithParams(ctx context.Context, guildID string, session *discordgo.Session, params PublishNowParams) (result *PublishResult, err error) {

	publishStart := time.Now()
	s.observability().RecordPublishAttempt(PublishModeManual)
	defer func() {
		duration := time.Since(publishStart)
		if err != nil {
			s.observability().RecordPublishFailure(PublishModeManual, ClassifyPublishFailure(err), duration)
			return
		}
		s.observability().RecordPublishSuccess(PublishModeManual, duration)
	}()

	if err = s.validate(); err != nil {
		return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	if session == nil {
		err = ErrDiscordUnavailable
		return nil, err
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	now := s.clock()
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	slotState, err := s.loadCurrentSlotState(ctx, guildID, cfg, now)
	if err != nil {
		return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	consumeAutomaticSlot := params.ShouldConsumeAutomaticSlot()
	publishDate := NormalizePublishDateUTC(now)
	rollbackSuppressionDate := time.Time{}
	keepSuppression := false
	defer func() {
		if keepSuppression || rollbackSuppressionDate.IsZero() {
			return
		}
		s.clearScheduledPublishSuppressionForDate(guildID, rollbackSuppressionDate)
	}()
	var existing *storage.QOTDOfficialPostRecord
	if params.PublishDateOverride != nil {
		publishDate = NormalizePublishDateUTC(*params.PublishDateOverride)
		existing, err = s.loadSlotOfficialPost(ctx, guildID, publishDate)
		if err != nil {
			return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
		}
	} else if slotState.ScheduleConfigured {
		if consumeAutomaticSlot {
			publishDate = slotState.PublishDateUTC
			existing = slotState.OfficialPost
		}
	} else {
		existing, err = s.loadSlotOfficialPost(ctx, guildID, publishDate)
		if err != nil {
			return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
		}
	}
	deck, ok := cfg.ActiveDeck()
	if !ok || !deck.Enabled || !canPublishQOTD(deck) {
		return nil, ErrQOTDDisabled
	}
	if existing != nil {
		if isOfficialPostPublished(*existing) {
			return nil, ErrAlreadyPublished
		}
		return nil, ErrPublishInProgress
	}

	todayDate := NormalizePublishDateUTC(now)
	if !params.IsReplacement && consumeAutomaticSlot && slotState.ScheduleConfigured && !todayDate.Equal(publishDate) {
		if err := s.suppressScheduledPublishForDate(guildID, todayDate); err != nil {
			log.ApplicationLogger().Warn(
				"QOTD manual publish failed to suppress today's scheduled slot",
				"guildID", guildID,
				"todayDateUTC", todayDate,
				"err", err,
			)
		} else {
			rollbackSuppressionDate = todayDate
		}
	}

	question, err := s.store.ReserveNextReadyQOTDQuestion(ctx, guildID, deck.ID, storage.QOTDQuestionSelectorQueue)
	if err != nil {
		return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	if question == nil {
		return nil, ErrNoQuestionsAvailable
	}
	counts, err := s.deckQuestionCounts(ctx, guildID, deck.ID)
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD question reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	availableQuestions := counts.Ready + counts.Draft

	lifecycle := EvaluateManualOfficialPost(now, now)
	nonce, err := generatePublishNonce()
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD question reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return nil, fmt.Errorf("generate qotd publish nonce: %w", err)
	}
	provisioned, err := s.store.CreateQOTDOfficialPostProvisioning(ctx, storage.QOTDOfficialPostRecord{
		GuildID:              guildID,
		DeckID:               deck.ID,
		DeckNameSnapshot:     deck.Name,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeManual),
		ConsumeAutomaticSlot: consumeAutomaticSlot,
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateProvisioning),
		ChannelID:            strings.TrimSpace(deck.ChannelID),
		QuestionTextSnapshot: question.Body,
		Nonce:                nonce,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD question reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return s.resolvePublishNowConflict(ctx, guildID, publishDate, err)
	}

	finalized, updatedQuestion, postURL, err := s.completeOfficialPostProvisioning(
		ctx,
		session,
		*provisioned,
		question,
		availableQuestions,
		buildOfficialThreadName(threadDisplayNumberFromUsedCount(counts.Used, question)),
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	if updatedQuestion != nil {
		question = updatedQuestion
	}

	if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, finalized.ID); err != nil {
		return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	if consumeAutomaticSlot {
		s.clearScheduledPublishSuppressionForDate(guildID, publishDate)
	}
	keepSuppression = true

	return &PublishResult{
		Question:     *question,
		OfficialPost: *finalized,
		PostURL:      postURL,
	}, nil
}

func (s *Service) resolvePublishNowConflict(ctx context.Context, guildID string, publishDate time.Time, err error) (*PublishResult, error) {
	existing, lookupErr := s.lookupPublishConflictPost(ctx, guildID, publishDate, err)
	if lookupErr != nil {
		return nil, lookupErr
	}
	if isOfficialPostPublished(*existing) {
		s.clearScheduledPublishSuppressionForDate(guildID, publishDate)
		return s.publishResultFromOfficialPost(ctx, *existing)
	}
	return nil, ErrPublishInProgress
}

func (s *Service) publishResultFromOfficialPost(ctx context.Context, post storage.QOTDOfficialPostRecord) (*PublishResult, error) {
	question, err := s.store.GetQOTDQuestion(ctx, post.GuildID, post.QuestionID)
	if err != nil {
		return nil, fmt.Errorf("Service.publishResultFromOfficialPost: %w", err)
	}

	resultQuestion := storage.QOTDQuestionRecord{
		ID:      post.QuestionID,
		GuildID: post.GuildID,
		DeckID:  post.DeckID,
		Body:    post.QuestionTextSnapshot,
		Status:  string(QuestionStatusUsed),
		UsedAt:  post.PublishedAt,
	}
	if question != nil {
		resultQuestion = *question
	}

	return &PublishResult{
		Question:     resultQuestion,
		OfficialPost: post,
		PostURL:      OfficialPostJumpURL(post),
	}, nil
}

func (s *Service) lookupPublishConflictPost(ctx context.Context, guildID string, publishDate time.Time, err error) (*storage.QOTDOfficialPostRecord, error) {
	if !isQOTDScheduledPublishConflict(err) {
		return nil, err
	}

	existing, lookupErr := s.loadSlotOfficialPost(ctx, guildID, publishDate)
	if lookupErr != nil {
		return nil, lookupErr
	}
	if existing == nil {
		return nil, err
	}
	return existing, nil
}
