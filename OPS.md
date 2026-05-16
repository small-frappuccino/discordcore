# Operations Guide

Runbook for keeping `discordcore` bots online. Companion to `README.md`
(product) and `CLAUDE.md`/`AGENTS.md` (engineering rules).

## What "online" means for this repo

A bot in this repo runs as a single Windows process (`discordmain.exe`,
`discordqotd.exe`, etc.) holding:

1. The Discord gateway WebSocket session.
2. A Postgres connection pool (`pgx`).
3. The embedded control HTTP server (`/v1/...`, `/manage/...`).

A hard process crash kills all three. Anthropic Claude or a human operator
will not be paged automatically — alerts come from the layers below.

## Three-layer outage defense

The repo combines three independent defenses against silent outages. Each
layer covers a different failure class; **all three should be wired in
production**, not just one.

| Layer | Where it lives | Covers |
|-------|----------------|--------|
| 1. `pgx` body-size cap | `pkg/persistence/open.go` (`maxPostgresMessageBodyBytes`) | Corrupted Postgres protocol streams that would otherwise allocate gigabytes and trigger `runtime.throw`. The driver returns an `error` instead of crashing. |
| 2. Lifecycle webhook | `pkg/app/lifecycle_webhook.go`, env `ALICE_LIFECYCLE_WEBHOOK_URL` | Graceful stops and recoverable panics. Posts a chat message before the deferred path unwinds. Does **not** cover `runtime.throw`. |
| 3. External liveness poller | `GET /v1/health/live` + an off-host poller (NSSM/Task Scheduler/UptimeRobot/etc.) | Any crash mode, including the ones Layer 2 misses. The only signal that survives `runtime.throw`, OOM kill, machine reboot, lost network. |

## Layer 1 — Postgres message size cap

Already on by default. The hard limit is 64 MiB per Postgres protocol
message; legitimate workloads in this repo trade kilobytes, so the cap
fires only on stream corruption (e.g. a load balancer returns HTML during
the connect handshake). When it fires, the connect or query returns a
normal `*pgconn.PgError` / `*pgproto3.ExceededMaxBodyLenErr`, which
`database/sql` surfaces through `Open` / `Ping` / `Query` and the
existing `pkg/persistence` log path picks up.

No configuration required. If you need to raise the ceiling for an
unusual workload, change the constant in `pkg/persistence/open.go` and
add a regression test.

## Layer 2 — Lifecycle webhook (graceful events)

Discord webhooks are the cheapest "is the bot up" signal an operator can
read in chat.

### Setup

1. In Discord: server settings → integrations → webhooks → "New Webhook"
   on the alerts channel. Copy the URL.
2. On the host running the bot, set the env var **before** launching the
   process:

   ```powershell
   setx ALICE_LIFECYCLE_WEBHOOK_URL "https://discord.com/api/webhooks/.../..."
   ```

   `setx` persists into the user/machine environment. For a service
   running as `LocalSystem` (NSSM/Task Scheduler), set it on the service
   environment (NSSM: "Environment" tab; Task Scheduler: action's
   "Add arguments" + a wrapper script that exports the var).

3. Restart the bot. On the next start you should see a
   `<botname> (v...) → starting` message in the alerts channel.

### What you'll see

- `... → starting` once per process boot.
- `... → stopping` once per clean exit (Ctrl-C, supervisor stop, normal
  shutdown).
- `... → fatal — <detail>` if a Go panic propagates to `Run`.

### What you will **not** see

- Anything from a `runtime.throw` (memory allocation failure, etc.) —
  these bypass deferred handlers. Layer 3 catches those.
- Anything from a machine power-loss or BSOD — Layer 3 catches those.

## Layer 3 — External liveness poller (catches everything)

`GET /v1/health/live` returns a small JSON payload identifying the bot,
its version, and its uptime. It requires the same bearer auth as the
rest of `/v1/*`.

```bash
curl -sf -H "Authorization: Bearer $ALICE_CONTROL_BEARER_TOKEN" \
     https://your-host:8376/v1/health/live
# {"status":"ok","app":"discordmain","app_version":"v0.146.0",...}
```

If the bot is dead, the request fails with a connection error or a
timeout. The external poller alerts on that.

### Three free options for the poller

Pick one. All are equivalent in coverage — the choice depends on what
you already have.

#### Option A — UptimeRobot (zero infra)

1. Create an HTTP(S) keyword monitor at uptimerobot.com (free tier:
   50 monitors, 5-minute interval).
2. URL: `https://your-host:8376/v1/health/live`.
3. Advanced settings → HTTP headers:
   `Authorization: Bearer <ALICE_CONTROL_BEARER_TOKEN>`.
4. Keyword: `"status":"ok"`.
5. Alert contacts: Discord webhook integration (built-in).

Caveat: your control server must be reachable from the public internet
(or via a tunnel like Cloudflare Tunnel / Tailscale Funnel).

#### Option B — Cron / scheduled task on a second host

If you have any second machine, the script is one line:

```bash
#!/usr/bin/env bash
# Run every 5 min. Adjust URL and webhook.
TARGET="https://your-host:8376/v1/health/live"
ALERT="https://discord.com/api/webhooks/.../..."
if ! curl -fsS --max-time 10 \
     -H "Authorization: Bearer $ALICE_CONTROL_BEARER_TOKEN" \
     "$TARGET" >/dev/null; then
  curl -s -H "Content-Type: application/json" \
       -d "{\"content\":\"🚨 Bot at $TARGET not responding\"}" \
       "$ALERT"
fi
```

Add to crontab: `*/5 * * * * /path/to/check.sh`.

#### Option C — GitHub Actions scheduled workflow

Free 2000 minutes/month is plenty for a 5-minute poll. Drop this in
`.github/workflows/bot-liveness.yml` in a **private** repo (do not put
this in `discordcore` itself — the bearer token must stay private):

```yaml
name: bot-liveness
on:
  schedule:
    - cron: "*/5 * * * *"
  workflow_dispatch:
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - name: Probe /v1/health/live
        env:
          TARGET: ${{ secrets.BOT_HEALTH_URL }}
          BEARER: ${{ secrets.BOT_BEARER }}
          ALERT: ${{ secrets.ALERT_WEBHOOK }}
        run: |
          set -e
          if ! curl -fsS --max-time 10 -H "Authorization: Bearer $BEARER" "$TARGET" >/dev/null; then
            curl -s -H "Content-Type: application/json" \
                 -d '{"content":"🚨 Bot not responding"}' "$ALERT"
            exit 1
          fi
```

## Process supervision (auto-restart)

Layers 2/3 tell you something went wrong. Process supervision makes the
bot **come back** on its own without operator intervention. Pick one:

### NSSM (recommended on Windows)

[NSSM](https://nssm.cc/) wraps any executable as a real Windows service
with automatic restart on failure.

```powershell
# Install (one-time)
choco install nssm  # or download from nssm.cc

# Register the bot as a service
nssm install discordmain "D:\path\to\discordmain.exe"
nssm set discordmain AppDirectory "D:\path\to"
nssm set discordmain AppEnvironmentExtra ^
     "ALICE_LIFECYCLE_WEBHOOK_URL=https://discord.com/api/webhooks/.../..." ^
     "ALICE_CONTROL_BEARER_TOKEN=..."
nssm set discordmain AppStdout "D:\path\to\logs\discordmain.log"
nssm set discordmain AppStderr "D:\path\to\logs\discordmain.err.log"
nssm set discordmain AppRotateFiles 1
nssm set discordmain AppRotateBytes 10485760
nssm set discordmain AppExit Default Restart
nssm set discordmain AppRestartDelay 5000

# Start it
nssm start discordmain
```

Repeat with `discordqotd` for the QOTD bot.

### Task Scheduler (no extra install)

If you cannot install NSSM, the built-in Task Scheduler works:

1. Open Task Scheduler → "Create Task" (not "Basic Task").
2. **General** tab: name `discordmain`. Run whether user is logged on
   or not. "Run with highest privileges" only if the bot needs admin.
3. **Triggers**: "At startup", delay 30 seconds.
4. **Actions**: "Start a program" → `D:\path\to\discordmain.exe`.
   "Start in": parent directory.
5. **Settings** tab — this is the key part:
   - ☑ "If the task fails, restart every: 1 minute".
   - "Attempt to restart up to: 999 times".
   - ☑ "If the running task does not end when requested, force it to stop".
   - ☐ "Stop the task if it runs longer than: ..." (uncheck).
6. Save (prompts for password to run on user logon).

Set the env vars on the task's user account (`setx`) before the trigger
fires for the first time.

### sc.exe + recovery actions (lower-level)

For headless servers without GUI:

```powershell
sc.exe create discordmain binPath= "D:\path\to\discordmain.exe" ^
       start= auto displayname= "Discordcore main bot"
sc.exe failure discordmain reset= 86400 actions= restart/5000/restart/5000/restart/30000
sc.exe failureflag discordmain 1
sc.exe start discordmain
```

`actions=` is `action/delay-ms` triples. The example restarts twice
after 5 s, then every 30 s thereafter, with the failure counter resetting
every 24 h.

## Postmortem checklist

When the alert channel says a bot fell over:

1. Read the most recent `level=FATAL`/`fatal error` line in
   `D:\Users\smallfrappuccino\.local\share\<bot>\logs\` (or wherever
   you redirected stdout/stderr in NSSM). If it's `runtime: cannot
   allocate memory` with a `pgx/v5/internal/iobufpool.Get(<large>)`
   frame, the 64 MiB cap is too low and you should investigate the
   upstream provider, not the cap.
2. Cross-check `/v1/health/qotd` and `/v1/health/moderation` snapshots
   from the last successful poll — `failure_by_cause` shows whether the
   crash followed a spike in `discord_unavailable` / `state_divergence`
   / `fetch_*` counters, which narrows the root cause.
3. The supervisor should already have relaunched. Confirm by hitting
   `/v1/health/live` directly and checking `uptime_seconds` is small.
4. If the same crash recurs, capture the stack trace, the few lines of
   `level=INFO`/`WARN` immediately preceding it, and the
   `/v1/health/*` snapshots — those are the inputs an investigation
   needs.
