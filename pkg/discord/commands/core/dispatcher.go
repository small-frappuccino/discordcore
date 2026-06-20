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

	cmd, found := d.registry.Get(data.Name)
	if !found {
		slog.Warn("Command not found in registry", slog.String("command", data.Name))
		return nil
	}

	ctx := NewInteractionContext(d.client, &event.InteractionEvent)

	if err := cmd.Handler(ctx); err != nil {
		slog.Error("Command handler failed",
			slog.String("command", data.Name),
			slog.String("error", err.Error()),
		)
		return &OperationalError{Op: "handler_" + data.Name, Err: err}
	}

	return nil
}

func (d *Dispatcher) DispatchRaw(payload []byte) error {
	var event gateway.InteractionCreateEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}
	return d.Dispatch(&event)
}
