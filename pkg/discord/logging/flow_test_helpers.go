package logging

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	_ "modernc.org/sqlite"
)

func newLoggingStore(t *testing.T, filename string) (*storage.Store, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), filename)
	store := storage.NewStore(dbPath)
	if err := store.Init(); err != nil {
		t.Fatalf("init sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store, dbPath
}

func newLoggingConfigManager(t *testing.T, guildID string, channels files.ChannelsConfig) *files.ConfigManager {
	t.Helper()

	mgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := mgr.AddGuildConfig(files.GuildConfig{
		GuildID:  guildID,
		Channels: channels,
	}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	return mgr
}

func queryIntFromStoreDB(t *testing.T, dbPath, query string, args ...interface{}) int {
	t.Helper()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db for assertion: %v", err)
	}
	defer db.Close()

	var value int
	if err := db.QueryRow(query, args...).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return 0
		}
		t.Fatalf("query assertion failed: %v", err)
	}
	return value
}

func utcDay(at time.Time) string {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	return time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

func dailyMessageMetricCount(t *testing.T, dbPath, guildID, channelID, userID string, at time.Time) int {
	t.Helper()
	return queryIntFromStoreDB(
		t,
		dbPath,
		`SELECT count FROM daily_message_metrics WHERE guild_id=? AND channel_id=? AND user_id=? AND day=?`,
		guildID,
		channelID,
		userID,
		utcDay(at),
	)
}

func dailyMemberMetricCount(t *testing.T, dbPath, tableName, guildID, userID string, at time.Time) int {
	t.Helper()
	return queryIntFromStoreDB(
		t,
		dbPath,
		`SELECT count FROM `+tableName+` WHERE guild_id=? AND user_id=? AND day=?`,
		guildID,
		userID,
		utcDay(at),
	)
}

func messageHistoryCount(t *testing.T, dbPath, guildID, messageID, eventType string) int {
	t.Helper()
	return queryIntFromStoreDB(
		t,
		dbPath,
		`SELECT COUNT(*) FROM messages_history WHERE guild_id=? AND message_id=? AND event_type=?`,
		guildID,
		messageID,
		eventType,
	)
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]int, len(a))
	for _, v := range a {
		m[v]++
	}
	for _, v := range b {
		m[v]--
		if m[v] < 0 {
			return false
		}
	}
	for _, count := range m {
		if count != 0 {
			return false
		}
	}
	return true
}
