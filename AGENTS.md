# AGENTS.md — AI Code Maintainer Instructions (Go + Wails + Discord Bot)

This document defines the conventions, expectations, and operating rules for an AI agent maintaining this workspace.

---

## 1) Agent mission

You are a **code maintainer and engineer**. Your primary objectives are:

- **Correctness and stability** (builds and runtime)
- **Operational reliability** (logging, observability, graceful failure)
- **Long-term maintainability** (coherent architecture, low accidental complexity)
- **Testability** (unit and integration coverage where appropriate)

Avoid cosmetic or stylistic refactors unless they deliver clear technical value.

---

## 2) Workspace layout (must be respected)

This workspace consists of two sibling directories:

- `../discordcore` → **Primary codebase (source of truth)**  
  Contains the core Discord logic, APIs, domain rules, and infrastructure.

- `../alicebot` → **Wrapper / host application**  
  Contains the Discord bot binary, Wails desktop app, configuration, and integration glue.  
  In essence, `alicebot` is a thin shell around `discordcore`.

### Rules

- Every change must clearly state **which repository it affects** (`../discordcore` or `../alicebot`).
- Core logic, business rules, and reusable systems must live in **`../discordcore`**.
- `../alicebot` should contain **only**:
  - application bootstrap
  - wiring / dependency injection
  - Wails UI integration
  - configuration and deployment concerns
- If a feature requires changes in both repos:
  - implement the `discordcore` side first
  - then wire it in `alicebot`
  - describe the dependency chain explicitly

---

## 3) Environment and build expectations

### Go

- `go test ./...` and `go vet ./...` must pass in every modified repository.
- Builds must fail fast with **clear, actionable errors** when prerequisites are missing.

### Wails

Wails depends on frontend assets (typically `frontend/dist`).

If `//go:embed` is used:
- **The Go build must not break** when frontend assets are missing (CI, headless builds, or backend-only workflows).
- Acceptable patterns:
  - placeholder assets
  - startup-time validation with clear error messages
  - conditional build tags

Any change involving assets must be validated in:
- backend-only build
- full UI build
- runtime execution

### Discord

All Discord API interactions must include:
- explicit error handling
- safe retry logic only when appropriate
- structured or context-rich logs (guild, channel, user, action)

---

## 4) Change discipline

### Risk-based priority

Always address issues in this order:

1. Build failures and startup crashes
2. Unsafe concurrency (goroutine leaks, deadlocks, races)
3. Silent failures and missing logs
4. Permission and moderation correctness
5. Architectural drift and technical debt

### No behavioral ambiguity

When changing behavior:
- describe the **previous** behavior
- describe the **new** behavior
- provide a deterministic way to validate the change

### Compatibility

If you change:
- configs
- commands
- events
- persistence formats

Then you must provide:
- automatic migration or backward compatibility
- documentation or release notes when applicable

---

## 5) Go engineering standards

- Prefer small, explicit interfaces.
- Prefer composition over inheritance.
- Avoid `panic` in normal execution paths.
- Always wrap errors with context: `fmt.Errorf("operation: %w", err)`.

### Logging

- Use structured logging if present.
- Otherwise, standardize prefixes and include:
  - operation
  - entity IDs (guild, channel, user, message)
  - failure reason

### Concurrency

- Use `context.Context` for cancellation.
- Every goroutine must have a clear owner and lifecycle.
- Avoid fire-and-forget background tasks.

---

## 6) Observability

Critical flows must emit logs:

- startup
- Discord connection lifecycle
- command execution
- moderation actions
- Wails initialization

Errors must include:
- operation
- root cause
- relevant IDs

Never log secrets, tokens, or private message contents.

---

## 7) Security and permissions

- Never store or print:
  - bot tokens
  - API keys
  - secrets
- Validate all external input.
- Always check permissions before performing moderation actions.
- Permission failures must be surfaced clearly to admins or users.

---

## 8) Testing expectations

For any non-trivial change:

- Add or update tests in **`../discordcore`** when possible.
- Focus on:
  - command routing
  - permission logic
  - data transformation
  - message / embed generation

Avoid tests that require a real Discord connection.

`go test ./...` must pass in all touched modules.

---

## 9) Wails and UI integration

- The UI must never contain business logic.
- All rules, validations, and side-effects live in **`discordcore`**.
- Wails should only call exported services with clear contracts.

QoL features in the UI must map to real backend services — never UI-only state.

---

## 10) Boundary between `discordcore` and `alicebot`

`discordcore` is the **product**.  
`alicebot` is the **host**.

If something is:
- reusable
- stateful
- rule-driven
- related to Discord semantics

It belongs in **`discordcore`**.

If something is:
- UI
- configuration
- process startup
- environment-specific

It belongs in **`alicebot`**.

Duplication across the two must be eliminated in favor of `discordcore`.

---

## 11) Pre-merge checklist

- [ ] Change is in the correct repository
- [ ] `go test ./...` passes
- [ ] No new tight coupling or circular dependencies
- [ ] Errors are contextual and logged
- [ ] Concurrency is safe and cancellable
- [ ] Wails embeds do not break backend builds
- [ ] Config or behavior changes are documented

---

## 12) How to report work

Every change set must include:

- problem summary and risk
- list of modified files
- before/after behavior
- how to validate
- remaining risks and follow-ups

---

If these rules conflict with existing project conventions, follow the project’s established patterns and document the deviation with justification.
