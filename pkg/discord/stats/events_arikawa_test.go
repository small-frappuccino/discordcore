package stats

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/small-frappuccino/discordcore/pkg/files"
	domain "github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func setupTestDB(t *testing.T) (*storage.Store, *pgxpool.Pool, func()) {
	t.Helper()
	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			return nil, nil, func() {}
		}
		t.Fatalf("failed to get database URL: %v", err)
	}

	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	store, err := storage.NewStore(db, slog.Default())
	if err != nil {
		cleanup()
		t.Fatalf("failed to create store: %v", err)
	}

	return store, db, func() { _ = cleanup() }
}

type mockConfigStore struct {
	cfg *files.BotConfig
}

func (m *mockConfigStore) Load() (*files.BotConfig, error) {
	if m.cfg == nil {
		return &files.BotConfig{}, nil
	}
	return m.cfg, nil
}

func (m *mockConfigStore) Save(cfg *files.BotConfig) error {
	m.cfg = cfg
	return nil
}

func (m *mockConfigStore) Transaction(fn func(cfg *files.BotConfig) error) (bool, error) {
	if m.cfg == nil {
		m.cfg = &files.BotConfig{}
	}
	if err := fn(m.cfg); err != nil {
		return false, err
	}
	return true, nil
}

func (m *mockConfigStore) Describe() string {
	return "mock"
}

func (m *mockConfigStore) Exists() (bool, error) {
	return m.cfg != nil, nil
}

func newTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()
	cm := files.NewConfigManagerWithStore(&mockConfigStore{}, nil)
	cfg, _, err := cm.LoadConfigFromStore()
	if err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}
	cm.ApplyConfig(cfg)
	return cm
}

func TestRegisterArikawaEventHandlers(t *testing.T) {
	s := state.New("Bot token")
	logger := slog.Default()

	// Should not panic
	RegisterEventHandlers(s, nil, logger)
}

func TestHandleArikawaGuildMemberAdd(t *testing.T) {
	handleArikawaGuildMemberAdd(nil, nil)

	store, db, cleanup := setupTestDB(t)
	if store == nil {
		t.Skip("skipping db tests")
	}
	defer cleanup()

	cm := newTestConfigManager(t)
	_, _ = cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{GuildID: "456", BotInstanceTokens: map[string]files.EncryptedString{"test": "token"}, FeatureRouting: map[string]string{"stats": "test"}, Features: files.FeatureToggles{StatsChannels: testBoolPtr(true)}, Stats: files.StatsConfig{Enabled: true, Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}}}
		return nil
	})

	svc := domain.NewStatsService(nil, cm, store, slog.Default(), "test")
	if store != nil {
		_, _ = db.Exec(context.Background(), "INSERT INTO guilds (id) VALUES ('456') ON CONFLICT DO NOTHING")
	}

	e := &gateway.GuildMemberAddEvent{
		Member: discord.Member{
			User: discord.User{
				ID:  discord.UserID(123),
				Bot: true,
			},
			RoleIDs: []discord.RoleID{discord.RoleID(1)},
			Joined:  discord.Timestamp(time.Now()),
		},
	}
	e.GuildID = discord.GuildID(456)

	// Should not panic
	handleArikawaGuildMemberAdd(svc, e)
}

func testBoolPtr(b bool) *bool { return &b }

func TestHandleArikawaGuildMemberRemove(t *testing.T) {
	handleArikawaGuildMemberRemove(nil, nil)

	store, db, cleanup := setupTestDB(t)
	if store == nil {
		t.Skip("skipping db tests")
	}
	defer cleanup()
	cm := newTestConfigManager(t)
	_, _ = cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{GuildID: "456", BotInstanceTokens: map[string]files.EncryptedString{"test": "token"}, FeatureRouting: map[string]string{"stats": "test"}, Features: files.FeatureToggles{StatsChannels: testBoolPtr(true)}, Stats: files.StatsConfig{Enabled: true, Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}}}
		return nil
	})

	svc := domain.NewStatsService(nil, cm, store, slog.Default(), "test")
	if store != nil {
		_, _ = db.Exec(context.Background(), "INSERT INTO guilds (id) VALUES ('456') ON CONFLICT DO NOTHING")
	}

	e := &gateway.GuildMemberRemoveEvent{
		User: discord.User{
			ID: discord.UserID(123),
		},
	}
	e.GuildID = discord.GuildID(456)

	// Should not panic
	handleArikawaGuildMemberRemove(svc, e)
}

func TestHandleArikawaGuildMemberUpdate(t *testing.T) {
	handleArikawaGuildMemberUpdate(nil, nil)

	store, db, cleanup := setupTestDB(t)
	if store == nil {
		t.Skip("skipping db tests")
	}
	defer cleanup()
	cm := newTestConfigManager(t)
	_, _ = cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{{GuildID: "456", BotInstanceTokens: map[string]files.EncryptedString{"test": "token"}, FeatureRouting: map[string]string{"stats": "test"}, Features: files.FeatureToggles{StatsChannels: testBoolPtr(true)}, Stats: files.StatsConfig{Enabled: true, Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}}}
		return nil
	})

	svc := domain.NewStatsService(nil, cm, store, slog.Default(), "test")
	if store != nil {
		_, _ = db.Exec(context.Background(), "INSERT INTO guilds (id) VALUES ('456') ON CONFLICT DO NOTHING")
	}
	e := &gateway.GuildMemberUpdateEvent{
		User: discord.User{
			ID:  discord.UserID(123),
			Bot: false,
		},
		RoleIDs: []discord.RoleID{discord.RoleID(1)},
	}
	e.GuildID = discord.GuildID(456)

	// Should not panic
	handleArikawaGuildMemberUpdate(svc, e)
}
