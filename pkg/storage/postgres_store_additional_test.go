package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

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
		"messages":                 false,
		"member_joins":             false,
		"persistent_cache":         false,
		"guild_meta":               false,
		"runtime_meta":             false,
		"moderation_cases":         false,
		"moderation_warnings":      false,
		"roles_current":            false,
		"message_version_counters": false,
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

func TestInitRepairsMissingMemberJoinStateColumns(t *testing.T) {
	store := newTempStore(t)
	joinedAt := time.Date(2022, 4, 5, 6, 7, 8, 0, time.UTC)
	if err := store.UpsertMemberJoin("g1", "u1", joinedAt); err != nil {
		t.Fatalf("seed member join: %v", err)
	}

	if _, err := store.db.Exec(`
		ALTER TABLE member_joins
			DROP COLUMN IF EXISTS left_at,
			DROP COLUMN IF EXISTS is_bot,
			DROP COLUMN IF EXISTS last_seen_at
	`); err != nil {
		t.Fatalf("drop member join state columns: %v", err)
	}

	missing, err := store.missingColumns(context.Background(), "member_joins", requiredSchemaColumns["member_joins"])
	if err != nil {
		t.Fatalf("missingColumns(before repair): %v", err)
	}
	if len(missing) != len(requiredSchemaColumns["member_joins"]) {
		t.Fatalf("expected all member_joins state columns to be absent, missing=%v", missing)
	}

	if err := store.Init(); err != nil {
		t.Fatalf("Init() should repair missing member_joins state columns: %v", err)
	}

	missing, err = store.missingColumns(context.Background(), "member_joins", requiredSchemaColumns["member_joins"])
	if err != nil {
		t.Fatalf("missingColumns(after repair): %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("expected member_joins state columns to be restored by Init(), missing=%v", missing)
	}

	var gotJoinedAt, gotLastSeen time.Time
	var gotIsBot sql.NullBool
	var gotLeftAt sql.NullTime
	if err := store.db.QueryRow(
		`SELECT joined_at, last_seen_at, is_bot, left_at FROM member_joins WHERE guild_id = 'g1' AND user_id = 'u1'`,
	).Scan(&gotJoinedAt, &gotLastSeen, &gotIsBot, &gotLeftAt); err != nil {
		t.Fatalf("read repaired join row: %v", err)
	}
	if !gotJoinedAt.Equal(joinedAt) {
		t.Fatalf("expected joined_at to remain %s, got %s", joinedAt.Format(time.RFC3339), gotJoinedAt.Format(time.RFC3339))
	}
	if !gotLastSeen.Equal(joinedAt) {
		t.Fatalf("expected last_seen_at to backfill from joined_at %s, got %s", joinedAt.Format(time.RFC3339), gotLastSeen.Format(time.RFC3339))
	}
	if gotIsBot.Valid {
		t.Fatalf("expected repaired is_bot to remain unknown, got %v", gotIsBot.Bool)
	}
	if gotLeftAt.Valid {
		t.Fatalf("expected repaired left_at to remain null, got %s", gotLeftAt.Time.Format(time.RFC3339))
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

func TestUpsertMessagesContextDeduplicatesByMessage(t *testing.T) {
	store := newTempStore(t)

	now := time.Now().UTC()
	err := store.UpsertMessagesContext(context.Background(), []MessageRecord{
		{GuildID: "g", MessageID: "m1", ChannelID: "c", AuthorID: "u", Content: "first", CachedAt: now},
		{GuildID: "g", MessageID: "m1", ChannelID: "c", AuthorID: "u", Content: "second", CachedAt: now.Add(time.Second)},
		{GuildID: "g", MessageID: "m2", ChannelID: "c", AuthorID: "u", Content: "other", CachedAt: now},
	})
	if err != nil {
		t.Fatalf("UpsertMessagesContext() failed: %v", err)
	}

	got, err := store.GetMessage("g", "m1")
	if err != nil {
		t.Fatalf("GetMessage(m1) failed: %v", err)
	}
	if got == nil || got.Content != "second" {
		t.Fatalf("expected latest content for m1, got %+v", got)
	}

	got, err = store.GetMessage("g", "m2")
	if err != nil {
		t.Fatalf("GetMessage(m2) failed: %v", err)
	}
	if got == nil || got.Content != "other" {
		t.Fatalf("expected second message to be inserted, got %+v", got)
	}
}

func TestCreateAndListModerationWarnings(t *testing.T) {
	store := newTempStore(t)

	first, err := store.CreateModerationWarning(
		"g1",
		"user-1",
		"mod-1",
		"First warning",
		time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("CreateModerationWarning(first) failed: %v", err)
	}
	if first.CaseNumber != 1 {
		t.Fatalf("expected first case number to be 1, got %+v", first)
	}

	second, err := store.CreateModerationWarning(
		"g1",
		"user-1",
		"mod-2",
		"Second warning",
		time.Date(2026, 3, 24, 13, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("CreateModerationWarning(second) failed: %v", err)
	}
	if second.CaseNumber != 2 {
		t.Fatalf("expected second case number to be 2, got %+v", second)
	}

	otherUser, err := store.CreateModerationWarning(
		"g1",
		"user-2",
		"mod-1",
		"Other user warning",
		time.Date(2026, 3, 24, 14, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("CreateModerationWarning(other user) failed: %v", err)
	}
	if otherUser.CaseNumber != 3 {
		t.Fatalf("expected shared guild case sequence, got %+v", otherUser)
	}

	warnings, err := store.ListModerationWarnings("g1", "user-1", 10)
	if err != nil {
		t.Fatalf("ListModerationWarnings() failed: %v", err)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %+v", warnings)
	}
	if warnings[0].CaseNumber != 2 || warnings[0].Reason != "Second warning" {
		t.Fatalf("expected latest warning first, got %+v", warnings[0])
	}
	if warnings[1].CaseNumber != 1 || warnings[1].Reason != "First warning" {
		t.Fatalf("expected oldest warning second, got %+v", warnings[1])
	}
}

func TestDeleteMessagesContextDeduplicatesKeys(t *testing.T) {
	store := newTempStore(t)

	now := time.Now().UTC()
	if err := store.UpsertMessagesContext(context.Background(), []MessageRecord{
		{GuildID: "g", MessageID: "m1", ChannelID: "c", AuthorID: "u", Content: "first", CachedAt: now},
		{GuildID: "g", MessageID: "m2", ChannelID: "c", AuthorID: "u", Content: "second", CachedAt: now},
	}); err != nil {
		t.Fatalf("seed messages: %v", err)
	}

	err := store.DeleteMessagesContext(context.Background(), []MessageDeleteKey{
		{GuildID: "g", MessageID: "m1"},
		{GuildID: "g", MessageID: "m1"},
		{GuildID: "g", MessageID: "m2"},
	})
	if err != nil {
		t.Fatalf("DeleteMessagesContext() failed: %v", err)
	}

	if got, err := store.GetMessage("g", "m1"); err != nil {
		t.Fatalf("GetMessage(m1) failed: %v", err)
	} else if got != nil {
		t.Fatalf("expected m1 to be deleted, got %+v", got)
	}
	if got, err := store.GetMessage("g", "m2"); err != nil {
		t.Fatalf("GetMessage(m2) failed: %v", err)
	} else if got != nil {
		t.Fatalf("expected m2 to be deleted, got %+v", got)
	}
}

func TestInsertMessageVersionsBatchContextPreservesExplicitVersions(t *testing.T) {
	store := newTempStore(t)

	createdAt := time.Now().UTC()
	err := store.InsertMessageVersionsBatchContext(context.Background(), []MessageVersion{
		{
			GuildID:   "g",
			MessageID: "m",
			ChannelID: "c",
			AuthorID:  "u",
			Version:   1,
			EventType: "create",
			Content:   "before",
			CreatedAt: createdAt,
		},
		{
			GuildID:   "g",
			MessageID: "m",
			ChannelID: "c",
			AuthorID:  "u",
			Version:   2,
			EventType: "edit",
			Content:   "after",
			CreatedAt: createdAt.Add(time.Second),
		},
	})
	if err != nil {
		t.Fatalf("InsertMessageVersionsBatchContext() failed: %v", err)
	}

	rows, err := store.db.Query(`SELECT version, event_type FROM messages_history WHERE guild_id='g' AND message_id='m' ORDER BY version`)
	if err != nil {
		t.Fatalf("query versions: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var version int
		var eventType string
		if err := rows.Scan(&version, &eventType); err != nil {
			t.Fatalf("scan version row: %v", err)
		}
		got = append(got, fmt.Sprintf("%d:%s", version, eventType))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate versions: %v", err)
	}
	if want := []string{"1:create", "2:edit"}; !sameStringSlice(got, want) {
		t.Fatalf("unexpected persisted versions: got=%v want=%v", got, want)
	}

	var lastVersion int
	if err := store.db.QueryRow(
		`SELECT last_version FROM message_version_counters WHERE guild_id='g' AND message_id='m'`,
	).Scan(&lastVersion); err != nil {
		t.Fatalf("query version counter: %v", err)
	}
	if lastVersion != 2 {
		t.Fatalf("expected version counter to track explicit batch at 2, got %d", lastVersion)
	}
}

func TestInsertMessageVersionsMixedBatchContextAssignsContiguousVersions(t *testing.T) {
	store := newTempStore(t)

	createdAt := time.Now().UTC()
	err := store.InsertMessageVersionsMixedBatchContext(context.Background(), []MessageVersion{
		{
			GuildID:   "g",
			MessageID: "m",
			ChannelID: "c",
			AuthorID:  "u",
			Version:   1,
			EventType: "create",
			Content:   "before",
			CreatedAt: createdAt,
		},
		{
			GuildID:   "g",
			MessageID: "m",
			ChannelID: "c",
			AuthorID:  "u",
			EventType: "edit",
			Content:   "after",
			CreatedAt: createdAt.Add(time.Second),
		},
		{
			GuildID:   "g",
			MessageID: "m",
			ChannelID: "c",
			AuthorID:  "u",
			EventType: "delete",
			Content:   "after",
			CreatedAt: createdAt.Add(2 * time.Second),
		},
	})
	if err != nil {
		t.Fatalf("InsertMessageVersionsMixedBatchContext() failed: %v", err)
	}

	rows, err := store.db.Query(`SELECT version, event_type FROM messages_history WHERE guild_id='g' AND message_id='m' ORDER BY version`)
	if err != nil {
		t.Fatalf("query versions: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var version int
		var eventType string
		if err := rows.Scan(&version, &eventType); err != nil {
			t.Fatalf("scan version row: %v", err)
		}
		got = append(got, fmt.Sprintf("%d:%s", version, eventType))
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate versions: %v", err)
	}
	if want := []string{"1:create", "2:edit", "3:delete"}; !sameStringSlice(got, want) {
		t.Fatalf("unexpected persisted versions: got=%v want=%v", got, want)
	}
}

func TestInsertMessageVersionBackfillsCounterFromHistoryWhenMissing(t *testing.T) {
	store := newTempStore(t)

	createdAt := time.Now().UTC().Add(-time.Minute)
	if _, err := store.db.Exec(
		`INSERT INTO messages_history (guild_id, message_id, channel_id, author_id, version, event_type, content, attachments, embeds_count, stickers, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, 0, 0, 0, $8)`,
		"g",
		"m",
		"c",
		"u",
		5,
		"edit",
		"before",
		createdAt,
	); err != nil {
		t.Fatalf("seed history row: %v", err)
	}

	if err := store.InsertMessageVersion(MessageVersion{
		GuildID:   "g",
		MessageID: "m",
		ChannelID: "c",
		AuthorID:  "u",
		EventType: "delete",
		Content:   "after",
		CreatedAt: createdAt.Add(time.Second),
	}); err != nil {
		t.Fatalf("InsertMessageVersion() failed: %v", err)
	}

	rows, err := store.db.Query(`SELECT version FROM messages_history WHERE guild_id='g' AND message_id='m' ORDER BY version`)
	if err != nil {
		t.Fatalf("query versions: %v", err)
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("scan version row: %v", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate versions: %v", err)
	}
	if want := []int{5, 6}; len(versions) != len(want) || versions[0] != want[0] || versions[1] != want[1] {
		t.Fatalf("expected backfilled versions %v, got %v", want, versions)
	}

	var lastVersion int
	if err := store.db.QueryRow(
		`SELECT last_version FROM message_version_counters WHERE guild_id='g' AND message_id='m'`,
	).Scan(&lastVersion); err != nil {
		t.Fatalf("query version counter: %v", err)
	}
	if lastVersion != 6 {
		t.Fatalf("expected backfilled counter to end at 6, got %d", lastVersion)
	}
}

func TestIncrementDailyMessageCountsContextAggregatesDeltas(t *testing.T) {
	store := newTempStore(t)

	err := store.IncrementDailyMessageCountsContext(context.Background(), []DailyMessageCountDelta{
		{GuildID: "g", ChannelID: "c", UserID: "u", Day: "2026-03-19", Count: 1},
		{GuildID: "g", ChannelID: "c", UserID: "u", Day: "2026-03-19", Count: 2},
		{GuildID: "g", ChannelID: "c", UserID: "v", Day: "2026-03-19", Count: 4},
	})
	if err != nil {
		t.Fatalf("IncrementDailyMessageCountsContext() failed: %v", err)
	}

	var count int
	if err := store.db.QueryRow(
		`SELECT count FROM daily_message_metrics WHERE guild_id='g' AND channel_id='c' AND user_id='u' AND day='2026-03-19'`,
	).Scan(&count); err != nil {
		t.Fatalf("query aggregated count: %v", err)
	}
	if count != 3 {
		t.Fatalf("expected aggregated count 3, got %d", count)
	}

	if err := store.db.QueryRow(
		`SELECT count FROM daily_message_metrics WHERE guild_id='g' AND channel_id='c' AND user_id='v' AND day='2026-03-19'`,
	).Scan(&count); err != nil {
		t.Fatalf("query second bucket count: %v", err)
	}
	if count != 4 {
		t.Fatalf("expected second bucket count 4, got %d", count)
	}
}

func sameStringSlice(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
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
	got, ok, err := store.Heartbeat()
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

	gotAliceHeartbeat, ok, err := store.HeartbeatForBot("alice")
	if err != nil || !ok || !gotAliceHeartbeat.Equal(aliceHeartbeat) {
		t.Fatalf("unexpected alice heartbeat: got=%v ok=%v err=%v", gotAliceHeartbeat, ok, err)
	}
	gotYuzuhaHeartbeat, ok, err := store.HeartbeatForBot("yuzuha")
	if err != nil || !ok || !gotYuzuhaHeartbeat.Equal(yuzuhaHeartbeat) {
		t.Fatalf("unexpected yuzuha heartbeat: got=%v ok=%v err=%v", gotYuzuhaHeartbeat, ok, err)
	}
	gotAliceLastEvent, ok, err := store.LastEventForBot("alice")
	if err != nil || !ok || !gotAliceLastEvent.Equal(aliceLastEvent) {
		t.Fatalf("unexpected alice last event: got=%v ok=%v err=%v", gotAliceLastEvent, ok, err)
	}
	gotYuzuhaLastEvent, ok, err := store.LastEventForBot("yuzuha")
	if err != nil || !ok || !gotYuzuhaLastEvent.Equal(yuzuhaLastEvent) {
		t.Fatalf("unexpected yuzuha last event: got=%v ok=%v err=%v", gotYuzuhaLastEvent, ok, err)
	}
}

func TestUpsertGuildMemberSnapshotsContext_BatchesAvatarRolesAndJoins(t *testing.T) {
	store := newTempStore(t)

	ctx := context.Background()
	guildID := "g1"
	firstSeen := time.Date(2022, 2, 10, 9, 0, 0, 0, time.UTC)
	secondSeen := firstSeen.Add(24 * time.Hour)

	err := store.UpsertGuildMemberSnapshotsContext(ctx, guildID, []GuildMemberSnapshot{
		{
			UserID:     "u1",
			AvatarHash: "avatar-old",
			HasAvatar:  true,
			Roles:      []string{"r1", "r2", "r2"},
			HasRoles:   true,
			JoinedAt:   firstSeen,
		},
		{
			UserID:   "u2",
			Roles:    []string{"r9"},
			HasRoles: true,
			JoinedAt: secondSeen,
		},
	}, time.Date(2022, 2, 10, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("UpsertGuildMemberSnapshotsContext(first) failed: %v", err)
	}

	err = store.UpsertGuildMemberSnapshotsContext(ctx, guildID, []GuildMemberSnapshot{
		{
			UserID:     "u1",
			AvatarHash: "avatar-new",
			HasAvatar:  true,
			Roles:      []string{"r3"},
			HasRoles:   true,
			JoinedAt:   secondSeen,
		},
		{
			UserID:   "u2",
			Roles:    nil,
			HasRoles: true,
			JoinedAt: firstSeen,
		},
	}, time.Date(2022, 2, 10, 11, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("UpsertGuildMemberSnapshotsContext(second) failed: %v", err)
	}

	avatarHash, _, ok, err := store.GetAvatar(guildID, "u1")
	if err != nil {
		t.Fatalf("GetAvatar() failed: %v", err)
	}
	if !ok || avatarHash != "avatar-new" {
		t.Fatalf("expected updated avatar hash, got hash=%q ok=%v", avatarHash, ok)
	}

	var historyCount int
	var oldHash, newHash string
	if err := store.db.QueryRow(
		`SELECT COUNT(*), COALESCE(MIN(old_hash), ''), COALESCE(MIN(new_hash), '') FROM avatars_history WHERE guild_id = $1 AND user_id = $2`,
		guildID,
		"u1",
	).Scan(&historyCount, &oldHash, &newHash); err != nil {
		t.Fatalf("query avatar history: %v", err)
	}
	if historyCount != 1 || oldHash != "avatar-old" || newHash != "avatar-new" {
		t.Fatalf("unexpected avatar history: count=%d old=%q new=%q", historyCount, oldHash, newHash)
	}

	roles, err := store.GetMemberRoles(guildID, "u1")
	if err != nil {
		t.Fatalf("GetMemberRoles(u1) failed: %v", err)
	}
	sort.Strings(roles)
	if len(roles) != 1 || roles[0] != "r3" {
		t.Fatalf("unexpected roles for u1: %v", roles)
	}

	roles, err = store.GetMemberRoles(guildID, "u2")
	if err != nil {
		t.Fatalf("GetMemberRoles(u2) failed: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected cleared roles for u2, got %v", roles)
	}

	joinedAt, ok, err := store.GetMemberJoin(guildID, "u1")
	if err != nil {
		t.Fatalf("GetMemberJoin(u1) failed: %v", err)
	}
	if !ok || !joinedAt.Equal(firstSeen) {
		t.Fatalf("expected earliest join for u1=%s, got %s (ok=%v)", firstSeen.Format(time.RFC3339), joinedAt.Format(time.RFC3339), ok)
	}

	joinedAt, ok, err = store.GetMemberJoin(guildID, "u2")
	if err != nil {
		t.Fatalf("GetMemberJoin(u2) failed: %v", err)
	}
	if !ok || !joinedAt.Equal(firstSeen) {
		t.Fatalf("expected earliest join for u2=%s, got %s (ok=%v)", firstSeen.Format(time.RFC3339), joinedAt.Format(time.RFC3339), ok)
	}
}

func TestUpsertGuildMemberSnapshotsContext_OptionalFieldsDoNotOverwriteExistingData(t *testing.T) {
	store := newTempStore(t)

	ctx := context.Background()
	guildID := "g1"
	joinedAt := time.Date(2023, 3, 5, 12, 0, 0, 0, time.UTC)

	if err := store.UpsertGuildMemberSnapshotsContext(ctx, guildID, []GuildMemberSnapshot{
		{
			UserID:     "u1",
			AvatarHash: "avatar-one",
			HasAvatar:  true,
			Roles:      []string{"r1"},
			HasRoles:   true,
			JoinedAt:   joinedAt,
		},
	}, time.Date(2023, 3, 5, 13, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("seed snapshots: %v", err)
	}

	if err := store.UpsertGuildMemberSnapshotsContext(ctx, guildID, []GuildMemberSnapshot{
		{
			UserID:   "u1",
			JoinedAt: joinedAt.Add(48 * time.Hour),
		},
	}, time.Date(2023, 3, 5, 14, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("join-only snapshot: %v", err)
	}

	avatarHash, _, ok, err := store.GetAvatar(guildID, "u1")
	if err != nil {
		t.Fatalf("GetAvatar() failed: %v", err)
	}
	if !ok || avatarHash != "avatar-one" {
		t.Fatalf("expected avatar hash to remain unchanged, got hash=%q ok=%v", avatarHash, ok)
	}

	roles, err := store.GetMemberRoles(guildID, "u1")
	if err != nil {
		t.Fatalf("GetMemberRoles() failed: %v", err)
	}
	sort.Strings(roles)
	if len(roles) != 1 || roles[0] != "r1" {
		t.Fatalf("expected roles to remain unchanged, got %v", roles)
	}

	gotJoin, ok, err := store.GetMemberJoin(guildID, "u1")
	if err != nil {
		t.Fatalf("GetMemberJoin() failed: %v", err)
	}
	if !ok || !gotJoin.Equal(joinedAt) {
		t.Fatalf("expected earliest join to remain %s, got %s (ok=%v)", joinedAt.Format(time.RFC3339), gotJoin.Format(time.RFC3339), ok)
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

	var lastVersion int
	if err := store.db.QueryRow(
		`SELECT last_version FROM message_version_counters WHERE guild_id='g' AND message_id='m'`,
	).Scan(&lastVersion); err != nil {
		t.Fatalf("query version counter: %v", err)
	}
	if lastVersion != writers {
		t.Fatalf("expected version counter to advance to %d, got %d", writers, lastVersion)
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
