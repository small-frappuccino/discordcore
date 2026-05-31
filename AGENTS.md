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

Doc-comment baseline for Go:

- exported types, functions, methods, package-level vars, constants, and the package itself carry a doc comment in stdlib style: start with the identifier name, complete sentences, adjacent to the declaration with no blank line between
- the first sentence is the summary `go doc` and pkg.go.dev consumers see; describe behavior, not just the signature
- document non-obvious contracts explicitly: concurrency guarantees (the default assumption is single-goroutine; state when a type or method deviates), returned-error semantics including sentinels and `errors.Is` branches, ownership of returned values, and lifecycle ordering (`Close`/`Stop`, idempotency)
- `Deprecated:` is a structured marker; use it on legacy fields and aliases (see `pkg/files/types.go`) rather than free-form "old"/"legacy" prose
- doc comments on exported APIs legitimately describe WHAT — the audience reads them through `go doc`, not the source; inline `//` comments inside function bodies stay WHY-only
- multi-paragraph contract docs with section headers (`# Contract`, `# Parameters`, `# Why not X`) are the local idiom for load-bearing seams; see `pkg/qotd/official_post_state_transition.go` and `pkg/qotd/observability.go`

TypeScript and UI: types in `ui/src/api/control.ts` and component prop interfaces are the contract; add JSDoc only when behavior cannot be expressed in the type (units, side effects, lifecycle ordering).

Across both:

- `TODO`/`FIXME` markers carry an owner (`TODO(user): …`) or a specific triggering event; ownerless `TODO:` is an anti-pattern
- do not preserve removed code as commented-out blocks, `// removed` markers, or `_unused` placeholders — git history is the museum
- do not write comments that restate the identifier or the next line; do not narrate straight-line code step by step
- do not add or rewrite comments on code that was not changed in the current task; comment churn pollutes blame
- do not embed history references ("added for ticket X", "renamed in 2024") — those belong in commit messages

## Scope Fidelity

Do exactly what the user requested. Nothing more, nothing less.

- treat the user’s message as the contract; the deliverable is the smallest safe change that satisfies it
- expect direct, task-focused instructions, occasionally in Portuguese (e.g., "Execute task M3", "Como isso fica..."). Acknowledge and proceed without conversational filler
- leverage artifacts (`implementation_plan.md`, `task.md`) when a plan is required for complex tasks, but execute straightforward fixes immediately
- do not bundle adjacent improvements, cleanup, renames, or reformatting with the requested change
- do not refactor surrounding code unless the requested change cannot land safely without it
- do not add features, options, flags, configuration, logging, telemetry, metrics, or hooks the user did not ask for
- do not anticipate future requirements; if a broader change would be required, surface that explicitly instead of silently widening scope
- if you notice an unrelated issue while working, mention it separately after the requested work is complete
- when the request is ambiguous, pick the smallest defensible interpretation and state the assumption briefly

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

Go and backend rules:

- put `context.Context` first when present
- prefer `:=` for new values and `=` when reusing outer `ctx` or `err`
- wrap inspectable errors with `fmt.Errorf("operation: %w", err)`
- use sentinel errors and `errors.Is` / `errors.As` when callers branch on failure
- logging goes through `pkg/log` or area-specific helpers, not stdlib `log`
- include relevant guild, channel, or user identifiers in operational logs
- never log secrets, OAuth credentials, tokens, or private message content
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

## Config Schema Evolution Pattern

When a persisted config field changes shape:

1. add the new field on the public struct in `pkg/files/types.go`
2. keep the legacy JSON key alive only inside the local `raw*` unmarshal struct
3. migrate legacy to canonical at decode time; do not emit the legacy key on the write path
4. update cloning, normalization, and `IsZero` logic together
5. update all callers and tests in the same change; do not leave shim-then-cleanup work behind

## Testing And Validation

Testing style:

- keep tests close to the changed package
- prefer deterministic seams already present in the repo
- do not require live Discord access
- use isolated Postgres helpers only where the package already follows that pattern
- mock the narrowest interface the unit under test actually exercises
- prefer pure unit tests for math, lifecycle calculations, and error classification
- keep failures in the current test or subtest when practical; helpers should return errors or diffs instead of failing the test themselves
- never call `t.Fatal`, `t.FailNow`, or similar methods from spawned goroutines

Validation expectations:

- backend changes: `go test ./...` and `go vet ./...`
- UI changes: `bun run test`, `bun run lint`, and `bun run build`
- formatting and line endings (any change touching tracked text files): `pwsh scripts/check-format.ps1` (or `bash scripts/check-format.sh`). The script gates `gofmt -l .` and CRLF residue against `.gitattributes`; both must be empty before reporting completion. New files you create must be LF — `.editorconfig` and `.gitattributes` are the contract
- route or embed contract changes: verify `ui/vite.config.ts`, `ui/src/app/routes.ts`, `pkg/control/http_routes.go`, `pkg/control/dashboard_handler.go`, and that `ui/dist/index.html` still exists
- feature or settings contract changes: verify the Go route and workspace builders, `ui/src/api/control.ts`, and the adapters or pages consuming the changed fields
- exported Go API, doc, or error-contract changes: update nearby tests and doc comments that pin the new behavior
- if a relevant validation step was not run, say so explicitly

Build and release commands:

- `go build ./...`
- `go test ./...`
- `go vet ./...`
- `go test -tags integration ./...`
- from `ui/`: `bun run test`, `bun run lint`, `bun run build`
- canonical release command: `release -m "<conventional commit subject>" -y --promote`

Release rules:

- do not push to `main` directly
- do not call `git tag` by hand
- do not bundle unrelated changes into a single release commit
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
