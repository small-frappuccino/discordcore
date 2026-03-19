package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestCleanupObsoleteMemberJoins_DoesNotDeleteHistoricalJoins(t *testing.T) {
	t.Parallel()

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
	defer func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	}()

	s := NewStore(db)
	if err := s.Init(); err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	defer func() { _ = s.Close() }()

	guildID := "g1"
	userID := "u1"
	veryOld := time.Date(2016, 1, 1, 0, 0, 0, 0, time.UTC)

	if err := s.UpsertMemberJoin(guildID, userID, veryOld); err != nil {
		t.Fatalf("UpsertMemberJoin() failed: %v", err)
	}

	if err := s.CleanupAllObsoleteData(); err != nil {
		t.Fatalf("CleanupAllObsoleteData() failed: %v", err)
	}

	got, ok, err := s.GetMemberJoin(guildID, userID)
	if err != nil {
		t.Fatalf("GetMemberJoin() failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected join to remain after cleanup")
	}
	if !got.Equal(veryOld) {
		t.Fatalf("expected join=%s, got %s", veryOld.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestTouchMemberJoin_PreservesHistoricalJoin(t *testing.T) {
	t.Parallel()

	store := newTempStore(t)
	guildID := "g1"
	userID := "u1"
	historicalJoin := time.Date(2021, 2, 10, 9, 0, 0, 0, time.UTC)

	if err := store.UpsertMemberJoin(guildID, userID, historicalJoin); err != nil {
		t.Fatalf("UpsertMemberJoin() failed: %v", err)
	}

	if err := store.TouchMemberJoin(guildID, userID); err != nil {
		t.Fatalf("TouchMemberJoin() failed: %v", err)
	}

	got, ok, err := store.GetMemberJoin(guildID, userID)
	if err != nil {
		t.Fatalf("GetMemberJoin() failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected join to remain after touch")
	}
	if !got.Equal(historicalJoin) {
		t.Fatalf("expected join=%s, got %s", historicalJoin.Format(time.RFC3339), got.Format(time.RFC3339))
	}

	lastSeen, ok, err := readMemberLastSeen(store, guildID, userID)
	if err != nil {
		t.Fatalf("readMemberLastSeen() failed: %v", err)
	}
	if !ok {
		t.Fatal("expected last_seen_at after touch")
	}
	if !lastSeen.After(historicalJoin) {
		t.Fatalf("expected last_seen_at newer than joined_at, got join=%s last_seen=%s", historicalJoin.Format(time.RFC3339), lastSeen.Format(time.RFC3339))
	}
}

func TestTouchMemberJoin_UpdatesLastSeenWithoutCreatingMissingJoin(t *testing.T) {
	t.Parallel()

	store := newTempStore(t)
	guildID := "g1"
	userID := "u1"
	historicalJoin := time.Date(2021, 2, 10, 9, 0, 0, 0, time.UTC)
	staleSeen := time.Date(2021, 2, 11, 9, 0, 0, 0, time.UTC)

	if err := store.UpsertMemberJoin(guildID, userID, historicalJoin); err != nil {
		t.Fatalf("UpsertMemberJoin() failed: %v", err)
	}
	if _, err := store.db.Exec(
		rebind(`UPDATE member_joins SET last_seen_at=? WHERE guild_id=? AND user_id=?`),
		staleSeen,
		guildID,
		userID,
	); err != nil {
		t.Fatalf("seed stale last_seen_at: %v", err)
	}

	if err := store.TouchMemberJoin(guildID, userID); err != nil {
		t.Fatalf("TouchMemberJoin() failed: %v", err)
	}

	gotJoin, ok, err := store.GetMemberJoin(guildID, userID)
	if err != nil {
		t.Fatalf("GetMemberJoin() failed: %v", err)
	}
	if !ok || !gotJoin.Equal(historicalJoin) {
		t.Fatalf("expected historical join=%s, got %s (ok=%v)", historicalJoin.Format(time.RFC3339), gotJoin.Format(time.RFC3339), ok)
	}

	lastSeen, ok, err := readMemberLastSeen(store, guildID, userID)
	if err != nil {
		t.Fatalf("readMemberLastSeen() failed: %v", err)
	}
	if !ok {
		t.Fatal("expected last_seen_at after touch")
	}
	if !lastSeen.After(staleSeen) {
		t.Fatalf("expected last_seen_at to advance past %s, got %s", staleSeen.Format(time.RFC3339), lastSeen.Format(time.RFC3339))
	}

	if err := store.TouchMemberJoin(guildID, "missing"); err != nil {
		t.Fatalf("TouchMemberJoin(missing) failed: %v", err)
	}
	if _, ok, err := store.GetMemberJoin(guildID, "missing"); err != nil {
		t.Fatalf("GetMemberJoin(missing) failed: %v", err)
	} else if ok {
		t.Fatal("expected missing member touch not to create a join row")
	}
}

func TestUpsertMemberPresenceAndMarkMemberLeftTracksActiveState(t *testing.T) {
	t.Parallel()

	store := newTempStore(t)
	guildID := "g1"
	userID := "u1"
	joinedAt := time.Date(2022, 7, 1, 8, 0, 0, 0, time.UTC)
	seenAt := joinedAt.Add(30 * time.Minute)

	if err := store.UpsertMemberPresenceContext(context.Background(), guildID, userID, joinedAt, seenAt, true); err != nil {
		t.Fatalf("UpsertMemberPresenceContext() failed: %v", err)
	}
	if err := store.UpsertMemberRoles(guildID, userID, []string{"r1", "r2"}, seenAt); err != nil {
		t.Fatalf("UpsertMemberRoles() failed: %v", err)
	}

	states, err := store.GetActiveGuildMemberStatesContext(context.Background(), guildID)
	if err != nil {
		t.Fatalf("GetActiveGuildMemberStatesContext() failed: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one active member state, got %d", len(states))
	}
	if !states[0].HasBot || !states[0].IsBot {
		t.Fatalf("expected bot flag to round-trip, got hasBot=%v isBot=%v", states[0].HasBot, states[0].IsBot)
	}
	if len(states[0].Roles) != 2 {
		t.Fatalf("expected active roles to round-trip, got %v", states[0].Roles)
	}

	leftAt := seenAt.Add(15 * time.Minute)
	if err := store.MarkMemberLeftContext(context.Background(), guildID, userID, leftAt); err != nil {
		t.Fatalf("MarkMemberLeftContext() failed: %v", err)
	}

	states, err = store.GetActiveGuildMemberStatesContext(context.Background(), guildID)
	if err != nil {
		t.Fatalf("GetActiveGuildMemberStatesContext(after leave) failed: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected no active members after leave, got %d", len(states))
	}

	roles, err := store.GetMemberRoles(guildID, userID)
	if err != nil {
		t.Fatalf("GetMemberRoles(after leave) failed: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected roles to be cleared on leave, got %v", roles)
	}

	row := store.db.QueryRow(
		rebind(`SELECT left_at FROM member_joins WHERE guild_id=? AND user_id=?`),
		guildID,
		userID,
	)
	var gotLeftAt sql.NullTime
	if err := row.Scan(&gotLeftAt); err != nil {
		t.Fatalf("query left_at: %v", err)
	}
	if !gotLeftAt.Valid || !gotLeftAt.Time.Equal(leftAt) {
		t.Fatalf("expected left_at=%s, got %v", leftAt.Format(time.RFC3339), gotLeftAt)
	}
}

func TestUpsertGuildMemberSnapshotsContextPersistsBotFlagsAndReactivatesMembers(t *testing.T) {
	t.Parallel()

	store := newTempStore(t)
	guildID := "g1"
	userID := "u1"
	joinedAt := time.Date(2022, 9, 5, 9, 30, 0, 0, time.UTC)

	if err := store.UpsertMemberPresenceContext(context.Background(), guildID, userID, joinedAt, joinedAt.Add(30*time.Minute), false); err != nil {
		t.Fatalf("UpsertMemberPresenceContext(seed) failed: %v", err)
	}
	if err := store.MarkMemberLeftContext(context.Background(), guildID, userID, joinedAt.Add(time.Hour)); err != nil {
		t.Fatalf("MarkMemberLeftContext() failed: %v", err)
	}

	if err := store.UpsertGuildMemberSnapshotsContext(context.Background(), guildID, []GuildMemberSnapshot{
		{
			UserID:   userID,
			Roles:    []string{"r9"},
			HasRoles: true,
			JoinedAt: joinedAt,
			IsBot:    false,
			HasBot:   true,
		},
	}, joinedAt.Add(2*time.Hour)); err != nil {
		t.Fatalf("UpsertGuildMemberSnapshotsContext() failed: %v", err)
	}

	states, err := store.GetActiveGuildMemberStatesContext(context.Background(), guildID)
	if err != nil {
		t.Fatalf("GetActiveGuildMemberStatesContext() failed: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected one active state after snapshot upsert, got %d", len(states))
	}
	if !states[0].HasBot || states[0].IsBot {
		t.Fatalf("expected human bot flag after snapshot upsert, got hasBot=%v isBot=%v", states[0].HasBot, states[0].IsBot)
	}
	if len(states[0].Roles) != 1 || states[0].Roles[0] != "r9" {
		t.Fatalf("unexpected roles after snapshot upsert: %v", states[0].Roles)
	}
}

func readMemberLastSeen(store *Store, guildID, userID string) (time.Time, bool, error) {
	row := store.db.QueryRow(
		rebind(`SELECT last_seen_at FROM member_joins WHERE guild_id=? AND user_id=?`),
		guildID,
		userID,
	)
	var lastSeen sql.NullTime
	if err := row.Scan(&lastSeen); err != nil {
		if err == sql.ErrNoRows {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	if !lastSeen.Valid {
		return time.Time{}, false, nil
	}
	return lastSeen.Time, true, nil
}
