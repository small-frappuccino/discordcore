package admin

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alice-bnuy/discordcore/pkg/discord/commands/core"
	"github.com/alice-bnuy/discordcore/pkg/service"
	"github.com/bwmarrin/discordgo"
)

// AdminCommands provides administrative commands for service management
type AdminCommands struct {
	serviceManager *service.ServiceManager
}

// NewAdminCommands creates a new admin commands handler
func NewAdminCommands(serviceManager *service.ServiceManager) *AdminCommands {
	return &AdminCommands{
		serviceManager: serviceManager,
	}
}

// RegisterCommands registers all admin commands with the router
func (ac *AdminCommands) RegisterCommands(router *core.CommandRouter) {
	// Main admin command with subcommands
	adminCmd := core.NewGroupCommand(
		"admin",
		"Administrative commands for bot management",
		core.NewResponder(router.GetSession()),
		core.NewPermissionChecker(router.GetSession(), router.GetConfigManager()),
	)

	// Service management subcommands
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

	return core.NewResponder(ctx.Session).SendEmbed(ctx.Interaction, embed)
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
		Color:       0x5865F2,
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

	return core.NewResponder(ctx.Session).SendEmbed(ctx.Interaction, embed)
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
	responder := core.NewResponder(ctx.Session)
	if err := responder.Info(ctx.Interaction, fmt.Sprintf("🔄 Restarting service: %s", serviceName)); err != nil {
		return err
	}

	// Restart service in background
	go func() {
		if err := cmd.adminCommands.serviceManager.RestartService(serviceName); err != nil {
			ctx.Logger.WithError(err).Error("Failed to restart service")
			// Try to follow up with error message
			responder.EditResponse(ctx.Interaction, fmt.Sprintf("❌ Failed to restart service '%s': %v", serviceName, err))
		} else {
			// Follow up with success message
			responder.EditResponse(ctx.Interaction, fmt.Sprintf("✅ Service '%s' restarted successfully", serviceName))
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

	return core.NewResponder(ctx.Session).SendEmbed(ctx.Interaction, embed)
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
		Color: 0x5865F2,
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

	return core.NewResponder(ctx.Session).SendEmbed(ctx.Interaction, embed)
}

// Helper methods

func (ac *AdminCommands) getStatusColor(state service.ServiceState, healthy bool) int {
	if state == service.StateRunning && healthy {
		return 0x00FF00 // Green
	} else if state == service.StateError {
		return 0xFF0000 // Red
	} else if state == service.StateRunning && !healthy {
		return 0xFFA500 // Orange
	}
	return 0x808080 // Gray
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
