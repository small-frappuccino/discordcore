package app

import (
	"context"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type Connection interface {
	ReadMessage() ([]byte, error)
}

type DiscordGatewayImpl struct {
	connection Connection
	handler    core.InteractionHandler
	workers    int64
}

func NewDiscordGatewayImpl(conn Connection, maxConcurrent int64) *DiscordGatewayImpl {
	return &DiscordGatewayImpl{
		connection: conn,
		workers:    maxConcurrent,
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

// ListenLoop runs a fixed, bounded worker pool over the decode/dispatch path.
//
// The pool is allocated ONCE per connection (§5 bounded ingress): the previous
// per-payload `eg.Go(func(){...})` allocated a closure + goroutine for every
// inbound frame — the "go process() per payload" anti-pattern. Here the only
// per-payload work is a single concrete-typed channel send (§5: channels carry
// concrete types, never interfaces/closures), which is allocation-free. The
// channel, WaitGroup, and worker goroutines below are per-connection construct
// state, outside the hot path.
func (g *DiscordGatewayImpl) ListenLoop(ctx context.Context) error {
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Buffered to g.workers to absorb micro-bursts (Little's Law) without
	// unbounding concurrency: exactly g.workers goroutines ever call the
	// handler, so concurrent dispatch is capped at g.workers.
	jobs := make(chan []byte, g.workers)

	var wg sync.WaitGroup
	wg.Add(int(g.workers))
	for i := int64(0); i < g.workers; i++ {
		// Method call in a go statement — not a func literal: no per-spawn
		// closure escape. Spawned g.workers times at connection start.
		go g.drain(workerCtx, jobs, &wg)
	}

	// Reader owns `jobs`: the read loop runs in this goroutine (the I/O
	// boundary) and closes the channel on exit so drainers terminate.
	g.read(workerCtx, jobs)
	wg.Wait()
	return nil
}

// read pulls frames off the connection and hands them to the worker pool over
// the concrete-typed channel. It owns `jobs` and closes it on exit.
func (g *DiscordGatewayImpl) read(ctx context.Context, jobs chan<- []byte) {
	defer close(jobs)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		payload, err := g.connection.ReadMessage()
		if err != nil {
			return
		}

		select {
		case jobs <- payload:
		case <-ctx.Done():
			return
		}
	}
}

// drain is a long-lived worker: it serially decodes/dispatches payloads off the
// shared channel until the reader closes it (or buffered work is exhausted).
func (g *DiscordGatewayImpl) drain(ctx context.Context, jobs <-chan []byte, wg *sync.WaitGroup) {
	defer wg.Done()
	for payload := range jobs {
		routeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		if g.handler != nil {
			_ = g.handler.HandleInteraction(routeCtx, core.InteractionPayload{Data: payload})
		}
		cancel()
	}
}
