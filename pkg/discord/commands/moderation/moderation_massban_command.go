package moderation

import (
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

type massBanCommand struct{}

func newMassBanCommand() *massBanCommand { return &massBanCommand{} }

// Name names.
func (c *massBanCommand) Name() string { return "massban" }

// Description descriptions.
func (c *massBanCommand) Description() string { return "Ban multiple users by ID or mention" }

// Options options.
func (c *massBanCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "members",
			Description: "Space, comma, or semicolon separated user IDs or mentions",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the bans",
			Required:    false,
		},
	}
}

// RequiresGuild requires guild.
func (c *massBanCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *massBanCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *massBanCommand) DefaultMemberPermissions() int64 { return discordgo.PermissionBanMembers }

// Handle handles.
func (c *massBanCommand) Handle(ctx *core.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.massban"); !enabled {
		return &core.CommandError{Message: "Mass ban command is disabled for this server.", Ephemeral: true}
	}
	extractor := core.OptionList(core.GetSubCommandOptions(ctx.Interaction))

	membersInput, err := extractor.StringRequired("members")
	if err != nil {
		return &core.CommandError{Message: err.Error(), Ephemeral: true}
	}

	memberIDs, invalidTokens := parseMemberIDs(membersInput)
	if len(memberIDs) == 0 {
		return &core.CommandError{Message: "No valid member IDs provided", Ephemeral: true}
	}
	if len(invalidTokens) > 0 {
		log.ApplicationLogger().Info("Massban ignored invalid member tokens", "guildID", ctx.GuildID, "invalid_count", len(invalidTokens))
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return fmt.Errorf("massBanCommand.Handle: %w", err)
	}

	bannedCount := 0
	var failed []string
	var skipped []string
	for _, memberID := range memberIDs {
		targetUsername := resolveUserDisplayName(ctx, memberID)
		logPayload := moderationLogPayload{
			Action:      "member_ban_add",
			TargetID:    memberID,
			TargetLabel: targetUsername,
			Reason:      reason,
			RequestedBy: ctx.UserID,
		}

		ok, reasonText := canBanTarget(ctx, banCtx, memberID)
		if !ok {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", memberID, reasonText))
			logPayload.Extra = "Status: Skipped | " + reasonText
			sendModerationCaseActionLog(ctx, logPayload)
			continue
		}

		if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, memberID, reason, 0); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", memberID, err))
			logPayload.Extra = fmt.Sprintf("Status: Failed | %v", err)
			sendModerationCaseActionLog(ctx, logPayload)
			continue
		}
		bannedCount++
		logPayload.Extra = "Status: Success"
		if truncated {
			logPayload.Extra += " | Reason truncated to 512 characters"
		}
		sendModerationCaseActionLog(ctx, logPayload)
	}
	if len(skipped) > 0 || len(failed) > 0 {
		log.ApplicationLogger().Info(
			"Massban finished with partial failures",
			"guildID", ctx.GuildID,
			"requested", len(memberIDs),
			"banned", bannedCount,
			"skipped", len(skipped),
			"failed", len(failed),
		)
	}
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildMassBanCommandMessage(bannedCount))
}
