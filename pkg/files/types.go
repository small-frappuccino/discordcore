package files

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// ## Config Types

// GuildConfig holds the configuration for a specific guild.
type GuildConfig struct {
	GuildID                 string    `json:"guild_id"`
	CommandChannelID        string    `json:"command_channel_id"`
	UserLogChannelID        string    `json:"user_log_channel_id"`         // Para logs de entrada/saída e avatares
	UserEntryLeaveChannelID string    `json:"user_entry_leave_channel_id"` // Canal dedicado para entradas/saídas de usuários
	MessageLogChannelID     string    `json:"message_log_channel_id"`      // Para logs de mensagens editadas/deletadas
	AutomodLogChannelID     string    `json:"automod_log_channel_id"`
	AllowedRoles            []string  `json:"allowed_roles"`
	Rulesets                []Ruleset `json:"rulesets,omitempty"`
	LooseLists              []Rule    `json:"loose_rules,omitempty"` // Regras soltas, não associadas a nenhuma ruleset
	Blocklist               []string  `json:"blocklist,omitempty"`
}

// BotConfig holds the configuration for the bot.
type BotConfig struct {
	Guilds      []GuildConfig `json:"guilds"`
	ActiveGuild string        `json:"active_guild,omitempty"`
}

// ConfigManager handles bot configuration management.
type ConfigManager struct {
	configFilePath string
	logsDirPath    string
	config         *BotConfig
	mu             sync.RWMutex
	jsonManager    *util.JSONManager
}

// AvatarChange holds information about a user's avatar change.
type AvatarChange struct {
	UserID    string
	Username  string
	OldAvatar string
	NewAvatar string
	Timestamp time.Time
}

// ## Rule and Ruleset Types

// RuleType distinguishes between native Discord rules and custom bot rules.
const (
	RuleTypeNative = "native"
	RuleTypeCustom = "custom"
)

// List represents a single list in the system.
type List struct {
	ID              string   `json:"id"`
	Type            string   `json:"type"` // "native" or "custom"
	Name            string   `json:"name"`
	Description     string   `json:"description,omitempty"`
	NativeID        string   `json:"native_id,omitempty"`        // Native list fields
	BlockedKeywords []string `json:"blocked_keywords,omitempty"` // Custom list fields
}

// Rule represents a rule that can load a set of lists.
type Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Lists   []List `json:"lists"` // Conjunto de listas associadas à regra
	Enabled bool   `json:"enabled"`
}

// Ruleset holds a collection of rules.
type Ruleset struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Rules   []Rule `json:"rules"`
	Enabled bool   `json:"enabled"`
}

// StatusString returns a human-readable status for the ruleset (Enabled/Disabled).
func (r Ruleset) StatusString() string {
	if r.Enabled {
		return "Enabled"
	}
	return "Disabled"
}

// ## ConfigManager Methods

// AddList adds a list to the LooseLists of a guild.
func (mgr *ConfigManager) AddList(guildID string, list List) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		log.Error().Errorf("GuildConfig not found for guildID: %s", guildID)
		return fmt.Errorf("guild not found")
	}
	guildConfig.LooseLists = append(guildConfig.LooseLists, Rule{
		ID:      list.ID,
		Name:    list.Name,
		Lists:   []List{list},
		Enabled: true,
	})
	log.Info().Databasef("List appended successfully for guildID: %s", guildID)
	return mgr.SaveConfig()
}

// AddRule adds a rule to the LooseLists of a guild.
func (mgr *ConfigManager) AddRule(guildID string, rule Rule) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		return fmt.Errorf("guild not found")
	}

	guildConfig.LooseLists = append(guildConfig.LooseLists, rule)
	return mgr.SaveConfig()
}

// AddRuleset adds a ruleset to a guild.
func (mgr *ConfigManager) AddRuleset(guildID string, ruleset Ruleset) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		return fmt.Errorf("guild not found")
	}

	guildConfig.Rulesets = append(guildConfig.Rulesets, ruleset)
	return mgr.SaveConfig()
}

// AddListToRule adds a list to a specific rule in a guild.
func (mgr *ConfigManager) AddListToRule(guildID string, ruleID string, list List) error {
	log.Info().Databasef("AddListToRule called with guildID: %s, ruleID: %s, listID: %s", guildID, ruleID, list.ID)
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		log.Error().Errorf("GuildConfig not found for guildID: %s", guildID)
		return fmt.Errorf("guild not found")
	}

	for i, rule := range guildConfig.LooseLists {
		if rule.ID == ruleID {
			log.Info().Databasef("Rule found for ruleID: %s, appending list", ruleID)
			guildConfig.LooseLists[i].Lists = append(guildConfig.LooseLists[i].Lists, list)
			log.Info().Databasef("List appended successfully to ruleID: %s", ruleID)
			return mgr.SaveConfig()
		}
	}

	log.Error().Errorf("Rule not found for ruleID: %s", ruleID)
	return fmt.Errorf("rule not found")
}

// ## GuildConfig Methods

// FindListByName searches for a list by its name in LooseLists.
func (gc *GuildConfig) FindListByName(name string) *List {
	for _, rule := range gc.LooseLists {
		for _, list := range rule.Lists {
			if list.Name == name {
				return &list
			}
		}
	}
	return nil
}

// ListExists checks if a list with the given name already exists in LooseLists.
func (gc *GuildConfig) ListExists(name string) bool {
	return gc.FindListByName(name) != nil
}

// ## Error Types

// ValidationError represents a validation error with field context.
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(field string, value interface{}, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// ConfigError represents configuration-related errors.
type ConfigError struct {
	Operation string
	Path      string
	Cause     error
}

func (e ConfigError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("config %s failed for %s: %v", e.Operation, e.Path, e.Cause)
	}
	return fmt.Sprintf("config %s failed for %s", e.Operation, e.Path)
}

func (e ConfigError) Unwrap() error {
	return e.Cause
}

// NewConfigError creates a new configuration error.
func NewConfigError(operation, path string, cause error) ConfigError {
	return ConfigError{
		Operation: operation,
		Path:      path,
		Cause:     cause,
	}
}

// DiscordError represents Discord API related errors.
type DiscordError struct {
	Operation string
	Code      int
	Message   string
	Cause     error
}

func (e DiscordError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("Discord API error during %s (code %d): %s", e.Operation, e.Code, e.Message)
	}
	return fmt.Sprintf("Discord API error during %s: %s", e.Operation, e.Message)
}

func (e DiscordError) Unwrap() error {
	return e.Cause
}

// NewDiscordError creates a new Discord API error.
func NewDiscordError(operation string, code int, message string, cause error) DiscordError {
	return DiscordError{
		Operation: operation,
		Code:      code,
		Message:   message,
		Cause:     cause,
	}
}

// ## Utility Functions

// IsRetryableError determines if an error can be retried.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific retryable errors.
	if errors.Is(err, ErrRateLimited) {
		return true
	}

	// Check for Discord errors that might be retryable.
	var discordErr DiscordError
	if errors.As(err, &discordErr) {
		// 5xx errors are typically retryable.
		return discordErr.Code >= 500 && discordErr.Code < 600
	}

	return false
}

// ## General Errors

var ErrRateLimited = errors.New("rate limited")
