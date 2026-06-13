package discordautomod

import (
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/ws"
)

type AutoModerationActionExecutionEvent struct {
	GuildID              discord.GuildID                   `json:"guild_id"`
	Action               discord.AutoModerationAction      `json:"action"`
	RuleID               discord.AutoModerationRuleID      `json:"rule_id"`
	RuleTriggerType      discord.AutoModerationTriggerType `json:"rule_trigger_type"`
	UserID               discord.UserID                    `json:"user_id"`
	ChannelID            discord.ChannelID                 `json:"channel_id,omitempty"`
	MessageID            discord.MessageID                 `json:"message_id,omitempty"`
	AlertSystemMessageID discord.MessageID                 `json:"alert_system_message_id,omitempty"`
	Content              string                            `json:"content"`
	MatchedKeyword       string                            `json:"matched_keyword"`
	MatchedContent       string                            `json:"matched_content"`
}

func (e *AutoModerationActionExecutionEvent) Op() ws.OpCode { return 0 }
func (e *AutoModerationActionExecutionEvent) EventType() ws.EventType {
	return "AUTO_MODERATION_ACTION_EXECUTION"
}

func init() {
	gateway.OpUnmarshalers.Add(func() ws.Event { return new(AutoModerationActionExecutionEvent) })
}
