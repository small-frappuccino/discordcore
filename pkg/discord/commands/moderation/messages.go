package moderation

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type modMsgKey int

const (
	// Feature-disabled messages
	modMsgBanDisabled modMsgKey = iota
	modMsgMassBanDisabled
	modMsgKickDisabled
	modMsgTimeoutDisabled
	modMsgWarnDisabled
	modMsgWarningsDisabled

	// Validation errors
	modMsgInvalidUser
	modMsgInvalidTimeoutDuration
	modMsgTimeoutTooLong
	modMsgNoValidMembers

	// Context / permission setup errors
	modMsgSessionNotReady
	modMsgPermResolverNA
	modMsgConfigNA
	modMsgRolesResolveFailed
	modMsgOwnerResolveFailed
	modMsgBotIdentityNA
	modMsgActorResolveFailed
	modMsgBotMemberResolveFailed

	// Required-permission messages for each action
	modMsgNeedBanPerm
	modMsgBotNeedBanPerm
	modMsgNeedKickPerm
	modMsgBotNeedKickPerm
	modMsgNeedModeratePerm
	modMsgBotNeedTimeoutPerm
	modMsgBotNeedWarnPerm
	modMsgNeedRolesPerm
	modMsgBotNeedRolesPerm

	// Mute-role errors
	modMsgMuteRoleFeatureOff
	modMsgMuteRoleNotConfigured
	modMsgMuteRoleContextNA
	modMsgMuteRoleGone
	modMsgMuteRoleManaged
	modMsgMuteRoleActorPosition
	modMsgMuteRoleBotPosition
	modMsgTargetHasMuteRole

	// Warnings-storage errors
	modMsgWarnStorageNA
	modMsgWarnStoreFailed
	modMsgWarnLoadFailed

	// Action-rejection reason fragments (translated in canModerateTarget)
	modMsgRejectSelf
	modMsgRejectBot
	modMsgRejectOwner
	modMsgRejectTargetResolveErr
	modMsgRejectTargetNotMember
	modMsgRejectActorRankLow
	modMsgRejectBotRankLow

	// Full cannot-action wrappers ("%s" = userID, "%s" = localized reason)
	modMsgCannotBan
	modMsgCannotKick
	modMsgCannotTimeout
	modMsgCannotMute
	modMsgCannotWarn
	modMsgCannotInspect

	// Action verbs (used as format args in reject messages)
	modVerbBan
	modVerbKick
	modVerbTimeout
	modVerbMute
	modVerbWarn
	modVerbInspect

	// Success messages
	modMsgBanned
	modMsgMassBanned1
	modMsgMassBannedN
	modMsgKicked
	modMsgMuted
	modMsgTimedOut
	modMsgWarned
	modMsgReasonTruncated
	modMsgUnknownUser

	// Warnings list
	modMsgWarningsNone
	modMsgWarningsHeader
	modMsgNoReason

	numModMsgKeys
)

var modCatalogLocales = []discordgo.Locale{discordgo.EnglishUS, discordgo.PortugueseBR}

var modCatalog = map[discordgo.Locale]map[modMsgKey]string{
	discordgo.EnglishUS: {
		modMsgBanDisabled:      "Ban command is disabled for this server.",
		modMsgMassBanDisabled:  "Mass ban command is disabled for this server.",
		modMsgKickDisabled:     "Kick command is disabled for this server.",
		modMsgTimeoutDisabled:  "Timeout command is disabled for this server.",
		modMsgWarnDisabled:     "Warn command is disabled for this server.",
		modMsgWarningsDisabled: "Warnings command is disabled for this server.",

		modMsgInvalidUser:            "Invalid user ID or mention.",
		modMsgInvalidTimeoutDuration: "Please provide a valid timeout duration in minutes.",
		modMsgTimeoutTooLong:         "Timeout duration cannot exceed 40320 minutes (28 days).",
		modMsgNoValidMembers:         "No valid member IDs provided",

		modMsgSessionNotReady:        "Session not ready. Try again shortly.",
		modMsgPermResolverNA:         "Permission resolver not available.",
		modMsgConfigNA:               "Configuration is not available right now.",
		modMsgRolesResolveFailed:     "Failed to resolve server roles.",
		modMsgOwnerResolveFailed:     "Failed to resolve server owner.",
		modMsgBotIdentityNA:          "Bot identity not available.",
		modMsgActorResolveFailed:     "Unable to resolve your member record.",
		modMsgBotMemberResolveFailed: "Unable to resolve the bot member record.",

		modMsgNeedBanPerm:       "You need the Ban Members permission to use this command.",
		modMsgBotNeedBanPerm:    "The bot needs the Ban Members permission to ban members.",
		modMsgNeedKickPerm:      "You need the Kick Members permission to use this command.",
		modMsgBotNeedKickPerm:   "The bot needs the Kick Members permission to kick members.",
		modMsgNeedModeratePerm:  "You need the Moderate Members permission to use this command.",
		modMsgBotNeedTimeoutPerm: "The bot needs the Moderate Members permission to timeout members.",
		modMsgBotNeedWarnPerm:   "The bot needs the Moderate Members permission to manage warnings.",
		modMsgNeedRolesPerm:     "You need the Manage Roles permission to use this command.",
		modMsgBotNeedRolesPerm:  "The bot needs the Manage Roles permission to mute members with the configured mute role.",

		modMsgMuteRoleFeatureOff:    "Mute role moderation is disabled for this server.",
		modMsgMuteRoleNotConfigured: "Mute role is not configured for this server.",
		modMsgMuteRoleContextNA:     "Mute role context is not available right now.",
		modMsgMuteRoleGone:          "Configured mute role is no longer available in this server.",
		modMsgMuteRoleManaged:       "Configured mute role is managed by an integration and cannot be assigned manually.",
		modMsgMuteRoleActorPosition: "Your highest role must stay above the configured mute role.",
		modMsgMuteRoleBotPosition:   "My highest role must stay above the configured mute role.",
		modMsgTargetHasMuteRole:     "target already has the configured mute role",

		modMsgWarnStorageNA:  "Warnings storage is not available for this bot instance.",
		modMsgWarnStoreFailed: "Failed to create warning for %s: %v",
		modMsgWarnLoadFailed:  "Failed to load warnings for %s: %v",

		modMsgRejectSelf:             "cannot %s yourself",
		modMsgRejectBot:              "cannot %s the bot",
		modMsgRejectOwner:            "cannot %s the server owner",
		modMsgRejectTargetResolveErr: "target member could not be resolved right now",
		modMsgRejectTargetNotMember:  "target is not a member of this server",
		modMsgRejectActorRankLow:     "target has an equal or higher role than you",
		modMsgRejectBotRankLow:       "target has an equal or higher role than the bot",

		modMsgCannotBan:     "Cannot ban `%s`: %s.",
		modMsgCannotKick:    "Cannot kick `%s`: %s.",
		modMsgCannotTimeout: "Cannot timeout `%s`: %s.",
		modMsgCannotMute:    "Cannot mute `%s`: %s.",
		modMsgCannotWarn:    "Cannot warn `%s`: %s.",
		modMsgCannotInspect: "Cannot inspect warnings for `%s`: %s.",

		modVerbBan:     "ban",
		modVerbKick:    "kick",
		modVerbTimeout: "timeout",
		modVerbMute:    "mute",
		modVerbWarn:    "warn",
		modVerbInspect: "inspect",

		modMsgBanned:          "%s was banned. Reason: %s.",
		modMsgMassBanned1:     "1 user was banned.",
		modMsgMassBannedN:     "%d users were banned.",
		modMsgKicked:          "%s was kicked. Reason: %s.",
		modMsgMuted:           "%s was muted with role %s. Reason: %s.",
		modMsgTimedOut:        "%s was timed out for %s. Reason: %s.",
		modMsgWarned:          "%s was warned. Case #%d. Reason: %s.",
		modMsgReasonTruncated: " Reason was truncated to fit this reply.",
		modMsgUnknownUser:     "unknown user",

		modMsgWarningsNone:   "No warnings are recorded for %s. This reply stays private because moderation history should stay private.",
		modMsgWarningsHeader: "Here is the recent warning history for %s. This reply stays private because moderation history should stay private:",
		modMsgNoReason:       "No reason provided",
	},
	discordgo.PortugueseBR: {
		modMsgBanDisabled:      "O comando de banimento está desativado neste servidor.",
		modMsgMassBanDisabled:  "O comando de banimento em massa está desativado neste servidor.",
		modMsgKickDisabled:     "O comando de expulsão está desativado neste servidor.",
		modMsgTimeoutDisabled:  "O comando de silenciamento temporário está desativado neste servidor.",
		modMsgWarnDisabled:     "O comando de advertência está desativado neste servidor.",
		modMsgWarningsDisabled: "O comando de histórico de advertências está desativado neste servidor.",

		modMsgInvalidUser:            "ID ou menção de usuário inválido(a).",
		modMsgInvalidTimeoutDuration: "Informe uma duração de silenciamento válida em minutos.",
		modMsgTimeoutTooLong:         "A duração do silenciamento não pode exceder 40320 minutos (28 dias).",
		modMsgNoValidMembers:         "Nenhum ID de membro válido fornecido.",

		modMsgSessionNotReady:        "Sessão não disponível. Tente novamente em breve.",
		modMsgPermResolverNA:         "O resolvedor de permissões não está disponível.",
		modMsgConfigNA:               "A configuração não está disponível no momento.",
		modMsgRolesResolveFailed:     "Falha ao resolver os cargos do servidor.",
		modMsgOwnerResolveFailed:     "Falha ao resolver o dono do servidor.",
		modMsgBotIdentityNA:          "Identidade do bot não disponível.",
		modMsgActorResolveFailed:     "Não foi possível resolver seu registro de membro.",
		modMsgBotMemberResolveFailed: "Não foi possível resolver o registro do membro do bot.",

		modMsgNeedBanPerm:        "Você precisa da permissão de Banir Membros para usar este comando.",
		modMsgBotNeedBanPerm:     "O bot precisa da permissão de Banir Membros para banir membros.",
		modMsgNeedKickPerm:       "Você precisa da permissão de Expulsar Membros para usar este comando.",
		modMsgBotNeedKickPerm:    "O bot precisa da permissão de Expulsar Membros para expulsar membros.",
		modMsgNeedModeratePerm:   "Você precisa da permissão de Moderar Membros para usar este comando.",
		modMsgBotNeedTimeoutPerm: "O bot precisa da permissão de Moderar Membros para silenciar membros temporariamente.",
		modMsgBotNeedWarnPerm:    "O bot precisa da permissão de Moderar Membros para gerenciar advertências.",
		modMsgNeedRolesPerm:      "Você precisa da permissão de Gerenciar Cargos para usar este comando.",
		modMsgBotNeedRolesPerm:   "O bot precisa da permissão de Gerenciar Cargos para silenciar membros com o cargo de mute configurado.",

		modMsgMuteRoleFeatureOff:    "A moderação por cargo de mute está desativada neste servidor.",
		modMsgMuteRoleNotConfigured: "O cargo de mute não está configurado neste servidor.",
		modMsgMuteRoleContextNA:     "O contexto do cargo de mute não está disponível no momento.",
		modMsgMuteRoleGone:          "O cargo de mute configurado não está mais disponível neste servidor.",
		modMsgMuteRoleManaged:       "O cargo de mute configurado é gerenciado por uma integração e não pode ser atribuído manualmente.",
		modMsgMuteRoleActorPosition: "Seu cargo mais alto deve estar acima do cargo de mute configurado.",
		modMsgMuteRoleBotPosition:   "Meu cargo mais alto deve estar acima do cargo de mute configurado.",
		modMsgTargetHasMuteRole:     "o alvo já possui o cargo de mute configurado",

		modMsgWarnStorageNA:   "O armazenamento de advertências não está disponível nesta instância do bot.",
		modMsgWarnStoreFailed: "Falha ao criar advertência para %s: %v",
		modMsgWarnLoadFailed:  "Falha ao carregar advertências para %s: %v",

		modMsgRejectSelf:             "não é possível %s você mesmo(a)",
		modMsgRejectBot:              "não é possível %s o bot",
		modMsgRejectOwner:            "não é possível %s o dono do servidor",
		modMsgRejectTargetResolveErr: "o membro alvo não pôde ser resolvido agora",
		modMsgRejectTargetNotMember:  "o alvo não é membro deste servidor",
		modMsgRejectActorRankLow:     "o alvo tem um cargo igual ou superior ao seu",
		modMsgRejectBotRankLow:       "o alvo tem um cargo igual ou superior ao do bot",

		modMsgCannotBan:     "Não é possível banir `%s`: %s.",
		modMsgCannotKick:    "Não é possível expulsar `%s`: %s.",
		modMsgCannotTimeout: "Não é possível silenciar temporariamente `%s`: %s.",
		modMsgCannotMute:    "Não é possível silenciar `%s`: %s.",
		modMsgCannotWarn:    "Não é possível advertir `%s`: %s.",
		modMsgCannotInspect: "Não é possível inspecionar advertências de `%s`: %s.",

		modVerbBan:     "banir",
		modVerbKick:    "expulsar",
		modVerbTimeout: "silenciar temporariamente",
		modVerbMute:    "silenciar",
		modVerbWarn:    "advertir",
		modVerbInspect: "inspecionar",

		modMsgBanned:          "%s foi banido(a). Motivo: %s.",
		modMsgMassBanned1:     "1 usuário foi banido.",
		modMsgMassBannedN:     "%d usuários foram banidos.",
		modMsgKicked:          "%s foi expulso(a). Motivo: %s.",
		modMsgMuted:           "%s foi silenciado(a) com o cargo %s. Motivo: %s.",
		modMsgTimedOut:        "%s foi silenciado(a) temporariamente por %s. Motivo: %s.",
		modMsgWarned:          "%s recebeu uma advertência. Caso nº %d. Motivo: %s.",
		modMsgReasonTruncated: " O motivo foi truncado para caber nesta resposta.",
		modMsgUnknownUser:     "usuário desconhecido",

		modMsgWarningsNone:   "Nenhuma advertência registrada para %s. Esta resposta fica privada porque o histórico de moderação deve permanecer privado.",
		modMsgWarningsHeader: "Aqui está o histórico recente de advertências de %s. Esta resposta fica privada porque o histórico de moderação deve permanecer privado:",
		modMsgNoReason:       "Sem motivo informado",
	},
}

func modMsg(locale discordgo.Locale, key modMsgKey, args ...any) string {
	msgs, ok := modCatalog[locale]
	if !ok {
		msgs = modCatalog[discordgo.EnglishUS]
	}
	template, ok := msgs[key]
	if !ok {
		template = modCatalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// localizeModVerb maps an internal English action verb to its localized form
// for use in reject-reason fragments such as "cannot %s yourself".
func localizeModVerb(locale discordgo.Locale, verb string) string {
	switch verb {
	case "ban":
		return modMsg(locale, modVerbBan)
	case "kick":
		return modMsg(locale, modVerbKick)
	case "timeout":
		return modMsg(locale, modVerbTimeout)
	case "mute":
		return modMsg(locale, modVerbMute)
	case "warn":
		return modMsg(locale, modVerbWarn)
	case "inspect":
		return modMsg(locale, modVerbInspect)
	default:
		return verb
	}
}
