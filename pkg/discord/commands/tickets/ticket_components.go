package tickets

import (
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/tickets"
)

// RegisterComponents registers components.
func RegisterComponents(registry *core.CommandRegistry, svc *tickets.TicketService) {
	// The core.CommandRegistry doesn't directly register component routes yet.
	// We need to bind these to the core router using InteractionRouteBinding if that's the pattern,
	// or register them wherever Component handlers are aggregated.
	// Looking at the architecture, usually it is done at the app level.
	// Since I don't see a `RegisterComponent` on `core.CommandRegistry`, I'll expose a function
	// returning bindings.
}

// GetBindings gets bindings.
func GetBindings(svc *tickets.TicketService) []core.InteractionRouteBinding {
	return []core.InteractionRouteBinding{
		{
			Path:      "ticket_category_select",
			Component: core.ComponentHandlerFunc(svc.HandleCategorySelect),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_close",
			Component: core.ComponentHandlerFunc(svc.HandleClose),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_transcript",
			Component: core.ComponentHandlerFunc(svc.HandleTranscript),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_reopen",
			Component: core.ComponentHandlerFunc(svc.HandleReopen),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
		{
			Path:      "ticket_delete",
			Component: core.ComponentHandlerFunc(svc.HandleDelete),
			AckPolicy: core.InteractionAckPolicy{Mode: core.InteractionAckModeNone},
		},
	}
}
