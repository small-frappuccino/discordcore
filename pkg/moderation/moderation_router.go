package moderation

import (
	"context"
	"errors"
	"strconv"

	"github.com/buger/jsonparser"
	"github.com/small-frappuccino/discordcore/pkg/core"
)

var ErrUnknownCommand = errors.New("comando desconhecido")

type Router struct {
	registry core.FeatureRegistry
}

func NewRouter(registry core.FeatureRegistry) *Router {
	return &Router{
		registry: registry,
	}
}

// JobIterator provides zero-allocation iteration over moderation jobs.
type JobIterator struct {
	payload     []byte
	botInstance core.BotInstance
	commandName []byte
}

func (it JobIterator) All(yield func(ModerationJob) bool) {
	if string(it.commandName) == "ban" {
		targetID, reason, deleteDays := parseBanCommand(it.payload)
		job := ModerationJob{
			Reason:       reason,
			Bot:          it.botInstance,
			TargetUserID: targetID,
			DeleteDays:   deleteDays,
			Action:       ActionBan,
		}
		yield(job)
	} else if string(it.commandName) == "kick" {
		targetID, reason := parseKickCommand(it.payload)
		job := ModerationJob{
			Reason:       reason,
			Bot:          it.botInstance,
			TargetUserID: targetID,
			Action:       ActionKick,
		}
		yield(job)
	} else if string(it.commandName) == "massban" {
		targetIDsStr, reason, deleteDays := parseMassBanCommand(it.payload)
		start := 0
		for i := 0; i <= len(targetIDsStr); i++ {
			if i == len(targetIDsStr) || targetIDsStr[i] == ' ' || targetIDsStr[i] == ',' {
				if start < i {
					idStr := targetIDsStr[start:i]
					if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
						job := ModerationJob{
							Reason:       reason,
							Bot:          it.botInstance,
							TargetUserID: id,
							DeleteDays:   deleteDays,
							Action:       ActionBan,
						}
						if !yield(job) {
							return
						}
					}
				}
				start = i + 1
			}
		}
	}
}

// ParseInteraction yields zero-allocation ModerationJobs parsed from the raw interaction payload.
func (r *Router) ParseInteraction(ctx context.Context, payload []byte) (JobIterator, error) {
	guildID := extractStringFast(payload, "guild_id")
	appID := extractStringFast(payload, "application_id")
	commandNameBytes := extractCommandNameFastBytes(payload)

	var feature core.Feature
	// Using b2s or string() in switch is 0 alloc in go for byte slices
	switch string(commandNameBytes) {
	case "ban", "massban":
		feature = core.FeatureBan
	case "kick":
		feature = core.FeatureKick
	default:
		return JobIterator{}, ErrUnknownCommand
	}

	botInstance, err := r.registry.ResolveOwner(ctx, guildID, feature)
	if err != nil || botInstance.ApplicationID != appID {
		return JobIterator{}, ErrFeatureUnauthorized
	}

	return JobIterator{
		payload:     payload,
		botInstance: botInstance,
		commandName: commandNameBytes,
	}, nil
}

func extractBytesFast(payload []byte, keys ...string) []byte {
	val, typ, _, err := jsonparser.Get(payload, keys...)
	if err != nil {
		return nil
	}
	if typ == jsonparser.String {
		return val
	}
	return val
}

func extractCommandNameFastBytes(payload []byte) []byte {
	return extractBytesFast(payload, "data", "name")
}
