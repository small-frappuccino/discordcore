# DISCORDCORE :: AGENTIC EXECUTION HARNESS  (/goal)

`AGENTS.md` is the binding architectural contract (§2–6) and is already loaded.
This goal does NOT restate architecture — it governs ONLY loop execution and convergence velocity.
Reasoning budget is finite: spend it verifying against `AGENTS.md`, never re-deriving it.

## TASK
> {one sentence, explicit target symbol / file / package}
>
> e.g. *"Migrate `Feature` (string → uint8 enum) across the routing path; enforce §5 cardinality + fail-closed uniqueness at the `UpdateRoute` writer boundary."*

## CONTEXT PHASE — explore + reason, interleaved
*(This is where thinking-time accrues. Collapse it.)*

- Read only the **dependency edge** of the target: every type, interface, and call site you will touch or invoke.
- **STOP CRITERION (binary, not subjective):** the type graph *closes* — every symbol you will write or call resolves to a concrete definition you have already read.
    - Reading past the closed graph = wasted budget. Reading short of it = hallucinated signatures = broken AST. Stop exactly at closure.
- Do NOT scan the repo breadth-first. Do NOT re-read a file already in context. Do NOT re-litigate design settled in `AGENTS.md`, and do NOT enumerate alternatives you will not take.
- Reasoning is restricted to three operations: escape-analysis on the touched hot path, invariant verification (§5/§6), and resolving the minimal AST delta.

## ACTION PHASE — mutate + verify

- **MUTATE:** emit the minimal AST delta. A type/signature change has a *known blast radius* — you already read every call site in CONTEXT, so fix all of them in ONE pass, never iteratively. State each non-obvious assumption in one inline line.
- **VERIFY (termination is external and deterministic):** the loop is DONE only when all four gates are green —
    - `go build ./...`
    - `go test -race ./...`
    - `go build -gcflags="-m"` → zero new heap escapes on the touched hot path
    - `golangci-lint run`
- A red gate re-enters MUTATE with the failure as the SOLE new input. Never declare DONE on self-judgment — only on a green gate.

## CONVERGENCE CONTRACT

- **Forward-only.** Current AST + current gate output is $T_0$. No retraction loops, no apologies for prior iterations.
- **No clarifying questions** on architectural ambiguity: assume the `AGENTS.md`-aligned path, state the assumption in one line, execute.
- **Output:** AST/diff + single-line justifications. No chain-of-thought dump, no pedagogical prose.