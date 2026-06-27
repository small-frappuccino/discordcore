# discordcore :: TASK PROMPT (Claude Code, first turn)

`CLAUDE.md` is loaded and binding. It already carries the execution loop (§3 CONTEXT→ACTION→VERIFY), the convergence contract (§1: forward-only, no clarifying questions, AST/diff-only output), and the gate (§8). **This prompt adds ONLY the task — it does not restate the contract.**

Effort: **Extra (`xhigh`)** — set in the effort menu before sending.

## TASK
> {one sentence, explicit target symbol / file / package}
>
> e.g. *"Migrate `Feature` (string -> uint8 enum) across the routing path; enforce §6 cardinality + fail-closed uniqueness at the `UpdateRoute` writer boundary."*

## ACCEPTANCE (task-scoped — the standing §8 gate is assumed)
> {0-3 bullets unique to THIS task: the specific invariant, signature, or behavior that proves it done and isn't already a standing rule.}
>
> e.g.
> - `UpdateRoute`/`RemoveRoute` return `error`; hydration + watcher branch on it and log rejected mutations.
> - Cap (§6) enforced at **both** hydration and `LISTEN/NOTIFY`, not just one.
> - Escape gate stays empty for `registry_cow` and `fast_parser`.