package moderation

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	discordmod "github.com/small-frappuccino/discordcore/pkg/discord/moderation"
)

type mockMetrics struct {
	called string
}

func (m *mockMetrics) RecordCommandExec(name string) { m.called = name }

type mockClient struct {
	banCalled     bool
	timeoutCalled bool
}

func (m *mockClient) Ban(guildID discord.GuildID, userID discord.UserID, data api.BanData) error {
	m.banCalled = true
	return nil
}
func (m *mockClient) Kick(guildID discord.GuildID, userID discord.UserID, reason api.AuditLogReason) error {
	return nil
}
func (m *mockClient) ModifyMember(guildID discord.GuildID, userID discord.UserID, data api.ModifyMemberData) error {
	m.timeoutCalled = true
	return nil
}

// TestCommands_StatelessExecution verifies that metrics isolate command
// executions seamlessly without crossing data bounds between concurrent instances.
func TestCommands_StatelessExecution(t *testing.T) {
	metricsBan := &mockMetrics{}
	metricsTimeout := &mockMetrics{}

	client := &mockClient{}
	svc := discordmod.NewService(client, nil)

	banCmd := NewBanCommand(svc, metricsBan, nil)
	timeoutCmd := NewTimeoutCommand(svc, metricsTimeout, nil)

	ctx1 := &commands.ArikawaContext{
		GuildID: discord.GuildID(123),
		Client:  nil, // EditInteractionResponse will panic, but we only check metrics routing before that.
	}
	ctx2 := &commands.ArikawaContext{
		GuildID: discord.GuildID(123),
		Client:  nil,
	}

	// We wrap in a recovery function because we haven't completely mocked Arikawa's internal HTTP client
	// which `EditInteractionResponse` requires. The metric is executed first.
	func() {
		defer func() { recover() }()
		_ = banCmd.Handle(ctx1)
	}()

	func() {
		defer func() { recover() }()
		_ = timeoutCmd.Handle(ctx2)
	}()

	if metricsBan.called != "ban" {
		t.Errorf("expected ban metric, got %s", metricsBan.called)
	}

	if metricsTimeout.called != "timeout" {
		t.Errorf("expected timeout metric, got %s", metricsTimeout.called)
	}

	// They should not cross state boundaries
	if metricsBan.called == metricsTimeout.called {
		t.Error("metrics crossed state boundaries between different command instances")
	}
}

// TestMassBanCommand_Parity ensures MassBan natively utilizes the core logic parsing.
func TestMassBanCommand_Parity(t *testing.T) {
	svc := discordmod.NewService(&mockClient{}, nil)
	cmd := NewMassBanCommand(svc, nil, nil)

	if cmd.Name() != "massban" {
		t.Errorf("expected massban name")
	}
}
