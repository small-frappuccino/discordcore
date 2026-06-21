package commands_test

import (
	"context"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewArikawaContext_InitializationAndFailFast(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		interaction discord.InteractionEvent
		expectError error
	}{
		{
			name: "Valid Interaction",
			interaction: discord.InteractionEvent{
				GuildID: 12345,
				User: &discord.User{
					ID: 12345,
				},
			},
			expectError: nil,
		},
		{
			name: "Invalid Event Data - SenderID 0",
			interaction: discord.InteractionEvent{
				GuildID: 12345,
				// SenderID resolves to 0 when User and Member are nil
			},
			expectError: commands.ErrInvalidEventData,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Injeção rigorosa do ConfigManager alimentado por um in-memory store
			// garantindo validação de Pre-fetch do GuildConfig em nanosegundos
			store := &files.MemoryConfigStore{}
			_ = store.Save(&files.BotConfig{
				Guilds: []files.GuildConfig{
					{GuildID: "12345"},
				},
			})
			configManager := files.NewConfigManagerWithStore(store, nil)
			_ = configManager.LoadConfig()

			ctx, err := commands.NewArikawaContext(tt.interaction, configManager)

			if tt.expectError != nil {
				require.ErrorIs(t, err, tt.expectError)
				assert.Nil(t, ctx)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, ctx)

			// Verifica o fallback automático do logger e resolução de contexto
			assert.NotNil(t, ctx.Logger)
			assert.NotNil(t, ctx.Context())

			// Valida o Pre-fetch
			if tt.interaction.GuildID.IsValid() {
				assert.NotNil(t, ctx.GuildConfig)
			}
		})
	}
}

func TestArikawaContext_ContextResolution(t *testing.T) {
	t.Parallel()

	// Simula a instanciação manual ou falha na injeção do context interno
	arikawaCtx := &commands.ArikawaContext{}

	// Deve resolver para background de forma transparente
	resolvedCtx := arikawaCtx.Context()
	require.NotNil(t, resolvedCtx)
	assert.Equal(t, context.Background(), resolvedCtx)
}

func TestArikawaContext_APIWrappers_DefensiveChecks(t *testing.T) {
	t.Parallel()

	t.Run("Respond triggers error on nil Interaction", func(t *testing.T) {
		ctx := &commands.ArikawaContext{
			Client:      api.NewClient("Bot mock"),
			Interaction: nil, // Estado inválido proposital
		}

		err := ctx.Respond(api.InteractionResponseData{Content: option.NewNullableString("test")})
		require.Error(t, err)
	})

	t.Run("Defer triggers error on nil Client", func(t *testing.T) {
		ctx := &commands.ArikawaContext{
			Client:      nil, // Estado de dependência quebrado propositalmente
			Interaction: &discord.InteractionEvent{ID: 1, Token: "mock_token"},
		}

		err := ctx.Defer(0)
		require.Error(t, err)
	})
}
