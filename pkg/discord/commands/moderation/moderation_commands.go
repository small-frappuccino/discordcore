package moderation

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cleanup"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

const (
	maxAuditLogReasonLen = 512
	minSnowflakeLength   = 15
	maxSnowflakeLength   = 21
	cleanMaxDelete       = 100
	cleanMaxFetch        = 500
	timeoutMaxMinutes    = 28 * 24 * 60
)

var userMentionRe = regexp.MustCompile(`^<@!?(\d+)>$`)

var moderationActionTypeLabels = map[discordgo.AuditLogAction]string{
	discordgo.AuditLogActionGuildUpdate: "Guild Update",

	discordgo.AuditLogActionChannelCreate:          "Channel Create",
	discordgo.AuditLogActionChannelUpdate:          "Channel Update",
	discordgo.AuditLogActionChannelDelete:          "Channel Delete",
	discordgo.AuditLogActionChannelOverwriteCreate: "Channel Overwrite Create",
	discordgo.AuditLogActionChannelOverwriteUpdate: "Channel Overwrite Update",
	discordgo.AuditLogActionChannelOverwriteDelete: "Channel Overwrite Delete",

	discordgo.AuditLogActionMemberKick:       "Member Kick",
	discordgo.AuditLogActionMemberPrune:      "Member Prune",
	discordgo.AuditLogActionMemberBanAdd:     "Member Ban Add",
	discordgo.AuditLogActionMemberBanRemove:  "Member Ban Remove",
	discordgo.AuditLogActionMemberUpdate:     "Member Update",
	discordgo.AuditLogActionMemberRoleUpdate: "Member Role Update",
	discordgo.AuditLogActionMemberMove:       "Member Move",
	discordgo.AuditLogActionMemberDisconnect: "Member Disconnect",
	discordgo.AuditLogActionBotAdd:           "Bot Add",

	discordgo.AuditLogActionRoleCreate: "Role Create",
	discordgo.AuditLogActionRoleUpdate: "Role Update",
	discordgo.AuditLogActionRoleDelete: "Role Delete",

	discordgo.AuditLogActionInviteCreate: "Invite Create",
	discordgo.AuditLogActionInviteUpdate: "Invite Update",
	discordgo.AuditLogActionInviteDelete: "Invite Delete",

	discordgo.AuditLogActionWebhookCreate: "Webhook Create",
	discordgo.AuditLogActionWebhookUpdate: "Webhook Update",
	discordgo.AuditLogActionWebhookDelete: "Webhook Delete",

	discordgo.AuditLogActionEmojiCreate: "Emoji Create",
	discordgo.AuditLogActionEmojiUpdate: "Emoji Update",
	discordgo.AuditLogActionEmojiDelete: "Emoji Delete",

	discordgo.AuditLogActionMessageDelete:     "Message Delete",
	discordgo.AuditLogActionMessageBulkDelete: "Message Bulk Delete",
	discordgo.AuditLogActionMessagePin:        "Message Pin",
	discordgo.AuditLogActionMessageUnpin:      "Message Unpin",

	discordgo.AuditLogActionIntegrationCreate:   "Integration Create",
	discordgo.AuditLogActionIntegrationUpdate:   "Integration Update",
	discordgo.AuditLogActionIntegrationDelete:   "Integration Delete",
	discordgo.AuditLogActionStageInstanceCreate: "Stage Instance Create",
	discordgo.AuditLogActionStageInstanceUpdate: "Stage Instance Update",
	discordgo.AuditLogActionStageInstanceDelete: "Stage Instance Delete",

	discordgo.AuditLogActionStickerCreate: "Sticker Create",
	discordgo.AuditLogActionStickerUpdate: "Sticker Update",
	discordgo.AuditLogActionStickerDelete: "Sticker Delete",

	discordgo.AuditLogAction(discordgo.AuditLogGuildScheduledEventCreate): "Guild Scheduled Event Create",
	discordgo.AuditLogAction(discordgo.AuditLogGuildScheduledEventUpdate): "Guild Scheduled Event Update",
	discordgo.AuditLogAction(discordgo.AuditLogGuildScheduledEventDelete): "Guild Scheduled Event Delete",

	discordgo.AuditLogActionThreadCreate: "Thread Create",
	discordgo.AuditLogActionThreadUpdate: "Thread Update",
	discordgo.AuditLogActionThreadDelete: "Thread Delete",

	discordgo.AuditLogActionApplicationCommandPermissionUpdate: "Application Command Permission Update",

	discordgo.AuditLogActionAutoModerationRuleCreate:                "Auto Moderation Rule Create",
	discordgo.AuditLogActionAutoModerationRuleUpdate:                "Auto Moderation Rule Update",
	discordgo.AuditLogActionAutoModerationRuleDelete:                "Auto Moderation Rule Delete",
	discordgo.AuditLogActionAutoModerationBlockMessage:              "Auto Moderation Block Message",
	discordgo.AuditLogActionAutoModerationFlagToChannel:             "Auto Moderation Flag To Channel",
	discordgo.AuditLogActionAutoModerationUserCommunicationDisabled: "Auto Moderation User Communication Disabled",

	discordgo.AuditLogActionCreatorMonetizationRequestCreated: "Creator Monetization Request Created",
	discordgo.AuditLogActionCreatorMonetizationTermsAccepted:  "Creator Monetization Terms Accepted",

	discordgo.AuditLogActionOnboardingPromptCreate: "Onboarding Prompt Create",
	discordgo.AuditLogActionOnboardingPromptUpdate: "Onboarding Prompt Update",
	discordgo.AuditLogActionOnboardingPromptDelete: "Onboarding Prompt Delete",
	discordgo.AuditLogActionOnboardingCreate:       "Onboarding Create",
	discordgo.AuditLogActionOnboardingUpdate:       "Onboarding Update",

	discordgo.AuditLogAction(discordgo.AuditLogActionHomeSettingsCreate): "Home Settings Create",
	discordgo.AuditLogAction(discordgo.AuditLogActionHomeSettingsUpdate): "Home Settings Update",
}

var moderationActionAliases = map[string]discordgo.AuditLogAction{
	"guildupdate":                             discordgo.AuditLogActionGuildUpdate,
	"channelcreate":                           discordgo.AuditLogActionChannelCreate,
	"channelupdate":                           discordgo.AuditLogActionChannelUpdate,
	"channeldelete":                           discordgo.AuditLogActionChannelDelete,
	"channeloverwritecreate":                  discordgo.AuditLogActionChannelOverwriteCreate,
	"channeloverwriteupdate":                  discordgo.AuditLogActionChannelOverwriteUpdate,
	"channeloverwritedelete":                  discordgo.AuditLogActionChannelOverwriteDelete,
	"memberkick":                              discordgo.AuditLogActionMemberKick,
	"memberprune":                             discordgo.AuditLogActionMemberPrune,
	"memberbanadd":                            discordgo.AuditLogActionMemberBanAdd,
	"memberbanremove":                         discordgo.AuditLogActionMemberBanRemove,
	"memberupdate":                            discordgo.AuditLogActionMemberUpdate,
	"memberroleupdate":                        discordgo.AuditLogActionMemberRoleUpdate,
	"membermove":                              discordgo.AuditLogActionMemberMove,
	"memberdisconnect":                        discordgo.AuditLogActionMemberDisconnect,
	"botadd":                                  discordgo.AuditLogActionBotAdd,
	"rolecreate":                              discordgo.AuditLogActionRoleCreate,
	"roleupdate":                              discordgo.AuditLogActionRoleUpdate,
	"roledelete":                              discordgo.AuditLogActionRoleDelete,
	"invitecreate":                            discordgo.AuditLogActionInviteCreate,
	"inviteupdate":                            discordgo.AuditLogActionInviteUpdate,
	"invitedelete":                            discordgo.AuditLogActionInviteDelete,
	"webhookcreate":                           discordgo.AuditLogActionWebhookCreate,
	"webhookupdate":                           discordgo.AuditLogActionWebhookUpdate,
	"webhookdelete":                           discordgo.AuditLogActionWebhookDelete,
	"emojicreate":                             discordgo.AuditLogActionEmojiCreate,
	"emojiupdate":                             discordgo.AuditLogActionEmojiUpdate,
	"emojidelete":                             discordgo.AuditLogActionEmojiDelete,
	"messagedelete":                           discordgo.AuditLogActionMessageDelete,
	"messagebulkdelete":                       discordgo.AuditLogActionMessageBulkDelete,
	"messagepin":                              discordgo.AuditLogActionMessagePin,
	"messageunpin":                            discordgo.AuditLogActionMessageUnpin,
	"integrationcreate":                       discordgo.AuditLogActionIntegrationCreate,
	"integrationupdate":                       discordgo.AuditLogActionIntegrationUpdate,
	"integrationdelete":                       discordgo.AuditLogActionIntegrationDelete,
	"stageinstancecreate":                     discordgo.AuditLogActionStageInstanceCreate,
	"stageinstanceupdate":                     discordgo.AuditLogActionStageInstanceUpdate,
	"stageinstancedelete":                     discordgo.AuditLogActionStageInstanceDelete,
	"stickercreate":                           discordgo.AuditLogActionStickerCreate,
	"stickerupdate":                           discordgo.AuditLogActionStickerUpdate,
	"stickerdelete":                           discordgo.AuditLogActionStickerDelete,
	"guildscheduledeventcreate":               discordgo.AuditLogAction(discordgo.AuditLogGuildScheduledEventCreate),
	"guildscheduledeventupdate":               discordgo.AuditLogAction(discordgo.AuditLogGuildScheduledEventUpdate),
	"guildscheduledeventdelete":               discordgo.AuditLogAction(discordgo.AuditLogGuildScheduledEventDelete),
	"threadcreate":                            discordgo.AuditLogActionThreadCreate,
	"threadupdate":                            discordgo.AuditLogActionThreadUpdate,
	"threaddelete":                            discordgo.AuditLogActionThreadDelete,
	"applicationcommandpermissionupdate":      discordgo.AuditLogActionApplicationCommandPermissionUpdate,
	"automoderationrulecreate":                discordgo.AuditLogActionAutoModerationRuleCreate,
	"automoderationruleupdate":                discordgo.AuditLogActionAutoModerationRuleUpdate,
	"automoderationruledelete":                discordgo.AuditLogActionAutoModerationRuleDelete,
	"automoderationblockmessage":              discordgo.AuditLogActionAutoModerationBlockMessage,
	"automoderationflagtochannel":             discordgo.AuditLogActionAutoModerationFlagToChannel,
	"automoderationusercommunicationdisabled": discordgo.AuditLogActionAutoModerationUserCommunicationDisabled,
	"creatormonetizationrequestcreated":       discordgo.AuditLogActionCreatorMonetizationRequestCreated,
	"creatormonetizationtermsaccepted":        discordgo.AuditLogActionCreatorMonetizationTermsAccepted,
	"onboardingpromptcreate":                  discordgo.AuditLogActionOnboardingPromptCreate,
	"onboardingpromptupdate":                  discordgo.AuditLogActionOnboardingPromptUpdate,
	"onboardingpromptdelete":                  discordgo.AuditLogActionOnboardingPromptDelete,
	"onboardingcreate":                        discordgo.AuditLogActionOnboardingCreate,
	"onboardingupdate":                        discordgo.AuditLogActionOnboardingUpdate,
	"homesettingscreate":                      discordgo.AuditLogAction(discordgo.AuditLogActionHomeSettingsCreate),
	"homesettingsupdate":                      discordgo.AuditLogAction(discordgo.AuditLogActionHomeSettingsUpdate),

	"ban":       discordgo.AuditLogActionMemberBanAdd,
	"massban":   discordgo.AuditLogActionMemberBanAdd,
	"unban":     discordgo.AuditLogActionMemberBanRemove,
	"kick":      discordgo.AuditLogActionMemberKick,
	"timeout":   discordgo.AuditLogActionMemberUpdate,
	"untimeout": discordgo.AuditLogActionMemberUpdate,
}

var (
	fallbackCaseSeqMu sync.Mutex
	fallbackCaseSeq   = map[string]int64{}
)

// RegisterModerationCommands registers slash commands under the /moderation group.
func RegisterModerationCommands(router *core.CommandRouter) {
	checker := router.GetPermissionChecker()
	if checker == nil {
		checker = core.NewPermissionChecker(router.GetSession(), router.GetConfigManager())
	}
	moderationGroup := core.NewGroupCommand("moderation", "Moderation commands", checker)

	moderationGroup.AddSubCommand(newBanCommand())
	moderationGroup.AddSubCommand(newMassBanCommand())
	moderationGroup.AddSubCommand(newUnbanCommand())
	moderationGroup.AddSubCommand(newKickCommand())
	moderationGroup.AddSubCommand(newTimeoutCommand())
	moderationGroup.AddSubCommand(newUntimeoutCommand())

	router.RegisterCommand(newCleanCommand())
	router.RegisterCommand(moderationGroup)
}

type banCommand struct{}

func newBanCommand() *banCommand { return &banCommand{} }

func (c *banCommand) Name() string { return "ban" }

func (c *banCommand) Description() string { return "Ban a user by ID or mention" }

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

func (c *banCommand) RequiresGuild() bool { return true }

func (c *banCommand) RequiresPermissions() bool { return true }

func (c *banCommand) Handle(ctx *core.Context) error {
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

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canBanTarget(ctx, banCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot ban `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx.Session, ctx.GuildID, userID)

	if err := ctx.Session.GuildBanCreateWithReason(ctx.GuildID, userID, reason, 0); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to ban user %s: %v", userID, err), true)
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

type massBanCommand struct{}

func newMassBanCommand() *massBanCommand { return &massBanCommand{} }

func (c *massBanCommand) Name() string { return "massban" }

func (c *massBanCommand) Description() string { return "Ban multiple users by ID or mention" }

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

func (c *massBanCommand) RequiresGuild() bool { return true }

func (c *massBanCommand) RequiresPermissions() bool { return true }

func (c *massBanCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	membersInput, err := extractor.StringRequired("members")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	memberIDs, invalidTokens := parseMemberIDs(membersInput)
	if len(memberIDs) == 0 {
		return core.NewCommandError("No valid member IDs provided", true)
	}
	if len(invalidTokens) > 0 {
		log.ApplicationLogger().Info("Massban ignored invalid member tokens", "guildID", ctx.GuildID, "invalid_count", len(invalidTokens))
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	banCtx, err := prepareBanContext(ctx)
	if err != nil {
		return err
	}

	bannedCount := 0
	var failed []string
	var skipped []string
	for _, memberID := range memberIDs {
		targetUsername := resolveUserDisplayName(ctx.Session, ctx.GuildID, memberID)
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

type unbanCommand struct{}

func newUnbanCommand() *unbanCommand { return &unbanCommand{} }

func (c *unbanCommand) Name() string { return "unban" }

func (c *unbanCommand) Description() string { return "Unban a user by ID or mention" }

func (c *unbanCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "User ID or mention to unban",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for the unban",
			Required:    false,
		},
	}
}

func (c *unbanCommand) RequiresGuild() bool { return true }

func (c *unbanCommand) RequiresPermissions() bool { return true }

func (c *unbanCommand) Handle(ctx *core.Context) error {
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

	if _, err := prepareUnbanContext(ctx); err != nil {
		return err
	}

	targetUsername := resolveUserDisplayName(ctx.Session, ctx.GuildID, userID)
	if err := ctx.Session.GuildBanDelete(ctx.GuildID, userID, discordgo.WithAuditLogReason(reason)); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to unban user %s: %v", userID, err), true)
	}

	details := "Status: Success"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "unban",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildUnbanCommandMessage(targetUsername, reason, truncated))
}

type kickCommand struct{}

func newKickCommand() *kickCommand { return &kickCommand{} }

func (c *kickCommand) Name() string { return "kick" }

func (c *kickCommand) Description() string { return "Kick a member by ID or mention" }

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

func (c *kickCommand) RequiresGuild() bool { return true }

func (c *kickCommand) RequiresPermissions() bool { return true }

func (c *kickCommand) Handle(ctx *core.Context) error {
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

	kickCtx, err := prepareKickContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canKickTarget(ctx, kickCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot kick `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx.Session, ctx.GuildID, userID)
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

type timeoutCommand struct{}

func newTimeoutCommand() *timeoutCommand { return &timeoutCommand{} }

func (c *timeoutCommand) Name() string { return "timeout" }

func (c *timeoutCommand) Description() string { return "Timeout a member" }

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
			MinValue:    floatPtr(1),
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

func (c *timeoutCommand) RequiresGuild() bool { return true }

func (c *timeoutCommand) RequiresPermissions() bool { return true }

func (c *timeoutCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

	rawUserID, err := extractor.StringRequired("user")
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	userID, ok := normalizeUserID(rawUserID)
	if !ok {
		return core.NewCommandError("Invalid user ID or mention.", true)
	}

	minutes := extractor.Int("minutes")
	if minutes <= 0 {
		return core.NewCommandError("Please provide a valid timeout duration in minutes.", true)
	}
	if minutes > timeoutMaxMinutes {
		return core.NewCommandError("Timeout duration cannot exceed 40320 minutes (28 days).", true)
	}

	reason, truncated := sanitizeReason(extractor.String("reason"))

	timeoutCtx, err := prepareTimeoutContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canTimeoutTarget(ctx, timeoutCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot timeout `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx.Session, ctx.GuildID, userID)
	until := time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
	if err := ctx.Session.GuildMemberTimeout(ctx.GuildID, userID, &until, discordgo.WithAuditLogReason(reason)); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to timeout user %s: %v", userID, err), true)
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

type untimeoutCommand struct{}

func newUntimeoutCommand() *untimeoutCommand { return &untimeoutCommand{} }

func (c *untimeoutCommand) Name() string { return "untimeout" }

func (c *untimeoutCommand) Description() string { return "Remove timeout from a member" }

func (c *untimeoutCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Member ID or mention to remove timeout from",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Reason for removing the timeout",
			Required:    false,
		},
	}
}

func (c *untimeoutCommand) RequiresGuild() bool { return true }

func (c *untimeoutCommand) RequiresPermissions() bool { return true }

func (c *untimeoutCommand) Handle(ctx *core.Context) error {
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

	timeoutCtx, err := prepareTimeoutContext(ctx)
	if err != nil {
		return err
	}

	if ok, reasonText := canUntimeoutTarget(ctx, timeoutCtx, userID); !ok {
		return core.NewCommandError(fmt.Sprintf("Cannot remove timeout from `%s`: %s.", userID, reasonText), true)
	}

	targetUsername := resolveUserDisplayName(ctx.Session, ctx.GuildID, userID)
	if err := ctx.Session.GuildMemberTimeout(ctx.GuildID, userID, nil, discordgo.WithAuditLogReason(reason)); err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to remove timeout from user %s: %v", userID, err), true)
	}

	details := "Timeout removed"
	if truncated {
		details += " | Reason truncated to 512 characters"
	}
	sendModerationCaseActionLog(ctx, moderationLogPayload{
		Action:      "untimeout",
		TargetID:    userID,
		TargetLabel: targetUsername,
		Reason:      reason,
		RequestedBy: ctx.UserID,
		Extra:       details,
	})

	return core.NewResponseBuilder(ctx.Session).Success(ctx.Interaction, buildUntimeoutCommandMessage(targetUsername, reason, truncated))
}

type cleanCommand struct{}

func newCleanCommand() *cleanCommand { return &cleanCommand{} }

func (c *cleanCommand) Name() string { return "clean" }

func (c *cleanCommand) Description() string {
	return "Delete recent messages in this channel"
}

func (c *cleanCommand) Options() []*discordgo.ApplicationCommandOption {
	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "num",
			Description: "How many messages to delete (max 100)",
			Required:    true,
			MinValue:    floatPtr(1),
			MaxValue:    float64(cleanMaxDelete),
		},
		{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        "user",
			Description: "Only delete messages from a specific user",
			Required:    false,
		},
	}
}

func (c *cleanCommand) RequiresGuild() bool { return true }

func (c *cleanCommand) RequiresPermissions() bool { return true }

func (c *cleanCommand) Handle(ctx *core.Context) error {
	if ctx == nil || ctx.Session == nil || ctx.Interaction == nil {
		return core.NewCommandError("Session not ready. Try again shortly.", true)
	}
	channelID := strings.TrimSpace(ctx.Interaction.ChannelID)
	if channelID == "" {
		return core.NewCommandError("Channel not available for this command.", true)
	}

	extractor := core.NewOptionExtractor(ctx.Interaction.ApplicationCommandData().Options)
	num := extractor.Int("num")
	if num <= 0 {
		return core.NewCommandError("Please provide a valid number of messages to delete.", true)
	}
	if num > cleanMaxDelete {
		return core.NewCommandError("You can delete up to 100 messages at a time.", true)
	}

	userID, userLabel, err := resolveCleanUserOption(ctx)
	if err != nil {
		return core.NewCommandError(err.Error(), true)
	}

	if err := ensureManageMessagesPermission(ctx); err != nil {
		return err
	}

	messages, err := fetchMessagesForClean(ctx.Session, channelID, int(num), userID)
	if err != nil {
		return core.NewCommandError(fmt.Sprintf("Failed to fetch messages: %v", err), true)
	}
	if len(messages) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, "No matching messages found.")
	}

	cutoff := time.Now().Add(-14 * 24 * time.Hour)
	deleteIDs := make([]string, 0, len(messages))
	skippedOld := 0
	for _, msg := range messages {
		if msg == nil || msg.ID == "" {
			continue
		}
		if !msg.Timestamp.IsZero() && msg.Timestamp.Before(cutoff) {
			skippedOld++
			continue
		}
		deleteIDs = append(deleteIDs, msg.ID)
	}
	if len(deleteIDs) == 0 {
		return core.NewResponseBuilder(ctx.Session).Ephemeral().Info(ctx.Interaction, "No messages could be deleted (all were too old).")
	}

	deleted, failed := cleanup.DeleteMessages(ctx.Session, channelID, deleteIDs, cleanup.DeleteOptions{
		Mode: cleanup.DeleteModeBulkPreferred,
	})
	filterLabel := "any user"
	if userID != "" {
		filterLabel = "<@" + userID + ">"
	}

	log.ApplicationLogger().Info(
		"Clean command executed",
		"guildID", ctx.GuildID,
		"channelID", channelID,
		"requested", num,
		"deleted", deleted,
		"skipped_old", skippedOld,
		"failed", failed,
		"user_filter", userID,
	)

	sendCleanUsageMessageDeleteEmbed(ctx, cleanUsagePayload{
		ChannelID:  channelID,
		ActorID:    ctx.UserID,
		UserID:     userID,
		UserLabel:  userLabel,
		Requested:  int(num),
		Deleted:    deleted,
		SkippedOld: skippedOld,
		Failed:     failed,
	})

	sendModerationLogForEvent(ctx, moderationLogPayload{
		Action:      "clean",
		TargetID:    userID,
		TargetLabel: userLabel,
		Reason:      fmt.Sprintf("Deleted %d message(s)", deleted),
		RequestedBy: ctx.UserID,
		Extra:       fmt.Sprintf("Channel: <#%s> (`%s`) | Filter: %s | Requested: %d | Deleted: %d | Skipped (old): %d | Failed: %d", channelID, channelID, filterLabel, num, deleted, skippedOld, failed),
	}, logging.LogEventCleanAction)

	message := fmt.Sprintf("Deleted %d message(s) in <#%s>.", deleted, channelID)
	if userID != "" {
		message = fmt.Sprintf("Deleted %d message(s) from <@%s> in <#%s>.", deleted, userID, channelID)
	}
	if skippedOld > 0 {
		message += fmt.Sprintf(" Skipped %d message(s) older than 14 days.", skippedOld)
	}
	if failed > 0 {
		message += fmt.Sprintf(" Failed to delete %d message(s).", failed)
	}

	return core.NewResponseBuilder(ctx.Session).Ephemeral().Success(ctx.Interaction, message)
}

func parseMemberIDs(input string) ([]string, []string) {
	rawIDs := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\t'
	})

	unique := make(map[string]struct{})
	invalidSet := make(map[string]struct{})
	var invalid []string

	for _, id := range rawIDs {
		clean := strings.TrimSpace(id)
		if clean == "" {
			continue
		}
		normalized, ok := normalizeUserID(clean)
		if !ok {
			if _, exists := invalidSet[clean]; !exists {
				invalidSet[clean] = struct{}{}
				invalid = append(invalid, clean)
			}
			continue
		}
		unique[normalized] = struct{}{}
	}

	ids := make([]string, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sort.Strings(invalid)
	return ids, invalid
}

func buildBanCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Banned **%s**. Reason: %s", targetLabel, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}

func buildMassBanCommandMessage(banned int) string {
	return fmt.Sprintf("Banned %d user(s).", banned)
}

func buildUnbanCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Unbanned **%s**. Reason: %s", targetLabel, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}

func buildKickCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Kicked **%s**. Reason: %s", targetLabel, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}

func buildTimeoutCommandMessage(targetUsername string, minutes int64, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Timed out **%s** for %s. Reason: %s", targetLabel, formatTimeoutDuration(minutes), reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}

func buildUntimeoutCommandMessage(targetUsername, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Removed timeout from **%s**. Reason: %s", targetLabel, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}

func formatTimeoutDuration(minutes int64) string {
	switch {
	case minutes >= 1440 && minutes%1440 == 0:
		return fmt.Sprintf("%d day(s)", minutes/1440)
	case minutes >= 60 && minutes%60 == 0:
		return fmt.Sprintf("%d hour(s)", minutes/60)
	default:
		return fmt.Sprintf("%d minute(s)", minutes)
	}
}

func resolveCleanUserOption(ctx *core.Context) (string, string, error) {
	if ctx == nil || ctx.Interaction == nil {
		return "", "", nil
	}
	for _, opt := range ctx.Interaction.ApplicationCommandData().Options {
		if opt == nil || opt.Name != "user" {
			continue
		}
		switch opt.Type {
		case discordgo.ApplicationCommandOptionUser:
			if user := opt.UserValue(ctx.Session); user != nil {
				return user.ID, user.Username, nil
			}
			if raw, ok := opt.Value.(string); ok {
				if normalized, ok := normalizeUserID(raw); ok {
					return normalized, "", nil
				}
				return "", "", fmt.Errorf("Invalid user ID or mention.")
			}
		case discordgo.ApplicationCommandOptionString:
			raw := strings.TrimSpace(opt.StringValue())
			if raw == "" {
				return "", "", nil
			}
			normalized, ok := normalizeUserID(raw)
			if !ok {
				return "", "", fmt.Errorf("Invalid user ID or mention.")
			}
			return normalized, "", nil
		}
	}
	return "", "", nil
}

func ensureManageMessagesPermission(ctx *core.Context) error {
	if ctx == nil || ctx.Session == nil {
		return core.NewCommandError("Session not ready. Try again shortly.", true)
	}

	roles, err := getGuildRoles(ctx.Session, ctx.GuildID)
	if err != nil {
		return core.NewCommandError("Failed to resolve server roles.", true)
	}
	rolesByID := buildRoleIndex(roles)

	ownerID, _ := getGuildOwnerID(ctx.Session, ctx.GuildID)

	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if botID == "" {
		return core.NewCommandError("Bot identity not available.", true)
	}

	actorMember := ctx.Interaction.Member
	if actorMember == nil || actorMember.User == nil {
		var ok bool
		actorMember, ok = getMember(ctx.Session, ctx.GuildID, ctx.UserID)
		if !ok || actorMember == nil {
			return core.NewCommandError("Unable to resolve your member record.", true)
		}
	}

	botMember, ok := getMember(ctx.Session, ctx.GuildID, botID)
	if !ok || botMember == nil {
		return core.NewCommandError("Unable to resolve the bot member record.", true)
	}

	actorIsOwner := ctx.IsOwner || (ownerID != "" && ctx.UserID == ownerID)
	botIsOwner := ownerID != "" && botID == ownerID

	if !actorIsOwner && !memberHasPermission(actorMember, rolesByID, ctx.GuildID, ownerID, discordgo.PermissionManageMessages) {
		return core.NewCommandError("You need the Manage Messages permission to use this command.", true)
	}
	if !botIsOwner && !memberHasPermission(botMember, rolesByID, ctx.GuildID, ownerID, discordgo.PermissionManageMessages) {
		return core.NewCommandError("I need the Manage Messages permission to delete messages.", true)
	}
	return nil
}

func fetchMessagesForClean(session *discordgo.Session, channelID string, target int, userID string) ([]*discordgo.Message, error) {
	if session == nil || channelID == "" || target <= 0 {
		return nil, nil
	}
	var out []*discordgo.Message
	beforeID := ""
	fetched := 0
	for fetched < cleanMaxFetch && len(out) < target {
		batchSize := minInt(100, cleanMaxFetch-fetched)
		msgs, err := session.ChannelMessages(channelID, batchSize, beforeID, "", "")
		if err != nil {
			return nil, err
		}
		if len(msgs) == 0 {
			break
		}
		fetched += len(msgs)
		for _, msg := range msgs {
			if msg == nil || msg.ID == "" {
				continue
			}
			if userID != "" {
				if msg.Author == nil || msg.Author.ID != userID {
					continue
				}
			}
			out = append(out, msg)
			if len(out) >= target {
				break
			}
		}
		beforeID = msgs[len(msgs)-1].ID
	}
	return out, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func floatPtr(v float64) *float64 { return &v }

type banContext struct {
	rolesByID    map[string]*discordgo.Role
	ownerID      string
	botID        string
	actorMember  *discordgo.Member
	botMember    *discordgo.Member
	actorIsOwner bool
	botIsOwner   bool
	actorRolePos int
	botRolePos   int
}

func sanitizeReason(input string) (string, bool) {
	reason := strings.TrimSpace(input)
	if reason == "" {
		return "No reason provided", false
	}
	reason = strings.ReplaceAll(reason, "\r", " ")
	reason = strings.ReplaceAll(reason, "\n", " ")
	reason = strings.TrimSpace(reason)
	if len(reason) <= maxAuditLogReasonLen {
		return reason, false
	}
	return reason[:maxAuditLogReasonLen], true
}

func normalizeUserID(input string) (string, bool) {
	clean := strings.TrimSpace(input)
	if clean == "" {
		return "", false
	}
	if match := userMentionRe.FindStringSubmatch(clean); len(match) == 2 {
		return match[1], true
	}
	if !isLikelySnowflake(clean) {
		return "", false
	}
	return clean, true
}

func isLikelySnowflake(value string) bool {
	if len(value) < minSnowflakeLength || len(value) > maxSnowflakeLength {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func prepareBanContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionBanMembers,
		"You need the Ban Members permission to use this command.",
		"I need the Ban Members permission to ban members.",
	)
}

func prepareUnbanContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionBanMembers,
		"You need the Ban Members permission to use this command.",
		"I need the Ban Members permission to unban members.",
	)
}

func prepareKickContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionKickMembers,
		"You need the Kick Members permission to use this command.",
		"I need the Kick Members permission to kick members.",
	)
}

func prepareTimeoutContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionModerateMembers,
		"You need the Moderate Members permission to use this command.",
		"I need the Moderate Members permission to timeout members.",
	)
}

func prepareModerationContext(ctx *core.Context, requiredPermission int64, actorPermissionError, botPermissionError string) (*banContext, error) {
	if ctx == nil || ctx.Session == nil {
		return nil, core.NewCommandError("Session not ready. Try again shortly.", true)
	}

	roles, err := getGuildRoles(ctx.Session, ctx.GuildID)
	if err != nil {
		return nil, core.NewCommandError("Failed to resolve server roles.", true)
	}
	rolesByID := buildRoleIndex(roles)

	ownerID, _ := getGuildOwnerID(ctx.Session, ctx.GuildID)

	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if botID == "" {
		return nil, core.NewCommandError("Bot identity not available.", true)
	}

	actorMember := ctx.Interaction.Member
	if actorMember == nil || actorMember.User == nil {
		var ok bool
		actorMember, ok = getMember(ctx.Session, ctx.GuildID, ctx.UserID)
		if !ok || actorMember == nil {
			return nil, core.NewCommandError("Unable to resolve your member record.", true)
		}
	}

	botMember, ok := getMember(ctx.Session, ctx.GuildID, botID)
	if !ok || botMember == nil {
		return nil, core.NewCommandError("Unable to resolve the bot member record.", true)
	}

	actorIsOwner := ctx.IsOwner || (ownerID != "" && ctx.UserID == ownerID)
	botIsOwner := ownerID != "" && botID == ownerID

	if !actorIsOwner && !memberHasPermission(actorMember, rolesByID, ctx.GuildID, ownerID, requiredPermission) {
		return nil, core.NewCommandError(actorPermissionError, true)
	}
	if !botIsOwner && !memberHasPermission(botMember, rolesByID, ctx.GuildID, ownerID, requiredPermission) {
		return nil, core.NewCommandError(botPermissionError, true)
	}

	return &banContext{
		rolesByID:    rolesByID,
		ownerID:      ownerID,
		botID:        botID,
		actorMember:  actorMember,
		botMember:    botMember,
		actorIsOwner: actorIsOwner,
		botIsOwner:   botIsOwner,
		actorRolePos: highestRolePosition(actorMember, rolesByID, ctx.GuildID),
		botRolePos:   highestRolePosition(botMember, rolesByID, ctx.GuildID),
	}, nil
}

func canBanTarget(ctx *core.Context, banCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, banCtx, targetID, "ban", false)
}

func canKickTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "kick", true)
}

func canTimeoutTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "timeout", true)
}

func canUntimeoutTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "remove timeout from", true)
}

func canModerateTarget(ctx *core.Context, actionCtx *banContext, targetID, actionVerb string, requireMember bool) (bool, string) {
	if targetID == ctx.UserID {
		return false, "cannot " + actionVerb + " yourself"
	}
	if targetID == actionCtx.botID {
		return false, "cannot " + actionVerb + " the bot"
	}
	if actionCtx.ownerID != "" && targetID == actionCtx.ownerID {
		return false, "cannot " + actionVerb + " the server owner"
	}

	targetMember, ok := getMember(ctx.Session, ctx.GuildID, targetID)
	if !ok || targetMember == nil {
		if requireMember {
			return false, "target is not a member of this server"
		}
		return true, ""
	}

	targetPos := highestRolePosition(targetMember, actionCtx.rolesByID, ctx.GuildID)
	if !actionCtx.actorIsOwner && actionCtx.actorRolePos <= targetPos {
		return false, "target has an equal or higher role than you"
	}
	if !actionCtx.botIsOwner && actionCtx.botRolePos <= targetPos {
		return false, "target has an equal or higher role than the bot"
	}
	return true, ""
}

func buildRoleIndex(roles []*discordgo.Role) map[string]*discordgo.Role {
	byID := make(map[string]*discordgo.Role, len(roles))
	for _, role := range roles {
		if role == nil || role.ID == "" {
			continue
		}
		byID[role.ID] = role
	}
	return byID
}

func getGuildRoles(session *discordgo.Session, guildID string) ([]*discordgo.Role, error) {
	if session == nil {
		return nil, fmt.Errorf("session not ready")
	}
	if session.State != nil {
		if g, _ := session.State.Guild(guildID); g != nil && len(g.Roles) > 0 {
			return g.Roles, nil
		}
	}
	return session.GuildRoles(guildID)
}

func getGuildOwnerID(session *discordgo.Session, guildID string) (string, bool) {
	if session == nil || guildID == "" {
		return "", false
	}
	if session.State != nil {
		if g, _ := session.State.Guild(guildID); g != nil && g.OwnerID != "" {
			return g.OwnerID, true
		}
	}
	guild, err := session.Guild(guildID)
	if err != nil || guild == nil || guild.OwnerID == "" {
		return "", false
	}
	return guild.OwnerID, true
}

func getMember(session *discordgo.Session, guildID, userID string) (*discordgo.Member, bool) {
	if session == nil || guildID == "" || userID == "" {
		return nil, false
	}
	if session.State != nil {
		if m, _ := session.State.Member(guildID, userID); m != nil {
			return m, true
		}
	}
	member, err := session.GuildMember(guildID, userID)
	if err != nil || member == nil {
		return nil, false
	}
	return member, true
}

func resolveUserDisplayName(session *discordgo.Session, guildID, userID string) string {
	if session == nil || userID == "" {
		return userID
	}
	if member, ok := getMember(session, guildID, userID); ok && member != nil && member.User != nil {
		if username := strings.TrimSpace(member.User.Username); username != "" {
			return username
		}
	}
	user, err := session.User(userID)
	if err == nil && user != nil {
		if username := strings.TrimSpace(user.Username); username != "" {
			return username
		}
	}
	return userID
}

func memberHasPermission(member *discordgo.Member, rolesByID map[string]*discordgo.Role, guildID, ownerID string, perm int64) bool {
	if member == nil || member.User == nil {
		return false
	}
	if ownerID != "" && member.User.ID == ownerID {
		return true
	}

	var permissions int64
	if role, ok := rolesByID[guildID]; ok && role != nil {
		permissions |= role.Permissions
	}
	for _, roleID := range member.Roles {
		if role, ok := rolesByID[roleID]; ok && role != nil {
			permissions |= role.Permissions
		}
	}

	if permissions&discordgo.PermissionAdministrator != 0 {
		return true
	}
	return permissions&perm != 0
}

func highestRolePosition(member *discordgo.Member, rolesByID map[string]*discordgo.Role, guildID string) int {
	if member == nil {
		return -1
	}

	pos := -1
	if role, ok := rolesByID[guildID]; ok && role != nil {
		pos = role.Position
	}
	for _, roleID := range member.Roles {
		if role, ok := rolesByID[roleID]; ok && role != nil && role.Position > pos {
			pos = role.Position
		}
	}
	return pos
}

type moderationLogPayload struct {
	Action      string
	TargetID    string
	TargetLabel string
	Reason      string
	RequestedBy string
	Extra       string
}

type cleanUsagePayload struct {
	ChannelID  string
	ActorID    string
	UserID     string
	UserLabel  string
	Requested  int
	Deleted    int
	SkippedOld int
	Failed     int
}

func buildMassBanLogDetails(total, banned int, invalid, skipped, failed []string) string {
	parts := []string{fmt.Sprintf("Total: %d", total), fmt.Sprintf("Banned: %d", banned)}
	if len(invalid) > 0 {
		parts = append(parts, fmt.Sprintf("Invalid: %d", len(invalid)))
	}
	if len(skipped) > 0 {
		parts = append(parts, fmt.Sprintf("Skipped: %d", len(skipped)))
	}
	if len(failed) > 0 {
		parts = append(parts, fmt.Sprintf("Failed: %d", len(failed)))
	}
	return strings.Join(parts, " | ")
}

func shouldSendCleanUsageMessageDeleteEmbed(ctx *core.Context) bool {
	if ctx == nil || ctx.Config == nil || ctx.GuildID == "" {
		return false
	}
	cfg := ctx.Config.Config()
	if cfg == nil {
		return false
	}
	if !cfg.ResolveFeatures(ctx.GuildID).Logging.MessageDelete {
		return false
	}
	rc := cfg.ResolveRuntimeConfig(ctx.GuildID)
	return !rc.DisableMessageLogs
}

func resolveMessageDeleteLogChannel(ctx *core.Context) string {
	if ctx == nil {
		return ""
	}
	if ctx.Config != nil && ctx.GuildID != "" {
		if cid := logging.ResolveLogChannel(logging.LogEventMessageDelete, ctx.GuildID, ctx.Config); cid != "" {
			return cid
		}
	}
	if ctx.GuildConfig == nil {
		return ""
	}
	if cid := strings.TrimSpace(ctx.GuildConfig.Channels.MessageDelete); cid != "" {
		return cid
	}
	if cid := strings.TrimSpace(ctx.GuildConfig.Channels.MessageEdit); cid != "" {
		return cid
	}
	return ""
}

func sendCleanUsageMessageDeleteEmbed(ctx *core.Context, payload cleanUsagePayload) {
	if ctx == nil || ctx.Session == nil || ctx.GuildConfig == nil {
		return
	}
	if !shouldSendCleanUsageMessageDeleteEmbed(ctx) {
		return
	}

	logChannelID := resolveMessageDeleteLogChannel(ctx)
	if logChannelID == "" {
		return
	}

	filterValue := "Any user"
	if payload.UserID != "" {
		targetLabel := strings.TrimSpace(payload.UserLabel)
		switch {
		case targetLabel == "":
			filterValue = fmt.Sprintf("<@%s> (`%s`)", payload.UserID, payload.UserID)
		case targetLabel == payload.UserID:
			filterValue = fmt.Sprintf("<@%s> (`%s`)", payload.UserID, payload.UserID)
		default:
			filterValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, payload.UserID, payload.UserID)
		}
	}

	channelValue := "Unknown"
	if payload.ChannelID != "" {
		channelValue = fmt.Sprintf("<#%s> (`%s`)", payload.ChannelID, payload.ChannelID)
	}
	actorValue := "Unknown"
	if payload.ActorID != "" {
		actorValue = fmt.Sprintf("<@%s> (`%s`)", payload.ActorID, payload.ActorID)
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Message Delete Context",
		Description: "Deletion executed via `/moderation clean`.",
		Color:       theme.MessageDelete(),
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields: []*discordgo.MessageEmbedField{
			{Name: "CASO", Value: "`/clean` da alicebot foi usado", Inline: true},
			{Name: "Actor", Value: actorValue, Inline: true},
			{Name: "Channel", Value: channelValue, Inline: true},
			{Name: "Filter", Value: filterValue, Inline: false},
			{Name: "Requested", Value: fmt.Sprintf("%d", payload.Requested), Inline: true},
			{Name: "Deleted", Value: fmt.Sprintf("%d", payload.Deleted), Inline: true},
			{Name: "Skipped (14d+)", Value: fmt.Sprintf("%d", payload.SkippedOld), Inline: true},
			{Name: "Failed", Value: fmt.Sprintf("%d", payload.Failed), Inline: true},
		},
	}

	if _, err := ctx.Session.ChannelMessageSendEmbed(logChannelID, embed); err != nil {
		log.ErrorLoggerRaw().Error(
			"Failed to send /clean usage context embed to message delete log",
			"guildID", ctx.GuildID,
			"channelID", logChannelID,
			"err", err,
		)
	}
}

func nextGuildCaseNumber(ctx *core.Context) (int64, bool) {
	if ctx == nil || ctx.GuildID == "" {
		return 0, false
	}
	router := ctx.Router()
	if router == nil {
		return nextFallbackCaseNumber(ctx.GuildID), true
	}
	store := router.GetStore()
	if store == nil {
		return nextFallbackCaseNumber(ctx.GuildID), true
	}

	n, err := store.NextModerationCaseNumber(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to allocate moderation case number", "guildID", ctx.GuildID, "err", err)
		return nextFallbackCaseNumber(ctx.GuildID), true
	}
	return n, true
}

func nextFallbackCaseNumber(guildID string) int64 {
	fallbackCaseSeqMu.Lock()
	defer fallbackCaseSeqMu.Unlock()
	fallbackCaseSeq[guildID]++
	return fallbackCaseSeq[guildID]
}

func buildModerationCaseTitle(caseNumber int64, hasCaseNumber bool, actionType string) string {
	casePart := "?"
	if hasCaseNumber && caseNumber > 0 {
		casePart = fmt.Sprintf("%d", caseNumber)
	}
	actionType = strings.ToLower(strings.TrimSpace(actionType))
	if actionType == "" {
		actionType = "action"
	}
	return actionType + " | case " + casePart
}

func resolveModerationActionType(action string) string {
	raw := strings.TrimSpace(action)
	if raw == "" {
		return "Unknown Action"
	}

	if code, err := strconv.Atoi(raw); err == nil {
		return moderationActionTypeForCode(discordgo.AuditLogAction(code))
	}

	key := compactModerationActionKey(raw)
	if auditAction, ok := moderationActionAliases[key]; ok {
		return moderationActionTypeForCode(auditAction)
	}

	return humanizeModerationAction(raw)
}

func moderationActionTypeForCode(action discordgo.AuditLogAction) string {
	if label, ok := moderationActionTypeLabels[action]; ok {
		return label
	}
	return fmt.Sprintf("Audit Action %d", int(action))
}

func compactModerationActionKey(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	replacer := strings.NewReplacer(
		" ", "",
		"_", "",
		"-", "",
		".", "",
		"/", "",
		":", "",
	)
	key = replacer.Replace(key)
	key = strings.TrimPrefix(key, "auditlogaction")
	key = strings.TrimPrefix(key, "auditlog")
	return key
}

func humanizeModerationAction(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	key = strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(key)
	parts := strings.Fields(key)
	if len(parts) == 0 {
		return "Unknown Action"
	}
	for i, part := range parts {
		if len(part) == 1 {
			parts[i] = strings.ToUpper(part)
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func sendModerationLog(ctx *core.Context, payload moderationLogPayload) {
	sendModerationLogForEvent(ctx, payload, logging.LogEventModerationCase)
}

func sendModerationLogForEvent(ctx *core.Context, payload moderationLogPayload, eventType logging.LogEventType) {
	if ctx == nil || ctx.Session == nil || ctx.Config == nil || ctx.GuildID == "" {
		return
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	emit := logging.ShouldEmitLogEvent(ctx.Session, ctx.Config, eventType, ctx.GuildID)
	if !emit.Enabled {
		return
	}
	channelID := emit.ChannelID

	action := strings.TrimSpace(payload.Action)
	targetID := strings.TrimSpace(payload.TargetID)
	targetLabel := strings.TrimSpace(payload.TargetLabel)
	targetValue := "Unknown"
	switch {
	case targetID == "" && targetLabel != "":
		targetValue = targetLabel
	case targetID != "" && (targetLabel == "" || targetLabel == targetID):
		targetValue = "<@" + targetID + "> (`" + targetID + "`)"
	case targetID != "":
		targetValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, targetID, targetID)
	}
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "No reason provided"
	}
	caseID := ""
	if ctx.Interaction != nil {
		caseID = strings.TrimSpace(ctx.Interaction.ID)
	}

	fields := []*discordgo.MessageEmbedField{
		{Name: "Action", Value: action, Inline: true},
	}
	if caseID != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Case ID", Value: "`" + caseID + "`", Inline: true})
	}
	fields = append(fields,
		&discordgo.MessageEmbedField{Name: "Target", Value: targetValue, Inline: true},
		&discordgo.MessageEmbedField{Name: "Actor", Value: "<@" + botID + "> (`" + botID + "`)", Inline: true},
	)
	if payload.RequestedBy != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Requested By",
			Value:  "<@" + payload.RequestedBy + "> (`" + payload.RequestedBy + "`)",
			Inline: true,
		})
	}
	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Reason",
		Value:  reason,
		Inline: false,
	})
	if payload.Extra != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Details",
			Value:  payload.Extra,
			Inline: false,
		})
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Moderation Action",
		Color:       theme.AutomodAction(),
		Description: fmt.Sprintf("Moderation action executed by <@%s>.", botID),
		Fields:      fields,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	if _, err := ctx.Session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send moderation log", "guildID", ctx.GuildID, "channelID", channelID, "action", action, "err", err)
	}
}

func resolveModerationCaseEmbedMeta(action, actionType string) (string, string, string) {
	switch compactModerationActionKey(action) {
	case "ban", "massban", "memberbanadd":
		return "ban", "Offender", "Details"
	case "unban", "memberbanremove":
		return "unban", "User", "Details"
	case "kick", "memberkick":
		return "kick", "Offender", "Details"
	case "timeout":
		return "timeout", "Offender", "Details"
	case "untimeout":
		return "untimeout", "User", "Details"
	default:
		label := strings.ToLower(strings.TrimSpace(actionType))
		if label == "" {
			label = "action"
		}
		return label, "Target", "Details"
	}
}

func sendModerationCaseActionLog(ctx *core.Context, payload moderationLogPayload) {
	if ctx == nil || ctx.Session == nil || ctx.Config == nil || ctx.GuildID == "" {
		return
	}
	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	emit := logging.ShouldEmitLogEvent(ctx.Session, ctx.Config, logging.LogEventModerationCase, ctx.GuildID)
	if !emit.Enabled {
		return
	}
	channelID := emit.ChannelID
	caseNumber, hasCaseNumber := nextGuildCaseNumber(ctx)

	action := strings.TrimSpace(payload.Action)
	if action == "" {
		action = "member_ban_add"
	}
	actionType := resolveModerationActionType(action)
	actionLabel, targetFieldName, detailsFieldName := resolveModerationCaseEmbedMeta(action, actionType)
	reason := strings.TrimSpace(payload.Reason)
	if reason == "" {
		reason = "No reason provided"
	}
	targetID := strings.TrimSpace(payload.TargetID)
	targetLabel := strings.TrimSpace(payload.TargetLabel)
	targetValue := "Unknown target"
	switch {
	case targetID == "" && targetLabel != "":
		targetValue = targetLabel
	case targetID != "" && (targetLabel == "" || targetLabel == targetID):
		targetValue = fmt.Sprintf("<@%s> (`%s`)", targetID, targetID)
	case targetID != "":
		targetValue = fmt.Sprintf("**%s** (<@%s>, `%s`)", targetLabel, targetID, targetID)
	}
	actorID := strings.TrimSpace(payload.RequestedBy)
	if actorID == "" {
		actorID = botID
	}
	actorValue := fmt.Sprintf("<@%s> (`%s`)", actorID, actorID)

	eventAt := time.Now()
	eventID := strings.TrimSpace(targetID)
	if eventID == "" && ctx.Interaction != nil {
		eventID = strings.TrimSpace(ctx.Interaction.ID)
	}
	if eventID == "" {
		eventID = "unknown"
	}

	descriptionLines := []string{
		fmt.Sprintf("**%s:** %s", targetFieldName, targetValue),
		fmt.Sprintf("**Reason:** %s", reason),
		fmt.Sprintf("**Responsible moderator:** %s", actorValue),
	}
	if payload.Extra != "" {
		descriptionLines = append(descriptionLines, fmt.Sprintf("**%s:** %s", detailsFieldName, payload.Extra))
	}
	descriptionLines = append(descriptionLines, fmt.Sprintf("ID: `%s` • <t:%d:F>", eventID, eventAt.Unix()))

	embed := &discordgo.MessageEmbed{
		Title:       buildModerationCaseTitle(caseNumber, hasCaseNumber, actionLabel),
		Description: strings.Join(descriptionLines, "\n"),
		Color:       theme.AutomodAction(),
		Timestamp:   eventAt.Format(time.RFC3339),
	}

	if _, err := ctx.Session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send moderation case action log", "guildID", ctx.GuildID, "channelID", channelID, "action", action, "err", err)
	}
}
