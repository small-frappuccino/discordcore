package qotd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type qotdInteractionRecorder struct {
	mu             sync.Mutex
	responses      []discordgo.InteractionResponse
	webhookPatches int
	lastPatchBody  string
}

func (r *qotdInteractionRecorder) addResponse(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *qotdInteractionRecorder) addWebhookPatch(body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhookPatches++
	r.lastPatchBody = body
}

func (r *qotdInteractionRecorder) lastResponse(t *testing.T) discordgo.InteractionResponse {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.responses) == 0 {
		t.Fatal("expected at least one interaction response")
	}
	return r.responses[len(r.responses)-1]
}

func (r *qotdInteractionRecorder) patchBody() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastPatchBody
}

func (r *qotdInteractionRecorder) patchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.webhookPatches
}

type stubAnswerSubmissionService struct {
	calls  []SubmitAnswerParams
	result *SubmitAnswerResult
	err    error
}

func (s *stubAnswerSubmissionService) SubmitAnswer(_ context.Context, _ *discordgo.Session, params SubmitAnswerParams) (*SubmitAnswerResult, error) {
	s.calls = append(s.calls, params)
	if s.err != nil {
		return nil, s.err
	}
	return s.result, nil
}

func newQOTDInteractionTestSession(t *testing.T) (*discordgo.Session, *qotdInteractionRecorder) {
	t.Helper()

	rec := &qotdInteractionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.Contains(req.URL.Path, "/callback"):
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			rec.addResponse(resp)
			w.WriteHeader(http.StatusOK)
			return
		case strings.Contains(req.URL.Path, "/webhooks/") && req.Method == http.MethodPatch:
			body, _ := io.ReadAll(req.Body)
			rec.addWebhookPatch(string(body))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"edited-response"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	return session, rec
}

func newQOTDComponentInteraction(customID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:        "interaction-answer",
			AppID:     "app-id",
			Token:     "token-id",
			Type:      discordgo.InteractionMessageComponent,
			GuildID:   "guild-1",
			ChannelID: "question-channel-1",
			Member: &discordgo.Member{
				Nick: "Display Name",
				User: &discordgo.User{
					ID:       "user-1",
					Username: "fallback-name",
					Avatar:   "avatar-hash",
				},
			},
			Data: discordgo.MessageComponentInteractionData{
				CustomID: customID,
			},
		},
	}
}

func newQOTDModalInteraction(customID, answer string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:        "interaction-submit",
			AppID:     "app-id",
			Token:     "token-id",
			Type:      discordgo.InteractionModalSubmit,
			GuildID:   "guild-1",
			ChannelID: "question-channel-1",
			Member: &discordgo.Member{
				Nick: "Display Name",
				User: &discordgo.User{
					ID:       "user-1",
					Username: "fallback-name",
					Avatar:   "avatar-hash",
				},
			},
			Data: discordgo.ModalSubmitInteractionData{
				CustomID: customID,
				Components: []discordgo.MessageComponent{
					&discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							&discordgo.TextInput{
								CustomID: answerModalFieldID,
								Value:    answer,
							},
						},
					},
				},
			},
		},
	}
}

func TestHandleQOTDInteractionsOpensAnswerModal(t *testing.T) {
	session, rec := newQOTDInteractionTestSession(t)
	service := &stubAnswerSubmissionService{}

	handler := HandleQOTDInteractions(service)
	handler(session, newQOTDComponentInteraction("qotd:answer:42"))

	if len(service.calls) != 0 {
		t.Fatalf("expected button interaction to only open modal, got %+v", service.calls)
	}

	resp := rec.lastResponse(t)
	if resp.Type != discordgo.InteractionResponseModal {
		t.Fatalf("expected modal response, got %+v", resp)
	}
	if resp.Data == nil || resp.Data.CustomID != "qotd:answer:submit:42" {
		t.Fatalf("expected modal custom id for official post, got %+v", resp.Data)
	}
	if resp.Data.Title != "Answer QOTD #42" {
		t.Fatalf("unexpected modal title: %+v", resp.Data)
	}
}

func TestHandleQOTDInteractionsSubmitsAnswerDeferredEphemeralAndEditsSuccess(t *testing.T) {
	session, rec := newQOTDInteractionTestSession(t)
	service := &stubAnswerSubmissionService{
		result: &SubmitAnswerResult{
			MessageID:  "answer-message-1",
			ChannelID:  "answers-channel-1",
			MessageURL: "https://discord.com/channels/guild-1/answers-channel-1/answer-message-1",
		},
	}

	handler := HandleQOTDInteractions(service)
	handler(session, newQOTDModalInteraction("qotd:answer:submit:42", "My final answer"))

	if len(service.calls) != 1 {
		t.Fatalf("expected one SubmitAnswer call, got %d", len(service.calls))
	}
	call := service.calls[0]
	if call.OfficialPostID != 42 || call.UserID != "user-1" {
		t.Fatalf("unexpected service call: %+v", call)
	}
	if call.UserDisplayName != "Display Name" {
		t.Fatalf("expected nick to be used as display name, got %+v", call)
	}
	if got, want := call.UserAvatarURL, (&discordgo.User{ID: "user-1", Avatar: "avatar-hash"}).AvatarURL("256"); got != want {
		t.Fatalf("expected avatar url to be forwarded, got %q want %q", got, want)
	}
	if call.AnswerText != "My final answer" {
		t.Fatalf("expected answer text from modal, got %+v", call)
	}

	resp := rec.lastResponse(t)
	if resp.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("expected deferred ephemeral response, got %+v", resp)
	}
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response flags, got %+v", resp.Data)
	}

	if rec.patchCount() != 1 {
		t.Fatalf("expected one webhook patch edit, got %d", rec.patchCount())
	}
	if !strings.Contains(rec.patchBody(), "Your answer was posted") {
		t.Fatalf("expected success message in patch body, got %q", rec.patchBody())
	}
}

func TestHandleQOTDInteractionsMapsKnownErrors(t *testing.T) {
	session, rec := newQOTDInteractionTestSession(t)
	service := &stubAnswerSubmissionService{err: ErrAnswerWindowClosed}

	handler := HandleQOTDInteractions(service)
	handler(session, newQOTDModalInteraction("qotd:answer:submit:99", "Too late"))

	resp := rec.lastResponse(t)
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response flags, got %+v", resp.Data)
	}
	if rec.patchCount() != 1 {
		t.Fatalf("expected one webhook patch edit, got %d", rec.patchCount())
	}
	if !strings.Contains(rec.patchBody(), "no longer accepting replies") {
		t.Fatalf("expected closed-window message, got %q", rec.patchBody())
	}
}

func TestHandleQOTDInteractionsIgnoresOtherComponents(t *testing.T) {
	session, rec := newQOTDInteractionTestSession(t)
	service := &stubAnswerSubmissionService{err: errors.New("should not be called")}

	handler := HandleQOTDInteractions(service)
	handler(session, newQOTDComponentInteraction("runtime:other"))
	handler(session, newQOTDModalInteraction("runtime:other", "ignored"))

	if len(service.calls) != 0 {
		t.Fatalf("expected handler to ignore non-qotd component")
	}
	if rec.patchCount() != 0 {
		t.Fatalf("expected no webhook edit for unrelated component")
	}
}
