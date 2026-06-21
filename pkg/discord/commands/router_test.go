package commands_test

import (
	"errors"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
)

// TestCommandRouter_RouteInteraction utilizes Table-Driven Testing (TDT) to
// exhaustively validate branching logic within the interaction router,
// employing pure stubs to avoid network dependency.
func TestCommandRouter_RouteInteraction(t *testing.T) {
	t.Parallel()

	// Operational Annotation: We do not initialize the API client to enforce
	// that routing is entirely decoupled from the REST execution layer.

	tests := []struct {
		name          string
		interaction   *discord.InteractionEvent
		registeredCmd commands.ArikawaCommand
		wantErr       error
	}{
		{
			name: "Valid Slash Command Routing",
			interaction: &discord.InteractionEvent{
				GuildID: discord.GuildID(123),
				User:    &discord.User{ID: discord.UserID(456)},
				Data: &discord.CommandInteraction{
					Name: "clean",
				},
			},
			registeredCmd: &mockArikawaCommand{name: "clean"},
			wantErr:       nil,
		},
		{
			name: "Unregistered Command Fallback",
			interaction: &discord.InteractionEvent{
				GuildID: discord.GuildID(123),
				User:    &discord.User{ID: discord.UserID(456)},
				Data: &discord.CommandInteraction{
					Name: "ghost_command",
				},
			},
			registeredCmd: &mockArikawaCommand{name: "clean"},
			wantErr:       commands.ErrCommandNotFound,
		},
		{
			name:        "Nil Interaction Protection",
			interaction: nil,
			wantErr:     nil, // We expect early graceful return without panic
		},
	}

	for _, tt := range tests {
		tt := tt // Pin variable for parallel subtests (Go <= 1.21 invariant, harmless in 1.22+)
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := commands.NewCommandRouter(nil, nil)
			if tt.registeredCmd != nil {
				router.Register(tt.registeredCmd)
			}

			err := router.HandleEvent(tt.interaction)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}
