package app

import (
	"context"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type Connection interface {
	// ReadMessage now takes a pre-allocated buffer from the pool to avoid allocations.
	ReadMessage(buf []byte) ([]byte, error)
}

type DiscordGatewayImpl struct {
	connection Connection
	handler    core.InteractionHandler
	workers    int64
	bufPool    sync.Pool
}

func NewDiscordGatewayImpl(conn Connection, maxConcurrent int64) *DiscordGatewayImpl {
	return &DiscordGatewayImpl{
		connection: conn,
		workers:    maxConcurrent,
		bufPool: sync.Pool{
			New: func() interface{} {
				// Allocate a 4KB buffer for standard Gateway payloads
				buf := make([]byte, 4096)
				return &buf
			},
		},
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
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan *[]byte, g.workers)

	var wg sync.WaitGroup
	wg.Add(int(g.workers))
	for i := int64(0); i < g.workers; i++ {
		go g.drain(workerCtx, jobs, &wg)
	}

	g.read(workerCtx, jobs)
	wg.Wait()
	return nil
}

func (g *DiscordGatewayImpl) read(ctx context.Context, jobs chan<- *[]byte) {
	defer close(jobs)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		bufPtr := g.bufPool.Get().(*[]byte)
		payload, err := g.connection.ReadMessage(*bufPtr)
		if err != nil {
			g.bufPool.Put(bufPtr)
			return
		}

		// Update the slice length to match the read payload length while keeping capacity
		// Assuming ReadMessage returns a sub-slice of the original buffer if it fits.
		*bufPtr = payload

		select {
		case jobs <- bufPtr:
		case <-ctx.Done():
			// Important to return the buffer if we drop it here due to context cancellation
			g.bufPool.Put(bufPtr)
			return
		}
	}
}

func (g *DiscordGatewayImpl) drain(ctx context.Context, jobs <-chan *[]byte, wg *sync.WaitGroup) {
	defer wg.Done()
	for bufPtr := range jobs {
		// No context allocation in the hot path, just process inline
		if g.handler != nil {
			_ = g.handler.HandleInteraction(ctx, core.InteractionPayload{Data: *bufPtr})
		}

		// Zero the buffer logically and return it to the pool
		// (jsonparser only reads, no need to actually zero the bytes, just reset len)
		// But the rule says: "Pools must be zeroed explicitly before returning to prevent residual state corruption"
		buf := *bufPtr
		clear(buf[:cap(buf)]) // Otimização para zeroar todo o array subjacente usando memclr
		*bufPtr = buf[:cap(buf)]
		g.bufPool.Put(bufPtr)
	}
}
