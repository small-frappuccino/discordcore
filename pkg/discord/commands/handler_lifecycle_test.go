package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func newCommandHandlerSession(t *testing.T, handler http.HandlerFunc) *discordgo.Session {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	oldApplications := discordgo.EndpointApplications
	oldGuilds := discordgo.EndpointGuilds
	oldChannels := discordgo.EndpointChannels
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	discordgo.EndpointApplications = discordgo.EndpointAPI + "applications"
	discordgo.EndpointGuilds = discordgo.EndpointAPI + "guilds/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
		discordgo.EndpointApplications = oldApplications
		discordgo.EndpointGuilds = oldGuilds
		discordgo.EndpointChannels = oldChannels
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.State = discordgo.NewState()
	session.State.User = &discordgo.User{ID: "app-id"}
	return session
}

func TestCommandHandlerSetupAndShutdownLifecycle(t *testing.T) {
	var commandListCalls int32
	var commandCreateCalls int32

	session := newCommandHandlerSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			atomic.AddInt32(&commandListCalls, 1)
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			atomic.AddInt32(&commandCreateCalls, 1)
			resp := map[string]any{
				"id":          "generated",
				"name":        "generated",
				"description": "generated",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/applications/app-id/commands/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewMemoryConfigManager()
	handler := NewCommandHandler(session, cfgMgr)

	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if handler.runtimeHandlerCancel == nil {
		t.Fatalf("expected runtime interaction handler cancel function to be set")
	}
	if handler.commandManager == nil {
		t.Fatalf("expected command manager to be initialized")
	}

	// Re-run setup to exercise reinit cleanup path.
	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if handler.runtimeHandlerCancel == nil {
		t.Fatalf("expected runtime interaction handler cancel function after reinit")
	}
	if handler.commandManager == nil {
		t.Fatalf("expected command manager after reinit")
	}

	if got := atomic.LoadInt32(&commandListCalls); got == 0 {
		t.Fatalf("expected command listing call during setup")
	}
	if got := atomic.LoadInt32(&commandCreateCalls); got == 0 {
		t.Fatalf("expected command creation calls during setup")
	}

	if err := handler.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if handler.runtimeHandlerCancel != nil {
		t.Fatalf("expected runtime handler cancel to be cleared on shutdown")
	}
	if handler.commandManager != nil {
		t.Fatalf("expected command manager to be cleared on shutdown")
	}

	// Idempotent shutdown.
	if err := handler.Shutdown(); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}

func TestCommandHandlerSetupRollbackOnManagerFailure(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.State = discordgo.NewState()
	session.State.User = nil

	cfgMgr := files.NewMemoryConfigManager()
	handler := NewCommandHandler(session, cfgMgr)

	err = handler.SetupCommands()
	if err == nil {
		t.Fatalf("expected setup error when command manager setup fails")
	}
	if !strings.Contains(err.Error(), "failed to setup commands") {
		t.Fatalf("unexpected setup error: %v", err)
	}
	if handler.runtimeHandlerCancel != nil {
		t.Fatalf("runtime handler cancel should be cleared on setup rollback")
	}
	if handler.commandManager != nil {
		t.Fatalf("command manager should be cleared on setup rollback")
	}
}

func TestCommandHandlerSkipsGuildWithoutCommandsFeature(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewMemoryConfigManager()
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:       "guild-1",
				BotInstanceID: "alice",
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: boolPtr(false),
					},
				},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "question-1",
					}},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	handler := NewCommandHandlerForBot(nil, cfgMgr, "alice", "alice")
	if handler.handlesGuild("guild-1") {
		t.Fatal("expected slash command handler to remain disabled for commands-off guild")
	}
}
