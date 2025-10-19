package core

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// OptionExtractor simplifica extração de opções de comandos Discord
type OptionExtractor struct {
	options []*discordgo.ApplicationCommandInteractionDataOption
}

// NewOptionExtractor cria um novo extrator de opções
func NewOptionExtractor(options []*discordgo.ApplicationCommandInteractionDataOption) *OptionExtractor {
	return &OptionExtractor{options: options}
}

// String extrai uma opção string pelo nome
func (e *OptionExtractor) String(name string) string {
	for _, opt := range e.options {
		if opt.Name == name {
			return strings.TrimSpace(opt.StringValue())
		}
	}
	return ""
}

// StringRequired extrai uma opção string obrigatória
func (e *OptionExtractor) StringRequired(name string) (string, error) {
	value := e.String(name)
	if value == "" {
		return "", NewValidationError(name, fmt.Sprintf("Option '%s' is required", name))
	}
	return value, nil
}

// Bool extrai uma opção booleana pelo nome
func (e *OptionExtractor) Bool(name string) bool {
	for _, opt := range e.options {
		if opt.Name == name {
			return opt.BoolValue()
		}
	}
	return false
}

// Int extrai uma opção inteira pelo nome
func (e *OptionExtractor) Int(name string) int64 {
	for _, opt := range e.options {
		if opt.Name == name {
			return opt.IntValue()
		}
	}
	return 0
}

// Float extrai uma opção float pelo nome
func (e *OptionExtractor) Float(name string) float64 {
	for _, opt := range e.options {
		if opt.Name == name {
			return opt.FloatValue()
		}
	}
	return 0
}

// HasOption verifica se uma opção existe
func (e *OptionExtractor) HasOption(name string) bool {
	for _, opt := range e.options {
		if opt.Name == name {
			return true
		}
	}
	return false
}

// GetAllOptions retorna todas as opções como map
func (e *OptionExtractor) GetAllOptions() map[string]interface{} {
	result := make(map[string]interface{})
	for _, opt := range e.options {
		switch opt.Type {
		case discordgo.ApplicationCommandOptionString:
			result[opt.Name] = opt.StringValue()
		case discordgo.ApplicationCommandOptionInteger:
			result[opt.Name] = opt.IntValue()
		case discordgo.ApplicationCommandOptionBoolean:
			result[opt.Name] = opt.BoolValue()
		case discordgo.ApplicationCommandOptionNumber:
			result[opt.Name] = opt.FloatValue()
		}
	}
	return result
}

// ConfigPersister gerencia persistência de configuração
type ConfigPersister struct {
	configManager *files.ConfigManager
}

// NewConfigPersister cria um novo persistidor de configuração
func NewConfigPersister(cm *files.ConfigManager) *ConfigPersister {
	return &ConfigPersister{configManager: cm}
}

// Save salva a configuração do servidor
func (cp *ConfigPersister) Save(config *files.GuildConfig) error {
	if err := cp.configManager.AddGuildConfig(*config); err != nil {
		return fmt.Errorf("failed to update config in memory: %w", err)
	}
	if err := cp.configManager.SaveConfig(); err != nil {
		return fmt.Errorf("failed to persist config: %w", err)
	}
	return nil
}

// SaveWithBackup salva a configuração com backup
func (cp *ConfigPersister) SaveWithBackup(config *files.GuildConfig) error {
	// Implementar backup se necessário
	return cp.Save(config)
}

// PermissionChecker gerencia verificação de permissões
// PermissionChecker verifica permissões do usuário
type PermissionChecker struct {
	session *discordgo.Session
	config  *files.ConfigManager
	store   *storage.Store
	cache   *cache.UnifiedCache
}

func NewPermissionChecker(session *discordgo.Session, config *files.ConfigManager) *PermissionChecker {
	return &PermissionChecker{session: session, config: config}
}

func (pc *PermissionChecker) SetStore(store *storage.Store) {
	pc.store = store
}

func (pc *PermissionChecker) SetCache(unifiedCache *cache.UnifiedCache) {
	pc.cache = unifiedCache
}

// HasPermission verifica se o usuário tem permissão para usar comandos
func (pc *PermissionChecker) HasPermission(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	guildConfig := pc.config.GuildConfig(guildID)

	// Try unified cache first
	var ownerID string
	if pc.cache != nil {
		if guild, ok := pc.cache.GetGuild(guildID); ok {
			ownerID = guild.OwnerID
		}
	}
	// Fallback: state cache
	if ownerID == "" && pc.session != nil && pc.session.State != nil {
		if g, _ := pc.session.State.Guild(guildID); g != nil {
			ownerID = g.OwnerID
			if pc.cache != nil {
				pc.cache.SetGuild(guildID, g)
			}
		}
	}
	// Fallback: store cache
	if ownerID == "" && pc.store != nil {
		if oid, ok, _ := pc.store.GetGuildOwnerID(guildID); ok {
			ownerID = oid
		}
	}
	// Fallback: REST and then persist in caches
	if ownerID == "" {
		guild, err := pc.session.Guild(guildID)
		if err != nil {
			return false
		}
		ownerID = guild.OwnerID
		if pc.cache != nil {
			pc.cache.SetGuild(guildID, guild)
		}
		if pc.store != nil && ownerID != "" {
			_ = pc.store.SetGuildOwnerID(guildID, ownerID)
		}
	}
	isOwner := ownerID == userID

	if guildConfig == nil || len(guildConfig.AllowedRoles) == 0 {
		return isOwner
	}

	if isOwner {
		return true
	}

	var member *discordgo.Member
	var err error
	// Try unified cache first
	if pc.cache != nil {
		if m, ok := pc.cache.GetMember(guildID, userID); ok {
			member = m
		}
	}
	// Fallback: state cache
	if member == nil && pc.session != nil && pc.session.State != nil {
		if m, _ := pc.session.State.Member(guildID, userID); m != nil {
			member = m
			if pc.cache != nil {
				pc.cache.SetMember(guildID, userID, m)
			}
		}
	}
	// Fallback: REST
	if member == nil {
		member, err = pc.session.GuildMember(guildID, userID)
		if err != nil {
			return false
		}
		if pc.cache != nil {
			pc.cache.SetMember(guildID, userID, member)
		}
	}

	for _, userRole := range member.Roles {
		if slices.Contains(guildConfig.AllowedRoles, userRole) {
			return true
		}
	}
	return false
}

// HasRole verifica se o usuário tem uma role específica
func (pc *PermissionChecker) HasRole(guildID, userID, roleID string) bool {
	var member *discordgo.Member
	var err error
	// Try unified cache first
	if pc.cache != nil {
		if m, ok := pc.cache.GetMember(guildID, userID); ok {
			member = m
		}
	}
	// Fallback: state cache
	if member == nil && pc.session != nil && pc.session.State != nil {
		if m, _ := pc.session.State.Member(guildID, userID); m != nil {
			member = m
			if pc.cache != nil {
				pc.cache.SetMember(guildID, userID, m)
			}
		}
	}
	// Fallback: REST
	if member == nil {
		member, err = pc.session.GuildMember(guildID, userID)
		if err != nil {
			return false
		}
		if pc.cache != nil {
			pc.cache.SetMember(guildID, userID, member)
		}
	}

	return slices.Contains(member.Roles, roleID)
}

// IsOwner verifica se o usuário é dono do servidor
func (pc *PermissionChecker) IsOwner(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	// Try unified cache first
	var ownerID string
	if pc.cache != nil {
		if guild, ok := pc.cache.GetGuild(guildID); ok {
			ownerID = guild.OwnerID
		}
	}
	// Fallback: state cache
	if ownerID == "" && pc.session != nil && pc.session.State != nil {
		if g, _ := pc.session.State.Guild(guildID); g != nil {
			ownerID = g.OwnerID
			if pc.cache != nil {
				pc.cache.SetGuild(guildID, g)
			}
		}
	}
	// Fallback: store cache
	if ownerID == "" && pc.store != nil {
		if oid, ok, _ := pc.store.GetGuildOwnerID(guildID); ok {
			ownerID = oid
		}
	}
	// Fallback: REST and then persist in caches
	if ownerID == "" {
		guild, err := pc.session.Guild(guildID)
		if err != nil {
			return false
		}
		ownerID = guild.OwnerID
		if pc.cache != nil {
			pc.cache.SetGuild(guildID, guild)
		}
		if pc.store != nil && ownerID != "" {
			_ = pc.store.SetGuildOwnerID(guildID, ownerID)
		}
	}
	return ownerID == userID
}

// StringUtils provides utilities for string manipulation
type StringUtils struct{}

// ProcessCommaSeparatedList parses a comma-separated string
func (StringUtils) ProcessCommaSeparatedList(input string) []string {
	if input == "" {
		return []string{}
	}

	items := strings.Split(input, ",")
	result := make([]string, 0, len(items))

	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// SanitizeInput sanitizes user input
func (StringUtils) SanitizeInput(input string) string {
	// Remove caracteres de controle e espaços extras
	input = strings.TrimSpace(input)
	// Remove quebras de linha múltiplas
	input = strings.ReplaceAll(input, "\n\n", "\n")
	return input
}

// TruncateString truncates a string if it is too long
func (StringUtils) TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ValidateStringLength validates a string length
func (StringUtils) ValidateStringLength(s string, minLen, maxLen int, fieldName string) error {
	if len(s) < minLen {
		return NewValidationError(fieldName, fmt.Sprintf("%s must be at least %d characters", fieldName, minLen))
	}
	if len(s) > maxLen {
		return NewValidationError(fieldName, fmt.Sprintf("%s must be at most %d characters", fieldName, maxLen))
	}
	return nil
}

// AutocompleteUtils provides utilities for autocomplete
type AutocompleteUtils struct{}

// FilterChoices filters choices based on user input
func (AutocompleteUtils) FilterChoices(choices []*discordgo.ApplicationCommandOptionChoice, input string) []*discordgo.ApplicationCommandOptionChoice {
	if input == "" {
		return choices
	}

	input = strings.ToLower(input)
	filtered := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	for _, choice := range choices {
		if strings.Contains(strings.ToLower(choice.Name), input) {
			filtered = append(filtered, choice)
		}
	}

	return filtered
}

// CreateChoice creates a choice for autocomplete
func (AutocompleteUtils) CreateChoice(name, value string) *discordgo.ApplicationCommandOptionChoice {
	return &discordgo.ApplicationCommandOptionChoice{
		Name:  name,
		Value: value,
	}
}

// CreateChoicesFromStrings creates choices from a slice of strings
func (AutocompleteUtils) CreateChoicesFromStrings(items []string) []*discordgo.ApplicationCommandOptionChoice {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(items))
	for i, item := range items {
		choices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  item,
			Value: item,
		}
	}
	return choices
}

// GetStringOption extracts a string option value from command options
func GetStringOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) string {
	for _, option := range options {
		if option.Name == name && option.Type == discordgo.ApplicationCommandOptionString {
			return option.StringValue()
		}
	}
	return ""
}

// GetIntegerOption extracts an integer option value from command options
func GetIntegerOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) int64 {
	for _, option := range options {
		if option.Name == name && option.Type == discordgo.ApplicationCommandOptionInteger {
			return option.IntValue()
		}
	}
	return 0
}

// GetBooleanOption extracts a boolean option value from command options
func GetBooleanOption(options []*discordgo.ApplicationCommandInteractionDataOption, name string) bool {
	for _, option := range options {
		if option.Name == name && option.Type == discordgo.ApplicationCommandOptionBoolean {
			return option.BoolValue()
		}
	}
	return false
}

// EmbedBuilder constrói embeds padronizados
type EmbedBuilder struct{}

// Success cria um embed de sucesso
func (EmbedBuilder) Success(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Success(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// Error cria um embed de erro
func (EmbedBuilder) Error(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Error(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// Info cria um embed informativo
func (EmbedBuilder) Info(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Info(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// Warning cria um embed de aviso
func (EmbedBuilder) Warning(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Warning(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// ConfigurationUtils fornece utilitários para configuração
type ConfigurationUtils struct{}

// EnsureGuildConfig garante que existe uma configuração para o servidor
func (ConfigurationUtils) EnsureGuildConfig(configManager *files.ConfigManager, guildID string) *files.GuildConfig {
	config := configManager.GuildConfig(guildID)
	if config == nil {
		config = &files.GuildConfig{
			GuildID:      guildID,
			AllowedRoles: []string{},
			Rulesets:     []files.Ruleset{},
			LooseLists:   []files.Rule{},
			Blocklist:    []string{},
		}
	}
	return config
}

// CompareCommands compara dois comandos para verificar se são semanticamente iguais
func CompareCommands(a, b *discordgo.ApplicationCommand) bool {
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

// GenerateID gera um ID único baseado no timestamp atual
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// RemoveFromSlice remove um item de uma slice
func RemoveFromSlice[T comparable](slice []T, item T) []T {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// RemoveAtIndex remove um item em um índice específico
func RemoveAtIndex[T any](slice []T, index int) []T {
	if index < 0 || index >= len(slice) {
		return slice
	}
	return append(slice[:index], slice[index+1:]...)
}

// ContainsAny verifica se a slice contém algum dos items
func ContainsAny[T comparable](slice []T, items ...T) bool {
	for _, item := range items {
		if slices.Contains(slice, item) {
			return true
		}
	}
	return false
}
