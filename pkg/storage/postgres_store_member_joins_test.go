package storage

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestCleanupObsoleteMemberJoins_DoesNotDeleteHistoricalJoins(t *testing.T) {
	t.Parallel()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
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
