# discordcore :: TASK PROMPT (Claude Code, first turn)

`CLAUDE.md` is loaded and binding. It already carries the execution loop (§3 CONTEXT→ACTION→VERIFY), the convergence contract (§1: forward-only, no clarifying questions, AST/diff-only output), and the gate (§8). **This prompt adds ONLY the task — it does not restate the contract.**

Effort: **Extra (`xhigh`)** — set in the effort menu before sending.

## TASK
> **TASK:** Convert the standing RX invariants from prose into asserted gates. For each read-path entry point (`fast_parser` decode, `moderation_router` route, `gateway_ingress` dispatch, `command_handler`, `registry_cow.ResolveOwner`), add an `AllocsPerRun` assertion pinned to its documented floor: `0` for stack-pinned, amortized-`0` for pooled (state the floor inline). Add one `BenchmarkRX_*` per entry point. No production code changes unless an assertion fails — then fix to the floor, never by relaxing the assertion.
## ACCEPTANCE (task-scoped — the standing §8 gate is assumed)
> - Each assertion fails first if its floor is regressed (verify by temporarily forcing one alloc), then passes — proving it's live, not vacuous.
> - Pooled floors cite the reason for any non-zero (pool refill); stack-pinned floors are exactly `0`.
> - No assertion is met by weakening the path (`string` key, `any`, mutex on read, global state) — §6/§5 semantics unchanged.
> - CoW write path and TX carry **no** alloc assertions (correctly allocating, exempt).