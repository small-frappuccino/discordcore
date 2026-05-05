//go:build integration

package qotd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgconn"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

var errFakePublishFailed = errors.New("fake publish failed")

const (
	integrationQuestionChannelID    = "123456789012345678"
	integrationQuestionChannelIDAlt = "223456789012345678"
	integrationCollectorChannelID   = "323456789012345678"
	integrationForumChannelID       = "423456789012345678"
)

func scheduledQOTDConfig(enabled bool, channelID string) files.QOTDConfig {
	hourUTC := 12
	minuteUTC := 43
	return files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		},
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   enabled,
			ChannelID: channelID,
		}},
	}
}

type fakePublisher struct {
	publishedParams  []discordqotd.PublishOfficialPostParams
	publishResponses []fakePublishResponse
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

type blockingPublisher struct {
	fakePublisher
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (p *blockingPublisher) PublishOfficialPost(ctx context.Context, session *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	p.once.Do(func() {
		if p.started != nil {
			close(p.started)
		}
	})
	if p.release != nil {
		<-p.release
	}
	return p.fakePublisher.PublishOfficialPost(ctx, session, params)
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
		PostURL:                    discordqotd.BuildMessageJumpURL(params.GuildID, params.ChannelID, messageID),
	}
}

func (p *fakePublisher) SetThreadState(_ context.Context, _ *discordgo.Session, threadID string, state discordqotd.ThreadState) error {
	if p.threadStates == nil {
		p.threadStates = make(map[string]discordqotd.ThreadState)
	}
	p.threadStates[threadID] = state
	return nil
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

func newIntegrationTestQOTDService(t *testing.T) (*Service, *storage.Store, *fakePublisher) {
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
	service, _, _ := newIntegrationTestQOTDService(t)

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
	if questions[0].DisplayID != 1 || questions[1].DisplayID != 2 || questions[2].DisplayID != 3 {
		t.Fatalf("expected reordered questions to receive sequential visible ids, got %+v", questions)
	}
}

func TestServiceReorderQuestionsChangesNextPublishSelection(t *testing.T) {
	service, _, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "First question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	second, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Second question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}
	third, err := service.CreateQuestion(context.Background(), "g1", "user-3", QuestionMutation{
		Body:   "Third question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(third) failed: %v", err)
	}

	if _, err := service.ReorderQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, []int64{third.ID, first.ID, second.ID}); err != nil {
		t.Fatalf("ReorderQuestions() failed: %v", err)
	}

	result, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}
	if result.Question.ID != third.ID {
		t.Fatalf("expected reordered question to publish next, got %+v", result.Question)
	}
	if len(fake.publishedParams) != 1 || fake.publishedParams[0].QuestionText != "Third question" {
		t.Fatalf("expected publish to use reordered first question, got %+v", fake.publishedParams)
	}
	if fake.publishedParams[0].DisplayID != 1 {
		t.Fatalf("expected reordered publish to keep visible id 1, got %+v", fake.publishedParams[0])
	}
}

func TestServiceSetNextQuestionMovesSelectedReadyQuestionToNextSlot(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created := make([]*storage.QOTDQuestionRecord, 0, 6)
	for idx := 1; idx <= 6; idx++ {
		question, err := service.CreateQuestion(context.Background(), "g1", fmt.Sprintf("user-%d", idx), QuestionMutation{
			Body:   fmt.Sprintf("Question %d", idx),
			Status: QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	for idx := 0; idx < 4; idx++ {
		usedAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
		created[idx].Status = string(QuestionStatusUsed)
		created[idx].UsedAt = &usedAt
		if _, err := store.UpdateQOTDQuestion(context.Background(), *created[idx]); err != nil {
			t.Fatalf("UpdateQOTDQuestion(%d) failed: %v", idx, err)
		}
	}

	moved, err := service.SetNextQuestion(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, created[5].ID)
	if err != nil {
		t.Fatalf("SetNextQuestion() failed: %v", err)
	}
	if moved == nil || moved.ID != created[5].ID {
		t.Fatalf("expected moved question to be returned, got %+v", moved)
	}
	if moved.DisplayID != 5 {
		t.Fatalf("expected moved question to become visible ID 5, got %+v", moved)
	}

	questions, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(questions) != 6 {
		t.Fatalf("expected six questions after reorder, got %+v", questions)
	}
	if questions[4].ID != created[5].ID || questions[4].DisplayID != 5 {
		t.Fatalf("expected selected question to become next ready slot, got %+v", questions)
	}
	if questions[5].ID != created[4].ID || questions[5].DisplayID != 6 {
		t.Fatalf("expected previous next question to shift back one slot, got %+v", questions)
	}
}

func TestServiceSetNextQuestionReturnsImmutableForUsedQuestion(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question 1",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	_, err = service.SetNextQuestion(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, created.ID)
	if !errors.Is(err, ErrImmutableQuestion) {
		t.Fatalf("expected ErrImmutableQuestion, got %v", err)
	}
}

func TestServiceRestoreUsedQuestionMovesQuestionBackToReady(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created := make([]*storage.QOTDQuestionRecord, 0, 4)
	for idx := 1; idx <= 4; idx++ {
		question, err := service.CreateQuestion(context.Background(), "g1", fmt.Sprintf("user-%d", idx), QuestionMutation{
			Body:   fmt.Sprintf("Question %d", idx),
			Status: QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	for _, idx := range []int{0, 3} {
		usedAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
		created[idx].Status = string(QuestionStatusUsed)
		created[idx].UsedAt = &usedAt
		if idx == 3 {
			publishedOnceAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
			slotDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
			created[idx].PublishedOnceAt = &publishedOnceAt
			created[idx].ScheduledForDateUTC = &slotDate
		}
		if _, err := store.UpdateQOTDQuestion(context.Background(), *created[idx]); err != nil {
			t.Fatalf("UpdateQOTDQuestion(%d) failed: %v", idx, err)
		}
	}

	restored, err := service.RestoreUsedQuestion(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, created[3].ID)
	if err != nil {
		t.Fatalf("RestoreUsedQuestion() failed: %v", err)
	}
	if restored == nil {
		t.Fatal("expected restored question")
	}
	if restored.Status != string(QuestionStatusReady) {
		t.Fatalf("expected restored status ready, got %+v", restored)
	}
	if restored.DisplayID != 2 {
		t.Fatalf("expected restored question to move ahead of the prior next-ready slot and become visible ID 2, got %+v", restored)
	}
	if restored.UsedAt != nil || restored.PublishedOnceAt != nil || restored.ScheduledForDateUTC != nil {
		t.Fatalf("expected restore to clear used/scheduled/published markers, got %+v", restored)
	}

	persisted, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(persisted) != 4 {
		t.Fatalf("expected four questions after recover, got %+v", persisted)
	}
	if persisted[1].ID != created[3].ID || persisted[1].DisplayID != 2 {
		t.Fatalf("expected recovered question to move to visible ID 2, got %+v", persisted)
	}
	if persisted[2].ID != created[1].ID || persisted[2].DisplayID != 3 {
		t.Fatalf("expected previous next-ready question to shift back one slot, got %+v", persisted)
	}
}

func TestServiceRestoreUsedQuestionRejectsNonUsedQuestion(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Still ready",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	_, err = service.RestoreUsedQuestion(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, created.ID)
	if !errors.Is(err, ErrQuestionNotUsed) {
		t.Fatalf("expected ErrQuestionNotUsed, got %v", err)
	}
}

func TestServiceRestoreUsedQuestionKeepsRecoveredQuestionAheadOfPriorNextReady(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created := make([]*storage.QOTDQuestionRecord, 0, 4)
	for idx := 1; idx <= 4; idx++ {
		question, err := service.CreateQuestion(context.Background(), "g1", fmt.Sprintf("user-%d", idx), QuestionMutation{
			Body:   fmt.Sprintf("Question %d", idx),
			Status: QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	for _, idx := range []int{0, 1} {
		usedAt := time.Date(2026, 4, 3, 13, idx, 0, 0, time.UTC)
		created[idx].Status = string(QuestionStatusUsed)
		created[idx].UsedAt = &usedAt
		if _, err := store.UpdateQOTDQuestion(context.Background(), *created[idx]); err != nil {
			t.Fatalf("UpdateQOTDQuestion(%d) failed: %v", idx, err)
		}
	}

	restored, err := service.RestoreUsedQuestion(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, created[0].ID)
	if err != nil {
		t.Fatalf("RestoreUsedQuestion() failed: %v", err)
	}
	if restored == nil {
		t.Fatal("expected restored question")
	}
	if restored.DisplayID != 1 {
		t.Fatalf("expected restored question to remain ahead of the prior ready queue at visible ID 1, got %+v", restored)
	}

	persisted, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(persisted) != 4 {
		t.Fatalf("expected four questions after recover, got %+v", persisted)
	}
	if persisted[0].ID != created[0].ID || persisted[0].DisplayID != 1 || persisted[0].Status != string(QuestionStatusReady) {
		t.Fatalf("expected recovered question to become the next ready question at visible ID 1, got %+v", persisted)
	}
	if persisted[1].ID != created[1].ID || persisted[1].Status != string(QuestionStatusUsed) {
		t.Fatalf("expected the other used question to keep its relative position behind the recovered one, got %+v", persisted)
	}
	if persisted[2].ID != created[2].ID || persisted[2].DisplayID != 3 {
		t.Fatalf("expected the prior next-ready question to stay behind the recovered one, got %+v", persisted)
	}
}

func TestServiceUpdateSettingsDeletesRemovedDeckQuestions(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)

	hourUTC := 12
	minuteUTC := 43
	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		},
		Decks: []files.QOTDDeckConfig{
			{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: integrationQuestionChannelID,
			},
			{
				ID:        "deck-b",
				Name:      "Deck B",
				Enabled:   false,
				ChannelID: integrationQuestionChannelIDAlt,
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

	updated, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID))
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

func TestServicePublishNowCreatesCurrentSlotManualPostAlongsidePreviousDayPost(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
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

	schedule, err := resolvePublishSchedule(scheduledQOTDConfig(true, "123456789012345678"))
	if err != nil {
		t.Fatalf("resolvePublishSchedule() failed: %v", err)
	}
	oldLifecycle := EvaluateOfficialPost(schedule, time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), service.clock())
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
		t.Fatalf("expected manual publish to consume the next queue question, got %+v", fake.publishedParams[0])
	}
	if fake.publishedParams[0].ThreadName != "Question of the Day" {
		t.Fatalf("expected manual publish to use the daily thread title format, got %+v", fake.publishedParams[0])
	}
	if result.Question.Status != string(QuestionStatusUsed) || result.Question.UsedAt == nil {
		t.Fatalf("expected manual publish to mark the queue question used, got %+v", result.Question)
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

	storedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", nextQuestion.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(next) failed: %v", err)
	}
	if storedQuestion == nil || storedQuestion.Status != string(QuestionStatusUsed) || storedQuestion.UsedAt == nil {
		t.Fatalf("expected manual publish to keep the published question used in storage, got %+v", storedQuestion)
	}

	summary, err := service.GetSummary(context.Background(), "g1")
	if err != nil {
		t.Fatalf("GetSummary() failed: %v", err)
	}
	if !summary.PublishedForCurrentSlot {
		t.Fatalf("expected manual publish to occupy the current slot, got %+v", summary)
	}

	automaticQueue, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	if automaticQueue.SlotStatus != AutomaticQueueSlotStatusPublished {
		t.Fatalf("expected manual publish to mark the current slot as published, got %+v", automaticQueue)
	}
	if automaticQueue.SlotOfficialPost == nil || automaticQueue.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected automatic queue state to expose the occupying manual post, got %+v", automaticQueue)
	}
	if automaticQueue.SlotQuestion == nil || automaticQueue.SlotQuestion.ID != nextQuestion.ID {
		t.Fatalf("expected automatic queue state to expose the current-slot question, got %+v", automaticQueue)
	}
	if automaticQueue.NextReadyQuestion != nil {
		t.Fatalf("expected no next ready question after the only ready question was published manually, got %+v", automaticQueue)
	}

	previousOfficial, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(previous) failed: %v", err)
	}
	if previousOfficial == nil || previousOfficial.State != string(OfficialPostStatePrevious) {
		t.Fatalf("expected the previous day's official post to remain available as the prior slot after the boundary, got %+v", previousOfficial)
	}
}

func TestServicePublishNowRejectsAdditionalManualPublishesForCurrentSlot(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created := make([]*storage.QOTDQuestionRecord, 0, 2)
	for idx := 1; idx <= 2; idx++ {
		question, err := service.CreateQuestion(context.Background(), "g1", fmt.Sprintf("user-%d", idx), QuestionMutation{
			Body:   fmt.Sprintf("Question %d", idx),
			Status: QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	firstResult, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishNow(first) failed: %v", err)
	}
	if firstResult.Question.ID != created[0].ID {
		t.Fatalf("expected first manual publish to use the first ready question, got %+v", firstResult.Question)
	}
	if firstResult.Question.Status != string(QuestionStatusUsed) || firstResult.Question.UsedAt == nil {
		t.Fatalf("expected first manual publish to consume the first question, got %+v", firstResult.Question)
	}

	_, err = service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if !errors.Is(err, ErrAlreadyPublished) {
		t.Fatalf("expected second manual publish to see the occupied slot, got %v", err)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected a single publish attempt for the current slot, got %d", len(fake.publishedParams))
	}

	firstStored, err := store.GetQOTDQuestion(context.Background(), "g1", created[0].ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(first) failed: %v", err)
	}
	if firstStored == nil || firstStored.Status != string(QuestionStatusUsed) || firstStored.UsedAt == nil {
		t.Fatalf("expected the first manual publish to consume the current-slot question, got %+v", firstStored)
	}
	secondStored, err := store.GetQOTDQuestion(context.Background(), "g1", created[1].ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(second) failed: %v", err)
	}
	if secondStored == nil || secondStored.Status != string(QuestionStatusReady) || secondStored.UsedAt != nil {
		t.Fatalf("expected the fallback question to remain ready when the slot is occupied, got %+v", secondStored)
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.QuestionID != created[0].ID || official.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected current slot to stay on the first manual publish, got %+v", official)
	}
}

func TestServicePublishScheduledIfDueSkipsWhenManualPostOccupiesCurrentSlot(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question 1",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	second, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Question 2",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}

	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	automaticQueue, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	if automaticQueue.SlotStatus != AutomaticQueueSlotStatusPublished {
		t.Fatalf("expected manual publish to occupy the current automatic slot, got %+v", automaticQueue)
	}
	if automaticQueue.SlotOfficialPost == nil || automaticQueue.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected automatic queue to expose the occupying manual post, got %+v", automaticQueue)
	}
	if automaticQueue.SlotQuestion == nil || automaticQueue.SlotQuestion.ID != first.ID {
		t.Fatalf("expected automatic queue to expose the current-slot question, got %+v", automaticQueue)
	}
	if automaticQueue.NextReadyQuestion == nil || automaticQueue.NextReadyQuestion.ID != second.ID {
		t.Fatalf("expected the next ready question to move on after the manual publish, got %+v", automaticQueue)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if published {
		t.Fatal("expected scheduled publish to stay idle while the current slot is already occupied")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected only the manual publish attempt, got %d", len(fake.publishedParams))
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.PublishMode != string(PublishModeManual) || official.PublishedAt == nil || official.QuestionID != first.ID {
		t.Fatalf("expected the manual publish to remain the current-slot record, got %+v", official)
	}

	storedSecond, err := store.GetQOTDQuestion(context.Background(), "g1", second.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(second) failed: %v", err)
	}
	if storedSecond == nil || storedSecond.Status != string(QuestionStatusReady) || storedSecond.UsedAt != nil {
		t.Fatalf("expected the remaining question to stay ready while the slot is occupied, got %+v", storedSecond)
	}
	storedFirst, err := store.GetQOTDQuestion(context.Background(), "g1", first.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(first) failed: %v", err)
	}
	if storedFirst == nil || storedFirst.Status != string(QuestionStatusUsed) || storedFirst.UsedAt == nil {
		t.Fatalf("expected manual publish to keep the first question consumed, got %+v", storedFirst)
	}

	summary, err := service.GetSummary(context.Background(), "g1")
	if err != nil {
		t.Fatalf("GetSummary() failed: %v", err)
	}
	if !summary.PublishedForCurrentSlot {
		t.Fatalf("expected the occupying manual publish to keep the slot marked published, got %+v", summary)
	}
}

func TestServicePublishNowCanSkipAutomaticSlotConsumption(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 11, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question 1",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	second, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Question 2",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}

	consumeAutomaticSlot := false
	result, err := service.PublishNowWithParams(context.Background(), "g1", &discordgo.Session{}, PublishNowParams{
		ConsumeAutomaticSlot: &consumeAutomaticSlot,
	})
	if err != nil {
		t.Fatalf("PublishNowWithParams() failed: %v", err)
	}
	if result.Question.ID != first.ID {
		t.Fatalf("expected non-consuming manual publish to use the first question, got %+v", result.Question)
	}
	if result.OfficialPost.ConsumeAutomaticSlot {
		t.Fatalf("expected non-consuming manual publish record, got %+v", result.OfficialPost)
	}

	automaticQueue, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	if automaticQueue.SlotStatus != AutomaticQueueSlotStatusDue {
		t.Fatalf("expected automatic slot to remain due, got %+v", automaticQueue)
	}
	if automaticQueue.SlotOfficialPost != nil {
		t.Fatalf("expected no occupying automatic-slot post after non-consuming manual publish, got %+v", automaticQueue)
	}
	if automaticQueue.NextReadyQuestion == nil || automaticQueue.NextReadyQuestion.ID != second.ID {
		t.Fatalf("expected next ready question to advance to the second item, got %+v", automaticQueue)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected scheduled publish to still run for the current slot")
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected one manual and one scheduled publish attempt, got %d", len(fake.publishedParams))
	}

	scheduledOfficial, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate() failed: %v", err)
	}
	if scheduledOfficial == nil || scheduledOfficial.QuestionID != second.ID {
		t.Fatalf("expected scheduled slot to still publish the second question, got %+v", scheduledOfficial)
	}
}

func TestServiceGetAutomaticQueueStateReflectsManualPublishForCurrentSlot(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question 1",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	second, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Question 2",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}

	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	state, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	if state.SlotStatus != AutomaticQueueSlotStatusPublished {
		t.Fatalf("expected automatic queue to show the current slot as published after a manual publish, got %+v", state)
	}
	if state.SlotOfficialPost == nil || state.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected automatic queue to expose the occupying manual post, got %+v", state)
	}
	if state.SlotQuestion == nil || state.SlotQuestion.ID != first.ID {
		t.Fatalf("expected automatic queue to expose the current-slot question after manual publish, got %+v", state)
	}
	if state.NextReadyQuestion == nil || state.NextReadyQuestion.ID != second.ID {
		t.Fatalf("expected the next automatic question to skip the manual publish, got %+v", state)
	}
	if state.NextReadyQuestion.ID == first.ID {
		t.Fatalf("expected the manually published question to be removed from the automatic queue, got %+v", state)
	}
	if !state.SlotPublishAtUTC.Equal(time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)) {
		t.Fatalf("unexpected automatic slot publish time: %+v", state)
	}
	if !state.SlotDateUTC.Equal(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected automatic slot date: %+v", state)
	}
	if state.Deck.ID != files.LegacyQOTDDefaultDeckID {
		t.Fatalf("expected automatic queue to use the active deck, got %+v", state)
	}
	if !state.CanPublish {
		t.Fatalf("expected automatic queue state to remain publishable, got %+v", state)
	}
	if !state.ScheduleConfigured {
		t.Fatalf("expected automatic queue state to expose the configured schedule, got %+v", state)
	}
}

func TestServicePublishNowUsesCurrentScheduledSlotBeforeBoundary(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 42, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question 1",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}

	result, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}
	if result.Question.ID != first.ID {
		t.Fatalf("expected manual publish to use the first ready question, got %+v", result)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one manual publish attempt, got %d", len(fake.publishedParams))
	}
	if got := fake.publishedParams[0].PublishDateUTC; !got.Equal(time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected manual publish to occupy the active pre-boundary slot, got %s", got.Format(time.RFC3339))
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected manual publish to persist on the active slot date, got %+v", official)
	}

	state, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	if state.SlotStatus != AutomaticQueueSlotStatusPublished || state.SlotOfficialPost == nil {
		t.Fatalf("expected manual pre-boundary publish to occupy the active automatic slot, got %+v", state)
	}
	if state.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected the active slot to be occupied by the manual post, got %+v", state)
	}
}

func TestServicePublishNowRejectsSecondCurrentSlotPublish(t *testing.T) {
	service, _, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	for idx := 0; idx < 2; idx++ {
		if _, err := service.CreateQuestion(context.Background(), "g1", fmt.Sprintf("user-%d", idx+1), QuestionMutation{
			Body:   fmt.Sprintf("Question %d", idx+1),
			Status: QuestionStatusReady,
		}); err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
	}

	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishNow(first) failed: %v", err)
	}
	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); !errors.Is(err, ErrAlreadyPublished) {
		t.Fatalf("expected second publish to fail with ErrAlreadyPublished, got %v", err)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one real publish attempt across both commands, got %d", len(fake.publishedParams))
	}
}

func TestServiceResolvePublishNowConflictTranslatesUniqueSlotConflicts(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	conflictErr := &pgconn.PgError{Code: postgresUniqueViolationCode, ConstraintName: qotdLegacyPublishDateConstraint}

	tests := []struct {
		name            string
		setup           func(t *testing.T) error
		wantErr         error
		wantQuestionID  int64
		wantOfficialID  int64
		wantPostURLPart string
	}{
		{
			name: "published post becomes already published",
			setup: func(t *testing.T) error {
				t.Helper()
				question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
					Body:   "Question 1",
					Status: QuestionStatusReady,
				})
				if err != nil {
					return fmt.Errorf("CreateQuestion(): %w", err)
				}

				official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
					GuildID:              "g1",
					DeckID:               files.LegacyQOTDDefaultDeckID,
					DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
					QuestionID:           question.ID,
					PublishMode:          string(PublishModeScheduled),
					PublishDateUTC:       publishDate,
					State:                string(OfficialPostStateCurrent),
					ChannelID:            "123456789012345678",
					QuestionTextSnapshot: question.Body,
					GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
					ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
				})
				if err != nil {
					return fmt.Errorf("CreateQOTDOfficialPostProvisioning(): %w", err)
				}
				if _, err := store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "questions-list-thread", "questions-list-entry", "thread-1", "message-1", "thread-1", time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)); err != nil {
					return fmt.Errorf("FinalizeQOTDOfficialPost(): %w", err)
				}
				return nil
			},
			wantQuestionID:  1,
			wantOfficialID:  1,
			wantPostURLPart: "https://discord.com/channels/g1/123456789012345678/message-1",
		},
		{
			name: "provisioning post becomes publish in progress",
			setup: func(t *testing.T) error {
				t.Helper()
				question, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
					Body:   "Question 2",
					Status: QuestionStatusReady,
				})
				if err != nil {
					return fmt.Errorf("CreateQuestion(): %w", err)
				}

				_, err = store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
					GuildID:              "g1",
					DeckID:               files.LegacyQOTDDefaultDeckID,
					DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
					QuestionID:           question.ID,
					PublishMode:          string(PublishModeScheduled),
					PublishDateUTC:       publishDate,
					State:                string(OfficialPostStateProvisioning),
					ChannelID:            "123456789012345678",
					QuestionTextSnapshot: question.Body,
					GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
					ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
				})
				if err != nil {
					return fmt.Errorf("CreateQOTDOfficialPostProvisioning(): %w", err)
				}
				return nil
			},
			wantErr: ErrPublishInProgress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, store, _ = newIntegrationTestQOTDService(t)
			if err := tt.setup(t); err != nil {
				t.Fatal(err)
			}

			result, err := service.resolvePublishNowConflict(context.Background(), "g1", publishDate, conflictErr)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("resolvePublishNowConflict() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if result != nil {
					t.Fatalf("resolvePublishNowConflict() result = %+v, want nil on error", result)
				}
				return
			}
			if result == nil {
				t.Fatal("resolvePublishNowConflict() result = nil, want published result")
			}
			if result.Question.ID != tt.wantQuestionID {
				t.Fatalf("resolvePublishNowConflict() question ID = %d, want %d", result.Question.ID, tt.wantQuestionID)
			}
			if result.OfficialPost.ID != tt.wantOfficialID {
				t.Fatalf("resolvePublishNowConflict() official post ID = %d, want %d", result.OfficialPost.ID, tt.wantOfficialID)
			}
			if result.PostURL != tt.wantPostURLPart {
				t.Fatalf("resolvePublishNowConflict() post URL = %q, want %q", result.PostURL, tt.wantPostURLPart)
			}
		})
	}
}

func TestServicePublishAcrossInstancesReportsInProgressDuringScheduledProvisioning(t *testing.T) {
	baseService, store, _ := newIntegrationTestQOTDService(t)
	blocked := &blockingPublisher{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	manualFake := &fakePublisher{}
	scheduledService := NewService(baseService.configManager, store, blocked)
	manualService := NewService(baseService.configManager, store, manualFake)
	beforeBoundary := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	afterBoundary := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	scheduledService.now = func() time.Time { return beforeBoundary }
	manualService.now = func() time.Time { return beforeBoundary }

	if _, err := scheduledService.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	first, err := scheduledService.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Scheduled winner",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	second, err := scheduledService.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Manual fallback",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}

	scheduledService.now = func() time.Time { return afterBoundary }
	manualService.now = func() time.Time { return afterBoundary }

	type scheduledResult struct {
		published bool
		err       error
	}
	results := make(chan scheduledResult, 1)
	go func() {
		published, err := scheduledService.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
		results <- scheduledResult{published: published, err: err}
	}()

	select {
	case <-blocked.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for scheduled publish to enter provisioning")
	}

	_, err = manualService.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if !errors.Is(err, ErrPublishInProgress) {
		close(blocked.release)
		t.Fatalf("expected manual publish to observe in-progress scheduled provisioning, got %v", err)
	}
	if len(manualFake.publishedParams) != 0 {
		close(blocked.release)
		t.Fatalf("expected manual publish not to reach discord publisher while the slot is occupied, got %+v", manualFake.publishedParams)
	}

	close(blocked.release)
	result := <-results
	if result.err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", result.err)
	}
	if !result.published {
		t.Fatal("expected scheduled publish to complete after the contention window")
	}
	if len(blocked.publishedParams) != 1 {
		t.Fatalf("expected scheduled publisher to be invoked once, got %d", len(blocked.publishedParams))
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.PublishMode != string(PublishModeScheduled) || official.PublishedAt == nil {
		t.Fatalf("expected the scheduled publish to own the current slot, got %+v", official)
	}

	storedFirst, err := store.GetQOTDQuestion(context.Background(), "g1", first.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(first) failed: %v", err)
	}
	if storedFirst == nil || storedFirst.Status != string(QuestionStatusUsed) || storedFirst.UsedAt == nil {
		t.Fatalf("expected scheduled winner question to be consumed, got %+v", storedFirst)
	}
	storedSecond, err := store.GetQOTDQuestion(context.Background(), "g1", second.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(second) failed: %v", err)
	}
	if storedSecond == nil || storedSecond.Status != string(QuestionStatusReady) || storedSecond.UsedAt != nil {
		t.Fatalf("expected manual fallback question to remain untouched, got %+v", storedSecond)
	}
}

func TestServiceResetDeckStateSuppressesAutomaticRepublishForCurrentSlot(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	for idx := 0; idx < 2; idx++ {
		if _, err := service.CreateQuestion(context.Background(), "g1", fmt.Sprintf("user-%d", idx+1), QuestionMutation{
			Body:   fmt.Sprintf("Question %d", idx+1),
			Status: QuestionStatusReady,
		}); err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
	}

	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	usedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", 1)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(used) failed: %v", err)
	}
	if usedQuestion == nil || usedQuestion.PublishedOnceAt == nil || usedQuestion.PublishedOnceAt.IsZero() {
		t.Fatalf("expected published question to carry the published-once marker before reset cleanup, got %+v", usedQuestion)
	}

	resetResult, err := service.ResetDeckState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ResetDeckState() failed: %v", err)
	}
	if resetResult.OfficialPostsCleared != 1 {
		t.Fatalf("expected reset to clear the published slot record, got %+v", resetResult)
	}
	if !resetResult.SuppressedCurrentSlotAutomaticPublish {
		t.Fatalf("expected reset to suppress automatic republish for the current slot, got %+v", resetResult)
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official != nil {
		t.Fatalf("expected reset to clear the published current-slot record, got %+v", official)
	}

	secondQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", 2)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(second) failed: %v", err)
	}
	if secondQuestion == nil {
		t.Fatal("expected second question to exist")
	}
	if _, err := service.SetNextQuestion(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, secondQuestion.ID); err != nil {
		t.Fatalf("SetNextQuestion() failed: %v", err)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if published {
		t.Fatal("expected reset to pause automatic publishing for the current slot during maintenance")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected scheduler to stay idle after reset suppression, got %d publish attempts", len(fake.publishedParams))
	}

	resetQuestions, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions(after reset) failed: %v", err)
	}
	if len(resetQuestions) < 2 || resetQuestions[0].PublishedOnceAt != nil || resetQuestions[1].PublishedOnceAt != nil {
		t.Fatalf("expected reset to clear published-once markers from deck questions, got %+v", resetQuestions)
	}
	settings, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() failed: %v", err)
	}
	if settings.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected reset to persist current-slot suppression, got %+v", settings)
	}
	manualResult, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishNow(manual after reset) failed: %v", err)
	}
	if manualResult.Question.ID != secondQuestion.ID {
		t.Fatalf("expected manual publish after reset to use the reordered next question, got %+v", manualResult)
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected explicit manual publish after reset to run once, got %d publish attempts", len(fake.publishedParams))
	}
	settings, err = service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings(after manual publish) failed: %v", err)
	}
	if settings.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected manual publish to clear current-slot suppression, got %+v", settings)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)
	}
	result, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishNow(next day after reset) failed: %v", err)
	}
	if result.Question.ID != usedQuestion.ID {
		t.Fatalf("expected reset to make the previously published question eligible again on a future slot, got %+v", result)
	}
	if len(fake.publishedParams) != 3 {
		t.Fatalf("expected future-slot republish after reset, got %d publish attempts", len(fake.publishedParams))
	}
}

func TestServiceGetAutomaticQueueStateUsesActiveScheduledSlotBeforeBoundary(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 42, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question 1",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	_, err = service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Question 2",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}

	state, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	wantSlotDate := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	if !state.SlotDateUTC.Equal(wantSlotDate) {
		t.Fatalf("expected pre-boundary automatic queue to report the active previous-day slot, got %+v", state)
	}
	wantPublishAt := time.Date(2026, 4, 2, 12, 43, 0, 0, time.UTC)
	if !state.SlotPublishAtUTC.Equal(wantPublishAt) {
		t.Fatalf("expected pre-boundary automatic queue to report the active previous-day publish time, got %+v", state)
	}
	if state.SlotStatus != AutomaticQueueSlotStatusDue {
		t.Fatalf("expected the previous-day slot to remain due before today's boundary, got %+v", state)
	}
	if state.NextReadyQuestion == nil || state.NextReadyQuestion.ID != first.ID {
		t.Fatalf("expected the first ready question to remain next for the active slot, got %+v", state)
	}
	if !state.CanPublish || !state.ScheduleConfigured {
		t.Fatalf("expected the automatic queue to remain publishable before the boundary, got %+v", state)
	}
	if state.SlotOfficialPost != nil || state.SlotQuestion != nil {
		t.Fatalf("expected no scheduled slot records in the pre-boundary setup, got %+v", state)
	}
	if summary, err := service.GetSummary(context.Background(), "g1"); err != nil {
		t.Fatalf("GetSummary() failed: %v", err)
	} else if !summary.CurrentPublishDateUTC.Equal(wantSlotDate) {
		t.Fatalf("expected queue state to match summary current publish date before the boundary, got summary=%+v state=%+v", summary, state)
	}
}

func TestServiceResetDeckStatePreservesQuestionOrder(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	first, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{Body: "Question 1", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion(first) failed: %v", err)
	}
	second, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{Body: "Question 2", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion(second) failed: %v", err)
	}
	third, err := service.CreateQuestion(context.Background(), "g1", "user-3", QuestionMutation{Body: "Question 3", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion(third) failed: %v", err)
	}

	if err := store.ReorderQOTDQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, []int64{third.ID, first.ID, second.ID}); err != nil {
		t.Fatalf("ReorderQOTDQuestions() failed: %v", err)
	}

	questions, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions(before reset) failed: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("expected three questions before reset, got %+v", questions)
	}

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	usedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	questions[0].Status = string(QuestionStatusUsed)
	questions[0].UsedAt = &usedAt
	if _, err := store.UpdateQOTDQuestion(context.Background(), questions[0]); err != nil {
		t.Fatalf("UpdateQOTDQuestion(used) failed: %v", err)
	}
	questions[1].Status = string(QuestionStatusReserved)
	questions[1].ScheduledForDateUTC = &publishDate
	if _, err := store.UpdateQOTDQuestion(context.Background(), questions[1]); err != nil {
		t.Fatalf("UpdateQOTDQuestion(reserved) failed: %v", err)
	}

	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           third.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateCurrent),
		ChannelID:            "123456789012345678",
		QuestionTextSnapshot: third.Body,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	if _, err := store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "questions-list-thread", "questions-list-entry", "thread-1", "message-1", "thread-1", usedAt); err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpsertQOTDSurface(context.Background(), storage.QOTDSurfaceRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		ChannelID:            "123456789012345678",
		QuestionListThreadID: "questions-list-thread",
	}); err != nil {
		t.Fatalf("UpsertQOTDSurface() failed: %v", err)
	}

	resetResult, err := service.ResetDeckState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ResetDeckState() failed: %v", err)
	}
	if resetResult.QuestionsReset != 2 || resetResult.OfficialPostsCleared != 1 {
		t.Fatalf("unexpected reset result: %+v", resetResult)
	}

	updated, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions(after reset) failed: %v", err)
	}
	if len(updated) != 3 {
		t.Fatalf("expected three questions after reset, got %+v", updated)
	}
	if updated[0].ID != third.ID || updated[1].ID != first.ID || updated[2].ID != second.ID {
		t.Fatalf("expected reset to preserve the reordered question order, got %+v", updated)
	}
	for _, question := range updated {
		if question.Status != string(QuestionStatusReady) || question.UsedAt != nil || question.ScheduledForDateUTC != nil {
			t.Fatalf("expected reset to restore ready state without touching order, got %+v", updated)
		}
	}

	storedOfficial, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if storedOfficial != nil {
		t.Fatalf("expected reset to clear published official posts for the deck, got %+v", storedOfficial)
	}
	surface, err := store.GetQOTDSurfaceByDeck(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetQOTDSurfaceByDeck() failed: %v", err)
	}
	if surface != nil {
		t.Fatalf("expected reset to clear the deck surface, got %+v", surface)
	}
}

func TestServiceCollectArchivedQuestionsStoresMatchedEmbeds(t *testing.T) {
	service, _, fake := newIntegrationTestQOTDService(t)
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

func TestServiceRemoveDeckDuplicatesFromCollectorUsesStoredHistory(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	mutableDuplicate, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "  WHAT is one habit you want to keep this month?  ",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(mutable duplicate) failed: %v", err)
	}
	immutableDuplicate, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "What are you excited to try next?",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(immutable duplicate) failed: %v", err)
	}
	immutableDuplicate.Status = string(QuestionStatusUsed)
	if _, err := store.UpdateQOTDQuestion(context.Background(), *immutableDuplicate); err != nil {
		t.Fatalf("UpdateQOTDQuestion(immutable duplicate) failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-3", QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "What changed in the release process this week?",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion(unique) failed: %v", err)
	}

	created, err := store.CreateQOTDCollectedQuestions(context.Background(), []storage.QOTDCollectedQuestionRecord{
		{
			GuildID:                  "g1",
			SourceChannelID:          integrationCollectorChannelID,
			SourceMessageID:          "message-1",
			SourceAuthorID:           "bot-1",
			SourceAuthorNameSnapshot: "QOTD Bot",
			SourceCreatedAt:          time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC),
			EmbedTitle:               "Question Of The Day",
			QuestionText:             "What is one habit you want to keep this month?",
		},
		{
			GuildID:                  "g1",
			SourceChannelID:          integrationCollectorChannelID,
			SourceMessageID:          "message-2",
			SourceAuthorID:           "bot-1",
			SourceAuthorNameSnapshot: "QOTD Bot",
			SourceCreatedAt:          time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC),
			EmbedTitle:               "question!!",
			QuestionText:             "What are you excited to try next?",
		},
	})
	if err != nil {
		t.Fatalf("CreateQOTDCollectedQuestions() failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected two stored collected questions, got %d", created)
	}

	fake.channelMessages = map[string][]discordqotd.ArchivedMessage{
		integrationCollectorChannelID: {
			{
				MessageID:          "live-message-1",
				AuthorID:           "bot-1",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"Live discord history should not be used here."}]`),
				CreatedAt:          time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC),
			},
		},
	}

	result, err := service.RemoveDeckDuplicatesFromCollector(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("RemoveDeckDuplicatesFromCollector() failed: %v", err)
	}
	if result.DeckID != files.LegacyQOTDDefaultDeckID {
		t.Fatalf("expected default deck id, got %+v", result)
	}
	if result.ScannedMessages != 2 || result.MatchedMessages != 2 {
		t.Fatalf("unexpected scan result: %+v", result)
	}
	if result.DuplicateQuestions != 2 || result.DeletedQuestions != 1 {
		t.Fatalf("unexpected duplicate removal result: %+v", result)
	}

	deleted, err := store.GetQOTDQuestion(context.Background(), "g1", mutableDuplicate.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(deleted) failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected mutable duplicate to be deleted, got %+v", deleted)
	}

	remaining, err := store.ListQOTDQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQOTDQuestions() failed: %v", err)
	}
	if len(remaining) != 2 {
		t.Fatalf("expected two questions to remain after duplicate removal, got %+v", remaining)
	}
	if remaining[0].ID != immutableDuplicate.ID || remaining[0].Status != string(QuestionStatusUsed) {
		t.Fatalf("expected immutable duplicate to remain, got %+v", remaining)
	}
}

func TestServiceImportArchivedQuestionsPrependsUsedHistoryAndWritesBackup(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	duplicate, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "What habit helps you reset after a long day?",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(duplicate) failed: %v", err)
	}
	remainingReady, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "What would you learn if you had an extra free hour?",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(remaining ready) failed: %v", err)
	}

	fake.channelMessages = map[string][]discordqotd.ArchivedMessage{
		integrationCollectorChannelID: {
			{
				MessageID:          "message-3",
				AuthorID:           "999999999999999999",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"What place would you revisit without changing a thing?"}]`),
				CreatedAt:          time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC),
			},
			{
				MessageID:          "message-2",
				AuthorID:           "999999999999999999",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"What habit helps you reset after a long day?\nAsked by another bot"}]`),
				CreatedAt:          time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC),
			},
			{
				MessageID:          "message-1",
				AuthorID:           "555555555555555555",
				AuthorNameSnapshot: "Ignored Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"Ignored question"}]`),
				CreatedAt:          time.Date(2026, 4, 11, 15, 0, 0, 0, time.UTC),
			},
		},
	}

	backupDir := t.TempDir()
	result, err := service.ImportArchivedQuestions(context.Background(), "g1", "importer-1", &discordgo.Session{}, ImportArchivedQuestionsParams{
		DeckID:          files.LegacyQOTDDefaultDeckID,
		SourceChannelID: integrationCollectorChannelID,
		AuthorIDs:       []string{"999999999999999999"},
		StartDate:       "2026-04-01",
		BackupDir:       backupDir,
	})
	if err != nil {
		t.Fatalf("ImportArchivedQuestions() failed: %v", err)
	}
	if result.DeckID != files.LegacyQOTDDefaultDeckID {
		t.Fatalf("expected default deck id, got %+v", result)
	}
	if result.ScannedMessages != 3 || result.MatchedMessages != 2 {
		t.Fatalf("unexpected scan result: %+v", result)
	}
	if result.StoredQuestions != 2 || result.ImportedQuestions != 2 {
		t.Fatalf("unexpected import counts: %+v", result)
	}
	if result.DuplicateQuestions != 1 || result.DeletedQuestions != 1 {
		t.Fatalf("unexpected duplicate removal counts: %+v", result)
	}
	if result.BackupPath == "" {
		t.Fatalf("expected backup path, got %+v", result)
	}

	backupBytes, err := os.ReadFile(result.BackupPath)
	if err != nil {
		t.Fatalf("ReadFile(backup) failed: %v", err)
	}
	backupText := string(backupBytes)
	expectedBackup := "What habit helps you reset after a long day?\nWhat place would you revisit without changing a thing?\n"
	if backupText != expectedBackup {
		t.Fatalf("unexpected backup text:\n%s", backupText)
	}

	deleted, err := store.GetQOTDQuestion(context.Background(), "g1", duplicate.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(deleted duplicate) failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected duplicate ready question to be deleted, got %+v", deleted)
	}

	questions, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(questions) != 3 {
		t.Fatalf("expected three questions after import, got %+v", questions)
	}
	if questions[0].DisplayID != 1 || questions[0].Body != "What habit helps you reset after a long day?" {
		t.Fatalf("expected oldest imported question first, got %+v", questions)
	}
	if questions[0].Status != string(QuestionStatusUsed) || questions[0].UsedAt == nil || questions[0].PublishedOnceAt == nil {
		t.Fatalf("expected oldest imported question to be marked used, got %+v", questions[0])
	}
	if questions[1].DisplayID != 2 || questions[1].Body != "What place would you revisit without changing a thing?" {
		t.Fatalf("expected second imported question next, got %+v", questions)
	}
	if questions[1].Status != string(QuestionStatusUsed) || questions[1].UsedAt == nil || questions[1].PublishedOnceAt == nil {
		t.Fatalf("expected second imported question to be marked used, got %+v", questions[1])
	}
	if questions[2].ID != remainingReady.ID || questions[2].DisplayID != 3 || questions[2].Status != string(QuestionStatusReady) {
		t.Fatalf("expected original ready question to shift after imported history, got %+v", questions)
	}

	totalCollected, err := store.CountQOTDCollectedQuestions(context.Background(), "g1")
	if err != nil {
		t.Fatalf("CountQOTDCollectedQuestions() failed: %v", err)
	}
	if totalCollected != 2 {
		t.Fatalf("expected two stored collected questions, got %d", totalCollected)
	}
}

func TestServicePublishScheduledIfDueCreatesScheduledPost(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	beforeBoundary := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	afterBoundary := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return beforeBoundary }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
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

	service.now = func() time.Time { return afterBoundary }

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
	if fake.publishedParams[0].ThreadName != "Question of the Day" {
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

func TestServiceEnableAfterCurrentSlotDueSuppressesImmediatePublish(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	afterBoundary := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return afterBoundary }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(false, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings(disabled) failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question held while QOTD is disabled",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	updated, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID))
	if err != nil {
		t.Fatalf("UpdateSettings(enabled) failed: %v", err)
	}
	if updated.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected enabling after the boundary to suppress the current slot, got %+v", updated)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if published {
		t.Fatal("expected scheduler to stay idle for the current slot immediately after enabling QOTD")
	}
	if len(fake.publishedParams) != 0 {
		t.Fatalf("expected no publish attempts while the enabled slot is suppressed, got %+v", fake.publishedParams)
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official != nil {
		t.Fatalf("expected no official post for the suppressed current slot, got %+v", official)
	}

	storedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if storedQuestion == nil || storedQuestion.Status != string(QuestionStatusReady) || storedQuestion.UsedAt != nil {
		t.Fatalf("expected ready question to remain untouched while the current slot is suppressed, got %+v", storedQuestion)
	}

	settings, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() failed: %v", err)
	}
	if settings.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected persisted suppression for the current slot after enabling, got %+v", settings)
	}
}

func TestServicePublishScheduledIfDueResumesFailedProvisioning(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	beforeBoundary := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	afterBoundary := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return beforeBoundary }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, "123456789012345678")); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Recoverable scheduled question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	service.now = func() time.Time { return afterBoundary }

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
	service, store, fake := newIntegrationTestQOTDService(t)
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

	schedule, err := resolvePublishSchedule(scheduledQOTDConfig(true, "123456789012345678"))
	if err != nil {
		t.Fatalf("resolvePublishSchedule() failed: %v", err)
	}
	lifecycle := EvaluateOfficialPost(schedule, time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC), service.clock())
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
	service, store, fake := newIntegrationTestQOTDService(t)
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
	schedule, err := resolvePublishSchedule(scheduledQOTDConfig(true, integrationForumChannelID))
	if err != nil {
		t.Fatalf("resolvePublishSchedule() failed: %v", err)
	}
	lifecycle := EvaluateOfficialPost(schedule, publishDate, service.clock())
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStatePrevious),
		ChannelID:            integrationForumChannelID,
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
	if fake.threadStates["official-thread-archive"] != (discordqotd.ThreadState{Pinned: false, Locked: false, Archived: false}) {
		t.Fatalf("expected archived official thread to remain available, got %+v", fake.threadStates["official-thread-archive"])
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
