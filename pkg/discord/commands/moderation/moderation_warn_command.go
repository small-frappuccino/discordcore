package moderation

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type warnCommand struct{}

func newWarnCommand() *warnCommand { return &warnCommand{} }

func (c *warnCommand) Name() string { return "warn" }

func (c *warnCommand) Description() string { return "Record a warning for a member" }

func (c *warnCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Member ID or mention to warn",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the warning",
			Required:    false,
		},
	}
}

func (c *warnCommand) RequiresGuild() bool { return true }

func (c *warnCommand) RequiresPermissions() bool { return true }

func (c *warnCommand) DefaultMemberPermissions() int64 { return discordgo.PermissionManageMessages }

func (c *warnCommand) Handle(ctx *core.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.warn"); !enabled {
		return core.NewCommandError("Warn command is disabled for this server.", true)
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

	warnCtx, err := prepareWarnContext(ctx)
	if err != nil {
		return fmt.Errorf("warnCommand.Handle: %w", err)
	}

	if ok, reasonText := canWarnTarget(ctx, warnCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot warn `%s`: %s.", userID, reasonText), true)
	}

	store := moderationStoreFromContext(ctx)
	if store == nil {
		return core.NewCommandError("Warnings storage is not available for this bot instance.", true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	warning, err := store.CreateModerationWarning(ctx.GuildID, userID, ctx.UserID, reason, time.Now().UTC())
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to create warning for %s: %v", userID, err), true)
	}

	details := "Warning recorded"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:        "warn",
		TargetID:      userID,
		TargetLabel:   targetUsername,
		Reason:        reason,
		RequestedBy:   ctx.UserID,
		Extra:         details,
		CaseNumber:    warning.CaseNumber,
		HasCaseNumber: true,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildWarnCommandMessage(targetUsername, warning.CaseNumber, reason, truncated))
}
