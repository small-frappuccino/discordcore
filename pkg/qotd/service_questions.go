package qotd

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func (s *Service) ListQuestions(ctx context.Context, guildID, deckID string) ([]storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.ListQuestions: %w", err)
	}
	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return nil, fmt.Errorf("Service.ListQuestions: %w", err)
	}
	return s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
}

func (s *Service) CreateQuestion(ctx context.Context, guildID, actorID string, mutation QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.CreateQuestion: %w", err)
	}
	deck, err := s.resolveDashboardDeck(guildID, mutation.DeckID)
	if err != nil {
		return nil, fmt.Errorf("Service.CreateQuestion: %w", err)
	}
	body, status, err := normalizeQuestionMutation(mutation)
	if err != nil {
		return nil, fmt.Errorf("Service.CreateQuestion: %w", err)
	}

	return s.store.CreateQOTDQuestion(ctx, storage.QOTDQuestionRecord{
		GuildID:   strings.TrimSpace(guildID),
		DeckID:    deck.ID,
		Body:      body,
		Status:    string(status),
		CreatedBy: normalizeActorID(actorID),
	})
}

func (s *Service) CreateQuestionsBatch(ctx context.Context, guildID, actorID string, mutations []QuestionMutation) ([]storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.CreateQuestionsBatch: %w", err)
	}

	guildID = strings.TrimSpace(guildID)
	var created []storage.QOTDQuestionRecord

	for _, mutation := range mutations {
		deck, err := s.resolveDashboardDeck(guildID, mutation.DeckID)
		if err != nil {
			return created, fmt.Errorf("Service.CreateQuestionsBatch: %w", err)
		}
		body, status, err := normalizeQuestionMutation(mutation)
		if err != nil {
			return created, fmt.Errorf("Service.CreateQuestionsBatch: %w", err)
		}

		record, err := s.store.CreateQOTDQuestion(ctx, storage.QOTDQuestionRecord{
			GuildID:   guildID,
			DeckID:    deck.ID,
			Body:      body,
			Status:    string(status),
			CreatedBy: normalizeActorID(actorID),
		})
		if err != nil {
			return created, fmt.Errorf("Service.CreateQuestionsBatch: %w", err)
		}
		created = append(created, *record)
	}

	return created, nil
}

func (s *Service) UpdateQuestion(ctx context.Context, guildID string, questionID int64, mutation QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.UpdateQuestion: %w", err)
	}
	current, err := s.store.GetQOTDQuestion(ctx, guildID, questionID)
	if err != nil {
		return nil, fmt.Errorf("Service.UpdateQuestion: %w", err)
	}
	if current == nil {
		return nil, ErrQuestionNotFound
	}
	if isImmutableQuestion(*current) {
		return nil, ErrImmutableQuestion
	}

	deckID := strings.TrimSpace(mutation.DeckID)
	if deckID == "" {
		deckID = current.DeckID
	} else if _, err := s.resolveDashboardDeck(guildID, deckID); err != nil {
		return nil, err
	}
	body, status, err := normalizeQuestionMutation(mutation)
	if err != nil {
		return nil, fmt.Errorf("Service.UpdateQuestion: %w", err)
	}

	current.DeckID = deckID
	current.Body = body
	current.Status = string(status)
	return s.store.UpdateQOTDQuestion(ctx, *current)
}

func (s *Service) DeleteQuestion(ctx context.Context, guildID string, questionID int64) error {
	if err := s.validate(); err != nil {
		return fmt.Errorf("Service.DeleteQuestion: %w", err)
	}
	current, err := s.store.GetQOTDQuestion(ctx, guildID, questionID)
	if err != nil {
		return fmt.Errorf("Service.DeleteQuestion: %w", err)
	}
	if current == nil {
		return ErrQuestionNotFound
	}
	if isImmutableQuestion(*current) {
		return ErrImmutableQuestion
	}
	return s.store.DeleteQOTDQuestion(ctx, guildID, questionID)
}

func (s *Service) GetAutomaticQueueState(ctx context.Context, guildID, deckID string) (AutomaticQueueState, error) {
	if err := s.validate(); err != nil {
		return AutomaticQueueState{}, fmt.Errorf("Service.GetAutomaticQueueState: %w", err)
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return AutomaticQueueState{}, fmt.Errorf("Service.GetAutomaticQueueState: %w", err)
	}

	state := AutomaticQueueState{Deck: deck}
	now := s.clock()
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return AutomaticQueueState{}, fmt.Errorf("Service.GetAutomaticQueueState: %w", err)
	}
	state.NextReadyQuestion = firstReadyUnscheduledQuestion(questions)

	settings, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return AutomaticQueueState{}, fmt.Errorf("Service.GetAutomaticQueueState: %w", err)
	}
	slotState, err := s.loadUpcomingSlotState(ctx, guildID, settings, now)
	if err != nil {
		return AutomaticQueueState{}, fmt.Errorf("Service.GetAutomaticQueueState: %w", err)
	}
	if slotState.ScheduleConfigured {
		state.ScheduleConfigured = true
		state.Schedule = slotState.Schedule
		state.SlotDateUTC = slotState.PublishDateUTC
		state.SlotPublishAtUTC = slotState.PublishAtUTC
		state.CanPublish = deck.Enabled && canPublishQOTD(deck)
		state.SlotOfficialPost = slotState.OfficialPost
		if slotState.OfficialPost != nil {
			state.SlotQuestion = questionByID(questions, slotState.OfficialPost.QuestionID)
		}
		if state.SlotQuestion == nil {
			state.SlotQuestion = reservedQuestionForDate(questions, state.SlotDateUTC)
		}
	} else {
		state.CanPublish = false
	}

	switch {
	case !state.ScheduleConfigured || !state.CanPublish:
		state.SlotStatus = AutomaticQueueSlotStatusDisabled
	case state.SlotOfficialPost != nil:
		if hasPublishedOfficialPostTarget(state.SlotOfficialPost) {
			state.SlotStatus = AutomaticQueueSlotStatusPublished
		} else {
			state.SlotStatus = AutomaticQueueSlotStatusRecovering
		}
	case state.SlotQuestion != nil:
		state.SlotStatus = AutomaticQueueSlotStatusReserved
	case now.Before(state.SlotPublishAtUTC):
		state.SlotStatus = AutomaticQueueSlotStatusWaiting
	default:
		state.SlotStatus = AutomaticQueueSlotStatusDue
	}

	return state, nil
}

func (s *Service) RestoreUsedQuestion(ctx context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.RestoreUsedQuestion: %w", err)
	}

	if questionID <= 0 {
		return nil, fmt.Errorf("%w: question id must be greater than zero", files.ErrInvalidQOTDInput)
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return nil, fmt.Errorf("Service.RestoreUsedQuestion: %w", err)
	}

	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return nil, fmt.Errorf("Service.RestoreUsedQuestion: %w", err)
	}
	if len(questions) == 0 {
		return nil, ErrQuestionNotFound
	}

	movedIndex := -1
	firstMutableIndex := -1
	for idx, question := range questions {
		if firstMutableIndex < 0 && !isImmutableQuestion(question) {
			firstMutableIndex = idx
		}
		if question.ID == questionID {
			movedIndex = idx
		}
	}
	if movedIndex < 0 {
		return nil, ErrQuestionNotFound
	}

	question := &questions[movedIndex]
	if question.DeckID != deck.ID {
		return nil, ErrQuestionNotFound
	}
	if QuestionStatus(strings.TrimSpace(question.Status)) != QuestionStatusUsed {
		return nil, ErrQuestionNotUsed
	}

	question.Status = string(QuestionStatusReady)
	question.UsedAt = nil
	question.ScheduledForDateUTC = nil
	question.PublishedOnceAt = nil

	if _, err := s.store.UpdateQOTDQuestion(ctx, *question); err != nil {
		return nil, fmt.Errorf("Service.RestoreUsedQuestion: %w", err)
	}

	if firstMutableIndex >= 0 && movedIndex > firstMutableIndex {
		orderedIDs := reorderQuestionIDsToIndex(questions, movedIndex, firstMutableIndex)
		if len(orderedIDs) > 0 {
			if err := s.store.ReorderQOTDQuestions(ctx, guildID, deck.ID, orderedIDs); err != nil {
				return nil, fmt.Errorf("Service.RestoreUsedQuestion: %w", err)
			}
		}
	}

	updated, err := s.store.GetQOTDQuestion(ctx, guildID, questionID)
	if err != nil {
		return nil, fmt.Errorf("Service.RestoreUsedQuestion: %w", err)
	}
	if updated == nil {
		return nil, ErrQuestionNotFound
	}
	return updated, nil
}

func (s *Service) MarkQuestionPublished(ctx context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.MarkQuestionPublished: %w", err)
	}

	if questionID <= 0 {
		return nil, fmt.Errorf("%w: question id must be greater than zero", files.ErrInvalidQOTDInput)
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return nil, fmt.Errorf("Service.MarkQuestionPublished: %w", err)
	}

	question, err := s.store.GetQOTDQuestion(ctx, guildID, questionID)
	if err != nil {
		return nil, fmt.Errorf("Service.MarkQuestionPublished: %w", err)
	}
	if question == nil || question.DeckID != deck.ID {
		return nil, ErrQuestionNotFound
	}

	status := QuestionStatus(strings.TrimSpace(question.Status))
	if status == QuestionStatusUsed {
		changed := false
		publishedAt := s.clock()
		if question.UsedAt == nil || question.UsedAt.IsZero() {
			question.UsedAt = &publishedAt
			changed = true
		}
		if question.PublishedOnceAt == nil || question.PublishedOnceAt.IsZero() {
			question.PublishedOnceAt = &publishedAt
			changed = true
		}
		if question.ScheduledForDateUTC != nil {
			question.ScheduledForDateUTC = nil
			changed = true
		}
		if !changed {
			return question, nil
		}
		return s.store.UpdateQOTDQuestion(ctx, *question)
	}

	if isImmutableQuestion(*question) {
		return nil, ErrImmutableQuestion
	}
	if status != QuestionStatusReady {
		return nil, ErrQuestionNotReady
	}

	publishedAt := s.clock()
	question.Status = string(QuestionStatusUsed)
	question.UsedAt = &publishedAt
	question.PublishedOnceAt = &publishedAt
	question.ScheduledForDateUTC = nil

	return s.store.UpdateQOTDQuestion(ctx, *question)
}

func (s *Service) ReorderQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) ([]storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, fmt.Errorf("Service.ReorderQuestions: %w", err)
	}
	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return nil, fmt.Errorf("Service.ReorderQuestions: %w", err)
	}

	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return nil, fmt.Errorf("Service.ReorderQuestions: %w", err)
	}
	if len(questions) == 0 {
		return nil, nil
	}

	fullOrder, err := normalizeReorderInput(questions, orderedIDs)
	if err != nil {
		return nil, fmt.Errorf("Service.ReorderQuestions: %w", err)
	}
	if err := s.store.ReorderQOTDQuestions(ctx, guildID, deck.ID, fullOrder); err != nil {
		return nil, fmt.Errorf("Service.ReorderQuestions: %w", err)
	}
	return s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
}

func (s *Service) GetSummary(ctx context.Context, guildID string) (Summary, error) {
	if err := s.validate(); err != nil {
		return Summary{}, fmt.Errorf("Service.GetSummary: %w", err)
	}

	now := s.clock()
	settings, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return Summary{}, fmt.Errorf("Service.GetSummary: %w", err)
	}
	displaySettings := files.DashboardQOTDConfig(settings)
	_, scheduleErr := resolvePublishSchedule(displaySettings)
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, "")
	if err != nil {
		return Summary{}, fmt.Errorf("Service.GetSummary: %w", err)
	}
	posts, err := s.store.GetCurrentAndPreviousQOTDPosts(ctx, guildID, now)
	if err != nil {
		return Summary{}, fmt.Errorf("Service.GetSummary: %w", err)
	}
	slotState, err := s.loadCurrentSlotState(ctx, guildID, displaySettings, now)
	if err != nil {
		return Summary{}, fmt.Errorf("Service.GetSummary: %w", err)
	}

	summary := Summary{
		Settings:                displaySettings,
		Counts:                  summarizeActiveDeckQuestions(displaySettings, questions),
		Decks:                   buildDeckSummaries(displaySettings, questions),
		CurrentPublishDateUTC:   time.Time{},
		PublishedForCurrentSlot: false,
	}
	if scheduleErr == nil {
		summary.CurrentPublishDateUTC = slotState.PublishDateUTC
		summary.PublishedForCurrentSlot = slotState.HasPublishedOfficialPost()
	}

	for idx := range posts {
		post := posts[idx]
		switch StateWithinWindow(post.GraceUntil, post.ArchiveAt, now) {
		case OfficialPostStateCurrent:
			summary.CurrentPost = &post
		case OfficialPostStatePrevious:
			summary.PreviousPost = &post
		}
	}

	return summary, nil
}

func countQuestions(questions []storage.QOTDQuestionRecord) QuestionCounts {
	counts := QuestionCounts{Total: len(questions)}
	for _, question := range questions {
		switch QuestionStatus(strings.TrimSpace(question.Status)) {
		case QuestionStatusDraft:
			counts.Draft++
		case QuestionStatusReady:
			counts.Ready++
		case QuestionStatusReserved:
			counts.Reserved++
		case QuestionStatusUsed:
			counts.Used++
		case QuestionStatusDisabled:
			counts.Disabled++
		}
	}
	return counts
}

func normalizeReorderInput(questions []storage.QOTDQuestionRecord, orderedIDs []int64) ([]int64, error) {
	if len(questions) == 0 {
		return nil, nil
	}

	currentOrder := make([]int64, 0, len(questions))
	known := make(map[int64]storage.QOTDQuestionRecord, len(questions))
	for _, question := range questions {
		currentOrder = append(currentOrder, question.ID)
		known[question.ID] = question
	}
	if len(orderedIDs) == 0 {
		return nil, fmt.Errorf("%w: ordered_ids is required", files.ErrInvalidQOTDInput)
	}

	seen := make(map[int64]struct{}, len(orderedIDs))
	normalized := make([]int64, 0, len(questions))
	for _, id := range orderedIDs {
		if _, ok := known[id]; !ok {
			return nil, fmt.Errorf("%w: ordered_ids contains unknown question id %d", files.ErrInvalidQOTDInput, id)
		}
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("%w: ordered_ids must be unique", files.ErrInvalidQOTDInput)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	for _, id := range currentOrder {
		if _, ok := seen[id]; ok {
			continue
		}
		normalized = append(normalized, id)
	}

	if len(normalized) != len(questions) {
		return nil, fmt.Errorf("%w: ordered_ids did not resolve to the full question set", files.ErrInvalidQOTDInput)
	}

	return normalized, nil
}

func ReorderQuestionIDs(current []storage.QOTDQuestionRecord, movedID int64, direction int) []int64 {
	if len(current) == 0 || movedID <= 0 || direction == 0 {
		return nil
	}

	ordered := make([]storage.QOTDQuestionRecord, 0, len(current))
	ordered = append(ordered, current...)
	slices.SortFunc(ordered, func(left, right storage.QOTDQuestionRecord) int {
		if left.QueuePosition != right.QueuePosition {
			if left.QueuePosition < right.QueuePosition {
				return -1
			}
			return 1
		}
		if left.ID < right.ID {
			return -1
		}
		if left.ID > right.ID {
			return 1
		}
		return 0
	})

	index := -1
	for idx, question := range ordered {
		if question.ID == movedID {
			index = idx
			break
		}
	}
	if index < 0 {
		return nil
	}

	target := index + direction
	if target < 0 || target >= len(ordered) {
		return idsFromQuestions(ordered)
	}
	ordered[index], ordered[target] = ordered[target], ordered[index]
	return idsFromQuestions(ordered)
}

func idsFromQuestions(questions []storage.QOTDQuestionRecord) []int64 {
	ids := make([]int64, 0, len(questions))
	for _, question := range questions {
		ids = append(ids, question.ID)
	}
	return ids
}

func reorderQuestionIDsToIndex(current []storage.QOTDQuestionRecord, movedIndex, targetIndex int) []int64 {
	if len(current) == 0 || movedIndex < 0 || movedIndex >= len(current) || targetIndex < 0 || targetIndex >= len(current) {
		return nil
	}
	if movedIndex == targetIndex {
		return idsFromQuestions(current)
	}

	ordered := append([]storage.QOTDQuestionRecord(nil), current...)
	moved := ordered[movedIndex]
	if targetIndex < movedIndex {
		copy(ordered[targetIndex+1:movedIndex+1], ordered[targetIndex:movedIndex])
	} else {
		copy(ordered[movedIndex:targetIndex], ordered[movedIndex+1:targetIndex+1])
	}
	ordered[targetIndex] = moved
	return idsFromQuestions(ordered)
}

func (s *Service) availableQuestionCount(ctx context.Context, guildID, deckID string) (int, error) {
	counts, err := s.deckQuestionCounts(ctx, guildID, deckID)
	if err != nil {
		return 0, fmt.Errorf("Service.availableQuestionCount: %w", err)
	}
	return counts.Ready + counts.Draft, nil
}

// deckQuestionCounts returns the per-status count breakdown for a deck. Used
// by the publish path to derive both the available-question count surfaced
// to the publisher and the visible thread display number (count of used
// questions, including legacy/imported ones marked Used outside the
// official_posts pipeline).
func (s *Service) deckQuestionCounts(ctx context.Context, guildID, deckID string) (QuestionCounts, error) {
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deckID)
	if err != nil {
		return QuestionCounts{}, fmt.Errorf("Service.deckQuestionCounts: %w", err)
	}
	return countQuestions(questions), nil
}

func summarizeActiveDeckQuestions(settings files.QOTDConfig, questions []storage.QOTDQuestionRecord) QuestionCounts {
	deck, ok := settings.ActiveDeck()
	if !ok {
		return QuestionCounts{}
	}
	return countQuestions(filterQuestionsByDeck(questions, deck.ID))
}

func buildDeckSummaries(settings files.QOTDConfig, questions []storage.QOTDQuestionRecord) []DeckSummary {
	decks := settings.Decks
	if len(decks) == 0 {
		return nil
	}
	activeDeck, hasActiveDeck := settings.ActiveDeck()
	summaries := make([]DeckSummary, 0, len(decks))
	for _, deck := range decks {
		counts := countQuestions(filterQuestionsByDeck(questions, deck.ID))
		summaries = append(summaries, DeckSummary{
			Deck:           deck,
			Counts:         counts,
			CardsRemaining: counts.Ready + counts.Draft,
			IsActive:       hasActiveDeck && deck.ID == activeDeck.ID,
			CanPublish:     deck.Enabled && canPublishQOTD(deck),
		})
	}
	return summaries
}

func filterQuestionsByDeck(questions []storage.QOTDQuestionRecord, deckID string) []storage.QOTDQuestionRecord {
	deckID = strings.TrimSpace(deckID)
	if deckID == "" || len(questions) == 0 {
		return nil
	}
	filtered := make([]storage.QOTDQuestionRecord, 0, len(questions))
	for _, question := range questions {
		if strings.TrimSpace(question.DeckID) == deckID {
			filtered = append(filtered, question)
		}
	}
	return filtered
}

func questionByID(questions []storage.QOTDQuestionRecord, questionID int64) *storage.QOTDQuestionRecord {
	if questionID <= 0 {
		return nil
	}
	for idx := range questions {
		if questions[idx].ID != questionID {
			continue
		}
		question := questions[idx]
		return &question
	}
	return nil
}

func firstReadyUnscheduledQuestion(questions []storage.QOTDQuestionRecord) *storage.QOTDQuestionRecord {
	for idx := range questions {
		question := questions[idx]
		if QuestionStatus(strings.TrimSpace(question.Status)) != QuestionStatusReady {
			continue
		}
		if question.PublishedOnceAt != nil && !question.PublishedOnceAt.IsZero() {
			continue
		}
		if question.ScheduledForDateUTC != nil && !question.ScheduledForDateUTC.IsZero() {
			continue
		}
		questionCopy := question
		return &questionCopy
	}
	return nil
}

func reservedQuestionForDate(questions []storage.QOTDQuestionRecord, publishDateUTC time.Time) *storage.QOTDQuestionRecord {
	publishDateUTC = NormalizePublishDateUTC(publishDateUTC)
	if publishDateUTC.IsZero() {
		return nil
	}
	for idx := range questions {
		question := questions[idx]
		if QuestionStatus(strings.TrimSpace(question.Status)) != QuestionStatusReserved {
			continue
		}
		if question.ScheduledForDateUTC == nil || question.ScheduledForDateUTC.IsZero() {
			continue
		}
		if !NormalizePublishDateUTC(*question.ScheduledForDateUTC).Equal(publishDateUTC) {
			continue
		}
		questionCopy := question
		return &questionCopy
	}
	return nil
}
