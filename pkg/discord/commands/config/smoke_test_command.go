package config

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const smokeTestSubCommandName = "smoke_test"

type SmokeTestSubCommand struct {
	configManager *files.ConfigManager
}

func NewSmokeTestSubCommand(configManager *files.ConfigManager) *SmokeTestSubCommand {
	return &SmokeTestSubCommand{configManager: configManager}
}

func (c *SmokeTestSubCommand) Name() string { return smokeTestSubCommandName }

func (c *SmokeTestSubCommand) Description() string {
	return "Show bootstrap readiness for general config and QOTD"
}

func (c *SmokeTestSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }
func (c *SmokeTestSubCommand) RequiresGuild() bool       { return true }
func (c *SmokeTestSubCommand) RequiresPermissions() bool { return true }

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

func qotdSmokeTestLines(ctx *core.Context) []string {
	settings := files.DashboardQOTDConfig(ctx.GuildConfig.QOTD)
	deck, ok := settings.ActiveDeck()
	if !ok {
		return []string{"[ACTION] QOTD configuration is unavailable."}
	}

	lines := []string{fmt.Sprintf("[PASS] Active QOTD deck: %s.", deck.Name)}
	if strings.TrimSpace(ctx.GuildConfig.BotInstanceIDOverrideForDomain(files.BotDomainQOTD)) == "" {
		lines = append(lines, "[ACTION] QOTD currently follows the guild-wide/default bot binding. If you expect a separate QOTD bot, set Bot Routing -> QOTD in Control Panel first.")
	} else {
		lines = append(lines, "[PASS] QOTD domain routing override is configured.")
	}
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