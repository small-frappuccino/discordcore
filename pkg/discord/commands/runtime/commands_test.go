package runtime

import (
	"context"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"go.uber.org/mock/gomock"
)

// TestHandler_HandleSlash_EphemeralValidation utilizes strict dependency injection
// generated via mockgen to isolate the interaction dispatcher from the global network layer.
// Condition of Victory: The dispatch behavior toggles Ephemeral strictly locally, validated
// entirely in memory without brittle HTTP fixtures.
func TestHandler_HandleSlash_EphemeralValidation(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	replier := NewMockInteractionReplier(ctrl)

	tmp := t.TempDir()
	_ = tmp
	store := &files.MemoryConfigStore{}
	cm := files.NewConfigManagerWithStore(store, nil)
	_ = cm.LoadConfig()

	handler := NewHandler(replier, cm, nil)

	// Construct an isolated, synthetic interaction mimicking a user triggering /config runtime.
	ev := &discord.InteractionEvent{
		ID:    discord.InteractionID(12345),
		Token: "test-token",
		User: &discord.User{
			ID: discord.UserID(987654),
		},
		Data: &discord.CommandInteraction{
			Name: "config runtime",
		},
	}

	// Structural enforcement: We assert geometrically that the handler mathematically emits
	// exactly one REST API translation payload to the mock replier, containing the mandatory
	// Ephemeral directive necessary for administrative panel privacy.
	replier.EXPECT().
		RespondInteraction(gomock.Any(), ev.ID, ev.Token, gomock.Any()).
		DoAndReturn(func(ctx context.Context, id discord.InteractionID, token string, resp api.InteractionResponse) error {
			if resp.Type != api.MessageInteractionWithSource {
				t.Errorf("Expected response type MessageInteractionWithSource, got %v", resp.Type)
			}
			if resp.Data.Flags != discord.EphemeralMessage {
				t.Errorf("Expected ephemeral flag for admin panel, got %v", resp.Data.Flags)
			}
			return nil
		}).
		Times(1)

	err := handler.HandleSlash(context.Background(), ev)
	if err != nil {
		t.Fatalf("HandleSlash returned unexpected error: %v", err)
	}
}
