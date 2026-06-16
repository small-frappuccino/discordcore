package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
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

func (m *mockStatsService) ForceGuildUpdate(guildID string) {}

func (m *mockStatsService) wasUpdateCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updateCalled
}

type interactionRecorder struct {
	mu        sync.Mutex
	responses []api.InteractionResponse
}

func (r *interactionRecorder) addResponse(resp api.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *interactionRecorder) lastResponse(t *testing.T) api.InteractionResponse {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.responses) == 0 {
		t.Fatal("expected at least one interaction response")
	}
	return r.responses[len(r.responses)-1]
}

type mockTransport struct {
	t   *testing.T
	rec *interactionRecorder
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "/interactions/") {
		var payload api.InteractionResponse
		if req.Body != nil {
			_ = json.NewDecoder(req.Body).Decode(&payload)
		}
		m.rec.addResponse(payload)
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader([]byte("{}")))}, nil
}

func newStatsCommandTestRouter(
	t *testing.T,
	guildID string,
	ownerID string,
	cfg files.GuildConfig,
) (*core.ArikawaCommandRouter, *files.ConfigManager, *mockStatsService, *interactionRecorder) {
	t.Helper()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if err := cm.AddGuildConfig(cfg); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}

	router := core.NewArikawaCommandRouter("token", cm)
	mockSvc := &mockStatsService{}
	logger := slog.Default()
	NewStatsCommands(cm, mockSvc, logger).RegisterCommands(router)

	rec := &interactionRecorder{}
	return router, cm, mockSvc, rec
}

func newStatsSlashInteraction(
	guildID string,
	userID string,
	subCommand string,
	options []discord.CommandInteractionOption,
) *discord.InteractionEvent {
	gID, _ := discord.ParseSnowflake(guildID)
	uID, _ := discord.ParseSnowflake(userID)

	return &discord.InteractionEvent{
		ID:      123456789,
		AppID:   123456789,
		Token:   "token",
		GuildID: discord.GuildID(gID),
		Member: &discord.Member{
			User: discord.User{ID: discord.UserID(uID)},
		},
		Data: &discord.CommandInteraction{
			ID:   123456789,
			Name: "stats",
			Options: []discord.CommandInteractionOption{{
				Name:    subCommand,
				Type:    discord.SubcommandOptionType,
				Options: options,
			}},
		},
	}
}

func handleRawStatsInteraction(t *testing.T, router *core.ArikawaCommandRouter, cm *files.ConfigManager, rec *interactionRecorder, ic *discord.InteractionEvent) {
	t.Helper()

	cmdData := ic.Data.(*discord.CommandInteraction)
	cmd := router.GetAllCommands()[cmdData.Name]
	if cmd == nil {
		t.Fatalf("command %s not found", cmdData.Name)
	}

	client := api.NewClient("token")
	client.Client.Client = httpdriver.WrapClient(http.Client{
		Transport: &mockTransport{t: t, rec: rec},
	})

	ctx := &core.ArikawaContext{
		Client:      client,
		Interaction: ic,
		Config:      cm,
		Logger:      slog.Default(),
		GuildID:     ic.GuildID,
		UserID:      ic.Member.User.ID,
		GuildConfig: cm.GuildConfig(ic.GuildID.String()),
	}

	if err := cmd.Handle(ctx); err != nil && err != core.ErrAlreadyAcknowledged {
		t.Fatalf("command handler failed: %v", err)
	}
}

func requireEphemeralResponse(t *testing.T, resp api.InteractionResponse) {
	t.Helper()
	if resp.Data == nil || resp.Data.Flags&discord.EphemeralMessage == 0 {
		t.Fatalf("expected ephemeral response, got flags=%v", resp.Data.Flags)
	}
}

func testBoolPtr(v bool) *bool {
	return &v
}
