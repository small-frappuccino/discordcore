package automod

import "github.com/diamondburned/arikawa/v3/discord"

// ExecutionActionMetadata holds the metadata for an AutoMod action.
type ExecutionActionMetadata struct {
	ChannelID     discord.ChannelID `json:"channel_id"`
	DurationSecs  int               `json:"duration_seconds"`
	CustomMessage string            `json:"custom_message"`
}

// ExecutionAction represents the action executed by AutoMod.
type ExecutionAction struct {
	Type     int                     `json:"type"`
	Metadata ExecutionActionMetadata `json:"metadata"`
}

// ExecutionEvent is the Discord payload for AUTO_MODERATION_ACTION_EXECUTION.
// This is defined locally because Arikawa v3 lacks support for this event.
type ExecutionEvent struct {
	GuildID              discord.GuildID   `json:"guild_id"`
	Action               ExecutionAction   `json:"action"`
	RuleID               discord.Snowflake `json:"rule_id"`
	RuleTriggerType      int               `json:"rule_trigger_type"`
	UserID               discord.UserID    `json:"user_id"`
	ChannelID            discord.ChannelID `json:"channel_id,omitempty"`
	MessageID            discord.MessageID `json:"message_id,omitempty"`
	AlertSystemMessageID discord.MessageID `json:"alert_system_message_id,omitempty"`
	Content              string            `json:"content"`
	MatchedKeyword       string            `json:"matched_keyword"`
	MatchedContent       string            `json:"matched_content"`
}
