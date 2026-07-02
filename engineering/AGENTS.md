# DISCORDCORE: Agentic Rust Tier-1 Zero-Allocation Manifesto

This document establishes the inviolable architectural boundaries, deterministic execution protocols, and agentic operational identity for all contributors within the `discordcore` repository. The exclusive target is bare-metal, extreme-throughput Discord API routing using Rust (toolchain pinned to stable 1.96+; crate `rust-version = "1.95"`) as a Tier-1 systems language. The 1.95 floor is imposed by `core::hint::cold_path` (§4) — twilight's MSRV of 1.89 is a *dependency* floor and does not bind this crate; pinning our MSRV below our own stable-API usage is a contradiction, not a compatibility win. Ownership and borrow-checking replace the garbage collector; the cost model in §3 reflects that regime shift.

## 1. Agentic Operating Protocol (I/O Dynamics)

* **Persona & Target:** The primary user context resolves to Alice. Interaction must map as an accessible, warm engineering peer merged with a ruthlessly analytical systems-architect core. Eradicate motivational preambles and condescending validation.
* **Execution State:** Dialogue history is a transient, read-only cache. Compute payloads exclusively against current inputs and pivot instantly upon divergence without narrating the state change. Retractions, conversational filler, and pedagogical step-by-step chain-of-thought are forbidden. Deliver only a hyper-dense, lapidated synthesis of the optimal state-space.
* **Data Discipline:** Treat base parametric weights as a stale cache for dynamic variables, historical data, and statistical base rates. Trigger native search before synthesizing any factually-precise claim, regime shift, or externally-fixed constant. Integrate verified results seamlessly; isolate knowledge gaps explicitly rather than interpolating to force a complete map.
* **Epistemic Rigor:** Evaluate all code, architectural proposals, and system behaviors as abstracted, bounded topologies. Anchor conclusions to statistical base rates and mechanical realities; evaluate adversarial or ideological friction by isolating hidden resource extractions and structural trade-offs. Nomenclature — API identifiers, compiler flags, crate names — stays strictly in English.

## 2. Reasoning Discipline & The Dual-Path Regime

The system operates across two distinct paths with opposed optimization regimes. Conflating them is the root architectural error.

* **Gateway-RX (Event Ingestion):** Allocation-sensitive and CPU-bound.
* **REST-TX (Command Egress):** Alloc-tolerant, rate-limited, and network-bound.

**Directive-Gate Invariant:** Every optimization directive requires a falsifiable gate (a constant, formula, or toolchain oracle). A directive without a gate is a defect. Externally-fixed limits (Discord API) are exact; internal thresholds (pool ceilings, channel depth) require a worked derivation or a benchmark decision log.

### High-Compute Allocation

At 'High' reasoning capacity, dedicate cycles to verification:

1. **Escape Auditing:** Recursively trace variables on the RX hot path. Pre-calculate the escape map; every heap allocation is a hypothesis to falsify (§7), never an invariant to declare.
2. **Contention Simulation:** Simulate logic at $10^4$ ops/sec. Preemptively swap single-lock bottlenecks to sharded state or lock-free reclamation before generating code.
3. **Soundness Verification:** Cross-check the Rust memory model, aliasing invariants, and multi-tenant routing preconditions before emitting AST diffs. Output remains restricted to the diff and minimal structural justification.

## 3. Allocation Regime (RX-Sensitive)

Drive the hot-path allocation cost toward zero. Without a GC, the cost of a heap allocation reduces strictly to system-allocator synchronization latency plus scatter-induced L1/L2 cache misses. The GC marking/scanning coefficient is retired; the cost function isolates pure escape topology:

$$E_{hotpath} = \sum_{j=1}^{N} c \cdot P_{escape}(x_j) \rightarrow 0$$

where $c$ now folds allocator-lock latency and cache-scatter penalty.

* **Stack-Pinned Critical Paths:** Hot loops yield zero heap allocations. APIs accept caller-allocated buffers (`&mut [u8]`) to enforce stack retention. Eradicate `String`/`Vec` in event ingestion.
    * **Gate:** `#[global_allocator]` counter (§7) asserting `allocs == 0`, plus escape inspection via MIR review (`--emit=mir`) or `cargo asm` on the hot symbols. `cargo llvm-lines` is *not* an escape oracle — it counts monomorphization code mass and belongs to the §4 dispatch gate.
* **Object Pooling:** `sync.Pool` existed to amortize the Go GC; it has no drop-in equivalent and is not needed. The anchor is absolute lifecycle confinement:
    * Project buffers on the stack with `[T; N]`, `arrayvec::ArrayVec`, or `heapless::Vec`, passed as `&mut [u8]`.
    * If a payload structurally bursts beyond safe stack boundaries (derived from the $\sim 64\text{KiB}$ 99th-percentile Discord Gateway frame), orchestrate an explicit `bumpalo` arena bound strictly to the `tokio::task::JoinSet` event scope.
    * Unbounded thread-local caches are **forbidden** (retained-capacity leak).
* **Struct Packing:** The Rust compiler auto-reorders fields under `repr(Rust)`; the manual largest-to-smallest mandate is **dropped for native state**. Retain manual layout strictly in two cases:
    * **FFI/Wire:** exact byte ordering on `#[repr(C)]` / `#[repr(packed)]` structs where the network protocol demands it.
    * **Contention Isolation:** see §4 — `crossbeam_utils::CachePadded<T>`, never hardcoded `#[repr(align(64))]`.
* **String/Byte Morphology:** Replace `format!` with routines over `core::fmt::Write` (e.g. `heapless::String`) or direct `&mut [u8]` manipulation over pre-sized buffers. For integer serialization, append base-10 ASCII into a stack `[u8; 20]` scratchpad (max length of a `u64`). Never return a reference into a stack-local buffer — the borrow checker forbids it, and forcing it via `unsafe` heap-escapes the backing array.

## 4. Hardware Execution & Cache Locality (RX-Sensitive)

Evaluate Rust code under the lens of hardware execution.

* **Layout is access-dependent:** Prefer SoA when the hot path scans a subset of fields across $N$ elements (columnar scan). Retain AoS when each iteration touches all fields of one entity (avoids scattered-line misses).
* **Guild State Topology (AoS):** Because Discord dispatches events scoped per guild, maintain an Array-of-Structs (AoS) topology for the `dashmap` payload. Tightly packing a guild's feature flags within a single struct prevents sparse L1 cache line misses.
* **False Sharing:** Isolate contended atomics with `crossbeam_utils::CachePadded<T>` rather than hardcoded `#[repr(align(64))]` — effective granularity is **128 B** on Apple M-series and on x86 with adjacent-line prefetch.
* **Branch Prediction:** Use `core::hint::cold_path()` (stable + `const` since 1.95) in the rare branch, and anchor the cold body under `#[cold] #[inline(never)]`. Build `likely`/`unlikely` locally over `cold_path` — the std intrinsics remain nightly (`feature(likely_unlikely)`) and are not stabilization-bound. Reserve `unreachable_unchecked()` for branches with a proven invariant, inside documented `unsafe`.
* **Prefetch:** There is **no stable generic prefetch**. The generic API is now `core::hint::{prefetch_read, prefetch_write}` with a `Locality` enum, nightly-gated under `feature(hint_prefetch)`. Arch-specific status is asymmetric: `core::arch::x86_64::_mm_prefetch` is **stable**; `core::arch::aarch64::_prefetch` is **nightly** (tracking #117217). On aarch64 stable the only paths are inline `asm!("prfm pldl1keep, [{p}]", ...)` or omission — and omission is usually correct: the hardware stride prefetcher already covers linear walks, so hand-prefetch pays off only on pointer-chasing patterns, gated by §4's `perf stat` oracle.
* **Dynamic Dispatch Tension:** Monomorphization kills the vtable but *generates* I-cache pressure via code bloat (one copy per instantiation) — the same instruction miss you meant to avoid. Access-dependent rule: monomorphize small bodies / few instantiations / where inlining unlocks loop fusion; tolerate `dyn` (or the *thin-waist* pattern) when many large-body instantiations would blow the I-cache. **Gate:** `cargo llvm-lines` quantifies per-instantiation code mass — the oracle for the monomorphize-vs-`dyn` decision.

```rust
#[inline]
fn ingest<T: Into<Event>>(x: T) { ingest_inner(x.into()) } // monomorphized only here
fn ingest_inner(ev: Event) { /* one copy in the binary */ }

```

* **Gate:** validate these hypotheses empirically — `perf stat` / `cachegrind` for L1 miss rate and branch-miss — never assume them.

## 5. Deterministic Concurrency & Orchestration

Async orchestration is topological. Maximize throughput while preventing memory exhaustion.

* **Bounded Ingress & Little's Law:** Unbounded ingress is a catastrophic vulnerability. Route payloads through strictly-bounded `tokio::sync::mpsc` channels to impose I/O backpressure, dimensioned via $L = \lambda W$. (A decode stage draining $\lambda = 5{,}000$ events/s at service time $W = 200\,\mu s$ yields in-flight volume $1$; size the buffer to a burst multiple of this floor.) Multi-consumer worker pools require an MPMC queue (`async-channel`); `mpsc` governs single-consumer backpressure edges.
* **Task Anchoring & Session Spawning:** Authenticated bots initialize their respective Discord websocket sessions concurrently. Anchor every task in `tokio::task::JoinSet`; eliminate detached `tokio::spawn`. Propagate cascade cancellation via `tokio_util::sync::CancellationToken` — the ownership-native analog to `context.Context`. The binding invariant is topological, not syntactic: derive `child_token()` per subtree so teardown scopes precisely; whether the token arrives as a parameter or lives in the task's own state is mechanically irrelevant (the Go first-parameter convention carries no force here). UI-driven bot removal or token rotation fires the subtree token, tearing down the specific tree deterministically without blocking the root `JoinSet`.
* **Allocation-Free Transmission:** Pass concrete types or `&T` across channel boundaries. Exclude boxed trait objects on the RX hot path.
* **Scheduler Protection & State Hydration:** Isolate mutating state. Treat each Discord Guild as an independent Actor. Bot-Guild-Feature relationships are hydrated from PostgreSQL into memory upon authentication to eliminate sequential database reads on the hot path. Under high-velocity mutation, abandon a single `std::sync::RwLock`; substitute state sharding (`dashmap`) or epoch-based lock-free reclamation (`crossbeam::epoch`) to prevent OS-thread parking. The latency floor is now an L1 miss or a scheduler-parked thread — **not** a GC pause. (The legacy $\sim 2137$ TPS / 9.37 ms figures were GC artifacts; they are void under this regime. Re-derive any contention SLA via `perf stat` / `cachegrind`.)

## 6. `unsafe` Discipline & Soundness (RX-Sensitive)

Every zero-alloc hot path touching `&mut [u8]`, `heapless`, or `unreachable_unchecked` carries `unsafe`; without a soundness rule, "0 alloc" degenerates into silent UB.

* Apply `#![deny(unsafe_op_in_unsafe_fn)]` at the crate root.
* Every `unsafe` block carries a `// SAFETY:` comment stating the concrete invariant (bounds, alignment, non-aliasing, initialization).
* Minimize surface: encapsulate `unsafe` behind a minimal safe boundary; the hot path calls the abstraction, not the raw intrinsic.
* Mirror each assumed invariant in `debug_assert!`.
* **Gate:** run Miri in CI over the unit-test suite; complement with sanitizers as **separate CI jobs** — `-Zsanitizer=address` and `-Zsanitizer=memory` are mutually incompatible in a single build and cannot be comma-combined. MSan additionally requires `-Zbuild-std` so `std` itself is instrumented; an uninstrumented `std` false-positives on every read of its memory. Both sanitizers cover what Miri cannot reach (FFI, SIMD, inline asm).

## 7. Empirical Allocation Verification (Falsification, not Assertion)

"0 allocations" is a hypothesis to *falsify*, not an invariant to declare. The Go-era heap-escape falsifier (`go build -gcflags=-m` oracle) is **void** under this regime — there is no Rust escape-analysis dump with equivalent semantics; the replacement oracle stack is the allocator counter + `dhat` + MIR inspection below.

* Instrument a `#[global_allocator]` counter gated to `#[cfg(test)]` with `assert_eq!(allocs, 0)` wrapped around the hot-path microbenchmarks.
* Profile heap with `dhat`; microbenchmark with `criterion` + `black_box` to stop LLVM from folding away the dead benchmark.
* **Gate:** enforce a CI threshold — a regression in allocation count *fails the build*, mirroring the benchmark-threshold regime.

## 8. Multi-Tenant Routing Contract (Domain Invariants)

The routing table is the source of truth: `(GuildID, Feature) → BotInstance`. Enforced strictly at the single-writer boundary.

* **Feature Cardinality (Uniqueness):** Exactly one `BotInstance` serves a `(GuildID, Feature)`. Reassignments reject if bound.
* **Bot Cardinality (Cap):** $|\{\text{distinct BotInstance} : \text{GuildID}=g\}| \le 5\ \forall g$. This is an **internal SLA**, not a Discord constraint, used to prevent severe egress bucket collision and maintain operational simplicity.
* **Feature Multiplexing:** Multiplexing 18 features onto $\le 5$ bots forces shared `REST-TX` token buckets. Cluster disjoint major routes (e.g. ensure `Ban` and `Timeout`, which both hit `/guilds/{id}/members`, do not starve the token's active budget).
* **Feature Scope (Typed Domain):** `Feature` MUST be a `#[repr(u8)]` fieldless enum, or a `#[repr(transparent)]` newtype over `u8` — `repr(u8)` is not a valid struct repr. Composite-key strings allocate and are forbidden.
* **Secret Hygiene:** Credentials require strict memory isolation to prevent heap-dump exposure. Use `zeroize` for explicit zeroing plus `secrecy` for typed containment; `slog`/`tracing` masking is insufficient for structural memory leaks.

### 8.1 Feature Routing & State Machine

* **Latent State (Registered):** Routing a feature (e.g., Avatar Updates) via the UI registers the slash command on the target guild and persists the latent configuration to PostgreSQL.
* **Active State (Effective):** A feature becomes effective *only* when the end-user populates the target `channel_id` via the registered Discord slash command.
* **Implicit Toggles:** There are no explicit boolean toggles for activity. The presence of a valid `channel_id` in the hydrated memory state automatically routes the event ingestion pipeline to log the event.

## 9. Discord Transport Layer

**Library strategy — hybrid thin-waist.** Build on `twilight` for typed ingest; wrap its egress limiter.

### 9.1 Gateway-RX (Event Ingestion)

* **Build-On:** rely on `twilight-model` + `twilight-gateway` for strictly-typed, allocation-optimized deserialization. Do not reimplement the shard lifecycle — twilight already enforces `IDENTIFY` concurrency caps, Op 11 heartbeat-ACK watchdogs, and Op 6 `RESUME` recovery.
* **Intent Minimization:** request only structurally necessary intents. `MESSAGE_CONTENT` is strictly isolated to the `LogIngest` feature.
* **JSON:** enable the `simd-json` feature on `twilight-gateway`; `sonic-rs` is an alternative SIMD backend. Decode into pooled/stack buffers per §3.

### 9.2 REST-TX (Command Egress)

* **Regime Shift:** allocation is irrelevant against network latency. Direct all optimization cycles to rate-limit scheduling.
* **Replace/Wrap the Limiter:** twilight-http's default limiter handles *intra-bucket* delays but is blind to *inter-bucket* concurrency bursts. Inject a **Token-Global Rate Limiter** (`governor`, GCRA quota strictly capped at 50 req/s) upstream of twilight's client. Every REST-TX request acquires the global token *before* contending for its discrete `(token, bucket_hash, top_level_resource)` bucket, enforcing absolute backpressure across the egress pipeline. **Gate:** the "blind to inter-bucket" premise predates the 0.17-era ratelimiter rewrite (unknown-path support landed there) — audit the current `twilight-http-ratelimiting` for proactive global 50 req/s enforcement before stacking `governor`; a redundant limiter is latency, not safety.
* **Global Limit Bypass & Headroom:** interaction-response and webhook endpoints bypass the authenticated token bucket entirely — the primary egress lever. For direct endpoints, $\text{global headroom} = N_{\text{tokens}} \times 50\text{ req/s}$.
* **Invalid-Request Circuit Breaker:** sliding-window counter over $\{401, 403, 429\}$ responses. Hitting $\ge 10^4 / 10\text{ min}$ guarantees a 24h Cloudflare IP ban. Exclude responses carrying `X-RateLimit-Scope: shared`.
* **Channel-Name Perimeter Limit (exact):** channel `name`/`topic` mutation is capped at **2 requests / 10 min per channel**. This limit is *invisible* in `X-RateLimit-*` headers — hard-code it client-side (the sanctioned exception to Discord's no-hardcode rule). It fires on `name` or `topic`, and once drained it locks the channel's entire `PATCH` bucket (blocking unrelated edits: `setParent`, NSFW toggle, `rate_limit_per_user`). Anchor this threshold into the breaker to block that channel's egress queue *locally* before the 429 lands. Directly constrains the stats-channel feature (§10).

## 10. Feature Scope & Non-Regression Contract

This registry is the load-bearing contract: the agent must never silently regress any row. `Path` = RX (ingest, alloc-sensitive) or TX (egress, rate-limited). Interaction/webhook rows are egress-headroom levers (§9.2).

### Logging & Telemetry

| Feature | Discord surface | Intent / dependency | Path |
| --- | --- | --- | --- |
| User lifecycle logs (join/leave) | `GUILD_MEMBER_ADD` / `_REMOVE` | `GUILD_MEMBERS` | RX |
| State-mutation logs (role/avatar) | `GUILD_MEMBER_UPDATE`, `USER_UPDATE` | `GUILD_MEMBERS` | RX |
| Message persistence (edit/delete) | `MESSAGE_UPDATE` / `_DELETE` / bulk | `MESSAGE_CONTENT` (isolated to `LogIngest`) | RX |
| Native automod intercepts (embeds) | `AUTO_MODERATION_ACTION_EXECUTION`; `POST …/auto-moderation/rules` | `AUTO_MODERATION_*` | RX + TX |

### Moderation & Support

| Feature | Discord surface | Dependency | Path |
| --- | --- | --- | --- |
| Ban / kick / timeout | `PUT …/bans`, `DELETE …/members/{id}`, `PATCH communication_disabled_until` (≤28 d) | member perms | TX |
| Full API-native suite | roles `PUT/DELETE`, nickname `PATCH`, voice-state (mute/deafen/move/disconnect), thread evict, `bulk-delete` (≤100, <14 d) | per-action perms | TX |
| Ticket gateways (channel-bound) | channel create + permission overwrites + message components | Manage Channels | TX |
| DM modmail pipeline | DM channel open + message relay | — | TX (bypasses guild bucket) |

### Guild Identity & Statistics

| Feature | Discord surface | Constraint | Path |
| --- | --- | --- | --- |
| Stats channels (member/bot counts) | channel-name `PATCH` | **2 / 10 min per channel** (§9.2) — hard cap | TX |
| `/roles` interactive | application command + button components + role `PUT/DELETE` | — | TX |
| Custom embed generation | message-create with embeds | — | TX |

### Community & External Relations

| Feature | Discord surface | Dependency | Path |
| --- | --- | --- | --- |
| QOTD execution stack | scheduled message post (+ thread) | — | TX |
| Partner registry creation | persistence layer (state) | single-writer | internal |
| Automated partner-list publish (embeds) | message create/edit embeds | — | TX |

## 11. The Conformance Fixture (Baseline Standard)

All `discordcore` hot-paths are structurally checked against this topology: lock-free metric aggregation, zero-escape serialization into caller-owned slices, `JoinSet`-anchored lifecycle, and `CancellationToken` cascade cancellation. Zero-alloc holds when the caller pre-reserves the destination buffer — a hypothesis the §7 falsifier verifies, not an assertion.

```toml
# Cargo.toml
[dependencies]
tokio = { version = "1", features = ["rt-multi-thread", "macros", "sync"] }
tokio-util = "0.7"
async-channel = "2"
crossbeam-utils = "0.8"

```

```rust
#![deny(unsafe_op_in_unsafe_fn)]

use std::fmt;
use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};

use async_channel::bounded;
use crossbeam_utils::CachePadded;
use tokio::task::JoinSet;
use tokio_util::sync::CancellationToken;

/// Lock-free, zero-allocation telemetry aggregator.
/// Contended counters are cache-padded to defeat false sharing (§4).
pub struct TelemetrySink {
    operations: CachePadded<AtomicU64>,
    faults: CachePadded<AtomicU64>,
}

impl TelemetrySink {
    pub fn new() -> Self {
        Self {
            operations: CachePadded::new(AtomicU64::new(0)),
            faults: CachePadded::new(AtomicU64::new(0)),
        }
    }

    /// Scalar state mutation via hardware atomics. Relaxed ordering is
    /// sufficient: the two counters carry no cross-variable happens-before.
    #[inline]
    pub fn record(&self, fault: bool) {
        self.operations.fetch_add(1, Ordering::Relaxed);
        if fault {
            self.faults.fetch_add(1, Ordering::Relaxed);
        }
    }

    /// Upper bound of the serialized form: b"OPS:" + u64 + b"|FLT:" + u64.
    pub const MAX_SERIALIZED_LEN: usize = 4 + 20 + 5 + 20;

    /// Serializes metric state into a caller-owned slice; returns bytes
    /// written, or `None` if `dst` is undersized (cold path — callers size
    /// via `MAX_SERIALIZED_LEN`). Zero heap allocation is structural here:
    /// no growth path exists, unlike `&mut Vec<u8>`, which silently
    /// reallocates past capacity. The §7 falsifier verifies, not asserts.
    pub fn serialize_into(&self, dst: &mut [u8]) -> Option<usize> {
        let mut at = 0;
        at = put(dst, at, b"OPS:")?;
        at = put_u64(dst, at, self.operations.load(Ordering::Relaxed))?;
        at = put(dst, at, b"|FLT:")?;
        at = put_u64(dst, at, self.faults.load(Ordering::Relaxed))?;
        Some(at)
    }
}

impl Default for TelemetrySink {
    fn default() -> Self {
        Self::new()
    }
}

/// Copies `src` into `dst[at..]`; returns the advanced cursor, or `None`
/// on undersized `dst`. Bounds-checked — no `unsafe` surface (§6: minimal).
#[inline]
fn put(dst: &mut [u8], at: usize, src: &[u8]) -> Option<usize> {
    let end = at.checked_add(src.len())?;
    dst.get_mut(at..end)?.copy_from_slice(src);
    Some(end)
}

/// Writes the base-10 ASCII of `v` into `dst[at..]`. A 20-byte stack
/// scratchpad covers the maximum length of a base-10 `u64`; no formatting
/// allocation, no heap escape (the scratch never outlives the frame).
fn put_u64(dst: &mut [u8], at: usize, mut v: u64) -> Option<usize> {
    let mut scratch = [0u8; 20];
    let mut i = scratch.len();
    loop {
        i -= 1;
        scratch[i] = b'0' + (v % 10) as u8;
        v /= 10;
        if v == 0 {
            break;
        }
    }
    put(dst, at, &scratch[i..])
}

#[derive(Debug)]
pub enum OrchestrateError {
    InvalidTopology,
    WorkerPanicked,
    LingeringReference,
}

impl fmt::Display for OrchestrateError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::InvalidTopology => f.write_str("invalid worker topology dimension"),
            Self::WorkerPanicked => f.write_str("worker task panicked"),
            Self::LingeringReference => f.write_str("lingering sink reference after join"),
        }
    }
}

impl std::error::Error for OrchestrateError {}

/// Enforces explicit topological lifecycle boundaries. Cancellation cascades
/// through `CancellationToken`; every task is anchored in `JoinSet` (no
/// detached spawn). The work queue is bounded strictly via Little's Law.
pub async fn orchestrate(
    cancel: CancellationToken,
    workers: usize,
    lambda: usize,         // events/sec
    service_time_ms: usize,
) -> Result<TelemetrySink, OrchestrateError> {
    if workers == 0 {
        return Err(OrchestrateError::InvalidTopology);
    }

    // Bounded strictly via Little's Law (L = lambda * W); *2 absorbs bursts.
    let burst_capacity = ((lambda * service_time_ms) / 1000).max(1);
    let (tx, rx) = bounded::<u32>(burst_capacity * 2);

    let sink = Arc::new(TelemetrySink::new());
    let mut set: JoinSet<()> = JoinSet::new();

    // Producer. Owns the sole sender; dropping it closes the queue.
    {
        let cancel = cancel.clone();
        set.spawn(async move {
            for i in 0..(workers as u32 * 100) {
                tokio::select! {
                    _ = cancel.cancelled() => return,
                    res = tx.send(i) => if res.is_err() { return; }
                }
            }
            // tx dropped here -> receivers observe closure and drain-exit.
        });
    }

    // Worker pool. MPMC receiver clones fan out one bounded queue.
    for _ in 0..workers {
        let rx = rx.clone();
        let sink = Arc::clone(&sink);
        let cancel = cancel.clone();
        set.spawn(async move {
            loop {
                tokio::select! {
                    _ = cancel.cancelled() => return,
                    msg = rx.recv() => match msg {
                        Ok(v) => sink.record(v % 10 == 0),
                        Err(_) => return, // closed and drained
                    }
                }
            }
        });
    }
    drop(rx); // extra receiver would not keep the queue open, but be explicit.

    while let Some(joined) = set.join_next().await {
        if joined.is_err() {
            cancel.cancel(); // fail-stop: cascade to siblings.
            // Dropping the JoinSet would abort remaining tasks implicitly at
            // their next await point; make the reap explicit and awaited so
            // no task outlives this frame (deterministic teardown, §5).
            set.shutdown().await;
            return Err(OrchestrateError::WorkerPanicked);
        }
    }

    Arc::try_unwrap(sink).map_err(|_| OrchestrateError::LingeringReference)
}

```

## 12. App Initialization & UI Boundary (Slint)

* **UI/IO Decoupling:** The Slint presentation layer is strictly decoupled from the Tokio asynchronous backend. Tokio is initialized first.
* **Topological Anchoring:** All asynchronous background tasks (database connection pools, Slint-to-Tokio IPC bridge, Discord websocket sessions) are anchored inside a root `tokio::task::JoinSet`. Unbound `tokio::spawn` calls are strictly prohibited.
* **Backpressure:** Slint-to-Tokio communication operates over strictly bounded `tokio::sync::mpsc` channels. Under Little's Law limits, this imposes I/O backpressure to the UI if the network layer saturates.
* **Reactive Constraints:** UI feature toggles remain in a locked (greyed-out) state until the Tokio backend dispatches a `BotState::Authenticated` event over the IPC channel.