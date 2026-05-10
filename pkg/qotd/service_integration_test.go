//go:build integration

package qotd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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

func TestServiceMarkQuestionPublishedMarksReadyQuestionWithoutDayState(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Mark me as already published",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	updated, err := service.MarkQuestionPublished(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, created.ID)
	if err != nil {
		t.Fatalf("MarkQuestionPublished() failed: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated question")
	}
	if updated.Status != string(QuestionStatusUsed) {
		t.Fatalf("expected marked question status=used, got %+v", updated)
	}
	if updated.UsedAt == nil || updated.PublishedOnceAt == nil {
		t.Fatalf("expected mark-published to set used/published timestamps, got %+v", updated)
	}
	if updated.ScheduledForDateUTC != nil {
		t.Fatalf("expected mark-published to leave the question detached from any slot, got %+v", updated)
	}

	persisted, err := service.ListQuestions(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(persisted) != 1 || persisted[0].ID != created.ID || persisted[0].Status != string(QuestionStatusUsed) {
		t.Fatalf("expected the same question to remain in the deck as used, got %+v", persisted)
	}
	if persisted[0].PublishedOnceAt == nil || persisted[0].ScheduledForDateUTC != nil {
		t.Fatalf("expected persisted mark-published state without slot date, got %+v", persisted[0])
	}
}

func TestServiceMarkQuestionPublishedRejectsReservedQuestion(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	created, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Reserved for a day",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}
	slotDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	created.Status = string(QuestionStatusReserved)
	created.ScheduledForDateUTC = &slotDate
	if _, err := store.UpdateQOTDQuestion(context.Background(), *created); err != nil {
		t.Fatalf("UpdateQOTDQuestion() failed: %v", err)
	}

	_, err = service.MarkQuestionPublished(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, created.ID)
	if !errors.Is(err, ErrImmutableQuestion) {
		t.Fatalf("expected ErrImmutableQuestion, got %v", err)
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
	// Ordinal 2 because the yesterday post seeded above already consumed
	// ordinal 1 in the (guild_id, deck_id) sequence.
	if fake.publishedParams[0].ThreadName != "Question #002" {
		t.Fatalf("expected manual publish to continue the publish-ordinal sequence, got %+v", fake.publishedParams[0])
	}
	if result.OfficialPost.PublishOrdinal != 2 {
		t.Fatalf("expected manual publish to receive ordinal 2 after seeded yesterday post, got %d", result.OfficialPost.PublishOrdinal)
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
		t.Fatalf("expected automatic queue to show tomorrow's slot as occupied after the manual publish, got %+v", automaticQueue)
	}
	if !automaticQueue.SlotDateUTC.Equal(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected automatic queue to point at tomorrow's slot date, got %+v", automaticQueue)
	}
	if !automaticQueue.SlotPublishAtUTC.Equal(time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)) {
		t.Fatalf("expected automatic queue to point at tomorrow's publish time, got %+v", automaticQueue)
	}
	if automaticQueue.SlotOfficialPost == nil || automaticQueue.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected the upcoming slot to expose the occupying manual post, got %+v", automaticQueue)
	}
	if automaticQueue.SlotQuestion == nil || automaticQueue.SlotQuestion.ID != nextQuestion.ID {
		t.Fatalf("expected the upcoming slot to expose the occupying question, got %+v", automaticQueue)
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

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.QuestionID != created[0].ID || official.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected the upcoming slot to stay on the first manual publish, got %+v", official)
	}
}

func TestServicePublishNowLateFailureDoesNotSuppressSameDayAutomaticPublish(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	afterBoundary := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return afterBoundary }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); !errors.Is(err, ErrNoQuestionsAvailable) {
		t.Fatalf("expected late manual publish with empty queue to fail with ErrNoQuestionsAvailable, got %v", err)
	}

	settings, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() after failed manual publish failed: %v", err)
	}
	if settings.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected failed late manual publish not to leave same-day suppression behind, got %+v", settings)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Recovered same-day automatic question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected scheduler to keep today's slot publishable after failed late manual publish")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one automatic publish attempt after recovery, got %d", len(fake.publishedParams))
	}

	official, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.QuestionID != question.ID {
		t.Fatalf("expected same-day automatic slot to publish the newly added question, got %+v", official)
	}
}

// TestServicePublishNowMidPublishFailureDoesNotOrphanSuppressionAlongsideRecovery
// extends the late-failure coverage to the harder case: the manual publish
// reaches completeOfficialPostProvisioning and fails THERE (publisher
// returned an error). Two invariants must hold simultaneously:
//
//   1. The deferred suppression rollback runs even when the failure is
//      raised by the publisher (not the reservation), so the same-day
//      scheduled publish is not blocked by a stale suppression.
//   2. The provisioning row that was created before the failure is left in
//      a state that ReconcileGuild can resume into a published post —
//      without the recovery and the separate scheduled publish racing each
//      other on the same date.
func TestServicePublishNowMidPublishFailureDoesNotOrphanSuppressionAlongsideRecovery(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	afterBoundary := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return afterBoundary }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question for tomorrow's manual slot",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	// Inject a publisher response that exercises the partial-progress
	// failure surface: Discord IDs come back populated but err is set, so
	// completeOfficialPostProvisioning persists the IDs and then transitions
	// the post into the failed state. The second response will recover the
	// same row on the reconcile pass below.
	fake.publishResponses = []fakePublishResponse{
		{
			result: &discordqotd.PublishedOfficialPost{
				QuestionListThreadID: "questions-list-thread",
				ThreadID:             "thread-mid-failure",
				StarterMessageID:     "starter-mid-failure",
				AnswerChannelID:      "thread-mid-failure",
				PostURL:              discordqotd.BuildThreadJumpURL("g1", "thread-mid-failure"),
			},
			err: errFakePublishFailed,
		},
		{
			result: &discordqotd.PublishedOfficialPost{
				QuestionListThreadID:       "questions-list-thread",
				QuestionListEntryMessageID: "list-entry-recovered",
				ThreadID:                   "thread-mid-failure",
				StarterMessageID:           "starter-mid-failure",
				AnswerChannelID:            "thread-mid-failure",
				PublishedAt:                afterBoundary,
				PostURL:                    discordqotd.BuildThreadJumpURL("g1", "thread-mid-failure"),
			},
		},
	}

	if _, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{}); err == nil || !strings.Contains(err.Error(), errFakePublishFailed.Error()) {
		t.Fatalf("expected manual publish to surface the publisher error, got %v", err)
	}

	settings, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() after mid-publish failure failed: %v", err)
	}
	if settings.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected mid-publish failure to roll back the same-day suppression, got %+v", settings)
	}

	// publishDate of the manual post is TOMORROW (consumeAutomaticSlot path
	// rolls forward once today's boundary has passed). The provisioning
	// row must remain so reconcile can recover it.
	tomorrow := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	stuck, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", tomorrow)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(tomorrow) failed: %v", err)
	}
	if stuck == nil || stuck.State != string(OfficialPostStateFailed) {
		t.Fatalf("expected failed manual provisioning row to persist for tomorrow, got %+v", stuck)
	}
	if stuck.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected provisioning row to retain manual publish mode, got %+v", stuck)
	}

	storedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if storedQuestion == nil || storedQuestion.Status != string(QuestionStatusReserved) {
		t.Fatalf("expected reserved question to stay reserved while provisioning row awaits recovery, got %+v", storedQuestion)
	}

	// Resume path: ReconcileGuild must pick up the failed row and finish
	// publishing it. Today's scheduled slot is independent: it has no
	// official post yet, so the scheduler is free to schedule one against
	// today without colliding with the failed manual row scoped to tomorrow.
	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild() recovery failed: %v", err)
	}

	recovered, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", tomorrow)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(recovered) failed: %v", err)
	}
	if recovered == nil || recovered.ID != stuck.ID {
		t.Fatalf("expected the original failed row to be resumed in place, got %+v", recovered)
	}
	if recovered.PublishedAt == nil || recovered.PublishedAt.IsZero() {
		t.Fatalf("expected reconcile to mark the recovered row published, got %+v", recovered)
	}
	if recovered.DiscordThreadID != "thread-mid-failure" {
		t.Fatalf("expected reconcile to reuse the discord artifacts persisted on the failed attempt, got %+v", recovered)
	}

	settings, err = service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() after recovery failed: %v", err)
	}
	if settings.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected suppression to remain cleared after recovery, got %+v", settings)
	}

	postRecovery, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(after recovery) failed: %v", err)
	}
	if postRecovery == nil || postRecovery.Status != string(QuestionStatusUsed) || postRecovery.UsedAt == nil {
		t.Fatalf("expected recovery to mark the question used, got %+v", postRecovery)
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
		t.Fatalf("expected manual publish to occupy tomorrow's active automatic slot, got %+v", automaticQueue)
	}
	if !automaticQueue.SlotDateUTC.Equal(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected automatic queue to move to tomorrow's slot date, got %+v", automaticQueue)
	}
	if automaticQueue.SlotOfficialPost == nil || automaticQueue.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected automatic queue to expose the occupying manual post for tomorrow's slot, got %+v", automaticQueue)
	}
	if automaticQueue.SlotQuestion == nil || automaticQueue.SlotQuestion.ID != first.ID {
		t.Fatalf("expected automatic queue to expose tomorrow's occupied question, got %+v", automaticQueue)
	}
	if automaticQueue.NextReadyQuestion == nil || automaticQueue.NextReadyQuestion.ID != second.ID {
		t.Fatalf("expected the next ready question to stay queued for the upcoming slot, got %+v", automaticQueue)
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

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official == nil || official.PublishMode != string(PublishModeManual) || official.PublishedAt == nil || official.QuestionID != first.ID {
		t.Fatalf("expected the manual publish to remain tomorrow's active-slot record, got %+v", official)
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
	if !result.OfficialPost.PublishDateUTC.Equal(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected delayed non-consuming manual publish to stay on today's date, got %+v", result.OfficialPost)
	}

	automaticQueue, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState() failed: %v", err)
	}
	if automaticQueue.SlotStatus != AutomaticQueueSlotStatusWaiting {
		t.Fatalf("expected automatic queue to point at the next day's slot after the boundary, got %+v", automaticQueue)
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
	if published {
		t.Fatal("expected scheduler to stay idle after the boundary because the active slot already moved to tomorrow")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected only the delayed manual publish attempt on the same day, got %d", len(fake.publishedParams))
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(next day) failed: %v", err)
	}
	if !published {
		t.Fatal("expected the next day's scheduled publish to still run after a delayed non-consuming manual publish")
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected one delayed manual and one next-day scheduled publish attempt, got %d", len(fake.publishedParams))
	}

	scheduledOfficial, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if scheduledOfficial == nil || scheduledOfficial.QuestionID != second.ID {
		t.Fatalf("expected the next day's scheduled slot to still publish the second question, got %+v", scheduledOfficial)
	}
}

func TestServicePublishScheduledIfDueRunsOncePerDayAcrossManualPublishScenarios(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	}

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

	dayOne := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	dayTwo := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	dayThree := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	dayFour := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	dayFive := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	}
	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(day one) failed: %v", err)
	}
	if !published {
		t.Fatal("expected the first day's scheduled publish to run at the boundary")
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(day one repeat) failed: %v", err)
	}
	if published {
		t.Fatal("expected the scheduler to publish at most once for the same day-one slot")
	}
	dayOneScheduled, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", dayOne)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate(day one) failed: %v", err)
	}
	if dayOneScheduled == nil || dayOneScheduled.QuestionID != created[0].ID || dayOneScheduled.ChannelID != integrationQuestionChannelID {
		t.Fatalf("expected one scheduled day-one publish in the configured channel, got %+v", dayOneScheduled)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)
	}
	manualDayTwo, err := service.PublishNow(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishNow(day two) failed: %v", err)
	}
	if manualDayTwo.Question.ID != created[1].ID {
		t.Fatalf("expected day-two manual publish to consume the second ready question, got %+v", manualDayTwo)
	}
	if !manualDayTwo.OfficialPost.PublishDateUTC.Equal(dayThree) {
		t.Fatalf("expected day-two manual publish after the boundary to occupy day three, got %+v", manualDayTwo)
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(day two) failed: %v", err)
	}
	if published {
		t.Fatal("expected day-two scheduler run to stay idle after the active slot moved to day three")
	}
	dayTwoScheduled, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", dayTwo)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate(day two) failed: %v", err)
	}
	if dayTwoScheduled != nil {
		t.Fatalf("expected no scheduled day-two post after the boundary was missed, got %+v", dayTwoScheduled)
	}
	dayThreeOfficial, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", dayThree)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(day three) failed: %v", err)
	}
	if dayThreeOfficial == nil || dayThreeOfficial.PublishMode != string(PublishModeManual) || dayThreeOfficial.QuestionID != created[1].ID || dayThreeOfficial.ChannelID != integrationQuestionChannelID {
		t.Fatalf("expected the day-three slot to remain occupied by the day-two manual publish, got %+v", dayThreeOfficial)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(day three) failed: %v", err)
	}
	if published {
		t.Fatal("expected day-three scheduled publish to stay idle while the manual publish occupies that slot")
	}
	dayThreeScheduled, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", dayThree)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate(day three) failed: %v", err)
	}
	if dayThreeScheduled != nil {
		t.Fatalf("expected no scheduled day-three publish while the manual publish occupies that slot, got %+v", dayThreeScheduled)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 6, 13, 0, 0, 0, time.UTC)
	}
	consumeAutomaticSlot := false
	manualDayFour, err := service.PublishNowWithParams(context.Background(), "g1", &discordgo.Session{}, PublishNowParams{
		ConsumeAutomaticSlot: &consumeAutomaticSlot,
	})
	if err != nil {
		t.Fatalf("PublishNowWithParams(day four) failed: %v", err)
	}
	if manualDayFour.Question.ID != created[2].ID {
		t.Fatalf("expected day-four non-consuming manual publish to use the third ready question, got %+v", manualDayFour)
	}
	if !manualDayFour.OfficialPost.PublishDateUTC.Equal(dayFour) {
		t.Fatalf("expected day-four delayed manual publish to stay on day four, got %+v", manualDayFour)
	}
	dayFourQueue, err := service.GetAutomaticQueueState(context.Background(), "g1", files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("GetAutomaticQueueState(day four before scheduler) failed: %v", err)
	}
	if dayFourQueue.SlotStatus != AutomaticQueueSlotStatusWaiting || dayFourQueue.SlotOfficialPost != nil {
		t.Fatalf("expected non-consuming day-four manual publish to move the queue to day five, got %+v", dayFourQueue)
	}
	if dayFourQueue.NextReadyQuestion == nil || dayFourQueue.NextReadyQuestion.ID != created[3].ID {
		t.Fatalf("expected the next automatic question to remain available for day-five scheduling, got %+v", dayFourQueue)
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(day four) failed: %v", err)
	}
	if published {
		t.Fatal("expected day-four scheduler run to stay idle after the active slot moved to day five")
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 7, 12, 43, 0, 0, time.UTC)
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(day five) failed: %v", err)
	}
	if !published {
		t.Fatal("expected day-five scheduled publish to still run after a delayed non-consuming manual publish")
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(day five repeat) failed: %v", err)
	}
	if published {
		t.Fatal("expected the scheduler to publish at most once for the day-five slot")
	}
	dayFiveScheduled, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", dayFive)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate(day five) failed: %v", err)
	}
	if dayFiveScheduled == nil || dayFiveScheduled.QuestionID != created[3].ID || dayFiveScheduled.ChannelID != integrationQuestionChannelID {
		t.Fatalf("expected one scheduled day-five publish in the configured channel after the delayed non-consuming manual publish, got %+v", dayFiveScheduled)
	}

	scheduledCount := 0
	for _, record := range []*storage.QOTDOfficialPostRecord{dayOneScheduled, dayTwoScheduled, dayThreeScheduled, dayFiveScheduled} {
		if record != nil {
			scheduledCount++
		}
	}
	if scheduledCount != 2 {
		t.Fatalf("expected exactly two scheduled publishes across the scenario, got %d", scheduledCount)
	}
	if len(fake.publishedParams) != 4 {
		t.Fatalf("expected two manual and two scheduled publish attempts across the scenario, got %d", len(fake.publishedParams))
	}
	wantDates := []time.Time{dayOne, dayThree, dayFour, dayFive}
	for idx, wantDate := range wantDates {
		if !fake.publishedParams[idx].PublishDateUTC.Equal(wantDate) {
			t.Fatalf("publish attempt %d used slot %s, want %s: %+v", idx, fake.publishedParams[idx].PublishDateUTC.Format(time.RFC3339), wantDate.Format(time.RFC3339), fake.publishedParams[idx])
		}
		if fake.publishedParams[idx].ChannelID != integrationQuestionChannelID {
			t.Fatalf("publish attempt %d used channel %q, want %q: %+v", idx, fake.publishedParams[idx].ChannelID, integrationQuestionChannelID, fake.publishedParams[idx])
		}
	}
}

// TestServiceReconcileGuildReclaimsOrphanReservationsAcrossCrash simulates a
// process that crashed between ReserveNextQOTDQuestion and the official-post
// insert: the question stays "reserved" with scheduled_for_date_utc=yesterday
// even though no qotd_official_posts row references it. Without the reclaim
// sweep this question would never come back to the queue, draining the deck
// silently. ReconcileGuild must restore it to "ready" so the next publish
// picks it up.
func TestServiceReconcileGuildReclaimsOrphanReservationsAcrossCrash(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	orphan, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Stranded after a crash",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(orphan) failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-2", QuestionMutation{
		Body:   "Healthy fallback",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion(fallback) failed: %v", err)
	}

	yesterday := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	if _, err := store.ReserveNextQOTDQuestion(context.Background(), "g1", files.LegacyQOTDDefaultDeckID, yesterday, storage.QOTDQuestionSelectorQueue); err != nil {
		t.Fatalf("ReserveNextQOTDQuestion(simulated crash) failed: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	}
	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild() failed: %v", err)
	}

	restored, err := store.GetQOTDQuestion(context.Background(), "g1", orphan.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if restored == nil || restored.Status != string(QuestionStatusReady) {
		t.Fatalf("expected reconcile to release the orphan reservation, got %+v", restored)
	}
	if restored.ScheduledForDateUTC != nil {
		t.Fatalf("expected scheduled date to be cleared on the orphan, got %+v", restored.ScheduledForDateUTC)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected scheduler to publish today's slot after orphan reservation reclaimed")
	}
	if len(fake.publishedParams) != 1 || fake.publishedParams[0].QuestionText != orphan.Body {
		t.Fatalf("expected reclaimed orphan to be the next published question, got %+v", fake.publishedParams)
	}
}

// TestServicePublishScheduledIfDuePublishesWhenTickArrivesAfterBoundary covers
// the regression where a 1-minute wall-clock-misaligned ticker would silently
// skip the daily QOTD: CurrentPublishDateUTC rolls forward to tomorrow as soon
// as today's publish time elapses by even one nanosecond, leaving
// BoundaryPassed false on every realistic tick after the boundary. The fix
// anchors the runtime publish path to today's date, so any tick after today's
// schedule (and before tomorrow's) still publishes today's slot.
func TestServicePublishScheduledIfDuePublishesWhenTickArrivesAfterBoundary(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Late tick question",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	// Tick 30 seconds past the 12:43 UTC boundary. Pre-fix this returned
	// false because CurrentPublishDateUTC immediately rolled to tomorrow.
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 43, 30, 0, time.UTC)
	}
	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected scheduler to publish today's slot 30 seconds after the boundary")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one publish attempt, got %d", len(fake.publishedParams))
	}

	today := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	scheduled, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", today)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate() failed: %v", err)
	}
	if scheduled == nil || scheduled.PublishMode != string(PublishModeScheduled) {
		t.Fatalf("expected today's slot to hold the scheduled record, got %+v", scheduled)
	}
}

// TestServicePublishScheduledIfDueBackfillsAfterBootPostBoundary covers the
// "bot restarted after the daily schedule" scenario. Pre-fix the runtime
// would treat today's slot as already passed and wait until tomorrow's
// schedule. Post-fix it backfills today's missing publish on the next tick.
func TestServicePublishScheduledIfDueBackfillsAfterBootPostBoundary(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Boot-late question",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	// Boot at 14:00 UTC, well past the 12:43 boundary. Realistic restart
	// scenario after a crash or deploy.
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC)
	}
	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected scheduler to backfill today's slot when boot lands after the boundary")
	}

	today := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	scheduled, err := store.GetScheduledQOTDOfficialPostByDate(context.Background(), "g1", today)
	if err != nil {
		t.Fatalf("GetScheduledQOTDOfficialPostByDate() failed: %v", err)
	}
	if scheduled == nil {
		t.Fatal("expected today's slot to be filled after a post-boundary boot")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected exactly one publish attempt, got %d", len(fake.publishedParams))
	}
}

// TestServicePublishScheduledIfDueIdleBeforeBoundary confirms the inverse:
// when the scheduler ticks before today's publish time, it stays idle so we
// don't accidentally publish twice in one day on a clock-skewed system.
func TestServicePublishScheduledIfDueIdleBeforeBoundary(t *testing.T) {
	service, _, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Pre-boundary question",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 42, 30, 0, time.UTC)
	}
	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if published {
		t.Fatal("expected scheduler to stay idle 30 seconds before the boundary")
	}
	if len(fake.publishedParams) != 0 {
		t.Fatalf("expected no publish attempts before the boundary, got %d", len(fake.publishedParams))
	}
}

// TestServicePublishScheduledIfDueDoesNotRepublishLateTicks confirms that
// repeated late ticks across the same day are idempotent — the slot record
// blocks the second attempt even though both ticks satisfy BoundaryPassed.
func TestServicePublishScheduledIfDueDoesNotRepublishLateTicks(t *testing.T) {
	service, _, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Idempotent question",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 44, 0, 0, time.UTC)
	}
	if _, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishScheduledIfDue(first) failed: %v", err)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected first late tick to publish once, got %d", len(fake.publishedParams))
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}
	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(second) failed: %v", err)
	}
	if published {
		t.Fatal("expected second late tick to stay idle once today's slot is filled")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected no additional publish attempts on the second late tick, got %d", len(fake.publishedParams))
	}
}

func TestServiceGetAutomaticQueueStateSkipsPublishedCurrentSlotAfterBoundary(t *testing.T) {
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
		t.Fatalf("expected automatic queue to point at tomorrow's occupied slot after the boundary, got %+v", state)
	}
	if state.SlotOfficialPost == nil || state.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected the upcoming slot to expose the occupying manual post, got %+v", state)
	}
	if state.SlotQuestion == nil || state.SlotQuestion.ID != first.ID {
		t.Fatalf("expected the upcoming slot to expose the occupying question, got %+v", state)
	}
	if state.NextReadyQuestion == nil || state.NextReadyQuestion.ID != second.ID {
		t.Fatalf("expected the next automatic question to skip the manual publish, got %+v", state)
	}
	if state.NextReadyQuestion.ID == first.ID {
		t.Fatalf("expected the manually published question to be removed from the automatic queue, got %+v", state)
	}
	if !state.SlotPublishAtUTC.Equal(time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)) {
		t.Fatalf("unexpected automatic slot publish time: %+v", state)
	}
	if !state.SlotDateUTC.Equal(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)) {
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
	if got := fake.publishedParams[0].PublishDateUTC; !got.Equal(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected manual publish to occupy the active pre-boundary slot, got %s", got.Format(time.RFC3339))
	}

	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
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
	if state.SlotStatus != AutomaticQueueSlotStatusPublished {
		t.Fatalf("expected queue state to show today's active slot as occupied before the boundary, got %+v", state)
	}
	if !state.SlotDateUTC.Equal(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected queue state to point at today's slot date before the boundary, got %+v", state)
	}
	if !state.SlotPublishAtUTC.Equal(time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)) {
		t.Fatalf("expected queue state to point at today's publish time before the boundary, got %+v", state)
	}
	if state.SlotOfficialPost == nil || state.SlotOfficialPost.PublishMode != string(PublishModeManual) {
		t.Fatalf("expected today's active slot to be occupied by the manual post, got %+v", state)
	}
	if state.SlotQuestion == nil || state.SlotQuestion.ID != first.ID {
		t.Fatalf("expected queue state to expose today's occupied question, got %+v", state)
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
	boundary := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
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

	scheduledService.now = func() time.Time { return boundary }
	manualService.now = func() time.Time { return boundary }

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
	activeSlotDate := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	official, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", activeSlotDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate(before reset) failed: %v", err)
	}
	if official == nil {
		t.Fatalf("expected manual publish to occupy the active slot %s before reset, got nil", activeSlotDate.Format("2006-01-02"))
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

	official, err = store.GetQOTDOfficialPostByDate(context.Background(), "g1", activeSlotDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if official != nil {
		t.Fatalf("expected reset to clear the active-slot record for %s, got %+v", activeSlotDate.Format("2006-01-02"), official)
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
	if settings.SuppressScheduledPublishDateUTC != "2026-04-04" {
		t.Fatalf("expected reset to persist current-slot suppression, got %+v", settings)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue(next-day suppressed slot) failed: %v", err)
	}
	if published {
		t.Fatal("expected reset to suppress the next active automatic slot after removing its published record")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected scheduler to stay idle for the suppressed next-day slot, got %d publish attempts", len(fake.publishedParams))
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
		return time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC)
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

func TestServiceGetAutomaticQueueStateUsesUpcomingScheduledSlotBeforeBoundary(t *testing.T) {
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
	wantSlotDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !state.SlotDateUTC.Equal(wantSlotDate) {
		t.Fatalf("expected pre-boundary automatic queue to report today's upcoming slot, got %+v", state)
	}
	wantPublishAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	if !state.SlotPublishAtUTC.Equal(wantPublishAt) {
		t.Fatalf("expected pre-boundary automatic queue to report today's publish time, got %+v", state)
	}
	if state.SlotStatus != AutomaticQueueSlotStatusWaiting {
		t.Fatalf("expected the upcoming slot to remain waiting before today's boundary, got %+v", state)
	}
	if state.NextReadyQuestion == nil || state.NextReadyQuestion.ID != first.ID {
		t.Fatalf("expected the first ready question to remain next for the upcoming slot, got %+v", state)
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
		t.Fatalf("expected summary to follow the active upcoming slot before the boundary, got summary=%+v state=%+v", summary, state)
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
	boundary := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
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

	service.now = func() time.Time { return boundary }

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected scheduled publish to run at the boundary")
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
	if fake.publishedParams[0].ThreadName != "Question #001" {
		t.Fatalf("expected scheduled publish to use the publish-ordinal thread title format, got %+v", fake.publishedParams[0])
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

func TestServicePublishScheduledIfDueClearsExpiredSuppression(t *testing.T) {
	service, _, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID).WithSuppressedScheduledPublishDate(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))); err != nil {
		t.Fatalf("UpdateSettings(with stale suppression) failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question after stale suppression",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}
	if !published {
		t.Fatal("expected stale suppression not to block the current scheduled slot")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one publish attempt after stale suppression cleanup, got %d", len(fake.publishedParams))
	}

	settings, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() failed: %v", err)
	}
	if settings.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected stale suppression to be cleared after scheduler run, got %+v", settings)
	}
}

func TestServiceReconcileGuildClearsExpiredSuppressionForSuppressionOnlyConfig(t *testing.T) {
	service, _, _ := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC)
	}

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{SuppressScheduledPublishDateUTC: "2026-04-03"}); err != nil {
		t.Fatalf("UpdateSettings(suppression-only) failed: %v", err)
	}

	before, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings(before reconcile) failed: %v", err)
	}
	if before.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected stale suppression before reconcile, got %+v", before)
	}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild() failed: %v", err)
	}

	after, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings(after reconcile) failed: %v", err)
	}
	if after.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected reconcile to clear stale suppression-only config, got %+v", after)
	}
	if !after.IsZero() {
		t.Fatalf("expected suppression-only config to collapse to zero after cleanup, got %+v", after)
	}
}

func TestServicePublishScheduledIfDueResumesFailedProvisioning(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	beforeBoundary := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	boundary := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
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

	service.now = func() time.Time { return boundary }

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
				PublishedAt:                time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC),
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

func TestServiceResumeProvisioningSkipsDiscordWhenPostDeletedConcurrently(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	now := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Provisioning race question",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	schedule, err := resolvePublishSchedule(scheduledQOTDConfig(true, integrationQuestionChannelID))
	if err != nil {
		t.Fatalf("resolvePublishSchedule() failed: %v", err)
	}
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	lifecycle := EvaluateOfficialPost(schedule, publishDate, now)
	provisioning, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:                    "g1",
		DeckID:                     files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:           files.LegacyQOTDDefaultDeckName,
		QuestionID:                 question.ID,
		PublishMode:                string(PublishModeScheduled),
		PublishDateUTC:             publishDate,
		State:                      string(OfficialPostStateProvisioning),
		ChannelID:                  integrationQuestionChannelID,
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "list-entry-1",
		DiscordThreadID:            "thread-race",
		DiscordStarterMessageID:    "starter-race",
		AnswerChannelID:            "thread-race",
		QuestionTextSnapshot:       question.Body,
		GraceUntil:                 lifecycle.BecomesPreviousAt,
		ArchiveAt:                  lifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	if err := store.DeleteQOTDOfficialPostByID(context.Background(), provisioning.ID); err != nil {
		t.Fatalf("DeleteQOTDOfficialPostByID() failed: %v", err)
	}

	if _, err := service.resumeOfficialPostProvisioning(context.Background(), &discordgo.Session{}, *provisioning, now); err == nil {
		t.Fatal("expected resumeOfficialPostProvisioning() to fail when the post row is gone")
	}
	if len(fake.publishedParams) != 0 {
		t.Fatalf("expected no Discord publish attempt when provisioning row is deleted, got %d", len(fake.publishedParams))
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
	// At ArchiveAt the bot closes+locks the Discord thread so members can
	// still read the retroactive QOTD but new replies are blocked; only
	// moderators (MANAGE_THREADS) can reopen it.
	if fake.threadStates["official-thread-archive"] != (discordqotd.ThreadState{Pinned: false, Locked: true, Archived: true}) {
		t.Fatalf("expected archived official thread to be closed and locked for retroactive read-only access, got %+v", fake.threadStates["official-thread-archive"])
	}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild(second) failed: %v", err)
	}
	if len(fake.fetchCalls) != 1 {
		t.Fatalf("expected archived posts to be skipped on repeat reconcile, got fetches=%v", fake.fetchCalls)
	}
}

// TestServiceArchiveClosesAndLocksThreadAndPreservesModeratorReopen pins
// down two invariants that together make the retroactive-read-only contract
// safe to ship: (1) when a QOTD ages past ArchiveAt the bot closes+locks the
// Discord thread so verified members can still read the conversation but
// not post; (2) once the post is in the archived state, a subsequent
// ReconcileGuild does NOT re-issue SetThreadState — meaning a moderator who
// later unlocks the thread (using MANAGE_THREADS) is not fought by the bot.
func TestServiceArchiveClosesAndLocksThreadAndPreservesModeratorReopen(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Lock me on archive",
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
	const threadID = "thread-archive-lock"
	post, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
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
	if _, err := store.FinalizeQOTDOfficialPost(context.Background(), post.ID, "questions-list-thread", "questions-list-entry-lock", threadID, "starter-lock", threadID, publishedAt); err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), post.ID, string(OfficialPostStatePrevious), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState(seed previous) failed: %v", err)
	}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild(initial archive) failed: %v", err)
	}

	if got := fake.threadStates[threadID]; got != (discordqotd.ThreadState{Pinned: false, Locked: true, Archived: true}) {
		t.Fatalf("expected archive transition to close+lock the thread, got %+v", got)
	}

	archived, err := store.GetQOTDOfficialPostByID(context.Background(), post.ID)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByID(archived) failed: %v", err)
	}
	if archived == nil || archived.State != string(OfficialPostStateArchived) || archived.ArchivedAt == nil {
		t.Fatalf("expected post to be marked archived in storage, got %+v", archived)
	}

	// Simulate a moderator manually unlocking the thread via the Discord
	// UI (MANAGE_THREADS). The next reconcile must not re-impose the lock,
	// otherwise the bot would fight moderator decisions in a loop.
	fake.threadStates[threadID] = discordqotd.ThreadState{Pinned: false, Locked: false, Archived: false}

	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild(after moderator reopen) failed: %v", err)
	}

	if got := fake.threadStates[threadID]; got != (discordqotd.ThreadState{Pinned: false, Locked: false, Archived: false}) {
		t.Fatalf("expected reconcile to leave a moderator-unlocked archived thread alone, got %+v", got)
	}
}

// TestServicePublishScheduledIfDueGeneratesIdempotencyNonce verifies that
// every publish flows an idempotency nonce all the way from the service
// (where it's generated and persisted to the DB record) into the publisher
// params (which forward it to Discord with enforce_nonce=true). Without the
// nonce a crash between the Discord send and our DB write of the message ID
// would produce a duplicate post on resume; the nonce lets Discord
// deduplicate server-side.
func TestServicePublishScheduledIfDueGeneratesIdempotencyNonce(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question with nonce",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	}
	if _, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}

	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected one publish attempt, got %d", len(fake.publishedParams))
	}
	if strings.TrimSpace(fake.publishedParams[0].Nonce) == "" {
		t.Fatal("expected publisher params to carry an idempotency nonce")
	}

	post, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if post == nil {
		t.Fatal("expected published record to be persisted")
	}
	if strings.TrimSpace(post.Nonce) == "" {
		t.Fatal("expected idempotency nonce to be persisted on the record")
	}
	if post.Nonce != fake.publishedParams[0].Nonce {
		t.Fatalf("expected DB nonce to match the one forwarded to publisher, db=%q forwarded=%q", post.Nonce, fake.publishedParams[0].Nonce)
	}
}

// TestServicePublishScheduledIfDueResumeReusesPersistedNonce simulates the
// crash window: the DB record exists with a nonce but no Discord IDs (the
// publish was never finalized). The reconcile path resumes via
// resumeOfficialPostProvisioning, which must forward the SAME nonce back to
// Discord so server-side dedup returns the original message instead of
// creating a duplicate.
func TestServicePublishScheduledIfDueResumeReusesPersistedNonce(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Resumed publish",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	// Synthesize the post-crash state: provisioning record with a nonce but
	// no Discord IDs persisted.
	persistedNonce := "deadbeefcafebabe"
	if _, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		ConsumeAutomaticSlot: true,
		PublishDateUTC:       time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		State:                string(OfficialPostStateProvisioning),
		ChannelID:            integrationQuestionChannelID,
		QuestionTextSnapshot: question.Body,
		Nonce:                persistedNonce,
		GraceUntil:           time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	}
	if _, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("PublishScheduledIfDue() failed: %v", err)
	}

	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected resume to call publisher exactly once, got %d", len(fake.publishedParams))
	}
	if fake.publishedParams[0].Nonce != persistedNonce {
		t.Fatalf("expected resume to reuse persisted nonce, got %q want %q", fake.publishedParams[0].Nonce, persistedNonce)
	}
}

// TestServicePublishScheduledIfDueAbandonsOnUnrecoverableDiscordError checks
// the regression where reconcile would retry forever after the channel was
// deleted or the bot lost permission to post. We force the fake publisher to
// return a 404 (Unknown Channel), expect the post to land in 'abandoned',
// then verify that subsequent ReconcileGuild and PublishScheduledIfDue calls
// do NOT call Discord again.
func TestServicePublishScheduledIfDueAbandonsOnUnrecoverableDiscordError(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question for a deleted channel",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	unknownChannelErr := &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusNotFound},
		Message:  &discordgo.APIErrorMessage{Code: discordgo.ErrCodeUnknownChannel, Message: "Unknown Channel"},
	}
	fake.publishResponses = []fakePublishResponse{{err: unknownChannelErr}}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	}

	published, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err == nil {
		t.Fatal("expected publish to surface the unrecoverable error")
	}
	if published {
		t.Fatal("expected publish to NOT report success when Discord returned an unrecoverable error")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected exactly one publish attempt before abandonment, got %d", len(fake.publishedParams))
	}

	today := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	abandonedPost, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", today)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if abandonedPost == nil {
		t.Fatal("expected the failed post record to remain in storage")
	}
	if abandonedPost.State != string(OfficialPostStateAbandoned) {
		t.Fatalf("expected state=abandoned for unrecoverable Discord error, got %q", abandonedPost.State)
	}

	// Subsequent reconcile must NOT retry the abandoned post.
	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild() after abandonment failed: %v", err)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected ReconcileGuild to skip abandoned post, got %d publish attempts", len(fake.publishedParams))
	}

	// Subsequent scheduled tick on the same day must also stay idle: the
	// abandoned record occupies today's slot and needs admin action.
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}
	published, err = service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{})
	if err != nil {
		t.Fatalf("PublishScheduledIfDue() after abandonment failed: %v", err)
	}
	if published {
		t.Fatal("expected scheduler to leave abandoned slot alone")
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected scheduler not to re-call Discord on abandoned slot, got %d publish attempts", len(fake.publishedParams))
	}
}

// TestServicePublishScheduledIfDueRetriesTransientFailures confirms the
// inverse: a transient (5xx) error keeps the record retryable so the next
// reconcile cycle picks it up and publishes successfully.
func TestServicePublishScheduledIfDueRetriesTransientFailures(t *testing.T) {
	service, store, fake := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}
	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question through transient failure",
		Status: QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	transientErr := &discordgo.RESTError{
		Response: &http.Response{StatusCode: http.StatusInternalServerError},
		Message:  &discordgo.APIErrorMessage{Message: "internal server error"},
	}
	fake.publishResponses = []fakePublishResponse{{err: transientErr}}

	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	}
	if _, err := service.PublishScheduledIfDue(context.Background(), "g1", &discordgo.Session{}); err == nil {
		t.Fatal("expected first publish attempt to surface the transient error")
	}

	today := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	post, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", today)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if post == nil {
		t.Fatal("expected failed post record to remain in storage for retry")
	}
	if post.State != string(OfficialPostStateFailed) {
		t.Fatalf("expected state=failed for transient error, got %q", post.State)
	}

	// Next reconcile cycle resumes; fake publisher now returns success.
	if err := service.ReconcileGuild(context.Background(), "g1", &discordgo.Session{}); err != nil {
		t.Fatalf("ReconcileGuild() after transient error failed: %v", err)
	}
	if len(fake.publishedParams) != 2 {
		t.Fatalf("expected reconcile to retry transient failure, got %d publish attempts", len(fake.publishedParams))
	}

	recovered, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", today)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() after retry failed: %v", err)
	}
	if recovered == nil || recovered.PublishedAt == nil {
		t.Fatalf("expected transient retry to mark the post published, got %+v", recovered)
	}
}

func TestServiceReanimateSlotClearsSingleAbandonedRecordAndReleasesQuestion(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	publishDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	publishedAt := time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC)
	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{
		Body:   "Question to reanimate",
		Status: QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}
	question.Status = string(QuestionStatusReserved)
	question.ScheduledForDateUTC = &publishDate
	question.PublishedOnceAt = &publishedAt
	if _, err := store.UpdateQOTDQuestion(context.Background(), *question); err != nil {
		t.Fatalf("UpdateQOTDQuestion(reserved) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:                 "g1",
		DeckID:                  files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:        files.LegacyQOTDDefaultDeckName,
		QuestionID:              question.ID,
		PublishMode:             string(PublishModeScheduled),
		ConsumeAutomaticSlot:    true,
		PublishDateUTC:          publishDate,
		State:                   string(OfficialPostStateAbandoned),
		ChannelID:               integrationQuestionChannelID,
		DiscordThreadID:         "thread-abandoned",
		DiscordStarterMessageID: "starter-abandoned",
		QuestionTextSnapshot:    question.Body,
		GraceUntil:              time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC),
		ArchiveAt:               time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(abandoned) failed: %v", err)
	}

	cfg, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID).WithSuppressedScheduledPublishDate(publishDate))
	if err != nil {
		t.Fatalf("UpdateSettings(with suppression) failed: %v", err)
	}
	if !cfg.SuppressesScheduledPublishDate(publishDate) {
		t.Fatalf("expected suppression before reanimate, got %+v", cfg)
	}

	result, err := service.ReanimateSlot(context.Background(), "g1", nil, SlotMaintenanceParams{DateUTC: &publishDate})
	if err != nil {
		t.Fatalf("ReanimateSlot() failed: %v", err)
	}
	if result.OfficialPostsCleared != 1 || result.QuestionsReleased != 1 {
		t.Fatalf("expected one post cleared and one question released, got %+v", result)
	}
	if !result.ClearedSuppression {
		t.Fatalf("expected suppression clear to be reported, got %+v", result)
	}

	record, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if record != nil {
		t.Fatalf("expected abandoned post to be deleted, got %+v", record)
	}

	releasedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if releasedQuestion == nil {
		t.Fatal("expected reserved question to remain")
	}
	if releasedQuestion.Status != string(QuestionStatusReady) {
		t.Fatalf("expected released question status=ready, got %+v", releasedQuestion)
	}
	if releasedQuestion.ScheduledForDateUTC != nil || releasedQuestion.UsedAt != nil || releasedQuestion.PublishedOnceAt != nil {
		t.Fatalf("expected release to clear reservation/publish markers, got %+v", releasedQuestion)
	}

	settings, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() after reanimate failed: %v", err)
	}
	if settings.SuppressesScheduledPublishDate(publishDate) {
		t.Fatalf("expected suppression to be cleared after reanimate, got %+v", settings)
	}
}

func TestServiceClearPublishedDayStateRemovesAllRecordsForDate(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	publishDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	questionA, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{Body: "Question A", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion(questionA) failed: %v", err)
	}
	questionB, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{Body: "Question B", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion(questionB) failed: %v", err)
	}

	// idx_qotd_questions_schedule is unique per (guild_id,
	// scheduled_for_date_utc), so only ONE question can hold the schedule
	// reservation for publishDate at a time. The realistic shape this test
	// exercises is "two posts published for the same day": questionA owns
	// the schedule reservation (used after a scheduled publish), questionB
	// is a second post for the same day published manually — it never
	// claimed scheduled_for_date_utc, only published_once_at links it back
	// to the slot, which is enough for questionStillLinkedToOfficialPost.
	questionA.Status = string(QuestionStatusUsed)
	questionA.ScheduledForDateUTC = &publishDate
	questionA.PublishedOnceAt = timePtr(time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC))
	if _, err := store.UpdateQOTDQuestion(context.Background(), *questionA); err != nil {
		t.Fatalf("UpdateQOTDQuestion(questionA) failed: %v", err)
	}
	questionB.Status = string(QuestionStatusUsed)
	questionB.PublishedOnceAt = timePtr(time.Date(2026, 5, 7, 13, 30, 0, 0, time.UTC))
	if _, err := store.UpdateQOTDQuestion(context.Background(), *questionB); err != nil {
		t.Fatalf("UpdateQOTDQuestion(questionB) failed: %v", err)
	}

	for _, question := range []*storage.QOTDQuestionRecord{questionA, questionB} {
		if _, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
			GuildID:              "g1",
			DeckID:               files.LegacyQOTDDefaultDeckID,
			DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
			QuestionID:           question.ID,
			PublishMode:          string(PublishModeManual),
			PublishDateUTC:       publishDate,
			State:                string(OfficialPostStateCurrent),
			ChannelID:            integrationQuestionChannelID,
			QuestionTextSnapshot: question.Body,
			GraceUntil:           time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC),
			ArchiveAt:            time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
		}
	}

	_, err = service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID).WithSuppressedScheduledPublishDate(publishDate))
	if err != nil {
		t.Fatalf("UpdateSettings(with suppression) failed: %v", err)
	}

	result, err := service.ClearPublishedDayState(context.Background(), "g1", nil, SlotMaintenanceParams{DateUTC: &publishDate})
	if err != nil {
		t.Fatalf("ClearPublishedDayState() failed: %v", err)
	}
	if result.OfficialPostsCleared != 2 {
		t.Fatalf("expected two posts deleted for the day, got %+v", result)
	}
	if result.QuestionsReleased != 2 {
		t.Fatalf("expected two linked questions released, got %+v", result)
	}

	posts, err := store.ListQOTDOfficialPostsByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("ListQOTDOfficialPostsByDate() failed: %v", err)
	}
	if len(posts) != 0 {
		t.Fatalf("expected all posts for date to be removed, got %+v", posts)
	}

	for _, id := range []int64{questionA.ID, questionB.ID} {
		released, err := store.GetQOTDQuestion(context.Background(), "g1", id)
		if err != nil {
			t.Fatalf("GetQOTDQuestion(%d) failed: %v", id, err)
		}
		if released == nil || released.Status != string(QuestionStatusReady) || released.ScheduledForDateUTC != nil || released.PublishedOnceAt != nil {
			t.Fatalf("expected question %d to be reset to ready for re-test publish, got %+v", id, released)
		}
	}
}

func TestServiceClearPublishedDayStateDoesNotReleaseQuestionReusedForAnotherDay(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	publishDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	reusedPublish := time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)
	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{Body: "Reused question", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	question.Status = string(QuestionStatusUsed)
	question.PublishedOnceAt = &reusedPublish
	question.ScheduledForDateUTC = nil
	if _, err := store.UpdateQOTDQuestion(context.Background(), *question); err != nil {
		t.Fatalf("UpdateQOTDQuestion(reused) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeManual),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateCurrent),
		ChannelID:            integrationQuestionChannelID,
		QuestionTextSnapshot: question.Body,
		GraceUntil:           time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}

	result, err := service.ClearPublishedDayState(context.Background(), "g1", nil, SlotMaintenanceParams{DateUTC: &publishDate})
	if err != nil {
		t.Fatalf("ClearPublishedDayState() failed: %v", err)
	}
	if result.QuestionsReleased != 0 {
		t.Fatalf("expected no question release when linked question was reused on another day, got %+v", result)
	}

	stored, err := store.GetQOTDQuestion(context.Background(), "g1", question.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion() failed: %v", err)
	}
	if stored == nil || stored.Status != string(QuestionStatusUsed) || stored.PublishedOnceAt == nil {
		t.Fatalf("expected reused question to stay used after clearing older day state, got %+v", stored)
	}
	if !NormalizePublishDateUTC(*stored.PublishedOnceAt).Equal(time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected reused question publish date to remain day two, got %+v", stored)
	}
}

func TestServiceClearPublishedDayStateContinuesAfterMixedDiscordFailure(t *testing.T) {
	service, store, _ := newIntegrationTestQOTDService(t)
	publishDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)

	if _, err := service.UpdateSettings("g1", scheduledQOTDConfig(true, integrationQuestionChannelID).WithSuppressedScheduledPublishDate(publishDate)); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	questionFail, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{Body: "Question with Discord cleanup failure", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion(questionFail) failed: %v", err)
	}
	questionOK, err := service.CreateQuestion(context.Background(), "g1", "user-1", QuestionMutation{Body: "Question that should still clear", Status: QuestionStatusReady})
	if err != nil {
		t.Fatalf("CreateQuestion(questionOK) failed: %v", err)
	}

	questionFail.Status = string(QuestionStatusUsed)
	questionFail.ScheduledForDateUTC = nil
	questionFail.PublishedOnceAt = timePtr(time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC))
	if _, err := store.UpdateQOTDQuestion(context.Background(), *questionFail); err != nil {
		t.Fatalf("UpdateQOTDQuestion(used %d) failed: %v", questionFail.ID, err)
	}

	questionOK.Status = string(QuestionStatusReserved)
	questionOK.ScheduledForDateUTC = &publishDate
	if _, err := store.UpdateQOTDQuestion(context.Background(), *questionOK); err != nil {
		t.Fatalf("UpdateQOTDQuestion(reserved %d) failed: %v", questionOK.ID, err)
	}

	failingPost, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           questionFail.ID,
		PublishMode:          string(PublishModeManual),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateCurrent),
		ChannelID:            integrationQuestionChannelID,
		DiscordThreadID:      "thread-fail",
		QuestionTextSnapshot: questionFail.Body,
		GraceUntil:           time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(failingPost) failed: %v", err)
	}

	if _, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           questionOK.ID,
		PublishMode:          string(PublishModeManual),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateCurrent),
		ChannelID:            integrationQuestionChannelID,
		QuestionTextSnapshot: questionOK.Body,
		GraceUntil:           time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC),
		ArchiveAt:            time.Date(2026, 5, 9, 12, 43, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning(successPost) failed: %v", err)
	}

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New() failed: %v", err)
	}
	session.Client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req != nil && strings.Contains(req.URL.Path, "/channels/thread-fail") {
			return nil, errors.New("discord transport unavailable")
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header), Request: req}, nil
	})}

	result, err := service.ClearPublishedDayState(context.Background(), "g1", session, SlotMaintenanceParams{DateUTC: &publishDate})
	if err == nil {
		t.Fatal("expected mixed clear-day run to return partial error")
	}
	if !errors.Is(err, ErrSlotMaintenancePartial) {
		t.Fatalf("expected partial maintenance sentinel, got %v", err)
	}
	var partialErr *SlotMaintenancePartialError
	if !errors.As(err, &partialErr) {
		t.Fatalf("expected SlotMaintenancePartialError, got %T", err)
	}
	if result.OfficialPostsCleared != 1 || result.QuestionsReleased != 1 {
		t.Fatalf("expected one post cleared and one question released despite mixed failure, got %+v", result)
	}
	if result.ClearedSuppression {
		t.Fatalf("expected suppression to remain when clear_day has partial failures, got %+v", result)
	}
	if len(partialErr.FailedOfficialPostIDs) != 1 || partialErr.FailedOfficialPostIDs[0] != failingPost.ID {
		t.Fatalf("expected failed post telemetry to include only failing post %d, got %+v", failingPost.ID, partialErr.FailedOfficialPostIDs)
	}

	remainingPosts, err := store.ListQOTDOfficialPostsByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("ListQOTDOfficialPostsByDate() failed: %v", err)
	}
	if len(remainingPosts) != 1 || remainingPosts[0].ID != failingPost.ID {
		t.Fatalf("expected only failing post to remain after mixed clear-day, got %+v", remainingPosts)
	}

	failedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", questionFail.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(questionFail) failed: %v", err)
	}
	if failedQuestion == nil || failedQuestion.Status != string(QuestionStatusUsed) {
		t.Fatalf("expected failed question to remain used, got %+v", failedQuestion)
	}

	releasedQuestion, err := store.GetQOTDQuestion(context.Background(), "g1", questionOK.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(questionOK) failed: %v", err)
	}
	if releasedQuestion == nil || releasedQuestion.Status != string(QuestionStatusReady) || releasedQuestion.ScheduledForDateUTC != nil || releasedQuestion.PublishedOnceAt != nil {
		t.Fatalf("expected successful question to be released to ready, got %+v", releasedQuestion)
	}

	settings, err := service.Settings("g1")
	if err != nil {
		t.Fatalf("Settings() failed: %v", err)
	}
	if !settings.SuppressesScheduledPublishDate(publishDate) {
		t.Fatalf("expected suppression to remain after partial clear-day failure, got %+v", settings)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func timePtr(value time.Time) *time.Time {
	normalized := value.UTC()
	return &normalized
}

func timePtrString(id int64) string {
	return strconv.FormatInt(id, 10)
}
