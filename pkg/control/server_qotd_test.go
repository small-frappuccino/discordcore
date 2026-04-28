//go:build integration

package control

import (
	"context"
	"encoding/json"
	"errors"
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

const (
	qotdRouteChannelID          = "123456789012345678"
	qotdRouteCollectorChannelID = "223456789012345678"
)

type qotdRouteResponse struct {
	Status    string                 `json:"status"`
	GuildID   string                 `json:"guild_id"`
	Settings  files.QOTDConfig       `json:"settings"`
	Summary   qotdSummaryResponse    `json:"summary"`
	Question  qotdQuestionResponse   `json:"question"`
	Questions []qotdQuestionResponse `json:"questions"`
}

type qotdPublishResultResponse struct {
	PostURL      string                    `json:"post_url"`
	OfficialPost *qotdOfficialPostResponse `json:"official_post"`
}

type qotdPublishRouteResponse struct {
	Status string                    `json:"status"`
	GuildID string                   `json:"guild_id"`
	Result qotdPublishResultResponse `json:"result"`
}

type qotdCollectorRouteResponse struct {
	Status          string                         `json:"status"`
	GuildID         string                         `json:"guild_id"`
	Summary         qotdCollectorSummaryResponse   `json:"summary"`
	CollectorResult qotdCollectorRunResultResponse `json:"result"`
}

type qotdCollectorRemoveDuplicatesRouteResponse struct {
	Status string                                     `json:"status"`
	GuildID string                                    `json:"guild_id"`
	Result qotdCollectorRemoveDuplicatesResultResponse `json:"result"`
}

type routeFakePublisher struct {
	channelMessages map[string][]discordqotd.ArchivedMessage
}

func routeQOTDSchedule() files.QOTDPublishScheduleConfig {
	hourUTC := 12
	minuteUTC := 43
	return files.QOTDPublishScheduleConfig{
		HourUTC:   &hourUTC,
		MinuteUTC: &minuteUTC,
	}
}

func (routeFakePublisher) PublishOfficialPost(_ context.Context, _ *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	messageID := "message-" + params.PublishDateUTC.Format("20060102")
	threadID := "thread-" + params.PublishDateUTC.Format("20060102")
	return &discordqotd.PublishedOfficialPost{
		ThreadID:                   threadID,
		StarterMessageID:           messageID,
		AnswerChannelID:            threadID,
		PublishedAt:                qotd.PublishTimeUTC(routeQOTDSchedule(), params.PublishDateUTC),
		PostURL:                    discordqotd.BuildMessageJumpURL(params.GuildID, params.ChannelID, messageID),
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

func decodeQOTDCollectorRemoveDuplicatesRouteResponse(t *testing.T, recBody string) qotdCollectorRemoveDuplicatesRouteResponse {
	t.Helper()

	var out qotdCollectorRemoveDuplicatesRouteResponse
	if err := json.Unmarshal([]byte(recBody), &out); err != nil {
		t.Fatalf("decode qotd collector duplicate removal response: %v body=%q", err, recBody)
	}
	return out
}

func decodeQOTDPublishRouteResponse(t *testing.T, recBody string) qotdPublishRouteResponse {
	t.Helper()

	var out qotdPublishRouteResponse
	if err := json.Unmarshal([]byte(recBody), &out); err != nil {
		t.Fatalf("decode qotd publish response: %v body=%q", err, recBody)
	}
	return out
}

func TestQOTDRoutesSettingsQuestionsAndSummary(t *testing.T) {
	t.Parallel()

	srv, _, _, _ := newQOTDControlTestServer(t)
	handler := srv.httpServer.Handler

	settingsRec := performHandlerJSONRequest(t, handler, "PUT", "/v1/guilds/g1/qotd/settings", files.QOTDConfig{
		VerifiedRoleID: "987654321098765432",
		ActiveDeckID:   files.LegacyQOTDDefaultDeckID,
		Schedule:       routeQOTDSchedule(),
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
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
	if !deck.Enabled || deck.ChannelID != "123456789012345678" {
		t.Fatalf("unexpected qotd settings response: %+v", settingsResp.Settings)
	}
	if settingsResp.Settings.VerifiedRoleID != "987654321098765432" {
		t.Fatalf("expected verified role to round-trip in qotd settings, got %+v", settingsResp.Settings)
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
	if firstResp.Question.DisplayID != 1 {
		t.Fatalf("expected first question visible id to be 1, got %+v", firstResp.Question)
	}

	createSecond := performHandlerJSONRequest(t, handler, "POST", "/v1/guilds/g1/qotd/questions", map[string]any{
		"body":   "Second question",
		"status": "draft",
	})
	if createSecond.Code != 201 {
		t.Fatalf("create second question status=%d body=%q", createSecond.Code, createSecond.Body.String())
	}
	secondResp := decodeQOTDRouteResponse(t, createSecond.Body.String())
	if secondResp.Question.DisplayID != 2 {
		t.Fatalf("expected second question visible id to be 2, got %+v", secondResp.Question)
	}

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
	if reorderResp.Questions[0].DisplayID != 1 || reorderResp.Questions[1].DisplayID != 2 {
		t.Fatalf("expected reordered questions to expose sequential visible ids, got %+v", reorderResp.Questions)
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

func TestQOTDRoutesPublishNowReturnsThreadAndAnswerChannelTargets(t *testing.T) {
	t.Parallel()

	srv, service, _, _ := newQOTDControlTestServer(t)
	handler := srv.httpServer.Handler

	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     routeQOTDSchedule(),
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: qotdRouteChannelID,
		}},
	}); err != nil {
		t.Fatalf("UpdateSettings() failed: %v", err)
	}

	if _, err := service.CreateQuestion(context.Background(), "g1", "user-1", qotd.QuestionMutation{
		Body:   "Publish route question",
		Status: qotd.QuestionStatusReady,
	}); err != nil {
		t.Fatalf("CreateQuestion() failed: %v", err)
	}

	publishRec := performHandlerJSONRequest(t, handler, "POST", "/v1/guilds/g1/qotd/actions/publish-now", nil)
	if publishRec.Code != 200 {
		t.Fatalf("publish qotd status=%d body=%q", publishRec.Code, publishRec.Body.String())
	}

	publishResp := decodeQOTDPublishRouteResponse(t, publishRec.Body.String())
	if publishResp.Result.OfficialPost == nil {
		t.Fatalf("expected official post payload, got body=%q", publishRec.Body.String())
	}
	officialPost := publishResp.Result.OfficialPost
	if officialPost.State != string(qotd.OfficialPostStateCurrent) {
		t.Fatalf("expected current official post state, got %+v", officialPost)
	}
	if officialPost.ThreadID == "" {
		t.Fatalf("expected thread id in official post response, got %+v", officialPost)
	}
	if officialPost.ThreadURL != discordqotd.BuildThreadJumpURL("g1", officialPost.ThreadID) {
		t.Fatalf("unexpected thread url in official post response: %+v", officialPost)
	}
	if officialPost.AnswerChannelID == "" {
		t.Fatalf("expected answer channel id in official post response, got %+v", officialPost)
	}
	if officialPost.AnswerChannelID != officialPost.ThreadID {
		t.Fatalf("expected daily thread to be the answer channel, got %+v", officialPost)
	}
	if officialPost.AnswerChannelURL != discordqotd.BuildChannelJumpURL("g1", officialPost.AnswerChannelID) {
		t.Fatalf("unexpected answer channel url in official post response: %+v", officialPost)
	}
	if officialPost.PostURL == "" || publishResp.Result.PostURL != officialPost.PostURL {
		t.Fatalf("expected publish result and official post to share post url, got result=%+v official=%+v", publishResp.Result, officialPost)
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

func TestQOTDRoutesRemoveDeckDuplicatesFromCollector(t *testing.T) {
	t.Parallel()

	srv, service, store, _ := newQOTDControlTestServer(t)
	handler := srv.httpServer.Handler

	settingsRec := performHandlerJSONRequest(t, handler, "PUT", "/v1/guilds/g1/qotd/settings", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     routeQOTDSchedule(),
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: qotdRouteChannelID,
		}},
	})
	if settingsRec.Code != 200 {
		t.Fatalf("put collector settings status=%d body=%q", settingsRec.Code, settingsRec.Body.String())
	}
	srv.SetDiscordSessionResolver(func(string) (*discordgo.Session, error) {
		return nil, errors.New("discord unavailable")
	})

	mutableDuplicate, err := service.CreateQuestion(context.Background(), "g1", "user-1", qotd.QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "Tell us about a person you look up to!",
		Status: qotd.QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(mutable duplicate) failed: %v", err)
	}
	immutableDuplicate, err := service.CreateQuestion(context.Background(), "g1", "user-2", qotd.QuestionMutation{
		DeckID: files.LegacyQOTDDefaultDeckID,
		Body:   "What food have you never eaten but would really like to try?",
		Status: qotd.QuestionStatusReady,
	})
	if err != nil {
		t.Fatalf("CreateQuestion(immutable duplicate) failed: %v", err)
	}
	immutableDuplicate.Status = string(qotd.QuestionStatusUsed)
	if _, err := store.UpdateQOTDQuestion(context.Background(), *immutableDuplicate); err != nil {
		t.Fatalf("UpdateQOTDQuestion(immutable duplicate) failed: %v", err)
	}

	created, err := store.CreateQOTDCollectedQuestions(context.Background(), []storage.QOTDCollectedQuestionRecord{
		{
			GuildID:                  "g1",
			SourceChannelID:          qotdRouteCollectorChannelID,
			SourceMessageID:          "message-1",
			SourceAuthorID:           "bot-1",
			SourceAuthorNameSnapshot: "QOTD Bot",
			SourceCreatedAt:          time.Date(2026, 4, 12, 15, 0, 0, 0, time.UTC),
			EmbedTitle:               "Question Of The Day",
			QuestionText:             "Tell us about a person you look up to!",
		},
		{
			GuildID:                  "g1",
			SourceChannelID:          qotdRouteCollectorChannelID,
			SourceMessageID:          "message-2",
			SourceAuthorID:           "bot-1",
			SourceAuthorNameSnapshot: "QOTD Bot",
			SourceCreatedAt:          time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC),
			EmbedTitle:               "question!!",
			QuestionText:             "What food have you never eaten but would really like to try?",
		},
	})
	if err != nil {
		t.Fatalf("CreateQOTDCollectedQuestions() failed: %v", err)
	}
	if created != 2 {
		t.Fatalf("expected two stored collected questions, got %d", created)
	}

	rec := performHandlerJSONRequest(t, handler, "POST", "/v1/guilds/g1/qotd/collector/remove-duplicates", map[string]any{
		"deck_id": files.LegacyQOTDDefaultDeckID,
	})
	if rec.Code != 200 {
		t.Fatalf("remove duplicates status=%d body=%q", rec.Code, rec.Body.String())
	}
	resp := decodeQOTDCollectorRemoveDuplicatesRouteResponse(t, rec.Body.String())
	if resp.Result.DeckID != files.LegacyQOTDDefaultDeckID {
		t.Fatalf("unexpected duplicate removal deck: %+v", resp.Result)
	}
	if resp.Result.ScannedMessages != 2 || resp.Result.MatchedMessages != 2 {
		t.Fatalf("unexpected duplicate removal scan: %+v", resp.Result)
	}
	if resp.Result.DuplicateQuestions != 2 || resp.Result.DeletedQuestions != 1 {
		t.Fatalf("unexpected duplicate removal result: %+v", resp.Result)
	}

	deleted, err := store.GetQOTDQuestion(context.Background(), "g1", mutableDuplicate.ID)
	if err != nil {
		t.Fatalf("GetQOTDQuestion(deleted) failed: %v", err)
	}
	if deleted != nil {
		t.Fatalf("expected mutable duplicate to be deleted, got %+v", deleted)
	}
}

func TestQOTDRoutesReconcileArchivesExpiredScheduledPost(t *testing.T) {
	t.Parallel()

	srv, service, store, _ := newQOTDControlTestServer(t)
	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     routeQOTDSchedule(),
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: qotdRouteChannelID,
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
	schedule := routeQOTDSchedule()
	publishDate := qotd.CurrentPublishDateUTC(schedule, baseNow).AddDate(0, 0, -3)
	publishedAt := qotd.PublishTimeUTC(schedule, publishDate)
	lifecycle := qotd.EvaluateOfficialPost(schedule, publishDate, baseNow)
	official, err := store.CreateQOTDOfficialPostProvisioning(context.Background(), storage.QOTDOfficialPostRecord{
		GuildID:              "g1",
		DeckID:               files.LegacyQOTDDefaultDeckID,
		DeckNameSnapshot:     files.LegacyQOTDDefaultDeckName,
		QuestionID:           question.ID,
		PublishMode:          string(qotd.PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(qotd.OfficialPostStatePrevious),
		ChannelID:            qotdRouteChannelID,
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
