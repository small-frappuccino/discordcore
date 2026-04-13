package qotd

import (
	"context"
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

type fakePublisher struct {
	publishedParams []discordqotd.PublishOfficialPostParams
	answerParams    []discordqotd.UpsertAnswerMessageParams
	threadStates    map[string]discordqotd.ThreadState
	fetchCalls      []string
	threadMessages  map[string][]discordqotd.ArchivedMessage
	fetchErrs       map[string]error
}

func (p *fakePublisher) PublishOfficialPost(_ context.Context, _ *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	p.publishedParams = append(p.publishedParams, params)
	messageID := "message-" + timePtrString(params.OfficialPostID)
	return &discordqotd.PublishedOfficialPost{
		StarterMessageID: messageID,
		PublishedAt:      time.Date(params.PublishDateUTC.Year(), params.PublishDateUTC.Month(), params.PublishDateUTC.Day(), 12, 43, 0, 0, time.UTC),
		PostURL:          discordqotd.BuildMessageJumpURL(params.GuildID, params.QuestionChannelID, messageID),
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
		ChannelID:  params.ResponseChannelID,
		MessageID:  messageID,
		MessageURL: discordqotd.BuildMessageJumpURL(params.GuildID, params.ResponseChannelID, messageID),
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

func TestServiceUpdateSettingsDeletesRemovedDeckQuestions(t *testing.T) {
	service, store, _ := newTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{
			{
				ID:                files.LegacyQOTDDefaultDeckID,
				Name:              files.LegacyQOTDDefaultDeckName,
				Enabled:           true,
				QuestionChannelID: "question-channel-1",
				ResponseChannelID: "answers-channel-1",
			},
			{
				ID:                "deck-b",
				Name:              "Deck B",
				Enabled:           false,
				QuestionChannelID: "question-channel-2",
				ResponseChannelID: "answers-channel-2",
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
				ID:                files.LegacyQOTDDefaultDeckID,
				Name:              files.LegacyQOTDDefaultDeckName,
				Enabled:           true,
				QuestionChannelID: "question-channel-1",
				ResponseChannelID: "answers-channel-1",
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
		Enabled:           true,
		QuestionChannelID: "123456789012345678",
		ResponseChannelID: "223456789012345679",
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
		ForumChannelID:       "123456789012345678",
		ResponseChannelID:    "223456789012345679",
		QuestionTextSnapshot: oldQuestion.Body,
		IsPinned:             true,
		GraceUntil:           oldLifecycle.BecomesPreviousAt,
		ArchiveAt:            oldLifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(old) failed: %v", err)
	}
	oldOfficial, err = store.FinalizeQOTDOfficialPost(context.Background(), oldOfficial.ID, "thread-previous", "message-previous", oldUsedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost(old) failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), oldOfficial.ID, string(OfficialPostStateCurrent), true, nil, nil); err != nil {
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
	if fake.publishedParams[0].Pinned {
		t.Fatalf("expected manual publish to stay unpinned, got %+v", fake.publishedParams[0])
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
	if result.OfficialPost.IsPinned {
		t.Fatalf("expected manual official post to remain unpinned, got %+v", result.OfficialPost)
	}

	currentState, ok := fake.threadStates["thread-previous"]
	if !ok {
		t.Fatalf("expected scheduled thread state update, got %+v", fake.threadStates)
	}
	if !currentState.Pinned || !currentState.Locked || currentState.Archived {
		t.Fatalf("expected scheduled current thread to stay pinned locked and unarchived, got %+v", currentState)
	}

	previousOfficial, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(previous) failed: %v", err)
	}
	if previousOfficial == nil || previousOfficial.State != string(OfficialPostStateCurrent) || !previousOfficial.IsPinned {
		t.Fatalf("expected scheduled official post to remain current/pinned before the boundary, got %+v", previousOfficial)
	}
}

func TestServiceSubmitAnswerCreatesAndUpdatesPerUserMessage(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		Enabled:           true,
		QuestionChannelID: "question-channel-1",
		ResponseChannelID: "answers-channel-1",
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
		ForumChannelID:       "question-channel-1",
		ResponseChannelID:    "response-channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	official, err = store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "", "question-message-1", publishedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), official.ID, string(OfficialPostStateCurrent), false, nil, nil); err != nil {
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

	stored, err := store.GetQOTDReplyThreadByOfficialPostAndUser(context.Background(), official.ID, "user-7")
	if err != nil {
		t.Fatalf("GetQOTDReplyThreadByOfficialPostAndUser() failed: %v", err)
	}
	if stored == nil || stored.State != string(ReplyThreadStateActive) || stored.DiscordStarterMessageID == "" {
		t.Fatalf("expected active stored answer record, got %+v", stored)
	}
	if stored.DiscordThreadID != "" {
		t.Fatalf("expected message-based answers to avoid thread ids, got %+v", stored)
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
	if strings.TrimSpace(stored.ForumChannelID) != "answers-channel-1" {
		t.Fatalf("expected stored answer record to track the response channel, got %+v", stored)
	}
}

func TestServicePublishScheduledIfDueCreatesScheduledPost(t *testing.T) {
	service, store, fake := newTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		Enabled:           true,
		QuestionChannelID: "question-channel-1",
		ResponseChannelID: "answers-channel-1",
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
	if !fake.publishedParams[0].Pinned {
		t.Fatalf("expected scheduled publish to pin the current slot thread, got %+v", fake.publishedParams[0])
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.State != string(OfficialPostStateCurrent) || !official.IsPinned {
		t.Fatalf("expected current pinned scheduled official post, got %+v", official)
	}

	usedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if usedQuestion == nil || usedQuestion.Status != string(QuestionStatusUsed) || usedQuestion.UsedAt == nil {
		t.Fatalf("expected scheduled question to be marked used, got %+v", usedQuestion)
	}
}

func TestServiceReconcileGuildArchivesExpiredPostsAndReplyThreads(t *testing.T) {
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
		"reply-thread-archive": {
			{
				MessageID:          "reply-message-1",
				AuthorID:           "user-2",
				AuthorNameSnapshot: "Answerer",
				Content:            "Reply archive snapshot",
				CreatedAt:          time.Date(2026, 4, 2, 14, 0, 0, 0, time.UTC),
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
		ForumChannelID:       "forum-1",
		ResponseChannelID:    "response-channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	official, err = store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "official-thread-archive", "official-message-archive", publishedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), official.ID, string(OfficialPostStatePrevious), false, nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState() failed: %v", err)
	}

	replyThread, err := store.CreateQOTDReplyThreadProvisioning(context.Background(), storage.QOTDReplyThreadRecord{
		GuildID:        "g1",
		OfficialPostID: official.ID,
		UserID:         "user-2",
		State:          string(ReplyThreadStateActive),
		ForumChannelID: "forum-1",
	})
	if err != nil {
		t.Fatalf("CreateQOTDReplyThreadProvisioning() failed: %v", err)
	}
	replyThread, err = store.FinalizeQOTDReplyThread(context.Background(), replyThread.ID, "reply-thread-archive", "reply-message-starter")
	if err != nil {
		t.Fatalf("FinalizeQOTDReplyThread() failed: %v", err)
	}
	if _, err := store.UpdateQOTDReplyThreadState(context.Background(), replyThread.ID, string(ReplyThreadStateActive), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDReplyThreadState() failed: %v", err)
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

	updatedReply, err := store.GetQOTDReplyThreadByOfficialPostAndUser(context.Background(), official.ID, "user-2")
	if err != nil {
		t.Fatalf("GetQOTDReplyThreadByOfficialPostAndUser() failed: %v", err)
	}
	if updatedReply == nil || updatedReply.State != string(ReplyThreadStateArchived) || updatedReply.ArchivedAt == nil {
		t.Fatalf("expected archived reply thread after reconcile, got %+v", updatedReply)
	}

	officialArchive, err := store.GetQOTDThreadArchiveByThreadID(context.Background(), "official-thread-archive")
	if err != nil {
		t.Fatalf("GetQOTDThreadArchiveByThreadID(official) failed: %v", err)
	}
	if officialArchive == nil {
		t.Fatal("expected official archive record to exist after reconcile")
	}
	replyArchive, err := store.GetQOTDThreadArchiveByThreadID(context.Background(), "reply-thread-archive")
	if err != nil {
		t.Fatalf("GetQOTDThreadArchiveByThreadID(reply) failed: %v", err)
	}
	if replyArchive == nil {
		t.Fatal("expected reply archive record to exist after reconcile")
	}

	if len(fake.fetchCalls) != 2 {
		t.Fatalf("expected reconcile to fetch both official and reply thread archives, got %v", fake.fetchCalls)
	}
	if fake.threadStates["official-thread-archive"] != (discordqotd.ThreadState{Pinned: false, Locked: true, Archived: true}) {
		t.Fatalf("expected archived official thread state, got %+v", fake.threadStates["official-thread-archive"])
	}
	if fake.threadStates["reply-thread-archive"] != (discordqotd.ThreadState{Pinned: false, Locked: true, Archived: true}) {
		t.Fatalf("expected archived reply thread state, got %+v", fake.threadStates["reply-thread-archive"])
	}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild(second) failed: %v", err)
	}
	if len(fake.fetchCalls) != 2 {
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
