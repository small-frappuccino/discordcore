package qotd

import (
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
