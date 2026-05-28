package moderation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

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
