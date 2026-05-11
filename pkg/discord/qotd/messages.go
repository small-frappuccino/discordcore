package qotd

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// embedCatalogLocales is the ordered set of locales that every embed message
// catalog entry must cover. Tests enforce completeness.
var embedCatalogLocales = []discordgo.Locale{
	discordgo.EnglishUS,
	discordgo.PortugueseBR,
}

type embedMsgKey int

const (
	embedMsgTitle       embedMsgKey = iota // embed title
	embedMsgEmpty                          // empty deck description
	embedMsgPageInfo                       // "Page %d of %d • %d questions"
	embedMsgPublishNext                    // "publishes next" label
	embedMsgFooter                         // footer: "%s -- Page %d/%d -- %d questions"
	embedMsgDeckDefault                    // fallback deck name

	embedMsgStatusReady    // status label: ready
	embedMsgStatusDraft    // status label: draft
	embedMsgStatusReserved // status label: reserved
	embedMsgStatusUsed     // status label: used
	embedMsgStatusDisabled // status label: disabled
	embedMsgStatusUnknown  // status label: unknown

	numEmbedMsgKeys // sentinel — keep last
)

var embedCatalog = map[discordgo.Locale]map[embedMsgKey]string{
	discordgo.EnglishUS: {
		embedMsgTitle:          "☆ questions list! ☆",
		embedMsgEmpty:          "This deck does not have any questions yet.\n\nPage 1 of 1 • 0 questions",
		embedMsgPageInfo:       "Page %d of %d • %d questions",
		embedMsgPublishNext:    "publishes next",
		embedMsgFooter:         "%s -- Page %d/%d -- %d questions",
		embedMsgDeckDefault:    "Default",
		embedMsgStatusReady:    "ready",
		embedMsgStatusDraft:    "draft",
		embedMsgStatusReserved: "reserved",
		embedMsgStatusUsed:     "used",
		embedMsgStatusDisabled: "disabled",
		embedMsgStatusUnknown:  "unknown",
	},
	discordgo.PortugueseBR: {
		embedMsgTitle:          "☆ lista de perguntas! ☆",
		embedMsgEmpty:          "Este deck ainda não tem perguntas.\n\nPágina 1 de 1 • 0 perguntas",
		embedMsgPageInfo:       "Página %d de %d • %d perguntas",
		embedMsgPublishNext:    "próxima a publicar",
		embedMsgFooter:         "%s -- Página %d/%d -- %d perguntas",
		embedMsgDeckDefault:    "Padrão",
		embedMsgStatusReady:    "pronta",
		embedMsgStatusDraft:    "rascunho",
		embedMsgStatusReserved: "reservada",
		embedMsgStatusUsed:     "usada",
		embedMsgStatusDisabled: "desativada",
		embedMsgStatusUnknown:  "desconhecida",
	},
}

// te looks up an embed message template for the given locale and key, applies
// fmt.Sprintf with any extra args when present, and falls back to en-US when
// the locale is not in the catalog.
func te(locale discordgo.Locale, key embedMsgKey, args ...any) string {
	msgs, ok := embedCatalog[locale]
	if !ok {
		msgs = embedCatalog[discordgo.EnglishUS]
	}
	tmpl, ok := msgs[key]
	if !ok {
		tmpl = embedCatalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}
