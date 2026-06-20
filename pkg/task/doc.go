//go:build !legacy
// +build !legacy

/*
Package task orchestrates background execution scheduling, topological routing,
and deterministic execution guarantees across the application ecosystem.

# Contract

The task orchestration boundary guarantees that identical GroupKey instances will
never be processed in parallel across active goroutines. Handlers are executed
sequentially, mitigating data-races natively without requiring consumer-side
distributed locks.

# Retry Semantics

Handlers returning errors are subjected to an exponential backoff formula combined
with an underlying container/heap priority queue. Context cancellation from the Close()
lifecycle propagates synchronously into executing tasks to immediately abort network I/O.

# Invariants

- Tasks with duplicate IdempotencyKey values within the TTL window are rejected synchronously with ErrDuplicateTask.
- Panic states within worker bounds are isolated, recovered, and logged without tearing down the routing engine.
- Handlers MUST NOT spawn detached background routines. All logic must obey the passed context.Context.
*/
package task
