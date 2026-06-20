package moderation

import (
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
)

// ModerationLogPayload defines the cross-boundary data structure
// utilized by the pure logic layer to instruct the Discord service
// to build and broadcast an audit embed.
type ModerationLogPayload struct {
	Action      string
	TargetID    string
	TargetLabel string
	Reason      string
	RequestedBy string
	Extra       string
	CaseNumber  int64
	CaseID      string
	ActorID     string
}

// BuildModerationEmbed statically constructs a Discord message embed
// representing a moderation audit event.
func BuildModerationEmbed(payload ModerationLogPayload, color discord.Color, timestamp time.Time) discord.Embed {
	action := strings.TrimSpace(payload.Action)
	targetID := strings.TrimSpace(payload.TargetID)
	targetLabel := strings.TrimSpace(payload.TargetLabel)

	targetValue := "Unknown"
	switch {
	case targetID == "" && targetLabel != "":
		targetValue = targetLabel
	case targetID != "" && (targetLabel == "" || targetLabel == targetID):
		targetValue = fmt.Sprintf("<@%s> (`%s`)", targetID, targetID)
	case targetID != "":
		targetValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, targetID, targetID)
	}

	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "No reason provided"
	}

	fields := []discord.EmbedField{
		{Name: "Action", Value: action, Inline: true},
	}

	if payload.CaseID != "" {
		fields = append(fields, discord.EmbedField{Name: "Case ID", Value: "`" + payload.CaseID + "`", Inline: true})
	}

	actorID := payload.ActorID
	if actorID == "" {
		actorID = "Unknown"
	}

	fields = append(fields,
		discord.EmbedField{Name: "Target", Value: targetValue, Inline: true},
		discord.EmbedField{Name: "Actor", Value: fmt.Sprintf("<@%s> (`%s`)", actorID, actorID), Inline: true},
	)

	if payload.RequestedBy != "" {
		fields = append(fields, discord.EmbedField{
			Name:   "Requested By",
			Value:  fmt.Sprintf("<@%s> (`%s`)", payload.RequestedBy, payload.RequestedBy),
			Inline: true,
		})
	}

	fields = append(fields, discord.EmbedField{
		Name:   "Reason",
		Value:  reason,
		Inline: false,
	})

	if payload.Extra != "" {
		fields = append(fields, discord.EmbedField{
			Name:   "Details",
			Value:  payload.Extra,
			Inline: false,
		})
	}

	return discord.Embed{
		Title:       "Moderation Action",
		Color:       color,
		Description: fmt.Sprintf("Moderation action executed by <@%s>.", actorID),
		Fields:      fields,
		Timestamp:   discord.NewTimestamp(timestamp),
	}
}
