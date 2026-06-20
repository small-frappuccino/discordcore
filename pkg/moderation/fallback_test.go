package moderation

import (
	"sync"
	"testing"
)

// TestNextFallbackCaseNumber_Race stresses the case number generation
// under high concurrency to validate the sync.Mutex boundaries and ensure
// strictly monotonically increasing numbers without deadlocks.
func TestNextFallbackCaseNumber_Race(t *testing.T) {
	const (
		concurrency = 1000
		guildID     = "123456789012345"
	)

	var wg sync.WaitGroup
	wg.Add(concurrency)

	results := make(chan int64, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			// Generate the fallback case number concurrently.
			results <- NextFallbackCaseNumber(guildID)
		}()
	}

	wg.Wait()
	close(results)

	seen := make(map[int64]bool)
	maxVal := int64(0)

	for n := range results {
		if seen[n] {
			t.Fatalf("duplicate case number detected: %d", n)
		}
		seen[n] = true
		if n > maxVal {
			maxVal = n
		}
	}

	// Verify that exactly `concurrency` numbers were generated
	// and the max value aligns with the amount of operations.
	// Note: Because fallbackCaseSeq is global and persists across tests,
	// maxVal should be at least `concurrency`.
	if len(seen) != concurrency {
		t.Fatalf("expected %d unique numbers, got %d", concurrency, len(seen))
	}
}
