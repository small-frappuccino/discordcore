# PR 1: Unified Interaction Router

## Purpose

This PR introduces one interaction router for slash commands, autocomplete, components, and modals.

It does not redesign the full command schema yet.
It does not change command sync scope yet.
It does not migrate every command package to a new API yet.

The goal of PR 1 is to remove the current split where slash and autocomplete go through the core router while runtime config components and modals install a separate session handler.

## Current Problem

The current interaction flow is split across two paths:

- `pkg/discord/commands/core/registry.go` handles slash commands and autocomplete
- `pkg/discord/commands/runtime/runtime_config_commands.go` exposes a second session handler for components and modals
- `pkg/discord/commands/handler.go` wires both paths into the session lifecycle separately

That split creates three problems:

1. There is no single dispatch pipeline for interaction behavior.
2. Guild filtering, telemetry, and error handling are not applied uniformly.
3. Runtime config is an architectural exception instead of a first-class interaction route.

## PR 1 Outcome

After PR 1:

- one session interaction handler is registered for commands-related interactions
- one core router dispatches slash, autocomplete, components, and modals
- runtime config components and modals register into the same router instead of using a parallel handler
- slash command packages keep working through compatibility adapters
- no user-facing behavior should change except for more consistent routing and lifecycle behavior

## Explicit Non-Goals

PR 1 must not include these changes:

- no rewrite of the command abstraction into a full Command Layer v2 schema
- no change to global versus guild command synchronization behavior
- no broad permission model redesign
- no new product behavior for `channels.commands`
- no full migration of every package to middleware-driven handlers
- no command metadata expansion for localizations, contexts, integration types, or NSFW

## Success Criteria

PR 1 is successful only if all of these are true:

1. `CommandHandler` installs exactly one interaction handler into the Discord session.
2. The core router dispatches all four interaction kinds: slash, autocomplete, component, and modal.
3. Runtime config no longer needs a separate `session.AddHandler(...)` wiring path.
4. Existing slash command tests continue to pass.
5. New tests prove component and modal dispatch through the unified router.
6. Shutdown and rollback still clean up all registered interaction handling state.

## Design Constraints

- preserve package ownership: runtime-specific logic stays in `pkg/discord/commands/runtime/`
- avoid regrowing `pkg/discord/commands/core/registry.go` into a larger hotspot
- preserve the current `CommandHandler` and `CommandManager` lifecycle semantics as much as possible
- keep the first PR narrow and reversible
- introduce compatibility adapters rather than forcing a full command API migration

## Planned Shape

### 1. New interaction kinds

Introduce an internal interaction-kind model in the core router:

- slash
- autocomplete
- component
- modal

This kind should be resolved once at the router entrypoint and passed through the dispatch pipeline.

### 2. Unified route key

Introduce an internal route key that separates interaction kind from route path.

Suggested shape:

```go
type InteractionKind int

const (
	InteractionKindSlash InteractionKind = iota
	InteractionKindAutocomplete
	InteractionKindComponent
	InteractionKindModal
)

type RouteKey struct {
	Kind          InteractionKind
	Path          string
	FocusedOption string
}
```

Routing behavior:

- slash: path comes from command plus subcommand path
- autocomplete: same path plus focused option
- component: path comes from a stable custom ID prefix
- modal: path comes from a stable custom ID prefix

### 3. Unified execution context

Extend the core execution context with unified interaction-routing data.

Add fields conceptually equivalent to:

- interaction kind
- normalized route path
- focused option
- custom ID
- component or modal prefix

This avoids scattering interaction-type parsing across router and feature packages.

### 4. Separate registries by interaction kind

Do not use one untyped map for all interaction handlers.

Add explicit registries for:

- slash handlers
- autocomplete handlers
- component handlers
- modal handlers

This keeps collisions visible and preserves type clarity.

### 5. Custom ID prefix contract

The router must not understand full encoded component or modal state.

It should only route by stable prefix, for example:

- `runtime_config`
- `partner_sync`
- `admin_metrics_watch`

The feature package remains responsible for parsing encoded state after routing.

## File Plan

### Files To Create

Create these new files under `pkg/discord/commands/core/`:

1. `router.go`
Purpose:
- own the unified `HandleInteraction` entrypoint
- resolve interaction kind
- build unified route key
- dispatch to the correct registry

2. `route_registry.go`
Purpose:
- own slash, autocomplete, component, and modal registries
- expose registration methods for each route type
- keep route storage out of `registry.go`

3. `route_path.go`
Purpose:
- resolve slash path from command and subcommand structure
- resolve autocomplete path from the same slash path
- resolve component and modal prefixes from custom IDs

4. `middleware.go`
Purpose:
- define middleware types for unified interaction execution
- provide a minimal chain builder even if PR 1 uses only a small set of middlewares

5. `router_test.go`
Purpose:
- cover unified dispatch behavior across all interaction kinds

6. `route_path_test.go`
Purpose:
- pin path resolution and custom ID prefix extraction behavior

7. `route_registry_test.go`
Purpose:
- pin registration and lookup semantics for each interaction kind

### Files To Alter

Alter these existing files:

1. `pkg/discord/commands/handler.go`
Changes:
- remove the separate runtime interaction session handler wiring
- register runtime interaction routes into the core router instead
- collapse lifecycle cleanup to one interaction handler registration path

2. `pkg/discord/commands/core/registry.go`
Changes:
- keep command synchronization and slash compatibility helpers only
- remove ownership of the main `HandleInteraction` dispatch path
- stop growing this file as the interaction router hotspot

3. `pkg/discord/commands/core/types.go`
Changes:
- extend context types with unified interaction-routing data
- add handler type definitions needed for component and modal registration

4. `pkg/discord/commands/runtime/runtime_config_commands.go`
Changes:
- replace `HandleRuntimeConfigInteractions(...)` as the primary integration seam
- add a registration function that registers component and modal handlers with the core router
- keep existing runtime config state parsing and rendering logic local to the package

5. `pkg/discord/commands/handler_lifecycle_test.go`
Changes:
- update lifecycle assertions for a single interaction handler path
- add rollback and shutdown checks that cover the unified router setup

6. `pkg/discord/commands/runtime/runtime_config_commands_error_test.go`
Changes:
- adapt tests so runtime config component and modal interactions flow through the unified router

7. `pkg/discord/commands/core/manager_lifecycle_test.go`
Changes:
- confirm one unified interaction handler registration still rolls back correctly

### Files To Split By Responsibility

PR 1 should move responsibility out of these hotspots without necessarily renaming them:

- from `pkg/discord/commands/core/registry.go` into `router.go`, `route_registry.go`, and `route_path.go`
- from `pkg/discord/commands/handler.go` into router registration paths owned by the runtime package

This is a code split, not necessarily a filesystem move or package rename.

### Files Not To Touch In PR 1 Unless Required

- `pkg/app/bot_runtime_runner.go`
- moderation command handlers
- partner command handlers except for compatibility with router registration
- metrics command handlers except for compatibility with router registration
- control-plane feature catalog or readiness logic

## Registration API To Introduce

The core router needs explicit registration methods for non-slash interactions.

Suggested minimal API:

```go
func (cr *CommandRouter) RegisterCommand(cmd Command)
func (cr *CommandRouter) RegisterAutocomplete(path string, option string, handler AutocompleteHandler)
func (cr *CommandRouter) RegisterComponent(prefix string, handler ComponentHandler)
func (cr *CommandRouter) RegisterModal(prefix string, handler ModalHandler)
```

Compatibility note:

- slash command packages can keep using the existing command registration path in PR 1
- only runtime config needs to adopt component and modal registration immediately

## Middleware Scope For PR 1

Introduce middleware infrastructure now, but keep usage minimal.

PR 1 should apply at least these shared behaviors in one place:

1. nil guard
2. guild filter
3. perf event creation
4. shared error mapping

Optional if cheap during implementation:

5. panic recovery
6. consistent not-found behavior by interaction kind

Do not block PR 1 on a full middleware framework.

## Runtime Config Migration Plan

Runtime config is the first real non-slash route to move.

### Current state

- runtime config exposes `HandleRuntimeConfigInteractions(...)`
- `CommandHandler` installs it through a second `session.AddHandler(...)`

### Target state in PR 1

- runtime config exposes `RegisterInteractionRoutes(router, configManager)` or equivalent
- component route registers under the runtime config custom ID prefix
- modal route registers under the same runtime config prefix contract
- existing `handleComponent` and `handleModalSubmit` logic stays in the runtime package and is reused internally

### Why this is the correct first migration

- it removes the current architectural exception
- it exercises both component and modal routing
- it does not force immediate redesign of slash command definitions

## Test Plan

PR 1 needs new or updated tests for all of the following:

1. slash route dispatch still works
2. autocomplete dispatch still works
3. component route dispatch by prefix works
4. modal route dispatch by prefix works
5. unknown component prefix is ignored or handled predictably
6. unknown modal prefix is ignored or handled predictably
7. guild filter applies to component and modal interactions the same way it applies to slash interactions
8. setup rollback clears unified interaction handler state
9. shutdown remains idempotent
10. runtime config interaction tests pass via the unified router instead of a separate session callback

## Validation Commands

Minimum validation for PR 1:

- `go test ./pkg/discord/commands/core/...`
- `go test ./pkg/discord/commands/runtime/...`
- `go test ./pkg/discord/commands/...`
- if runtime setup wiring changes widen further, run `go test ./pkg/app/...`

## PR 1 Task Breakdown

### Step 1

Add interaction kind, route key, route registry, and path resolver in new core files.

### Step 2

Move unified dispatch out of `registry.go` and into `router.go`.

### Step 3

Extend context and handler types to support component and modal execution.

### Step 4

Expose runtime config route registration into the core router.

### Step 5

Remove the parallel runtime session handler from `handler.go`.

### Step 6

Update lifecycle and runtime interaction tests.

### Step 7

Run focused command package tests, then broader validation if wiring spread requires it.

## Review Checklist

Before merging PR 1, verify:

1. there is only one `session.AddHandler(...)` path for commands-related interactions
2. runtime config component and modal interactions no longer depend on a separate closure installed by `CommandHandler`
3. the core router no longer silently drops non-slash interaction types that belong to registered routes
4. slash and autocomplete behavior is unchanged for existing commands
5. new files reduced hotspot pressure instead of growing `registry.go`

## Follow-On Work After PR 1

PR 1 intentionally stops before these next steps:

- PR 2: defer policy and long-running interaction handling
- PR 3: richer command schema metadata and deterministic ordering improvements
- PR 4: sync scope redesign for guild-scoped versus global command exposure