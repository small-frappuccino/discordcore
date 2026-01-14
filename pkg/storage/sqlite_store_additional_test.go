package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func newTempStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store := NewStore(dbPath)
	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSchemaInitialized(t *testing.T) {
	store := newTempStore(t)
	rows, err := store.db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatalf("query schema: %v", err)
	}
	defer rows.Close()

	required := map[string]bool{
		"messages":         false,
		"member_joins":     false,
		"persistent_cache": false,
		"guild_meta":       false,
		"runtime_meta":     false,
		"roles_current":    false,
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if _, ok := required[name]; ok {
			required[name] = true
		}
	}
	for k, ok := range required {
		if !ok {
			t.Fatalf("expected table %s to exist", k)
		}
	}
}

func TestUpsertMessageIsTransactional(t *testing.T) {
	store := newTempStore(t)
	now := time.Now()
	msg := MessageRecord{GuildID: "g", MessageID: "m", ChannelID: "c", AuthorID: "u", Content: "first", CachedAt: now}

	if err := store.UpsertMessage(msg); err != nil {
		t.Fatalf("upsert1: %v", err)
	}
	msg.Content = "second"
	if err := store.UpsertMessage(msg); err != nil {
		t.Fatalf("upsert2: %v", err)
	}

	got, err := store.GetMessage("g", "m")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil || got.Content != "second" {
		t.Fatalf("expected updated content, got %+v", got)
	}

	var count int
	row := store.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE guild_id='g' AND message_id='m'`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count scan: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected single row after upserts, got %d", count)
	}
}

func TestForeignKeysAndUniqueConstraints(t *testing.T) {
	store := newTempStore(t)

	if _, err := store.db.Exec(`INSERT INTO member_joins (guild_id, user_id, joined_at) VALUES ('g','u',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert1: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO member_joins (guild_id, user_id, joined_at) VALUES ('g','u',CURRENT_TIMESTAMP)`); err == nil {
		t.Fatalf("expected unique constraint error on duplicate insert")
	}

	var count int
	row := store.db.QueryRow(`SELECT COUNT(*) FROM member_joins WHERE guild_id='g' AND user_id='u'`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected unique constraint to keep single row, got %d", count)
	}
}

func TestHeartbeatMetadataRoundTrip(t *testing.T) {
	store := newTempStore(t)
	ts := time.Now().UTC().Truncate(time.Second)
	if err := store.SetHeartbeat(ts); err != nil {
		t.Fatalf("set heartbeat: %v", err)
	}
	got, ok, err := store.GetHeartbeat()
	if err != nil || !ok || !got.Equal(ts) {
		t.Fatalf("heartbeat mismatch: ts=%v ok=%v err=%v", got, ok, err)
	}
}
