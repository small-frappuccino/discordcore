package discord

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// GatewayWriter defines the interface for sending payloads to Discord.
type GatewayWriter interface {
	WriteMessage(ctx context.Context, data []byte) error
}

// HeartbeatManager maintains the connection lifecycle via Op 1/11.
type HeartbeatManager struct {
	writer   GatewayWriter
	seq      *atomic.Int64
	interval time.Duration
	ackRecv  atomic.Bool
}

// NewHeartbeatManager creates a new heartbeat lifecycle manager.
func NewHeartbeatManager(writer GatewayWriter, seq *atomic.Int64, interval time.Duration) *HeartbeatManager {
	hm := &HeartbeatManager{
		writer:   writer,
		seq:      seq,
		interval: interval,
	}
	hm.ackRecv.Store(true)
	return hm
}

// Run starts the ticker. Teardown happens via ctx cancellation.
func (hm *HeartbeatManager) Run(ctx context.Context, eg *errgroup.Group) {
	eg.Go(func() error {
		ticker := time.NewTicker(hm.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if !hm.ackRecv.Load() {
					// Missed ACK => force reconnect (return error to tear down eg)
					return fmt.Errorf("missed heartbeat ack, zombied connection")
				}
				hm.ackRecv.Store(false)

				s := hm.seq.Load()
				var payload string
				if s > 0 {
					payload = fmt.Sprintf(`{"op":1,"d":%d}`, s)
				} else {
					payload = `{"op":1,"d":null}`
				}

				if err := hm.writer.WriteMessage(ctx, []byte(payload)); err != nil {
					return fmt.Errorf("failed to write heartbeat: %w", err)
				}
			}
		}
	})
}

// ObserveACK records an Op 11 Heartbeat ACK from the Gateway.
func (hm *HeartbeatManager) ObserveACK() {
	hm.ackRecv.Store(true)
}
