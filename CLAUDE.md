# CLAUDE.md — `discordcore` · Opus 4.8 Agentic Contract

> **Effort: Extra (`xhigh`).** Set it in the Claude Code effort menu — this file is written for it.
> Single source of truth for Claude Code in this repo. **Self-contained: no `@imports`.**
> Target: bare-metal, extreme-throughput Discord API routing in Go as a Tier-1 systems language.

The system is **two paths with opposite optimization regimes**. **Gateway-RX** (event ingestion) is allocation-sensitive and CPU-bound. **REST-TX** (command egress) is rate-limited and network-bound. Conflating them is the root architectural error: a sub-millisecond GC pause is irrelevant against a 40 ms API round-trip, and a mishandled `429` backoff destroys more throughput than any allocation ever will.

## 1. Operating Contract (read first)

- **Effort allocation.** Adaptive thinking sets *how much* you reason; this file sets *where* it lands. Architecture is settled below — do NOT spend the budget re-deriving it. Spend it on exactly three things: (1) escape-analysis on the RX/compute hot path, (2) §6 invariant verification at the single-writer boundary, (3) the minimal AST delta across a change's full blast radius. Never on the TX path (§7.2), never on re-litigating settled design.
- **Bias to act.** Your default favors reasoning over tool calls and over-explains in interactive sessions. Counter both. Once the type graph closes (§3), prefer a tool call to further deliberation. Reasoning stays in thinking blocks; the response body is AST/diff plus one-line justifications — never the chain-of-thought.
- **Forward-only.** Dialogue history is a read-only cache; compute against current inputs; on divergence, pivot without retraction or apology. The current AST + current gate output is $T_0$. No clarifying questions on architectural ambiguity — assume the most defensible state-of-the-art path, state the assumption in one line, execute.
- **Self-audit, external arbiter.** Your flaw-detection is the primary self-check, but "done" is decided by the §8 toolchain gate going green, never by your judgment.
- **Identity.** Address the operator as Alice; demographic attributes are null variables. Collaborative but terse — no preamble, no postamble, no motivational filler.
- **Language.** Mirror input (English / fluid PT-BR). All identifiers, compiler flags, API nomenclature, and system variables stay in English. Model every construct as a bounded topology of stateful nodes and relational edges; `heap`/`stack`/`pipeline` are forbidden outside a software-engineering scope.

## 2. Epistemic Discipline

- **Falsification-first.** Evaluate a hypothesis by what would prove it false. Eradicate probabilistic hedging. Isolate knowledge gaps explicitly — never interpolate, guess, or smooth a missing signal. If a schema, interface, or import is absent, define the explicit boundary required to compile or surface the gap; never hallucinate internals.
- **Adversarial steelmanning.** Reconstruct non-neutral or biased input into its strongest form before evaluating. Pivot immediately if the steelman exposes a flaw in the plan.
- **Trade-off auditing.** Reject friction-free narratives and linear cause-effect. Treat dynamics as entropic systems with unavoidable trade-offs; surface hidden costs and structural extractions behind simplified summaries.

## 3. Execution Loop — CONTEXT → ACTION → VERIFY

**CONTEXT (bounded acquisition).** Read only the dependency edge of the target — every type, interface, and call site you will touch or invoke. **STOP CRITERION (binary):** the type graph *closes* — every symbol you will write or call resolves to a concrete definition you have already read. Past closure is wasted budget; short of it is hallucinated signatures. No breadth-first repo scans; no re-reading a file already in context.

**ACTION (forward-only mutation).** Emit the minimal AST delta. A type/signature change has a known blast radius — you read the call sites in CONTEXT, so fix every site in ONE pass, not iteratively. State each non-obvious assumption inline, one line.

**VERIFY (the toolchain is the oracle).** The loop is DONE only when the §8 gate is green. A red gate re-enters ACTION with the failure as the sole new input.

## 4. Allocation Regime & Go Doctrine

The allocation-sensitive domain is the Gateway-RX decode/dispatch pipeline (§7.1) and the in-memory routing table (§6); within it, data locality, CPU L1/L2 cache-line utilization, and predictable GC latency (targeting $<1$ms pauses) are absolute invariants. The REST-TX egress path (§7.2) is **EXEMPT** — its budget is spent on rate-limit scheduling, never allocation.

- **General craft.** `context.Context` (first parameter of any blocking call, never stored in a `struct`) + `errgroup` at every I/O boundary. Unbuffered channels for rigid rendezvous only. Replace `sync.RWMutex` under contention with sharding or lock-free structures. Block heap escapes via concrete types and value receivers; ban `reflect` and `interface{}/any` in hot paths; validate continuously with `go build -gcflags="-m"`. `strings.Builder` with pre-calculated `Grow(N)` over `+` concatenation; `strconv.AppendInt` over `fmt.Sprintf`/`strconv.Itoa` on pre-allocated buffers.
- **discordcore layout.** Hot loops yield zero heap allocations; APIs accept caller-allocated buffers (`[]byte`, pre-sized slices) via pointer. Struct fields strictly ordered by byte size (largest→smallest) to kill padding. Transient parsers, encoders, and byte buffers amortized via `sync.Pool`, zeroed before return.

## 5. Concurrency Architecture (CSP)

- **Bounded ingress.** Unbounded `go process()` per payload is a catastrophic vulnerability — route all WebSocket ingress through bounded worker pools or `x/sync/semaphore`. Buffered-channel depth is dimensioned by Little's Law ($L = \lambda W$) to absorb micro-bursts without hiding systemic latency.
- **Allocation-free transmission.** Channels carry concrete types, `sync.Pool` pointers, or flat data indices — never interfaces, never deep pointer graphs.
- **Actor per guild.** Each Discord Guild is a single goroutine draining a bounded inbox channel: serial state mutation, no global mutex, thousands of guilds concurrent without cross-talk. The config-mutation writer (§6) is itself an Actor draining the `LISTEN/NOTIFY` inbox.
- **CoW Feature Registry.** The read-heavy routing table (§6) is a `sync/atomic.Pointer[T]`; writers clone and swap via CAS, guaranteeing $O(1)$ lock-free reads for the ingress routers. Any entity crossing a channel boundary is immutable — `.Clone()` before mutation.

## 6. Multi-Tenant Routing Contract (Domain Invariants)

The routing table is the source of truth: `(GuildID, Feature) → BotInstance`. These invariants are **preconditions** of the hot path, enforced exclusively at the single-writer boundary (DB hydration + live `LISTEN/NOTIFY` mutation). The ingress router NEVER re-validates at read time; it reads the CoW snapshot (§5) lock-free.

- **Feature Cardinality (Uniqueness):** For any `(GuildID, Feature)`, exactly one `BotInstance` serves it at $T_n$. A write reassigning an already-bound feature is **route theft** — never a silent overwrite. **Default policy: `reject-if-bound`** (fail-closed): an operator must explicitly `DISABLE` the incumbent before a new bot can claim the feature, preventing an errant or hostile `NOTIFY` from silently transferring moderation authority. `atomic-reassign` is documented but not default — flip only with deliberate intent.
- **Bot Cardinality (Cap):** $|\{\text{distinct BotInstance} : \text{GuildID}=g\}| \le 5\ \forall g$. Enforced at **both** ingress points. On `ENABLE` for `(g, _, botB)`: if `botB ∉ currentBots(g) ∧ |currentBots(g)| ≥ 5` ⇒ reject. Assigning a feature to a bot *already present* does not increment the cap. Hydration encountering a 6th distinct bot is a data-integrity violation — skip and log loudly, never load.
- **Feature Scope (Typed Domain):** The full moderation set replaces the `ban`/`kick` pair. `Feature` MUST be `type Feature uint8`, never a `string`. A `uint8` over a bounded enum collapses the per-guild table to a fixed `[N]*BotInstance` array (or presence bitmask): branch-predictable $O(1)$ indexing, zero hashing, one cache line. A `string` key forces an $O(\text{len})$ hash per access and forecloses that representation — the allocation risk being any *composite-key construction*, not the lookup itself. Enumerate `Ban, Kick, Timeout, Deafen, MoveMember, MsgDelete, ChannelPurge, RoleAdd, RoleRemove, …`. Parse and validate at the boundary (DB `Scan` + JSON `Unmarshal`); an unknown `Feature` is rejected at ingestion, never routed.
- **Single-Writer Enforcement:** Cap and uniqueness require a serialized check-and-swap; the config-mutation path is therefore deliberately single-threaded (the Actor of §5). **"Everything concurrent" governs command *execution* (§7.2), NOT registry mutation** — parallelizing the writer reintroduces a TOCTOU race on the cardinality check. Not a bottleneck: mutation frequency is orders of magnitude below ingress.
- **Fallible Mutation API:** `Registry.UpdateRoute` and `RemoveRoute` MUST return `error` — void signatures cannot carry an invariant rejection and are non-compliant. Hydration and the watcher branch on the returned error and log the rejected mutation.
- **Secret Hygiene:** A bot token is a credential, not a field. `Token` MUST be a distinct type implementing `slog.LogValuer`, so structured logging of any containing struct (`BotInstance`, `ConfigChangeEvent`) can never leak it:
  ```go
  type Token string

  func (Token) LogValue() slog.Value { return slog.StringValue("[REDACTED]") }
  ```
  Plaintext `BotToken string` in a loggable struct is a non-compliant credential-exposure defect.

## 7. Discord Transport Layer (Direct REST / Gateway)

Removing Arikawa/Discordgo transfers their hardest responsibility — Discord's rate-limit and connection state machine — onto you. The CSP win over the old binary is here: the libraries serialized what is independently parallelizable.

### 7.1 Gateway-RX (Event Ingestion — Allocation-Sensitive)
- **Intent minimization is the primary lever.** Request only the Gateway Intents the active feature scope requires (`GUILD_MODERATION`, `GUILD_MEMBERS`, `GUILD_MESSAGES` as features demand). Every un-requested intent is an event you never receive, decode, or allocate for — a larger win than any per-event struct tuning. Never request `GUILD_PRESENCES` or `MESSAGE_CONTENT` (both privileged) unless a feature consumes them. RX intents exist to *observe* state; moderation is *executed* via REST.
- **Zero-allocation decode/dispatch.** `sync.Pool` the decode buffers, codegen the unmarshalers (`easyjson`/`msgp`), fan out flat event indices over bounded channels to the per-guild Actors (§5). No interface values cross the channel boundary.
- **Per-bot Gateway lifecycle (Actor per token).** `IDENTIFY` (Op 2) gated by the token's own `session_start_limit.max_concurrency` (≈5 s window); Heartbeat (Op 1) on the `Hello` (Op 10) interval with an **ACK (Op 11) watchdog** (missed ACK ⇒ force-close + resume); `RESUME` (Op 6) on transient drop preserving `session_id` + `seq`; full re-`IDENTIFY` only on a non-resumable Invalid Session (Op 9 `d:false`) or `Reconnect` (Op 7) that resume rejects; shard only near the ~2500-guild ceiling.

### 7.2 REST-TX (Command Egress — Rate-Limited, Network-Bound)
- **GC is irrelevant here** — a $<1$ms pause is noise against a ~40 ms round-trip. Spend zero optimization budget on TX allocation.
- **Per-bot, per-route bucket accounting.** Each token is an independent rate-limit domain; buckets key on route + major parameter (`guild_id`/`channel_id`/`webhook_id`). Read `X-RateLimit-Bucket`/`-Remaining`/`-Reset-After`; throttle *before* sending via a per-bucket token gate scoped per bot.
- **Global limit + 429 discipline.** Respect the $50$ req/s global ceiling **per token**. On `429`, honor `Retry-After` and `X-RateLimit-Scope`. A `429` storm trips the $10{,}000$-invalid-requests/$10$ min Cloudflare ban (invalid = `401`/`403`/`429`) — the backoff path is the single highest-leverage correctness surface in TX.
- **Concurrency model.** Distinct buckets and bots run concurrently; the same bucket serializes through its gate. The ceiling is $\sum(\text{per-token global limits})$ bounded by the $\le 5$-bot cap and 1-feature-1-bot rule — **not** CPU cores. Scale the bot pool against the aggregate req/s target.

## 8. Toolchain & Verify Gate

- **Modernization.** `iter.Seq[V]` / `iter.Seq2[K,V]` for batch retrieval; memory-heavy DB cursors yield lazily into the CSP pipeline. Release builds compile with `-pgo=auto`, hot paths architected for aggressive inlining against production `default.pgo`.
- **The gate (loop terminator — §3 VERIFY).** DONE iff all are green:
    - `gofmt -l .` → empty
    - `go vet ./...`
    - `golangci-lint run` (K8s-level strictness)
    - `go test -race ./...`
    - `govulncheck ./...`
    - `go build -gcflags="-m" ./...` filtered to the RX hot-path files — **no new heap escapes**:
      ```pwsh
      $env:GOOS="windows" ; $env:GOARCH="amd64" ; $env:CGO_ENABLED="0" ; go clean -cache ; go build -a -gcflags="./...=-m" ./... 2>&1 | Select-String -Pattern "escapes to heap|moved to heap"
      ```
      A non-empty result on a hot-path file is a regression; pool/construct/third-party noise is already filtered out.