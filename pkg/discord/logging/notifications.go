package logging

import (
	"fmt"
	"time"

	"github.com/alice-bnuy/discordcore/v2/pkg/files"
	"github.com/alice-bnuy/discordcore/v2/pkg/task"

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

// SendMemberJoinNotification envia notificação de entrada de membro
func (ns *NotificationSender) SendMemberJoinNotification(channelID string, member *discordgo.GuildMemberAdd, accountAge time.Duration) error {
	embed := &discordgo.MessageEmbed{
		Title:       "👋 Membro entrou",
		Color:       0x00FF00, // Verde
		Description: fmt.Sprintf("**%s** (<@%s>, `%s`)", member.User.Username, member.User.ID, member.User.ID),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Conta criada há",
				Value:  formatDuration(accountAge),
				Inline: true,
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: ns.buildAvatarURL(member.User.ID, member.User.Avatar),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMemberLeaveNotification envia notificação de saída de membro
func (ns *NotificationSender) SendMemberLeaveNotification(channelID string, member *discordgo.GuildMemberRemove, serverTime time.Duration) error {
	embed := &discordgo.MessageEmbed{
		Title:       "👋 Membro saiu",
		Color:       0xFF0000, // Vermelho
		Description: fmt.Sprintf("**%s** (<@%s>, `%s`)", member.User.Username, member.User.ID, member.User.ID),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: ns.buildAvatarURL(member.User.ID, member.User.Avatar),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if serverTime > 0 {
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Tempo no servidor",
				Value:  formatDuration(serverTime),
				Inline: true,
			},
		}
	} else {
		embed.Fields = []*discordgo.MessageEmbedField{
			{
				Name:   "Tempo no servidor",
				Value:  "Tempo desconhecido",
				Inline: true,
			},
		}
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMessageEditNotification envia notificação de edição de mensagem
func (ns *NotificationSender) SendMessageEditNotification(channelID string, original *task.CachedMessage, edited *discordgo.MessageUpdate) error {
	embed := &discordgo.MessageEmbed{
		Title:       "✏️ Mensagem editada",
		Color:       0xFFA500, // Laranja
		Description: fmt.Sprintf("**%s** (<@%s>) editou uma mensagem em <#%s>", original.Author.Username, original.Author.ID, original.ChannelID),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Antes",
				Value:  truncateString(original.Content, 1000),
				Inline: false,
			},
			{
				Name:   "Depois",
				Value:  truncateString(edited.Content, 1000),
				Inline: false,
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: ns.buildAvatarURL(original.Author.ID, original.Author.Avatar),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMessageDeleteNotification envia notificação de deleção de mensagem
func (ns *NotificationSender) SendMessageDeleteNotification(channelID string, deleted *task.CachedMessage, deletedBy string) error {
	embed := &discordgo.MessageEmbed{
		Title:       "🗑️ Mensagem deletada",
		Color:       0xFF0000, // Vermelho
		Description: fmt.Sprintf("Mensagem de **%s** (<@%s>) deletada em <#%s>", deleted.Author.Username, deleted.Author.ID, deleted.ChannelID),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Conteúdo",
				Value:  truncateString(deleted.Content, 1000),
				Inline: false,
			},
			{
				Name:   "Deletado por",
				Value:  deletedBy,
				Inline: true,
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: ns.buildAvatarURL(deleted.Author.ID, deleted.Author.Avatar),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// formatDuration formata uma duração de tempo de forma legível
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "Tempo desconhecido"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 365 {
		years := days / 365
		remainingDays := days % 365
		if years == 1 {
			return fmt.Sprintf("1 ano, %d dias", remainingDays)
		}
		return fmt.Sprintf("%d anos, %d dias", years, remainingDays)
	}

	if days > 30 {
		months := days / 30
		remainingDays := days % 30
		if months == 1 {
			return fmt.Sprintf("1 mês, %d dias", remainingDays)
		}
		return fmt.Sprintf("%d meses, %d dias", months, remainingDays)
	}

	if days > 0 {
		if days == 1 {
			return fmt.Sprintf("1 dia, %d horas", hours)
		}
		return fmt.Sprintf("%d dias, %d horas", days, hours)
	}

	if hours > 0 {
		if hours == 1 {
			return fmt.Sprintf("1 hora, %d minutos", minutes)
		}
		return fmt.Sprintf("%d horas, %d minutos", hours, minutes)
	}

	if minutes > 0 {
		if minutes == 1 {
			return "1 minuto"
		}
		return fmt.Sprintf("%d minutos", minutes)
	}

	return "Menos de 1 minuto"
}

// truncateString trunca uma string para um tamanho máximo
func truncateString(s string, maxLen int) string {
	if s == "" {
		return "*mensagem vazia*"
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

func (ns *NotificationSender) SendAutomodActionNotification(channelID string, e *discordgo.AutoModerationActionExecution) error {
	if e == nil || channelID == "" {
		return nil
	}

	title := "AutoMod action executed"
	desc := "A native AutoMod rule was triggered."
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: desc,
		Color:       0xFF5555,
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "User",
				Value:  "<@" + e.UserID + "> (`" + e.UserID + "`)",
				Inline: true,
			},
			{
				Name: "Channel",
				Value: func() string {
					if e.ChannelID != "" {
						return "<#" + e.ChannelID + ">"
					}
					return "(DM/unknown)"
				}(),
				Inline: true,
			},
		},
	}

	if e.RuleID != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Rule ID",
			Value:  "`" + e.RuleID + "`",
			Inline: true,
		})
	}
	if e.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Matched",
			Value:  "`" + e.MatchedKeyword + "`",
			Inline: true,
		})
	}

	content := e.Content
	if content == "" && e.MatchedContent != "" {
		content = e.MatchedContent
	}
	if content != "" {
		excerpt := content
		if len(excerpt) > 200 {
			excerpt = excerpt[:200] + "…"
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Excerpt",
			Value:  "```" + excerpt + "```",
			Inline: false,
		})
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}
