package moderation

import (
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordgo"
)

type banCommand struct{}

func newBanCommand() *banCommand { return &banCommand{} }

// Name names.
func (c *banCommand) Name() string { return "ban" }

// Description descriptions.
func (c *banCommand) Description() string { return "Ban a user by ID or mention" }

// Options options.
func (c *banCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "User ID or mention to ban",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the ban",
			Required:    false,
		},
	}
}

// RequiresGuild requires guild.
func (c *banCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *banCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *banCommand) DefaultMemberPermissions() int64 { return discordgo.PermissionBanMembers }

// Handle handles.
func (c *banCommand) Handle(ctx *core.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.ban"); !enabled {
		return core.NewMissingConfigError(ctx.GuildID, "Moderation Ban", "/moderation")
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

	reason, truncated := sanitizeReason(extractor.String("reason"))

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return fmt.Errorf("banCommand.Handle: %w", err)
	}

	if ok, reasonText := canBanTarget(ctx, banCtx, userID); !ok {
		return &core.CommandError{Message: fmt.Sprintf("Cannot ban `%s`: %s.", userID, reasonText), Ephemeral: true}
	}

	targetUsername := resolveUserDisplayName(ctx, userID)

	if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, userID, reason, 0); err != nil {
		return &core.CommandError{Message: fmt.Sprintf("Failed to ban user %s: %v", userID, err), Ephemeral: true}
	}

	details := "Status: Success"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "member_ban_add",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildBanCommandMessage(targetUsername, reason, truncated))
}
