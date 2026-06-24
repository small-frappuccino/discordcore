package stats

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"golang.org/x/sync/errgroup"
)

// blockingStore is a mock that blocks indefinitely until context is canceled.
type blockingStore struct {
	StateStore
	entered chan struct{}
}

func (b *blockingStore) HeartbeatForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error) {
	<-ctx.Done()
	return time.Time{}, false, ctx.Err()
}

func (b *blockingStore) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	close(b.entered)
	<-ctx.Done()
	return time.Time{}, false, ctx.Err()
}

func (b *blockingStore) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error] {
	<-ctx.Done()
	return func(yield func(members.CurrentState, error) bool) {}
}

func (b *blockingStore) SetMetadata(ctx context.Context, key string, at time.Time) error {
	return nil
}

func (b *blockingStore) UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error {
	return nil
}

func (b *blockingStore) UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error {
	return nil
}

func (b *blockingStore) MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error {
	return nil
}

func TestStatsService_DatabasePreemption(t *testing.T) {
	t.Parallel()

	store := &blockingStore{
		entered: make(chan struct{}),
	}
	configManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	monitoringEnabled := true
	guildCfg := files.GuildConfig{
		GuildID: "test-guild",
		BotInstanceTokens: map[string]files.EncryptedString{
			"test-bot": "fake-token",
		},
		FeatureRouting: map[string]string{
			"stats": "test-bot",
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: &monitoringEnabled,
			},
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{
					ChannelID: "stats-channel",
					RoleID:    "stats-role",
				},
			},
		},
	}
	if err := configManager.AddGuildConfig(guildCfg); err != nil {
		t.Fatalf("Failed to add guild config: %v", err)
	}

	gateway := &mockGateway{}
	s := NewStatsService(gateway, configManager, store, nil, "test-bot")

	ctx, cancel := context.WithCancel(context.Background())

	// Start the service
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait until it hits the blocking store
	<-store.entered

	// Preempt the execution via context cancellation
	cancel()

	eg, egCtx := errgroup.WithContext(context.Background())
	done := make(chan struct{})
	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			return egCtx.Err()
		default:
		}
		s.Stop(context.Background())
		close(done)
		return nil
	})

	select {
	case <-done:
		// Success! The database mock cleanly yielded control to ctx.Done()
	case <-time.After(2 * time.Second):
		t.Fatal("Service failed to preempt database operation on context cancellation")
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("unexpected errgroup wait error: %v", err)
	}
}
