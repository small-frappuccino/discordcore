package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type handlerQOTDServiceStub struct{}

func (handlerQOTDServiceStub) Settings(string) (files.QOTDConfig, error) {
	return files.QOTDConfig{}, nil
}
func (handlerQOTDServiceStub) ListQuestions(context.Context, string, string) ([]storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) CreateQuestion(context.Context, string, string, applicationqotd.QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) DeleteQuestion(context.Context, string, int64) error { return nil }
func (handlerQOTDServiceStub) RestoreUsedQuestion(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) MarkQuestionPublished(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) GetAutomaticQueueState(context.Context, string, string) (applicationqotd.AutomaticQueueState, error) {
	return applicationqotd.AutomaticQueueState{}, nil
}
func (handlerQOTDServiceStub) PublishNowWithParams(context.Context, string, *discordgo.Session, applicationqotd.PublishNowParams) (*applicationqotd.PublishResult, error) {
	return nil, nil
}

func (handlerQOTDServiceStub) ReplaceCurrentPublish(context.Context, string, *discordgo.Session) (*applicationqotd.PublishResult, error) {
	return nil, nil
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
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			atomic.AddInt32(&commandCreateCalls, 1)
			var commands []discordgo.ApplicationCommand
			_ = json.NewDecoder(r.Body).Decode(&commands)
			for i := range commands {
				commands[i].ID = "generated"
			}
			_ = json.NewEncoder(w).Encode(&commands)
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/applications/app-id/commands/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	handler := NewCommandHandler(session, cfgMgr)

	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if handler.commandManager == nil {
		t.Fatalf("expected command manager to be initialized")
	}

	// Re-run setup to exercise reinit cleanup path.
	if err := handler.SetupCommands(); err != nil {
		t.Fatalf("second setup: %v", err)
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

	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	handler := NewCommandHandler(session, cfgMgr)

	err = handler.SetupCommands()
	if err == nil {
		t.Fatalf("expected setup error when command manager setup fails")
	}
	if !strings.Contains(err.Error(), "failed to setup commands") {
		t.Fatalf("unexpected setup error: %v", err)
	}
	if handler.commandManager != nil {
		t.Fatalf("command manager should be cleared on setup rollback")
	}
}

func TestCommandHandlerSkipsGuildWithoutCommandsFeature(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
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

	handler := NewCommandHandlerForBot(nil, cfgMgr, "main")
	if handler.handlesGuild("guild-1") {
		t.Fatal("expected slash command handler to remain disabled for commands-off guild")
	}
}

func TestCommandHandlerRegistersAdminCatalogOnlyWhenCapabilityEnabled(t *testing.T) {
	t.Parallel()

	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})

	withoutCapability := NewCommandHandlerForBot(nil, cfgMgr, "test-bot")
	withoutCapability.commandManager = core.NewCommandManager(nil, cfgMgr)
	withoutCapability.SetCommandCatalogRegistrars(AdminCommandCatalogRegistrar())
	withoutCapability.SetAdminCommandServices(service.NewServiceManager())
	if err := withoutCapability.registerCommandCatalog(); err != nil {
		t.Fatalf("register admin catalog without capability: %v", err)
	}
	if _, ok := withoutCapability.commandManager.GetRouter().GetRegistry().GetCommand("admin"); ok {
		t.Fatal("expected admin catalog to stay disabled without admin capability")
	}

	withCapability := NewCommandHandler(nil, cfgMgr)
	withCapability.commandManager = core.NewCommandManager(nil, cfgMgr)
	withCapability.SetCommandCatalogRegistrars(AdminCommandCatalogRegistrar())
	withCapability.SetCommandCatalogCapabilities(CommandCatalogCapabilities{Admin: true})
	withCapability.SetAdminCommandServices(service.NewServiceManager())
	if err := withCapability.registerCommandCatalog(); err != nil {
		t.Fatalf("register admin catalog with capability: %v", err)
	}
	if _, ok := withCapability.commandManager.GetRouter().GetRegistry().GetCommand("admin"); !ok {
		t.Fatal("expected admin catalog to register when admin capability is enabled")
	}
}
