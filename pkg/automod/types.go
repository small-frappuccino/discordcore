package automod

import "time"

// Trigger types mirror Discord's AutoModerationRuleTriggerType but remain SDK-agnostic.
const (
	TriggerKeyword       = 1
	TriggerHarmfulLink   = 2
	TriggerSpam          = 3
	TriggerKeywordPreset = 4
	TriggerMentionSpam   = 5
	TriggerMemberProfile = 6
)

// Action types mirror Discord's AutoModerationActionType but remain SDK-agnostic.
const (
	ActionBlockMessage           = 1
	ActionSendAlert              = 2
	ActionTimeout                = 3
	ActionBlockMemberInteraction = 4
)

// ActionExecution represents a domain-agnostic AutoMod execution event.
type ActionExecution struct {
	GuildID              string
	ChannelID            string
	UserID               string
	RuleID               string
	ActionType           int
	TriggerType          int
	MessageID            string
	AlertSystemMessageID string
	MatchedKeyword       string
	Content              string
	MatchedContent       string
}

// Embed represents a domain-agnostic rich message embed.
type Embed struct {
	Title       string
	Description string
	Color       int
	Timestamp   time.Time
	Fields      []EmbedField
}

// EmbedField represents a field in a domain-agnostic embed.
type EmbedField struct {
	Name   string
	Value  string
	Inline bool
}
