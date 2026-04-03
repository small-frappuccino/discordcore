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

type stubReplyThreadService struct {
	calls  []EnsureReplyThreadParams
	result *EnsureReplyThreadResult
	err    error
}

func (s *stubReplyThreadService) EnsureReplyThread(_ context.Context, _ *discordgo.Session, params EnsureReplyThreadParams) (*EnsureReplyThreadResult, error) {
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
			ChannelID: "official-thread-1",
			Member: &discordgo.Member{
				Nick: "Display Name",
				User: &discordgo.User{
					ID:       "user-1",
					Username: "fallback-name",
				},
			},
			Data: discordgo.MessageComponentInteractionData{
				CustomID: customID,
			},
		},
	}
}

func TestHandleAnswerButtonInteractionsDeferredEphemeralAndEditsSuccess(t *testing.T) {
	session, rec := newQOTDInteractionTestSession(t)
	service := &stubReplyThreadService{
		result: &EnsureReplyThreadResult{
			ThreadID:  "reply-thread-1",
			ThreadURL: "https://discord.com/channels/guild-1/reply-thread-1",
		},
	}

	handler := HandleAnswerButtonInteractions(service)
	handler(session, newQOTDComponentInteraction("qotd:answer:42"))

	if len(service.calls) != 1 {
		t.Fatalf("expected one EnsureReplyThread call, got %d", len(service.calls))
	}
	call := service.calls[0]
	if call.OfficialPostID != 42 || call.OfficialThreadID != "official-thread-1" || call.UserID != "user-1" {
		t.Fatalf("unexpected service call: %+v", call)
	}
	if call.UserDisplayName != "Display Name" {
		t.Fatalf("expected nick to be used as display name, got %+v", call)
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
	if !strings.Contains(rec.patchBody(), "Reply thread ready") {
		t.Fatalf("expected success message in patch body, got %q", rec.patchBody())
	}
}

func TestHandleAnswerButtonInteractionsMapsKnownErrors(t *testing.T) {
	session, rec := newQOTDInteractionTestSession(t)
	service := &stubReplyThreadService{err: ErrAnswerWindowClosed}

	handler := HandleAnswerButtonInteractions(service)
	handler(session, newQOTDComponentInteraction("qotd:answer:99"))

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

func TestHandleAnswerButtonInteractionsIgnoresOtherComponents(t *testing.T) {
	session, rec := newQOTDInteractionTestSession(t)
	service := &stubReplyThreadService{err: errors.New("should not be called")}

	handler := HandleAnswerButtonInteractions(service)
	handler(session, newQOTDComponentInteraction("runtime:other"))

	if len(service.calls) != 0 {
		t.Fatalf("expected handler to ignore non-qotd component")
	}
	if rec.patchCount() != 0 {
		t.Fatalf("expected no webhook edit for unrelated component")
	}
}
