This file is designed to be **read by coding agents before writing or modifying any frontend code**.
It enforces a **deterministic dashboard design system** so AI agents consistently produce **high-quality developer tooling UI**, not consumer SaaS layouts.

It complements the existing `AGENTS.md` (referenced here: ).

---

# UI_RULES.md

Dashboard UI Rules for Coding Agents

Version: **v1**

This file defines **mandatory UI rules** for any frontend code written in this workspace.

Coding agents **must read and follow this file before generating UI code**.

These rules exist to ensure the dashboard behaves like a **professional developer control panel**, not a consumer SaaS interface.

---

# 1. Core philosophy

The dashboard is an **operational control interface** for managing a Discord system.

The UI must prioritize:

* clarity
* predictability
* information density
* operational efficiency

The UI must resemble modern developer dashboards such as:

* GitHub
* Vercel
* Stripe Dashboard
* Linear
* Raycast settings panels

The UI must **not resemble**:

* marketing websites
* consumer SaaS onboarding flows
* landing pages
* mobile-first card layouts

---

# 2. Absolute UI constraints

These rules are **mandatory**.

Coding agents must **never violate them**.

---

## 2.1 No business logic in the frontend

Frontend code must **never implement domain logic**.

Forbidden responsibilities:

```
permission logic
rule evaluation
moderation decisions
derived domain state
persistence logic
```

The UI must call backend services and render results.

If the UI requires derived data, the backend must expose it.

---

## 2.2 Navigation represents product areas

Navigation must represent **product areas**, not actions.

Correct:

```
Overview
Partner Board
Moderation
Automations
Settings
```

Incorrect:

```
Add Partner
Create Rule
Run Sync
Delete Entry
```

Actions belong inside pages.

---

## 2.3 Only one primary workflow per page

Each page must answer a single question:

> What did the user come here to do?

Examples:

```
manage partner entries
configure posting destination
review moderation actions
view automation rules
```

Do not mix multiple unrelated workflows on a page.

---

## 2.4 Pages must be task-oriented

Design pages around **tasks**, not backend data structures.

Bad:

```
raw API field editors
scope configuration panels
internal storage identifiers
```

Good:

```
Choose posting channel
Add partner server
Manage moderation rules
```

---

# 3. Layout architecture

All pages must follow this layout.

```
Sidebar
Header
Workspace
Secondary context
```

---

## 3.1 Sidebar

Sidebar contains:

```
product identity
navigation
server/guild context
account controls
```

Sidebar rules:

```
width: 220–240px
persistent across pages
no feature configuration controls
```

Sidebar navigation items represent **product areas**.

---

## 3.2 Header

Page headers contain:

```
page title
optional status
primary action
```

Rules:

```
compact height
no long paragraphs
no cards inside header
```

Headers exist to **orient the user**, not explain the system.

---

## 3.3 Workspace

The workspace contains the **primary feature UI**.

Examples:

```
entity tables
configuration editors
rule lists
activity panels
```

This area must dominate the page.

---

## 3.4 Secondary context

Secondary areas contain:

```
summaries
diagnostics
recent activity
debug tools
```

These must **never dominate the page**.

---

# 4. Entity management pattern

All entities must follow this structure.

```
Table / List
Row actions
Drawer or modal editor
```

Example:

```
Partner entries table
Edit button on each row
Drawer editor for editing entry
```

Do not build pages like:

```
Add form
Edit form
Delete form
```

as separate sections.

Entities are managed **from the list itself**.

---

# 5. Tabs and sub-navigation

Tabs must represent **real sub-areas**.

Example:

```
Entries
Layout
Destination
```

Tabs must change:

```
route
data context
visible UI
```

Tabs must not be used as visual separators.

---

# 6. UI design tokens

All UI must use these tokens.

---

## Typography

```
PageTitle      40px   weight 600
SectionTitle   28px   weight 600
CardTitle      18px   weight 600
Body           15px   weight 400
Secondary      13px   weight 400
Meta           11px   weight 500
```

Rules:

* Only one `PageTitle` per page
* Avoid large paragraph headers

---

## Spacing

Only these spacing values are allowed.

```
4
8
12
16
24
32
48
```

Agents must **not invent new spacing values**.

---

## Border radius

```
Cards      12px
Inputs      8px
Buttons     8px
Badges      6px
Dialogs    16px
```

Avoid pill-shaped UI components.

---

## Surface layers

Dark theme surfaces:

```
background     #0f1115
surface        #161a20
card           #1c2128
elevated       #232a33
```

Surfaces must be visually distinct.

---

## Accent colors

Accent color is reserved for:

```
primary actions
selected navigation
critical state indicators
```

Accent colors must not be decorative.

---

# 7. Density rules

The dashboard must prioritize **information density**.

Avoid:

```
large hero sections
oversized cards
excessive whitespace
card-heavy layouts
```

Cards should exist only when representing **separate surfaces**.

---

# 8. Progressive disclosure

Technical data must not dominate the default UI.

Default UI shows:

```
tasks
entities
actions
user-facing labels
```

Advanced UI contains:

```
IDs
internal metadata
debug information
backend identifiers
```

Expose through:

```
Advanced panels
Diagnostics sections
Debug views
```

---

# 9. Terminology rules

UI must use **product language**, not internal system language.

Forbidden terms:

```
origin
scope
snapshot
storage identifiers
internal enum values
```

Preferred:

```
Server
Destination
Posting channel
Partner group
```

The backend may expose internal names; the UI must map them.

---

# 10. Empty state rules

Empty states must be compact.

Structure:

```
title
short explanation
primary action
```

Avoid giant empty containers.

---

# 11. Component rules

---

## Buttons

Button hierarchy:

```
Primary
Secondary
Danger
Ghost
```

Only one primary button per section.

---

## Tables

Tables must include:

```
primary column
secondary information
status indicator
row actions
```

Rows must remain compact.

---

## Forms

Forms must:

```
group related fields
validate via backend
avoid large mega-forms
```

Large features should use **multiple screens**, not one giant form.

---

# 12. Anti-pattern detection

Coding agents must detect and avoid these patterns.

---

## Mega-form pages

Bad:

```
entire feature implemented as one giant form
```

Fix:

```
split into sections or multiple pages
```

---

## Navigation actions

Bad:

```
navigation items that trigger actions
```

Navigation represents **product areas**, not actions.

---

## Diagnostic-first UI

Bad:

```
IDs
raw JSON
debug metadata
```

These must be hidden behind **Advanced / Diagnostics**.

---

## Card explosion

Bad:

```
every section wrapped in a card
```

Use cards only for **separate conceptual surfaces**.

---

# 13. Visual restraint rules

Agents must avoid:

```
heavy gradients
blur-heavy backgrounds
glassmorphism
decorative backgrounds
oversized rounded UI
```

The design must remain **functional and restrained**.

---

# 14. UI change reporting

When modifying UI, agents must report:

```
previous UI behavior
new UI behavior
reason for change
```

UI changes must maintain established patterns.

---

# 15. When rules conflict

When rules conflict, prioritize:

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
marketing layouts
```

---

# End of UI rules

Any UI generated in this workspace **must conform to these rules**.

Agents must treat these rules as **design constraints**, not suggestions.