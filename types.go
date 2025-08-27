package discordcore

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	configPath     string
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

// DiscordCore holds the core configuration for the Discord bot package.
type DiscordCore struct {
	BotName     string
	Token       string
	SupportPath string
	ConfigPath  string
	Branch      string
}

// NewDiscordCore creates a new DiscordCore instance.
// It fetches the bot name from the Discord API and initializes paths based on the current git branch.
func NewDiscordCore() *DiscordCore {
	branch := getCurrentGitBranch()
	token := getDiscordBotToken(branch)
	botName := getBotNameFromAPI(token)
	supportPath := getApplicationSupportPath(branch, botName)
	configPath := filepath.Join(supportPath, "configs")

	// Ensure directories exist
	ensureDirectories([]string{supportPath, configPath})

	// Ensure config files exist
	if err := EnsureConfigFiles(configPath); err != nil {
		log.Fatalf("Failed to ensure config files: %v", err)
	}

	return &DiscordCore{
		BotName:     botName,
		Token:       token,
		SupportPath: supportPath,
		ConfigPath:  configPath,
		Branch:      branch,
	}
}

// NewConfigManager creates a new ConfigManager using this DiscordCore's config path.
func (core *DiscordCore) NewConfigManager() *ConfigManager {
	return NewConfigManager(core.ConfigPath)
}

// NewAvatarCacheManager creates a new AvatarCacheManager using this DiscordCore's config path.
func (core *DiscordCore) NewAvatarCacheManager() *AvatarCacheManager {
	return NewAvatarCacheManager(core.ConfigPath)
}

// GetToken returns the Discord bot token.
func (core *DiscordCore) GetToken() string {
	return core.Token
}

// GetBotName returns the bot name.
func (core *DiscordCore) GetBotName() string {
	return core.BotName
}

// GetConfigPath returns the config path.
func (core *DiscordCore) GetConfigPath() string {
	return core.ConfigPath
}

// GetSupportPath returns the support path.
func (core *DiscordCore) GetSupportPath() string {
	return core.SupportPath
}

// GetBranch returns the current branch.
func (core *DiscordCore) GetBranch() string {
	return core.Branch
}

// getCurrentGitBranch gets the current git branch.
func getCurrentGitBranch() string {
	data, err := os.ReadFile(".git/HEAD")
	if err != nil {
		log.Printf("Failed to read git HEAD: %v", err)
		return "unknown"
	}
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref: ") {
		parts := strings.Split(line, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return line
}

// getApplicationSupportPath returns the application support path based on branch and bot name.
func getApplicationSupportPath(branch, botName string) string {
	if branch == "main" {
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", botName)
	}
	return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", fmt.Sprintf("%s (Development)", botName))
}

// getDiscordBotToken returns the Discord bot token based on the branch.
func getDiscordBotToken(branch string) string {
	var token string
	switch branch {
	case "main":
		token = os.Getenv("DISCORD_BOT_TOKEN_MAIN")
	case "development":
		token = os.Getenv("DISCORD_BOT_TOKEN_DEV")
	default:
		token = os.Getenv("DISCORD_BOT_TOKEN_DEFAULT")
	}

	if token == "" {
		log.Fatalf("Discord bot token is not set for branch: %s", branch)
	}

	log.Printf("Discord bot token set for branch: %s", branch)
	return token
}

// getBotNameFromAPI fetches the bot's username from the Discord API using the token.
func getBotNameFromAPI(token string) string {
	req, err := http.NewRequest("GET", "https://discord.com/api/v10/users/@me", nil)
	if err != nil {
		log.Fatalf("Failed to create request for bot info: %v", err)
	}

	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to fetch bot info from Discord API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Discord API returned status %d when fetching bot info", resp.StatusCode)
	}

	var user struct {
		Username string `json:"username"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		log.Fatalf("Failed to decode bot info response: %v", err)
	}

	log.Printf("Fetched bot name from API: %s", user.Username)
	return user.Username
}
