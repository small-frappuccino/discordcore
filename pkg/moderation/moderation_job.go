package moderation

import (
	"github.com/small-frappuccino/discordcore/pkg/core"
)

type ActionType uint8

const (
	ActionBan ActionType = iota
	ActionKick
)

type ModerationJob struct {
	Reason       string
	Bot          *core.BotInstance
	TargetUserID uint64
	DeleteDays   int
	Action       ActionType
}
