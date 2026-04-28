package qotd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type publishCommandStubService struct {
	settings           files.QOTDConfig
	publishResult      *applicationqotd.PublishResult
	publishErr         error
	publishCalls       int
	lastPublishGuild   string
	lastPublishSession *discordgo.Session
}

type listCommandStubService struct {
	settings  files.QOTDConfig
	views     [][]storage.QOTDQuestionRecord
	listCalls int
}

func (s *publishCommandStubService) Settings(string) (files.QOTDConfig, error) {
	return s.settings, nil
}

func (s *publishCommandStubService) ListQuestions(context.Context, string, string) ([]storage.QOTDQuestionRecord, error) {
	panic("unexpected ListQuestions call")
}

func (s *publishCommandStubService) CreateQuestion(context.Context, string, string, applicationqotd.QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	panic("unexpected CreateQuestion call")
}

func (s *publishCommandStubService) DeleteQuestion(context.Context, string, int64) error {
	panic("unexpected DeleteQuestion call")
}

func (s *publishCommandStubService) SetNextQuestion(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	panic("unexpected SetNextQuestion call")
}

func (s *publishCommandStubService) ResetDeckState(context.Context, string, string) (applicationqotd.ResetDeckResult, error) {
	panic("unexpected ResetDeckState call")
}

func (s *publishCommandStubService) GetAutomaticQueueState(context.Context, string, string) (applicationqotd.AutomaticQueueState, error) {
	panic("unexpected GetAutomaticQueueState call")
}

func (s *publishCommandStubService) PublishNow(_ context.Context, guildID string, session *discordgo.Session) (*applicationqotd.PublishResult, error) {
	s.publishCalls++
	s.lastPublishGuild = guildID
	s.lastPublishSession = session
	return s.publishResult, s.publishErr
}

func (s *listCommandStubService) Settings(string) (files.QOTDConfig, error) {
	return s.settings, nil
}

func (s *listCommandStubService) ListQuestions(context.Context, string, string) ([]storage.QOTDQuestionRecord, error) {
	if len(s.views) == 0 {
		return nil, nil
	}
	idx := s.listCalls
	if idx >= len(s.views) {
		idx = len(s.views) - 1
	}
	s.listCalls++
	return append([]storage.QOTDQuestionRecord(nil), s.views[idx]...), nil
}

func (s *listCommandStubService) CreateQuestion(context.Context, string, string, applicationqotd.QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	panic("unexpected CreateQuestion call")
}

func (s *listCommandStubService) DeleteQuestion(context.Context, string, int64) error {
	panic("unexpected DeleteQuestion call")
}

func (s *listCommandStubService) SetNextQuestion(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	panic("unexpected SetNextQuestion call")
}

func (s *listCommandStubService) ResetDeckState(context.Context, string, string) (applicationqotd.ResetDeckResult, error) {
	panic("unexpected ResetDeckState call")
}

func (s *listCommandStubService) GetAutomaticQueueState(context.Context, string, string) (applicationqotd.AutomaticQueueState, error) {
	panic("unexpected GetAutomaticQueueState call")
}

func (s *listCommandStubService) PublishNow(context.Context, string, *discordgo.Session) (*applicationqotd.PublishResult, error) {
	panic("unexpected PublishNow call")
}

type interactionRecorder struct {
	mu        sync.Mutex
	responses []discordgo.InteractionResponse
}

func (r *interactionRecorder) addResponse(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *interactionRecorder) lastResponse(t *testing.T) discordgo.InteractionResponse {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.responses) == 0 {
		t.Fatal("expected at least one interaction response")
	}
	return r.responses[len(r.responses)-1]
}

func newQOTDCommandTestSession(t *testing.T) (*discordgo.Session, *interactionRecorder) {
	t.Helper()

	rec := &interactionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/callback") {
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			rec.addResponse(resp)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("failed to create discord session: %v", err)
	}
	if session.State == nil {
		t.Fatal("expected session state to be initialized")
	}
	return session, rec
}

func newQOTDCommandTestRouterWithService(
	t *testing.T,
	session *discordgo.Session,
	guildID string,
	ownerID string,
	service QuestionCatalogService,
) (*core.CommandRouter, *files.ConfigManager) {
	t.Helper()

	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewCommands(service).RegisterCommands(router)
	return router, cm
}

func dueQOTDCommandSchedule() files.QOTDPublishScheduleConfig {
	now := time.Now().UTC()
	hourUTC := now.Hour()
	minuteUTC := now.Minute()
	switch {
	case minuteUTC > 0:
		minuteUTC--
	case hourUTC > 0:
		hourUTC--
		minuteUTC = 59
	}
	return files.QOTDPublishScheduleConfig{
		HourUTC:   &hourUTC,
		MinuteUTC: &minuteUTC,
	}
}
