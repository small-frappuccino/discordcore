package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"
)

type MockConnection struct {
	messages chan []byte
}

func (m *MockConnection) ReadMessage() ([]byte, error) {
	msg, ok := <-m.messages
	if !ok {
		return nil, errors.New("connection closed")
	}
	return msg, nil
}

type MockInteractionHandler struct {
	concurrency     int32
	maxConcurrency  int32
	processedCount  int32
	processingDelay time.Duration
}

func (m *MockInteractionHandler) HandleInteraction(ctx context.Context, payload core.InteractionPayload) error {
	current := atomic.AddInt32(&m.concurrency, 1)
	defer atomic.AddInt32(&m.concurrency, -1)

	for {
		max := atomic.LoadInt32(&m.maxConcurrency)
		if current <= max {
			break
		}
		if atomic.CompareAndSwapInt32(&m.maxConcurrency, max, current) {
			break
		}
	}

	time.Sleep(m.processingDelay)
	atomic.AddInt32(&m.processedCount, 1)
	return nil
}

func TestGatewayBoundedIngress(t *testing.T) {
	messages := make(chan []byte, 10)
	conn := &MockConnection{messages: messages}
	handler := &MockInteractionHandler{processingDelay: 10 * time.Millisecond}

	// Gateway with semaphore limit of 3 concurrent routines
	g := NewDiscordGatewayImpl(conn, 3)
	g.OnInteraction(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run Gateway in a separate goroutine
	go g.ListenLoop(ctx)

	// Send 6 messages
	for i := 0; i < 6; i++ {
		messages <- []byte(`{"event":"test"}`)
	}

	// Wait for processing to complete
	time.Sleep(100 * time.Millisecond)

	maxSeen := atomic.LoadInt32(&handler.maxConcurrency)
	if maxSeen > 3 {
		t.Fatalf("expected max concurrent processing to be bounded by 3, but observed %d", maxSeen)
	}
	if maxSeen == 0 {
		t.Fatalf("expected some messages to be processed, but observed 0")
	}

	processed := atomic.LoadInt32(&handler.processedCount)
	if processed != 6 {
		t.Fatalf("expected all 6 messages to be processed, got %d", processed)
	}

	close(messages)
}
