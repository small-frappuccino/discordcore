package admin

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// AdminCommands provides administrative commands for service management.
//
// Cache and persistent-store observability used to surface here via
// /admin metrics; that command was removed in favor of /v1/health/cache,
// so AdminCommands no longer carries those dependencies.
type AdminCommands struct {
	serviceManager *service.ServiceManager
}

// NewAdminCommands creates a new admin commands handler. Only the
// service.ServiceManager is required — restart/status/list/health all read
// from it; cache and persistent-store stats moved to /v1/health/cache.
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

		core.NewPermissionChecker(router.GetSession(), router.GetConfigManager()),
	)

	// Service management subcommands
	adminCmd.AddSubCommand(ac.createServiceStatusCommand())
	adminCmd.AddSubCommand(ac.createServiceListCommand())
	adminCmd.AddSubCommand(ac.createServiceRestartCommand())
	adminCmd.AddSubCommand(ac.createHealthCheckCommand())

	router.RegisterSlashCommand(adminCmd)
}

// createServiceStatusCommand creates the service status subcommand
func (ac *AdminCommands) createServiceStatusCommand() core.Command {
	return &ServiceStatusCommand{
		adminCommands: ac,
	}
}

// createServiceListCommand creates the service list subcommand
func (ac *AdminCommands) createServiceListCommand() core.Command {
	return &ServiceListCommand{
		adminCommands: ac,
	}
}

// createServiceRestartCommand creates the service restart subcommand
func (ac *AdminCommands) createServiceRestartCommand() core.Command {
	return &ServiceRestartCommand{
		adminCommands: ac,
	}
}

// createHealthCheckCommand creates the health check subcommand
func (ac *AdminCommands) createHealthCheckCommand() core.Command {
	return &HealthCheckCommand{
		adminCommands: ac,
	}
}

// createSystemInfoCommand creates the system info subcommand
func (ac *AdminCommands) createSystemInfoCommand() core.Command {
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
		return core.NewCommandError("This command needs a service name before it can continue, so this reply stays private.", true)
	}

	info, err := cmd.adminCommands.serviceManager.GetServiceInfo(serviceName)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("No service named %s was found, so this reply stays private.", serviceName), true)
	}

	// Perform health check
	healthCtx, cancel := context.WithTimeoutCause(context.Background(), 10*time.Second, fmt.Errorf("service status health check timeout"))
	defer cancel()

	health := info.Service.HealthCheck(healthCtx)
	stats := info.Service.Stats()

	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Service Status: %s", serviceName),
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

	// Add display metric rows in producer order (the ServiceMetric stable-
	// ordering contract). Values are pre-formatted on the producer side, so
	// the consumer is just a label/value join.
	if len(stats.Metrics) > 0 {
		metrics := make([]string, 0, len(stats.Metrics))
		for _, row := range stats.Metrics {
			metrics = append(metrics, fmt.Sprintf("%s: %s", row.Label, row.Value))
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   "Metrics",
			Value:  strings.Join(metrics, "\n"),
			Inline: false,
		})
	}

	return core.NewResponseManager(ctx.Session).
		WithConfig(core.ResponseConfig{Ephemeral: true}).
		Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
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
		Title:       "Registered Services",
		Color:       theme.ServiceList(),
		Description: fmt.Sprintf("Here is the current service registry. This reply stays private because it is operational state. Total services: %d", len(services)),
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
		servicesByType[sType] = append(servicesByType[sType], fmt.Sprintf("%s: %s", status, name))
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

	return core.NewResponseManager(ctx.Session).
		WithConfig(core.ResponseConfig{Ephemeral: true}).
		Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
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
		return core.NewCommandError("This command needs a service name before it can continue, so this reply stays private.", true)
	}

	// Check if service exists
	_, err := cmd.adminCommands.serviceManager.GetServiceInfo(serviceName)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("No service named %s was found, so this reply stays private.", serviceName), true)
	}

	// Send initial response
	rm := core.NewResponseManager(ctx.Session).WithConfig(core.ResponseConfig{Ephemeral: true})
	if err := rm.Info(ctx.Interaction, fmt.Sprintf("Restarting service %s now. This reply stays private while the restart runs.", serviceName)); err != nil {
		return fmt.Errorf("ServiceRestartCommand.Handle: %w", err)
	}

	// Restart service in background
	go func() {
		if err := cmd.adminCommands.serviceManager.RestartService(serviceName); err != nil {
			ctx.Logger.Error().Errorf("Failed to restart service: %v", err)
			// Try to follow up with error message
			rm.EditResponse(ctx.Interaction, fmt.Sprintf("Service %s couldn't be restarted. This reply stays private because it includes internal service details: %v", serviceName, err))
		} else {
			// Follow up with success message
			rm.EditResponse(ctx.Interaction, fmt.Sprintf("Service %s was restarted.", serviceName))
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
		Title:       "System Health Check",
		Color:       color,
		Description: "Here is the current system health snapshot. This reply stays private because it reflects internal service status.",
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
			Name:   "Unhealthy Services",
			Value:  strings.Join(unhealthyServices, "\n"),
			Inline: false,
		})
	}

	return core.NewResponseManager(ctx.Session).
		WithConfig(core.ResponseConfig{Ephemeral: true}).
		Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
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

	botName := files.EffectiveBotName()
	if files.AppVersion != "" {
		botName = fmt.Sprintf("%s %s", botName, files.AppVersion)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "System Information",
		Color:       theme.SystemInfo(),
		Description: "Here is the current runtime and service summary. This reply stays private because it is operational data.",
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Bot",
				Value:  botName,
				Inline: true,
			},
			{
				Name:   "Core",
				Value:  fmt.Sprintf("discordcore %s", files.DiscordCoreVersion),
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

	return core.NewResponseManager(ctx.Session).
		WithConfig(core.ResponseConfig{Ephemeral: true}).
		Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
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
		return "Healthy"
	}
	return "Unhealthy"
}

func (ac *AdminCommands) getOverallHealthString(healthy bool) string {
	if healthy {
		return "All systems operational"
	}
	return "Issues detected"
}

func (ac *AdminCommands) getServiceStatusIcon(state service.ServiceState) string {
	switch state {
	case service.StateRunning:
		return "Running"
	case service.StateError:
		return "Error"
	case service.StateStopped:
		return "Stopped"
	case service.StateInitializing:
		return "Initializing"
	case service.StateStopping:
		return "Stopping"
	default:
		return "Unknown"
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
