package core

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
)

type Dispatcher struct {
	client   *api.Client
	registry *CommandRegistry
}

func NewDispatcher(client *api.Client, registry *CommandRegistry) *Dispatcher {
	return &Dispatcher{
		client:   client,
		registry: registry,
	}
}

func (d *Dispatcher) Dispatch(event *gateway.InteractionCreateEvent) error {
	data, ok := event.Data.(*discord.CommandInteraction)
	if !ok || data == nil {
		return nil
	}

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
