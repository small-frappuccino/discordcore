package app

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type dummyConn struct {
	payload []byte
}

func (d *dummyConn) ReadMessage(buf []byte) ([]byte, error) {
	n := copy(buf[:cap(buf)], d.payload)
	return buf[:n], nil
}

type dummyHandler struct{}

func (d *dummyHandler) HandleInteraction(ctx context.Context, payload core.InteractionPayload) error {
	return nil
}

func BenchmarkGatewayIngress(b *testing.B) {
	b.ReportAllocs()

	payload := []byte(`{"guild_id":"123","data":{"name":"ban"}}`)
	conn := &dummyConn{payload: payload}

	gw := NewDiscordGatewayImpl(conn, 1)
	gw.OnInteraction(&dummyHandler{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create channels explicitly for bench
	jobs := make(chan *[]byte, 100)

	// Start single drainer
	go func() {
		for bufPtr := range jobs {
			gw.handler.HandleInteraction(ctx, core.InteractionPayload{Data: *bufPtr})
			buf := *bufPtr
			clear(buf[:cap(buf)])
			*bufPtr = buf[:cap(buf)]
			gw.bufPool.Put(bufPtr)
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bufPtr := gw.bufPool.Get().(*[]byte)
		p, _ := conn.ReadMessage(*bufPtr)
		*bufPtr = p
		jobs <- bufPtr
	}

	// Wait for drain to finish
	b.StopTimer()
	close(jobs)
}
