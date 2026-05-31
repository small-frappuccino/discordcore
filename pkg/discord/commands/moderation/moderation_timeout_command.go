package moderation

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type timeoutCommand struct{}

func newTimeoutCommand() *timeoutCommand { return &timeoutCommand{} }

func (c *timeoutCommand) Name() string { return "timeout" }

func (c *timeoutCommand) Description() string { return "Timeout a member" }

func (c *timeoutCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Member ID or mention to timeout",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "minutes",
			Description: "Timeout duration in minutes (max 40320)",
			Required:    true,
			MinValue:    floatPtr(1),
			MaxValue:    float64(timeoutMaxMinutes),
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the timeout",
			Required:    false,
		},
	}
}

func (c *timeoutCommand) RequiresGuild() bool { return true }

func (c *timeoutCommand) RequiresPermissions() bool { return true }

func (c *timeoutCommand) DefaultMemberPermissions() int64 {
	return discordgo.PermissionModerateMembers
}

func (c *timeoutCommand) Handle(ctx *core.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.timeout"); !enabled {
		return core.NewCommandError("Timeout command is disabled for this server.", true)
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	minutes := extractor.Int("minutes")
	if minutes <= 0 {
		return core.NewCommandError("Please provide a valid timeout duration in minutes.", true)
	}
	if minutes > timeoutMaxMinutes {
		return core.NewCommandError("Timeout duration cannot exceed 40320 minutes (28 days).", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	timeoutCtx, err := prepareTimeoutContext(ctx)
	if err != nil {
		return fmt.Errorf("timeoutCommand.Handle: %w", err)
	}

	if ok, reasonText := canTimeoutTarget(ctx, timeoutCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot timeout `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	until := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	if err := ctx.Session.GuildMemberTimeout(ctx.GuildID, userID, &until, discordgo.WithAuditLogReason(reason)); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to timeout user %s: %v", userID, err), true)
	}

	details := fmt.Sprintf("Duration: %s | Ends: <t:%d:F> (<t:%d:R>)", formatTimeoutDuration(minutes), until.Unix(), until.Unix())
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "timeout",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildTimeoutCommandMessage(targetUsername, minutes, reason, truncated))
}
