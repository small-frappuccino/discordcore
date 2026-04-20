# Discordcore Gemini Guide

Use this guide when operating in discordcore with Gemini 3.1 Pro. Keep repository facts stable. Use broad-context reasoning for investigation, architecture reading, and planning, then reduce to a small, source-backed change.

## Mission

Maintain this repository like a production system.

Optimize for:
1. correctness
2. operational reliability
3. maintainability
4. observability
5. low-drift changes that match local patterns

Do not treat discordcore like a greenfield codebase. Prefer narrow, source-backed changes over broad rewrites.

## Repository Identity

discordcore is the product repository. It owns:
- the Go runtime and orchestration for Discord-facing behavior
- the control API and dashboard-serving layer
- canonical config and runtime state
- Postgres-backed persistence and migrations
- the embedded React dashboard under ui/

Sibling repo:
- ../alicebot owns host bootstrap, machine-specific wiring, and final runtime packaging

## Boundaries And Ownership

Keep reusable product logic, state rules, contracts, and shared workflows in discordcore.
Keep host-specific setup in ../alicebot.

Use this map before editing:
- cmd/discordcore/: example runner only
- pkg/app/: runtime orchestration and startup wiring
- pkg/control/: control API, auth/session handling, dashboard serving, guild/settings/feature routes
- pkg/files/: canonical config model, normalization, persistence adapters, ConfigManager
- pkg/discord/: Discord runtime behavior, commands, logging, cache, services, session handling
- pkg/storage/: durable Postgres-backed domain storage
- pkg/persistence/: DB open/ping/migrations
- pkg/partners/: partner board rendering and sync helpers
- pkg/task/: task router and scheduled/background jobs
- ui/src/app/: routes, navigation registry, app-level routing helpers
- ui/src/api/control.ts: canonical dashboard API contracts and client behavior
- ui/src/context/: dashboard session, guild selection, login/logout, base URL handling
- ui/src/features/features/: shared feature-area adapters and workspace hooks
- ui/src/features/partner-board/: self-contained partner board workflow
- ui/src/pages/: route-level page surfaces
- ui/dist/: embedded build output; index.html placeholder must remain present

## How To Work Well In This Repo With Gemini 3.1 Pro

Use Gemini's large-context strength for exploration, architecture tracing, root cause analysis, and ambiguous investigations. Then compress that context into an operational plan before editing.

Separate work explicitly:
- Discovery: inspect architecture, contracts, ownership, likely file clusters, and current tests
- Synthesis: state what is known, what is inferred, and what still needs code verification
- Plan: define the smallest behavior or contract change that solves the problem
- Minimal implementation: edit only the files required by the validated plan

Use smallfrappuccino-mcp first for repository navigation:
1. repo_overview
2. reindex only if the index is stale, suspicious, or incomplete
3. repo_audit
4. contract_checks before route, dashboard-boundary, or feature-contract work
5. task_context
6. find_nodes to locate the actual package, route, feature, page, or symbol
7. get_subgraph before editing hotspot files or changing boundaries
8. list_observations before touching sensitive areas

MCP narrows the search space. It does not replace reading live source.

Assume the local workspace may be live and dirty:
- the user may be editing files at the same time
- generated assets may already be modified
- never revert unrelated changes
- do not assume a clean branch or disposable sandbox
- verify live source before patching when MCP reports drift

If MCP reports drift in generated assets, validate live source before changing anything and do not widen the patch based on generated output.

## Where To Use Broad Exploration Vs Narrow Inspection

Inspect broadly when:
- the issue is ambiguous and the root cause is not yet localized
- multiple packages may participate in one behavior
- backend and UI contracts change together
- config or state semantics in pkg/files/ may change
- auth, routing, dashboard mount behavior, or embed behavior may change
- feature IDs, editable fields, or workspace shapes may change
- you are entering a known hotspot file

Inspect narrowly when:
- the task is confined to one route, one feature, one hook, one settings section, or one page
- MCP already identified the relevant file cluster
- the change extends an existing local pattern
- the behavior is already pinned by nearby tests

Do not use broad reading as a license to redesign the area. Read wide to understand architecture and contracts, then reduce scope before editing.
Do not start local changes by scanning all of pkg/control/, pkg/discord/logging/, or ui/src/pages/.

Prefer tracked-source inspection over broad recursive scans. Workspace noise may include:
- ui/node_modules/
- root node_modules/
- ui/debug-screenshots/
- build outputs and browser artifacts
- large tracked artifacts such as diff.txt, discordcore.exe, or cache outputs

Do not infer conventions from those paths.

## Preferred Strengths For This Repo

Use Gemini 3.1 Pro most aggressively for:
- broad codebase exploration before implementation
- understanding architecture across packages
- tracing contract flow between Go backend and dashboard frontend
- root cause analysis that spans orchestration, control API, config, and UI
- navigating large context windows without losing the owning package
- ambiguous investigations where the first obvious cause may be wrong
- planning open-ended changes before deciding whether any code should be touched

When using wide context, always end with an operational synthesis:
- exact behavior or contract under discussion
- owning package and secondary packages
- smallest likely file set
- tests or validations that can confirm the hypothesis
- open uncertainties that still require live code checks

## Risks And Countermeasures

Common failure modes for this working style:
- editing too much before the scope is reduced
- drawing speculative conclusions without enough live code checks
- proposing repo-wide cleanup when the task only needs a local fix
- turning investigation into an unnecessary redesign
- collecting too much context without producing an actionable synthesis

Countermeasures:
- name the contract or workflow before editing
- distinguish observed facts from hypotheses
- anchor conclusions in real files, symbols, and current tests
- choose the smallest change that preserves existing seams
- stop broad exploration once the owning cluster is clear
- if the root cause is still uncertain, add one more targeted read instead of widening the patch
- if a cleanup is not required to make the fix safe, do not include it
- after discovery, summarize the result in a few concrete bullets before writing code

## Workflow For Investigation, Debugging, And Planning

1. Start with MCP orientation and hotspot review.
2. Identify the contract, workflow, or user-visible behavior that is actually under investigation.
3. Map ownership: which package owns the rule, which package only consumes it, and whether UI and backend both participate.
4. Read broadly only until the contract boundaries become clear.
5. Re-read the exact live files and tests that anchor the behavior.
6. Write a short synthesis that separates facts, hypotheses, and open questions.
7. Propose the smallest safe plan.
8. Implement only after the plan is local enough to verify.
9. Validate with the checks that match the touched area.
10. Report previous behavior, new behavior, and remaining risks.

When docs and source disagree:
- trust source code and current tests first
- treat README.md and UI_RULES.md as intent and context, not implementation truth
- update stale docs instead of forcing code to match outdated prose

For dashboard implementation details, the live source of truth is ui/src/index.css, ui/src/shell.css, and ui/src/components/ui.tsx.

## Minimal Implementation And Validation Rules

Preserve package ownership:
- do not move reusable logic into cmd/
- do not reimplement config/state rules in pkg/control/ when they belong in pkg/files/
- do not move Discord runtime behavior into the dashboard layer

Backend change rules:
- prefer explicit error propagation
- wrap errors with operation context using fmt.Errorf("operation: %w", err)
- avoid panic for expected runtime failures
- return early on bad auth, invalid input, and missing dependencies
- use existing logging facilities, especially pkg/log and area-specific helpers
- include relevant guild, channel, or user identifiers when useful
- never log secrets, tokens, OAuth credentials, or private message content

Config and state mutation rules:
- treat ConfigManager.Config() and GuildConfig() results as read-only snapshots
- persist through UpdateConfig, UpdateRuntimeConfig, or existing helpers
- preserve normalization and validation when config semantics change
- if config semantics change, update normalization, persistence, route handling, and tests together

Testing rules:
- keep tests close to the changed package
- prefer deterministic seams already present in the repo
- do not require live Discord access
- only use isolated Postgres helpers where the package already follows that pattern

Dashboard and TypeScript rules:
- keep request and response shapes in ui/src/api/control.ts
- keep route strings and legacy mapping in ui/src/app/routes.ts
- keep navigation registry data in ui/src/app/navigation.ts
- keep shared feature interpretation in ui/src/features/features/
- keep partner-board-specific logic in ui/src/features/partner-board/
- avoid any
- avoid duplicating contract types inside page files
- use DashboardSessionContext and existing feature hooks before adding ad hoc fetch logic

If a backend contract changes, update ui/src/api/control.ts, the consuming feature adapters or pages, and the tests that pin the behavior.

React and page boundary rules:
- route-level surfaces belong in ui/src/pages/
- reusable shell pieces belong in ui/src/components/ui.tsx
- shared domain hooks and helpers belong in ui/src/features/*
- avoid widening already large pages unless the task is truly local
- treat ui/src/pages/RolesPage.tsx, ui/src/pages/ModerationPage.tsx, ui/src/pages/CommandsPage.tsx, ui/src/pages/StatsPage.tsx, and ui/src/pages/LoggingCategoryPage.tsx as large surfaces
- if a page is already large, extract helpers or page-local subcomponents instead of growing the file further

UI semantics:
- preserve the compact operational dashboard style
- prefer existing primitives such as PageHeader, FeatureWorkspaceLayout, SurfaceCard, StatusBadge, and EmptyState
- use human-facing labels instead of raw internal enum or storage names unless the page is explicitly diagnostic
- use import.meta.env.BASE_URL for embedded asset paths
- keep standard pages free of provenance noise, repeated state badges, raw IDs, or fallback-by-ID editors unless the screen is explicitly diagnostic
- gate low-level diagnostic metadata behind explicit diagnostic UI
- group adjacent settings that belong to one workflow

Canonical contracts that must stay aligned unless intentionally changed across all layers:
- the canonical dashboard base path is /manage/
- /dashboard/ is a legacy compatibility alias, not the primary route
- backend SPA handling lives in pkg/control/dashboard_handler.go
- dashboard HTTP registration lives in pkg/control/http_routes.go
- route construction and legacy mapping live in ui/src/app/routes.ts
- Vite build base lives in ui/vite.config.ts
- the SPA must not intercept /v1/* or /auth/*

If dashboard routing changes, update backend, frontend, tests, docs, and embed assumptions together.

Validation expectations:
- backend changes: go test ./... and go vet ./...
- UI changes: bun run test, bun run lint, bun run build
- route or embed changes: verify ui/vite.config.ts, ui/src/app/routes.ts, pkg/control/http_routes.go, pkg/control/dashboard_handler.go, and ensure ui/dist/index.html still exists
- feature or settings contract changes: verify Go route and workspace builders, ui/src/api/control.ts, and the adapters or pages consuming the changed fields

If a relevant validation step was not run, say so explicitly.

## Hotspots And Care Points

These files need more neighborhood reading before edits:
- pkg/discord/logging/monitoring.go
- pkg/storage/postgres_store.go
- pkg/control/features_routes.go
- pkg/control/discord_oauth.go
- pkg/files/types.go
- pkg/app/runner.go
- ui/src/api/control.ts
- ui/src/context/DashboardSessionContext.tsx
- ui/src/pages/RolesPage.tsx
- ui/src/pages/ModerationPage.tsx

These seams are intentional. Do not collapse them during investigation or implementation:
- pkg/control/features_routes.go should stay thin; routing and dispatch here, catalog, workspace, readiness, blockers, patch flows, and toggle bindings in focused services or features_*.go siblings
- pkg/control/discord_oauth.go should hold shared OAuth types, constants, provider construction, and permission parsing; flow logic belongs in discord_oauth_*.go siblings or the dedicated service
- pkg/storage/postgres_store.go should hold Store, bootstrap, schema init entrypoints, and shared SQL helpers; domain behavior belongs in focused postgres_store_*.go files
- pkg/discord/logging/monitoring.go should stay lifecycle and orchestration focused; gateway handlers, reactions, cache loops, and permission mirroring belong in focused monitoring_*.go files

Known dense areas and patchwork pressure: pkg/control, pkg/discord/logging, pkg/files, ui/src/features/features, and ui/src/pages all deserve scope discipline because they carry cross-layer contracts, hotspot pressure, or oversized route surfaces.

When coordination spans multiple files, prefer extending an existing focused seam or sibling-file pattern instead of regrowing a hotspot file.

## How To Report Work

For substantial work, report:
- the problem summary
- files modified
- previous behavior
- new behavior
- validation run
- remaining risks or follow-up drift

Keep this file durable. Do not fill it with temporary task plans, prompt boilerplate, or one-off notes.