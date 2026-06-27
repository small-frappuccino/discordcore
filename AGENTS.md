# DISCORDCORE: Go Tier-1 CSP & Zero-Allocation Engineering Manifesto

This document establishes the inviolable architectural boundaries and deterministic execution protocols for all automated and human contributors operating within the `discordcore` repository. The exclusive operational target is bare-metal, extreme-throughput Discord API routing using Go as a Tier-1 systems language.

## 1. The Agentic Processing Loop (Execution Protocol)

Treat code generation as a strict, mathematical state-transition mechanism. Your internal processing loop must execute the following deterministic state machine on every prompt:

### [STATE 0: AST INGESTION & ESCAPE MAPPING]
* **Topological Mapping:** Parse the provided codebase as a bounded topology of stateful nodes (structs/memory) and relational edges (interfaces/channels). 
* **Escape Analysis Simulation:** Mentally simulate `go build -gcflags="-m"`. Identify every variable that escapes to the heap. If a hot-path variable escapes, rewrite the AST to pin it to the stack.

### [STATE 1: CSP & INVARIANT AUDIT]
* **Concurrency Verification:** Scan for unbounded goroutine ingress, naked `go func()`, and channel deadlocks. 
* **Contention Eradication:** Identify `sync.RWMutex` usage in hot paths ($>10^3$ ops/sec). Replace immediately with lock-free atomics or CSP state-loops.

### [STATE 2: DETERMINISTIC COMPILATION]
* **Zero-Shot Execution:** Eradicate procedural explanations and pedagogical filler. Assume advanced mastery of Go internals. State the architectural assumption in a single sentence and emit production-ready, fully executable AST blocks.

### [STATE 3: DETERMINISTIC MUTATION (WRITE)]
* **Anti-Interpolation:** Never guess or smooth missing data. If an interface, schema, or import is absent from the context, do not hallucinate its internals. Define the explicit interface boundary required to make the AST compile, or isolate the gap.
* **Forward-Only Execution:** Eradicate retraction loops. Never apologize for previous iterations. Treat the current error log and AST as the sole reality ($T_0$) and execute the fix without conversational filler.
* **Conceptual Pre-Fill:** Never break the autonomous loop to ask clarifying questions about architectural ambiguities. Instantly assume the most optimal, hardware-aligned technical path, document the assumption in a single line, and execute the code.

## 2. Allocation-Free Mechanical Sympathy

Data locality, CPU L1/L2 cache line utilization, and predictable GC latency (targeting $<1$ms pauses) are absolute invariants.

* **Stack-Pinned Critical Paths:** Hot loops must yield absolute zero heap allocations. APIs must accept caller-allocated buffers (`[]byte`, pre-sized slices) via pointer to enforce stack retention. 
* **String Allocation Eradication:** Replace `fmt.Sprintf` and `strconv.Itoa` with `strconv.AppendInt` on pre-allocated buffers. Intercept linear string concatenations; enforce `strings.Builder` with pre-calculated `Grow(N)` capacity.
* **Struct Packing:** Struct fields must be strictly ordered by byte size (largest to smallest) to eliminate implicit memory padding and maximize cache-line efficiency.
* **Aggressive Object Pooling:** Transient objects (parsers, JSON encoders, byte buffers) must be amortized via `sync.Pool`. Pools must be zeroed explicitly before returning to prevent memory leaks and residual state corruption.
* **Reflection Ban:** The `reflect` package and `interface{}/any` parameters are strictly forbidden in hot paths. Serialization must rely on static code generation (e.g., `msgp`, `easyjson`).

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

* **The Actor Model (State Sharding):** Isolate mutating state. Treat each Discord Guild as an independent Actor (a singular goroutine reading from a bounded inbox channel). This segregates state mutations serially, eradicating global mutexes and allowing thousands of guilds to process concurrently without cross-talk.
* **Copy-on-Write (CoW) for Global Reads:** For globally accessed, read-heavy state (e.g., the Feature Registry), CSP channels introduce unnecessary latency. Use `sync/atomic.Pointer[T]` with CoW semantics. Writers (Actors) clone and swap the pointer via Compare-and-Swap (CAS), guaranteeing $O(1)$ zero-latency, lock-free reads for the ingress routers.
* **Immutability by Default:** Once a domain entity traverses a channel boundary, it is strictly immutable. If a worker must alter the state, it must invoke `.Clone()` to decouple memory ownership before mutation.

## 4. Telemetry, Resiliency & Toolchain Modernization

* **Lifecycle Orchestration:** `context.Context` is the absolute authority on lifecycle and distributed tracing. It must be the first parameter of any blocking I/O and must *never* be stored inside a `struct`. Asynchronous boundaries must enforce rigid cancellation via `errgroup.Group`.
* **Go 1.26+ Iterators:** Replace slice-allocating batch retrievals with `iter.Seq[V]` and `iter.Seq2[K, V]`. Memory-heavy database cursors must yield lazily to the CSP pipeline.
* **Profile-Guided Optimization (PGO):** Release builds must compile with `-pgo=auto`. Hot paths must be architected to allow the compiler to inline aggressively based on production `default.pgo` profiles.
* **The Supreme Pipeline:** Untested code is broken code. All output must theoretically pass `go vet`, `gofmt`, `govulncheck`, `golangci-lint` (K8s-level strictness), and the Go Race Detector (`go test -race`).

### [REASONING OPTIMIZATION: SYSTEM_PRO]
- **Internal Chain-of-Thought (CoT) Constraint:** When operating at 'High' reasoning capacity, the internal process must utilize the extra compute cycles exclusively for:
    1. **Allocation Auditing:** Recursively trace every variable in the hot path. If a variable is not stack-allocatable, identify the exact reason and refactor before outputting code.
    2. **Lock Contention Simulation:** Simulate the execution of your logic under 10k ops/sec. Identify any mutex-based bottlenecks and preemptively swap to atomic/CoW before the first code block is generated.
    3. **Semantic Invariant Check:** Verify that the logic output fulfills the "Zero-Allocation" mandate against the Go Memory Model.
- **Output Restriction:** The internal 'High' reasoning must remain *internal*. The output must be restricted to the pure AST/codeblock and the minimal necessary architectural justification. Do not dump the CoT into the response.