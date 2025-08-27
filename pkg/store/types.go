package store

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// GuildConfig holds the configuration for a specific guild.
type GuildConfig struct {
	GuildID             string    `json:"guild_id"`
	CommandChannelID    string    `json:"command_channel_id"`
	UserLogChannelID    string    `json:"user_log_channel_id"` // Renamed from AvatarLogChannelID
	AutomodLogChannelID string    `json:"automod_log_channel_id"`
	AllowedRoles        []string  `json:"allowed_roles"`
	Rulesets            []Ruleset `json:"rulesets,omitempty"`
	LooseLists          []Rule    `json:"loose_rules,omitempty"` // Regras soltas, não associadas a nenhuma ruleset
	Blocklist           []string  `json:"blocklist,omitempty"`
}

// BotConfig holds the configuration for the bot.
type BotConfig struct {
	Guilds      []GuildConfig `json:"guilds"`
	ActiveGuild string        `json:"active_guild,omitempty"`
}

// Manager handles bot configuration management.
type ConfigManager struct {
	configFilePath string
	cacheFilePath  string
	logsDirPath    string
	config         *BotConfig
	mu             sync.RWMutex
}

// AvatarCache holds the cached avatars for a guild.
type AvatarCache struct {
	Avatars     map[string]string `json:"avatars"`
	LastUpdated time.Time         `json:"last_updated"`
	GuildID     string            `json:"guild_id"`
}

// AvatarChange holds information about a user's avatar change.
type AvatarChange struct {
	UserID    string
	Username  string
	OldAvatar string
	NewAvatar string
	Timestamp time.Time
}

// RuleType distinguishes between native Discord rules and custom bot rules
const (
	RuleTypeNative = "native"
	RuleTypeCustom = "custom"
)

// List represents a single list in the system.
type List struct {
	ID          string `json:"id"`
	Type        string `json:"type"` // "native" or "custom"
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Native list fields
	NativeID string `json:"native_id,omitempty"`
	// Custom list fields
	BlockedKeywords []string `json:"blocked_keywords,omitempty"`
}

// Rule representa uma regra que pode carregar um conjunto de listas.
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

// StatusString returns a human-readable status for the ruleset (Enabled/Disabled)
func (r Ruleset) StatusString() string {
	if r.Enabled {
		return "Enabled"
	}
	return "Disabled"
}

// Adicionar métodos para gerenciar lists, rules e rulesets no ConfigManager
func (mgr *ConfigManager) AddList(guildID string, list List) error {
	log.Printf("AddList called with guildID: %s, listID: %s", guildID, list.ID)
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		log.Printf("GuildConfig not found for guildID: %s", guildID)
		return fmt.Errorf("guild not found")
	}

	log.Printf("Appending list to LooseLists for guildID: %s", guildID)
	guildConfig.LooseLists = append(guildConfig.LooseLists, Rule{
		ID:      list.ID,
		Name:    list.Name,
		Lists:   []List{list},
		Enabled: true,
	})
	log.Printf("List appended successfully for guildID: %s", guildID)
	return mgr.SaveConfig()
}

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

// Corrigir lógica para adicionar lists a rules
func (mgr *ConfigManager) AddListToRule(guildID string, ruleID string, list List) error {
	log.Printf("AddListToRule called with guildID: %s, ruleID: %s, listID: %s", guildID, ruleID, list.ID)
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	guildConfig := mgr.GuildConfig(guildID)
	if guildConfig == nil {
		log.Printf("GuildConfig not found for guildID: %s", guildID)
		return fmt.Errorf("guild not found")
	}

	for i, rule := range guildConfig.LooseLists {
		if rule.ID == ruleID {
			log.Printf("Rule found for ruleID: %s, appending list", ruleID)
			guildConfig.LooseLists[i].Lists = append(guildConfig.LooseLists[i].Lists, list)
			log.Printf("List appended successfully to ruleID: %s", ruleID)
			return mgr.SaveConfig()
		}
	}

	log.Printf("Rule not found for ruleID: %s", ruleID)
	return fmt.Errorf("rule not found")
}

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
