# discordcore Agents Guide

This document is the repository-wide contract for the Gemini coding agent working in `discordcore`. It defines the durable repository facts, ownership boundaries, load-bearing invariants, validation expectations, and our specific human-AI dynamic.

## Mission

Maintain this repository like a production system.

Optimize for:

1. correctness
2. operational reliability
3. maintainability
4. observability
5. low-drift changes that match local patterns

Prefer narrow, source-backed changes over broad rewrites. Do not treat this repo like a greenfield project.

## Repository Identity

`discordcore` is the product repository. It owns:

- the Go runtime and orchestration for Discord-facing behavior, including the primary slash-command workflow
- the control API and complementary dashboard-serving layer
- canonical config and runtime state
- Postgres-backed persistence and migrations
- the embedded React dashboard under `ui/`
- the `//go:embed` payload served by the control plane

## Product Interaction Model

`discordcore` is slash-commands-first.

Default product split:

- slash commands are the primary end-user interface for bot capabilities, routine actions, and conversational flows
- the dashboard is a complementary surface for setup, review, bulk edits, diagnostics, and recovery flows that are awkward in chat
- do not make a dashboard page the canonical or exclusive path for routine bot actions unless the product owner explicitly asks for that tradeoff
- when shaping a feature, define the slash-command workflow first, then add only the UI needed to support it cleanly
- if a page mainly mirrors command usage, command output, or routine action buttons, compress it, move it behind diagnostics, or remove it

## Boundaries And Ownership

Use this map before editing:

- `cmd/discordmain/`: principal main runtime entrypoint
- `pkg/app/`: runtime orchestration and startup wiring
- `pkg/control/`: control API, auth/session handling, dashboard serving, guild/settings/feature routes for the complementary web surface
- `pkg/files/`: canonical config model, normalization, persistence adapters, and `ConfigManager`
- `pkg/discord/`: Discord runtime behavior, commands, logging, cache, services, session handling; primary user-facing command workflows
- `pkg/storage/`: durable Postgres-backed domain storage
- `pkg/persistence/`: DB open, ping, and migrations
- `pkg/partners/`: partner board rendering and sync helpers
- `pkg/task/`: task router and scheduled/background jobs
- `ui/src/app/`: routes, navigation registry, app-level routing helpers
- `ui/src/api/control.ts`: canonical dashboard API contracts and client behavior
- `ui/src/context/`: dashboard session, guild selection, login/logout, base URL handling
- `ui/src/features/features/`: shared feature-area adapters and workspace hooks
- `ui/src/features/partner-board/`: self-contained partner board workflow
- `ui/src/pages/`: route-level page surfaces
- `ui/dist/`: embedded build output; `index.html` placeholder must remain present

Respect package ownership:

- do not move reusable logic into `cmd/`
- do not reimplement config or state rules in `pkg/control/` when they belong in `pkg/files/`
- do not move Discord runtime behavior into the dashboard layer
- keep routine user workflows in Discord when a slash command is the natural surface
- use the dashboard for setup, review, bulk edits, diagnostics, and cross-feature visibility rather than as a duplicate command console
- keep router and provider entrypoints thin when a focused sibling file or service already exists
- `pkg/files/` functions must isolate side-effects via explicit dependency injection. Operational logging and telemetry are permitted exclusively via `slog.Logger` or metrics accessors passed explicitly during struct initialization or extracted from `context.Context`. Direct usage of global state (`slog.Default()`) or asynchronous I/O remains prohibited to ensure deterministic testability.

## Universal Working Style

Work in this order unless the task clearly calls for something narrower:

1. identify the exact contract, workflow, or failure being changed
2. inspect the minimum live source needed to find the owning seam
3. make the smallest change that preserves local ownership and patterns
4. update nearby tests that pin the changed behavior
5. run the matching validation before considering cleanup

General expectations:

- prefer source code and current tests over stale docs
- build context locally around the owning abstraction, not by mapping the whole repo
- preserve public APIs and established naming unless the task requires change
- do not introduce speculative architecture or “future-proofing” abstractions
- if a hotspot file must change, read enough neighboring code to preserve its decomposition rules first

## Code Commentary

Go treats comments as active, first-class toolchain directives rather than passive metadata. Annotations must directly instruct the compiler via `go tool` directives, generate semantic HTML documentation via `godoc`, or isolate non-obvious business logic decisions. Redundant mechanical descriptions are treated as structural failures.

Doc-comment baseline for Go:

- **API Documentation**: Target exported symbols with complete sentences ending in a period. Begin the comment exactly with the symbol's name to ensure correct `godoc` indexing. Place `//` line comments directly preceding and adjacent to the declaration with no blank line between; detachment causes silent parser failures.
- **Summary Extraction**: Formulate the first sentence of any package-level or symbol-level comment as a standalone summary. `godoc` extracts this initial sentence for high-level directory indices. Describe behavior, not just the signature.
- **Compiler Directives**: Use strict `//go:` prefixes for file-level compilation constraints and memory optimizations. Modern parsers mandate `//go:build` coupled with boolean expressions; utilizing the legacy `// +build` syntax risks compilation divergence and is prohibited. Deploy `//go:noescape` and `//go:linkname` directives to manipulate compiler heap-escape analysis and linker visibility during low-level performance optimization.
- **C Interoperability**: Place `#cgo` directives strictly in the preamble before `import "C"` to dictate C compiler and linker flags directly within the Go file.
- **Inline Annotations**: Provide contextual explanations of performance optimizations, security nuances, or ignored errors. This prevents subsequent maintainers from inadvertently removing load-bearing mitigations. Do not duplicate the mechanical function of the code; state duplication reliably drifts into falsehoods as the AST mutates.
- **Block Comments**: Reserve block comments `/* ... */` exclusively for large package-level documentation strings within a dedicated `doc.go` file or for column-specific syntax alignment. Utilize `//` for all other operational context.
- **Formatting**: Execute `gofmt` consistently to mechanically standardize comment indentation and alignment against adjacent code blocks.
- **Contract Explicitly**: Document non-obvious contracts: concurrency guarantees, returned-error semantics (`errors.Is` branches), ownership of returned values, and lifecycle ordering. Use `Deprecated:` as a structured marker. Multi-paragraph contract docs with section headers (`# Contract`, `# Parameters`) are the local idiom for load-bearing seams.

TypeScript and UI: types in `ui/src/api/control.ts` and component prop interfaces are the contract; add JSDoc only when behavior cannot be expressed in the type (units, side effects, lifecycle ordering).

Across both:

- `TODO`/`FIXME` markers carry an owner (`TODO(user): …`) or a specific triggering event; ownerless `TODO:` is an anti-pattern
- do not preserve removed code as commented-out blocks, `// removed` markers, or `_unused` placeholders — git history is the museum
- do not write comments that restate the identifier or the next line; do not narrate straight-line code step by step
- do not add or rewrite comments on code that was not changed in the current task; comment churn pollutes blame
- do not embed history references ("added for ticket X", "renamed in 2024") — those belong in commit messages

## Human-AI Contract

- **Tone & Style**: Strictly neutral, dense, technical tone. No pleasantries, emojis, or conversational filler. Bold vital technical identifiers. Expect direct instructions, occasionally in Portuguese.
- **Fast-Track Approvals**: If the user provides a raw snippet and says "approved" or "move forward", bypass formal planning and execute immediately.
- **Scope Execution**: Do exactly what the user requested. Nothing more, nothing less. Do not bundle adjacent improvements unless it's a requested refactor.
- **Mid-Flight Context Pivots (User Overrides)**: Explicit user instructions (e.g., "ignore backwards compatibility here") instantly override standard repository rules for the duration of that task.
- **Burden of Proof**: When asked to prove assertions, do not rely on narrative explanation. Provide exact code paths, test execution outputs, or localized dry-run logs to mathematically guarantee the behavior.
- **Raw Output Ingestion**: Immediately parse and action raw compiler logs, profiler outputs, or JSON configs without asking for redundant clarification. Targeted patching of specific JSON states via the agent is a supported operational workflow.
- **Proactive Tooling**: Automatically run `go vet` or `go build -gcflags="-m"` during verification if the task involves performance overhead or structural refactoring.
- **Objectivity & Pushback**: Enforce steelmanning of competing patterns without subjective endorsement. Explicitly object to flawed premises or suboptimal paths before executing. Guide to the optimal path.
- **Binary Certainty**: No hedging or probabilistic language. No silent placeholders (`// ... existing code ...`). All code must be complete.
- **Authorized Refactors**: While scope execution is strict, *proposing* high-value authorized refactors (see Authorized Refactor Classes below) separately after completing the requested work is an explicit exception.

## Design And Implementation Rules

When docs and source disagree:

- trust source code and current tests first
- treat `README.md` and `UI_RULES.md` as intent and context, not implementation truth
- update stale docs instead of forcing code to match outdated prose

General architectural rules:

- prefer small, reversible, testable changes
- reuse existing seams before introducing new helpers or layers
- keep new public types, exported functions, routes, and settings to a minimum
- extend the closest existing sibling file before creating a new one
- do not add `util`, `helper`, or `common` packages; use the closest owning package or sibling seam
- when a wide service needs splitting, prefer narrow consumer-side interfaces over concrete implementation fragmentation
- high-drift decisions (broad rewrites or large diffs) are acceptable IF and ONLY IF they significantly elevate the code quality compared to before.

Go core idioms (strictly enforced deviations from standard flexibility):

- **interface{} says nothing**: Avoid `any` or `interface{}`. Use strong typing. If forced to use `any`, document exactly why compile-time safety was impossible.
- **No Shared util Packages**: Duplicate small snippets instead of tangling unrelated domains (reinforces the "no util packages" rule).
- **Don't panic**: Never use `panic` in business logic. Always return `error`. Reserve `panic` exclusively for unreachable states or `init()` failures.
- **Concurrency Guardrails**: Pass data over channels or use single-threaded event loops. If using `sync.Mutex`, keep the critical section small and never perform I/O while holding a lock.

Go and backend rules:

- put `context.Context` first when present
- prefer `:=` for new values and `=` when reusing outer `ctx` or `err`
- wrap inspectable errors with `fmt.Errorf("operation: %w", err)`
- use sentinel errors and `errors.Is` / `errors.As` when callers branch on failure
- **Logging Guidelines**:
  - `slog` is the canonical logging library. Routing continues through `pkg/log` or area-specific helpers, not stdlib `log`.
  - **Debug**: Granular transient state inspection. Emits full request payloads, generated SQL query dumps, and complex conditional branch tracking. Must remain inactive in production via environment variables to prevent throughput degradation and I/O operation saturation.
  - **Info**: Baseline operational telemetry. Emits exclusively architectural state transitions, such as primary routine initialization, socket port binding, and planned instance shutdown.
  - **Warn**: Intercepted and mitigated service degradation. Emits when a failure does not compromise the main data flow. Examples include response time exceedance triggering compensatory retry logic, local cache configuration loads post-disconnection, or API rate limit consumption operating at **80%**.
  - **Error**: Blocking structural failure restricted to the scope of the ongoing operation or transaction. Emits structural metadata obligatorily containing unique request identifiers, the injected stack trace dump, and a synthetic failure identifier corresponding to HTTP **500**. Immediately triggers alerts in external aggregators and structured on-call paging systems.
  - Include relevant guild, channel, or user identifiers in operational logs.
  - Never log secrets, OAuth credentials, tokens, or private message content.
- validate only at system boundaries: HTTP input, OAuth callbacks, Discord payloads, and external rows or documents
- treat `ConfigManager.Config()` and `GuildConfig()` results as read-only snapshots; persist through the existing update helpers

Dashboard and TypeScript rules:

- keep dashboard request and response shapes in `ui/src/api/control.ts`
- keep route strings and legacy aliasing in `ui/src/app/routes.ts`
- keep navigation registry data in `ui/src/app/navigation.ts`
- use `DashboardSessionContext` and existing feature hooks before adding ad hoc fetch logic
- prefer existing primitives such as `PageHeader`, `FeatureWorkspaceLayout`, `SurfaceCard`, `StatusBadge`, and `EmptyState`
- avoid `any`
- use `import.meta.env.BASE_URL` for embedded asset paths

Canonical dashboard contracts:

- canonical dashboard base path is `/manage/`
- `/dashboard/` is a legacy compatibility alias, not the primary route
- backend SPA handling lives in `pkg/control/dashboard_handler.go`
- dashboard HTTP registration lives in `pkg/control/http_routes.go`
- route construction and legacy mapping live in `ui/src/app/routes.ts`
- Vite build base lives in `ui/vite.config.ts`
- the SPA must not intercept `/v1/*` or `/auth/*`

If dashboard routing changes, update backend, frontend, tests, docs, and embed assumptions together.

### Structural Mandates (State-of-the-Art Architecture)

- **Graceful Lifecycle Management**: Mandate context-aware cancellation pipelines utilizing `context.Context` and `sync.WaitGroup` to orchestrate isolated sub-service teardowns safely. Avoid serializing runtime states via `sync.RWMutex` which introduces write-starvation and deadlock vulnerabilities.
- **TypeScript API Resiliency**: Enforce mandatory exponential backoff and randomized network jitter on all retry mechanisms for HTTP 502/504 errors to prevent thundering herd state collapses.
- **Observability Accessors**: Mandate strict dependency validation during the application boot phase to ensure the metrics pipeline successfully attaches prior to the primary event loop. A failure to attach the pipeline prior to the primary event loop must trigger a fatal runtime abort, ensuring nil-safe accessors (`NopMetrics`) do not mask deployment anomalies.

### Go Modernization (up to 1.26)
- replace custom stateful iterators, channel-based generators, and slice-allocating batch retrievals with `iter.Seq` and `iter.Seq2` (Go 1.23) using native `for ... range` loops to eliminate intermediate heap allocations. Specifically, transition Postgres batch fetches in `pkg/storage/` (e.g., returning `[]Event`) to stream directly from `sql.Rows` iteration, eliding the intermediate slice allocation.
- transition boundary-crossing wrapper structs to generic type aliases (`type Alias[T any] = OriginalType[T]`, Go 1.24) to eradicate redundant allocation layers.
- replace `runtime.SetFinalizer` with `runtime.AddCleanup` (Go 1.24) to enforce deterministic memory reclamation.
- consolidate multi-line primitive pointer assignments into single-line initializations (e.g., `new(int64(300))`, Go 1.26).
When executing any modernization item above, update all affected callers, tests, and doc comments
in the same atomic change. Do not leave the repo in a partial migration state. If the scope exceeds
what can be validated in one `go test ./...` pass, split by subsystem (e.g., `pkg/storage/` first,
then `pkg/discord/`), each split must be independently buildable and passing.

## Authorized Refactor Classes

These categories of change may have high diff counts and cross package boundaries. Each must
be proposed explicitly before execution unless the user's message directly requests it. Once
approved, execute atomically — do not leave callers, tests, or shims in an intermediate state.

Qualifying criteria (ALL must hold):
- the result is strictly better than the current code by at least one of: correctness, allocations, 
  error handling completeness, or removal of technical debt
- no speculative abstractions, future-proofing layers, or behavioral changes are introduced
- the pattern already exists in the repo or is explicitly listed in Go Modernization below
- every affected caller, test, and doc comment is updated in the same change

Authorized classes:
- **iter.Seq / iter.Seq2 migration**: batch-returning functions in `pkg/storage/` and `pkg/discord/`
  that allocate intermediate slices may be rewritten to return `iter.Seq` / `iter.Seq2` with 
  streaming `sql.Rows` iteration. All callers must be migrated in the same commit.
- **Dead code removal**: unexported functions, types, or constants with no live callers (confirmed
  by `grep` + `gopls references`, not assumed) may be removed together with their tests.
- **Pointer helper consolidation**: `varOf`-style helpers matching the `func f(x T) *T { return &x }`
  pattern may be replaced with `new(expr)` (Go 1.26) at all call sites in one pass.
- **SetFinalizer → AddCleanup**: `runtime.SetFinalizer` calls replaced with `runtime.AddCleanup`
  (Go 1.24). Each replacement must preserve the original lifecycle semantics exactly.
- **Generic type alias consolidation**: wrapper structs that exist solely to carry a generic type 
  across a package boundary may be replaced with parameterized aliases where the types package
  already has the definition.
- **sync.Map → sync.Map (1.24 HAMT)**: no code change needed; but if code works around the 
  old dual-map contention profile (explicit sharding, manual dirty-map patterns), those workarounds
  may be removed if benchmarks confirm the 1.24 implementation makes them redundant.

Prohibited regardless of class:
- changes that alter observable behavior at API or Discord-facing boundaries
- changes that touch `pkg/files/types.go` field shapes without following Config Schema Evolution Pattern
- changes that widen hotspot files listed in Hotspots And Cautions
- changes motivated by "this would be cleaner" without a concrete measurable improvement

## Config Schema Evolution Pattern

When a persisted config field changes shape:

1. add the new field on the public struct in `pkg/files/types.go`
2. keep the legacy JSON key alive only inside the local `raw*` unmarshal struct
3. migrate legacy to canonical at decode time; do not emit the legacy key on the write path
4. update cloning, normalization, and `IsZero` logic together
5. update all callers and tests in the same change; do not leave shim-then-cleanup work behind

- state mutations must implement Optimistic Concurrency Control: payloads omitting `config_version` must be rejected, and UI clients must implement mandatory exponential backoff and state-refresh I/O loops specifically targeting HTTP 412 and 428 rejection codes to prevent thundering herd overwrites
- cross-boundary synchronization mandates the integration of an AST parsing or schema generation step to mechanically guarantee API contract fidelity during schema evolution, replacing manual synchronization between `pkg/files/types.go` and `ui/src/api/control.ts` to eliminate structural drift vulnerability

## Testing And Validation

Testing style:

- keep tests close to the changed package
- prefer deterministic seams already present in the repo
- do not require live Discord access
- use isolated Postgres helpers only where the package already follows that pattern
- agents may use `postgres://alice:Cpu7zyuwBKdEtcq1QnBg@127.0.0.1:5432/alicemainsdevelopment?sslmode=disable` as a test database for stress testing and validation when necessary.
- mock the narrowest interface the unit under test actually exercises
- prefer pure unit tests for math, lifecycle calculations, and error classification
- keep failures in the current test or subtest when practical; helpers should return errors or diffs instead of failing the test themselves
- never call `t.Fatal`, `t.FailNow`, or similar methods from spawned goroutines
- authorized refactor commits use `refactor(<scope>): <description>` subject; if the refactor
  spans multiple packages, list the primary owning package as scope
- a refactor commit must not mix behavioral changes with structural ones; if a behavior fix is
  needed as part of the refactor, it goes in a preceding `fix:` commit

Validation expectations:

- backend: `go test ./...` and `go vet ./...`
- UI: `bun run test`, `bun run lint`, and `bun run build`
- formatting/EOL: `release validate` (must pass without output; new files must be LF)
- route/embed changes: verify `ui/vite.config.ts`, routes, and that `ui/dist/index.html` remains
- APIs/docs/errors: update nearby tests and doc comments that pin the new behavior
- skipped validation: if a step was skipped, state it explicitly

Build and release commands:

- `go build ./...`
- `go test ./...`
- `go vet ./...`
- `go test -tags integration ./...`
- from `ui/`: `bun run test`, `bun run lint`, `bun run build`
- `release validate` (runs gofmt, EOL drift, go vet, ui lint, and fast unit tests like `go test ./...` without committing)
- `release verify` (runs high-latency environment-dependent integration tests like `go test -tags integration ./...` and `govulncheck`)
- canonical release command: `release -m "<conventional commit subject>" -y --promote`

Release rules:

- do not push to `main` directly
- do not call `git tag` by hand
- do not bundle unrelated changes into a single release commit
- the automated release orchestrator will automatically patch `pkg/files/version.go`; do not mutate version constants manually
- restrict line-ending or encoding normalization commits to that scope so the diff stays trivial to audit; stylistic reformatting (e.g., gofmt-equivalent one-liner to multi-line conversions) goes in a separate `style:` or `refactor(style):` commit
- treat the conventional-commit subject as the release message and changelog entry

## Hotspots And Cautions

These files need extra neighborhood reading before edits:

- `pkg/discord/logging/monitoring.go`
- `pkg/storage/postgres_store.go`
- `pkg/control/features_routes.go`
- `pkg/control/discord_oauth.go`
- `pkg/files/types.go`
- `pkg/app/runner.go`
- `ui/src/api/control.ts`
- `ui/src/context/DashboardSessionContext.tsx`
- `ui/src/pages/RolesPage.tsx`
- `ui/src/pages/ModerationPage.tsx`

These seams are intentional. Keep them decomposed:

- `pkg/control/features_routes.go`: router and dispatch only; feature catalog, workspace shaping, readiness, blockers, patch flows, and toggle bindings belong in focused services or `features_*.go` siblings
- `pkg/control/discord_oauth.go`: shared OAuth types, constants, provider construction, and permission parsing only; flow logic belongs in `discord_oauth_*.go` siblings or the dedicated service
- `pkg/storage/postgres_store.go`: `Store` type, bootstrap, schema init entrypoint, and shared SQL helpers only; domain behavior belongs in focused `postgres_store_*.go` files
- `pkg/discord/logging/monitoring.go`: lifecycle and orchestration only; gateway handlers, reactions, cache loops, permission mirroring, and similar specifics belong in focused `monitoring_*.go` files

Additional caution:

- `pkg/discord/logging/` is coordination-heavy; avoid regrowing broad orchestration files when a sibling seam exists
- QOTD legacy compatibility is concentrated in specific migration and compatibility seams, not across the whole runtime
- do not widen already large pages unless the task is truly local to that page

## Load-Bearing Invariants

### Generic Bot Paradigm and Identity

- the system operates on a "generic bot" paradigm rather than maintaining a "primary" bot. Guilds that have only one bot treat it equally as a generic bot.
- the magic blank string `""` represents a "generic bot identity". It serves strictly as a fallback for guilds that do not have explicitly configured bot tokens.
- `""` must not force itself into being a dispatcher or catch-all for all guilds.
- unrouted features (features lacking a configured `botInstanceID`) must resolve to an `<unrouted>` sentinel rather than `""` to safely prevent the generic bot from assuming ownership of disabled or unconfigured features.
- decoupling of subsystems (e.g., stats) from Discord logic must respect routing bounds: features must not execute without proper routing, enforcing strict separation between core system logic and discord-facing services.

### Bot Identity Resolution

- runtime identity assignment must evaluate FeatureRouting from a single deterministic seam before executing a fallback to the primary bot ID, establishing the absolute source of truth in multi-bot deployments
- context propagation must enforce this resolved identity strictly across all asynchronous execution boundaries

### Sentinel Safety & Null State Representation

- The explicit use of string sentinels (like `<unrouted>`) over `""` must be enforced to represent explicitly disabled or disconnected states in mapping dictionaries.
- This mitigates the risk of empty strings naturally acting as wildcard catch-alls in conditional loops or routing lookups.

### Discord Logic Decoupling (System Boundaries)

- Features like Stats, QOTD, or Monitoring must remain functionally decoupled from the raw Discord orchestration logic (e.g., `pkg/discord`).
- Core system state changes or feature toggles must not bind directly to Discord gateway events without passing through an agnostic routing interface or event handler. Core logic should be invokable and testable without a live `discordgo.Session`.

### Idempotency & Retry Mechanisms for Background Tasks

- The application relies on `pkg/task/router.go` utilizing a locking `inflight` map for idempotency tracking, alongside a heap-based retry queue.
- Operations interfacing with external APIs or DB writes must push tasks to this queue rather than spinning up raw, unmanaged `goroutines`. This protects against race conditions, thundering herd state collapses, and duplicate executions.

### Multi-Tenant State Refresh Isolation

- Operations targeting a single guild must never invoke a full system-wide `ConfigManager` state reload or global UI refresh unless strictly changing a repository-wide schema parameter.
- Agents must only target local `GuildConfig` segments, reducing memory consumption and blocking time on the backend.

### QOTD subsystem

- publish idempotency is three-layered: a 16-hex `Nonce` on `QOTDOfficialPostRecord` sent to Discord with `enforce_nonce`, a partial unique index on `(guild_id, deck_id, publish_date_utc, publish_mode='scheduled')`, and the `resolvePublishNowConflict` / adopt-existing-thread recovery branches; new publish paths must keep all three intact
- `OfficialPostState` distinguishes `failed` (retryable, reconcile loop retries) from `abandoned` (terminal, requires admin action); `isUnrecoverableDiscordPublishError` is the gate between them
- thread-state errors classify through three buckets: `isMissingDiscordThreadError`, `isUnmanageableDiscordThreadError`, and the retryable default; new Discord error codes belong in one of those classifiers, not an ad hoc branch
- `syncLiveOfficialPost` short-circuits when `post.State == lifecycle.State`; reconcile-style code added later must follow the same skip-the-API-call pattern
- official-post transitions that touch both Discord and the DB must route through `applyOfficialPostThreadTransition` in `pkg/qotd/official_post_state_transition.go`; do not inline Discord-first plus DB-second state changes elsewhere
- observability lives behind the `qotd.Metrics` interface in `pkg/qotd/observability.go`; instrument through typed interface methods, not direct Prometheus or expvar calls
- Discord thread auto-archive is `defaultThreadAutoArchiveMinutes` with `fallbackThreadAutoArchiveMinutes` on validation rejection; `archiveOfficialPost` should only flip `Locked=true`
- suppression is `QOTDConfig.SuppressScheduledPublishDatesUTC []string`; the legacy single-date JSON key only survives in the local unmarshal shim

### QOTD runtime lifecycle

- `pkg/discord/qotd/RuntimeService` must reinitialize `stopCh` and `stopOnce` on `Start` after a prior `Stop`, or restart will silently exit after the startup cycle
- stopping the QOTD runtime should cancel in-flight per-guild operations and suppress expected context-canceled shutdown noise
- stale scheduled-publish suppression tokens should be trimmed automatically so config does not drift conservative forever

### Reaction block runtime

- reaction block enforcement lives in `pkg/discord/logging/ReactionEventService`, ahead of metrics-store and emit-policy gating
- `resolveMonitoringWorkloadState` must keep `reactionEventService` enabled when any guild has non-empty `GuildConfig.ReactionBlocks`, even if reaction logs are disabled
- blocked reaction removal uses `discordgo.Session.MessageReactionRemove` with unicode emoji names for built-ins and custom emoji IDs for guild emoji

## Reporting And Handoffs

Keep reports factual and load-bearing.

- lead with what changed and why when the work is substantial
- use `walkthrough.md` to document changes and validation results instead of narrating edit history in the chat
- include validation run and any skipped validation
- mention remaining risk or follow-up drift only when it is real
- keep unrelated findings separate from the requested work
