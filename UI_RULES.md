# UI_RULES.md

## Purpose

This file defines the current UI direction for the Discordcore dashboard.
It exists for coding agents and engineers making frontend changes.

These rules are meant to reduce drift, generic admin-template output, and UI that depends on explanatory text to function.

If the source code and this file disagree, trust the current source code first and update this file.
If a direct product-owner instruction conflicts with this file, follow the product-owner instruction and then update this file later if the new direction should persist.

---

## 1. Current reference point

The current `Home` page is the best available reference in this repository for:

- overall scan path
- section grouping
- card rhythm
- compact operational summaries
- semantic status treatment
- shell integration

Use `Home` as a quality bar for structure and clarity, not as a frozen template.

Do not blindly clone the Home layout into every new page.
Different workflows may need different structures.
Reuse the discipline, not the exact shape.

Also note:

- `Home` is currently stronger than the other dashboard pages and should guide future cleanup
- `Home` is still not final; palette and visual identity may continue to evolve

### 1.1 Home constraints

`Home` may evolve, but only inside these constraints:

- do not introduce new UI components as part of page cleanup or redesign work
- reuse existing layout primitives, cards, fields, rows, and shell patterns first
- do not add extra explanatory text by default
- do not compensate for weak hierarchy by adding more copy

Allowed evolution:

- spacing refinement
- grouping refinement
- density tuning
- hierarchy improvements using existing primitives
- stronger visual polish that preserves the current dashboard discipline

`Home` must continue to preserve:

- compact scan path
- low copy density
- direct action visibility
- stable shared navigation structure
- operational clarity over visual flourish

If a `Home` change materially alters these constraints, update this file in the same change.

---

## 2. Primary UI rule

The interface must guide users visually first.

Users should understand:

- where they are
- what matters most on the screen
- what can be acted on next
- what state the system is in

This guidance should come primarily from:

- layout
- hierarchy
- spacing
- grouping
- contrast
- alignment
- scale
- affordances
- interaction design

Do not rely on paragraphs of instructional copy to explain basic orientation.

---

## 3. Copy policy

Explanatory copy is optional.

Only add it when:

- the product owner explicitly wants it
- the interface genuinely needs it to avoid ambiguity
- the message is carrying real content, not compensating for weak layout

Rules:

- Prefer short, direct, scannable labels
- Use sentence case
- Keep titles compact
- Avoid repeating the same context in headings, helper text, and buttons
- Avoid filler, hype, apology, or decorative tone
- Avoid jargon unless the page is explicitly diagnostic
- Write messages so they still make sense out of context

Examples of bad copy:

- `Refreshing session`
- `An error occurred`
- `Blocked`
- `This field is invalid`

Better patterns:

- keep routine background refresh silent
- state what happened only when the user needs to know
- say what needs to be fixed when the user needs to act

### 3.1 Rare helper text exception

Small subdued helper text is allowed only in rare cases where a setting would otherwise be ambiguous.

Allowed cases:

- the setting has a non-obvious side effect
- the label alone does not explain when the behavior applies
- the setting controls edge-case or conditional behavior
- the user could plausibly misconfigure the setting without that context

Rules:

- use one short line only
- place it directly under the relevant control
- style it as secondary text
- attach it to the specific control, not the whole section
- do not repeat the label in different words
- do not use helper text for obvious settings

If multiple controls in the same section need helper text, the layout, grouping, or labeling is probably wrong and should be redesigned instead.

---

## 4. Source of truth and reuse

Use the current implementation as the main reference:

- `ui/src/index.css`
- `ui/src/shell.css`
- `ui/src/components/ui.tsx`
- `ui/src/pages/HomePage.tsx`

Rules:

- Reuse existing components, tokens, and patterns before inventing new ones
- Do not add new components for routine page polish, layout cleanup, or visual alignment work
- Do not hardcode fresh visual systems inside page files
- Do not document raw token values in this file; token roles matter more than frozen hex values
- If a new token is needed, add it centrally instead of scattering raw values across components

---

## 5. Layout and flow

Every page must have one primary job.

Build the visual scan path in this order:

1. page context
2. section
3. active surface
4. status or controls
5. action

Rules:

- Use spacing and grouping before using color
- Keep related controls and status in the same visual block
- Put tertiary content behind the primary workflow visually
- Avoid abrupt density changes between adjacent sections
- Preserve clear left and right edges in grids and sections
- Keep partial rows left-aligned

For overview pages, the default structure should usually be:

`section -> card grid -> title -> compact facts -> action`

Do not stretch cards vertically to fake symmetry.
If cards have different content lengths, preserve their natural height and align the grid cleanly.

If a page should feel like part of the dashboard canvas, do not wrap it in an unnecessary floating slab or inset surface.

---

## 6. Surfaces, color, and emphasis

The dashboard currently favors flat solid surfaces over gradient-heavy treatment.

Rules:

- Prefer flat solid surfaces
- Use neutrals for canvas and surfaces
- Use semantic colors for meaning, not decoration
- Create emphasis with contrast, borders, rails, chips, focus states, and placement
- Keep the number of competing accents low on a single page

Do not use gradients as the default emphasis model.

If a gradient is introduced, it must have a clear product reason and survive review.
If the same result can be achieved with solid surfaces, borders, spacing, and hierarchy, prefer that.

Avoid nested-surface or double-elevation UI unless there is a real layer boundary.

If a softer background weakens contrast:

- strengthen the border
- increase text contrast
- improve spacing and grouping

Do not solve it with decorative glow or ornamental effects.

Do not use success green, warning amber, or danger red merely to make a page feel less empty.

---

## 7. State and feedback

Background work should stay visually quiet unless the user must wait, decide, retry, or recover.

Rules:

- Silent background refresh should usually remain silent
- Do not expose manual `Refresh` buttons for normal page loading, workspace revalidation, or routine data sync
- Manual refresh controls imply unstable or stale software and should be treated as a UX smell in normal dashboard flows
- Only use a manual refresh or retry control when the user is explicitly recovering from a failed load, a degraded dependency, or a diagnostic workflow
- Do not mount empty notice wrappers that create layout shift
- Avoid page jumps caused by transient banners or refresh chrome
- Loading states should preserve the final layout footprint where possible
- Skeletons and placeholders should match the final structure
- Status styling must stay semantically consistent across pages

Routine revalidation must not create the impression that the page is unstable.

That means:

- no transient `Refreshing session` style copy for normal silent work
- no wrapper elements that appear briefly and push the layout down
- no full-page reflow when only local content is refreshing

---

## 8. Controls and affordances

Controls should look actionable without needing explanatory text.

Rules:

- Primary actions should stand out by placement, contrast, and grouping
- Secondary actions should remain quieter
- In cards and single-page sections, action alignment should be consistent
- Do not introduce tags as a dashboard pattern unless the user explicitly requests them
- Treat decorative, categorical, or metadata tags as prohibited by default
- Do not make non-interactive tags or status pills look clickable
- Use color to support affordance, not replace it
- For selectable item lists, default to a compact collapsed picker or disclosure that opens only on user intent
- Do not render large checkbox lists fully expanded by default when they can grow the page or distort the layout
- The closed state of a multi-select should summarize the current selection compactly instead of exposing the whole option set

If a control needs a paragraph to explain what it is, the control, grouping, or labeling is probably wrong.

Status indicators that already represent real system state are separate from decorative tags.
Do not replace meaningful status treatment with generic tag-like pills.

Do not use a page-level `Refresh` button as a default affordance for keeping data current.
If the product depends on frequent manual refresh to feel correct, fix the data flow or state model instead.

---

## 9. Typography and rhythm

Typography should support scanning and operational clarity.

Rules:

- Start from the existing type scale before creating exceptions
- Bigger type is for hierarchy, not decoration
- Supporting text should be smaller and dimmer than operational data
- Repeated spacing increments should create a stable vertical rhythm
- Use readable line heights and avoid crowded stacks
- Do not introduce one-off text sizes unless there is a clear reason

Dense pages should still feel breathable.
Breathing room should come from rhythm and grouping, not giant empty gaps.

---

## 10. Required anti-patterns to avoid

Do not introduce:

- generic hero sections or marketing-style dashboard headers
- text-heavy UI that teaches the user where to look
- floating inner canvases when the page should feel continuous
- default gradient chrome or gradient cards
- equal-height card stretching that distorts proportions
- routine page-level `Refresh` buttons for normal data loading or revalidation
- transient refresh text for routine background work
- always-expanded multi-select or checkbox lists that can make a page suddenly grow in height
- duplicated context across section title, card title, and button label
- decorative, categorical, or metadata tags unless the user explicitly asked for them
- decorative use of semantic colors
- non-semantic shadows or elevation used as a crutch
- whole-page jumps caused by notice mounts or refresh wrappers
- pages that copy Home literally even when the workflow is different

---

## 11. Agent implementation checklist

Before finalizing a page, check:

1. What is the single primary job of this page?
2. What must the user understand before reading any paragraph?
3. Does the layout answer that visually?
4. Are existing shell/components/tokens being reused?
5. Are semantic colors being used only where meaning exists?
6. Is the page stable during loading, refresh, empty, and error states?
7. Are card proportions and grid alignment still intact?
8. Did extra copy get added because the layout was weak?

If the answer to step 8 is yes, fix the layout first.

---

## 12. External influences

These sources informed the mindset in this file.
They are influences, not the repository source of truth.

- Primer, [Color usage](https://primer.style/product/getting-started/foundations/color-usage/): semantic color roles, token discipline, and restoring contrast with borders when using softer surfaces
- Fluent 2, [Design tokens](https://fluent2.microsoft.design/design-tokens): global vs alias tokens, semantic naming, theming, and accessibility-minded token structure
- Atlassian Design, [Content](https://atlassian.design/foundations/content/) and [Success messages](https://atlassian.design/foundations/content/designing-messages/success-messages/): concise, scannable UI writing where every word must earn its place
- GOV.UK Design System, [Error message](https://design-system.service.gov.uk/components/error-message/) and [Type scale](https://design-system.service.gov.uk/styles/type-scale/): specific repair-oriented messaging and consistent readable rhythm
