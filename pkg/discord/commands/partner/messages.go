package partner

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type ptnMsgKey int

const (
	ptnMsgAlreadyExists     ptnMsgKey = iota
	ptnMsgDuplicateUpdate
	ptnMsgCreateFailed
	ptnMsgCreateLookupFailed
	ptnMsgAddedPrefix
	ptnMsgNotFound
	ptnMsgReadFailed
	ptnMsgReadPrefix
	ptnMsgLoadFailed
	ptnMsgUpdateFailed
	ptnMsgUpdateLookupFailed
	ptnMsgUpdatedPrefix
	ptnMsgDeleteFailed
	ptnMsgDeleted
	ptnMsgListFailed
	ptnMsgListEmpty
	ptnMsgListHeader
	ptnMsgListTitle
	ptnMsgFandomDefault
	ptnMsgEntryName
	ptnMsgEntryFandom
	ptnMsgEntryInvite
	ptnMsgSyncNotConfigured
	ptnMsgSyncFailed
	ptnMsgSynced
	numPtnMsgKeys
)

var ptnCatalogLocales = []discordgo.Locale{discordgo.EnglishUS, discordgo.PortugueseBR}

var ptnCatalog = map[discordgo.Locale]map[ptnMsgKey]string{
	discordgo.EnglishUS: {
		ptnMsgAlreadyExists:      "That partner couldn't be added because another entry already uses the same name or invite. This reply stays private.",
		ptnMsgDuplicateUpdate:    "That partner couldn't be updated because another entry already uses the same name or invite. This reply stays private.",
		ptnMsgCreateFailed:       "Failed to create partner: %v",
		ptnMsgCreateLookupFailed: "Partner created but lookup failed: %v",
		ptnMsgAddedPrefix:        "The partner entry was added and this reply stays private because it changes the partner board setup for this server.",
		ptnMsgNotFound:           "That partner entry couldn't be found, so this reply stays private because it concerns the partner board setup.",
		ptnMsgReadFailed:         "Failed to read partner: %v",
		ptnMsgReadPrefix:         "Here are the saved details for that partner entry. This reply stays private because it shows the current partner board setup.",
		ptnMsgLoadFailed:         "Failed to load current partner: %v",
		ptnMsgUpdateFailed:       "Failed to update partner: %v",
		ptnMsgUpdateLookupFailed: "Partner updated but lookup failed: %v",
		ptnMsgUpdatedPrefix:      "The partner entry was updated and this reply stays private because it changes the partner board setup for this server.",
		ptnMsgDeleteFailed:       "Failed to delete partner: %v",
		ptnMsgDeleted:            "Partner `%s` was removed and this reply stays private because it changes the partner board setup for this server.",
		ptnMsgListFailed:         "Failed to list partners: %v",
		ptnMsgListEmpty:          "No partners are configured yet. This reply stays private because it reflects the current partner board setup.",
		ptnMsgListHeader:         "These are the configured partner entries. This reply stays private because it reflects the current partner board setup:\n",
		ptnMsgListTitle:          "Partner List",
		ptnMsgFandomDefault:      "Other",
		ptnMsgEntryName:          "Name: `%s`",
		ptnMsgEntryFandom:        "Fandom: `%s`",
		ptnMsgEntryInvite:        "Invite: %s",
		ptnMsgSyncNotConfigured:  "Partner sync is not configured",
		ptnMsgSyncFailed:         "Failed to sync partner board: %v",
		ptnMsgSynced:             "The partner board was synced and this reply stays private because it is an internal admin action.",
	},
	discordgo.PortugueseBR: {
		ptnMsgAlreadyExists:      "Este parceiro não pôde ser adicionado porque outra entrada já usa o mesmo nome ou convite. Esta resposta fica privada.",
		ptnMsgDuplicateUpdate:    "Este parceiro não pôde ser atualizado porque outra entrada já usa o mesmo nome ou convite. Esta resposta fica privada.",
		ptnMsgCreateFailed:       "Falha ao criar o parceiro: %v",
		ptnMsgCreateLookupFailed: "Parceiro criado, mas a consulta falhou: %v",
		ptnMsgAddedPrefix:        "A entrada de parceiro foi adicionada e esta resposta fica privada porque altera a configuração da board de parceiros deste servidor.",
		ptnMsgNotFound:           "Essa entrada de parceiro não foi encontrada. Esta resposta fica privada porque diz respeito à configuração da board de parceiros.",
		ptnMsgReadFailed:         "Falha ao ler o parceiro: %v",
		ptnMsgReadPrefix:         "Aqui estão os detalhes salvos dessa entrada de parceiro. Esta resposta fica privada porque mostra a configuração atual da board de parceiros.",
		ptnMsgLoadFailed:         "Falha ao carregar o parceiro atual: %v",
		ptnMsgUpdateFailed:       "Falha ao atualizar o parceiro: %v",
		ptnMsgUpdateLookupFailed: "Parceiro atualizado, mas a consulta falhou: %v",
		ptnMsgUpdatedPrefix:      "A entrada de parceiro foi atualizada e esta resposta fica privada porque altera a configuração da board de parceiros deste servidor.",
		ptnMsgDeleteFailed:       "Falha ao excluir o parceiro: %v",
		ptnMsgDeleted:            "O parceiro `%s` foi removido e esta resposta fica privada porque altera a configuração da board de parceiros deste servidor.",
		ptnMsgListFailed:         "Falha ao listar os parceiros: %v",
		ptnMsgListEmpty:          "Nenhum parceiro está configurado ainda. Esta resposta fica privada porque reflete a configuração atual da board de parceiros.",
		ptnMsgListHeader:         "Estas são as entradas de parceiros configuradas. Esta resposta fica privada porque reflete a configuração atual da board de parceiros:\n",
		ptnMsgListTitle:          "Lista de Parceiros",
		ptnMsgFandomDefault:      "Outro",
		ptnMsgEntryName:          "Nome: `%s`",
		ptnMsgEntryFandom:        "Fandom: `%s`",
		ptnMsgEntryInvite:        "Convite: %s",
		ptnMsgSyncNotConfigured:  "A sincronização de parceiros não está configurada.",
		ptnMsgSyncFailed:         "Falha ao sincronizar a board de parceiros: %v",
		ptnMsgSynced:             "A board de parceiros foi sincronizada e esta resposta fica privada porque é uma ação administrativa interna.",
	},
}

func ptnMsg(locale discordgo.Locale, key ptnMsgKey, args ...any) string {
	msgs, ok := ptnCatalog[locale]
	if !ok {
		msgs = ptnCatalog[discordgo.EnglishUS]
	}
	template, ok := msgs[key]
	if !ok {
		template = ptnCatalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}
