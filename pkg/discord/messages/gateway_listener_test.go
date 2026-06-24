package messages

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/handler"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

type mockRepository struct {
	mu           sync.Mutex
	upserted     []messages.Record
	deleted      []messages.DeleteKey
	upsertSignal chan struct{}
}

func (m *mockRepository) UpsertMessage(rec messages.Record) error {
	m.mu.Lock()
	m.upserted = append(m.upserted, rec)
	m.mu.Unlock()
	if m.upsertSignal != nil {
		select {
		case <-m.upsertSignal:
		default:
			close(m.upsertSignal)
		}
	}
	return nil
}

func (m *mockRepository) UpsertMessagesContext(ctx context.Context, records []messages.Record) error {
	m.mu.Lock()
	m.upserted = append(m.upserted, records...)
	m.mu.Unlock()
	if m.upsertSignal != nil {
		select {
		case <-m.upsertSignal:
		default:
			close(m.upsertSignal)
		}
	}
	return nil
}

func (m *mockRepository) GetMessage(ctx context.Context, guildID, messageID string) (*messages.Record, error) {
	return nil, nil
}

func (m *mockRepository) DeleteMessagesContext(ctx context.Context, keys []messages.DeleteKey) error {
	m.mu.Lock()
	m.deleted = append(m.deleted, keys...)
	m.mu.Unlock()
	return nil
}

func (m *mockRepository) InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []messages.Version) error {
	return nil
}

func (m *mockRepository) CleanupExpiredMessages() error {
	return nil
}

func (m *mockRepository) IncrementDailyMessageCountsContext(ctx context.Context, deltas []messages.DailyCountDelta) error {
	return nil
}

func (m *mockRepository) DeleteMessage(ctx context.Context, guildID, messageID string) error {
	return nil
}

func (m *mockRepository) InsertMessageVersion(ctx context.Context, v messages.Version) error {
	return nil
}

func (m *mockRepository) IncrementDailyMessageCount(ctx context.Context, guildID string) error {
	return nil
}

type mockMessageSink struct {
	mu      sync.Mutex
	deletes []messages.MessageDeleteIntent
	updates []messages.MessageUpdateIntent
}

func (s *mockMessageSink) OnMessageDelete(ctx context.Context, m messages.MessageDeleteIntent, cachedMessage *messages.CachedMessageData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes = append(s.deletes, m)
}

func (s *mockMessageSink) OnMessageUpdate(ctx context.Context, m messages.MessageUpdateIntent, cachedMessage *messages.CachedMessageData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, m)
}

func (s *mockMessageSink) OnMessageDeleteBulk(ctx context.Context, intent messages.MessageDeleteBulkIntent) {
}

func TestGatewayListener_Lifecycle(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stateVal := state.New("Bot Token")
	stateVal.PreHandler = handler.New()

	// Config manager setup
	storeConfig := &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "12345",
				Channels: files.ChannelsConfig{
					MessageEdit:   "67890",
					MessageDelete: "67890",
				},
			},
		},
	}
	store := &files.MemoryConfigStore{}
	_ = store.Save(storeConfig)
	configMgr := files.NewConfigManagerWithStore(store, logger)
	_ = configMgr.LoadConfig()

	repo := &mockRepository{
		upsertSignal: make(chan struct{}),
	}
	sink := &mockMessageSink{}

	deps := messages.EventServiceDeps{
		ConfigManager:  configMgr,
		Sink:           sink,
		Store:          repo,
		SystemRepo:     nil,
		BotInstanceID:  "",
		Logger:         logger,
		DiscordAdapter: NewArikawaAdapter(stateVal),
	}

	msgSvc := messages.NewMessageEventServiceForBot(deps)
	_ = msgSvc.Start(context.Background())
	defer msgSvc.Stop(context.Background())

	listener := NewGatewayListener(stateVal, msgSvc)

	// Verify Name, Type, Priority, Dependencies, HealthCheck, Stats
	if name := listener.Name(); name != "discord_messages_listener" {
		t.Errorf("expected Name to be 'discord_messages_listener', got %q", name)
	}
	if svcType := listener.Type(); svcType != service.ServiceType("gateway_listener") {
		t.Errorf("expected Type to be 'gateway_listener', got %q", svcType)
	}
	if priority := listener.Priority(); priority != service.PriorityNormal {
		t.Errorf("expected Priority to be PriorityNormal, got %v", priority)
	}
	if deps := listener.Dependencies(); len(deps) != 1 || deps[0] != "messages" {
		t.Errorf("expected Dependencies to be ['messages'], got %v", deps)
	}
	if health := listener.HealthCheck(context.Background()); !health.Healthy {
		t.Errorf("expected HealthCheck to be healthy")
	}
	_ = listener.Stats()

	// IsRunning before start
	if listener.IsRunning() {
		t.Error("expected IsRunning to be false before Start")
	}

	// Start
	err := listener.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error starting listener: %v", err)
	}
	if !listener.IsRunning() {
		t.Error("expected IsRunning to be true after Start")
	}

	// Cache a channel in Arikawa so that message events find the channel/guild
	_ = stateVal.Cabinet.ChannelStore.ChannelSet(&discord.Channel{
		ID:      67890,
		GuildID: 12345,
	}, false)

	// Trigger MessageCreateEvent
	stateVal.Call(&gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        discord.MessageID(111),
			ChannelID: discord.ChannelID(67890),
			GuildID:   discord.GuildID(12345),
			Author: discord.User{
				ID:       discord.UserID(999),
				Username: "testuser",
				Bot:      false,
			},
			Content: "hello",
		},
	})

	// Wait for processing to complete deterministically
	select {
	case <-repo.upsertSignal:
		// Success!
	case <-t.Context().Done():
		t.Fatal("test context canceled while waiting for message process")
	}

	// Stop
	err = listener.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error stopping listener: %v", err)
	}
	if listener.IsRunning() {
		t.Error("expected IsRunning to be false after Stop")
	}

	// Stop message service to flush writer
	err = msgSvc.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error stopping message service: %v", err)
	}

	// Check if mock repos were invoked
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.upserted) == 0 {
		t.Error("expected mockRepository to receive upserted message")
	}
}
