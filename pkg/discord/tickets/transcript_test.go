//go:build integration

package tickets

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

func TestHandleTranscript_Streaming(t *testing.T) {
	guildID := "g1"
	ownerID := "u1"
	h := newTicketCommandTestHarness(t, guildID, ownerID)

	interaction := newTicketInteraction(guildID, "user-1", "ticket-123-id", "ticket_transcript", nil)

	ctx := &core.Context{
		Session:     h.session,
		Interaction: interaction,
		Config:      h.cm,
		GuildID:     guildID,
		UserID:      "user-1",
		GuildConfig: h.cm.GuildConfig(guildID),
		Logger:      log.GlobalLogger,
	}

	err := h.svc.HandleTranscript(ctx)
	if err != nil {
		t.Fatalf("HandleTranscript failed: %v", err)
	}

	h.rec.mu.Lock()
	defer h.rec.mu.Unlock()

	// Verify multipart file was sent
	filename := fmt.Sprintf("transcript-%s.json", "ticket-123-id")
	fileContent, ok := h.rec.multipartFiles[filename]
	if !ok {
		t.Fatalf("expected multipart file %q to be uploaded", filename)
	}

	// Verify JSON content
	var messages []discordgo.Message
	if err := json.Unmarshal(fileContent, &messages); err != nil {
		t.Fatalf("failed to unmarshal transcript json: %v", err)
	}

	// The mock server returns 2 messages if query "before" is empty.
	if len(messages) != 2 {
		t.Fatalf("expected exactly 2 messages in transcript, got %d", len(messages))
	}

	if messages[0].Content != "Hello" || messages[1].Content != "World" {
		t.Errorf("unexpected message content: %v", messages)
	}
}
