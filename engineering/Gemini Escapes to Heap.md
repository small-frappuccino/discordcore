# discordcore — refactor (behavior-preserving, 1:1)

Per AGENTS.md. Emit complete compilable Go — no implied imports, no placeholders — plus a testing.B with ReportAllocs() per target. Optimize ONLY within verified hot paths; leave cold paths idiomatic and simple. State memory-alignment assumptions in one line, then emit.

TARGETS:
- Workers: bounded, context-aware lifecycle, explicit load-shedding.
- Moderation ingress (/ban, /massban, /kick): concrete Application Command defs + zero-alloc parsers. Extract snowflake IDs via strconv.ParseUint straight from the interaction byte buffer into stack-allocated structs — no reflection maps — dispatch async to the Guild Actor inbox without blocking the ACK gateway thread.
- Rate limiting: at the HTTP transport, read X-RateLimit-Remaining / X-RateLimit-Reset-After; apply per-bucket token-bucket backpressure to the calling Actor's dispatch loop. Zero 429, no stalling of orthogonal routes.