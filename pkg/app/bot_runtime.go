package app

import (
	"context"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

// HexagonalRuntime represents the clean architecture container for the bot.
// It relies entirely on injected ports rather than concrete dependencies.
type HexagonalRuntime struct {
	instanceID string
	gateway    core.DiscordGateway
	store      core.ConfigStore
	queue      core.TaskQueue
	logger     core.Logger
	handler    core.InteractionHandler
}

// NewHexagonalRuntime injects the adapters via ports.
func NewHexagonalRuntime(
	instanceID string,
	gateway core.DiscordGateway,
	store core.ConfigStore,
	queue core.TaskQueue,
	logger core.Logger,
	handler core.InteractionHandler,
) *HexagonalRuntime {
	return &HexagonalRuntime{
		instanceID: instanceID,
		gateway:    gateway,
		store:      store,
		queue:      queue,
		logger:     logger,
		handler:    handler,
	}
}

// Run attaches the generic handler to the gateway and connects.
func (h *HexagonalRuntime) Run(ctx context.Context) error {
	h.gateway.OnInteraction(h.handler)
	return h.gateway.Connect(ctx)
}
