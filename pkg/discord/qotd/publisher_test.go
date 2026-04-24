package qotd

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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

func TestBuildOfficialPostNameMatchesDailyForumFormat(t *testing.T) {
	t.Parallel()

	got := buildOfficialPostName(
		time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC),
		1,
		"",
	)

	if got != "question of the day #1" {
		t.Fatalf("unexpected official post name: %q", got)
	}
}

func TestTruncateEmbedTextPreservesUTF8Boundaries(t *testing.T) {
	t.Parallel()

	got := truncateEmbedText(strings.Repeat("á", 5), 4)
	if got != "á..." {
		t.Fatalf("unexpected truncated embed text: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8 after truncation, got %q", got)
	}
}

func TestTruncateThreadNamePreservesUTF8Boundaries(t *testing.T) {
	t.Parallel()

	got := truncateThreadName(strings.Repeat("😀", 100) + "!")
	want := strings.Repeat("😀", 97) + "..."
	if got != want {
		t.Fatalf("unexpected truncated thread name: %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8 after truncation, got %q", got)
	}
}

func TestBuildOfficialPostStarterMessageOmitsAnswerButton(t *testing.T) {
	t.Parallel()

	embed := buildOfficialQuestionEmbed(
		"Final Mix",
		62,
		"What song best represents the current mood you are in?",
		345,
	)
	message := buildOfficialPostStarterMessage(embed)

	if message == nil || len(message.Embeds) != 1 {
		t.Fatalf("expected one embed starter message, got %+v", message)
	}
	if len(message.Components) != 0 {
		t.Fatalf("expected no message components on official post starter message, got %+v", message.Components)
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
