package roles

import (
	"context"
	"errors"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	rolesvc "github.com/small-frappuccino/discordcore/pkg/discord/roles"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRolePanelComponentHandler_InjectionAndRouting(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	// Pre-configure a panel and button
	guildID := discord.GuildID(12345)
	roleID := "987654321"

	_, err := cm.UpdateConfig(context.Background(), func(bc *files.BotConfig) error {
		b := true
		bc.Guilds = append(bc.Guilds, files.GuildConfig{
			GuildID: guildID.String(),
			RolePanels: []files.RolePanelConfig{
				{
					Key: "test-panel",
					Buttons: []files.RolePanelButtonConfig{
						{RoleID: roleID, Label: "Test Role"},
					},
				},
			},
			Features: files.FeatureToggles{RolePanels: &b},
		})
		return nil
	})
	if err != nil {
		t.Fatalf("failed to init guild config: %v", err)
	}

	tests := []struct {
		name          string
		customID      string
		mockHasRole   bool
		mockLookupErr error
		mockAddErr    error
		mockRemoveErr error
		expectAdd     int
		expectRemove  int
	}{
		{
			name:         "valid assignment",
			customID:     rolesvc.RolePanelButtonCustomID(roleID),
			mockHasRole:  false,
			expectAdd:    1,
			expectRemove: 0,
		},
		{
			name:         "valid removal",
			customID:     rolesvc.RolePanelButtonCustomID(roleID),
			mockHasRole:  true,
			expectAdd:    0,
			expectRemove: 1,
		},
		{
			name:         "malformed custom id",
			customID:     "role_panel:button:",
			mockHasRole:  false,
			expectAdd:    0,
			expectRemove: 0,
		},
		{
			name:         "unknown role (not in config)",
			customID:     rolesvc.RolePanelButtonCustomID("111111111"),
			mockHasRole:  false,
			expectAdd:    0,
			expectRemove: 0,
		},
		{
			name:          "lookup error",
			customID:      rolesvc.RolePanelButtonCustomID(roleID),
			mockHasRole:   false,
			mockLookupErr: errors.New("API down"),
			expectAdd:     0,
			expectRemove:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var addCalls, removeCalls int
			var capturedRoleID string

			handler := &rolePanelComponentHandler{
				configManager: cm,
				memberLookup: func(ctx *legacycore.ArikawaContext, targetRoleID string) (bool, error) {
					return tt.mockHasRole, tt.mockLookupErr
				},
				addRole: func(ctx *legacycore.ArikawaContext, gID, uID, rID string) error {
					addCalls++
					capturedRoleID = rID
					return tt.mockAddErr
				},
				removeRole: func(ctx *legacycore.ArikawaContext, gID, uID, rID string) error {
					removeCalls++
					capturedRoleID = rID
					return tt.mockRemoveErr
				},
			}

			router := legacycore.NewArikawaCommandRouter("fake-token", cm)
			router.RegisterComponent(rolesvc.RolePanelComponentRouteID, handler)

			interaction := &discord.InteractionEvent{
				ID:      discord.InteractionID(111),
				GuildID: guildID,
				Member: &discord.Member{
					User: discord.User{ID: discord.UserID(222)},
				},
				Data: &discord.ButtonInteraction{
					CustomID: discord.ComponentID(tt.customID),
				},
			}

			// Call HandleInteractionEvent to test router structural partitioning
			router.HandleInteractionEvent(interaction)

			if addCalls != tt.expectAdd {
				t.Errorf("expected %d addRole calls, got %d", tt.expectAdd, addCalls)
			}
			if removeCalls != tt.expectRemove {
				t.Errorf("expected %d removeRole calls, got %d", tt.expectRemove, removeCalls)
			}
			if (tt.expectAdd > 0 || tt.expectRemove > 0) && capturedRoleID != roleID {
				t.Errorf("expected captured role ID to be %q, got %q", roleID, capturedRoleID)
			}
		})
	}
}
