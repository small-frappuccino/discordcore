package config

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
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

	return NewPostgresConfigStore(db, "test", nil)
}

func TestPostgresConfigStoreSaveLoadRoundTrip(t *testing.T) {
	t.Parallel()
	store := openIsolatedPostgresConfigStore(t)

	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("check config existence before save: %v", err)
	}
	if exists {
		t.Fatalf("expected config row to be absent before save")
	}

	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "guild-1",
				Channels: files.ChannelsConfig{
					Commands:      "channel-1",
					AvatarLogging: "channel-2",
				},
				Roles: files.RolesConfig{
					Allowed: []string{"role-1", "role-2"},
				},
			},
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: BoolPtr(true),
				Commands:   BoolPtr(false),
			},
			Logging: files.FeatureLoggingToggles{
				AvatarLogging: BoolPtr(false),
			},
		},
		RuntimeConfig: files.RuntimeConfig{
			BotTheme: "matrix",
			Database: files.DatabaseRuntimeConfig{
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

func BoolPtr(b bool) *bool { return &b }
