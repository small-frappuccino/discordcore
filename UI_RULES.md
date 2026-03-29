# UI_RULES.md

## Purpose

This file defines the non-negotiable UI implementation rules for the Discordcore dashboard refactor. It is intended for coding agents and engineers working on the frontend. Read this before writing or modifying any dashboard UI.

These rules exist to keep the interface operational, scannable, consistent, and easy to maintain. When a proposed implementation conflicts with these rules, follow this file unless an explicit exception is documented.

---

## 1. Product intent

Discordcore is a server-scoped bot dashboard. The UI must optimize for:

- fast status comprehension
- direct configuration
- low navigation depth
- operational clarity
- low-friction maintenance

The dashboard is not a marketing site, not a consumer social app, and not a generic admin template. Avoid decorative complexity that weakens legibility or hides state.

---

## 2. Core principles

### 2.1 Show the state directly
Users should be able to tell, at a glance:

- what area they are in
- what action or page is available next
- whether the selected server is readable or writable when that matters
- what is blocking a workflow when something is actually broken

### 2.2 Prefer inline configuration
Do not hide primary controls behind extra views, drawers, accordions, or modal chains when the controls can reasonably be shown on the page.

### 2.3 Keep one page focused on one operational job
Each page should support one primary administrative workflow. Secondary diagnostics may appear on the same page, but they must not dominate the layout.

### 2.4 Use consistent semantic patterns
Similar concepts must be rendered the same way everywhere:

- toggles behave identically
- status badges use the same meanings
- cards use the same structure
- tables use the same spacing and typography

### 2.5 Explain machine state in human language
Every critical status must expose a short signal message that explains why the state exists.

Bad: `Blocked`

Good: `Channel details missing`

---

## 3. Information hierarchy

All pages must follow this hierarchy:

### Primary information
Actionable controls and current operational state.

Examples:

- module enabled state
- selected server
- configured channels
- role access
- current blockers

### Secondary information
Supportive context that helps the user understand the current setup.

Examples:

- status summaries
- scope explanations
- inheritance markers
- usage notes

### Tertiary information
Diagnostics, advanced controls, maintenance notes, and edge-case explanations.

Examples:

- cache/connection tools
- low-level runtime notes
- advanced permission caveats

Tertiary information must not compete visually with primary workflows.

---

## 4. Design tokens

Use tokens only. Do not hardcode raw values in components.

## 4.1 Color tokens

```css
:root {
  --bg-canvas: #08111f;
  --bg-app: #0b1423;
  --bg-surface-1: #101a2b;
  --bg-surface-2: #132033;
  --bg-surface-3: #17263a;
  --bg-elevated: #1b2b40;

  --border-subtle: rgba(255, 255, 255, 0.06);
  --border-default: rgba(255, 255, 255, 0.10);
  --border-strong: rgba(255, 255, 255, 0.16);

  --text-primary: rgba(255, 255, 255, 0.94);
  --text-secondary: rgba(255, 255, 255, 0.72);
  --text-tertiary: rgba(255, 255, 255, 0.48);
  --text-disabled: rgba(255, 255, 255, 0.34);

  --accent-primary: #d6a58a;
  --accent-primary-hover: #e0b096;
  --accent-primary-muted: rgba(214, 165, 138, 0.18);

  --status-success: #57c084;
  --status-success-bg: rgba(87, 192, 132, 0.16);
  --status-danger: #ef6b6b;
  --status-danger-bg: rgba(239, 107, 107, 0.16);
  --status-warning: #d8b15d;
  --status-warning-bg: rgba(216, 177, 93, 0.16);
  --status-info: #78a8ff;
  --status-info-bg: rgba(120, 168, 255, 0.16);

  --focus-ring: rgba(120, 168, 255, 0.45);
  --overlay-scrim: rgba(5, 10, 18, 0.72);
}
```

## 4.2 Color semantics

Use colors by meaning, not preference.

- Success = functional and ready
- Danger = disabled, broken, or blocked
- Warning = partial/incomplete/needs attention
- Info = neutral metadata or secondary emphasis
- Accent = primary product action

Never use success green for a decorative highlight unrelated to success.
Never use red merely for visual balance.

## 4.3 Toggle colors

Toggle behavior is mandatory:

- Enabled: green track, knob on the right
- Disabled: red track, knob on the left

There is no alternate toggle style.

## 4.4 Spacing tokens

```css
:root {
  --space-1: 4px;
  --space-2: 8px;
  --space-3: 12px;
  --space-4: 16px;
  --space-5: 20px;
  --space-6: 24px;
  --space-8: 32px;
  --space-10: 40px;
  --space-12: 48px;
}
```

## 4.5 Radius tokens

```css
:root {
  --radius-sm: 8px;
  --radius-md: 12px;
  --radius-lg: 16px;
  --radius-xl: 20px;
  --radius-pill: 999px;
}
```

## 4.6 Shadow tokens

```css
:root {
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.20);
  --shadow-md: 0 10px 30px rgba(0, 0, 0, 0.22);
  --shadow-lg: 0 18px 50px rgba(0, 0, 0, 0.28);
}
```

## 4.7 Typography tokens

```css
:root {
  --font-sans: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;

  --text-xs: 12px;
  --text-sm: 13px;
  --text-md: 14px;
  --text-lg: 16px;
  --text-xl: 20px;
  --text-2xl: 28px;
  --text-3xl: 40px;

  --leading-tight: 1.2;
  --leading-normal: 1.45;
  --leading-relaxed: 1.6;
}
```

## 4.8 Typography rules

- Page title: `--text-2xl` or `--text-3xl`
- Card title: `--text-xl`
- Body copy: `--text-md`
- Supporting copy: `--text-sm`
- Labels, eyebrow text, column headers: `--text-xs`

Avoid oversized text inside dense tables. Avoid tiny text for critical controls.

---

## 5. Layout rules

## 5.1 App shell

The dashboard shell must contain:

- sticky top bar
- left sidebar
- main content area

The top bar owns product identity, selected server, and account controls.
The sidebar stays structurally stable across modules and contains navigation only.

## 5.2 Content width

Use full-width desktop layouts, but constrain inner readability.

Recommended max widths:

- main page body: 1440px
- dense form content: 1280px
- reading-heavy panels: 960px where appropriate

## 5.3 Grid rules

Preferred desktop grids:

- overview cards: 4-column grid
- content + summary rail: 8/4 or 9/3 split
- dense table pages: full-width main panel plus summary side rail

Avoid arbitrary one-off grid fractions unless the page has a documented reason.

## 5.4 Panel spacing

- card internal padding: 20–24px
- page section gap: 16–24px
- card-to-card gap: 16px
- grouped form rows: 12–16px

## 5.5 Page rhythm

Each page should visually read in this order:

1. page header/context
2. overview/status strip
3. main workspace surface
4. summary/guidance rail
5. secondary sections

Do not place diagnostics before the primary configuration surface.

---

## 6. Navigation architecture

Sidebar and Home may both render navigation, but they must derive from one shared registry.
Duplicated navigation is acceptable only as a duplicated view, never as duplicated structure.

Sidebar order is fixed unless a future navigation redesign explicitly replaces it.

```txt
Core
  - Control Panel
  - Stats
  - Commands
Moderation
  - Moderation
  - Logging
Partner Board
Roles
  - Autorole
  - Level Roles
```

Rules:

- preserve order
- sidebar stays route-focused and visually light
- Home may mirror these areas as cards, but card order and labels must come from the same registry
- do not place Settings-like diagnostics in the primary workflow cluster unless the page’s purpose is diagnostics

---

## 7. Component contracts

The following components are required patterns. Their contract must remain stable.

## 7.1 PageHeader

Purpose: establish page identity and operational scope.

Must include:

- eyebrow label or area label
- page title
- 1–2 sentence description
- optional context chips for selected server / access level
- optional top-right actions

Must not include:

- dense form controls
- redundant data already repeated in overview cards

## 7.2 ContextChip

Use for compact metadata such as:

- selected server
- connection URL
- signed-in state
- workspace scope

Rules:

- visually subdued
- horizontally compact
- not used as primary actions unless explicitly clickable and styled as action chips

## 7.3 OverviewCard

Purpose: summarize one operational dimension.

Structure:

- eyebrow
- primary value or state
- short explanatory line
- optional semantic indicator dot or badge

Examples:

- Ready
- 4 channels
- 2/2 routes configured
- 1 active blocker

Rules:

- keep copy concise
- avoid long paragraphs
- do not embed multiple unrelated controls

## 7.4 StatusBadge

Purpose: normalize visible state language.

Allowed statuses:

- Operational
- Ready
- Needs setup
- Incomplete
- Disabled
- Blocked

Each badge must map to a semantic token pair:

- label
- foreground/background colors

Do not invent new status words casually.

## 7.5 SignalText

Purpose: explain state in plain language.

Examples:

- `Everything mapped to this module is ready.`
- `Channel details missing.`
- `At least one route is disabled.`

Rules:

- max 1 sentence in dense views
- avoid raw internal jargon unless unavoidable
- must not repeat badge text verbatim

## 7.6 ToggleField

Purpose: control an enabled/disabled behavior.

Contract:

- label
- current boolean state
- optional supporting text
- optional dependency explanation

Behavior:

- enabled = green + knob right
- disabled = red + knob left
- immediate visual feedback
- keyboard accessible
- click target must include label area where appropriate

Do not create custom toggle semantics per module.

## 7.7 SelectField

Use for:

- role selection
- channel selection
- server-bound entity selection

Rules:

- clear placeholder
- show selected count for multi-select
- allow empty state where valid
- support long labels gracefully

## 7.8 DataTable

Purpose: manage repeatable operational rows.

Requirements:

- sticky headers when height is constrained
- clear row dividers
- dense but readable spacing
- actions aligned consistently
- no hidden critical status

Each row should make these obvious:

- subject
- current state
- explanation
- action

## 7.9 SummaryRail

Purpose: provide compact confirmation and guidance.

May include:

- selected server
- key configured entities
- current signal
- guidance bullets

Must not become the primary place where the user configures the feature.

## 7.10 EmptyState

Use when a module or table has no data.

Must include:

- title
- explanation
- obvious next action

Avoid passive wording like `No data found` without guidance.

---

## 8. Control visibility rules

## 8.1 Direct controls first

If a user can reasonably operate a setting directly on the page, show it directly.

Examples:

- module enable/disable
- route activation
- role selection
- access settings
- channel mapping

## 8.2 Remove redundant configure loops

Do not implement this pattern:

1. card shows summary
2. user clicks Configure
3. separate view repeats same summary
4. user finally sees actual controls

Replace with:

- summary plus controls on one page
- or summary above an immediately visible control surface

## 8.3 Use detail drawers sparingly

Allowed only for:

- advanced diagnostics
- verbose audit detail
- row-level detail too large for the main surface

Not allowed for the primary happy-path configuration.

---

## 9. Page-specific implementation rules

## 9.1 Home

Home is the visual index of the product areas.

Must include:

- section titles that match the sidebar groups
- one card per sub-area
- short, relevant facts per card
- one direct CTA per card

Must not include:

- debug strips
- quick shortcuts that drift from the sidebar
- module tables or blocker panels from the old operational dashboard
- duplicate navigation definitions outside the shared registry

## 9.2 Core > Control Panel

Must expose directly on page:

- read access roles
- write access roles
- admin override note

Contract:

- multi-select role input for read
- multi-select role input for write
- explanation that Discord administrators remain implicitly allowed

## 9.3 Core > Commands

Must focus on:

- command channel configuration
- admin access roles
- enabled/disabled state
- current signal

Do not bury the module state in a deep panel.

## 9.4 Core > Stats

Must focus on:

- enabled state
- update rule
- update interval
- configured channels inventory

Configured channels should appear in a readable table.

## 9.5 Moderation > Moderation

Must group controls by workflow:

- general moderation
- timeout
- mute
- kick
- ban
- warnings

Use visible toggles and direct fields. Follow reference screenshots structurally, but keep Discordcore’s visual system.

## 9.6 Moderation > Logging

Must present route-level configuration in one visible table.

Each row should expose:

- route name
- destination
- status badge
- signal
- actions/toggle

## 9.7 Partner Board

Must center on publish readiness.

Keep separate but visible sections for:

- entries
n- layout
- destination

Use blockers and readiness clearly.

## 9.8 Roles > Autorole

Feature contract:

- target role: single-select
- required roles: multi-select
- match mode toggle

Match mode toggle behavior:

- OFF by default = requires any selected role
- ON = requires all selected roles

Label the toggle explicitly. Do not rely on implied logic.

Recommended label:

- `Require all selected roles`

Supporting text when off:

- `When disabled, any selected role can trigger the assignment.`

Supporting text when on:

- `When enabled, the user must have every selected role.`

## 9.9 Roles > Level Roles

Must use a table-based editor.

Columns:

- Role
- Level
- Active

Rules:

- rows editable inline
- add-entry button below table
- only one row may be active at a time
- enabling one active toggle must disable the previously active row automatically

Validation:

- no duplicate active rows
- role cannot be empty if row is saved
- level must be numeric and within allowed bounds

---

## 10. State modeling rules

Every module page must distinguish these concepts explicitly:

- enabled state
- configured state
- ready state
- blocked state

These are not interchangeable.

Example:

- enabled = true
- configured = partial
- ready = false
- blocked = true
- signal = `Mute role missing.`

Do not compress all state into a single boolean.

---

## 11. Copywriting rules

## 11.1 Tone

Use concise operational language.

Good:

- `Mute role is ready for role-based mute actions.`
- `Command channel is configured for this server.`

Bad:

- `Everything should be working now.`
- `Looks good.`

## 11.2 Labels

Use domain-accurate labels.

Prefer:

- `Configured role`
- `Current signal`
- `Allowed roles`
- `Destination channel`

Avoid vague labels like:

- `Details`
- `Info`
- `Stuff`

## 11.3 Help text

Help text should explain behavior, not restate the label.

Bad:

- Label: `Update interval`
- Help: `Set the update interval`

Good:

- `Controls how often the bot updates configured stats channels.`

---

## 12. Accessibility rules

Minimum requirements:

- all interactive controls keyboard accessible
- focus visible with consistent focus ring
- color not the only state indicator
- toggle labels always present
- semantic headings in page order
- table headers associated correctly
- body text contrast must remain readable on dark surfaces

Do not rely on tiny color dots as the only status signal.

---

## 13. Responsive behavior

## 13.1 Desktop-first, tablet-safe

This dashboard is primarily desktop-oriented, but it must collapse cleanly.

## 13.2 Breakpoint behavior

Recommended:

- `>= 1280px`: full desktop layout with summary rail
- `>= 1024px and < 1280px`: reduce column counts, preserve page hierarchy
- `< 1024px`: stack rails below main workspace

## 13.3 On smaller widths

- keep primary actions visible
- avoid horizontal overflow for core controls
- allow tables to scroll only when unavoidable
- do not hide status meaning behind icons

---

## 14. Anti-pattern detection

The following patterns are prohibited unless explicitly justified.

## 14.1 Hidden critical controls

Bad:

- primary setup hidden behind `Configure`
- row state only visible after expansion
- important toggle only shown in modal

## 14.2 Redundant summary repetition

Bad:

- card says `2 roles`
- next panel headline says `2 roles`
- summary rail again says `2 roles`

Each layer should add information, not repeat it.

## 14.3 State ambiguity

Bad:

- green card with text saying incomplete
- enabled toggle with blocker hidden elsewhere
- one label used for both runtime health and config completeness

## 14.4 Overloaded cards

Bad:

- one card contains metrics, toggles, help text, audit notes, and actions together

Cards should have one clear responsibility.

## 14.5 Decorative density

Bad:

- too many gradients
- too many highlight colors
- too many pill chips without hierarchy
- visual noise that competes with data

## 14.6 Underspecified tables

Bad:

- row has item name only
- no visible status
- no visible explanation
- actions inconsistent per row

## 14.7 Action vagueness

Bad CTA labels:

- `Manage`
- `Open`
- `Do action`

Prefer task-specific labels:

- `Configure command channel`
- `Add entry`
- `Select mute role`

## 14.8 Multiple interaction patterns for the same concept

Bad:

- one module uses checkbox for enable
- another uses toggle
- another uses dropdown state selector

Use one consistent interaction model.

---

## 15. Engineering rules

## 15.1 Separate domain state from presentation state

Maintain separate layers for:

- API/domain entities
- page view models
- component UI state

Do not push raw API response shapes directly into components when the UI needs normalized state.

## 15.2 Normalize module status

Create a shared status adapter or utility that converts module data into a standard UI shape:

```ts
interface ModuleUiState {
  enabled: boolean;
  configured: boolean;
  ready: boolean;
  blocked: boolean;
  badge: "Operational" | "Ready" | "Needs setup" | "Incomplete" | "Disabled" | "Blocked";
  signal: string;
}
```

## 15.3 Component APIs must be stable

Design shared components with explicit props and avoid one-off optional prop sprawl.

## 15.4 Do not encode business logic only in the UI

Rules such as:

- admins implicitly allowed
- only one level-role row may be active
- autorole any/all matching

must exist in domain-safe logic, not merely visual state.

## 15.5 Form state must support optimistic clarity

Users should immediately understand what changed, what is unsaved, and what failed.

Minimum patterns:

- dirty state detection
- inline validation
- save feedback
- failure feedback tied to the affected control or section

---

## 16. Suggested shared TypeScript contracts

```ts
export type StatusBadgeKind =
  | "operational"
  | "ready"
  | "needs-setup"
  | "incomplete"
  | "disabled"
  | "blocked";

export interface StatusSignal {
  kind: StatusBadgeKind;
  label: string;
  message: string;
}

export interface OverviewCardData {
  eyebrow: string;
  value: string;
  description: string;
  indicator?: "success" | "danger" | "warning" | "info";
}

export interface ToggleFieldProps {
  id: string;
  label: string;
  checked: boolean;
  description?: string;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}

export interface LevelRoleRow {
  id: string;
  roleId: string | null;
  level: number | null;
  active: boolean;
}
```

---

## 17. Review checklist for coding agents

Before shipping a UI change, verify all of the following:

### Structure
- Is the page centered on one primary administrative job?
- Is the page header clear and scoped?
- Is the information hierarchy obvious?

### Controls
- Are primary controls directly visible?
- Are toggles standardized green/red with right/left knob behavior?
- Are labels explicit and unambiguous?

### State
- Can the user distinguish enabled vs ready vs blocked?
- Is every major status accompanied by a signal message?
- Are blockers visible without extra clicks?

### Redundancy
- Did we remove repeated summaries?
- Did we avoid configure loops?
- Did we avoid duplicate navigation paths for the same action?

### Roles-specific behavior
- Does autorole support multi-select required roles?
- Does autorole correctly support any vs all matching?
- Does level roles enforce a single active row?

### Accessibility
- Can everything be used with keyboard navigation?
- Is focus visible?
- Is color supplemented by text/state labels?

### Engineering
- Are tokens used instead of hardcoded values?
- Is domain logic separated from view logic?
- Are component contracts reusable rather than page-specific hacks?

---

## 18. Final rule

When uncertain between:

- more visually impressive vs more operationally clear
- more abstract vs more direct
- more compact vs more legible

choose:

- operationally clear
- direct
- legible

The Discordcore dashboard should feel like a reliable control surface, not a concept showcase.
