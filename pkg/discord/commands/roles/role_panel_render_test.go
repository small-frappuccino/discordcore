package roles

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRolePanelButtonCustomIDRoundTrip(t *testing.T) {
	t.Parallel()
	roleID := "1380646673482518639"
	cid := rolePanelButtonCustomID(roleID)
	if !strings.HasPrefix(cid, rolePanelComponentRouteID+rolePanelCustomIDSeparator) {
		t.Fatalf("custom ID missing route prefix: %q", cid)
	}
	if got := rolePanelButtonRoleIDFromCustomID(cid); got != roleID {
		t.Fatalf("extracted role id = %q want %q", got, roleID)
	}
	if got := rolePanelButtonRoleIDFromCustomID("other:route|x"); got != "" {
		t.Fatalf("expected empty extraction for unrelated route, got %q", got)
	}
}

func TestRenderRolePanelEmbedHonorsConfig(t *testing.T) {
	t.Parallel()
	panel := files.RolePanelConfig{
		Key:         "pings",
		Title:       "⋆｡°✩ ! ✩°｡⋆ Pings! ⋆｡°✩ ! ✩°｡⋆",
		Description: "Please select some of our optional roles below!",
		Color:       16753104,
	}
	embed := renderRolePanelEmbed(panel)
	if embed.Title != panel.Title {
		t.Fatalf("embed title = %q want %q", embed.Title, panel.Title)
	}
	if embed.Description != panel.Description {
		t.Fatalf("embed description = %q want %q", embed.Description, panel.Description)
	}
	if embed.Color != panel.Color {
		t.Fatalf("embed color = %d want %d", embed.Color, panel.Color)
	}
}

func TestRenderRolePanelComponentsLayout(t *testing.T) {
	t.Parallel()
	panel := files.RolePanelConfig{
		Key: "pings",
		Buttons: []files.RolePanelButtonConfig{
			{RoleID: "1380646673482518639", Label: "Announcements", EmojiName: "clouud", EmojiID: "1378934415186464808"},
			{RoleID: "1380644552700067963", Label: "Partnerships", EmojiName: "pinkypeep", EmojiID: "1378934457876222113"},
			{RoleID: "1380646828294410342", Label: "Giveaway and Codes", EmojiName: "bunnysits", EmojiID: "1378934522783207506"},
			{RoleID: "1380646772698910862", Label: "server revive", EmojiName: "aliceu_hyvoff", EmojiID: "1381462246642941952"},
			{RoleID: "1391513234091151430", Label: "server events", EmojiName: "bugcatalice", EmojiID: "1390839396936454315"},
		},
	}
	components := renderRolePanelComponents(panel)
	if len(components) != 1 {
		t.Fatalf("expected one ActionsRow for five buttons, got %d", len(components))
	}
	row, ok := components[0].(discordgo.ActionsRow)
	if !ok {
		t.Fatalf("expected ActionsRow, got %T", components[0])
	}
	if len(row.Components) != 5 {
		t.Fatalf("expected 5 buttons, got %d", len(row.Components))
	}
	for i, c := range row.Components {
		button, ok := c.(discordgo.Button)
		if !ok {
			t.Fatalf("component %d is %T, want Button", i, c)
		}
		if button.Style != discordgo.SecondaryButton {
			t.Fatalf("button %d style = %d", i, button.Style)
		}
		if button.Emoji == nil || button.Emoji.ID == "" {
			t.Fatalf("button %d missing custom emoji", i)
		}
		expected := rolePanelButtonCustomID(panel.Buttons[i].RoleID)
		if button.CustomID != expected {
			t.Fatalf("button %d custom id = %q want %q", i, button.CustomID, expected)
		}
	}
}

func TestRenderRolePanelComponentsChunksAcrossRows(t *testing.T) {
	t.Parallel()
	buttons := make([]files.RolePanelButtonConfig, rolePanelMaxButtonsPerRow+2)
	for i := range buttons {
		buttons[i] = files.RolePanelButtonConfig{
			RoleID: "1" + strings.Repeat("0", 18-i),
			Label:  "Button",
		}
	}
	panel := files.RolePanelConfig{Key: "k", Buttons: buttons}
	components := renderRolePanelComponents(panel)
	if len(components) != 2 {
		t.Fatalf("expected 2 ActionsRows for %d buttons, got %d", len(buttons), len(components))
	}
	first := components[0].(discordgo.ActionsRow)
	second := components[1].(discordgo.ActionsRow)
	if len(first.Components) != rolePanelMaxButtonsPerRow {
		t.Fatalf("first row has %d buttons, want %d", len(first.Components), rolePanelMaxButtonsPerRow)
	}
	if len(second.Components) != 2 {
		t.Fatalf("second row has %d buttons, want 2", len(second.Components))
	}
}

func TestFormatRolePanelButtonForListContainsAllFields(t *testing.T) {
	t.Parallel()
	line := formatRolePanelButtonForList(files.RolePanelButtonConfig{
		RoleID:    "1380646673482518639",
		Label:     "Announcements",
		EmojiName: "clouud",
		EmojiID:   "1378934415186464808",
	})
	if !strings.Contains(line, "Announcements") {
		t.Fatalf("missing label in line: %q", line)
	}
	if !strings.Contains(line, "<@&1380646673482518639>") {
		t.Fatalf("missing role mention in line: %q", line)
	}
	if !strings.Contains(line, "<:clouud:1378934415186464808>") {
		t.Fatalf("missing custom emoji in line: %q", line)
	}
}
