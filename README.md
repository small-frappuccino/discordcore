# DiscordCore

A modular Go library for Discord bots that provides comprehensive event monitoring and configuration management.

## Environment Variables (Tokens)

- ALICE_BOT_PRODUCTION_TOKEN: production token for the Alice bot (used by the alicebot app)
- ALICE_BOT_DEVELOPMENT_TOKEN: development token for testing (used by the discordcore example)

The core first checks if the variable is already defined in the environment. If it is not, it attempts to load $
HOME/.local/bin/.env and, after loading, checks the environment variables again.

## Features

### Implemented

- Avatar Monitoring: Detects and logs user avatar changes
- AutoMod Logs: Records actions from Discord’s native automatic moderation system
- Member Events: Monitors user join and leave events with detailed information
- Message Logs: Tracks message edits and deletions
- Configuration Management: Flexible per-guild configuration system
- Command System: Framework for Discord slash commands

### Log Characteristics

#### User Joins

- Shows how long ago the account was created on Discord
- User avatar
- Mention info and ID

#### User Leaves

- Time in the server (limited - no historical data by default)
- User avatar
- Mention info and ID

#### Edited Messages

- Content before and after the edit
- Channel where it was edited
- Message author
- Edit timestamp
- Separate channel for message logs

#### Deleted Messages

- Content of the original message
- Channel where it was deleted
- Message author
- Indication of who deleted it (limited by the Discord API)
- Separate channel for message logs

## Architecture

### Main Components

```
discordcore/
├── internal/
│   ├── discord/
│   │   ├── commands/         # Slash command system
│   │   ├── logging/          # Logging and monitoring services
│   │   │   ├── monitoring.go      # Main monitoring service
│   │   │   ├── member_events.go   # Join/leave events
│   │   │   ├── message_events.go  # Message events
│   │   │   ├── notifications.go   # Embeds/notifications system
│   │   │   └── automod.go         # Automod logs
│   │   └── session/          # Discord session management
│   ├── files/                # File management and cache
│   └── util/                 # General utilities
└── cmd/discordcore/          # Implementation example
```

## Installation

```bash
go get github.com/alice-bnuy/discordcore/v2
```

## Basic Usage

### Simple Implementation

```go
package main

import (
	"github.com/alice-bnuy/discordcore/v2/internal/discord/commands"
	"github.com/alice-bnuy/discordcore/v2/internal/discord/logging"
	"github.com/alice-bnuy/discordcore/v2/internal/discord/session"
	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/alice-bnuy/discordcore/v2/internal/storage"
	"github.com/alice-bnuy/discordcore/v2/internal/util"
)

func main() {
	// Configure token
	token := os.Getenv("DISCORD_BOT_TOKEN")

	// Initialize components
	configManager := files.NewConfigManager()
	discordSession, err := session.NewDiscordSession(token)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize SQLite store for persistence
	store := storage.NewStore(util.GetMessageDBPath())
	if err := store.Init(); err != nil {
		log.Fatal(err)
	}

	// Initialize monitoring services
	monitorService, err := logging.NewMonitoringService(discordSession, configManager, store)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize automod
	automodService := logging.NewAutomodService(discordSession, configManager)

	// Initialize commands
	commandHandler := commands.NewCommandHandler(discordSession, configManager)

	// Start everything
	monitorService.Start()
	automodService.Start()
	commandHandler.SetupCommands()

	// Logs are sent to separate channels:
	// - user_entry_leave_channel_id: joins/leaves (moderation/logs)
	// - welcome_backlog_channel_id: public welcome/goodbye channel used for backfill (optional)
	// - user_log_channel_id: avatar changes (and fallback if entry/leave channel isn't set)
	// - message_log_channel_id: message edits/deletions
	// - automod_log_channel_id: moderation actions

	defer func() {
		monitorService.Stop()
		automodService.Stop()
	}()

	// Wait for interrupt
	util.WaitForInterrupt()
}
```

### Per-Guild Configuration

```json
{
	  "guilds": [
	    {
	      "guild_id": "123456789",
	      "command_channel_id": "987654321",
	      "user_log_channel_id": "111111111",
	      "user_entry_leave_channel_id": "444444444",
	      "welcome_backlog_channel_id": "555555555",
	      "message_log_channel_id": "999999999",
	      "automod_log_channel_id": "222222222",
	      "allowed_roles": ["333333333"]
	    }
	  ]
	}
```

## Specific Services

### MonitoringService

Coordinates all monitoring services:

```go
// Initialize
monitorService, err := logging.NewMonitoringService(session, configManager, cache)
if err != nil {
    return err
}

// Start all services
err = monitorService.Start()
if err != nil {
    return err
}

// The MonitoringService automatically manages:
// - UserWatcher (avatar changes)
// - MemberEventService (joins/leaves)
// - MessageEventService (edits/deletions)
```

### Individual Services

#### MemberEventService

```go
// Direct usage (optional - usually managed by MonitoringService)
memberService := logging.NewMemberEventService(session, configManager, notifier)
memberService.Start()
```

#### MessageEventService

```go
// Direct usage (optional)
messageService := logging.NewMessageEventService(session, configManager, notifier)
messageService.Start()

// Message storage is now persisted via SQLite; in-memory cache metrics have been discontinued.
```

## Customization

### Implementing New Handlers

```go
// Extend NotificationSender
func (ns *NotificationSender) SendCustomNotification(channelID string, data interface{}) error {
    embed := &discordgo.MessageEmbed{
        Title:       "Custom Event",
        Color:       0x5865F2,
        Description: "Your custom logic here",
    }
    
    _, err := ns.session.ChannelMessageSendEmbed(channelID, embed)
    return err
}
```

### Adding New Commands

```go
// Implement within the existing command structure
func (ch *CommandHandler) registerCustomCommands() error {
    // Your custom command logic
    return nil
}
```

## Logs and Debugging

### Log Levels

- Info: Main events (joins/leaves, avatar changes)
- Debug: Message cache, internal details
- Error: Notification delivery failures, API errors

### Stats

```go
// Per-guild configuration
config := configManager.GuildConfig("guild_id")
```

## Performance

### Message Cache

- Stores messages for 24 hours to detect edits
- Automatic cleanup every hour
- Thread-safe protection with RWMutex

### Avatar Debounce

- Prevents duplicate notifications
- 5-second temporary cache
- Automatic cleanup of old entries

### Periodic Checks

- Avatar checks every 30 minutes
- Automatic cache initialization for new servers

## Required Permissions

The bot needs the following permissions:

- `View Channels`
- `Send Messages`
- `Embed Links`
- `Read Message History`
- `Use Slash Commands`

### Channel Configuration

The library supports separate channels for different types of logs:

- `user_entry_leave_channel_id`: User joins/leaves (moderation/logs)
- `welcome_backlog_channel_id`: Public welcome/goodbye channel used for backlog/backfill (optional)
- `user_log_channel_id`: Avatar changes (and fallback channel)
- `message_log_channel_id`: Message edits and deletions
- `automod_log_channel_id`: Actions from the automatic moderation system

#### Environment flags

- `ALICE_DISABLE_ENTRY_EXIT_LOGS` — When set to a truthy value, disables the member join/leave pipeline (no entry/exit
  embeds or metrics).
- `ALICE_DISABLE_USER_LOGS` — Disables avatar/role monitoring and notifications.
- `ALICE_DISABLE_MESSAGE_LOGS` — Disables message edit/delete logging.
- `ALICE_DISABLE_REACTION_LOGS` — Disables reaction metrics.

#### Entry/Exit backfill (Option A)

Enable an automatic, one-shot backfill right after services start to reconstruct join/leave data from a welcome/backlog channel:

- `ALICE_BACKFILL_ENTRY_EXIT_ENABLED` — truthy to enable the job at startup.
- `ALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID` — the channel to scan (e.g., `1413465672708657216`).
- `ALICE_BACKFILL_ENTRY_EXIT_START_DAY` — UTC day to scan in `YYYY-MM-DD` format; defaults to “today” if omitted.

Behavior:

- Runs once after initialization via the internal task system.
- Scans the configured channel for the specified day, paging newest → oldest (100 messages per page) until leaving the day.
- Parses:
    - This bot’s own embeds with titles “Member joined” and “Member left”.
    - Mimu-like text messages containing a user mention and the keywords “welcome” / “goodbye”.
- Updates the database by:
    - Upserting member join timestamps using the embed/message timestamp.
    - Incrementing `daily_member_joins` and `daily_member_leaves` counters for metrics.

If `ALICE_BACKFILL_ENTRY_EXIT_ENABLED=true` and `ALICE_BACKFILL_ENTRY_EXIT_CHANNEL_ID` is empty, the backfill will be
auto-dispatched for each configured guild, preferring `welcome_backlog_channel_id` and falling back to
`user_entry_leave_channel_id`.

This allows better organization of logs and configuring permissions specific to each event type.

## Known Limitations

1. Time in Server: Without historical data, it is not possible to precisely calculate how long older users were in the
   server
2. Who Deleted: The Discord API does not directly provide information about who deleted a message
3. Message Cache: Messages sent before the bot starts are not tracked for edits

## Roadmap

### Future Improvements

- [ ] Integration with audit logs to detect moderators
- [ ] Persist join data for precise time-in-server calculation
- [ ] Webhook system for external notifications
- [ ] Web dashboard for configuration
- [ ] Advanced metrics and analytics

## Dependencies

```go
require (
    github.com/alice-bnuy/errutil v1.1.0
    github.com/alice-bnuy/logutil v1.0.0
    github.com/bwmarrin/discordgo v0.29.0
    github.com/joho/godotenv v1.5.1
)
```

## Embed Examples

### User Join

```
Member joined
@user (123456789)
Account created: 2 years, 5 months ago
```

### User Leave

```
Member left
@user (123456789)  
Time in server: Unknown
```

### Message Edited

```
Message edited
@user edited a message in #general

Before: Hello world
After: Hello world!!!
```

### Message Deleted

```
Message deleted
Message by @user deleted in #general

Content: Message that was deleted
Deleted by: User
```

## Contributing

1. Fork the project
2. Create a branch for your feature
3. Commit your changes
4. Open a Pull Request

## License

This project is an internal library. Refer to the appropriate terms of use.