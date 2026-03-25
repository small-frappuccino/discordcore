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
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

const (
	maxAuditLogReasonLen = 512
	minSnowflakeLength   = 15
	maxSnowflakeLength   = 21
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
	"mute":      discordgo.AuditLogActionMemberRoleUpdate,
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
	moderationGroup.AddSubCommand(newKickCommand())
	moderationGroup.AddSubCommand(newMuteCommand())
	moderationGroup.AddSubCommand(newTimeoutCommand())
	moderationGroup.AddSubCommand(newWarnCommand())
	moderationGroup.AddSubCommand(newWarningsCommand())

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

	targetUsername := resolveUserDisplayName(ctx, userID)

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

	targetUsername := resolveUserDisplayName(ctx, userID)
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

func (c *warnCommand) Handle(ctx *core.Context) error {
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

	warnCtx, err := prepareWarnContext(ctx)
	if err != nil {
		return err
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

type warningsCommand struct{}

func newWarningsCommand() *warningsCommand { return &warningsCommand{} }

func (c *warningsCommand) Name() string { return "warnings" }

func (c *warningsCommand) Description() string { return "List recent warnings for a member" }

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

func (c *warningsCommand) RequiresGuild() bool { return true }

func (c *warningsCommand) RequiresPermissions() bool { return true }

func (c *warningsCommand) Handle(ctx *core.Context) error {
	extractor := core.NewOptionExtractor(core.GetSubCommandOptions(ctx.Interaction))

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
		return err
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

func buildMuteCommandMessage(targetUsername string, muteRole *discordgo.Role, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Muted **%s** with **%s**. Reason: %s", targetLabel, formatRoleDisplayName(muteRole), reason)
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

func buildWarnCommandMessage(targetUsername string, caseNumber int64, reason string, truncated bool) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	message := fmt.Sprintf("Warned **%s**. Case #%d. Reason: %s", targetLabel, caseNumber, reason)
	if truncated {
		message += " (reason truncated to 512 characters)"
	}
	return message
}

func buildWarningsCommandMessage(targetUsername string, warnings []storage.ModerationWarning) string {
	targetLabel := strings.TrimSpace(targetUsername)
	if targetLabel == "" {
		targetLabel = "unknown user"
	}
	if len(warnings) == 0 {
		return fmt.Sprintf("No warnings recorded for **%s**.", targetLabel)
	}

	lines := []string{fmt.Sprintf("Recent warnings for **%s**:", targetLabel)}
	for _, warning := range warnings {
		reason := strings.TrimSpace(warning.Reason)
		if reason == "" {
			reason = "No reason provided"
		}
		createdAt := warning.CreatedAt
		if createdAt.IsZero() {
			lines = append(lines, fmt.Sprintf("#%d • by <@%s> • %s", warning.CaseNumber, warning.ModeratorID, reason))
			continue
		}
		lines = append(lines, fmt.Sprintf("#%d • <t:%d:d> • by <@%s> • %s", warning.CaseNumber, createdAt.Unix(), warning.ModeratorID, reason))
	}
	return strings.Join(lines, "\n")
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

func permissionCheckerForContext(ctx *core.Context) *core.PermissionChecker {
	if ctx == nil {
		return nil
	}
	if router := ctx.Router(); router != nil {
		if checker := router.GetPermissionChecker(); checker != nil {
			return checker
		}

		checker := core.NewPermissionChecker(ctx.Session, ctx.Config)
		if store := router.GetStore(); store != nil {
			checker.SetStore(store)
		}
		return checker
	}

	return core.NewPermissionChecker(ctx.Session, ctx.Config)
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

func prepareMuteContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionManageRoles,
		"You need the Manage Roles permission to use this command.",
		"I need the Manage Roles permission to mute members with the configured mute role.",
	)
}

func prepareWarnContext(ctx *core.Context) (*banContext, error) {
	return prepareModerationContext(
		ctx,
		discordgo.PermissionModerateMembers,
		"You need the Moderate Members permission to use this command.",
		"I need the Moderate Members permission to manage warnings.",
	)
}

func prepareModerationContext(ctx *core.Context, requiredPermission int64, actorPermissionError, botPermissionError string) (*banContext, error) {
	if ctx == nil || ctx.Session == nil {
		return nil, core.NewCommandError("Session not ready. Try again shortly.", true)
	}

	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		return nil, core.NewCommandError("Permission resolver not available.", true)
	}

	roles, err := checker.ResolveRoles(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve guild roles",
			"operation", "commands.moderation.prepare_context.resolve_roles",
			"guildID", ctx.GuildID,
			"userID", ctx.UserID,
			"err", err,
		)
		return nil, core.NewCommandError("Failed to resolve server roles.", true)
	}
	rolesByID := buildRoleIndex(roles)

	ownerID, ownerFound, err := checker.ResolveOwnerID(ctx.GuildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve guild owner",
			"operation", "commands.moderation.prepare_context.resolve_owner",
			"guildID", ctx.GuildID,
			"userID", ctx.UserID,
			"err", err,
		)
		return nil, core.NewCommandError("Failed to resolve server owner.", true)
	}
	if !ownerFound {
		ownerID = ""
	}

	botID := ""
	if ctx.Session.State != nil && ctx.Session.State.User != nil {
		botID = ctx.Session.State.User.ID
	}
	if botID == "" {
		return nil, core.NewCommandError("Bot identity not available.", true)
	}

	var actorMember *discordgo.Member
	if ctx.Interaction != nil {
		actorMember = ctx.Interaction.Member
	}
	if actorMember == nil || actorMember.User == nil {
		var ok bool
		actorMember, ok, err = checker.ResolveMember(ctx.GuildID, ctx.UserID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Moderation context failed to resolve actor member",
				"operation", "commands.moderation.prepare_context.resolve_actor_member",
				"guildID", ctx.GuildID,
				"userID", ctx.UserID,
				"err", err,
			)
			return nil, core.NewCommandError("Unable to resolve your member record.", true)
		}
		if !ok || actorMember == nil {
			return nil, core.NewCommandError("Unable to resolve your member record.", true)
		}
	}

	botMember, ok, err := checker.ResolveMember(ctx.GuildID, botID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation context failed to resolve bot member",
			"operation", "commands.moderation.prepare_context.resolve_bot_member",
			"guildID", ctx.GuildID,
			"botID", botID,
			"err", err,
		)
		return nil, core.NewCommandError("Unable to resolve the bot member record.", true)
	}
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

func canMuteTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "mute", true)
}

func canWarnTarget(ctx *core.Context, actionCtx *banContext, targetID string) (bool, string) {
	return canModerateTarget(ctx, actionCtx, targetID, "warn", true)
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

	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		if requireMember {
			return false, "target member could not be resolved right now"
		}
		return true, ""
	}

	targetMember, ok, err := checker.ResolveMember(ctx.GuildID, targetID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation target validation failed to resolve target member",
			"operation", "commands.moderation.can_moderate_target.resolve_target_member",
			"guildID", ctx.GuildID,
			"targetID", targetID,
			"action", actionVerb,
			"err", err,
		)
		if requireMember {
			return false, "target member could not be resolved right now"
		}
		return true, ""
	}
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

func resolveConfiguredMuteRole(ctx *core.Context, actionCtx *banContext) (*discordgo.Role, string, error) {
	if ctx == nil || ctx.Config == nil {
		return nil, "", core.NewCommandError("Configuration is not available right now.", true)
	}
	cfg := ctx.Config.Config()
	if cfg == nil {
		return nil, "", core.NewCommandError("Configuration is not available right now.", true)
	}
	if !cfg.ResolveFeatures(ctx.GuildID).MuteRole {
		return nil, "", core.NewCommandError("Mute role moderation is disabled for this server.", true)
	}

	roleID := ""
	if ctx.GuildConfig != nil {
		roleID = strings.TrimSpace(ctx.GuildConfig.Roles.MuteRole)
	}
	if roleID == "" {
		for _, guild := range cfg.Guilds {
			if guild.GuildID == ctx.GuildID {
				roleID = strings.TrimSpace(guild.Roles.MuteRole)
				break
			}
		}
	}
	if roleID == "" {
		return nil, "", core.NewCommandError("Mute role is not configured for this server.", true)
	}
	if actionCtx == nil {
		return nil, roleID, core.NewCommandError("Mute role context is not available right now.", true)
	}

	role, ok := actionCtx.rolesByID[roleID]
	if !ok || role == nil {
		return nil, roleID, core.NewCommandError("Configured mute role is no longer available in this server.", true)
	}
	if role.Managed {
		return nil, roleID, core.NewCommandError("Configured mute role is managed by an integration and cannot be assigned manually.", true)
	}
	if !actionCtx.actorIsOwner && actionCtx.actorRolePos <= role.Position {
		return nil, roleID, core.NewCommandError("Your highest role must stay above the configured mute role.", true)
	}
	if !actionCtx.botIsOwner && actionCtx.botRolePos <= role.Position {
		return nil, roleID, core.NewCommandError("My highest role must stay above the configured mute role.", true)
	}
	return role, roleID, nil
}

func resolveRoleTargetMember(ctx *core.Context, targetID string) (*discordgo.Member, bool, string) {
	checker := permissionCheckerForContext(ctx)
	if checker == nil {
		return nil, false, "target member could not be resolved right now"
	}
	targetMember, ok, err := checker.ResolveMember(ctx.GuildID, targetID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Moderation role action failed to resolve target member",
			"operation", "commands.moderation.resolve_role_target_member",
			"guildID", ctx.GuildID,
			"targetID", targetID,
			"err", err,
		)
		return nil, false, "target member could not be resolved right now"
	}
	if !ok || targetMember == nil {
		return nil, false, "target is not a member of this server"
	}
	return targetMember, true, ""
}

func memberHasRole(member *discordgo.Member, roleID string) bool {
	if member == nil {
		return false
	}
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return false
	}
	for _, existingRoleID := range member.Roles {
		if strings.TrimSpace(existingRoleID) == roleID {
			return true
		}
	}
	return false
}

func formatRoleDisplayName(role *discordgo.Role) string {
	if role == nil {
		return "mute role"
	}
	if role.ID == role.Name || strings.TrimSpace(role.Name) == "" {
		return role.ID
	}
	return role.Name
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

func resolveUserDisplayName(ctx *core.Context, userID string) string {
	if ctx == nil || ctx.Session == nil || userID == "" {
		return userID
	}

	checker := permissionCheckerForContext(ctx)
	if checker != nil {
		member, ok, err := checker.ResolveMember(ctx.GuildID, userID)
		if err != nil {
			log.ErrorLoggerRaw().Error(
				"Moderation failed to resolve display name member",
				"operation", "commands.moderation.resolve_display_name.resolve_member",
				"guildID", ctx.GuildID,
				"userID", userID,
				"err", err,
			)
		} else if ok && member != nil && member.User != nil {
			if username := strings.TrimSpace(member.User.Username); username != "" {
				return username
			}
		}
	}

	user, err := ctx.Session.User(userID)
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
	Action        string
	TargetID      string
	TargetLabel   string
	Reason        string
	RequestedBy   string
	Extra         string
	CaseNumber    int64
	HasCaseNumber bool
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

func moderationStoreFromContext(ctx *core.Context) *storage.Store {
	if ctx == nil {
		return nil
	}
	router := ctx.Router()
	if router == nil {
		return nil
	}
	return router.GetStore()
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

	switch compactModerationActionKey(raw) {
	case "mute":
		return "Member Role Update"
	case "warn":
		return "Warning Issued"
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
	case "mute", "memberroleupdate":
		return "mute", "Offender", "Details"
	case "timeout":
		return "timeout", "Offender", "Details"
	case "untimeout":
		return "untimeout", "User", "Details"
	case "warn":
		return "warn", "Offender", "Details"
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
	caseNumber, hasCaseNumber := payload.CaseNumber, payload.HasCaseNumber
	if !hasCaseNumber || caseNumber <= 0 {
		caseNumber, hasCaseNumber = nextGuildCaseNumber(ctx)
	}

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
