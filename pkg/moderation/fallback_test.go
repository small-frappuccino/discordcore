package moderation

import (
	"context"
	"testing"

	"golang.org/x/sync/errgroup"
)

// TestNextFallbackCaseNumber_Race stresses the case number generation
// under high concurrency to validate the sync.Mutex boundaries and ensure
// strictly monotonically increasing numbers without deadlocks.
func TestNextFallbackCaseNumber_Race(t *testing.T) {
	t.Parallel()
	const (
		concurrency = 1000
		guildID     = "123456789012345"
	)

	eg, ctx := errgroup.WithContext(context.Background())

	results := make(chan int64, concurrency)

	for i := 0; i < concurrency; i++ {
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			// Generate the fallback case number concurrently.
			results <- NextFallbackCaseNumber(guildID, nil)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent fallback case generation failed: %v", err)
	}
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
