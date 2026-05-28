package moderation

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type muteCommand struct{}

func newMuteCommand() *muteCommand { return &muteCommand{} }

func (c *muteCommand) Name() string { return "mute" }

func (c *muteCommand) Description() string { return "Apply the configured mute role to a member" }

func (c *muteCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Member ID or mention to mute",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the mute",
			Required:    false,
		},
	}
}

func (c *muteCommand) RequiresGuild() bool { return true }

func (c *muteCommand) RequiresPermissions() bool { return true }

func (c *muteCommand) DefaultMemberPermissions() int64 { return discordgo.PermissionManageRoles }

func (c *muteCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	muteCtx, err := prepareMuteContext(ctx)
	if err != nil {
		return err
	}

	muteRole, roleID, err := resolveConfiguredMuteRole(ctx, muteCtx)
	if err != nil {
		return err
	}

	if ok, reasonText := canMuteTarget(ctx, muteCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot mute `%s`: %s.", userID, reasonText), true)
	}

	targetMember, ok, reasonText := resolveRoleTargetMember(ctx, userID)
	if !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot mute `%s`: %s.", userID, reasonText), true)
	}
	if memberHasRole(targetMember, roleID) {
		return core.NewCommandError(fmt.Sprintf("Cannot mute `%s`: target already has the configured mute role.", userID), true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	if err := ctx.Session.GuildMemberRoleAdd(ctx.GuildID, userID, roleID); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to mute user %s: %v", userID, err), true)
	}

	details := fmt.Sprintf("Role applied: %s (`%s`)", formatRoleDisplayName(muteRole), roleID)
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "mute",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildMuteCommandMessage(targetUsername, muteRole, reason, truncated))
}
