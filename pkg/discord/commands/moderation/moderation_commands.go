package moderation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
)

// RegisterModerationCommands registers slash commands under the /moderation group.
func RegisterModerationCommands(router *core.CommandRouter) {
	checker := core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	moderationGroup := core.NewGroupCommand("moderation", "Moderation commands", checker)

	moderationGroup.AddSubCommand(newBanCommand())
	moderationGroup.AddSubCommand(newMassBanCommand())

	router.RegisterCommand(moderationGroup)
}

type banCommand struct{}

func newBanCommand() *banCommand { return &banCommand{} }

func (c *banCommand) Name() string { return "ban" }

func (c *banCommand) Description() string { return "Ban a user by ID" }

func (c *banCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "User ID to ban",
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

func (c *banCommand) RequiresGuild() bool { return true }

func (c *banCommand) RequiresPermissions() bool { return true }

func (c *banCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	userID, err := extractor.StringRequired("user")
	if err != nil {
		return err
	}

	reason := strings.TrimSpace(extractor.String("reason"))
	if reason == "" {
		reason = "No reason provided"
	}

	if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, userID, reason, 0); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to ban user %s: %v", userID, err), true)
	}

	message := fmt.Sprintf("Banned user `%s`. Reason: %s", userID, reason)
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, message)
}

type massBanCommand struct{}

func newMassBanCommand() *massBanCommand { return &massBanCommand{} }

func (c *massBanCommand) Name() string { return "massban" }

func (c *massBanCommand) Description() string { return "Ban multiple users by ID" }

func (c *massBanCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "members",
			Description: "Space or comma separated user IDs",
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

func (c *massBanCommand) RequiresGuild() bool { return true }

func (c *massBanCommand) RequiresPermissions() bool { return true }

func (c *massBanCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	membersInput, err := extractor.StringRequired("members")
	if err != nil {
		return err
	}

	memberIDs := parseMemberIDs(membersInput)
	if len(memberIDs) == 0 {
		return core.NewCommandError("No member IDs provided", true)
	}

	reason := strings.TrimSpace(extractor.String("reason"))
	if reason == "" {
		reason = "No reason provided"
	}

	var failed []string
	for _, memberID := range memberIDs {
		if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, memberID, reason, 0); err != nil {
			failed = append(failed, fmt.Sprintf("%s (%v)", memberID, err))
		}
	}

	message := buildMassBanMessage(memberIDs, failed, reason)
	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, message)
}

func parseMemberIDs(input string) []string {
	rawIDs := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\n' || r == '\t'
	})

	unique := make(map[string]struct{})
	for _, id := range rawIDs {
		clean := strings.TrimSpace(id)
		if clean == "" {
			continue
		}
		unique[clean] = struct{}{}
	}

	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func buildMassBanMessage(memberIDs, failed []string, reason string) string {
	message := fmt.Sprintf("Banned %d user(s). Reason: %s", len(memberIDs)-len(failed), reason)
	if len(failed) == 0 {
		return message
	}

	return fmt.Sprintf("%s\nFailed: %s", message, strings.Join(failed, "; "))
}
