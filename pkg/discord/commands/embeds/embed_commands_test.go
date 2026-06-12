package embeds

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	embedspkg "github.com/small-frappuccino/discordcore/pkg/embeds"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestEmbedKeyFromOptions(t *testing.T) {
	t.Parallel()

	// Valid key
	interactionValid := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  "key",
						Type:  discordgo.ApplicationCommandOptionString,
						Value: "Embed-Key-123",
					},
				},
			},
		},
	}
	key, err := embedKeyFromOptions(interactionValid)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if key != "embed-key-123" {
		t.Fatalf("expected normalized key 'embed-key-123', got %q", key)
	}

	// Empty key
	interactionEmpty := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type: discordgo.InteractionApplicationCommand,
			Data: discordgo.ApplicationCommandInteractionData{
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:  "key",
						Type:  discordgo.ApplicationCommandOptionString,
						Value: "   ",
					},
				},
			},
		},
	}
	_, err = embedKeyFromOptions(interactionEmpty)
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

type mockEmbedInteractionRecorder struct {
	responses []discordgo.InteractionResponse
}

func (r *mockEmbedInteractionRecorder) handleCallback(w http.ResponseWriter, req *http.Request) {
	var resp discordgo.InteractionResponse
	_ = json.NewDecoder(req.Body).Decode(&resp)
	r.responses = append(r.responses, resp)
	w.WriteHeader(http.StatusOK)
}

func TestEmbedCommandsIntegration(t *testing.T) {
	rec := &mockEmbedInteractionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/callback") {
			rec.handleCallback(w, req)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/"
	t.Cleanup(func() { discordgo.EndpointAPI = oldAPI })

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if session.State == nil {
		session.State = discordgo.NewState()
	}

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	guildID := "guild-embed-test"
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: "operator"}); err != nil {
		t.Fatalf("add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	embedService := embedspkg.NewEmbedService(cm)
	embedCommands := NewEmbedCommands(cm, embedService)
	embedCommands.RegisterCommands(router)

	// Test 1: List empty embeds
	interaction := &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "int-list",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: "operator"}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "embed",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name: "list",
						Type: discordgo.ApplicationCommandOptionSubCommand,
					},
				},
			},
		},
	}

	router.HandleInteraction(session, interaction)
	if len(rec.responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(rec.responses))
	}
	resp := rec.responses[0]
	if !strings.Contains(resp.Data.Content, "No custom embeds are configured yet") {
		t.Fatalf("unexpected list output: %q", resp.Data.Content)
	}
}
