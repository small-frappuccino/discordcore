package commands_test

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
)

// FuzzContextBuilder_PayloadResilience uses property-based fuzzing to ensure
// the context builder never panics, even under malformed binary conditions.
// This isolates the parsing layer from upstream Arikawa/Discord API oddities.
func FuzzContextBuilder_PayloadResilience(f *testing.F) {
	// Seed with valid and invalid bounds to guide the fuzzer.
	f.Add(uint64(123456789), uint64(987654321), "en-US")
	f.Add(uint64(0), uint64(0), "")
	f.Add(uint64(1), uint64(1), "pt-BR")

	f.Fuzz(func(t *testing.T, guildID uint64, userID uint64, locale string) {
		// Mock a structurally loose InteractionEvent directly from raw bytes.
		event := discord.InteractionEvent{
			GuildID: discord.GuildID(guildID),
			// The SenderID property infers the user ID from Member or User.
			User: &discord.User{
				ID: discord.UserID(userID),
			},
		}

		// The core invariant here: NewArikawaContext MUST NOT panic,
		// regardless of how bizarre the data injected by Discord is.
		ctx, err := commands.NewArikawaContext(event, nil) // nil config manager for isolation

		// Ensure proper error signaling.
		if err == nil && ctx == nil {
			t.Fatal("returned nil context without a corresponding error")
		}
		if err != nil && ctx != nil {
			t.Fatal("returned non-nil context alongside an error")
		}
	})
}
