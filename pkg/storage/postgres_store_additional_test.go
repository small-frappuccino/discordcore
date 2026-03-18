package storage

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func newTempStore(t *testing.T) *Store {
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

	store := NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSchemaInitialized(t *testing.T) {
	store := newTempStore(t)
	rows, err := store.db.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema()`)
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
		"moderation_cases": false,
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

func TestInitRepairsMissingMemberJoinLastSeenColumn(t *testing.T) {
	store := newTempStore(t)
	joinedAt := time.Date(2022, 4, 5, 6, 7, 8, 0, time.UTC)
	if err := store.UpsertMemberJoin("g1", "u1", joinedAt); err != nil {
		t.Fatalf("seed member join: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	migrator := persistence.NewPostgresMigrator(store.db)
	if err := migrator.Down(ctx, 1); err != nil {
		t.Fatalf("rollback migration 3: %v", err)
	}

	var exists bool
	if err := store.db.QueryRow(
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'member_joins'
			  AND column_name = 'last_seen_at'
		)`,
	).Scan(&exists); err != nil {
		t.Fatalf("check column removed: %v", err)
	}
	if exists {
		t.Fatal("expected last_seen_at column to be absent after rollback")
	}

	if err := store.Init(); err != nil {
		t.Fatalf("Init() should repair missing last_seen_at column: %v", err)
	}

	if err := store.db.QueryRow(
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'member_joins'
			  AND column_name = 'last_seen_at'
		)`,
	).Scan(&exists); err != nil {
		t.Fatalf("check column restored: %v", err)
	}
	if !exists {
		t.Fatal("expected last_seen_at column to be restored by Init()")
	}

	var gotJoinedAt, gotLastSeen time.Time
	if err := store.db.QueryRow(
		`SELECT joined_at, last_seen_at FROM member_joins WHERE guild_id = 'g1' AND user_id = 'u1'`,
	).Scan(&gotJoinedAt, &gotLastSeen); err != nil {
		t.Fatalf("read repaired join row: %v", err)
	}
	if !gotJoinedAt.Equal(joinedAt) {
		t.Fatalf("expected joined_at to remain %s, got %s", joinedAt.Format(time.RFC3339), gotJoinedAt.Format(time.RFC3339))
	}
	if !gotLastSeen.Equal(joinedAt) {
		t.Fatalf("expected last_seen_at to backfill from joined_at %s, got %s", joinedAt.Format(time.RFC3339), gotLastSeen.Format(time.RFC3339))
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

func TestRuntimeMetadataIsNamespacedByBot(t *testing.T) {
	store := newTempStore(t)
	aliceHeartbeat := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	yuzuhaHeartbeat := time.Now().UTC().Truncate(time.Second)
	aliceLastEvent := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Second)
	yuzuhaLastEvent := time.Now().UTC().Add(-30 * time.Second).Truncate(time.Second)

	if err := store.SetHeartbeatForBot("alice", aliceHeartbeat); err != nil {
		t.Fatalf("set alice heartbeat: %v", err)
	}
	if err := store.SetHeartbeatForBot("yuzuha", yuzuhaHeartbeat); err != nil {
		t.Fatalf("set yuzuha heartbeat: %v", err)
	}
	if err := store.SetLastEventForBot("alice", aliceLastEvent); err != nil {
		t.Fatalf("set alice last event: %v", err)
	}
	if err := store.SetLastEventForBot("yuzuha", yuzuhaLastEvent); err != nil {
		t.Fatalf("set yuzuha last event: %v", err)
	}

	gotAliceHeartbeat, ok, err := store.GetHeartbeatForBot("alice")
	if err != nil || !ok || !gotAliceHeartbeat.Equal(aliceHeartbeat) {
		t.Fatalf("unexpected alice heartbeat: got=%v ok=%v err=%v", gotAliceHeartbeat, ok, err)
	}
	gotYuzuhaHeartbeat, ok, err := store.GetHeartbeatForBot("yuzuha")
	if err != nil || !ok || !gotYuzuhaHeartbeat.Equal(yuzuhaHeartbeat) {
		t.Fatalf("unexpected yuzuha heartbeat: got=%v ok=%v err=%v", gotYuzuhaHeartbeat, ok, err)
	}
	gotAliceLastEvent, ok, err := store.GetLastEventForBot("alice")
	if err != nil || !ok || !gotAliceLastEvent.Equal(aliceLastEvent) {
		t.Fatalf("unexpected alice last event: got=%v ok=%v err=%v", gotAliceLastEvent, ok, err)
	}
	gotYuzuhaLastEvent, ok, err := store.GetLastEventForBot("yuzuha")
	if err != nil || !ok || !gotYuzuhaLastEvent.Equal(yuzuhaLastEvent) {
		t.Fatalf("unexpected yuzuha last event: got=%v ok=%v err=%v", gotYuzuhaLastEvent, ok, err)
	}
}

func TestInsertMessageVersion_ConcurrentAutoVersioning(t *testing.T) {
	store := newTempStore(t)

	const writers = 24
	var wg sync.WaitGroup
	errCh := make(chan error, writers)
	start := make(chan struct{})

	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errCh <- store.InsertMessageVersion(MessageVersion{
				GuildID:   "g",
				MessageID: "m",
				ChannelID: "c",
				AuthorID:  "u",
				EventType: "edit",
				Content:   fmt.Sprintf("content-%d", i),
				CreatedAt: time.Now().UTC(),
			})
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("insert message version concurrently: %v", err)
		}
	}

	rows, err := store.db.Query(`SELECT version FROM messages_history WHERE guild_id='g' AND message_id='m' ORDER BY version`)
	if err != nil {
		t.Fatalf("query versions: %v", err)
	}
	defer rows.Close()

	versions := make([]int, 0, writers)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate versions: %v", err)
	}

	if len(versions) != writers {
		t.Fatalf("expected %d versions, got %d (%v)", writers, len(versions), versions)
	}
	for i, version := range versions {
		expected := i + 1
		if version != expected {
			t.Fatalf("expected contiguous versions starting at 1; got %v", versions)
		}
	}
}

func TestNextModerationCaseNumberSequentialPerGuild(t *testing.T) {
	store := newTempStore(t)

	n1, err := store.NextModerationCaseNumber("g1")
	if err != nil {
		t.Fatalf("next case 1: %v", err)
	}
	n2, err := store.NextModerationCaseNumber("g1")
	if err != nil {
		t.Fatalf("next case 2: %v", err)
	}
	if n1 != 1 || n2 != 2 {
		t.Fatalf("unexpected sequence for g1: n1=%d n2=%d", n1, n2)
	}

	nOther, err := store.NextModerationCaseNumber("g2")
	if err != nil {
		t.Fatalf("next case g2: %v", err)
	}
	if nOther != 1 {
		t.Fatalf("expected independent sequence for g2 to start at 1, got %d", nOther)
	}
}

func TestNextModerationCaseNumberRejectsEmptyGuildID(t *testing.T) {
	store := newTempStore(t)

	if _, err := store.NextModerationCaseNumber("   "); err == nil {
		t.Fatal("expected error for empty guildID")
	}
}
