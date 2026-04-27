# PR 1: Unified Interaction Router

## Status

This document now reflects the shape currently implemented in the repository.

PR 1 is effectively complete for the interaction-router slice.
What remains in the tree is mostly intentional carryover or next-phase cleanup,
not missing core router work.

The only known repo-wide red unrelated to this PR is the QOTD test failure in
`pkg/discord/qotd/publisher_test.go`.

## Purpose

PR 1 introduced one interaction router for slash commands, autocomplete,
components, and modals.

It did not redesign the full command abstraction into Command Layer v2.
It did not change command synchronization scope.
It did not remove every compatibility API in one step.

The main goal of PR 1 was to remove the split where slash and autocomplete went
through the core router while runtime config components and modals used a
parallel session handler path.

## Implemented Outcome

After the current PR 1 implementation:

- one commands-related interaction handler is installed into the Discord session
- one core router dispatches slash, autocomplete, component, and modal interactions
- runtime config registers into the same core router instead of installing a parallel handler
- slash command trees can derive both slash and autocomplete routing from the same source
- components and modals register through the same declarative route catalog used by the core router
- existing slash command packages still work through compatibility adapters where needed
- no intentional user-facing behavior change was introduced beyond more consistent routing and lifecycle behavior

## What Actually Landed

### 1. One session handler path

The commands lifecycle now installs one interaction callback through
`cm.session.AddHandler(cm.router.HandleInteraction)`.

Rollback and shutdown both clean up that single registration path.

### 2. Unified interaction kinds and route key

The core router resolves one normalized `InteractionRouteKey` with:

- `Kind`
- `Path`
- `FocusedOption`
- `CustomID`

Routing behavior is now:

- slash: full command path, including nested subcommand groups
- autocomplete: the same full command path plus the focused option name
- component: exact stable route ID extracted from the custom ID before encoded state
- modal: exact stable route ID extracted from the custom ID before encoded state

Component and modal routing no longer rely on loose prefix matching.
The router extracts the stable route ID and routes by exact match.

### 3. Unified dispatch entrypoint

The main dispatch path lives in `pkg/discord/commands/core/router.go`.

That file now owns:

- interaction kind resolution
- route-key resolution
- dispatch into slash, autocomplete, component, and modal execution paths
- common route execution via the middleware pipeline

`pkg/discord/commands/core/registry.go` no longer owns the main interaction
dispatch path.

### 4. Middleware actually in use

PR 1 shipped real default middleware, not only scaffolding.

Current default middleware covers:

- telemetry for routed interactions
- slash permission gating
- slash error mapping

The middleware chain is installed by default in `NewCommandRouter`.

### 5. Final registry shape that landed

The code did not end on four completely separate top-level maps.
Instead, it now uses one normalized interaction route catalog keyed by route path
or route ID, with typed entries per interaction kind.

Conceptually, one route entry can carry:

- slash handler
- autocomplete handler
- component handler
- modal handler

This is represented by `InteractionRouteBinding` in the core types and stored
through `RegisterInteractionRoute(...)` and `RegisterInteractionRoutes(...)`.

That final shape is denser than the original draft and better matches the
implemented direction of the router.

### 6. Slash registration API moved forward beyond the original draft

The current preferred slash registration path is:

- `RegisterSlashCommand(cmd)`
- `RegisterSlashSubCommand(parent, subcmd)`

Those APIs update both:

- the command sync registry
- the derived slash and autocomplete route catalog

This removed the need for manual `RegisterSlashRoute(...)` calls in the main
production slash registrars.

### 7. Autocomplete now shares the slash tree source

Autocomplete is no longer only an independently registered side-path.

The core now supports deriving autocomplete handlers from the same slash command
tree through `AutocompleteRouteProvider`.

For simple commands, `SimpleCommand.WithAutocomplete(...)` is available.
For custom command types, the route can expose
`AutocompleteRouteHandler() AutocompleteHandler` directly.

This means slash and autocomplete can share the same route tree declaration.

### 8. Runtime config is no longer an architectural exception

Runtime config now registers through the core router and no longer installs a
parallel session handler.

It also moved into a feature-local interaction catalog so that the feature owns:

- its slash tree registration
- its component bindings
- its modal bindings

without pushing that wiring back into the general config or core lifecycle code.

### 9. Feature-local catalog pattern has started

PR 1 went beyond the original minimal migration and started a small local
catalog pattern for features with richer interaction surfaces.

Currently this exists in:

- `pkg/discord/commands/runtime/runtime_config_catalog.go`
- `pkg/discord/commands/config/webhook_embed_update_catalog.go`

This is not yet a repo-wide mandate, but it is now a proven local pattern for
keeping interaction declarations close to the owning feature.

### 10. First production autocomplete path shipped

The first real production autocomplete path now exists in the `/config`
webhook-embed workflow.

`message_id` autocomplete for read, update, and delete is derived through the
new route-tree contract and resolves values from the persisted webhook-embed
update entries in the selected scope.

This means the new autocomplete contract is no longer exercised only by tests
or examples.

## Production Registrars Already Migrated

The following command areas are already on the current PR 1 registration shape:

- config
- runtime config
- metrics
- partner
- moderation
- admin

These registrars now primarily use `RegisterSlashCommand(...)` rather than
manually wiring slash route paths one by one.

## Intentional Carryover Still In The Tree

These items still exist on purpose and are not considered PR 1 blockers.

### Compatibility wrappers in `core`

The following compatibility APIs remain:

- `RegisterCommand(...)`
- `RegisterSubCommand(...)`
- `RegisterAutocomplete(...)`
- `RegisterComponentHandler(...)`
- `RegisterModalHandler(...)`

They currently forward into the newer APIs.

This carryover is intentional because PR 1 prioritized a safe router unification
over a full compatibility cleanup. Removing these wrappers can happen in a
later phase after all call sites are comfortably migrated.

### Typed convenience route helpers

The following helpers also remain and forward into the unified interaction
binding API:

- `RegisterSlashRoute(...)`
- `RegisterAutocompleteRoute(...)`
- `RegisterComponentRoute(...)`
- `RegisterModalRoute(...)`

They are useful as thin adapters even though the implemented core now centers on
`RegisterInteractionRoute(...)` and `RegisterInteractionRoutes(...)`.

### Feature-local catalog rollout is partial

The local catalog pattern exists and is proven, but it has not been applied to
every command package.

That is considered follow-on cleanup, not missing router work for PR 1.

## What PR 1 Did Not Attempt

These remain explicit non-goals for this PR:

- no Command Layer v2 redesign
- no synchronization scope redesign for global versus guild exposure
- no broad permission model redesign
- no localization, contexts, NSFW metadata, or integration-type expansion
- no full conversion of every feature package into local interaction catalogs
- no removal of every compatibility API in the first unification step

## Validation Status

The interaction-router slice has been repeatedly validated with focused command
package checks.

Relevant green validations include:

- `go test ./pkg/discord/commands/core`
- `go test ./pkg/discord/commands/runtime`
- `go test ./pkg/discord/commands/config`
- `go test ./pkg/discord/commands/...`
- `go vet ./...`

The full repo test suite is not fully green because of the existing QOTD test
failure outside this PR:

- `pkg/discord/qotd/publisher_test.go`

That failure does not currently indicate an interaction-router defect.

## PR 1 Review Checklist

PR 1 should be considered technically complete only if all of these remain true:

1. there is only one `session.AddHandler(...)` path for commands-related interactions
2. the core router dispatches slash, autocomplete, component, and modal interactions
3. runtime config does not depend on a parallel session handler path
4. slash and autocomplete route through the same normalized route-key model
5. component and modal routing use the same core route catalog
6. rollback and shutdown still clean up the single installed interaction handler
7. focused command package tests remain green

The current tree satisfies that checklist.

## Files That Matter Most In The Final Shape

Core router shape:

- `pkg/discord/commands/core/router.go`
- `pkg/discord/commands/core/route_registry.go`
- `pkg/discord/commands/core/route_path.go`
- `pkg/discord/commands/core/middleware.go`
- `pkg/discord/commands/core/types.go`
- `pkg/discord/commands/core/registry.go`

Lifecycle and setup:

- `pkg/discord/commands/handler.go`
- `pkg/discord/commands/core/manager_lifecycle_test.go`
- `pkg/discord/commands/handler_lifecycle_test.go`

Feature integrations that prove the pattern:

- `pkg/discord/commands/runtime/runtime_config_commands.go`
- `pkg/discord/commands/runtime/runtime_config_catalog.go`
- `pkg/discord/commands/runtime/runtime_config_commands_error_test.go`
- `pkg/discord/commands/config/config_commands.go`
- `pkg/discord/commands/config/webhook_embed_update_catalog.go`
- `pkg/discord/commands/config/webhook_embed_update_handlers.go`
- `pkg/discord/commands/config/webhook_embed_update_autocomplete_test.go`

## Remaining Work After PR 1

There is no obvious missing core interaction-router implementation left in this
PR slice.

What remains after PR 1 is next-stage work, for example:

- expanding the feature-local catalog pattern to more command packages
- retiring compatibility wrappers once enough call sites migrate
- deciding how much of the command surface should move toward richer local declarative schemas
- fixing or isolating unrelated repo-wide failures such as the QOTD test if a fully green repository is required for merge

## Practical Merge Read

If the merge criterion is "PR 1 interaction-router work is done", the current
tree is ready.

If the merge criterion is "the entire repository must be green", the remaining
QOTD failure must be fixed or isolated separately because it is outside the
interaction-router scope.