package config

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// cfgCatalogLocales is the ordered set of locales that every QOTD config
// message catalog entry must cover. Tests enforce completeness.
var cfgCatalogLocales = []discordgo.Locale{
	discordgo.EnglishUS,
	discordgo.PortugueseBR,
}

type cfgMsgKey int

const (
	// Get command display strings.
	cfgMsgHeader          cfgMsgKey = iota // "**QOTD Configuration:**"
	cfgMsgActiveDeckMulti                  // "Active Deck: %s (... %d decks total)"
	cfgMsgActiveDeckSingle                 // "Active Deck: %s"
	cfgMsgEnabledLine                      // "QOTD Enabled: %t"
	cfgMsgChannelLine                      // "QOTD Channel: %s"
	cfgMsgScheduleLine                     // "QOTD Schedule: %s"
	cfgMsgEmbedTitle                       // embed title: "QOTD Configuration"

	// Confirmation messages.
	cfgMsgPublishingState // "QOTD publishing is now %s for deck `%s`."
	cfgMsgChannelSet      // "QOTD posts for deck `%s` will now go to <#%s>. Publishing stays %s."
	cfgMsgScheduleSet     // "QOTD for deck `%s` will now post at %s."

	// State label fragments used inside confirmation messages.
	cfgMsgStateEnabled  // "enabled"
	cfgMsgStateDisabled // "disabled"

	// Fallback deck name.
	cfgMsgDeckDefault // "default"

	// Error messages (all ephemeral).
	cfgMsgErrNoChannel          // no channel set when enabling
	cfgMsgErrSetChannelFirst    // channel required before applying change
	cfgMsgErrSaveFailed         // config save failed
	cfgMsgErrSetupNotLoaded     // QOTD setup could not be loaded
	cfgMsgErrInvalidInput       // generic invalid QOTD input
	cfgMsgErrIncompleteSchedule // schedule incomplete when enabling

	numCfgMsgKeys // sentinel — keep last
)

var cfgCatalog = map[discordgo.Locale]map[cfgMsgKey]string{
	discordgo.EnglishUS: {
		cfgMsgHeader:            "**QOTD Configuration:**",
		cfgMsgActiveDeckMulti:   "Active Deck: %s (these settings apply to this deck only — %d decks total)",
		cfgMsgActiveDeckSingle:  "Active Deck: %s",
		cfgMsgEnabledLine:       "QOTD Enabled: %t",
		cfgMsgChannelLine:       "QOTD Channel: %s",
		cfgMsgScheduleLine:      "QOTD Schedule: %s",
		cfgMsgEmbedTitle:        "QOTD Configuration",
		cfgMsgPublishingState:   "QOTD publishing is now %s for deck `%s`.",
		cfgMsgChannelSet:        "QOTD posts for deck `%s` will now go to <#%s>. Publishing stays %s.",
		cfgMsgScheduleSet:       "QOTD for deck `%s` will now post at %s.",
		cfgMsgStateEnabled:      "enabled",
		cfgMsgStateDisabled:     "disabled",
		cfgMsgDeckDefault:       "default",
		cfgMsgErrNoChannel:      "QOTD publishing couldn't be turned on yet because this deck still has no channel. This reply stays private so that can be fixed first.",
		cfgMsgErrSetChannelFirst: "This change needs a channel before it can be applied, so this reply stays private.",
		cfgMsgErrSaveFailed:     "That change couldn't be saved. This reply stays private so it can be adjusted and retried without extra channel noise.",
		cfgMsgErrSetupNotLoaded: "The QOTD setup for this server couldn't be loaded, so this reply stays private.",
		cfgMsgErrInvalidInput:   "That QOTD setup couldn't be applied because part of the configuration is invalid. This reply stays private.",
		cfgMsgErrIncompleteSchedule: "QOTD publishing couldn't be turned on yet because the schedule is incomplete. This reply stays private so the setup can be finished first.",
	},
	discordgo.PortugueseBR: {
		cfgMsgHeader:            "**Configuração do QOTD:**",
		cfgMsgActiveDeckMulti:   "Deck Ativo: %s (estas configurações se aplicam apenas a este deck — %d decks no total)",
		cfgMsgActiveDeckSingle:  "Deck Ativo: %s",
		cfgMsgEnabledLine:       "QOTD Ativado: %t",
		cfgMsgChannelLine:       "Canal do QOTD: %s",
		cfgMsgScheduleLine:      "Agendamento do QOTD: %s",
		cfgMsgEmbedTitle:        "Configuração do QOTD",
		cfgMsgPublishingState:   "A publicação do QOTD está agora %s para o deck `%s`.",
		cfgMsgChannelSet:        "As publicações do QOTD para o deck `%s` agora serão enviadas para <#%s>. A publicação permanece %s.",
		cfgMsgScheduleSet:       "O QOTD para o deck `%s` será publicado às %s.",
		cfgMsgStateEnabled:      "ativada",
		cfgMsgStateDisabled:     "desativada",
		cfgMsgDeckDefault:       "padrão",
		cfgMsgErrNoChannel:      "A publicação do QOTD ainda não pôde ser ativada porque este deck não tem canal configurado. Esta resposta fica privada para que isso possa ser corrigido primeiro.",
		cfgMsgErrSetChannelFirst: "Esta alteração precisa de um canal antes de ser aplicada, por isso a resposta fica privada.",
		cfgMsgErrSaveFailed:     "A alteração não pôde ser salva. Esta resposta fica privada para que possa ser ajustada e tentada novamente sem ruído no canal.",
		cfgMsgErrSetupNotLoaded: "A configuração do QOTD para este servidor não pôde ser carregada, por isso a resposta fica privada.",
		cfgMsgErrInvalidInput:   "A configuração do QOTD não pôde ser aplicada porque parte dela está inválida. Esta resposta fica privada.",
		cfgMsgErrIncompleteSchedule: "A publicação do QOTD ainda não pôde ser ativada porque o agendamento está incompleto. Esta resposta fica privada para que a configuração possa ser concluída primeiro.",
	},
}

// tc looks up a QOTD config message template for the given locale and key,
// applies fmt.Sprintf with any extra args when present, and falls back to
// en-US when the locale is not in the catalog.
func tc(locale discordgo.Locale, key cfgMsgKey, args ...any) string {
	msgs, ok := cfgCatalog[locale]
	if !ok {
		msgs = cfgCatalog[discordgo.EnglishUS]
	}
	tmpl, ok := msgs[key]
	if !ok {
		tmpl = cfgCatalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}
