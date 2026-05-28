package roles

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// customEmojiPattern matches the Discord rich-text custom emoji form,
// e.g. <:name:123> or <a:name:123>.
var customEmojiPattern = regexp.MustCompile(`^<(a?):([A-Za-z0-9_]{2,32}):(\d{15,21})>$`)

// parseRolePanelButtonEmoji parses the value passed to the slash command
// `emoji` option. Accepted forms:
//   - empty string → no emoji
//   - <:name:id> / <a:name:id> → custom emoji (animated when the `a` flag is set)
//   - any other non-empty string → unicode emoji glyph
//
// The function returns the canonical fields ready to store on
// files.RolePanelButtonConfig. The caller is responsible for plumbing the
// fields into the button.
func parseRolePanelButtonEmoji(raw string) (name, id string, animated bool, err error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false, nil
	}

	if matches := customEmojiPattern.FindStringSubmatch(trimmed); matches != nil {
		return matches[2], matches[3], matches[1] == "a", nil
	}

	if strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">") {
		return "", "", false, fmt.Errorf("invalid custom emoji format (expected <:name:id> or <a:name:id>)")
	}

	if utf8.RuneCountInString(trimmed) > files.RolePanelLabelMaxLen {
		return "", "", false, fmt.Errorf("emoji glyph is too long")
	}
	return trimmed, "", false, nil
}
