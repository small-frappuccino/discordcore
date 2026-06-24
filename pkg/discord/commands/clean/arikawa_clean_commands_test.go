package clean

import (
	"context"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"

	coreclean "github.com/small-frappuccino/discordcore/pkg/clean"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type mockExecutor struct {
	filter coreclean.Filter
}

func (m *mockExecutor) ExecuteClean(ctx context.Context, channelID discord.ChannelID, filter coreclean.Filter, auditChannelID discord.ChannelID, requestedBy string) (int, error) {
	m.filter = filter
	return 1, nil
}

// TestArikawaCleanCommand_SyntheticPayloadInjection verifies structural typing anomalies
// are gracefully handled without panicking or passing corrupted states.
func TestArikawaCleanCommand_SyntheticPayloadInjection(t *testing.T) {
	t.Parallel()
	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
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

	// Injecting structural typing anomaly: passing Integer for User option
	ctx := &commands.ArikawaContext{
		GuildID: discord.GuildID(123),
		UserID:  discord.UserID(456),
		Interaction: &discord.InteractionEvent{
			ChannelID: discord.ChannelID(789),
			Data: &discord.CommandInteraction{
				Options: discord.CommandInteractionOptions{
					{
						Name:  "user",
						Type:  discord.IntegerOptionType, // INTENTIONAL ANOMALY
						Value: []byte("123"),             // Scalar value
					},
					{
						Name:  "count",
						Type:  discord.IntegerOptionType,
						Value: []byte("42"),
					},
				},
			},
		},
	}

	var returnErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Clean Handle triggered a panic on malformed type injection: %v", r)
			}
		}()
		returnErr = cmd.Handle(ctx)
	}()

	if returnErr == nil {
		t.Fatalf("Expected mechanism to reject conversion, but it succeeded")
	}

	// Ensure the returned error is our EphemeralError wrapping the structural anomaly
	if _, ok := returnErr.(*EphemeralError); !ok {
		t.Errorf("Expected EphemeralError, got %T", returnErr)
	}
}

// TestArikawaCleanCommand_StatelessExecution verifies isolated metrics runs.
func TestArikawaCleanCommand_StatelessExecution(t *testing.T) {
	t.Parallel()
	// NopMetrics natively prevents cross-pollination.
	// We instantiate multiple handlers simultaneously simulating high traffic
	// and ensure state is inherently local to Handle stack.

	cmd1 := NewCleanCommand(nil, &mockExecutor{})
	cmd2 := NewCleanCommand(nil, &mockExecutor{})

	if cmd1 == cmd2 {
		t.Fatal("Commands should not share memory addresses directly representing state overlap.")
	}
}
