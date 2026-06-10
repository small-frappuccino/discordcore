package storagetest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestFailingStore(t *testing.T) {
	store := NewFailingStore()
	err := store.UpsertCacheEntriesContext(context.Background(), []storage.CacheEntryRecord{
		{
			Key: "test", CacheType: "test", Data: "test", ExpiresAt: time.Now(),
		},
	})
	fmt.Printf("err = %v\n", err)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
