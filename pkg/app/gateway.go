package app

import (
	"context"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type Connection interface {
	ReadMessage() ([]byte, error)
}

type DiscordGatewayImpl struct {
	connection Connection
	handler    core.InteractionHandler
	sem        *semaphore.Weighted
}

func NewDiscordGatewayImpl(conn Connection, maxConcurrent int64) *DiscordGatewayImpl {
	return &DiscordGatewayImpl{
		connection: conn,
		sem:        semaphore.NewWeighted(maxConcurrent),
	}
}

func (g *DiscordGatewayImpl) Connect(ctx context.Context) error {
	return nil
}

func (g *DiscordGatewayImpl) Disconnect(ctx context.Context) error {
	return nil
}

func (g *DiscordGatewayImpl) OnInteraction(handler core.InteractionHandler) {
	g.handler = handler
}

func (g *DiscordGatewayImpl) UpdatePresence(ctx context.Context, status string) error {
	return nil
}

func (g *DiscordGatewayImpl) ListenLoop(ctx context.Context) error {
	eg, egCtx := errgroup.WithContext(ctx)

	for {
		select {
		case <-egCtx.Done():
			return eg.Wait()
		default:
		}

		payload, err := g.connection.ReadMessage()
		if err != nil {
			break
		}

		// Enforce bounded ingress via x/sync/semaphore
		if err := g.sem.Acquire(egCtx, 1); err != nil {
			break
		}

		// Erradicado goroutine nua - orquestrado via errgroup.
		eg.Go(func() error {
			defer g.sem.Release(1)

			routeCtx, cancel := context.WithTimeout(egCtx, 5*time.Second)
			defer cancel()

			interaction := core.InteractionPayload{
				Data: payload,
			}

			if g.handler != nil {
				_ = g.handler.HandleInteraction(routeCtx, interaction)
			}
			return nil
		})
	}
	return eg.Wait()
}
