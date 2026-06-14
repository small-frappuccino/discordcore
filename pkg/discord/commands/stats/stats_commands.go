package stats

import (
	"context"
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	domainstats "github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

// StatsCommands wiring.
type StatsCommands struct {
	configManager *files.ConfigManager
	statsService  *domainstats.StatsService
}

// NewStatsCommands returns the root stats command tree.
func NewStatsCommands(configManager *files.ConfigManager, statsService *domainstats.StatsService) *StatsCommands {
	return &StatsCommands{
		configManager: configManager,
		statsService:  statsService,
	}
}

// RegisterCommands registers the commands.
func (c *StatsCommands) RegisterCommands(router *core.CommandRouter) {
	if router == nil || c.configManager == nil {
		return
	}

	checker := core.NewPermissionChecker(router.GetSession(), c.configManager)

	statsGroup := core.NewGroupCommand(
		"stats",
		"Configure stats channels for this server",
		checker,
	)

	statsGroup.AddSubCommand(newStatsAddSubCommand(c.configManager, c.statsService))
	statsGroup.AddSubCommand(newStatsRemoveSubCommand(c.configManager, c.statsService))
	statsGroup.AddSubCommand(newStatsListSubCommand(c.configManager))
	statsGroup.AddSubCommand(newStatsSettingsSubCommand(c.configManager, c.statsService))

	router.RegisterSlashCommand(statsGroup)
}

func ensureStatsEnabled(ctx *core.Context) error {
	if ctx == nil || ctx.Config == nil || ctx.GuildID == "" {
		return fmt.Errorf("invalid context")
	}
	cfg := ctx.Config.GuildConfig(ctx.GuildID)
	if cfg == nil {
		return fmt.Errorf("guild config not found")
	}
	features := ctx.Config.Config().ResolveFeatures(ctx.GuildID)
	if !features.StatsChannels {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, "Stats channels feature is currently disabled for this server.")
	}
	return nil
}

type statsAddSubCommand struct {
	configManager *files.ConfigManager
	statsService  *domainstats.StatsService
}

func newStatsAddSubCommand(cm *files.ConfigManager, ss *domainstats.StatsService) *statsAddSubCommand {
	return &statsAddSubCommand{configManager: cm, statsService: ss}
}

func (c *statsAddSubCommand) Name() string              { return "add" }
func (c *statsAddSubCommand) Description() string       { return "Add a new stats channel" }
func (c *statsAddSubCommand) RequiresGuild() bool       { return true }
func (c *statsAddSubCommand) RequiresPermissions() bool { return true }
func (c *statsAddSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        "channel",
			Description: "The voice channel to rename",
			Required:    true,
			ChannelTypes: []discordgo.ChannelType{
				discordgo.ChannelTypeGuildVoice,
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "type",
			Description: "Type of members to count (default: All)",
			Required:    false,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "All Members", Value: "all"},
				{Name: "Humans Only", Value: "humans"},
				{Name: "Bots Only", Value: "bots"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "name_template",
			Description: "Template for the channel name (e.g. 'Members: {count}')",
			Required:    false,
		},
		{
			Type:        discordgo.ApplicationCommandOptionRole,
			Name:        "role_filter",
			Description: "Only count members with this role",
			Required:    false,
		},
	}
}
func (c *statsAddSubCommand) Handle(ctx *core.Context) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}
	opts := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	channelID := opts.String("channel")
	if channelID == "" {
		return fmt.Errorf("channel is required")
	}

	memberType := opts.String("type")
	if memberType == "" {
		memberType = "all"
	}
	nameTemplate := opts.String("name_template")
	roleFilter := opts.String("role_filter")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID, func(cfg *files.GuildConfig) error {
		for i, ch := range cfg.Stats.Channels {
			if ch.ChannelID == channelID {
				cfg.Stats.Channels[i].MemberType = memberType
				cfg.Stats.Channels[i].NameTemplate = nameTemplate
				cfg.Stats.Channels[i].RoleID = roleFilter
				return nil
			}
		}

		cfg.Stats.Channels = append(cfg.Stats.Channels, files.StatsChannelConfig{
			ChannelID:    channelID,
			MemberType:   memberType,
			NameTemplate: nameTemplate,
			RoleID:       roleFilter,
		})
		return nil
	})

	if err != nil {
		return err
	}

	if c.statsService != nil {
		_ = c.statsService.UpdateStatsChannels(context.WithoutCancel(context.Background()))
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, fmt.Sprintf("Updated stats configuration for <#%s>.", channelID))
}

type statsRemoveSubCommand struct {
	configManager *files.ConfigManager
	statsService  *domainstats.StatsService
}

func newStatsRemoveSubCommand(cm *files.ConfigManager, ss *domainstats.StatsService) *statsRemoveSubCommand {
	return &statsRemoveSubCommand{configManager: cm, statsService: ss}
}

func (c *statsRemoveSubCommand) Name() string              { return "remove" }
func (c *statsRemoveSubCommand) Description() string       { return "Remove a stats channel" }
func (c *statsRemoveSubCommand) RequiresGuild() bool       { return true }
func (c *statsRemoveSubCommand) RequiresPermissions() bool { return true }
func (c *statsRemoveSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionChannel,
			Name:        "channel",
			Description: "The stats channel to remove",
			Required:    true,
			ChannelTypes: []discordgo.ChannelType{
				discordgo.ChannelTypeGuildVoice,
			},
		},
	}
}
func (c *statsRemoveSubCommand) Handle(ctx *core.Context) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}
	opts := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))
	channelID := opts.String("channel")
	if channelID == "" {
		return fmt.Errorf("channel is required")
	}

	removed := false
	err := c.configManager.UpdateGuildConfig(ctx.GuildID, func(cfg *files.GuildConfig) error {
		filtered := make([]files.StatsChannelConfig, 0, len(cfg.Stats.Channels))
		for _, ch := range cfg.Stats.Channels {
			if ch.ChannelID == channelID {
				removed = true
				continue
			}
			filtered = append(filtered, ch)
		}
		cfg.Stats.Channels = filtered
		return nil
	})

	if err != nil {
		return err
	}
	if !removed {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Error(ctx.Interaction, fmt.Sprintf("<#%s> is not configured as a stats channel.", channelID))
	}

	if c.statsService != nil {
		_ = c.statsService.UpdateStatsChannels(context.WithoutCancel(context.Background()))
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, fmt.Sprintf("Removed <#%s> from stats channels.", channelID))
}

type statsListSubCommand struct {
	configManager *files.ConfigManager
}

func newStatsListSubCommand(cm *files.ConfigManager) *statsListSubCommand {
	return &statsListSubCommand{configManager: cm}
}

func (c *statsListSubCommand) Name() string                                   { return "list" }
func (c *statsListSubCommand) Description() string                            { return "List all configured stats channels" }
func (c *statsListSubCommand) RequiresGuild() bool                            { return true }
func (c *statsListSubCommand) RequiresPermissions() bool                      { return true }
func (c *statsListSubCommand) Options() []*discordgo.ApplicationCommandOption { return nil }
func (c *statsListSubCommand) Handle(ctx *core.Context) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}

	cfg := ctx.Config.GuildConfig(ctx.GuildID)
	if len(cfg.Stats.Channels) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, "There are no stats channels configured for this server.")
	}

	var buf strings.Builder
	for _, ch := range cfg.Stats.Channels {
		filterStr := "All Members"
		switch ch.MemberType {
		case "humans":
			filterStr = "Humans Only"
		case "bots":
			filterStr = "Bots Only"
		}
		if ch.RoleID != "" {
			filterStr += fmt.Sprintf(" (Role: <@&%s>)", ch.RoleID)
		}
		templateStr := ch.NameTemplate
		if templateStr == "" {
			templateStr = "☆ {label} ☆ : {count}"
		}

		buf.WriteString(fmt.Sprintf("• <#%s>\n  Filter: %s\n  Template: `%s`\n\n", ch.ChannelID, filterStr, templateStr))
	}

	interval := cfg.Stats.UpdateIntervalMins
	if interval <= 0 {
		interval = 30
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Stats Channels",
		Description: buf.String(),
		Color:       0x5865F2, // Discord Blurple
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Update Interval: %d minutes", interval),
		},
	}

	rm := core.NewResponseBuilder(ctx.Session).Build()
	return rm.Custom(ctx.Interaction, "", []*discordgo.MessageEmbed{embed})
}

type statsSettingsSubCommand struct {
	configManager *files.ConfigManager
	statsService  *domainstats.StatsService
}

func newStatsSettingsSubCommand(cm *files.ConfigManager, ss *domainstats.StatsService) *statsSettingsSubCommand {
	return &statsSettingsSubCommand{configManager: cm, statsService: ss}
}

func (c *statsSettingsSubCommand) Name() string              { return "settings" }
func (c *statsSettingsSubCommand) Description() string       { return "Configure global stats settings" }
func (c *statsSettingsSubCommand) RequiresGuild() bool       { return true }
func (c *statsSettingsSubCommand) RequiresPermissions() bool { return true }

var minInterval = float64(5)

func (c *statsSettingsSubCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "update_interval_mins",
			Description: "How often to update the channels (in minutes)",
			Required:    false,
			MinValue:    &minInterval,
			MaxValue:    1440,
		},
	}
}
func (c *statsSettingsSubCommand) Handle(ctx *core.Context) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}
	opts := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))
	intervalMins := opts.Int("update_interval_mins")

	if !opts.HasOption("update_interval_mins") {
		cfg := ctx.Config.GuildConfig(ctx.GuildID)
		interval := cfg.Stats.UpdateIntervalMins
		if interval <= 0 {
			interval = 30
		}
		enabledStr := "Enabled"
		if !cfg.Stats.Enabled {
			enabledStr = "Disabled"
		}
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, fmt.Sprintf("Stats Channels are currently **%s**. Update interval is **%d minutes**.", enabledStr, interval))
	}

	err := c.configManager.UpdateGuildConfig(ctx.GuildID, func(cfg *files.GuildConfig) error {
		cfg.Stats.UpdateIntervalMins = int(intervalMins)
		return nil
	})
	if err != nil {
		return err
	}

	if c.statsService != nil {
		_ = c.statsService.UpdateStatsChannels(context.WithoutCancel(context.Background()))
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, "Stats settings updated successfully.")
}
