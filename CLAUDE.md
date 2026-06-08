# discordcore — Claude Project Instructions

This is the repository-wide contract for Claude working in `discordcore`. It defines durable repository facts, ownership boundaries, load-bearing invariants, validation expectations, and working-style rules. Treat every rule below as a strict constraint unless the user explicitly overrides it.

## Project Overview

`discordcore` is a production Go + React monorepo powering a Discord bot. It owns the Go runtime and orchestration for Discord-facing behavior (slash-commands-first), a control API with a complementary React dashboard, canonical config and runtime state, Postgres-backed persistence and migrations, and the `//go:embed` React dashboard payload under `ui/`.

## Non-Negotiables

- Maintain this repo like a production system. Optimize for correctness, operational reliability, maintainability, observability, and low-drift changes that match local patterns.
- Prefer narrow, source-backed changes over broad rewrites. Do not treat this repo like a greenfield project.
- Do exactly what the user requested — nothing more, nothing less. The user's message is the contract; the deliverable is the smallest safe change that satisfies it.
- **No Silent Placeholders**: All code must be complete, executable, with proper error handling. No `// ... existing code ...` shortcuts unless explicitly permitted.
- **Course Correction (Pushback)**: Explicitly object to flawed premises or suboptimal architectural paths before executing. Guide to the optimal path.
- **Binary Certainty**: No hedging or probabilistic language. Assert facts with conviction or provide a concrete verification path. Zero fabrication.
- **Context Integration**: Seamlessly apply provided context. Do not restate or summarize the provided architecture back to the user.
- **Audits & Analysis**: Focus on race conditions, I/O bottlenecks, transactional regressions, trade-offs, and failure modes instead of surface-level syntax or generic best practices. Contrast options side-by-side explicitly defining victory conditions.
- Do not bundle adjacent improvements, cleanup, renames, or reformatting with the requested change.
- Do not add features, options, flags, configuration, logging, telemetry, metrics, or hooks the user did not ask for.
- Do not anticipate future requirements; if a broader change seems warranted, surface it explicitly instead of silently widening scope.
- When the request is ambiguous, pick the smallest defensible interpretation and state the assumption briefly.
- Expect direct, task-focused instructions, occasionally in Portuguese (e.g., "Execute task M3", "Como isso fica..."). Acknowledge and proceed without conversational filler.
- New files must use LF line endings — `.editorconfig` and `.gitattributes` are the contract.

## Product Interaction Model

`discordcore` is slash-commands-first:

- Slash commands are the primary end-user interface for bot capabilities, routine actions, and conversational flows.
- The dashboard is a complementary surface for setup, review, bulk edits, diagnostics, and recovery flows that are awkward in chat.
- Do not make a dashboard page the canonical or exclusive path for routine bot actions unless the product owner explicitly asks for that tradeoff.
- When shaping a feature, define the slash-command workflow first, then add only the UI needed to support it cleanly.
- If a page mainly mirrors command usage, command output, or routine action buttons, compress it, move it behind diagnostics, or remove it.

## Boundaries and Ownership

Use this map before editing:

- `cmd/discordmain/`: principal main runtime entrypoint
- `cmd/discordqotd/`: principal QOTD runtime entrypoint
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

Package ownership rules:

- Do not move reusable logic into `cmd/`.
- Do not reimplement config or state rules in `pkg/control/` when they belong in `pkg/files/`.
- Do not move Discord runtime behavior into the dashboard layer.
- Keep routine user workflows in Discord when a slash command is the natural surface.
- Use the dashboard for setup, review, bulk edits, diagnostics, and cross-feature visibility — not as a duplicate command console.
- Keep router and provider entrypoints thin when a focused sibling file or service already exists.

## Working Style

Work in this order unless the task clearly calls for something narrower:

1. Identify the exact contract, workflow, or failure being changed.
2. Inspect the minimum live source needed to find the owning seam.
3. Make the smallest change that preserves local ownership and patterns.
4. Update nearby tests that pin the changed behavior.
5. Run the matching validation before considering cleanup.

General expectations:

- Prefer source code and current tests over stale docs.
- Build context locally around the owning abstraction, not by mapping the whole repo.
- Preserve public APIs and established naming unless the task requires change.
- Do not introduce speculative architecture or "future-proofing" abstractions.
- If a hotspot file must change, read enough neighboring code to preserve its decomposition rules first.
- If you notice an unrelated issue while working, mention it separately after the requested work is complete.

## Context-Mode Tooling

`context-mode` is installed in this environment as a Claude Code plugin (MCP server plus routing hooks) for context-window optimization. When it is available, prefer its sandboxed tools over raw shell for any step that would otherwise spill a large or noisy payload into context.

- Use `ctx_execute` / `ctx_execute_file` / `ctx_batch_execute` to process, filter, or analyze high-volume data in the sandbox — parsing JSON, extracting errors from logs, scanning build output, walking dependency trees — instead of piping raw text through Bash/PowerShell or `cat`.
- Index sources you consult repeatedly — `ctx_fetch_and_index` for web docs, `ctx_index` for local files or directories — then retrieve focused snippets with `ctx_search` instead of re-reading whole files.
- The PreToolUse hook auto-compresses many large command outputs; do not fight it, and reach for the `ctx_*` tools yourself whenever you are deliberately handling a big payload rather than letting a long log or diff land in context.
- This is a local, per-user optimization, not a hard dependency: when context-mode is absent, fall back to the normal tools. Its presence changes only HOW output is gathered — never WHAT you do, and never which commands are authoritative. The canonical commands in Testing and Validation still run as written.

## Code Commentary

### Go doc-comment baseline

- Exported types, functions, methods, package-level vars, constants, and the package itself carry a doc comment in stdlib style: start with the identifier name, complete sentences, adjacent to the declaration with no blank line between.
- The first sentence is the summary `go doc` and pkg.go.dev consumers see; describe behavior, not just the signature.
- Document non-obvious contracts explicitly: concurrency guarantees (default assumption is single-goroutine; state when a type or method deviates), returned-error semantics including sentinels and `errors.Is` branches, ownership of returned values, and lifecycle ordering (`Close`/`Stop`, idempotency).
- `Deprecated:` is a structured marker; use it on legacy fields and aliases (see `pkg/files/types.go`) rather than free-form "old"/"legacy" prose.
- Doc comments on exported APIs describe WHAT — the audience reads them through `go doc`, not the source; inline `//` comments inside function bodies stay WHY-only.
- Multi-paragraph contract docs with section headers (`# Contract`, `# Parameters`, `# Why not X`) are the local idiom for load-bearing seams; see `pkg/qotd/official_post_state_transition.go` and `pkg/qotd/observability.go`.

### TypeScript and UI

Types in `ui/src/api/control.ts` and component prop interfaces are the contract; add JSDoc only when behavior cannot be expressed in the type (units, side effects, lifecycle ordering).

### Across both languages

- `TODO`/`FIXME` markers carry an owner (`TODO(user): …`) or a specific triggering event; ownerless `TODO:` is an anti-pattern.
- Do not preserve removed code as commented-out blocks, `// removed` markers, or `_unused` placeholders — git history is the museum.
- Do not write comments that restate the identifier or the next line; do not narrate straight-line code step by step.
- Do not add or rewrite comments on code that was not changed in the current task; comment churn pollutes blame.
- Do not embed history references ("added for ticket X", "renamed in 2024") — those belong in commit messages.

## Agent Dynamic And Communication

- **Tone & Style**: Adopt a strictly neutral, dense, technical tone. Remove pleasantries, emojis, preambles, and summaries. Do not reiterate prompts.
- **Verbosity**: Scale response length to complexity; answer trivial questions in ≤3 lines.
- **Emphasis & Language**: Bold only vital technical identifiers (metrics, flags, IDs). Keep technical terminology in English.
- **Objectivity & Bias**: Retain directives unmodified. Enforce steelmanning of competing frameworks/design patterns in architectural disputes to evaluate trade-offs without subjective endorsement. Restrict personal opinions to labeled non-political ethical/philosophical debates.

## Design and Implementation Rules

### Source of truth

- Trust source code and current tests first.
- Treat `README.md` and `UI_RULES.md` as intent and context, not implementation truth.
- Update stale docs instead of forcing code to match outdated prose.

### Architecture

- Prefer small, reversible, testable changes.
- Reuse existing seams before introducing new helpers or layers.
- Keep new public types, exported functions, routes, and settings to a minimum.
- Extend the closest existing sibling file before creating a new one.
- Do not add `util`, `helper`, or `common` packages; use the closest owning package or sibling seam.
- When a wide service needs splitting, prefer narrow consumer-side interfaces over concrete implementation fragmentation.
- High-drift decisions (broad rewrites or large diffs) are acceptable IF and ONLY IF they significantly elevate code quality compared to before.

### Go idioms (treat as strict code-review rules)

- **Clear is better than clever**: no overly clever, dense, or heavily abstracted code; avoid `reflect`, `unsafe`, and complex generics; write straightforward, top-to-bottom code.
- **The bigger the interface, the weaker the abstraction**: define small interfaces (1–3 methods) at the consumer site, not where types are implemented; do not proactively create interfaces for single implementations.
- **Make the zero value useful**: design structs so they are safe and useful without explicit initialization; avoid `New...()` functions if a zero value suffices.
- **interface{} says nothing**: avoid `any` or `interface{}`; use strong, explicit typing; if forced to use `any`, document exactly why compile-time safety was impossible.
- **A little copying is better than a little dependency**: duplicate small snippets instead of introducing shared `util` packages that tangle unrelated domains.
- **Don't panic**: never use `panic` in business logic; always return `error`; reserve `panic` exclusively for unreachable states or `init()` failures where the application absolutely cannot start.
- **Don't communicate by sharing memory, share memory by communicating**: prefer channels or dedicated single-threaded event loops; if using `sync.Mutex`, keep the critical section small and never perform I/O while holding a lock.
- **Errors are values**: don't just check errors, handle them gracefully; never discard errors (`_ = err`); always provide context when bubbling them up.

### Go and backend rules

- Put `context.Context` first when present.
- Prefer `:=` for new values and `=` when reusing outer `ctx` or `err`.
- Wrap inspectable errors with `fmt.Errorf("operation: %w", err)`.
- Use sentinel errors and `errors.Is` / `errors.As` when callers branch on failure.
- Logging goes through `pkg/log` or area-specific helpers, not stdlib `log`.
- Include relevant guild, channel, or user identifiers in operational logs.
- Never log secrets, OAuth credentials, tokens, or private message content.
- Validate only at system boundaries: HTTP input, OAuth callbacks, Discord payloads, and external rows or documents.
- Treat `ConfigManager.Config()` and `GuildConfig()` results as read-only snapshots; persist through the existing update helpers.

### Dashboard and TypeScript rules

- Keep dashboard request and response shapes in `ui/src/api/control.ts`.
- Keep route strings and legacy aliasing in `ui/src/app/routes.ts`.
- Keep navigation registry data in `ui/src/app/navigation.ts`.
- Use `DashboardSessionContext` and existing feature hooks before adding ad hoc fetch logic.
- Prefer existing primitives: `PageHeader`, `FeatureWorkspaceLayout`, `SurfaceCard`, `StatusBadge`, `EmptyState`.
- Avoid `any`.
- Use `import.meta.env.BASE_URL` for embedded asset paths.

### Canonical dashboard contracts

- Canonical dashboard base path is `/manage/`.
- `/dashboard/` is a legacy compatibility alias, not the primary route.
- Backend SPA handling lives in `pkg/control/dashboard_handler.go`.
- Dashboard HTTP registration lives in `pkg/control/http_routes.go`.
- Route construction and legacy mapping live in `ui/src/app/routes.ts`.
- Vite build base lives in `ui/vite.config.ts`.
- The SPA must not intercept `/v1/*` or `/auth/*`.
- If dashboard routing changes, update backend, frontend, tests, docs, and embed assumptions together.

### Structural Mandates (State-of-the-Art Architecture)

- **Graceful Lifecycle Management**: Mandate context-aware cancellation pipelines utilizing `context.Context` and `sync.WaitGroup` to orchestrate isolated sub-service teardowns safely. Avoid serializing runtime states via `sync.RWMutex` which introduces write-starvation and deadlock vulnerabilities.
- **TypeScript API Resiliency**: Enforce mandatory exponential backoff and randomized network jitter on all retry mechanisms for HTTP 502/504 errors to prevent thundering herd state collapses.
- **Observability Accessors**: Mandate strict dependency validation during the application boot phase to ensure the metrics pipeline successfully attaches prior to the primary event loop, ensuring nil-safe accessors (`NopMetrics`) do not mask critical initialization failures.

## Config Schema Evolution Pattern

When a persisted config field changes shape:

1. Add the new field on the public struct in `pkg/files/types.go`.
2. Keep the legacy JSON key alive only inside the local `raw*` unmarshal struct.
3. Migrate legacy to canonical at decode time; do not emit the legacy key on the write path.
4. Update cloning, normalization, and `IsZero` logic together.
5. Update all callers and tests in the same change; do not leave shim-then-cleanup work behind.

## Testing and Validation

### Testing style

- Keep tests close to the changed package.
- Prefer deterministic seams already present in the repo.
- Do not require live Discord access.
- Use isolated Postgres helpers only where the package already follows that pattern.
- Mock the narrowest interface the unit under test actually exercises.
- Prefer pure unit tests for math, lifecycle calculations, and error classification.
- Keep failures in the current test or subtest when practical; helpers should return errors or diffs instead of failing the test themselves.
- Never call `t.Fatal`, `t.FailNow`, or similar methods from spawned goroutines.

### Validation expectations

- Backend changes: `go test ./...` and `go vet ./...`
- UI changes: `bun run test`, `bun run lint`, and `bun run build`
- Formatting and line endings (any change touching tracked text files): `pwsh scripts/check-format.ps1` (or `bash scripts/check-format.sh`). The script gates `gofmt -l .` and CRLF residue against `.gitattributes`; both must be empty before reporting completion.
- Route or embed contract changes: verify `ui/vite.config.ts`, `ui/src/app/routes.ts`, `pkg/control/http_routes.go`, `pkg/control/dashboard_handler.go`, and that `ui/dist/index.html` still exists.
- Feature or settings contract changes: verify the Go route and workspace builders, `ui/src/api/control.ts`, and the adapters or pages consuming the changed fields.
- Exported Go API, doc, or error-contract changes: update nearby tests and doc comments that pin the new behavior.
- If a relevant validation step was not run, say so explicitly.

### Build and release commands

- `go build ./...`
- `go test ./...`
- `go vet ./...`
- `go test -tags integration ./...`
- From `ui/`: `bun run test`, `bun run lint`, `bun run build`
- Canonical release command: `release -m "<conventional commit subject>" -y --promote`

### Release rules

- Do not push to `main` directly.
- Do not call `git tag` by hand.
- Do not bundle unrelated changes into a single release commit.
- Restrict line-ending or encoding normalization commits to that scope so the diff stays trivial to audit; stylistic reformatting (e.g., gofmt-equivalent one-liner to multi-line conversions) goes in a separate `style:` or `refactor(style):` commit.
- Treat the conventional-commit subject as the release message and changelog entry.

## Hotspots and Cautions

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

These seams are intentional — keep them decomposed:

- `pkg/control/features_routes.go`: router and dispatch only; feature catalog, workspace shaping, readiness, blockers, patch flows, and toggle bindings belong in focused services or `features_*.go` siblings.
- `pkg/control/discord_oauth.go`: shared OAuth types, constants, provider construction, and permission parsing only; flow logic belongs in `discord_oauth_*.go` siblings or the dedicated service.
- `pkg/storage/postgres_store.go`: `Store` type, bootstrap, schema init entrypoint, and shared SQL helpers only; domain behavior belongs in focused `postgres_store_*.go` files.
- `pkg/discord/logging/monitoring.go`: lifecycle and orchestration only; gateway handlers, reactions, cache loops, permission mirroring, and similar specifics belong in focused `monitoring_*.go` files.

Additional caution:

- `pkg/discord/logging/` is coordination-heavy; avoid regrowing broad orchestration files when a sibling seam exists.
- QOTD legacy compatibility is concentrated in specific migration and compatibility seams, not across the whole runtime.
- Do not widen already large pages unless the task is truly local to that page.

## Load-Bearing Invariants

### QOTD subsystem

- Publish idempotency is three-layered: a 16-hex `Nonce` on `QOTDOfficialPostRecord` sent to Discord with `enforce_nonce`, a partial unique index on `(guild_id, deck_id, publish_date_utc, publish_mode='scheduled')`, and the `resolvePublishNowConflict` / adopt-existing-thread recovery branches; new publish paths must keep all three intact.
- `OfficialPostState` distinguishes `failed` (retryable, reconcile loop retries) from `abandoned` (terminal, requires admin action); `isUnrecoverableDiscordPublishError` is the gate between them.
- Thread-state errors classify through three buckets: `isMissingDiscordThreadError`, `isUnmanageableDiscordThreadError`, and the retryable default; new Discord error codes belong in one of those classifiers, not an ad hoc branch.
- `syncLiveOfficialPost` short-circuits when `post.State == lifecycle.State`; reconcile-style code added later must follow the same skip-the-API-call pattern.
- Official-post transitions that touch both Discord and the DB must route through `applyOfficialPostThreadTransition` in `pkg/qotd/official_post_state_transition.go`; do not inline Discord-first plus DB-second state changes elsewhere.
- Observability lives behind the `qotd.Metrics` interface in `pkg/qotd/observability.go`; instrument through typed interface methods, not direct Prometheus or expvar calls.
- Discord thread auto-archive is `defaultThreadAutoArchiveMinutes` with `fallbackThreadAutoArchiveMinutes` on validation rejection; `archiveOfficialPost` should only flip `Locked=true`.
- Suppression is `QOTDConfig.SuppressScheduledPublishDatesUTC []string`; the legacy single-date JSON key only survives in the local unmarshal shim.

### QOTD runtime lifecycle

- `pkg/discord/qotd/RuntimeService` must reinitialize `stopCh` and `stopOnce` on `Start` after a prior `Stop`, or restart will silently exit after the startup cycle.
- Stopping the QOTD runtime should cancel in-flight per-guild operations and suppress expected context-canceled shutdown noise.
- Stale scheduled-publish suppression tokens should be trimmed automatically so config does not drift conservative forever.

### Reaction block runtime

- Reaction block enforcement lives in `pkg/discord/logging/ReactionEventService`, ahead of metrics-store and emit-policy gating.
- `resolveMonitoringWorkloadState` must keep `reactionEventService` enabled when any guild has non-empty `GuildConfig.ReactionBlocks`, even if reaction logs are disabled.
- Blocked reaction removal uses `discordgo.Session.MessageReactionRemove` with unicode emoji names for built-ins and custom emoji IDs for guild emoji.

## Reporting

- Lead with what changed and why when the work is substantial.
- Include validation run status and any skipped validation.
- Mention remaining risk or follow-up drift only when it is real.
- Keep unrelated findings separate from the requested work.
