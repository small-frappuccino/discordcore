package logging

import (
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/cmd"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// LoggingCommands wiring.
type LoggingCommands struct {
	configManager config.Provider
}

// NewLoggingCommands returns the root logging command tree.
func NewLoggingCommands(configManager config.Provider) cmd.CommandGroup {
	return commands.NewLegacyAdapter(&loggingRootCommand{
		configManager: configManager,
	})
}

// RegisterCommands is deprecated.

type loggingRootCommand struct {
	configManager config.Provider
}

func (c *loggingRootCommand) Name() string              { return "logging" }
func (c *loggingRootCommand) Description() string       { return "Manage server logging configuration" }
func (c *loggingRootCommand) RequiresGuild() bool       { return true }
func (c *loggingRootCommand) RequiresPermissions() bool { return true }

func (c *loggingRootCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionManageGuild
}

func (c *loggingRootCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.SubcommandOption{
			OptionName:  "avatar",
			Description: "Configure avatar update logging",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:   "channel",
					Description:  "Channel to send avatar updates to",
					Required:     true,
					ChannelTypes: []discord.ChannelType{discord.GuildText},
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "role_update",
			Description: "Configure role update logging",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:   "channel",
					Description:  "Channel to send role updates to",
					Required:     true,
					ChannelTypes: []discord.ChannelType{discord.GuildText},
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "automod",
			Description: "Configure Discord native automod logging",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:   "channel",
					Description:  "Channel to send automod logs to",
					Required:     true,
					ChannelTypes: []discord.ChannelType{discord.GuildText},
				},
				&discord.StringOption{
					OptionName:  "rule_id",
					Description: "Optional native Discord AutoMod rule ID to assign this channel to",
					Required:    false,
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "messages",
			Description: "Configure message edit and delete logging",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:   "channel",
					Description:  "Channel to send message edit/delete logs to",
					Required:     true,
					ChannelTypes: []discord.ChannelType{discord.GuildText},
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "entry",
			Description: "Configure member join logging",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:   "channel",
					Description:  "Channel to send member join logs to",
					Required:     true,
					ChannelTypes: []discord.ChannelType{discord.GuildText},
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "exit",
			Description: "Configure member leave logging",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:   "channel",
					Description:  "Channel to send member leave logs to",
					Required:     true,
					ChannelTypes: []discord.ChannelType{discord.GuildText},
				},
			},
		},
		&discord.SubcommandOption{
			OptionName:  "warnings",
			Description: "Configure moderation action logging",
			Options: []discord.CommandOptionValue{
				&discord.ChannelOption{
					OptionName:   "channel",
					Description:  "Channel to send moderation logs to",
					Required:     true,
					ChannelTypes: []discord.ChannelType{discord.GuildText},
				},
				&discord.StringOption{
					OptionName:  "log_warning_from_other_bots",
					Description: "Scope of moderation events to log",
					Required:    false,
					Choices: []discord.StringChoice{
						{Name: "discordcore (only this bot)", Value: "discordcore"},
						{Name: "All Bots", Value: "all_bots"},
						{Name: "All (Bots and Humans)", Value: "all"},
					},
				},
			},
		},
	}
}

func (c *loggingRootCommand) Handle(ctx *commands.ArikawaContext) error {
	data, ok := ctx.Interaction.Data.(*discord.CommandInteraction)
	if !ok || len(data.Options) == 0 {
		return nil
	}

	subcommand := data.Options[0]

	switch subcommand.Name {
	case "avatar":
		return c.handleAvatar(ctx, subcommand.Options)
	case "role_update":
		return c.handleRoleUpdate(ctx, subcommand.Options)
	case "automod":
		return c.handleAutomod(ctx, subcommand.Options)
	case "messages":
		return c.handleMessages(ctx, subcommand.Options)
	case "entry":
		return c.handleEntry(ctx, subcommand.Options)
	case "exit":
		return c.handleExit(ctx, subcommand.Options)
	case "warnings":
		return c.handleWarnings(ctx, subcommand.Options)
	}
	return nil
}

func (c *loggingRootCommand) handleAvatar(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Channels.AvatarLogging = channelID
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Avatar update logs will now be sent to <#" + channelID + ">."),
	})
}

func (c *loggingRootCommand) handleRoleUpdate(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Channels.RoleUpdate = channelID
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Role update logs will now be sent to <#" + channelID + ">."),
	})
}

func (c *loggingRootCommand) handleAutomod(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")
	desc := "Discord native AutoMod logs will now be sent to <#" + channelID + ">."

	ruleIDStr := parsedOpts.String("rule_id")
	if ruleIDStr != "" {
		guildID := ctx.GuildID
		ruleID, _ := discord.ParseSnowflake(ruleIDStr)
		chID, _ := discord.ParseSnowflake(channelID)

		rule, err := ctx.Client.GetAutoModerationRule(guildID, discord.AutoModerationRuleID(ruleID))
		if err != nil {
			slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
			return ctx.Respond(api.InteractionResponseData{
				Content: option.NewNullableString(fmt.Sprintf("Failed to fetch rule `%s`: %v\nThe logging channel was NOT configured internally because the Discord native rule could not be found.", ruleIDStr, err)),
			})
		}

		if !rule.Enabled {
			desc += "\n⚠️ **Aviso**: A regra `" + ruleIDStr + "` está desativada no Discord. O envio de alertas não funcionará até que ela seja ativada."
		}

		hasAction := false
		for i, action := range rule.Actions {
			if action.Type == discord.AutoModerationSendAlertMessage {
				hasAction = true
				if action.Metadata.ChannelID != discord.ChannelID(chID) {
					rule.Actions[i].Metadata.ChannelID = discord.ChannelID(chID)
				}
			}
		}

		if !hasAction {
			rule.Actions = append(rule.Actions, discord.AutoModerationAction{
				Type: discord.AutoModerationSendAlertMessage,
				Metadata: discord.AutoModerationActionMetadata{
					ChannelID: discord.ChannelID(chID),
				},
			})
		}

		_, err = ctx.Client.ModifyAutoModerationRule(guildID, discord.AutoModerationRuleID(ruleID), api.ModifyAutoModerationRuleData{
			Actions: &rule.Actions,
		})
		if err != nil {
			slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
			return ctx.Respond(api.InteractionResponseData{
				Content: option.NewNullableString(fmt.Sprintf("Failed to update Discord rule `%s`: %v\nThe logging channel was NOT configured internally because the Discord native rule could not be updated.", ruleIDStr, err)),
			})
		}

		desc += "\nSuccessfully attached channel to native AutoMod rule `" + ruleIDStr + "`."
	} else {
		// If no rule ID is provided, check if the native keyword rules exist and are enabled to warn the user
		rules, err := ctx.Client.ListAutoModerationRules(ctx.GuildID)
		if err == nil {
			keywordRuleActive := false
			profileRuleActive := false
			for _, r := range rules {
				if r.TriggerType == discord.AutoModerationKeyword && r.Enabled {
					keywordRuleActive = true
				}
				if r.TriggerType == discord.AutoModerationMemberProfile && r.Enabled {
					profileRuleActive = true
				}
			}
			if !keywordRuleActive || !profileRuleActive {
				desc += "\n⚠️ **Aviso**: O 'Block Custom Words' e/ou 'Block Words in Member Profiles' nativo do Discord não está totalmente ativado no servidor. O bot configurou o canal internamente, mas os alertas dependem da ativação dessas regras no painel do Discord."
			}
		}
	}

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Channels.AutomodAction = channelID
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString(desc),
	})
}

func (c *loggingRootCommand) handleMessages(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Channels.MessageEdit = channelID
		cfg.Channels.MessageDelete = channelID
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Message edit and delete logs will now be sent to <#" + channelID + ">."),
	})
}

func (c *loggingRootCommand) handleEntry(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Channels.MemberJoin = channelID
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Member join logs will now be sent to <#" + channelID + ">."),
	})
}

func (c *loggingRootCommand) handleExit(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Channels.MemberLeave = channelID
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Member leave logs will now be sent to <#" + channelID + ">."),
	})
}

func (c *loggingRootCommand) handleWarnings(ctx *commands.ArikawaContext, opts []discord.CommandInteractionOption) error {
	parsedOpts := commands.ArikawaOptionList(opts)
	channelID := parsedOpts.ChannelID("channel")

	scope := "discordcore" // Default
	if scopeOpt := parsedOpts.String("log_warning_from_other_bots"); scopeOpt != "" {
		scope = scopeOpt
	}

	err := c.configManager.UpdateGuildConfig(ctx.GuildID.String(), func(cfg *files.GuildConfig) error {
		cfg.Channels.ModerationCase = channelID
		cfg.LogModerationScope = scope
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("Operational telemetry: Logging channel updated", slog.String("channel_id", channelID))
	return ctx.Respond(api.InteractionResponseData{
		Content: option.NewNullableString("Moderation action logs will now be sent to <#" + channelID + ">\nScope: `" + scope + "`"),
	})
}
