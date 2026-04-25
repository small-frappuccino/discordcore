package files

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

// --- Initialization & Persistence ---

// NewConfigManagerWithStore creates a new configuration manager backed by the
// provided persistence store.
func NewConfigManagerWithStore(store ConfigStore) *ConfigManager {
	description := ""
	if store != nil {
		description = store.Describe()
	}
	return &ConfigManager{
		configFilePath: description,
		store:          store,
	}
}

// Load loads the configuration from file.
func (mgr *ConfigManager) LoadConfig() error {
	mgr.mu.Lock()

	if mgr.config == nil {
		mgr.config = &BotConfig{Guilds: []GuildConfig{}}
	}

	if mgr.store == nil {
		mgr.mu.Unlock()
		return fmt.Errorf("config store is not configured")
	}
	cfg, err := mgr.store.Load()
	if err != nil {
		mgr.mu.Unlock()
		return fmt.Errorf("load config from %s: %w", mgr.ConfigPath(), err)
	}
	mgr.config = cfg

	if len(mgr.config.Guilds) == 0 {
		log.ApplicationLogger().Info(fmt.Sprintf(LogLoadConfigNoGuilds, mgr.ConfigPath()))
	}

	dupCount, err := mgr.rebuildGuildIndexLocked("load")
	if err != nil {
		log.ApplicationLogger().Warn("Guild config index rebuild warning", "error", err, "path", mgr.ConfigPath())
	}
	orderMigrated := normalizeAutoAssignmentRoleOrder(mgr.config)
	if validationErr := validateBotConfig(mgr.config); validationErr != nil {
		mgr.mu.Unlock()
		return wrapValidationError(validationErr)
	}
	mgr.publishSnapshotLocked()
	mgr.mu.Unlock()

	if dupCount > 0 || orderMigrated {
		if saveErr := mgr.SaveConfig(); saveErr != nil {
			return fmt.Errorf("save config after normalization: %w", saveErr)
		}
		log.ApplicationLogger().Info("Saved config after normalization", "path", mgr.ConfigPath(), "duplicates", dupCount, "autoRoleOrderMigrated", orderMigrated)
	} else if exists, err := mgr.store.Exists(); err == nil && !exists {
		log.ApplicationLogger().Info(fmt.Sprintf(LogLoadConfigFileNotFound, mgr.ConfigPath()))
	}
	return nil
}

// Save saves the current configuration to file.
func (mgr *ConfigManager) SaveConfig() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if err := mgr.saveConfigLocked(); err != nil {
		return err
	}
	mgr.publishSnapshotLocked()
	return nil
}

func (mgr *ConfigManager) saveConfigLocked() error {
	if mgr.config == nil {
		return errors.New(ErrCannotSaveNilConfig)
	}
	if mgr.store == nil {
		return fmt.Errorf("config store is not configured")
	}
	if validationErr := validateBotConfig(mgr.config); validationErr != nil {
		return wrapValidationError(validationErr)
	}

	if err := mgr.store.Save(mgr.config); err != nil {
		return fmt.Errorf("save config to %s: %w", mgr.ConfigPath(), err)
	}

	log.ApplicationLogger().Info(fmt.Sprintf(LogSaveConfigSuccess, mgr.ConfigPath()))
	return nil
}

// UpdateRuntimeConfig mutates runtime_config and persists the change.
func (mgr *ConfigManager) UpdateRuntimeConfig(fn func(*RuntimeConfig) error) (RuntimeConfig, error) {
	snapshot, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		if fn == nil {
			return nil
		}
		return fn(&cfg.RuntimeConfig)
	})
	if err != nil {
		return RuntimeConfig{}, err
	}
	return snapshot.RuntimeConfig, nil
}

// --- Getters ---

// ConfigPath returns a human-readable description of the active config backend.
func (mgr *ConfigManager) ConfigPath() string {
	if mgr == nil {
		return ""
	}
	if strings.TrimSpace(mgr.configFilePath) != "" {
		return mgr.configFilePath
	}
	if mgr.store != nil {
		return mgr.store.Describe()
	}
	return ""
}

// Config returns the current published read-only configuration snapshot.
// Callers must treat the returned value as immutable. Use SnapshotConfig when a
// defensive copy is required for mutation.
func (mgr *ConfigManager) Config() *BotConfig {
	snap := mgr.currentPublishedSnapshot()
	if snap == nil {
		return nil
	}
	return snap.config
}

// HasGuilds checks if there are configured guilds.
func (mgr *ConfigManager) HasAnyGuilds() bool {
	snap := mgr.currentPublishedSnapshot()
	return snap != nil && snap.config != nil && len(snap.config.Guilds) > 0
}

// --- Guild Config Management ---

// GuildConfig returns the current published read-only snapshot for a specific guild.
// Callers must treat the returned value as immutable.
func (mgr *ConfigManager) GuildConfig(guildID string) *GuildConfig {
	if mgr == nil || guildID == "" {
		return nil
	}
	snap := mgr.currentPublishedSnapshot()
	if snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
		return nil
	}
	mgr.indexMisses.Add(1)
	return mgr.guildConfigWithPublish(guildID)
}

func (mgr *ConfigManager) guildConfigWithPublish(guildID string) *GuildConfig {
	if mgr == nil {
		return nil
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil || guildID == "" {
		return nil
	}
	if snap := mgr.publishSnapshotLocked(); snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
	}
	if _, err := mgr.rebuildGuildIndexLocked("lookup_miss"); err != nil {
		log.ApplicationLogger().Warn("Guild config index rebuild warning", "guildID", guildID, "error", err)
	}
	if snap := mgr.publishSnapshotLocked(); snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
	}
	log.ApplicationLogger().Info("Guild config not found", "guildID", guildID)
	return nil
}

func (mgr *ConfigManager) rebuildGuildIndexLocked(reason string) (int, error) {
	mgr.indexRebuilds.Add(1)
	if mgr.config == nil {
		mgr.guildIndex = nil
		log.ApplicationLogger().Info("Guild config index cleared", "reason", reason)
		return 0, nil
	}
	index := make(map[string]int, len(mgr.config.Guilds))
	deduped := make([]GuildConfig, 0, len(mgr.config.Guilds))
	dupCount := 0

	for _, g := range mgr.config.Guilds {
		gid := g.GuildID
		if gid == "" {
			deduped = append(deduped, g)
			continue
		}
		if _, exists := index[gid]; exists {
			dupCount++
			continue
		}
		index[gid] = len(deduped)
		deduped = append(deduped, g)
	}

	if dupCount > 0 {
		mgr.indexDuplicates.Add(uint64(dupCount))
		log.ApplicationLogger().Warn("Duplicate guild configs removed", "reason", reason, "duplicates", dupCount, "remaining", len(deduped))
		mgr.config.Guilds = deduped
	}

	mgr.guildIndex = index
	log.ApplicationLogger().Info("Guild config index rebuilt", "reason", reason, "guilds", len(mgr.config.Guilds))
	if dupCount > 0 {
		return dupCount, fmt.Errorf("removed %d duplicate guild configs", dupCount)
	}
	return dupCount, nil
}

// GuildIndexStats returns counters for index rebuilds, misses, and duplicate removals.
func (mgr *ConfigManager) GuildIndexStats() GuildIndexStats {
	if mgr == nil {
		return GuildIndexStats{}
	}
	return GuildIndexStats{
		Rebuilds:   mgr.indexRebuilds.Load(),
		Misses:     mgr.indexMisses.Load(),
		Duplicates: mgr.indexDuplicates.Load(),
	}
}

// AddGuildConfig adds or replaces a guild configuration.
func (mgr *ConfigManager) AddGuildConfig(guildCfg GuildConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	next := cloneBotConfigPtr(mgr.config)
	if next == nil {
		next = &BotConfig{Guilds: []GuildConfig{}}
	}
	// Remove any existing entry with the same GuildID, then append the new config.
	next.Guilds = append(slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildCfg.GuildID
	}), guildCfg)
	mgr.config = next
	if _, err := mgr.rebuildGuildIndexLocked("add"); err != nil {
		return fmt.Errorf("add guild config: %w", err)
	}
	mgr.publishSnapshotLocked()
	return nil
}

// RemoveGuildConfig removes a guild configuration.
func (mgr *ConfigManager) RemoveGuildConfig(guildID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil {
		return
	}
	next := cloneBotConfigPtr(mgr.config)
	next.Guilds = slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildID
	})
	mgr.config = next
	if _, err := mgr.rebuildGuildIndexLocked("remove"); err != nil {
		log.ApplicationLogger().Warn("Guild config index rebuild warning", "guildID", guildID, "error", err)
	}
	mgr.publishSnapshotLocked()
}

// --- Guild Detection & Addition ---

// AutoDetectGuilds automatically detects guilds where the bot is present.
func (mgr *ConfigManager) DetectGuilds(session *discordgo.Session) error {
	return mgr.DetectGuildsForBot(session, "")
}

// DetectGuildsForBot automatically detects guilds and binds them to a bot
// instance when one is provided.
func (mgr *ConfigManager) DetectGuildsForBot(session *discordgo.Session, botInstanceID string) error {
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	detected := make([]GuildConfig, 0, len(session.State.Guilds))

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
		if entryLeaveID == "" {
			entryLeaveID = channelID
		}
		guildCfg := GuildConfig{
			GuildID:       g.ID,
			BotInstanceID: botInstanceID,
			Channels: ChannelsConfig{
				Commands:      channelID,
				AvatarLogging: channelID,
				RoleUpdate:    channelID,
				MemberJoin:    entryLeaveID,
				MemberLeave:   entryLeaveID,
				MessageEdit:   channelID,
				MessageDelete: channelID,
			},
			Roles: RolesConfig{
				Allowed: roles,
			},
		}
		detected = append(detected, guildCfg)
		log.ApplicationLogger().Info("Guild added", "guildName", fullGuild.Name, "guildID", g.ID, "channelID", channelID)
	}

	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		cfg.Guilds = detected
		return nil
	})
	return err
}

// AddGuildToConfig adds a new guild to the configuration.
func (mgr *ConfigManager) RegisterGuild(session *discordgo.Session, guildID string) error {
	return mgr.RegisterGuildForBot(session, guildID, "")
}

// RegisterGuildForBot adds a new guild to the configuration and binds it to
// the provided bot instance when one is specified.
func (mgr *ConfigManager) RegisterGuildForBot(session *discordgo.Session, guildID, botInstanceID string) error {
	if session == nil {
		return fmt.Errorf("%w: discord session is unavailable", ErrGuildBootstrapDiscordFetch)
	}
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	if mgr.GuildConfig(guildID) != nil {
		log.ApplicationLogger().Info("Guild already configured, skipping", "guildID", guildID)
		return nil
	}
	guild, err := session.Guild(guildID)
	if err != nil {
		return fmt.Errorf("%w: "+ErrGuildInfoFetchMsg, ErrGuildBootstrapDiscordFetch, guildID, err)
	}
	channelID := FindSuitableChannel(session, guildID)
	if channelID == "" {
		return fmt.Errorf("%w: "+ErrNoSuitableChannelMsg, ErrGuildBootstrapPrerequisite, guild.Name)
	}
	roles := FindAdminRoles(session, guildID, guild.OwnerID)
	entryLeaveID := FindEntryLeaveChannel(session, guildID)
	if entryLeaveID == "" {
		entryLeaveID = channelID
	}

	guildCfg := GuildConfig{
		GuildID:       guildID,
		BotInstanceID: botInstanceID,
		Channels: ChannelsConfig{
			Commands:      channelID,
			AvatarLogging: channelID,
			RoleUpdate:    channelID,
			MemberJoin:    entryLeaveID,
			MemberLeave:   entryLeaveID,
			MessageEdit:   channelID,
			MessageDelete: channelID,
		},
		Roles: RolesConfig{
			Allowed: roles,
		},
	}

	if _, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		cfg.Guilds = append(cfg.Guilds, guildCfg)
		return nil
	}); err != nil {
		return fmt.Errorf("register guild: save config: %w", err)
	}

	channelName := channelID
	if ch, err := session.Channel(channelID); err == nil {
		channelName = ch.Name
	}
	log.ApplicationLogger().Info(LogGuildAdded, "guildName", guild.Name, "guildID", guildID, "channel", channelName)
	return nil
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
	return LogConfiguredGuildsForBot(configManager, session, "", "")
}

// LogConfiguredGuildsForBot logs the guild subset assigned to the provided bot
// instance. Legacy guilds without a binding are included when botInstanceID is
// empty.
func LogConfiguredGuildsForBot(configManager *ConfigManager, session *discordgo.Session, botInstanceID, defaultBotInstanceID string) error {
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.ApplicationLogger().Warn(LogNoConfiguredGuilds)
		return nil
	}

	guilds := cfg.Guilds
	if normalizedBotInstanceID := NormalizeBotInstanceID(botInstanceID); normalizedBotInstanceID != "" {
		guilds = cfg.GuildsForBotInstance(normalizedBotInstanceID, defaultBotInstanceID)
	}
	if len(guilds) == 0 {
		log.ApplicationLogger().Warn(LogNoConfiguredGuilds)
		return nil
	}

	log.ApplicationLogger().Info(fmt.Sprintf(LogFoundConfiguredGuilds, len(guilds)))
	var errCount int
	for _, g := range guilds {
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
