package moderation

import (
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordgo"
)

type timeoutCommand struct{}

func newTimeoutCommand() *timeoutCommand { return &timeoutCommand{} }

// Name names.
func (c *timeoutCommand) Name() string { return "timeout" }

// Description descriptions.
func (c *timeoutCommand) Description() string { return "Timeout a member" }

// Options options.
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
			MinValue:    new(float64(1)),
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

// RequiresGuild requires guild.
func (c *timeoutCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *timeoutCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *timeoutCommand) DefaultMemberPermissions() int64 {
	return discordgo.PermissionModerateMembers
}

// Handle handles.
func (c *timeoutCommand) Handle(ctx *core.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.timeout"); !enabled {
		return core.NewMissingConfigError(ctx.GuildID, "Moderation Timeout", "/moderation")
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return &core.CommandError{Message: err.Error(), Ephemeral: true}
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return &core.CommandError{Message: "Invalid user ID or mention.", Ephemeral: true}
	}

	minutes := extractor.Int("minutes")
	if minutes <= 0 {
		return &core.CommandError{Message: "Please provide a valid timeout duration in minutes.", Ephemeral: true}
	}
	if minutes > timeoutMaxMinutes {
		return &core.CommandError{Message: "Timeout duration cannot exceed 40320 minutes (28 days).", Ephemeral: true}
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	timeoutCtx, err := prepareTimeoutContext(ctx)
	if err != nil {
		return fmt.Errorf("timeoutCommand.Handle: %w", err)
	}

	if ok, reasonText := canTimeoutTarget(ctx, timeoutCtx, userID); !ok {
		return &core.CommandError{Message: fmt.Sprintf("Cannot timeout `%s`: %s.", userID, reasonText), Ephemeral: true}
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	until := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	if err := ctx.Session.GuildMemberTimeout(ctx.GuildID, userID, &until, discordgo.WithAuditLogReason(reason)); err != nil {
		return &core.CommandError{Message: fmt.Sprintf("Failed to timeout user %s: %v", userID, err), Ephemeral: true}
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
