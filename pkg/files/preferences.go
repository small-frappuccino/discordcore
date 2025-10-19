package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/errutil"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// --- Initialization & Persistence ---

func NewConfigManager() *ConfigManager {
	configFilePath := util.GetSettingsFilePath()
	return &ConfigManager{
		configFilePath: configFilePath,

		jsonManager: util.NewJSONManager(configFilePath),
	}
}

// NewConfigManagerWithPath creates a new configuration manager.
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
			log.ApplicationLogger().Info(fmt.Sprintf(LogLoadConfigFileNotFound, mgr.configFilePath))
			return nil
		}
		return errutil.HandleConfigError("read", mgr.configFilePath, func() error { return err })
	}

	if len(mgr.config.Guilds) == 0 {
		log.ApplicationLogger().Info(fmt.Sprintf(LogLoadConfigNoGuilds, mgr.configFilePath))
	}

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

	log.ApplicationLogger().Info(fmt.Sprintf(LogSaveConfigSuccess, mgr.configFilePath))
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
			// preserve the guildID field and format the message as a warning
			log.ApplicationLogger().Warn("Could not fetch guild details for guild", "guildID", g.ID, "err", err)
			continue
		}

		channelID := FindSuitableChannel(session, g.ID)
		if channelID == "" {
			log.ApplicationLogger().Warn("No suitable channel found in guild", "guildName", fullGuild.Name, "guildID", g.ID)
			continue
		}

		// Determine allowed roles
		roles := FindAdminRoles(session, g.ID, fullGuild.OwnerID)

		entryLeaveID := FindEntryLeaveChannel(session, g.ID)
		guildCfg := GuildConfig{
			GuildID:                 g.ID,
			CommandChannelID:        channelID,
			UserLogChannelID:        channelID,
			UserEntryLeaveChannelID: entryLeaveID,
			AllowedRoles:            roles,
		}
		mgr.mu.Lock()
		mgr.config.Guilds = append(mgr.config.Guilds, guildCfg)
		mgr.mu.Unlock()
		log.ApplicationLogger().Info("Guild added", "guildName", fullGuild.Name, "guildID", g.ID, "channelID", channelID)
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
				log.ApplicationLogger().Info("Guild already configured, skipping", "guildID", guildID)
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
	entryLeaveID := FindEntryLeaveChannel(session, guildID)
	if entryLeaveID == "" {
		entryLeaveID = channelID
	}

	guildCfg := GuildConfig{
		GuildID:                 guildID,
		CommandChannelID:        channelID,
		UserLogChannelID:        channelID,
		UserEntryLeaveChannelID: entryLeaveID,
		AllowedRoles:            roles,
	}
	mgr.mu.Lock()
	mgr.config.Guilds = append(mgr.config.Guilds, guildCfg)
	mgr.mu.Unlock()
	channelName := channelID
	if ch, err := session.Channel(channelID); err == nil {
		channelName = ch.Name
	}
	log.ApplicationLogger().Info(LogGuildAdded, "guildName", guild.Name, "guildID", guildID, "channel", channelName)
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
			log.ApplicationLogger().Info(LogMonitorGuild, "guildName", guild.Name, "guildID", guild.ID)
		} else {
			log.ApplicationLogger().Warn(LogGuildNotAccessible, "guildID", guildConfig.GuildID)
		}
	}
}

func FindSuitableChannel(session *discordgo.Session, guildID string) string {
	// Verify session state is properly initialized
	if session == nil || session.State == nil || session.State.User == nil {
		return ""
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil || channels == nil {
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

func FindEntryLeaveChannel(session *discordgo.Session, guildID string) string {
	// Verify session state is properly initialized
	if session == nil || session.State == nil || session.State.User == nil {
		return ""
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return ""
	}
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			name := strings.ToLower(channel.Name)
			if name == "user-entry-leave" {
				if HasSendPermission(session, channel.ID) {
					return channel.ID
				}
			}
		}
	}
	return ""
}

// Deprecated: prefer FindEntryLeaveChannel. This function may create a new channel;
// avoid using it during detection/registration flows.
func IsCanonicalEntryLeaveName(name string) bool {
	name = strings.ToLower(name)
	switch name {
	case "user-entry-leave", "entry-leave", "joins-leaves", "join-leave", "member-log", "members-log", "member-logs", "user-logs", "welcome-goodbye":
		return true
	default:
		return false
	}
}

func HasSendPermission(session *discordgo.Session, channelID string) bool {
	if session == nil || session.State == nil || session.State.User == nil || channelID == "" {
		return false
	}
	if perms, err := session.UserChannelPermissions(session.State.User.ID, channelID); err == nil {
		return (perms & discordgo.PermissionSendMessages) != 0
	}
	return false
}

func FindOrCreateEntryLeaveChannel(session *discordgo.Session, guildID string) string {
	// Verify session state is properly initialized
	if session == nil || session.State == nil || session.State.User == nil {
		return ""
	}
	channels, err := session.GuildChannels(guildID)
	if err == nil {
		for _, channel := range channels {
			if channel.Type == discordgo.ChannelTypeGuildText {
				name := strings.ToLower(channel.Name)
				if name == "user-entry-leave" || name == "entry-leave" || name == "joins-leaves" || name == "join-leave" || name == "member-log" || name == "members-log" || name == "member-logs" || name == "user-logs" || name == "welcome-goodbye" {
					if perms, err2 := session.UserChannelPermissions(session.State.User.ID, channel.ID); err2 == nil && (perms&discordgo.PermissionSendMessages) != 0 {
						return channel.ID
					}
				}
			}
		}
	}

	newCh, err := session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name:  "user-entry-leave",
		Type:  discordgo.ChannelTypeGuildText,
		Topic: "User entry/leave notifications",
	})
	if err == nil && newCh != nil {
		return newCh.ID
	}

	return FindSuitableChannel(session, guildID)
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
	// Verify session state is properly initialized
	if session == nil || session.State == nil || session.State.User == nil {
		return nil, fmt.Errorf("session not properly initialized")
	}
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
	// Verify session state is properly initialized
	if session == nil || session.State == nil || session.State.User == nil {
		return errors.New("session not properly initialized")
	}
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
	if err := os.MkdirAll(util.ApplicationSupportPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Ensure settings file
	if err := EnsureSettingsFile(); err != nil {
		return fmt.Errorf("failed to ensure settings file: %w", err)
	}

	return nil
}

// EnsureSettingsFile ensures the settings.json file exists and is properly initialized.
// If the file already exists and has a valid structure, it will not be modified.
func EnsureSettingsFile() error {
	// Ensure base config directory exists
	if err := os.MkdirAll(util.ApplicationSupportPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	// Ensure preferences subdirectory (explicit layout: ~/.config/<BotName>/preferences/settings.json)
	preferencesDir := filepath.Join(util.ApplicationSupportPath, "preferences")
	if err := os.MkdirAll(preferencesDir, 0755); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	// Determine settings file status
	exists, valid, settingsFilePath, err := SettingsFileStatus()
	if err != nil {
		return fmt.Errorf("failed to check settings file status: %w", err)
	}

	// If file does not exist, create default
	if !exists {
		log.ApplicationLogger().Info(fmt.Sprintf("Settings file not found, creating default at %s", settingsFilePath))
		defaultConfig := BotConfig{Guilds: []GuildConfig{}}
		configData, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to create settings file: %w", err)
		}
		if err := os.WriteFile(settingsFilePath, configData, 0644); err != nil {
			return fmt.Errorf("failed to write settings file: %w", err)
		}
		return nil
	}

	// If it exists and is valid, do not modify it
	if valid {
		log.ApplicationLogger().Info(fmt.Sprintf("Settings file exists and is valid at %s; no changes made", settingsFilePath))
		return nil
	}

	// If it exists but is invalid, replace with a default structure
	log.ApplicationLogger().Warn(fmt.Sprintf("Settings file at %s exists but is invalid JSON structure; rewriting with default schema", settingsFilePath))
	defaultConfig := BotConfig{Guilds: []GuildConfig{}}
	configData, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to create default settings content: %w", err)
	}
	if err := os.WriteFile(settingsFilePath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

// SettingsFileStatus reports whether settings.json exists and whether its structure is valid.
func SettingsFileStatus() (exists bool, valid bool, path string, err error) {
	path = util.GetSettingsFilePath()
	info, statErr := os.Stat(path)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return false, false, path, nil
		}
		return false, false, path, fmt.Errorf("failed to stat settings file: %w", statErr)
	}
	if info.IsDir() {
		return true, false, path, fmt.Errorf("settings path is a directory")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return true, false, path, fmt.Errorf("failed to read settings file: %w", readErr)
	}

	// Validate minimal structure by attempting to unmarshal into BotConfig
	var tmp BotConfig
	if json.Unmarshal(data, &tmp) != nil {
		return true, false, path, nil
	}

	// Consider it valid if it unmarshals into BotConfig (even if empty)
	return true, true, path, nil
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

// LogConfiguredGuilds logs a summary of configured guilds. Returns error if any guilds are inaccessible.
func LogConfiguredGuilds(configManager *ConfigManager, session *discordgo.Session) error {
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.ApplicationLogger().Warn(LogNoConfiguredGuilds)
		return nil
	}
	log.ApplicationLogger().Info(fmt.Sprintf(LogFoundConfiguredGuilds, len(cfg.Guilds)))
	var errCount int
	for _, g := range cfg.Guilds {
		guild, err := session.Guild(g.GuildID)
		if err == nil {
			log.ApplicationLogger().Info(fmt.Sprintf("🔎 Will monitor this guild: %s (%s)", guild.Name, guild.ID))
		} else {
			log.ApplicationLogger().Warn(fmt.Sprintf("%s: %s", LogGuildNotAccessible, g.GuildID))
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
