package moderation

import (
	"context"
	"fmt"
	"strconv"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type Router struct {
	registry core.FeatureRegistry
	service  *Service
}

func NewRouter(registry core.FeatureRegistry, service *Service) *Router {
	return &Router{
		registry: registry,
		service:  service,
	}
}

func (r *Router) HandleInteraction(ctx context.Context, payload core.InteractionPayload) error {
	return r.HandleRawInteraction(ctx, payload.Data)
}

func (r *Router) HandleRawInteraction(ctx context.Context, payload []byte) error {
	guildID := extractStringFast(payload, "guild_id")
	appID := extractStringFast(payload, "application_id")

	botInstance, err := r.registry.ResolveOwner(ctx, guildID, "moderation")
	if err != nil || botInstance.ApplicationID != appID {
		return ErrFeatureUnauthorized
	}

	commandName := extractCommandNameFast(payload)

	switch commandName {
	case "ban":
		targetIDStr := extractOptionString(payload, "target_id")
		targetID, _ := strconv.ParseUint(targetIDStr, 10, 64)
		reason := extractOptionString(payload, "reason")
		deleteDays := extractOptionInt(payload, "delete_days")

		job := ModerationJob{
			Bot:          botInstance,
			Action:       ActionBan,
			TargetUserID: targetID,
			Reason:       reason,
			DeleteDays:   deleteDays,
		}
		return r.service.EnqueueTask(job)

	case "kick":
		targetIDStr := extractOptionString(payload, "target_id")
		targetID, _ := strconv.ParseUint(targetIDStr, 10, 64)
		reason := extractOptionString(payload, "reason")

		job := ModerationJob{
			Bot:          botInstance,
			Action:       ActionKick,
			TargetUserID: targetID,
			Reason:       reason,
		}
		return r.service.EnqueueTask(job)

	case "massban":
		// Massban takes a list of IDs. Often passed as a comma-separated string or array.
		// Assuming it's a string option "target_ids" separated by spaces or commas.
		targetIDsStr := extractOptionString(payload, "target_ids")
		reason := extractOptionString(payload, "reason")
		deleteDays := extractOptionInt(payload, "delete_days")

		// Parse in a zero-allocation way by iterating over the string and enqueuing each.
		start := 0
		for i := 0; i <= len(targetIDsStr); i++ {
			if i == len(targetIDsStr) || targetIDsStr[i] == ' ' || targetIDsStr[i] == ',' {
				if start < i {
					idStr := targetIDsStr[start:i]
					if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
						job := ModerationJob{
							Bot:          botInstance,
							Action:       ActionBan,
							TargetUserID: id,
							Reason:       reason,
							DeleteDays:   deleteDays,
						}
						// Enqueue each separately.
						if err := r.service.EnqueueTask(job); err != nil {
							// If queue is full, we load-shed the rest of the massban.
							return err
						}
					}
				}
				start = i + 1
			}
		}
		return nil

	default:
		return fmt.Errorf("comando desconhecido: %s", commandName)
	}
}
