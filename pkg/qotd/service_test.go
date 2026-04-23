package qotd

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

var errFakePublishFailed = errors.New("fake publish failed")

type fakePublisher struct {
	publishedParams  []discordqotd.PublishOfficialPostParams
	publishResponses []fakePublishResponse
	setupParams      []discordqotd.SetupForumParams
	setupResults     []fakeSetupForumResponse
	answerParams     []discordqotd.UpsertAnswerMessageParams
	threadStates     map[string]discordqotd.ThreadState
	fetchCalls       []string
	threadMessages   map[string][]discordqotd.ArchivedMessage
	channelMessages  map[string][]discordqotd.ArchivedMessage
	fetchErrs        map[string]error
}

type fakePublishResponse struct {
	result *discordqotd.PublishedOfficialPost
	err    error
}

type fakeSetupForumResponse struct {
	result *discordqotd.SetupForumResult
	err    error
}

func (p *fakePublisher) PublishOfficialPost(_ context.Context, _ *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	p.publishedParams = append(p.publishedParams, params)
	if len(p.publishResponses) > 0 {
		response := p.publishResponses[0]
		p.publishResponses = p.publishResponses[1:]
		if response.result == nil {
			return nil, response.err
		}
		out := *response.result
		return &out, response.err
	}
	return defaultFakePublishedOfficialPost(params), nil
}

func defaultFakePublishedOfficialPost(params discordqotd.PublishOfficialPostParams) *discordqotd.PublishedOfficialPost {
	listThreadID := strings.TrimSpace(params.QuestionListThreadID)
	if listThreadID == "" {
		listThreadID = "questions-list-thread"
	}
	listEntryID := strings.TrimSpace(params.QuestionListEntryMessageID)
	if listEntryID == "" {
		listEntryID = "list-entry-" + timePtrString(params.OfficialPostID)
	}
	threadID := strings.TrimSpace(params.OfficialThreadID)
	if threadID == "" {
		threadID = "thread-" + timePtrString(params.OfficialPostID)
	}
	messageID := strings.TrimSpace(params.OfficialStarterMessageID)
	if messageID == "" {
		messageID = "message-" + timePtrString(params.OfficialPostID)
	}
	answerChannelID := strings.TrimSpace(params.OfficialAnswerChannelID)
	if answerChannelID == "" {
		answerChannelID = threadID
	}
	publishedAt := params.ExistingPublishedAt
	if publishedAt.IsZero() {
		publishedAt = time.Date(params.PublishDateUTC.Year(), params.PublishDateUTC.Month(), params.PublishDateUTC.Day(), 12, 43, 0, 0, time.UTC)
	}
	return &discordqotd.PublishedOfficialPost{
		QuestionListThreadID:       listThreadID,
		QuestionListEntryMessageID: listEntryID,
		ThreadID:                   threadID,
		StarterMessageID:           messageID,
		AnswerChannelID:            answerChannelID,
		PublishedAt:                publishedAt,
		PostURL:                    discordqotd.BuildThreadJumpURL(params.GuildID, threadID),
	}
}

func (p *fakePublisher) SetupForum(_ context.Context, _ *discordgo.Session, params discordqotd.SetupForumParams) (*discordqotd.SetupForumResult, error) {
	p.setupParams = append(p.setupParams, params)
	if len(p.setupResults) > 0 {
		response := p.setupResults[0]
		p.setupResults = p.setupResults[1:]
		if response.result == nil {
			return nil, response.err
		}
		out := *response.result
		return &out, response.err
	}
	channelID := strings.TrimSpace(params.PreferredChannelID)
	if channelID == "" {
		channelID = "channel-setup-1"
	}
	questionListThreadID := strings.TrimSpace(params.PreferredQuestionListThreadID)
	if questionListThreadID == "" {
		questionListThreadID = "questions-list-thread"
	}
	return &discordqotd.SetupForumResult{
		ChannelID:            channelID,
		ChannelName:          "☆-qotd-☆",
		ChannelURL:           discordqotd.BuildChannelJumpURL(params.GuildID, channelID),
		QuestionListThreadID: questionListThreadID,
		QuestionListPostURL:  discordqotd.BuildChannelJumpURL(params.GuildID, questionListThreadID),
	}, nil
}

func (p *fakePublisher) SetThreadState(_ context.Context, _ *discordgo.Session, threadID string, state discordqotd.ThreadState) error {
	if p.threadStates == nil {
		p.threadStates = make(map[string]discordqotd.ThreadState)
	}
	p.threadStates[threadID] = state
	return nil
}

func (p *fakePublisher) UpsertAnswerMessage(_ context.Context, _ *discordgo.Session, params discordqotd.UpsertAnswerMessageParams) (*discordqotd.UpsertedAnswerMessage, error) {
	p.answerParams = append(p.answerParams, params)
	messageID := "answer-message-" + timePtrString(params.OfficialPostID) + "-" + params.UserID
	if strings.TrimSpace(params.ExistingMessageID) != "" {
		messageID = strings.TrimSpace(params.ExistingMessageID)
	}
	return &discordqotd.UpsertedAnswerMessage{
		ChannelID:  params.AnswerChannelID,
		MessageID:  messageID,
		MessageURL: discordqotd.BuildMessageJumpURL(params.GuildID, params.AnswerChannelID, messageID),
		Updated:    strings.TrimSpace(params.ExistingMessageID) != "",
	}, nil
}

func (p *fakePublisher) FetchThreadMessages(_ context.Context, _ *discordgo.Session, threadID string) ([]discordqotd.ArchivedMessage, error) {
	p.fetchCalls = append(p.fetchCalls, threadID)
	if p.fetchErrs != nil {
		if err := p.fetchErrs[threadID]; err != nil {
			return nil, err
		}
	}
	if p.threadMessages == nil {
		return nil, nil
	}
	return append([]discordqotd.ArchivedMessage(nil), p.threadMessages[threadID]...), nil
}

func (p *fakePublisher) FetchChannelMessages(_ context.Context, _ *discordgo.Session, channelID, beforeMessageID string, limit int) ([]discordqotd.ArchivedMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	messages := p.channelMessages[channelID]
	if len(messages) == 0 {
		return nil, nil
	}

	start := 0
	if beforeMessageID = strings.TrimSpace(beforeMessageID); beforeMessageID != "" {
		start = len(messages)
		for idx, message := range messages {
			if strings.TrimSpace(message.MessageID) == beforeMessageID {
				start = idx + 1
				break
			}
		}
	}
	if start >= len(messages) {
		return nil, nil
	}

	end := start + limit
	if end > len(messages) {
		end = len(messages)
	}
	return append([]discordqotd.ArchivedMessage(nil), messages[start:end]...), nil
}

func newTestQOTDService(t *testing.T) (*Service, *storage.Store, *fakePublisher) {
	t.Helper()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}
	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	fake := &fakePublisher{}
	service := NewService(configManager, store, fake)
	return service, store, fake
}

func TestServiceReorderQuestionsUsesOrderedIDs(t *testing.T) {
	service, _, _ := newTestQOTDService(t)

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "First question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	second, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Second question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}
	third, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Third question",
		Status: QuestionStatusDraft,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(third) failed: %v", err)
	}

	questions, err := service.ReorderQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, []int64{third.ID, first.ID, second.ID})
	if err != nil {
		t.Fatalf("ReorderQuestions() failed: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("expected 3 questions after reorder, got %d", len(questions))
	}
	if questions[0].ID != third.ID || questions[1].ID != first.ID || questions[2].ID != second.ID {
		t.Fatalf("unexpected order after reorder: %+v", questions)
	}
}

func TestBuildOfficialThreadNameMatchesForumTitleFormat(t *testing.T) {
	t.Parallel()

	got := buildOfficialThreadName("What's your go-to comfort drink?", 1)
	if got != "question of the day #1" {
		t.Fatalf("unexpected official thread title: %q", got)
	}
}

func TestSetupForumEnablesActiveDeckAndPersistsForumSurface(t *testing.T) {
	t.Parallel()

	service, store, fake := newTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:      files.LegacyQOTDDefaultDeckID,
			Name:    files.LegacyQOTDDefaultDeckName,
			Enabled: false,
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings(initial) failed: %v", err)
	}

	result, err := service.SetupForum(context.Background(), "g1", "", &discordgo.Session{})
	if err != nil {
		t.Fatalf("SetupForum() failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected setup result")
	}
	if len(fake.setupParams) != 1 {
		t.Fatalf("expected one setup publisher call, got %+v", fake.setupParams)
	}
	if fake.setupParams[0].GuildID != "g1" {
		t.Fatalf("unexpected setup params: %+v", fake.setupParams[0])
	}
	if result.ChannelID != "channel-setup-1" || result.QuestionListThreadID != "questions-list-thread" {
		t.Fatalf("unexpected setup result: %+v", result)
	}

	settings, err := service.GetSettings("g1")
	if err != nil {
		t.Fatalf("GetSettings() failed: %v", err)
	}
	deck, ok := settings.ActiveDeck()
	if !ok {
		t.Fatalf("expected active deck after setup: %+v", settings)
	}
	if !deck.Enabled || deck.ChannelID != "channel-setup-1" {
		t.Fatalf("expected setup to enable active deck and persist channel id, got %+v", deck)
	}

	surface, err := store.GetQOTDSurfaceByDeck(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetQOTDSurfaceByDeck() failed: %v", err)
	}
	if surface == nil || surface.ChannelID != "channel-setup-1" || surface.QuestionListThreadID != "questions-list-thread" {
		t.Fatalf("unexpected persisted qotd surface: %+v", surface)
	}
}

func TestServiceUpdateSettingsDeletesRemovedDeckQuestions(t *testing.T) {
	service, store, _ := newTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{
			{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "question-channel-1",
			},
			{
				ID:        "deck-b",
				Name:      "Deck B",
				Enabled:   false,
				ChannelID: "question-channel-2",
			},
		},
	}); err != nil {
		t.Fatalf("UpdateSettings(initial) failed: %v", err)
	}

	defaultQuestion, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		DeckID: "default",
		Body:   "Default question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(default) failed: %v", err)
	}
	deckBQuestion, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		DeckID: "deck-b",
		Body:   "Deck B question",
		Status: QuestionStatusDraft,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(deck-b) failed: %v", err)
	}

	updated, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{
			{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "question-channel-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateSettings(remove deck) failed: %v", err)
	}

	if len(updated.Decks) != 1 || updated.Decks[0].ID != files.LegacyQOTDDefaultDeckID {
		t.Fatalf("expected only default deck to remain, got %+v", updated.Decks)
	}

	allQuestions, err := store.ListQOTDQuestions(context.Background(), "g1", "")
	if err != nil {
		t.Fatalf("ListQOTDQuestions(all) failed: %v", err)
	}
	if len(allQuestions) != 1 || allQuestions[0].ID != defaultQuestion.ID {
		t.Fatalf("expected only default-deck questions to remain, got %+v", allQuestions)
	}

	deletedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", deckBQuestion.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(deck-b) failed: %v", err)
	}
	if deletedQuestion != nil {
		t.Fatalf("expected removed deck question to be deleted, got %+v", deletedQuestion)
	}
}

func TestServicePublishNowCreatesIndependentManualPost(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	oldQuestion, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Yesterday question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(old) failed: %v", err)
	}
	oldQuestion.Status = string(QuestionStatusUsed)
	oldUsedAt := time.Date(2026, 4, 2, 12, 43, 0, 0, time.UTC)
	oldQuestion.ScheduledForDateUTC = timePtr(time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC))
	oldQuestion.UsedAt = &oldUsedAt
	oldQuestion, err = store.UpdateQOTDQuestion(context.Background(), *oldQuestion)
	if err != nil {
		t.Fatalf("UpdateQOTDQuestion(old used) failed: %v", err)
	}

	oldLifecycle := EvaluateOfficialPost(time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), service.clock())
	oldOfficial, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           oldQuestion.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		State:                string(OfficialPostStateCurrent),
		ChannelID:            "123456789012345678",
		QuestionTextSnapshot: oldQuestion.Body,
		GraceUntil:           oldLifecycle.BecomesPreviousAt,
		ArchiveAt:            oldLifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(old) failed: %v", err)
	}
	oldOfficial, err = store.FinalizeQOTDOfficialPost(context.Background(), oldOfficial.ID, "questions-list-thread", "questions-list-entry-previous", "thread-previous", "message-previous", "thread-previous", oldUsedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost(old) failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), oldOfficial.ID, string(OfficialPostStateCurrent), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState(old) failed: %v", err)
	}

	nextQuestion, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Today question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(next) failed: %v", err)
	}
	_ = nextQuestion

	result, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one publish call, got %d", len(fake.publishedParams))
	}
	if fake.publishedParams[0].QuestionText != "Today question" {
		t.Fatalf("unexpected published question text: %+v", fake.publishedParams[0])
	}
	if fake.publishedParams[0].AvailableQuestions != 0 {
		t.Fatalf("expected no remaining available questions after manual publish, got %+v", fake.publishedParams[0])
	}
	if fake.publishedParams[0].ThreadName != "question of the day #2" {
		t.Fatalf("expected manual publish to use the daily thread title format, got %+v", fake.publishedParams[0])
	}
	if result.Question.Status != string(QuestionStatusUsed) {
		t.Fatalf("expected published question to be marked used, got %+v", result.Question)
	}
	if result.OfficialPost.State != string(OfficialPostStateCurrent) {
		t.Fatalf("expected current official post state, got %+v", result.OfficialPost)
	}
	if result.OfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected manual publish mode, got %+v", result.OfficialPost)
	}
	currentState, ok := fake.threadStates["thread-previous"]
	if !ok {
		t.Fatalf("expected scheduled thread state update, got %+v", fake.threadStates)
	}
	if currentState.Pinned || currentState.Locked || currentState.Archived {
		t.Fatalf("expected scheduled current thread to stay open and unpinned, got %+v", currentState)
	}

	previousOfficial, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(previous) failed: %v", err)
	}
	if previousOfficial == nil || previousOfficial.State != string(OfficialPostStateCurrent) {
		t.Fatalf("expected scheduled official post to remain current and unpinned before the boundary, got %+v", previousOfficial)
	}
}

func TestServiceSubmitAnswerCreatesAndUpdatesPerUserMessage(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "question-channel-1",
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "What did you learn today?",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	lifecycle := EvaluateOfficialPost(publishDate, service.clock())
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateCurrent),
		ChannelID:            "question-channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	official, err = store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "questions-list-thread", "question-entry-1", "", "question-message-1", "response-channel-1", publishedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), official.ID, string(OfficialPostStateCurrent), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState() failed: %v", err)
	}

	first, err := service.SubmitAnswer(context.Background(), &discordgo.Session{}, discordqotd.SubmitAnswerParams{
		GuildID:         "g1",
		OfficialPostID:  official.ID,
		UserID:          "user-7",
		UserDisplayName: "Answerer",
		UserAvatarURL:   "https://cdn.discordapp.com/avatars/user-7/avatar-hash.png?size=256",
		InteractionID:   "interaction-1",
		AnswerText:      "First answer",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer(first) failed: %v", err)
	}
	if first.Updated {
		t.Fatalf("expected first answer to create a message, got %+v", first)
	}
	if first.MessageURL == "" {
		t.Fatalf("expected first answer to return a message url, got %+v", first)
	}
	if len(fake.answerParams) != 1 || fake.answerParams[0].UserAvatarURL != "https://cdn.discordapp.com/avatars/user-7/avatar-hash.png?size=256" {
		t.Fatalf("expected answer publisher params to carry avatar url, got %+v", fake.answerParams)
	}
	if fake.answerParams[0].DeckName != files.LegacyQOTDDefaultDeckName {
		t.Fatalf("expected answer publisher params to carry deck name snapshot, got %+v", fake.answerParams[0])
	}
	if got := fake.answerParams[0].PublishDateUTC; !got.Equal(publishDate) {
		t.Fatalf("expected answer publisher params to carry publish date, got %v", got)
	}

	stored, err := store.GetQOTDAnswerMessageByOfficialPostAndUser(context.Background(), official.ID, "user-7")
	if err != nil {
		t.Fatalf("GetQOTDAnswerMessageByOfficialPostAndUser() failed: %v", err)
	}
	if stored == nil || stored.State != string(AnswerRecordStateActive) || stored.DiscordMessageID == "" {
		t.Fatalf("expected active stored answer record, got %+v", stored)
	}
	second, err := service.SubmitAnswer(context.Background(), &discordgo.Session{}, discordqotd.SubmitAnswerParams{
		GuildID:         "g1",
		OfficialPostID:  official.ID,
		UserID:          "user-7",
		UserDisplayName: "Answerer",
		UserAvatarURL:   "https://cdn.discordapp.com/avatars/user-7/avatar-hash.png?size=256",
		InteractionID:   "interaction-2",
		AnswerText:      "Edited answer",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer(second) failed: %v", err)
	}
	if !second.Updated {
		t.Fatalf("expected second answer to update the existing message, got %+v", second)
	}
	if second.MessageURL != first.MessageURL {
		t.Fatalf("expected updated answer to keep the same message url, got %q want %q", second.MessageURL, first.MessageURL)
	}
	if strings.TrimSpace(stored.AnswerChannelID) != "response-channel-1" {
		t.Fatalf("expected stored answer record to track the response channel, got %+v", stored)
	}
}

func TestServiceSubmitAnswerTargetsOfficialThreadWhenAvailable(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "question-channel-1",
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Thread-backed question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	lifecycle := EvaluateOfficialPost(publishDate, service.clock())
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateCurrent),
		ChannelID:            "question-channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	official, err = store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "questions-list-thread", "question-entry-1", "official-thread-1", "question-message-1", "official-thread-1", publishedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), official.ID, string(OfficialPostStateCurrent), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState() failed: %v", err)
	}

	result, err := service.SubmitAnswer(context.Background(), &discordgo.Session{}, discordqotd.SubmitAnswerParams{
		GuildID:         "g1",
		OfficialPostID:  official.ID,
		UserID:          "user-7",
		UserDisplayName: "Answerer",
		InteractionID:   "interaction-1",
		AnswerText:      "Thread answer",
	})
	if err != nil {
		t.Fatalf("SubmitAnswer() failed: %v", err)
	}
	if result.ChannelID != "official-thread-1" {
		t.Fatalf("expected answer to target the official thread, got %+v", result)
	}
	if len(fake.answerParams) != 1 || fake.answerParams[0].AnswerChannelID != "official-thread-1" {
		t.Fatalf("expected publisher params to target the official thread, got %+v", fake.answerParams)
	}

	stored, err := store.GetQOTDAnswerMessageByOfficialPostAndUser(context.Background(), official.ID, "user-7")
	if err != nil {
		t.Fatalf("GetQOTDAnswerMessageByOfficialPostAndUser() failed: %v", err)
	}
	if stored == nil || strings.TrimSpace(stored.AnswerChannelID) != "official-thread-1" {
		t.Fatalf("expected stored answer record to track the official thread id, got %+v", stored)
	}
}

func TestServiceCollectArchivedQuestionsStoresMatchedEmbeds(t *testing.T) {
	service, _, fake := newTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		Collector: files.QOTDCollectorConfig{
			SourceChannelID: "123456789012345678",
			AuthorIDs:       []string{"999999999999999999"},
			TitlePatterns:   []string{"Question Of The Day", "question!!"},
			StartDate:       "2026-01-01",
		},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	fake.channelMessages = map[string][]discordqotd.ArchivedMessage{
		"123456789012345678": {
			{
				MessageID:          "",
				AuthorID:           "999999999999999999",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"This malformed message should be skipped"}]`),
				CreatedAt:          time.Date(2026, 4, 13, 16, 0, 0, 0, time.UTC),
			},
			{
				MessageID:          "message-3",
				AuthorID:           "999999999999999999",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"✰ question!! ✰","description":"What food have you never eaten but would really like to try?\nAuthor: QOTD Bot"}]`),
				CreatedAt:          time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC),
			},
			{
				MessageID:          "message-2",
				AuthorID:           "999999999999999999",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"Tell us about a person you look up to!\n\nPreset Question"}]`),
				CreatedAt:          time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC),
			},
			{
				MessageID:          "message-1",
				AuthorID:           "555555555555555555",
				AuthorNameSnapshot: "Ignored Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"Ignored question"}]`),
				CreatedAt:          time.Date(2025, 12, 31, 15, 0, 0, 0, time.UTC),
			},
		},
	}

	result, err := service.CollectArchivedQuestions(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("CollectArchivedQuestions() failed: %v", err)
	}
	if result.ScannedMessages != 4 || result.MatchedMessages != 2 || result.NewQuestions != 2 || result.TotalQuestions != 2 {
		t.Fatalf("unexpected collector result: %+v", result)
	}

	summary, err := service.GetCollectorSummary(context.Background(), "g1")
	if err != nil {
		t.Fatalf("GetCollectorSummary() failed: %v", err)
	}
	if summary.TotalQuestions != 2 || len(summary.RecentQuestions) != 2 {
		t.Fatalf("unexpected collector summary: %+v", summary)
	}
	if summary.RecentQuestions[0].QuestionText != "What food have you never eaten but would really like to try?" {
		t.Fatalf("expected most recent collected question first, got %+v", summary.RecentQuestions)
	}

	exported, err := service.ExportCollectedQuestionsTXT(context.Background(), "g1")
	if err != nil {
		t.Fatalf("ExportCollectedQuestionsTXT() failed: %v", err)
	}
	expected := "Tell us about a person you look up to!\nWhat food have you never eaten but would really like to try?\n"
	if exported != expected {
		t.Fatalf("unexpected exported collector text:\n%s", exported)
	}
}

func TestServicePublishScheduledIfDueCreatesScheduledPost(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "question-channel-1",
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Scheduled question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Reserve for later",
		Status: QuestionStatusDraft,
	}); err != nil {
		t.Fatalf("CreateQuestion(draft) failed: %v", err)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected scheduled publish to run after the boundary")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one scheduled publish call, got %d", len(fake.publishedParams))
	}
	if got := fake.publishedParams[0].PublishDateUTC; !got.Equal(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected scheduled publish date: %s", got.Format(time.RFC3339))
	}
	if fake.publishedParams[0].AvailableQuestions != 1 {
		t.Fatalf("expected one remaining available question after scheduled publish, got %+v", fake.publishedParams[0])
	}
	if fake.publishedParams[0].ThreadName != "question of the day #1" {
		t.Fatalf("expected scheduled publish to use the daily thread title format, got %+v", fake.publishedParams[0])
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.State != string(OfficialPostStateCurrent) {
		t.Fatalf("expected current unpinned scheduled official post, got %+v", official)
	}
	if official.DiscordThreadID == "" {
		t.Fatalf("expected scheduled publish to persist the official thread id, got %+v", official)
	}
	if fake.threadStates[official.DiscordThreadID] != (discordqotd.ThreadState{Pinned: false, Locked: false, Archived: false}) {
		t.Fatalf("expected the daily thread to remain open for answers, got %+v", fake.threadStates[official.DiscordThreadID])
	}

	usedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if usedQuestion == nil || usedQuestion.Status != string(QuestionStatusUsed) || usedQuestion.UsedAt == nil {
		t.Fatalf("expected scheduled question to be marked used, got %+v", usedQuestion)
	}
}

func TestServicePublishScheduledIfDueResumesFailedProvisioning(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Recoverable scheduled question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	fake.publishResponses = []fakePublishResponse{
		{
			result: &discordqotd.PublishedOfficialPost{
				QuestionListThreadID: "questions-list-thread",
				ThreadID:             "thread-partial",
				StarterMessageID:     "starter-partial",
				AnswerChannelID:      "thread-partial",
				PostURL:              discordqotd.BuildThreadJumpURL("g1", "thread-partial"),
			},
			err: errFakePublishFailed,
		},
		{
			result: &discordqotd.PublishedOfficialPost{
				QuestionListThreadID:       "questions-list-thread",
				QuestionListEntryMessageID: "list-entry-recovered",
				ThreadID:                   "thread-partial",
				StarterMessageID:           "starter-partial",
				AnswerChannelID:            "thread-partial",
				PublishedAt:                time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
				PostURL:                    discordqotd.BuildThreadJumpURL("g1", "thread-partial"),
			},
		},
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err == nil || !strings.Contains(err.Error(), errFakePublishFailed.Error()) {
		t.Fatalf("expected publish failure to surface, got published=%v err=%v", published, err)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	stored, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(partial) failed: %v", err)
	}
	if stored == nil || stored.State != string(OfficialPostStateFailed) {
		t.Fatalf("expected failed provisioning record to remain stored, got %+v", stored)
	}
	if stored.DiscordThreadID != "thread-partial" || stored.DiscordStarterMessageID != "starter-partial" {
		t.Fatalf("expected partial discord artifacts to persist, got %+v", stored)
	}
	if strings.TrimSpace(stored.QuestionListEntryMessageID) != "" {
		t.Fatalf("expected list entry to remain missing after partial failure, got %+v", stored)
	}

	reservedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(reserved) failed: %v", err)
	}
	if reservedQuestion == nil || reservedQuestion.Status != string(QuestionStatusReserved) {
		t.Fatalf("expected question to stay reserved during failed provisioning, got %+v", reservedQuestion)
	}

	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(recovery) failed: %v", err)
	}
	if !published {
		t.Fatal("expected recovery publish to report work performed")
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected two publish attempts, got %d", len(fake.publishedParams))
	}
	if fake.publishedParams[1].OfficialThreadID != "thread-partial" || fake.publishedParams[1].OfficialStarterMessageID != "starter-partial" {
		t.Fatalf("expected recovery attempt to reuse persisted discord artifacts, got %+v", fake.publishedParams[1])
	}

	recovered, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(recovered) failed: %v", err)
	}
	if recovered == nil || recovered.State != string(OfficialPostStateCurrent) || recovered.ID != stored.ID {
		t.Fatalf("expected same official post to recover into current state, got %+v", recovered)
	}
	if recovered.QuestionListEntryMessageID != "list-entry-recovered" || recovered.PublishedAt == nil {
		t.Fatalf("expected recovered official post to finalize list entry and published timestamp, got %+v", recovered)
	}

	usedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(used) failed: %v", err)
	}
	if usedQuestion == nil || usedQuestion.Status != string(QuestionStatusUsed) || usedQuestion.UsedAt == nil {
		t.Fatalf("expected recovered publish to mark question used, got %+v", usedQuestion)
	}
}

func TestServiceReconcileGuildRecoversPendingOfficialPostProvisioning(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Pending recovery question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}
	question.Status = string(QuestionStatusReserved)
	if question, err = store.UpdateQOTDQuestion(context.Background(), *question); err != nil {
		t.Fatalf("UpdateQOTDQuestion(reserved) failed: %v", err)
	}

	lifecycle := EvaluateOfficialPost(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC), service.clock())
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:                    "g1",
		DeckID:                     files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:           files.LegacyQOTDDefaultDeckName,
		QuestionID:                 question.ID,
		PublishMode:                string(PublishModeScheduled),
		PublishDateUTC:             time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                      string(OfficialPostStateFailed),
		ChannelID:                  "123456789012345678",
		QuestionListThreadID:       "questions-list-thread",
		DiscordThreadID:            "thread-recover",
		DiscordStarterMessageID:    "starter-recover",
		AnswerChannelID:            "thread-recover",
		QuestionTextSnapshot:       question.Body,
		GraceUntil:                 lifecycle.BecomesPreviousAt,
		ArchiveAt:                  lifecycle.ArchiveAt,
		QuestionListEntryMessageID: "",
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	fake.publishResponses = []fakePublishResponse{{
		result: &discordqotd.PublishedOfficialPost{
			QuestionListThreadID:       "questions-list-thread",
			QuestionListEntryMessageID: "list-entry-recovered",
			ThreadID:                   "thread-recover",
			StarterMessageID:           "starter-recover",
			AnswerChannelID:            "thread-recover",
			PublishedAt:                time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC),
			PostURL:                    discordqotd.BuildThreadJumpURL("g1", "thread-recover"),
		},
	}}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild() failed: %v", err)
	}

	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one recovery publish attempt, got %d", len(fake.publishedParams))
	}
	if fake.publishedParams[0].OfficialThreadID != "thread-recover" || fake.publishedParams[0].QuestionListThreadID != "questions-list-thread" {
		t.Fatalf("expected reconcile to reuse stored artifacts, got %+v", fake.publishedParams[0])
	}

	recovered, err := store.GetQOTDOfficialPostByID(context.Background(), official.ID)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByID() failed: %v", err)
	}
	if recovered == nil || recovered.State != string(OfficialPostStateCurrent) || recovered.QuestionListEntryMessageID != "list-entry-recovered" {
		t.Fatalf("expected reconcile to finalize pending official post, got %+v", recovered)
	}

	usedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if usedQuestion == nil || usedQuestion.Status != string(QuestionStatusUsed) || usedQuestion.UsedAt == nil {
		t.Fatalf("expected reconcile to mark the recovered question used, got %+v", usedQuestion)
	}
}

func TestServiceReconcileGuildArchivesExpiredPostsAndAnswerRecords(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}
	fake.threadMessages = map[string][]discordqotd.ArchivedMessage{
		"official-thread-archive": {
			{
				MessageID:          "official-message-1",
				AuthorID:           "user-1",
				AuthorNameSnapshot: "Author One",
				Content:            "Official archive snapshot",
				CreatedAt:          time.Date(2026, 4, 1, 12, 43, 0, 0, time.UTC),
			},
		},
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Archive me",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	publishDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 4, 1, 12, 43, 0, 0, time.UTC)
	lifecycle := EvaluateOfficialPost(publishDate, service.clock())
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStatePrevious),
		ChannelID:            "forum-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	official, err = store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "questions-list-thread", "questions-list-entry-archive", "official-thread-archive", "official-message-archive", "official-thread-archive", publishedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), official.ID, string(OfficialPostStatePrevious), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState() failed: %v", err)
	}

	answerRecord, err := store.CreateQOTDAnswerMessage(context.Background(), storage.QOTDAnswerMessageRecord{
		GuildID:         "g1",
		OfficialPostID:  official.ID,
		UserID:          "user-2",
		State:           string(AnswerRecordStateActive),
		AnswerChannelID: "official-thread-archive",
	})
	if err != nil {
		t.Fatalf("CreateQOTDAnswerMessage() failed: %v", err)
	}
	answerRecord, err = store.FinalizeQOTDAnswerMessage(context.Background(), answerRecord.ID, "reply-message-starter")
	if err != nil {
		t.Fatalf("FinalizeQOTDAnswerMessage() failed: %v", err)
	}
	if _, err := store.UpdateQOTDAnswerMessageState(context.Background(), answerRecord.ID, string(AnswerRecordStateActive), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDAnswerMessageState() failed: %v", err)
	}

	if _, err := store.CreateQOTDThreadArchive(context.Background(), storage.QOTDThreadArchiveRecord{
		GuildID:         "g1",
		OfficialPostID:  official.ID,
		SourceKind:      qotdArchiveSourceOfficial,
		DiscordThreadID: "official-thread-archive",
		ArchivedAt:      service.clock().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("CreateQOTDThreadArchive(existing official) failed: %v", err)
	}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild() failed: %v", err)
	}

	updatedOfficial, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(updated) failed: %v", err)
	}
	if updatedOfficial == nil || updatedOfficial.State != string(OfficialPostStateArchived) || updatedOfficial.ArchivedAt == nil {
		t.Fatalf("expected archived official post after reconcile, got %+v", updatedOfficial)
	}

	updatedReply, err := store.GetQOTDAnswerMessageByOfficialPostAndUser(context.Background(), official.ID, "user-2")
	if err != nil {
		t.Fatalf("GetQOTDAnswerMessageByOfficialPostAndUser() failed: %v", err)
	}
	if updatedReply == nil || updatedReply.State != string(AnswerRecordStateArchived) || updatedReply.ArchivedAt == nil {
		t.Fatalf("expected archived answer record after reconcile, got %+v", updatedReply)
	}

	officialArchive, err := store.GetQOTDThreadArchiveByThreadID(context.Background(), "official-thread-archive")
	if err != nil {
		t.Fatalf("GetQOTDThreadArchiveByThreadID(official) failed: %v", err)
	}
	if officialArchive == nil {
		t.Fatal("expected official archive record to exist after reconcile")
	}
	if len(fake.fetchCalls) != 1 {
		t.Fatalf("expected reconcile to fetch the official thread archive only, got %v", fake.fetchCalls)
	}
	if fake.threadStates["official-thread-archive"] != (discordqotd.ThreadState{Pinned: false, Locked: true, Archived: true}) {
		t.Fatalf("expected archived official thread state, got %+v", fake.threadStates["official-thread-archive"])
	}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild(second) failed: %v", err)
	}
	if len(fake.fetchCalls) != 1 {
		t.Fatalf("expected archived posts to be skipped on repeat reconcile, got fetches=%v", fake.fetchCalls)
	}
}

func timePtr(value time.Time) *time.Time {
	normalized := value.UTC()
	return &normalized
}

func timePtrString(id int64) string {
	return strconv.FormatInt(id, 10)
}
