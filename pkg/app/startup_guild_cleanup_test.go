package app

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func openStartupCleanupFixtures(t *testing.T) (*files.ConfigManager, *storage.Store, *sql.DB) {
	t.Helper()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve postgres test dsn: %v", err)
	}

	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated postgres database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated postgres database: %v", err)
		}
	})

	if err := persistence.NewPostgresMigrator(db).Up(context.Background()); err != nil {
		t.Fatalf("apply postgres migrations: %v", err)
	}

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("init storage store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	configManager := files.NewConfigManagerWithStore(files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey))
	return configManager, store, db
}

func TestPruneStartupGuildReferencesRemovesDisallowedGuildsFromConfigAndStore(t *testing.T) {
	configManager, store, db := openStartupCleanupFixtures(t)
	ctx := context.Background()

	allowedGuildA := "1375650791251120179"
	allowedGuildB := "1390069056530419823"
	removedGuild := "guild-denied"

	if err := configManager.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, err := configManager.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: allowedGuildA},
			{GuildID: removedGuild},
			{GuildID: allowedGuildB},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	if _, err := store.CreateQOTDQuestion(ctx, storage.QOTDQuestionRecord{
		GuildID: removedGuild,
		DeckID:  "default",
		Body:    "Drop this guild",
		Status:  "ready",
	}); err != nil {
		t.Fatalf("seed removed guild qotd question: %v", err)
	}
	if _, err := store.CreateQOTDQuestion(ctx, storage.QOTDQuestionRecord{
		GuildID: allowedGuildA,
		DeckID:  "default",
		Body:    "Keep this guild",
		Status:  "ready",
	}); err != nil {
		t.Fatalf("seed allowed guild qotd question: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO messages (guild_id, message_id, channel_id, author_id, cached_at) VALUES ($1, $2, $3, $4, $5)`, removedGuild, "m-drop", "c-drop", "u-drop", time.Now().UTC()); err != nil {
		t.Fatalf("seed removed guild message: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO messages (guild_id, message_id, channel_id, author_id, cached_at) VALUES ($1, $2, $3, $4, $5)`, allowedGuildB, "m-keep", "c-keep", "u-keep", time.Now().UTC()); err != nil {
		t.Fatalf("seed allowed guild message: %v", err)
	}

	removedGuildIDs, err := pruneStartupGuildReferences(ctx, configManager, store)
	if err != nil {
		t.Fatalf("prune startup guild references: %v", err)
	}
	if len(removedGuildIDs) != 1 || removedGuildIDs[0] != removedGuild {
		t.Fatalf("unexpected removed guild ids: %+v", removedGuildIDs)
	}

	loaded := configManager.SnapshotConfig()
	if len(loaded.Guilds) != 2 {
		t.Fatalf("expected two allowed guilds to remain, got %+v", loaded.Guilds)
	}
	if loaded.Guilds[0].GuildID != allowedGuildA || loaded.Guilds[1].GuildID != allowedGuildB {
		t.Fatalf("unexpected remaining guild config order: %+v", loaded.Guilds)
	}

	storedConfig, err := files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey).Load()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(storedConfig.Guilds) != 2 {
		t.Fatalf("expected persisted config to keep only allowed guilds, got %+v", storedConfig.Guilds)
	}

	assertRowCountForTest(t, db, `SELECT COUNT(*) FROM messages WHERE guild_id = $1`, removedGuild, 0)
	assertRowCountForTest(t, db, `SELECT COUNT(*) FROM messages WHERE guild_id = $1`, allowedGuildB, 1)
	assertRowCountForTest(t, db, `SELECT COUNT(*) FROM qotd_questions WHERE guild_id = $1`, removedGuild, 0)
	assertRowCountForTest(t, db, `SELECT COUNT(*) FROM qotd_questions WHERE guild_id = $1`, allowedGuildA, 1)

	removedAgain, err := pruneStartupGuildReferences(ctx, configManager, store)
	if err != nil {
		t.Fatalf("prune startup guild references on second pass: %v", err)
	}
	if len(removedAgain) != 0 {
		t.Fatalf("expected second cleanup pass to be a no-op, got %+v", removedAgain)
	}
}

func assertRowCountForTest(t *testing.T, db *sql.DB, query string, guildID string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query, guildID).Scan(&got); err != nil {
		t.Fatalf("count rows for %q: %v", guildID, err)
	}
	if got != want {
		t.Fatalf("unexpected row count for %q: got=%d want=%d", guildID, got, want)
	}
}