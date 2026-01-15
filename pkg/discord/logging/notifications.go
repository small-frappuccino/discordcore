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
	// Handle both empty string and "default" sentinel for default avatars
	if avatarHash == "" || avatarHash == "default" {
		// Generate Discord default avatar based on user ID
		// Discord uses: (user_id >> 22) % 6 for new users
		// For compatibility, we'll use a simplified version
		var userIDNum uint64
		for _, char := range userID {
			if char >= '0' && char <= '9' {
				userIDNum = userIDNum*10 + uint64(char-'0')
			}
		}

		// Use a formula that gives the correct result for this user
		avatarIndex := int((userIDNum >> 22) % 6)

		return fmt.Sprintf("https://cdn.discordapp.com/embed/avatars/%d.png", avatarIndex)
	}

	// Check if it's an animated avatar (starts with 'a_')
	format := "png"
	if len(avatarHash) > 2 && avatarHash[:2] == "a_" {
		format = "gif"
	}

	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s?size=128", userID, avatarHash, format)
}

// SendMemberJoinNotification sends member join notification
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

// SendMemberLeaveNotification sends member leave notification
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

// SendMessageEditNotification sends message edit notification
func (ns *NotificationSender) SendMessageEditNotification(channelID string, original *task.CachedMessage, edited *discordgo.MessageUpdate) error {
	// Build jump link (best effort)
	var jumpURL string
	if original.GuildID != "" && original.ChannelID != "" && edited.ID != "" {
		jumpURL = fmt.Sprintf("https://discord.com/channels/%s/%s/%s", original.GuildID, original.ChannelID, edited.ID)
	}

	// Resolve channel name (best effort; avoid API call by using session state)
	channelName := ""
	_ = channelName
	if ns.session != nil && ns.session.State != nil {
		if ch, _ := ns.session.State.Channel(original.ChannelID); ch != nil {
			channelName = ch.Name
		}
	}

	userField := fmt.Sprintf("**%s** (<@%s>, `%s`)", original.Author.Username, original.Author.ID, original.Author.ID)
	channelField := fmt.Sprintf("<#%s>, `%s`", original.ChannelID, original.ChannelID)
	messageTime := original.Timestamp.Format("January 2, 2006 at 3:04 PM")

	desc := ""
	if jumpURL != "" {
		desc = "[Jump to message](" + jumpURL + ")"
	}

	embed := &discordgo.MessageEmbed{
		Title:       "✏️ Message Edited",
		Color:       theme.MessageEdit(),
		Description: desc,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "User",
				Value:  userField,
				Inline: true,
			},
			{
				Name:   "Channel",
				Value:  channelField,
				Inline: true,
			},
			{
				Name:   "Message Timestamp",
				Value:  messageTime,
				Inline: true,
			},
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
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Message ID: " + edited.ID,
		},
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMessageDeleteNotification sends message deletion notification
func (ns *NotificationSender) SendMessageDeleteNotification(channelID string, deleted *task.CachedMessage, deletedBy string) error {
	// Resolve channel name (best effort; avoid API call by using session state)
	channelName := ""
	_ = channelName
	if ns.session != nil && ns.session.State != nil {
		if ch, _ := ns.session.State.Channel(deleted.ChannelID); ch != nil {
			channelName = ch.Name
		}
	}

	userField := fmt.Sprintf("**%s** (<@%s>, `%s`)", deleted.Author.Username, deleted.Author.ID, deleted.Author.ID)
	channelField := fmt.Sprintf("<#%s>, `%s`", deleted.ChannelID, deleted.ChannelID)
	messageTime := deleted.Timestamp.Format("January 2, 2006 at 3:04 PM")

	embed := &discordgo.MessageEmbed{
		Title: "🗑️ Message Deleted",
		Color: theme.MessageDelete(),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "User",
				Value:  userField,
				Inline: true,
			},
			{
				Name:   "Channel",
				Value:  channelField,
				Inline: true,
			},
			{
				Name:   "Message Timestamp",
				Value:  messageTime,
				Inline: true,
			},
			{
				Name:   "Message",
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
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Message ID: " + deleted.ID,
		},
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// formatDurationFull shows the full duration, omitting leading zero-valued units.
// e.g.: "0 days 2 minutes 5 seconds" -> "2 minutes 5 seconds"
//
// "0 days 3 hours 0 minutes"   -> "3 hours 0 minutes"
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

// formatDurationSmart lists all non-zero units (no abbreviations).
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
	// Include seconds if > 0 or if no other unit was included (e.g., everything else is zero)
	if seconds > 0 {
		if seconds == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", seconds))
		}
	}

	return strings.Join(parts, " ")
}

// formatDuration formats a time duration in a human-readable way
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

// truncateString truncates a string to a maximum length
func truncateString(s string, maxLen int) string {
	if s == "" {
		return "*empty message*"
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
		Color:       theme.MemberRoleUpdate(),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

// SendMemberRoleUpdateNotification sends role update notification (add/remove)
func (ns *NotificationSender) SendMemberRoleUpdateNotification(
	channelID string,
	actorID string, // the actor who performed the action (moderator/admin)
	targetID string, // target user
	targetUsername string, // target user's name (optional, may be empty)
	roleID string, // affected role ID
	roleName string, // role name (fallback when mention is not desired)
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

	desc := fmt.Sprintf("<@%s> %s role for **%s** (<@%s>, `%s`)", actorID, strings.ToLower(act), displayName, targetID, targetID)
	embed := &discordgo.MessageEmbed{
		Title:       "Role updated",
		Color:       theme.MemberRoleUpdate(),
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
		Color:       theme.MessageDelete(),
	}

	_, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
	return err
}

func (ns *NotificationSender) SendSuccessMessage(channelID, message string) error {
	embed := &discordgo.MessageEmbed{
		Title:       ErrSuccessTitle,
		Description: message,
		Color:       theme.MemberJoin(),
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
						return "<#" + e.ChannelID + ">, `" + e.ChannelID + "`"
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
