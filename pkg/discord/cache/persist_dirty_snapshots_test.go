package cache

import (
	"context"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage/storagetest"
)

func TestPersistDirtySnapshots_RollbackOnFailure(t *testing.T) {
	// Create a unified cache with a failing store to simulate I/O timeout/failure.
	store := storagetest.NewFailingStore()
	cfg := DefaultCacheConfig()
	cfg.Store = store
	cfg.PersistEnabled = true
	uc := NewUnifiedCache(cfg)

	// Inject a member into the cache so it's marked dirty.
	member := &discordgo.Member{
		User:    &discordgo.User{ID: "user-1"},
		GuildID: "guild-1",
	}
	uc.SetMember("guild-1", "user-1", member)

	// Verify the member is dirty.
	snapshots := uc.members.TakeDirtySnapshot(time.Now().Add(1 * time.Second))
	if len(snapshots) == 0 {
		t.Fatal("expected member to be dirty after UpdateMember")
	}

	// Re-dirty the member for the actual test.
	uc.members.MarkDirty([]string{"guild-1:user-1"})

	// Attempt to persist dirty snapshots. This should fail because of the failing store.
	err := uc.persistDirtyMembers(context.Background(), time.Now().Add(1*time.Second))
	if err == nil {
		t.Fatal("expected error from persistDirtyMembers with failing store, got nil")
	}

	// Verify that the member is still dirty (MarkDirty was re-invoked in the rollback path).
	snapshots = uc.members.TakeDirtySnapshot(time.Now().Add(1 * time.Second))
	if len(snapshots) == 0 {
		t.Fatal("expected member to remain dirty after failed persist due to atomic rollback re-invoking MarkDirty")
	}
	if snapshots[0].Key != "guild-1:user-1" {
		t.Fatalf("expected dirty key guild-1:user-1, got %s", snapshots[0].Key)
	}
}
