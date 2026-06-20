package clean

import (
	"context"
	"errors"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"

	coreclean "github.com/small-frappuccino/discordcore/pkg/clean"
	discordclean "github.com/small-frappuccino/discordcore/pkg/discord/clean"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type mockClient struct {
	discordclean.Client
}

func TestArikawaCleanCommand_EarlyRejection(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	disabled := false
	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{{
			GuildID: "123",
			Features: files.FeatureToggles{
				Moderation: files.FeatureModerationToggles{Clean: &disabled},
			},
		}},
	}
	cm.ApplyConfig(cfg)

	// Default config has everything disabled
	svc := discordclean.NewService(&mockClient{}, nil, nil)
	cmd := NewCleanCommand(cm, svc)

	ctx := &legacycore.ArikawaContext{
		GuildID:     discord.GuildID(123),
		Interaction: &discord.InteractionEvent{},
	}

	err := cmd.Handle(ctx)
	if err == nil {
		t.Fatalf("expected error due to disabled feature")
	}

	var eph *EphemeralError
	if !errors.As(err, &eph) {
		t.Fatalf("expected EphemeralError, got %T", err)
	}

	if eph.UserMessage != "Moderation Clean is disabled." {
		t.Errorf("unexpected user message: %s", eph.UserMessage)
	}
}

func TestEphemeralError_Structure(t *testing.T) {
	internalErr := errors.New("network timeout")
	err := &EphemeralError{
		UserMessage: "Something went wrong",
		InternalErr: internalErr,
	}

	var unwrappedEph *EphemeralError
	if !errors.As(err, &unwrappedEph) {
		t.Fatalf("errors.As should match EphemeralError")
	}

	if !errors.Is(err.Unwrap(), internalErr) {
		t.Errorf("Unwrap did not return the exact internal error")
	}

	resp := err.InteractionResponse()
	if resp.Type != api.MessageInteractionWithSource {
		t.Errorf("expected MessageInteractionWithSource")
	}

	if resp.Data == nil {
		t.Fatalf("expected non-nil Data")
	}

	if int(resp.Data.Flags)&64 != 64 {
		t.Errorf("expected Ephemeral flag (64) to be present, got %d", resp.Data.Flags)
	}

	content := resp.Data.Content.Val
	if content != "Something went wrong" {
		t.Errorf("expected clean user message, got %s", content)
	}
}

type mockExecutor struct {
	filter coreclean.Filter
}

func (m *mockExecutor) ExecuteClean(ctx context.Context, channelID discord.ChannelID, filter coreclean.Filter, auditChannelID discord.ChannelID, requestedBy string) (int, error) {
	m.filter = filter
	return 1, nil
}

func TestArikawaCleanCommand_OptionsMapping(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	enabled := true
	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{{
			GuildID: "123",
			Features: files.FeatureToggles{
				Moderation: files.FeatureModerationToggles{Clean: &enabled},
			},
		}},
	}
	cm.ApplyConfig(cfg)

	mockExec := &mockExecutor{}
	cmd := NewCleanCommand(cm, mockExec)

	ctx := &legacycore.ArikawaContext{
		GuildID: discord.GuildID(123),
		UserID:  discord.UserID(456),
		Interaction: &discord.InteractionEvent{
			ChannelID: discord.ChannelID(789),
			Data: &discord.CommandInteraction{
				Options: discord.CommandInteractionOptions{
					{Name: "count", Value: []byte("42")},
					{Name: "user", Value: []byte(`"999"`)},
					{Name: "contains", Value: []byte(`"badword"`)},
					{Name: "from", Value: []byte(`"1000"`)},
					{Name: "to", Value: []byte(`"2000"`)},
				},
			},
		},
	}

	func() {
		defer func() { recover() }()
		_ = cmd.Handle(ctx)
	}()

	if mockExec.filter.Count != 42 {
		t.Errorf("expected count 42, got %d", mockExec.filter.Count)
	}
	if mockExec.filter.UserID != "999" {
		t.Errorf("expected user 999, got %s", mockExec.filter.UserID)
	}
	if mockExec.filter.Contains != "badword" {
		t.Errorf("expected contains badword, got %s", mockExec.filter.Contains)
	}
	if mockExec.filter.FromID != "1000" {
		t.Errorf("expected from 1000, got %s", mockExec.filter.FromID)
	}
	if mockExec.filter.ToID != "2000" {
		t.Errorf("expected to 2000, got %s", mockExec.filter.ToID)
	}
}
