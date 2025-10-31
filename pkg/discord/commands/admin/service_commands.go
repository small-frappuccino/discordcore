package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// AdminCommands provides administrative commands for service management
type AdminCommands struct {
	serviceManager *service.ServiceManager
	unifiedCache   *cache.UnifiedCache
}

// NewAdminCommands creates a new admin commands handler
func NewAdminCommands(serviceManager *service.ServiceManager, unifiedCache *cache.UnifiedCache) *AdminCommands {
	return &AdminCommands{
		serviceManager: serviceManager,
		unifiedCache:   unifiedCache,
	}
}

// RegisterCommands registers all admin commands with the router
func (ac *AdminCommands) RegisterCommands(router *core.CommandRouter) {
	// Main admin command with subcommands
	adminCmd := core.NewGroupCommand(
		"admin",
		"Administrative commands for bot management",

		core.NewPermissionChecker(router.GetSession(), router.GetConfigManager()),
	)

	// Service management subcommands
	adminCmd.AddSubCommand(ac.createMetricsCommand())
	adminCmd.AddSubCommand(ac.createMetricsWatchCommand())
	adminCmd.AddSubCommand(ac.createServiceStatusCommand())
	adminCmd.AddSubCommand(ac.createServiceListCommand())
	adminCmd.AddSubCommand(ac.createServiceRestartCommand())
	adminCmd.AddSubCommand(ac.createHealthCheckCommand())

	router.RegisterCommand(adminCmd)
}

// createServiceStatusCommand creates the service status subcommand
func (ac *AdminCommands) createServiceStatusCommand() core.SubCommand {
	return &ServiceStatusCommand{
		adminCommands: ac,
	}
}

// createServiceListCommand creates the service list subcommand
func (ac *AdminCommands) createServiceListCommand() core.SubCommand {
	return &ServiceListCommand{
		adminCommands: ac,
	}
}

// createServiceRestartCommand creates the service restart subcommand
func (ac *AdminCommands) createServiceRestartCommand() core.SubCommand {
	return &ServiceRestartCommand{
		adminCommands: ac,
	}
}

// createHealthCheckCommand creates the health check subcommand
func (ac *AdminCommands) createHealthCheckCommand() core.SubCommand {
	return &HealthCheckCommand{
		adminCommands: ac,
	}
}

// createSystemInfoCommand creates the system info subcommand
func (ac *AdminCommands) createSystemInfoCommand() core.SubCommand {
	return &SystemInfoCommand{
		adminCommands: ac,
	}
}

// createMetricsCommand creates the metrics subcommand
func (ac *AdminCommands) createMetricsCommand() core.SubCommand {
	return &MetricsCommand{
		adminCommands: ac,
	}
}

// createMetricsWatchCommand creates the metrics watch subcommand
func (ac *AdminCommands) createMetricsWatchCommand() core.SubCommand {
	return &MetricsWatchCommand{
		adminCommands: ac,
	}
}

// MetricsCommand shows aggregate API/cache metrics for core services
type MetricsCommand struct {
	adminCommands *AdminCommands
}

func (cmd *MetricsCommand) Name() string {
	return "metrics"
}

func (cmd *MetricsCommand) Description() string {
	return "Show API/cache metrics for core services"
}

func (cmd *MetricsCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}

func (cmd *MetricsCommand) RequiresGuild() bool {
	return true
}

func (cmd *MetricsCommand) RequiresPermissions() bool {
	return true
}

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

func (cmd *MetricsCommand) formatMetrics(ctx *core.Context) string {
	var lines []string

	services := []string{"monitoring", "automod"}
	for _, name := range services {
		info, err := cmd.adminCommands.serviceManager.GetServiceInfo(name)
		if err != nil || info == nil || info.Service == nil {
			continue
		}
		stats := info.Service.Stats()
		var ms []string
		if len(stats.CustomMetrics) > 0 {
			for k, v := range stats.CustomMetrics {
				ms = append(ms, fmt.Sprintf("• %s: %v", k, v))
			}
		} else {
			ms = append(ms, "• No custom metrics available")
		}

		// Add aggregated unified cache metrics (using typed getters) when available
		if name == "monitoring" {
			if uc := cmd.adminCommands.unifiedCache; uc != nil {
				mE, mH, mM, mEv := uc.MemberMetrics()
				gE, gH, gM, gEv := uc.GuildMetrics()
				rE, rH, rM, rEv := uc.RolesMetrics()
				cE, cH, cM, cEv := uc.ChannelMetrics()
				total := mE + gE + rE + cE

				ms = append(ms, fmt.Sprintf("• cache_entries_total: %d", total))
				ms = append(ms, fmt.Sprintf("• members: entries=%d hits=%d misses=%d evictions=%d", mE, mH, mM, mEv))
				ms = append(ms, fmt.Sprintf("• guilds: entries=%d hits=%d misses=%d evictions=%d", gE, gH, gM, gEv))
				ms = append(ms, fmt.Sprintf("• roles: entries=%d hits=%d misses=%d evictions=%d", rE, rH, rM, rEv))
				ms = append(ms, fmt.Sprintf("• channels: entries=%d hits=%d misses=%d evictions=%d", cE, cH, cM, cEv))
			}
		}

		// Extra: display the roles cache TTL configured for the guild in the monitoring section
		if name == "monitoring" && ctx != nil && ctx.GuildConfig != nil {
			ttl := strings.TrimSpace(ctx.GuildConfig.RolesCacheTTL)
			if ttl == "" {
				ttl = "default (5m)"
			}
			ms = append(ms, fmt.Sprintf("• roles_cache_ttl: %s", ttl))
		}

		lines = append(lines,
			fmt.Sprintf("**Service:** %s", name),
			strings.Join(ms, "\n"),
			"",
		)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// MetricsWatchCommand streams metrics updates to the current channel for a period
type MetricsWatchCommand struct {
	adminCommands *AdminCommands
}

func (cmd *MetricsWatchCommand) Name() string {
	return "metrics_watch"
}

func (cmd *MetricsWatchCommand) Description() string {
	return "Continuously update API/cache metrics in this channel for a period"
}

func (cmd *MetricsWatchCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "interval_seconds",
			Description: "Refresh interval in seconds (default: 30)",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "duration_seconds",
			Description: "Total duration to update in seconds (default: 300)",
			Required:    false,
		},
	}
}

func (cmd *MetricsWatchCommand) RequiresGuild() bool {
	return true
}

func (cmd *MetricsWatchCommand) RequiresPermissions() bool {
	return true
}

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

// ServiceStatusCommand shows detailed status of a specific service
type ServiceStatusCommand struct {
	adminCommands *AdminCommands
}

func (cmd *ServiceStatusCommand) Name() string {
	return "status"
}

func (cmd *ServiceStatusCommand) Description() string {
	return "Show detailed status of a service"
}

func (cmd *ServiceStatusCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "service",
			Description: "Name of the service to check",
			Required:    true,
		},
	}
}

func (cmd *ServiceStatusCommand) RequiresGuild() bool {
	return true
}

func (cmd *ServiceStatusCommand) RequiresPermissions() bool {
	return true
}

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
	healthCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

// ServiceListCommand lists all registered services
type ServiceListCommand struct {
	adminCommands *AdminCommands
}

func (cmd *ServiceListCommand) Name() string {
	return "list"
}

func (cmd *ServiceListCommand) Description() string {
	return "List all registered services"
}

func (cmd *ServiceListCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{}
}

func (cmd *ServiceListCommand) RequiresGuild() bool {
	return true
}

func (cmd *ServiceListCommand) RequiresPermissions() bool {
	return true
}

func (cmd *ServiceListCommand) Handle(ctx *core.Context) error {
	services := cmd.adminCommands.serviceManager.GetAllServices()

	embed := &discordgo.MessageEmbed{
		Title:       "🔧 Registered Services",
		Color:       theme.ServiceList(),
		Description: fmt.Sprintf("Total services: %d", len(services)),
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	// Group services by type
	servicesByType := make(map[service.ServiceType][]string)
	for name, info := range services {
		sType := info.Service.Type()
		status := cmd.adminCommands.getServiceStatusIcon(info.State)
		servicesByType[sType] = append(servicesByType[sType], fmt.Sprintf("%s %s", status, name))
	}

	// Add fields for each service type
	for sType, serviceList := range servicesByType {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   string(sType),
			Value:  strings.Join(serviceList, "\n"),
			Inline: true,
		})
	}

	return core.NewResponseManager(ctx.Session).Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}

// ServiceRestartCommand restarts a specific service
type ServiceRestartCommand struct {
	adminCommands *AdminCommands
}

func (cmd *ServiceRestartCommand) Name() string {
	return "restart"
}

func (cmd *ServiceRestartCommand) Description() string {
	return "Restart a specific service"
}

func (cmd *ServiceRestartCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "service",
			Description: "Name of the service to restart",
			Required:    true,
		},
	}
}

func (cmd *ServiceRestartCommand) RequiresGuild() bool {
	return true
}

func (cmd *ServiceRestartCommand) RequiresPermissions() bool {
	return true
}

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

// HealthCheckCommand performs system-wide health check
type HealthCheckCommand struct {
	adminCommands *AdminCommands
}

func (cmd *HealthCheckCommand) Name() string {
	return "health"
}

func (cmd *HealthCheckCommand) Description() string {
	return "Perform system-wide health check"
}

func (cmd *HealthCheckCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{}
}

func (cmd *HealthCheckCommand) RequiresGuild() bool {
	return true
}

func (cmd *HealthCheckCommand) RequiresPermissions() bool {
	return true
}

func (cmd *HealthCheckCommand) Handle(ctx *core.Context) error {
	services := cmd.adminCommands.serviceManager.GetAllServices()

	healthyCount := 0
	unhealthyServices := []string{}
	totalServices := len(services)

	// Check health of all services
	healthCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

// SystemInfoCommand shows general system information
type SystemInfoCommand struct {
	adminCommands *AdminCommands
}

func (cmd *SystemInfoCommand) Name() string {
	return "info"
}

func (cmd *SystemInfoCommand) Description() string {
	return "Show general system information"
}

func (cmd *SystemInfoCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{}
}

func (cmd *SystemInfoCommand) RequiresGuild() bool {
	return true
}

func (cmd *SystemInfoCommand) RequiresPermissions() bool {
	return true
}

func (cmd *SystemInfoCommand) Handle(ctx *core.Context) error {
	services := cmd.adminCommands.serviceManager.GetAllServices()
	runningServices := cmd.adminCommands.serviceManager.GetRunningServices()

	embed := &discordgo.MessageEmbed{
		Title: "ℹ️ System Information",
		Color: theme.SystemInfo(),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Bot Name",
				Value:  "DiscordCore v2",
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

// Helper methods

func (ac *AdminCommands) getStatusColor(state service.ServiceState, healthy bool) int {
	if state == service.StateRunning && healthy {
		return theme.StatusOK()
	} else if state == service.StateError {
		return theme.StatusError()
	} else if state == service.StateRunning && !healthy {
		return theme.StatusDegraded()
	}
	return theme.StatusDefault()
}

func (ac *AdminCommands) getHealthString(healthy bool) string {
	if healthy {
		return "✅ Healthy"
	}
	return "❌ Unhealthy"
}

func (ac *AdminCommands) getOverallHealthString(healthy bool) string {
	if healthy {
		return "✅ All systems operational"
	}
	return "❌ Issues detected"
}

func (ac *AdminCommands) getServiceStatusIcon(state service.ServiceState) string {
	switch state {
	case service.StateRunning:
		return "✅"
	case service.StateError:
		return "❌"
	case service.StateStopped:
		return "⏹️"
	case service.StateInitializing:
		return "🔄"
	case service.StateStopping:
		return "⏸️"
	default:
		return "❓"
	}
}

func (ac *AdminCommands) formatDuration(d time.Duration) string {
	if d == 0 {
		return "Not running"
	}

	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else {
		return fmt.Sprintf("%dm", minutes)
	}
}

func (ac *AdminCommands) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
