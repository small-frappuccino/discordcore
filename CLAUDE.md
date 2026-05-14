# discordcore Claude Guide

Operational instructions for Claude working in `discordcore`.

This file is the Claude-centric companion to `AGENTS.md`. `AGENTS.md` carries the cross-agent repository contract for any coding agent working in this repo. This file preserves the deeper Claude workflow, MCP discipline, and reporting conventions that have proven effective here. When the two documents overlap, keep durable repository facts in sync and treat `AGENTS.md` as the baseline contract.

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
- `cmd/discordqotd/`: principal QOTD runtime entrypoint
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

## How To Work Well In This Repo With Claude

Claude is most useful here when the task is explicit, bounded, and validated against live source.

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

## Context-Mode MCP Discipline

`context-mode` is registered as an MCP plugin in this environment (`.claude/settings.local.json`, `~/.claude/plugins/cache/context-mode/context-mode/<version>/cli.bundle.mjs`). It is complementary to `smallfrappuccino-mcp` (`.vscode/mcp.json`), not a replacement: `smallfrappuccino-mcp` answers graph-level orientation questions (`repo_overview`, `find_nodes`, `get_subgraph`, `task_context`, `list_observations`); `context-mode` runs commands, analyses, and external fetches inside a sandbox so raw output never floods the context window.

Always route discovery, analysis, validation, and large-output work through `context-mode` before and during code edits. Treat raw `Bash`/`PowerShell` output as a cost the policy is paying to avoid.

Required tools when editing code in this repo:

- discovery and command runs: `mcp__plugin_context-mode_context-mode__ctx_batch_execute(commands, queries)`; pass labeled commands plus follow-up queries in one call so the indexed chunk titles are descriptive
- follow-up lookups across indexed chunks: `mcp__plugin_context-mode_context-mode__ctx_search(queries)`; many queries per call, and after a resume lead with `sort: "timeline"` to recover prior session context
- analysis or processing of large files: `mcp__plugin_context-mode_context-mode__ctx_execute_file(path, language, code)`; ad-hoc computation: `mcp__plugin_context-mode_context-mode__ctx_execute(language, code)`
- fetching external docs, references, or pages: `mcp__plugin_context-mode_context-mode__ctx_fetch_and_index`
- explicit indexing of an artifact already on disk: `mcp__plugin_context-mode_context-mode__ctx_index`
- session telemetry, diagnostics, reset: `ctx_stats`, `ctx_doctor`, `ctx_purge`

Forbidden when editing code:

- `Bash` or `PowerShell` for any command whose output may exceed 20 lines; reserve raw shell tools for `git`, `mkdir`, `rm`, `mv`, and navigation
- `Read` for exploratory analysis; `Read` is correct only when the file's contents must enter context to drive a follow-up `Edit` or `Write`
- `Grep` or `Glob` runs whose hit set is large enough to risk flooding context; route those searches through `ctx_execute(language: "shell", code: "...")` so only the summary enters context
- `WebFetch` for documentation or external references; use `ctx_fetch_and_index`
- using `ctx_execute`, `ctx_execute_file`, or any shell tool to create or modify files; file edits always go through native `Write` and `Edit`
- bypassing `context-mode` "to save a step"; the discipline is load-bearing across long sessions

Editing flow when changing code in `pkg/`, `cmd/`, or `ui/`:

1. orient with `ctx_batch_execute` (combine `git log`, `git status`, focused greps, and `smallfrappuccino-mcp` orientation calls) and follow-up `ctx_search` queries
2. read only the exact files you will `Edit` or `Write` with `Read`; analyse everything else through `ctx_execute_file`
3. apply edits with `Edit` or `Write`
4. run validation (`go test ./...`, `go vet ./...`, `bun run test`, `bun run lint`, `bun run build`) through `ctx_batch_execute` so the output stays sandboxed; surface only the printed summary in the work report
5. if a `context-mode`-backed step was skipped or failed, say so explicitly; do not pretend full validation ran

After `/clear` or `/compact`, the `context-mode` knowledge base is preserved; resume with `ctx_search(sort: "timeline")` before asking the user to restate prior context.

## Preferred Strengths For This Repo

Use Claude preferentially for:

- multi-file implementation when the contract and ownership boundaries are already known
- structured workflows where this document and the live source already specify the constraints
- writing or tightening tests near the changed package
- iterative debugging with a read-change-validate loop
- contract-guided refactors that stay inside existing seams
- disciplined execution of well-defined tasks with explicit validation

This model should add value by connecting existing local patterns across files, not by inventing a new architecture. Treat the ownership map, hotspots, and validation expectations in this document as load-bearing constraints, not suggestions; do not ask clarifying questions when the answer is already specified here or in the live source.

## Scope Fidelity

Do exactly what the user requested. Nothing more, nothing less. "More thorough" is not a goal; matching the request is the goal.

- treat the user's message as the contract; the deliverable is the smallest change that satisfies it
- do not bundle adjacent improvements, cleanup, renames, or reformatting with the requested change
- do not refactor surrounding code unless the requested change cannot land safely without it
- do not add features, options, flags, configuration, logging, telemetry, metrics, or hooks the user did not ask for
- do not anticipate future requirements; if a hypothetical extension would require restructuring now, surface it as a question before doing it
- if you notice an unrelated issue while working, mention it in the report after completing the task; do not silently fold it into the change
- when the request is ambiguous, pick the smallest defensible interpretation and state the assumption in one line; do not pick the broader interpretation because it seems more useful
- when the user says "fix X", fix X; do not also fix Y, Z, and the surrounding style nearby
- when the user says "remove X", remove X completely; do not leave a stub, alias, or comment in its place
- when the user says "rename X to Y", rename X to Y; do not also restructure the file or update unrelated callers' formatting

## Risks And Countermeasures

Common failure modes for this model in `discordcore`:

- over-engineering a local fix into a framework change
- adding wrappers, helpers, or abstractions that do not remove real complexity
- widening a small task into a broad refactor
- making speculative changes outside the stated bug or contract
- growing public surface area when internal changes would suffice
- mixing cleanup into a behavior fix when the cleanup is not required for safety
- adding doc comments, docstrings, or inline comments to code that was not changed, or comments that restate what well-named identifiers already convey
- adding defensive error handling, validation, or fallbacks for cases that cannot occur given current call sites and framework guarantees
- preserving removed code as commented-out blocks, `// removed` markers, deprecated aliases, or unused renamed identifiers instead of deleting outright
- creating a new file or package when an existing sibling seam is the natural home
- losing scope discipline across long agentic sessions and drifting from the original task as context grows

Countermeasures:

- prefer small, reversible, testable changes
- reuse existing seams before introducing new helpers or layers
- keep new public types, exported functions, routes, and settings to a minimum
- verify that each added abstraction removes a repeated or unstable burden, not just local discomfort
- do not combine unrelated refactors with a bug fix unless the refactor is required to make the change safe
- if a hotspot file must change, confirm the surrounding live source before patching and keep the edit tightly scoped
- if the repo already has a local primitive, hook, service, or builder for the job, extend it instead of creating a parallel path
- default to extending the closest existing sibling file rather than creating a new one; create a new file only when the new behavior is a distinct responsibility that breaks the existing file's cohesion, the existing file is a documented hotspot whose explicit countermeasure is a sibling-file split, or the directory's naming pattern mandates separation (`monitoring_*.go`, `postgres_store_*.go`, `discord_oauth_*.go`, `features_*.go`)
- name new files after the new responsibility, never after the task or feature; never introduce a new package without contract justification
- write comments only where the WHY is non-obvious; do not annotate code that was not changed and do not restate what the identifier already says
- validate only at true system boundaries: HTTP request bodies and query params, OAuth callbacks, Discord interaction payloads, and rows read from sources outside the package's trust scope
- inside the package, treat arguments from internal callers as already validated; do not nil-check pointers, slices, or maps that the type system or immediate construction proves non-nil; do not bounds-check indexes derived from a length just measured; do not re-validate values already validated upstream
- do not add error handling for calls that cannot fail in the current code path (e.g., `regexp.MustCompile` of a constant pattern, `time.ParseDuration` of a constant string); do not wrap errors in `fmt.Errorf` when the wrap adds no operation context beyond what the underlying error already carries
- for invariants that must hold but are not type-enforced, prefer `panic` with a clear message over a silent fallback that papers over a bug
- when removing a symbol, delete it cleanly; do not retain a stub, deprecated alias, `// removed` marker, commented-out block, renamed `_unused` placeholder, or re-export
- git history is the record of prior versions; the working tree is not a museum
- the only exception to clean removal is staged removal or a backwards-compatibility shim explicitly requested in the conversation; do not assume the request was implied
- if a removed public symbol has external callers, name the breaking change and stop before editing
- when a persisted config field changes shape (string → slice, scalar → struct, etc.), keep the legacy JSON key alive inside the local `raw*` unmarshal struct only and migrate it into the canonical field at decode time; do not leave the legacy key on the public `*Config` type, and do not emit it on the write path
- in long sessions, re-anchor on the original task before each new edit; if scope has grown beyond what the user requested, stop and confirm before continuing

## Recommended Strategy By Task Type

For bug fixes:

1. reproduce or pin the failing behavior
2. locate the smallest owning seam
3. fix the root cause without broad cleanup
4. add or adjust tests near the changed package
5. run the narrowest relevant validation, then broader checks if the surface warrants it

For multi-file feature work:

1. identify the owning contract first
2. define the slash-command workflow before expanding the dashboard workflow
3. trace all required layers before editing
4. update backend and UI together when the contract crosses both and the UI is justified
5. keep route, feature, and config contracts centralized
6. verify the exact files that define the contract before editing dependents

For refactors:

1. use refactors to clarify an existing seam, not to redesign the area
2. preserve behavior first, then tighten tests
3. prefer extracting within an existing sibling-file pattern over creating new architectural layers
4. stop if the change starts widening beyond the original contract or workflow
5. when a "god object" or wide service needs splitting, prefer role-segregation interfaces over a concrete implementation split; let one concrete type keep satisfying multiple narrow consumer-side interfaces (see `pkg/qotd.QuestionCatalog` and `PublishCoordinator`) before fragmenting the storage and lifecycle wiring

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

- treat the dashboard as a complement to slash-command workflows, not the primary product surface
- default standard pages to setup, review, bulk edits, diagnostics, or exception handling rather than routine bot usage
- do not use the dashboard as the default home for actions that are cleaner as slash commands, command responses, or Discord-native flows
- prefer concise state, blockers, and next configuration actions over mirroring full bot behavior on the page
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
- when a package exposes role-segregated interfaces (e.g. `pkg/qotd.QuestionCatalog`, `PublishCoordinator`), mock the narrowest role the unit under test exercises; do not paste a union stub when the function only needs `Settings` and `ListQuestions`
- prefer pure unit tests for state-machine math, lifecycle window calculations, and error classification; integration tests that round-trip through Postgres or a fake Discord publisher should pin behavior that genuinely crosses those boundaries, not arithmetic that lives in pure helpers
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
- formatting and line endings (any change touching tracked text files): `pwsh scripts/check-format.ps1` (or `bash scripts/check-format.sh`). The script gates `gofmt -l .` and CRLF residue against `.gitattributes`; both must be empty before reporting completion. New files you create must be LF — `.editorconfig` and `.gitattributes` are the contract
- route or embed contract changes: verify `ui/vite.config.ts`, `ui/src/app/routes.ts`, `pkg/control/http_routes.go`, `pkg/control/dashboard_handler.go`, and that `ui/dist/index.html` still exists
- feature or settings contract changes: verify Go route and workspace builders, `ui/src/api/control.ts`, and the adapters or pages consuming the changed fields
- exported Go API, doc, or error-contract changes: update nearby tests and doc comments that pin the new behavior
- integration tests in `pkg/qotd/` and `pkg/discord/commands/qotd/` are heavy (~60s and ~25s respectively); when iterating, run targeted `-run` filters first and the full integration suite once before reporting completion
- if a relevant validation step was not run, say so explicitly

Releases:

- the canonical way to land a change is the local `release` CLI: `release -m "<conventional commit subject>" -y --promote`; it stages changes, bumps `pkg/util/application.go`, runs the build hook, fast-forwards `development` into `main`, and tags the release
- do not push to `main` directly, do not call `git tag` by hand, and do not bundle multiple unrelated changes into one release commit — split into separate `release` invocations so each version is a coherent unit
- the conventional-commit subject becomes the release message; treat it as the changelog entry, not a throwaway commit line

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

QOTD subsystem — load-bearing patterns that new edits must preserve:

- idempotency triad on the publish path: a 16-hex `Nonce` on `QOTDOfficialPostRecord` propagated to Discord via `enforce_nonce`, a partial unique index on `(guild_id, deck_id, publish_date_utc, publish_mode='scheduled')`, and the `resolvePublishNowConflict` / adopt-existing-thread recovery branches; new publish-side edits must keep all three intact or document the new safety story
- `OfficialPostState` distinguishes `failed` (transient, retried every reconcile cycle) from `abandoned` (terminal, requires admin action); `isUnrecoverableDiscordPublishError` is the gate between them, and new Discord error codes belong in one classifier rather than ad-hoc branches in the publish flow
- thread-state errors are classified by `isMissingDiscordThreadError` (404 → flip to `missing_discord`), `isUnmanageableDiscordThreadError` (403 / 50001 / 50013 → degrade silently with log-once dedup via `Service.unmanageableThreadLogs`), and the unclassified default (retry next cycle); new failure modes slot into one of those three buckets
- `syncLiveOfficialPost` short-circuits when `post.State == lifecycle.State`; reconcile-style code added later must follow the same "skip API call when DB already reflects target" pattern to avoid rate-limit pressure and to respect manual mod actions on the thread
- official-post lifecycle transitions that touch BOTH Discord and the DB must route through `applyOfficialPostThreadTransition` in `pkg/qotd/official_post_state_transition.go`; that helper documents the divergence-window contract (Discord first, DB second, reconcile loop is the recovery path) and emits the `qotd_official_post_state_divergence` structured log when the asymmetric "Discord OK, DB failed" path is hit. The typed `*OfficialPostStateDivergenceError` (sentinel `ErrOfficialPostStateDivergence`) is what new callers should branch on if they need to distinguish transient lag from a genuine publish failure
- Discord thread auto-archive is set to `defaultThreadAutoArchiveMinutes` (48h, matching the answer window) with a fallback to `fallbackThreadAutoArchiveMinutes` (4320) on validation rejection; the `archiveOfficialPost` transition only flips `Locked=true` — re-introducing `Archived=true` would race the platform-driven archive
- suppression is a set, not a single date: `QOTDConfig.SuppressScheduledPublishDatesUTC []string` is the canonical field; the unmarshal path migrates the legacy `suppress_scheduled_publish_date_utc` string in `rawQOTDConfig` only, and the runtime trims expired entries on each cycle via `clearExpiredScheduledPublishSuppression`
- the `pkg/qotd` package exposes role-segregated interfaces (`QuestionCatalog`, `PublishCoordinator`, `ReconcileCoordinator`) that the monolithic `*Service` satisfies; consumers (commands, runtime, future HTTP routes) should depend on the narrow role they exercise so test mocks stay small — do not re-widen a consumer's dependency to the union without a contract-level reason
- observability writes go through `qotd.Metrics` and the in-memory implementation `qotd.NewInMemoryMetrics()`. The interface is typed (`RecordPublishSuccess`, `RecordReconcileCycle`, `RecordStateDivergence`, etc.) so a new event is a method addition, not a free-form string key. The Service constructor accepts the metrics value (`NewServiceWithMetrics`) and falls back to `NopMetrics` when nil; new instrumentation goes through `s.observability().RecordX(...)` so unit tests that build `&Service{}` directly still work. The `/v1/health/qotd` route reads via the `SnapshotProvider` type assertion and returns 503 when only `NopMetrics` is attached, so operators can distinguish "QOTD up, telemetry off" from "QOTD unavailable". Do not introduce a separate `prometheus.Counter` import path inside `pkg/qotd` — the Metrics interface is the migration seam if Prometheus is wired later

Feature toggle subsystem — load-bearing patterns that new edits must preserve:

- `pkg/files/feature_registry.go` is the single source of truth for boolean feature toggles. Resolve, clone, defaults, dashboard binding, override detection (`HasAnyOverride`), and the per-command enabled check (`Lookup`) all iterate `featureRegistry`; do not reintroduce hand-maintained per-toggle switches in any of those sites
- `FeatureToggles` (with `*bool` fields and JSON tags) is the persisted schema and must not change shape — adding a toggle means a new field on the relevant sub-struct PLUS one entry in `featureRegistry`. The dotted `Path` must match the field name on both `FeatureToggles` and `ResolvedFeatureToggles`; an invariant test in `feature_registry_test.go` enforces this
- `ResolvedFeatureToggles` is intentionally a mirrored struct (not a `map[string]bool` or accessor-only API) so call-sites keep tight, typo-safe access like `resolved.Moderation.Clean`. The reflection-driven populate path keeps that struct from drifting from the registry; do not "consolidate" it away
- product-facing metadata (label, description, area, tags, editable fields, `LogEvent`) lives in `pkg/control/features_catalog.go` (`featureDefinitions`), not in the registry. The two sets of IDs are kept in bijection by `TestFeatureRegistryMatchesCatalog` and against `ui/src/features/features/featureContract.json` by `TestFeatureRegistryMatchesUIContract`; both must stay green when adding a toggle
- `moderationCommandFeatureEnabled` and the dashboard toggle bindings in `pkg/control/features_toggle_bindings.go` are thin shims over `Lookup`/`LookupToggle`/`SetToggle` — do not regrow a per-feature switch inside them

Additional caution:

- `pkg/discord/logging/` contains broader patchwork than a single file; treat the area as coordination-heavy
- QOTD legacy compatibility is comparatively concentrated in migration and compatibility seams, not spread across the whole feature runtime
- avoid regrowing a hotspot root file when an existing sibling seam can absorb the change
- do not widen already large pages unless the task is truly local to that page

## Conversation Discipline

Output prose must be load-bearing. The diff carries the work; prose carries only what the diff cannot.

- do not open with task restatements ("Looking at this...", "You want me to..."), intent narrations ("I will now read...", "Let me start by..."), or acknowledgments ("Got it", "Sure", "Of course")
- do not close with summaries that the diff already shows; do not list touched files in prose when the diff or git output already lists them
- do not append optimistic next steps, suggested follow-ups, or "you may also want to consider..." paragraphs unless the user asked for recommendations
- do not use filler transitions ("Now let me...", "Next, I will...", "Great, moving on to...", "With that done...")
- do not narrate what tools you are about to call; call them
- when reporting work, lead with what changed and why; trust the user to read the diff
- when asking a question, ask the question without context preface the user already has
- when stating uncertainty, name the uncertainty and the smallest verification step in one line; do not hedge with adverbs ("perhaps", "possibly", "it might be the case that")
- match length to the task: short questions get short answers, terse confirmations get one line, completed work gets the structured report below

## How To Report Work

Substantial work reports should include:

- problem summary
- files modified
- previous behavior
- new behavior
- validation run
- remaining risks or follow-up drift

Skip preamble that restates the task and postamble that summarizes what the diff already shows. If a validation step was skipped, say so directly; do not pad the gap with prose.

Keep this document durable. Do not fill it with task-local plans, prompt boilerplate, vendor rituals, or one-off implementation notes.
