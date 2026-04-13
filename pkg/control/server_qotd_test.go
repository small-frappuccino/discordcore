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
	Status    string                 `json:"status"`
	GuildID   string                 `json:"guild_id"`
	Settings  files.QOTDConfig       `json:"settings"`
	Summary   qotdSummaryResponse    `json:"summary"`
	Question  qotdQuestionResponse   `json:"question"`
	Questions []qotdQuestionResponse `json:"questions"`
}

type routeFakePublisher struct{}

func (routeFakePublisher) PublishOfficialPost(_ context.Context, _ *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	messageID := "message-" + params.PublishDateUTC.Format("20060102")
	return &discordqotd.PublishedOfficialPost{
		StarterMessageID: messageID,
		PublishedAt:      qotd.PublishTimeUTC(params.PublishDateUTC),
		PostURL:          discordqotd.BuildMessageJumpURL(params.GuildID, params.QuestionChannelID, messageID),
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

func newQOTDControlTestServer(t *testing.T) (*Server, *qotd.Service, *storage.Store) {
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
	service := qotd.NewService(cm, store, routeFakePublisher{})
	srv.SetQOTDService(service)
	srv.SetDiscordSessionResolver(func(string) (*discordgo.Session, error) {
		return &discordgo.Session{}, nil
	})
	return srv, service, store
}

func decodeQOTDRouteResponse(t *testing.T, recBody string) qotdRouteResponse {
	t.Helper()

	var out qotdRouteResponse
	if err := json.Unmarshal([]byte(recBody), &out); err != nil {
		t.Fatalf("decode qotd response: %v body=%q", err, recBody)
	}
	return out
}

func TestQOTDRoutesSettingsQuestionsAndSummary(t *testing.T) {
	t.Parallel()

	srv, _, _ := newQOTDControlTestServer(t)
	handler := srv.httpServer.Handler

	settingsRec := performHandlerJSONRequest(t, handler, "PUT", "/v1/guilds/g1/qotd/settings", files.QOTDConfig{
		Enabled:           true,
		QuestionChannelID: "123456789012345678",
		ResponseChannelID: "223456789012345678",
	})
	if settingsRec.Code != 200 {
		t.Fatalf("put settings status=%d body=%q", settingsRec.Code, settingsRec.Body.String())
	}
	settingsResp := decodeQOTDRouteResponse(t, settingsRec.Body.String())
	if !settingsResp.Settings.Enabled || settingsResp.Settings.QuestionChannelID != "123456789012345678" {
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
	if !summaryResp.Summary.Settings.Enabled {
		t.Fatalf("expected summary settings to include enabled qotd config: %+v", summaryResp.Summary.Settings)
	}
}

func TestQOTDRoutesReconcileArchivesExpiredScheduledPost(t *testing.T) {
	t.Parallel()

	srv, service, store := newQOTDControlTestServer(t)
	if _, err := service.UpdateSettings("g1", files.QOTDConfig{
		Enabled:           true,
		QuestionChannelID: "question-channel-1",
		ResponseChannelID: "answers-channel-1",
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
	official, err = store.FinalizeQOTDOfficialPost(context.Background(), official.ID, "route-thread", "route-message", publishedAt)
	if err != nil {
		t.Fatalf("FinalizeQOTDOfficialPost() failed: %v", err)
	}
	if _, err := store.UpdateQOTDOfficialPostState(context.Background(), official.ID, string(qotd.OfficialPostStatePrevious), false, nil, nil); err != nil {
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
