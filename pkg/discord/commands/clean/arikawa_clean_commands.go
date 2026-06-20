package clean

import (
	"context"
	"fmt"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"

	coreclean "github.com/small-frappuccino/discordcore/pkg/clean"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type CleanExecutor interface {
	ExecuteClean(ctx context.Context, channelID discord.ChannelID, filter coreclean.Filter, auditChannelID discord.ChannelID, requestedBy string) (int, error)
}

type CleanCommand struct {
	configManager *files.ConfigManager
	cleanExecutor CleanExecutor
}

func NewCleanCommand(cfg *files.ConfigManager, executor CleanExecutor) *CleanCommand {
	return &CleanCommand{
		configManager: cfg,
		cleanExecutor: executor,
	}
}

func (c *CleanCommand) Name() string { return "clean" }

func (c *CleanCommand) Description() string { return "Delete recent messages in this channel" }

func (c *CleanCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.IntegerOption{
			OptionName:  "count",
			Description: "How many matching messages to remove (max 100)",
			Required:    true,
			Min:         option.NewInt(1),
			Max:         option.NewInt(100),
		},
		&discord.UserOption{
			OptionName:  "user",
			Description: "Only remove messages from this user",
			Required:    false,
		},
		&discord.StringOption{
			OptionName:  "contains",
			Description: "Only remove messages containing this text",
			Required:    false,
		},
		&discord.StringOption{
			OptionName:  "from",
			Description: "Older message ID bound",
			Required:    false,
		},
		&discord.StringOption{
			OptionName:  "to",
			Description: "Newer message ID bound",
			Required:    false,
		},
	}
}

func (c *CleanCommand) RequiresGuild() bool { return true }

func (c *CleanCommand) RequiresPermissions() bool { return true }

func (c *CleanCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionManageMessages
}

type EphemeralError struct {
	UserMessage string
	InternalErr error
}

func (e *EphemeralError) Error() string {
	return fmt.Sprintf("%s: %v", e.UserMessage, e.InternalErr)
}

func (e *EphemeralError) Unwrap() error {
	return e.InternalErr
}

func (e *EphemeralError) InteractionResponse() api.InteractionResponse {
	return api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(e.UserMessage),
			Flags:   discord.EphemeralMessage, // 64
		},
	}
}

func (c *CleanCommand) Handle(ctx *legacycore.ArikawaContext) error {
	if !ctx.GuildID.IsValid() {
		return &EphemeralError{UserMessage: "This command must be used in a server.", InternalErr: fmt.Errorf("missing guild_id")}
	}

	enabled, _ := c.configManager.Config().ResolveFeatures(ctx.GuildID.String()).Lookup("moderation.clean")
	if !enabled {
		return &EphemeralError{UserMessage: "Moderation Clean is disabled.", InternalErr: fmt.Errorf("feature moderation.clean is disabled")}
	}

	var count int
	var userID, contains, fromID, toID string

	if ctx.Interaction != nil && ctx.Interaction.Data != nil && ctx.Interaction.Data.InteractionType() == discord.CommandInteractionType {
		cmdData := ctx.Interaction.Data.(*discord.CommandInteraction)
		for _, opt := range cmdData.Options {
			switch opt.Name {
			case "count":
				val, err := opt.IntValue()
				if err == nil {
					count = int(val)
				}
			case "user":
				val, err := opt.SnowflakeValue()
				if err == nil {
					userID = val.String()
				}
			case "contains":
				contains = opt.String()
			case "from":
				fromID = opt.String()
			case "to":
				toID = opt.String()
			}
		}
	}

	if count < 1 || count > 100 {
		return &EphemeralError{UserMessage: "Count must be between 1 and 100.", InternalErr: fmt.Errorf("invalid count %d", count)}
	}

	filter := coreclean.Filter{
		Count:    count,
		UserID:   userID,
		Contains: contains,
		FromID:   fromID,
		ToID:     toID,
	}

	var auditChannel discord.ChannelID
	if ctx.GuildConfig != nil && ctx.GuildConfig.Channels.CleanAction != "" {
		parsed, _ := discord.ParseSnowflake(ctx.GuildConfig.Channels.CleanAction)
		auditChannel = discord.ChannelID(parsed)
	}

	deleted, err := c.cleanExecutor.ExecuteClean(context.Background(), ctx.Interaction.ChannelID, filter, auditChannel, ctx.UserID.String())
	if err != nil {
		return &EphemeralError{UserMessage: "Failed to clean messages.", InternalErr: err}
	}

	msg := fmt.Sprintf("Cleaned %d message(s).", deleted)
	_, editErr := ctx.Client.EditInteractionResponse(ctx.Interaction.AppID, ctx.Interaction.Token, api.EditInteractionResponseData{
		Content: option.NewNullableString(msg),
	})
	if editErr != nil {
		return fmt.Errorf("failed to edit interaction response: %w", editErr)
	}

	return nil
}
