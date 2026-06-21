package commands_test

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
)

func TestNewArikawaMissingConfigErrorData(t *testing.T) {
	// Garante velocidade máxima rodando os testes em paralelo,
	// já que a função geradora não possui side-effects ou estado global.
	t.Parallel()

	tests := []struct {
		name        string
		feature     string
		wantContent string
	}{
		{
			name:        "standard_feature_missing",
			feature:     "Stats Channels",
			wantContent: "❌ Configuration missing for Stats Channels. Please ensure it is configured in the dashboard.",
		},
		{
			name:        "ignored_parameters_do_not_mutate_output",
			feature:     "Audit Logs",
			wantContent: "❌ Configuration missing for Audit Logs. Please ensure it is configured in the dashboard.",
		},
		{
			name:        "empty_feature_string_edge_case",
			feature:     "",
			wantContent: "❌ Configuration missing for . Please ensure it is configured in the dashboard.",
		},
		{
			name:        "special_characters_in_feature",
			feature:     "Auto-Mod (Beta) & Spam Filters",
			wantContent: "❌ Configuration missing for Auto-Mod (Beta) & Spam Filters. Please ensure it is configured in the dashboard.",
		},
	}

	for _, tt := range tests {
		tt := tt // Pin da variável para a closure rodar com segurança no t.Parallel()

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := commands.NewArikawaMissingConfigErrorData(tt.feature)

			// Assert
			require.NotNil(t, got, "O payload de retorno não pode ser nil")

			// 1. Valida a serialização estrita do NullableString
			expectedContent := option.NewNullableString(tt.wantContent)
			assert.Equal(t, expectedContent, got.Content, "O campo Content deve encapsular a string formatada em um NullableString válido")

			// 2. Valida o Isolamento de Sessão (invariante de segurança de UX)
			assert.Equal(t, discord.EphemeralMessage, got.Flags, "A flag EphemeralMessage é obrigatória para evitar poluição visual no chat principal")

			// 3. Valida a ausência de lixo de memória ou campos acidentais
			assert.Nil(t, got.Embeds, "Embeds não devem ser inicializados para erros simples")
			assert.Nil(t, got.Components, "Components de UI devem ser nulos")
			assert.Nil(t, got.AllowedMentions, "AllowedMentions deve ser nulo para evitar pings indesejados caso a feature string contenha '@'")
		})
	}
}
