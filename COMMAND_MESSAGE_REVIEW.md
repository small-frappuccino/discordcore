# Discord Command Message Review

Generated: 2026-05-04T16:08:09-03:00

Scope:
- Non-test Go functions under `pkg/discord/commands/` that directly emit user-facing command responses or construct nearby message text.
- Included when a function contains response calls such as `Success`, `Info`, `Error`, `EditResponse`, `FollowUp`, `InteractionRespond`, `NewCommandError`, or when the function name clearly looks like a message/embed builder.
- This is a review inventory, not a policy decision. Some functions build helper fragments instead of final strings.

Coverage: 141 functions across 22 files.

## Review Notes
- Use this file to review copy, tone, and whether the response should likely be public or ephemeral.
- For dynamic messages, review the full function body below rather than only the first string literal.

## pkg/discord/commands/admin/service_commands.go

### `(*MetricsCommand).Handle`

- Location: `pkg/discord/commands/admin/service_commands.go:131`
- Signals: `response:Info`

```go
func (cmd *MetricsCommand) Handle(ctx *core.Context) error {
	summary := cmd.formatMetrics(ctx)
	if strings.TrimSpace(summary) == "" {
		summary = "No metrics available"
	}

	builder := core.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("📊 Metrics").
		WithColor(theme.Info()).
		WithTimestamp()

	return builder.Info(ctx.Interaction, summary)
}
```

### `(*MetricsWatchCommand).Handle`

- Location: `pkg/discord/commands/admin/service_commands.go:254`
- Signals: `response:Ephemeral`, `response:Info`, `response:Success`

```go
func (cmd *MetricsWatchCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	intervalSec := extractor.Int("interval_seconds")
	if intervalSec <= 0 {
		intervalSec = 30
	}
	durationSec := extractor.Int("duration_seconds")
	if durationSec <= 0 {
		durationSec = 300
	}

	// Acknowledge start
	if err := core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, fmt.Sprintf("Starting metrics watch: interval=%ds, duration=%ds", intervalSec, durationSec)); err != nil {
		return err
	}

	// Prepare first payload
	channelID := ctx.Interaction.ChannelID
	mc := &MetricsCommand{adminCommands: cmd.adminCommands}
	format := func() string { return mc.formatMetrics(ctx) }
	send := func() (*discordgo.Message, error) {
		embed := &discordgo.MessageEmbed{
			Title:       "📊 Metrics (live)",
			Description: format(),
			Color:       theme.Info(),
			Timestamp:   time.Now().Format(time.RFC3339),
		}
		return ctx.Session.ChannelMessageSendEmbed(channelID, embed)
	}

	// Send initial message
	msg, err := send()
	if err != nil || msg == nil {
		return nil
	}

	// Periodically update
	go func(chID, msgID string, interval, total time.Duration) {
		if interval < 10*time.Second {
			interval = 10 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		timeout := time.After(total)
		for {
			select {
			case <-ticker.C:
				embed := &discordgo.MessageEmbed{
					Title:       "📊 Metrics (live)",
					Description: format(),
					Color:       theme.Info(),
					Timestamp:   time.Now().Format(time.RFC3339),
				}
				_, _ = ctx.Session.ChannelMessageEditEmbed(chID, msgID, embed)
			case <-timeout:
				return

			}
		}
	}(channelID, msg.ID, time.Duration(intervalSec)*time.Second, time.Duration(durationSec)*time.Second)

	return nil
}
```

### `(*ServiceStatusCommand).Handle`

- Location: `pkg/discord/commands/admin/service_commands.go:350`
- Signals: `response:Custom`

```go
func (cmd *ServiceStatusCommand) Handle(ctx *core.Context) error {
	serviceName := core.GetStringOption(core.GetSubCommandOptions(ctx.Interaction), "service")
	if serviceName == "" {
		return core.NewCommandError("Service name is required", true)
	}

	info, err := cmd.adminCommands.serviceManager.GetServiceInfo(serviceName)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Service '%s' not found", serviceName), true)
	}

	// Perform health check
	healthCtx, cancel := context.WithTimeoutCause(context.Background(), 10*time.Second, fmt.Errorf("service status health check timeout"))
	defer cancel()

	health := info.Service.HealthCheck(healthCtx)
	stats := info.Service.Stats()

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("🔧 Service Status: %s", serviceName),
		Color: cmd.adminCommands.getStatusColor(info.State, health.Healthy),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "State",
				Value:  string(info.State),
				Inline: true,
			},
			{
				Name:   "Type",
				Value:  string(info.Service.Type()),
				Inline: true,
			},
			{
				Name:   "Priority",
				Value:  fmt.Sprintf("%d", info.Service.Priority()),
				Inline: true,
			},
			{
				Name:   "Health",
				Value:  cmd.adminCommands.getHealthString(health.Healthy),
				Inline: true,
			},
			{
				Name:   "Uptime",
				Value:  cmd.adminCommands.formatDuration(stats.Uptime),
				Inline: true,
			},
			{
				Name:   "Restarts",
				Value:  fmt.Sprintf("%d", stats.RestartCount),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Add dependencies if any
	deps := info.Service.Dependencies()
	if len(deps) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Dependencies",
			Value:  strings.Join(deps, ", "),
			Inline: false,
		})
	}

	// Add health message if unhealthy
	if !health.Healthy {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Health Issue",
			Value:  health.Message,
			Inline: false,
		})
	}

	// Add custom metrics if available
	if len(stats.CustomMetrics) > 0 {
		var metrics []string
		for k, v := range stats.CustomMetrics {
			metrics = append(metrics, fmt.Sprintf("%s: %v", k, v))
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Metrics",
			Value:  strings.Join(metrics, "\n"),
			Inline: false,
		})
	}

	return core.NewResponseManager(ctx.Session).Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}
```

### `(*ServiceListCommand).Handle`

- Location: `pkg/discord/commands/admin/service_commands.go:466`
- Signals: `response:Custom`

```go
func (cmd *ServiceListCommand) Handle(ctx *core.Context) error {
	services := cmd.adminCommands.serviceManager.GetAllServices()

	embed := &discordgo.MessageEmbed{
		Title:       "🔧 Registered Services",
		Color:       theme.ServiceList(),
		Description: fmt.Sprintf("Total services: %d", len(services)),
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	// Build sorted service names for deterministic output
	names := slices.Collect(maps.Keys(services))
	slices.Sort(names)

	// Group by type in the order discovered from sorted service names
	servicesByType := make(map[service.ServiceType][]string)
	typeOrder := make([]service.ServiceType, 0)
	seenType := make(map[service.ServiceType]bool)
	for _, name := range names {
		info := services[name]
		sType := info.Service.Type()
		status := cmd.adminCommands.getServiceStatusIcon(info.State)
		servicesByType[sType] = append(servicesByType[sType], fmt.Sprintf("%s %s", status, name))
		if !seenType[sType] {
			seenType[sType] = true
			typeOrder = append(typeOrder, sType)
		}
	}

	// Emit fields in deterministic type order and sort within each group
	for _, sType := range typeOrder {
		list := servicesByType[sType]
		// Sort for deterministic output
		slices.Sort(list)
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   string(sType),
			Value:  strings.Join(list, "\n"),
			Inline: true,
		})
	}

	return core.NewResponseManager(ctx.Session).Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}
```

### `(*ServiceRestartCommand).Handle`

- Location: `pkg/discord/commands/admin/service_commands.go:542`
- Signals: `response:EditResponse`, `response:Error`, `response:Info`

```go
func (cmd *ServiceRestartCommand) Handle(ctx *core.Context) error {
	serviceName := core.GetStringOption(core.GetSubCommandOptions(ctx.Interaction), "service")
	if serviceName == "" {
		return core.NewCommandError("Service name is required", true)
	}

	// Check if service exists
	_, err := cmd.adminCommands.serviceManager.GetServiceInfo(serviceName)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Service '%s' not found", serviceName), true)
	}

	// Send initial response
	rm := core.NewResponseManager(ctx.Session)
	if err := rm.Info(ctx.Interaction, fmt.Sprintf("🔄 Restarting service: %s", serviceName)); err != nil {
		return err
	}

	// Restart service in background
	go func() {
		if err := cmd.adminCommands.serviceManager.RestartService(serviceName); err != nil {
			ctx.Logger.Error().Errorf("Failed to restart service: %v", err)
			// Try to follow up with error message
			rm.EditResponse(ctx.Interaction, fmt.Sprintf("❌ Failed to restart service '%s': %v", serviceName, err))
		} else {
			// Follow up with success message
			rm.EditResponse(ctx.Interaction, fmt.Sprintf("✅ Service '%s' restarted successfully", serviceName))
		}
	}()

	return nil
}
```

### `(*HealthCheckCommand).Handle`

- Location: `pkg/discord/commands/admin/service_commands.go:600`
- Signals: `response:Custom`

```go
func (cmd *HealthCheckCommand) Handle(ctx *core.Context) error {
	services := cmd.adminCommands.serviceManager.GetAllServices()

	healthyCount := 0
	unhealthyServices := []string{}
	totalServices := len(services)

	// Check health of all services
	healthCtx, cancel := context.WithTimeoutCause(context.Background(), 30*time.Second, fmt.Errorf("admin health check timeout"))
	defer cancel()

	for name, info := range services {
		if info.State == service.StateRunning {
			health := info.Service.HealthCheck(healthCtx)
			if health.Healthy {
				healthyCount++
			} else {
				unhealthyServices = append(unhealthyServices, fmt.Sprintf("%s: %s", name, health.Message))
			}
		}
	}

	// Determine overall health
	overallHealthy := len(unhealthyServices) == 0
	color := 0x00FF00 // Green
	if !overallHealthy {
		color = 0xFF0000 // Red
	}

	embed := &discordgo.MessageEmbed{
		Title: "🏥 System Health Check",
		Color: color,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Overall Status",
				Value:  cmd.adminCommands.getOverallHealthString(overallHealthy),
				Inline: true,
			},
			{
				Name:   "Services",
				Value:  fmt.Sprintf("%d/%d healthy", healthyCount, totalServices),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if len(unhealthyServices) > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "❌ Unhealthy Services",
			Value:  strings.Join(unhealthyServices, "\n"),
			Inline: false,
		})
	}

	return core.NewResponseManager(ctx.Session).Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}
```

### `(*SystemInfoCommand).Handle`

- Location: `pkg/discord/commands/admin/service_commands.go:683`
- Signals: `response:Custom`

```go
func (cmd *SystemInfoCommand) Handle(ctx *core.Context) error {
	services := cmd.adminCommands.serviceManager.GetAllServices()
	runningServices := cmd.adminCommands.serviceManager.GetRunningServices()

	botName := util.EffectiveBotName()
	if util.AppVersion != "" {
		botName = fmt.Sprintf("%s %s", botName, util.AppVersion)
	}

	embed := &discordgo.MessageEmbed{
		Title: "ℹ️ System Information",
		Color: theme.SystemInfo(),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Bot",
				Value:  botName,
				Inline: true,
			},
			{
				Name:   "Core",
				Value:  fmt.Sprintf("discordcore %s", util.DiscordCoreVersion),
				Inline: true,
			},
			{
				Name:   "Total Services",
				Value:  fmt.Sprintf("%d", len(services)),
				Inline: true,
			},
			{
				Name:   "Running Services",
				Value:  fmt.Sprintf("%d", len(runningServices)),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return core.NewResponseManager(ctx.Session).Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}
```

## pkg/discord/commands/config/config_commands.go

### `(*pingCommand).Handle`

- Location: `pkg/discord/commands/config/config_commands.go:96`
- Signals: `response:Success`

```go
func (c *pingCommand) Handle(ctx *core.Context) error {
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "🏓 Pong!")
}
```

### `(*echoCommand).Handle`

- Location: `pkg/discord/commands/config/config_commands.go:124`
- Signals: `response:Ephemeral`, `response:Info`

```go
func (c *echoCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)

	message, err := extractor.StringRequired("message")
	if err != nil {
		return err
	}
	ephemeral := extractor.Bool("ephemeral")

	builder := core.NewResponseBuilder(ctx.Session)
	if ephemeral {
		builder = builder.Ephemeral()
	}
	return builder.Info(ctx.Interaction, fmt.Sprintf("Echo: %s", message))
}
```

### `(*ConfigSetSubCommand).Handle`

- Location: `pkg/discord/commands/config/config_commands.go:186`
- Signals: `response:Error`, `response:Success`

```go
func (c *ConfigSetSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	key, err := extractor.StringRequired("key")
	if err != nil {
		return err
	}
	value, err := extractor.StringRequired("value")
	if err != nil {
		return err
	}

	// Safely mutate guild config
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		switch key {
		case "channels.commands":
			guildConfig.Channels.Commands = value
		case "channels.avatar_logging":
			guildConfig.Channels.AvatarLogging = value
		case "channels.role_update":
			guildConfig.Channels.RoleUpdate = value
		case "channels.member_join":
			guildConfig.Channels.MemberJoin = value
		case "channels.member_leave":
			guildConfig.Channels.MemberLeave = value
		case "channels.message_edit":
			guildConfig.Channels.MessageEdit = value
		case "channels.message_delete":
			guildConfig.Channels.MessageDelete = value
		case "channels.automod_action":
			guildConfig.Channels.AutomodAction = value
		case "channels.moderation_case":
			guildConfig.Channels.ModerationCase = value
		case "channels.entry_backfill":
			guildConfig.Channels.EntryBackfill = value
		case "channels.verification_cleanup":
			guildConfig.Channels.VerificationCleanup = value
		default:
			return core.NewValidationError("key", "Invalid configuration key")
		}
		return nil
	}); err != nil {
		return err
	}

	// Persist changes
	persister := core.NewConfigPersister(c.configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save config: %v", err)
		return core.NewCommandError("Failed to save configuration", false)
	}

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Configuration `%s` set to `%s`", key, value))
}
```

### `(*ConfigGetSubCommand).Handle`

- Location: `pkg/discord/commands/config/config_commands.go:257`
- Signals: `response:Info`

```go
func (c *ConfigGetSubCommand) Handle(ctx *core.Context) error {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("**Server Configuration:**\n")
	commandsEnabled := false
	if snapshot := ctx.Config.Config(); snapshot != nil {
		commandsEnabled = snapshot.ResolveFeatures(ctx.GuildID).Services.Commands
	}
	b.WriteString(fmt.Sprintf("Commands Enabled: %t\n", commandsEnabled))
	b.WriteString(fmt.Sprintf("Command Channel: %s\n", emptyToDash(ctx.GuildConfig.Channels.Commands)))
	b.WriteString(fmt.Sprintf("Avatar Logging: %s\n", emptyToDash(ctx.GuildConfig.Channels.AvatarLogging)))
	b.WriteString(fmt.Sprintf("Role Update: %s\n", emptyToDash(ctx.GuildConfig.Channels.RoleUpdate)))
	b.WriteString(fmt.Sprintf("Member Join: %s\n", emptyToDash(ctx.GuildConfig.Channels.MemberJoin)))
	b.WriteString(fmt.Sprintf("Member Leave: %s\n", emptyToDash(ctx.GuildConfig.Channels.MemberLeave)))
	b.WriteString(fmt.Sprintf("Message Edit: %s\n", emptyToDash(ctx.GuildConfig.Channels.MessageEdit)))
	b.WriteString(fmt.Sprintf("Message Delete: %s\n", emptyToDash(ctx.GuildConfig.Channels.MessageDelete)))
	b.WriteString(fmt.Sprintf("Automod Action: %s\n", emptyToDash(ctx.GuildConfig.Channels.AutomodAction)))
	b.WriteString(fmt.Sprintf("Moderation Case: %s\n", emptyToDash(ctx.GuildConfig.Channels.ModerationCase)))
	b.WriteString(fmt.Sprintf("Entry Backfill: %s\n", emptyToDash(ctx.GuildConfig.Channels.EntryBackfill)))
	b.WriteString(fmt.Sprintf("Verification Cleanup: %s\n", emptyToDash(ctx.GuildConfig.Channels.VerificationCleanup)))
	qotdSettings := files.DashboardQOTDConfig(ctx.GuildConfig.QOTD)
	qotdDeck, _ := qotdSettings.ActiveDeck()
	qotdEnabled := false
	qotdChannel := ""
	if qotdDeck.ID != "" {
		qotdEnabled = qotdDeck.Enabled
		qotdChannel = qotdDeck.ChannelID
	}
	b.WriteString(fmt.Sprintf("QOTD Enabled: %t\n", qotdEnabled))
	b.WriteString(fmt.Sprintf("QOTD Channel: %s\n", emptyToDash(qotdChannel)))
	b.WriteString(fmt.Sprintf("QOTD Schedule (UTC): %s\n", formatQOTDSchedule(qotdSettings.Schedule)))
	b.WriteString(fmt.Sprintf("Allowed Roles: %d configured\n", len(ctx.GuildConfig.Roles.Allowed)))

	builder := core.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Server Configuration").
		WithColor(theme.Info())

	return builder.Info(ctx.Interaction, b.String())
}
```

### `(*ConfigListSubCommand).Handle`

- Location: `pkg/discord/commands/config/config_commands.go:319`
- Signals: `response:Info`

```go
func (c *ConfigListSubCommand) Handle(ctx *core.Context) error {
	options := []string{
		"**Available Configuration Options:**",
		"`/config smoke_test` - Show bootstrap readiness for general config and QOTD",
		"`/config commands_enabled <enabled>` - Enable or disable slash command handling for this guild",
		"`/config command_channel <channel>` - Set the channel used for command routing or references",
		"`/config allowed_role_add <role>` - Allow one role to use admin-level slash commands",
		"`/config allowed_role_remove <role>` - Remove one allowed admin role",
		"`/config allowed_role_list` - Show the current allowed admin roles",
		"",
		"`channels.commands` - Channel for bot commands",
		"`channels.avatar_logging` - Channel for avatar change logs",
		"`channels.role_update` - Channel for role update logs",
		"`channels.member_join` - Channel for member join logs",
		"`channels.member_leave` - Channel for member leave logs",
		"`channels.message_edit` - Channel for message edit logs",
		"`channels.message_delete` - Channel for message delete logs",
		"`channels.automod_action` - Channel for automod action logs",
		"`channels.moderation_case` - Dedicated channel for moderation case logs",
		"`channels.entry_backfill` - Channel used by entry/leave backfill",
		"`channels.verification_cleanup` - Channel used for verification cleanup routines",
		"",
		"Use `/config set <key> <value>` to modify these settings.",
		"",
		"`/config qotd_schedule <hour> <minute>` - Set the QOTD publish schedule in UTC",
		"`/config qotd_enabled <enabled>` - Enable or disable QOTD publishing for the active deck",
		"`/config qotd_channel <channel>` - Set the QOTD delivery channel for the active deck",
		"",
		"`/config webhook_embed_create` - Add webhook embed patch entry",
		"`/config webhook_embed_read` - Show one webhook embed patch entry",
		"`/config webhook_embed_update` - Update existing webhook embed patch entry",
		"`/config webhook_embed_delete` - Delete webhook embed patch entry",
		"`/config webhook_embed_list` - List webhook embed patch entries",
	}

	builder := core.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Configuration Options")

	return builder.Info(ctx.Interaction, strings.Join(options, "\n"))
}
```

## pkg/discord/commands/config/qotd_commands.go

### `(*QOTDEnabledSubCommand).Handle`

- Location: `pkg/discord/commands/config/qotd_commands.go:49`
- Signals: `response:Success`

```go
func (c *QOTDEnabledSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	enabled := extractor.Bool(qotdEnabledOptionName)

	updatedDeck, err := updateActiveQOTDDeck(ctx, c.configManager, func(deck *files.QOTDDeckConfig) error {
		if enabled && strings.TrimSpace(deck.ChannelID) == "" {
			return core.NewCommandError("Set a QOTD channel before enabling publishing", false)
		}
		deck.Enabled = enabled
		return nil
	})
	if err != nil {
		return err
	}

	state := "disabled"
	if updatedDeck.Enabled {
		state = "enabled"
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD is now %s for deck `%s`.", state, updatedDeck.Name))
}
```

### `(*QOTDChannelSubCommand).Handle`

- Location: `pkg/discord/commands/config/qotd_commands.go:100`
- Signals: `response:Success`

```go
func (c *QOTDChannelSubCommand) Handle(ctx *core.Context) error {
	channelID := channelOptionID(ctx.Session, core.GetSubCommandOptions(ctx.Interaction), qotdChannelOptionName)
	if channelID == "" {
		return core.NewCommandError("Channel is required", false)
	}

	updatedDeck, err := updateActiveQOTDDeck(ctx, c.configManager, func(deck *files.QOTDDeckConfig) error {
		deck.ChannelID = channelID
		return nil
	})
	if err != nil {
		return err
	}

	state := "disabled"
	if updatedDeck.Enabled {
		state = "enabled"
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD channel set to <#%s> for deck `%s`. Publishing remains %s.", channelID, updatedDeck.Name, state))
}
```

### `(*QOTDScheduleSubCommand).Handle`

- Location: `pkg/discord/commands/config/qotd_commands.go:153`
- Signals: `response:Success`

```go
func (c *QOTDScheduleSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	hourUTC := int(extractor.Int(qotdScheduleHourOptionName))
	minuteUTC := int(extractor.Int(qotdScheduleMinuteOptionName))

	updatedConfig, err := updateQOTDConfig(ctx, c.configManager, func(cfg *files.QOTDConfig) error {
		cfg.Schedule = files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		}
		return nil
	})
	if err != nil {
		return err
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD publish schedule set to %s UTC.", formatQOTDSchedule(updatedConfig.Schedule)))
}
```

### `updateQOTDConfig`

- Location: `pkg/discord/commands/config/qotd_commands.go:173`
- Signals: `helper-call:translateQOTDConfigError`, `response:Error`

```go
func updateQOTDConfig(
	ctx *core.Context,
	configManager *files.ConfigManager,
	mutate func(*files.QOTDConfig) error,
) (files.QOTDConfig, error) {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return files.QOTDConfig{}, err
	}

	var updatedConfig files.QOTDConfig
	err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		next := files.DashboardQOTDConfig(guildConfig.QOTD)
		if err := mutate(&next); err != nil {
			return err
		}

		normalized, err := files.NormalizeQOTDConfig(next)
		if err != nil {
			return translateQOTDConfigError(err)
		}
		guildConfig.QOTD = normalized
		updatedConfig = files.DashboardQOTDConfig(normalized)
		return nil
	})
	if err != nil {
		return files.QOTDConfig{}, err
	}

	persister := core.NewConfigPersister(configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save QOTD config: %v", err)
		return files.QOTDConfig{}, core.NewCommandError("Failed to save configuration", false)
	}

	return updatedConfig, nil
}
```

### `translateQOTDConfigError`

- Location: `pkg/discord/commands/config/qotd_commands.go:260`
- Signals: `helper-func`, `response:Error`

```go
func translateQOTDConfigError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, files.ErrInvalidQOTDInput) {
		message := strings.TrimSpace(strings.TrimPrefix(err.Error(), files.ErrInvalidQOTDInput.Error()+":"))
		if message == "" {
			message = "Invalid QOTD configuration"
		}
		if message == "schedule.hour_utc and schedule.minute_utc are required when enabled" {
			message = "Set the QOTD publish hour and minute before enabling publishing"
		}
		return core.NewCommandError(message, false)
	}
	return err
}
```

## pkg/discord/commands/config/service_commands.go

### `(*CommandsEnabledSubCommand).Handle`

- Location: `pkg/discord/commands/config/service_commands.go:46`
- Signals: `response:Success`

```go
func (c *CommandsEnabledSubCommand) Handle(ctx *core.Context) error {
	enabled := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction)).Bool(commandEnabledOptionName)
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		guildConfig.Features.Services.Commands = boolPtr(enabled)
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Slash commands are now %s for this guild.", state))
}
```

### `(*CommandChannelSubCommand).Handle`

- Location: `pkg/discord/commands/config/service_commands.go:87`
- Signals: `response:Success`

```go
func (c *CommandChannelSubCommand) Handle(ctx *core.Context) error {
	channelID := channelOptionID(ctx.Session, core.GetSubCommandOptions(ctx.Interaction), commandChannelOptionName)
	if channelID == "" {
		return core.NewCommandError("Channel is required", false)
	}
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		guildConfig.Channels.Commands = channelID
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Command channel set to <#%s>.", channelID))
}
```

### `(*AllowedRoleAddSubCommand).Handle`

- Location: `pkg/discord/commands/config/service_commands.go:126`
- Signals: `response:Success`

```go
func (c *AllowedRoleAddSubCommand) Handle(ctx *core.Context) error {
	roleID := roleOptionID(core.GetSubCommandOptions(ctx.Interaction), allowedRoleOptionName)
	if roleID == "" {
		return core.NewCommandError("Role is required", false)
	}
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		if slices.Contains(guildConfig.Roles.Allowed, roleID) {
			return nil
		}
		guildConfig.Roles.Allowed = append(guildConfig.Roles.Allowed, roleID)
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Allowed role added: <@&%s>.", roleID))
}
```

### `(*AllowedRoleRemoveSubCommand).Handle`

- Location: `pkg/discord/commands/config/service_commands.go:168`
- Signals: `response:Success`

```go
func (c *AllowedRoleRemoveSubCommand) Handle(ctx *core.Context) error {
	roleID := roleOptionID(core.GetSubCommandOptions(ctx.Interaction), allowedRoleOptionName)
	if roleID == "" {
		return core.NewCommandError("Role is required", false)
	}
	if err := core.SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		guildConfig.Roles.Allowed = removeString(guildConfig.Roles.Allowed, roleID)
		return nil
	}); err != nil {
		return err
	}
	if err := persistGuildConfig(ctx, c.configManager); err != nil {
		return err
	}
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Allowed role removed: <@&%s>.", roleID))
}
```

### `(*AllowedRoleListSubCommand).Handle`

- Location: `pkg/discord/commands/config/service_commands.go:200`
- Signals: `response:Info`

```go
func (c *AllowedRoleListSubCommand) Handle(ctx *core.Context) error {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return err
	}
	if len(ctx.GuildConfig.Roles.Allowed) == 0 {
		return core.NewResponseBuilder(ctx.Session).Info(ctx.Interaction, "No allowed admin roles are configured.")
	}
	roles := make([]string, 0, len(ctx.GuildConfig.Roles.Allowed))
	for _, roleID := range ctx.GuildConfig.Roles.Allowed {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		roles = append(roles, fmt.Sprintf("- <@&%s>", roleID))
	}
	if len(roles) == 0 {
		return core.NewResponseBuilder(ctx.Session).Info(ctx.Interaction, "No allowed admin roles are configured.")
	}
	return core.NewResponseBuilder(ctx.Session).Info(ctx.Interaction, "Allowed admin roles:\n"+strings.Join(roles, "\n"))
}
```

### `persistGuildConfig`

- Location: `pkg/discord/commands/config/service_commands.go:221`
- Signals: `response:Error`

```go
func persistGuildConfig(ctx *core.Context, configManager *files.ConfigManager) error {
	persister := core.NewConfigPersister(configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save config: %v", err)
		return core.NewCommandError("Failed to save configuration", false)
	}
	return nil
}
```

## pkg/discord/commands/config/smoke_test_command.go

### `(*SmokeTestSubCommand).Handle`

- Location: `pkg/discord/commands/config/smoke_test_command.go:32`
- Signals: `helper-call:generalSmokeTestLines`, `helper-call:qotdSmokeTestLines`, `response:Info`

```go
func (c *SmokeTestSubCommand) Handle(ctx *core.Context) error {
	if err := core.RequiresGuildConfig(ctx); err != nil {
		return err
	}

	lines := []string{
		"**General / Initial Setup**",
	}
	lines = append(lines, generalSmokeTestLines(ctx)...)
	lines = append(lines, "", "**QOTD**")
	lines = append(lines, qotdSmokeTestLines(ctx)...)

	return core.NewResponseBuilder(ctx.Session).
		Info(ctx.Interaction, strings.Join(lines, "\n"))
}
```

### `generalSmokeTestLines`

- Location: `pkg/discord/commands/config/smoke_test_command.go:48`
- Signals: `helper-func`

```go
func generalSmokeTestLines(ctx *core.Context) []string {
	commandsEnabled := false
	if snapshot := ctx.Config.Config(); snapshot != nil {
		commandsEnabled = snapshot.ResolveFeatures(ctx.GuildID).Services.Commands
	}

	lines := make([]string, 0, 5)
	listRouteAllowed := AllowsDormantGuildBootstrapRoute(core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "config list"})
	blockedRouteAllowed := AllowsDormantGuildBootstrapRoute(core.InteractionRouteKey{Kind: core.InteractionKindSlash, Path: "partner list"})

	if listRouteAllowed {
		if commandsEnabled {
			lines = append(lines, "[PASS] /config list is in the bootstrap allowlist, and the full slash surface is already enabled.")
		} else {
			lines = append(lines, "[PASS] /config list remains available while this guild is still dormant.")
		}
	} else {
		lines = append(lines, "[ACTION] /config list is missing from the dormant bootstrap allowlist.")
	}

	commandChannelID := strings.TrimSpace(ctx.GuildConfig.Channels.Commands)
	if commandChannelID == "" {
		lines = append(lines, "[ACTION] Command channel is not configured. Run /config command_channel <channel>.")
	} else {
		lines = append(lines, fmt.Sprintf("[PASS] Command channel configured: <#%s>.", commandChannelID))
	}

	if commandsEnabled {
		lines = append(lines, "[PASS] Full slash command surface is enabled.")
	} else {
		lines = append(lines, "[ACTION] Full slash command surface is still disabled. Run /config commands_enabled true when bootstrap setup is complete.")
	}

	if commandsEnabled {
		lines = append(lines, "[PASS] Non-bootstrap routes are no longer gated because commands are enabled.")
	} else if !blockedRouteAllowed {
		lines = append(lines, "[PASS] Non-bootstrap routes remain blocked until /config commands_enabled true.")
	} else {
		lines = append(lines, "[ACTION] Non-bootstrap routes are unexpectedly allowed during dormant bootstrap.")
	}

	allowedRoles := len(ctx.GuildConfig.Roles.Allowed)
	if allowedRoles == 0 {
		lines = append(lines, "[INFO] No allowed admin roles are configured. Guild owner / Administrator / Manage Guild can still bootstrap this guild.")
	} else {
		lines = append(lines, fmt.Sprintf("[PASS] Allowed admin roles configured: %d.", allowedRoles))
	}

	return lines
}
```

### `qotdSmokeTestLines`

- Location: `pkg/discord/commands/config/smoke_test_command.go:99`
- Signals: `helper-func`

```go
func qotdSmokeTestLines(ctx *core.Context) []string {
	settings := files.DashboardQOTDConfig(ctx.GuildConfig.QOTD)
	deck, ok := settings.ActiveDeck()
	if !ok {
		return []string{"[ACTION] QOTD configuration is unavailable."}
	}

	lines := []string{fmt.Sprintf("[PASS] Active QOTD deck: %s.", deck.Name)}
	channelConfigured := strings.TrimSpace(deck.ChannelID) != ""
	scheduleConfigured := settings.Schedule.IsComplete()

	if channelConfigured {
		lines = append(lines, fmt.Sprintf("[PASS] QOTD channel configured: <#%s>.", deck.ChannelID))
	} else {
		lines = append(lines, "[ACTION] QOTD channel is not configured. Run /config qotd_channel <channel>.")
	}

	if scheduleConfigured {
		lines = append(lines, fmt.Sprintf("[PASS] QOTD publish schedule configured: %s UTC.", formatQOTDSchedule(settings.Schedule)))
	} else {
		lines = append(lines, fmt.Sprintf("[ACTION] QOTD publish schedule is not complete (%s UTC). Run /config qotd_schedule <hour> <minute>.", formatQOTDSchedule(settings.Schedule)))
	}

	switch {
	case deck.Enabled && channelConfigured && scheduleConfigured:
		lines = append(lines, fmt.Sprintf("[PASS] QOTD publishing is enabled for deck %s.", deck.Name))
	case channelConfigured && scheduleConfigured:
		lines = append(lines, "[ACTION] QOTD is ready to enable. Run /config qotd_enabled true.")
	case !channelConfigured && !scheduleConfigured:
		lines = append(lines, "[ACTION] QOTD is not ready to enable yet. Set the QOTD channel and schedule first.")
	case !channelConfigured:
		lines = append(lines, "[ACTION] QOTD is not ready to enable yet. Set the QOTD channel first.")
	default:
		lines = append(lines, "[ACTION] QOTD is not ready to enable yet. Set the QOTD publish hour and minute first.")
	}

	return lines
}
```

## pkg/discord/commands/config/webhook_embed_update_handlers.go

### `(*ConfigWebhookEmbedCreateSubCommand).Handle`

- Location: `pkg/discord/commands/config/webhook_embed_update_handlers.go:56`
- Signals: `response:Info`, `response:Success`

```go
func (c *ConfigWebhookEmbedCreateSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	messageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}
	webhookURL, err := extractor.StringRequired(optionWebhookURL)
	if err != nil {
		return err
	}
	embedRaw, err := parseEmbedRaw(extractor)
	if err != nil {
		return err
	}
	applyNow := extractor.Bool(optionApplyNow)
	validationWarning := ""

	if !applyNow {
		validationWarning, err = validateWebhookTargetBeforePersist(
			ctx,
			c.configManager,
			scopeGuildID,
			messageID,
			webhookURL,
		)
		if err != nil {
			return err
		}
	}

	err = c.configManager.CreateWebhookEmbedUpdate(scopeGuildID, files.WebhookEmbedUpdateConfig{
		MessageID:  messageID,
		WebhookURL: webhookURL,
		Embed:      embedRaw,
	})
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateAlreadyExists) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "A webhook embed update with this message_id already exists in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to create webhook embed update: %v", err))
	}

	if applyNow {
		saved, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, messageID)
		if err != nil {
			rollbackErr := rollbackCreatedWebhookEmbedUpdate(c.configManager, scopeGuildID, messageID)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Create aborted: apply_now lookup failed and rollback failed (lookup=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Create aborted because apply_now lookup failed; entry was rolled back: %v", err),
			)
		}
		if err := patchWebhookMessageNow(ctx, scopeGuildID, saved); err != nil {
			rollbackErr := rollbackCreatedWebhookEmbedUpdate(c.configManager, scopeGuildID, saved.MessageID)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Create aborted: apply_now failed and rollback failed (apply=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Create aborted because apply_now failed; entry was rolled back: %v", err),
			)
		}
	}

	msg := fmt.Sprintf(
		"Created webhook embed update in `%s` for message_id `%s` (webhook `%s`). apply_now=%t",
		renderScopeLabel(scopeGuildID),
		strings.TrimSpace(messageID),
		maskWebhookURL(webhookURL),
		applyNow,
	)
	if validationWarning != "" {
		msg += "\n" + validationWarning
		return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityDetailedError).Info(ctx.Interaction, msg)
	}
	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityShortConfirmation).Success(ctx.Interaction, msg)
}
```

### `(*ConfigWebhookEmbedReadSubCommand).Handle`

- Location: `pkg/discord/commands/config/webhook_embed_update_handlers.go:177`
- Signals: `response:Info`

```go
func (c *ConfigWebhookEmbedReadSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	messageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}

	entry, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, messageID)
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to read webhook embed update: %v", err))
	}

	content := strings.Join([]string{
		fmt.Sprintf("Scope: `%s`", renderScopeLabel(scopeGuildID)),
		fmt.Sprintf("Message ID: `%s`", strings.TrimSpace(entry.MessageID)),
		fmt.Sprintf("Webhook: `%s`", maskWebhookURL(entry.WebhookURL)),
		"Embed JSON:",
		renderEmbedPreview(entry.Embed),
	}, "\n")

	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityRenderedPayload).Info(ctx.Interaction, content)
}
```

### `(*ConfigWebhookEmbedUpdateSubCommand).Handle`

- Location: `pkg/discord/commands/config/webhook_embed_update_handlers.go:257`
- Signals: `response:Info`, `response:Success`

```go
func (c *ConfigWebhookEmbedUpdateSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	targetMessageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}
	newMessageID := strings.TrimSpace(extractor.String(optionNewMessage))
	if newMessageID == "" {
		newMessageID = targetMessageID
	}
	webhookURL, err := extractor.StringRequired(optionWebhookURL)
	if err != nil {
		return err
	}
	embedRaw, err := parseEmbedRaw(extractor)
	if err != nil {
		return err
	}
	applyNow := extractor.Bool(optionApplyNow)
	validationWarning := ""

	previous, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, targetMessageID)
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to load webhook embed update before update: %v", err))
	}

	if !applyNow {
		validationWarning, err = validateWebhookTargetBeforePersist(
			ctx,
			c.configManager,
			scopeGuildID,
			newMessageID,
			webhookURL,
		)
		if err != nil {
			return err
		}
	}

	err = c.configManager.UpdateWebhookEmbedUpdate(scopeGuildID, targetMessageID, files.WebhookEmbedUpdateConfig{
		MessageID:  newMessageID,
		WebhookURL: webhookURL,
		Embed:      embedRaw,
	})
	if err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		if errors.Is(err, files.ErrWebhookEmbedUpdateAlreadyExists) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "The new message_id is already used by another entry in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to update webhook embed update: %v", err))
	}

	if applyNow {
		saved, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, newMessageID)
		if err != nil {
			rollbackErr := rollbackUpdatedWebhookEmbedUpdate(c.configManager, scopeGuildID, newMessageID, previous)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Update aborted: apply_now lookup failed and rollback failed (lookup=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Update aborted because apply_now lookup failed; previous entry was restored: %v", err),
			)
		}
		if err := patchWebhookMessageNow(ctx, scopeGuildID, saved); err != nil {
			rollbackErr := rollbackUpdatedWebhookEmbedUpdate(c.configManager, scopeGuildID, saved.MessageID, previous)
			if rollbackErr != nil {
				return webhookEmbedCommandError(
					webhookEmbedVisibilityDetailedError,
					fmt.Sprintf("Update aborted: apply_now failed and rollback failed (apply=%v rollback=%v)", err, rollbackErr),
				)
			}
			return webhookEmbedCommandError(
				webhookEmbedVisibilityDetailedError,
				fmt.Sprintf("Update aborted because apply_now failed; previous entry was restored: %v", err),
			)
		}
	}

	msg := fmt.Sprintf(
		"Updated webhook embed entry in `%s`: `%s` -> `%s` (webhook `%s`). apply_now=%t",
		renderScopeLabel(scopeGuildID),
		strings.TrimSpace(targetMessageID),
		strings.TrimSpace(newMessageID),
		maskWebhookURL(webhookURL),
		applyNow,
	)
	if validationWarning != "" {
		msg += "\n" + validationWarning
		return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityDetailedError).Info(ctx.Interaction, msg)
	}
	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityShortConfirmation).Success(ctx.Interaction, msg)
}
```

### `(*ConfigWebhookEmbedDeleteSubCommand).Handle`

- Location: `pkg/discord/commands/config/webhook_embed_update_handlers.go:395`
- Signals: `response:Success`

```go
func (c *ConfigWebhookEmbedDeleteSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	messageID, err := extractor.StringRequired(optionMessageID)
	if err != nil {
		return err
	}
	applyNow := extractor.Bool(optionApplyNow)

	if applyNow {
		current, err := c.configManager.GetWebhookEmbedUpdate(scopeGuildID, messageID)
		if err != nil {
			if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
				return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
			}
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to load webhook embed update before delete: %v", err))
		}

		if err := patchWebhookMessageNow(ctx, scopeGuildID, current); err != nil {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Delete aborted because apply_now failed: %v", err))
		}
	}

	if err := c.configManager.DeleteWebhookEmbedUpdate(scopeGuildID, messageID); err != nil {
		if errors.Is(err, files.ErrWebhookEmbedUpdateNotFound) {
			return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, "No webhook embed update found with that message_id in the selected scope")
		}
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to delete webhook embed update: %v", err))
	}

	msg := fmt.Sprintf(
		"Deleted webhook embed update from `%s` for message_id `%s`. apply_now=%t",
		renderScopeLabel(scopeGuildID),
		strings.TrimSpace(messageID),
		applyNow,
	)
	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityShortConfirmation).Success(ctx.Interaction, msg)
}
```

### `(*ConfigWebhookEmbedListSubCommand).Handle`

- Location: `pkg/discord/commands/config/webhook_embed_update_handlers.go:458`
- Signals: `response:Info`

```go
func (c *ConfigWebhookEmbedListSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	scopeGuildID, err := parseScope(ctx, extractor)
	if err != nil {
		return err
	}
	updates, err := c.configManager.ListWebhookEmbedUpdates(scopeGuildID)
	if err != nil {
		return webhookEmbedCommandError(webhookEmbedVisibilityDetailedError, fmt.Sprintf("Failed to list webhook embed updates: %v", err))
	}
	if len(updates) == 0 {
		return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityList).Info(
			ctx.Interaction,
			fmt.Sprintf("No webhook embed updates configured in `%s`.", renderScopeLabel(scopeGuildID)),
		)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Webhook embed updates in `%s`:\n", renderScopeLabel(scopeGuildID)))

	limit := len(updates)
	if limit > maxListEntries {
		limit = maxListEntries
	}

	for i := 0; i < limit; i++ {
		item := updates[i]
		b.WriteString(fmt.Sprintf(
			"%d. message_id=`%s` webhook=`%s` embed_bytes=%d\n",
			i+1,
			strings.TrimSpace(item.MessageID),
			maskWebhookURL(item.WebhookURL),
			len(bytes.TrimSpace(item.Embed)),
		))
	}
	if len(updates) > limit {
		b.WriteString(fmt.Sprintf("...and %d more entries.\n", len(updates)-limit))
	}
	b.WriteString("Use `/config webhook_embed_read` with `message_id` for full details.")

	return webhookEmbedResponseBuilder(ctx.Session, webhookEmbedVisibilityList).Info(ctx.Interaction, b.String())
}
```

## pkg/discord/commands/config/webhook_embed_update_visibility.go

### `webhookEmbedResponseBuilder`

- Location: `pkg/discord/commands/config/webhook_embed_update_visibility.go:34`
- Signals: `response:Ephemeral`

```go
func webhookEmbedResponseBuilder(session *discordgo.Session, class webhookEmbedVisibilityClass) *core.ResponseBuilder {
	builder := core.NewResponseBuilder(session)
	if webhookEmbedVisibilityIsEphemeral(class) {
		builder = builder.Ephemeral()
	}
	return builder
}
```

## pkg/discord/commands/core/base.go

### `ValidateGuildContext`

- Location: `pkg/discord/commands/core/base.go:109`
- Signals: `NewCommandError`

```go
func ValidateGuildContext(ctx *Context) error {
	if ctx.GuildID == "" {
		return NewCommandError("This command can only be used in a server", true)
	}

	if ctx.GuildConfig == nil {
		return NewCommandError("Server configuration not found", true)
	}

	return nil
}
```

### `ValidateUserContext`

- Location: `pkg/discord/commands/core/base.go:122`
- Signals: `NewCommandError`

```go
func ValidateUserContext(ctx *Context) error {
	if ctx.UserID == "" {
		return NewCommandError("Unable to identify user", true)
	}

	return nil
}
```

### `RequiresGuildConfig`

- Location: `pkg/discord/commands/core/base.go:204`
- Signals: `NewCommandError`

```go
func RequiresGuildConfig(ctx *Context) error {
	if err := ValidateGuildContext(ctx); err != nil {
		return err
	}

	if ctx.GuildConfig == nil {
		return NewCommandError("Server configuration is required for this command", true)
	}

	return nil
}
```

## pkg/discord/commands/core/examples.go

### `(*PingCommand).Handle`

- Location: `pkg/discord/commands/core/examples.go:46`
- Signals: `response:Success`

```go
func (c *PingCommand) Handle(ctx *Context) error {
	return NewResponseBuilder(ctx.Session).Success(ctx.Interaction, "🏓 Pong!")
}
```

### `(*EchoCommand).Handle`

- Location: `pkg/discord/commands/core/examples.go:94`
- Signals: `response:Ephemeral`, `response:Info`

```go
func (c *EchoCommand) Handle(ctx *Context) error {
	// Extract command options
	extractor := NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)

	message, err := extractor.StringRequired("message")
	if err != nil {
		return err
	}

	ephemeral := extractor.Bool("ephemeral")

	// Use ResponseBuilder for a more flexible response
	builder := NewResponseBuilder(ctx.Session)
	if ephemeral {
		builder = builder.Ephemeral()
	}

	return builder.Info(ctx.Interaction, fmt.Sprintf("Echo: %s", message))
}
```

### `(*UserInfoSubCommand).Handle`

- Location: `pkg/discord/commands/core/examples.go:152`
- Signals: `response:Info`

```go
func (c *UserInfoSubCommand) Handle(ctx *Context) error {
	extractor := NewOptionExtractor(GetSubCommandOptions(ctx.Interaction))

	// If no user is specified, use the command author
	var targetUser *discordgo.User
	if extractor.HasOption("user") {
		// Logic to extract the user from the option
		targetUser = ctx.Interaction.Member.User
	} else {
		targetUser = ctx.Interaction.Member.User
	}

	// Create an embed with user information
	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("User Information").
		WithTimestamp()

	message := fmt.Sprintf("**Username:** %s\n**ID:** %s", targetUser.Username, targetUser.ID)

	return builder.Info(ctx.Interaction, message)
}
```

### `(*ConfigSetSubCommand).Handle`

- Location: `pkg/discord/commands/core/examples.go:244`
- Signals: `NewCommandError`, `response:Error`, `response:Success`

```go
func (c *ConfigSetSubCommand) Handle(ctx *Context) error {
	extractor := NewOptionExtractor(GetSubCommandOptions(ctx.Interaction))

	key, err := extractor.StringRequired("key")
	if err != nil {
		return err
	}

	value, err := extractor.StringRequired("value")
	if err != nil {
		return err
	}

	// Use SafeGuildAccess for safe configuration manipulation
	err = SafeGuildAccess(ctx, func(guildConfig *files.GuildConfig) error {
		switch key {
		case "channels.commands":
			guildConfig.Channels.Commands = value
		case "channels.avatar_logging":
			guildConfig.Channels.AvatarLogging = value
		case "channels.automod_action":
			guildConfig.Channels.AutomodAction = value
		default:
			return NewValidationError("key", "Invalid configuration key")
		}
		return nil
	})

	if err != nil {
		return err
	}

	// Persist configuration
	persister := NewConfigPersister(c.configManager)
	if err := persister.Save(ctx.GuildConfig); err != nil {
		ctx.Logger.Error().Errorf("Failed to save config: %v", err)
		return NewCommandError("Failed to save configuration", true)
	}

	return NewResponseBuilder(ctx.Session).Success(ctx.Interaction, fmt.Sprintf("Configuration `%s` set to `%s`", key, value))
}
```

### `(*ConfigGetSubCommand).Handle`

- Location: `pkg/discord/commands/core/examples.go:315`
- Signals: `response:Info`

```go
func (c *ConfigGetSubCommand) Handle(ctx *Context) error {
	if err := RequiresGuildConfig(ctx); err != nil {
		return err
	}

	var config strings.Builder
	config.WriteString("**Server Configuration:**\n")
	config.WriteString(fmt.Sprintf("Command Channel: %s\n", ctx.GuildConfig.Channels.Commands))
	config.WriteString(fmt.Sprintf("Avatar Logging: %s\n", ctx.GuildConfig.Channels.AvatarLogging))
	config.WriteString(fmt.Sprintf("Automod Action: %s\n", ctx.GuildConfig.Channels.AutomodAction))
	config.WriteString(fmt.Sprintf("Allowed Roles: %d configured\n", len(ctx.GuildConfig.Roles.Allowed)))

	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Server Configuration").
		WithColor(theme.Info())

	return builder.Info(ctx.Interaction, config.String())
}
```

### `(*ConfigListSubCommand).Handle`

- Location: `pkg/discord/commands/core/examples.go:364`
- Signals: `response:Ephemeral`, `response:Info`

```go
func (c *ConfigListSubCommand) Handle(ctx *Context) error {
	options := []string{
		"**Available Configuration Options:**",
		"`channels.commands` - Channel for bot commands",
		"`channels.avatar_logging` - Channel for avatar logs",
		"`channels.automod_action` - Channel for automod logs",
		"",
		"Use `/config set <key> <value>` to modify these settings.",
	}

	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Configuration Options").
		Ephemeral()

	return builder.Info(ctx.Interaction, strings.Join(options, "\n"))
}
```

### `(*AdvancedCommand).Handle`

- Location: `pkg/discord/commands/core/examples.go:501`
- Signals: `NewCommandError`, `response:Error`, `response:Success`

```go
func (c *AdvancedCommand) Handle(ctx *Context) error {
	extractor := NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)

	input, err := extractor.StringRequired("input")
	if err != nil {
		return err // Validation error will be handled automatically
	}

	// Custom validations
	stringUtils := StringUtils{}
	if err := stringUtils.ValidateStringLength(input, 1, 100, "input"); err != nil {
		return err
	}

	// Operation that may fail
	result, err := c.processInput(input)
	if err != nil {
		// Log the error
		ctx.Logger.Error().Errorf("Failed to process input: %v", err)

		// Return a user-friendly error
		return NewCommandError("Failed to process your input. Please try again.", true)
	}

	// Success response
	builder := NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Processing Complete").
		WithTimestamp()

	return builder.Success(ctx.Interaction, fmt.Sprintf("Result: %s", result))
}
```

## pkg/discord/commands/core/guild_resolver.go

### `(*PermissionChecker).ResolveOwnerID`

- Location: `pkg/discord/commands/core/guild_resolver.go:15`
- Signals: `response:Error`

```go
func (pc *PermissionChecker) ResolveOwnerID(guildID string) (string, bool, error) {
	if pc == nil || guildID == "" {
		return "", false, nil
	}

	if pc.cache != nil {
		if g, ok := pc.cache.GetGuild(guildID); ok && g != nil && g.OwnerID != "" {
			return g.OwnerID, true, nil
		}
	}

	if pc.session != nil && pc.session.State != nil {
		if g, _ := pc.session.State.Guild(guildID); g != nil && g.OwnerID != "" {
			if pc.cache != nil {
				pc.cache.SetGuild(guildID, g)
			}
			if pc.store != nil {
				if err := pc.store.SetGuildOwnerID(guildID, g.OwnerID); err != nil {
					log.ErrorLoggerRaw().Error(
						"Guild resolver failed to persist owner from state",
						"operation", "commands.guild_resolver.resolve_owner.store_write",
						"guildID", guildID,
						"ownerID", g.OwnerID,
						"source", "state",
						"err", err,
					)
				}
			}
			return g.OwnerID, true, nil
		}
	}

	if pc.store != nil {
		ownerID, ok, err := pc.store.GetGuildOwnerID(guildID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Guild resolver failed to read owner from store",
				"operation", "commands.guild_resolver.resolve_owner.store_read",
				"guildID", guildID,
				"err", err,
			)
		} else if ok && ownerID != "" {
			return ownerID, true, nil
		}
	}

	if pc.session == nil {
		return "", false, fmt.Errorf("resolve owner id for guild %s: session not ready", guildID)
	}

	guild, err := pc.session.Guild(guildID)
	if err != nil {
		if isNotFoundRESTError(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("resolve owner id via rest for guild %s: %w", guildID, err)
	}
	if guild == nil || guild.OwnerID == "" {
		return "", false, nil
	}

	if pc.cache != nil {
		pc.cache.SetGuild(guildID, guild)
	}
	if pc.store != nil {
		if err := pc.store.SetGuildOwnerID(guildID, guild.OwnerID); err != nil {
			log.ErrorLoggerRaw().Error(
				"Guild resolver failed to persist owner from rest",
				"operation", "commands.guild_resolver.resolve_owner.store_write",
				"guildID", guildID,
				"ownerID", guild.OwnerID,
				"source", "rest",
				"err", err,
			)
		}
	}

	return guild.OwnerID, true, nil
}
```

## pkg/discord/commands/core/middleware.go

### `(*CommandRouter).permissionGateMiddleware`

- Location: `pkg/discord/commands/core/middleware.go:60`
- Signals: `NewCommandError`

```go
func (cr *CommandRouter) permissionGateMiddleware() InteractionMiddleware {
	return func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
		if routeKey.Kind != InteractionKindSlash {
			return next
		}

		return func(ctx *Context) error {
			handler, exists := cr.lookupSlashHandler(ctx.RouteKey)
			if !exists {
				return next(ctx)
			}

			if handler.RequiresGuild() && ctx.GuildID == "" {
				slog.Warn("Command used outside of guild", "commandPath", ctx.RouteKey.Path)
				return NewCommandError("This command can only be used in a server", true)
			}

			if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !cr.permChecker.HasPermission(ctx.GuildID, ctx.UserID) {
				slog.Warn("User without allowed role tried to use command", "commandPath", ctx.RouteKey.Path)
				return NewCommandError("You do not have permission to use this command", true)
			}

			if handler.RequiresPermissions() && !cr.permChecker.HasPermission(ctx.GuildID, ctx.UserID) {
				slog.Warn("User without permission tried to use command", "commandPath", ctx.RouteKey.Path)
				return NewCommandError("You do not have permission to use this command", true)
			}

			return next(ctx)
		}
	}
}
```

### `(*CommandRouter).errorMappingMiddleware`

- Location: `pkg/discord/commands/core/middleware.go:92`
- Signals: `response:Error`

```go
func (cr *CommandRouter) errorMappingMiddleware() InteractionMiddleware {
	return func(routeKey InteractionRouteKey, next InteractionHandlerFunc) InteractionHandlerFunc {
		if routeKey.Kind != InteractionKindSlash {
			return next
		}

		return func(ctx *Context) error {
			err := next(ctx)
			if err == nil {
				return nil
			}

			slog.Error("Slash route failed", "commandPath", ctx.RouteKey.Path, "err", err)
			respondToSlashError(ctx, err)
			return nil
		}
	}
}
```

### `applyInteractionAckPolicy`

- Location: `pkg/discord/commands/core/middleware.go:128`
- Signals: `response:InteractionRespond`

```go
func applyInteractionAckPolicy(ctx *Context, routeKey InteractionRouteKey, policy InteractionAckPolicy) error {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil || !policy.requiresAck() {
		return nil
	}

	switch routeKey.Kind {
	case InteractionKindSlash:
		return NewResponseManager(ctx.Session).DeferResponse(ctx.Interaction, policy.Ephemeral)
	case InteractionKindComponent, InteractionKindModal:
		return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
	default:
		return nil
	}
}
```

### `respondToSlashError`

- Location: `pkg/discord/commands/core/middleware.go:145`
- Signals: `response:Ephemeral`, `response:Error`

```go
func respondToSlashError(ctx *Context, err error) {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil || err == nil {
		return
	}

	if cmdErr, ok := err.(*CommandError); ok {
		builder := NewResponseBuilder(ctx.Session)
		if cmdErr.Ephemeral {
			builder = builder.Ephemeral()
		}
		builder.Error(ctx.Interaction, cmdErr.Message)
		return
	}

	NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, "An error occurred while executing the command")
}
```

## pkg/discord/commands/core/registry.go

### `(*CommandManager).SetupCommands`

- Location: `pkg/discord/commands/core/registry.go:174`
- Signals: `response:Info`

```go
func (cm *CommandManager) SetupCommands() error {
	// Verify session state is properly initialized
	if cm.session == nil || cm.session.State == nil || cm.session.State.User == nil {
		return fmt.Errorf("session not properly initialized")
	}

	// Prevent duplicated interaction handling in reinit/hot-reload paths.
	if cm.interactionHandlerCancel != nil {
		cm.interactionHandlerCancel()
		cm.interactionHandlerCancel = nil
	}
	cm.interactionHandlerCancel = cm.session.AddHandler(cm.router.HandleInteraction)
	rollback := func(err error) error {
		if cm.interactionHandlerCancel != nil {
			cm.interactionHandlerCancel()
			cm.interactionHandlerCancel = nil
		}
		return err
	}

	// Fetch commands already registered on Discord
	registered, err := cm.session.ApplicationCommands(cm.session.State.User.ID, "")
	if err != nil {
		return rollback(fmt.Errorf("failed to fetch registered commands: %w", err))
	}

	// Build map of registered commands
	regByName := make(map[string]*discordgo.ApplicationCommand, len(registered))
	for _, rc := range registered {
		regByName[rc.Name] = rc
	}

	// Build map of code-defined commands
	codeCommands := cm.router.registry.GetAllCommands()
	codeByName := maps.Clone(codeCommands)

	// Create/Update commands as needed
	created, updated, unchanged := 0, 0, 0
	commandIDs := make(map[string]string, len(codeCommands))
	for name, cmd := range codeCommands {
		desired := &discordgo.ApplicationCommand{
			Name:        cmd.Name(),
			Description: cmd.Description(),
			Options:     normalizeCommandOptions(cmd.Options()),
		}
		if existing, ok := regByName[name]; ok {
			// Command already exists, check if it needs updating
			if CompareCommands(existing, desired) {
				slog.Info(fmt.Sprintf("Command unchanged: /%s %s - %s", name, FormatOptions(cmd.Options()), cmd.Description()))
				unchanged++
				commandIDs[name] = existing.ID
				continue
			}

			// Update command
			updatedCmd, err := cm.session.ApplicationCommandEdit(cm.session.State.User.ID, "", existing.ID, desired)
			if err != nil {
				return rollback(fmt.Errorf("error updating command '%s': %w", name, err))
			}
			if updatedCmd != nil {
				commandIDs[name] = updatedCmd.ID
			} else {
				commandIDs[name] = existing.ID
			}
			slog.Info(fmt.Sprintf("Command updated: /%s %s - %s", name, FormatOptions(cmd.Options()), cmd.Description()))
			updated++
		} else {
			// Create new command
			createdCmd, err := cm.session.ApplicationCommandCreate(cm.session.State.User.ID, "", desired)
			if err != nil {
				return rollback(fmt.Errorf("error creating command '%s': %w", name, err))
			}
			if createdCmd != nil {
				commandIDs[name] = createdCmd.ID
			}
			slog.Info(fmt.Sprintf("Command created: /%s %s - %s", name, FormatOptions(cmd.Options()), cmd.Description()))
			created++
		}
	}

	// Remove orphaned commands (present on Discord but not in code)
	deleted := 0
	for _, rc := range registered {
		if _, exists := codeByName[rc.Name]; !exists {
			if err := cm.session.ApplicationCommandDelete(cm.session.State.User.ID, "", rc.ID); err != nil {
				slog.Warn(fmt.Sprintf("Error removing orphan command: %s, error: %v", rc.Name, err))
				continue
			}
			slog.Info(fmt.Sprintf("Orphan command removed: /%s %s - %s", rc.Name, FormatOptions(rc.Options), rc.Description))
			deleted++
		}
	}
	// Log do resumo
	slog.Info(fmt.Sprintf("Command synchronization completed: created=%d, updated=%d, deleted=%d, unchanged=%d, total=%d, mode=incremental", created, updated, deleted, unchanged, len(codeCommands)))

	return nil
}
```

### `(*GroupCommand).Handle`

- Location: `pkg/discord/commands/core/registry.go:364`
- Signals: `NewCommandError`

```go
func (gc *GroupCommand) Handle(ctx *Context) error {
	subCommandName := GetSubCommandName(ctx.Interaction)
	if subCommandName == "" {
		return NewCommandError("Subcommand is required", true)
	}

	subcmd, exists := gc.subcommands[subCommandName]
	if !exists {
		return NewCommandError("Unknown subcommand", true)
	}

	// Check subcommand-specific permissions
	if subcmd.RequiresGuild() && ctx.GuildID == "" {
		return NewCommandError("This subcommand can only be used in a server", true)
	}

	if ctx.GuildConfig != nil && len(ctx.GuildConfig.Roles.Allowed) > 0 && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		return NewCommandError("You do not have permission to use this subcommand", true)
	}

	if subcmd.RequiresPermissions() && !gc.checker.HasPermission(ctx.GuildID, ctx.UserID) {
		return NewCommandError("You don't have permission to use this subcommand", true)
	}

	return subcmd.Handle(ctx)
}
```

## pkg/discord/commands/core/response.go

### `(*ResponseManager).Ephemeral`

- Location: `pkg/discord/commands/core/response.go:81`
- Signals: `response:Info`

```go
func (rm *ResponseManager) Ephemeral(i *discordgo.InteractionCreate, message string) error {
	config := rm.config
	config.Ephemeral = true
	return rm.WithConfig(config).Info(i, message)
}
```

### `(*ResponseManager).Custom`

- Location: `pkg/discord/commands/core/response.go:88`
- Signals: `response:InteractionRespond`

```go
func (rm *ResponseManager) Custom(i *discordgo.InteractionCreate, content string, embeds []*discordgo.MessageEmbed) error {
	data := rm.buildResponseData(content, embeds)
	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}
```

### `(*ResponseManager).sendTextResponse`

- Location: `pkg/discord/commands/core/response.go:128`
- Signals: `response:Custom`

```go
func (rm *ResponseManager) sendTextResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	content := rm.formatTextMessage(message, responseType)
	return rm.Custom(i, content, nil)
}
```

### `(*ResponseManager).sendEmbedResponse`

- Location: `pkg/discord/commands/core/response.go:134`
- Signals: `response:Custom`

```go
func (rm *ResponseManager) sendEmbedResponse(i *discordgo.InteractionCreate, message string, responseType ResponseType) error {
	embed := rm.createEmbed(message, responseType)
	return rm.Custom(i, "", []*discordgo.MessageEmbed{embed})
}
```

### `(*ResponseManager).getColorForType`

- Location: `pkg/discord/commands/core/response.go:184`
- Signals: `response:Error`, `response:Info`, `response:Loading`, `response:Success`, `response:Warning`

```go
func (rm *ResponseManager) getColorForType(responseType ResponseType) int {
	if rm.config.Color != 0 {
		return rm.config.Color
	}

	switch responseType {
	case ResponseSuccess:
		return theme.Success()
	case ResponseError:
		return theme.Error()
	case ResponseWarning:
		return theme.Warning()
	case ResponseInfo:
		return theme.Info()
	case ResponseLoading:
		return theme.Loading()
	default:
		return theme.Muted()
	}
}
```

### `(*ResponseManager).Autocomplete`

- Location: `pkg/discord/commands/core/response.go:224`
- Signals: `response:InteractionRespond`

```go
func (rm *ResponseManager) Autocomplete(i *discordgo.InteractionCreate, choices []*discordgo.ApplicationCommandOptionChoice) error {
	if len(choices) > 25 {
		choices = choices[:25]
	}

	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}
```

### `(*ResponseManager).DeferResponse`

- Location: `pkg/discord/commands/core/response.go:236`
- Signals: `response:InteractionRespond`

```go
func (rm *ResponseManager) DeferResponse(i *discordgo.InteractionCreate, ephemeral bool) error {
	return rm.session.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: rm.buildFlags(ephemeral),
		},
	})
}
```

### `(*ResponseBuilder).Success`

- Location: `pkg/discord/commands/core/response.go:364`
- Signals: `response:Success`

```go
func (rb *ResponseBuilder) Success(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Success(i, message)
}
```

### `(*ResponseBuilder).Error`

- Location: `pkg/discord/commands/core/response.go:369`
- Signals: `response:Error`

```go
func (rb *ResponseBuilder) Error(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Error(i, message)
}
```

### `(*ResponseBuilder).Info`

- Location: `pkg/discord/commands/core/response.go:374`
- Signals: `response:Info`

```go
func (rb *ResponseBuilder) Info(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Info(i, message)
}
```

### `(*ResponseBuilder).Warning`

- Location: `pkg/discord/commands/core/response.go:379`
- Signals: `response:Warning`

```go
func (rb *ResponseBuilder) Warning(i *discordgo.InteractionCreate, message string) error {
	return rb.Build().Warning(i, message)
}
```

## pkg/discord/commands/core/router.go

### `(*CommandRouter).handleSlashCommandRoute`

- Location: `pkg/discord/commands/core/router.go:46`
- Signals: `NewCommandError`, `response:Error`

```go
func (cr *CommandRouter) handleSlashCommandRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = commandRoutePath(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupSlashHandler(ctx.RouteKey)
		if !exists {
			return NewCommandError("Command not found", true)
		}
		return handler.Handle(ctx)
	})
	if err != nil {
		slog.Error("Slash route returned unmapped error", "commandPath", routeKey.Path, "err", err)
		respondToSlashError(ctx, err)
	}
}
```

### `(*CommandRouter).handleAutocompleteRoute`

- Location: `pkg/discord/commands/core/router.go:75`
- Signals: `response:Error`

```go
func (cr *CommandRouter) handleAutocompleteRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = commandRoutePath(i)
	}
	if routeKey.FocusedOption == "" {
		routeKey.FocusedOption = focusedOptionName(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupAutocompleteHandler(ctx.RouteKey)
		if !exists || ctx.RouteKey.FocusedOption == "" {
			return nil
		}

		var err error
		choices, err = handler.HandleAutocomplete(ctx, ctx.RouteKey.FocusedOption)
		return err
	})
	if err != nil {
		slog.Error("Autocomplete handler failed", "err", err)
		choices = []*discordgo.ApplicationCommandOptionChoice{}
	}

	NewResponseBuilder(ctx.Session).Build().Autocomplete(i, choices)
}
```

### `(*CommandRouter).handleComponentRoute`

- Location: `pkg/discord/commands/core/router.go:104`
- Signals: `response:Error`

```go
func (cr *CommandRouter) handleComponentRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = interactionCustomRouteID(interactionCustomID(i))
	}
	if routeKey.CustomID == "" {
		routeKey.CustomID = interactionCustomID(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupComponentHandler(ctx.RouteKey)
		if !exists {
			return nil
		}
		return handler.HandleComponent(ctx)
	})
	if err != nil {
		slog.Error("Component handler failed", "routeID", routeKey.Path, "customID", routeKey.CustomID, "err", err)
	}
}
```

### `(*CommandRouter).handleModalRoute`

- Location: `pkg/discord/commands/core/router.go:125`
- Signals: `response:Error`

```go
func (cr *CommandRouter) handleModalRoute(i *discordgo.InteractionCreate, routeKey InteractionRouteKey) {
	if routeKey.Path == "" {
		routeKey.Path = interactionCustomRouteID(interactionCustomID(i))
	}
	if routeKey.CustomID == "" {
		routeKey.CustomID = interactionCustomID(i)
	}

	ctx := cr.contextBuilder.BuildContext(i)
	err := cr.executeRoute(ctx, routeKey, func(ctx *Context) error {
		handler, exists := cr.lookupModalHandler(ctx.RouteKey)
		if !exists {
			return nil
		}
		return handler.HandleModal(ctx)
	})
	if err != nil {
		slog.Error("Modal handler failed", "routeID", routeKey.Path, "customID", routeKey.CustomID, "err", err)
	}
}
```

## pkg/discord/commands/core/utils.go

### `(*PermissionChecker).HasPermission`

- Location: `pkg/discord/commands/core/utils.go:153`
- Signals: `response:Error`

```go
func (pc *PermissionChecker) HasPermission(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	if pc.hasAdministrativeAccess(guildID, userID) {
		return true
	}
	guildConfig := pc.config.GuildConfig(guildID)
	if guildConfig == nil || len(guildConfig.Roles.Allowed) == 0 {
		return false
	}

	member, memberFound, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member",
			"operation", "commands.permission.has_permission.resolve_member",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !memberFound || member == nil {
		return false
	}

	for _, userRole := range member.Roles {
		if slices.Contains(guildConfig.Roles.Allowed, userRole) {
			return true
		}
	}
	return false
}
```

### `(*PermissionChecker).hasAdministrativeAccess`

- Location: `pkg/discord/commands/core/utils.go:188`
- Signals: `response:Error`

```go
func (pc *PermissionChecker) hasAdministrativeAccess(guildID, userID string) bool {
	ownerID, ownerFound, err := pc.ResolveOwnerID(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild owner",
			"operation", "commands.permission.has_permission.resolve_owner",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
	}
	if err == nil && ownerFound && ownerID == userID {
		return true
	}

	member, memberFound, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member for admin access",
			"operation", "commands.permission.has_permission.resolve_member_admin",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !memberFound || member == nil {
		return false
	}

	roles, err := pc.ResolveRoles(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild roles for admin access",
			"operation", "commands.permission.has_permission.resolve_roles_admin",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	permissions := memberPermissionBits(member, roles, guildID)
	if permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return true
	}
	if permissions&discordgo.PermissionManageGuild == discordgo.PermissionManageGuild {
		return true
	}
	return false
}
```

### `(*PermissionChecker).HasRole`

- Location: `pkg/discord/commands/core/utils.go:264`
- Signals: `response:Error`

```go
func (pc *PermissionChecker) HasRole(guildID, userID, roleID string) bool {
	member, ok, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member for role check",
			"operation", "commands.permission.has_role.resolve_member",
			"guildID", guildID,
			"userID", userID,
			"roleID", roleID,
			"err", err,
		)
		return false
	}
	if !ok || member == nil {
		return false
	}
	return slices.Contains(member.Roles, roleID)
}
```

### `(*PermissionChecker).IsOwner`

- Location: `pkg/discord/commands/core/utils.go:284`
- Signals: `response:Error`

```go
func (pc *PermissionChecker) IsOwner(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	ownerID, ok, err := pc.ResolveOwnerID(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild owner for owner check",
			"operation", "commands.permission.is_owner.resolve_owner",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !ok {
		return false
	}
	return ownerID == userID
}
```

### `(EmbedBuilder).Success`

- Location: `pkg/discord/commands/core/utils.go:433`
- Signals: `response:Success`

```go
func (EmbedBuilder) Success(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Success(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
```

### `(EmbedBuilder).Error`

- Location: `pkg/discord/commands/core/utils.go:443`
- Signals: `response:Error`

```go
func (EmbedBuilder) Error(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Error(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
```

### `(EmbedBuilder).Info`

- Location: `pkg/discord/commands/core/utils.go:453`
- Signals: `response:Info`

```go
func (EmbedBuilder) Info(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Info(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
```

### `(EmbedBuilder).Warning`

- Location: `pkg/discord/commands/core/utils.go:463`
- Signals: `response:Warning`

```go
func (EmbedBuilder) Warning(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Warning(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
```

## pkg/discord/commands/handler.go

### `(*CommandHandler).SetupCommands`

- Location: `pkg/discord/commands/handler.go:57`
- Signals: `response:Error`, `response:Info`

```go
func (ch *CommandHandler) SetupCommands() error {
	log.ApplicationLogger().Info("Setting up bot commands...")

	// Re-init safety: avoid duplicated handlers if setup is called more than once.
	if ch.commandManager != nil {
		log.ApplicationLogger().Warn("Command setup called with active handlers; cleaning previous registrations first")
		if err := ch.Shutdown(); err != nil {
			return fmt.Errorf("cleanup previous command handlers: %w", err)
		}
	}

	// Create the command manager
	ch.commandManager = core.NewCommandManager(ch.session, ch.configManager)
	if router := ch.commandManager.GetRouter(); router != nil {
		router.SetGuildRouteFilter(ch.handlesGuildRoute)
	}

	// Register configuration commands
	if err := ch.registerConfigCommands(); err != nil {
		return fmt.Errorf("failed to register config commands: %w", err)
	}

	// Configure commands on Discord
	if err := ch.commandManager.SetupCommands(); err != nil {
		if ch.commandManager != nil {
			if shutdownErr := ch.commandManager.Shutdown(); shutdownErr != nil {
				log.ErrorLoggerRaw().Error("Failed to rollback command manager handler registration", "err", shutdownErr)
			}
			ch.commandManager = nil
		}
		return fmt.Errorf("failed to setup commands: %w", err)
	}

	log.ApplicationLogger().Info("Bot commands setup completed successfully")
	return nil
}
```

### `(*CommandHandler).registerConfigCommands`

- Location: `pkg/discord/commands/handler.go:115`
- Signals: `response:Info`

```go
func (ch *CommandHandler) registerConfigCommands() error {
	router := ch.commandManager.GetRouter()

	// Register the /config group and simple commands (ping/echo)
	config.NewConfigCommands(ch.configManager).RegisterCommands(router)

	// Register the /config runtime panel (replaces env-var operational toggles)
	runtime.NewRuntimeConfigCommands(ch.configManager).RegisterCommands(router)

	// Register metrics commands (activity, members)
	metrics.RegisterMetricsCommands(router)
	// Register partner CRUD commands
	if ch.partnerBoardService != nil || ch.partnerSyncExecutor != nil {
		boardService := ch.partnerBoardService
		if boardService == nil {
			boardService = partners.NewBoardApplicationService(ch.configManager, nil)
		}
		partner.NewPartnerCommandsWithServices(boardService, ch.partnerSyncExecutor).RegisterCommands(router)
	} else {
		partner.NewPartnerCommands(ch.configManager).RegisterCommands(router)
	}
	// Register moderation commands
	moderation.RegisterModerationCommands(router)
	qotdcmd.NewCommands(ch.qotdService).RegisterCommands(router)

	log.ApplicationLogger().Info("Config, partner, metrics, and moderation commands registered successfully")
	return nil
}
```

### `(*CommandHandler).Shutdown`

- Location: `pkg/discord/commands/handler.go:145`
- Signals: `response:Info`

```go
func (ch *CommandHandler) Shutdown() error {
	log.ApplicationLogger().Info("Shutting down command handler...")

	var errs []error
	if ch.commandManager != nil {
		if err := ch.commandManager.Shutdown(); err != nil {
			errs = append(errs, fmt.Errorf("shutdown command manager: %w", err))
		}
		ch.commandManager = nil
	}

	return errors.Join(errs...)
}
```

## pkg/discord/commands/metrics/metrics_commands.go

### `handleActivity`

- Location: `pkg/discord/commands/metrics/metrics_commands.go:109`
- Signals: `response:Error`

```go
func handleActivity(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	// Parse options
	rangeOpt := getStringOpt(i, "range", "7d")
	topN := clampInt(int(getIntOpt(i, "top", 5)), 1, 10)
	channelID := getChannelOpt(s, i, "channel", "")
	section := strings.ToLower(getStringOpt(i, "section", "both")) // both|messages|reactions
	scope := strings.ToLower(getStringOpt(i, "scope", "both"))     // both|channels|users
	format := strings.ToLower(getStringOpt(i, "format", "full"))   // full|compact
	if format == "compact" {
		topN = clampInt(topN, 1, 3)
	}

	cutoff, label := cutoffForRange(rangeOpt)

	store := ctx.Router().GetStore()
	if store == nil {
		return respondError(s, i, "Metrics storage is not configured.")
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Collect activity
	msgTotalsByChannel, err := store.MessageTotalsByChannel(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.message_totals_by_channel",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	msgTotalsByUser, err := store.MessageTotalsByUser(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.message_totals_by_user",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	reactTotalsByChannel, err := store.ReactionTotalsByChannel(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.reaction_totals_by_channel",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	reactTotalsByUser, err := store.ReactionTotalsByUser(ctxTimeout, ctx.GuildID, cutoff, channelID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Metrics activity query failed",
			"operation", "metrics.activity.query.reaction_totals_by_user",
			"guildID", ctx.GuildID,
			"channelID", channelID,
			"cutoffDay", cutoff,
			"err", err,
		)
		return respondError(s, i, "Failed to query activity metrics from the database. Try again shortly.")
	}

	// Build embed
	chFilterStr := ""
	if channelID != "" {
		chFilterStr = fmt.Sprintf(" in <#%s>", channelID)
	}
	title := fmt.Sprintf("Activity: %s%s", label, chFilterStr)

	fields := []*discordgo.MessageEmbedField{}
	includeMessages := section == "both" || section == "messages"
	includeReactions := section == "both" || section == "reactions"
	includeChannels := scope == "both" || scope == "channels"
	includeUsers := scope == "both" || scope == "users"

	if includeMessages && includeChannels {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Messages - Top Channels",
			Value:  renderTop(msgTotalsByChannel, topN, func(id string) string { return channelMention(id) }),
			Inline: true,
		})
	}
	if includeMessages && includeUsers {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Messages - Top Users",
			Value:  renderTop(msgTotalsByUser, topN, func(id string) string { return userMention(id) }),
			Inline: true,
		})
	}
	if includeReactions && includeChannels {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Reactions - Top Channels",
			Value:  renderTop(reactTotalsByChannel, topN, func(id string) string { return channelMention(id) }),
			Inline: true,
		})
	}
	if includeReactions && includeUsers {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Reactions - Top Users",
			Value:  renderTop(reactTotalsByUser, topN, func(id string) string { return userMention(id) }),
			Inline: true,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       title,
		Color:       theme.Primary(),
		Description: "Message and reaction activity across channels and users.",
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      fields,
	}

	return respondEmbed(s, i, embed)
}
```

### `handleServerStatsHealth`

- Location: `pkg/discord/commands/metrics/metrics_commands.go:269`
- Signals: `response:Error`, `response:Info`

```go
func handleServerStatsHealth(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	store := ctx.Router().GetStore()
	if store == nil {
		return respondError(s, i, "Metrics storage is not configured.")
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1) Current members (only those currently in the server)
	currentMemberCount := 0
	if g, err := s.State.Guild(ctx.GuildID); err == nil && g != nil {
		currentMemberCount = g.MemberCount
	}
	if currentMemberCount == 0 {
		// Fallback if the state doesn't have the information
		if g, err := s.Guild(ctx.GuildID); err == nil && g != nil {
			currentMemberCount = g.MemberCount
		}
	}

	// 2) Historical total of members who have ever joined (based on member_joins)
	totalHistoricJoins, totalHistoricJoinsErr := store.CountDistinctMemberJoins(ctxTimeout, ctx.GuildID)
	hasHistoricJoins := totalHistoricJoinsErr == nil

	// 3) How many of those historically recorded members are still in the server
	stillPresentCount := int64(0)
	hasStillPresent := hasHistoricJoins
	if hasHistoricJoins && totalHistoricJoins > 0 {
		joinedUserIDs, err := store.ListDistinctMemberJoinUserIDs(ctxTimeout, ctx.GuildID)
		if err != nil {
			hasStillPresent = false
			log.ErrorLoggerRaw().Error(
				"Metrics health retention query failed",
				"operation", "metrics.serverstats.health.retention_query",
				"guildID", ctx.GuildID,
				"err", err,
			)
		} else {
			for _, userID := range joinedUserIDs {
				// Check if the user is present in the bot state cache.
				if _, err := s.State.Member(ctx.GuildID, userID); err == nil {
					stillPresentCount++
				}
			}
		}
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "👥 Current Members",
			Value:  fmt.Sprintf("`%d` members currently in the server.", currentMemberCount),
			Inline: false,
		},
		{
			Name:   "📥 Join History",
			Value:  fmt.Sprintf("`%s` unique users recorded in the database since tracking began.", formatMaybe(totalHistoricJoins, hasHistoricJoins)),
			Inline: false,
		},
		{
			Name:   "✅ Retention",
			Value:  fmt.Sprintf("`%s` of historically recorded users are still in the server.", formatMaybe(stillPresentCount, hasStillPresent)),
			Inline: false,
		},
	}

	// Database health
	dbSizeLabel := "N/A"
	if size, err := store.DatabaseSizeBytes(ctxTimeout); err == nil {
		dbSizeLabel = formatBytes(size)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "📊 Server Health Stats",
		Color:       theme.Info(),
		Description: fmt.Sprintf("Data extracted from the database and bot state.\nDatabase size: `%s`", dbSizeLabel),
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Note: retention accuracy depends on the bot's member cache.",
		},
	}

	return respondEmbed(s, i, embed)
}
```

### `handleServerStatsPeriodic`

- Location: `pkg/discord/commands/metrics/metrics_commands.go:361`
- Signals: `response:Success`

```go
func handleServerStatsPeriodic(ctx *core.Context, rangeVal string) error {
	s := ctx.Session
	i := ctx.Interaction
	if ctx.GuildID == "" {
		return respondError(s, i, "This command must be used in a server.")
	}

	store := ctx.Router().GetStore()
	if store == nil {
		return respondError(s, i, "Metrics storage is not configured.")
	}

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cutoff, label := cutoffForRange(rangeVal)

	joins, joinsErr := store.SumDailyMemberJoinsSince(ctxTimeout, ctx.GuildID, cutoff)
	leaves, leavesErr := store.SumDailyMemberLeavesSince(ctxTimeout, ctx.GuildID, cutoff)
	hasJoins := joinsErr == nil
	hasLeaves := leavesErr == nil

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "📥 Members Joined",
			Value:  fmt.Sprintf("`%s` joins in the last %s.", formatMaybe(joins, hasJoins), label),
			Inline: true,
		},
		{
			Name:   "📤 Members Left",
			Value:  fmt.Sprintf("`%s` leaves in the last %s.", formatMaybe(leaves, hasLeaves), label),
			Inline: true,
		},
		{
			Name:   "📈 Net Growth",
			Value:  fmt.Sprintf("`%s` members.", formatMaybeNet(joins, hasJoins, leaves, hasLeaves)),
			Inline: true,
		},
	}

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("📊 Server Stats (%s)", label),
		Color:     theme.Success(),
		Fields:    fields,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return respondEmbed(s, i, embed)
}
```

### `respondError`

- Location: `pkg/discord/commands/metrics/metrics_commands.go:426`
- Signals: `response:InteractionRespond`

```go
func respondError(s *discordgo.Session, i *discordgo.InteractionCreate, msg string) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: msg,
			Flags:   1 << 6, // ephemeral
		},
	})
}
```

### `respondEmbed`

- Location: `pkg/discord/commands/metrics/metrics_commands.go:436`
- Signals: `response:InteractionRespond`

```go
func respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) error {
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
```

### `handleBackfillRun`

- Location: `pkg/discord/commands/metrics/metrics_commands.go:606`
- Signals: `response:Info`

```go
func handleBackfillRun(ctx *core.Context) error {
	s := ctx.Session
	i := ctx.Interaction

	router := ctx.Router()
	if router == nil {
		return respondError(s, i, "Command router not available.")
	}
	taskRouter := router.GetTaskRouter()
	if taskRouter == nil {
		return respondError(s, i, "Task router not available (Monitoring service might be disabled).")
	}

	channelID := getChannelOpt(s, i, "channel", "")
	if channelID == "" {
		if ctx.GuildConfig != nil {
			channelID = ctx.GuildConfig.Channels.BackfillChannelID()
		}
	}

	if channelID == "" {
		return respondError(s, i, "No channel specified and no default welcome channel configured.")
	}

	days := getIntOpt(i, "days", 7)
	startDateRaw := getStringOpt(i, "start_date", "")

	var taskType string
	var payload any
	var desc string

	if startDateRaw != "" {
		// Day mode
		_, err := time.Parse("2006-01-02", startDateRaw)
		if err != nil {
			return respondError(s, i, "Invalid start_date format. Use YYYY-MM-DD.")
		}
		taskType = "monitor.backfill_entry_exit_day"
		payload = struct{ ChannelID, Day string }{ChannelID: channelID, Day: startDateRaw}
		desc = fmt.Sprintf("Scanning channel <#%s> for day `%s`.", channelID, startDateRaw)
	} else {
		// Range mode
		now := time.Now().UTC()
		start := now.AddDate(0, 0, -int(days)).Format(time.RFC3339)
		end := now.Format(time.RFC3339)
		taskType = "monitor.backfill_entry_exit_range"
		payload = struct{ ChannelID, Start, End string }{ChannelID: channelID, Start: start, End: end}
		desc = fmt.Sprintf("Scanning channel <#%s> for the last `%d` days.", channelID, days)
	}

	err := taskRouter.Dispatch(context.Background(), task.Task{
		Type:    taskType,
		Payload: payload,
		Options: task.TaskOptions{GroupKey: "backfill:" + channelID},
	})

	if err != nil {
		return respondError(s, i, fmt.Sprintf("Failed to dispatch backfill task: %v", err))
	}

	embed := &discordgo.MessageEmbed{
		Title:       "▶️ Backfill Started",
		Description: desc,
		Color:       theme.Info(),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "This process runs in the background. Use /metrics backfill-status to check progress.",
		},
	}

	return respondEmbed(s, i, embed)
}
```

## pkg/discord/commands/moderation/moderation_commands.go

### `(*banCommand).Handle`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:272`
- Signals: `helper-call:buildBanCommandMessage`, `response:Error`, `response:Success`

```go
func (c *banCommand) Handle(ctx *core.Context) error {
	if err := ensureModerationCommandEnabled(ctx, "moderation.ban", "Ban command is disabled for this server."); err != nil {
		return err
	}
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canBanTarget(ctx, banCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot ban `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)

	if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, userID, reason, 0); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to ban user %s: %v", userID, err), true)
	}

	details := "Status: Success"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "member_ban_add",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildBanCommandMessage(targetUsername, reason, truncated))
}
```

### `(*massBanCommand).Handle`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:349`
- Signals: `helper-call:buildMassBanCommandMessage`, `response:Error`, `response:Info`, `response:Success`

```go
func (c *massBanCommand) Handle(ctx *core.Context) error {
	if err := ensureModerationCommandEnabled(ctx, "moderation.massban", "Mass ban command is disabled for this server."); err != nil {
		return err
	}
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	membersInput, err := extractor.StringRequired("members")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	memberIDs, invalidTokens := parseMemberIDs(membersInput)
	if len(memberIDs) == 0 {
		return core.NewCommandError("No valid member IDs provided", true)
	}
	if len(invalidTokens) > 0 {
		log.ApplicationLogger().Info("Massban ignored invalid member tokens", "guildID", ctx.GuildID, "invalid_count", len(invalidTokens))
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return err
	}

	bannedCount := 0
	var failed []string
	var skipped []string
	for _, memberID := range memberIDs {
		targetUsername := resolveUserDisplayName(ctx, memberID)
		logPayload := moderationLogPayload{
			Action:      "member_ban_add",
			TargetID:    memberID,
			TargetLabel: targetUsername,
			Reason:      reason,
			RequestedBy: ctx.UserID,
		}

		ok, reasonText := canBanTarget(ctx, banCtx, memberID)
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", memberID, reasonText))
			logPayload.Extra = "Status: Skipped | " + reasonText
			sendModerationCaseActionLog(ctx, logPayload)
			continue
		}

		if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, memberID, reason, 0); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", memberID, err))
			logPayload.Extra = fmt.Sprintf("Status: Failed | %v", err)
			sendModerationCaseActionLog(ctx, logPayload)
			continue
		}
		bannedCount++
		logPayload.Extra = "Status: Success"
		if truncated {
			logPayload.Extra += " | Reason truncated to 512 characters"
		}
		sendModerationCaseActionLog(ctx, logPayload)
	}
	if len(skipped) > 0 || len(failed) > 0 {
		log.ApplicationLogger().Info(
			"Massban finished with partial failures",
			"guildID", ctx.GuildID,
			"requested", len(memberIDs),
			"banned", bannedCount,
			"skipped", len(skipped),
			"failed", len(failed),
		)
	}
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildMassBanCommandMessage(bannedCount))
}
```

### `(*kickCommand).Handle`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:451`
- Signals: `helper-call:buildKickCommandMessage`, `response:Error`, `response:Success`

```go
func (c *kickCommand) Handle(ctx *core.Context) error {
	if err := ensureModerationCommandEnabled(ctx, "moderation.kick", "Kick command is disabled for this server."); err != nil {
		return err
	}
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	kickCtx, err := prepareKickContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canKickTarget(ctx, kickCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot kick `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	if err := ctx.Session.GuildMemberDeleteWithReason(ctx.GuildID, userID, reason); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to kick user %s: %v", userID, err), true)
	}

	details := "Status: Success"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "kick",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildKickCommandMessage(targetUsername, reason, truncated))
}
```

### `(*timeoutCommand).Handle`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:536`
- Signals: `helper-call:buildTimeoutCommandMessage`, `response:Error`, `response:Success`

```go
func (c *timeoutCommand) Handle(ctx *core.Context) error {
	if err := ensureModerationCommandEnabled(ctx, "moderation.timeout", "Timeout command is disabled for this server."); err != nil {
		return err
	}
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	minutes := extractor.Int("minutes")
	if minutes <= 0 {
		return core.NewCommandError("Please provide a valid timeout duration in minutes.", true)
	}
	if minutes > timeoutMaxMinutes {
		return core.NewCommandError("Timeout duration cannot exceed 40320 minutes (28 days).", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	timeoutCtx, err := prepareTimeoutContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canTimeoutTarget(ctx, timeoutCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot timeout `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	until := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	if err := ctx.Session.GuildMemberTimeout(ctx.GuildID, userID, &until, discordgo.WithAuditLogReason(reason)); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to timeout user %s: %v", userID, err), true)
	}

	details := fmt.Sprintf("Duration: %s | Ends: <t:%d:F> (<t:%d:R>)", formatTimeoutDuration(minutes), until.Unix(), until.Unix())
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "timeout",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildTimeoutCommandMessage(targetUsername, minutes, reason, truncated))
}
```

### `(*muteCommand).Handle`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:622`
- Signals: `helper-call:buildMuteCommandMessage`, `response:Error`, `response:Success`

```go
func (c *muteCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	muteCtx, err := prepareMuteContext(ctx)
	if err != nil {
		return err
	}

	muteRole, roleID, err := resolveConfiguredMuteRole(ctx, muteCtx)
	if err != nil {
		return err
	}

	if ok, reasonText := canMuteTarget(ctx, muteCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot mute `%s`: %s.", userID, reasonText), true)
	}

	targetMember, ok, reasonText := resolveRoleTargetMember(ctx, userID)
	if !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot mute `%s`: %s.", userID, reasonText), true)
	}
	if memberHasRole(targetMember, roleID) {
		return core.NewCommandError(fmt.Sprintf("Cannot mute `%s`: target already has the configured mute role.", userID), true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	if err := ctx.Session.GuildMemberRoleAdd(ctx.GuildID, userID, roleID); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to mute user %s: %v", userID, err), true)
	}

	details := fmt.Sprintf("Role applied: %s (`%s`)", formatRoleDisplayName(muteRole), roleID)
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "mute",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildMuteCommandMessage(targetUsername, muteRole, reason, truncated))
}
```

### `(*warnCommand).Handle`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:709`
- Signals: `helper-call:buildWarnCommandMessage`, `response:Error`, `response:Success`

```go
func (c *warnCommand) Handle(ctx *core.Context) error {
	if err := ensureModerationCommandEnabled(ctx, "moderation.warn", "Warn command is disabled for this server."); err != nil {
		return err
	}
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	warnCtx, err := prepareWarnContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canWarnTarget(ctx, warnCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot warn `%s`: %s.", userID, reasonText), true)
	}

	store := moderationStoreFromContext(ctx)
	if store == nil {
		return core.NewCommandError("Warnings storage is not available for this bot instance.", true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	warning, err := store.CreateModerationWarning(ctx.GuildID, userID, ctx.UserID, reason, time.Now().UTC())
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to create warning for %s: %v", userID, err), true)
	}

	details := "Warning recorded"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:        "warn",
		TargetID:      userID,
		TargetLabel:   targetUsername,
		Reason:        reason,
		RequestedBy:   ctx.UserID,
		Extra:         details,
		CaseNumber:    warning.CaseNumber,
		HasCaseNumber: true,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildWarnCommandMessage(targetUsername, warning.CaseNumber, reason, truncated))
}
```

### `(*warningsCommand).Handle`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:796`
- Signals: `helper-call:buildWarningsCommandMessage`, `response:Ephemeral`, `response:Error`, `response:Info`

```go
func (c *warningsCommand) Handle(ctx *core.Context) error {
	if err := ensureModerationCommandEnabled(ctx, "moderation.warnings", "Warnings command is disabled for this server."); err != nil {
		return err
	}
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	limit := int(extractor.Int("limit"))
	if limit <= 0 {
		limit = 5
	}

	warnCtx, err := prepareWarnContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canWarnTarget(ctx, warnCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot inspect warnings for `%s`: %s.", userID, reasonText), true)
	}

	store := moderationStoreFromContext(ctx)
	if store == nil {
		return core.NewCommandError("Warnings storage is not available for this bot instance.", true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	warnings, err := store.ListModerationWarnings(ctx.GuildID, userID, limit)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to load warnings for %s: %v", userID, err), true)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, buildWarningsCommandMessage(targetUsername, warnings))
}
```

### `buildBanCommandMessage`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:874`
- Signals: `helper-func`

```go
func buildBanCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Banned **%s**. Reason: %s", targetLabel, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}
```

### `buildMassBanCommandMessage`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:886`
- Signals: `helper-func`

```go
func buildMassBanCommandMessage(banned int) string {
	return fmt.Sprintf("Banned %d user(s).", banned)
}
```

### `buildKickCommandMessage`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:890`
- Signals: `helper-func`

```go
func buildKickCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Kicked **%s**. Reason: %s", targetLabel, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}
```

### `buildMuteCommandMessage`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:902`
- Signals: `helper-func`

```go
func buildMuteCommandMessage(targetUsername string, muteRole *discordgo.Role, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Muted **%s** with **%s**. Reason: %s", targetLabel, formatRoleDisplayName(muteRole), reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}
```

### `buildTimeoutCommandMessage`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:914`
- Signals: `helper-func`

```go
func buildTimeoutCommandMessage(targetUsername string, minutes int64, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Timed out **%s** for %s. Reason: %s", targetLabel, formatTimeoutDuration(minutes), reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}
```

### `buildWarnCommandMessage`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:926`
- Signals: `helper-func`

```go
func buildWarnCommandMessage(targetUsername string, caseNumber int64, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Warned **%s**. Case #%d. Reason: %s", targetLabel, caseNumber, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}
```

### `buildWarningsCommandMessage`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:938`
- Signals: `helper-func`

```go
func buildWarningsCommandMessage(targetUsername string, warnings []storage.ModerationWarning) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	if len(warnings) == 0 {
		return fmt.Sprintf("No warnings recorded for **%s**.", targetLabel)
	}

	lines := []string{fmt.Sprintf("Recent warnings for **%s**:", targetLabel)}
	for _, warning := range warnings {
		reason := strings.TrimSpace(warning.Reason)
		if reason == "" {
			reason = "No reason provided"
		}
		createdAt := warning.CreatedAt
		if createdAt.IsZero() {
			lines = append(lines, fmt.Sprintf("#%d • by <@%s> • %s", warning.CaseNumber, warning.ModeratorID, reason))
			continue
		}
		lines = append(lines, fmt.Sprintf("#%d • <t:%d:d> • by <@%s> • %s", warning.CaseNumber, createdAt.Unix(), warning.ModeratorID, reason))
	}
	return strings.Join(lines, "\n")
}
```

### `prepareModerationContext`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:1092`
- Signals: `response:Error`

```go
func prepareModerationContext(ctx *core.Context, requiredPermission int64, actorPermissionError, botPermissionError string) (*banContext, error) {
	if ctx == nil || ctx.Session == nil {
		return nil, core.NewCommandError("Session not ready. Try again shortly.", true)
	}

	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		return nil, core.NewCommandError("Permission resolver not available.", true)
	}

	roles, err := checker.ResolveRoles(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve guild roles",
			"operation", "commands.moderation.prepare_context.resolve_roles",
			"guildID", ctx.GuildID,
			"userID", ctx.UserID,
			"err", err,
		)
		return nil, core.NewCommandError("Failed to resolve server roles.", true)
	}
	rolesByID := buildRoleIndex(roles)

	ownerID, ownerFound, err := checker.ResolveOwnerID(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve guild owner",
			"operation", "commands.moderation.prepare_context.resolve_owner",
			"guildID", ctx.GuildID,
			"userID", ctx.UserID,
			"err", err,
		)
		return nil, core.NewCommandError("Failed to resolve server owner.", true)
	}
	if !ownerFound {
		ownerID = ""
	}

	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if botID == "" {
		return nil, core.NewCommandError("Bot identity not available.", true)
	}

	var actorMember *discordgo.Member
	if ctx.Interaction != nil {
		actorMember = ctx.Interaction.Member
	}
	if actorMember == nil || actorMember.User == nil {
		var ok bool
		actorMember, ok, err = checker.ResolveMember(ctx.GuildID, ctx.UserID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Moderation context failed to resolve actor member",
				"operation", "commands.moderation.prepare_context.resolve_actor_member",
				"guildID", ctx.GuildID,
				"userID", ctx.UserID,
				"err", err,
			)
			return nil, core.NewCommandError("Unable to resolve your member record.", true)
		}
		if !ok || actorMember == nil {
			return nil, core.NewCommandError("Unable to resolve your member record.", true)
		}
	}

	botMember, ok, err := checker.ResolveMember(ctx.GuildID, botID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve bot member",
			"operation", "commands.moderation.prepare_context.resolve_bot_member",
			"guildID", ctx.GuildID,
			"botID", botID,
			"err", err,
		)
		return nil, core.NewCommandError("Unable to resolve the bot member record.", true)
	}
	if !ok || botMember == nil {
		return nil, core.NewCommandError("Unable to resolve the bot member record.", true)
	}

	actorIsOwner := ctx.IsOwner || (ownerID != "" && ctx.UserID == ownerID)
	botIsOwner := ownerID != "" && botID == ownerID

	if !actorIsOwner && !memberHasPermission(actorMember, rolesByID, ctx.GuildID, ownerID, requiredPermission) {
		return nil, core.NewCommandError(actorPermissionError, true)
	}
	if !botIsOwner && !memberHasPermission(botMember, rolesByID, ctx.GuildID, ownerID, requiredPermission) {
		return nil, core.NewCommandError(botPermissionError, true)
	}

	return &banContext{
		rolesByID:    rolesByID,
		ownerID:      ownerID,
		botID:        botID,
		actorMember:  actorMember,
		botMember:    botMember,
		actorIsOwner: actorIsOwner,
		botIsOwner:   botIsOwner,
		actorRolePos: highestRolePosition(actorMember, rolesByID, ctx.GuildID),
		botRolePos:   highestRolePosition(botMember, rolesByID, ctx.GuildID),
	}, nil
}
```

### `canModerateTarget`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:1218`
- Signals: `response:Error`

```go
func canModerateTarget(ctx *core.Context, actionCtx *banContext, targetID, actionVerb string, requireMember bool) (bool, string) {
	if targetID == ctx.UserID {
		return false, "cannot " + actionVerb + " yourself"
	}
	if targetID == actionCtx.botID {
		return false, "cannot " + actionVerb + " the bot"
	}
	if actionCtx.ownerID != "" && targetID == actionCtx.ownerID {
		return false, "cannot " + actionVerb + " the server owner"
	}

	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		if requireMember {
			return false, "target member could not be resolved right now"
		}
		return true, ""
	}

	targetMember, ok, err := checker.ResolveMember(ctx.GuildID, targetID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation target validation failed to resolve target member",
			"operation", "commands.moderation.can_moderate_target.resolve_target_member",
			"guildID", ctx.GuildID,
			"targetID", targetID,
			"action", actionVerb,
			"err", err,
		)
		if requireMember {
			return false, "target member could not be resolved right now"
		}
		return true, ""
	}
	if !ok || targetMember == nil {
		if requireMember {
			return false, "target is not a member of this server"
		}
		return true, ""
	}

	targetPos := highestRolePosition(targetMember, actionCtx.rolesByID, ctx.GuildID)
	if !actionCtx.actorIsOwner && actionCtx.actorRolePos <= targetPos {
		return false, "target has an equal or higher role than you"
	}
	if !actionCtx.botIsOwner && actionCtx.botRolePos <= targetPos {
		return false, "target has an equal or higher role than the bot"
	}
	return true, ""
}
```

### `resolveRoleTargetMember`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:1316`
- Signals: `response:Error`

```go
func resolveRoleTargetMember(ctx *core.Context, targetID string) (*discordgo.Member, bool, string) {
	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		return nil, false, "target member could not be resolved right now"
	}
	targetMember, ok, err := checker.ResolveMember(ctx.GuildID, targetID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation role action failed to resolve target member",
			"operation", "commands.moderation.resolve_role_target_member",
			"guildID", ctx.GuildID,
			"targetID", targetID,
			"err", err,
		)
		return nil, false, "target member could not be resolved right now"
	}
	if !ok || targetMember == nil {
		return nil, false, "target is not a member of this server"
	}
	return targetMember, true, ""
}
```

### `resolveUserDisplayName`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:1375`
- Signals: `response:Error`

```go
func resolveUserDisplayName(ctx *core.Context, userID string) string {
	if ctx == nil || ctx.Session == nil || userID == "" {
		return userID
	}

	checker := permissionCheckerForContext(ctx)
	if checker != nil {
		member, ok, err := checker.ResolveMember(ctx.GuildID, userID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Moderation failed to resolve display name member",
				"operation", "commands.moderation.resolve_display_name.resolve_member",
				"guildID", ctx.GuildID,
				"userID", userID,
				"err", err,
			)
		} else if ok && member != nil && member.User != nil {
			if username := strings.TrimSpace(member.User.Username); username != "" {
				return username
			}
		}
	}

	user, err := ctx.Session.User(userID)
	if err == nil && user != nil {
		if username := strings.TrimSpace(user.Username); username != "" {
			return username
		}
	}

	return userID
}
```

### `nextGuildCaseNumber`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:1474`
- Signals: `response:Error`

```go
func nextGuildCaseNumber(ctx *core.Context) (int64, bool) {
	if ctx == nil || ctx.GuildID == "" {
		return 0, false
	}
	router := ctx.Router()
	if router == nil {
		return nextFallbackCaseNumber(ctx.GuildID), true
	}
	store := router.GetStore()
	if store == nil {
		return nextFallbackCaseNumber(ctx.GuildID), true
	}

	n, err := store.NextModerationCaseNumber(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to allocate moderation case number", "guildID", ctx.GuildID, "err", err)
		return nextFallbackCaseNumber(ctx.GuildID), true
	}
	return n, true
}
```

### `sendModerationLogForEvent`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:1594`
- Signals: `response:Error`

```go
func sendModerationLogForEvent(ctx *core.Context, payload moderationLogPayload, eventType logging.LogEventType) {
	if ctx == nil || ctx.Session == nil || ctx.Config == nil || ctx.GuildID == "" {
		return
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	emit := logging.ShouldEmitLogEvent(ctx.Session, ctx.Config, eventType, ctx.GuildID)
	if !emit.Enabled {
		return
	}
	channelID := emit.ChannelID

	action := strings.TrimSpace(payload.Action)
	targetID := strings.TrimSpace(payload.TargetID)
	targetLabel := strings.TrimSpace(payload.TargetLabel)
	targetValue := "Unknown"
	switch {
	case targetID == "" && targetLabel != "":
		targetValue = targetLabel
	case targetID != "" && (targetLabel == "" || targetLabel == targetID):
		targetValue = "<@" + targetID + "> (`" + targetID + "`)"
	case targetID != "":
		targetValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, targetID, targetID)
	}
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "No reason provided"
	}
	caseID := ""
	if ctx.Interaction != nil {
		caseID = strings.TrimSpace(ctx.Interaction.ID)
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Action", Value: action, Inline: true},
	}
	if caseID != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Case ID", Value: "`" + caseID + "`", Inline: true})
	}
	fields = append(fields,
		&discordgo.MessageEmbedField{Name: "Target", Value: targetValue, Inline: true},
		&discordgo.MessageEmbedField{Name: "Actor", Value: "<@" + botID + "> (`" + botID + "`)", Inline: true},
	)
	if payload.RequestedBy != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Requested By",
			Value:  "<@" + payload.RequestedBy + "> (`" + payload.RequestedBy + "`)",
			Inline: true,
		})
	}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Reason",
		Value:  reason,
		Inline: false,
	})
	if payload.Extra != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Details",
			Value:  payload.Extra,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Moderation Action",
		Color:       theme.AutomodAction(),
		Description: fmt.Sprintf("Moderation action executed by <@%s>.", botID),
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if _, err := ctx.Session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send moderation log", "guildID", ctx.GuildID, "channelID", channelID, "action", action, "err", err)
	}
}
```

### `sendModerationCaseActionLog`

- Location: `pkg/discord/commands/moderation/moderation_commands.go:1697`
- Signals: `response:Error`

```go
func sendModerationCaseActionLog(ctx *core.Context, payload moderationLogPayload) {
	if ctx == nil || ctx.Session == nil || ctx.Config == nil || ctx.GuildID == "" {
		return
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	emit := logging.ShouldEmitLogEvent(ctx.Session, ctx.Config, logging.LogEventModerationCase, ctx.GuildID)
	if !emit.Enabled {
		return
	}
	channelID := emit.ChannelID
	caseNumber, hasCaseNumber := payload.CaseNumber, payload.HasCaseNumber
	if !hasCaseNumber || caseNumber <= 0 {
		caseNumber, hasCaseNumber = nextGuildCaseNumber(ctx)
	}

	action := strings.TrimSpace(payload.Action)
	if action == "" {
		action = "member_ban_add"
	}
	actionType := resolveModerationActionType(action)
	actionLabel, targetFieldName, detailsFieldName := resolveModerationCaseEmbedMeta(action, actionType)
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "No reason provided"
	}
	targetID := strings.TrimSpace(payload.TargetID)
	targetLabel := strings.TrimSpace(payload.TargetLabel)
	targetValue := "Unknown target"
	switch {
	case targetID == "" && targetLabel != "":
		targetValue = targetLabel
	case targetID != "" && (targetLabel == "" || targetLabel == targetID):
		targetValue = fmt.Sprintf("<@%s> (`%s`)", targetID, targetID)
	case targetID != "":
		targetValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, targetID, targetID)
	}
	actorID := strings.TrimSpace(payload.RequestedBy)
	if actorID == "" {
		actorID = botID
	}
	actorValue := fmt.Sprintf("<@%s> (`%s`)", actorID, actorID)

	eventAt := time.Now()
	eventID := strings.TrimSpace(targetID)
	if eventID == "" && ctx.Interaction != nil {
		eventID = strings.TrimSpace(ctx.Interaction.ID)
	}
	if eventID == "" {
		eventID = "unknown"
	}

	descriptionLines := []string{
		fmt.Sprintf("**%s:** %s", targetFieldName, targetValue),
		fmt.Sprintf("**Reason:** %s", reason),
		fmt.Sprintf("**Responsible moderator:** %s", actorValue),
	}
	if payload.Extra != "" {
		descriptionLines = append(descriptionLines, fmt.Sprintf("**%s:** %s", detailsFieldName, payload.Extra))
	}
	descriptionLines = append(descriptionLines, fmt.Sprintf("ID: `%s` • <t:%d:F>", eventID, eventAt.Unix()))

	embed := &discordgo.MessageEmbed{
		Title:       buildModerationCaseTitle(caseNumber, hasCaseNumber, actionLabel),
		Description: strings.Join(descriptionLines, "\n"),
		Color:       theme.AutomodAction(),
		Timestamp:   eventAt.Format(time.RFC3339),
	}

	if _, err := ctx.Session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send moderation case action log", "guildID", ctx.GuildID, "channelID", channelID, "action", action, "err", err)
	}
}
```

## pkg/discord/commands/partner/partner_commands.go

### `(*PartnerAddSubCommand).Handle`

- Location: `pkg/discord/commands/partner/partner_commands.go:121`
- Signals: `helper-call:formatPartnerEntry`, `response:Ephemeral`, `response:Success`

```go
func (c *PartnerAddSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}
	link, err := extractor.StringRequired(optionLink)
	if err != nil {
		return err
	}

	entry := files.PartnerEntryConfig{
		Fandom: extractor.String(optionFandom),
		Name:   name,
		Link:   link,
	}
	if err := c.boardService.CreatePartner(ctx.GuildID, entry); err != nil {
		if errors.Is(err, files.ErrPartnerAlreadyExists) {
			return core.NewCommandError("Partner with same name or invite already exists", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to create partner: %v", err), true)
	}

	saved, err := c.boardService.Partner(ctx.GuildID, name)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Partner created but lookup failed: %v", err), true)
	}

	content := formatPartnerEntry("Partner added", saved)
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, content)
}
```

### `(*PartnerReadSubCommand).Handle`

- Location: `pkg/discord/commands/partner/partner_commands.go:178`
- Signals: `helper-call:formatPartnerEntry`, `response:Ephemeral`, `response:Info`

```go
func (c *PartnerReadSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}

	entry, err := c.boardService.Partner(ctx.GuildID, name)
	if err != nil {
		if errors.Is(err, files.ErrPartnerNotFound) {
			return core.NewCommandError("Partner not found", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to read partner: %v", err), true)
	}

	content := formatPartnerEntry("Partner details", entry)
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, content)
}
```

### `(*PartnerUpdateSubCommand).Handle`

- Location: `pkg/discord/commands/partner/partner_commands.go:240`
- Signals: `helper-call:formatPartnerEntry`, `response:Ephemeral`, `response:Success`

```go
func (c *PartnerUpdateSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	currentName, err := extractor.StringRequired(optionCurrentName)
	if err != nil {
		return err
	}
	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}
	link, err := extractor.StringRequired(optionLink)
	if err != nil {
		return err
	}

	existing, err := c.boardService.Partner(ctx.GuildID, currentName)
	if err != nil {
		if errors.Is(err, files.ErrPartnerNotFound) {
			return core.NewCommandError("Partner not found", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to load current partner: %v", err), true)
	}

	fandom := extractor.String(optionFandom)
	if !extractor.HasOption(optionFandom) {
		fandom = existing.Fandom
	}

	entry := files.PartnerEntryConfig{
		Fandom: fandom,
		Name:   name,
		Link:   link,
	}
	if err := c.boardService.UpdatePartner(ctx.GuildID, currentName, entry); err != nil {
		if errors.Is(err, files.ErrPartnerAlreadyExists) {
			return core.NewCommandError("Another partner already uses this name or invite", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to update partner: %v", err), true)
	}

	saved, err := c.boardService.Partner(ctx.GuildID, name)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Partner updated but lookup failed: %v", err), true)
	}

	content := formatPartnerEntry("Partner updated", saved)
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, content)
}
```

### `(*PartnerDeleteSubCommand).Handle`

- Location: `pkg/discord/commands/partner/partner_commands.go:314`
- Signals: `response:Ephemeral`, `response:Success`

```go
func (c *PartnerDeleteSubCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	name, err := extractor.StringRequired(optionName)
	if err != nil {
		return err
	}

	if err := c.boardService.DeletePartner(ctx.GuildID, name); err != nil {
		if errors.Is(err, files.ErrPartnerNotFound) {
			return core.NewCommandError("Partner not found", true)
		}
		return core.NewCommandError(fmt.Sprintf("Failed to delete partner: %v", err), true)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(
		ctx.Interaction,
		fmt.Sprintf("Partner `%s` deleted", strings.TrimSpace(name)),
	)
}
```

### `(*PartnerListSubCommand).Handle`

- Location: `pkg/discord/commands/partner/partner_commands.go:352`
- Signals: `response:Ephemeral`, `response:Info`

```go
func (c *PartnerListSubCommand) Handle(ctx *core.Context) error {
	partners, err := c.boardService.ListPartners(ctx.GuildID)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to list partners: %v", err), true)
	}
	if len(partners) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, "No partners configured.")
	}

	var b strings.Builder
	b.WriteString("Configured partners:\n")
	for i, p := range partners {
		fandom := strings.TrimSpace(p.Fandom)
		if fandom == "" {
			fandom = "Other"
		}
		b.WriteString(fmt.Sprintf(
			"%d. `%s` | `%s` | %s\n",
			i+1,
			p.Name,
			fandom,
			p.Link,
		))
	}

	builder := core.NewResponseBuilder(ctx.Session).
		WithEmbed().
		WithTitle("Partner List").
		WithColor(theme.Info()).
		Ephemeral()
	return builder.Info(ctx.Interaction, strings.TrimSpace(b.String()))
}
```

### `formatPartnerEntry`

- Location: `pkg/discord/commands/partner/partner_commands.go:385`
- Signals: `helper-func`

```go
func formatPartnerEntry(prefix string, entry files.PartnerEntryConfig) string {
	fandom := strings.TrimSpace(entry.Fandom)
	if fandom == "" {
		fandom = "Other"
	}
	return strings.Join([]string{
		prefix,
		fmt.Sprintf("Name: `%s`", strings.TrimSpace(entry.Name)),
		fmt.Sprintf("Fandom: `%s`", fandom),
		fmt.Sprintf("Invite: %s", strings.TrimSpace(entry.Link)),
	}, "\n")
}
```

### `(*PartnerSyncSubCommand).Handle`

- Location: `pkg/discord/commands/partner/partner_commands.go:415`
- Signals: `response:Ephemeral`, `response:Success`

```go
func (c *PartnerSyncSubCommand) Handle(ctx *core.Context) error {
	if c.syncExecutor == nil {
		return core.NewCommandError("Partner sync is not configured", true)
	}

	syncCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := c.syncExecutor.SyncGuild(syncCtx, ctx.GuildID); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to sync partner board: %v", err), true)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(
		ctx.Interaction,
		"Partner board synced successfully.",
	)
}
```

## pkg/discord/commands/qotd/questions_list_commands.go

### `(*questionsAddCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:237`
- Signals: `helper-call:translateQuestionsMutationError`, `response:Success`

```go
func (c *questionsAddCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	body, err := extractor.StringRequired(questionsBodyOptionName)
	if err != nil {
		return err
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}

	created, err := c.service.CreateQuestion(context.Background(), ctx.GuildID, ctx.UserID, applicationqotd.QuestionMutation{
		DeckID: deck.ID,
		Body:   body,
	})
	if err != nil {
		return translateQuestionsMutationError(err)
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("Added QOTD question ID %d to deck `%s`.", visibleQuestionID(*created), deck.Name))
}
```

### `(*questionsResetCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:399`
- Signals: `helper-call:describeResetDeckResult`, `helper-call:translateQuestionsMutationError`, `response:Info`, `response:Success`

```go
func (c *questionsResetCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}

	result, err := c.service.ResetDeckState(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return translateQuestionsMutationError(err)
	}
	if result.QuestionsReset == 0 && result.OfficialPostsCleared == 0 {
		return core.NewResponseBuilder(ctx.Session).
			Info(ctx.Interaction, fmt.Sprintf("No QOTD question states or publish history needed reset in deck `%s`. Question order was unchanged.", deck.Name))
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, describeResetDeckResult(result, deck.Name))
}
```

### `(*questionsImportCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:423`
- Signals: `helper-call:describeQuestionsImportError`, `helper-call:describeQuestionsImportResult`, `helper-call:translateQuestionsImportError`, `response:EditResponse`

```go
func (c *questionsImportCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	rawUserIDs, err := extractor.StringRequired(questionsImportUsersName)
	if err != nil {
		return err
	}
	authorIDs, err := parseQuestionsImportAuthorIDs(rawUserIDs)
	if err != nil {
		return err
	}
	channelID := questionsChannelOptionID(ctx.Session, core.GetSubCommandOptions(ctx.Interaction), questionsImportChannel)
	if channelID == "" {
		return core.NewCommandError("Channel is required.", false)
	}
	startDate, err := extractor.StringRequired(questionsImportStartDate)
	if err != nil {
		return err
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}

	rm := core.NewResponseManager(ctx.Session)
	if err := rm.DeferResponse(ctx.Interaction, false); err != nil {
		return err
	}

	result, err := c.service.ImportArchivedQuestions(context.Background(), ctx.GuildID, ctx.UserID, ctx.Session, applicationqotd.ImportArchivedQuestionsParams{
		DeckID:          deck.ID,
		SourceChannelID: channelID,
		AuthorIDs:       authorIDs,
		StartDate:       startDate,
		BackupDir:       defaultQuestionsImportBackupDir(),
	})
	if err != nil {
		return rm.EditResponse(ctx.Interaction, describeQuestionsImportError(translateQuestionsImportError(err)))
	}
	if result.MatchedMessages == 0 {
		return rm.EditResponse(ctx.Interaction, fmt.Sprintf("No historical QOTD questions matched in <#%s> since %s for deck `%s`.", channelID, startDate, deck.Name))
	}

	return rm.EditResponse(ctx.Interaction, describeQuestionsImportResult(deck.Name, channelID, result))
}
```

### `describeQuestionsImportError`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:472`
- Signals: `helper-func`, `response:Error`

```go
func describeQuestionsImportError(err error) string {
	if err == nil {
		return "An error occurred while importing historical QOTD questions."
	}
	var cmdErr *core.CommandError
	if errors.As(err, &cmdErr) && cmdErr != nil && strings.TrimSpace(cmdErr.Message) != "" {
		return cmdErr.Message
	}
	return err.Error()
}
```

### `(*questionsQueueCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:483`
- Signals: `helper-call:formatAutomaticQueueState`, `helper-call:translateQuestionsMutationError`, `response:Info`

```go
func (c *questionsQueueCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	state, err := c.service.GetAutomaticQueueState(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return translateQuestionsMutationError(err)
	}

	return core.NewResponseBuilder(ctx.Session).
		Info(ctx.Interaction, formatAutomaticQueueState(state))
}
```

### `(*questionsNextCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:502`
- Signals: `helper-call:translateQuestionsSetNextError`, `response:Info`, `response:Success`

```go
func (c *questionsNextCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	displayID := extractor.Int(questionsIDOptionName)
	if displayID <= 0 {
		return core.NewCommandError("Question ID must be greater than zero.", false)
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return err
	}
	question := findQuestionByDisplayID(questions, displayID)
	if question == nil {
		return translateQuestionsSetNextError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	updated, err := c.service.SetNextQuestion(context.Background(), ctx.GuildID, deck.ID, question.ID)
	if err != nil {
		return translateQuestionsSetNextError(displayID, err)
	}
	if updated == nil {
		return translateQuestionsSetNextError(displayID, applicationqotd.ErrQuestionNotFound)
	}
	if visibleQuestionID(*updated) == displayID {
		return core.NewResponseBuilder(ctx.Session).
			Info(ctx.Interaction, fmt.Sprintf("QOTD question ID %d is already the next ready question in deck `%s`.", displayID, deck.Name))
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("QOTD question ID %d is now the next ready question in deck `%s` and is now listed as ID %d.", displayID, deck.Name, visibleQuestionID(*updated)))
}
```

### `(*qotdPublishCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:552`
- Signals: `helper-call:translatePublishNowError`, `response:Success`

```go
func (c *qotdPublishCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	deck, err := loadCommandDeck(ctx, c.service, "")
	if err != nil {
		return err
	}
	if !deck.Enabled {
		return core.NewCommandError("Enable QOTD publishing for the active deck before publishing manually.", false)
	}
	if strings.TrimSpace(deck.ChannelID) == "" {
		return core.NewCommandError("Set a QOTD channel for the active deck before publishing manually.", false)
	}

	result, err := c.service.PublishNow(context.Background(), ctx.GuildID, ctx.Session)
	if err != nil {
		return translatePublishNowError(err)
	}

	message := fmt.Sprintf("Published QOTD question ID %d manually.", visibleQuestionID(result.Question))
	if postURL := strings.TrimSpace(result.PostURL); postURL != "" {
		message = fmt.Sprintf("%s %s", message, postURL)
	}
	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, message)
}
```

### `(*questionsRemoveCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:581`
- Signals: `helper-call:translateQuestionsDeleteError`, `response:Success`

```go
func (c *questionsRemoveCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	displayID := extractor.Int(questionsIDOptionName)
	if displayID <= 0 {
		return core.NewCommandError("Question ID must be greater than zero.", false)
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return err
	}
	question := findQuestionByDisplayID(questions, displayID)
	if question == nil {
		return translateQuestionsDeleteError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	if err := c.service.DeleteQuestion(context.Background(), ctx.GuildID, question.ID); err != nil {
		return translateQuestionsDeleteError(displayID, err)
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("Removed QOTD question ID %d from deck `%s`.", displayID, deck.Name))
}
```

### `(*questionsRecoverCommand).Handle`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:612`
- Signals: `helper-call:translateQuestionsRecoverError`, `response:Success`

```go
func (c *questionsRecoverCommand) Handle(ctx *core.Context) error {
	if err := requireQuestionsGuild(ctx); err != nil {
		return err
	}

	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))
	displayID := extractor.Int(questionsIDOptionName)
	if displayID <= 0 {
		return core.NewCommandError("Question ID must be greater than zero.", false)
	}
	deck, err := loadCommandDeck(ctx, c.service, extractor.String(questionsDeckOptionName))
	if err != nil {
		return err
	}
	questions, err := c.service.ListQuestions(context.Background(), ctx.GuildID, deck.ID)
	if err != nil {
		return err
	}
	question := findQuestionByDisplayID(questions, displayID)
	if question == nil {
		return translateQuestionsRecoverError(displayID, applicationqotd.ErrQuestionNotFound)
	}

	updated, err := c.service.RestoreUsedQuestion(context.Background(), ctx.GuildID, deck.ID, question.ID)
	if err != nil {
		return translateQuestionsRecoverError(displayID, err)
	}
	if updated == nil {
		return translateQuestionsRecoverError(displayID, applicationqotd.ErrQuestionNotFound)
	}
	if visibleQuestionID(*updated) == displayID {
		return core.NewResponseBuilder(ctx.Session).
			Success(ctx.Interaction, fmt.Sprintf("Recovered QOTD question ID %d from used to ready in deck `%s`.", displayID, deck.Name))
	}

	return core.NewResponseBuilder(ctx.Session).
		Success(ctx.Interaction, fmt.Sprintf("Recovered QOTD question ID %d from used to ready in deck `%s` and it is now listed as ID %d.", displayID, deck.Name, visibleQuestionID(*updated)))
}
```

### `(*questionsListCommand).HandleComponent`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:675`
- Signals: `response:Ephemeral`, `response:Error`

```go
func (c *questionsListCommand) HandleComponent(ctx *core.Context) error {
	if ctx == nil || ctx.Interaction == nil {
		return nil
	}
	action, state, err := parseQuestionsListState(ctx.RouteKey.CustomID)
	if err != nil {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, "Invalid questions list action.")
	}
	if strings.TrimSpace(ctx.UserID) != state.UserID {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, questionsListDeniedText)
	}

	view, err := c.loadView(ctx, state.DeckID)
	if err != nil {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, err.Error())
	}

	totalPages := discordqotdBuildPageCount(len(view.questions))
	state.Page = nextQuestionsListPage(action, state.Page, totalPages)
	if err := respondQuestionsList(ctx, view, state, false, false); err != nil {
		return err
	}
	c.armQuestionsListIdleTimeoutForMessage(ctx)
	return nil
}
```

### `sendQuestionsListResponse`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:790`
- Signals: `response:Custom`, `response:Ephemeral`, `response:InteractionRespond`

```go
func sendQuestionsListResponse(
	ctx *core.Context,
	embed *discordgo.MessageEmbed,
	components []discordgo.MessageComponent,
	ephemeral bool,
	initial bool,
) error {
	if initial {
		builder := core.NewResponseBuilder(ctx.Session).WithComponents(components...)
		if ephemeral {
			builder = builder.Ephemeral()
		}
		return builder.Build().Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
	}

	return ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     []*discordgo.MessageEmbed{embed},
			Components: components,
		},
	})
}
```

### `describeQuestionsImportResult`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1068`
- Signals: `helper-func`

```go
func describeQuestionsImportResult(deckName, channelID string, result applicationqotd.ImportArchivedQuestionsResult) string {
	parts := []string{fmt.Sprintf("Scanned %d messages in <#%s> and matched %d historical QOTD prompts for deck `%s`.", result.ScannedMessages, channelID, result.MatchedMessages, deckName)}
	parts = append(parts, fmt.Sprintf("Imported %s as used history.", formatCountNoun(result.ImportedQuestions, "historical QOTD question", "historical QOTD questions")))
	if result.DeletedQuestions > 0 {
		parts = append(parts, fmt.Sprintf("Removed %s duplicate questions from the current queue.", formatCountNoun(result.DeletedQuestions, "duplicate question", "duplicate questions")))
	} else if result.DuplicateQuestions > 0 {
		parts = append(parts, fmt.Sprintf("Found %s already locked in history.", formatCountNoun(result.DuplicateQuestions, "duplicate question", "duplicate questions")))
	}
	if result.StoredQuestions > 0 {
		parts = append(parts, fmt.Sprintf("Stored %s in local collector history.", formatCountNoun(result.StoredQuestions, "historical message", "historical messages")))
	}
	if backupPath := displayQuestionsImportBackupPath(result.BackupPath); backupPath != "" {
		parts = append(parts, fmt.Sprintf("Local backup: `%s`.", backupPath))
	}
	return strings.Join(parts, " ")
}
```

### `formatAutomaticQueueState`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1085`
- Signals: `helper-func`

```go
func formatAutomaticQueueState(state applicationqotd.AutomaticQueueState) string {
	deckName := strings.TrimSpace(state.Deck.Name)
	if deckName == "" {
		deckName = "Default"
	}
	lines := []string{fmt.Sprintf("Automatic QOTD queue for deck `%s`.", deckName)}

	if !state.ScheduleConfigured {
		lines = append(lines, "Automatic publish schedule is not configured.")
	} else {
		lines = append(lines, fmt.Sprintf("Automatic schedule: %s UTC.", formatAutomaticQueueSchedule(state.Schedule)))
		lines = append(lines, fmt.Sprintf("Current automatic slot: %s (%s).", formatAutomaticQueueTimestamp(state.SlotPublishAtUTC), formatAutomaticQueueSlotStatus(state.SlotStatus)))
	}

	if !state.Deck.Enabled {
		lines = append(lines, "Publishing is disabled for this deck.")
	} else if strings.TrimSpace(state.Deck.ChannelID) == "" {
		lines = append(lines, "Set a QOTD channel before automatic publishing can run.")
	}

	if state.SlotQuestion != nil {
		lines = append(lines, fmt.Sprintf("Current automatic slot question: %s.", formatAutomaticQueueQuestion(*state.SlotQuestion)))
	}

	if state.NextReadyQuestion != nil {
		label := "Next automatic question"
		if state.SlotQuestion != nil || state.SlotStatus == applicationqotd.AutomaticQueueSlotStatusPublished {
			label = "After that"
		}
		lines = append(lines, fmt.Sprintf("%s: %s.", label, formatAutomaticQueueQuestion(*state.NextReadyQuestion)))
	} else if state.SlotQuestion == nil {
		lines = append(lines, "No ready QOTD questions are available for the automatic queue.")
	}

	return strings.Join(lines, "\n")
}
```

### `describeResetDeckResult`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1164`
- Signals: `helper-func`

```go
func describeResetDeckResult(result applicationqotd.ResetDeckResult, deckName string) string {
	parts := make([]string, 0, 2)
	if result.QuestionsReset > 0 {
		parts = append(parts, fmt.Sprintf("reset %s", formatCountNoun(result.QuestionsReset, "QOTD question state", "QOTD question states")))
	}
	if result.OfficialPostsCleared > 0 {
		parts = append(parts, fmt.Sprintf("cleared %s", formatCountNoun(result.OfficialPostsCleared, "QOTD publish record", "QOTD publish records")))
	}
	if len(parts) == 0 {
		message := fmt.Sprintf("No QOTD question states or publish history needed reset in deck `%s`. Question order was unchanged.", deckName)
		if result.SuppressedCurrentSlotAutomaticPublish {
			message += " Automatic publishing for the current slot is paused until you publish manually."
		}
		return message
	}
	message := fmt.Sprintf("%s in deck `%s`. Question order was preserved.", strings.Join(parts, " and "), deckName)
	if result.SuppressedCurrentSlotAutomaticPublish {
		message += " Automatic publishing for the current slot is paused until you publish manually."
	}
	return message
}
```

### `translateQuestionsMutationError`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1202`
- Signals: `helper-func`, `response:Error`

```go
func translateQuestionsMutationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, files.ErrInvalidQOTDInput) {
		message := strings.TrimSpace(strings.TrimPrefix(err.Error(), files.ErrInvalidQOTDInput.Error()+":"))
		if message == "" {
			message = "Invalid QOTD question input"
		}
		return core.NewCommandError(message, false)
	}
	return err
}
```

### `translateQuestionsDeleteError`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1216`
- Signals: `helper-call:translateQuestionsMutationError`, `helper-func`

```go
func translateQuestionsDeleteError(questionID int64, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotFound) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d was not found.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrImmutableQuestion) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d is already scheduled or used and cannot be removed.", questionID), false)
	}
	return translateQuestionsMutationError(err)
}
```

### `translateQuestionsSetNextError`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1229`
- Signals: `helper-call:translateQuestionsMutationError`, `helper-func`

```go
func translateQuestionsSetNextError(questionID int64, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotFound) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d was not found.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrImmutableQuestion) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d is already scheduled or used and cannot be set as next.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotReady) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d must be ready before it can be set as next.", questionID), false)
	}
	return translateQuestionsMutationError(err)
}
```

### `translateQuestionsRecoverError`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1245`
- Signals: `helper-call:translateQuestionsMutationError`, `helper-func`

```go
func translateQuestionsRecoverError(questionID int64, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotFound) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d was not found.", questionID), false)
	}
	if errors.Is(err, applicationqotd.ErrQuestionNotUsed) {
		return core.NewCommandError(fmt.Sprintf("QOTD question ID %d is not used and cannot be recovered.", questionID), false)
	}
	return translateQuestionsMutationError(err)
}
```

### `translateQuestionsImportError`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1258`
- Signals: `helper-call:translateQuestionsMutationError`, `helper-func`

```go
func translateQuestionsImportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrDiscordUnavailable) {
		return core.NewCommandError("Discord session unavailable for QOTD history import.", false)
	}
	return translateQuestionsMutationError(err)
}
```

### `translatePublishNowError`

- Location: `pkg/discord/commands/qotd/questions_list_commands.go:1268`
- Signals: `helper-func`

```go
func translatePublishNowError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, applicationqotd.ErrAlreadyPublished) {
		return core.NewCommandError("A QOTD question has already been published for the current slot.", false)
	}
	if errors.Is(err, applicationqotd.ErrPublishInProgress) {
		return core.NewCommandError("A QOTD publish is already in progress for the current slot.", false)
	}
	if errors.Is(err, applicationqotd.ErrNoQuestionsAvailable) {
		return core.NewCommandError("No ready QOTD questions are available in the active deck.", false)
	}
	if errors.Is(err, applicationqotd.ErrQOTDDisabled) {
		return core.NewCommandError("Enable QOTD publishing and set a channel before publishing manually.", false)
	}
	if errors.Is(err, applicationqotd.ErrDiscordUnavailable) {
		return core.NewCommandError("Discord session unavailable for manual publish.", false)
	}
	return err
}
```

## pkg/discord/commands/runtime/runtime_config_commands.go

### `(*runtimeSubCommand).Handle`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:445`
- Signals: `helper-call:renderMainEmbed`, `response:Custom`, `response:Ephemeral`, `response:Error`

```go
func (c *runtimeSubCommand) Handle(ctx *core.Context) error {
	rc, err := loadRuntimeConfig(ctx.Config, "global")
	if err != nil {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, fmt.Sprintf("Failed to load runtime config: %v", err))
	}

	st := panelState{
		Mode:  pageMain,
		Group: "ALL",
		Scope: "global",
	}

	if ctx.Interaction.GuildID != "" {
		st.Scope = ctx.Interaction.GuildID
		// Try to load guild config, if fails or empty, we still have the global one as base
		if grc, err := loadRuntimeConfig(ctx.Config, st.Scope); err == nil {
			rc = grc
		}
	}

	embed := renderMainEmbed(rc, st)
	components := renderMainComponents(rc, st)

	rm := core.NewResponseBuilder(ctx.Session).Build()
	cfg := core.ResponseConfig{
		Ephemeral:  runtimeVisibilityIsEphemeral(runtimeVisibilityAdministrativePanel),
		WithEmbed:  true,
		Title:      embed.Title,
		Color:      embed.Color,
		Timestamp:  true,
		Components: components,
		Footer:     embed.Footer.Text,
	}
	return rm.WithConfig(cfg).Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}
```

### `renderMainEmbed`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:787`
- Signals: `helper-func`, `response:Info`

```go
func renderMainEmbed(rc files.RuntimeConfig, st panelState) *discordgo.MessageEmbed {
	sp, _ := specByKey(st.Key)

	scopeDesc := "Global"
	if st.Scope != "global" {
		scopeDesc = fmt.Sprintf("Guild (`%s`)", st.Scope)
	}

	desc := strings.Join([]string{
		"Painel para editar **runtime_config** (substitui as env vars operacionais).",
		"",
		fmt.Sprintf("Escopo: **%s**", scopeDesc),
		fmt.Sprintf("Selecionada: `%s` • Tipo: **%s** • Default: **%s** • %s", sp.Key, sp.Type, sp.DefaultHint, sp.RestartHint),
		"Use os menus para filtrar e navegar, e os botões para editar.",
	}, "\n")

	fields := []*discordgo.MessageEmbedField{}
	fields = append(fields, groupFieldsForMain(rc, st)...)

	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME)",
		Description: desc,
		Color:       theme.Info(),
		Fields:      fields,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Dica: alterações podem ser aplicadas em tempo real para THEME e alguns ALICE_DISABLE_*.",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
```

### `renderDetailsEmbed`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:911`
- Signals: `helper-call:errorEmbed`, `helper-func`

```go
func renderDetailsEmbed(rc files.RuntimeConfig, st panelState) *discordgo.MessageEmbed {
	sp, ok := specByKey(st.Key)
	if !ok {
		return errorEmbed("Unknown key")
	}
	raw, _ := getValue(rc, sp.Key)
	cur := formatForDetails(raw, sp)

	scopeDesc := "Global"
	if st.Scope != "global" {
		scopeDesc = fmt.Sprintf("Guild (`%s`)", st.Scope)
	}

	lines := []string{
		fmt.Sprintf("`%s`", sp.Key),
		"",
		fmt.Sprintf("**Scope:** %s", scopeDesc),
		fmt.Sprintf("**Group:** %s", sp.Group),
		fmt.Sprintf("**Type:** %s", sp.Type),
		fmt.Sprintf("**Default:** %s", sp.DefaultHint),
		fmt.Sprintf("**Current:** %s", cur),
		"",
		fmt.Sprintf("**Description:** %s", sp.ShortHelp),
		fmt.Sprintf("**Effect:** %s", sp.RestartHint),
	}

	if sp.GuildOnly {
		lines = append(lines, "", "⚠️ **Note:** This setting can only be configured per-guild.")
	}

	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME) — DETAILS",
		Description: strings.Join(lines, "\n"),
		Color:       theme.Muted(),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use BACK to return to the panel.",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}
}
```

### `renderHelpEmbed`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:952`
- Signals: `helper-func`, `response:Info`

```go
func renderHelpEmbed() *discordgo.MessageEmbed {
	desc := strings.Join([]string{
		"This panel edits the persisted `runtime_config`.",
		"",
		"**Notes:**",
		"• Names stay in ALL CAPS to preserve mental compatibility with env vars.",
		"• The bot no longer reads these options from the environment (the token is still env).",
		"• Some changes can be hot-applied (THEME and some ALICE_DISABLE_*).",
		"",
		"**How to edit:**",
		"1) Filter by group (optional) and select a key.",
		"2) Boolean: use TOGGLE.",
		"3) Other types: use EDIT and fill the modal.",
		"4) RESET clears the value and restores the code default.",
	}, "\n")

	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME) — HELP",
		Description: desc,
		Color:       theme.Info(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
```

### `handleComponent`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:1199`
- Signals: `helper-call:errorEmbed`, `helper-call:renderDetailsEmbed`, `helper-call:renderHelpEmbed`, `helper-call:renderMainEmbed`, `helper-call:withHotApplyWarning`

```go
func handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier runtimeConfigApplier) {
	cc := i.MessageComponentData()
	routeID, _, _ := strings.Cut(cc.CustomID, stateSep)
	ackPolicy := runtimeComponentAckPolicy(routeID)

	action, st := parseActionAndState(cc.CustomID)
	if action == "" {
		if ackPolicy.Mode == core.InteractionAckModeNone {
			respondInteractionWithLog(s, i, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{errorEmbed("Invalid interaction state")},
				},
			}, routeID+".invalid_state.respond_error")
			return
		}

		editInteractionMessageWithLog(s, i, errorEmbed("Invalid interaction state"), nil, routeID+".invalid_state.render_error")
		return
	}

	respond := func(resp *discordgo.InteractionResponse, stage string) {
		respondInteractionWithLog(s, i, resp, action+"."+stage)
	}
	edit := func(embed *discordgo.MessageEmbed, components []discordgo.MessageComponent, stage string) {
		editInteractionMessageWithLog(s, i, embed, components, action+"."+stage)
	}

	rc, err := loadRuntimeConfig(configManager, st.Scope)
	if err != nil {
		if ackPolicy.Mode == core.InteractionAckModeNone {
			respond(&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)),
					},
				},
			}, "load_runtime_config_error")
			return
		}
		edit(errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)), nil, "load_runtime_config_error")
		return
	}

	// Guard: enforce restrictions
	if sp, ok := specByKey(st.Key); ok {
		if sp.GuildOnly && st.Scope == "global" {
			// Skip editing if global
			if action == cidButtonEdit || action == cidButtonToggle || action == cidButtonReset {
				edit(errorEmbed("This setting can only be configured per-guild."), renderMainComponents(rc, st), "guild_only_restriction")
				return
			}
		}
	}

	switch action {
	case cidSelectGroup, cidSelectKey:
		if len(cc.Values) == 0 {
			embed := renderMainEmbed(rc, st.withMode(pageMain))
			edit(embed, renderMainComponents(rc, st.withMode(pageMain)), "select.empty_values")
			return
		}
		// The value in the select menu options is the full encoded state.
		st = decodeState(cc.Values[0])
		if refreshed, loadErr := loadRuntimeConfig(configManager, st.Scope); loadErr == nil {
			rc = refreshed
		} else {
			slog.Warn("Runtime config panel failed to refresh state after selection",
				"action", action,
				"scope", st.Scope,
				"key", string(st.Key),
				"err", loadErr,
			)
		}
		st = ensureKeyInGroup(st.withMode(pageMain))
		embed := renderMainEmbed(rc, st)
		edit(embed, renderMainComponents(rc, st), "select.apply_state")
		return

	case cidButtonMain, cidButtonBack:
		st = st.withMode(pageMain)
		st = ensureKeyInGroup(st)
		embed := renderMainEmbed(rc, st)
		edit(embed, renderMainComponents(rc, st), "nav.main")
		return

	case cidButtonHelp:
		st = st.withMode(pageHelp)
		embed := renderHelpEmbed()
		edit(embed, renderHelpComponents(st), "nav.help")
		return

	case cidButtonDetail:
		st = st.withMode(pageDetail)
		embed := renderDetailsEmbed(rc, st)
		edit(embed, renderDetailComponents(st), "nav.detail")
		return

	case cidButtonReload:
		if refreshed, loadErr := loadRuntimeConfig(configManager, st.Scope); loadErr == nil {
			rc = refreshed
		} else {
			slog.Warn("Runtime config panel failed to reload from storage",
				"action", action,
				"scope", st.Scope,
				"key", string(st.Key),
				"err", loadErr,
			)
		}
		st = ensureKeyInGroup(st)
		switch st.Mode {
		case pageHelp:
			embed := renderHelpEmbed()
			edit(embed, renderHelpComponents(st), "reload.help")
		case pageDetail:
			embed := renderDetailsEmbed(rc, st)
			edit(embed, renderDetailComponents(st), "reload.detail")
		default:
			embed := renderMainEmbed(rc, st.withMode(pageMain))
			edit(embed, renderMainComponents(rc, st.withMode(pageMain)), "reload.main")
		}
		return

	case cidButtonReset:
		st = st.withMode(pageMain)
		rc2, ok := resetValue(rc, st.Key)
		if !ok {
			edit(errorEmbed("Unknown key"), nil, "reset.unknown_key")
			return
		}
		if err := saveRuntimeConfig(configManager, rc2, st.Scope); err != nil {
			edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "reset.save_error")
			return
		}
		applyErr := applyRuntimeConfigWithLog(applier, rc2, i, action+".reset.hot_apply", st)
		embed := renderMainEmbed(rc2, st)
		embed = withHotApplyWarning(embed, applyErr)
		edit(embed, renderMainComponents(rc2, st), "reset.render")
		return

	case cidButtonToggle:
		st = st.withMode(pageMain)
		sp, ok := specByKey(st.Key)
		if !ok {
			edit(errorEmbed("Unknown key"), nil, "toggle.unknown_key")
			return
		}
		if sp.Type != vtBool {
			edit(errorEmbed("TOGGLE is only valid for boolean keys"), renderMainComponents(rc, st), "toggle.invalid_type")
			return
		}
		rc2, err := toggleBool(rc, st.Key)
		if err != nil {
			edit(errorEmbed(fmt.Sprintf("Toggle failed: %v", err)), renderMainComponents(rc, st), "toggle.failed")
			return
		}
		if err := saveRuntimeConfig(configManager, rc2, st.Scope); err != nil {
			edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "toggle.save_error")
			return
		}
		applyErr := applyRuntimeConfigWithLog(applier, rc2, i, action+".toggle.hot_apply", st)
		embed := renderMainEmbed(rc2, st)
		embed = withHotApplyWarning(embed, applyErr)
		edit(embed, renderMainComponents(rc2, st), "toggle.render")
		return

	case cidButtonEdit:
		sp, ok := specByKey(st.Key)
		if !ok {
			// This interaction path normally opens a modal, so we intentionally do NOT
			// ack with a message update earlier. If we hit an error, we must still
			// respond once to avoid an "interaction failed" on the client.
			respond(&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed("Unknown key"),
					},
				},
			}, "edit.unknown_key")
			return
		}
		if sp.Type == vtBool {
			respond(&discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						errorEmbed("EDIT is not valid for boolean keys (use TOGGLE)"),
					},
				},
			}, "edit.invalid_type")
			return
		}

		cur, _ := getValue(rc, st.Key)
		if strings.TrimSpace(cur) == "" {
			cur = ""
		}
		if sp.Type == vtInt && strings.TrimSpace(cur) == "0" {
			cur = ""
		}

		maxLen := sp.MaxInputLen
		if maxLen <= 0 {
			maxLen = 200
		}
		label := fmt.Sprintf("%s (%s)", sp.Key, sp.Type)

		respond(&discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseModal,
			Data: &discordgo.InteractionResponseData{
				CustomID: encodeRuntimeModalState(st, runtimeInteractionUserID(i)),
				Title:    string(sp.Key),
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.TextInput{
								CustomID:    modalFieldValue,
								Label:       label,
								Style:       discordgo.TextInputShort,
								Placeholder: sp.DefaultHint,
								Value:       cur,
								Required:    false,
								MinLength:   0,
								MaxLength:   maxLen,
							},
						},
					},
				},
			},
		}, "edit.open_modal")
		return

	default:
		edit(errorEmbed("Unknown action"), nil, "unknown_action")
		return
	}
}
```

### `handleModalSubmit`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:1443`
- Signals: `helper-call:errorEmbed`, `helper-call:renderMainEmbed`, `helper-call:withHotApplyWarning`

```go
func handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate, configManager *files.ConfigManager, applier runtimeConfigApplier) {
	m := i.ModalSubmitData()
	st, _, ok := decodeRuntimeModalState(m.CustomID)
	if !ok {
		return
	}

	edit := func(embed *discordgo.MessageEmbed, components []discordgo.MessageComponent, stage string) {
		editInteractionMessageWithLog(s, i, embed, components, "modal_submit."+stage)
	}

	sp, ok := specByKey(st.Key)
	if !ok {
		embed := errorEmbed("Unknown key")
		edit(embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)), "unknown_key")
		return
	}
	if sp.Type == vtBool {
		embed := errorEmbed("Invalid modal for bool key")
		edit(embed, renderMainComponents(files.RuntimeConfig{}, st.withMode(pageMain)), "invalid_bool_key")
		return
	}

	val := extractModalValue(m, modalFieldValue)

	rc, err := loadRuntimeConfig(configManager, st.Scope)
	if err != nil {
		edit(errorEmbed(fmt.Sprintf("Failed to load runtime config: %v", err)), nil, "load_runtime_config_error")
		return
	}

	next, err := setValue(rc, sp, val)
	if err != nil {
		embed := errorEmbed(fmt.Sprintf("Invalid value: %v", err))
		st = ensureKeyInGroup(st.withMode(pageMain))
		edit(embed, renderMainComponents(rc, st), "invalid_value")
		return
	}
	if err := saveRuntimeConfig(configManager, next, st.Scope); err != nil {
		edit(errorEmbed(fmt.Sprintf("Failed to save: %v", err)), nil, "save_error")
		return
	}

	applyErr := applyRuntimeConfigWithLog(applier, next, i, "modal_submit.hot_apply", st)

	// After saving, return to MAIN with refreshed values so the user can keep navigating.
	st = ensureKeyInGroup(st.withMode(pageMain))
	embed := renderMainEmbed(next, st)
	embed = withHotApplyWarning(embed, applyErr)
	edit(embed, renderMainComponents(next, st), "render")
}
```

### `respondInteractionWithLog`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:1525`
- Signals: `response:Error`, `response:InteractionRespond`

```go
func respondInteractionWithLog(s *discordgo.Session, i *discordgo.InteractionCreate, resp *discordgo.InteractionResponse, reason string) {
	if s == nil || i == nil || i.Interaction == nil {
		slog.Error("Runtime config interaction respond skipped due to missing context", "reason", reason)
		return
	}
	if err := s.InteractionRespond(i.Interaction, resp); err != nil {
		fields := []any{"reason", reason, "err", err}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config interaction respond failed", fields...)
	}
}
```

### `editInteractionMessageWithLog`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:1537`
- Signals: `response:Error`

```go
func editInteractionMessageWithLog(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	embed *discordgo.MessageEmbed,
	components []discordgo.MessageComponent,
	reason string,
) {
	if s == nil || i == nil || i.Interaction == nil {
		slog.Error("Runtime config interaction edit skipped due to missing context", "reason", reason)
		return
	}
	if err := editInteractionMessage(s, i, embed, components); err != nil {
		fields := []any{"reason", reason, "err", err}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config interaction edit failed", fields...)
	}
}
```

### `applyRuntimeConfigWithLog`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:1555`
- Signals: `response:Error`

```go
func applyRuntimeConfigWithLog(
	applier runtimeConfigApplier,
	next files.RuntimeConfig,
	i *discordgo.InteractionCreate,
	reason string,
	st panelState,
) error {
	if applier == nil {
		return nil
	}

	if err := applier.Apply(context.Background(), next); err != nil {
		fields := []any{
			"reason", reason,
			"scope", st.Scope,
			"key", string(st.Key),
			"err", err,
		}
		fields = append(fields, interactionLogFields(i)...)
		slog.Error("Runtime config hot-apply failed", fields...)
		return err
	}
	return nil
}
```

### `withHotApplyWarning`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:1580`
- Signals: `helper-func`

```go
func withHotApplyWarning(embed *discordgo.MessageEmbed, applyErr error) *discordgo.MessageEmbed {
	if embed == nil || applyErr == nil {
		return embed
	}

	clone := *embed
	msg := fmt.Sprintf(
		"Saved runtime config, but failed to apply changes immediately. Restart may be required.\nError: %v",
		applyErr,
	)
	if strings.TrimSpace(clone.Description) == "" {
		clone.Description = msg
	} else {
		clone.Description = strings.TrimSpace(clone.Description) + "\n\n" + msg
	}
	return &clone
}
```

### `errorEmbed`

- Location: `pkg/discord/commands/runtime/runtime_config_commands.go:1684`
- Signals: `helper-func`, `response:Error`

```go
func errorEmbed(msg string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "CONFIG (RUNTIME) — ERROR",
		Description: msg,
		Color:       theme.Error(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}
```

## pkg/discord/commands/runtime/runtime_config_policy.go

### `denyRuntimeInteraction`

- Location: `pkg/discord/commands/runtime/runtime_config_policy.go:164`
- Signals: `response:Ephemeral`, `response:Error`, `response:FollowUp`

```go
func denyRuntimeInteraction(ctx *core.Context, ackPolicy core.InteractionAckPolicy, message string) error {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
		return nil
	}
	if ackPolicy.Mode != core.InteractionAckModeNone {
		return core.NewResponseManager(ctx.Session).FollowUp(ctx.Interaction, message, true)
	}
	return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, message)
}
```

