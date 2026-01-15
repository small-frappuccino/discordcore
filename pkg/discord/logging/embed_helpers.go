package logging

import (
	"fmt"
	"strings"
)

func formatUserLabel(username, userID string) string {
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

func formatUserRef(userID string) string {
	return formatUserLabel("", userID)
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
