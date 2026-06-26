package app

import "context"

// HexagonalRuntime represents the clean architecture container for the bot.
// It relies entirely on injected ports rather than concrete dependencies.
type HexagonalRuntime struct {
	instanceID string
	gateway    DiscordGateway
	store      ConfigStore
	queue      TaskQueue
	logger     Logger
	handler    InteractionHandler
}

// NewHexagonalRuntime injects the adapters via ports.
func NewHexagonalRuntime(
	instanceID string,
	gateway DiscordGateway,
	store ConfigStore,
	queue TaskQueue,
	logger Logger,
	handler InteractionHandler,
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
