package moderation

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"
	"github.com/small-frappuccino/discordcore/pkg/discord"
)

type MockDiscordGateway struct {
	banCount  int32
	kickCount int32
}

func (m *MockDiscordGateway) ExecuteBan(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string, deleteSeconds int) error {
	atomic.AddInt32(&m.banCount, 1)
	return nil
}

func (m *MockDiscordGateway) ExecuteKick(ctx context.Context, bot core.BotInstance, targetUserID uint64, reason string) error {
	atomic.AddInt32(&m.kickCount, 1)
	return nil
}

func TestServiceWorkersAndLoadShedding(t *testing.T) {
	mockGateway := &MockDiscordGateway{}
	router := NewRouter(&core.InMemoryFeatureRegistry{})
	svc := NewService(mockGateway, 2, router)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx, 2)

	inbox := svc.Route(12345)

	// Since we are mocking the GatewayEvent execution, we can simulate an event.
	// But note that processInteraction tries to parse the JSON and route via registry.
	// We just want to test load shedding on the ActorInbox.
	err := inbox.EnqueueEvent(&discord.GatewayEvent{})
	if err != nil {
		t.Fatalf("failed to enqueue first job: %v", err)
	}

	err = inbox.EnqueueEvent(&discord.GatewayEvent{})
	if err != nil {
		t.Fatalf("failed to enqueue second job: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	cancel()
	time.Sleep(10 * time.Millisecond)

	err = inbox.EnqueueEvent(&discord.GatewayEvent{})
	if err != ErrActorClosed {
		t.Fatalf("expected ErrActorClosed, got %v", err)
	}
}

func TestServiceContextAwareCancellation(t *testing.T) {
	mockGateway := &MockDiscordGateway{}
	router := NewRouter(&core.InMemoryFeatureRegistry{})
	svc := NewService(mockGateway, 10, router)

	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx, 2)

	inbox := svc.Route(12345)

	cancel()

	err := inbox.EnqueueEvent(&discord.GatewayEvent{})
	if err != ErrActorClosed && err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Benchmark primitives
func BenchmarkService_EnqueueTask(b *testing.B) {
	b.ReportAllocs()
	mockGateway := &MockDiscordGateway{}
	router := NewRouter(&core.InMemoryFeatureRegistry{})
	svc := NewService(mockGateway, b.N+1, router)
	svc.Start(context.Background(), 1)
	inbox := svc.Route(12345)

	evt := &discord.GatewayEvent{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = inbox.EnqueueEvent(evt)
	}
}
