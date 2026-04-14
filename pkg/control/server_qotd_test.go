package control

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

type qotdRouteResponse struct {
	Status          string                         `json:"status"`
	GuildID         string                         `json:"guild_id"`
	Settings        files.QOTDConfig               `json:"settings"`
	Summary         qotdSummaryResponse            `json:"summary"`
	Question        qotdQuestionResponse           `json:"question"`
	Questions       []qotdQuestionResponse         `json:"questions"`
	CollectorResult qotdCollectorRunResultResponse `json:"result"`
}

type qotdCollectorRouteResponse struct {
	Status          string                         `json:"status"`
	GuildID         string                         `json:"guild_id"`
	Summary         qotdCollectorSummaryResponse   `json:"summary"`
	CollectorResult qotdCollectorRunResultResponse `json:"result"`
}

type routeFakePublisher struct {
	channelMessages map[string][]discordqotd.ArchivedMessage
}

func (routeFakePublisher) PublishOfficialPost(_ context.Context, _ *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	messageID := "message-" + params.PublishDateUTC.Format("20060102")
	threadID := "thread-" + params.PublishDateUTC.Format("20060102")
	return &discordqotd.PublishedOfficialPost{
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: "questions-list-entry-" + params.PublishDateUTC.Format("20060102"),
		ThreadID:                   threadID,
		StarterMessageID:           messageID,
		AnswerChannelID:            threadID,
		PublishedAt:                qotd.PublishTimeUTC(params.PublishDateUTC),
		PostURL:                    discordqotd.BuildThreadJumpURL(params.GuildID, threadID),
	}, nil
}

func (routeFakePublisher) UpsertAnswerMessage(context.Context, *discordgo.Session, discordqotd.UpsertAnswerMessageParams) (*discordqotd.UpsertedAnswerMessage, error) {
	return &discordqotd.UpsertedAnswerMessage{
		ChannelID:  "answers-channel",
		MessageID:  "answer-message",
		MessageURL: discordqotd.BuildMessageJumpURL("g1", "answers-channel", "answer-message"),
	}, nil
}

func (routeFakePublisher) SetThreadState(context.Context, *discordgo.Session, string, discordqotd.ThreadState) error {
	return nil
}

func (routeFakePublisher) FetchThreadMessages(context.Context, *discordgo.Session, string) ([]discordqotd.ArchivedMessage, error) {
	return nil, nil
}

func (p *routeFakePublisher) FetchChannelMessages(_ context.Context, _ *discordgo.Session, channelID, beforeMessageID string, limit int) ([]discordqotd.ArchivedMessage, error) {
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

func newQOTDControlTestServer(t *testing.T) (*Server, *qotd.Service, *storage.Store, *routeFakePublisher) {
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

	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil || srv.httpServer == nil || srv.httpServer.Handler == nil {
		t.Fatal("expected control server handler")
	}
	srv.SetBearerToken(controlTestAuthToken)
	publisher := &routeFakePublisher{}
	service := qotd.NewService(cm, store, publisher)
	srv.SetQOTDService(service)
	srv.SetDiscordSessionResolver(func(string) (*discordgo.Session, error) {
		return &discordgo.Session{}, nil
	})
	return srv, service, store, publisher
}

func decodeQOTDRouteResponse(t *testing.T, recBody string) qotdRouteResponse {
	t.Helper()

	var out qotdRouteResponse
	if err := json.Unmarshal([]byte(recBody), &out); err != nil {
		t.Fatalf("decode qotd response: %v body=%q", err, recBody)
	}
	return out
}

func decodeQOTDCollectorRouteResponse(t *testing.T, recBody string) qotdCollectorRouteResponse {
	t.Helper()

	var out qotdCollectorRouteResponse
	if err := json.Unmarshal([]byte(recBody), &out); err != nil {
		t.Fatalf("decode qotd collector response: %v body=%q", err, recBody)
	}
	return out
}

func TestQOTDRoutesSettingsQuestionsAndSummary(t *testing.T) {
	t.Parallel()

	srv, _, _, _ := newQOTDControlTestServer(t)
	handler := srv.httpServer.Handler

	settingsRec := performHandlerJSONRequest(t, handler, "PUT", "/v1/guilds/g1/qotd/settings", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:             files.LegacyQOTDDefaultDeckID,
			Name:           files.LegacyQOTDDefaultDeckName,
			Enabled:        true,
			ForumChannelID: "123456789012345678",
		}},
	})
	if settingsRec.Code != 200 {
		t.Fatalf("put settings status=%d body=%q", settingsRec.Code, settingsRec.Body.String())
	}
	settingsResp := decodeQOTDRouteResponse(t, settingsRec.Body.String())
	deck, ok := settingsResp.Settings.ActiveDeck()
	if !ok {
		t.Fatalf("expected qotd settings response to expose an active deck: %+v", settingsResp.Settings)
	}
	if !deck.Enabled || deck.ForumChannelID != "123456789012345678" {
		t.Fatalf("unexpected qotd settings response: %+v", settingsResp.Settings)
	}
	if strings.Contains(settingsRec.Body.String(), "staff_role_ids") {
		t.Fatalf("expected qotd settings payload to omit deprecated staff roles, body=%q", settingsRec.Body.String())
	}

	createFirst := performHandlerJSONRequest(t, handler, "POST", "/v1/guilds/g1/qotd/questions", map[string]any{
		"body":   "First question",
		"status": "ready",
	})
	if createFirst.Code != 201 {
		t.Fatalf("create first question status=%d body=%q", createFirst.Code, createFirst.Body.String())
	}
	firstResp := decodeQOTDRouteResponse(t, createFirst.Body.String())

	createSecond := performHandlerJSONRequest(t, handler, "POST", "/v1/guilds/g1/qotd/questions", map[string]any{
		"body":   "Second question",
		"status": "draft",
	})
	if createSecond.Code != 201 {
		t.Fatalf("create second question status=%d body=%q", createSecond.Code, createSecond.Body.String())
	}
	secondResp := decodeQOTDRouteResponse(t, createSecond.Body.String())

	reorderRec := performHandlerJSONRequest(t, handler, "POST", "/v1/guilds/g1/qotd/questions/reorder", map[string]any{
		"ordered_ids": []int64{secondResp.Question.ID, firstResp.Question.ID},
	})
	if reorderRec.Code != 200 {
		t.Fatalf("reorder questions status=%d body=%q", reorderRec.Code, reorderRec.Body.String())
	}
	reorderResp := decodeQOTDRouteResponse(t, reorderRec.Body.String())
	if len(reorderResp.Questions) != 2 || reorderResp.Questions[0].ID != secondResp.Question.ID {
		t.Fatalf("unexpected reordered questions: %+v", reorderResp.Questions)
	}

	summaryRec := performHandlerJSONRequest(t, handler, "GET", "/v1/guilds/g1/qotd", nil)
	if summaryRec.Code != 200 {
		t.Fatalf("summary status=%d body=%q", summaryRec.Code, summaryRec.Body.String())
	}
	summaryResp := decodeQOTDRouteResponse(t, summaryRec.Body.String())
	if summaryResp.Summary.Counts.Total != 2 || summaryResp.Summary.Counts.Ready != 1 || summaryResp.Summary.Counts.Draft != 1 {
		t.Fatalf("unexpected qotd summary counts: %+v", summaryResp.Summary.Counts)
	}
	activeDeck, ok := summaryResp.Summary.Settings.ActiveDeck()
	if !ok || !activeDeck.Enabled {
		t.Fatalf("expected summary settings to include an enabled active deck: %+v", summaryResp.Summary.Settings)
	}
}

func TestQOTDRoutesCollectAndExportArchivedQuestions(t *testing.T) {
	t.Parallel()

	srv, _, _, publisher := newQOTDControlTestServer(t)
	handler := srv.httpServer.Handler

	settingsRec := performHandlerJSONRequest(t, handler, "PUT", "/v1/guilds/g1/qotd/settings", files.QOTDConfig{
		Collector: files.QOTDCollectorConfig{
			SourceChannelID: "123456789012345678",
			AuthorIDs:       []string{"999999999999999999"},
			TitlePatterns:   []string{"Question Of The Day", "question!!"},
			StartDate:       "2026-01-01",
		},
	})
	if settingsRec.Code != 200 {
		t.Fatalf("put collector settings status=%d body=%q", settingsRec.Code, settingsRec.Body.String())
	}

	publisher.channelMessages = map[string][]discordqotd.ArchivedMessage{
		"123456789012345678": {
			{
				MessageID:          "message-2",
				AuthorID:           "999999999999999999",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"✰ question!! ✰","description":"What food have you never eaten but would really like to try?\nAuthor: QOTD Bot"}]`),
				CreatedAt:          time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC),
			},
			{
				MessageID:          "message-1",
				AuthorID:           "999999999999999999",
				AuthorNameSnapshot: "QOTD Bot",
				AuthorIsBot:        true,
				EmbedsJSON:         []byte(`[{"title":"Question Of The Day","description":"Tell us about a person you look up to!\n\nPreset Question"}]`),
				CreatedAt:          time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC),
			},
		},
	}

	collectRec := performHandlerJSONRequest(t, handler, "POST", "/v1/guilds/g1/qotd/collector/collect", nil)
	if collectRec.Code != 200 {
		t.Fatalf("collect status=%d body=%q", collectRec.Code, collectRec.Body.String())
	}
	collectResp := decodeQOTDCollectorRouteResponse(t, collectRec.Body.String())
	if collectResp.CollectorResult.MatchedMessages != 2 || collectResp.CollectorResult.NewQuestions != 2 {
		t.Fatalf("unexpected collector result: %+v", collectResp.CollectorResult)
	}
	if collectResp.CollectorResult.TotalQuestions != 2 || collectResp.Summary.TotalQuestions != 2 {
		t.Fatalf("expected collector summary to report two stored questions, got result=%+v summary=%+v", collectResp.CollectorResult, collectResp.Summary)
	}

	exportRec := performHandlerJSONRequest(t, handler, "GET", "/v1/guilds/g1/qotd/collector/export", nil)
	if exportRec.Code != 200 {
		t.Fatalf("export status=%d body=%q", exportRec.Code, exportRec.Body.String())
	}
	expected := "Tell us about a person you look up to!\nWhat food have you never eaten but would really like to try?\n"
	if exportRec.Body.String() != expected {
		t.Fatalf("unexpected export body:\n%s", exportRec.Body.String())
	}
	if got := exportRec.Header().Get("Content-Disposition"); !strings.Contains(got, "qotd-collected-questions.txt") {
		t.Fatalf("expected export filename header, got %q", got)
	}
}

func TestQOTDRoutesReconcileArchivesExpiredScheduledPost(t *testing.T) {
	t.Parallel()

	srv, service, store, _ := newQOTDControlTestServer(t)
	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:             files.LegacyQOTDDefaultDeckID,
			Name:           files.LegacyQOTDDefaultDeckName,
			Enabled:        true,
			ForumChannelID: "question-channel-1",
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	question, err := service.CreateQuestion(context.Background(), "g1", "user-1", qotd.QuestionMutation{
		Body:   "Archive from route",
		Status: qotd.QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	baseNow := time.Now().UTC()
	publishDate := qotd.CurrentPublishDateUTC(baseNow).AddDate(0, 0, -3)
	publishedAt := qotd.PublishTimeUTC(publishDate)
	lifecycle := qotd.EvaluateOfficialPost(publishDate, baseNow)
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		QuestionID:           question.ID,
		PublishMode:          string(qotd.PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(qotd.OfficialPostStatePrevious),
		ForumChannelID:       "question-channel-1",
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		t.Fatalf("CreateQOTDOfficialPostProvisioning() failed: %v", err)
	}
	official, err = store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "questions-list-thread", "questions-list-entry-route", "route-thread", "route-message", "route-thread", publishedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), official.ID, string(qotd.OfficialPostStatePrevious), nil, nil); err != nil {
		t.Fatalf("UpdateQOTDOfficialPostState() failed: %v", err)
	}

	rec := performHandlerJSONRequest(t, srv.httpServer.Handler, "POST", "/v1/guilds/g1/qotd/actions/reconcile", nil)
	if rec.Code != 200 {
		t.Fatalf("reconcile status=%d body=%q", rec.Code, rec.Body.String())
	}
	resp := decodeQOTDRouteResponse(t, rec.Body.String())
	if resp.Summary.CurrentPost != nil || resp.Summary.PreviousPost != nil {
		t.Fatalf("expected expired post to be archived and removed from summary window, got %+v", resp.Summary)
	}

	updated, err := store.GetQOTDOfficialPostByDate(context.Background(), "g1", publishDate)
	if err != nil {
		t.Fatalf("GetQOTDOfficialPostByDate() failed: %v", err)
	}
	if updated == nil || updated.State != string(qotd.OfficialPostStateArchived) || updated.ArchivedAt == nil {
		t.Fatalf("expected archived post after reconcile route, got %+v", updated)
	}
}
