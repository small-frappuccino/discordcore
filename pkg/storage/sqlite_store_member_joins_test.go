package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCleanupObsoleteMemberJoins_DoesNotDeleteHistoricalJoins(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	s := NewStore(dbPath)
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
