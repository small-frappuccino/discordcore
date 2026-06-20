package moderation

import (
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordgo"
)

type warnCommand struct{}

func newWarnCommand() *warnCommand { return &warnCommand{} }

// Name names.
func (c *warnCommand) Name() string { return "warn" }

// Description descriptions.
func (c *warnCommand) Description() string { return "Record a warning for a member" }

// Options options.
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

// RequiresGuild requires guild.
func (c *warnCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *warnCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *warnCommand) DefaultMemberPermissions() int64 { return discordgo.PermissionManageMessages }

// Handle handles.
func (c *warnCommand) Handle(ctx *legacycore.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.warn"); !enabled {
		return legacycore.NewMissingConfigError(ctx.GuildID, "Moderation Warn", "/moderation")
	}
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

	warnCtx, err := prepareWarnContext(ctx)
	if err != nil {
		return fmt.Errorf("warnCommand.Handle: %w", err)
	}

	if ok, reasonText := canWarnTarget(ctx, warnCtx, userID); !ok {
		return &legacycore.CommandError{Message: fmt.Sprintf("Cannot warn `%s`: %s.", userID, reasonText), Ephemeral: true}
	}

	store := moderationStoreFromContext(ctx)
	if store == nil {
		return &legacycore.CommandError{Message: "Warnings storage is not available for this bot instance.", Ephemeral: true}
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	warning, err := store.CreateModerationWarning(ctx.GuildID, userID, ctx.UserID, reason, time.Now().UTC())
	if err != nil {
		return &legacycore.CommandError{Message: fmt.Sprintf("Failed to create warning for %s: %v", userID, err), Ephemeral: true}
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

	return legacycore.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildWarnCommandMessage(targetUsername, warning.CaseNumber, reason, truncated))
}
