package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type runtimePanelRecorder struct {
	mu             sync.Mutex
	callbackCalls  int
	webhookPatches int
	lastPatchBody  string
}

func (r *runtimePanelRecorder) addCallbackCall() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbackCalls++
}

func (r *runtimePanelRecorder) callbackCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.callbackCalls
}

func (r *runtimePanelRecorder) addWebhookPatch(body string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhookPatches++
	r.lastPatchBody = body
}

func (r *runtimePanelRecorder) webhookPatchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.webhookPatches
}

func (r *runtimePanelRecorder) patchBody() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastPatchBody
}

func newRuntimePanelTestSession(
	t *testing.T,
	callbackStatus int,
	webhookPatchStatus int,
) (*discordgo.Session, *runtimePanelRecorder) {
	t.Helper()

	if callbackStatus == 0 {
		callbackStatus = http.StatusOK
	}
	if webhookPatchStatus == 0 {
		webhookPatchStatus = http.StatusOK
	}

	rec := &runtimePanelRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.Contains(req.URL.Path, "/callback"):
			rec.addCallbackCall()
			if callbackStatus != http.StatusOK {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(callbackStatus)
				_, _ = w.Write([]byte(`{"message":"forced callback failure"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			return

		case strings.Contains(req.URL.Path, "/webhooks/") && req.Method == http.MethodPatch:
			body, _ := io.ReadAll(req.Body)
			rec.addWebhookPatch(string(body))

			w.Header().Set("Content-Type", "application/json")
			if webhookPatchStatus != http.StatusOK {
				w.WriteHeader(webhookPatchStatus)
				_, _ = w.Write([]byte(`{"message":"forced patch failure"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"edited-message","content":""}`))
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
		t.Fatalf("failed to create discord session: %v", err)
	}
	return session, rec
}

func withCapturedDefaultLogger(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(old)
	})
	return &buf
}

func newRuntimeComponentInteraction(customID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-component",
			AppID:   "app-id",
			Token:   "token-id",
			Type:    discordgo.InteractionMessageComponent,
			GuildID: "guild-1",
			Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
			Data: discordgo.MessageComponentInteractionData{
				CustomID: customID,
			},
		},
	}
}

func newRuntimeModalInteraction(st panelState, value string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-modal",
			AppID:   "app-id",
			Token:   "token-id",
			Type:    discordgo.InteractionModalSubmit,
			GuildID: "guild-1",
			Member:  &discordgo.Member{User: &discordgo.User{ID: "user-1"}},
			Data: discordgo.ModalSubmitInteractionData{
				CustomID: modalEditValueID + stateSep + st.encode(),
				Components: []discordgo.MessageComponent{
					&discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							&discordgo.TextInput{
								CustomID: modalFieldValue,
								Value:    value,
							},
						},
					},
				},
			},
		},
	}
}

type stubRuntimeApplier struct {
	err   error
	calls int
}

func (s *stubRuntimeApplier) Apply(_ context.Context, _ files.RuntimeConfig) error {
	s.calls++
	return s.err
}

func TestRespondInteractionWithLog_LogsFailure(t *testing.T) {
	session, _ := newRuntimePanelTestSession(t, http.StatusInternalServerError, http.StatusOK)
	interaction := newRuntimeComponentInteraction(cidButtonMain + stateSep + panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyBotTheme,
		Scope: "global",
	}.encode())
	logBuf := withCapturedDefaultLogger(t)

	respondInteractionWithLog(session, interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}, "test.respond")

	logText := logBuf.String()
	if !strings.Contains(logText, "Runtime config interaction respond failed") {
		t.Fatalf("expected respond failure to be logged, got logs=%q", logText)
	}
	if !strings.Contains(logText, "test.respond") {
		t.Fatalf("expected respond failure reason in log, got logs=%q", logText)
	}
}

func TestEditInteractionMessageWithLog_LogsFailure(t *testing.T) {
	session, rec := newRuntimePanelTestSession(t, http.StatusOK, http.StatusInternalServerError)
	interaction := newRuntimeComponentInteraction(cidButtonMain + stateSep + panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyBotTheme,
		Scope: "global",
	}.encode())
	logBuf := withCapturedDefaultLogger(t)

	editInteractionMessageWithLog(session, interaction, errorEmbed("forced"), nil, "test.edit")

	if rec.webhookPatchCount() == 0 {
		t.Fatalf("expected at least one webhook patch call")
	}
	logText := logBuf.String()
	if !strings.Contains(logText, "Runtime config interaction edit failed") {
		t.Fatalf("expected edit failure to be logged, got logs=%q", logText)
	}
	if !strings.Contains(logText, "test.edit") {
		t.Fatalf("expected edit failure reason in log, got logs=%q", logText)
	}
}

func TestHandleModalSubmit_WarnsWhenHotApplyFailsButPersists(t *testing.T) {
	session, rec := newRuntimePanelTestSession(t, http.StatusOK, http.StatusOK)
	cm := files.NewMemoryConfigManager()
	if err := cm.LoadConfig(); err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}

	st := panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyBotTheme,
		Scope: "global",
	}
	interaction := newRuntimeModalInteraction(st, "nebula")
	applier := &stubRuntimeApplier{err: errors.New("forced apply failure")}

	handleModalSubmit(session, interaction, cm, applier)

	if applier.calls != 1 {
		t.Fatalf("expected one hot-apply call, got %d", applier.calls)
	}
	if rec.webhookPatchCount() == 0 {
		t.Fatalf("expected panel edit after modal submit")
	}
	if !strings.Contains(rec.patchBody(), "Saved runtime config, but failed to apply changes immediately") {
		t.Fatalf("expected hot-apply warning in edited embed payload, got body=%q", rec.patchBody())
	}

	rc, err := loadRuntimeConfig(cm, "global")
	if err != nil {
		t.Fatalf("failed to reload runtime config: %v", err)
	}
	if rc.BotTheme != "nebula" {
		t.Fatalf("expected runtime config to persist updated bot_theme, got %q", rc.BotTheme)
	}
}

func TestRegisterCommands_RoutesRuntimeComponentThroughCoreRouter(t *testing.T) {
	session, rec := newRuntimePanelTestSession(t, http.StatusOK, http.StatusOK)
	cm := files.NewMemoryConfigManager()
	if err := cm.LoadConfig(); err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewRuntimeConfigCommands(cm).RegisterCommands(router)

	interaction := newRuntimeComponentInteraction(cidButtonMain + stateSep + panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyBotTheme,
		Scope: "global",
	}.encode())
	router.HandleInteraction(session, interaction)

	if rec.callbackCount() != 1 {
		t.Fatalf("expected exactly one interaction callback ack, got %d", rec.callbackCount())
	}
	if rec.webhookPatchCount() == 0 {
		t.Fatalf("expected runtime component to be handled through core router")
	}
}

func TestRegisterCommands_RoutesRuntimeModalThroughCoreRouter(t *testing.T) {
	session, rec := newRuntimePanelTestSession(t, http.StatusOK, http.StatusOK)
	cm := files.NewMemoryConfigManager()
	if err := cm.LoadConfig(); err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewRuntimeConfigCommands(cm).RegisterCommands(router)

	st := panelState{
		Mode:  pageMain,
		Group: "ALL",
		Key:   runtimeKeyBotTheme,
		Scope: "global",
	}
	router.HandleInteraction(session, newRuntimeModalInteraction(st, "nebula"))

	if rec.callbackCount() != 1 {
		t.Fatalf("expected exactly one modal callback ack, got %d", rec.callbackCount())
	}
	if rec.webhookPatchCount() == 0 {
		t.Fatalf("expected runtime modal to edit the panel through core router")
	}

	rc, err := loadRuntimeConfig(cm, "global")
	if err != nil {
		t.Fatalf("failed to reload runtime config: %v", err)
	}
	if rc.BotTheme != "nebula" {
		t.Fatalf("expected runtime modal to persist updated bot_theme, got %q", rc.BotTheme)
	}
}
