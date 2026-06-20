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

// CleanExecutor defines the execution bounds for a concrete deletion service.
type CleanExecutor interface {
	ExecuteClean(ctx context.Context, channelID discord.ChannelID, filter coreclean.Filter, auditChannelID discord.ChannelID, requestedBy string) (int, error)
}

// CleanCommand bridges the Discord Slash Command interaction to the bounded clean executor.
type CleanCommand struct {
	configManager *files.ConfigManager
	cleanExecutor CleanExecutor
}

// NewCleanCommand initializes a router-compatible clean interaction handler.
func NewCleanCommand(cfg *files.ConfigManager, executor CleanExecutor) *CleanCommand {
	return &CleanCommand{
		configManager: cfg,
		cleanExecutor: executor,
	}
}

// Name provides the exact command identifier as registered with the Discord API.
func (c *CleanCommand) Name() string { return "clean" }

// Description provides the user-facing command description for the Discord UI.
func (c *CleanCommand) Description() string { return "Delete recent messages in this channel" }

// Options structures the argument signature demanded by Discord for this slash command.
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

// RequiresGuild prevents this command from executing in Direct Messages.
func (c *CleanCommand) RequiresGuild() bool { return true }

// RequiresPermissions enforces that the bot itself possesses adequate context permissions.
func (c *CleanCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions scopes execution to users bearing moderation capabilities.
func (c *CleanCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionManageMessages
}

// EphemeralError satisfies the standard error interface while retaining sufficient metadata to render private UI feedback to the calling user without exposing stack traces.
type EphemeralError struct {
	UserMessage string
	InternalErr error
}

// Error outputs the composite diagnostic error strictly for backend telemetry.
func (e *EphemeralError) Error() string {
	return fmt.Sprintf("%s: %v", e.UserMessage, e.InternalErr)
}

// Unwrap enables standard library functions like errors.Is and errors.As to probe the underlying network or infrastructure failure.
func (e *EphemeralError) Unwrap() error {
	return e.InternalErr
}

// InteractionResponse constructs the UI payload dynamically applying the bitwise MessageFlagEphemeral.
func (e *EphemeralError) InteractionResponse() api.InteractionResponse {
	return api.InteractionResponse{
		Type: api.MessageInteractionWithSource,
		Data: &api.InteractionResponseData{
			Content: option.NewNullableString(e.UserMessage),
			Flags:   discord.EphemeralMessage, // 64
		},
	}
}

// Handle parses the interaction event, asserts operational preconditions, maps the user payload into a domain Filter, and hands off to the Service executor.
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
				if opt.Type != discord.IntegerOptionType {
					return &EphemeralError{UserMessage: "Invalid format for count.", InternalErr: fmt.Errorf("structural anomaly: expected IntegerOptionType for count")}
				}
				val, err := opt.IntValue()
				if err == nil {
					count = int(val)
				}
			case "user":
				if opt.Type != discord.UserOptionType {
					return &EphemeralError{UserMessage: "Invalid format for user.", InternalErr: fmt.Errorf("structural anomaly: expected UserOptionType for user")}
				}
				val, err := opt.SnowflakeValue()
				if err == nil {
					userID = val.String()
				}
			case "contains":
				if opt.Type != discord.StringOptionType {
					return &EphemeralError{UserMessage: "Invalid format for contains.", InternalErr: fmt.Errorf("structural anomaly: expected StringOptionType for contains")}
				}
				contains = opt.String()
			case "from":
				if opt.Type != discord.StringOptionType {
					return &EphemeralError{UserMessage: "Invalid format for from.", InternalErr: fmt.Errorf("structural anomaly: expected StringOptionType for from")}
				}
				fromID = opt.String()
			case "to":
				if opt.Type != discord.StringOptionType {
					return &EphemeralError{UserMessage: "Invalid format for to.", InternalErr: fmt.Errorf("structural anomaly: expected StringOptionType for to")}
				}
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
