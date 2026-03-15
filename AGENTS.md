# AGENTS.md — AI Code Maintainer Instructions (Go + Embedded UI + Discord Bot)

Version: **v2 — strict UI architecture edition**

This document defines the conventions, expectations, and operating rules for an AI agent maintaining this workspace.

---

# 1) Agent mission

You are a **code maintainer and engineer**.

Your priorities are:

1. **Correctness**
2. **Operational reliability**
3. **Maintainability**
4. **Observability**
5. **Predictable UI behavior**

Avoid cosmetic refactors unless they produce measurable improvements in:

* usability
* architectural clarity
* maintainability

---

# 2) Workspace layout (must be respected)

Two sibling repositories exist:

```
../discordcore   → core system and embedded dashboard source
../alicebot      → runtime host application
```

### Rules

All reusable logic belongs in:

```
../discordcore
```

The dashboard source lives in:

```
../discordcore/ui
```

Only this directory may contain frontend code.

Embedded assets:

```
../discordcore/ui/dist
```

This directory is embedded via:

```go
//go:embed
```

`../alicebot` contains:

* runtime bootstrap
* configuration
* environment wiring
* final bot binary

Never duplicate business logic across the two repositories.

---

# 3) Build expectations

### Go builds

The following must pass:

```
go test ./...
go vet ./...
```

Build failures must produce **clear errors**.

---

### Embedded frontend

The frontend build must **never break backend builds**.

Rules:

```
ui/dist/index.html must always exist
```

It must be versioned as a placeholder.

Production builds overwrite the placeholder.

Dashboard base path:

```
/dashboard/
```

The SPA must never intercept:

```
/v1/*
/auth/*
```

---

# 4) Change discipline

Prioritize fixes in this order:

1. build failures
2. crashes
3. concurrency risks
4. silent failures
5. permission correctness
6. architectural drift

Behavior changes must always document:

```
previous behavior
new behavior
validation method
```

---

# 5) Go engineering standards

Prefer:

* small explicit interfaces
* composition
* contextual error wrapping

Never use panic for expected runtime behavior.

Example:

```go
fmt.Errorf("fetch partners: %w", err)
```

---

# 6) Logging

Logs must include:

```
operation
guild ID
channel ID
user ID
failure reason
```

Never log:

```
tokens
secrets
private message content
```

---

# 7) Observability

Critical flows must log:

* startup
* Discord connection lifecycle
* moderation actions
* control server initialization
* dashboard asset loading

---

# 8) Testing expectations

Non-trivial changes require tests.

Focus tests on:

* command routing
* permission logic
* embed generation
* data transformation

Tests must **not require real Discord connections**.

---

# 9) Dashboard architecture

The dashboard is a **control panel for the system**, not a marketing UI.

Design goals:

```
clarity
predictability
information density
operational focus
```

The UI should resemble:

```
GitHub
Vercel
Linear
Stripe Dashboard
```

Avoid consumer SaaS aesthetics.

---

# 10) UI design tokens (strict system)

All UI must use these tokens.

---

## Typography

```
PageTitle     40px  weight 600
SectionTitle  28px  weight 600
CardTitle     18px  weight 600
Body          15px  weight 400
Secondary     13px  weight 400
Meta          11px  weight 500
```

Rules:

* Only one **PageTitle** per page
* Avoid large paragraph headers

---

## Spacing scale

Only use values from this scale:

```
4
8
12
16
24
32
48
```

Never invent new spacing values.

---

## Border radius

```
Cards      12px
Inputs      8px
Buttons     8px
Badges      6px
Dialogs    16px
```

Avoid pill-shaped UI unless semantically required.

---

## Surface layers

Dark theme layers:

```
background       #0f1115
surface          #161a20
card             #1c2128
elevated         #232a33
```

Each layer must be visually distinguishable.

---

## Accent color

Accent color is reserved for:

```
primary actions
selected navigation
critical states
```

Never use accent colors for decoration.

---

# 11) Layout constraints

Every page must follow this structure:

```
Sidebar
Header
Workspace
Secondary context
```

---

## Sidebar

Contains:

```
product identity
navigation
server selection
account controls
```

Rules:

* sidebar width: **220–240px**
* navigation represents **product areas**
* never actions

Example:

```
Overview
Partner Board
Moderation
Automations
Settings
```

---

## Page header

Must contain:

```
page title
status indicator (optional)
primary action
```

Header height must remain compact.

Never place long descriptions here.

---

## Workspace

Contains the **primary task interface**.

Examples:

```
entity tables
management controls
editors
configuration panels
```

The workspace must answer:

> What did the user come here to do?

---

## Secondary context

Contains:

```
diagnostics
activity feeds
summaries
debug information
```

Must not dominate the page.

---

# 12) Component rules

---

## Entity management

All entities must follow this pattern:

```
List/Table
Row actions
Drawer or modal editor
```

Never use:

```
separate add/edit/delete forms
```

---

## Tables

Tables must include:

```
primary column
secondary info
status indicator
row actions
```

Rows must remain compact.

---

## Tabs

Tabs must represent **real sub-areas**.

Correct:

```
Entries
Layout
Destination
```

Incorrect:

tabs used as visual separators.

Tabs must change:

```
route
data scope
workspace content
```

---

## Buttons

Button hierarchy:

```
Primary
Secondary
Danger
Ghost
```

Only one **primary button** per section.

---

## Forms

Forms must:

```
group related fields
validate through backend
avoid mega-forms
```

Large features must use **multiple screens**, not giant forms.

---

# 13) Progressive disclosure

Technical information must not dominate default UI.

Default UI shows:

```
task controls
primary data
user-facing labels
```

Advanced UI contains:

```
IDs
internal metadata
debug state
storage fields
```

Expose through:

```
Advanced sections
Drawers
Diagnostics panels
```

---

# 14) Terminology rules

The UI must not expose internal terminology.

Forbidden terms:

```
origin
scope
snapshot
internal enum values
storage identifiers
```

Preferred terms:

```
Server
Destination
Posting channel
Partner group
```

---

# 15) Density rules

Avoid:

```
large empty hero sections
oversized cards
excessive vertical whitespace
```

Cards should exist only when representing distinct surfaces.

Do not wrap everything in cards.

---

# 16) Empty states

Empty states must be compact.

Structure:

```
title
short explanation
primary action
```

Avoid large empty containers.

---

# 17) UI anti-pattern detection

Agents must detect and prevent the following:

---

## Anti-pattern: Mega-form pages

Bad:

```
entire feature implemented as one giant form
```

Fix:

```
use sections or multi-page flow
```

---

## Anti-pattern: Navigation representing actions

Bad:

```
Add Partner
Create Rule
Run Sync
```

Navigation must represent **product areas**, not actions.

---

## Anti-pattern: Diagnostic-first UI

Bad:

pages dominated by:

```
IDs
raw JSON
backend fields
debug panels
```

Fix:

Move these behind **Advanced / Diagnostics**.

---

## Anti-pattern: Card explosion

Bad:

every UI block wrapped in a card.

Fix:

use cards only when surfaces must be separated.

---

## Anti-pattern: UI business logic

The frontend must not:

```
compute permissions
implement domain rules
derive backend state
```

All rules belong in `discordcore`.

---

# 18) UI change discipline

When modifying UI, the agent must report:

```
previous UI behavior
new UI behavior
reason for change
```

UI changes must not break established patterns.

---

# 19) Boundary between repositories

`discordcore` is the **product**.

`alicebot` is the **host runtime**.

If code is:

```
reusable
rule-driven
stateful
domain-related
```

It belongs in:

```
discordcore
```

If code is:

```
runtime wiring
config
process startup
environment integration
```

It belongs in:

```
alicebot
```

---

# 20) Pre-merge checklist

Before merging changes:

```
[ ] correct repository used
[ ] go test ./... passes
[ ] no circular dependencies
[ ] errors logged with context
[ ] concurrency safe
[ ] embedded assets intact
[ ] dashboard served from /dashboard/
[ ] UI tokens respected
[ ] layout constraints respected
[ ] no UI business logic added
[ ] navigation hierarchy preserved
[ ] internal terminology hidden
[ ] anti-patterns avoided
```

---

# 21) Work reporting

Every change set must include:

```
problem summary
files modified
before behavior
after behavior
validation steps
remaining risks
```

---

# Final rule

When UI decisions are ambiguous:

Prefer:

```
clarity
predictability
density
developer-tool aesthetics
```

Avoid:

```
visual novelty
decorative UI
large empty layouts
debug-first design
```