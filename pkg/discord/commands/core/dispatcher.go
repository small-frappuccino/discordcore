package core

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
)

// Dispatcher routes incoming Discord interaction events to their corresponding command handlers.
// It bridges the raw Arikawa gateway event stream with the abstracted command registry.
type Dispatcher struct {
	client   *api.Client
	registry *CommandRegistry
}

// NewDispatcher constructs a new Dispatcher leveraging the provided API client and registry.
// The registry should ideally be sealed before binding the dispatcher to live gateway events
// to prevent concurrent mutation during active dispatch cycles.
func NewDispatcher(client *api.Client, registry *CommandRegistry) *Dispatcher {
	return &Dispatcher{
		client:   client,
		registry: registry,
	}
}

// Dispatch evaluates a typed interaction creation event and invokes the matching handler.
// It guarantees isolated execution boundaries per command, capturing and logging panics
// or operational errors returned by the underlying handler implementation.
func (d *Dispatcher) Dispatch(event *gateway.InteractionCreateEvent) error {
	// Fast-path rejection for non-command interactions (e.g. message components, modals).
	// These require separate routing domains beyond slash command registration.
	data, ok := event.Data.(*discord.CommandInteraction)
	if !ok || data == nil {
		return nil
	}

	// Extract standard contextual identifiers for structured logging tracing.
	// Fallback to "unknown" prevents nil pointer dereferences during DM interactions.
	guildID := "unknown"
	if event.GuildID.IsValid() {
		guildID = event.GuildID.String()
	}
	userID := "unknown"
	if event.Member != nil {
		userID = event.Member.User.ID.String()
	} else if event.User != nil {
		userID = event.User.ID.String()
	}

	cmd, found := d.registry.Get(data.Name)
	if !found {
		slog.Warn("Command not found in registry",
			slog.String("operation", "dispatch.not_found"),
			slog.String("command", data.Name),
			slog.String("interactionID", event.ID.String()),
			slog.String("guildID", guildID),
			slog.String("userID", userID),
		)
		return nil
	}

	ctx := NewInteractionContext(d.client, &event.InteractionEvent)

	if err := cmd.Handler(ctx); err != nil {
		slog.Error("Command handler failed",
			slog.String("operation", "dispatch.handler_failed"),
			slog.String("command", data.Name),
			slog.String("interactionID", event.ID.String()),
			slog.String("guildID", guildID),
			slog.String("userID", userID),
			slog.String("error", err.Error()),
			slog.String("syntheticFailure", "500"),
		)
		return &OperationalError{Op: "handler_" + data.Name, Err: err}
	}

	return nil
}

// DispatchRaw decodes a raw JSON payload into an interaction event and routes it.
// This supports serverless or direct webhook-based interaction ingestion models where
// events bypass the standard gateway websocket connection.
func (d *Dispatcher) DispatchRaw(payload []byte) error {
	var event gateway.InteractionCreateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		slog.Error("Failed to parse interaction payload",
			slog.String("operation", "dispatch.parse_failed"),
			slog.String("error", err.Error()),
			slog.String("syntheticFailure", "400"),
		)
		return fmt.Errorf("failed to parse payload: %w", err)
	}
	return d.Dispatch(&event)
}
