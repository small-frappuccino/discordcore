package logging

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

var (
	mockHTTPStatus      = http.StatusOK
	mockHTTPBody        = []byte(`{}`)
	mockHTTPPatchStatus = http.StatusOK
	mockHTTPPatchBody   = []byte(`{}`)
	mockHTTPReqs        []*http.Request
	mockHTTPReqBodies   [][]byte
	mockHTTPMu          sync.Mutex
)

type mockRoundTripper struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func init() {
	http.DefaultTransport = &mockRoundTripper{
		roundTrip: func(req *http.Request) (*http.Response, error) {
			mockHTTPMu.Lock()
			defer mockHTTPMu.Unlock()
			mockHTTPReqs = append(mockHTTPReqs, req)
			var body []byte
			if req.Body != nil {
				body, _ = io.ReadAll(req.Body)
			}
			mockHTTPReqBodies = append(mockHTTPReqBodies, body)

			// Default behavior for non-AutoMod requests (e.g. interaction responses/callbacks) is 200 OK
			status := http.StatusOK
			respBody := []byte(`{}`)

			// Only intercept and inject mock responses on AutoMod rules API calls
			if strings.Contains(req.URL.Path, "/auto-moderation/rules") {
				if req.Method == http.MethodPatch && mockHTTPPatchStatus != http.StatusOK {
					status = mockHTTPPatchStatus
					respBody = mockHTTPPatchBody
				} else {
					status = mockHTTPStatus
					respBody = mockHTTPBody
				}
			}

			return &http.Response{
				StatusCode: status,
				Body:       io.NopCloser(bytes.NewReader(respBody)),
				Header:     make(http.Header),
			}, nil
		},
	}
}

func resetMockHTTP() {
	mockHTTPMu.Lock()
	defer mockHTTPMu.Unlock()
	mockHTTPStatus = http.StatusOK
	mockHTTPBody = []byte(`{}`)
	mockHTTPPatchStatus = http.StatusOK
	mockHTTPPatchBody = []byte(`{}`)
	mockHTTPReqs = nil
	mockHTTPReqBodies = nil
}

func getLastResponse() string {
	mockHTTPMu.Lock()
	defer mockHTTPMu.Unlock()
	if len(mockHTTPReqBodies) == 0 {
		return ""
	}
	return string(mockHTTPReqBodies[len(mockHTTPReqBodies)-1])
}

func newTestContext(event discord.InteractionEvent, cm *files.ConfigManager) *commands.ArikawaContext {
	ctx, _ := commands.NewArikawaContext(event, cm)
	if ctx != nil {
		ctx.Client = api.NewClient("mockToken")
	}
	return ctx
}

type spyRouter struct {
	registered commands.ArikawaCommand
}

func (s *spyRouter) Register(cmd commands.ArikawaCommand) {
	s.registered = cmd
}

func (s *spyRouter) RegisterComponent(customIDPrefix string, handler commands.ComponentHandler) {}

func TestLoggingCommands_RegisterCommands(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	lc := NewLoggingCommands(cm)
	sr := &spyRouter{}
	lc.RegisterCommands(sr)

	if sr.registered == nil {
		t.Fatal("expected command to be registered")
	}
	if sr.registered.Name() != "logging" {
		t.Errorf("expected command name 'logging', got %s", sr.registered.Name())
	}
	if sr.registered.Description() == "" {
		t.Error("expected description to be non-empty")
	}
	if !sr.registered.RequiresGuild() || !sr.registered.RequiresPermissions() {
		t.Error("expected requires guild/perms to be true")
	}
	if permProv, ok := sr.registered.(commands.DefaultMemberPermissionsProvider); ok {
		if permProv.DefaultMemberPermissions() != discord.PermissionManageGuild {
			t.Error("unexpected default member permissions")
		}
	} else {
		t.Error("expected registered command to implement DefaultMemberPermissionsProvider")
	}
	if len(sr.registered.Options()) == 0 {
		t.Error("expected options to be configured")
	}

	// Nil routing safety
	lc.RegisterCommands(nil)
	lcNil := NewLoggingCommands(nil)
	lcNil.RegisterCommands(sr)
}

func TestLoggingRootCommand_HandleSafety(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	cmd := &loggingRootCommand{configManager: cm}

	// Interaction without Options
	ctxEmpty := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: nil,
		},
	}, cm)
	err := cmd.Handle(ctxEmpty)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Unknown subcommand safety
	ctxUnknown := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{Name: "unknown", Type: discord.SubcommandOptionType},
			},
		},
	}, cm)
	err = cmd.Handle(ctxUnknown)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoggingRootCommand_Avatar(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "avatar",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"11111"`)},
					},
				},
			},
		},
	}, cm)

	if ctx == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := cm.GuildConfig("12345")
	if cfg.Channels.AvatarLogging != "11111" {
		t.Errorf("expected AvatarLogging channel to be 11111, got %s", cfg.Channels.AvatarLogging)
	}
	if !strings.Contains(getLastResponse(), "11111") {
		t.Errorf("expected response to mention channel, got: %s", getLastResponse())
	}
}

func TestLoggingRootCommand_RoleUpdate(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "role_update",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"22222"`)},
					},
				},
			},
		},
	}, cm)

	if ctx == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := cm.GuildConfig("12345")
	if cfg.Channels.RoleUpdate != "22222" {
		t.Errorf("expected RoleUpdate channel to be 22222, got %s", cfg.Channels.RoleUpdate)
	}
	if !strings.Contains(getLastResponse(), "22222") {
		t.Errorf("expected response to mention channel, got: %s", getLastResponse())
	}
}

func TestLoggingRootCommand_Messages(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "messages",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"33333"`)},
					},
				},
			},
		},
	}, cm)

	if ctx == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := cm.GuildConfig("12345")
	if cfg.Channels.MessageEdit != "33333" || cfg.Channels.MessageDelete != "33333" {
		t.Errorf("expected message logging channels to be 33333, got edit=%s, delete=%s", cfg.Channels.MessageEdit, cfg.Channels.MessageDelete)
	}
}

func TestLoggingRootCommand_EntryExit(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	// Entry
	ctxEntry := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "entry",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"44444"`)},
					},
				},
			},
		},
	}, cm)

	if ctxEntry == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err := cmd.Handle(ctxEntry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := cm.GuildConfig("12345")
	if cfg.Channels.MemberJoin != "44444" {
		t.Errorf("expected MemberJoin channel to be 44444, got %s", cfg.Channels.MemberJoin)
	}

	// Exit
	ctxExit := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "exit",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"55555"`)},
					},
				},
			},
		},
	}, cm)

	if ctxExit == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err = cmd.Handle(ctxExit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg = cm.GuildConfig("12345")
	if cfg.Channels.MemberLeave != "55555" {
		t.Errorf("expected MemberLeave channel to be 55555, got %s", cfg.Channels.MemberLeave)
	}
}

func TestLoggingRootCommand_Warnings(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "warnings",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"66666"`)},
						{Name: "log_warning_from_other_bots", Type: discord.StringOptionType, Value: []byte(`"all_bots"`)},
					},
				},
			},
		},
	}, cm)

	if ctx == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := cm.GuildConfig("12345")
	if cfg.Channels.ModerationCase != "66666" || cfg.LogModerationScope != "all_bots" {
		t.Errorf("expected moderation config channel=66666, scope=all_bots, got channel=%s, scope=%s", cfg.Channels.ModerationCase, cfg.LogModerationScope)
	}
}

func TestLoggingRootCommand_AutomodNoRule(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	// 1. Success, auto-check native rules
	mockHTTPMu.Lock()
	mockHTTPBody = []byte(`[
		{"id": "111", "guild_id": "12345", "name": "Keywords Rule", "trigger_type": 1, "enabled": true},
		{"id": "222", "guild_id": "12345", "name": "Profile Rule", "trigger_type": 5, "enabled": true}
	]`)
	mockHTTPMu.Unlock()

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "automod",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"77777"`)},
					},
				},
			},
		},
	}, cm)

	if ctx == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg := cm.GuildConfig("12345")
	if cfg.Channels.AutomodAction != "77777" {
		t.Errorf("expected AutomodAction channel to be 77777, got %s", cfg.Channels.AutomodAction)
	}

	// 2. Warning case (rules disabled/missing)
	resetMockHTTP()
	mockHTTPMu.Lock()
	mockHTTPBody = []byte(`[]`)
	mockHTTPMu.Unlock()

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "Aviso") {
		t.Errorf("expected warning in response when native rules are not active, got: %s", getLastResponse())
	}
}

func TestLoggingRootCommand_AutomodWithRule(t *testing.T) {
	resetMockHTTP()
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	// 1. Success with Rule ID (rule disabled, no alert action, we update it)
	mockHTTPMu.Lock()
	mockHTTPBody = []byte(`{
		"id": "999",
		"guild_id": "12345",
		"name": "AutoMod Rule",
		"enabled": false,
		"actions": []
	}`)
	mockHTTPMu.Unlock()

	ctx := newTestContext(discord.InteractionEvent{
		GuildID: 12345,
		Member:  &discord.Member{User: discord.User{ID: 999}},
		Data: &discord.CommandInteraction{
			Options: []discord.CommandInteractionOption{
				{
					Name: "automod",
					Type: discord.SubcommandOptionType,
					Options: []discord.CommandInteractionOption{
						{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"77777"`)},
						{Name: "rule_id", Type: discord.StringOptionType, Value: []byte(`"999"`)},
					},
				},
			},
		},
	}, cm)

	if ctx == nil {
		t.Fatal("expected test context to be non-nil")
	}

	err := cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "Aviso") {
		t.Errorf("expected warning because rule is disabled, got: %s", getLastResponse())
	}

	// 2. Error fetching rule
	resetMockHTTP()
	mockHTTPMu.Lock()
	mockHTTPStatus = http.StatusNotFound
	mockHTTPBody = []byte(`{"message": "Unknown Rule", "code": 10015}`)
	mockHTTPMu.Unlock()

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "Failed to fetch rule") {
		t.Errorf("expected fetch failure message, got: %s", getLastResponse())
	}

	// 3. Error modifying rule
	resetMockHTTP()
	mockHTTPMu.Lock()
	// Return valid rule on GET
	mockHTTPBody = []byte(`{
		"id": "999",
		"guild_id": "12345",
		"name": "AutoMod Rule",
		"enabled": true,
		"actions": [{"type": 1, "metadata": {"channel_id": "11111"}}]
	}`)
	// Fail on PATCH
	mockHTTPPatchStatus = http.StatusBadRequest
	mockHTTPPatchBody = []byte(`{"message": "Bad request modifying rule"}`)
	mockHTTPMu.Unlock()

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(), "Failed to update Discord rule") {
		t.Errorf("expected update failure message, got: %s", getLastResponse())
	}
}
