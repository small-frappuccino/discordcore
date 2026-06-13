package notifications

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/automod"
	"github.com/small-frappuccino/discordcore/pkg/files"

	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// ErrSuccessTitle defines err success title.
// ErrErrorTitle defines err error title.
// ErrSendMessage defines err send message.
const (
	ErrSendMessage  = "error sending message: %w"
	ErrErrorTitle   = "Error"
	ErrSuccessTitle = "Success"
)

// NotificationSender renders and sends operational log notifications (such as
// avatar-change embeds) to configured guild channels over a Discord session.
type NotificationSender struct {
	publisher NotificationPublisher
	logger    *slog.Logger
}

// NewNotificationSender news notification sender.
func NewNotificationSender(publisher NotificationPublisher, logger *slog.Logger) *NotificationSender {
	return &NotificationSender{
		publisher: publisher,
		logger:    logger,
	}
}

func FormatUserLabel(username, userID string) string {
	userID = strings.TrimSpace(userID)
	username = strings.TrimSpace(username)
	if userID == "" {
		if username != "" {
			return "**" + username + "**"
		}
		return "Unknown"
	}
	if username == "" {
		return "<@" + userID + "> (`" + userID + "`)"
	}
	return fmt.Sprintf("**%s** (<@%s>, `%s`)", username, userID, userID)
}

func FormatUserRef(userID string) string {
	return FormatUserLabel("", userID)
}

func formatChannelLabel(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "Unknown"
	}
	return "<#" + channelID + ">, `" + channelID + "`"
}

func formatRoleLabel(roleID, roleName string) string {
	roleID = strings.TrimSpace(roleID)
	roleName = strings.TrimSpace(roleName)
	if roleID != "" {
		return "<@&" + roleID + "> (`" + roleID + "`)"
	}
	if roleName != "" {
		return "`" + roleName + "`"
	}
	return "Unknown"
}

// SendAvatarChangeNotification sends avatar change notification.
func (ns *NotificationSender) SendAvatarChangeNotification(channelID string, change files.AvatarChange) error {
	// Check if username is empty, ignore if so
	if change.Username == "" {
		return nil
	}

	// Check if this is a real avatar change or just default avatar -> default avatar
	if change.OldAvatar == "" && change.NewAvatar == "" {
		// Both are default avatars, do not send notification
		return nil
	}

	embed := ns.createAvatarChangeEmbed(change)

	err := ns.publisher.SendEmbed(channelID, embed)
	if err != nil {
		return fmt.Errorf(ErrSendMessage, err)
	}

	return nil
}

func (ns *NotificationSender) createAvatarChangeEmbed(change files.AvatarChange) *Embed {
	// Build avatar URLs
	oldAvatarURL := ns.buildAvatarURL(change.UserID, change.OldAvatar)
	newAvatarURL := ns.buildAvatarURL(change.UserID, change.NewAvatar)

	embed := &Embed{
		Color:       theme.AvatarChange(),
		Description: "Avatar updated",
		Timestamp:   change.Timestamp.Format(time.RFC3339),
		Author: &EmbedAuthor{
			Name: "Avatar Updated",
		},
		Fields: []*EmbedField{
			{
				Name:   "User",
				Value:  FormatUserLabel(change.Username, change.UserID),
				Inline: true,
			},
			{
				Name:   "Previous Avatar",
				Value:  "[See previous avatar](" + oldAvatarURL + ")",
				Inline: true,
			},
		},
	}

	// Show only the new avatar in the embed media.
	embed.Thumbnail = &EmbedThumbnail{
		URL: newAvatarURL,
	}

	return embed
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
func (ns *NotificationSender) SendMemberJoinNotification(channelID string, member *MemberJoin, accountAge time.Duration) error {
	joinAgeText := formatDurationSmart(accountAge)
	if joinAgeText == "" {
		joinAgeText = "- ago"
	} else {
		joinAgeText = joinAgeText + " ago"
	}
	embed := &Embed{
		Title:       "Member Joined",
		Color:       theme.MemberJoin(),
		Description: FormatUserLabel(member.User.Username, member.User.ID),
		Fields: []*EmbedField{
			{
				Name:   "Account Created",
				Value:  joinAgeText,
				Inline: true,
			},
		},
		Thumbnail: &EmbedThumbnail{
			URL: ns.buildAvatarURL(member.User.ID, member.User.Avatar),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	err := ns.publisher.SendEmbed(channelID, embed)
	return err
}

// SendMemberLeaveNotification sends member leave notification
func (ns *NotificationSender) SendMemberLeaveNotification(channelID string, member *MemberLeave, serverTime time.Duration, botTime time.Duration) error {
	embed := &Embed{
		Title:       "Member Left",
		Color:       theme.MemberLeave(),
		Description: FormatUserLabel(member.User.Username, member.User.ID),
		Thumbnail: &EmbedThumbnail{
			URL: ns.buildAvatarURL(member.User.ID, member.User.Avatar),
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	var fields []*EmbedField

	if serverTime < 0 {
		fields = append(fields, &EmbedField{
			Name:   "Time on Server",
			Value:  "N/A",
			Inline: true,
		})
	} else if serverTime > 0 {
		// Build human-readable server time with fallback when formatting yields empty (e.g., <1s)
		serverTimeText := formatDurationSmart(serverTime)
		if serverTimeText == "" {
			serverTimeText = "- ago"
		} else {
			serverTimeText = serverTimeText + " ago"
		}
		fields = append(fields, &EmbedField{
			Name:   "Time on Server",
			Value:  serverTimeText,
			Inline: true,
		})
	} else {
		fields = append(fields, &EmbedField{
			Name:   "Time on Server",
			Value:  "- ago",
			Inline: true,
		})
	}

	if len(fields) > 0 {
		embed.Fields = fields
	}

	err := ns.publisher.SendEmbed(channelID, embed)
	return err
}

// SendMessageEditNotification sends message edit notification
func (ns *NotificationSender) SendMessageEditNotification(channelID string, original *CachedMessage, edited *MessageUpdate) error {
	// Build jump link (best effort)
	var jumpURL string
	if original.GuildID != "" && original.ChannelID != "" && edited.ID != "" {
		jumpURL = fmt.Sprintf("https://discord.com/channels/%s/%s/%s", original.GuildID, original.ChannelID, edited.ID)
	}

	userField := FormatUserLabel(original.Author.Username, original.Author.ID)
	channelField := formatChannelLabel(original.ChannelID)
	messageTime := original.Timestamp.Format("January 2, 2006 at 3:04 PM")

	desc := ""
	if jumpURL != "" {
		desc = "[Jump to message](" + jumpURL + ")"
	}

	embed := &Embed{
		Color:       theme.MessageEdit(),
		Description: desc,
		Author: &EmbedAuthor{
			Name:    "Message Edited",
			IconURL: ns.buildAvatarURL(original.Author.ID, original.Author.Avatar),
		},
		Fields: []*EmbedField{
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
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &EmbedFooter{
			Text: "Message ID: " + edited.ID,
		},
	}

	err := ns.publisher.SendEmbed(channelID, embed)
	return err
}

// SendMessageDeleteNotification sends message deletion notification
func (ns *NotificationSender) SendMessageDeleteNotification(channelID string, deleted *CachedMessage, deletedBy string) error {
	userField := FormatUserLabel(deleted.Author.Username, deleted.Author.ID)
	channelField := formatChannelLabel(deleted.ChannelID)
	messageTime := deleted.Timestamp.Format("January 2, 2006 at 3:04 PM")
	moderatorID := strings.TrimSpace(deletedBy)
	showModerator := moderatorID != ""
	if deleted.Author != nil && deleted.Author.ID != "" && moderatorID == deleted.Author.ID {
		showModerator = false
	}

	fields := []*EmbedField{
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
	}
	if showModerator {
		fields = append(fields, &EmbedField{
			Name:   "Responsible Moderator",
			Value:  FormatUserRef(moderatorID),
			Inline: true,
		})
	}

	embed := &Embed{
		Color: theme.MessageDelete(),
		Author: &EmbedAuthor{
			Name:    "Message Deleted",
			IconURL: ns.buildAvatarURL(deleted.Author.ID, deleted.Author.Avatar),
		},
		Fields:    fields,
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &EmbedFooter{
			Text: "Message ID: " + deleted.ID,
		},
	}

	err := ns.publisher.SendEmbed(channelID, embed)
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

// SendInfoMessage sends info message.
func (ns *NotificationSender) SendInfoMessage(channelID, message string) error {
	embed := &Embed{
		Title:       "Info",
		Description: message,
		Color:       theme.MemberRoleUpdate(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	err := ns.publisher.SendEmbed(channelID, embed)
	return err
}

// MemberRoleUpdateNotice describes a single role add/remove to announce.
type MemberRoleUpdateNotice struct {
	ChannelID      string // destination log channel
	ActorID        string // actor who performed the action (moderator/admin)
	TargetID       string // target user ID
	TargetUsername string // target username
	RoleID         string // affected role ID
	RoleName       string // role name (fallback when mention is not desired)
	Action         string // "add" | "remove" | "added" | "removed"
}

// SendMemberRoleUpdateNotification sends role update notification (add/remove)
func (ns *NotificationSender) SendMemberRoleUpdateNotification(notice MemberRoleUpdateNotice) error {
	if notice.ChannelID == "" || notice.TargetID == "" || (notice.RoleID == "" && notice.RoleName == "") {
		return nil
	}

	act := "Updated"
	switch {
	case strings.EqualFold(notice.Action, "add") || strings.EqualFold(notice.Action, "added"):
		act = "Added"
	case strings.EqualFold(notice.Action, "remove") || strings.EqualFold(notice.Action, "removed"):
		act = "Removed"
	}

	roleDisplay := formatRoleLabel(notice.RoleID, notice.RoleName)
	targetLabel := FormatUserLabel(notice.TargetUsername, notice.TargetID)
	actorLabel := FormatUserRef(notice.ActorID)
	embed := &Embed{
		Title:       "Role Updated",
		Color:       theme.MemberRoleUpdate(),
		Description: targetLabel,
		Fields: []*EmbedField{
			{
				Name:   "Actor",
				Value:  actorLabel,
				Inline: true,
			},
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

	err := ns.publisher.SendEmbed(notice.ChannelID, embed)
	return err
}

// SendErrorMessage sends error message.
func (ns *NotificationSender) SendErrorMessage(channelID, message string) error {
	embed := &Embed{
		Title:       ErrErrorTitle,
		Description: message,
		Color:       theme.MessageDelete(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	err := ns.publisher.SendEmbed(channelID, embed)
	return err
}

// SendSuccessMessage sends success message.
func (ns *NotificationSender) SendSuccessMessage(channelID, message string) error {
	embed := &Embed{
		Title:       ErrSuccessTitle,
		Description: message,
		Color:       theme.MemberJoin(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	err := ns.publisher.SendEmbed(channelID, embed)
	return err
}

// SendAutomodActionNotification sends automod action notification.
func (ns *NotificationSender) SendAutomodActionNotification(channelID string, e *automod.ActionExecution) error {
	if e == nil || channelID == "" {
		return nil
	}

	domainEmbed := automod.BuildAutomodEmbed(e)

	embed := &Embed{
		Title:       domainEmbed.Title,
		Description: domainEmbed.Description,
		Color:       domainEmbed.Color,
		Timestamp:   domainEmbed.Timestamp.Format(time.RFC3339),
	}
	for _, f := range domainEmbed.Fields {
		embed.Fields = append(embed.Fields, &EmbedField{
			Name:   f.Name,
			Value:  f.Value,
			Inline: f.Inline,
		})
	}

	err := ns.publisher.SendEmbed(channelID, embed)
	return err
}
