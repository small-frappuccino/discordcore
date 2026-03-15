package files

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func openIsolatedPostgresConfigStore(t *testing.T) *PostgresConfigStore {
	t.Helper()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve postgres test dsn: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, cleanup, err := testdb.OpenIsolatedDatabase(ctx, baseDSN)
	if err != nil {
		t.Fatalf("open isolated postgres database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated postgres database: %v", err)
		}
	})

	return NewPostgresConfigStore(db, "test")
}

func TestPostgresConfigStoreSaveLoadRoundTrip(t *testing.T) {
	store := openIsolatedPostgresConfigStore(t)

	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("check config existence before save: %v", err)
	}
	if exists {
		t.Fatalf("expected config row to be absent before save")
	}

	cfg := &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "guild-1",
				Channels: ChannelsConfig{
					Commands:      "channel-1",
					AvatarLogging: "channel-2",
				},
				Roles: RolesConfig{
					Allowed: []string{"role-1", "role-2"},
				},
			},
		},
		Features: FeatureToggles{
			Services: FeatureServiceToggles{
				Monitoring: boolPtr(true),
				Commands:   boolPtr(false),
			},
			Logging: FeatureLoggingToggles{
				AvatarLogging: boolPtr(false),
			},
		},
		RuntimeConfig: RuntimeConfig{
			BotTheme: "matrix",
			Database: DatabaseRuntimeConfig{
				Driver:        "postgres",
				DatabaseURL:   "postgres://example.invalid/test",
				MaxOpenConns:  7,
				MaxIdleConns:  3,
				PingTimeoutMS: 4000,
			},
		},
	}

	if err := store.Save(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	exists, err = store.Exists()
	if err != nil {
		t.Fatalf("check config existence after save: %v", err)
	}
	if !exists {
		t.Fatalf("expected config row to exist after save")
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if got := loaded.RuntimeConfig.BotTheme; got != "matrix" {
		t.Fatalf("expected bot theme matrix, got %q", got)
	}
	if got := loaded.RuntimeConfig.Database.DatabaseURL; got != "postgres://example.invalid/test" {
		t.Fatalf("expected database url to round-trip, got %q", got)
	}
	if len(loaded.Guilds) != 1 {
		t.Fatalf("expected one guild, got %d", len(loaded.Guilds))
	}
	if got := loaded.Guilds[0].Channels.AvatarLogging; got != "channel-2" {
		t.Fatalf("expected avatar logging channel channel-2, got %q", got)
	}
	if resolved := loaded.ResolveFeatures("guild-1"); resolved.Services.Commands {
		t.Fatalf("expected commands feature override to remain disabled after round-trip")
	}
}
