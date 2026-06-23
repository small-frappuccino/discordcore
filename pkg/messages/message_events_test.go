package messages

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// Mock implementation of Repository
type mockRepository struct {
	mu                     sync.Mutex
	upsertErr              error
	upsertMessagesErr      error
	getMsg                 *Record
	getMsgErr              error
	deleteErr              error
	insertVersionErr       error
	incrementDailyErr      error
	cleanupErr             error
	upserted               []Record
	upsertMessages         []Record
	deleted                []DeleteKey
	versions               []Version
	deltas                 []DailyCountDelta
	singleDeleted          []struct{ GuildID, MessageID string }
	cleanupCalled          bool
	messageCreateWriterErr error
}

func (m *mockRepository) UpsertMessage(r Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upserted = append(m.upserted, r)
	return m.upsertErr
}

func (m *mockRepository) UpsertMessagesContext(ctx context.Context, records []Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertMessages = append(m.upsertMessages, records...)
	return m.upsertMessagesErr
}

func (m *mockRepository) GetMessage(ctx context.Context, guildID, messageID string) (*Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getMsg, m.getMsgErr
}

func (m *mockRepository) SetGetMsg(rec *Record) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getMsg = rec
}

func (m *mockRepository) DeleteMessagesContext(ctx context.Context, keys []DeleteKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleted = append(m.deleted, keys...)
	return m.deleteErr
}

func (m *mockRepository) SetErrors(upsertMessages, delete, insertVersion, incrementDaily error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertMessagesErr = upsertMessages
	m.deleteErr = delete
	m.insertVersionErr = insertVersion
	m.incrementDailyErr = incrementDaily
}

func (m *mockRepository) InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []Version) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions = append(m.versions, versions...)
	return m.insertVersionErr
}

func (m *mockRepository) CleanupExpiredMessages() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupCalled = true
	return m.cleanupErr
}

func (m *mockRepository) IncrementDailyMessageCountsContext(ctx context.Context, deltas []DailyCountDelta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deltas = append(m.deltas, deltas...)
	return m.incrementDailyErr
}

func (m *mockRepository) DeleteMessage(ctx context.Context, guildID, messageID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.singleDeleted = append(m.singleDeleted, struct{ GuildID, MessageID string }{guildID, messageID})
	return m.deleteErr
}

func (m *mockRepository) InsertMessageVersion(ctx context.Context, v Version) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.versions = append(m.versions, v)
	return m.insertVersionErr
}

func (m *mockRepository) IncrementDailyMessageCount(ctx context.Context, guildID string) error {
	return m.incrementDailyErr
}

// Mock implementation of MessageSink
type mockMessageSink struct {
	mu      sync.Mutex
	deletes []struct {
		M        *gateway.MessageDeleteEvent
		Cached   *discord.Message
		Executor *discord.User
	}
	updates []struct {
		M      *gateway.MessageUpdateEvent
		Cached *discord.Message
	}
	bulkDeletes []struct {
		GuildID   discord.GuildID
		ChannelID discord.ChannelID
		Messages  []string
	}
}

func (s *mockMessageSink) OnMessageDelete(ctx context.Context, m *gateway.MessageDeleteEvent, cachedMessage *discord.Message, executor *discord.User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes = append(s.deletes, struct {
		M        *gateway.MessageDeleteEvent
		Cached   *discord.Message
		Executor *discord.User
	}{m, cachedMessage, executor})
}

func (s *mockMessageSink) OnMessageUpdate(ctx context.Context, m *gateway.MessageUpdateEvent, cachedMessage *discord.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, struct {
		M      *gateway.MessageUpdateEvent
		Cached *discord.Message
	}{m, cachedMessage})
}

func (s *mockMessageSink) OnMessageDeleteBulk(ctx context.Context, guildID discord.GuildID, channelID discord.ChannelID, messages []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bulkDeletes = append(s.bulkDeletes, struct {
		GuildID   discord.GuildID
		ChannelID discord.ChannelID
		Messages  []string
	}{guildID, channelID, messages})
}

func TestInMemoryMetrics(t *testing.T) {
	m := NewInMemoryMetrics()
	m.RecordMessageSent()
	snap := m.Snapshot()
	if snap.MessagesSentTotal != 1 {
		t.Errorf("expected 1, got %d", snap.MessagesSentTotal)
	}

	var nop NopMetrics
	nop.RecordMessageSent()
}

func TestMessageWriterMetrics(t *testing.T) {
	m := NewInMemoryMessageWriterMetrics()
	m.RecordEnqueueUpsert(true, true)
	m.RecordEnqueueDelete(true)
	m.RecordEnqueueVersion()
	m.RecordEnqueueFailure(MessageWriterEnqueueFailureQueueFull)
	m.ObserveQueueDepth(10)
	m.ObserveQueueDepth(5) // lower, should not update max
	m.RecordFlush(5, 50*time.Millisecond)
	m.RecordFlushSuccess(MessageWriterFlushOpUpsert, 5)
	m.RecordFlushFallback(MessageWriterFlushOpDelete, 2)

	snap := m.Snapshot()
	if snap.Enqueue.UpsertsTotal != 1 {
		t.Errorf("unexpected upserts total")
	}
	if snap.Enqueue.VersionsTotal != 3 {
		t.Errorf("unexpected versions total")
	}
	if snap.Enqueue.FailuresByCause[MessageWriterEnqueueFailureQueueFull] != 1 {
		t.Errorf("unexpected failures")
	}
	if snap.Enqueue.MaxQueueDepth != 10 {
		t.Errorf("unexpected queue depth")
	}
	if snap.Flush.CyclesTotal != 1 {
		t.Errorf("unexpected cycles")
	}
	if snap.Flush.FlushedByOp[MessageWriterFlushOpUpsert] != 5 {
		t.Errorf("unexpected flushed op count")
	}
	if snap.Flush.FallbackByOp[MessageWriterFlushOpDelete] != 2 {
		t.Errorf("unexpected fallback count")
	}

	// Test boundary conditions on methods
	m.RecordFlushSuccess("op", -1)  // noop
	m.RecordFlushFallback("op", -1) // noop
	m.ObserveQueueDepth(-1)         // noop

	// Test NopMessageWriterMetrics
	var nm NopMessageWriterMetrics
	nm.RecordEnqueueUpsert(true, true)
	nm.RecordEnqueueDelete(true)
	nm.RecordEnqueueVersion()
	nm.RecordEnqueueFailure("")
	nm.ObserveQueueDepth(1)
	nm.RecordFlush(1, 1)
	nm.RecordFlushSuccess("", 1)
	nm.RecordFlushFallback("", 1)
}

func TestMessageCreateWriter_Basic(t *testing.T) {
	repo := &mockRepository{}
	metrics := NewInMemoryMessageWriterMetrics()
	logger := slog.Default()

	w := newMessageCreateWriter(repo, metrics, logger)
	if w == nil {
		t.Fatalf("failed to create writer")
	}

	w.flushInterval = 10 * time.Millisecond
	w.maxBatch = 3
	w.Start()
	defer w.Stop(context.Background())

	// Enqueue upsert
	rec := Record{
		GuildID:        "111",
		MessageID:      "222",
		ChannelID:      "333",
		AuthorID:       "444",
		AuthorUsername: "alice",
		Content:        "hello",
		CachedAt:       time.Now(),
	}
	ver := Version{
		GuildID:   "111",
		MessageID: "222",
		EventType: "create",
		Content:   "hello",
	}
	delta := DailyCountDelta{
		GuildID:   "111",
		ChannelID: "333",
		UserID:    "444",
		Day:       time.Now(),
		Count:     1,
	}

	err := w.Enqueue(rec, &ver, delta)
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}

	// Lookup pending
	cached := w.Lookup("111", "222")
	if cached == nil || cached.Content != "hello" {
		t.Errorf("expected cached message, got %+v", cached)
	}

	// Enqueue delete
	err = w.EnqueueDelete("111", "222", &ver)
	if err != nil {
		t.Fatalf("enqueue delete error: %v", err)
	}

	// Enqueue version
	err = w.EnqueueVersion(ver)
	if err != nil {
		t.Fatalf("enqueue version error: %v", err)
	}

	// Wait deterministically for immediate flush due to batch=3
	for metrics.Snapshot().Flush.CyclesTotal < 1 {
		runtime.Gosched()
	}

	// Verify sequential / fallback flows by forcing error
	repo.SetErrors(
		errors.New("upsert messages batch err"),
		errors.New("delete batch err"),
		errors.New("insert version batch err"),
		errors.New("increment daily batch err"),
	)

	err = w.Enqueue(rec, &ver, delta)
	if err != nil {
		t.Fatalf("enqueue error: %v", err)
	}
	// Stop forces deterministic final flush
	w.Stop(context.Background())
}

func TestAuditCacheState(t *testing.T) {
	s := newAuditCacheState(10*time.Millisecond, 20*time.Millisecond)
	if _, ok := s.get("111"); ok {
		t.Errorf("expected miss")
	}

	s.set("111", auditCacheEntry{
		fetchedAt: time.Now(),
		entries: map[string]auditCacheValue{
			"key": {
				userID:    "999",
				createdAt: time.Now(),
			},
		},
	})

	entry, ok := s.get("111")
	if !ok {
		t.Errorf("expected hit")
	}

	userID := s.pickEntry(entry.entries, "key")
	if userID != "999" {
		t.Errorf("expected 999, got %q", userID)
	}

	// Test zero TTL
	sZero := newAuditCacheState(0, 0)
	sZero.set("111", auditCacheEntry{})
	if _, ok := sZero.get("111"); ok {
		t.Errorf("expected false for zero ttl")
	}

	// Test max age expiry
	sAge := newAuditCacheState(10*time.Second, 1*time.Millisecond)
	sAge.set("111", auditCacheEntry{
		fetchedAt: time.Now(),
		entries: map[string]auditCacheValue{
			"key": {
				userID:    "999",
				createdAt: time.Now().Add(-10 * time.Millisecond),
			},
		},
	})
	entry, _ = sAge.get("111")
	if sAge.pickEntry(entry.entries, "key") != "" {
		t.Errorf("expected expired pickEntry to return empty string")
	}
	if sAge.pickEntry(nil, "key") != "" {
		t.Errorf("expected empty string for nil map")
	}
	if sAge.pickEntry(entry.entries, "nonexistent") != "" {
		t.Errorf("expected empty string for missing key")
	}
}

func TestMessageEventService_LifecycleAndMetadata(t *testing.T) {
	st := state.New("token")
	store := &mockRepository{}
	store.cleanupErr = errors.New("cleanup failed") // coverage for cleanup failure warning
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		RuntimeConfig: files.RuntimeConfig{
			MessageCacheCleanup: true,
		},
	})

	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}

	svc := NewMessageEventServiceForBot(deps)
	if svc.Name() != "messages" {
		t.Errorf("expected messages")
	}
	if svc.Type() != "messages" {
		t.Errorf("expected messages type")
	}
	if svc.Priority() != service.PriorityNormal {
		t.Errorf("expected PriorityNormal")
	}
	if len(svc.Dependencies()) != 0 {
		t.Errorf("expected no deps")
	}

	err := svc.Start(context.Background())
	if err != nil {
		t.Fatalf("failed to start service: %v", err)
	}

	if !svc.IsRunning() {
		t.Errorf("expected running")
	}

	health := svc.HealthCheck(context.Background())
	if !health.Healthy {
		t.Errorf("expected healthy")
	}

	svc.Stats()

	// Set task router
	tr := task.NewRouter(task.Defaults())
	defer tr.Close()
	svc.SetTaskRouter(tr)
	svc.SetWriterMetrics(NewInMemoryMessageWriterMetrics())

	err = svc.Stop(context.Background())
	if err != nil {
		t.Fatalf("failed to stop service: %v", err)
	}
}

func TestMessageEventService_IngestMessageCreate(t *testing.T) {
	st := state.New("token")
	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	// Add guild config
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		Channels: files.ChannelsConfig{
			MessageEdit: "888",
		},
	})

	sink := &mockMessageSink{}
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          sink,
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}
	svc := NewMessageEventServiceForBot(deps)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())

	// nil event
	svc.IngestMessageCreate(context.Background(), nil)

	// invalid author
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{})

	// bot author
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{
		Message: discord.Message{
			Author: discord.User{
				ID:  123,
				Bot: true,
			},
		},
	})

	// context canceled
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()
	svc.IngestMessageCreate(ctxCancel, &gateway.MessageCreateEvent{
		Message: discord.Message{
			Author: discord.User{
				ID: 123,
			},
		},
	})

	// DM / no valid guildID, lookup channel
	_ = st.Cabinet.ChannelStore.ChannelSet(&discord.Channel{
		ID:      222,
		GuildID: 0, // DM
	}, false)
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "hello",
		},
	})

	// DM lookup channel fails
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			ChannelID: 444, // missing channel
			Author: discord.User{
				ID: 123,
			},
			Content: "hello",
		},
	})

	// Valid Guild, but logging policy check enabled false (no logs config etc)
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   999, // missing guild config
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "hello",
		},
	})

	// Non-text message summary building
	_ = st.Cabinet.ChannelStore.ChannelSet(&discord.Channel{
		ID:      222,
		GuildID: 111,
	}, false)
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Attachments: []discord.Attachment{{}},
			Embeds:      []discord.Embed{{}},
			Stickers:    []discord.StickerItem{{}},
		},
	})

	// Empty content will not cache test
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
		},
	})

	// Successful cache
	svc.IngestMessageCreate(context.Background(), &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "hello",
		},
	})
}

func TestMessageEventService_IngestMessageUpdate_And_Delete(t *testing.T) {
	st := state.New("token")
	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	// Add guild config
	deleteOnLog := true
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		Channels: files.ChannelsConfig{
			MessageEdit:   "888",
			MessageDelete: "888",
		},
		Features: files.FeatureToggles{
			MessageCache: files.FeatureMessageCacheToggles{
				DeleteOnLog: &deleteOnLog,
			},
		},
	})

	sink := &mockMessageSink{}
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          sink,
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}
	svc := NewMessageEventServiceForBot(deps)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())

	// --- Test Update ---
	// nil event
	svc.IngestMessageUpdate(context.Background(), nil)

	// bot author
	svc.IngestMessageUpdate(context.Background(), &gateway.MessageUpdateEvent{
		Message: discord.Message{
			Author: discord.User{
				ID:  123,
				Bot: true,
			},
		},
	})

	// Cache miss in store
	svc.IngestMessageUpdate(context.Background(), &gateway.MessageUpdateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "hello",
		},
	})

	// Cache hit in writer pending map (upsert pending)
	svc.persistMessageCreate("111", &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID:       123,
				Username: "alice",
			},
			Content: "hello",
		},
	})

	// Content actually changed update
	svc.IngestMessageUpdate(context.Background(), &gateway.MessageUpdateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "world",
		},
	})

	// Content unchanged update
	svc.IngestMessageUpdate(context.Background(), &gateway.MessageUpdateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "world",
		},
	})

	// Content update resolving via state.Message
	_ = st.Cabinet.MessageStore.MessageSet(&discord.Message{
		ID:        999,
		GuildID:   111,
		ChannelID: 222,
		Author: discord.User{
			ID: 123,
		},
		Content: "world state",
	}, false)
	svc.IngestMessageUpdate(context.Background(), &gateway.MessageUpdateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "", // empty forces lookup
		},
	})

	// --- Test Delete ---
	// nil event
	svc.IngestMessageDelete(context.Background(), nil)

	// cache hit delete
	svc.IngestMessageDelete(context.Background(), &gateway.MessageDeleteEvent{
		ID:        999,
		GuildID:   111,
		ChannelID: 222,
	})

	// cache miss delete
	svc.IngestMessageDelete(context.Background(), &gateway.MessageDeleteEvent{
		ID:        888,
		GuildID:   111,
		ChannelID: 222,
	})

	// cache hit delete but author is bot
	svc.persistMessageCreate("111", &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        777,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID:       123,
				Bot:      true,
				Username: "bot",
			},
			Content: "hello bot",
		},
	})
	svc.IngestMessageDelete(context.Background(), &gateway.MessageDeleteEvent{
		ID:        777,
		GuildID:   111,
		ChannelID: 222,
	})
}

func TestMessageEventService_ActiveBotInstanceRouting(t *testing.T) {
	st := state.New("token")
	store := &mockRepository{
		getMsg: &Record{
			MessageID:      "999",
			GuildID:        "111",
			ChannelID:      "222",
			AuthorID:       "123",
			AuthorUsername: "alice",
			Content:        "hello",
		},
	}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	// Add guild config that belongs to bot-1 and routes logging to bot-1
	deleteOnLog := true
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		BotInstanceTokens: map[string]files.EncryptedString{
			"bot-1": "some-token",
		},
		FeatureRouting: map[string]string{
			"logging": "bot-1",
		},
		Channels: files.ChannelsConfig{
			MessageEdit:   "888",
			MessageDelete: "888",
		},
		Features: files.FeatureToggles{
			MessageCache: files.FeatureMessageCacheToggles{
				DeleteOnLog: &deleteOnLog,
			},
		},
	})

	sink := &mockMessageSink{}
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          sink,
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}

	svc := NewMessageEventServiceForBot(deps)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())

	// Configure mock audit log cache value directly to bypass Discord API call
	svc.auditCache.set("111", auditCacheEntry{
		fetchedAt: time.Now(),
		entries: map[string]auditCacheValue{
			"123:222": {
				userID:    "333",
				createdAt: time.Now(),
			},
		},
	})

	// IngestMessageUpdate with matching bot instance -> processMessageUpdate runs
	svc.IngestMessageUpdate(context.Background(), &gateway.MessageUpdateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "hello edited",
		},
	})

	// IngestMessageDelete with matching bot instance -> processMessageDelete runs
	svc.IngestMessageDelete(context.Background(), &gateway.MessageDeleteEvent{
		ID:        999,
		GuildID:   111,
		ChannelID: 222,
	})

	// Verify both callbacks were invoked on the sink
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.updates) != 1 {
		t.Errorf("expected 1 update, got %d", len(sink.updates))
	}
	if len(sink.deletes) != 1 {
		t.Errorf("expected 1 delete, got %d", len(sink.deletes))
	}
	actor := svc.determineDeletedBy("111", "222", "123")
	if actor != "333" {
		t.Errorf("expected resolved actor ID to be 333, got %q", actor)
	}

	// Trigger cache miss retry path in processMessageDelete by passing an uncached ID
	store.getMsg = nil
	err := svc.processMessageDelete(context.Background(), &gateway.MessageDeleteEvent{
		ID:        888,
		GuildID:   111,
		ChannelID: 222,
	}, false)
	if !errors.Is(err, task.ErrRetrySilent) {
		t.Errorf("expected ErrRetrySilent, got %v", err)
	}
}

func TestMessageEventService_TaskRouterAsynchronousHandling(t *testing.T) {
	st := state.New("token")
	store := &mockRepository{
		getMsg: &Record{
			MessageID:      "999",
			GuildID:        "111",
			ChannelID:      "222",
			AuthorID:       "123",
			AuthorUsername: "alice",
			Content:        "hello",
		},
	}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "111",
		BotInstanceTokens: map[string]files.EncryptedString{
			"bot-1": "some-token",
		},
		FeatureRouting: map[string]string{
			"logging": "bot-1",
		},
		Channels: files.ChannelsConfig{
			MessageEdit:   "888",
			MessageDelete: "888",
		},
	})

	sink := &mockMessageSink{}
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          sink,
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "bot-1",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}

	svc := NewMessageEventServiceForBot(deps)
	tr := task.NewRouter(task.Defaults())
	svc.SetTaskRouter(tr)
	_ = svc.Start(context.Background())
	defer svc.Stop(context.Background())
	defer tr.Close()

	// Ingest via Task Router
	svc.IngestMessageUpdate(context.Background(), &gateway.MessageUpdateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "hello edited",
		},
	})

	svc.IngestMessageDelete(context.Background(), &gateway.MessageDeleteEvent{
		ID:        999,
		GuildID:   111,
		ChannelID: 222,
	})

	// Wait deterministically for Task Router workers to process
	for i := 0; i < 1000; i++ {
		sink.mu.Lock()
		count := len(sink.updates)
		sink.mu.Unlock()
		if count == 1 {
			break
		}
		runtime.Gosched()
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.updates) != 1 {
		t.Errorf("expected 1 update via async task, got %d", len(sink.updates))
	}
}

func TestLookupCachedMessage_PollingAndCancellation(t *testing.T) {
	st := state.New("token")
	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}
	svc := NewMessageEventServiceForBot(deps)

	// Canceled context should exit poll loop immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cached := svc.lookupCachedMessage(ctx, "111", "999", true)
	if cached != nil {
		t.Errorf("expected nil result on canceled context")
	}

	// Poll loop returns message after it appears in mock store
	ctx2 := context.Background()
	go func() {
		for i := 0; i < 10; i++ {
			runtime.Gosched()
		}
		store.SetGetMsg(&Record{
			MessageID:      "999",
			GuildID:        "111",
			ChannelID:      "222",
			AuthorID:       "123",
			AuthorUsername: "alice",
			Content:        "hello",
		})
	}()

	cached = svc.lookupCachedMessage(ctx2, "111", "999", true)
	if cached == nil || cached.Content != "hello" {
		t.Errorf("expected message to be found eventually via polling, got %+v", cached)
	}
}

func TestMessageEventService_PersistFallbacks(t *testing.T) {
	st := state.New("token")
	store := &mockRepository{
		upsertErr:         errors.New("sync upsert err"),
		insertVersionErr:  errors.New("sync insert version err"),
		incrementDailyErr: errors.New("sync increment daily err"),
		deleteErr:         errors.New("sync delete err"),
	}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}
	svc := NewMessageEventServiceForBot(deps)
	svc.versioningEnabled = true

	// Test persistMessageCreate fallback warning logs
	svc.persistMessageCreate("111", &gateway.MessageCreateEvent{
		Message: discord.Message{
			ID:        999,
			GuildID:   111,
			ChannelID: 222,
			Author: discord.User{
				ID: 123,
			},
			Content: "hello",
		},
	})

	// Test persistMessageUpdate fallback warning logs
	svc.persistMessageUpdate(&CachedMessage{
		ID:        "999",
		Content:   "hello",
		Author:    &discord.User{ID: 123},
		ChannelID: "222",
		GuildID:   "111",
	}, "hello edited")

	// Test persistMessageDelete fallback warning logs
	svc.persistMessageDelete(&CachedMessage{
		ID:        "999",
		Content:   "hello",
		Author:    &discord.User{ID: 123},
		ChannelID: "222",
		GuildID:   "111",
	}, true, true, "op")

	// Empty handlers / noops
	svc.persistMessageCreate("111", nil)
	svc.persistMessageUpdate(nil, "")
	svc.persistMessageDelete(nil, true, true, "op")
}

func TestAuditLogFetchFailureFallback(t *testing.T) {
	st := state.New("token")
	// Make State Client call mockable, or at least fail gracefully.
	// Since Client.AuditLog will make actual HTTP calls and fail because of invalid token, it returns error.
	// We verify that it returns empty string on AuditLog fetch failure.
	store := &mockRepository{}
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	deps := EventServiceDeps{
		ConfigManager: cfgMgr,
		Sink:          &mockMessageSink{},
		Store:         store,
		SystemRepo:    nil,
		BotInstanceID: "",
		Logger:        slog.Default(),
		ArikawaState:  st,
	}
	svc := NewMessageEventServiceForBot(deps)
	actor := svc.determineDeletedBy("111", "222", "123")
	if actor != "" {
		t.Errorf("expected empty string on fetch failure, got %q", actor)
	}

	// Verify snowflakeTimestamp boundary checks
	ts, ok := snowflakeTimestamp("")
	if ok || !ts.IsZero() {
		t.Errorf("expected false/zero for empty snowflake")
	}
	ts, ok = snowflakeTimestamp("invalid")
	if ok || !ts.IsZero() {
		t.Errorf("expected false/zero for invalid snowflake")
	}
}

func TestMessageEventService_SummarizeMessageContent(t *testing.T) {
	deps := EventServiceDeps{
		Logger: slog.Default(),
	}
	svc := NewMessageEventServiceForBot(deps)
	if svc.summarizeMessageContent(nil, "base") != "base" {
		t.Errorf("expected base")
	}

	msg := &discord.Message{
		Attachments: []discord.Attachment{{}},
		Embeds:      []discord.Embed{{}},
		Stickers:    []discord.StickerItem{{}},
	}
	res := svc.summarizeMessageContent(msg, "hello")
	expected := "hello [attachments: 1] [embeds: 1] [stickers: 1]"
	if res != expected {
		t.Errorf("expected %q, got %q", expected, res)
	}
}

func TestNewerAuditEntry(t *testing.T) {
	t1 := time.Now()
	t2 := t1.Add(time.Second)

	// both empty
	res := newerAuditEntry(auditCacheValue{}, auditCacheValue{})
	if res.userID != "" {
		t.Errorf("expected empty")
	}

	// current empty, candidate filled
	res = newerAuditEntry(auditCacheValue{}, auditCacheValue{userID: "1", createdAt: t1})
	if res.userID != "1" {
		t.Errorf("expected 1")
	}

	// candidate empty
	res = newerAuditEntry(auditCacheValue{userID: "1", createdAt: t1}, auditCacheValue{})
	if res.userID != "1" {
		t.Errorf("expected 1")
	}

	// candidate newer
	res = newerAuditEntry(auditCacheValue{userID: "1", createdAt: t1}, auditCacheValue{userID: "2", createdAt: t2})
	if res.userID != "2" {
		t.Errorf("expected 2")
	}

	// current newer
	res = newerAuditEntry(auditCacheValue{userID: "1", createdAt: t2}, auditCacheValue{userID: "2", createdAt: t1})
	if res.userID != "1" {
		t.Errorf("expected 1")
	}
}

func TestDeleteOnLogEnabled(t *testing.T) {
	// mes.deleteOnLog == false
	svc := &MessageEventService{deleteOnLog: false}
	if svc.deleteOnLogEnabled("111") {
		t.Errorf("expected false")
	}

	// mes.deleteOnLog == true, configManager == nil
	svc = &MessageEventService{deleteOnLog: true}
	if !svc.deleteOnLogEnabled("111") {
		t.Errorf("expected true")
	}

	// config is nil -> returns mes.deleteOnLog (true)
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	svc.configManager = cfgMgr
	if !svc.deleteOnLogEnabled("111") {
		t.Errorf("expected true")
	}

	// guild exists but DeleteOnLog is false by default
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{GuildID: "111"})
	if svc.deleteOnLogEnabled("111") {
		t.Errorf("expected false")
	}

	// guild exists with DeleteOnLog = true
	deleteOnLog := true
	_ = cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: "222",
		Features: files.FeatureToggles{
			MessageCache: files.FeatureMessageCacheToggles{
				DeleteOnLog: &deleteOnLog,
			},
		},
	})
	if !svc.deleteOnLogEnabled("222") {
		t.Errorf("expected true")
	}
}
