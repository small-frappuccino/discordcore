package embeds

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRenderCustomEmbed(t *testing.T) {
	t.Parallel()
	ce := files.CustomEmbedConfig{
		Key:           "test-key",
		Title:         "Test Embed Title",
		Description:   "Test Embed Description",
		Color:         16711680, // Red
		AuthorName:    "Author",
		AuthorIconURL: "http://author.com/icon.png",
		FooterText:    "Footer",
		FooterIconURL: "http://footer.com/icon.png",
		ImageURL:      "http://image.com/img.png",
		ThumbnailURL:  "http://thumb.com/thumb.png",
		Fields: []files.CustomEmbedFieldConfig{
			{Name: "Field1", Value: "Value1", Inline: true},
			{Name: "Field2", Value: "Value2", Inline: false},
		},
	}

	embed := renderCustomEmbed(ce)

	if embed.Title != ce.Title {
		t.Fatalf("embed.Title = %q, want %q", embed.Title, ce.Title)
	}
	if embed.Description != ce.Description {
		t.Fatalf("embed.Description = %q, want %q", embed.Description, ce.Description)
	}
	if embed.Color != ce.Color {
		t.Fatalf("embed.Color = %d, want %d", embed.Color, ce.Color)
	}
	if embed.Author == nil || embed.Author.Name != ce.AuthorName || embed.Author.IconURL != ce.AuthorIconURL {
		t.Fatalf("embed.Author mismatch")
	}
	if embed.Footer == nil || embed.Footer.Text != ce.FooterText || embed.Footer.IconURL != ce.FooterIconURL {
		t.Fatalf("embed.Footer mismatch")
	}
	if embed.Image == nil || embed.Image.URL != ce.ImageURL {
		t.Fatalf("embed.Image mismatch")
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != ce.ThumbnailURL {
		t.Fatalf("embed.Thumbnail mismatch")
	}
	if len(embed.Fields) != 2 {
		t.Fatalf("len(embed.Fields) = %d, want 2", len(embed.Fields))
	}
	if embed.Fields[0].Name != "Field1" || embed.Fields[0].Value != "Value1" || !embed.Fields[0].Inline {
		t.Fatalf("embed.Fields[0] mismatch")
	}
}

func TestCustomEmbedPostingSyncer(t *testing.T) {
	t.Parallel()

	cm := files.NewMemoryConfigManager()
	guildID := "guild-sync"
	key := "embed-key"

	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	ce := files.CustomEmbedConfig{
		Key:         key,
		Title:       "Title",
		Description: "Desc",
		Postings: []files.CustomEmbedPostingConfig{
			{ChannelID: "ch1", MessageID: "msg1"},
			{ChannelID: "ch2", MessageID: "msg2"},
		},
	}
	if err := cm.SetCustomEmbedProperties(guildID, key, ce); err != nil {
		t.Fatalf("set custom embed: %v", err)
	}
	for _, p := range ce.Postings {
		if err := cm.AddCustomEmbedPosting(guildID, key, p); err != nil {
			t.Fatalf("add posting: %v", err)
		}
	}

	var editedPaths []string
	syncer := &customEmbedPostingSyncer{
		configManager: cm,
		editMessage: func(s *discordgo.Session, edit *discordgo.MessageEdit) error {
			if edit.ID == "msg2" {
				// simulate deleted message on second posting
				return &discordgo.RESTError{
					Message: &discordgo.APIErrorMessage{
						Code: 10008, // Unknown Message
					},
				}
			}
			editedPaths = append(editedPaths, edit.ID)
			return nil
		},
		dropPostings: func(c *files.ConfigManager, gid, k string, mid []string) error {
			return c.RemoveCustomEmbedPostings(gid, k, mid)
		},
	}

	session := &discordgo.Session{}
	result := syncer.Sync(session, guildID, key, ce.Postings, renderCustomEmbed(ce))

	if result.Edited != 1 {
		t.Fatalf("expected 1 edit, got %d", result.Edited)
	}
	if len(result.Dropped) != 1 || result.Dropped[0].MessageID != "msg2" {
		t.Fatalf("expected msg2 to be dropped")
	}

	// Verify that msg2 was dropped from config Manager
	updated, err := cm.CustomEmbed(guildID, key)
	if err != nil {
		t.Fatalf("load custom embed: %v", err)
	}
	if len(updated.Postings) != 1 || updated.Postings[0].MessageID != "msg1" {
		t.Fatalf("expected only msg1 to remain in custom embed postings, got %+v", updated.Postings)
	}
}

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

	cm := files.NewMemoryConfigManager()
	guildID := "guild-embed-test"
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: "operator"}); err != nil {
		t.Fatalf("add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	embedCommands := NewEmbedCommands(cm)
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
