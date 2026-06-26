package app

import (
	"context"
)

// DomainService abstracts the business logic execution for a given route.
type DomainService interface {
	ExecuteCommand(ctx context.Context, payload InteractionPayload) error
}

// HexagonalCommandHandler acts as a pure dispatcher.
// It receives generic payloads, routes them in O(1) time, and injects execution into a TaskQueue.
type HexagonalCommandHandler struct {
	queue  TaskQueue
	router map[string]DomainService
	logger Logger
}

// NewHexagonalCommandHandler initializes the dispatcher with its routing table and async queue.
func NewHexagonalCommandHandler(queue TaskQueue, logger Logger, router map[string]DomainService) *HexagonalCommandHandler {
	return &HexagonalCommandHandler{
		queue:  queue,
		router: router,
		logger: logger,
	}
}

// HandleInteraction implements the InteractionHandler port.
func (ch *HexagonalCommandHandler) HandleInteraction(ctx context.Context, payload InteractionPayload) error {
	service, exists := ch.router[payload.RoutePath]
	if !exists {
		ch.logger.Info("No handler found for route", "routePath", payload.RoutePath)
		return nil
	}

	// Dispatch responsibility to an isolated async context via TaskQueue.
	// This prevents the main Goroutine from blocking.
	return ch.queue.Enqueue(ctx, func(taskCtx context.Context) error {
		// Domain logic parsing and execution.
		return service.ExecuteCommand(taskCtx, payload)
	})
}
