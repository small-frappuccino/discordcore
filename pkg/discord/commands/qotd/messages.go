package qotd

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// catalogLocales is the ordered set of locales that every QOTD command
// message catalog entry must cover. Tests enforce completeness.
var catalogLocales = []discordgo.Locale{
	discordgo.EnglishUS,
	discordgo.PortugueseBR,
}

type msgKey int

const (
	// Error / guard messages.
	msgListDenied                        msgKey = iota // list pagination denied to non-owner
	msgDeckNotFound                                    // QOTD deck not found
	msgMissingGuild                                    // command only usable in a server
	msgPublishEnableFirst                              // QOTD must be enabled before manual publish
	msgPublishSetChannelFirst                          // channel must be set before manual publish
	msgQuestionIDMustBePositive                        // question ID validation
	msgInvalidQuestionInput                            // fallback for invalid question input
	msgQuestionNotFound                                // question ID not found (%d)
	msgQuestionImmutableDelete                         // question scheduled/used, cannot remove (%d)
	msgQuestionNotUsed                                 // question not used, cannot recover (%d)
	msgQuestionImmutableMarkPublished                  // question already scheduled/published (%d)
	msgQuestionMustBeReadyToMarkPublished              // question must be ready before marking (%d)
	msgAlreadyPublished                                // slot already has a publish
	msgPublishInProgress                               // publish already running
	msgNoQuestionsAvailable                            // no ready questions in active deck
	msgQOTDDisabledPublish                             // QOTD disabled and no channel set
	msgDiscordUnavailable                              // discord session unavailable

	// Success messages.
	msgAddedQuestion               // Added question ID %d to deck `%s`
	msgRemovedQuestion             // Removed question ID %d from deck `%s`
	msgRecoveredQuestion           // Recovered question ID %d ... deck `%s`
	msgRecoveredQuestionRenumbered // Recovered ID %d ... deck `%s` ... now ID %d
	msgMarkedPublished             // Marked question ID %d as published in deck `%s`
	msgPublishedManually           // Published question ID %d from deck `%s`
	msgPublishedManuallyNoSlot     // Published question ID %d from deck `%s` without consuming slot

	// Queue state messages.
	msgQueueHeader             // Automatic QOTD queue for deck `%s`
	msgQueueNoSchedule         // schedule not configured
	msgQueueSchedule           // Automatic schedule: %s UTC
	msgQueueNextSlot           // Next automatic slot: %s (%s)
	msgQueuePublishingDisabled // Publishing is disabled for this deck
	msgQueueNoChannel          // Set a QOTD channel before automatic publishing can run
	msgQueueNextSlotQuestion   // Next automatic slot question: %s
	msgQueueNextAutoQuestion   // Next automatic question: %s
	msgQueueAfterThat          // After that: %s
	msgQueueNoReadyQuestions   // No ready questions available for automatic queue
	msgQueueQuestionRef        // QOTD question ID %d (%s)
	msgQueueDeckNameDefault    // fallback deck name when none is set
	msgQueueSlotUnavailable    // "unavailable" label for unset schedule/slot

	// Queue slot status strings.
	msgSlotStatusWaiting    // waiting for the scheduled publish
	msgSlotStatusDue        // ready to publish now
	msgSlotStatusReserved   // question reserved for the slot
	msgSlotStatusRecovering // slot publish recovery pending
	msgSlotStatusPublished  // slot already published
	msgSlotStatusDisabled   // automatic publishing unavailable

	numMsgKeys // sentinel — keep last
)

var catalog = map[discordgo.Locale]map[msgKey]string{
	discordgo.EnglishUS: {
		msgListDenied:                        "Only the user who opened this list can change pages.",
		msgDeckNotFound:                      "QOTD deck not found",
		msgMissingGuild:                      "This command can only be used in a server",
		msgPublishEnableFirst:                "Enable QOTD publishing for the active deck before publishing manually.",
		msgPublishSetChannelFirst:            "Set a QOTD channel for the active deck before publishing manually.",
		msgQuestionIDMustBePositive:          "Question ID must be greater than zero.",
		msgInvalidQuestionInput:              "Invalid QOTD question input",
		msgQuestionNotFound:                  "QOTD question ID %d was not found.",
		msgQuestionImmutableDelete:           "QOTD question ID %d is already scheduled or used and cannot be removed.",
		msgQuestionNotUsed:                   "QOTD question ID %d is not used and cannot be recovered.",
		msgQuestionImmutableMarkPublished:    "QOTD question ID %d is already scheduled or published and cannot be marked manually.",
		msgQuestionMustBeReadyToMarkPublished: "QOTD question ID %d must be ready before it can be marked as published.",
		msgAlreadyPublished:                  "A QOTD question has already been published for the current slot.",
		msgPublishInProgress:                 "A QOTD publish is already in progress for the current slot.",
		msgNoQuestionsAvailable:              "No ready QOTD questions are available in the active deck.",
		msgQOTDDisabledPublish:               "Enable QOTD publishing and set a channel before publishing manually.",
		msgDiscordUnavailable:                "Discord session unavailable for manual publish.",
		msgAddedQuestion:                     "Added QOTD question ID %d to deck `%s`.",
		msgRemovedQuestion:                   "Removed QOTD question ID %d from deck `%s`.",
		msgRecoveredQuestion:                 "Recovered QOTD question ID %d from used to ready in deck `%s`.",
		msgRecoveredQuestionRenumbered:       "Recovered QOTD question ID %d from used to ready in deck `%s` and it is now listed as ID %d.",
		msgMarkedPublished:                   "Marked QOTD question ID %d as already published in deck `%s` without changing the day state.",
		msgPublishedManually:                 "Published QOTD question ID %d manually from deck `%s`.",
		msgPublishedManuallyNoSlot:           "Published QOTD question ID %d manually from deck `%s` without consuming the automatic slot.",
		msgQueueHeader:                       "Automatic QOTD queue for deck `%s`.",
		msgQueueNoSchedule:                   "Automatic publish schedule is not configured.",
		msgQueueSchedule:                     "Automatic schedule: %s UTC.",
		msgQueueNextSlot:                     "Next automatic slot: %s (%s).",
		msgQueuePublishingDisabled:           "Publishing is disabled for this deck.",
		msgQueueNoChannel:                    "Set a QOTD channel before automatic publishing can run.",
		msgQueueNextSlotQuestion:             "Next automatic slot question: %s.",
		msgQueueNextAutoQuestion:             "Next automatic question: %s.",
		msgQueueAfterThat:                    "After that: %s.",
		msgQueueNoReadyQuestions:             "No ready QOTD questions are available for the automatic queue.",
		msgQueueQuestionRef:                  "QOTD question ID %d (%s)",
		msgQueueDeckNameDefault:              "Default",
		msgQueueSlotUnavailable:              "unavailable",
		msgSlotStatusWaiting:                 "waiting for the scheduled publish",
		msgSlotStatusDue:                     "ready to publish now",
		msgSlotStatusReserved:                "question reserved for the slot",
		msgSlotStatusRecovering:              "slot publish recovery pending",
		msgSlotStatusPublished:               "slot already published",
		msgSlotStatusDisabled:                "automatic publishing unavailable",
	},
	discordgo.PortugueseBR: {
		msgListDenied:                        "Somente quem abriu esta lista pode trocar de página.",
		msgDeckNotFound:                      "Deck QOTD não encontrado",
		msgMissingGuild:                      "Este comando só pode ser usado em um servidor",
		msgPublishEnableFirst:                "Ative a publicação do QOTD para o deck ativo antes de publicar manualmente.",
		msgPublishSetChannelFirst:            "Configure um canal QOTD para o deck ativo antes de publicar manualmente.",
		msgQuestionIDMustBePositive:          "O ID da pergunta deve ser maior que zero.",
		msgInvalidQuestionInput:              "Entrada de pergunta QOTD inválida",
		msgQuestionNotFound:                  "Pergunta QOTD ID %d não encontrada.",
		msgQuestionImmutableDelete:           "A pergunta QOTD ID %d já está agendada ou usada e não pode ser removida.",
		msgQuestionNotUsed:                   "A pergunta QOTD ID %d não está usada e não pode ser recuperada.",
		msgQuestionImmutableMarkPublished:    "A pergunta QOTD ID %d já está agendada ou publicada e não pode ser marcada manualmente.",
		msgQuestionMustBeReadyToMarkPublished: "A pergunta QOTD ID %d precisa estar pronta antes de ser marcada como publicada.",
		msgAlreadyPublished:                  "Uma pergunta QOTD já foi publicada para o slot atual.",
		msgPublishInProgress:                 "Uma publicação QOTD já está em andamento para o slot atual.",
		msgNoQuestionsAvailable:              "Não há perguntas QOTD prontas disponíveis no deck ativo.",
		msgQOTDDisabledPublish:               "Ative a publicação do QOTD e configure um canal antes de publicar manualmente.",
		msgDiscordUnavailable:                "Sessão do Discord indisponível para publicação manual.",
		msgAddedQuestion:                     "Pergunta QOTD ID %d adicionada ao deck `%s`.",
		msgRemovedQuestion:                   "Pergunta QOTD ID %d removida do deck `%s`.",
		msgRecoveredQuestion:                 "Pergunta QOTD ID %d recuperada de usada para pronta no deck `%s`.",
		msgRecoveredQuestionRenumbered:       "Pergunta QOTD ID %d recuperada de usada para pronta no deck `%s` e agora aparece como ID %d.",
		msgMarkedPublished:                   "Pergunta QOTD ID %d marcada como já publicada no deck `%s` sem alterar o estado do dia.",
		msgPublishedManually:                 "Pergunta QOTD ID %d publicada manualmente do deck `%s`.",
		msgPublishedManuallyNoSlot:           "Pergunta QOTD ID %d publicada manualmente do deck `%s` sem consumir o slot automático.",
		msgQueueHeader:                       "Fila automática do QOTD para o deck `%s`.",
		msgQueueNoSchedule:                   "O agendamento de publicação automática não está configurado.",
		msgQueueSchedule:                     "Agendamento automático: %s UTC.",
		msgQueueNextSlot:                     "Próximo slot automático: %s (%s).",
		msgQueuePublishingDisabled:           "A publicação está desativada para este deck.",
		msgQueueNoChannel:                    "Configure um canal QOTD antes de a publicação automática funcionar.",
		msgQueueNextSlotQuestion:             "Próxima pergunta do slot automático: %s.",
		msgQueueNextAutoQuestion:             "Próxima pergunta automática: %s.",
		msgQueueAfterThat:                    "Depois disso: %s.",
		msgQueueNoReadyQuestions:             "Não há perguntas QOTD prontas disponíveis para a fila automática.",
		msgQueueQuestionRef:                  "Pergunta QOTD ID %d (%s)",
		msgQueueDeckNameDefault:              "Padrão",
		msgQueueSlotUnavailable:              "indisponível",
		msgSlotStatusWaiting:                 "aguardando a publicação agendada",
		msgSlotStatusDue:                     "pronta para publicar agora",
		msgSlotStatusReserved:                "pergunta reservada para o slot",
		msgSlotStatusRecovering:              "recuperação de publicação do slot pendente",
		msgSlotStatusPublished:               "slot já publicado",
		msgSlotStatusDisabled:                "publicação automática indisponível",
	},
}

// msg looks up the message template for the given locale and key, applies
// fmt.Sprintf with any extra args when present, and falls back to en-US when
// the locale is not in the catalog.
func msg(locale discordgo.Locale, key msgKey, args ...any) string {
	msgs, ok := catalog[locale]
	if !ok {
		msgs = catalog[discordgo.EnglishUS]
	}
	tmpl, ok := msgs[key]
	if !ok {
		tmpl = catalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}
