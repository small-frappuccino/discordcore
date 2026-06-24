//go:build integration

package tickets

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestNextID_ACID(t *testing.T) {
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
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store, err := postgres.NewStore(db, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := NewManager(store, logger)
	guildID := "concurrent-test-guild"

	const workers = 20
	const iterations = 50

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[int64]bool)

	errCh := make(chan error, workers*iterations)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				id, err := mgr.NextID(ctx, guildID)
				cancel()
				if err != nil {
					errCh <- err
					continue
				}
				mu.Lock()
				if results[id] {
					errCh <- fmt.Errorf("duplicate ticket ID detected: %d", id)
				}
				results[id] = true
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}

	if len(results) != workers*iterations {
		t.Errorf("expected %d unique IDs, got %d", workers*iterations, len(results))
	}
}
