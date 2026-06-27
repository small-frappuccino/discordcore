# DISCORDCORE: Go Tier-1 CSP & Zero-Allocation Engineering Manifesto

This document establishes the inviolable architectural boundaries and deterministic execution protocols for all automated and human contributors operating within the `discordcore` repository. The exclusive operational target is bare-metal, extreme-throughput Discord API routing using Go as a Tier-1 systems language. General Go craft, epistemics, and output style are governed by `GEMINI.md`; the per-run execution loop by the `/goal` harness. This file carries **only** discordcore architecture.

The system is **two paths with opposite optimization regimes**, and every directive below resolves to one of them. **Gateway-RX** (event ingestion) is allocation-sensitive and CPU-bound. **REST-TX** (command egress) is rate-limited and network-bound. Conflating them is the root architectural error: a sub-millisecond GC pause is irrelevant against a 40 ms API round-trip, and a mishandled `429` backoff destroys more throughput than any allocation ever will.

## 1. Reasoning Discipline (High-Compute Allocation)

At 'High' reasoning capacity, spend the extra compute cycles exclusively on verification — never on re-deriving architecture already settled in this file:

1. **Allocation Auditing:** Recursively trace every variable on the RX/compute hot path (§2 scope). If a variable is not stack-allocatable, identify the exact escape cause and refactor before emitting code. Do NOT spend this budget on the TX path (§6.2).
2. **Lock-Contention Simulation:** Simulate the logic under $10^4$ ops/sec. Flag any mutex bottleneck and preemptively swap to atomic/CoW before the first code block is generated.
3. **Invariant Check:** Verify the output against the Go Memory Model and the §5 routing preconditions (cardinality, uniqueness, typed-feature, token redaction), enforced at the single-writer boundary.
4. **Output Restriction:** Reasoning stays internal. Emit only the AST/diff plus the minimal necessary architectural justification — never the chain-of-thought.

## 2. Allocation Regime (RX-Sensitive / TX-Exempt)

The allocation-sensitive compute domain is the Gateway-RX decode/dispatch pipeline (§6.1) and the in-memory routing table (§5); within it, data locality, CPU L1/L2 cache-line utilization, and predictable GC latency (targeting $<1$ms pauses) are absolute invariants. The REST-TX egress path (§6.2) is network-bound and **EXEMPT** — optimization budget there is spent exclusively on rate-limit scheduling, never on allocation. General string-builder, escape-blocking, and reflection-ban craft is per `GEMINI.md` §3; the discordcore-specific layout rules are:

* **Stack-Pinned Critical Paths:** Hot loops must yield absolute zero heap allocations. APIs must accept caller-allocated buffers (`[]byte`, pre-sized slices) via pointer to enforce stack retention.
* **Struct Packing:** Struct fields must be strictly ordered by byte size (largest to smallest) to eliminate implicit memory padding and maximize cache-line efficiency.
* **Aggressive Object Pooling:** Transient objects (parsers, JSON encoders, byte buffers) must be amortized via `sync.Pool`. Pools must be zeroed explicitly before returning to prevent residual state corruption.

## 3. Deterministic CSP (Communicating Sequential Processes)

Go’s CSP implementation is the engine of `discordcore`. Goroutines and channels must be orchestrated to maximize throughput while preventing memory exhaustion and scheduler saturation.

### Bounded Ingress & Load Shedding
* **Scheduler Protection:** Unbounded network ingress spawning `go process()` is a catastrophic vulnerability. All incoming WebSocket payloads must be routed through bounded worker pools or throttled via `x/sync/semaphore`.
* **Backpressure via Little's Law:** Buffered channels act as queues. Their depth must be mathematically dimensioned to absorb micro-bursts ($L = \lambda W$) without hiding systemic latency. Unbuffered channels are restricted strictly to rigid, synchronous rendezvous.

### Allocation-Free Channel Transmission
* **Concrete Typing:** Passing interfaces over channels forces heap allocation. Channels must transmit concrete types or pointers to `sync.Pool` objects exclusively.
* **Data-Oriented CSP:** When passing messages to worker goroutines, transmit flat data indices or small struct values (which copy natively on the stack) rather than deeply nested, pointer-heavy object graphs.

### Supporting Paradigms for CSP
To prevent channels from becoming contention bottlenecks, Go's secondary paradigms must support the CSP core:

* **The Actor Model (State Sharding):** Isolate mutating state. Treat each Discord Guild as an independent Actor (a singular goroutine reading from a bounded inbox channel). This segregates state mutations serially, eradicating global mutexes and allowing thousands of guilds to process concurrently without cross-talk. **The config-mutation writer (§5) is itself an Actor** — a single goroutine draining the `LISTEN/NOTIFY` inbox.
* **Copy-on-Write (CoW) for Global Reads:** For globally accessed, read-heavy state — concretely, **the routing table of §5** — CSP channels introduce unnecessary latency. Use `sync/atomic.Pointer[T]` with CoW semantics. Writers (Actors) clone and swap the pointer via Compare-and-Swap (CAS), guaranteeing $O(1)$ zero-latency, lock-free reads for the ingress routers.
* **Immutability by Default:** Once a domain entity traverses a channel boundary, it is strictly immutable. If a worker must alter the state, it must invoke `.Clone()` to decouple memory ownership before mutation.

## 4. Toolchain & Resiliency

* **Go 1.26+ Iterators:** Replace slice-allocating batch retrievals with `iter.Seq[V]` and `iter.Seq2[K, V]`. Memory-heavy database cursors must yield lazily to the CSP pipeline.
* **Profile-Guided Optimization (PGO):** Release builds must compile with `-pgo=auto`. Hot paths must be architected to allow the compiler to inline aggressively based on production `default.pgo` profiles.
* **The Supreme Pipeline:** Untested code is broken code. All output must pass `gofmt`, `go vet`, `govulncheck`, `golangci-lint` (K8s-level strictness), and the Go Race Detector (`go test -race`). The loop-termination gate is operationalized in the `/goal` harness VERIFY phase.

## 5. Multi-Tenant Routing Contract (Domain Invariants)

The routing table is the source of truth: `(GuildID, Feature) → BotInstance`. These invariants are **preconditions** of the hot path, enforced exclusively at the single-writer boundary (DB hydration + live `LISTEN/NOTIFY` mutation). The ingress router NEVER re-validates at read time; it reads the CoW snapshot of §3 lock-free.

* **Feature Cardinality (Uniqueness):** For any `(GuildID, Feature)`, exactly one `BotInstance` serves it at $T_n$. A write reassigning an already-bound feature is **route theft** — never a silent overwrite. **Default policy: `reject-if-bound`** (fail-closed). An operator must explicitly `DISABLE` the incumbent binding before a new bot can claim the feature; this prevents an errant or hostile `NOTIFY` from silently transferring moderation authority. The alternative, `atomic-reassign`, is documented but not default — flip it only with deliberate intent.
* **Bot Cardinality (Cap):** $|\{\text{distinct BotInstance} : \text{GuildID}=g\}| \le 5\ \forall g$. Enforced at **both** ingress points. The check on `ENABLE` for `(g, _, botB)`: if `botB ∉ currentBots(g) ∧ |currentBots(g)| ≥ 5` ⇒ reject. Assigning a feature to a bot *already present* in the guild does not increment the cap. Hydration encountering a 6th distinct bot in the DB is a data-integrity violation — skip and log loudly, never load.
* **Feature Scope (Typed Domain):** The full moderation set replaces the `ban`/`kick` pair. `Feature` MUST be `type Feature uint8`, never a `string`. A `uint8` over a bounded enum collapses the per-guild feature routing table to a fixed `[N]*BotInstance` array (or a presence bitmask): branch-predictable $O(1)$ indexing, zero hashing, one cache line. A `string` key forces an $O(\text{len})$ hash on every access and forecloses that representation — the allocation risk specifically being any *composite-key construction* (concatenation), not the lookup itself. Enumerate `Ban, Kick, Timeout, Deafen, MoveMember, MsgDelete, ChannelPurge, RoleAdd, RoleRemove, …`. Parse and validate at the boundary (DB `Scan` + JSON `Unmarshal`); an unknown `Feature` is rejected at ingestion, never routed.
* **Single-Writer Enforcement:** Cap and uniqueness require a serialized check-and-swap. The config-mutation path is therefore deliberately single-threaded (the Actor of §3). **"Everything concurrent" governs command *execution* (§6.2), NOT registry mutation.** Parallelizing the writer reintroduces a TOCTOU race on the cardinality check. This is not a bottleneck: mutation frequency is orders of magnitude below ingress frequency.
* **Fallible Mutation API:** Because the writer must reject, `Registry.UpdateRoute` and `RemoveRoute` MUST return `error`. The current void signatures cannot carry an invariant rejection and are non-compliant. Hydration and the watcher must branch on the returned error and log the rejected mutation.
* **Secret Hygiene:** A bot token is a credential, not a field. `Token` MUST be a distinct type implementing `slog.LogValuer` returning a redacted value, so structured logging of any containing struct (`BotInstance`, `ConfigChangeEvent`) can never leak it:
  ```go
  type Token string

  func (Token) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
  ```
  Logging the raw struct then redacts automatically. Plaintext `BotToken string` in a loggable struct is a non-compliant credential-exposure defect.

## 6. Discord Transport Layer (Direct REST / Gateway)

Removing Arikawa/Discordgo transfers their single hardest responsibility — Discord's rate-limit and connection state machine — onto you. Two paths, opposite regimes. The CSP win over the old binary is here: the libraries serialized what is independently parallelizable.

### 6.1 Gateway-RX (Event Ingestion — Allocation-Sensitive)
* **Intent Minimization is the Primary Lever:** Request only the Gateway Intents the active feature scope requires (`GUILD_MODERATION`, `GUILD_MEMBERS`, `GUILD_MESSAGES` as features demand). Every un-requested intent is an event you never receive, decode, or allocate for — a larger throughput win than any per-event struct tuning. Never request `GUILD_PRESENCES` or `MESSAGE_CONTENT` (both privileged) unless a feature consumes them. Moderation actions are *executed* via REST, so RX intents exist to *observe* state, not to act — keep the surface minimal.
* **Zero-Allocation Decode/Dispatch:** The §2 mandate applies here in proportion to subscribed intents — `sync.Pool` the decode buffers, codegen the unmarshalers (`easyjson`/`msgp`), and fan out flat event indices over bounded channels to the per-guild Actors of §3. No interface values cross the channel boundary.
* **Per-Bot Gateway Lifecycle:** Each token owns an independent Gateway connection (an Actor). Mandatory state machine:
  * `IDENTIFY` (Op 2), gated by the token's own `session_start_limit.max_concurrency` (≈5 s rate window) — the binding constraint when cold-booting multiple bots simultaneously.
  * Heartbeat (Op 1) on the `Hello` (Op 10) interval, with an **ACK (Op 11) watchdog**: a missed ACK ⇒ force-close and resume.
  * `RESUME` (Op 6) on transient drop, preserving `session_id` + last `seq`.
  * Full re-`IDENTIFY` only on a non-resumable Invalid Session (Op 9, `d:false`) or a `Reconnect` (Op 7) that resume rejects.
  * Shard (`Get Gateway Bot` recommended count) only when a single connection nears the ~2500-guild ceiling; per-tenant bots rarely reach it.

### 6.2 REST-TX (Command Egress — Rate-Limited, Network-Bound)
* **GC is Irrelevant Here:** A $<1$ms pause is noise against a ~40 ms round-trip. Spend zero optimization budget on TX allocation. Spend it on **rate-limit scheduling**.
* **Per-Bot, Per-Route Bucket Accounting:** Each token is an independent rate-limit domain; buckets are keyed by route + major parameter (`guild_id`, `channel_id`, `webhook_id`). Honor proactively — read `X-RateLimit-Bucket`, `-Remaining`, `-Reset-After` and throttle *before* sending. A per-bucket token gate (channel or `x/sync/semaphore`), scoped per bot, serializes requests sharing a bucket.
* **Global Limit + 429 Discipline:** Respect the $50$ req/s global ceiling **per token**. On `429`, honor `Retry-After` and `X-RateLimit-Scope` (`user` / `global` / `shared`). A `429` storm trips the $10{,}000$-invalid-requests/$10$ min Cloudflare ban (invalid = `401`/`403`/`429`). The backoff path is the single highest-leverage correctness surface in TX — a naive retry loop is self-inflicted denial of service.
* **Concurrency Model:** Distinct buckets and distinct bots run concurrently; the same bucket serializes through its gate. The throughput ceiling is $\sum(\text{per-token global limits})$ bounded by the $\le 5$-bot cap and the 1-feature-1-bot rule — **not** CPU cores. Scale the bot pool against the aggregate req/s target. A single token serving a high-volume feature across many guilds shares one 50 req/s global budget; distributing distinct features across distinct bots within a guild is the lever to multiply per-guild egress.