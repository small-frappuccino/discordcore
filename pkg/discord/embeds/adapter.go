package discordembeds

import (
	"errors"
	embedspkg "github.com/small-frappuccino/discordcore/pkg/embeds"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func RenderCustomEmbed(ce files.CustomEmbedConfig) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	if title := strings.TrimSpace(ce.Title); title != "" {
		embed.Title = title
	}
	if desc := strings.TrimSpace(ce.Description); desc != "" {
		embed.Description = desc
	}
	if ce.Color > 0 {
		embed.Color = ce.Color
	}

	authorName := strings.TrimSpace(ce.AuthorName)
	authorIcon := strings.TrimSpace(ce.AuthorIconURL)
	if authorName != "" || authorIcon != "" {
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: authorIcon,
		}
	}

	footerText := strings.TrimSpace(ce.FooterText)
	footerIcon := strings.TrimSpace(ce.FooterIconURL)
	if footerText != "" || footerIcon != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text:    footerText,
			IconURL: footerIcon,
		}
	}

	if imageURL := strings.TrimSpace(ce.ImageURL); imageURL != "" {
		embed.Image = &discordgo.MessageEmbedImage{URL: imageURL}
	}
	if thumbnailURL := strings.TrimSpace(ce.ThumbnailURL); thumbnailURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: thumbnailURL}
	}

	if len(ce.Fields) > 0 {
		embed.Fields = make([]*discordgo.MessageEmbedField, 0, len(ce.Fields))
		for _, f := range ce.Fields {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return embed
}

// Adapter provides an implementation of pkg/embeds.Publisher using discordgo.
type Adapter struct {
	Session *discordgo.Session
}

// UpdatePosting edits an existing message with the provided custom embed layout.
func (a *Adapter) UpdatePosting(channelID, messageID string, embed files.CustomEmbedConfig) error {
	if a.Session == nil {
		return errors.New("discord session is nil")
	}
	dEmbed := RenderCustomEmbed(embed)
	msgEmbeds := []*discordgo.MessageEmbed{dEmbed}
	edit := &discordgo.MessageEdit{
		ID:      strings.TrimSpace(messageID),
		Channel: strings.TrimSpace(channelID),
		Embeds:  &msgEmbeds,
	}

	_, err := a.Session.ChannelMessageEditComplex(edit)
	if err != nil {
		var rest *discordgo.RESTError
		if errors.As(err, &rest) && rest.Message != nil {
			// 10003 is Unknown Channel, 10008 is Unknown Message
			if rest.Message.Code == 10003 || rest.Message.Code == 10008 {
				return embedspkg.ErrPostingMissing
			}
		}
		return err
	}
	return nil
}
