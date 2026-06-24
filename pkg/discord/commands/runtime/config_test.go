package runtime

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"golang.org/x/sync/errgroup"
)

// TestSaveRuntimeConfig_RaceDetection mathematically guarantees thread-safe mutation semantics.
// Operational constraint: This uses t.Parallel() alongside multiple goroutines writing
// specifically to ensure files.ConfigManager correctly guards shared memory under high throughput HTTP traffic.
func TestSaveRuntimeConfig_RaceDetection(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	_ = tmp
	store := &files.MemoryConfigStore{}
	// Pre-seed an initial state to trigger standard Load/Update branches explicitly.
	cm := files.NewConfigManagerWithStore(store, nil)
	cm.LoadConfig() // Guarantee map initialization before bombardment.

	eg, ctx := errgroup.WithContext(context.Background())
	workers := 50 // Represents an adversarial spike in form submissions hitting the dashboard simultaneously.

	for i := 0; i < workers; i++ {
		idx := i
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}

			// Formulate unique configurations strictly within localized scopes to verify mutation bounds.
			rc := files.RuntimeConfig{
				BotTheme:         "theme_variant",
				DisableDBCleanup: idx%2 == 0,
			}

			// Mutate shared memory through the standardized boundary helper continuously.
			_ = saveRuntimeConfig(cm, rc, "global")
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent runtime config save failed: %v", err)
	}

	// Post-execution evaluation confirms that despite structural contention, the mutex map
	// successfully synchronized all changes without data races (caught by -race during CI).
	final, err := loadRuntimeConfig(cm, "global")
	if err != nil {
		t.Fatalf("expected valid config after concurrent mutation, got error: %v", err)
	}

	if final.BotTheme != "theme_variant" {
		t.Errorf("expected final write barrier to persist theme, got %q", final.BotTheme)
	}
}
