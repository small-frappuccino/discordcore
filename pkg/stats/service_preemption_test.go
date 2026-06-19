package stats_test

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// blockingStore is a mock that blocks indefinitely until context is canceled.
type blockingStore struct {
	stats.StateStore
}

func (b *blockingStore) HeartbeatForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error) {
	<-ctx.Done()
	return time.Time{}, false, ctx.Err()
}

func (b *blockingStore) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	<-ctx.Done()
	return time.Time{}, false, ctx.Err()
}

func (b *blockingStore) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[storage.GuildMemberCurrentState, error] {
	<-ctx.Done()
	return func(yield func(storage.GuildMemberCurrentState, error) bool) {}
}

func TestStatsService_DatabasePreemption(t *testing.T) {
	store := &blockingStore{}
	configManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	s := stats.NewStatsService(nil, configManager, store, nil, "test-bot")

	ctx, cancel := context.WithCancel(context.Background())

	// Start the service
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Trigger a background update that uses the database
	go func() {
		s.UpdateStatsChannels(ctx)
	}()

	// Wait briefly to ensure it hits the blocking store
	time.Sleep(50 * time.Millisecond)

	// Preempt the execution via context cancellation
	cancel()

	// Service should stop gracefully without leaking or hanging indefinitely
	done := make(chan struct{})
	go func() {
		s.Stop(context.Background())
		close(done)
	}()

	select {
	case <-done:
		// Success! The database mock cleanly yielded control to ctx.Done()
	case <-time.After(1 * time.Second):
		t.Fatal("Service failed to preempt database operation on context cancellation")
	}
}
