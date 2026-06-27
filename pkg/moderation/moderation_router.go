package moderation

import (
	"context"
	"errors"
	"strconv"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

// ErrUnknownCommand is a package sentinel for the dispatch reject branch.
// The command name is attacker-controlled and the branch is amplifiable, so it
// must stay alloc-free: a static sentinel avoids the fmt.Errorf boxing that
// would force commandName + *errorString onto the heap per bad payload.
var ErrUnknownCommand = errors.New("comando desconhecido")

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
		targetID, reason, deleteDays := parseBanCommand(payload)

		job := ModerationJob{
			Reason:       reason,
			Bot:          botInstance,
			TargetUserID: targetID,
			DeleteDays:   deleteDays,
			Action:       ActionBan,
		}
		return r.service.EnqueueTask(job)

	case "kick":
		targetID, reason := parseKickCommand(payload)

		job := ModerationJob{
			Reason:       reason,
			Bot:          botInstance,
			TargetUserID: targetID,
			Action:       ActionKick,
		}
		return r.service.EnqueueTask(job)

	case "massban":
		targetIDsStr, reason, deleteDays := parseMassBanCommand(payload)

		// Parse in a zero-allocation way by iterating over the string and enqueuing each.
		start := 0
		for i := 0; i <= len(targetIDsStr); i++ {
			if i == len(targetIDsStr) || targetIDsStr[i] == ' ' || targetIDsStr[i] == ',' {
				if start < i {
					idStr := targetIDsStr[start:i]
					if id, err := strconv.ParseUint(idStr, 10, 64); err == nil {
						job := ModerationJob{
							Reason:       reason,
							Bot:          botInstance,
							TargetUserID: id,
							DeleteDays:   deleteDays,
							Action:       ActionBan,
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
		return ErrUnknownCommand
	}
}
