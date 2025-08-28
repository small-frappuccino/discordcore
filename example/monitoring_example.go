package example

// Example usage of the modular monitoring service:
//
//	import "github.com/alice-bnuy/discordcore/v2"
//
//	// Create a monitoring service
//	monitoring := discordcore.NewMonitoringService()
//
//	// Create a custom event processor
//	type CustomProcessor struct{}
//	func (cp *CustomProcessor) ProcessEvent(event discordcore.Event) {
//		fmt.Printf("Processing event: %s\n", event.GetEventType())
//	}
//	func (cp *CustomProcessor) Start() { /* initialization */ }
//	func (cp *CustomProcessor) Stop() { /* cleanup */ }
//
//	// Add the processor
//	monitoring.AddProcessor(&CustomProcessor{})
//
//	// Register a simple event handler
//	monitoring.RegisterEventHandler("presence_update", func(event discordcore.Event) {
//		data := event.GetData()
//		fmt.Printf("User %s is now %s\n", data["username"], data["status"])
//	})
//
//	// For Discord integration:
//	core, _ := discordcore.NewDiscordCore("YOUR_BOT_TOKEN")
//	session, _ := core.NewDiscordSession()
//	defer session.Close()
//
//	// Create Discord adapter
//	adapter := discordcore.NewDiscordEventAdapter(session, core.ConfigManager, monitoring)
//	monitoring.AddProcessor(adapter)
//
//	// Start monitoring
//	monitoring.Start()
//	defer monitoring.Stop()
//
// This allows you to build custom monitoring services on top of discordcore
// without being tightly coupled to Discord-specific implementations.

// Example of a comprehensive user monitoring system:
//
//	type UserMonitoringProcessor struct {
//		userJoinTimes map[string]time.Time // userID -> joinTime
//	}
//
//	func (ump *UserMonitoringProcessor) ProcessEvent(event discordcore.Event) {
//		switch event.GetEventType() {
//		case "guild_member_add":
//			data := event.GetData()
//			userID := event.GetUserID()
//			joinedAt := data["joined_at"].(time.Time)
//			accountAge := data["account_age_days"].(float64)
//
//			ump.userJoinTimes[userID] = joinedAt
//
//			fmt.Printf("User %s joined! Account age: %.1f days\n",
//				data["username"], accountAge)
//
//		case "guild_member_remove":
//			data := event.GetData()
//			userID := event.GetUserID()
//			timeInGuild := data["time_in_guild_days"].(float64)
//
//			delete(ump.userJoinTimes, userID)
//
//			fmt.Printf("User %s left after %.1f days in the server\n",
//				data["username"], timeInGuild)
//
//		case "guild_member_update":
//			data := event.GetData()
//			timeInGuild := data["time_in_guild_days"].(float64)
//
//			fmt.Printf("User %s updated profile after %.1f days in server\n",
//				data["username"], timeInGuild)
//		}
//	}
//
//	func (ump *UserMonitoringProcessor) Start() {
//		ump.userJoinTimes = make(map[string]time.Time)
//	}
//
//	func (ump *UserMonitoringProcessor) Stop() {
//		// cleanup
//	}
//
// Usage:
//	monitoring.AddProcessor(&UserMonitoringProcessor{})
