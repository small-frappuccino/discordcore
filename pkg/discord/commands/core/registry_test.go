package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	config := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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
	config := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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
	config := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	_ = config.AddGuildConfig(files.GuildConfig{GuildID: "guild", AllowedRoles: []string{"role"}})
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
			config := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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
	cfg := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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
