package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/alice-bnuy/discordcore/v2/internal/util"
	"github.com/alice-bnuy/errutil"
	"github.com/alice-bnuy/logutil"
	"github.com/bwmarrin/discordgo"
)

func init() {
	// Ensure config files exist in the new paths
	if err := EnsureConfigFiles(); err != nil {
		log.Fatalf("Failed to ensure config files: %v", err)
	}
}

// --- Initialization & Persistence ---

func NewConfigManager() *ConfigManager {
	configFilePath := util.GetSettingsFilePath()
	return &ConfigManager{
		configFilePath: configFilePath,
		cacheFilePath:  util.GetCacheFilePath(),
		jsonManager:    util.NewJSONManager(configFilePath),
	}
}

// NewManager creates a new configuration manager.
func NewConfigManagerWithPath(configPath string) *ConfigManager {
	return &ConfigManager{
		configFilePath: configPath,
		jsonManager:    util.NewJSONManager(configPath),
	}
}

// Load loads the configuration from file.
func (mgr *ConfigManager) LoadConfig() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		mgr.config = &BotConfig{Guilds: []GuildConfig{}}
	}

	err := mgr.jsonManager.Load(mgr.config)
	if err != nil {
		if os.IsNotExist(err) {
			logutil.Warnf(LogLoadConfigFileNotFound, mgr.configFilePath)
			return nil
		}
		return errutil.HandleConfigError("read", mgr.configFilePath, func() error { return err })
	}

	if len(mgr.config.Guilds) == 0 {
		logutil.Warnf(LogLoadConfigNoGuilds, mgr.configFilePath)
	}

	logutil.Infof(LogLoadConfigSuccess, mgr.configFilePath)
	return nil
}

// Save saves the current configuration to file.
func (mgr *ConfigManager) SaveConfig() error {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if mgr.config == nil {
		return errors.New(ErrCannotSaveNilConfig)
	}

	err := mgr.jsonManager.Save(mgr.config)
	if err != nil {
		return errutil.HandleConfigError("write", mgr.configFilePath, func() error { return err })
	}

	logutil.Infof(LogSaveConfigSuccess, mgr.configFilePath)
	return nil
}

// --- Getters ---

// GetConfigPath returns the config file path.
func (mgr *ConfigManager) ConfigPath() string { return mgr.configFilePath }

// GetConfig returns the current configuration.
func (mgr *ConfigManager) Config() *BotConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.config
}

// HasGuilds checks if there are configured guilds.
func (mgr *ConfigManager) HasAnyGuilds() bool {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.config != nil && len(mgr.config.Guilds) > 0
}

// --- Guild Config Management ---

// GuildConfig returns the configuration for a specific guild.
func (mgr *ConfigManager) GuildConfig(guildID string) *GuildConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	if mgr.config == nil {
		return nil
	}
	for i := range mgr.config.Guilds {
		if mgr.config.Guilds[i].GuildID == guildID {
			return &mgr.config.Guilds[i]
		}
	}
	return nil
}

// AddGuildConfig adds or replaces a guild configuration.
func (mgr *ConfigManager) AddGuildConfig(guildCfg GuildConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil {
		mgr.config = &BotConfig{Guilds: []GuildConfig{}}
	}
	var guilds []GuildConfig
	for _, g := range mgr.config.Guilds {
		if g.GuildID != guildCfg.GuildID {
			guilds = append(guilds, g)
		}
	}
	mgr.config.Guilds = append(guilds, guildCfg)
	return nil
}

// RemoveGuildConfig removes a guild configuration.
func (mgr *ConfigManager) RemoveGuildConfig(guildID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil {
		return
	}
	var guilds []GuildConfig
	for _, g := range mgr.config.Guilds {
		if g.GuildID != guildID {
			guilds = append(guilds, g)
		}
	}
	mgr.config.Guilds = guilds
}

// --- Guild Detection & Addition ---

// AutoDetectGuilds automatically detects guilds where the bot is present.
func (mgr *ConfigManager) DetectGuilds(session *discordgo.Session) error {
	mgr.mu.Lock()
	if mgr.config == nil {
		mgr.config = &BotConfig{Guilds: []GuildConfig{}}
	}
	mgr.config.Guilds = []GuildConfig{}
	mgr.mu.Unlock()

	for _, g := range session.State.Guilds {
		fullGuild, err := session.Guild(g.ID)
		if err != nil {
			logutil.WithField("guildID", g.ID).Warnf(LogCouldNotFetchGuild, err)
			continue
		}

		channelID := FindSuitableChannel(session, g.ID)
		if channelID == "" {
			logutil.WithField("guildID", g.ID).Warnf(LogNoSuitableChannel, fullGuild.Name)
			continue
		}

		roles := FindAdminRoles(session, g.ID, fullGuild.OwnerID)
		guildCfg := GuildConfig{
			GuildID:          g.ID,
			CommandChannelID: channelID,
			UserLogChannelID: channelID,
			AllowedRoles:     roles,
		}
		mgr.mu.Lock()
		mgr.config.Guilds = append(mgr.config.Guilds, guildCfg)
		mgr.mu.Unlock()
		logutil.WithFields(map[string]any{
			"guildName": fullGuild.Name,
			"guildID":   g.ID,
			"channelID": channelID,
		}).Info(LogGuildAdded)
	}
	return mgr.SaveConfig()
}

// AddGuildToConfig adds a new guild to the configuration.
func (mgr *ConfigManager) RegisterGuild(session *discordgo.Session, guildID string) error {
	// ensure config exists
	mgr.mu.RLock()
	cfgNil := mgr.config == nil
	mgr.mu.RUnlock()
	if cfgNil {
		mgr.mu.Lock()
		if mgr.config == nil {
			mgr.config = &BotConfig{Guilds: []GuildConfig{}}
		}
		mgr.mu.Unlock()
	} else {
		mgr.mu.RLock()
		for _, g := range mgr.config.Guilds {
			if g.GuildID == guildID {
				mgr.mu.RUnlock()
				logutil.WithField("guildID", guildID).Info(LogGuildAlreadyConfigured)
				return nil
			}
		}
		mgr.mu.RUnlock()
	}
	guild, err := session.Guild(guildID)
	if err != nil {
		return fmt.Errorf(ErrGuildInfoFetchMsg, guildID, err)
	}
	channelID := FindSuitableChannel(session, guildID)
	if channelID == "" {
		return fmt.Errorf(ErrNoSuitableChannelMsg, guild.Name)
	}
	roles := FindAdminRoles(session, guildID, guild.OwnerID)
	guildCfg := GuildConfig{
		GuildID:          guildID,
		CommandChannelID: channelID,
		UserLogChannelID: channelID,
		AllowedRoles:     roles,
	}
	mgr.mu.Lock()
	mgr.config.Guilds = append(mgr.config.Guilds, guildCfg)
	mgr.mu.Unlock()
	channelName := channelID
	if ch, err := session.Channel(channelID); err == nil {
		channelName = ch.Name
	}
	logutil.WithFields(map[string]any{
		"guildName": guild.Name,
		"guildID":   guildID,
		"channel":   channelName,
	}).Info(LogGuildAdded)
	return mgr.SaveConfig()
}

// --- Utility & Logging ---

// ShowConfiguredGuilds logs the configured guilds (no active guild concept).
func ShowConfiguredGuilds(s *discordgo.Session, configManager *ConfigManager) {
	configuration := configManager.Config()
	if configuration == nil || len(configuration.Guilds) == 0 {
		return
	}
	for _, guildConfig := range configuration.Guilds {
		if guild, err := s.Guild(guildConfig.GuildID); err == nil {
			logutil.WithFields(map[string]any{
				"guildName": guild.Name,
				"guildID":   guild.ID,
			}).Info(LogMonitorGuild)
		} else {
			logutil.WithField("guildID", guildConfig.GuildID).Warn(LogGuildNotAccessible)
		}
	}
}

func FindSuitableChannel(session *discordgo.Session, guildID string) string {
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return ""
	}
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			permissions, err := session.UserChannelPermissions(session.State.User.ID, channel.ID)
			if err == nil && (permissions&discordgo.PermissionSendMessages) != 0 {
				if channel.Name == "general" || channel.Name == "geral" || channel.Name == "bot-logs" || channel.Name == "logs" {
					return channel.ID
				}
				if channel.ID != "" {
					return channel.ID
				}
			}
		}
	}
	return ""
}

func FindAdminRoles(session *discordgo.Session, guildID, ownerID string) []string {
	var allowedRoles []string
	roles, err := session.GuildRoles(guildID)
	if err == nil {
		for _, role := range roles {
			if role.Name != "@everyone" && (role.Permissions&discordgo.PermissionAdministrator) != 0 {
				allowedRoles = append(allowedRoles, role.ID)
			}
		}
	}
	if len(allowedRoles) == 0 && ownerID != "" {
		if member, err := session.GuildMember(guildID, ownerID); err == nil && len(member.Roles) > 0 {
			allowedRoles = append(allowedRoles, member.Roles[0])
		}
	}
	return allowedRoles
}

func GetTextChannels(session *discordgo.Session, guildID string) ([]*discordgo.Channel, error) {
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return nil, err
	}
	var textChannels []*discordgo.Channel
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			permissions, err := session.UserChannelPermissions(session.State.User.ID, channel.ID)
			if err == nil && (permissions&discordgo.PermissionSendMessages) != 0 {
				textChannels = append(textChannels, channel)
			}
		}
	}
	return textChannels, nil
}

func ValidateChannel(session *discordgo.Session, guildID, channelID string) error {
	channel, err := session.Channel(channelID)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrChannelNotFound, err)
	}
	if channel.GuildID != guildID {
		return errors.New(ErrChannelWrongGuild)
	}
	if channel.Type != discordgo.ChannelTypeGuildText {
		return errors.New(ErrChannelWrongType)
	}
	permissions, err := session.UserChannelPermissions(session.State.User.ID, channelID)
	if err != nil {
		return fmt.Errorf(ErrFailedCheckPerms, err)
	}
	if (permissions & discordgo.PermissionSendMessages) == 0 {
		return errors.New(ErrChannelNoPermissions)
	}
	return nil
}

func EnsureConfigFiles() error {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(ApplicationSupportPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ensure settings file
	if err := EnsureSettingsFile(); err != nil {
		return fmt.Errorf("failed to ensure settings file: %w", err)
	}

	// Ensure application cache file
	if err := EnsureApplicationCacheFile(); err != nil {
		return fmt.Errorf("failed to ensure application cache file: %w", err)
	}

	return nil
}

// EnsureSettingsFile ensures the settings.json file exists and is properly initialized
func EnsureSettingsFile() error {
	// Create preferences subdirectory if it doesn't exist
	preferencesDir := filepath.Join(ApplicationSupportPath, "preferences")
	if err := os.MkdirAll(preferencesDir, 0755); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	// Check if settings file exists
	settingsFilePath := filepath.Join(preferencesDir, "settings.json")
	if _, err := os.Stat(settingsFilePath); os.IsNotExist(err) {
		logutil.Infof("Settings file not found, creating default at %s", settingsFilePath)

		// Create basic empty config
		defaultConfig := BotConfig{
			Guilds: []GuildConfig{},
		}
		configData, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to create settings file: %w", err)
		}
		if err := os.WriteFile(settingsFilePath, configData, 0644); err != nil {
			return fmt.Errorf("failed to write settings file: %w", err)
		}
	}

	return nil
}

// EnsureApplicationCacheFile ensures the application_cache.json file exists and is properly initialized
func EnsureApplicationCacheFile() error {
	// Create data subdirectory if it doesn't exist
	dataDir := filepath.Join(ApplicationSupportPath, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Check if cache file exists
	cacheFilePath := filepath.Join(dataDir, "application_cache.json")
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		logutil.Infof("Application cache file not found, creating default at %s", cacheFilePath)

		// Create basic empty cache
		defaultCache := `{"guilds":{},"last_updated":"","version":"2.0"}`
		if err := os.WriteFile(cacheFilePath, []byte(defaultCache), 0644); err != nil {
			return fmt.Errorf("failed to write application cache file: %w", err)
		}
	}

	return nil
}

// --- Unified Settings Operations ---
//
// These functions provide a standardized way to work with settings.json
//
// Example usage:
//   config, err := LoadSettingsFile()
//   if err != nil { /* handle error */ }
//
//   // Modify config...
//
//   err = SaveSettingsFile(config)
//   if err != nil { /* handle error */ }

// LoadSettingsFile loads settings from the standardized settings.json file
func LoadSettingsFile() (*BotConfig, error) {
	settingsPath := util.GetSettingsFilePath()
	jsonManager := util.NewJSONManager(settingsPath)

	config := &BotConfig{Guilds: []GuildConfig{}}
	err := jsonManager.Load(config)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return empty config if file doesn't exist
		}
		return nil, fmt.Errorf("failed to load settings from %s: %w", settingsPath, err)
	}

	return config, nil
}

// SaveSettingsFile saves settings to the standardized settings.json file
func SaveSettingsFile(config *BotConfig) error {
	if config == nil {
		return fmt.Errorf("cannot save nil config")
	}

	settingsPath := util.GetSettingsFilePath()
	jsonManager := util.NewJSONManager(settingsPath)

	if err := jsonManager.Save(config); err != nil {
		return fmt.Errorf("failed to save settings to %s: %w", settingsPath, err)
	}

	return nil
}

// --- Unified Application Cache Operations ---
//
// These functions provide a standardized way to work with application_cache.json
// They can be used as an alternative to AvatarCacheManager for simple operations
//
// Example usage:
//   cache, err := LoadApplicationCacheFile()
//   if err != nil { /* handle error */ }
//
//   // Modify cache...
//
//   err = SaveApplicationCacheFile(cache)
//   if err != nil { /* handle error */ }
//
// Note: For complex cache operations with threading and throttling,
// prefer using AvatarCacheManager which provides additional features like:
// - Thread-safe operations
// - Throttled saves
// - Guild-specific operations

// LoadApplicationCacheFile loads cache from the standardized application_cache.json file
func LoadApplicationCacheFile() (*AvatarMultiGuildCache, error) {
	cachePath := util.GetCacheFilePath()
	jsonManager := util.NewJSONManager(cachePath)

	cache := &AvatarMultiGuildCache{
		Guilds:      make(map[string]*AvatarCache),
		LastUpdated: time.Now(),
		Version:     "2.0",
	}

	err := jsonManager.Load(cache)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil // Return empty cache if file doesn't exist
		}
		return nil, fmt.Errorf("failed to load application cache from %s: %w", cachePath, err)
	}

	return cache, nil
}

// SaveApplicationCacheFile saves cache to the standardized application_cache.json file
func SaveApplicationCacheFile(cache *AvatarMultiGuildCache) error {
	if cache == nil {
		return fmt.Errorf("cannot save nil cache")
	}

	cachePath := util.GetCacheFilePath()
	jsonManager := util.NewJSONManager(cachePath)

	if err := jsonManager.Save(cache); err != nil {
		return fmt.Errorf("failed to save application cache to %s: %w", cachePath, err)
	}

	return nil
}

// LogConfiguredGuilds logs a summary of configured guilds. Returns error if any guilds are inaccessible.
func LogConfiguredGuilds(configManager *ConfigManager, session *discordgo.Session) error {
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		logutil.Warn(LogNoConfiguredGuilds)
		return nil
	}
	logutil.Infof(LogFoundConfiguredGuilds, len(cfg.Guilds))
	var errCount int
	for _, g := range cfg.Guilds {
		guild, err := session.Guild(g.GuildID)
		if err == nil {
			logutil.WithFields(map[string]any{"guildName": guild.Name, "guildID": guild.ID}).Info("🔎 Will monitor this guild")
		} else {
			logutil.WithField("guildID", g.GuildID).Warn(LogGuildNotAccessible)
			errCount++
		}
	}
	if errCount > 0 {
		return fmt.Errorf(ErrGuildsNotAccessible, errCount)
	}
	return nil
}

// FindRulesetByID searches for a ruleset by its ID in the guild configuration.
func (cfg *GuildConfig) FindRulesetByID(id string) (*Ruleset, int) {
	for idx, rs := range cfg.Rulesets {
		if rs.ID == id {
			return &cfg.Rulesets[idx], idx
		}
	}
	return nil, -1
}
