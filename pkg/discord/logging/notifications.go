package logging

import (
	"fmt"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/theme"
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
		Color:       theme.AvatarChange(),
		Description: fmt.Sprintf("**%s** (<@%s>, `%s`)", change.Username, change.UserID, change.UserID),
	}

	// Always add old avatar thumbnail
	firstEmbed.Thumbnail = &discordgo.MessageEmbedThumbnail{
		URL: oldAvatarURL,
	}

	// Second embed - New avatar (always sent)
	secondEmbed := &discordgo.MessageEmbed{
		Title:     "...To",
		Color:     theme.AvatarChange(),
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
	joinAgeText := formatDurationSmart(accountAge)
	if joinAgeText == "" {
		joinAgeText = "— ago"
	} else {
		joinAgeText = joinAgeText + " ago"
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Member joined",
		Color:       theme.MemberJoin(),
		Description: fmt.Sprintf("**%s** (<@%s>, `%s`)", member.User.Username, member.User.ID, member.User.ID),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Account created",
				Value:  joinAgeText,
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMemberLeaveNotification envia notificação de saída de membro
func (ns *NotificationSender) SendMemberLeaveNotification(channelID string, member *discordgo.GuildMemberRemove, serverTime time.Duration, botTime time.Duration) error {
	embed := &discordgo.MessageEmbed{
		Title:       "Member left",
		Color:       theme.MemberLeave(),
		Description: fmt.Sprintf("**%s** (<@%s>, `%s`)", member.User.Username, member.User.ID, member.User.ID),
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	var fields []*discordgo.MessageEmbedField

	if serverTime > 0 {
		// Build human-readable server time with fallback when formatting yields empty (e.g., <1s)
		serverTimeText := formatDurationSmart(serverTime)
		if serverTimeText == "" {
			serverTimeText = "— ago"
		} else {
			serverTimeText = serverTimeText + " ago"
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Time on server",
			Value:  serverTimeText,
			Inline: true,
		})
	} else {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Time on server",
			Value:  "— ago",
			Inline: true,
		})
	}

	// if botTime > 0 {
	// 	fields = append(fields, &discordgo.MessageEmbedField{
	// 		Name:   "Bot time on server",
	// 		Value:  formatDuration(botTime),
	// 		Inline: true,
	// 	})
	// } else {
	// 	fields = append(fields, &discordgo.MessageEmbedField{
	// 		Name:   "Bot time on server",
	// 		Value:  "Unknown time",
	// 		Inline: true,
	// 	})
	// }

	if len(fields) > 0 {
		embed.Fields = fields
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMessageEditNotification envia notificação de edição de mensagem
func (ns *NotificationSender) SendMessageEditNotification(channelID string, original *task.CachedMessage, edited *discordgo.MessageUpdate) error {
	embed := &discordgo.MessageEmbed{
		Title:       "✏️ Message Edited",
		Color:       theme.MessageEdit(),
		Description: fmt.Sprintf("**%s** (<@%s>) edited a message in <#%s>", original.Author.Username, original.Author.ID, original.ChannelID),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Before",
				Value:  truncateString(original.Content, 1000),
				Inline: false,
			},
			{
				Name:   "After",
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
		Title:       "🗑️ Message Deleted",
		Color:       theme.MessageDelete(),
		Description: fmt.Sprintf("Message from **%s** (<@%s>) was deleted in <#%s>", deleted.Author.Username, deleted.Author.ID, deleted.ChannelID),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Content",
				Value:  truncateString(deleted.Content, 1000),
				Inline: false,
			},
			{
				Name:   "Deleted by",
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

// formatDurationFull mostra a duração no formato completo, omitindo unidades iniciais iguais a zero.
// Ex.: "0 days 2 minutes 5 seconds" -> "2 minutes 5 seconds"
//
//	"0 days 3 hours 0 minutes"   -> "3 hours 0 minutes"
func formatDurationFull(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	type comp struct {
		label string
		value int64
	}
	parts := []comp{
		{"days", days},
		{"hours", hours},
		{"minutes", minutes},
		{"seconds", seconds},
	}

	// Trim leading zero-valued units, but keep remaining units as-is
	for len(parts) > 1 && parts[0].value == 0 {
		parts = parts[1:]
	}

	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%d %s", p.value, p.label)
	}
	return out
}

// formatDurationSmart formata listando todas as unidades com valor diferente de zero (sem abreviações).
func formatDurationSmart(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int64(d.Seconds())
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	parts := []string{}

	if days > 0 {
		if days == 1 {
			parts = append(parts, "1 day")
		} else {
			parts = append(parts, fmt.Sprintf("%d days", days))
		}
	}
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	// Inclui seconds se > 0 ou se nenhuma outra unidade foi incluída (ex.: tudo zero)
	if seconds > 0 {
		if seconds == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", seconds))
		}
	}

	return strings.Join(parts, " ")
}

// formatDuration formata uma duração de tempo de forma legível
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "`            `"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 365 {
		years := days / 365
		remainingDays := days % 365
		if years == 1 {
			return fmt.Sprintf("1 year, %d days", remainingDays)
		}
		return fmt.Sprintf("%d years, %d days", years, remainingDays)
	}

	if days > 30 {
		months := days / 30
		remainingDays := days % 30
		if months == 1 {
			return fmt.Sprintf("1 month, %d days", remainingDays)
		}
		return fmt.Sprintf("%d months, %d days", months, remainingDays)
	}

	if days > 0 {
		if days == 1 {
			return fmt.Sprintf("1 day, %d hours", hours)
		}
		return fmt.Sprintf("%d days, %d hours", days, hours)
	}

	if hours > 0 {
		if hours == 1 {
			return fmt.Sprintf("1 hour, %d minutes", minutes)
		}
		return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
	}

	if minutes > 0 {
		if minutes == 1 {
			return "1 minutes"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}

	return "Less than 1 minute"
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
		Color:       theme.Info(),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMemberRoleUpdateNotification envia notificação de atualização de cargo (add/remove)
func (ns *NotificationSender) SendMemberRoleUpdateNotification(
	channelID string,
	actorID string, // quem realizou a ação (moderador/admin)
	targetID string, // usuário alvo
	targetUsername string, // nome do usuário alvo (opcional, pode vir vazio)
	roleID string, // ID do cargo afetado
	roleName string, // nome do cargo (fallback caso mention não seja desejada)
	action string, // "add" | "remove" | "added" | "removed"
) error {
	if channelID == "" || targetID == "" || (roleID == "" && roleName == "") {
		return nil
	}

	displayName := targetUsername
	if displayName == "" {
		displayName = targetID
	}

	act := "Updated"
	switch {
	case strings.EqualFold(action, "add") || strings.EqualFold(action, "added"):
		act = "Added"
	case strings.EqualFold(action, "remove") || strings.EqualFold(action, "removed"):
		act = "Removed"
	}

	roleDisplay := ""
	if roleID != "" {
		roleDisplay = "<@&" + roleID + ">"
	}
	if roleDisplay == "" && roleName != "" {
		roleDisplay = "`" + roleName + "`"
	}
	if roleDisplay == "" && roleID != "" {
		roleDisplay = "`" + roleID + "`"
	}

	desc := fmt.Sprintf("<@%s> %s role for **%s** (<@%s>)", actorID, strings.ToLower(act), displayName, targetID)
	embed := &discordgo.MessageEmbed{
		Title:       "Role updated",
		Color:       theme.Info(),
		Description: desc,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Role",
				Value:  roleDisplay,
				Inline: true,
			},
			{
				Name:   "Action",
				Value:  act,
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

func (ns *NotificationSender) SendErrorMessage(channelID, message string) error {
	embed := &discordgo.MessageEmbed{
		Title:       ErrErrorTitle,
		Description: message,
		Color:       theme.Error(),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

func (ns *NotificationSender) SendSuccessMessage(channelID, message string) error {
	embed := &discordgo.MessageEmbed{
		Title:       ErrSuccessTitle,
		Description: message,
		Color:       theme.Success(),
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
		Color:       theme.AutomodAction(),
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
