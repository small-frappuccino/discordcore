package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alice-bnuy/discordcore/v2/internal/discord/logging"
	"github.com/alice-bnuy/discordcore/v2/internal/files"

	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

// ======================
// Initializers
// ======================

type CommandHandler struct {
	session            *discordgo.Session
	configManager      *files.ConfigManager
	avatarCacheManager *files.AvatarCacheManager
	monitorService     *logging.MonitoringService
	automodService     *logging.AutomodService
	notifier           *logging.NotificationSender
	commands           []SlashCommand
}

func NewCommandHandler(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	avatarCacheManager *files.AvatarCacheManager,
	monitorService *logging.MonitoringService,
	automodService *logging.AutomodService) *CommandHandler {
	ch := &CommandHandler{
		session:            session,
		configManager:      configManager,
		avatarCacheManager: avatarCacheManager,
		monitorService:     monitorService,
		automodService:     automodService,
		notifier:           logging.NewNotificationSender(session),
	}

	ch.initializeCommands()
	return ch
}

func (ch *CommandHandler) initializeCommands() {
	ch.commands = []SlashCommand{
		ch.buildAutomodCommand(),
		ch.buildNativeRuleRegisterCommand(),
		ch.buildCustomRuleCreateCommand(),
		/* ch.buildCleanCommand(),
		ch.buildTimeoutCommand(),
		ch.buildKickCommand(),
		ch.buildBanCommand(),
		ch.buildUnbanCommand(),
		ch.buildCreateCommand(),
		ch.buildDeleteCommand(), */
	}
}

// Setup and registration of slash commands (application commands)
func (ch *CommandHandler) SetupCommands() error {
	// Remove old message handlers if they exist
	ch.session.AddHandler(ch.handleInteractionCreate)

	// Obter comandos já registrados (globais)
	registered, err := ch.session.ApplicationCommands(ch.session.State.User.ID, "")
	if err != nil {
		return fmt.Errorf("failed to fetch registered commands: %w", err)
	}
	regByName := make(map[string]*discordgo.ApplicationCommand, len(registered))
	for _, rc := range registered {
		regByName[rc.Name] = rc
	}

	// Mapear comandos do código por nome
	codeByName := make(map[string]SlashCommand, len(ch.commands))
	for _, c := range ch.commands {
		codeByName[c.Name] = c
	}

	// Criar/Atualizar apenas quando houver diferença
	created, updated, unchanged := 0, 0, 0
	for _, c := range ch.commands {
		desired := &discordgo.ApplicationCommand{
			Name:        c.Name,
			Description: c.Description,
			Options:     c.Options,
		}
		if existing, ok := regByName[c.Name]; ok {
			if commandsSemanticallyEqual(existing, desired) {
				logutil.WithField("command", c.Name).Debug("Slash command unchanged; skipping")
				unchanged++
				continue
			}
			if _, err := ch.session.ApplicationCommandEdit(ch.session.State.User.ID, "", existing.ID, desired); err != nil {
				return fmt.Errorf("error editing command '%s': %w", c.Name, err)
			}
			logutil.WithField("command", c.Name).Info("Slash command updated")
			updated++
			continue
		}
		if _, err := ch.session.ApplicationCommandCreate(ch.session.State.User.ID, "", desired); err != nil {
			return fmt.Errorf("error creating command '%s': %w", c.Name, err)
		}
		logutil.WithField("command", c.Name).Info("Slash command created")
		created++
	}

	// Remover órfãos (existem na API, não no código)
	deleted := 0
	for _, rc := range registered {
		if _, exists := codeByName[rc.Name]; !exists {
			if err := ch.session.ApplicationCommandDelete(ch.session.State.User.ID, "", rc.ID); err != nil {
				logutil.WithFields(map[string]interface{}{"command": rc.Name, "error": err}).Warn("Error removing orphan command")
				continue
			}
			logutil.WithField("command", rc.Name).Info("Orphan slash command removed")
			deleted++
		}
	}

	// Log resumo
	logutil.WithFields(map[string]interface{}{
		"created":   created,
		"updated":   updated,
		"deleted":   deleted,
		"unchanged": unchanged,
		"total":     len(ch.commands),
		"mode":      "incremental",
	}).Info("Slash commands sync summary")

	return nil
}

// Main handler for interactions
func (ch *CommandHandler) handleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Handle autocomplete interactions for /automod ruleadd ruleset and value
	if i.Type == discordgo.InteractionApplicationCommandAutocomplete {
		if i.ApplicationCommandData().Name == "automod" {
			opts := i.ApplicationCommandData().Options
			if len(opts) > 0 {
				for _, opt := range opts[0].Options {
					if opt.Focused {
						switch opt.Name {
						case "ruleset":
							ch.handleRulesetAutocomplete(s, i)
							return
						case "rule":
							switch opts[0].Name {
							case "rulecreate":
								var ruleType string
								for _, o := range opts[0].Options {
									if o.Name == "type" {
										ruleType = strings.ToLower(o.StringValue())
									}
								}
								if ruleType == "native" {
									ch.handleNativeRuleAutocomplete(s, i)
								} else {
									ch.handleRuleTypeAutocomplete(s, i)
								}
							case "ruleadd":
								ch.handleRuleTypeAutocomplete(s, i)
							default:
								ch.handleRuleAutocomplete(s, i)
							}
							return
						}
					}
				}
			}
		}
	}

	// Ignore non-slash interactions (e.g., component clicks go to the wizard handler)
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name == "" {
		return
	}

	userID := ""
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
	} else if i.User != nil {
		userID = i.User.ID
	}

	logEntry := logutil.WithFields(map[string]interface{}{
		"command": i.ApplicationCommandData().Name,
		"userID":  userID,
		"guildID": i.GuildID,
	})

	logEntry.Debug("Processing slash command")

	if !ch.hasPermission(i.GuildID, userID) {
		logEntry.Warn("User without permission tried to use command")
		ch.respondWithError(s, i, "You do not have permission to use this command.")
		return
	}

	for _, cmd := range ch.commands {
		if cmd.Name == i.ApplicationCommandData().Name {
			logEntry.Info("Executing command")
			cmd.Handler(s, i)
			return
		}
	}

	logEntry.Error("Command not found")
	ch.respondWithError(s, i, "Command not found.")
}

// ======================
// Autocomplete Handlers
// ======================

// Autocomplete handler for the 'ruleset' field in /automod ruleadd
func (ch *CommandHandler) handleRulesetAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Autocomplete handler for the 'rule' field em comandos como ruledelete: lista todas as regras cadastradas
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig == nil || len(guildConfig.Rulesets) == 0 {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	for _, rs := range guildConfig.Rulesets {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  rs.Name,
			Value: rs.ID,
		})
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// Autocomplete handler for the 'rule' field in /automod ruleadd (keyword, native, website, serverlink)
func (ch *CommandHandler) handleRuleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	// Get current input for filtering
	var input string
	opts := i.ApplicationCommandData().Options
	if len(opts) > 0 && opts[0].Name == "ruleadd" {
		for _, opt := range opts[0].Options {
			if opt.Name == "rule" && opt.Focused {
				input = strings.ToLower(opt.StringValue())
			}
		}
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	guildConfig := ch.configManager.GuildConfig(i.GuildID)
	if guildConfig != nil {
		for _, rs := range guildConfig.Rulesets {
			for _, rule := range rs.Rules {
				if input == "" ||
					strings.Contains(strings.ToLower(rule.ID), input) {
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  rule.Name,
						Value: rs.ID + ":" + rule.ID,
					})
				}
			}
		}
		for _, rule := range guildConfig.LooseLists {
			if input == "" ||
				strings.Contains(strings.ToLower(rule.ID), input) {
				choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
					Name:  rule.ID,
					Value: rule.ID,
				})
			}
		}
	}
	// Não sugere tipos, apenas regras cadastradas
	if len(choices) > 25 {
		choices = choices[:25]
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// Autocomplete handler para tipos de regra em /automod ruleadd
func (ch *CommandHandler) handleRuleTypeAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	var input string
	opts := i.ApplicationCommandData().Options
	if len(opts) > 0 {
		for _, opt := range opts[0].Options {
			if opt.Name == "type" && opt.Focused {
				input = strings.ToLower(opt.StringValue())
			}
		}
	}

	types := []string{"keyword", "native", "website", "serverlink"}
	var choices []*discordgo.ApplicationCommandOptionChoice
	for _, t := range types {
		if input == "" || strings.Contains(t, input) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  t,
				Value: t,
			})
		}
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// Autocomplete handler para o campo 'rule' em rulecreate do tipo native: sugere regras nativas existentes do Discord
func (ch *CommandHandler) handleNativeRuleAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.GuildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	var input string
	opts := i.ApplicationCommandData().Options
	if len(opts) > 0 {
		for _, opt := range opts[0].Options {
			if opt.Name == "rule" && opt.Focused {
				input = strings.ToLower(opt.StringValue())
			}
		}
	}

	rules, err := s.AutoModerationRules(i.GuildID)
	if err != nil {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionApplicationCommandAutocompleteResult,
			Data: &discordgo.InteractionResponseData{Choices: []*discordgo.ApplicationCommandOptionChoice{}},
		})
		return
	}

	// Log de depuração: mostrar input e regras retornadas
	var idsList []string
	for _, r := range rules {
		idsList = append(idsList, r.ID+":"+r.Name)
		fmt.Println(r.ID + ":" + r.Name)
	}
	logutil.WithFields(map[string]interface{}{
		"input":         input,
		"available_ids": idsList,
	}).Debug("Debug autocomplete nativerule: input e regras retornadas")

	var choices []*discordgo.ApplicationCommandOptionChoice
	for _, r := range rules {
		if input == "" || strings.Contains(strings.ToLower(r.Name), input) || strings.Contains(strings.ToLower(r.ID), input) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  r.Name + " (" + r.ID + ")",
				Value: r.ID,
			})
		}
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

// ======================
// Utility/Helper Methods
// ======================

// Função utilitária para listar rulesets de forma reutilizável
func buildListsListMessage(guildConfig *files.GuildConfig) string {
	var b strings.Builder
	if guildConfig == nil || len(guildConfig.LooseLists) == 0 {
		b.WriteString("There are no lists configured in this server.")
	} else {
		b.WriteString("Lists on this server:\n```")
		for _, rule := range guildConfig.LooseLists {
			b.WriteString(fmt.Sprintf("%s\n", rule.ID))
		}
		b.WriteString("```")
	}
	return b.String()
}

// Função utilitária para listar rulesets ou regras soltas de forma reutilizável
func buildRulesetsListMessage(guildConfig *files.GuildConfig) string {
	var b strings.Builder
	if guildConfig == nil || len(guildConfig.Rulesets) == 0 {
		b.WriteString("There are no rulesets configured in this server.")
	} else {
		b.WriteString("Rulesets on this server:\n```")
		for _, ruleset := range guildConfig.Rulesets {
			b.WriteString(fmt.Sprintf("%s: %s\n", ruleset.Name, ruleset.StatusString()))
		}
		b.WriteString("```")
	}
	return b.String()
}

// Respond with a success message and persist the guild config
func (ch *CommandHandler) respondWithPersistedConfig(
	s *discordgo.Session,
	i *discordgo.InteractionCreate,
	content string,
) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	})
}

// Extract user ID from interaction
func getUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	} else if i.User != nil {
		return i.User.ID
	}
	return ""
}

// Find ruleset by ID, returns pointer or nil
func findRulesetByID(rulesets []files.Ruleset, id string) *files.Ruleset {
	for idx, rs := range rulesets {
		if rs.ID == id {
			return &rulesets[idx]
		}
	}
	return nil
}

// Persist guild config, returns true if success, false if already responded with error
func (ch *CommandHandler) persistGuildConfig(guildConfig *files.GuildConfig, s *discordgo.Session, i *discordgo.InteractionCreate, logEntry interface{}) bool {
	if err := ch.configManager.AddGuildConfig(*guildConfig); err != nil {
		if l, ok := logEntry.(interface {
			WithField(string, interface{}) interface{ Error(args ...interface{}) }
		}); ok {
			l.WithField("error", err).Error("Failed to update guild config in memory")
		}
		ch.respondWithError(s, i, "Failed to update guild config in memory.")
		return false
	}
	if err := ch.configManager.SaveConfig(); err != nil {
		if l, ok := logEntry.(interface {
			WithField(string, interface{}) interface{ Error(args ...interface{}) }
		}); ok {
			l.WithField("error", err).Error("Failed to persist config to file")
		}
		ch.respondWithError(s, i, "Failed to persist config to file.")
		return false
	}
	return true
}

// commandsSemanticallyEqual compara Nome, Descrição e Opções recursivamente
func commandsSemanticallyEqual(a, b *discordgo.ApplicationCommand) bool {
	ca := struct {
		Name        string                                `json:"name"`
		Description string                                `json:"description"`
		Options     []*discordgo.ApplicationCommandOption `json:"options"`
	}{a.Name, a.Description, a.Options}
	cb := struct {
		Name        string                                `json:"name"`
		Description string                                `json:"description"`
		Options     []*discordgo.ApplicationCommandOption `json:"options"`
	}{b.Name, b.Description, b.Options}
	ba, _ := json.Marshal(ca)
	bb, _ := json.Marshal(cb)
	return string(ba) == string(bb)
}

// Helper to parse comma/space separated values
func parseListValue(val string) []string {
	var out []string
	for _, tok := range strings.Fields(strings.ReplaceAll(val, ",", " ")) {
		w := strings.TrimSpace(tok)
		if w != "" {
			out = append(out, w)
		}
	}
	return out
}

func (ch *CommandHandler) hasPermission(guildID, userID string) bool {
	guildConfig := ch.configManager.GuildConfig(guildID)

	// Fetch guild to resolve owner ID reliably
	guild, err := ch.session.Guild(guildID)
	if err != nil {
		return false
	}
	isOwner := guild.OwnerID == userID

	// Sem configuração ou sem roles permitidas: apenas o dono do servidor pode usar
	if guildConfig == nil || len(guildConfig.AllowedRoles) == 0 {
		return isOwner
	}

	// Com roles configuradas: dono sempre pode; além disso, qualquer usuário com uma das roles
	if isOwner {
		return true
	}

	member, err := ch.session.GuildMember(guildID, userID)
	if err != nil {
		return false
	}
	for _, userRole := range member.Roles {
		for _, allowedRole := range guildConfig.AllowedRoles {
			if userRole == allowedRole {
				return true
			}
		}
	}
	return false
}

// Método utilitário para enviar resposta de erro
func (ch *CommandHandler) respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	ch.sendEphemeral(s, i, "❌ "+message)
}

// Método utilitário para enviar resposta de sucesso
func (ch *CommandHandler) respondWithSuccess(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	ch.sendEphemeral(s, i, "✅ "+message)
}

// Método utilitário para enviar resposta ephemeral
func (ch *CommandHandler) sendEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) error {
	return ch.respondWithMessage(s, i, content, true)
}

// Método utilitário para responder com mensagem normal ou ephemeral
func (ch *CommandHandler) respondWithMessage(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) error {
	var flags discordgo.MessageFlags
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	return s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   flags,
		},
	})
}

// Placeholder for building the native rule register command
func (ch *CommandHandler) buildNativeRuleRegisterCommand() SlashCommand {
	return SlashCommand{
		Name:        "nativeruleregister",
		Description: "Register a native rule as a list",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "rule",
				Description: "The ID of the native rule to register",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
		},
		Handler: ch.handleAutomodNativeRuleRegister,
	}
}

// Placeholder for building the custom rule create command
func (ch *CommandHandler) buildCustomRuleCreateCommand() SlashCommand {
	return SlashCommand{
		Name:        "customrulecreate",
		Description: "Create a custom rule with a list",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Name:        "type",
				Description: "The type of the list (keyword, website, serverlink)",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
			{
				Name:        "value",
				Description: "The value of the list",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
			{
				Name:        "mode",
				Description: "The mode of the list (allowlist or denylist)",
				Type:        discordgo.ApplicationCommandOptionString,
				Required:    true,
			},
		},
		Handler: ch.handleAutomodCustomRuleCreate,
	}
}
