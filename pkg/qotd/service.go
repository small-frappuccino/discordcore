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
	ErrNoQuestionsAvailable     = errors.New("no qotd questions available")
	ErrImmutableQuestion        = errors.New("qotd question is already scheduled or used")
	ErrQuestionNotFound         = errors.New("qotd question not found")
	ErrDeckNotFound             = errors.New("qotd deck not found")
	ErrDiscordUnavailable       = errors.New("discord session unavailable")
	ErrOfficialPostNotFound     = discordqotd.ErrOfficialPostNotFound
	ErrAnswerWindowClosed       = discordqotd.ErrAnswerWindowClosed
	ErrAnswerChannelUnavailable = discordqotd.ErrAnswerChannelUnavailable
)

type Publisher interface {
	PublishOfficialPost(ctx context.Context, session *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error)
	SetupForum(ctx context.Context, session *discordgo.Session, params discordqotd.SetupForumParams) (*discordqotd.SetupForumResult, error)
	UpsertAnswerMessage(ctx context.Context, session *discordgo.Session, params discordqotd.UpsertAnswerMessageParams) (*discordqotd.UpsertedAnswerMessage, error)
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

type Service struct {
	configManager       *files.ConfigManager
	store               *storage.Store
	publisher           Publisher
	now                 func() time.Time
	answerRecordLocks   sync.Map
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

func (s *Service) GetSettings(guildID string) (files.QOTDConfig, error) {
	if err := s.validate(); err != nil {
		return files.QOTDConfig{}, err
	}
	settings, err := s.configManager.GetQOTDConfig(guildID)
	if err != nil {
		return files.QOTDConfig{}, err
	}
	return files.DashboardQOTDConfig(settings), nil
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

func (s *Service) ReorderQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) ([]storage.QOTDQuestionRecord, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
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
	settings, err := s.configManager.GetQOTDConfig(guildID)
	if err != nil {
		return Summary{}, err
	}
	displaySettings := files.DashboardQOTDConfig(settings)
	questions, err := s.store.ListQOTDQuestions(ctx, guildID, "")
	if err != nil {
		return Summary{}, err
	}
	posts, err := s.store.GetCurrentAndPreviousQOTDPosts(ctx, guildID, now)
	if err != nil {
		return Summary{}, err
	}
	currentPublishDate := CurrentPublishDateUTC(now)
	currentSlotPost, err := s.store.GetQOTDOfficialPostByDate(ctx, guildID, currentPublishDate)
	if err != nil {
		return Summary{}, err
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
	publishDate := NormalizePublishDateUTC(now)
	cfg, err := s.configManager.GetQOTDConfig(guildID)
	if err != nil {
		return nil, err
	}
	deck, ok := cfg.ActiveDeck()
	if !ok || !deck.Enabled || !canPublishQOTD(deck) {
		return nil, ErrQOTDDisabled
	}
	if recovered, err := s.resumeOldestPendingOfficialPost(ctx, guildID, session, now); err != nil {
		return nil, err
	} else if recovered != nil {
		if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, recovered.OfficialPost.ID); err != nil {
			return nil, err
		}
		return recovered, nil
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
		ForumChannelID:       strings.TrimSpace(deck.ForumChannelID),
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
		buildOfficialThreadName(question.Body, question.QueuePosition),
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

func (s *Service) SubmitAnswer(ctx context.Context, session *discordgo.Session, params discordqotd.SubmitAnswerParams) (*discordqotd.SubmitAnswerResult, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrDiscordUnavailable
	}

	normalized, err := normalizeSubmitAnswerParams(params)
	if err != nil {
		return nil, err
	}

	officialPost, err := s.store.GetQOTDOfficialPostByID(ctx, normalized.OfficialPostID)
	if err != nil {
		return nil, err
	}
	if officialPost == nil || officialPost.GuildID != normalized.GuildID {
		return nil, ErrOfficialPostNotFound
	}
	if !isOfficialPostPublished(*officialPost) {
		return nil, ErrAnswerChannelUnavailable
	}

	lifecycle := EvaluateOfficialPostWindow(
		officialPost.PublishDateUTC,
		derefTime(officialPost.PublishedAt),
		officialPost.GraceUntil,
		officialPost.ArchiveAt,
		s.clock(),
	)
	if !lifecycle.AnswerWindow.IsOpen {
		return nil, ErrAnswerWindowClosed
	}

	defaultAnswerChannelID := strings.TrimSpace(officialPost.AnswerChannelID)
	if defaultAnswerChannelID == "" {
		defaultAnswerChannelID = strings.TrimSpace(officialPost.DiscordThreadID)
	}
	if defaultAnswerChannelID == "" {
		return nil, ErrAnswerChannelUnavailable
	}

	lock := s.answerRecordLock(officialPost.ID, normalized.UserID)
	lock.Lock()
	defer lock.Unlock()

	record, err := s.store.GetQOTDAnswerMessageByOfficialPostAndUser(ctx, officialPost.ID, normalized.UserID)
	if err != nil {
		return nil, err
	}
	if record == nil {
		record, err = s.store.CreateQOTDAnswerMessage(ctx, storage.QOTDAnswerMessageRecord{
			GuildID:                 officialPost.GuildID,
			OfficialPostID:          officialPost.ID,
			UserID:                  normalized.UserID,
			State:                   string(AnswerRecordStateActive),
			AnswerChannelID:         defaultAnswerChannelID,
			CreatedViaInteractionID: normalized.InteractionID,
		})
		if err != nil {
			if !isQOTDUniqueConstraintError(err) {
				return nil, err
			}
			record, err = s.store.GetQOTDAnswerMessageByOfficialPostAndUser(ctx, officialPost.ID, normalized.UserID)
			if err != nil {
				return nil, err
			}
		}
	}
	if record == nil {
		return nil, ErrAnswerChannelUnavailable
	}

	targetChannelID := strings.TrimSpace(record.AnswerChannelID)
	if targetChannelID == "" {
		targetChannelID = defaultAnswerChannelID
	}

	upserted, err := s.publisher.UpsertAnswerMessage(ctx, session, discordqotd.UpsertAnswerMessageParams{
		GuildID:           officialPost.GuildID,
		OfficialPostID:    officialPost.ID,
		DeckName:          officialPost.DeckNameSnapshot,
		PublishDateUTC:    officialPost.PublishDateUTC,
		AnswerChannelID:   targetChannelID,
		QuestionText:      officialPost.QuestionTextSnapshot,
		QuestionURL:       officialPostJumpURL(*officialPost),
		AnswerText:        normalized.AnswerText,
		UserID:            normalized.UserID,
		UserDisplayName:   normalized.UserDisplayName,
		UserAvatarURL:     normalized.UserAvatarURL,
		ExistingMessageID: strings.TrimSpace(record.DiscordMessageID),
	})
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(record.DiscordMessageID) != upserted.MessageID {
		record, err = s.store.FinalizeQOTDAnswerMessage(ctx, record.ID, upserted.MessageID)
		if err != nil {
			return nil, err
		}
	}
	record, err = s.store.UpdateQOTDAnswerMessageState(ctx, record.ID, string(AnswerRecordStateActive), nil, nil)
	if err != nil {
		return nil, err
	}

	return &discordqotd.SubmitAnswerResult{
		MessageID:  upserted.MessageID,
		ChannelID:  upserted.ChannelID,
		MessageURL: upserted.MessageURL,
		Updated:    upserted.Updated,
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

func buildOfficialThreadName(questionText string, queuePosition int64) string {
	if queuePosition > 0 {
		return fmt.Sprintf("question of the day #%d", queuePosition)
	}
	return "question of the day"
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

func normalizeSubmitAnswerParams(params discordqotd.SubmitAnswerParams) (discordqotd.SubmitAnswerParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.UserID = strings.TrimSpace(params.UserID)
	params.UserDisplayName = strings.TrimSpace(params.UserDisplayName)
	params.UserAvatarURL = strings.TrimSpace(params.UserAvatarURL)
	params.InteractionID = strings.TrimSpace(params.InteractionID)
	params.AnswerText = strings.TrimSpace(params.AnswerText)

	switch {
	case params.GuildID == "":
		return discordqotd.SubmitAnswerParams{}, fmt.Errorf("%w: guild id is required", files.ErrInvalidQOTDInput)
	case params.OfficialPostID <= 0:
		return discordqotd.SubmitAnswerParams{}, fmt.Errorf("%w: official post id is required", files.ErrInvalidQOTDInput)
	case params.UserID == "":
		return discordqotd.SubmitAnswerParams{}, fmt.Errorf("%w: user id is required", files.ErrInvalidQOTDInput)
	case params.AnswerText == "":
		return discordqotd.SubmitAnswerParams{}, fmt.Errorf("%w: answer text is required", files.ErrInvalidQOTDInput)
	default:
		return params, nil
	}
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

func (s *Service) resolveDashboardDeck(guildID, deckID string) (files.QOTDDeckConfig, error) {
	settings, err := s.GetSettings(guildID)
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

func (s *Service) answerRecordLock(officialPostID int64, userID string) *sync.Mutex {
	key := fmt.Sprintf("%d:%s", officialPostID, strings.TrimSpace(userID))
	lock, _ := s.answerRecordLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func (s *Service) guildLifecycleLock(guildID string) *sync.Mutex {
	key := strings.TrimSpace(guildID)
	lock, _ := s.guildLifecycleLocks.LoadOrStore(key, &sync.Mutex{})
	return lock.(*sync.Mutex)
}

func isQOTDUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}

func canPublishQOTD(deck files.QOTDDeckConfig) bool {
	return strings.TrimSpace(deck.ForumChannelID) != ""
}

func hasPublishedOfficialPostTarget(post *storage.QOTDOfficialPostRecord) bool {
	if post == nil {
		return false
	}
	return isOfficialPostPublished(*post)
}

func officialPostJumpURL(post storage.QOTDOfficialPostRecord) string {
	if threadID := strings.TrimSpace(post.DiscordThreadID); threadID != "" {
		return discordqotd.BuildThreadJumpURL(post.GuildID, threadID)
	}
	channelID := strings.TrimSpace(post.QuestionListThreadID)
	messageID := strings.TrimSpace(post.QuestionListEntryMessageID)
	if channelID == "" || messageID == "" {
		return ""
	}
	return discordqotd.BuildMessageJumpURL(post.GuildID, channelID, messageID)
}

func qotdForumSurfaceQuestionListThreadID(surface *storage.QOTDForumSurfaceRecord) string {
	if surface == nil {
		return ""
	}
	return strings.TrimSpace(surface.QuestionListThreadID)
}
