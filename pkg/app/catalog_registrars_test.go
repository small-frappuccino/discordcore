package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	qotdcmd "github.com/small-frappuccino/discordcore/pkg/discord/commands/qotd"
	"github.com/small-frappuccino/discordcore/pkg/discord/embeds"
	"github.com/small-frappuccino/discordcore/pkg/discord/partners"
	"github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	appstats "github.com/small-frappuccino/discordcore/pkg/stats"
)

type MockRegistrarContext struct {
	sessionToken      string
	configManager     *files.ConfigManager
	runtimeApplier    *runtimeapply.Manager
	partnerService    *partners.PartnerService
	moderationMetrics moderation.Metrics
	rolePanelService  *roles.RolePanelService
	embedService      *embeds.EmbedService
	qotdService       qotdcmd.Service
	statsService      *appstats.StatsService
}

func (m MockRegistrarContext) SessionToken() string { return m.sessionToken }
func (m MockRegistrarContext) ConfigProvider() config.Provider {
	if m.configManager == nil {
		return nil
	}
	return m.configManager
}
func (m MockRegistrarContext) RuntimeApplier() *runtimeapply.Manager     { return m.runtimeApplier }
func (m MockRegistrarContext) PartnerService() *partners.PartnerService  { return m.partnerService }
func (m MockRegistrarContext) ModerationMetrics() moderation.Metrics     { return m.moderationMetrics }
func (m MockRegistrarContext) RolePanelService() *roles.RolePanelService { return m.rolePanelService }
func (m MockRegistrarContext) EmbedService() *embeds.EmbedService        { return m.embedService }
func (m MockRegistrarContext) QOTDService() qotdcmd.Service              { return m.qotdService }
func (m MockRegistrarContext) StatsService() *appstats.StatsService      { return m.statsService }

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
			mockCtx := MockRegistrarContext{
				sessionToken:  "mock-token",
				configManager: files.NewConfigManagerWithStore(nil, nil),
			}
			spyRouter := commands.NewSpyRouter()
			registrar := tt.factory()

			require.NotNil(t, registrar.RegisterArikawa, "RegisterArikawa must be implemented")

			// Act
			registrar.RegisterArikawa(mockCtx, spyRouter)

			// Assert
			allCmds := spyRouter.GetRegisteredArikawaCommands()
			require.Len(t, allCmds, len(tt.expectedCmds), "Mismatch in registered commands count")

			for _, expectedName := range tt.expectedCmds {
				exists := spyRouter.HasCommand(expectedName)
				assert.True(t, exists, "Missing expected Arikawa command: %s", expectedName)
				if exists {
					cmdData := spyRouter.GetCommandData(expectedName)
					assert.NotEmpty(t, cmdData.Description, "Command %s must have a description", expectedName)
				}
			}
		})
	}
}

func TestCatalogRegistrars_DIFailures(t *testing.T) {
	t.Parallel()

	t.Run("StatsRegistrar_Requires_ConfigManager", func(t *testing.T) {
		t.Parallel()

		// Intentional missing dependency (configManager is nil in MockRegistrarContext)
		mockCtx := MockRegistrarContext{
			configManager: nil,
			statsService:  nil,
		}

		registrar := StatsCommandCatalogRegistrar()
		require.NotNil(t, registrar.RegisterArikawa)

		spyRouter := commands.NewSpyRouter()

		// Expect the factory or the closure execution to handle the missing dependency gracefully
		// stats.NewStatsCommands().RegisterCommands(router) safely aborts if configManager is nil.
		assert.NotPanics(t, func() {
			registrar.RegisterArikawa(mockCtx, spyRouter)
		}, "Registrar should not panic if configManager is missing")

		allCmds := spyRouter.GetRegisteredArikawaCommands()
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

func TestCommandCatalogCapabilities_BitmaskIntegrity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		baseMask   CommandCatalogCapabilities
		targetMask CommandCatalogCapabilities
		wantHas    bool
	}{
		{
			name:       "CapNone evaluates as true against any base mask",
			baseMask:   CapStats | CapBanMembers,
			targetMask: CapNone,
			wantHas:    true,
		},
		{
			name:       "Empty mask rejects any specific capability",
			baseMask:   CapNone,
			targetMask: CapStats,
			wantHas:    false,
		},
		{
			name:       "Composite mask contains singular target",
			baseMask:   CapStats | CapKickMembers | CapManageMessages,
			targetMask: CapKickMembers,
			wantHas:    true,
		},
		{
			name:       "Composite mask does not contain missing target",
			baseMask:   CapStats | CapKickMembers,
			targetMask: CapBanMembers,
			wantHas:    false,
		},
		{
			name:       "Composite mask contains exact multiple targets",
			baseMask:   CapStats | CapKickMembers | CapBanMembers,
			targetMask: CapKickMembers | CapBanMembers,
			wantHas:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.baseMask.Has(tt.targetMask)
			if got != tt.wantHas {
				t.Fatalf("bit structural evaluation failure: base (%s) operating against target (%s). Expected: %t, Got: %t",
					tt.baseMask.String(), tt.targetMask.String(), tt.wantHas, got)
			}
		})
	}
}

func TestRuntimeCommandCatalogRegistrar_FailFastBarrier(t *testing.T) {
	t.Parallel()

	// Panic interceptor focused on standard Go scope
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Structural invariant broken: fail-fast barrier did not trigger expected panic")
		}

		panicMsg, ok := r.(string)
		if !ok || panicMsg != "fail-fast violation: runtimeApplier is strictly required for RuntimeCommandCatalogRegistrar" {
			t.Fatalf("The triggered panic diverged from expected contract. Got: %v", r)
		}
	}()

	// Deliberately injecting invalid state
	mockCtx := MockRegistrarContext{
		sessionToken:   "dead-token",
		runtimeApplier: nil, // Explicit panic trigger
	}

	spyRouter := commands.NewSpyRouter()
	registrar := RuntimeCommandCatalogRegistrar()

	// This must abort execution and transfer control to deferred routine above
	registrar.RegisterArikawa(mockCtx, spyRouter)
}
