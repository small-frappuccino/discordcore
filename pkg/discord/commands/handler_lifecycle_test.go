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

func (handlerQOTDServiceStub) Settings(string) (files.QOTDConfig, error) { return files.QOTDConfig{}, nil }
func (handlerQOTDServiceStub) ListQuestions(context.Context, string, string) ([]storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) CreateQuestion(context.Context, string, string, applicationqotd.QuestionMutation) (*storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) DeleteQuestion(context.Context, string, int64) error { return nil }
func (handlerQOTDServiceStub) SetNextQuestion(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) RestoreUsedQuestion(context.Context, string, string, int64) (*storage.QOTDQuestionRecord, error) {
	return nil, nil
}
func (handlerQOTDServiceStub) ResetDeckState(context.Context, string, string) (applicationqotd.ResetDeckResult, error) {
	return applicationqotd.ResetDeckResult{}, nil
}
func (handlerQOTDServiceStub) GetAutomaticQueueState(context.Context, string, string) (applicationqotd.AutomaticQueueState, error) {
	return applicationqotd.AutomaticQueueState{}, nil
}
func (handlerQOTDServiceStub) ImportArchivedQuestions(context.Context, string, string, *discordgo.Session, applicationqotd.ImportArchivedQuestionsParams) (applicationqotd.ImportArchivedQuestionsResult, error) {
	return applicationqotd.ImportArchivedQuestionsResult{}, nil
}
func (handlerQOTDServiceStub) PublishNow(context.Context, string, *discordgo.Session) (*applicationqotd.PublishResult, error) {
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

	cfgMgr := files.NewMemoryConfigManager()
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
	cfgMgr := files.NewMemoryConfigManager()
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:       "guild-1",
				BotInstanceID: "main",
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

	handler := NewCommandHandlerForBot(nil, cfgMgr, "main", "main")
	if handler.handlesGuild("guild-1") {
		t.Fatal("expected slash command handler to remain disabled for commands-off guild")
	}
}

func TestCommandHandlerAllowsDormantGuildBootstrapRoutes(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewMemoryConfigManager()
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{
			GuildID:       "guild-1",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
			Features: files.FeatureToggles{
				Services: files.FeatureServiceToggles{
					Commands: boolPtr(false),
				},
			},
		}}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	handler := NewCommandHandlerForBot(nil, cfgMgr, "main", "main")
	if !handler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config commands_enabled"}) {
		t.Fatal("expected dormant guild bootstrap command route to remain enabled")
	}
	if !handler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config smoke_test"}) {
		t.Fatal("expected dormant guild smoke test route to remain enabled")
	}
	if handler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config qotd_schedule"}) {
		t.Fatal("expected dormant guild qotd bootstrap route to move to the qotd bot instance")
	}
	if handler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "partner list"}) {
		t.Fatal("expected non-bootstrap route to remain disabled for dormant guild")
	}

	companionHandler := NewCommandHandlerForBot(nil, cfgMgr, "companion", "main")
	if !companionHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config qotd_schedule"}) {
		t.Fatal("expected dormant guild qotd bootstrap route to remain enabled on the qotd bot instance")
	}
	if companionHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config commands_enabled"}) {
		t.Fatal("expected base bootstrap route to stay off the qotd bot instance")
	}
}

func TestCommandHandlerFiltersRoutesByDomainBinding(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewMemoryConfigManager()
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{
			GuildID:       "guild-1",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
			Features: files.FeatureToggles{
				Services: files.FeatureServiceToggles{
					Commands: boolPtr(true),
				},
			},
		}}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	mainHandler := NewCommandHandlerForBot(nil, cfgMgr, "main", "main")
	mainHandler.SetQOTDService(handlerQOTDServiceStub{})
	mainHandler.commandManager = core.NewCommandManager(nil, cfgMgr)
	if err := mainHandler.registerCommandCatalog(); err != nil {
		t.Fatalf("register main command catalog: %v", err)
	}

	companionHandler := NewCommandHandlerForBot(nil, cfgMgr, "companion", "main")
	companionHandler.SetQOTDService(handlerQOTDServiceStub{})
	companionHandler.commandManager = core.NewCommandManager(nil, cfgMgr)
	if err := companionHandler.registerCommandCatalog(); err != nil {
		t.Fatalf("register companion command catalog: %v", err)
	}

	if !mainHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "partner list"}) {
		t.Fatal("expected base-domain slash route to stay on main")
	}
	if mainHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "qotd publish"}) {
		t.Fatal("expected qotd slash route to move off main")
	}
	if mainHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindAutocomplete, Path: "config qotd_channel"}) {
		t.Fatal("expected qotd autocomplete route to move off main")
	}
	if mainHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindComponent, Path: "qotd:questions:list:next"}) {
		t.Fatal("expected qotd component route to move off main")
	}

	if companionHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "partner list"}) {
		t.Fatal("expected base-domain slash route to stay off companion")
	}
	if !companionHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "qotd publish"}) {
		t.Fatal("expected qotd slash route to move to companion")
	}
	if !companionHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindAutocomplete, Path: "config qotd_channel"}) {
		t.Fatal("expected qotd autocomplete route to move to companion")
	}
	if !companionHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Kind: core.InteractionKindComponent, Path: "qotd:questions:list:next"}) {
		t.Fatal("expected qotd component route to move to companion")
	}
}

func TestCommandHandlerRegistersOnlySupportedDomains(t *testing.T) {
	t.Parallel()

	cfgMgr := files.NewMemoryConfigManager()
	handler := NewCommandHandler(nil, cfgMgr)
	handler.SetSupportedDomains(files.BotDomainQOTD)
	handler.SetQOTDService(handlerQOTDServiceStub{})
	handler.commandManager = core.NewCommandManager(nil, cfgMgr)

	if err := handler.registerCommandCatalog(); err != nil {
		t.Fatalf("registerCommandCatalog() failed: %v", err)
	}

	router := handler.commandManager.GetRouter()
	configCommand, ok := router.GetRegistry().GetCommand("config")
	if !ok {
		t.Fatal("expected /config to be registered for qotd domain")
	}
	options := configCommand.Options()
	got := make(map[string]struct{}, len(options))
	for _, option := range options {
		if option != nil {
			got[option.Name] = struct{}{}
		}
	}
	for _, name := range []string{"qotd_enabled", "qotd_channel", "qotd_schedule"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("expected qotd-only config fragment to include %q, got %#v", name, got)
		}
	}
	for _, name := range []string{"commands_enabled", "command_channel", "allowed_role_add"} {
		if _, ok := got[name]; ok {
			t.Fatalf("expected qotd-only config fragment to omit %q, got %#v", name, got)
		}
	}
	if _, ok := router.GetRegistry().GetCommand("ping"); ok {
		t.Fatal("expected qotd-only catalog to omit /ping")
	}
	if _, ok := router.GetRegistry().GetCommand("partner"); ok {
		t.Fatal("expected qotd-only catalog to omit /partner")
	}
	if _, ok := router.GetRegistry().GetCommand("qotd"); !ok {
		t.Fatal("expected qotd-only catalog to include /qotd")
	}
	if got := router.InteractionRouteDomain(core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config qotd_channel"}); got != files.BotDomainQOTD {
		t.Fatalf("expected qotd config route domain, got %q", got)
	}
	if got := router.InteractionRouteDomain(core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "qotd publish"}); got != files.BotDomainQOTD {
		t.Fatalf("expected qotd publish route domain, got %q", got)
	}
}

func TestCommandHandlerAppliesInjectedCatalogRegistrars(t *testing.T) {
	t.Parallel()

	cfgMgr := files.NewMemoryConfigManager()
	handler := NewCommandHandler(nil, cfgMgr)
	handler.SetSupportedDomains(files.BotDomainQOTD)
	handler.commandManager = core.NewCommandManager(nil, cfgMgr)

	called := make([]string, 0, 2)
	handler.SetCommandCatalogRegistrars(
		CommandCatalogRegistrar{
			Domain: "",
			Register: func(*CommandHandler, *core.CommandRouter) {
				called = append(called, "default")
			},
		},
		CommandCatalogRegistrar{
			Domain: files.BotDomainQOTD,
			Register: func(*CommandHandler, *core.CommandRouter) {
				called = append(called, files.BotDomainQOTD)
			},
		},
		CommandCatalogRegistrar{
			Domain: "",
			RequiredCapabilities: CommandCatalogCapabilities{
				Admin: true,
			},
			Register: func(*CommandHandler, *core.CommandRouter) {
				called = append(called, "admin")
			},
		},
	)

	if err := handler.registerCommandCatalog(); err != nil {
		t.Fatalf("registerCommandCatalog() failed: %v", err)
	}
	if len(called) != 1 || called[0] != files.BotDomainQOTD {
		t.Fatalf("expected only qotd registrar to run, got %#v", called)
	}
}

func TestCommandHandlerRegistersAdminCatalogOnlyWhenCapabilityEnabled(t *testing.T) {
	t.Parallel()

	cfgMgr := files.NewMemoryConfigManager()

	withoutCapability := NewCommandHandler(nil, cfgMgr)
	withoutCapability.commandManager = core.NewCommandManager(nil, cfgMgr)
	withoutCapability.SetCommandCatalogRegistrars(AdminCommandCatalogRegistrar())
	withoutCapability.SetAdminCommandServices(service.NewServiceManager(nil), nil, nil)
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
	withCapability.SetAdminCommandServices(service.NewServiceManager(nil), nil, nil)
	if err := withCapability.registerCommandCatalog(); err != nil {
		t.Fatalf("register admin catalog with capability: %v", err)
	}
	if _, ok := withCapability.commandManager.GetRouter().GetRegistry().GetCommand("admin"); !ok {
		t.Fatal("expected admin catalog to register when admin capability is enabled")
	}
}
