package logging

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"context"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/config"
	localdiscord "github.com/small-frappuccino/discordcore/pkg/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

var (
	testMocks sync.Map // map[string]*testHTTPMock
)

type testHTTPMock struct {
	mu          sync.Mutex
	status      int
	body        []byte
	patchStatus int
	patchBody   []byte
	reqs        []*http.Request
	reqBodies   [][]byte
}

func (m *testHTTPMock) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reqs = append(m.reqs, req)
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
	}
	m.reqBodies = append(m.reqBodies, body)

	status := http.StatusOK
	respBody := []byte(`{}`)

	if strings.Contains(req.URL.Path, "/auto-moderation/rules") {
		if req.Method == http.MethodPatch && m.patchStatus != 0 && m.patchStatus != http.StatusOK {
			status = m.patchStatus
			respBody = m.patchBody
		} else {
			status = m.status
			respBody = m.body
		}
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(respBody)),
		Header:     make(http.Header),
	}, nil
}

func resetMockHTTP(t *testing.T) {
	m, ok := testMocks.Load(t.Name())
	if ok {
		mock := m.(*testHTTPMock)
		mock.mu.Lock()
		defer mock.mu.Unlock()
		mock.status = http.StatusOK
		mock.body = []byte(`{}`)
		mock.patchStatus = 0
		mock.patchBody = nil
		mock.reqs = nil
		mock.reqBodies = nil
	} else {
		mock := &testHTTPMock{
			status: http.StatusOK,
			body:   []byte(`{}`),
		}
		testMocks.Store(t.Name(), mock)
	}
}

func getLastResponse(t *testing.T) string {
	m, ok := testMocks.Load(t.Name())
	if !ok {
		return ""
	}
	mock := m.(*testHTTPMock)
	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.reqBodies) == 0 {
		return ""
	}
	return string(mock.reqBodies[len(mock.reqBodies)-1])
}

func setMockStatusAndBody(t *testing.T, status int, body []byte) {
	if m, ok := testMocks.Load(t.Name()); ok {
		mock := m.(*testHTTPMock)
		mock.mu.Lock()
		defer mock.mu.Unlock()
		mock.status = status
		mock.body = body
	}
}

func setMockPatchStatusAndBody(t *testing.T, status int, body []byte) {
	if m, ok := testMocks.Load(t.Name()); ok {
		mock := m.(*testHTTPMock)
		mock.mu.Lock()
		defer mock.mu.Unlock()
		mock.patchStatus = status
		mock.patchBody = body
	}
}

func getMockReqs(t *testing.T) []*http.Request {
	m, ok := testMocks.Load(t.Name())
	if !ok {
		return nil
	}
	mock := m.(*testHTTPMock)
	mock.mu.Lock()
	defer mock.mu.Unlock()
	return mock.reqs
}

func getMockReqBodies(t *testing.T) [][]byte {
	m, ok := testMocks.Load(t.Name())
	if !ok {
		return nil
	}
	mock := m.(*testHTTPMock)
	mock.mu.Lock()
	defer mock.mu.Unlock()
	return mock.reqBodies
}

func newTestContext(t *testing.T, event discord.InteractionEvent, cm config.Provider) *commands.ArikawaContext {
	ctx, _ := commands.NewArikawaContext(event, cm)
	if ctx != nil {
		ctx.Client = api.NewClient("mockToken")
		if m, ok := testMocks.Load(t.Name()); ok {
			customClient := http.Client{Transport: m.(*testHTTPMock)}
			ctx.Client.Client.Client = httpdriver.WrapClient(customClient)
			ctx.WithContext(context.WithValue(ctx.Context(), localdiscord.HTTPTransportContextKey, m.(*testHTTPMock)))
		}
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
	t.Parallel()
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
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
	t.Parallel()
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	cmd := &loggingRootCommand{configManager: cm}

	// Interaction without Options
	ctxEmpty := newTestContext(t, discord.InteractionEvent{
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
	ctxUnknown := newTestContext(t, discord.InteractionEvent{
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
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "11111") {
		t.Errorf("expected response to mention channel, got: %s", getLastResponse(t))
	}
}

func TestLoggingRootCommand_RoleUpdate(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "22222") {
		t.Errorf("expected response to mention channel, got: %s", getLastResponse(t))
	}
}

func TestLoggingRootCommand_Messages(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(t, discord.InteractionEvent{
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
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	// Entry
	ctxEntry := newTestContext(t, discord.InteractionEvent{
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
	ctxExit := newTestContext(t, discord.InteractionEvent{
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
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	ctx := newTestContext(t, discord.InteractionEvent{
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
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	// 1. Success, auto-check native rules
	setMockStatusAndBody(t, http.StatusOK, []byte(`[
		{"id": "111", "guild_id": "12345", "name": "Keywords Rule", "trigger_type": 1, "enabled": true},
		{"id": "222", "guild_id": "12345", "name": "Profile Rule", "trigger_type": 5, "enabled": true}
	]`))

	ctx := newTestContext(t, discord.InteractionEvent{
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
	resetMockHTTP(t)
	setMockStatusAndBody(t, http.StatusOK, []byte(`[]`))

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "Aviso") {
		t.Errorf("expected warning in response when native rules are not active, got: %s", getLastResponse(t))
	}
}

func TestLoggingRootCommand_AutomodWithRule(t *testing.T) {
	t.Parallel()
	resetMockHTTP(t)
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	_ = cm.AddGuildConfig(files.GuildConfig{
		GuildID: "12345",
	})
	cmd := &loggingRootCommand{configManager: cm}

	// 1. Success with Rule ID (rule disabled, no alert action, we update it)
	setMockStatusAndBody(t, http.StatusOK, []byte(`{
		"id": "999",
		"guild_id": "12345",
		"name": "AutoMod Rule",
		"enabled": false,
		"actions": []
	}`))

	ctx := newTestContext(t, discord.InteractionEvent{
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
	if !strings.Contains(getLastResponse(t), "Aviso") {
		t.Errorf("expected warning because rule is disabled, got: %s", getLastResponse(t))
	}

	// 2. Error fetching rule
	resetMockHTTP(t)
	setMockStatusAndBody(t, http.StatusNotFound, []byte(`{"message": "Unknown Rule", "code": 10015}`))

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "Failed to fetch rule") {
		t.Errorf("expected fetch failure message, got: %s", getLastResponse(t))
	}

	// 3. Error modifying rule
	resetMockHTTP(t)
	// Return valid rule on GET
	setMockStatusAndBody(t, http.StatusOK, []byte(`{
		"id": "999",
		"guild_id": "12345",
		"name": "AutoMod Rule",
		"enabled": true,
		"actions": [{"type": 1, "metadata": {"channel_id": "11111"}}]
	}`))
	// Fail on PATCH
	setMockPatchStatusAndBody(t, http.StatusBadRequest, []byte(`{"message": "Bad request modifying rule"}`))

	err = cmd.Handle(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(getLastResponse(t), "Failed to update Discord rule") {
		t.Errorf("expected update failure message, got: %s", getLastResponse(t))
	}
}
