package qotd

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

var (
	ErrServiceUnavailable       = errors.New("qotd service unavailable")
	ErrQOTDDisabled             = errors.New("qotd is disabled")
	ErrAlreadyPublished         = errors.New("qotd already published for the current slot")
	ErrPublishInProgress        = errors.New("qotd publish already in progress for the current slot")
	ErrNoQuestionsAvailable     = errors.New("no qotd questions available")
	ErrImmutableQuestion        = errors.New("qotd question is already scheduled or used")
	ErrQuestionNotFound         = errors.New("qotd question not found")
	ErrQuestionNotReady         = errors.New("qotd question is not ready")
	ErrDeckNotFound             = errors.New("qotd deck not found")
	ErrDiscordUnavailable       = errors.New("discord session unavailable")
)

type Publisher interface {
	PublishOfficialPost(ctx context.Context, session *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error)
	SetThreadState(ctx context.Context, session *discordgo.Session, threadID string, state discordqotd.ThreadState) error
	FetchThreadMessages(ctx context.Context, session *discordgo.Session, threadID string) ([]discordqotd.ArchivedMessage, error)
	FetchChannelMessages(ctx context.Context, session *discordgo.Session, channelID, beforeMessageID string, limit int) ([]discordqotd.ArchivedMessage, error)
}

type QuestionMutation struct {
	DeckID string
	Body   string
	Status QuestionStatus
}

type QuestionCounts struct {
	Total    int `json:"total"`
	Draft    int `json:"draft"`
	Ready    int `json:"ready"`
	Reserved int `json:"reserved"`
	Used     int `json:"used"`
	Disabled int `json:"disabled"`
}

type Summary struct {
	Settings                files.QOTDConfig
	Counts                  QuestionCounts
	Decks                   []DeckSummary
	CurrentPublishDateUTC   time.Time
	PublishedForCurrentSlot bool
	CurrentPost             *storage.QOTDOfficialPostRecord
	PreviousPost            *storage.QOTDOfficialPostRecord
}

type PublishResult struct {
	Question     storage.QOTDQuestionRecord
	OfficialPost storage.QOTDOfficialPostRecord
	PostURL      string
}

type ResetDeckResult struct {
	QuestionsReset      int
	OfficialPostsCleared int
}

type AutomaticQueueSlotStatus string

const (
	AutomaticQueueSlotStatusDisabled   AutomaticQueueSlotStatus = "disabled"
	AutomaticQueueSlotStatusWaiting    AutomaticQueueSlotStatus = "waiting"
	AutomaticQueueSlotStatusDue        AutomaticQueueSlotStatus = "due"
	AutomaticQueueSlotStatusReserved   AutomaticQueueSlotStatus = "reserved"
	AutomaticQueueSlotStatusRecovering AutomaticQueueSlotStatus = "recovering"
	AutomaticQueueSlotStatusPublished  AutomaticQueueSlotStatus = "published"
)

type AutomaticQueueState struct {
	Deck               files.QOTDDeckConfig
	Schedule           PublishSchedule
	ScheduleConfigured bool
	CanPublish         bool
	SlotDateUTC        time.Time
	SlotPublishAtUTC   time.Time
	SlotStatus         AutomaticQueueSlotStatus
	SlotOfficialPost   *storage.QOTDOfficialPostRecord
	SlotQuestion       *storage.QOTDQuestionRecord
	NextReadyQuestion  *storage.QOTDQuestionRecord
}

type Service struct {
	configManager       *files.ConfigManager
	store               *storage.Store
	publisher           Publisher
	now                 func() time.Time
	guildLifecycleLocks sync.Map
}

func NewService(configManager *files.ConfigManager, store *storage.Store, publisher Publisher) *Service {
	if publisher == nil {
		publisher = discordqotd.NewPublisher()
	}
	return &Service{
		configManager: configManager,
		store:         store,
		publisher:     publisher,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) Settings(guildID string) (files.QOTDConfig, error) {
	if err := s.validate(); err != nil {
		return files.QOTDConfig{}, err
	}
	settings, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	return files.DashboardQOTDConfig(settings), nil
}

func (s *Service) GetSettings(guildID string) (files.QOTDConfig, error) {
	return s.Settings(guildID)
}

func (s *Service) UpdateSettings(guildID string, cfg files.QOTDConfig) (files.QOTDConfig, error) {
	if err := s.validate(); err != nil {
		return files.QOTDConfig{}, err
	}
	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()
	return s.updateSettingsLocked(guildID, cfg)
}

func (s *Service) ListQuestions(ctx context.Context, guildID, deckID string) ([]storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return nil, err
	}
	return s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
}

func (s *Service) CreateQuestion(ctx context.Context, guildID, actorID string, mutation QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	deck, err := s.resolveDashboardDeck(guildID, mutation.DeckID)
	if err != nil {
		return nil, err
	}
	body, status, err := normalizeQuestionMutation(mutation)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	guildID = strings.TrimSpace(guildID)
	var created []storage.QOTDQuestionRecord

	for _, mutation := range mutations {
		deck, err := s.resolveDashboardDeck(guildID, mutation.DeckID)
		if err != nil {
			return created, err
		}
		body, status, err := normalizeQuestionMutation(mutation)
		if err != nil {
			return created, err
		}

		record, err := s.store.CreateQOTDQuestion(ctx, storage.QOTDQuestionRecord{
			GuildID:   guildID,
			DeckID:    deck.ID,
			Body:      body,
			Status:    string(status),
			CreatedBy: normalizeActorID(actorID),
		})
		if err != nil {
			return created, err
		}
		created = append(created, *record)
	}

	return created, nil
}

func (s *Service) UpdateQuestion(ctx context.Context, guildID string, questionID int64, mutation QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	current, err := s.store.GetQOTDQuestion(ctx, guildID, questionID)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	current.DeckID = deckID
	current.Body = body
	current.Status = string(status)
	return s.store.UpdateQOTDQuestion(ctx, *current)
}

func (s *Service) DeleteQuestion(ctx context.Context, guildID string, questionID int64) error {
	if err := s.validate(); err != nil {
		return err
	}
	current, err := s.store.GetQOTDQuestion(ctx, guildID, questionID)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrQuestionNotFound
	}
	if isImmutableQuestion(*current) {
		return ErrImmutableQuestion
	}
	return s.store.DeleteQOTDQuestion(ctx, guildID, questionID)
}

func (s *Service) ResetDeckQuestionStates(ctx context.Context, guildID, deckID string) (int, error) {
	result, err := s.ResetDeckState(ctx, guildID, deckID)
	if err != nil {
		return 0, err
	}
	return result.QuestionsReset, nil
}

func (s *Service) ResetDeckState(ctx context.Context, guildID, deckID string) (ResetDeckResult, error) {
	if err := s.validate(); err != nil {
		return ResetDeckResult{}, err
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return ResetDeckResult{}, err
	}
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return ResetDeckResult{}, err
	}

	result := ResetDeckResult{}
	for idx := range questions {
		question := questions[idx]
		if question.PublishedOnceAt != nil && !question.PublishedOnceAt.IsZero() {
			question.PublishedOnceAt = nil
			question.Status = string(QuestionStatusReady)
			question.ScheduledForDateUTC = nil
			question.UsedAt = nil
			if _, err := s.store.UpdateQOTDQuestion(ctx, question); err != nil {
				return result, err
			}
			result.QuestionsReset++
			continue
		}
		switch QuestionStatus(strings.TrimSpace(question.Status)) {
		case QuestionStatusReserved, QuestionStatusUsed:
			question.Status = string(QuestionStatusReady)
			question.ScheduledForDateUTC = nil
			question.UsedAt = nil
			if _, err := s.store.UpdateQOTDQuestion(ctx, question); err != nil {
				return result, err
			}
			result.QuestionsReset++
		}
	}

	result.OfficialPostsCleared, err = s.store.DeleteQOTDUnpublishedOfficialPostsByDeck(ctx, guildID, deck.ID)
	if err != nil {
		return result, err
	}
	if err := s.store.DeleteQOTDSurfaceByDeck(ctx, guildID, deck.ID); err != nil {
		return result, err
	}

	return result, nil
}

func (s *Service) GetAutomaticQueueState(ctx context.Context, guildID, deckID string) (AutomaticQueueState, error) {
	if err := s.validate(); err != nil {
		return AutomaticQueueState{}, err
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return AutomaticQueueState{}, err
	}

	state := AutomaticQueueState{Deck: deck}
	now := s.clock()
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return AutomaticQueueState{}, err
	}
	state.NextReadyQuestion = firstReadyUnscheduledQuestion(questions)

	settings, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return AutomaticQueueState{}, err
	}
	schedule, scheduleErr := resolvePublishSchedule(settings)
	if scheduleErr == nil {
		state.ScheduleConfigured = true
		state.Schedule = schedule
		state.SlotDateUTC = CurrentPublishDateUTC(schedule, now)
		state.SlotPublishAtUTC = PublishTimeUTC(schedule, state.SlotDateUTC)
		state.CanPublish = deck.Enabled && canPublishQOTD(deck)

		officialPost, err := s.store.GetQOTDOfficialPostByDate(ctx, guildID, state.SlotDateUTC)
		if err != nil {
			return AutomaticQueueState{}, err
		}
		state.SlotOfficialPost = officialPost
		if officialPost != nil {
			state.SlotQuestion = questionByID(questions, officialPost.QuestionID)
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

func (s *Service) SetNextQuestion(ctx context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, err
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
		return nil, err
	}

	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return nil, err
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

	moved := questions[movedIndex]
	if moved.DeckID != deck.ID {
		return nil, ErrQuestionNotFound
	}
	if isImmutableQuestion(moved) {
		return nil, ErrImmutableQuestion
	}
	if QuestionStatus(strings.TrimSpace(moved.Status)) != QuestionStatusReady {
		return nil, ErrQuestionNotReady
	}
	if firstMutableIndex < 0 {
		return nil, ErrNoQuestionsAvailable
	}
	if movedIndex == firstMutableIndex {
		return &moved, nil
	}

	orderedIDs := reorderQuestionIDsToIndex(questions, movedIndex, firstMutableIndex)
	if len(orderedIDs) == 0 {
		return &moved, nil
	}
	if err := s.store.ReorderQOTDQuestions(ctx, guildID, deck.ID, orderedIDs); err != nil {
		return nil, err
	}

	updated, err := s.store.GetQOTDQuestion(ctx, guildID, questionID)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, ErrQuestionNotFound
	}
	return updated, nil
}

func (s *Service) ReorderQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) ([]storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	deck, err := s.resolveDashboardDeck(guildID, deckID)
	if err != nil {
		return nil, err
	}

	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
	if err != nil {
		return nil, err
	}
	if len(questions) == 0 {
		return nil, nil
	}

	fullOrder, err := normalizeReorderInput(questions, orderedIDs)
	if err != nil {
		return nil, err
	}
	if err := s.store.ReorderQOTDQuestions(ctx, guildID, deck.ID, fullOrder); err != nil {
		return nil, err
	}
	return s.store.ListQOTDQuestions(ctx, guildID, deck.ID)
}

func (s *Service) GetSummary(ctx context.Context, guildID string) (Summary, error) {
	if err := s.validate(); err != nil {
		return Summary{}, err
	}

	now := s.clock()
	settings, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return Summary{}, err
	}
	displaySettings := files.DashboardQOTDConfig(settings)
	schedule, scheduleErr := resolvePublishSchedule(displaySettings)
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, "")
	if err != nil {
		return Summary{}, err
	}
	posts, err := s.store.GetCurrentAndPreviousQOTDPosts(ctx, guildID, now)
	if err != nil {
		return Summary{}, err
	}
	currentPublishDate := time.Time{}
	var currentSlotPost *storage.QOTDOfficialPostRecord
	if scheduleErr == nil {
		currentPublishDate = CurrentPublishDateUTC(schedule, now)
		currentSlotPost, err = s.store.GetQOTDOfficialPostByDate(ctx, guildID, currentPublishDate)
		if err != nil {
			return Summary{}, err
		}
	}

	summary := Summary{
		Settings:                displaySettings,
		Counts:                  summarizeActiveDeckQuestions(displaySettings, questions),
		Decks:                   buildDeckSummaries(displaySettings, questions),
		CurrentPublishDateUTC:   currentPublishDate,
		PublishedForCurrentSlot: hasPublishedOfficialPostTarget(currentSlotPost),
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

func (s *Service) PublishNow(ctx context.Context, guildID string, session *discordgo.Session) (*PublishResult, error) {
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
	publishDate := NormalizePublishDateUTC(now)
	if schedule, scheduleErr := resolvePublishSchedule(cfg); scheduleErr == nil {
		publishDate = CurrentPublishDateUTC(schedule, now)
	}
	deck, ok := cfg.ActiveDeck()
	if !ok || !deck.Enabled || !canPublishQOTD(deck) {
		return nil, ErrQOTDDisabled
	}
	existing, err := s.store.GetQOTDOfficialPostByDate(ctx, guildID, publishDate)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if isOfficialPostPublished(*existing) {
			return nil, ErrAlreadyPublished
		}
		return nil, ErrPublishInProgress
	}

	question, err := s.store.ReserveNextReadyQOTDQuestion(ctx, guildID, deck.ID)
	if err != nil {
		return nil, err
	}
	if question == nil {
		return nil, ErrNoQuestionsAvailable
	}
	availableQuestions, err := s.availableQuestionCount(ctx, guildID, deck.ID)
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD question reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return nil, err
	}

	lifecycle := EvaluateManualOfficialPost(now, now)
	provisioned, err := s.store.CreateQOTDOfficialPostProvisioning(ctx, storage.QOTDOfficialPostRecord{
		GuildID:              guildID,
		DeckID:               deck.ID,
		DeckNameSnapshot:     deck.Name,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeManual),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateProvisioning),
		ChannelID:            strings.TrimSpace(deck.ChannelID),
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD question reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return nil, err
	}

	finalized, updatedQuestion, postURL, err := s.completeOfficialPostProvisioning(
		ctx,
		session,
		*provisioned,
		question,
		availableQuestions,
		buildOfficialThreadName(question.DisplayID),
		now,
	)
	if err != nil {
		return nil, err
	}
	if updatedQuestion != nil {
		question = updatedQuestion
	}

	if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, finalized.ID); err != nil {
		return nil, err
	}

	return &PublishResult{
		Question:     *question,
		OfficialPost: *finalized,
		PostURL:      postURL,
	}, nil
}

func (s *Service) reconcileOfficialPostWindow(ctx context.Context, guildID string, session *discordgo.Session, now time.Time, currentOfficialPostID int64) error {
	posts, err := s.store.GetCurrentAndPreviousQOTDPosts(ctx, guildID, now)
	if err != nil {
		return err
	}

	for _, post := range posts {
		lifecycle := EvaluateOfficialPostWindow(post.PublishDateUTC, derefTime(post.PublishedAt), post.GraceUntil, post.ArchiveAt, now)
		if err := s.syncLiveOfficialPost(ctx, session, post, lifecycle); err != nil {
			return err
		}
	}

	candidates, err := s.store.ListQOTDOfficialPostsNeedingArchive(ctx, now)
	if err != nil {
		return err
	}
	for _, post := range candidates {
		if post.GuildID != guildID || post.ID == currentOfficialPostID {
			continue
		}
		if err := s.archiveOfficialPost(ctx, session, post, now.UTC()); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) releaseReservedQuestion(ctx context.Context, question storage.QOTDQuestionRecord) error {
	question.Status = string(QuestionStatusReady)
	question.ScheduledForDateUTC = nil
	question.UsedAt = nil
	_, err := s.store.UpdateQOTDQuestion(ctx, question)
	return err
}

func (s *Service) validate() error {
	if s == nil {
		return ErrServiceUnavailable
	}
	if s.configManager == nil || s.store == nil || s.publisher == nil {
		return ErrServiceUnavailable
	}
	return nil
}

func (s *Service) clock() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
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

func normalizeActorID(actorID string) string {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return "control_api"
	}
	return actorID
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}

func buildOfficialThreadName(displayID int64) string {
	return "Question of the Day"
}

func normalizeQuestionMutation(mutation QuestionMutation) (string, QuestionStatus, error) {
	body := strings.TrimSpace(mutation.Body)
	if body == "" {
		return "", "", fmt.Errorf("%w: question body is required", files.ErrInvalidQOTDInput)
	}

	status := mutation.Status
	if status == "" {
		status = QuestionStatusReady
	}
	switch status {
	case QuestionStatusDraft, QuestionStatusReady, QuestionStatusDisabled:
		return body, status, nil
	default:
		return "", "", fmt.Errorf("%w: question status must be draft, ready, or disabled", files.ErrInvalidQOTDInput)
	}
}

func isImmutableQuestion(question storage.QOTDQuestionRecord) bool {
	if question.PublishedOnceAt != nil && !question.PublishedOnceAt.IsZero() {
		return true
	}
	if question.ScheduledForDateUTC != nil && !question.ScheduledForDateUTC.IsZero() {
		return true
	}
	switch QuestionStatus(strings.TrimSpace(question.Status)) {
	case QuestionStatusReserved, QuestionStatusUsed:
		return true
	default:
		return false
	}
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
	copy(ordered[targetIndex+1:movedIndex+1], ordered[targetIndex:movedIndex])
	ordered[targetIndex] = moved
	return idsFromQuestions(ordered)
}

func (s *Service) availableQuestionCount(ctx context.Context, guildID, deckID string) (int, error) {
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, deckID)
	if err != nil {
		return 0, err
	}
	counts := countQuestions(questions)
	return counts.Ready + counts.Draft, nil
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

func (s *Service) resolveDashboardDeck(guildID, deckID string) (files.QOTDDeckConfig, error) {
	settings, err := s.Settings(guildID)
	if err != nil {
		return files.QOTDDeckConfig{}, err
	}
	deckID = strings.TrimSpace(deckID)
	if deckID != "" {
		deck, ok := settings.DeckByID(deckID)
		if !ok {
			return files.QOTDDeckConfig{}, ErrDeckNotFound
		}
		return deck, nil
	}
	deck, ok := settings.ActiveDeck()
	if !ok {
		return files.QOTDDeckConfig{}, ErrDeckNotFound
	}
	return deck, nil
}

func (s *Service) deleteRemovedDeckQuestions(ctx context.Context, guildID string, current, next files.QOTDConfig) error {
	removedDeckIDs := missingDeckIDs(current.Decks, next.Decks)
	if len(removedDeckIDs) == 0 {
		return nil
	}
	if err := s.store.DeleteQOTDQuestionsByDecks(ctx, guildID, removedDeckIDs); err != nil {
		return fmt.Errorf("delete removed qotd deck questions: %w", err)
	}
	return nil
}

func missingDeckIDs(current, next []files.QOTDDeckConfig) []string {
	nextIDs := make(map[string]struct{}, len(next))
	for _, deck := range next {
		nextIDs[strings.TrimSpace(deck.ID)] = struct{}{}
	}
	removed := make([]string, 0)
	for _, deck := range current {
		deckID := strings.TrimSpace(deck.ID)
		if deckID == "" {
			continue
		}
		if _, ok := nextIDs[deckID]; ok {
			continue
		}
		removed = append(removed, deckID)
	}
	return removed
}

func (s *Service) guildLifecycleLock(guildID string) *sync.Mutex {
	key := strings.TrimSpace(guildID)
	lock, _ := s.guildLifecycleLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func canPublishQOTD(deck files.QOTDDeckConfig) bool {
	return strings.TrimSpace(deck.ChannelID) != ""
}

func hasPublishedOfficialPostTarget(post *storage.QOTDOfficialPostRecord) bool {
	if post == nil {
		return false
	}
	return isOfficialPostPublished(*post)
}
