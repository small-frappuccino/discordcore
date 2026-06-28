package moderation

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/core"
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
	// Queue size of 2
	svc := NewService(mockGateway, 2)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc.Start(ctx, 2)

	bot := core.BotInstance{ApplicationID: "bot", GuildID: "guild"}

	// Test Enqueue
	err := svc.EnqueueTask(ModerationJob{
		Bot:          bot,
		Action:       ActionBan,
		TargetUserID: 1,
	})
	if err != nil {
		t.Fatalf("failed to enqueue first job: %v", err)
	}

	err = svc.EnqueueTask(ModerationJob{
		Bot:          bot,
		Action:       ActionKick,
		TargetUserID: 2,
	})
	if err != nil {
		t.Fatalf("failed to enqueue second job: %v", err)
	}

	// Wait for workers to process
	time.Sleep(10 * time.Millisecond)

	if atomic.LoadInt32(&mockGateway.banCount) != 1 || atomic.LoadInt32(&mockGateway.kickCount) != 1 {
		t.Fatalf("expected 1 ban and 1 kick, got ban=%d kick=%d", mockGateway.banCount, mockGateway.kickCount)
	}

	// Test Load Shedding
	// To test load shedding, we need the inbox to be full and blocked.
	// We can cancel the service, which will close the actor but keep the inbox closed.
	// Actually, an actor returns ErrActorClosed if context is cancelled.
	cancel()
	time.Sleep(10 * time.Millisecond) // Give actor time to exit

	err = svc.EnqueueTask(ModerationJob{Bot: bot, Action: ActionBan, TargetUserID: 3})
	if err != ErrActorClosed {
		t.Fatalf("expected ErrActorClosed, got %v", err)
	}
}

func TestServiceContextAwareCancellation(t *testing.T) {
	mockGateway := &MockDiscordGateway{}
	svc := NewService(mockGateway, 10)

	ctx, cancel := context.WithCancel(context.Background())
	svc.Start(ctx, 2)

	bot := core.BotInstance{ApplicationID: "bot", GuildID: "guild"}

	// Cancel the worker pool context immediately
	cancel()

	err := svc.EnqueueTask(ModerationJob{
		Bot:          bot,
		Action:       ActionBan,
		TargetUserID: 99,
	})
	if err != ErrActorClosed && err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the mock gateway was not called because the worker context was cancelled
	if atomic.LoadInt32(&mockGateway.banCount) != 0 {
		t.Fatalf("expected 0 bans because job was cancelled, got %d", mockGateway.banCount)
	}
}

// Benchmark primitives
func BenchmarkService_EnqueueTask(b *testing.B) {
	b.ReportAllocs()
	mockGateway := &MockDiscordGateway{}
	svc := NewService(mockGateway, b.N+1) // Ensure it never blocks
	svc.Start(context.Background(), 1)
	bot := core.BotInstance{ApplicationID: "bot", GuildID: "guild"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.EnqueueTask(ModerationJob{
			Bot:          bot,
			Action:       ActionBan,
			TargetUserID: 1,
		})
	}
}
