package logging

import (
	"regexp"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

var mentionRe = regexp.MustCompile(`<@!?(\d+)>`)

// parseEntryExitBackfillMessage extracts (eventType, userID) from messages in a welcome/entry-leave channel.
// It supports:
// - Alice embeds (sent by our bot) with title "Member Joined" / "Member Left".
// - Plain text messages containing a user mention and configurable keywords.
func parseEntryExitBackfillMessage(m *discordgo.Message, botID string, rc files.RuntimeConfig) (string, string, bool) {
	if m == nil {
		return "", "", false
	}

	// 1) Our own embed format (legacy backfill)
	if m.Author != nil && botID != "" && m.Author.ID == botID && len(m.Embeds) > 0 {
		for _, em := range m.Embeds {
			if em == nil || em.Title == "" || em.Description == "" {
				continue
			}
			title := strings.ToLower(strings.TrimSpace(em.Title))
			if title != "member joined" && title != "member left" {
				continue
			}

			// Extract user ID from description: "**username** (<@id>, `id`)"
			desc := em.Description
			userID := ""
			if i := strings.Index(desc, "`"); i >= 0 {
				if j := strings.Index(desc[i+1:], "`"); j >= 0 {
					userID = desc[i+1 : i+1+j]
				}
			}
			if userID == "" {
				continue
			}
			if title == "member joined" {
				return "join", userID, true
			}
			return "leave", userID, true
		}
	}

	// 2) Configurable plain text
	content := strings.TrimSpace(m.Content)
	if content == "" {
		return "", "", false
	}

	lc := strings.ToLower(content)

	welcomeStr := "welcome to alice mains!"
	if rc.MimuWelcomeString != "" {
		welcomeStr = strings.ToLower(rc.MimuWelcomeString)
	}
	goodbyeStr := "has left the server... :("
	if rc.MimuGoodbyeString != "" {
		goodbyeStr = strings.ToLower(rc.MimuGoodbyeString)
	}

	if welcomeStr != "" && strings.HasPrefix(lc, welcomeStr) {
		mm := mentionRe.FindStringSubmatch(content)
		if len(mm) >= 2 {
			return "join", mm[1], true
		}
	}
	if goodbyeStr != "" && strings.HasSuffix(lc, goodbyeStr) {
		mm := mentionRe.FindStringSubmatch(content)
		if len(mm) >= 2 {
			return "leave", mm[1], true
		}
	}

	mm := mentionRe.FindStringSubmatch(content)
	if len(mm) < 2 {
		return "", "", false
	}
	userID := mm[1]
	if userID == "" {
		return "", "", false
	}

	// Keep it intentionally permissive; this is best-effort reconstruction.
	if strings.Contains(lc, "goodbye") {
		return "leave", userID, true
	}
	if strings.Contains(lc, "welcome") {
		return "join", userID, true
	}
	return "", "", false
}
