package embeds

import (
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func renderCustomEmbed(ce files.CustomEmbedConfig) *discordgo.MessageEmbed {
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
