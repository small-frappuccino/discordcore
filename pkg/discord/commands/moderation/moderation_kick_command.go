package moderation

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type kickCommand struct{}

func newKickCommand() *kickCommand { return &kickCommand{} }

// Name names.
func (c *kickCommand) Name() string { return "kick" }

// Description descriptions.
func (c *kickCommand) Description() string { return "Kick a member by ID or mention" }

// Options options.
func (c *kickCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Member ID or mention to kick",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the kick",
			Required:    false,
		},
	}
}

// RequiresGuild requires guild.
func (c *kickCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *kickCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *kickCommand) DefaultMemberPermissions() int64 { return discordgo.PermissionKickMembers }

// Handle handles.
func (c *kickCommand) Handle(ctx *core.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.kick"); !enabled {
		return core.NewCommandError("Kick command is disabled for this server.", true)
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

	reason, truncated := sanitizeReason(extractor.String("reason"))

	kickCtx, err := prepareKickContext(ctx)
	if err != nil {
		return fmt.Errorf("kickCommand.Handle: %w", err)
	}

	if ok, reasonText := canKickTarget(ctx, kickCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot kick `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	if err := ctx.Session.GuildMemberDeleteWithReason(ctx.GuildID, userID, reason); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to kick user %s: %v", userID, err), true)
	}

	details := "Status: Success"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "kick",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildKickCommandMessage(targetUsername, reason, truncated))
}
