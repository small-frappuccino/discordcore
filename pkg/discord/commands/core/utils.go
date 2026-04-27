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
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// OptionExtractor simplifies extraction of options for Discord commands
type OptionExtractor struct {
	options []*discordgo.ApplicationCommandInteractionDataOption
}

// NewOptionExtractor creates a new option extractor
func NewOptionExtractor(options []*discordgo.ApplicationCommandInteractionDataOption) *OptionExtractor {
	return &OptionExtractor{options: options}
}

// String extracts a string option by name
func (e *OptionExtractor) String(name string) string {
	for _, opt := range e.options {
		if opt.Name == name {
			return strings.TrimSpace(opt.StringValue())
		}
	}
	return ""
}

// StringRequired extracts a required string option
func (e *OptionExtractor) StringRequired(name string) (string, error) {
	value := e.String(name)
	if value == "" {
		return "", NewValidationError(name, fmt.Sprintf("Option '%s' is required", name))
	}
	return value, nil
}

// Bool extracts a boolean option by name
func (e *OptionExtractor) Bool(name string) bool {
	for _, opt := range e.options {
		if opt.Name == name {
			return opt.BoolValue()
		}
	}
	return false
}

// Int extracts an integer option by name
func (e *OptionExtractor) Int(name string) int64 {
	for _, opt := range e.options {
		if opt.Name == name {
			return opt.IntValue()
		}
	}
	return 0
}

// Float extracts a float option by name
func (e *OptionExtractor) Float(name string) float64 {
	for _, opt := range e.options {
		if opt.Name == name {
			return opt.FloatValue()
		}
	}
	return 0
}

// HasOption checks whether an option exists
func (e *OptionExtractor) HasOption(name string) bool {
	for _, opt := range e.options {
		if opt.Name == name {
			return true
		}
	}
	return false
}

// GetAllOptions returns all options as a map
func (e *OptionExtractor) GetAllOptions() map[string]any {
	result := make(map[string]any)
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

// ConfigPersister manages configuration persistence
type ConfigPersister struct {
	configManager *files.ConfigManager
}

// NewConfigPersister creates a new configuration persister
func NewConfigPersister(cm *files.ConfigManager) *ConfigPersister {
	return &ConfigPersister{configManager: cm}
}

// Save saves the guild configuration
func (cp *ConfigPersister) Save(config *files.GuildConfig) error {
	if err := cp.configManager.AddGuildConfig(*config); err != nil {
		return fmt.Errorf("failed to update config in memory: %w", err)
	}
	if err := cp.configManager.SaveConfig(); err != nil {
		return fmt.Errorf("failed to persist config: %w", err)
	}
	return nil
}

// SaveWithBackup saves the configuration with a backup
func (cp *ConfigPersister) SaveWithBackup(config *files.GuildConfig) error {
	// Implement backup if needed
	return cp.Save(config)
}

// PermissionChecker manages user permission checks
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

// HasPermission checks whether the user has permission to use commands
func (pc *PermissionChecker) HasPermission(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	if pc.hasAdministrativeAccess(guildID, userID) {
		return true
	}
	guildConfig := pc.config.GuildConfig(guildID)
	if guildConfig == nil || len(guildConfig.Roles.Allowed) == 0 {
		return false
	}

	member, memberFound, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member",
			"operation", "commands.permission.has_permission.resolve_member",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !memberFound || member == nil {
		return false
	}

	for _, userRole := range member.Roles {
		if slices.Contains(guildConfig.Roles.Allowed, userRole) {
			return true
		}
	}
	return false
}

func (pc *PermissionChecker) hasAdministrativeAccess(guildID, userID string) bool {
	ownerID, ownerFound, err := pc.ResolveOwnerID(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild owner",
			"operation", "commands.permission.has_permission.resolve_owner",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
	}
	if err == nil && ownerFound && ownerID == userID {
		return true
	}

	member, memberFound, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member for admin access",
			"operation", "commands.permission.has_permission.resolve_member_admin",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !memberFound || member == nil {
		return false
	}

	roles, err := pc.ResolveRoles(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild roles for admin access",
			"operation", "commands.permission.has_permission.resolve_roles_admin",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	permissions := memberPermissionBits(member, roles, guildID)
	if permissions&discordgo.PermissionAdministrator == discordgo.PermissionAdministrator {
		return true
	}
	if permissions&discordgo.PermissionManageGuild == discordgo.PermissionManageGuild {
		return true
	}
	return false
}

func memberPermissionBits(member *discordgo.Member, roles []*discordgo.Role, guildID string) int64 {
	if member == nil {
		return 0
	}
	rolesByID := make(map[string]*discordgo.Role, len(roles))
	for _, role := range roles {
		if role == nil {
			continue
		}
		rolesByID[role.ID] = role
	}

	var permissions int64
	if role, ok := rolesByID[guildID]; ok && role != nil {
		permissions |= role.Permissions
	}
	for _, roleID := range member.Roles {
		if role, ok := rolesByID[roleID]; ok && role != nil {
			permissions |= role.Permissions
		}
	}
	return permissions
}

// HasRole checks whether the user has a specific role
func (pc *PermissionChecker) HasRole(guildID, userID, roleID string) bool {
	member, ok, err := pc.ResolveMember(guildID, userID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild member for role check",
			"operation", "commands.permission.has_role.resolve_member",
			"guildID", guildID,
			"userID", userID,
			"roleID", roleID,
			"err", err,
		)
		return false
	}
	if !ok || member == nil {
		return false
	}
	return slices.Contains(member.Roles, roleID)
}

// IsOwner checks whether the user is the server owner
func (pc *PermissionChecker) IsOwner(guildID, userID string) bool {
	if guildID == "" {
		return false
	}
	ownerID, ok, err := pc.ResolveOwnerID(guildID)
	if err != nil {
		log.ErrorLoggerRaw().Error(
			"Permission checker failed to resolve guild owner for owner check",
			"operation", "commands.permission.is_owner.resolve_owner",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
		return false
	}
	if !ok {
		return false
	}
	return ownerID == userID
}

// StringUtils provides utilities for string manipulation
type StringUtils struct{}

// ProcessCommaSeparatedList parses a comma-separated string
func (StringUtils) ProcessCommaSeparatedList(input string) []string {
	if input == "" {
		return nil
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
	// Remove control characters and extra spaces
	input = strings.TrimSpace(input)
	// Remove multiple line breaks
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

// EmbedBuilder builds standardized embeds
type EmbedBuilder struct{}

// Success creates a success embed
func (EmbedBuilder) Success(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Success(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// Error creates an error embed
func (EmbedBuilder) Error(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Error(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// Info creates an informational embed
func (EmbedBuilder) Info(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Info(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// Warning creates a warning embed
func (EmbedBuilder) Warning(title, description string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       theme.Warning(),
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// ConfigurationUtils provides configuration utilities
type ConfigurationUtils struct{}

// EnsureGuildConfig ensures there is a configuration for the server
func (ConfigurationUtils) EnsureGuildConfig(configManager *files.ConfigManager, guildID string) *files.GuildConfig {
	config := configManager.GuildConfig(guildID)
	if config == nil {
		config = &files.GuildConfig{
			GuildID: guildID,
			Roles:   files.RolesConfig{Allowed: []string{}},
		}
	}
	return config
}

// normalizeCommandOptions ensures required options come before optional options,
// recursively for nested option trees, while preserving relative order.
func normalizeCommandOptions(options []*discordgo.ApplicationCommandOption) []*discordgo.ApplicationCommandOption {
	if len(options) == 0 {
		return nil
	}

	required := make([]*discordgo.ApplicationCommandOption, 0, len(options))
	optional := make([]*discordgo.ApplicationCommandOption, 0, len(options))

	for _, opt := range options {
		if opt == nil {
			continue
		}
		cloned := *opt
		cloned.Options = normalizeCommandOptions(opt.Options)

		if len(opt.Choices) > 0 {
			cloned.Choices = append([]*discordgo.ApplicationCommandOptionChoice(nil), opt.Choices...)
		}
		if len(opt.ChannelTypes) > 0 {
			cloned.ChannelTypes = append([]discordgo.ChannelType(nil), opt.ChannelTypes...)
		}

		if cloned.Required {
			required = append(required, &cloned)
		} else {
			optional = append(optional, &cloned)
		}
	}

	out := make([]*discordgo.ApplicationCommandOption, 0, len(required)+len(optional))
	out = append(out, required...)
	out = append(out, optional...)
	return out
}

// CompareCommands compares two commands to check if they are semantically equal.
// Option order is normalized so equivalent command definitions with required-first
// normalization compare as equal.
func CompareCommands(a, b *discordgo.ApplicationCommand) bool {
	if a == nil || b == nil {
		return a == b
	}
	ca := struct {
		Name        string                                `json:"name"`
		Description string                                `json:"description"`
		Options     []*discordgo.ApplicationCommandOption `json:"options"`
	}{a.Name, a.Description, normalizeCommandOptions(a.Options)}
	cb := struct {
		Name        string                                `json:"name"`
		Description string                                `json:"description"`
		Options     []*discordgo.ApplicationCommandOption `json:"options"`
	}{b.Name, b.Description, normalizeCommandOptions(b.Options)}
	ba, _ := json.Marshal(ca)
	bb, _ := json.Marshal(cb)
	return string(ba) == string(bb)
}

// GenerateID generates a unique ID based on the current timestamp
func GenerateID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// RemoveFromSlice removes an item from a slice
func RemoveFromSlice[T comparable](slice []T, item T) []T {
	for i, v := range slice {
		if v == item {
			return slices.Delete(slice, i, i+1)
		}
	}
	return slice
}

// RemoveAtIndex removes an item at a specific index
func RemoveAtIndex[T any](slice []T, index int) []T {
	if index < 0 || index >= len(slice) {
		return slice
	}
	return slices.Delete(slice, index, index+1)
}

// ContainsAny checks whether the slice contains any of the items
func ContainsAny[T comparable](slice []T, items ...T) bool {
	for _, item := range items {
		if slices.Contains(slice, item) {
			return true
		}
	}
	return false
}

// FormatOptions format options for logging
func FormatOptions(options []*discordgo.ApplicationCommandOption) string {
	if len(options) == 0 {
		return ""
	}
	var parts []string
	for _, opt := range options {
		parts = append(parts, fmt.Sprintf("%s (%s)", opt.Name, opt.Type.String()))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
