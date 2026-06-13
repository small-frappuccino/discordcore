package partners

import (
	"errors"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/partners"
	"github.com/small-frappuccino/discordgo"
)

const (
	discordErrUnknownChannel = 10003
	discordErrUnknownMessage = 10008
)

// DiscordgoBoardPublisher implements partners.BoardPublisher using discordgo.
type DiscordgoBoardPublisher struct {
	session *discordgo.Session
}

// NewDiscordgoBoardPublisher creates a new board publisher.
func NewDiscordgoBoardPublisher(session *discordgo.Session) *DiscordgoBoardPublisher {
	return &DiscordgoBoardPublisher{session: session}
}

// Publish implements partners.BoardPublisher.
func (p *DiscordgoBoardPublisher) Publish(guildID string, postings []files.CustomEmbedPostingConfig, embeds []partners.BoardEmbed) partners.PartnerSyncResult {
	var result partners.PartnerSyncResult
	if len(postings) == 0 {
		return result
	}

	var dgoEmbeds []*discordgo.MessageEmbed
	for _, e := range embeds {
		dgoEmbeds = append(dgoEmbeds, &discordgo.MessageEmbed{
			Title:       e.Title,
			Description: e.Description,
			Color:       e.Color,
			Footer: &discordgo.MessageEmbedFooter{
				Text: e.FooterText,
			},
		})
	}

	for _, posting := range postings {
		edit := &discordgo.MessageEdit{
			ID:      strings.TrimSpace(posting.MessageID),
			Channel: strings.TrimSpace(posting.ChannelID),
			Embeds:  &dgoEmbeds,
		}
		var err error
		if posting.WebhookID != "" && posting.WebhookToken != "" {
			err = p.editWebhookMessage(edit, posting.WebhookID, posting.WebhookToken)
		} else {
			err = p.editMessage(edit)
		}
		
		if err == nil {
			result.Edited++
			continue
		}

		if p.isPartnerPostingMissingError(err) {
			result.Dropped = append(result.Dropped, posting)
			continue
		}

		result.Failed = append(result.Failed, partners.PartnerSyncFailure{Posting: posting, Err: err})
	}

	return result
}

func (p *DiscordgoBoardPublisher) editMessage(edit *discordgo.MessageEdit) error {
	if p.session == nil {
		return errors.New("discord session is nil")
	}
	_, err := p.session.ChannelMessageEditComplex(edit)
	return err
}

func (p *DiscordgoBoardPublisher) editWebhookMessage(edit *discordgo.MessageEdit, webhookID, webhookToken string) error {
	if p.session == nil {
		return errors.New("discord session is nil")
	}
	_, err := p.session.WebhookMessageEdit(webhookID, webhookToken, edit.ID, &discordgo.WebhookEdit{
		Embeds: edit.Embeds,
	})
	return err
}

func (p *DiscordgoBoardPublisher) isPartnerPostingMissingError(err error) bool {
	var rest *discordgo.RESTError
	if !errors.As(err, &rest) || rest.Message == nil {
		return false
	}
	switch rest.Message.Code {
	case discordErrUnknownChannel, discordErrUnknownMessage:
		return true
	}
	return false
}
