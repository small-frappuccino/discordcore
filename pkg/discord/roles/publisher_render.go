package discordroles

import (
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/roles"
	"github.com/small-frappuccino/discordgo"
)

const (
	rolePanelMaxButtonsPerRow = 5
)

// RolePanelButtonCustomID builds the persistent component custom ID for
// one role-toggle button. The role ID is the only piece of state the
// handler needs: Discord guarantees only the bot can author a component,
// so the encoded role ID is trusted at click time.
func RenderEmbed(panel files.RolePanelConfig) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{}
	if title := strings.TrimSpace(panel.Title); title != "" {
		embed.Title = title
	}
	if desc := strings.TrimSpace(panel.Description); desc != "" {
		embed.Description = desc
	}
	if panel.Color > 0 {
		embed.Color = panel.Color
	}

	authorName := strings.TrimSpace(panel.AuthorName)
	authorIcon := strings.TrimSpace(panel.AuthorIconURL)
	if authorName != "" || authorIcon != "" {
		embed.Author = &discordgo.MessageEmbedAuthor{
			Name:    authorName,
			IconURL: authorIcon,
		}
	}

	footerText := strings.TrimSpace(panel.FooterText)
	footerIcon := strings.TrimSpace(panel.FooterIconURL)
	if footerText != "" || footerIcon != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{
			Text:    footerText,
			IconURL: footerIcon,
		}
	}

	if imageURL := strings.TrimSpace(panel.ImageURL); imageURL != "" {
		embed.Image = &discordgo.MessageEmbedImage{URL: imageURL}
	}
	if thumbnailURL := strings.TrimSpace(panel.ThumbnailURL); thumbnailURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: thumbnailURL}
	}

	if len(panel.Fields) > 0 {
		embed.Fields = make([]*discordgo.MessageEmbedField, 0, len(panel.Fields))
		for _, f := range panel.Fields {
			embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}

	return embed
}

// renderRolePanelComponents builds the ActionRow / Button tree for one
// panel. Buttons are emitted in the order they were configured so
// operators see the same layout they set up. Empty button slices return
// nil so callers can omit components without sending an empty action row.
func RenderComponents(panel files.RolePanelConfig) []discordgo.MessageComponent {
	if len(panel.Buttons) == 0 {
		return nil
	}

	rows := make([]discordgo.MessageComponent, 0, (len(panel.Buttons)+rolePanelMaxButtonsPerRow-1)/rolePanelMaxButtonsPerRow)
	current := discordgo.ActionsRow{Components: make([]discordgo.MessageComponent, 0, rolePanelMaxButtonsPerRow)}
	for _, b := range panel.Buttons {
		if len(current.Components) == rolePanelMaxButtonsPerRow {
			rows = append(rows, current)
			current = discordgo.ActionsRow{Components: make([]discordgo.MessageComponent, 0, rolePanelMaxButtonsPerRow)}
		}
		current.Components = append(current.Components, buildRolePanelButton(b))
	}
	if len(current.Components) > 0 {
		rows = append(rows, current)
	}
	return rows
}

func buildRolePanelButton(b files.RolePanelButtonConfig) discordgo.Button {
	button := discordgo.Button{
		Style:    discordgo.SecondaryButton,
		Label:    strings.TrimSpace(b.Label),
		CustomID: roles.RolePanelButtonCustomID(b.RoleID),
	}
	if b.HasEmoji() {
		button.Emoji = &discordgo.ComponentEmoji{
			Name:     strings.TrimSpace(b.EmojiName),
			ID:       strings.TrimSpace(b.EmojiID),
			Animated: b.EmojiAnimated,
		}
	}
	return button
}

// FormatRolePanelButtonForList renders one button as a single text line
// for the /roles panel button list output.
