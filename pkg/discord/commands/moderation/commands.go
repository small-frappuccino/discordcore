package moderation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/cmd"
	discordmod "github.com/small-frappuccino/discordcore/pkg/discord/moderation"
	coremod "github.com/small-frappuccino/discordcore/pkg/moderation"
)

// Metrics defines observability hooks for moderation commands.
type Metrics interface {
	RecordCommandExec(name string)
}

// NopMetrics provides a nil-safe implementation of Metrics.
type NopMetrics struct{}

func (NopMetrics) RecordCommandExec(name string) {}

// InMemoryMetrics implements Metrics and lifecycle hooks for the pipeline.
type InMemoryMetrics struct{}

func (m *InMemoryMetrics) RecordCommandExec(name string)    {}
func (m *InMemoryMetrics) Attach(ctx context.Context) error { return nil }

// NewCommandGroup aggregates the moderation commands.
func NewCommandGroup(svc *discordmod.Service, metrics Metrics, logger *slog.Logger) cmd.CommandGroup {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return commands.NewLegacyAdapter(
		&BanCommand{service: svc, metrics: metrics, logger: logger},
		&TimeoutCommand{service: svc, metrics: metrics, logger: logger},
		&MassBanCommand{service: svc, metrics: metrics, logger: logger},
	)
}

// NewBanCommand is deprecated.
func NewBanCommand(svc *discordmod.Service, metrics Metrics, logger *slog.Logger) *BanCommand {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &BanCommand{service: svc, metrics: metrics, logger: logger}
}

// BanCommand encapsulates the `/ban` slash command execution.
type BanCommand struct {
	service *discordmod.Service
	metrics Metrics
	logger  *slog.Logger
}

func (c *BanCommand) Name() string        { return "ban" }
func (c *BanCommand) Description() string { return "Ban a user from the server" }
func (c *BanCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.UserOption{
			OptionName:  "user",
			Description: "User to ban",
			Required:    true,
		},
		&discord.StringOption{
			OptionName:  "reason",
			Description: "Reason for the ban",
			Required:    false,
		},
	}
}

func (c *BanCommand) RequiresGuild() bool       { return true }
func (c *BanCommand) RequiresPermissions() bool { return true }
func (c *BanCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionBanMembers
}

func (c *BanCommand) Handle(ctx *commands.ArikawaContext) error {
	c.metrics.RecordCommandExec("ban")

	if !ctx.GuildID.IsValid() {
		return fmt.Errorf("must be used in a server")
	}

	var userID discord.UserID
	var reason string

	if ctx.Interaction != nil && ctx.Interaction.Data != nil && ctx.Interaction.Data.InteractionType() == discord.CommandInteractionType {
		cmdData := ctx.Interaction.Data.(*discord.CommandInteraction)
		for _, opt := range cmdData.Options {
			switch opt.Name {
			case "user":
				val, err := opt.SnowflakeValue()
				if err == nil {
					userID = discord.UserID(val)
				}
			case "reason":
				reason = opt.String()
			}
		}
	}

	if !userID.IsValid() {
		return respondEphemeral(ctx, "Invalid user specified.")
	}

	c.logger.Info("Architectural state transition: Executing moderation action from slash command",
		slog.String("command", "ban"),
		slog.String("guild_id", ctx.GuildID.String()),
		slog.String("target_id", userID.String()),
	)

	err := c.service.Ban(context.Background(), ctx.GuildID, userID, 0, reason)
	if err != nil {
		c.logger.Error("Blocking structural failure: Ban command execution aborted",
			slog.String("guild_id", ctx.GuildID.String()),
			slog.String("error", err.Error()),
		)
		return respondEphemeral(ctx, "Failed to ban the user.")
	}

	return respondEphemeral(ctx, fmt.Sprintf("Successfully banned user %s.", userID))
}

// TimeoutCommand encapsulates the `/timeout` slash command execution.
type TimeoutCommand struct {
	service *discordmod.Service
	metrics Metrics
	logger  *slog.Logger
}

func NewTimeoutCommand(svc *discordmod.Service, metrics Metrics, logger *slog.Logger) *TimeoutCommand {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &TimeoutCommand{service: svc, metrics: metrics, logger: logger}
}

func (c *TimeoutCommand) Name() string        { return "timeout" }
func (c *TimeoutCommand) Description() string { return "Timeout a user in the server" }
func (c *TimeoutCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.UserOption{
			OptionName:  "user",
			Description: "User to timeout",
			Required:    true,
		},
		&discord.IntegerOption{
			OptionName:  "minutes",
			Description: "Duration in minutes",
			Required:    true,
			Min:         option.NewInt(1),
		},
	}
}

func (c *TimeoutCommand) RequiresGuild() bool       { return true }
func (c *TimeoutCommand) RequiresPermissions() bool { return true }
func (c *TimeoutCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionModerateMembers
}

func (c *TimeoutCommand) Handle(ctx *commands.ArikawaContext) error {
	c.metrics.RecordCommandExec("timeout")

	var userID discord.UserID
	var minutes int

	if ctx.Interaction != nil && ctx.Interaction.Data != nil && ctx.Interaction.Data.InteractionType() == discord.CommandInteractionType {
		cmdData := ctx.Interaction.Data.(*discord.CommandInteraction)
		for _, opt := range cmdData.Options {
			switch opt.Name {
			case "user":
				val, err := opt.SnowflakeValue()
				if err == nil {
					userID = discord.UserID(val)
				}
			case "minutes":
				val, err := opt.IntValue()
				if err == nil {
					minutes = int(val)
				}
			}
		}
	}

	if !userID.IsValid() {
		return respondEphemeral(ctx, "Invalid user specified.")
	}

	until := discord.NewTimestamp(time.Now().Add(time.Duration(minutes) * time.Minute))

	c.logger.Info("Architectural state transition: Executing moderation action from slash command",
		slog.String("command", "timeout"),
		slog.String("guild_id", ctx.GuildID.String()),
		slog.String("target_id", userID.String()),
	)

	err := c.service.Timeout(context.Background(), ctx.GuildID, userID, until)
	if err != nil {
		c.logger.Error("Blocking structural failure: Timeout command execution aborted",
			slog.String("guild_id", ctx.GuildID.String()),
			slog.String("error", err.Error()),
		)
		return respondEphemeral(ctx, "Failed to timeout the user.")
	}

	return respondEphemeral(ctx, fmt.Sprintf("Successfully timed out user %s.", userID))
}

func respondEphemeral(ctx *commands.ArikawaContext, msg string) error {
	_, err := ctx.Client.EditInteractionResponse(ctx.Interaction.AppID, ctx.Interaction.Token, api.EditInteractionResponseData{
		Content: option.NewNullableString(msg),
	})
	return err
}

// MassBanCommand encapsulates the `/massban` execution utilizing core logic.
type MassBanCommand struct {
	service *discordmod.Service
	metrics Metrics
	logger  *slog.Logger
}

func NewMassBanCommand(svc *discordmod.Service, metrics Metrics, logger *slog.Logger) *MassBanCommand {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &MassBanCommand{service: svc, metrics: metrics, logger: logger}
}

func (c *MassBanCommand) Name() string        { return "massban" }
func (c *MassBanCommand) Description() string { return "Ban multiple users at once" }
func (c *MassBanCommand) Options() []discord.CommandOption {
	return []discord.CommandOption{
		&discord.StringOption{
			OptionName:  "users",
			Description: "Comma separated list of user IDs",
			Required:    true,
		},
	}
}

func (c *MassBanCommand) RequiresGuild() bool       { return true }
func (c *MassBanCommand) RequiresPermissions() bool { return true }
func (c *MassBanCommand) DefaultMemberPermissions() discord.Permissions {
	return discord.PermissionBanMembers
}

func (c *MassBanCommand) Handle(ctx *commands.ArikawaContext) error {
	c.metrics.RecordCommandExec("massban")

	var rawUsers string
	if ctx.Interaction != nil && ctx.Interaction.Data != nil && ctx.Interaction.Data.InteractionType() == discord.CommandInteractionType {
		cmdData := ctx.Interaction.Data.(*discord.CommandInteraction)
		for _, opt := range cmdData.Options {
			if opt.Name == "users" {
				rawUsers = opt.String()
			}
		}
	}

	// Delegate ID normalization to the purely Discord-agnostic core package
	validIDs, _ := coremod.ParseMemberIDs(rawUsers)

	c.logger.Info("Architectural state transition: Executing mass moderation action from slash command",
		slog.String("command", "massban"),
		slog.String("guild_id", ctx.GuildID.String()),
		slog.Int("target_count", len(validIDs)),
	)

	for _, idStr := range validIDs {
		sf, err := discord.ParseSnowflake(idStr)
		if err == nil {
			_ = c.service.Ban(context.Background(), ctx.GuildID, discord.UserID(sf), 0, "Massban")
		}
	}

	return respondEphemeral(ctx, fmt.Sprintf("Massban processed %d users.", len(validIDs)))
}
