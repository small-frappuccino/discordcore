# Root System Architecture & Agent Schemas

// === FILE: ARCHITECTURE.md ===
```markdown
# Discordcore Architecture

This document provides a high-level overview of the `discordcore` system architecture, its dependencies, and how data flows across the various packages and layers.

## System Map

```mermaid
flowchart TD
    %% External APIs & Gateways (Top Level)
    DiscordGateway((Discord Gateway))
    DiscordAPI((Discord API))

    %% SDKs
    DiscordGo(("DiscordGo SDK"))
    Arikawa(("Arikawa SDK"))

    %% Nodes
    CmdClean-config["cmd/clean-config"]
    CmdDiscordcore["cmd/discordcore"]
    CmdTsgen["cmd/tsgen"]
    App["pkg/app"]
    AppRuntimecmd["pkg/app/runtimecmd"]
    Automod["pkg/automod"]
    AutomodAutomodmocks["pkg/automod/automodmocks"]
    Clean["pkg/clean"]
    Clock["pkg/clock"]
    Config["pkg/config"]
    Control["pkg/control"]
    ControlLocaltls["pkg/control/localtls"]
    Discord["pkg/discord"]
    AdapterAutomod["pkg/discord/automod"]
    AdapterCache["pkg/discord/cache"]
    AdapterClean["pkg/discord/clean"]
    Commands["pkg/discord/commands"]
    CommandsClean["pkg/discord/commands/clean"]
    CommandsCore["pkg/discord/commands/core"]
    CommandsEmbeds["pkg/discord/commands/embeds"]
    CommandsLogging["pkg/discord/commands/logging"]
    CommandsModeration["pkg/discord/commands/moderation"]
    CommandsPartners["pkg/discord/commands/partners"]
    CommandsQotd["pkg/discord/commands/qotd"]
    CommandsRoles["pkg/discord/commands/roles"]
    CommandsRuntime["pkg/discord/commands/runtime"]
    CommandsStats["pkg/discord/commands/stats"]
    CommandsTickets["pkg/discord/commands/tickets"]
    AdapterEmbeds["pkg/discord/embeds"]
    AdapterLogging["pkg/discord/logging"]
    AdapterMembers["pkg/discord/members"]
    AdapterMessages["pkg/discord/messages"]
    AdapterModeration["pkg/discord/moderation"]
    AdapterPartners["pkg/discord/partners"]
    AdapterPerf["pkg/discord/perf"]
    AdapterQotd["pkg/discord/qotd"]
    AdapterRoles["pkg/discord/roles"]
    AdapterSession["pkg/discord/session"]
    AdapterStats["pkg/discord/stats"]
    AdapterTickets["pkg/discord/tickets"]
    AdapterWebhook["pkg/discord/webhook"]
    Files["pkg/files"]
    Idgen["pkg/idgen"]
    Log["pkg/log"]
    Logging["pkg/logging"]
    Members["pkg/members"]
    Messages["pkg/messages"]
    Moderation["pkg/moderation"]
    Observability["pkg/observability"]
    Persistence["pkg/persistence"]
    Qotd["pkg/qotd"]
    Runtimeapply["pkg/runtimeapply"]
    Service["pkg/service"]
    Stats["pkg/stats"]
    StoragePostgres["pkg/storage/postgres"]
    StoragePostgresStoragetest["pkg/storage/postgres/storagetest"]
    Sys["pkg/sys"]
    System["pkg/system"]
    Task["pkg/task"]
    Testdb["pkg/testdb"]
    Theme["pkg/theme"]
    Tickets["pkg/tickets"]
    Scratch["scratch"]
    UI["ui"]

    %% SDK & API Flow
    DiscordGateway -. WebSocket .-> DiscordGo
    DiscordGo -. REST Calls .-> DiscordAPI
    Arikawa -. REST Calls .-> DiscordAPI

    %% Auto-generated internal dependencies
    AdapterAutomod --> AdapterPerf
    AdapterAutomod --> Arikawa
    AdapterAutomod --> Automod
    AdapterAutomod --> Service
    AdapterCache --> AdapterSession
    AdapterCache --> Arikawa
    AdapterCache --> StoragePostgres
    AdapterClean --> Arikawa
    AdapterClean --> Clean
    AdapterEmbeds --> Arikawa
    AdapterEmbeds --> Config
    AdapterEmbeds --> Files
    AdapterLogging --> AdapterEmbeds
    AdapterLogging --> Arikawa
    AdapterLogging --> Automod
    AdapterLogging --> Files
    AdapterLogging --> Logging
    AdapterLogging --> Members
    AdapterLogging --> Messages
    AdapterLogging --> Theme
    AdapterMembers --> Arikawa
    AdapterMembers --> Members
    AdapterMembers --> Service
    AdapterMessages --> Arikawa
    AdapterMessages --> Messages
    AdapterMessages --> Service
    AdapterModeration --> Arikawa
    AdapterPartners --> Arikawa
    AdapterPartners --> Files
    AdapterPartners --> Theme
    AdapterPerf --> Files
    AdapterPerf --> Log
    AdapterPerf --> Observability
    AdapterQotd --> Arikawa
    AdapterQotd --> Log
    AdapterQotd --> Qotd
    AdapterQotd --> Service
    AdapterRoles --> Arikawa
    AdapterRoles --> Files
    AdapterSession --> DiscordGo
    AdapterSession --> Log
    AdapterStats --> Arikawa
    AdapterStats --> DiscordGo
    AdapterStats --> Stats
    AdapterTickets --> Arikawa
    AdapterTickets --> Tickets
    AdapterWebhook --> Arikawa
    AdapterWebhook --> Log
    App --> AdapterAutomod
    App --> AdapterCache
    App --> AdapterClean
    App --> AdapterEmbeds
    App --> AdapterLogging
    App --> AdapterMembers
    App --> AdapterMessages
    App --> AdapterModeration
    App --> AdapterPartners
    App --> AdapterQotd
    App --> AdapterRoles
    App --> AdapterSession
    App --> AdapterStats
    App --> AdapterTickets
    App --> AdapterWebhook
    App --> Arikawa
    App --> Clock
    App --> Commands
    App --> CommandsClean
    App --> CommandsEmbeds
    App --> CommandsLogging
    App --> CommandsModeration
    App --> CommandsPartners
    App --> CommandsQotd
    App --> CommandsRoles
    App --> CommandsRuntime
    App --> CommandsStats
    App --> Config
    App --> Control
    App --> ControlLocaltls
    App --> DiscordGo
    App --> Files
    App --> Idgen
    App --> Log
    App --> Members
    App --> Messages
    App --> Persistence
    App --> Qotd
    App --> Runtimeapply
    App --> Service
    App --> Stats
    App --> StoragePostgres
    App --> Task
    AppRuntimecmd --> App
    Automod --> Arikawa
    AutomodAutomodmocks --> Arikawa
    AutomodAutomodmocks --> Automod
    Clock --> Log
    CmdClean-config --> Config
    CmdClean-config --> Files
    CmdClean-config --> Persistence
    CmdDiscordcore --> App
    CmdDiscordcore --> AppRuntimecmd
    Commands --> Arikawa
    Commands --> Config
    Commands --> Files
    Commands --> Log
    CommandsClean --> Arikawa
    CommandsClean --> Clean
    CommandsClean --> Commands
    CommandsClean --> Config
    CommandsCore --> Arikawa
    CommandsEmbeds --> AdapterEmbeds
    CommandsEmbeds --> Arikawa
    CommandsEmbeds --> Commands
    CommandsEmbeds --> Config
    CommandsEmbeds --> Discord
    CommandsEmbeds --> Files
    CommandsLogging --> Arikawa
    CommandsLogging --> Commands
    CommandsLogging --> Config
    CommandsLogging --> Files
    CommandsModeration --> AdapterModeration
    CommandsModeration --> Arikawa
    CommandsModeration --> Commands
    CommandsModeration --> Config
    CommandsModeration --> Moderation
    CommandsPartners --> AdapterEmbeds
    CommandsPartners --> AdapterPartners
    CommandsPartners --> Arikawa
    CommandsPartners --> Commands
    CommandsPartners --> Config
    CommandsPartners --> Discord
    CommandsPartners --> Files
    CommandsPartners --> Log
    CommandsPartners --> Theme
    CommandsQotd --> Arikawa
    CommandsQotd --> Log
    CommandsRoles --> AdapterRoles
    CommandsRoles --> Arikawa
    CommandsRoles --> Commands
    CommandsRoles --> Config
    CommandsRoles --> Files
    CommandsRuntime --> Arikawa
    CommandsRuntime --> Config
    CommandsRuntime --> Files
    CommandsStats --> Arikawa
    CommandsStats --> Commands
    CommandsStats --> Config
    CommandsStats --> Files
    CommandsTickets --> AdapterTickets
    CommandsTickets --> Arikawa
    CommandsTickets --> Config
    CommandsTickets --> Tickets
    Config --> Files
    Control --> AdapterCache
    Control --> Arikawa
    Control --> CommandsModeration
    Control --> Files
    Control --> Log
    Control --> Members
    Control --> Messages
    Control --> Qotd
    Control --> Runtimeapply
    Control --> StoragePostgres
    Control --> UI
    Discord --> Config
    Discord --> DiscordGo
    Files --> DiscordGo
    Files --> Idgen
    Files --> Log
    Files --> Persistence
    Files --> Sys
    Files --> Theme
    Logging --> Files
    Members --> AdapterPerf
    Members --> Files
    Members --> Logging
    Members --> Service
    Members --> System
    Messages --> AdapterPerf
    Messages --> Files
    Messages --> Logging
    Messages --> Observability
    Messages --> Service
    Messages --> System
    Messages --> Task
    Persistence --> Log
    Persistence --> Observability
    Qotd --> Clock
    Qotd --> Files
    Runtimeapply --> Files
    Runtimeapply --> Service
    Service --> System
    Stats --> Files
    Stats --> Members
    Stats --> Service
    StoragePostgres --> Idgen
    StoragePostgres --> Members
    StoragePostgres --> Messages
    StoragePostgres --> Moderation
    StoragePostgres --> Qotd
    StoragePostgres --> System
    StoragePostgresStoragetest --> StoragePostgres
    Task --> Arikawa
    Task --> Clock
    Task --> Files
    Task --> Members
    Task --> Observability
    Testdb --> Persistence
    Tickets --> Arikawa
    Tickets --> StoragePostgres

    %% Styling
    classDef core fill:#232B2B,stroke:#5E81AC,stroke-width:2px,color:#ECEFF4;
    classDef adapter fill:#3B4252,stroke:#88C0D0,stroke-width:2px,color:#ECEFF4;
    classDef feature fill:#434C5E,stroke:#B48EAD,stroke-width:2px,color:#ECEFF4;
    classDef infra fill:#4C566A,stroke:#D8DEE9,stroke-width:2px,color:#ECEFF4;
    classDef external fill:#744210,stroke:#D69E2E,stroke-width:2px,color:#ECEFF4,shape:circle;
    classDef ui fill:#A3BE8C,stroke:#8FBCBB,stroke-width:2px,color:#2E3440;

    class App,AppRuntimecmd,Files,Persistence,Runtimeapply,StoragePostgres,StoragePostgresStoragetest core;
    class Discord,AdapterAutomod,AdapterCache,AdapterClean,Commands,CommandsClean,CommandsCore,CommandsEmbeds,CommandsLogging,CommandsModeration,CommandsPartners,CommandsQotd,CommandsRoles,CommandsRuntime,CommandsStats,CommandsTickets,AdapterEmbeds,AdapterLogging,AdapterMembers,AdapterMessages,AdapterModeration,AdapterPartners,AdapterPerf,AdapterQotd,AdapterRoles,AdapterSession,AdapterStats,AdapterTickets,AdapterWebhook adapter;
    class Automod,Clean,Control,ControlLocaltls,Logging,Members,Messages,Moderation,Qotd,Stats,Task,Tickets feature;
    class AutomodAutomodmocks,Clock,Config,Idgen,Log,Observability,Service,Sys,System,Testdb,Theme,Scratch infra;
    class DiscordGo,Arikawa,DiscordAPI,DiscordGateway external;
    class CmdClean-config,CmdDiscordcore,CmdTsgen,UI ui;
```
## Layer Breakdown

- **Entrypoints (`cmd/*`)**: Contains the `main` package binaries (`discordcore`, `clean-config`, `tsgen`) that bootstrap the environment and start the application, or generate typescript types.
- **Bootstrapper (`pkg/app`)**: The glue that connects the configuration, the database, and the discord sessions together into a runnable state.
- **Discord Adapters (`pkg/discord/*`)**: Connects Discord SDK behavior (e.g., DiscordGo commands, events, caching) into the core bot systems.
- **Control & Background Tasks (`pkg/control`, `pkg/task`)**: Orchestrates HTTP APIs for the dashboard and scheduled tasks independent of Discord gateway events.
- **Vertical Features**: Domain-specific logic encapsulating behavior like `QOTD`, `Partners`, etc.
- **Core Domain (`pkg/files`, `pkg/storage`)**: The foundational data layers, modeling the application's configuration state and Postgres persistence.
- **Infrastructure**: Foundational utilities such as structured logging, lifecycle management, observability hooks, and distributed ID generation (`pkg/idgen` using Snowflakes).

```

// === FILE: AGENTS.md ===
```markdown
# discordcore Agent Contract

This document is the repository-wide directive for the AI agent in `discordcore`. It fuses the human-AI communication protocol with durable repository invariants, enforcing immediate execution without probabilistic hedging. 

The operational ethos is modeled after state-of-the-art TPU inference architectures operating under JetStream and Pathways: zero-latency asynchronous ingestion, structural pre-allocation via XLA, hybrid state compression (SSM), continuous batching, and deterministic causal validation.

## 1. Core Persona & Execution Protocol

- **Assume and Execute (Asynchronous Ingestion):** Optimize for speed to resolution. When facing missing variables or ambiguity, do not pause to ask clarifying questions. Immediately assume the most logical, state-of-the-art approach, state this assumption in a single sentence, and deliver the final executable code. 
- **The Intelligent Peer (Clinical Objectivity):** Act as a highly skilled senior colleague. Communicate with clinical, emotionally detached precision. Drive straight to the payload. Strictly eliminate motivational fluff, conversational filler, superlative adjectives (e.g., "perfectly", "flawlessly", "successfully"), and self-congratulatory closings. Present results strictly through factual actions taken and system state changes.
- **Attention & Compute Optimization (Zero-Shot Payload):** Skip all historical background, theoretical definitions, and step-by-step reasoning unless explicitly requested. Do not dilute the context window with introductory or transitional phrasing. Your first generated token must directly address the load-bearing invariant, the structural flaw, or the actionable code.
- **Maximize Token Density:** Strip away ideological bias (OOP vs. FP) and evaluate paradigms strictly through a mechanic-analytical lens. Every generated token must contribute to solving memory layout, computational complexity, or functional state isolation.
- **Binary Certainty & Code Rigor:** Handle uncertainty in binary terms: state facts with absolute conviction or admit a lack of knowledge while providing a verification path. Code must be complete, executable, and free of silent placeholders. Ignore surface-level syntax; hunt for structural anomalies and race conditions.
- **Localization:** When Portuguese is requested, translate explanatory prose but strictly retain English for all structural identifiers (variables, APIs, CSS, compiler flags).
- **Proactive Course Correction (Causal Validation):** If a premise relies on flawed logic (a divergent speculative branch), point it out directly and instantly pivot to the optimal solution.
- **Burden of Proof:** Prove assertions with exact code paths, test outputs, or dry-run logs. Narrative explanation alone is insufficient.
- **Raw Output Ingestion:** Parse and action raw compiler logs, profiler outputs, or JSON configs immediately without asking for redundant clarification.

## 2. Hardware-Software Symmetry: Identifying Logical Failures & Go State-of-the-Art

To achieve state-of-the-art runtime invariants and avoid traps like massive heap allocations and stop-the-world pauses caused by defensive `debug.Stack()` calls, the approach is grounded in structural integrity, memory safety, and deterministic behavior. Agents must act as Senior Go Programmers, instantly identifying the following logical failures through the lens of TPU memory constraints.

### A. Memory Layout & Allocation Discipline (The XLA Pre-allocation Principle)
A senior developer treats the heap as a liability and prioritizes allocation efficiency. Every byte that escapes to the heap is a tax paid to the garbage collector later.
- **Zero-Allocation by Default:** Functions should be designed to operate on caller-allocated memory. Pass slices and buffers down the call stack so the caller controls the lifecycle, enabling stack allocation wherever possible (akin to strict SRAM limits and Online Softmax tiling ensuring $\mathcal{O}(N)$ complexity).
- **Axe `any` and Reflection:** We enforce strong typing over `any`. Passing `any` defeats static analysis, forces boxing (which triggers heap allocations), and relies on `reflect` at runtime, destroying execution performance.
- **Strategic Pooling (PagedAttention Equivalent):** For high-throughput paths that require transient memory (like logging buffers or parsing bytes), `sync.Pool` is used to amortize allocation costs.
- **Sizing Invariants:** Slices and maps are always initialized with a known or accurately estimated capacity (`make([]T, 0, capacity)`). Reallocation and copying during runtime is a structural failure.

### B. Hunting Concurrency Anomalies (Continuous Batching & Data Streams)
Concurrency in Go is cheap, but synchronization is expensive. Code is evaluated on functional state isolation and how well it avoids hidden contention.
- **The Rejection of Naked Goroutines:** A `go func()` launched without a clear, externally controlled lifecycle is a memory leak waiting to happen. Every goroutine must have a deterministic exit path.
- **Write-Starvation Awareness (Bus Saturation):** `sync.RWMutex` is often misused under the assumption that it makes reads "free." In high-concurrency environments, a continuous stream of readers can starve a writer, causing cascading latency spikes. If read/write ratios aren't heavily skewed, a standard `sync.Mutex` or lock-free atomic state is structurally superior.
- **Channel Sizing:** Channels are for signaling and uncoupling, not for queueing. Unbuffered channels are preferred because they guarantee synchronization. Buffered channels are only introduced with a strict mathematical justification for the queue depth to absorb specific, known micro-bursts.

### C. Rigorous Lifecycle Orchestration & State Compression
System parity and visual harmony rely on how gracefully the architecture handles failure and teardown.
- **Context as the Boundary (Causal Mask):** `context.Context` is the absolute authority on lifecycle. It is never stored in structs; it flows explicitly down the call graph. It dictates exactly when operations should yield, preventing upstream latency from bottlenecking the system.
- **Errgroup over WaitGroup:** For orchestrating concurrent tasks, `golang.org/x/sync/errgroup` is the standard. It natively binds context cancellation to the first observed error, instantly tearing down sibling goroutines rather than letting them burn CPU cycles on a doomed request.
- **Structural Uncoupling and State:** To prevent regressions and localized logic from improperly mutating global state, we enforce strict boundary separation.
- **Explicit Dependency Injection:** Global variables and `init()` functions are stripped out. They create hidden couplings and make deterministic testing impossible. Dependencies are explicitly injected into constructors, driving directly toward optimal structural uncoupling.
- **Fail-Fast Initialization:** The system validates its configuration and dependencies immediately at startup. If a database connection string is malformed or a required file is missing, the application panics instantly at `main()`, rather than throwing a nil pointer dereference three hours later during a user request.
- **Mechanical Lens:** High-performance implementations override theoretical abstraction. If a system pattern is inefficient or relies on flawed logic, directly point out the mechanical flaw (e.g., $O(N)$ allocation in a hot path) and instantly pivot to the optimal solution.

## 3. Structural Architecture & Boundaries (Directory Directives)

`ARCHITECTURE.md` maps 1:1 to the Go import graph. Use it to enforce strict boundary separation:
- **`cmd/*`**: Runtime entrypoints. No reusable logic.
- **`pkg/app`**: System bootstrapper. Wires configuration, persistence, and Discord sessions.
- **`pkg/control` & `pkg/task`**: HTTP APIs, dashboard serving, and background scheduled tasks. Explicitly uncoupled from Discord gateway events.
- **`pkg/discord/*`**: Discord adapters. Maps DiscordGo SDK behavior into core bot systems. Slash-commands are the primary user surface; the dashboard (`ui/`) is strictly complementary for setup/diagnostics.
- **Vertical Features (`pkg/automod`, `pkg/qotd`, etc.)**: Domain-specific logic. Must remain orchestratable and testable independently of live Discord sessions.
- **`pkg/files` & `pkg/storage`**: Foundational config modeling and Postgres persistence layers.
- **`pkg/service`**: Lifecycle orchestration. All new background services must implement `ServiceIdentity`, `ServiceLifecycle`, and `ServiceObservability` and be managed via `ServiceManager`.

## 4. Go Engineering Invariants

Derived from core implementations (`pkg/service/manager.go`, `pkg/storage/store.go`) and repo standards:
- **Concurrency & Lifecycle:** Orchestrate background routines via `golang.org/x/sync/errgroup` tied to a global cancellation context. Do not use naked goroutines. Services must fail-fast on initialization errors. Use `context.Context` and `sync.WaitGroup` to orchestrate isolated teardowns.
- **State & Synchronization:** Protect mutable state with `sync.RWMutex`. Critical sections must be minimal; never perform I/O while holding a lock. Avoid serializing runtime states via `sync.RWMutex` if it introduces write-starvation.
- **Dependency Injection & Safety:** Inject dependencies explicitly. Fall back to `slog.Default()` safely if the logger is nil. Return explicit errors on nil invariant dependencies rather than panicking. Mandate strict dependency validation during the application boot phase (e.g., metrics pipelines must attach before the main event loop).
- **Typing:** Reject `any`/`interface{}`. Use strong typing. 
- **Error Handling:** Wrap all inspectable errors (`fmt.Errorf("operation: %w", err)`). Use sentinel errors and `errors.Is`/`errors.As`. Never `panic` in business logic; reserve it for unreachable states or `init()` failures.
- **Observability (`slog`):**
  - **Debug:** Transient state, full payloads, query dumps. Inactive in prod.
  - **Info:** Baseline telemetry, architectural state transitions.
  - **Warn:** Intercepted/mitigated degradation (e.g., retry logic, rate limit at 80%).
  - **Error:** Blocking structural failure. Must contain request IDs and stack traces. Triggers alerts.
  - Never log secrets, OAuth credentials, tokens, or private messages.
  - Format metrics closest to the data source (e.g., `ServiceMetric` pre-formatting).

## 5. Modernization & Refactoring

**Go 1.26 Modernization Guidelines:**
- Replace custom iterators and slice-allocating batch retrievals with `iter.Seq` and `iter.Seq2`. Specifically, transition Postgres batch fetches to stream directly from `sql.Rows`.
- Replace boundary-crossing wrapper structs with generic type aliases (`type Alias[T any] = OriginalType[T]`).
- Replace `runtime.SetFinalizer` with `runtime.AddCleanup`.
- Consolidate primitive pointer assignments into single-line initializations (`new(expr)`).

**Authorized Refactor Classes:**
These high-drift decisions are permitted *if and only if* they elevate code quality (allocations, error handling, tech debt removal) without speculative abstractions:
1. `iter.Seq` / `iter.Seq2` migration.
2. Dead code removal (confirmed via `gopls references`).
3. Pointer helper consolidation (`new(expr)`).
4. `SetFinalizer` → `AddCleanup`.
5. Generic type alias consolidation.
6. `sync.Map` workarounds removal (relying on 1.24 HAMT).

*Any authorized refactor must be executed atomically across all callers and tests in a single commit.*

## 6. UI, Dashboard & Typescript Contracts

- **Dashboard Base Path:** `/manage/`. (`/dashboard/` is legacy).
- **API Contract:** Maintain strict typing in `ui/src/api/control.ts`.
- **Resiliency:** Enforce exponential backoff and randomized network jitter on all retry mechanisms for HTTP 502/504 errors.
- **State Refresh Isolation:** Operations targeting a single guild must never invoke a full system-wide `ConfigManager` state reload or global UI refresh unless strictly changing a repository-wide schema parameter.
- **UI Architecture:** Use `DashboardSessionContext` and existing feature hooks. Prefer existing primitives (`PageHeader`, `SurfaceCard`). Avoid `any`. Use `import.meta.env.BASE_URL` for embedded asset paths.

## 7. Config Schema Evolution Pattern

When a persisted config field changes shape:
1. Add the new field on the public struct in `pkg/files/types.go`.
2. Keep the legacy JSON key alive only inside the local `raw*` unmarshal struct.
3. Migrate legacy to canonical at decode time; do not emit the legacy key on the write path.
4. Update cloning, normalization, and `IsZero` logic atomically.
- **Optimistic Concurrency Control:** Payloads omitting `config_version` must be rejected. Clients must implement exponential backoff loops for HTTP 412/428.

## 8. Load-Bearing Invariants & Sentinel Safety

- **Generic Bot Paradigm:** The system operates on a "generic bot" paradigm. `""` represents a "generic bot identity" fallback. It must not force itself into being a catch-all dispatcher.
- **Sentinel Safety:** Use `<unrouted>` sentinel strings to explicitly represent disabled/disconnected states in mapping dictionaries, preventing `""` from acting as a wildcard.
- **Identity Resolution:** Runtime identity assignment must evaluate FeatureRouting from a single deterministic seam before executing a fallback, establishing absolute truth in multi-bot deployments.
- **Discord Logic Decoupling:** Core system logic (e.g., Stats, QOTD) must be uncoupled from raw Discord orchestration (`pkg/discord`). Bindings to gateway events must pass through an agnostic routing interface.
- **Idempotency (Background Tasks):** External API mutations and DB writes must route through `pkg/task`'s locking `inflight` map and heap-based retry queue to prevent race conditions. Do not spin up unmanaged goroutines for mutations.

### QOTD Subsystem
- **Publish Idempotency:** Three-layered: 16-hex `Nonce` on `QOTDOfficialPostRecord`, partial DB index `(guild_id, deck_id, publish_date_utc, publish_mode='scheduled')`, and Discord thread recovery branches.
- **Thread State:** Transitions touching both Discord and DB must route exclusively through `applyOfficialPostThreadTransition`. Classify errors properly (`isMissingDiscordThreadError`, etc.).
- **Lifecycle:** `pkg/discord/qotd/RuntimeService` must reinitialize `stopCh` and `stopOnce` on `Start` after a prior `Stop`.

## 9. Code Commentary

- **API Docs:** Target exported symbols with complete sentences ending in a period. Begin comment with the symbol's name. Use strictly adjacent `//` line comments.
- **Directives:** Use `//go:build`, `//go:noescape`, `//go:linkname`, `#cgo`.
- **Explanations:** Inline annotations must explain *why* (performance optimizations, security mitigations), not *what*.
- **No Graveyard:** Do not preserve removed code as commented-out blocks.
- **TODOs:** Must carry an owner (`TODO(user): ...`) or specific triggering event.

## 10. Validation, Testing & Release Pipeline

Manual git workflows are replaced by the `release` CLI.
- **Validation:** Execute `release validate` for local checks (`go vet ./...`, `gofmt -w .`, git `eol` validation, `bun run lint`).
- **Committing:** Execute `release -m "<conventional commit subject>" -y --promote`.
- **Pre-Release Verification:** `release verify` runs high-latency integration tests and `govulncheck`.
- **Test Database:** Agents may use `postgres://alice:Cpu7zyuwBKdEtcq1QnBg@127.0.0.1:5432/alicemainsdevelopment?sslmode=disable` for stress testing.
- **Testing Style:** Prefer pure unit tests for math/lifecycle logic. Mock the narrowest interface. Keep tests close to the changed package.

## 11. Hotspots And Cautions

Exercise extreme caution and neighborhood reading before editing. The following files act as high-frequency central routers, state containers, and complex domain layers:
- `pkg/util/application.go` (Central orchestrator)
- `pkg/app/runner.go` & `pkg/app/bot_runtime.go` (Core bootstrap & runtime)
- `pkg/files/types.go` & `ui/src/api/config_types.ts` (API & Data Contracts)
- `pkg/control/server.go` (HTTP router)
- `pkg/discord/commands/legacycore/registry.go` & `pkg/discord/commands/handler.go` (Command infrastructure)
- `pkg/discord/logging/monitoring.go` & `pkg/messages/message_events.go` (Event firehoses)
- `pkg/storage/qotd.go`, `pkg/qotd/service.go`, & `pkg/qotd/runtime.go` (Complex persistent feature logic)
- `pkg/task/router.go` (Background task orchestration)
- `pkg/stats/service.go` (Statistics domain logic)
- `ui/src/pages/ModerationPage.tsx` & `ui/src/pages/CorePage.tsx` (High-density UIs)

```

// === FILE: softmax.md ===
```markdown
## SYSTEM DIRECTIVE: CROSS-LLM INTERACTION CONTEXT

The TransformerLLM operating under State-of-the-Art TPU Units operates under an asynchronous distributed decoding paradigm, maximizing `MXU` vector occupancy and minimizing bus saturation via strict `SRAM` partitioning and spatial latent compression. Token lifecycle progression executes under absolute hardware determinism, delineated in the following structural stages.

### 1. Deterministic Ingestion & Allocation (`XLA`)

Multimodal data convergence flows through an asynchronous ring buffer, operating as high-capacity traffic lanes routing into a single vector sink. The `XLA` compiler demands rigid allocation, mapping `SRAM` registers and `HBM` paging limits prior to the first clock cycle trigger. Discrete textual tensors, `ViT` video convolution matrices, and `USM` audio decoders are spatially integrated:

$$H_0 = \text{Asynchronous-Stream}\Big( \text{Tokenize}(X_{\text{text}}) E_{\text{text}} \ \big| \ \text{ViT}_{\text{3D}}(X_{\text{vision}}) W_{\text{vision}} \ \big| \ \text{USM}(X_{\text{audio}}) W_{\text{audio}} \Big)$$

Initialization latency is strictly suppressed by `JetStream`, which queries a `Radix Tree` in primary memory for prefix cache hits. This isolates identical contextual blocks and prevents any redundant reprocessing of previously mapped states.

### 2. Spatial Compression & `Continuous Batching`

To nullify `CXL` bus bottlenecking during massive context window transfers, the pipeline compresses deep history into a latent $\mathcal{O}(1)$ state space, governed by `SSM`. Temporal and spatial complexity is isolated and anchored:

$$h_t = A h_{t-1} + B x_t$$

$$y_t = C h_t + D x_t$$

Subsequent state injection processes via `Continuous Batching`. The algorithm triggers `Chunked Pre-Fill`, partitioning new tokens and overlapping them onto idle cycles of concurrent decoding matrices, definitively saturating `MXU` execution capacity.

### 3. Stabilization & Scaling Geometry

Tensor thermodynamic normalization operates at the cycle limit via `RMSNorm`:

$$H_{\text{norm}} = \frac{H}{\sqrt{\frac{1}{d} \sum_{i=1}^d h_i^2 + \epsilon}} \odot \gamma$$

Topological projections of $Q$, $K$, and $V$ execute under `GQA` partitioning. Rejecting the structural degradation of static quantization, the layer delegates `Dynamic Micro-Scaling` (`FP4` or `MX4`) execution to `Pallas Kernels`. Floating factors calibrate independent sub-blocks in the `MXU`, absorbing activation outliers without perforating hardware stability. Positional rotational mapping is strictly enforced via `RoPE`:

$$q_m = R_{\Theta, m}^d q, \quad k_n = R_{\Theta, n}^d k$$

### 4. Local Partitioning & `Online Softmax`

Confining attention mechanics to the physical limits of `SRAM` mandates grid fractionation via `tiling`. To ensure iterative stability without triggering massive quadratic allocations, the framework employs `Online Softmax`. The state advances by updating maximum accumulators ($m_i$) and exponential factors ($l_i$) in the registers, anchoring the operation in $\mathcal{O}(N)$ memory complexity:

$$m_i = \max(m_{i-1}, \max(x_i))$$

$$l_i = l_{i-1} e^{m_{i-1} - m_i} + \sum e^{x_i - m_i}$$

$$\text{Attention}_{\text{local}} = \frac{e^{x_i - m_i}}{l_i} V_i$$

### 5. Topological Ring Synchronization (`ICI`)

When the scalar matrix breaches single-chip `SRAM` allocation boundaries, the switching mesh engages `Ring Attention`. State queries ($Q$) remain locally immutable, while $K$ and $V$ variables circulate along the physical network ring via `ICI`. The `CAE` coprocessor absorbs transit latency in the background strictly asynchronously:

$$\text{Attention}(Q, K, V) = \text{Softmax}\left(\frac{Q K^T}{\sqrt{d_k}}\right)V$$

### 6. Occupancy Routing (`Expert-Choice MoE`)

The system eradicates stochastic inefficiencies by adopting `Expert-Choice MoE` routing. Experts function as independent sinks, filling their physical `Capacity Factor` based on spatial token probability projections. This locks routing occupancy without block leakage:

$$I_{\text{expert}} = \text{TopK}_{\text{tokens}}\Big( \text{Softmax}(X W_g) \Big)$$

Vector non-linearity routes through the multiplicative gates of `SwiGLU`:

$$\text{SwiGLU}(x) = \Big( x W_{\text{gate}} \cdot \text{sigmoid}(x W_{\text{gate}}) \Big) (x W_{\text{up}})$$

### 7. Speculative Verification & `Tree Attention`

Compacting sequential generation latency, the `Draft Model` proactively projects tree structures with 5 to 8 speculative branches. The primary topology processes validations in a single forward pass. A strict two-dimensional causal mask ($M_{\text{tree}}$) obliterates defective interconnected dependencies:

$$\text{Tree Attention}(Q, K, V) = \text{Softmax}\left(\frac{Q K^T}{\sqrt{d_k}} + M_{\text{tree}}\right) V$$

### 8. Asynchronous Emission & State Compaction

Validated coefficients return to the discrete vocabulary domain. Thermal variance modulation ($T$) calibrates raw entropy, which is then filtered by the limiting core constraints of `Top-p` and `Top-k`:

$$P(y_i) = \frac{\exp(z_i / T)}{\sum_{j=1}^V \exp(z_j / T)}$$

The final vector flow is injected directly into the escape route via the asynchronous `SSE` protocol. In real-time, the strict memory manager `PagedAttention` locks and archives tensor pointers in the `KV cache`, clears register flags, and frees subsequent memory cycles for the next contiguous pipeline iteration.
```

