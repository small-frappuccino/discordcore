package app

import (
	"context"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// FeatureService defines the passive watcher triggered by state changes.
// Allocation Footprint: Minimal overhead. Goroutine spawned for Start.
// Preemption Rules: Must respect context cancellation boundaries strictly.
type FeatureService interface {
	// Start traps execution inside its spawned goroutine and yields control exclusively to errgroup.
	// It must gracefully unwind its stack the exact clock cycle ctx.Done() returns.
	// Allocation Footprint: Small stack allocation for goroutine. No sustained heap growth.
	// Preemption Rules: Blocking call. Yields completely upon context cancellation.
	Start(ctx context.Context) error

	// Stop signals the feature service to shutdown and release any held resources gracefully.
	// Allocation Footprint: Minimal. No heap allocations expected.
	// Preemption Rules: Must return immediately or block only briefly for teardown.
	Stop() error

	// WatchConfig is the passive Pub/Sub reactive receiver for configuration events.
	// It receives a ConfigEvent containing the GuildID and the read-only configuration snapshot.
	// The implementer must compute feature toggles (Enable/Disable) deterministically in O(1) or O(N) local memory without acquiring global write locks.
	// Allocation Footprint: O(1) or local O(N) for evaluation. Must not escape variables to the heap unnecessarily.
	// Preemption Rules: Must execute synchronously and quickly. Should preempt if context is canceled during complex evaluation.
	WatchConfig(ctx context.Context, event files.ConfigEvent)
}
