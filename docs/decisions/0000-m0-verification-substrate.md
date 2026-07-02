# 0000 — M0 verification substrate: gates and scoping decisions

Status: accepted (M0). Directive-Gate log (§2): each threshold below carries
its derivation or its deferral point.

## Allocation oracle accounting model

`testkit::CountingAlloc` counts **thread-locally** (`thread_local!` +
`const { Cell::new(0) }`). Derivation: the falsifier attributes allocation
events to the measuring thread only, so concurrent test threads and criterion
bookkeeping threads cannot pollute a measurement; the `const` initializer
guarantees the accounting path itself performs no allocation (no lazy init,
no TLS destructor registration). `alloc`/`alloc_zeroed`/`realloc` = 1 event
each; `dealloc` = 0 (a release is not an escape).

## Oracle confinement

`testkit` enters consumers exclusively through `[dev-dependencies]`; inside
libraries the allocator is installed in a `#[cfg(test)]` module. Bench
targets (`harness = false`) build without `cfg(test)`, so the bench binary
installs it at top level — equally outside any production artifact. The
production binary never links the counting allocator.

## Alloc gate runs in release mode

`cargo xtask alloc-check` runs `--release`. Optimization can only *remove*
allocations relative to debug, and §7's regime measures the optimized artifact
that ships; a release-mode count > 0 is therefore always a genuine regression,
never opt-level noise.

## soundness.yml topology

Miri and ASan run per-push/PR. MSan runs on weekly schedule +
`workflow_dispatch` only: `-Zbuild-std` rebuilds std per run (§6 requires it —
uninstrumented std false-positives on every read), which is M6's stated
scheduled-job regime, adopted from day one. ASan and MSan remain separate jobs
(mutually incompatible instrumentation).

Nightly pin: `nightly-2026-06-28` (miri component verified available for
`x86_64-unknown-linux-gnu` via rustup-components-history on 2026-07-02).
Bumps are deliberate edits to `RUST_NIGHTLY` in `soundness.yml`, decoupled
from the stable pin in `rust-toolchain.toml`.

## perf.yml scope at M0

- `alloc-gate` is the load-bearing job: a counted allocation in a monitored
  scope fails the build (§7 CI threshold, threshold = 0 exactly).
- `bench` runs criterion and uploads the report artifact. Cross-run threshold
  comparison needs a persisted baseline (branch-keyed cache or gh-pages
  history); deferred to the milestone that adds the first RX hot path (M2),
  where a regression SLA becomes derivable. Until then the bench falsifies
  allocs==0 inline and records timings.
- `llvm-lines` is report-only: the §4 monomorphize-vs-`dyn` decisions it
  adjudicates arrive with `ingest` (M2). The artifact makes code-mass diffs
  reviewable from day one.

## Toolchain floor

`rust-version = "1.95"` (workspace): floor imposed by `core::hint::cold_path`
per the manifesto header; `rust-toolchain.toml` pins stable 1.96.0. Twilight's
1.89 MSRV is a dependency floor and does not bind this workspace.
