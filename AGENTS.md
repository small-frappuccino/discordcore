# AGENTS.md

Repository-specific operating rules for AI agents working in `discordcore`.

## 1. Mission

You are maintaining a production repository. Optimize for:

1. correctness
2. operational reliability
3. maintainability
4. observability
5. low-drift changes that match existing local patterns

Do not treat this repo like a greenfield project. Prefer targeted changes over broad rewrites.

## 2. Repository Identity

`discordcore` is the product repository. It contains:

- the Go runtime, control API, config/state model, Discord services, and Postgres-backed storage
- the embedded React dashboard under `ui/`
- the `//go:embed` payload served by the control plane

Sibling repo:

- `../alicebot` is the host runtime and environment wiring

Boundary rule:

- reusable logic, domain rules, state transitions, and dashboard/backend contracts belong in `discordcore`
- host bootstrap, machine-specific wiring, and final runtime packaging belong in `alicebot`

## 3. Real Repo Map

Use this responsibility map before editing:

- `cmd/discordcore/`
  - example runner only, not the place for reusable product logic
- `pkg/app/`
  - orchestration and startup wiring for the bot runtime
- `pkg/control/`
  - control API, auth/session handling, dashboard serving, guild/settings/feature routes
  - keep router/provider entrypoints thin; feature and OAuth workflows should live in focused sibling files or explicit internal services
- `pkg/files/`
  - canonical config model, normalization, persistence adapters, and `ConfigManager`
- `pkg/discord/`
  - Discord runtime behavior: commands, logging, cache, services, session handling
- `pkg/storage/`
  - durable Postgres-backed domain storage
- `pkg/persistence/`
  - DB open/ping/migrations
- `pkg/partners/`
  - partner board rendering and sync helpers
- `pkg/task/`
  - task router and scheduled/background jobs
- `ui/src/app/`
  - route definitions, navigation registry, app-level routing helpers
- `ui/src/api/control.ts`
  - canonical dashboard API contracts and client behavior
- `ui/src/context/`
  - session, guild selection, login/logout, base URL handling
- `ui/src/features/features/`
  - feature-area adapters, presentation helpers, and reusable workspace hooks
- `ui/src/features/partner-board/`
  - self-contained partner board workflow
- `ui/src/pages/`
  - route-level page surfaces; do not let domain logic sprawl here unnecessarily
- `ui/dist/`
  - embedded build output; `index.html` placeholder must remain present

## 4. Workspace Noise And Navigability

This repo is medium-sized when measured by tracked source, but the working tree may contain large non-source areas.

Prefer `git ls-files`, `discordcore-mcp`, and targeted file opens over broad recursive scans because local workspaces may include:

- `ui/node_modules/`
- `ui/debug-screenshots/`
- root `node_modules/`
- build outputs and browser/test artifacts
- tracked non-source artifacts such as `discordcore.exe`, `diff.txt`, and cache results

Do not infer conventions from those artifacts.

## 5. Required Use Of `discordcore-mcp`

`discordcore-mcp` is the default repo navigation layer for this workspace.

Use it first for:

- repo orientation
- tracked-source pressure, hotspots, and workspace-noise review
- explicit dashboard/backend contract checks
- symbol/package/route lookup
- scoped dependency tracing
- hotspot discovery
- prior observations and invariant recovery

Required startup flow for a new coding session:

1. `discordcore_repo_overview`
2. if the index is stale, incomplete, or negative results look suspicious, run `discordcore_reindex`
3. `discordcore_repo_audit` to surface tracked-source hotspots, noisy paths, and doc drift
4. `discordcore_contract_checks` before route, feature-contract, or dashboard-boundary work
5. `discordcore_task_context` for implementation, debugging, review, or architecture work
6. `discordcore_find_nodes` to locate the exact package, route, feature, or page
7. `discordcore_get_subgraph` before editing central files or changing boundaries
8. `discordcore_list_observations` before touching sensitive areas

After discovering a non-obvious invariant or drift point, add an observation with `discordcore_put_observation`.

`discordcore-mcp` narrows the search space. It does not replace source verification.

## 6. Inspect Narrowly Vs Broadly

Default to narrow inspection.

Inspect narrowly when:

- the task is confined to one route, one feature, one hook, one settings section, or one page
- MCP has already identified the relevant package/file cluster
- the change is an extension of an existing local pattern

Inspect broadly when:

- changing shared contracts between backend and dashboard
- changing config/state semantics in `pkg/files/`
- changing routing, auth, or dashboard mount behavior
- changing feature IDs, editable fields, or settings workspace shapes
- touching a hotspot file

If the task is local, do not start by reading all of `pkg/control/`, `pkg/discord/logging/`, or `ui/src/pages/`.

## 7. Canonical Runtime And Route Contracts

These are current source-backed contracts and should be preserved unless intentionally changed across all layers.

- Canonical dashboard base path is `/manage/`
- `/dashboard/` is a legacy compatibility alias, not the primary route
- backend route handling for the embedded SPA lives in `pkg/control/dashboard_handler.go`
- HTTP registration and redirects live in `pkg/control/http_routes.go`
- route construction and legacy mapping live in `ui/src/app/routes.ts`
- Vite build base lives in `ui/vite.config.ts`
- the SPA must not intercept `/v1/*` or `/auth/*`

If you change dashboard routing, update backend, frontend, tests, and docs together.

## 8. Source Of Truth Rules

When docs and code disagree:

- trust source code and current tests first
- treat `README.md` and `UI_RULES.md` as intent/context, not infallible implementation truth
- update stale docs instead of forcing the code to match outdated prose

For UI implementation details, current CSS variables and shell/layout components are the source of truth:

- `ui/src/index.css`
- `ui/src/shell.css`
- `ui/src/components/ui.tsx`

## 9. Go Conventions For This Repo

### 9.1 Package roles

Respect the existing package responsibilities.

- do not move business logic into `cmd/`
- do not reimplement config/state rules in `pkg/control/` if they belong in `pkg/files/`
- do not put Discord runtime rules into the dashboard

### 9.2 Errors and control flow

Current code consistently favors explicit error propagation.

- wrap errors with operation context using `fmt.Errorf("operation: %w", err)`
- avoid `panic` for expected runtime failures
- return early on bad auth, invalid input, and unavailable dependencies

### 9.3 Logging and observability

Use the repo logging facilities.

- prefer `pkg/log` loggers and existing control/runtime logging helpers
- include operation context and guild/channel/user identifiers when relevant
- never log secrets, bearer tokens, OAuth credentials, or private message content

### 9.4 Config and state mutation

`pkg/files` is the canonical config/state layer.

- treat `ConfigManager.Config()` and `GuildConfig()` results as read-only snapshots
- mutate persisted config through `ConfigManager.UpdateConfig`, `UpdateRuntimeConfig`, or the existing helpers
- preserve normalization and validation paths when editing config structures
- if you change config semantics, update the normalization, persistence, route layer, and tests together

### 9.5 Control API contracts

The dashboard depends on explicit JSON contracts.

- keep response/request shape changes synchronized with `ui/src/api/control.ts`
- if you add or rename feature IDs, editable fields, or workspace sections, update both the Go route layer and the UI adapters/pages/tests in the same change

### 9.6 Testing style

Existing Go tests are targeted and package-local.

- keep tests close to the package being changed
- prefer deterministic test seams already present in the codebase over new mocking frameworks
- do not require live Discord access for tests
- use isolated Postgres helpers only where the package already does so

### 9.7 Go anti-patterns in this repo

Do not:

- mutate published config snapshots directly
- bypass validation/normalization before persisting config
- add ad hoc logger stacks when `pkg/log` already covers the use case
- widen giant files further when a helper or sibling file is the clearer move
- re-grow decomposed entrypoints such as `pkg/control/features_routes.go`, `pkg/control/discord_oauth.go`, `pkg/storage/postgres_store.go`, or `pkg/discord/logging/monitoring.go`
- add new cross-cutting free functions to a hotspot root file when the area already has a focused internal service or sibling-file seam
- put dashboard-only semantics into `pkg/app/`

## 10. TypeScript Conventions For This Repo

The TS codebase is already strict. Preserve that strictness.

- keep API contracts in `ui/src/api/control.ts`
- keep route strings and legacy path mapping in `ui/src/app/routes.ts`
- keep navigation registry data in `ui/src/app/navigation.ts`
- keep shared feature/workspace interpretation in `ui/src/features/features/`
- keep partner-board-specific state and transformations in `ui/src/features/partner-board/`
- avoid `any`; prefer precise interface/type additions
- avoid duplicating request/response shapes inside page files

When a backend contract changes, update:

1. `ui/src/api/control.ts`
2. feature/page adapters that consume it
3. tests that pin that behavior

## 11. TSX / React Conventions For This Repo

### 11.1 Component boundaries

Current structure is:

- route-level surfaces in `ui/src/pages/`
- reusable page shell pieces in `ui/src/components/ui.tsx`
- shared domain hooks/helpers in `ui/src/features/*`

Keep it that way.

### 11.2 Session and fetch behavior

- use `DashboardSessionContext` for auth state, selected guild, base URL, login/logout, and shared client access
- use existing feature hooks such as `useFeatureWorkspace`, `useFeatureMutation`, `useGuildRoleOptions`, `useGuildChannelOptions`, and `useGuildMemberOptions`
- do not add scattered `fetch` logic to page files when the existing client/hooks model covers the need

### 11.3 Page growth

Several pages are already large.

- `ui/src/pages/RolesPage.tsx`
- `ui/src/pages/ModerationPage.tsx`
- `ui/src/pages/CommandsPage.tsx`
- `ui/src/pages/StatsPage.tsx`
- `ui/src/pages/LoggingCategoryPage.tsx`

Do not make these pages broader by default. When adding a new sub-workflow or repeated block:

- extract helpers into `ui/src/features/features/*`
- extract page-local subcomponents when the file is already difficult to scan
- keep raw `feature.details` parsing out of JSX where possible

### 11.4 UI semantics

- preserve the compact operational dashboard style already present in the app
- prefer existing primitives such as `PageHeader`, `FeatureWorkspaceLayout`, `SurfaceCard`, `StatusBadge`, `EmptyState`, and picker fields before inventing new layout systems
- use human-facing labels in the UI; avoid exposing raw internal enum or storage names unless the page is explicitly diagnostic
- use `import.meta.env.BASE_URL` for embedded asset paths
- default settings-style pages to a direct composition: short title, visible control, optional one-line secondary text only when it prevents ambiguity
- keep standard dashboard pages free of diagnostic metadata such as provenance, override state, raw IDs, fallback inputs, and repeated status badges when the control already shows the state
- if low-level metadata is still needed for debugging, gate it behind explicit diagnostic UI instead of exposing it in the default page flow
- when adjacent settings belong to the same workflow, prefer one subtle grouped surface with internal row dividers and whitespace between groups instead of full-width divider lines between unrelated sections
- expanded child controls should stay inside the parent setting group; do not let an inline expansion escape into a separate decorative slab or detached field block

### 11.5 CSS and styling

- use the existing CSS variables and component classes in `ui/src/index.css` and `ui/src/shell.css`
- do not reintroduce stale token values from docs when they disagree with code
- keep shell/layout changes aligned with current `DashboardLayout` behavior rather than old static design rules

### 11.6 React anti-patterns in this repo

Do not:

- duplicate route or navigation definitions inside pages
- bypass `ui/src/api/control.ts` with hand-rolled request code
- move backend business rules into components
- hardcode `/dashboard/` as if it were the primary dashboard path
- keep expanding megafile pages when the logic belongs in hooks/helpers

## 12. Known Hotspots

These files are central and high-context. Read more of their neighborhood before editing them.

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

If a change only needs one branch/helper inside a hotspot, avoid refactoring unrelated sections.

## 12.1 Decomposition Invariants

These seams are intentional and should be preserved when adding features or evolving existing code:

- `pkg/control/features_routes.go`
  - router/dispatch only
  - feature catalog, workspace shaping, readiness, blockers, patch flows, and toggle binding changes belong in `featureControlService` or focused `features_*.go` siblings
- `pkg/control/discord_oauth.go`
  - shared OAuth types, constants, provider construction, and permission parsing only
  - login/callback/session/cookie/guild-access flows belong in `discordOAuthControlService` or focused `discord_oauth_*.go` siblings
- `pkg/storage/postgres_store.go`
  - `Store` type, bootstrap, schema init entrypoint, and shared SQL helpers only
  - domain behavior belongs in focused `postgres_store_*.go` files
- `pkg/discord/logging/monitoring.go`
  - lifecycle/orchestration only
  - gateway handlers, user/avatar/role reactions, state/cache loops, and bot permission mirroring belong in focused `monitoring_*.go` files

When a workflow needs coordination across multiple sibling files, prefer adding or extending an explicit internal service instead of pushing more logic back into the root file.

## 13. Preferred Modification Strategy

For production changes:

1. identify the exact contract or workflow being changed
2. locate the minimal backend and UI files through MCP plus source verification
3. preserve the existing local style in that area
4. update tests near the changed behavior
5. validate the narrow change before considering any cleanup

Do not combine unrelated refactors with a behavior fix unless the refactor is required to make the fix safe.

## 14. Validation Expectations

At minimum, run the checks appropriate to the touched area.

Backend changes:

- `go test ./...`
- `go vet ./...`

UI changes:

- `bun run test`
- `bun run lint`
- `bun run build`

Route or embed contract changes:

- verify `ui/vite.config.ts`
- verify `ui/src/app/routes.ts`
- verify `pkg/control/http_routes.go`
- verify `pkg/control/dashboard_handler.go`
- ensure `ui/dist/index.html` still exists

Feature/settings contract changes:

- verify Go route/workspace builders
- verify `ui/src/api/control.ts`
- verify feature adapters/pages consuming the changed fields

## 15. Pre-Merge Checklist

- correct repository boundary respected (`discordcore` vs `alicebot`)
- MCP overview/context lookup used before broad edits
- source files, not workspace artifacts, were used as the primary basis for changes
- canonical `/manage/` routing preserved unless intentionally changed across layers
- dashboard/backend contracts kept in sync
- config mutations still normalize and validate
- logging still carries operational context
- no UI business logic added for backend-owned rules
- tests/lint/build steps for the touched area were run or explicitly reported as not run

## 16. Work Reporting

Every substantial change report should include:

- problem summary
- files modified
- previous behavior
- new behavior
- validation steps
- remaining risks or follow-up drift

If you discover stale repo guidance while working, call it out explicitly instead of silently working around it.
