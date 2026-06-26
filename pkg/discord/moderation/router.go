package moderation

import (
	"context"
	"fmt"
)

type Router struct {
	registry app.FeatureRegistry
	service  *Service
}

// HandleInteraction pode rodar milhares de vezes concorrentemente.
// Como não há locks (Mutex) de escrita aqui, não há concorrência (race conditions).
func (r *Router) HandleInteraction(ctx context.Context, payload []byte) error {
	// Extração focada de JSON (Ex: fastjson.GetString(payload, "guild_id"))
	// Isso evita reflexão e o Garbage Collector fica intocado.
	guildID := extractStringFast(payload, "guild_id")
	appID := extractStringFast(payload, "application_id")

	// Validação Multi-Tenant: O bot tem a feature de moderação nesta guilda?
	botInstance, err := r.registry.ResolveOwner(ctx, guildID, "moderation")
	if err != nil || botInstance.ApplicationID != appID {
		return ErrFeatureUnauthorized
	}

	commandName := extractCommandNameFast(payload)

	// Cria o Job estrito na Stack (sem new(), sem alocação na Heap)
	job := ModerationJob{
		Ctx:          ctx,
		Bot:          botInstance,
		TargetUserID: extractStringFast(payload, "data", "options", "target_id"),
	}

	switch commandName {
	case "ban":
		job.Action = ActionBan
		job.Reason = extractStringFast(payload, "data", "options", "reason")
		job.DeleteDays = extractIntFast(payload, "data", "options", "delete_days")
	case "kick":
		job.Action = ActionKick
		job.Reason = extractStringFast(payload, "data", "options", "reason")
	default:
		return fmt.Errorf("comando desconhecido: %s", commandName)
	}

	targetUserID := extractOptionString(payload, "target_id")
	reason := extractOptionString(payload, "reason")

	switch commandName {
	case "ban":
		deleteDays := extractOptionInt(payload, "delete_days")

		return r.service.EnqueueTask(ModerationJob{
			Ctx:          ctx,
			Bot:          botInstance,
			Action:       ActionBan,
			TargetUserID: targetUserID,
			Reason:       reason,
			DeleteDays:   deleteDays,
		})
	case "kick":
		return r.service.EnqueueTask(ModerationJob{
			Ctx:          ctx,
			Bot:          botInstance,
			Action:       ActionKick,
			TargetUserID: targetUserID,
			Reason:       reason,
		})
	default:
		return fmt.Errorf("comando desconhecido: %s", commandName)
	}
}
