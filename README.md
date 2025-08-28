# DiscordCore

A modular Discord bot core library for Go that provides flexible monitoring and configuration management.

## Features

- **Modular Monitoring**: Generic event system that can be extended for various monitoring needs
- **Configuration Management**: Guild-specific configuration with persistence
- **Discord Integration**: Easy integration with Discord events
- **Extensible Architecture**: Build custom services on top of the core

## Modular Monitoring System

The monitoring system is designed to be highly modular, allowing you to build custom monitoring services in separate repositories.

### Core Components

- **`MonitoringService`**: Generic service for handling events
- **`Event`**: Interface for events that can be monitored
- **`EventProcessor`**: Interface for processing events
- **`EventHandler`**: Function type for handling specific event types
- **`DiscordEventAdapter`**: Bridges Discord events to the generic monitoring system

### Basic Usage

```go
// Create a monitoring service
monitoring := discordcore.NewMonitoringService()

// Add custom processors
monitoring.AddProcessor(&MyCustomProcessor{})

// Register event handlers
monitoring.RegisterEventHandler("presence_update", func(event discordcore.Event) {
    // Handle presence updates
})

// Start monitoring
monitoring.Start()
defer monitoring.Stop()
```

### Discord Integration

```go
// Initialize Discord core
core, err := discordcore.NewDiscordCore("YOUR_BOT_TOKEN")
if err != nil {
    log.Fatal(err)
}

session, err := core.NewDiscordSession()
if err != nil {
    log.Fatal(err)
}
defer session.Close()

// Create Discord adapter
adapter := discordcore.NewDiscordEventAdapter(session, core.ConfigManager, monitoring)
monitoring.AddProcessor(adapter)
```

### Building Custom Services

You can build custom monitoring services in separate repositories:

```go
// In your custom repository
type AvatarMonitor struct{}

func (am *AvatarMonitor) ProcessEvent(event discordcore.Event) {
    if event.GetEventType() == "presence_update" {
        data := event.GetData()
        oldAvatar := data["avatar"]
        // Custom avatar monitoring logic
    }
}

func (am *AvatarMonitor) Start() { /* setup */ }
func (am *AvatarMonitor) Stop() { /* cleanup */ }

// Usage
monitoring := discordcore.NewMonitoringService()
monitoring.AddProcessor(&AvatarMonitor{})
```

## Migration from Legacy API

The legacy `CoreMonitoringService` is still available but deprecated. Migrate to the new modular system:

```go
// Old way (deprecated)
coreMonitoring := discordcore.NewCoreMonitoringService(session, configManager)
coreMonitoring.AddMonitor(myMonitor)

// New way (recommended)
monitoring := discordcore.NewMonitoringService()
monitoring.AddProcessor(myProcessor)
adapter := discordcore.NewDiscordEventAdapter(session, configManager, monitoring)
monitoring.AddProcessor(adapter)
```

## Slash Commands

The library provides a modular system for handling Discord slash commands:

### Core Components

- **`SlashCommand`**: Interface for implementing custom slash commands
- **`SlashCommandManager`**: Manages command registration and execution
- **`SlashCommandHandler`**: Function type for simple command handling

### Basic Usage

```go
// Create command manager
manager := discordcore.NewSlashCommandManager(session)

// Implement custom command
type PingCommand struct{}

func (pc *PingCommand) GetName() string { return "ping" }
func (pc *PingCommand) GetDescription() string { return "Responds with pong" }
func (pc *PingCommand) Execute(session *discordgo.Session, interaction *discordgo.InteractionCreate) error {
    return session.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
        Type: discordgo.InteractionResponseChannelMessageWithSource,
        Data: &discordgo.InteractionResponseData{Content: "Pong!"},
    })
}

// Register and start
manager.RegisterCommand(&PingCommand{})
manager.Start()
```

### Advanced Features

- **Command Options**: Support for parameters, choices, and subcommands
- **Permission Validation**: Built-in error handling and permission checks
- **Modular Design**: Easy to extend and customize
- **Integration**: Works seamlessly with the monitoring system

## Advanced Usage: Time Tracking

To properly track how long users stay in your server, implement a storage mechanism:

```go
type UserTimeTracker struct {
	joinTimes map[string]time.Time // userID -> joinTime
}

func (utt *UserTimeTracker) ProcessEvent(event discordcore.Event) {
	switch event.GetEventType() {
	case "guild_member_add":
		userID := event.GetUserID()
		data := event.GetData()
		utt.joinTimes[userID] = data["joined_at"].(time.Time)

	case "guild_member_remove":
		userID := event.GetUserID()
		if joinTime, exists := utt.joinTimes[userID]; exists {
			timeInGuild := time.Since(joinTime)
			// Store or process the timeInGuild duration
			delete(utt.joinTimes, userID)
		}
	}
}
```

This pattern allows you to:
- Calculate account age from Discord snowflake IDs
- Track time spent in server
- Monitor user retention patterns
- Generate detailed analytics

## Configuration

The library uses a configuration system that persists guild-specific settings. See `types.go` for configuration structures.

## Dependencies

- `github.com/bwmarrin/discordgo`: Discord API wrapper
- `github.com/alice-bnuy/logutil`: Logging utilities
- `github.com/alice-bnuy/errutil`: Error handling utilities
