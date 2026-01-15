# Discordcore

Discordcore is the core Discord bot library and service layer used by Alicebot. It owns all Discord-facing behavior, data persistence, caching, and runtime configuration.

## Highlights

- Monitoring services for members, messages, reactions, and avatar changes
- Native AutoMod action logging
- Moderation and audit logging helpers
- Slash command framework with runtime configuration panel
- SQLite-backed persistence for metrics and message history
- Unified cache with TTL and persistence
- Task router for backfill and scheduled jobs
- Gateway handler performance warnings (slow-path logging)

## Repository layout

```
cmd/discordcore/      # Example runner
pkg/discord/          # Discord services, logging, commands, cache
pkg/files/            # settings.json configuration
pkg/storage/          # SQLite store
pkg/task/             # Task router and scheduler
pkg/util/             # Shared utilities
```

## Quick start (example)

```go
package main

import (
	"log"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

func main() {
	cfg := files.NewConfigManager()
	token, err := util.LoadEnvWithLocalBinFallback("ALICE_BOT_PRODUCTION_TOKEN")
	if err != nil {
		log.Fatal(err)
	}

	dg, err := session.NewDiscordSession(token)
	if err != nil {
		log.Fatal(err)
	}

	store := storage.NewStore(util.GetMessageDBPath())
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

## Configuration (settings.json)

A minimal example:

```json
{
  "guilds": [
    {
      "guild_id": "123456789",
      "command_channel_id": "987654321",
      "user_log_channel_id": "111111111",
      "user_entry_leave_channel_id": "444444444",
      "welcome_backlog_channel_id": "555555555",
      "verification_channel_id": "666666666",
      "message_log_channel_id": "999999999",
      "automod_log_channel_id": "222222222",
      "allowed_roles": ["333333333"],
      "runtime_config": {
        "disable_message_logs": false
      }
    }
  ],
  "runtime_config": {
    "moderation_log_mode": "alice_only"
  }
}
```

## Runtime configuration panel

Use `/config runtime` in Discord to edit `settings.json` at runtime. Toggles include:

- `disable_entry_exit_logs`
- `disable_user_logs`
- `disable_message_logs`
- `disable_reaction_logs`
- `disable_automod_logs`
- `moderation_log_mode`
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

## Entry/exit backfill

Backfill runs automatically on startup when configured:

- If `backfill_start_day` is set, a day scan runs for that date.
- Otherwise, if `backfill_initial_date` is set and there is no prior progress, a range scan runs from that date to now.
- If a last event exists and downtime exceeds the threshold, a range scan runs from last event to now.

Channels are resolved in this order:

- `runtime_config.backfill_channel_id` (global)
- `welcome_backlog_channel_id`
- `user_entry_leave_channel_id`

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

go test ./...

go vet ./...
```

## License

Internal project. Refer to the repository license for terms.
