package stats

import (
	"context"
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"log/slog"
)

// StatsService interface for dependency injection.
type StatsService interface {
	UpdateStatsChannels(ctx context.Context) error
	ForceGuildUpdate(guildID string)
}

// StatsCommands wiring.
type StatsCommands struct {
	configManager *files.ConfigManager
	statsService  StatsService
}

// NewStatsCommands returns the root stats command tree.
func NewStatsCommands(configManager *files.ConfigManager, statsService StatsService) *StatsCommands {
	return &StatsCommands{
		configManager: configManager,
		statsService:  statsService,
	}
}

// RegisterCommands registers the commands.
func (c *StatsCommands) RegisterCommands(router *core.ArikawaCommandRouter) {
	if router == nil || c.configManager == nil {
		return
	}

	router.Register(&statsRootCommand{
		configManager: c.configManager,
		statsService:  c.statsService,
	})
}

type statsRootCommand struct {
	configManager *files.ConfigManager
	statsService  StatsService
}

func (c *statsRootCommand) Name() string              { return "stats" }
func (c *statsRootCommand) Description() string       { return "Configure stats channels for this server" }
func (c *statsRootCommand) RequiresGuild() bool       { return true }
func (c *statsRootCommand) RequiresPermissions() bool { return true }

func (c *statsRootCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionManageGuild
}

func (c *statsRootCommand) Options() []discord.CommandOption {
	minInterval := 5.0
	return []discord.CommandOption{
		&discord.SubcommandOption{
			OptionName:  "add",
			Description: "Add a new stats channel",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:  "channel",
					Description: "The voice channel to rename",
					Required:    true,
					ChannelTypes: []discord.ChannelType{
						discord.GuildVoice,
					},
				},
				&discord.StringOption{
					OptionName:  "type",
					Description: "Type of members to count (default: All)",
					Required:    false,
					Choices: []discord.StringChoice{
						{Name: "All Members", Value: "all"},
						{Name: "Humans Only", Value: "humans"},
						{Name: "Bots Only", Value: "bots"},
					},
				},
				&discord.StringOption{
					OptionName:  "name_template",
					Description: "Template for the channel name (e.g. 'Members: {count}')",
					Required:    false,
				},
				&discord.RoleOption{
					OptionName:  "role_filter",
					Description: "Only count members with this role",
					Required:    false,
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "remove",
			Description: "Remove a stats channel",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:  "channel",
					Description: "The stats channel to remove",
					Required:    true,
					ChannelTypes: []discord.ChannelType{
						discord.GuildVoice,
					},
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "list",
			Description: "List all configured stats channels",
		},
		&discord.SubcommandOption{
			OptionName:  "settings",
			Description: "Configure global stats settings",
			Options: []discord.CommandOptionValue{
				&discord.NumberOption{
					OptionName:  "interval",
					Description: "Update interval in minutes (min 5)",
					Required:    true,
					Min:         option.NewFloat(minInterval),
				},
			},
		},
	}
}

func ensureStatsEnabled(ctx *core.ArikawaContext) error {
	if ctx == nil || ctx.Config == nil {
		return fmt.Errorf("invalid context")
	}
	cfg := ctx.Config.Config()
	features := cfg.ResolveFeatures(ctx.GuildID.String())
	route, isFallback := ctx.Config.GuildConfig(ctx.GuildID.String()).ResolveFeatureBotInstanceID("stats")

	ctx.Logger.Debug("Transient state inspection: Evaluated feature enablement for Stats",
		slog.Bool("toggle_enabled", features.StatsChannels),
		slog.String("resolved_route", route),
		slog.Bool("route_is_fallback", isFallback),
	)

	if route == "<unrouted>" {
		_ = ctx.Respond(api.InteractionResponseData{
			Content: option.NewNullableString("Stats channels feature is currently disabled for this server."),
			Flags:   discord.EphemeralMessage,
		})
		return core.ErrAlreadyAcknowledged
	}
	return nil
}

func (c *statsRootCommand) Handle(ctx *core.ArikawaContext) error {
	data, ok := ctx.Interaction.Data.(*discord.CommandInteraction)
	if !ok || len(data.Options) == 0 {
		return nil
	}

	subcommand := data.Options[0]

	switch subcommand.Name {
	case "add":
		return c.handleAdd(ctx, subcommand.Options)
	case "remove":
		return c.handleRemove(ctx, subcommand.Options)
	case "list":
		return c.handleList(ctx)
	case "settings":
		return c.handleSettings(ctx, subcommand.Options)
	}
	return nil
}

func (c *statsRootCommand) handleAdd(ctx *core.ArikawaContext, opts []discord.CommandInteractionOption) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}

	parsedOpts := core.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")
	if channelID == "" {
		return fmt.Errorf("channel is required")
	}

	roleFilter := parsedOpts.RoleID("role_filter")
	memberType := parsedOpts.String("type")
	if memberType == "" {
		memberType = "all"
	}
	nameTemplate := parsedOpts.String("name_template")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
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
		c.statsService.ForceGuildUpdate(ctx.GuildID.String())
		_ = c.statsService.UpdateStatsChannels(context.WithoutCancel(context.Background()))
	}

	ctx.Logger.Debug("Added or updated stats channel",
		slog.String("guild_id", ctx.GuildID.String()),
		slog.String("channel_id", channelID),
		slog.String("member_type", memberType),
	)

	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf("Updated stats configuration for <#%s>.", channelID)),
		Flags:   discord.EphemeralMessage,
	})
}

func (c *statsRootCommand) handleRemove(ctx *core.ArikawaContext, opts []discord.CommandInteractionOption) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}

	parsedOpts := core.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")

	if channelID == "" {
		return fmt.Errorf("channel is required")
	}

	removed := false
	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
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
		return ctx.Respond(api.InteractionResponseData{
			Content: option.NewNullableString(fmt.Sprintf("<#%s> is not configured as a stats channel.", channelID)),
			Flags:   discord.EphemeralMessage,
		})
	}

	if c.statsService != nil {
		_ = c.statsService.UpdateStatsChannels(context.WithoutCancel(context.Background()))
	}

	ctx.Logger.Debug("Removed stats channel",
		slog.String("guild_id", ctx.GuildID.String()),
		slog.String("channel_id", channelID),
	)

	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf("Removed <#%s> from stats channels.", channelID)),
		Flags:   discord.EphemeralMessage,
	})
}

func (c *statsRootCommand) handleList(ctx *core.ArikawaContext) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}

	cfg := ctx.GuildConfig
	if len(cfg.Stats.Channels) == 0 {
		return ctx.Respond(api.InteractionResponseData{
			Content: option.NewNullableString("There are no stats channels configured for this server."),
			Flags:   discord.EphemeralMessage,
		})
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

	embed := discord.Embed{
		Title:       "Stats Channels",
		Description: buf.String(),
		Color:       0x5865F2, // Discord Blurple
		Footer: &discord.EmbedFooter{
			Text: fmt.Sprintf("Update Interval: %d minutes", interval),
		},
	}

	return ctx.Respond(api.InteractionResponseData{
		Embeds: &[]discord.Embed{embed},
		Flags:  discord.EphemeralMessage,
	})
}

func (c *statsRootCommand) handleSettings(ctx *core.ArikawaContext, opts []discord.CommandInteractionOption) error {
	if err := ensureStatsEnabled(ctx); err != nil {
		return err
	}

	parsedOpts := core.ArikawaOptionList(opts)
	interval := parsedOpts.Float("interval")

	if interval < 5 {
		interval = 5
	}

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Stats.UpdateIntervalMins = int(interval)
		return nil
	})

	if err != nil {
		return err
	}

	ctx.Logger.Debug("Updated stats update interval",
		slog.String("guild_id", ctx.GuildID.String()),
		slog.Float64("interval_mins", interval),
	)

	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString(fmt.Sprintf("Stats update interval set to **%d minutes**.", int(interval))),
		Flags:   discord.EphemeralMessage,
	})
}
