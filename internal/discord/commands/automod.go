package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

// --- Handlers ---

// Handler /automod dispatcher (refatorado para usar estrutura modular)
func (ch *CommandHandler) handleAutomodCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod", getUserID(i))

	if i.GuildID == "" {
		logEntry.Warn("Command used outside of a guild")
		ch.respondWithError(s, i, "Use this command inside a server.")
		return
	}

	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		logEntry.Warn("No subcommand provided")
		ch.respondWithError(s, i, "No subcommand specified.")
		return
	}

	sub := options[0]

	// Mapeamento de subcomando para handler
	subcommandHandlers := map[string]func(*discordgo.Session, *discordgo.InteractionCreate){
		"lists":              ch.handleAutomodLists,
		"listcreate":         ch.handleAutomodListCreate,
		"listdelete":         ch.handleAutomodListDelete,
		"listrename":         ch.handleAutomodListRename,
		"rulesets":           ch.handleAutomodRulesets,
		"rulesetcreate":      ch.handleAutomodRulesetCreate,
		"rulesetdelete":      ch.handleAutomodRulesetDelete,
		"rulesetrename":      ch.handleAutomodRulesetRename,
		"rulesettoggle":      ch.handleAutomodRulesetToggle,
		"nativeruleregister": ch.handleAutomodNativeRuleRegister,
	}

	if handler, ok := subcommandHandlers[sub.Name]; ok {
		handler(s, i)
	} else {
		ch.respondWithError(s, i, "Unknown subcommand.")
	}
}

// Handler para registrar regras nativas do Discord

// handleAutomodNativeRuleRegister registra uma regra nativa do Discord no config local
func (ch *CommandHandler) handleAutomodNativeRuleRegister(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod nativeruleregister", getUserID(i))

	// Extrai o ID da regra do input (obrigatório pelo Discord)
	ruleID := extractOptionString(i, "rule")

	// Busca regras nativas do servidor via API do Discord
	nativeRules, err := s.AutoModerationRules(i.GuildID)
	if err != nil {
		logEntry.WithField("error", err).Error("Failed to fetch native rules from Discord API")
		ch.respondWithError(s, i, "Failed to fetch native rules from Discord.")
		return
	}

	// Log de depuração: mostrar todos os IDs disponíveis
	logNativeRuleDebug(ruleID, nativeRules)

	// Procura a regra pelo ID
	foundRule := findNativeRuleByID(nativeRules, ruleID)
	if foundRule == nil {
		ch.respondWithError(s, i, "Native rule ID not found in Discord.")
		return
	}

	// Busca config do servidor
	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	// Adiciona a regra nativa ao config
	addNativeRuleToConfig(guildConfig, foundRule)

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		logEntry.Error("Failed to persist guild configuration")
		ch.respondWithError(s, i, "An error occurred while saving the configuration. Please try again later.")
		return
	}

	ch.respondWithPersistedConfig(s, i, "Native rule registered: "+foundRule.Name)
}

// Handler for /automod rulecreate subcommand

// handleAutomodListCreate cria uma nova lista customizada (keyword, website, serverlink)
func (ch *CommandHandler) handleAutomodListCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod listcreate", getUserID(i))

	ruleType := strings.ToLower(extractOptionString(i, "type"))
	ruleValue := extractOptionString(i, "list")
	if ruleValue == "" {
		ruleValue = extractOptionString(i, "value")
	}
	mode := strings.ToLower(extractOptionString(i, "mode"))

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	// Processa o argumento `list` para separar valores por vírgulas e remover espaços extras
	words := strings.Split(ruleValue, ",")
	for i := range words {
		words[i] = strings.TrimSpace(words[i])
	}

	// Atualiza o valor de `ruleValue` com a lista processada
	ruleValue = strings.Join(words, ",")

	// Cria a regra customizada com a lista processada
	newRule, err := buildCustomRule(ruleType, ruleValue, mode)
	if err != nil {
		ch.respondWithError(s, i, err.Error())
		return
	}

	// Adiciona ao ruleset ou como loose rule
	message, ok := addListToConfig(guildConfig, newRule)
	if !ok {
		ch.respondWithError(s, i, message)
		return
	}

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		logEntry.Error("Failed to persist guild configuration")
		ch.respondWithError(s, i, "An error occurred while saving the configuration. Please try again later.")
		return
	}

	ch.respondWithPersistedConfig(s, i, message)
}

func (ch *CommandHandler) handleAutomodCustomRuleCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod customrulecreate", getUserID(i))

	ruleType := strings.ToLower(extractOptionString(i, "type"))
	ruleValue := extractOptionString(i, "value")
	mode := strings.ToLower(extractOptionString(i, "mode"))

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	// Create custom rule
	newRule, err := buildCustomRule(ruleType, ruleValue, mode)
	if err != nil {
		ch.respondWithError(s, i, err.Error())
		return
	}

	// Add to loose lists
	message, ok := addListToConfig(guildConfig, newRule)
	if !ok {
		ch.respondWithError(s, i, message)
		return
	}

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		logEntry.Error("Failed to persist guild configuration")
		ch.respondWithError(s, i, "An error occurred while saving the configuration. Please try again later.")
		return
	}

	ch.respondWithPersistedConfig(s, i, message)
}

// Handler for /automod ruledelete subcommand

// handleAutomodListDelete remove uma lista de um ruleset ou das loose lists
func (ch *CommandHandler) handleAutomodListDelete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod listdelete", getUserID(i))

	// Parse argumentos (list pode vir como "rulesetID:listID" ou só "listID")
	rulesetID, listID := parseListDeleteArgs(i)

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	// Remove a regra
	message, ok := removeListFromConfig(guildConfig, rulesetID, listID)
	if !ok {
		ch.respondWithError(s, i, message)
		return
	}

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		return
	}

	ch.respondWithPersistedConfig(s, i, message)
}

// Handler for /automod rulesets subcommand

// handleAutomodRulesets responde com a lista de rulesets do servidor
func (ch *CommandHandler) handleAutomodRulesets(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod rulesets", getUserID(i))

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	msg := buildRulesetsListMessage(guildConfig)

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		return
	}

	ch.respondWithPersistedConfig(s, i, msg)
}

// Handler for /automod rulesetcreate subcommand

// handleAutomodRulesetCreate cria um novo ruleset para o servidor
func (ch *CommandHandler) handleAutomodRulesetCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod rulesetcreate", getUserID(i))

	rulesetName := extractOptionString(i, "name")

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	newRuleset := buildNewRuleset(rulesetName)
	guildConfig.Rulesets = append(guildConfig.Rulesets, newRuleset)

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		logEntry.Error("Failed to persist guild configuration")
		ch.respondWithError(s, i, "An error occurred while saving the configuration. Please try again later.")
		return
	}

	ch.respondWithPersistedConfig(s, i, fmt.Sprintf("Ruleset '%s' created with ID: %s", rulesetName, newRuleset.ID))
}

// handleAutomodRulesetDelete remove um ruleset do servidor
func (ch *CommandHandler) handleAutomodRulesetDelete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod rulesetdelete", getUserID(i))

	rulesetID := extractOptionString(i, "ruleset")

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	message, ok := removeRulesetFromConfig(guildConfig, rulesetID)
	if !ok {
		ch.respondWithError(s, i, message)
		return
	}

	if err := ch.configManager.AddGuildConfig(*guildConfig); err != nil {
		logEntry.WithField("error", err).Error("Failed to update guild config in memory")
		ch.respondWithError(s, i, "Failed to update guild config in memory.")
		return
	}
	if err := ch.configManager.SaveConfig(); err != nil {
		logEntry.WithField("error", err).Error("Failed to persist config to file")
		ch.respondWithError(s, i, "Failed to persist config to file.")
		return
	}

	ch.respondWithPersistedConfig(s, i, message)
}

// Handler for /automod rulesetrename subcommand

// handleAutomodRulesetRename renomeia um ruleset
func (ch *CommandHandler) handleAutomodRulesetRename(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod rulesetrename", getUserID(i))

	rulesetID := extractOptionString(i, "ruleset")
	newName := extractOptionString(i, "name")

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	message, ok := renameRulesetInConfig(guildConfig, rulesetID, newName)
	if !ok {
		ch.respondWithError(s, i, message)
		return
	}

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		return
	}

	ch.respondWithPersistedConfig(s, i, message)
}

// renameRulesetInConfig renomeia o ruleset e retorna mensagem
func renameRulesetInConfig(guildConfig *files.GuildConfig, rulesetID, newName string) (string, bool) {
	ruleset := findRulesetByID(guildConfig.Rulesets, rulesetID)
	if ruleset == nil {
		return "Ruleset not found.", false
	}
	oldName := ruleset.Name
	ruleset.Name = newName
	return fmt.Sprintf("Ruleset '%s' renamed to '%s'!", oldName, newName), true
}

// Handler for /automod rulesettoggle subcommand

// handleAutomodRulesetToggle ativa/desativa um ruleset
func (ch *CommandHandler) handleAutomodRulesetToggle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod rulesettoggle", getUserID(i))

	if i.GuildID == "" {
		logEntry.Warn("Command used outside of a guild")
		ch.respondWithError(s, i, "Use this command inside a server.")
		return
	}

	rulesetID := extractOptionString(i, "ruleset")

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	message, ok := toggleRulesetInConfig(guildConfig, rulesetID)
	if !ok {
		ch.respondWithError(s, i, message)
		return
	}

	if err := ch.configManager.AddGuildConfig(*guildConfig); err != nil {
		logEntry.WithField("error", err).Error("Failed to update guild config in memory")
		ch.respondWithError(s, i, "Failed to update guild config in memory.")
		return
	}
	if err := ch.configManager.SaveConfig(); err != nil {
		logEntry.WithField("error", err).Error("Failed to persist config to file")
		ch.respondWithError(s, i, "Failed to persist config to file.")
		return
	}

	ch.respondWithPersistedConfig(s, i, message)
}

// --- Helpers ---

// addListToConfig adiciona a lista ao config do guild
func addListToConfig(guildConfig *files.GuildConfig, rule files.Rule) (string, bool) {
	guildConfig.LooseLists = []files.Rule{}
	guildConfig.LooseLists = append(guildConfig.LooseLists, rule)
	return fmt.Sprintf("Loose rule created: %s", rule.ID), true
}

// addNativeRuleToConfig adiciona uma regra nativa ao config local
func addNativeRuleToConfig(guildConfig *files.GuildConfig, rule *discordgo.AutoModerationRule) {
	newList := files.List{
		ID:          "native-list-" + rule.ID,
		Type:        "native",
		NativeID:    rule.ID,
		Name:        rule.Name,
		Description: "Native rule from Discord treated as a list",
	}

	newRule := files.Rule{
		ID:      "native-rule-" + rule.ID,
		Name:    rule.Name,
		Lists:   []files.List{newList},
		Enabled: true,
	}

	if guildConfig.LooseLists == nil {
		guildConfig.LooseLists = []files.Rule{}
	}
	guildConfig.LooseLists = append(guildConfig.LooseLists, newRule)
}

// buildCustomRule cria uma files.Rule a partir dos argumentos
func buildCustomRule(ruleType, ruleValue, mode string) (files.Rule, error) {
	ruleID := fmt.Sprintf("rule-%d", time.Now().UnixNano())
	newList := files.List{
		ID:          fmt.Sprintf("list-%d", time.Now().UnixNano()),
		Type:        ruleType,
		Name:        ruleValue,
		Description: fmt.Sprintf("Custom %s list", mode),
	}
	rule := files.Rule{
		ID:      ruleID,
		Name:    "Custom Rule",
		Lists:   []files.List{newList},
		Enabled: true,
	}
	return rule, nil
}

// buildNewRuleset cria um novo ruleset com nome e ID únicos
func buildNewRuleset(name string) files.Ruleset {
	id := fmt.Sprintf("rs-%d-%d", time.Now().UnixNano(), (1000 + len(name)))
	return files.Ruleset{
		ID:      id,
		Name:    name,
		Rules:   []files.Rule{},
		Enabled: true,
	}
}

// Helper function to centralize log entry creation for commands
func (ch *CommandHandler) commandLogEntry(i *discordgo.InteractionCreate, command string, userID string) *logutil.Logger {
	return logutil.WithFields(map[string]interface{}{
		"command": command,
		"guildID": i.GuildID,
		"userID":  userID,
	})
}

// extractOptionString busca o valor string de uma option pelo nome
func extractOptionString(i *discordgo.InteractionCreate, name string) string {
	for _, opt := range i.ApplicationCommandData().Options[0].Options {
		if opt.Name == name {
			return strings.TrimSpace(opt.StringValue())
		}
	}
	return ""
}

// findNativeRuleByID retorna a regra nativa pelo ID
func findNativeRuleByID(rules []*discordgo.AutoModerationRule, id string) *discordgo.AutoModerationRule {
	for _, r := range rules {
		if strings.TrimSpace(r.ID) == id {
			return r
		}
	}
	return nil
}

// Handler for /automod rules subcommand
func (ch *CommandHandler) handleAutomodLists(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Fetch rules from config
	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	var b strings.Builder
	b.WriteString(buildListsListMessage(guildConfig))

	// Respond with the updated configuration with a message
	ch.respondWithPersistedConfig(s, i, b.String())
}

// logNativeRuleDebug faz log dos IDs disponíveis e do input
func logNativeRuleDebug(inputID string, rules []*discordgo.AutoModerationRule) {
	var idsList []string
	for _, r := range rules {
		idsList = append(idsList, r.ID+":"+r.Name)
	}
	logutil.WithFields(map[string]interface{}{
		"input_rule_id": inputID,
		"available_ids": idsList,
	}).Debug("Debug nativeruleregister: comparando IDs")
}

// parseListDeleteArgs extrai rulesetID e listID do input
func parseListDeleteArgs(i *discordgo.InteractionCreate) (string, string) {
	var rulesetID, listID string
	for _, opt := range i.ApplicationCommandData().Options[0].Options {
		if opt.Name == "ruleset" {
			rulesetID = strings.TrimSpace(opt.StringValue())
		}
		if opt.Name == "list" {
			val := strings.TrimSpace(opt.StringValue())
			if idx := strings.Index(val, ":"); idx != -1 {
				rulesetID = val[:idx]
				listID = val[idx+1:]
			} else {
				listID = val
			}
		}
	}
	return rulesetID, listID
}

// removeListFromConfig remove a lista do ruleset ou das loose lists
func removeListFromConfig(guildConfig *files.GuildConfig, ruleID, listID string) (string, bool) {
	// Procurar a regra correspondente
	for _, rule := range guildConfig.LooseLists {
		if rule.ID == ruleID {
			// Procurar a lista dentro da regra
			for listIdx, list := range rule.Lists {
				if list.ID == listID {
					rule.Lists = append(rule.Lists[:listIdx], rule.Lists[listIdx+1:]...)
					return "List deleted from rule!", true
				}
			}
			return "List not found in rule.", false
		}
	}
	return "Rule not found.", false
}

// removeRulesetFromConfig remove o ruleset do config e retorna mensagem
func removeRulesetFromConfig(guildConfig *files.GuildConfig, rulesetID string) (string, bool) {
	ruleset, idx := guildConfig.FindRulesetByID(rulesetID)
	if ruleset == nil {
		return "Ruleset not found.", false
	}
	guildConfig.Rulesets = append(guildConfig.Rulesets[:idx], guildConfig.Rulesets[idx+1:]...)
	return fmt.Sprintf("Ruleset '%s' deleted!", ruleset.Name), true
}

// toggleRulesetInConfig ativa/desativa o ruleset e retorna mensagem
func toggleRulesetInConfig(guildConfig *files.GuildConfig, rulesetID string) (string, bool) {
	ruleset := findRulesetByID(guildConfig.Rulesets, rulesetID)
	if ruleset == nil {
		return "Ruleset not found.", false
	}
	ruleset.Enabled = !ruleset.Enabled
	return fmt.Sprintf("Ruleset '%s' is now %s!", ruleset.Name, ruleset.StatusString()), true
}

// Handler for /automod listrename subcommand
func (ch *CommandHandler) handleAutomodListRename(s *discordgo.Session, i *discordgo.InteractionCreate) {
	logEntry := ch.commandLogEntry(i, "automod listrename", getUserID(i))

	oldName := extractOptionString(i, "old_name")
	newName := extractOptionString(i, "new_name")

	if oldName == "" || newName == "" {
		ch.respondWithError(s, i, "Both old_name and new_name must be provided.")
		return
	}

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil {
		ch.respondWithError(s, i, "Guild config not found.")
		return
	}

	// Find the list by oldName
	list := guildConfig.FindListByName(oldName)
	if list == nil {
		ch.respondWithError(s, i, "List not found.")
		return
	}

	// Check if newName already exists
	if guildConfig.ListExists(newName) {
		ch.respondWithError(s, i, "A list with the new name already exists.")
		return
	}

	// Rename the list
	list.Name = newName

	if !ch.persistGuildConfig(guildConfig, s, i, logEntry) {
		logEntry.Error("Failed to persist guild configuration")
		ch.respondWithError(s, i, "An error occurred while saving the configuration. Please try again later.")
		return
	}

	ch.respondWithPersistedConfig(s, i, fmt.Sprintf("List renamed from %s to %s", oldName, newName))
}
