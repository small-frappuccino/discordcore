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

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

type publishCommandStubService struct {
	t                 *testing.T
	settings          files.QOTDConfig
	publishResult     *applicationqotd.PublishResult
	publishErr        error
	publishCalls      int
	lastPublishGuild  string
	lastPublishParams applicationqotd.PublishNowParams
}

type listCommandStubService struct {
	t                       *testing.T
	settings                files.QOTDConfig
	views                   [][]storage.QOTDQuestionRecord
	listCalls               int
	markPublishedResult     *storage.QOTDQuestionRecord
	markPublishedErr        error
	markPublishedCalls      int
	lastMarkPublishedGuild  string
	lastMarkPublishedDeckID string
	lastMarkPublishedID     int64
}

func (s *publishCommandStubService) Settings(string) (files.QOTDConfig, error) {
	return s.settings, nil
}

func (s *publishCommandStubService) ListQuestions(context.Context, string, string) ([]storage.QOTDQuestionRecord, error) {
	s.t.Fatal("unexpected ListQuestions call")
	return nil, nil
}

func (s *publishCommandStubService) CreateQuestion(context.Context, string, string, applicationqotd.QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	s.t.Fatal("unexpected CreateQuestion call")
	return nil, nil
}

func (s *publishCommandStubService) DeleteQuestion(context.Context, string, int64) error {
	s.t.Fatal("unexpected DeleteQuestion call")
	return nil
}

func (s *publishCommandStubService) RestoreUsedQuestion(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	s.t.Fatal("unexpected RestoreUsedQuestion call")
	return nil, nil
}

func (s *publishCommandStubService) MarkQuestionPublished(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	s.t.Fatal("unexpected MarkQuestionPublished call")
	return nil, nil
}

func (s *publishCommandStubService) GetAutomaticQueueState(context.Context, string, string) (applicationqotd.AutomaticQueueState, error) {
	s.t.Fatal("unexpected GetAutomaticQueueState call")
	return applicationqotd.AutomaticQueueState{}, nil
}

func (s *publishCommandStubService) PublishNowWithParams(_ context.Context, guildID string, params applicationqotd.PublishNowParams) (*applicationqotd.PublishResult, error) {
	s.publishCalls++
	s.lastPublishGuild = guildID
	s.lastPublishParams = params
	return s.publishResult, s.publishErr
}

func (s *publishCommandStubService) ReplaceCurrentPublish(_ context.Context, guildID string) (*applicationqotd.PublishResult, error) {
	s.publishCalls++
	s.lastPublishGuild = guildID
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
	s.t.Fatal("unexpected CreateQuestion call")
	return nil, nil
}

func (s *listCommandStubService) DeleteQuestion(context.Context, string, int64) error {
	s.t.Fatal("unexpected DeleteQuestion call")
	return nil
}

func (s *listCommandStubService) RestoreUsedQuestion(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	s.t.Fatal("unexpected RestoreUsedQuestion call")
	return nil, nil
}

func (s *listCommandStubService) MarkQuestionPublished(_ context.Context, guildID, deckID string, questionID int64) (*storage.QOTDQuestionRecord, error) {
	s.markPublishedCalls++
	s.lastMarkPublishedGuild = guildID
	s.lastMarkPublishedDeckID = deckID
	s.lastMarkPublishedID = questionID
	return s.markPublishedResult, s.markPublishedErr
}

func (s *listCommandStubService) GetAutomaticQueueState(context.Context, string, string) (applicationqotd.AutomaticQueueState, error) {
	return applicationqotd.AutomaticQueueState{}, nil
}

func (s *listCommandStubService) PublishNowWithParams(context.Context, string, applicationqotd.PublishNowParams) (*applicationqotd.PublishResult, error) {
	return nil, nil
}

func (s *listCommandStubService) ReplaceCurrentPublish(context.Context, string) (*applicationqotd.PublishResult, error) {
	return nil, nil
}

type interactionRecorder struct {
	mu        sync.Mutex
	responses []discordgo.InteractionResponse
	edits     []string
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

func (r *interactionRecorder) addEdit(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edits = append(r.edits, content)
}

func (r *interactionRecorder) lastEdit(t *testing.T) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.edits) == 0 {
		t.Fatal("expected at least one webhook edit")
	}
	return r.edits[len(r.edits)-1]
}

func newQOTDCommandTestSession(t *testing.T) (*discordgo.Session, *interactionRecorder) {
	t.Helper()

	rec := &interactionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/callback") {
			var resp discordgo.InteractionResponse
			json.NewDecoder(req.Body).Decode(&resp)
			rec.addResponse(resp)
			w.WriteHeader(http.StatusOK)
			return
		}
		if req.Method == http.MethodPatch && strings.Contains(req.URL.Path, "/messages/@original") {
			var payload struct {
				Content *string `json:"content"`
			}
			json.NewDecoder(req.Body).Decode(&payload)
			if payload.Content != nil {
				rec.addEdit(*payload.Content)
			} else {
				rec.addEdit("")
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"message-1"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = discordgo.EndpointAPI + "webhooks/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
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

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{
		ID:      guildID,
		OwnerID: ownerID,
		Roles: []*discordgo.Role{
			{ID: "admin-role", Permissions: discordgo.PermissionAdministrator},
		},
	}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}
	if err := session.State.MemberAdd(&discordgo.Member{
		GuildID: guildID,
		User:    &discordgo.User{ID: ownerID},
		Roles:   []string{"admin-role"},
	}); err != nil {
		t.Fatalf("failed to add member to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewCommands(service).RegisterCommands(router)
	return router, cm
}

func newQOTDSlashInteraction(
	guildID string,
	userID string,
	subCommand string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-qotd-questions-" + subCommand,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member: &discordgo.Member{
				User:  &discordgo.User{ID: userID},
				Roles: []string{"admin-role"},
			},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: groupName,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{{
					Name: questionsGroupName,
					Type: discordgo.ApplicationCommandOptionSubCommandGroup,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{{
						Name:    subCommand,
						Type:    discordgo.ApplicationCommandOptionSubCommand,
						Options: options,
					}},
				}},
			},
		},
	}
}

func newQOTDRootSlashInteraction(
	guildID string,
	userID string,
	subCommand string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-qotd-" + subCommand,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member: &discordgo.Member{
				User:  &discordgo.User{ID: userID},
				Roles: []string{"admin-role"},
			},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: groupName,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{{
					Name:    subCommand,
					Type:    discordgo.ApplicationCommandOptionSubCommand,
					Options: options,
				}},
			},
		},
	}
}

func newQOTDComponentInteraction(guildID, userID, customID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-qotd-questions-list-component",
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionMessageComponent,
			GuildID: guildID,
			Member: &discordgo.Member{
				User:  &discordgo.User{ID: userID},
				Roles: []string{"admin-role"},
			},
			Data: discordgo.MessageComponentInteractionData{
				CustomID: customID,
			},
		},
	}
}

func requireEphemeralResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func requirePublicResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

// requirePublicDeferredAck asserts that the response is the public deferred
// channel message ack the router sends before a long-running slash handler
// runs. The actual user-visible content arrives through a follow-up edit and
// should be inspected via interactionRecorder.lastEdit instead.
func requirePublicDeferredAck(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("expected deferred channel message ack, got type=%v content=%q", resp.Type, resp.Data.Content)
	}
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public deferred ack, got ephemeral flag set")
	}
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
