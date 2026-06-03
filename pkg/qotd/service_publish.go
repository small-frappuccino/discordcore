package qotd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
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

	val, err := s.ExecuteInGuildActorWithResult(guildID, func() (any, error) {
		now := s.clock()
		cfg, err := s.configManager.QOTDConfig(guildID)
		if err != nil {
			return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
		}
		slotState, err := s.loadCurrentSlotState(ctx, guildID, cfg, now)
		if err != nil {
			return nil, fmt.Errorf("Service.PublishNowWithParams: %w", err)
		}

		rollbackSuppressionDate := time.Time{}
		keepSuppression := false
		defer func() {
			if keepSuppression || rollbackSuppressionDate.IsZero() {
				return
			}
			s.clearScheduledPublishSuppressionForDate(guildID, rollbackSuppressionDate)
		}()

		publishDate, existing, consumeAutomaticSlot, err := s.resolveManualPublishTarget(ctx, guildID, cfg, slotState, params, now)
		if err != nil {
			return nil, err
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

		publishResult, keep, err := s.provisionManualOfficialPost(ctx, publishScope{GuildID: guildID, Session: session, Now: now}, deck, publishDate, consumeAutomaticSlot)
		keepSuppression = keep
		if err != nil {
			return nil, err
		}
		return publishResult, nil
	})

	if err != nil {
		return nil, err
	}
	return val.(*PublishResult), nil
}

// resolveManualPublishTarget determines the publish date, any existing official-post record
// at that date, and whether this manual publish consumes the automatic slot, based on the
// publish params and the current slot state.
func (s *Service) resolveManualPublishTarget(ctx context.Context, guildID string, cfg files.QOTDConfig, slotState currentSlotState, params PublishNowParams, now time.Time) (publishDate time.Time, existing *storage.QOTDOfficialPostRecord, consumeAutomaticSlot bool, err error) {
	consumeAutomaticSlot = params.ShouldConsumeAutomaticSlot()
	publishDate = NormalizePublishDateUTC(now)

	if params.PublishDateOverride != nil {
		publishDate = NormalizePublishDateUTC(*params.PublishDateOverride)
		existing, err = s.loadSlotOfficialPost(ctx, guildID, publishDate)
		if err != nil {
			return time.Time{}, nil, false, fmt.Errorf("Service.PublishNowWithParams: %w", err)
		}
		return publishDate, existing, consumeAutomaticSlot, nil
	}
	if slotState.ScheduleConfigured {
		if consumeAutomaticSlot {
			publishDate = slotState.PublishDateUTC
			existing = slotState.OfficialPost
		}
		return publishDate, existing, consumeAutomaticSlot, nil
	}
	existing, err = s.loadSlotOfficialPost(ctx, guildID, publishDate)
	if err != nil {
		return time.Time{}, nil, false, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	return publishDate, existing, consumeAutomaticSlot, nil
}

// provisionManualOfficialPost reserves a question and provisions the manual official post,
// completing the Discord publish and realigning the window. The bool result reports whether
// the caller should keep any applied scheduled-slot suppression (true only on a fully
// successful publish). On a provisioning conflict it adopts the existing record via
// resolvePublishNowConflict.
func (s *Service) provisionManualOfficialPost(ctx context.Context, scope publishScope, deck files.QOTDDeckConfig, publishDate time.Time, consumeAutomaticSlot bool) (*PublishResult, bool, error) {
	question, err := s.store.ReserveNextReadyQOTDQuestion(ctx, scope.GuildID, deck.ID, storage.QOTDQuestionSelectorQueue)
	if err != nil {
		return nil, false, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	if question == nil {
		return nil, false, ErrNoQuestionsAvailable
	}
	counts, err := s.deckQuestionCounts(ctx, scope.GuildID, deck.ID)
	if err != nil {
		s.releaseManualReservedQuestion(ctx, scope.GuildID, *question)
		return nil, false, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	availableQuestions := counts.Ready + counts.Draft

	lifecycle := EvaluateManualOfficialPost(scope.Now, scope.Now)
	nonce, err := generatePublishNonce()
	if err != nil {
		s.releaseManualReservedQuestion(ctx, scope.GuildID, *question)
		return nil, false, fmt.Errorf("generate qotd publish nonce: %w", err)
	}
	provisioned, err := s.store.CreateQOTDOfficialPostProvisioning(ctx, storage.QOTDOfficialPostRecord{
		GuildID:              scope.GuildID,
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
		s.releaseManualReservedQuestion(ctx, scope.GuildID, *question)
		conflictResult, conflictErr := s.resolvePublishNowConflict(ctx, scope.GuildID, publishDate, err)
		return conflictResult, false, conflictErr
	}

	finalized, updatedQuestion, postURL, err := s.completeOfficialPostProvisioning(ctx, scope.Session, officialPostProvisioningParams{
		Post:               *provisioned,
		Question:           question,
		AvailableQuestions: availableQuestions,
		ThreadName:         buildOfficialThreadName(threadDisplayNumberFromUsedCount(counts.Used, question)),
		Now:                scope.Now,
	})
	if err != nil {
		return nil, false, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	if updatedQuestion != nil {
		question = updatedQuestion
	}

	if err := s.reconcileOfficialPostWindow(ctx, scope.GuildID, scope.Session, scope.Now, finalized.ID); err != nil {
		return nil, false, fmt.Errorf("Service.PublishNowWithParams: %w", err)
	}
	if consumeAutomaticSlot {
		s.clearScheduledPublishSuppressionForDate(scope.GuildID, publishDate)
	}

	return &PublishResult{
		Question:     *question,
		OfficialPost: *finalized,
		PostURL:      postURL,
	}, true, nil
}

// releaseManualReservedQuestion returns a reserved question to the pool on a manual-publish
// error path, logging but not failing on release errors.
func (s *Service) releaseManualReservedQuestion(ctx context.Context, guildID string, question storage.QOTDQuestionRecord) {
	if releaseErr := s.releaseReservedQuestion(ctx, question); releaseErr != nil {
		log.ApplicationLogger().Warn("QOTD question reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
	}
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
