package runtime

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type runtimeMsgKey int

const (
	runtimeMsgLoadFailed         runtimeMsgKey = iota
	runtimeMsgPanelTitle
	runtimeMsgPanelDesc1
	runtimeMsgPanelScopeLabel
	runtimeMsgPanelSelectedLine
	runtimeMsgPanelNavHint
	runtimeMsgPanelFooter
	runtimeMsgScopeGlobal
	runtimeMsgScopeGuild
	runtimeMsgDetailTitle
	runtimeMsgDetailFooter
	runtimeMsgDetailScope
	runtimeMsgDetailGroup
	runtimeMsgDetailType
	runtimeMsgDetailDefault
	runtimeMsgDetailCurrent
	runtimeMsgDetailDescription
	runtimeMsgDetailEffect
	runtimeMsgDetailGuildOnly
	runtimeMsgHelpTitle
	runtimeMsgHelpDesc
	runtimeMsgErrorTitle
	runtimeMsgFilterPlaceholder
	runtimeMsgFilterGroupDesc
	runtimeMsgSelectKeyPlaceholder
	runtimeMsgSelectKeyInGroup
	runtimeMsgSelectTooMany
	runtimeMsgSelectFirst25
	runtimeMsgBtnDetails
	runtimeMsgBtnToggle
	runtimeMsgBtnEdit
	runtimeMsgBtnReset
	runtimeMsgBtnReload
	runtimeMsgBtnHelp
	runtimeMsgBtnMain
	runtimeMsgBtnSwitchGlobal
	runtimeMsgBtnBack
	runtimeMsgInvalidState
	runtimeMsgGuildOnlyRestriction
	runtimeMsgUnknownKey
	runtimeMsgUnknownAction
	runtimeMsgToggleOnlyBool
	runtimeMsgSaveFailed
	runtimeMsgToggleFailed
	runtimeMsgEditOnlyNonBool
	runtimeMsgHotApplyWarning
	runtimeMsgInvalidModalBool
	runtimeMsgInvalidValue
	runtimeMsgRestartRequired
	runtimeMsgRestartRecommended
	runtimeMsgExpiredPanel
	runtimeMsgDeniedPanel
	numRuntimeMsgKeys
)

var runtimeCatalogLocales = []discordgo.Locale{discordgo.EnglishUS, discordgo.PortugueseBR}

var runtimeCatalog = map[discordgo.Locale]map[runtimeMsgKey]string{
	discordgo.EnglishUS: {
		runtimeMsgLoadFailed:           "The runtime configuration couldn't be loaded, so this reply stays private: %v",
		runtimeMsgPanelTitle:           "Runtime Configuration",
		runtimeMsgPanelDesc1:           "This panel lets you edit the persisted runtime configuration that replaced the old operational environment variables.",
		runtimeMsgPanelScopeLabel:      "Scope: **%s**",
		runtimeMsgPanelSelectedLine:    "Selected: `%s` | Type: **%s** | Default: **%s** | %s",
		runtimeMsgPanelNavHint:         "Use the menus to filter and navigate, then use the buttons to edit the selected setting.",
		runtimeMsgPanelFooter:          "Some changes can be applied immediately, especially THEME and selected ALICE_DISABLE_* settings.",
		runtimeMsgScopeGlobal:          "Global",
		runtimeMsgScopeGuild:           "Guild (`%s`)",
		runtimeMsgDetailTitle:          "Runtime Configuration - Details",
		runtimeMsgDetailFooter:         "Use BACK to return to the panel.",
		runtimeMsgDetailScope:          "**Scope:** %s",
		runtimeMsgDetailGroup:          "**Group:** %s",
		runtimeMsgDetailType:           "**Type:** %s",
		runtimeMsgDetailDefault:        "**Default:** %s",
		runtimeMsgDetailCurrent:        "**Current:** %s",
		runtimeMsgDetailDescription:    "**Description:** %s",
		runtimeMsgDetailEffect:         "**Effect:** %s",
		runtimeMsgDetailGuildOnly:      "**Note:** This setting can only be configured per guild.",
		runtimeMsgHelpTitle:            "Runtime Configuration - Help",
		runtimeMsgHelpDesc:             "This panel edits the persisted `runtime_config`.\n\n**Notes:**\n- Names stay in ALL CAPS so they still map cleanly to the old env var mental model.\n- The bot no longer reads these options from the environment, except for the token.\n- Some changes can be hot-applied, especially THEME and selected ALICE_DISABLE_* settings.\n\n**How to edit:**\n1) Filter by group if needed and select a key.\n2) For boolean values, use TOGGLE.\n3) For other values, use EDIT and fill in the modal.\n4) RESET clears the saved value and restores the code default.",
		runtimeMsgErrorTitle:           "Runtime Configuration - Error",
		runtimeMsgFilterPlaceholder:    "Filter by group\u2026",
		runtimeMsgFilterGroupDesc:      "Filter keys by group",
		runtimeMsgSelectKeyPlaceholder: "Select key\u2026",
		runtimeMsgSelectKeyInGroup:     "Select key in %s\u2026",
		runtimeMsgSelectTooMany:        "Too many keys \u2014 select a group first\u2026",
		runtimeMsgSelectFirst25:        "Showing first 25 keys in %s\u2026",
		runtimeMsgBtnDetails:           "DETAILS",
		runtimeMsgBtnToggle:            "TOGGLE",
		runtimeMsgBtnEdit:              "EDIT",
		runtimeMsgBtnReset:             "RESET",
		runtimeMsgBtnReload:            "RELOAD",
		runtimeMsgBtnHelp:              "HELP",
		runtimeMsgBtnMain:              "MAIN",
		runtimeMsgBtnSwitchGlobal:      "SWITCH TO GLOBAL",
		runtimeMsgBtnBack:              "BACK",
		runtimeMsgInvalidState:         "Invalid interaction state",
		runtimeMsgGuildOnlyRestriction: "This setting can only be configured per-guild.",
		runtimeMsgUnknownKey:           "Unknown key",
		runtimeMsgUnknownAction:        "Unknown action",
		runtimeMsgToggleOnlyBool:       "TOGGLE is only valid for boolean keys",
		runtimeMsgSaveFailed:           "Failed to save: %v",
		runtimeMsgToggleFailed:         "Toggle failed: %v",
		runtimeMsgEditOnlyNonBool:      "EDIT is not valid for boolean keys (use TOGGLE)",
		runtimeMsgHotApplyWarning:      "The runtime configuration was saved, but the change couldn't be applied immediately. A restart may be required.\nError: %v",
		runtimeMsgInvalidModalBool:     "Invalid modal for bool key",
		runtimeMsgInvalidValue:         "Invalid value: %v",
		runtimeMsgRestartRequired:      "restart required",
		runtimeMsgRestartRecommended:   "restart recommended",
		runtimeMsgExpiredPanel:         "This runtime config panel is no longer valid. Reopen /config runtime to continue.",
		runtimeMsgDeniedPanel:          "Only the person who opened this runtime config panel can use it. This reply stays private because it belongs to that admin session.",
	},
	discordgo.PortugueseBR: {
		runtimeMsgLoadFailed:           "A configuração de runtime não pôde ser carregada. Esta resposta fica privada: %v",
		runtimeMsgPanelTitle:           "Configuração de Runtime",
		runtimeMsgPanelDesc1:           "Este painel permite editar a configuração de runtime persistida que substituiu as variáveis de ambiente operacionais.",
		runtimeMsgPanelScopeLabel:      "Escopo: **%s**",
		runtimeMsgPanelSelectedLine:    "Selecionado: `%s` | Tipo: **%s** | Padrão: **%s** | %s",
		runtimeMsgPanelNavHint:         "Use os menus para filtrar e navegar, depois use os botões para editar a configuração selecionada.",
		runtimeMsgPanelFooter:          "Algumas mudanças podem ser aplicadas imediatamente, especialmente THEME e as configurações ALICE_DISABLE_*.",
		runtimeMsgScopeGlobal:          "Global",
		runtimeMsgScopeGuild:           "Guild (`%s`)",
		runtimeMsgDetailTitle:          "Configuração de Runtime - Detalhes",
		runtimeMsgDetailFooter:         "Use VOLTAR para retornar ao painel.",
		runtimeMsgDetailScope:          "**Escopo:** %s",
		runtimeMsgDetailGroup:          "**Grupo:** %s",
		runtimeMsgDetailType:           "**Tipo:** %s",
		runtimeMsgDetailDefault:        "**Padrão:** %s",
		runtimeMsgDetailCurrent:        "**Atual:** %s",
		runtimeMsgDetailDescription:    "**Descrição:** %s",
		runtimeMsgDetailEffect:         "**Efeito:** %s",
		runtimeMsgDetailGuildOnly:      "**Nota:** Esta configuração só pode ser definida por guild.",
		runtimeMsgHelpTitle:            "Configuração de Runtime - Ajuda",
		runtimeMsgHelpDesc:             "Este painel edita a configuração `runtime_config` persistida.\n\n**Notas:**\n- Os nomes ficam em MAIÚSCULAS para manter a correspondência com o modelo mental das variáveis de ambiente antigas.\n- O bot não lê mais essas opções do ambiente, exceto pelo token.\n- Algumas mudanças podem ser aplicadas imediatamente, especialmente THEME e as configurações ALICE_DISABLE_*.\n\n**Como editar:**\n1) Filtre por grupo se necessário e selecione uma chave.\n2) Para valores booleanos, use ALTERNAR.\n3) Para outros valores, use EDITAR e preencha o modal.\n4) RESETAR limpa o valor salvo e restaura o padrão do código.",
		runtimeMsgErrorTitle:           "Configuração de Runtime - Erro",
		runtimeMsgFilterPlaceholder:    "Filtrar por grupo\u2026",
		runtimeMsgFilterGroupDesc:      "Filtrar chaves por grupo",
		runtimeMsgSelectKeyPlaceholder: "Selecionar chave\u2026",
		runtimeMsgSelectKeyInGroup:     "Selecionar chave em %s\u2026",
		runtimeMsgSelectTooMany:        "Muitas chaves \u2014 selecione um grupo primeiro\u2026",
		runtimeMsgSelectFirst25:        "Mostrando as primeiras 25 chaves em %s\u2026",
		runtimeMsgBtnDetails:           "DETALHES",
		runtimeMsgBtnToggle:            "ALTERNAR",
		runtimeMsgBtnEdit:              "EDITAR",
		runtimeMsgBtnReset:             "RESETAR",
		runtimeMsgBtnReload:            "RECARREGAR",
		runtimeMsgBtnHelp:              "AJUDA",
		runtimeMsgBtnMain:              "PRINCIPAL",
		runtimeMsgBtnSwitchGlobal:      "MUDAR PARA GLOBAL",
		runtimeMsgBtnBack:              "VOLTAR",
		runtimeMsgInvalidState:         "Estado de interação inválido",
		runtimeMsgGuildOnlyRestriction: "Esta configuração só pode ser definida por guild.",
		runtimeMsgUnknownKey:           "Chave desconhecida",
		runtimeMsgUnknownAction:        "Ação desconhecida",
		runtimeMsgToggleOnlyBool:       "ALTERNAR só é válido para chaves booleanas",
		runtimeMsgSaveFailed:           "Falha ao salvar: %v",
		runtimeMsgToggleFailed:         "Falha ao alternar: %v",
		runtimeMsgEditOnlyNonBool:      "EDITAR não é válido para chaves booleanas (use ALTERNAR)",
		runtimeMsgHotApplyWarning:      "A configuração de runtime foi salva, mas a mudança não pôde ser aplicada imediatamente. Pode ser necessário reiniciar.\nErro: %v",
		runtimeMsgInvalidModalBool:     "Modal inválido para chave booleana",
		runtimeMsgInvalidValue:         "Valor inválido: %v",
		runtimeMsgRestartRequired:      "reinicialização obrigatória",
		runtimeMsgRestartRecommended:   "reinicialização recomendada",
		runtimeMsgExpiredPanel:         "Este painel de configuração de runtime não é mais válido. Reabra /config runtime para continuar.",
		runtimeMsgDeniedPanel:          "Apenas a pessoa que abriu este painel de configuração de runtime pode usá-lo. Esta resposta fica privada porque pertence àquela sessão administrativa.",
	},
}

func runtimeMsg(locale discordgo.Locale, key runtimeMsgKey, args ...any) string {
	msgs, ok := runtimeCatalog[locale]
	if !ok {
		msgs = runtimeCatalog[discordgo.EnglishUS]
	}
	s, ok := msgs[key]
	if !ok {
		s = runtimeCatalog[discordgo.EnglishUS][key]
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

func localizeRestartHint(locale discordgo.Locale, h restartHint) string {
	switch h {
	case restartRequired:
		return runtimeMsg(locale, runtimeMsgRestartRequired)
	case restartRecommended:
		return runtimeMsg(locale, runtimeMsgRestartRecommended)
	default:
		return string(h)
	}
}

func localeFromInteraction(i *discordgo.InteractionCreate) discordgo.Locale {
	if i == nil || i.Interaction == nil {
		return discordgo.EnglishUS
	}
	if i.GuildLocale != nil {
		return *i.GuildLocale
	}
	if i.Locale != "" {
		return i.Locale
	}
	return discordgo.EnglishUS
}
