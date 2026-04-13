package qotd

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestBuildReplyThreadArtifactsCarryProvisioningNonce(t *testing.T) {
	t.Parallel()

	name := buildReplyThreadName(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC), "A very long answerer display name that should still keep the nonce suffix visible for recovery", "user-1", "abcdef1234567890")
	if got, want := name[len(name)-14:], "[qrp-abcdef12]"; got != want {
		t.Fatalf("expected reply thread name to preserve nonce suffix, got %q", name)
	}

	embed := buildReplyThreadEmbed("Question text", "https://discord.com/channels/g1/t1", "abcdef1234567890")
	if embed.Footer == nil || embed.Footer.Text != "QOTD reply ref: abcdef1234567890" {
		t.Fatalf("expected reply thread embed footer to carry the provisioning nonce, got %+v", embed.Footer)
	}

	message := &discordgo.Message{
		ID: "starter-message",
		Embeds: []*discordgo.MessageEmbed{
			embed,
		},
	}
	if !starterMessageMatchesReplyNonce(message, "QOTD reply ref: abcdef1234567890") {
		t.Fatal("expected starter message nonce matcher to recognize the embed footer marker")
	}
}

func TestBuildOfficialQuestionEmbedCarriesPromptMetadata(t *testing.T) {
	t.Parallel()

	embed := buildOfficialQuestionEmbed(
		80,
		62,
		"What song best represents the current mood you are in?",
	)

	if embed.Title != "Question Of The Day" {
		t.Fatalf("unexpected title: %+v", embed)
	}
	if embed.Footer == nil || embed.Footer.Text != "Official QOTD #80 • 62 available" {
		t.Fatalf("expected qotd footer metadata, got %+v", embed.Footer)
	}
	if embed.Timestamp != "" {
		t.Fatalf("expected publish timestamp to be omitted, got %q", embed.Timestamp)
	}
	if len(embed.Fields) != 0 {
		t.Fatalf("expected prompt metadata fields to be removed, got %+v", embed.Fields)
	}
	if !strings.Contains(embed.Description, "What song best represents") {
		t.Fatalf("expected question text in description, got %q", embed.Description)
	}
}

func TestBuildAnswerEmbedIncludesAvatarAndContext(t *testing.T) {
	t.Parallel()

	embed := buildAnswerEmbed(
		80,
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
	if embed.Footer == nil || embed.Footer.Text != "Official QOTD #80" {
		t.Fatalf("expected response footer metadata, got %+v", embed.Footer)
	}
	if embed.Timestamp != "" {
		t.Fatalf("expected response timestamp to be omitted, got %q", embed.Timestamp)
	}
	if len(embed.Fields) != 1 || embed.Fields[0].Name != "Question" {
		t.Fatalf("expected only question context field, got %+v", embed.Fields)
	}
	if !strings.Contains(embed.Description, "late-night synthwave") {
		t.Fatalf("expected answer text in description, got %q", embed.Description)
	}
	if strings.Contains(embed.Description, "Submitted answer") {
		t.Fatalf("expected submitted answer label to be removed, got %q", embed.Description)
	}
}
