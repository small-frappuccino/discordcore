//go:build integration

package storage

import (
	"context"
	"sync"
	"testing"
)

func TestNextTicketID_AtomicIncrements(t *testing.T) {
	store := newTempStore(t)
	ctx := context.Background()
	guildID := "guild-tickets-test"

	const concurrentWorkers = 20
	const iterationsPerWorker = 10

	var wg sync.WaitGroup
	// We'll collect all generated IDs in a slice, guarded by a mutex.
	// Since we expect (concurrentWorkers * iterationsPerWorker) total calls,
	// the set of returned IDs must be exactly {1, 2, ..., N}.

	results := make(map[int64]bool)
	var mu sync.Mutex
	var errs []error

	wg.Add(concurrentWorkers)
	for i := 0; i < concurrentWorkers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterationsPerWorker; j++ {
				id, err := store.NextTicketID(ctx, guildID)

				mu.Lock()
				if err != nil {
					errs = append(errs, err)
				} else {
					if results[id] {
						errs = append(errs, nil) // sentinel for duplicate
					}
					results[id] = true
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(errs) > 0 {
		t.Fatalf("encountered errors during concurrent NextTicketID calls: %v", errs)
	}

	totalExpected := int64(concurrentWorkers * iterationsPerWorker)
	if int64(len(results)) != totalExpected {
		t.Fatalf("expected exactly %d unique IDs, got %d", totalExpected, len(results))
	}

	for i := int64(1); i <= totalExpected; i++ {
		if !results[i] {
			t.Errorf("expected ID %d to be generated, but it was missing", i)
		}
	}
}
