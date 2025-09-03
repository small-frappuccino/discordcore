package logging

import (
	"fmt"

	"github.com/alice-bnuy/discordcore/v2/internal/files"

	"github.com/bwmarrin/discordgo"
)

const (
	ErrSendMessage  = "error sending message: %w"
	ErrErrorTitle   = "❌ Error"
	ErrSuccessTitle = "✅ Success"
)

type NotificationSender struct {
	session *discordgo.Session
}

func NewNotificationSender(session *discordgo.Session) *NotificationSender {
	return &NotificationSender{
		session: session,
	}
}

func (ns *NotificationSender) SendAvatarChangeNotification(channelID string, change files.AvatarChange) error {
	// Check if username is empty, ignore if so
	if change.Username == "" {
		return nil
	}

	// Check if this is a real avatar change or just default avatar → default avatar
	if change.OldAvatar == "" && change.NewAvatar == "" {
		// Both are default avatars, do not send notification
		return nil
	}

	embeds := ns.createAvatarChangeEmbeds(change)

	_, err := ns.session.ChannelMessageSendEmbeds(channelID, embeds)
	if err != nil {
		return fmt.Errorf(ErrSendMessage, err)
	}

	return nil
}

func (ns *NotificationSender) createAvatarChangeEmbeds(change files.AvatarChange) []*discordgo.MessageEmbed {
	// Build avatar URLs
	oldAvatarURL := ns.buildAvatarURL(change.UserID, change.OldAvatar)
	newAvatarURL := ns.buildAvatarURL(change.UserID, change.NewAvatar)

	// First embed - Always keep the title "Avatar changed"
	firstEmbed := &discordgo.MessageEmbed{
		Title:       "Avatar changed",
		Color:       0x5865F2, // Discord blue color
		Description: fmt.Sprintf("**%s** (<@%s>, `%s`)", change.Username, change.UserID, change.UserID),
	}

	// Always add old avatar thumbnail
	firstEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
		URL: oldAvatarURL,
	}

	// Second embed - New avatar (always sent)
	secondEmbed := &discordgo.MessageEmbed{
		Title:     "...To",
		Color:     0x5865F2, // Discord blue color
		Timestamp: change.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Always add new avatar thumbnail
	secondEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
		URL: newAvatarURL,
	}

	return []*discordgo.MessageEmbed{firstEmbed, secondEmbed}
}

func (ns *NotificationSender) buildAvatarURL(userID, avatarHash string) string {
	if avatarHash == "" {
		// Generate Discord default avatar based on user ID
		// Discord uses: (user_id >> 22) % 6 for new users
		// For compatibility, we'll use a simplified version
		var userIDNum int64
		for _, char := range userID {
			if char >= '0' && char <= '9' {
				userIDNum = userIDNum*10 + int64(char-'0')
			}
		}

		// Use a formula that gives the correct result for this user
		avatarIndex := (userIDNum >> 22) % 6

		return fmt.Sprintf("https://cdn.discordapp.com/embed/avatars/%d.png", avatarIndex)
	}

	// Check if it's an animated avatar (starts with 'a_')
	format := "png"
	if len(avatarHash) > 2 && avatarHash[:2] == "a_" {
		format = "gif"
	}

	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s?size=128", userID, avatarHash, format)
}

func (ns *NotificationSender) SendInfoMessage(channelID, message string) error {
	embed := &discordgo.MessageEmbed{
		Title:       "ℹ️ Info",
		Description: message,
		Color:       0x0099ff, // Blue
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

func (ns *NotificationSender) SendErrorMessage(channelID, message string) error {
	embed := &discordgo.MessageEmbed{
		Title:       ErrErrorTitle,
		Description: message,
		Color:       0xff0000, // Red
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

func (ns *NotificationSender) SendSuccessMessage(channelID, message string) error {
	embed := &discordgo.MessageEmbed{
		Title:       ErrSuccessTitle,
		Description: message,
		Color:       0x00ff00, // Green
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}
