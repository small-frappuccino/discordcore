package tickets

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordgo"
	"golang.org/x/sync/errgroup"
)

// HandleTranscript handles transcript.
func (s *TicketService) HandleTranscript(ctx *legacycore.Context) error {
	channelID := ctx.Interaction.ChannelID

	// Resolve the target channel from config
	var auditChannelID string
	if ctx.GuildConfig != nil {
		auditChannelID = ctx.GuildConfig.Tickets.TranscriptChannelID
	}
	if auditChannelID == "" {
		// Fallback to replying in the current channel or returning an error
		return &legacycore.CommandError{Message: "Audit channel is not configured.", Ephemeral: true}
	}

	pr, pw := io.Pipe()
	enc := json.NewEncoder(pw)

	var eg errgroup.Group

	// Goroutine for fetching and encoding
	eg.Go(func() error {
		defer pw.Close()

		if _, err := pw.Write([]byte("[")); err != nil {
			return err
		}

		var beforeID string
		first := true

		for {
			messages, err := ctx.Session.ChannelMessages(channelID, 100, beforeID, "", "")
			if err != nil {
				pw.CloseWithError(err)
				return fmt.Errorf("fetch messages: %w", err)
			}

			if len(messages) == 0 {
				break
			}

			for _, msg := range messages {
				if !first {
					if _, err := pw.Write([]byte(",")); err != nil {
						return err
					}
				}
				first = false
				if err := enc.Encode(msg); err != nil {
					return err
				}
			}

			beforeID = messages[len(messages)-1].ID

			if len(messages) < 100 {
				break
			}
		}

		if _, err := pw.Write([]byte("]")); err != nil {
			return err
		}

		return nil
	})

	// Upload directly to the audit channel
	fileName := fmt.Sprintf("transcript-%s.json", channelID)

	// Acknowledge the interaction first because the upload might take time
	legacycore.NewResponseBuilder(ctx.Session).WithContext(ctx).Ephemeral().Success(ctx.Interaction, "Generating transcript...")

	_, err := ctx.Session.ChannelMessageSendComplex(auditChannelID, &discordgo.MessageSend{
		Content: fmt.Sprintf("Transcript for ticket <#%s> (Channel ID: %s)", channelID, channelID),
		Files: []*discordgo.File{
			{
				Name:        fileName,
				ContentType: "application/json",
				Reader:      pr,
			},
		},
	})

	if err != nil {
		return fmt.Errorf("upload transcript: %w", err)
	}

	// Wait for the encoder goroutine to finish safely
	if err := eg.Wait(); err != nil {
		return fmt.Errorf("encode transcript: %w", err)
	}

	// Update the original response
	msg := "Transcript saved to audit channel."
	_, err = ctx.Session.InteractionResponseEdit(ctx.Interaction.Interaction, &discordgo.WebhookEdit{
		Content: &msg,
	})

	return nil
}
