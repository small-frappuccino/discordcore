package moderation

import (
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordgo"
)

type muteCommand struct{}

func newMuteCommand() *muteCommand { return &muteCommand{} }

// Name names.
func (c *muteCommand) Name() string { return "mute" }

// Description descriptions.
func (c *muteCommand) Description() string { return "Apply the configured mute role to a member" }

// Options options.
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

// RequiresGuild requires guild.
func (c *muteCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *muteCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *muteCommand) DefaultMemberPermissions() int64 { return discordgo.PermissionManageRoles }

// Handle handles.
func (c *muteCommand) Handle(ctx *legacycore.Context) error {
	extractor := legacycore.OptionList(legacycore.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return &legacycore.CommandError{Message: err.Error(), Ephemeral: true}
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return &legacycore.CommandError{Message: "Invalid user ID or mention.", Ephemeral: true}
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	muteCtx, err := prepareMuteContext(ctx)
	if err != nil {
		return fmt.Errorf("muteCommand.Handle: %w", err)
	}

	muteRole, roleID, err := resolveConfiguredMuteRole(ctx, muteCtx)
	if err != nil {
		return fmt.Errorf("muteCommand.Handle: %w", err)
	}

	if ok, reasonText := canMuteTarget(ctx, muteCtx, userID); !ok {
		return &legacycore.CommandError{Message: fmt.Sprintf("Cannot mute `%s`: %s.", userID, reasonText), Ephemeral: true}
	}

	targetMember, ok, reasonText := resolveRoleTargetMember(ctx, userID)
	if !ok {
		return &legacycore.CommandError{Message: fmt.Sprintf("Cannot mute `%s`: %s.", userID, reasonText), Ephemeral: true}
	}
	if memberHasRole(targetMember, roleID) {
		return &legacycore.CommandError{Message: fmt.Sprintf("Cannot mute `%s`: target already has the configured mute role.", userID), Ephemeral: true}
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	if err := ctx.Session.GuildMemberRoleAdd(ctx.GuildID, userID, roleID); err != nil {
		return &legacycore.CommandError{Message: fmt.Sprintf("Failed to mute user %s: %v", userID, err), Ephemeral: true}
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

	return legacycore.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildMuteCommandMessage(targetUsername, muteRole, reason, truncated))
}
