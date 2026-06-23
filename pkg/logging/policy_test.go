package logging

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Mock transport to avoid live API calls
type mockTransport struct{}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func TestFormatUserLabel(t *testing.T) {
	tests := []struct {
		username string
		userID   string
		expected string
	}{
		{"", "", "Unknown"},
		{"alice", "", "**alice**"},
		{"", "12345", "<@12345> (`12345`)"},
		{"alice", "12345", "**alice** (<@12345>, `12345`)"},
		{" alice ", " 12345 ", "**alice** (<@12345>, `12345`)"},
	}

	for _, tt := range tests {
		got := FormatUserLabel(tt.username, tt.userID)
		if got != tt.expected {
			t.Errorf("FormatUserLabel(%q, %q) = %q; expected %q", tt.username, tt.userID, got, tt.expected)
		}
	}
}

func TestFormatUserRef(t *testing.T) {
	got := FormatUserRef("123")
	expected := "<@123> (`123`)"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatChannelLabel(t *testing.T) {
	if FormatChannelLabel("") != "Unknown" {
		t.Errorf("expected Unknown for empty channel ID")
	}
	got := FormatChannelLabel("123")
	expected := "<#123>, `123`"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatRoleLabel(t *testing.T) {
	if FormatRoleLabel("123", "") != "<@&123> (`123`)" {
		t.Errorf("unexpected role reference output")
	}
	if FormatRoleLabel("", "admin") != "`admin`" {
		t.Errorf("unexpected role name output")
	}
	if FormatRoleLabel("", "") != "Unknown" {
		t.Errorf("expected Unknown for empty role")
	}
}

func TestFormatDurationFull(t *testing.T) {
	if FormatDurationFull(-1) != "0 seconds" {
		t.Errorf("expected 0 seconds for negative duration")
	}
	d := 24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second
	got := FormatDurationFull(d)
	expected := "1 days 3 hours 4 minutes 5 seconds"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatDurationSmart(t *testing.T) {
	if FormatDurationSmart(-1) != "" {
		t.Errorf("expected empty string for negative duration")
	}
	d1 := 24*time.Hour + 1*time.Hour + 1*time.Minute + 1*time.Second
	if FormatDurationSmart(d1) != "1 day 1 hour 1 minute 1 second" {
		t.Errorf("unexpected output: %q", FormatDurationSmart(d1))
	}
	d2 := 48*time.Hour + 2*time.Hour + 2*time.Minute + 2*time.Second
	if FormatDurationSmart(d2) != "2 days 2 hours 2 minutes 2 seconds" {
		t.Errorf("unexpected output: %q", FormatDurationSmart(d2))
	}
}

func TestFormatDuration(t *testing.T) {
	if FormatDuration(0) != "`            `" {
		t.Errorf("unexpected zero duration output")
	}
	// Years
	if FormatDuration(400*24*time.Hour) != "1 year, 35 days" {
		t.Errorf("unexpected year output: %q", FormatDuration(400*24*time.Hour))
	}
	if FormatDuration(800*24*time.Hour) != "2 years, 70 days" {
		t.Errorf("unexpected years output")
	}
	// Months
	if FormatDuration(45*24*time.Hour) != "1 month, 15 days" {
		t.Errorf("unexpected month output")
	}
	if FormatDuration(75*24*time.Hour) != "2 months, 15 days" {
		t.Errorf("unexpected months output")
	}
	// Days
	if FormatDuration(1*24*time.Hour+3*time.Hour) != "1 day, 3 hours" {
		t.Errorf("unexpected day output")
	}
	if FormatDuration(2*24*time.Hour+4*time.Hour) != "2 days, 4 hours" {
		t.Errorf("unexpected days output")
	}
	// Hours
	if FormatDuration(1*time.Hour+3*time.Minute) != "1 hour, 3 minutes" {
		t.Errorf("unexpected hour output")
	}
	if FormatDuration(2*time.Hour+4*time.Minute) != "2 hours, 4 minutes" {
		t.Errorf("unexpected hours output")
	}
	// Minutes
	if FormatDuration(1*time.Minute) != "1 minutes" {
		t.Errorf("unexpected minute output")
	}
	if FormatDuration(5*time.Minute) != "5 minutes" {
		t.Errorf("unexpected minutes output")
	}
	// Less than 1 minute
	if FormatDuration(30*time.Second) != "Less than 1 minute" {
		t.Errorf("unexpected seconds output")
	}
}

func TestTruncateString(t *testing.T) {
	if TruncateString("", 10) != "*empty message*" {
		t.Errorf("unexpected empty message output")
	}
	if TruncateString("hello", 10) != "hello" {
		t.Errorf("unexpected no truncate output")
	}
	if TruncateString("hello world", 8) != "hello..." {
		t.Errorf("unexpected truncate output: %q", TruncateString("hello world", 8))
	}
}

func TestLogEventCapabilities(t *testing.T) {
	caps := LogEventCapabilities()
	if len(caps) != len(logEventCapabilities) {
		t.Errorf("expected same length")
	}
}

func TestResolveLogChannel(t *testing.T) {
	// nil inputs
	if ResolveLogChannel(LogEventAvatarChange, "111", nil) != "" {
		t.Errorf("expected empty string for nil config manager")
	}
	if ResolveLogChannel(LogEventAvatarChange, "", &files.ConfigManager{}) != "" {
		t.Errorf("expected empty string for empty guild ID")
	}

	store := &files.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111",
				Channels: files.ChannelsConfig{
					AvatarLogging:  "avatar_ch",
					RoleUpdate:     "role_ch",
					MemberJoin:     "join_ch",
					MemberLeave:    "leave_ch",
					MessageEdit:    "edit_ch",
					MessageDelete:  "delete_ch",
					AutomodAction:  "automod_ch",
					ModerationCase: "mod_ch",
					CleanAction:    "clean_ch",
				},
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()

	// Test resolutions for all events
	eventsToChannels := map[LogEventType]string{
		LogEventAvatarChange:    "avatar_ch",
		LogEventRoleChange:      "role_ch",
		LogEventMemberJoin:      "join_ch",
		LogEventMemberLeave:     "leave_ch",
		LogEventMessageEdit:     "edit_ch",
		LogEventMessageDelete:   "delete_ch",
		LogEventAutomodAction:   "automod_ch",
		LogEventModerationCase:  "mod_ch",
		LogEventCleanAction:     "clean_ch",
		LogEventType("unknown"): "",
	}

	for evt, expected := range eventsToChannels {
		got := ResolveLogChannel(evt, "111", mgr)
		if got != expected {
			t.Errorf("ResolveLogChannel(%s) = %q; expected %q", evt, got, expected)
		}
	}
}

func TestCheckFeatureEnabled_Errors(t *testing.T) {
	// Unknown event
	dec := CheckFeatureEnabled(nil, LogEventType("unknown"), "111")
	if dec.Enabled || dec.Reason != EmitReasonUnknownEvent {
		t.Errorf("expected EmitReasonUnknownEvent")
	}

	// nil config manager
	dec = CheckFeatureEnabled(nil, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonConfigManagerUnavailable {
		t.Errorf("expected EmitReasonConfigManagerUnavailable")
	}

	// config unavailable (nil config store load)
	mgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	dec = CheckFeatureEnabled(mgr, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonConfigUnavailable {
		t.Errorf("expected EmitReasonConfigUnavailable")
	}

	// guild config missing
	_ = mgr.LoadConfig()
	dec = CheckFeatureEnabled(mgr, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonGuildConfigMissing {
		t.Errorf("expected EmitReasonGuildConfigMissing")
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestCheckFeatureEnabled_Toggles(t *testing.T) {
	tests := []struct {
		eventType LogEventType
		rc        files.RuntimeConfig
		expected  EmitReason
	}{
		{LogEventAvatarChange, files.RuntimeConfig{DisableUserLogs: true}, EmitReasonRuntimeDisableUserLogs},
		{LogEventRoleChange, files.RuntimeConfig{DisableUserLogs: true}, EmitReasonRuntimeDisableUserLogs},
		{LogEventMemberJoin, files.RuntimeConfig{DisableEntryExitLogs: true}, EmitReasonRuntimeDisableEntryExitLogs},
		{LogEventMemberLeave, files.RuntimeConfig{DisableEntryExitLogs: true}, EmitReasonRuntimeDisableEntryExitLogs},
		{LogEventMessageProcess, files.RuntimeConfig{DisableMessageLogs: true}, EmitReasonRuntimeDisableMessageLogs},
		{LogEventMessageEdit, files.RuntimeConfig{DisableMessageLogs: true}, EmitReasonRuntimeDisableMessageLogs},
		{LogEventMessageDelete, files.RuntimeConfig{DisableMessageLogs: true}, EmitReasonRuntimeDisableMessageLogs},
		{LogEventReactionMetric, files.RuntimeConfig{DisableReactionLogs: true}, EmitReasonRuntimeDisableReactionLogs},
		{LogEventCleanAction, files.RuntimeConfig{DisableCleanLog: true}, EmitReasonRuntimeDisableCleanLog},
		{LogEventModerationCase, files.RuntimeConfig{ModerationLogging: boolPtr(false)}, EmitReasonRuntimeModerationLoggingOff},
	}

	for _, tt := range tests {
		store := &files.MemoryConfigStore{}
		_ = store.Save(&files.BotConfig{
			Guilds: []files.GuildConfig{
				{
					GuildID: "111",
					Channels: files.ChannelsConfig{
						AvatarLogging:  "ch",
						RoleUpdate:     "ch",
						MemberJoin:     "ch",
						MemberLeave:    "ch",
						MessageEdit:    "ch",
						MessageDelete:  "ch",
						AutomodAction:  "ch",
						ModerationCase: "ch",
						CleanAction:    "ch",
					},
					RuntimeConfig: tt.rc,
				},
			},
		})
		mgr := files.NewConfigManagerWithStore(store, nil)
		_ = mgr.LoadConfig()

		dec := CheckFeatureEnabled(mgr, tt.eventType, "111")
		if dec.Enabled || dec.Reason != tt.expected {
			t.Errorf("event %s: expected decision gated off by %s, got enabled=%t reason=%s", tt.eventType, tt.expected, dec.Enabled, dec.Reason)
		}
	}
}

func TestCheckFeatureEnabled_NoChannelConfigured(t *testing.T) {
	store := &files.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111", // all channels empty
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()

	dec := CheckFeatureEnabled(mgr, LogEventAvatarChange, "111")
	if dec.Enabled || dec.Reason != EmitReasonNoChannelConfigured {
		t.Errorf("expected EmitReasonNoChannelConfigured, got %v", dec.Reason)
	}
}

func TestValidateLogCapability(t *testing.T) {
	// Test not enabled decision
	dec := EmitDecision{Enabled: false, Reason: EmitReasonUnknownEvent}
	reason, _, ok := ValidateLogCapability(nil, 0, dec, "111", nil)
	if ok || reason != EmitReasonUnknownEvent {
		t.Errorf("expected false and reason unknown_event")
	}

	// Test validate resolved log channel: when ValidateChannelPerms is false
	decEnabled := EmitDecision{
		Enabled: true,
		Capability: LogEventCapability{
			RequiresChannel:      true,
			ValidateChannelPerms: false,
		},
		ChannelID: "123",
	}
	reason, _, ok = ValidateLogCapability(nil, 0, decEnabled, "111", nil)
	if !ok || reason != EmitReasonEnabled {
		t.Errorf("expected validation success when ValidateChannelPerms is false")
	}

	// Test validate resolved log channel: exclusive moderation channel conflict
	store := &files.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111",
				Channels: files.ChannelsConfig{
					Commands:       "123", // conflicts with channelID 123
					ModerationCase: "123",
				},
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()

	decExcl := EmitDecision{
		Enabled: true,
		Capability: LogEventCapability{
			RequiresChannel:            true,
			ValidateChannelPerms:       true,
			RequireExclusiveModeration: true,
		},
		ChannelID: "123",
	}
	reason, _, ok = ValidateLogCapability(nil, 0, decExcl, "111", mgr)
	if ok || reason != EmitReasonChannelInvalid {
		t.Errorf("expected EmitReasonChannelInvalid for shared moderation channel, got reason=%s ok=%t", reason, ok)
	}

	// Test validate resolved log channel: check intent requirement validation
	decIntent := EmitDecision{
		Enabled: true,
		Capability: LogEventCapability{
			RequiresChannel:      true,
			ValidateChannelPerms: false,
			RequiredIntentsMask:  gateway.IntentGuildMembers,
		},
		ChannelID: "123",
	}
	reason, mask, ok := ValidateLogCapability(nil, 0, decIntent, "111", mgr)
	if ok || reason != EmitReasonMissingIntent || mask != gateway.IntentGuildMembers {
		t.Errorf("expected missing intent, got reason=%s mask=%v ok=%t", reason, mask, ok)
	}
}

func TestValidateModerationLogChannel(t *testing.T) {
	// nil state
	if err := ValidateModerationLogChannel(nil, "111", "222"); err == nil || !strings.Contains(err.Error(), "state is nil") {
		t.Errorf("expected error for nil state")
	}

	// missing guildID
	st := state.New("token")
	st.Client.Client.Client = httpdriver.WrapClient(http.Client{Transport: &mockTransport{}})
	if err := ValidateModerationLogChannel(st, "", "222"); err == nil || !strings.Contains(err.Error(), "missing guildID") {
		t.Errorf("expected error for missing guildID")
	}

	// invalid channel/guild ID
	if err := ValidateModerationLogChannel(st, "abc", "222"); err == nil || !strings.Contains(err.Error(), "invalid guild ID") {
		t.Errorf("expected error for invalid guild ID")
	}
	if err := ValidateModerationLogChannel(st, "111", "abc"); err == nil || !strings.Contains(err.Error(), "invalid channel ID") {
		t.Errorf("expected error for invalid channel ID")
	}

	// channel lookup failed
	if err := ValidateModerationLogChannel(st, "111", "222"); err == nil || !strings.Contains(err.Error(), "channel lookup failed") {
		t.Errorf("expected error for channel lookup failed")
	}

	// Guild mismatch
	meID := discord.UserID(123)
	guildID := discord.GuildID(111)
	channelID := discord.ChannelID(222)

	_ = st.Cabinet.ChannelStore.ChannelSet(&discord.Channel{
		ID:      channelID,
		GuildID: discord.GuildID(999), // mismatch
		Type:    discord.GuildText,
	}, false)
	if err := ValidateModerationLogChannel(st, "111", "222"); err == nil || !strings.Contains(err.Error(), "channel guild mismatch") {
		t.Errorf("expected error for guild mismatch, got %v", err)
	}

	// Invalid channel type
	_ = st.Cabinet.ChannelStore.ChannelSet(&discord.Channel{
		ID:      channelID,
		GuildID: guildID,
		Type:    discord.GuildVoice, // invalid
	}, false)
	if err := ValidateModerationLogChannel(st, "111", "222"); err == nil || !strings.Contains(err.Error(), "channel is not a guild text channel") {
		t.Errorf("expected error for invalid channel type, got %v", err)
	}

	// Bot identity not available
	_ = st.Cabinet.ChannelStore.ChannelSet(&discord.Channel{
		ID:      channelID,
		GuildID: guildID,
		Type:    discord.GuildText,
	}, false)
	if err := ValidateModerationLogChannel(st, "111", "222"); err == nil || !strings.Contains(err.Error(), "bot identity not available") {
		t.Errorf("expected error for missing bot identity, got %v", err)
	}

	// Permission checks - set bot me ID
	_ = st.Cabinet.MeStore.MyselfSet(discord.User{
		ID: meID,
	}, false)
	// Without guild in store, permission check fails
	if err := ValidateModerationLogChannel(st, "111", "222"); err == nil || !strings.Contains(err.Error(), "permission check failed") {
		t.Errorf("expected error for missing guild store entry (permission check failed)")
	}

	// Setup guild where bot is owner (giving it all permissions)
	_ = st.Cabinet.GuildStore.GuildSet(&discord.Guild{
		ID:      guildID,
		OwnerID: meID,
	}, false)
	// Setting member is also required by state.Permissions under some calculation branches
	_ = st.Cabinet.MemberStore.MemberSet(guildID, &discord.Member{
		User: discord.User{ID: meID},
	}, false)
	// Setting roles is required to prevent live API calls during permissions check
	_ = st.Cabinet.RoleStore.RoleSet(guildID, &discord.Role{
		ID:          discord.RoleID(guildID),
		Name:        "@everyone",
		Permissions: discord.PermissionAll,
	}, false)

	// Success!
	if err := ValidateModerationLogChannel(st, "111", "222"); err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestIsSharedModerationChannel(t *testing.T) {
	if IsSharedModerationChannel("123", nil) {
		t.Errorf("expected false for nil guild config")
	}
	if IsSharedModerationChannel("", &files.GuildConfig{}) {
		t.Errorf("expected false for empty channel ID")
	}

	gcfg := &files.GuildConfig{
		Channels: files.ChannelsConfig{
			Commands: "123",
		},
	}
	if !IsSharedModerationChannel("123", gcfg) {
		t.Errorf("expected true for commands channel match")
	}
	if IsSharedModerationChannel("456", gcfg) {
		t.Errorf("expected false for no match")
	}
}
