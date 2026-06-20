package tickets

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
)

// RegisterComponents registers the ticket interactive components onto the core router.
func RegisterComponents(router *legacycore.CommandRouter, svc *tickets.TicketService) {
	if router == nil || svc == nil {
		return
	}

	bindings := []legacycore.InteractionRouteBinding{
		{
			Path:      "ticket_category_select",
			Domain:    "tickets",
			Component: legacycore.ComponentHandlerFunc(svc.HandleCategorySelect),
			AckPolicy: legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeNone},
		},
		{
			Path:      "ticket_close",
			Domain:    "tickets",
			Component: legacycore.ComponentHandlerFunc(svc.HandleClose),
			AckPolicy: legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeNone},
		},
		{
			Path:      "ticket_transcript",
			Domain:    "tickets",
			Component: legacycore.ComponentHandlerFunc(svc.HandleTranscript),
			AckPolicy: legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeNone},
		},
		{
			Path:      "ticket_reopen",
			Domain:    "tickets",
			Component: legacycore.ComponentHandlerFunc(svc.HandleReopen),
			AckPolicy: legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeNone},
		},
		{
			Path:      "ticket_delete",
			Domain:    "tickets",
			Component: legacycore.ComponentHandlerFunc(svc.HandleDelete),
			AckPolicy: legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeNone},
		},
	}

	for _, binding := range bindings {
		router.RegisterInteractionRoute(binding)
	}
}
