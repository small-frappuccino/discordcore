package stats

import (
	"context"
	"fmt"
	"strings"

	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
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
	logger        *slog.Logger
}

// NewStatsCommands returns the root stats command tree.
func NewStatsCommands(configManager *files.ConfigManager, statsService StatsService, logger *slog.Logger) *StatsCommands {
	return &StatsCommands{
		configManager: configManager,
		statsService:  statsService,
		logger:        logger,
	}
}

// RegisterCommands registers the commands.
func (c *StatsCommands) RegisterCommands(router commands.ArikawaRegisterer) {
	if router == nil || c.configManager == nil {
		return
	}

	router.Register(&statsRootCommand{
		configManager: c.configManager,
		statsService:  c.statsService,
		logger:        c.logger,
	})
}

type statsRootCommand struct {
	configManager *files.ConfigManager
	statsService  StatsService
	logger        *slog.Logger
}

func (c *statsRootCommand) Name() string              { return "stats" }
func (c *statsRootCommand) Description() string       { return "Configure stats channels for this server" }
func (c *statsRootCommand) RequiresGuild() bool       { return true }
func (c *statsRootCommand) RequiresPermissions() bool { return true }

func (c *statsRootCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionManageGuild
}

func (c *statsRootCommand) Options() []discord.CommandOption {
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
					OptionName:  "label",
					Description: "The exact name/prefix to use (e.g. '☆ Members ☆ : ')",
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
	}
}

func (c *statsRootCommand) Handle(ctx *commands.ArikawaContext) error {
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
	}
	return nil
}

func (c *statsRootCommand) handleAdd(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")
	if channelID == "" {
		return fmt.Errorf("channel is required")
	}

	roleFilter := parsedOpts.RoleID("role_filter")
	memberType := parsedOpts.String("type")
	if memberType == "" {
		memberType = "all"
	}
	label := parsedOpts.String("label")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		for i, ch := range cfg.Stats.Channels {
			if ch.ChannelID == channelID {
				cfg.Stats.Channels[i].MemberType = memberType
				cfg.Stats.Channels[i].NameTemplate = "" // clear it in case it was previously set
				cfg.Stats.Channels[i].RoleID = roleFilter
				cfg.Stats.Channels[i].Label = label
				return nil
			}
		}

		cfg.Stats.Channels = append(cfg.Stats.Channels, files.StatsChannelConfig{
			ChannelID:  channelID,
			Label:      label,
			MemberType: memberType,
			RoleID:     roleFilter,
		})
		return nil
	})

	if err != nil {
		return err
	}

	if c.statsService != nil {
		c.statsService.ForceGuildUpdate(ctx.GuildID.String())
		c.statsService.UpdateStatsChannels(context.WithoutCancel(context.Background()))
	}

	if c.logger != nil {
		c.logger.Debug("Added or updated stats channel",
			slog.String("guild_id", ctx.GuildID.String()),
			slog.String("channel_id", channelID),
			slog.String("member_type", memberType),
		)
	}

	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Updated stats configuration for <#" + channelID + ">."),
	})
}

func (c *statsRootCommand) handleRemove(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	cfg := ctx.GuildConfig
	if len(cfg.Stats.Channels) == 0 {
		ctx.Respond(commands.NewArikawaMissingConfigErrorData(ctx.GuildID.String(), "Stats Channels", "/stats"))
		return commands.ErrAlreadyAcknowledged
	}

	parsedOpts := commands.ArikawaOptionList(opts)
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
			Content: option.NewNullableString("<#" + channelID + "> is not configured as a stats channel."),
			Flags:   discord.EphemeralMessage,
		})
	}

	if c.statsService != nil {
		c.statsService.UpdateStatsChannels(context.WithoutCancel(context.Background()))
	}

	if c.logger != nil {
		c.logger.Debug("Removed stats channel",
			slog.String("guild_id", ctx.GuildID.String()),
			slog.String("channel_id", channelID),
		)
	}

	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Removed <#" + channelID + "> from stats channels."),
	})
}

func (c *statsRootCommand) handleList(ctx *commands.ArikawaContext) error {
	cfg := ctx.GuildConfig
	if len(cfg.Stats.Channels) == 0 {
		return ctx.Respond(commands.NewArikawaMissingConfigErrorData(ctx.GuildID.String(), "Stats Channels", "/stats"))
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
		buf.WriteString("• <#")
		buf.WriteString(ch.ChannelID)
		buf.WriteString(">\n  Label: `")
		buf.WriteString(ch.Label)
		buf.WriteString("`\n  Filter: ")
		buf.WriteString(filterStr)
		buf.WriteString("\n\n")
	}

	embed := discord.Embed{
		Title:       "Stats Channels",
		Description: buf.String(),
		Color:       0x5865F2, // Discord Blurple
		Footer: &discord.EmbedFooter{
			Text: "Updates every 5 minutes",
		},
	}

	return ctx.Respond(api.InteractionResponseData{
		Embeds: &[]discord.Embed{embed},
	})
}
