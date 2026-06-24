package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordgo"
)

type handlerQOTDServiceStub struct{}

func (handlerQOTDServiceStub) ExecuteInGuildActorWithResult(guildID string, fn func() (any, error)) (any, error) {
	return fn()
}

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
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"

	oldTransport := http.DefaultTransport
	http.DefaultTransport = &mockTransport{serverURL: server.URL, transport: oldTransport}
	http.DefaultClient.Transport = http.DefaultTransport

	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
		discordgo.EndpointApplications = oldApplications
		discordgo.EndpointGuilds = oldGuilds
		discordgo.EndpointChannels = oldChannels
		http.DefaultTransport = oldTransport
		http.DefaultClient.Transport = oldTransport
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.State = discordgo.NewState()
	session.State.User = &discordgo.User{ID: "123456789"}
	return session
}

func TestCommandHandlerSetupAndShutdownLifecycle(t *testing.T) {
	var commandListCalls int32
	var commandCreateCalls int32

	session := newCommandHandlerSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/123456789/commands"):
			atomic.AddInt32(&commandListCalls, 1)
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/applications/123456789/commands"):
			atomic.AddInt32(&commandCreateCalls, 1)
			var commands []discordgo.ApplicationCommand
			json.NewDecoder(r.Body).Decode(&commands)
			for i := range commands {
				commands[i].ID = "123456789"
			}
			json.NewEncoder(w).Encode(&commands)
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/applications/123456789/commands/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	handler, err := NewCommandHandler(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}

	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if handler.GetRouter() == nil {
		t.Fatalf("expected command manager to be initialized")
	}

	// Re-run setup to exercise reinit cleanup path.
	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if handler.GetRouter() == nil {
		t.Fatalf("expected command manager after reinit")
	}

	if atomic.LoadInt32(&commandCreateCalls) == 0 {
		t.Fatalf("expected command create call during setup")
	}

	if err := handler.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if handler.GetRouter() != nil {
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

	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	handler, err := NewCommandHandler(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}

	err = handler.SetupCommands()
	if err == nil {
		t.Fatalf("expected setup error when command manager setup fails")
	}
	if !strings.Contains(err.Error(), "cannot setup commands: session user state is missing") {
		t.Fatalf("unexpected setup error: %v", err)
	}
}

func TestCommandHandlerSkipsGuildWithoutCommandsFeature(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	if _, err := cfgMgr.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "a"},
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

	session, _ := discordgo.New("Bot test-token")
	handler, err := NewCommandHandlerForBot(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		BotInstanceID:  "generic",
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}
	if handler.handlesGuild("guild-1") {
		t.Fatal("expected slash command handler to remain disabled for commands-off guild")
	}
}

type mockTransport struct {
	serverURL string
	transport http.RoundTripper
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, "discord.com") {
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(m.serverURL, "http://")
	}
	transport := m.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if transport == nil {
		return nil, fmt.Errorf("no transport")
	}
	return transport.RoundTrip(req)
}
