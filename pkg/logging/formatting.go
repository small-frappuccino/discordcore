package logging

import (
	"fmt"
	"strings"
	"time"
)

// FormatUserLabel returns a standardized markdown label for a user.
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

// FormatUserRef returns a standardized mention reference for a user.
func FormatUserRef(userID string) string {
	return FormatUserLabel("", userID)
}

// FormatChannelLabel returns a standardized markdown mention for a channel.
func FormatChannelLabel(channelID string) string {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return "Unknown"
	}
	return "<#" + channelID + ">, `" + channelID + "`"
}

// FormatRoleLabel returns a standardized markdown mention for a role.
func FormatRoleLabel(roleID, roleName string) string {
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

// FormatDurationFull shows the full duration, omitting leading zero-valued units.
func FormatDurationFull(d time.Duration) string {
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

// FormatDurationSmart lists all non-zero units (no abbreviations).
func FormatDurationSmart(d time.Duration) string {
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
	if seconds > 0 {
		if seconds == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", seconds))
		}
	}

	return strings.Join(parts, " ")
}

// FormatDuration formats a time duration in a human-readable way.
func FormatDuration(d time.Duration) string {
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

// TruncateString truncates a string to a maximum length.
func TruncateString(s string, maxLen int) string {
	if s == "" {
		return "*empty message*"
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
