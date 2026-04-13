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
		"What song best represents the current mood you are in?",
		time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
	)

	if embed.Title != "Question Of The Day" {
		t.Fatalf("unexpected title: %+v", embed)
	}
	if embed.Footer == nil || embed.Footer.Text != "Official QOTD #80" {
		t.Fatalf("expected qotd footer metadata, got %+v", embed.Footer)
	}
	if embed.Timestamp != "2026-04-13T00:00:00Z" {
		t.Fatalf("expected publish timestamp to be carried, got %q", embed.Timestamp)
	}
	if len(embed.Fields) != 4 {
		t.Fatalf("expected prompt metadata fields, got %+v", embed.Fields)
	}
	if got := embed.Fields[1].Value; got != "`#80`" {
		t.Fatalf("expected question id field, got %q", got)
	}
	if !strings.Contains(embed.Description, "Use **Answer** below") {
		t.Fatalf("expected actionable prompt description, got %q", embed.Description)
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

	if embed.Title != "QOTD Answer" {
		t.Fatalf("unexpected title: %+v", embed)
	}
	if embed.Author == nil || embed.Author.Name != "Submitted by Alice" {
		t.Fatalf("expected author metadata, got %+v", embed.Author)
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != "https://cdn.discordapp.com/avatars/user-1/avatar-hash.png?size=256" {
		t.Fatalf("expected thumbnail avatar, got %+v", embed.Thumbnail)
	}
	if embed.Footer == nil || embed.Footer.Text != "QOTD response for question #80" {
		t.Fatalf("expected response footer metadata, got %+v", embed.Footer)
	}
	if len(embed.Fields) < 4 {
		t.Fatalf("expected richer answer fields, got %+v", embed.Fields)
	}
	if got := embed.Fields[0].Name; got != "Responder" {
		t.Fatalf("expected responder field first, got %+v", embed.Fields[0])
	}
	if !strings.Contains(embed.Description, "late-night synthwave") {
		t.Fatalf("expected answer text in description, got %q", embed.Description)
	}
}
