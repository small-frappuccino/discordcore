package roles

import (
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const (
	RolePanelComponentRouteID = "roles_panel:toggle"
	rolePanelCustomIDSeparator = "|"
)

func RolePanelButtonCustomID(roleID string) string {
	return RolePanelComponentRouteID + rolePanelCustomIDSeparator + strings.TrimSpace(roleID)
}

func RolePanelButtonRoleIDFromCustomID(customID string) string {
	prefix := RolePanelComponentRouteID + rolePanelCustomIDSeparator
	if !strings.HasPrefix(customID, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(customID, prefix))
}

func FormatRolePanelButtonForList(b files.RolePanelButtonConfig) string {
	var sb strings.Builder
	if b.HasEmoji() {
		sb.WriteString(formatButtonEmojiDisplay(b))
		sb.WriteString(" ")
	}
	sb.WriteString("")
	sb.WriteString(b.Label)
	sb.WriteString(" → <@&")
	sb.WriteString(b.RoleID)
	sb.WriteString(">")
	return sb.String()
}

func formatButtonEmojiDisplay(b files.RolePanelButtonConfig) string {
	name := strings.TrimSpace(b.EmojiName)
	if id := strings.TrimSpace(b.EmojiID); id != "" {
		prefix := ":"
		if b.EmojiAnimated {
			prefix = "a:"
		}
		if name == "" {
			name = "emoji"
		}
		return "<" + prefix + name + ":" + id + ">"
	}
	return name
}
