package stats

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

type mockStatsService struct {
	mu           sync.Mutex
	updateCalled bool
}

func (m *mockStatsService) UpdateStatsChannels(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCalled = true
	return nil
}

func (m *mockStatsService) wasUpdateCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateCalled
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

func newStatsCommandTestSession(t *testing.T) (*discordgo.Session, *interactionRecorder) {
	t.Helper()

	rec := &interactionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "" {
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			rec.addResponse(resp)
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

func newStatsCommandTestRouter(
	t *testing.T,
	session *discordgo.Session,
	guildID string,
	ownerID string,
	cfg files.GuildConfig,
) (*core.CommandRouter, *files.ConfigManager, *mockStatsService) {
	t.Helper()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if err := cm.AddGuildConfig(cfg); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	mockSvc := &mockStatsService{}
	NewStatsCommands(cm, mockSvc).RegisterCommands(router)
	return router, cm, mockSvc
}

func newStatsSlashInteraction(
	guildID string,
	userID string,
	subCommand string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-stats-" + subCommand,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "stats",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{{
					Name:    subCommand,
					Type:    discordgo.ApplicationCommandOptionSubCommand,
					Options: options,
				}},
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

func testBoolPtr(v bool) *bool {
	return &v
}
