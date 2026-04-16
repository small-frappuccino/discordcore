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
# AGENTS.md

Repository instructions for AI coding agents working in `discordcore`.

This version is tuned for GitHub Copilot agents in VS Code, Copilot CLI, and related Copilot surfaces. Keep the file focused on durable repo facts, boundaries, contracts, and validation rules. Do not assume Codex-specific behavior such as isolated cloud sandboxes, mandatory auto-commits, clean worktrees, or custom citation syntax.

## 1. Mission

Maintain this repository like a production system.

Optimize for:

1. correctness
2. operational reliability
3. maintainability
4. observability
5. low-drift changes that match local patterns

Prefer narrow, source-backed changes over broad rewrites.

## 2. Environment Assumptions

Agents in this repo may be running inside VS Code against a live local workspace.

Assume:

- the worktree may already be dirty
- the user may be editing files at the same time
- generated assets may be present and modified locally
- repo instructions are only one input alongside system, developer, user, skills, and memory

Therefore:

- never revert unrelated user changes
- do not assume a clean branch or a disposable sandbox
- verify live source before patching when MCP reports drift
- keep instructions here durable and repo-specific, not tool-vendor-specific

## 3. Repository Identity

`discordcore` is the product repository. It owns:

- the Go runtime and orchestration for Discord-facing behavior
- the control API and dashboard-serving layer
- canonical config and runtime state
- Postgres-backed persistence and migrations
- the embedded React dashboard under `ui/`

Sibling repo:

- `../alicebot` owns host bootstrap, machine-specific wiring, and final runtime packaging

Boundary rule:

- reusable product logic, state rules, contracts, and shared workflows belong in `discordcore`
- host-specific setup belongs in `alicebot`

## 4. Repo Map

Use this responsibility map before editing:

- `cmd/discordcore/`: example runner only
- `pkg/app/`: runtime orchestration and startup wiring
- `pkg/control/`: control API, auth/session handling, dashboard serving, guild/settings/feature routes
- `pkg/files/`: canonical config model, normalization, persistence adapters, `ConfigManager`
- `pkg/discord/`: Discord runtime behavior, commands, logging, cache, services, session handling
- `pkg/storage/`: durable Postgres-backed domain storage
- `pkg/persistence/`: DB open/ping/migrations
- `pkg/partners/`: partner board rendering and sync helpers
- `pkg/task/`: task router and scheduled/background jobs
- `ui/src/app/`: routes, navigation registry, app-level routing helpers
- `ui/src/api/control.ts`: canonical dashboard API contracts and client behavior
- `ui/src/context/`: dashboard session, guild selection, login/logout, base URL handling
- `ui/src/features/features/`: shared feature-area adapters and workspace hooks
- `ui/src/features/partner-board/`: self-contained partner board workflow
- `ui/src/pages/`: route-level page surfaces
- `ui/dist/`: embedded build output; `index.html` placeholder must remain present

## 5. Discovery Workflow

Use `smallfrappuccino-mcp` first. It is the default repo navigation layer.

Recommended session startup flow:

1. `repo_overview`
2. `reindex` only if the index is stale, suspicious, or incomplete
3. `repo_audit`
4. `contract_checks` before route, dashboard-boundary, or feature-contract work
5. `task_context`
6. `find_nodes` to locate the package, route, feature, page, or symbol you actually need
7. `get_subgraph` before editing hotspot files or changing boundaries
8. `list_observations` before touching sensitive areas

After finding a non-obvious invariant, hotspot, or drift point, store it with `put_observation`.

MCP narrows the search space. It does not replace reading the live source.

## 6. Workspace Noise

Prefer tracked-source inspection over broad recursive scans. The local workspace may include noise such as:

- `ui/node_modules/`
- root `node_modules/`
- `ui/debug-screenshots/`
- build outputs and browser artifacts
- large tracked artifacts like `diff.txt`

Do not infer conventions from those paths.

If the MCP index reports drift in generated assets, validate live source before changing anything and avoid widening the patch based on generated files.

## 7. Inspect Narrowly Vs Broadly

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

Do not start local changes by reading all of `pkg/control/`, `pkg/discord/logging/`, or `ui/src/pages/`.

## 8. Source Of Truth

When docs and source disagree:

- trust source code and current tests first
- treat `README.md` and `UI_RULES.md` as intent and context, not implementation truth
- update stale docs instead of forcing code to match outdated prose

For dashboard implementation details, the current source of truth is the live code in:

- `ui/src/index.css`
- `ui/src/shell.css`
- `ui/src/components/ui.tsx`

## 9. Canonical Contracts

These contracts are current and source-backed unless intentionally changed across all layers.

- canonical dashboard base path is `/manage/`
- `/dashboard/` is a legacy compatibility alias, not the primary route
- backend SPA handling lives in `pkg/control/dashboard_handler.go`
- dashboard HTTP registration lives in `pkg/control/http_routes.go`
- route construction and legacy mapping live in `ui/src/app/routes.ts`
- Vite build base lives in `ui/vite.config.ts`
- the SPA must not intercept `/v1/*` or `/auth/*`

If dashboard routing changes, update backend, frontend, tests, docs, and embed assumptions together.

## 10. Backend Rules

Respect package ownership.

- do not move reusable logic into `cmd/`
- do not reimplement config/state rules in `pkg/control/` when they belong in `pkg/files/`
- do not move Discord runtime behavior into the dashboard layer

Error handling and control flow:

- prefer explicit error propagation
- wrap with operation context using `fmt.Errorf("operation: %w", err)`
- avoid `panic` for expected runtime failures
- return early on bad auth, invalid input, and missing dependencies

Logging and observability:

- use existing repo logging facilities, especially `pkg/log` and area-specific helpers
- include operation context and relevant guild, channel, or user identifiers
- never log secrets, tokens, OAuth credentials, or private message content

Config and state mutation:

- treat `ConfigManager.Config()` and `GuildConfig()` results as read-only snapshots
- persist via `UpdateConfig`, `UpdateRuntimeConfig`, or existing helpers
- preserve normalization and validation when changing config semantics
- if config semantics change, update normalization, persistence, route handling, and tests together

Testing style:

- keep tests close to the changed package
- prefer deterministic seams already present in the repo
- do not require live Discord access
- only use isolated Postgres helpers where the package already follows that pattern

## 11. Dashboard And TypeScript Rules

Keep API and route contracts centralized.

- keep dashboard request and response shapes in `ui/src/api/control.ts`
- keep route strings and legacy mapping in `ui/src/app/routes.ts`
- keep navigation registry data in `ui/src/app/navigation.ts`
- keep shared feature interpretation in `ui/src/features/features/`
- keep partner-board-specific logic in `ui/src/features/partner-board/`
- avoid `any`
- avoid duplicating contract types inside page files

When a backend contract changes, update:

1. `ui/src/api/control.ts`
2. the feature adapters or pages that consume it
3. tests that pin the behavior

React and page boundaries:

- route-level surfaces belong in `ui/src/pages/`
- reusable shell pieces belong in `ui/src/components/ui.tsx`
- shared domain hooks and helpers belong in `ui/src/features/*`
- use `DashboardSessionContext` and existing feature hooks before adding ad hoc fetch logic

Avoid widening already large pages unless the task is truly local to that page:

- `ui/src/pages/RolesPage.tsx`
- `ui/src/pages/ModerationPage.tsx`
- `ui/src/pages/CommandsPage.tsx`
- `ui/src/pages/StatsPage.tsx`
- `ui/src/pages/LoggingCategoryPage.tsx`

If a page is already large, extract helpers or page-local subcomponents instead of growing the file further.

## 12. UI Semantics

Preserve the compact operational dashboard style already present in the app.

- prefer existing primitives such as `PageHeader`, `FeatureWorkspaceLayout`, `SurfaceCard`, `StatusBadge`, and `EmptyState`
- use human-facing labels instead of raw internal enum or storage names unless the page is explicitly diagnostic
- use `import.meta.env.BASE_URL` for embedded asset paths
- default settings-style pages to direct controls with minimal helper text
- keep standard dashboard pages free of provenance noise, repeated state badges, raw IDs, or fallback-by-ID editors unless the screen is explicitly diagnostic
- gate low-level diagnostic metadata behind explicit diagnostic UI instead of exposing it in the default flow
- keep adjacent settings in grouped surfaces with internal row dividers when they belong to the same workflow
- keep expanded child controls inside the same parent setting group

When in doubt, follow `UI_RULES.md` and current source patterns rather than inventing a new dashboard visual language.

## 13. Hotspots And Decomposition Invariants

These files are central and need more neighborhood reading before edits:

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

These seams are intentional and should stay decomposed:

- `pkg/control/features_routes.go`: router and dispatch only; feature catalog, workspace shaping, readiness, blockers, patch flows, and toggle bindings belong in focused services or `features_*.go` siblings
- `pkg/control/discord_oauth.go`: shared OAuth types, constants, provider construction, and permission parsing only; flow logic belongs in `discord_oauth_*.go` siblings or the dedicated service
- `pkg/storage/postgres_store.go`: `Store` type, bootstrap, schema init entrypoint, and shared SQL helpers only; domain behavior belongs in focused `postgres_store_*.go` files
- `pkg/discord/logging/monitoring.go`: lifecycle and orchestration only; gateway handlers, reactions, cache loops, and permission mirroring belong in focused `monitoring_*.go` files

When coordination spans multiple files, prefer extending an existing focused service or sibling-file seam instead of regrowing the root hotspot file.

## 14. Preferred Change Strategy

For production changes:

1. identify the exact contract or workflow being changed
2. locate the minimal files through MCP plus source verification
3. preserve local style and ownership boundaries
4. update tests near the changed behavior
5. validate the narrow change before considering cleanup

Do not combine unrelated refactors with a behavior fix unless the refactor is required to make the fix safe.

## 15. Validation Expectations

Run the checks that match the touched area.

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

Feature or settings contract changes:

- verify Go route and workspace builders
- verify `ui/src/api/control.ts`
- verify the adapters or pages consuming the changed fields

If a relevant validation step was not run, say so explicitly.

## 16. Durable Instructions Vs Skills Or Memory

Keep this file for stable repo rules.

Good content for `AGENTS.md`:

- repository boundaries
- ownership rules
- canonical contracts
- validation expectations
- hotspot and decomposition invariants

Do not bloat this file with:

- temporary task plans
- prompt boilerplate
- vendor-specific formatting rituals
- ephemeral one-off implementation notes

If the repo later adds Copilot skills or benefits from Copilot memory, keep those for specialized or learned workflows. Keep `AGENTS.md` as the durable baseline.

## 17. Work Reporting

Substantial work reports should include:

- problem summary
- files modified
- previous behavior
- new behavior
- validation run
- remaining risks or follow-up drift

If repo guidance is stale, call it out explicitly and update the guidance when appropriate.
