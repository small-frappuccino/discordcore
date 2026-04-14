package qotd

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestBuildOfficialQuestionEmbedCarriesPromptMetadata(t *testing.T) {
	t.Parallel()

	embed := buildOfficialQuestionEmbed(
		"Final Mix",
		62,
		"What song best represents the current mood you are in?",
		345,
	)

	if embed.Title != "☆ question!! ☆" {
		t.Fatalf("unexpected title: %+v", embed)
	}
	if embed.Footer == nil || embed.Footer.Text != "Deck: Final Mix | Question #345 -- 62 Cards Remaining" {
		t.Fatalf("expected qotd footer metadata, got %+v", embed.Footer)
	}
	if embed.Timestamp != "" {
		t.Fatalf("expected publish timestamp to be omitted, got %q", embed.Timestamp)
	}
	if len(embed.Fields) != 0 {
		t.Fatalf("expected prompt metadata fields to be removed, got %+v", embed.Fields)
	}
	if !strings.Contains(embed.Description, "what song best represents") {
		t.Fatalf("expected question text in description, got %q", embed.Description)
	}
}

func TestBuildAnswerEmbedIncludesAvatarAndContext(t *testing.T) {
	t.Parallel()

	embed := buildAnswerEmbed(
		"Final Mix",
		time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		"What song best represents the current mood you are in?",
		"https://discord.com/channels/g1/c1/m1",
		"Right now it is a late-night synthwave track.",
		"user-1",
		"Alice",
		"https://cdn.discordapp.com/avatars/user-1/avatar-hash.png?size=256",
	)

	if embed.Title != "" {
		t.Fatalf("expected title to be removed, got %+v", embed)
	}
	if embed.Author == nil || embed.Author.Name != "Alice" {
		t.Fatalf("expected author metadata, got %+v", embed.Author)
	}
	if embed.Thumbnail != nil {
		t.Fatalf("expected thumbnail avatar to be removed, got %+v", embed.Thumbnail)
	}
	if embed.Footer == nil || embed.Footer.Text != "Final Mix | 2026-04-03" {
		t.Fatalf("expected response footer metadata, got %+v", embed.Footer)
	}
	if embed.Timestamp != "" {
		t.Fatalf("expected response timestamp to be omitted, got %q", embed.Timestamp)
	}
	if len(embed.Fields) != 0 {
		t.Fatalf("expected question context field to be removed, got %+v", embed.Fields)
	}
	if !strings.Contains(embed.Description, "What song best") {
		t.Fatalf("expected question text inline in description, got %q", embed.Description)
	}
	if !strings.Contains(embed.Description, "late-night synthwave") {
		t.Fatalf("expected answer text in description, got %q", embed.Description)
	}
	if strings.Contains(embed.Description, "Submitted answer") {
		t.Fatalf("expected submitted answer label to be removed, got %q", embed.Description)
	}
}

func TestBuildOfficialPostNameMatchesDailyForumFormat(t *testing.T) {
	t.Parallel()

	got := buildOfficialPostName(
		time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
		"What's your go-to comfort drink?",
		1,
		"",
	)

	if got != "what's your go-to comfort drink? - qotd #1" {
		t.Fatalf("unexpected official post name: %q", got)
	}
}

func TestBuildThreadStateChannelEditOmitsFlagsWhenPinIsFalse(t *testing.T) {
	t.Parallel()

	edit := buildThreadStateChannelEdit(ThreadState{
		Locked:   true,
		Archived: false,
	})

	if edit == nil {
		t.Fatal("expected channel edit")
	}
	if edit.Flags != nil {
		t.Fatalf("expected flags to be omitted, got %+v", *edit.Flags)
	}
	if edit.Locked == nil || !*edit.Locked {
		t.Fatalf("expected locked=true, got %+v", edit.Locked)
	}
	if edit.Archived == nil || *edit.Archived {
		t.Fatalf("expected archived=false, got %+v", edit.Archived)
	}
}

func TestBuildThreadStateChannelEditIncludesPinnedFlagsWhenRequested(t *testing.T) {
	t.Parallel()

	edit := buildThreadStateChannelEdit(ThreadState{
		Pinned:   true,
		Locked:   true,
		Archived: true,
	})

	if edit == nil || edit.Flags == nil {
		t.Fatalf("expected pinned flags, got %+v", edit)
	}
	if *edit.Flags != discordgo.ChannelFlagPinned {
		t.Fatalf("expected pinned flag, got %+v", *edit.Flags)
	}
}
