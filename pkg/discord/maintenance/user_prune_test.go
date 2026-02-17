package maintenance

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func newPurgeTestStore(t *testing.T) *storage.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "purge_test.sqlite")
	s := storage.NewStore(dbPath)
	if err := s.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestIsPruneExecutionDay(t *testing.T) {
	t.Parallel()

	if !isPruneExecutionDay(time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)) {
		t.Fatal("expected day 28 to be execution day")
	}
	if isPruneExecutionDay(time.Date(2026, 2, 27, 23, 59, 0, 0, time.UTC)) {
		t.Fatal("did not expect day 27 to be execution day")
	}
}

func TestDidRunGuildPruneThisMonth(t *testing.T) {
	store := newPurgeTestStore(t)
	svc := &UserPruneService{store: store}

	guildID := "g1"
	now := time.Date(2026, 2, 28, 12, 0, 0, 0, time.UTC)

	if svc.didRunGuildPruneThisMonth(guildID, now) {
		t.Fatal("expected false before marking run")
	}

	if err := svc.markGuildPruneRun(guildID, now); err != nil {
		t.Fatalf("mark run: %v", err)
	}

	if !svc.didRunGuildPruneThisMonth(guildID, now.Add(2*time.Hour)) {
		t.Fatal("expected true for same month after marking run")
	}

	if svc.didRunGuildPruneThisMonth(guildID, now.AddDate(0, 1, 0)) {
		t.Fatal("expected false for next month")
	}
}

func TestMarkGuildPruneRunRequiresGuildID(t *testing.T) {
	store := newPurgeTestStore(t)
	svc := &UserPruneService{store: store}

	if err := svc.markGuildPruneRun("   ", time.Now().UTC()); err == nil {
		t.Fatal("expected error for empty guild id")
	}
}
