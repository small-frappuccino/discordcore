package embeds

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/small-frappuccino/discordcore/pkg/config"
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

	svc := NewEmbedService(nil)
	embed := svc.Render(ce)

	if embed.Title != ce.Title {
		t.Fatalf("embed.Title = %q, want %q", embed.Title, ce.Title)
	}
	if embed.Description != ce.Description {
		t.Fatalf("embed.Description = %q, want %q", embed.Description, ce.Description)
	}
	if embed.Color != discord.Color(ce.Color) {
		t.Fatalf("embed.Color = %d, want %d", embed.Color, ce.Color)
	}
	if embed.Author == nil || embed.Author.Name != ce.AuthorName || embed.Author.Icon != ce.AuthorIconURL {
		t.Fatalf("embed.Author mismatch")
	}
	if embed.Footer == nil || embed.Footer.Text != ce.FooterText || embed.Footer.Icon != ce.FooterIconURL {
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

	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	svc := NewEmbedService(cm)
	guildID := "123456789012345678"
	key := "embed-key"

	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	ce := files.CustomEmbedConfig{
		Key:         key,
		Title:       "Title",
		Description: "Desc",
		Postings: []files.CustomEmbedPostingConfig{
			{ChannelID: "111111111111111111", MessageID: "222222222222222222"},
			{ChannelID: "333333333333333333", MessageID: "444444444444444444"},
		},
	}
	if err := svc.SetCustomEmbedProperties(guildID, key, ce); err != nil {
		t.Fatalf("set custom embed: %v", err)
	}
	for _, p := range ce.Postings {
		if err := svc.AddCustomEmbedPosting(guildID, key, p); err != nil {
			t.Fatalf("add posting: %v", err)
		}
	}

	var editedPaths []discord.MessageID
	svc.editMessage = func(client *api.Client, channelID discord.ChannelID, messageID discord.MessageID, data api.EditMessageData) error {
		if messageID == discord.MessageID(444444444444444444) {
			return &httputil.HTTPError{Code: discordErrUnknownMessage}
		}
		editedPaths = append(editedPaths, messageID)
		return nil
	}

	client := &api.Client{}
	embed := svc.Render(ce)
	result := svc.Sync(client, guildID, key, ce.Postings, &embed)

	if result.Edited != 1 {
		t.Fatalf("expected 1 edit, got %d", result.Edited)
	}
	if len(result.Dropped) != 1 || result.Dropped[0].MessageID != "444444444444444444" {
		t.Fatalf("expected msg2 to be dropped")
	}

	// Verify that msg2 was dropped from config Manager
	updated, err := svc.CustomEmbed(guildID, key)
	if err != nil {
		t.Fatalf("load custom embed: %v", err)
	}
	if len(updated.Postings) != 1 || updated.Postings[0].MessageID != "222222222222222222" {
		t.Fatalf("expected only msg1 to remain in custom embed postings, got %+v", updated.Postings)
	}
}
