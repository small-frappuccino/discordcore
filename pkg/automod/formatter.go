package automod

import (
	"fmt"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/theme"
)

const automodExcerptMaxLen = 200

// BuildAutomodEmbed dispatches to the trigger-specific embed builder.
// MEMBER_PROFILE events have no message context and get a distinct embed; all
// other triggers reuse the message-keyword shape.
func BuildAutomodEmbed(e *ActionExecution) *Embed {
	if e.TriggerType == TriggerMemberProfile {
		return buildAutomodMemberProfileEmbed(e)
	}
	return buildAutomodMessageEmbed(e)
}

func buildAutomodMessageEmbed(e *ActionExecution) *Embed {
	desc := "Blocked content detected in a message."
	if e.GuildID != "" && e.ChannelID != "" && e.MessageID != "" {
		desc += "\n[Jump to message](https://discord.com/channels/" + e.GuildID + "/" + e.ChannelID + "/" + e.MessageID + ")"
	}
	embed := &Embed{
		Title:       "AutoMod • Message Blocked",
		Description: desc,
		Color:       theme.AutomodAction(),
		Timestamp:   time.Now(),
		Fields: []EmbedField{
			{Name: "User", Value: formatUserRef(e.UserID), Inline: true},
			{Name: "Channel", Value: automodChannelLabel(e.ChannelID), Inline: true},
		},
	}
	if label := automodTriggerLabel(e.TriggerType); label != "" {
		embed.Fields = append(embed.Fields, EmbedField{Name: "Trigger", Value: label, Inline: true})
	}
	if e.RuleID != "" {
		embed.Fields = append(embed.Fields, EmbedField{Name: "Rule ID", Value: "`" + e.RuleID + "`", Inline: true})
	}
	if e.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, EmbedField{Name: "Matched keyword", Value: "`" + e.MatchedKeyword + "`", Inline: true})
	}
	if excerpt := automodExcerpt(e); excerpt != "" {
		embed.Fields = append(embed.Fields, EmbedField{Name: "Excerpt", Value: "```" + excerpt + "```", Inline: false})
	}
	return embed
}

func buildAutomodMemberProfileEmbed(e *ActionExecution) *Embed {
	// The per-action Action.Type is intentionally not surfaced on the
	// embed: the package-level coalescing collapses Block Member
	// Interactions + Send Alert Message into a single embed per violation,
	// and "user is quarantined" is already conveyed by the description.
	embed := &Embed{
		Title:       "AutoMod • Member Profile Quarantined",
		Description: "Blocked words detected in this member's profile. The user is quarantined until the profile is updated.",
		Color:       theme.AutomodAction(),
		Timestamp:   time.Now(),
		Fields: []EmbedField{
			{Name: "Member", Value: formatUserRef(e.UserID), Inline: true},
			{Name: "Trigger", Value: "Member profile", Inline: true},
		},
	}
	if e.RuleID != "" {
		embed.Fields = append(embed.Fields, EmbedField{Name: "Rule ID", Value: "`" + e.RuleID + "`", Inline: true})
	}
	if e.MatchedKeyword != "" {
		embed.Fields = append(embed.Fields, EmbedField{Name: "Matched keyword", Value: "`" + e.MatchedKeyword + "`", Inline: true})
	}
	if excerpt := automodExcerpt(e); excerpt != "" {
		embed.Fields = append(embed.Fields, EmbedField{Name: "Offending fragment", Value: "```" + excerpt + "```", Inline: false})
	}
	return embed
}

func automodTriggerLabel(t int) string {
	switch t {
	case TriggerKeyword:
		return "Keyword"
	case TriggerHarmfulLink:
		return "Harmful link"
	case TriggerSpam:
		return "Spam"
	case TriggerKeywordPreset:
		return "Keyword preset"
	case TriggerMentionSpam:
		return "Mention spam"
	case TriggerMemberProfile:
		return "Member profile"
	}
	return ""
}

// automodActionLabel returns a human-readable label for a Discord AutoMod
// action type. The standard embed builders deliberately do not
// surface this label because the per-action stream is coalesced into one
// embed per violation. See the package-level "AutoMod logging" comment block.
func automodActionLabel(t int) string {
	switch t {
	case ActionBlockMessage:
		return "Block message"
	case ActionSendAlert:
		return "Send alert"
	case ActionTimeout:
		return "Timeout"
	case ActionBlockMemberInteraction:
		return "Block member interactions"
	}
	return ""
}

func automodChannelLabel(channelID string) string {
	if strings.TrimSpace(channelID) == "" {
		return "Unknown"
	}
	return formatChannelLabel(channelID)
}

func automodExcerpt(e *ActionExecution) string {
	content := strings.TrimSpace(e.Content)
	if content == "" {
		content = strings.TrimSpace(e.MatchedContent)
	}
	if content == "" {
		return ""
	}
	if len(content) > automodExcerptMaxLen {
		content = content[:automodExcerptMaxLen] + "..."
	}
	return sanitizeForCodeBlock(content)
}

// sanitizeForCodeBlock prevents breaking out of the code fence and removes backticks.
func sanitizeForCodeBlock(input string) string {
	// Replace backticks and normalize newlines for safer preview in a code block
	s := strings.ReplaceAll(input, "`", "'")
	// Discord code blocks tolerate newlines; keep them but trim excessive whitespace
	return strings.TrimSpace(s)
}

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
