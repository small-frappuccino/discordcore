package logging

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func newLoggingStore(t *testing.T, _ string) (*storage.Store, *sql.DB) {
	t.Helper()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}
	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store, db
}

func newLoggingConfigManager(t *testing.T, guildID string, channels files.ChannelsConfig) *files.ConfigManager {
	t.Helper()

	mgr := files.NewMemoryConfigManager()
	if err := mgr.AddGuildConfig(files.GuildConfig{
		GuildID:  guildID,
		Channels: channels,
	}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	return mgr
}

func queryIntFromStoreDB(t *testing.T, db *sql.DB, query string, args ...interface{}) int {
	t.Helper()

	var value int
	if err := db.QueryRow(rebindQuestionPlaceholders(query), args...).Scan(&value); err != nil {
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

func dailyMessageMetricCount(t *testing.T, db *sql.DB, guildID, channelID, userID string, at time.Time) int {
	t.Helper()
	return queryIntFromStoreDB(
		t,
		db,
		`SELECT count FROM daily_message_metrics WHERE guild_id=? AND channel_id=? AND user_id=? AND day=?`,
		guildID,
		channelID,
		userID,
		utcDay(at),
	)
}

func dailyMemberMetricCount(t *testing.T, db *sql.DB, tableName, guildID, userID string, at time.Time) int {
	t.Helper()
	return queryIntFromStoreDB(
		t,
		db,
		`SELECT count FROM `+tableName+` WHERE guild_id=? AND user_id=? AND day=?`,
		guildID,
		userID,
		utcDay(at),
	)
}

func dailyReactionMetricCount(t *testing.T, db *sql.DB, guildID, channelID, userID string, at time.Time) int {
	t.Helper()
	return queryIntFromStoreDB(
		t,
		db,
		`SELECT count FROM daily_reaction_metrics WHERE guild_id=? AND channel_id=? AND user_id=? AND day=?`,
		guildID,
		channelID,
		userID,
		utcDay(at),
	)
}

func messageHistoryCount(t *testing.T, db *sql.DB, guildID, messageID, eventType string) int {
	t.Helper()
	return queryIntFromStoreDB(
		t,
		db,
		`SELECT COUNT(*) FROM messages_history WHERE guild_id=? AND message_id=? AND event_type=?`,
		guildID,
		messageID,
		eventType,
	)
}

func waitForCondition(t *testing.T, timeout time.Duration, description string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if fn() {
		return
	}
	t.Fatalf("timed out waiting for %s", description)
}

func waitForDailyMessageMetricCount(t *testing.T, db *sql.DB, guildID, channelID, userID string, at time.Time, want int) {
	t.Helper()
	waitForCondition(t, 3*time.Second, fmt.Sprintf("daily_message_metrics=%d for %s/%s/%s", want, guildID, channelID, userID), func() bool {
		return dailyMessageMetricCount(t, db, guildID, channelID, userID, at) == want
	})
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

func rebindQuestionPlaceholders(query string) string {
	if query == "" {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	i := 1
	for idx := 0; idx < len(query); idx++ {
		if query[idx] == '?' {
			b.WriteString("$")
			b.WriteString(strconv.Itoa(i))
			i++
			continue
		}
		b.WriteByte(query[idx])
	}
	return b.String()
}
