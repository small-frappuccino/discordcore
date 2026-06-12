package tickets

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
)

// RegisterComponents registers the ticket interactive components onto the core router.
func RegisterComponents(router *core.CommandRouter, svc *tickets.TicketService) {
	if router == nil || svc == nil {
		return
	}

	bindings := []core.InteractionRouteBinding{
		{
			Path:      "ticket_category_select",
			Domain:    "tickets",
			Component: core.ComponentHandlerFunc(svc.HandleCategorySelect),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_close",
			Domain:    "tickets",
			Component: core.ComponentHandlerFunc(svc.HandleClose),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_transcript",
			Domain:    "tickets",
			Component: core.ComponentHandlerFunc(svc.HandleTranscript),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_reopen",
			Domain:    "tickets",
			Component: core.ComponentHandlerFunc(svc.HandleReopen),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_delete",
			Domain:    "tickets",
			Component: core.ComponentHandlerFunc(svc.HandleDelete),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
	}

	for _, binding := range bindings {
		router.RegisterInteractionRoute(binding)
	}
}
