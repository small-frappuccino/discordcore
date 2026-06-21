package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestCatalogRegistrars_RegisterArikawa(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		factory      func() CommandCatalogRegistrar
		expectedCmds []string
	}{
		{
			name:         "Moderation_Catalog_Wiring",
			factory:      ModerationCommandCatalogRegistrar,
			expectedCmds: []string{"ban", "timeout", "massban", "reaction_block"},
		},
		{
			name:         "Stats_Catalog_Wiring",
			factory:      StatsCommandCatalogRegistrar,
			expectedCmds: []string{"stats"},
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel execution
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			mockHandler := &CommandHandler{
				session:       &discordgo.Session{Token: "mock-token"},
				configManager: files.NewConfigManagerWithStore(nil, nil),
			}
			spyRouter := commands.NewCommandRouter(nil, nil)
			registrar := tt.factory()

			require.NotNil(t, registrar.RegisterArikawa, "RegisterArikawa must be implemented")

			// Act
			registrar.RegisterArikawa(mockHandler, spyRouter)

			// Assert
			registry := spyRouter.Registry()
			allCmds := registry.GetAllCommands()
			require.Len(t, allCmds, len(tt.expectedCmds), "Mismatch in registered commands count")

			for _, expectedName := range tt.expectedCmds {
				cmd, exists := registry.GetCommand(expectedName)
				assert.True(t, exists, "Missing expected Arikawa command: %s", expectedName)
				if exists {
					assert.NotEmpty(t, cmd.Description(), "Command %s must have a description", expectedName)
				}
			}
		})
	}
}

func TestCatalogRegistrars_DIFailures(t *testing.T) {
	t.Parallel()

	t.Run("StatsRegistrar_Requires_ConfigManager", func(t *testing.T) {
		t.Parallel()

		// Intentional missing dependency (configManager is nil in CommandHandler)
		handlerWithoutConfig := &CommandHandler{
			configManager: nil,
			statsService:  nil,
		}

		registrar := StatsCommandCatalogRegistrar()
		require.NotNil(t, registrar.RegisterArikawa)

		spyRouter := commands.NewCommandRouter(nil, nil)

		// Expect the factory or the closure execution to handle the missing dependency gracefully
		// stats.NewStatsCommands().RegisterCommands(router) safely aborts if configManager is nil.
		assert.NotPanics(t, func() {
			registrar.RegisterArikawa(handlerWithoutConfig, spyRouter)
		}, "Registrar should not panic if configManager is missing")

		registry := spyRouter.Registry()
		allCmds := registry.GetAllCommands()
		assert.Empty(t, allCmds, "No commands should be registered if configManager is nil")
	})
}

func TestCatalogRegistrars_Capabilities(t *testing.T) {
	t.Parallel()

	t.Run("Moderation_Capabilities", func(t *testing.T) {
		t.Parallel()
		registrar := ModerationCommandCatalogRegistrar()

		assert.True(t, registrar.RequiredCapabilities == CapNone, "Moderation registrar should not require any specific capability")
	})

	t.Run("Stats_Capabilities", func(t *testing.T) {
		t.Parallel()
		registrar := StatsCommandCatalogRegistrar()

		assert.True(t, registrar.RequiredCapabilities.Has(CapStats), "Stats registrar must require CapStats")
	})
}
