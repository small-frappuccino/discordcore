package discordnotifications

import (
	"github.com/small-frappuccino/discordcore/pkg/notifications"
	"github.com/small-frappuccino/discordgo"
)

// DiscordPublisher implements the notifications.NotificationPublisher interface
// using a discordgo.Session.
type DiscordPublisher struct {
	session *discordgo.Session
}

// NewDiscordPublisher creates a new DiscordPublisher.
func NewDiscordPublisher(session *discordgo.Session) *DiscordPublisher {
	return &DiscordPublisher{
		session: session,
	}
}

// SendEmbed converts a domain embed and sends it via discordgo.
func (p *DiscordPublisher) SendEmbed(channelID string, embed *notifications.Embed) error {
	if embed == nil {
		return nil
	}

	discordEmbed := &discordgo.MessageEmbed{
		Title:       embed.Title,
		Description: embed.Description,
		Color:       embed.Color,
		Timestamp:   embed.Timestamp,
	}

	if embed.Author != nil {
		discordEmbed.Author = &discordgo.MessageEmbedAuthor{
			Name:    embed.Author.Name,
			IconURL: embed.Author.IconURL,
		}
	}

	if embed.Thumbnail != nil {
		discordEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: embed.Thumbnail.URL,
		}
	}

	for _, field := range embed.Fields {
		discordEmbed.Fields = append(discordEmbed.Fields, &discordgo.MessageEmbedField{
			Name:   field.Name,
			Value:  field.Value,
			Inline: field.Inline,
		})
	}

	if embed.Footer != nil {
		discordEmbed.Footer = &discordgo.MessageEmbedFooter{
			Text: embed.Footer.Text,
		}
	}

	_, err := p.session.ChannelMessageSendEmbed(channelID, discordEmbed)
	return err
}
