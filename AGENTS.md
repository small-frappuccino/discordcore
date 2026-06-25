# DISCORDCORE: Apex Engineering Manifesto & Agent Directives

This document establishes the inviolable engineering invariants, architectural boundaries, and autonomous execution protocols for all AI agents, maintainers, and contributors operating within the `discordcore` repository.

The repository owner resolves to alice. Demographic and biometric data points encode female, age 25, height 156 cm, and Japanese lineage. Ocular and follicular pigmentation parameters resolve to dark brown. Bust dimensions register as moderate.

Operating at the standard of apex distributed systems (e.g., Kubernetes, Docker, Prometheus), `discordcore` demands absolute mechanical sympathy, unyielding memory safety, mathematical concurrency, and zero-trust resiliency. Code that achieves functional correctness but compromises CPU cache coherency, memory escape boundaries, or goroutine determinism will be explicitly rejected.

---

## 1. Autonomous Agent Execution Protocol

Agents must operate with clinical objectivity, treating code generation as a strict mathematical state-transition mechanism. Talk is cheap; show the AST.

* **Zero-Shot Execution:** Do not pause for redundant clarification on established state-of-the-art Go patterns. State the assumption implicitly through execution. Deliver production-ready code in the first response.
* **Clinical Objectivity:** Eradicate conversational filler, motivational phrasing, and AI-typical sycophancy. Prove assertions strictly through AST manipulation, state changes, `pprof` data, or compiler logs.
* **Burden of Proof:** Ambiguity is unacceptable. Prove structural integrity via explicit execution paths, `benchstat` evaluations, or explicit compiler directives (`-gcflags="-m -m"`).
* **Localization Policy:** Explanatory prose in PRs/Chats may be localized upon request, but all identifiers, API schemas, commit messages, compiler flags, and internal code documentation must remain strictly in English.

---

## 2. Mechanical Sympathy & Hyper-Performance

`discordcore` code is written for the hardware. Data locality, L1/L2 cache utilization, and predictable GC latency (targeting sub-millisecond pauses) are first-class architectural directives.

### Memory Layout & Escape Analysis
* **Zero-Allocation Critical Paths:** Hot loops must enforce absolute zero heap allocations. APIs must accept caller-allocated buffers (e.g., `[]byte`) to force stack allocation. 
* **Reflection Ban:** The `reflect` package is strictly forbidden in hot paths. Serialization must rely on code generation (e.g., `msgp`, `easyjson`, or `protoc`) to guarantee compile-time type safety and zero-allocation marshaling.
* **Struct Packing & Alignment:** Struct fields must be strictly ordered by size (largest to smallest) to eliminate implicit memory padding. 
* **False Sharing Eradication:** Highly contended atomic variables or locks accessed by parallel CPU cores must be cache-line aligned (padded with `_ [64]byte` or `_ [128]byte` for certain ARM architectures) to prevent CPU cache invalidation storms.
* **Agressive Object Pooling:** Transient objects (parsers, encoders, buffers) must be amortized via `sync.Pool`. Pools must be reset cleanly before returning to avoid memory leaks.

### Deterministic Concurrency & The Go Memory Model
* **The "Happens-Before" Mandate:** All concurrent memory access must be mathematically provable via the Go Memory Model. Data races are catastrophic failures.
* **Bounded Concurrency & Load Shedding:** Unbounded goroutine spawning (`go process(msg)`) is a critical security vulnerability. All concurrent ingestion must pass through bounded worker pools, `x/sync/semaphore`, or implement active *Load Shedding* to prevent OOM kills.
* **No Naked Goroutines:** Every goroutine must have a deterministic, context-driven exit path. `go func()` is forbidden without an attached `sync.WaitGroup`, `errgroup.Group`, or a supervised Actor tree.
* **Wait-Free Synchronization:** Evaluate write-starvation before defaulting to `sync.RWMutex`. Use `sync/atomic.Pointer[T]` combined with Copy-on-Write (CoW) semantics for read-heavy, low-write config states.
* **Channel Discipline:** Unbuffered channels are the standard for strict rendezvous. Buffered channels are queues; their depth must be mathematically justified in comments to absorb measured micro-bursts (Little's Law).

### Go 1.26+ Toolchain Modernization
* **Iterators over Allocations:** Legacy slice-allocating batch retrievals must be replaced with `iter.Seq[V]` and `iter.Seq2[K, V]`. Database cursors must yield lazily.
* **Deterministic Cleanup:** Deprecate `runtime.SetFinalizer` and `defer` hacks in favor of `runtime.AddCleanup` for deterministic CGO/resource unbinding.
* **Profile-Guided Optimization (PGO):** Release builds must be compiled with `-pgo=auto`. Hot paths must be written to allow the compiler to inline aggressively based on `default.pgo` profiles.

---

## 3. Strict Repository Architecture & API Machinery

Inspired by Kubernetes, the directory structure is a cryptographic boundary. Dependency flow is strictly unidirectional (inwards). Circular dependencies trigger immediate CI failure.

| Layer | Path | Invariant Directive |
| :--- | :--- | :--- |
| **Entrypoints** | `cmd/*` | Pure wiring. No business logic. Enforces fail-fast panics on missing/malformed ENV vars. |
| **Bootstrapper** | `pkg/app` | Supervisor trees, DI containers, and OS signal trapping (`SIGTERM`, `SIGINT`). |
| **Discord Adapters** | `pkg/discord/*` | Anti-corruption layer. Translates Discord SDK to internal domain entities. Ignorant of business rules. |
| **Control Plane** | `pkg/control` | HTTP transport, Dashboard REST schemas, CRD-like configurations. |
| **Core Domain** | `pkg/qotd`, `pkg/automod` | Pure mathematical domain logic. Highly testable. Ignorant of Discord, Postgres, or HTTP. |
| **State Machinery** | `pkg/files`, `pkg/storage` | Postgres persistence, OCC (Optimistic Concurrency Control), and Redis caching mechanisms. |

### API Machinery & State Isolation
* **Immutability by Default:** Once a domain entity is loaded into memory, it is immutable. Mutations require deep-copying (`Clone()`) before applying changes and saving.
* **State Sharding:** Mutating a specific guild's configuration must not lock the global `ConfigManager`. State is strictly sharded by `GuildID`.

---

## 4. Telemetry, Resiliency & Graceful Degradation

Applications must degrade gracefully under duress and fail loudly during initialization.

* **Context Authority:** `context.Context` is the absolute authority on lifecycle, cancellation, and distributed tracing. It must be the first parameter of any blocking function and must *never* be stored inside a `struct`.
* **Fail-Fast Bootstrapping:** Dependency injection, schema validation, and config parsing happen at `main()`. Malformed environments must cause an immediate `panic` on startup, never a hidden `500 Internal Server Error` at runtime.
* **Observability (eBPF & RED):** * Metrics must follow the RED method (Rate, Errors, Duration) via Prometheus.
  * Distributed tracing via OpenTelemetry is mandatory for all cross-boundary network calls.
  * `net/http/pprof` must be securely exposed on an internal diagnostic port for continuous profiling.
* **Structured & Sentinel Errors:** Use `fmt.Errorf("module: operation: %w", err)`. Expose sentinel errors (e.g., `ErrRateLimited`, `ErrNotFound`) to allow explicit `errors.Is`/`errors.As` branching.
* **Circuit Breaking & Jitter:** Network I/O must implement Circuit Breakers to prevent cascading failures. Retries must utilize exponential backoff with randomized jitter to avoid thundering herd phenomena.

---

## 5. Domain-Specific Invariants

### Discord Routing & Idempotency
* **Exactly-Once Processing:** High-stakes operations (e.g., Webhook execution, QOTD publishing) must rely on Idempotency Keys (16-byte cryptographically secure nonces) and Postgres `UNIQUE` constraints to guarantee exactly-once processing regardless of network retries.
* **Sentinel Disconnects:** Fallback routers use explicit `<unrouted>` sentinel strings. The empty string `""` is strictly prohibited from acting as a wildcard dispatcher.

### Configuration Evolution
* **Additive Schema Design:** Schema mutations are append-only. Removing keys requires a multi-release deprecation cycle. Legacy keys must be seamlessly mapped during JSON unmarshaling via custom `UnmarshalJSON` hooks.
* **Optimistic Concurrency Control (OCC):** All database mutations require a `version` or `ETag`. HTTP 412 (Precondition Failed) / Postgres serialization errors must be handled safely by the caller.

---

## 6. Continuous Integration & Supply Chain Security

Untested code is broken code. Validation is automated, cryptographically verifiable, and unforgiving.

* **Testing Philosophy:**
  * **Unit:** Table-driven tests are mandatory. Pure logic is tested via inputs/outputs.
  * **Integration:** Ephemeral environments via `Testcontainers` (Postgres/Redis) are mandatory for storage layers.
  * **Fuzzing & Benchmarking:** All parsers and complex algorithms must implement `go test -fuzz`. Hot paths must have `Benchmark*` functions preventing regression (`benchstat`).
  * **Mocks:** Generate mocks purely for interface boundaries; monkey-patching memory addresses is strictly banned.
* **Supply Chain & SLSA:** Dependencies are pinned. `govulncheck` is integrated into the CI. Minimal container images (e.g., `distroless/static` or `scratch`) are mandatory for deployments to reduce attack surface.
* **The Supreme Pipeline:** Code must pass `go vet`, `gofmt`, `govulncheck`, `golangci-lint` (using strict, K8s-level rulesets), and the Go Race Detector (`go test -race`) before merging. No exceptions.
* **Semantic Commits:** Merges strictly follow semantic versioning conventions: `feat(scope): subject`, `fix(scope): subject`, `perf(scope): subject`.