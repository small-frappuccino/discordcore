package partners

import (
	"errors"
	"strings"
	"testing"
)

func TestBoardRendererRender_DefaultGroupingAndSorting(t *testing.T) {
	t.Parallel()

	renderer := NewBoardRenderer()
	embeds, err := renderer.Render(PartnerBoardTemplate{}, []PartnerRecord{
		{
			Fandom: "Genshin Impact",
			Name:   "Citlali Mains",
			Link:   "discord.gg/citlali",
		},
		{
			Fandom: "",
			Name:   "Jane Mains",
			Link:   "https://discord.gg/jane",
		},
		{
			Fandom: "Genshin Impact",
			Name:   "Alice Mains",
			Link:   "https://discord.gg/alice",
		},
	})
	if err != nil {
		t.Fatalf("render returned error: %v", err)
	}
	if len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(embeds))
	}

	desc := embeds[0].Description
	if !strings.Contains(desc, "**Genshin Impact**") {
		t.Fatalf("expected Genshin section in description: %q", desc)
	}
	if !strings.Contains(desc, "**Other Servers**") {
		t.Fatalf("expected Other Servers section in description: %q", desc)
	}

	idxAlice := strings.Index(desc, "[Alice Mains](https://discord.gg/alice)")
	idxCitlali := strings.Index(desc, "[Citlali Mains](https://discord.gg/citlali)")
	if idxAlice < 0 || idxCitlali < 0 {
		t.Fatalf("expected normalized partner links in description: %q", desc)
	}
	if idxAlice >= idxCitlali {
		t.Fatalf("expected partner sorting by name (Alice before Citlali): %q", desc)
	}

	idxGenshin := strings.Index(desc, "**Genshin Impact**")
	idxOther := strings.Index(desc, "**Other Servers**")
	if idxGenshin < 0 || idxOther < 0 {
		t.Fatalf("expected both sections in description: %q", desc)
	}
	if idxGenshin >= idxOther {
		t.Fatalf("expected fandom sorting (Genshin before Other): %q", desc)
	}
}

func TestBoardRendererRender_CustomTemplateAndFooter(t *testing.T) {
	t.Parallel()

	renderer := NewBoardRenderer()
	template := PartnerBoardTemplate{
		Title:                 "Community Partners",
		SectionHeaderTemplate: "## {fandom} ({count})",
		LineTemplate:          "{index}. {name} -> {link}",
		FooterTemplate:        "partners={total_partners} page={embed_index}/{embed_count}",
		DisableFandomSorting:  true,
		DisablePartnerSorting: true,
	}

	embeds, err := renderer.Render(template, []PartnerRecord{
		{Fandom: "ZZZ", Name: "Jane Mains", Link: "https://discord.gg/jane"},
		{Fandom: "Genshin Impact", Name: "Columbina Mains", Link: "https://discord.gg/columbina"},
	})
	if err != nil {
		t.Fatalf("render returned error: %v", err)
	}
	if len(embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(embeds))
	}
	if embeds[0].Title != "Community Partners" {
		t.Fatalf("unexpected title: %q", embeds[0].Title)
	}
	if embeds[0].Footer == nil {
		t.Fatalf("expected footer to be set")
	}
	if embeds[0].Footer.Text != "partners=2 page=1/1" {
		t.Fatalf("unexpected footer: %q", embeds[0].Footer.Text)
	}

	desc := embeds[0].Description
	if !strings.Contains(desc, "## ZZZ (1)") {
		t.Fatalf("expected custom section header: %q", desc)
	}
	if !strings.Contains(desc, "1. Jane Mains -> https://discord.gg/jane") {
		t.Fatalf("expected custom line template for Jane: %q", desc)
	}
	if !strings.Contains(desc, "## Genshin Impact (1)") {
		t.Fatalf("expected second custom section header: %q", desc)
	}
}

func TestBoardRendererRender_SplitsAcrossEmbeds(t *testing.T) {
	t.Parallel()

	renderer := newBoardRendererWithLimits(170, 10)
	partners := []PartnerRecord{
		{Fandom: "Genshin Impact", Name: "Alice Mains", Link: "https://discord.gg/alice"},
		{Fandom: "Genshin Impact", Name: "Citlali Mains", Link: "https://discord.gg/citlali"},
		{Fandom: "Genshin Impact", Name: "Mavuika Mains", Link: "https://discord.gg/mavuika"},
		{Fandom: "Genshin Impact", Name: "Lauma Mains", Link: "https://discord.gg/lauma"},
		{Fandom: "Genshin Impact", Name: "Mizuki Mains", Link: "https://discord.gg/mizuki"},
		{Fandom: "Genshin Impact", Name: "Ineffa Mains", Link: "https://discord.gg/ineffa"},
	}

	embeds, err := renderer.Render(PartnerBoardTemplate{}, partners)
	if err != nil {
		t.Fatalf("render returned error: %v", err)
	}
	if len(embeds) < 2 {
		t.Fatalf("expected at least 2 embeds after split, got %d", len(embeds))
	}

	if embeds[0].Title != defaultBoardTitle {
		t.Fatalf("unexpected first title: %q", embeds[0].Title)
	}
	if embeds[1].Title != defaultBoardTitle+defaultBoardContinuationTitleSufx {
		t.Fatalf("unexpected continuation title: %q", embeds[1].Title)
	}
	if !strings.Contains(embeds[1].Description, defaultSectionContinuationSuffix) {
		t.Fatalf("expected section continuation marker in second embed: %q", embeds[1].Description)
	}
}

func TestBoardRendererRender_InvalidLink(t *testing.T) {
	t.Parallel()

	renderer := NewBoardRenderer()
	_, err := renderer.Render(PartnerBoardTemplate{}, []PartnerRecord{
		{
			Fandom: "Genshin Impact",
			Name:   "Citlali Mains",
			Link:   "ftp://discord.gg/citlali",
		},
	})
	if err == nil {
		t.Fatal("expected error for invalid link")
	}
	if !errors.Is(err, ErrInvalidPartnerBoardEntry) {
		t.Fatalf("expected ErrInvalidPartnerBoardEntry, got %v", err)
	}
}

func TestBoardRendererRender_ExceedsEmbedLimit(t *testing.T) {
	t.Parallel()

	renderer := newBoardRendererWithLimits(90, 1)
	partners := []PartnerRecord{
		{Fandom: "Genshin Impact", Name: "Alice Mains", Link: "https://discord.gg/alice"},
		{Fandom: "Genshin Impact", Name: "Citlali Mains", Link: "https://discord.gg/citlali"},
		{Fandom: "Genshin Impact", Name: "Mavuika Mains", Link: "https://discord.gg/mavuika"},
		{Fandom: "Genshin Impact", Name: "Lauma Mains", Link: "https://discord.gg/lauma"},
	}

	_, err := renderer.Render(PartnerBoardTemplate{}, partners)
	if err == nil {
		t.Fatal("expected overflow error")
	}
	if !errors.Is(err, ErrPartnerBoardExceedsEmbedLimit) {
		t.Fatalf("expected ErrPartnerBoardExceedsEmbedLimit, got %v", err)
	}
}

func TestBoardRendererRender_EmptyState(t *testing.T) {
	t.Parallel()

	renderer := NewBoardRenderer()
	embeds, err := renderer.Render(PartnerBoardTemplate{
		Intro:          "Configured board",
		EmptyStateText: "No entries yet.",
	}, nil)
	if err != nil {
		t.Fatalf("render returned error: %v", err)
	}
	if len(embeds) != 1 {
		t.Fatalf("expected 1 embed for empty state, got %d", len(embeds))
	}
	if !strings.Contains(embeds[0].Description, "Configured board") {
		t.Fatalf("expected intro in description: %q", embeds[0].Description)
	}
	if !strings.Contains(embeds[0].Description, "No entries yet.") {
		t.Fatalf("expected empty-state text in description: %q", embeds[0].Description)
	}
}
