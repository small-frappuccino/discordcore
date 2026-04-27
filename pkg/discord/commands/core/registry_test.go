package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type testCommand struct {
	name                string
	requiresGuild       bool
	requiresPermissions bool
	handler             func(*Context) error
	autocomplete        AutocompleteHandler
	ackPolicy           InteractionAckPolicy
}

func (tc testCommand) Name() string        { return tc.name }
func (tc testCommand) Description() string { return tc.name }
func (tc testCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}
func (tc testCommand) Handle(ctx *Context) error {
	if tc.handler != nil {
		return tc.handler(ctx)
	}
	return nil
}
func (tc testCommand) AutocompleteRouteHandler() AutocompleteHandler { return tc.autocomplete }
func (tc testCommand) InteractionAckPolicy() InteractionAckPolicy    { return tc.ackPolicy }
func (tc testCommand) RequiresGuild() bool       { return tc.requiresGuild }
func (tc testCommand) RequiresPermissions() bool { return tc.requiresPermissions }

type testSubCommand struct {
	name string
}

func (ts testSubCommand) Name() string                                   { return ts.name }
func (ts testSubCommand) Description() string                            { return ts.name }
func (ts testSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }
func (ts testSubCommand) Handle(ctx *Context) error                      { return nil }
func (ts testSubCommand) RequiresGuild() bool                            { return false }
func (ts testSubCommand) RequiresPermissions() bool                      { return false }

type responseRecorder struct {
	mu        sync.Mutex
	responses []discordgo.InteractionResponse
}

func (r *responseRecorder) add(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *responseRecorder) all() []discordgo.InteractionResponse {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]discordgo.InteractionResponse, len(r.responses))
	copy(out, r.responses)
	return out
}

func newTestSession(t *testing.T) (*discordgo.Session, *responseRecorder) {
	t.Helper()
	rec := &responseRecorder{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/callback") {
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(r.Body).Decode(&resp)
			rec.add(resp)
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
		t.Fatalf("failed to create session: %v", err)
	}
	return session, rec
}

func buildInteraction(command, guildID, userID string) *discordgo.InteractionCreate {
	data := discordgo.ApplicationCommandInteractionData{
		ID:      "cmd-" + command,
		Name:    command,
		Options: []*discordgo.ApplicationCommandInteractionDataOption{},
	}
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-" + command,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data:    data,
		},
	}
}

func buildInteractionWithOptions(command, guildID, userID string, interactionType discordgo.InteractionType, options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	data := discordgo.ApplicationCommandInteractionData{
		ID:      "cmd-" + command,
		Name:    command,
		Options: options,
	}
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-" + command,
			AppID:   "app",
			Token:   "token",
			Type:    interactionType,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data:    data,
		},
	}
}

func buildComponentInteraction(customID, guildID, userID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-component",
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionMessageComponent,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.MessageComponentInteractionData{
				CustomID: customID,
			},
		},
	}
}

func buildModalInteraction(customID, guildID, userID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-modal",
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionModalSubmit,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ModalSubmitInteractionData{
				CustomID: customID,
			},
		},
	}
}

func TestCommandRegistryRegisterLookup(t *testing.T) {
	registry := NewCommandRegistry()
	first := testCommand{name: "ping"}
	registry.Register(first)

	if got, ok := registry.GetCommand("ping"); !ok || got.Name() != first.Name() {
		t.Fatalf("expected to find command, got ok=%v value=%v", ok, got)
	}

	second := testCommand{name: "ping", requiresGuild: true}
	registry.Register(second)
	if got, ok := registry.GetCommand("ping"); !ok || got.RequiresGuild() != second.requiresGuild {
		t.Fatalf("expected duplicate registration to overwrite, got ok=%v value=%v", ok, got)
	}

	registry.RegisterSubCommand("group", testSubCommand{name: "sub"})
	if _, ok := registry.GetSubCommand("group", "sub"); !ok {
		t.Fatalf("expected subcommand to be registered")
	}
}

func TestHandleSlashCommandUnknownCommand(t *testing.T) {
	session, rec := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)

	router.handleSlashCommand(buildInteraction("missing", "guild", "user"))

	responses := rec.all()
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if !strings.Contains(responses[0].Data.Content, "Command not found") {
		t.Fatalf("unexpected content: %q", responses[0].Data.Content)
	}
	if responses[0].Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral flag to be set")
	}
}

func TestHandleSlashCommandRequiresGuild(t *testing.T) {
	session, rec := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)

	router.RegisterCommand(testCommand{name: "guild", requiresGuild: true, handler: func(*Context) error {
		t.Fatalf("handler should not execute when missing guild")
		return nil
	}})

	router.handleSlashCommand(buildInteraction("guild", "", "user"))

	responses := rec.all()
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if !strings.Contains(responses[0].Data.Content, "only be used in a server") {
		t.Fatalf("unexpected content: %q", responses[0].Data.Content)
	}
}

func TestHandleSlashCommandPermissionDenied(t *testing.T) {
	session, rec := newTestSession(t)
	config := files.NewMemoryConfigManager()
	_ = config.AddGuildConfig(files.GuildConfig{GuildID: "guild", Roles: files.RolesConfig{Allowed: []string{"role"}}})
	router := NewCommandRouter(session, config)

	router.RegisterCommand(testCommand{name: "secure", requiresPermissions: true, handler: func(*Context) error {
		t.Fatalf("handler should not execute when permission denied")
		return nil
	}})

	router.handleSlashCommand(buildInteraction("secure", "guild", "user"))

	responses := rec.all()
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	if !strings.Contains(responses[0].Data.Content, "permission") {
		t.Fatalf("unexpected content: %q", responses[0].Data.Content)
	}
	if responses[0].Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral flag to be set")
	}
}

func TestHandleSlashCommandCommandErrorMapping(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		expectFlag bool
	}{
		{name: "ephemeral", err: NewCommandError("boom", true), expectFlag: true},
		{name: "public", err: NewCommandError("boom", false), expectFlag: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, rec := newTestSession(t)
			config := files.NewMemoryConfigManager()
			router := NewCommandRouter(session, config)

			router.RegisterCommand(testCommand{name: "cmd", handler: func(*Context) error {
				return tt.err
			}})

			router.handleSlashCommand(buildInteraction("cmd", "guild", "user"))

			responses := rec.all()
			if len(responses) != 1 {
				t.Fatalf("expected 1 response, got %d", len(responses))
			}
			gotFlag := responses[0].Data.Flags&discordgo.MessageFlagsEphemeral != 0
			if gotFlag != tt.expectFlag {
				t.Fatalf("ephemeral flag mismatch: got %v want %v", gotFlag, tt.expectFlag)
			}
			if !strings.Contains(responses[0].Data.Content, "boom") {
				t.Fatalf("unexpected content: %q", responses[0].Data.Content)
			}
		})
	}
}

func TestGroupCommandDispatch(t *testing.T) {
	cfg := files.NewMemoryConfigManager()
	checker := NewPermissionChecker(nil, cfg)
	group := NewGroupCommand("group", "", checker)

	handled := false
	group.AddSubCommand(testSubCommand{name: "inner"})
	group.AddSubCommand(testCommand{name: "runner", handler: func(*Context) error {
		handled = true
		return nil
	}})

	interaction := buildInteraction("group", "guild", "user")
	interaction.Interaction.Data = discordgo.ApplicationCommandInteractionData{
		Name: "group",
		Options: []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "runner", Type: discordgo.ApplicationCommandOptionSubCommand},
		},
	}

	session, _ := discordgo.New("Bot test-group")
	_ = session.State.GuildAdd(&discordgo.Guild{ID: "guild"})
	ctx := (&ContextBuilder{session: session, configManager: cfg, checker: checker}).BuildContext(interaction)
	if err := group.Handle(ctx); err != nil {
		t.Fatalf("group handle returned error: %v", err)
	}
	if !handled {
		t.Fatalf("expected subcommand handler to run")
	}
}

func TestHandleComponentRouteUsesExactRouteID(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)

	called := 0
	router.RegisterComponentHandler("runtimecfg:action:edit", ComponentHandlerFunc(func(ctx *Context) error {
		called++
		if ctx.Router() == nil {
			t.Fatalf("expected component context to carry router")
		}
		if ctx.RouteKey.Path != "runtimecfg:action:edit" {
			t.Fatalf("unexpected component route path: %q", ctx.RouteKey.Path)
		}
		return nil
	}))

	router.HandleInteraction(session, buildComponentInteraction("runtimecfg:action:edit|main|ALL|bot_theme|global", "guild", "user"))
	if called != 1 {
		t.Fatalf("expected exact component route to match once, got %d", called)
	}

	router = NewCommandRouter(session, config)
	called = 0
	router.RegisterComponentHandler("runtimecfg:action", ComponentHandlerFunc(func(*Context) error {
		called++
		return nil
	}))
	router.HandleInteraction(session, buildComponentInteraction("runtimecfg:action:edit|main|ALL|bot_theme|global", "guild", "user"))
	if called != 0 {
		t.Fatalf("expected prefix-only component route to stop matching, got %d calls", called)
	}
}

func TestHandleModalRouteUsesExactRouteID(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)

	called := 0
	router.RegisterModalHandler("runtimecfg:modal:edit", ModalHandlerFunc(func(ctx *Context) error {
		called++
		if ctx.Router() == nil {
			t.Fatalf("expected modal context to carry router")
		}
		if ctx.RouteKey.Path != "runtimecfg:modal:edit" {
			t.Fatalf("unexpected modal route path: %q", ctx.RouteKey.Path)
		}
		return nil
	}))

	router.HandleInteraction(session, buildModalInteraction("runtimecfg:modal:edit|main|ALL|bot_theme|global", "guild", "user"))
	if called != 1 {
		t.Fatalf("expected exact modal route to match once, got %d", called)
	}

	router = NewCommandRouter(session, config)
	called = 0
	router.RegisterModalHandler("runtimecfg:modal", ModalHandlerFunc(func(*Context) error {
		called++
		return nil
	}))
	router.HandleInteraction(session, buildModalInteraction("runtimecfg:modal:edit|main|ALL|bot_theme|global", "guild", "user"))
	if called != 0 {
		t.Fatalf("expected prefix-only modal route to stop matching, got %d calls", called)
	}
}

func TestHandleSlashCommandUsesFullRoutePathRegistry(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)
	checker := NewPermissionChecker(session, config)

	metricsGroup := NewGroupCommand("metrics", "", checker)
	serverStats := NewGroupCommand("serverstats", "", checker)

	called := 0
	serverStats.AddSubCommand(testCommand{name: "health", handler: func(ctx *Context) error {
		called++
		if ctx.RouteKey.Kind != InteractionKindSlash {
			t.Fatalf("unexpected route kind: %v", ctx.RouteKey.Kind)
		}
		if ctx.RouteKey.Path != "metrics serverstats health" {
			t.Fatalf("unexpected slash route path: %q", ctx.RouteKey.Path)
		}
		return nil
	}})
	metricsGroup.AddSubCommand(serverStats)
	router.RegisterCommand(metricsGroup)

	interaction := buildInteractionWithOptions(
		"metrics",
		"guild",
		"user",
		discordgo.InteractionApplicationCommand,
		[]*discordgo.ApplicationCommandInteractionDataOption{{
			Name: "serverstats",
			Type: discordgo.ApplicationCommandOptionSubCommandGroup,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{{
				Name: "health",
				Type: discordgo.ApplicationCommandOptionSubCommand,
			}},
		}},
	)

	router.HandleInteraction(session, interaction)
	if called != 1 {
		t.Fatalf("expected nested slash route to dispatch once, got %d", called)
	}
}

func TestHandleAutocompleteUsesFullRoutePathRegistry(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)

	called := 0
	router.RegisterAutocomplete("config set", AutocompleteHandlerFunc(func(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
		called++
		if ctx.RouteKey.Kind != InteractionKindAutocomplete {
			t.Fatalf("unexpected route kind: %v", ctx.RouteKey.Kind)
		}
		if ctx.RouteKey.Path != "config set" {
			t.Fatalf("unexpected autocomplete route path: %q", ctx.RouteKey.Path)
		}
		if ctx.RouteKey.FocusedOption != "key" {
			t.Fatalf("unexpected focused option in route key: %q", ctx.RouteKey.FocusedOption)
		}
		if focusedOption != "key" {
			t.Fatalf("unexpected focused option callback arg: %q", focusedOption)
		}
		return []*discordgo.ApplicationCommandOptionChoice{}, nil
	}))

	interaction := buildInteractionWithOptions(
		"config",
		"guild",
		"user",
		discordgo.InteractionApplicationCommandAutocomplete,
		[]*discordgo.ApplicationCommandInteractionDataOption{{
			Name: "set",
			Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{{
				Name:    "key",
				Type:    discordgo.ApplicationCommandOptionString,
				Focused: true,
			}},
		}},
	)

	router.HandleInteraction(session, interaction)
	if called != 1 {
		t.Fatalf("expected autocomplete route to dispatch once, got %d", called)
	}

	router = NewCommandRouter(session, config)
	called = 0
	router.RegisterAutocomplete("config", AutocompleteHandlerFunc(func(*Context, string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
		called++
		return []*discordgo.ApplicationCommandOptionChoice{}, nil
	}))
	router.HandleInteraction(session, interaction)
	if called != 0 {
		t.Fatalf("expected top-level autocomplete route to stop matching nested path, got %d calls", called)
	}
}

func TestExplicitSlashRouteOverridesCompatibilityRegistration(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)
	checker := NewPermissionChecker(session, config)

	compatibilityCalls := 0
	explicitCalls := 0
	group := NewGroupCommand("config", "", checker)
	group.AddSubCommand(testCommand{name: "runtime", handler: func(*Context) error {
		compatibilityCalls++
		return nil
	}})

	router.RegisterSlashRoute(JoinRoutePath("config", "runtime"), testCommand{name: "runtime", handler: func(ctx *Context) error {
		explicitCalls++
		if ctx.RouteKey.Path != "config runtime" {
			t.Fatalf("unexpected explicit route path: %q", ctx.RouteKey.Path)
		}
		return nil
	}})
	router.RegisterCommand(group)

	interaction := buildInteractionWithOptions(
		"config",
		"guild",
		"user",
		discordgo.InteractionApplicationCommand,
		[]*discordgo.ApplicationCommandInteractionDataOption{{
			Name: "runtime",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		}},
	)

	router.HandleInteraction(session, interaction)
	if explicitCalls != 1 {
		t.Fatalf("expected explicit slash route to dispatch once, got %d", explicitCalls)
	}
	if compatibilityCalls != 0 {
		t.Fatalf("expected compatibility auto-route not to override explicit route, got %d calls", compatibilityCalls)
	}
}

func TestRegisterSlashCommandRefreshesDerivedRouteHandlers(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)
	checker := NewPermissionChecker(session, config)

	firstCalls := 0
	secondCalls := 0
	group := NewGroupCommand("config", "", checker)
	group.AddSubCommand(testCommand{name: "runtime", handler: func(*Context) error {
		firstCalls++
		return nil
	}})
	router.RegisterSlashCommand(group)

	group.AddSubCommand(testCommand{name: "runtime", handler: func(ctx *Context) error {
		secondCalls++
		if ctx.RouteKey.Path != "config runtime" {
			t.Fatalf("unexpected route path after refresh: %q", ctx.RouteKey.Path)
		}
		return nil
	}})
	router.RegisterSlashCommand(group)

	interaction := buildInteractionWithOptions(
		"config",
		"guild",
		"user",
		discordgo.InteractionApplicationCommand,
		[]*discordgo.ApplicationCommandInteractionDataOption{{
			Name: "runtime",
			Type: discordgo.ApplicationCommandOptionSubCommand,
		}},
	)

	router.HandleInteraction(session, interaction)
	if firstCalls != 0 {
		t.Fatalf("expected stale derived route handler to be replaced, got %d calls", firstCalls)
	}
	if secondCalls != 1 {
		t.Fatalf("expected refreshed derived route handler to run once, got %d calls", secondCalls)
	}
}

func TestRegisterSlashCommandDerivesAutocompleteHandler(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)
	checker := NewPermissionChecker(session, config)

	autocompleteCalls := 0
	group := NewGroupCommand("config", "", checker)
	group.AddSubCommand(testCommand{
		name: "set",
		autocomplete: AutocompleteHandlerFunc(func(ctx *Context, focusedOption string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
			autocompleteCalls++
			if ctx.RouteKey.Kind != InteractionKindAutocomplete {
				t.Fatalf("unexpected route kind: %v", ctx.RouteKey.Kind)
			}
			if ctx.RouteKey.Path != "config set" {
				t.Fatalf("unexpected derived autocomplete route path: %q", ctx.RouteKey.Path)
			}
			if focusedOption != "key" {
				t.Fatalf("unexpected focused option callback arg: %q", focusedOption)
			}
			return []*discordgo.ApplicationCommandOptionChoice{}, nil
		}),
	})
	router.RegisterSlashCommand(group)

	interaction := buildInteractionWithOptions(
		"config",
		"guild",
		"user",
		discordgo.InteractionApplicationCommandAutocomplete,
		[]*discordgo.ApplicationCommandInteractionDataOption{{
			Name: "set",
			Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{{
				Name:    "key",
				Type:    discordgo.ApplicationCommandOptionString,
				Focused: true,
			}},
		}},
	)

	router.HandleInteraction(session, interaction)
	if autocompleteCalls != 1 {
		t.Fatalf("expected derived autocomplete handler to dispatch once, got %d", autocompleteCalls)
	}
}

func TestRegisterInteractionRouteDispatchesComponentAndModalHandlers(t *testing.T) {
	session, _ := newTestSession(t)
	config := files.NewMemoryConfigManager()
	router := NewCommandRouter(session, config)

	componentCalls := 0
	modalCalls := 0
	router.RegisterInteractionRoutes(
		InteractionRouteBinding{Path: "runtimecfg:action:edit", Component: ComponentHandlerFunc(func(ctx *Context) error {
			componentCalls++
			if ctx.RouteKey.Kind != InteractionKindComponent {
				t.Fatalf("unexpected component route kind: %v", ctx.RouteKey.Kind)
			}
			if ctx.RouteKey.Path != "runtimecfg:action:edit" {
				t.Fatalf("unexpected component route path: %q", ctx.RouteKey.Path)
			}
			return nil
		})},
		InteractionRouteBinding{Path: "runtimecfg:modal:edit", Modal: ModalHandlerFunc(func(ctx *Context) error {
			modalCalls++
			if ctx.RouteKey.Kind != InteractionKindModal {
				t.Fatalf("unexpected modal route kind: %v", ctx.RouteKey.Kind)
			}
			if ctx.RouteKey.Path != "runtimecfg:modal:edit" {
				t.Fatalf("unexpected modal route path: %q", ctx.RouteKey.Path)
			}
			return nil
		})},
	)

	router.HandleInteraction(session, buildComponentInteraction("runtimecfg:action:edit|main", "guild", "user"))
	router.HandleInteraction(session, buildModalInteraction("runtimecfg:modal:edit|main", "guild", "user"))

	if componentCalls != 1 {
		t.Fatalf("expected component route to dispatch once, got %d", componentCalls)
	}
	if modalCalls != 1 {
		t.Fatalf("expected modal route to dispatch once, got %d", modalCalls)
	}
}
