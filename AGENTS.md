# discordcore GPT-5.4 Guide

Operational instructions for GPT-5.4 Xhigh working in `discordcore`.

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

- the Go runtime and orchestration for Discord-facing behavior
- the control API and dashboard-serving layer
- canonical config and runtime state
- Postgres-backed persistence and migrations
- the embedded React dashboard under `ui/`
- the `//go:embed` payload served by the control plane

Sibling repo:

- `../alicebot` owns host bootstrap, machine-specific wiring, and final runtime packaging

Boundary rule:

- reusable product logic, state rules, contracts, and shared workflows belong in `discordcore`
- host-specific setup belongs in `alicebot`

## Boundaries And Ownership

Use this map before editing:

- `cmd/discordcore/`: example runner only, not reusable product logic
- `pkg/app/`: runtime orchestration and startup wiring
- `pkg/control/`: control API, auth/session handling, dashboard serving, guild/settings/feature routes
- `pkg/files/`: canonical config model, normalization, persistence adapters, and `ConfigManager`
- `pkg/discord/`: Discord runtime behavior, commands, logging, cache, services, session handling
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
- keep router and provider entrypoints thin when a focused sibling file or service already exists

## How To Work Well In This Repo With GPT-5.4 Xhigh

GPT-5.4 Xhigh is most useful here when the task is explicit, bounded, and validated against live source.

Work in this order:

1. identify the exact contract, workflow, or failure being changed
2. use `smallfrappuccino-mcp` first for orientation
3. inspect the minimum relevant files, then verify the live source before editing
4. make the smallest change that preserves local ownership and patterns
5. update nearby tests that pin the changed behavior
6. run the matching validation before considering cleanup

Recommended discovery flow:

1. `repo_overview`
2. `reindex` only if the index is stale, suspicious, or incomplete
3. `repo_audit`
4. `contract_checks` before route, dashboard-boundary, or feature-contract work
5. `task_context`
6. `find_nodes` to locate the exact package, route, feature, page, or symbol
7. `get_subgraph` before editing hotspot files or changing boundaries
8. `list_observations` before touching sensitive areas

MCP narrows the search space. It does not replace reading the live source.

Inspect narrowly when:

- the task is confined to one route, one feature, one hook, one settings section, or one page
- MCP already identified the relevant package or file cluster
- the change extends an existing local pattern

Inspect broadly when:

- changing backend and UI contracts together
- changing config or state semantics in `pkg/files/`
- changing auth, routing, dashboard mount behavior, or embed behavior
- changing feature IDs, editable fields, or workspace shapes
- touching a hotspot file

Do not start by reading all of `pkg/control/`, `pkg/discord/logging/`, or `ui/src/pages/`.

## Preferred Strengths For This Repo

Use GPT-5.4 Xhigh preferentially for:

- multi-file implementation when the contract and ownership boundaries are already known
- writing or tightening tests near the changed package
- iterative debugging with a read-change-validate loop
- contract-guided refactors that stay inside existing seams
- disciplined execution of well-defined tasks with explicit validation

This model should add value by connecting existing local patterns across files, not by inventing a new architecture.

## Risks And Countermeasures

Common failure modes for this model in `discordcore`:

- over-engineering a local fix into a framework change
- adding wrappers, helpers, or abstractions that do not remove real complexity
- widening a small task into a broad refactor
- making speculative changes outside the stated bug or contract
- growing public surface area when internal changes would suffice
- mixing cleanup into a behavior fix when the cleanup is not required for safety

Countermeasures:

- prefer small, reversible, testable changes
- reuse existing seams before introducing new helpers or layers
- keep new public types, exported functions, routes, and settings to a minimum
- verify that each added abstraction removes a repeated or unstable burden, not just local discomfort
- do not combine unrelated refactors with a bug fix unless the refactor is required to make the change safe
- if a hotspot file must change, confirm the surrounding live source before patching and keep the edit tightly scoped
- if the repo already has a local primitive, hook, service, or builder for the job, extend it instead of creating a parallel path

## Recommended Strategy By Task Type

For bug fixes:

1. reproduce or pin the failing behavior
2. locate the smallest owning seam
3. fix the root cause without broad cleanup
4. add or adjust tests near the changed package
5. run the narrowest relevant validation, then broader checks if the surface warrants it

For multi-file feature work:

1. identify the owning contract first
2. trace all required layers before editing
3. update backend and UI together when the contract crosses both
4. keep route, feature, and config contracts centralized
5. verify the exact files that define the contract before editing dependents

For refactors:

1. use refactors to clarify an existing seam, not to redesign the area
2. preserve behavior first, then tighten tests
3. prefer extracting within an existing sibling-file pattern over creating new architectural layers
4. stop if the change starts widening beyond the original contract or workflow

For debugging:

1. use iterative validation, not speculative rewrites
2. keep observations tied to concrete files, tests, and runtime behavior
3. re-check live source before each edit in hot or drifting areas

## Editing And Validation Rules

When docs and source disagree:

- trust source code and current tests first
- treat `README.md` and `UI_RULES.md` as intent and context, not implementation truth
- update stale docs instead of forcing code to match outdated prose

For dashboard implementation details, the live source of truth is:

- `ui/src/index.css`
- `ui/src/shell.css`
- `ui/src/components/ui.tsx`

Canonical contracts:

- canonical dashboard base path is `/manage/`
- `/dashboard/` is a legacy compatibility alias, not the primary route
- backend SPA handling lives in `pkg/control/dashboard_handler.go`
- dashboard HTTP registration lives in `pkg/control/http_routes.go`
- route construction and legacy mapping live in `ui/src/app/routes.ts`
- Vite build base lives in `ui/vite.config.ts`
- the SPA must not intercept `/v1/*` or `/auth/*`

If dashboard routing changes, update backend, frontend, tests, docs, and embed assumptions together.

For Go code in `pkg/`, `cmd/`, and `ui/*.go`:

- name functions, methods, packages, and test doubles from the call site: avoid repeating package, receiver, type, or parameter names; avoid `Get` prefixes for getters; prefer noun-like accessors and verb-like mutators; avoid new `util`, `helper`, or `common` package names
- keep packages cohesive and files focused; extend existing sibling seams first, but split by responsibility before a file becomes an unsearchable hotspot
- prefer `:=` for new non-zero values; use zero values or `new(T)` when an empty value is the point; initialize maps before writes; keep pointer receivers on types that contain mutexes or other no-copy fields
- avoid accidental shadowing, especially for `ctx`, `err`, and imported package names; when reusing an outer variable across a nested scope, use `=` instead of `:=`
- keep exported APIs readable: put `context.Context` first, do not hide it in options, avoid long parameter lists, and prefer option structs or ordered functional options only when they materially improve common call sites
- prefer explicit dependency passing and instance types over new mutable package globals; package-level convenience wrappers, if unavoidable, must stay thin and must not be the only API that library code relies on
- avoid unnecessary interfaces; define small consumer-side interfaces only when multiple implementations, real substitution, or import-cycle pressure justify them; accept interfaces and usually return concrete types
- specify channel direction where ownership is one-way, and prefer real transports or generated clients over hand-rolled RPC or HTTP stand-ins when testing integrations
- update doc comments when exported behavior changes, especially for concurrency, cleanup, context, or returned-error contracts; document the non-obvious behavior, not every parameter name

Backend rules:

- prefer explicit error propagation
- wrap errors with operation context when callers should still be able to inspect the underlying failure using `fmt.Errorf("operation: %w", err)`; use `%v` when translating or intentionally hiding internals at a boundary
- prefer structured or sentinel errors when callers need to branch on failure; do not string-match on error text
- avoid `panic` for expected runtime failures; reserve panic or fatal termination for unrecoverable invariants or top-level startup failures
- return early on bad auth, invalid input, and missing dependencies
- use existing repo logging facilities, especially `pkg/log` and area-specific helpers
- avoid logging and returning the same error unless the local log is the only actionable sink
- gate expensive debug logging behind cheap conditions or verbosity checks
- include relevant guild, channel, or user identifiers in operational context
- never log secrets, tokens, OAuth credentials, or private message content
- prefer `+` for simple string joins, `fmt.Sprintf` or `fmt.Fprintf` for formatted output, and `strings.Builder` for piecemeal string construction
- treat `ConfigManager.Config()` and `GuildConfig()` results as read-only snapshots
- persist through `UpdateConfig`, `UpdateRuntimeConfig`, or existing helpers
- preserve normalization and validation when changing config semantics
- if config semantics change, update normalization, persistence, route handling, and tests together

Dashboard and TypeScript rules:

- keep dashboard request and response shapes in `ui/src/api/control.ts`
- keep route strings and legacy mapping in `ui/src/app/routes.ts`
- keep navigation registry data in `ui/src/app/navigation.ts`
- keep shared feature interpretation in `ui/src/features/features/`
- keep partner-board-specific logic in `ui/src/features/partner-board/`
- avoid `any`
- avoid duplicating contract types inside page files
- use `DashboardSessionContext` and existing feature hooks before adding ad hoc fetch logic

When a backend contract changes, update:

1. `ui/src/api/control.ts`
2. the feature adapters or pages that consume it
3. tests that pin the behavior

UI semantics:

- preserve the compact operational dashboard style already present in the app
- prefer existing primitives such as `PageHeader`, `FeatureWorkspaceLayout`, `SurfaceCard`, `StatusBadge`, and `EmptyState`
- use human-facing labels instead of raw internal enum or storage names unless the page is explicitly diagnostic
- use `import.meta.env.BASE_URL` for embedded asset paths
- default settings-style pages to direct controls with minimal helper text
- keep standard dashboard pages free of provenance noise, repeated state badges, raw IDs, or fallback-by-ID editors unless the screen is explicitly diagnostic
- gate low-level diagnostic metadata behind explicit diagnostic UI instead of exposing it in the default flow
- keep adjacent settings in grouped surfaces with internal row dividers when they belong to the same workflow
- keep expanded child controls inside the same parent setting group

Testing style:

- keep tests close to the changed package
- prefer deterministic seams already present in the repo
- do not require live Discord access
- only use isolated Postgres helpers where the package already follows that pattern
- keep failures in the `Test` or subtest when possible; shared validators should return `error`, diffs, or `cmp.Option` rather than failing the test themselves
- mark setup and cleanup helpers with `t.Helper()` and use `t.Fatal` there only for unrecoverable test-environment failures
- prefer `t.Error` plus `continue` to keep table tests going; use `t.Fatal` when setup failure prevents the current test or subtest from continuing
- never call `t.Fatal`, `t.FailNow`, or similar methods from spawned goroutines
- use field names in larger table-driven test case literals
- scope expensive setup to the tests that need it; use `sync.Once` only for shared setup with no teardown, and `TestMain` only when package-wide setup and teardown are genuinely required
- prefer real transports for HTTP or RPC integration tests when practical

Validation expectations:

- backend changes: `go test ./...` and `go vet ./...`
- UI changes: `bun run test`, `bun run lint`, and `bun run build`
- route or embed contract changes: verify `ui/vite.config.ts`, `ui/src/app/routes.ts`, `pkg/control/http_routes.go`, `pkg/control/dashboard_handler.go`, and that `ui/dist/index.html` still exists
- feature or settings contract changes: verify Go route and workspace builders, `ui/src/api/control.ts`, and the adapters or pages consuming the changed fields
- exported Go API, doc, or error-contract changes: update nearby tests and doc comments that pin the new behavior
- if a relevant validation step was not run, say so explicitly

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

- `pkg/discord/logging/` contains broader patchwork than a single file; treat the area as coordination-heavy
- QOTD legacy compatibility is comparatively concentrated in migration and compatibility seams, not spread across the whole feature runtime
- avoid regrowing a hotspot root file when an existing sibling seam can absorb the change
- do not widen already large pages unless the task is truly local to that page

## How To Report Work

Substantial work reports should include:

- problem summary
- files modified
- previous behavior
- new behavior
- validation run
- remaining risks or follow-up drift

Keep this document durable. Do not fill it with task-local plans, prompt boilerplate, vendor rituals, or one-off implementation notes.
