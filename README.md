# Discordcore

Discordcore is the core Discord bot library and service layer used by Alicebot. It owns all Discord-facing behavior, data persistence, caching, and runtime configuration.

## Highlights

- Monitoring services for members, messages, reactions, and avatar changes
- Native AutoMod action logging
- Moderation and audit logging helpers
- Slash command framework with runtime configuration panel
- Postgres-backed persistence for metrics and message history
- Unified cache with TTL and persistence
- Task router for backfill and scheduled jobs
- Gateway handler performance warnings (slow-path logging)

## Repository layout

```
cmd/discordmain/      # Primary main runtime entrypoint
cmd/discordqotd/      # QOTD-specialized runtime entrypoint
pkg/discord/          # Discord services, logging, commands, cache
pkg/files/            # Bot config model and persistence stores
pkg/persistence/      # DB connection, health, migrator
pkg/partners/         # Partner board rendering services (template + list -> embeds)
pkg/storage/          # Bot domain persistence store (Postgres)
pkg/task/             # Task router and scheduler
pkg/util/             # Shared utilities
ui/                   # Embedded dashboard source, build output, and //go:embed helper
```

## Quick start (example)

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

func main() {
	token, err := util.LoadEnvWithLocalBinFallback("ALICE_BOT_PRODUCTION_TOKEN")
	if err != nil {
		log.Fatal(err)
	}

	databaseURL := os.Getenv("ALICE_DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("ALICE_DATABASE_URL is required")
	}

	db, err := persistence.Open(context.Background(), persistence.Config{
		Driver:      "postgres",
		DatabaseURL: databaseURL,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := persistence.NewPostgresMigrator(db).Up(context.Background()); err != nil {
		log.Fatal(err)
	}

	cfg := files.NewConfigManagerWithStore(files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey))
	if err := cfg.LoadConfig(); err != nil {
		log.Fatal(err)
	}

	dg, err := session.NewDiscordSession(token)
	if err != nil {
		log.Fatal(err)
	}

	botCfg := cfg.Config()
	if botCfg == nil {
		log.Fatal("config store not loaded")
	}

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		log.Fatal(err)
	}

	monitor, err := logging.NewMonitoringService(dg, cfg, store)
	if err != nil {
		log.Fatal(err)
	}
	automod := logging.NewAutomodService(dg, cfg)
	cmds := commands.NewCommandHandler(dg, cfg)

	if err := monitor.Start(); err != nil {
		log.Fatal(err)
	}
	automod.Start()
	if err := cmds.SetupCommands(); err != nil {
		log.Fatal(err)
	}

	defer monitor.Stop()
	defer automod.Stop()

	util.WaitForInterrupt()
}
```

## Configuration (canonical config store)

The canonical bot config is stored in Postgres as one JSONB document. A minimal example:

```json
{
  "guilds": [
    {
      "guild_id": "123456789",
      "channels": {
        "commands": "987654321",
        "avatar_logging": "111111111",
        "role_update": "111111111",
        "member_join": "444444444",
        "member_leave": "444444444",
        "message_edit": "999999999",
        "message_delete": "999999999",
        "automod_action": "222222222",
        "moderation_case": "333333334",
        "clean_action": "333333335",
        "entry_backfill": "555555555",
        "verification_cleanup": "666666666"
      },
      "features": {
        "logging": {
          "avatar_logging": true,
          "role_update": true,
          "member_join": true,
          "member_leave": true,
          "message_process": true,
          "message_edit": true,
          "message_delete": true,
          "reaction_metric": true,
          "automod_action": true,
          "moderation_case": true,
          "clean_action": false
        }
      },
      "roles": {
        "allowed": ["333333333"],
        "verification_role": "333333335"
      },
      "user_prune": {
        "enabled": false
      },
      "runtime_config": {
        "disable_message_logs": false
      }
    }
  ],
  "runtime_config": {
    "database": {
      "driver": "postgres",
      "database_url": "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable",
      "max_open_conns": 20,
      "max_idle_conns": 10,
      "conn_max_lifetime_secs": 1800,
      "conn_max_idle_time_secs": 300,
      "ping_timeout_ms": 5000
    },
    "moderation_logging": true,
    "webhook_embed_updates": [
      {
        "message_id": "123456789012345678",
        "webhook_url": "https://discord.com/api/webhooks/WEBHOOK_ID/WEBHOOK_TOKEN",
        "embed": {
          "title": "Updated embed title",
          "description": "Updated embed description"
        }
      }
    ]
  }
}
```

## Runtime configuration panel

Use `/config runtime` in Discord to edit persisted runtime config. Toggles include:

- `disable_entry_exit_logs`
- `disable_user_logs`
- `disable_message_logs`
- `disable_reaction_logs`
- `disable_automod_logs`
- `moderation_logging`
- `message_cache_ttl_hours`
- `message_delete_on_log`
- `message_cache_cleanup`
- `presence_watch_user_id`
- `presence_watch_bot`
- `backfill_channel_id`
- `backfill_start_day`
- `backfill_initial_date`
- `disable_bot_role_perm_mirror`
- `bot_role_perm_mirror_actor_role_id`
- `webhook_embed_updates` (manual JSON list: message_id + webhook_url + embed)

Webhook embed update CRUD commands:

- `/config webhook_embed_create`
- `/config webhook_embed_read`
- `/config webhook_embed_update`
- `/config webhook_embed_delete`
- `/config webhook_embed_list`

Partner CRUD commands:

- `/partner add`
- `/partner read`
- `/partner update`
- `/partner delete`
- `/partner list`
- `/partner sync`

Note: `/addpartner` is not registered. Use `/partner add`.

## Control API (Bearer + OAuth session)

When the shared runner is used via `Run`, the Control API starts on the default listener `127.0.0.1:8376`. Hosts can override the listener and public origin through `RunWithOptions`; `discordmain` uses that path to expose embedded local HTTPS on `127.0.0.1:8443` with canonical public origin `https://discordmain.localhost:8443`. The control server serves a minimal landing page at `/`, while the embedded dashboard is mounted canonically under `/manage/` with `/dashboard/` retained only as a legacy compatibility alias.

- `ALICE_CONTROL_BEARER_TOKEN` (optional; enables trusted internal bearer auth for control routes)
- `ALICE_CONTROL_PUBLIC_ORIGIN` (optional; absolute canonical browser origin such as `https://discordmain.localhost:8443`)
- optional TLS listener:
  - `ALICE_CONTROL_TLS_CERT_FILE`
  - `ALICE_CONTROL_TLS_KEY_FILE`

Behavior summary:

- `/` serves the minimal control landing page with Discord login and dashboard entry actions.
- `/manage/` is the canonical authenticated dashboard base path.
- `/dashboard/` remains supported as a legacy compatibility alias and resolves into the same authenticated dashboard shell.
- Guild-specific web configuration uses Discord OAuth session auth and is limited to guilds returned by `/auth/guilds/manageable`.
- When bearer auth is configured without Discord OAuth, the control API remains available for trusted automation, but the browser dashboard stays gated because no Discord session can be established.
- If the fixed listener `127.0.0.1:8376` cannot be bound, startup continues without the control server and the failure is logged as degraded control-plane availability.

Authentication contract for `/v1/*` routes:

- Supports `Authorization: Bearer <token>` (internal automation) or Discord OAuth session cookie (dashboard).
- Bearer: missing/invalid scheme/token returns `401`, wrong token returns `403`.
- Bearer is rejected when an `Origin` header is present (browser context).
- Session: created by OAuth callback and read from HttpOnly cookie.
- OAuth cookies are always issued with `HttpOnly`, `SameSite=Lax`, and `Secure`.
- For OAuth session requests, mutable routes (`POST`/`PUT`/`DELETE`) require `X-CSRF-Token` matching the server-issued token.
- Requests without any valid auth return `401`.
- Guild routes under `/v1/guilds/{guild_id}/*` require guild-level authorization for OAuth sessions (`owner` or `ADMINISTRATOR`/`MANAGE_GUILD`, intersected with guilds where the bot is present).

Discord OAuth2 endpoints (optional, same control server):

- `GET /auth/discord/login` redirects to Discord OAuth authorize URL and emits anti-CSRF `state` cookie.
- `GET /auth/discord/callback` validates `state`, exchanges `code` at Discord token endpoint, resolves `/users/@me`, creates server-side session, and sets session cookie.
- `GET /auth/discord/login?next=/` stores the post-login landing redirect, and the callback redirects back to `/` after the session cookie is issued.
- `GET /auth/discord/login?next=/manage/` is the canonical way to return directly to the authenticated dashboard shell.
- `GET /auth/discord/login?next=/dashboard/` remains valid as a legacy compatibility alias when older callers still send it.
- `GET /auth/me` returns current authenticated session user.
- `GET /auth/me` also returns `csrf_token` for explicit CSRF header usage.
- OAuth status/login/dashboard URLs are emitted against the configured public origin when available, so browser flows do not start on one host and callback on another.
- `POST /auth/logout` invalidates current session and clears session cookie.
- `GET /auth/guilds/manageable` lists guilds from `/users/@me/guilds` (Discord OAuth user token, paginated at `limit=200`), filtered to `owner` or `ADMINISTRATOR`/`MANAGE_GUILD`, then intersected with guild IDs where the bot is present.
- OAuth sessions are persisted on disk (not only in memory), so authenticated sessions survive process restart until session expiry/logout.
- Discord access tokens are refreshed server-side via `refresh_token`; when Discord rotates the refresh token, the session store is updated atomically.
- Token exchange request uses `Content-Type: application/x-www-form-urlencoded`.
- OAuth cookie-based auth requires HTTPS transport because cookies are always `Secure`.

Enable OAuth routes by setting these vars:

- `ALICE_CONTROL_DISCORD_OAUTH_CLIENT_SECRET`
- `ALICE_CONTROL_DISCORD_OAUTH_REDIRECT_URI` (optional when `ALICE_CONTROL_PUBLIC_ORIGIN` or host `RunWithOptions` wiring can derive the callback)
- `ALICE_CONTROL_DISCORD_OAUTH_SESSION_STORE_PATH` (optional; defaults to `<app-cache>/control/oauth_sessions.json`)
- use `ALICE_CONTROL_TLS_CERT_FILE` + `ALICE_CONTROL_TLS_KEY_FILE` for direct HTTPS on the control listener, or configure host-managed HTTPS such as `discordmain`'s embedded local TLS mode.

The product ships with a versioned default Discord OAuth client ID (`1396606252506681395`).
Set `ALICE_CONTROL_DISCORD_OAUTH_CLIENT_ID` only if you need to override that default with a different Discord application.
When `ALICE_CONTROL_DISCORD_OAUTH_CLIENT_SECRET` is set and `ALICE_CONTROL_DISCORD_OAUTH_REDIRECT_URI` is unset, Discordcore derives the redirect URI from the configured public origin and the fixed callback path `/auth/discord/callback`.

Scopes:

- default/minimum: `identify guilds`
- optional member scope: set `ALICE_CONTROL_DISCORD_OAUTH_INCLUDE_GUILDS_MEMBERS_READ=true` to include `guilds.members.read`

Partner board endpoints (all under `/v1/guilds/{guild_id}`):

- `GET /partner-board`
- `GET|PUT /partner-board/target`
- `GET|PUT /partner-board/template`
- `GET|POST|PUT|DELETE /partner-board/partners`
- `POST /partner-board/sync`

Notes:

- In guild context, omitted `scope` defaults to `guild` (safer default).
- Use `scope=global` explicitly when you want to change global runtime config.
- `webhook_embed_create`, `webhook_embed_update`, and `webhook_embed_delete` support `apply_now=true` to patch the message immediately.

## Partner board renderer foundation

`pkg/partners` provides a reusable render service that converts:

- a board template (`PartnerBoardTemplate`)
- a partner list (`[]PartnerRecord`)

into final Discord embeds (`[]*discordgo.MessageEmbed`).

Current capabilities:

- token-based templates for section headers, lines, and footer text
- deterministic grouping by fandom (with optional sort disable)
- deterministic partner sorting within each fandom (with optional sort disable)
- auto-splitting across multiple embeds while respecting description limits
- validation for required fields and URL format

This package is intentionally UI-agnostic and command-agnostic.

## Partner board config primitives

Guild config now includes `partner_board` with:

- `target` (`type`, `message_id`, `channel_id`, `webhook_url`)
- `template` (render template fields)
- `partners` (fandom/name/link records)

Target abstraction supports:

- `webhook_message` (`message_id` + `webhook_url`)
- `channel_message` (`message_id` + `channel_id`)

Partner CRUD helper methods are available on `ConfigManager`:

- `SetPartnerBoardTarget` / `GetPartnerBoardTarget`
- `CreatePartner` / `GetPartner` / `ListPartners` / `UpdatePartner` / `DeletePartner`

Rules enforced by CRUD:

- invite link must be a Discord invite URL
- deduplication by normalized partner name and invite link
- deterministic ordering by fandom, then name, then link

## Dashboard scaffold

`ui/` contains the Bun + Vite + React + TypeScript dashboard scaffold for the Control API:

- typed Control API client (`ui/src/api/control.ts`)
- partner board admin wiring (`ui/src/App.tsx`)
- baseline ESLint and TypeScript configuration
- `ui/embed.go` embeds `ui/dist` into the final bot binary
- `ui/dist/index.html` is a versioned embed shell that never points at hashed build artifacts directly
- `bun run build` writes ignored hashed bundles plus `.vite/manifest.json`, then restores the tracked embed shell so backend-only Go builds remain safe

Local dev contract:

- The Vite dev server proxies `/auth/*` and `/v1/*` to `VITE_CONTROL_API_PROXY_TARGET` (default: `http://127.0.0.1:8080`)
- `VITE_CONTROL_API_BASE_URL` defaults to current origin
- Dashboard requests use OAuth session cookie auth (`credentials: include`); bearer tokens are not stored in browser code.
- `VITE_CONTROL_API_GUILD_ID` can prefill the guild selector
- Production builds use Vite `base: "/manage/"`, and the embedded dashboard is served by the control server under `/manage/`
- `/dashboard/` remains available only as a legacy compatibility alias
- The public landing page is served by the Go control server at `/`; the React bundle remains dashboard-only.

Quick start:

```bash
cd ui
bun install
bun run dev
```

Embedded dashboard build flow:

```bash
cd ui
bun run build
cd ../../alicebot
go build -o discordmain ./cmd/discordmain
```

Build behavior:

- `ui/dist/index.html` remains the tracked shell used by `//go:embed`
- when `.vite/manifest.json` and hashed assets are present, the shell loads the built React entrypoint from `/manage/`
- when frontend assets are absent, the shell stays in placeholder mode with a clear message instead of embedding broken hashed paths

`discordcore` owns the Control API routes, OAuth/session handling, partner board services, and all Discord/domain rules consumed by that frontend. The final executable remains the `discordmain` binary, which embeds the assets produced in `discordcore/ui/dist`.

Policy precedence for logging/event emission:

1. `runtime_config.disable_*` (or `runtime_config.moderation_logging=false`) acts as an operational kill switch and wins first.
2. `features.logging.*` controls product behavior (fine-grain enablement) when kill switch is not active.
3. Channel resolution/validation and intent checks run after toggles.

## Entry/exit backfill

Backfill runs automatically on startup when configured:

- If `backfill_start_day` is set, a day scan runs for that date.
- Otherwise, if `backfill_initial_date` is set and there is no prior progress, a range scan runs from that date to now.
- If a last event exists and downtime exceeds the threshold, a range scan runs from last event to now.

Channels are resolved in this order:

- `runtime_config.backfill_channel_id` (global)
- `channels.entry_backfill`

Parsed sources:

- Alicebot embeds titled "Member Joined" / "Member Left"
- Mimu-style welcome/goodbye messages with mentions

## Gateway performance warnings

Slow gateway handlers are logged by default.

- `ALICE_GATEWAY_PERF_THRESHOLD_MS` (default: 200)
- Set to `0` to disable

## Required permissions

- View Channels
- Send Messages
- Embed Links
- Read Message History
- Use Slash Commands

## Testing

```bash
go test ./pkg/qotd ./pkg/discord/commands/qotd ./pkg/files ./pkg/discord/commands/config
go vet ./...
cd ui
bun run build
```

QOTD DB-backed integration suites are excluded from the default `go test` path behind the `integration` build tag. Run them explicitly:

```bash
set DISCORDCORE_TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable
powershell -ExecutionPolicy Bypass -File .\scripts\test-qotd-integration.ps1
```

Focused QOTD race coverage uses the same test database and requires CGO support:

```bash
set DISCORDCORE_TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable
set CGO_ENABLED=1
powershell -ExecutionPolicy Bypass -File .\scripts\test-qotd-race.ps1
```

Other Postgres-backed suites outside the QOTD area still require `DISCORDCORE_TEST_DATABASE_URL` when run directly.

## License

Internal project. Refer to the repository license for terms.
