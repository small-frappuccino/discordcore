package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	DiscordBotName         string = "Alice Bot"
	DiscordBotToken        string
	ApplicationSupportPath string
	ApplicationConfigPath  string
	CurrentGitBranch       string
)

func init() {
	// Get current git branch
	branch := getCurrentGitBranch()
	CurrentGitBranch = branch

	// Set DiscordBotToken and ApplicationSupportPath
	DiscordBotToken = getDiscordBotToken()
	ApplicationSupportPath = getApplicationSupportPath(branch)
	ApplicationConfigPath = filepath.Join(ApplicationSupportPath, "configs")

	// Ensure all application directories exist
	configDirectories := []string{ApplicationSupportPath, ApplicationConfigPath}
	if err := ensureDirectories(configDirectories); err != nil {
		log.Fatalf("Failed to initialize application directory: %v", err)
	}

	// Ensure config files exist in the new paths
	if err := EnsureConfigFiles(); err != nil {
		log.Fatalf("Failed to ensure config files: %v", err)
	}
}

func ensureDirectories(directories []string) error {
	for _, dir := range directories {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Printf("Failed to create directory: %v", err)
				Errorf("Failed to create directory: %s, error: %v", dir, err)
				return fmt.Errorf("failed to create directory: %w", err)
			}
			log.Printf("Directory created at %s", dir)
			Infof("Directory created at %s", dir)
		}
	}
	return nil
}

func getApplicationSupportPath(branch string) string {
	if branch == "main" {
		return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", DiscordBotName)
	}
	return filepath.Join(os.Getenv("HOME"), "Library", "Application Support", fmt.Sprintf("%s (Development)", DiscordBotName))
}

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

// SetDiscordBotToken returns the Discord bot token based on the current Git branch.
func getDiscordBotToken() string {
	var token string
	switch CurrentGitBranch {
	case "main":
		token = os.Getenv("DISCORD_BOT_TOKEN_MAIN")
	case "development":
		token = os.Getenv("DISCORD_BOT_TOKEN_DEV")
	default:
		token = os.Getenv("DISCORD_BOT_TOKEN_DEFAULT")
	}

	if token == "" {
		log.Fatalf("Discord bot token is not set for branch: %s", CurrentGitBranch)
	}

	log.Printf("Discord bot token set for branch: %s", CurrentGitBranch)
	return token
}

// --- Initialization & Persistence ---

func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		configFilePath: ConfigFilePath,
		cacheFilePath:  CacheFilePath,
		logsDirPath:    LogsDirPath,
	}
}

// NewManager creates a new configuration manager.
func NewConfigManagerWithPath(configPath string) *ConfigManager {
	return &ConfigManager{configFilePath: configPath}
}

// Load loads the configuration from file.
func (mgr *ConfigManager) LoadConfig() error {
	path, err := safeJoin(ApplicationConfigPath, mgr.configFilePath)
	if err != nil {
		Debugf(LogLoadConfigFailedJoinPaths, mgr.configFilePath, err)
		return fmt.Errorf(ErrFailedResolveConfigPath, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			Warnf(LogLoadConfigFileNotFound, path)
			mgr.mu.Lock()
			mgr.config = &BotConfig{Guilds: []GuildConfig{}}
			mgr.mu.Unlock()
			return nil
		}
		Errorf(LogLoadConfigFailedReadFile, path, err)
		return HandleConfigError("read", mgr.configFilePath, func() error { return err })
	}

	var config BotConfig
	if err := json.Unmarshal(data, &config); err != nil {
		Errorf(LogLoadConfigFailedUnmarshal, path, err)
		return HandleConfigError("unmarshal", mgr.configFilePath, func() error { return err })
	}

	// Validando a configuração carregada
	if len(config.Guilds) == 0 {
		Warnf(LogLoadConfigNoGuilds, path)
	}

	mgr.mu.Lock()
	mgr.config = &config
	mgr.mu.Unlock()
	Infof(LogLoadConfigSuccess, path)
	return nil
}

// Save saves the current configuration to file.
func (mgr *ConfigManager) SaveConfig() error {
	log.Println("SaveConfig called")
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if mgr.config == nil {
		log.Println("SaveConfig: config is nil")
		Errorf(LogSaveConfigNilConfig)
		return errors.New(ErrCannotSaveNilConfig)
	}

	path, err := safeJoin(ApplicationConfigPath, mgr.configFilePath)
	if err != nil {
		log.Printf("SaveConfig: failed to resolve path: %v", err)
		Errorf(LogSaveConfigFailedResolvePath, mgr.ConfigPath(), err)
		return fmt.Errorf(ErrFailedResolveConfigPath, err)
	}

	data, err := json.MarshalIndent(mgr.config, "", "  ")
	if err != nil {
		log.Printf("SaveConfig: failed to marshal config: %v", err)
		Errorf(LogSaveConfigFailedMarshal, err)
		return fmt.Errorf(ErrFailedMarshalConfig, err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("SaveConfig: failed to write file: %v", err)
		Errorf(LogSaveConfigFailedWriteFile, path, err)
		return HandleConfigError("write", mgr.configFilePath, func() error { return err })
	}

	log.Printf("SaveConfig: successfully saved to %s", path)
	Infof(LogSaveConfigSuccess, path)
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
			WithField("guildID", g.ID).Warnf(LogCouldNotFetchGuild, err)
			continue
		}

		channelID := FindSuitableChannel(session, g.ID)
		if channelID == "" {
			WithField("guildID", g.ID).Warnf(LogNoSuitableChannel, fullGuild.Name)
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
		WithFields(map[string]interface{}{
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
				WithField("guildID", guildID).Info(LogGuildAlreadyConfigured)
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
	WithFields(map[string]interface{}{
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
			WithFields(map[string]interface{}{
				"guildName": guild.Name,
				"guildID":   guild.ID,
			}).Info(LogMonitorGuild)
		} else {
			WithField("guildID", guildConfig.GuildID).Warn(LogGuildNotAccessible)
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
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(ApplicationConfigPath, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file exists
	configFilePath := filepath.Join(ApplicationConfigPath, "config.json")
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		Infof("Config file not found, creating default at %s", configFilePath)

		// Create basic empty config
		defaultConfig := BotConfig{
			Guilds: []GuildConfig{},
		}
		configData, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}
		if err := os.WriteFile(configFilePath, configData, 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
	}

	// Check if cache file exists
	cacheFilePath := filepath.Join(ApplicationConfigPath, "cache.json")
	if _, err := os.Stat(cacheFilePath); os.IsNotExist(err) {
		Infof("Cache file not found, creating default at %s", cacheFilePath)

		// Create basic empty cache
		defaultCache := `{"guilds":{},"last_updated":"","version":"1.0"}`
		if err := os.WriteFile(cacheFilePath, []byte(defaultCache), 0644); err != nil {
			return fmt.Errorf("failed to write cache file: %w", err)
		}
	}

	return nil
}

// LogConfiguredGuilds logs a summary of configured guilds. Returns error if any guilds are inaccessible.
func LogConfiguredGuilds(configManager *ConfigManager, session *discordgo.Session) error {
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		Warn(LogNoConfiguredGuilds)
		return nil
	}
	Infof(LogFoundConfiguredGuilds, len(cfg.Guilds))
	var errCount int
	for _, g := range cfg.Guilds {
		guild, err := session.Guild(g.GuildID)
		if err == nil {
			WithFields(map[string]interface{}{"guildName": guild.Name, "guildID": guild.ID}).Info("🔎 Will monitor this guild")
		} else {
			WithField("guildID", g.GuildID).Warn(LogGuildNotAccessible)
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
