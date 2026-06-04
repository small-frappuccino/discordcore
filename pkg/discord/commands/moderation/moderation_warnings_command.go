package moderation

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

type warningsCommand struct{}

func newWarningsCommand() *warningsCommand { return &warningsCommand{} }

// Name names.
func (c *warningsCommand) Name() string { return "warnings" }

// Description descriptions.
func (c *warningsCommand) Description() string { return "List recent warnings for a member" }

// Options options.
func (c *warningsCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Member ID or mention to inspect",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "limit",
			Description: "How many recent warnings to show (default 5, max 25)",
			Required:    false,
			MinValue:    floatPtr(1),
			MaxValue:    25,
		},
	}
}

// RequiresGuild requires guild.
func (c *warningsCommand) RequiresGuild() bool { return true }

// RequiresPermissions requires permissions.
func (c *warningsCommand) RequiresPermissions() bool { return true }

// DefaultMemberPermissions defaults member permissions.
func (c *warningsCommand) DefaultMemberPermissions() int64 {
	return discordgo.PermissionManageMessages
}

// Handle handles.
func (c *warningsCommand) Handle(ctx *core.Context) error {
	if enabled, _ := ctx.Config.Config().ResolveFeatures(ctx.GuildID).Lookup("moderation.warnings"); !enabled {
		return core.NewCommandError("Warnings command is disabled for this server.", true)
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

	limit := int(extractor.Int("limit"))
	if limit <= 0 {
		limit = 5
	}

	warnCtx, err := prepareWarnContext(ctx)
	if err != nil {
		return fmt.Errorf("warningsCommand.Handle: %w", err)
	}

	if ok, reasonText := canWarnTarget(ctx, warnCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot inspect warnings for `%s`: %s.", userID, reasonText), true)
	}

	store := moderationStoreFromContext(ctx)
	if store == nil {
		return core.NewCommandError("Warnings storage is not available for this bot instance.", true)
	}

	targetUsername := resolveUserDisplayName(ctx, userID)
	warnings, err := store.ListModerationWarnings(ctx.GuildID, userID, limit)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to load warnings for %s: %v", userID, err), true)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, buildWarningsCommandMessage(targetUsername, warnings))
}
