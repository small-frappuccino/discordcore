package partners

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
)

func TestPartnerService_Render(t *testing.T) {
	t.Parallel()
	svc := NewPartnerService(nil)
	template := PartnerBoardTemplate{
		Title: "Test Board",
		Color: 12345,
	}
	partners := []PartnerRecord{
		{Fandom: "Game1", Name: "Server A", Link: "https://discord.gg/A"},
		{Fandom: "Game1", Name: "Server B", Link: "https://discord.gg/B"},
	}

	embeds, err := svc.Render(template, partners)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(embeds))
	}

	embed := embeds[0]
	if embed.Title != "Test Board" {
		t.Errorf("expected Title 'Test Board', got %q", embed.Title)
	}
	if embed.Color != discord.Color(12345) {
		t.Errorf("expected Color 12345, got %v", embed.Color)
	}
}
