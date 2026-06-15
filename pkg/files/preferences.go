package files

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
)

// log.GenerateRequestID creates a unique transient identifier for error correlation.
func GenerateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(bytes)
}

// log.EmitBlockingError encapsulates the emission of structural failures with mandatory metadata.
func EmitBlockingError(msg string, err error, requestID string) {
	slog.Error(msg,
		slog.String("request_id", requestID),
		slog.String("synthetic_code", "500"),
		slog.String("stack_trace", string(debug.Stack())),
		slog.Any("error", err),
	)
}

// --- Initialization and Persistence ---

// NewConfigManagerWithStore instantiates a new config manager backed by the
// provided persistence layer.
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

// LoadConfigFromStore performs an atomic read and validation of the configuration
// on the persistence layer without mutating the active manager state.
func (mgr *ConfigManager) LoadConfigFromStore() (*BotConfig, bool, error) {
	if mgr.store == nil {
		err := fmt.Errorf("config store is not configured")
		log.EmitBlockingError("Failed to initialize configuration read", err, log.GenerateRequestID())
		return nil, false, err
	}
	cfg, err := mgr.store.Load()
	if err != nil {
		errWrap := fmt.Errorf("load configuration from %s: %w", mgr.ConfigPath(), err)
		log.EmitBlockingError("Structural failure in file loading", errWrap, log.GenerateRequestID())
		return nil, false, errWrap
	}

	orderMigrated := normalizeAutoAssignmentRoleOrder(cfg)

	if validationErr := validateBotConfig(cfg); validationErr != nil {
		errWrap := wrapValidationError(validationErr)
		log.EmitBlockingError("Validation failure of loaded configuration", errWrap, log.GenerateRequestID())
		return nil, false, errWrap
	}
	return cfg, orderMigrated, nil
}

// ApplyConfig atomically rotates the global configuration pointer and rebuilds indexes.
func (mgr *ConfigManager) ApplyConfig(cfg *BotConfig) int {
	if cfg == nil {
		return 0
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	slog.Debug("Starting atomic transition of configuration state",
		slog.Int("guilds_payload_size", len(cfg.Guilds)),
	)

	oldCfg := mgr.config
	mgr.config = cfg

	if len(mgr.config.Guilds) == 0 {
		slog.Warn("Applied configuration does not contain active guilds. Running in basal mode.",
			slog.String("path", mgr.ConfigPath()),
		)
	}

	dupCount, err := mgr.rebuildGuildIndexLocked("apply")
	if err != nil {
		slog.Warn("Mitigated degradation in index rebuilding",
			slog.String("error", err.Error()),
			slog.String("path", mgr.ConfigPath()),
		)
	}

	mgr.publishSnapshotLocked()
	mgr.notifySubscribersLocked(oldCfg, cfg)

	slog.Info("Configuration state transition completed",
		slog.Int("duplicates_removed", dupCount),
	)
	return dupCount
}

// LoadConfig loads the configuration directly from the filesystem.
func (mgr *ConfigManager) LoadConfig() error {
	cfg, orderMigrated, err := mgr.LoadConfigFromStore()
	if err != nil {
		return err
	}

	dupCount := mgr.ApplyConfig(cfg)

	if dupCount > 0 || orderMigrated {
		slog.Debug("Structural anomaly resolved in memory, forcing compensatory persistence",
			slog.Bool("order_migrated", orderMigrated),
			slog.Int("duplicates", dupCount),
		)
		if saveErr := mgr.SaveConfig(); saveErr != nil {
			errWrap := fmt.Errorf("save configuration after normalization: %w", saveErr)
			log.EmitBlockingError("Failed to write structural corrections to configuration", errWrap, log.GenerateRequestID())
			return errWrap
		}
		slog.Info("Configuration re-persisted after runtime normalization",
			slog.String("path", mgr.ConfigPath()),
			slog.Int("duplicates", dupCount),
			slog.Bool("autoRoleOrderMigrated", orderMigrated),
		)
	} else if exists, err := mgr.store.Exists(); err == nil && !exists {
		slog.Info("Initialized in clean state: primary file not detected",
			slog.String("path", mgr.ConfigPath()),
		)
	}
	return nil
}

// SaveConfig persists the active configuration to the filesystem.
func (mgr *ConfigManager) SaveConfig() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if err := mgr.saveConfigLocked(); err != nil {
		errWrap := fmt.Errorf("ConfigManager.SaveConfig: %w", err)
		log.EmitBlockingError("Blocking global persistence failure", errWrap, log.GenerateRequestID())
		return errWrap
	}
	mgr.publishSnapshotLocked()
	return nil
}

// SaveGuildConfig updates a specific guild configuration and persists the change immediately.
func (mgr *ConfigManager) SaveGuildConfig(cfg GuildConfig) error {
	slog.Debug("Updating granular guild state",
		slog.String("guildID", cfg.GuildID),
	)
	if err := mgr.AddGuildConfig(cfg); err != nil {
		return fmt.Errorf("failed to update in-memory configuration: %w", err)
	}
	if err := mgr.SaveConfig(); err != nil {
		return fmt.Errorf("failed to persist guild configuration: %w", err)
	}
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
		return fmt.Errorf("save configuration for %s: %w", mgr.ConfigPath(), err)
	}

	slog.Info("I/O state transition: Configuration successfully persisted",
		slog.String("path", mgr.ConfigPath()),
	)

	return nil
}

// UpdateRuntimeConfig mutates runtime_config and persists the change to disk.
func (mgr *ConfigManager) UpdateRuntimeConfig(fn func(*RuntimeConfig) error) (RuntimeConfig, error) {
	snapshot, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		if fn == nil {
			return nil
		}
		return fn(&cfg.RuntimeConfig)
	})
	if err != nil {
		errWrap := fmt.Errorf("ConfigManager.UpdateRuntimeConfig: %w", err)
		log.EmitBlockingError("Mutational failure in runtime configuration", errWrap, log.GenerateRequestID())
		return RuntimeConfig{}, errWrap
	}
	return snapshot.RuntimeConfig, nil
}

// --- Getters ---

// ConfigPath returns a text description of the active configuration backend.
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

// Config returns the current read-only published snapshot of the configuration.
func (mgr *ConfigManager) Config() *BotConfig {
	snap := mgr.currentPublishedSnapshot()
	if snap == nil {
		return nil
	}
	return snap.config
}

// HasAnyGuilds evaluates the existence of configured guilds.
func (mgr *ConfigManager) HasAnyGuilds() bool {
	snap := mgr.currentPublishedSnapshot()
	return snap != nil && snap.config != nil && len(snap.config.Guilds) > 0
}

// --- Guild Config Management ---

// GuildConfig returns the current read-only published snapshot of the configuration for a guild.
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
		slog.Warn("Index rebuilding triggered via mitigated cache miss",
			slog.String("guildID", guildID),
			slog.String("error", err.Error()),
		)
	}
	if snap := mgr.publishSnapshotLocked(); snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
	}
	slog.Debug("Guild mapping does not exist in consolidated index",
		slog.String("guildID", guildID),
	)
	return nil
}

func (mgr *ConfigManager) rebuildGuildIndexLocked(reason string) (int, error) {
	mgr.indexRebuilds.Add(1)
	if mgr.config == nil {
		mgr.guildIndex = nil
		slog.Info("Guild index cleared due to nil configuration",
			slog.String("reason", reason),
		)
		return 0, nil
	}

	slog.Debug("Iterating guild structures for hash index rebuilding",
		slog.String("reason", reason),
	)

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
			slog.Debug("Key collision avoided during index construction",
				slog.String("guildID", gid),
			)
			dupCount++
			continue
		}
		index[gid] = len(deduped)
		deduped = append(deduped, g)
	}

	if dupCount > 0 {
		mgr.indexDuplicates.Add(uint64(dupCount))
		slog.Warn("Structural integrity corrected locally: duplicate guilds purged from vector",
			slog.String("reason", reason),
			slog.Int("duplicates", dupCount),
			slog.Int("remaining", len(deduped)),
		)
		mgr.config.Guilds = deduped
	}

	mgr.guildIndex = index
	slog.Info("Structural state transition completed: Guild index rebuilt",
		slog.String("reason", reason),
		slog.Int("guilds_count", len(mgr.config.Guilds)),
	)

	if dupCount > 0 {
		return dupCount, fmt.Errorf("removed %d duplicate guild configurations", dupCount)
	}
	return dupCount, nil
}

// GuildIndexStats returns operational counters for index metrics.
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

// AddGuildConfig injects or replaces the mapped configuration of a guild.
func (mgr *ConfigManager) AddGuildConfig(guildCfg GuildConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	next := cloneBotConfigPtr(mgr.config)
	if next == nil {
		next = &BotConfig{Guilds: []GuildConfig{}}
	}

	slog.Debug("Granular guild injection into configuration tree",
		slog.String("guildID", guildCfg.GuildID),
	)

	next.Guilds = append(slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildCfg.GuildID
	}), guildCfg)

	mgr.config = next
	if _, err := mgr.rebuildGuildIndexLocked("add"); err != nil {
		errWrap := fmt.Errorf("add guild configuration: %w", err)
		log.EmitBlockingError("Critical failure attaching configuration to state tree", errWrap, log.GenerateRequestID())
		return errWrap
	}
	mgr.publishSnapshotLocked()
	return nil
}

// RemoveGuildConfig purges a guild configuration.
func (mgr *ConfigManager) RemoveGuildConfig(guildID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil {
		return
	}

	slog.Debug("Atomic removal of guild node in configuration",
		slog.String("guildID", guildID),
	)

	next := cloneBotConfigPtr(mgr.config)
	next.Guilds = slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildID
	})
	mgr.config = next

	if _, err := mgr.rebuildGuildIndexLocked("remove"); err != nil {
		slog.Warn("Collision mitigated during post-removal rebuild",
			slog.String("guildID", guildID),
			slog.String("error", err.Error()),
		)
	}
	mgr.publishSnapshotLocked()
}

// --- Guild Detection & Addition ---

// DetectGuilds automatically detects guilds where the bot is active.
func (mgr *ConfigManager) DetectGuilds(session *discordgo.Session) error {
	return mgr.DetectGuildsForBot(session, "")
}

// DetectGuildsForBot automates guild discovery and binds it to the
// corresponding bot identifier.
func (mgr *ConfigManager) DetectGuildsForBot(session *discordgo.Session, botInstanceID string) error {
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	detected := make([]GuildConfig, 0, len(session.State.Guilds))

	for _, g := range session.State.Guilds {
		fullGuild, err := session.Guild(g.ID)
		if err != nil {
			slog.Warn("Degradation in fetching guild architectural data; main operation will continue idly",
				slog.String("guildID", g.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		channelID := FindSuitableChannel(session, g.ID)
		if channelID == "" {
			slog.Warn("Mitigated failure: primary operational channel missing in target guild",
				slog.String("guildName", fullGuild.Name),
				slog.String("guildID", g.ID),
			)
			continue
		}

		roles := FindAdminRoles(session, g.ID)

		entryLeaveID := FindEntryLeaveChannel(session, g.ID)
		if entryLeaveID == "" {
			slog.Debug("Dynamic routing: using main channel as fallback for entry_leave",
				slog.String("guildID", g.ID),
			)
			entryLeaveID = channelID
		}

		guildCfg := GuildConfig{
			GuildID: g.ID,
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
		slog.Info("Network transition: Guild linked to discovery matrix",
			slog.String("guildName", fullGuild.Name),
			slog.String("guildID", g.ID),
			slog.String("channelID", channelID),
		)
	}

	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		cfg.Guilds = detected
		return nil
	})
	if err != nil {
		log.EmitBlockingError("Block update failure during heuristic detection phase", err, log.GenerateRequestID())
	}
	return err
}

// RegisterGuild explicitly injects a new guild.
func (mgr *ConfigManager) RegisterGuild(session *discordgo.Session, guildID string) error {
	return mgr.RegisterGuildForBot(session, guildID, "")
}

// RegisterGuildForBot injects and binds the guild to the parameterized bot instance.
func (mgr *ConfigManager) RegisterGuildForBot(session *discordgo.Session, guildID, botInstanceID string) error {
	if session == nil {
		err := fmt.Errorf("%w: discord session is not available", ErrGuildBootstrapDiscordFetch)
		log.EmitBlockingError("Corrupted state in register routine: Null session", err, log.GenerateRequestID())
		return err
	}
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	if mgr.GuildConfig(guildID) != nil {
		slog.Info("Pre-existing condition silently resolved: guild already injected",
			slog.String("guildID", guildID),
		)
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
	roles := FindAdminRoles(session, guildID)
	entryLeaveID := FindEntryLeaveChannel(session, guildID)
	if entryLeaveID == "" {
		entryLeaveID = channelID
	}

	guildCfg := GuildConfig{
		GuildID: guildID,
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
		errWrap := fmt.Errorf("register guild: save configuration: %w", err)
		log.EmitBlockingError("Blocking failure in primary injection routine", errWrap, log.GenerateRequestID())
		return errWrap
	}

	channelName := channelID
	if ch, err := session.Channel(channelID); err == nil {
		channelName = ch.Name
	}
	slog.Info("Architectural state transition: Guild registration completed and coupled to serial port",
		slog.String("guildName", guild.Name),
		slog.String("guildID", guildID),
		slog.String("channel", channelName),
	)
	return nil
}

// --- Utility & Logging ---

// ShowConfiguredGuilds emits summary logs of the indexed instances.
func ShowConfiguredGuilds(s *discordgo.Session, configManager *ConfigManager) {
	configuration := configManager.Config()
	if configuration == nil || len(configuration.Guilds) == 0 {
		return
	}
	for _, guildConfig := range configuration.Guilds {
		if guild, err := s.Guild(guildConfig.GuildID); err == nil {
			slog.Info("Compliant procedure: Active monitoring on guild telemetry channel",
				slog.String("guildName", guild.Name),
				slog.String("guildID", guild.ID),
			)
		} else {
			slog.Warn("Obstruction in communication network: Registered guild inaccessible to telemetry inspection",
				slog.String("guildID", guildConfig.GuildID),
			)
		}
	}
}

// FindSuitableChannel searches for the suitable primary channel for pipe allocation.
func FindSuitableChannel(session *discordgo.Session, guildID string) string {
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

// FindEntryLeaveChannel searches for the primary channel for logging user I/O events.
func FindEntryLeaveChannel(session *discordgo.Session, guildID string) string {
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

// HasSendPermission validates authorization vectors against the target bitmask.
func HasSendPermission(session *discordgo.Session, channelID string) bool {
	if session == nil || session.State == nil || session.State.User == nil || channelID == "" {
		return false
	}
	if perms, err := session.UserChannelPermissions(session.State.User.ID, channelID); err == nil {
		return (perms & discordgo.PermissionSendMessages) != 0
	}
	return false
}

// FindAdminRoles extracts roles containing the administrator bitmask from the vector.
func FindAdminRoles(session *discordgo.Session, guildID string) []string {
	var allowedRoles []string
	roles, err := session.GuildRoles(guildID)
	if err == nil {
		for _, role := range roles {
			if role.Name != "@everyone" && (role.Permissions&discordgo.PermissionAdministrator) != 0 {
				allowedRoles = append(allowedRoles, role.ID)
			}
		}
	}
	return allowedRoles
}

// TextChannels converts and extracts channels suitable for text transmission from the multiplexer.
func TextChannels(session *discordgo.Session, guildID string) ([]*discordgo.Channel, error) {
	if session == nil || session.State == nil || session.State.User == nil {
		return nil, fmt.Errorf("session not initialized")
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("TextChannels: %w", err)
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

// ValidateChannel validates node properties, hierarchical structure, and constraint integrity.
func ValidateChannel(session *discordgo.Session, guildID, channelID string) error {
	if session == nil || session.State == nil || session.State.User == nil {
		return errors.New("session not initialized")
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

// LogConfiguredGuilds logs a summary of the mapped node tree.
func LogConfiguredGuilds(configManager *ConfigManager, session *discordgo.Session) error {
	return LogConfiguredGuildsForBot(configManager, session, "")
}

// LogConfiguredGuildsForBot summarizes the mapped subset designated for routing of explicit bot instance.
func LogConfiguredGuildsForBot(configManager *ConfigManager, session *discordgo.Session, botInstanceID string) error {
	return logConfiguredGuildSubset(configManager, session, func(cfg *BotConfig) []GuildConfig {
		guilds := cfg.Guilds
		if normalizedBotInstanceID := NormalizeBotInstanceID(botInstanceID); normalizedBotInstanceID != "" {
			guilds = cfg.GuildsForBotInstance(normalizedBotInstanceID)
		}
		return guilds
	})
}

func logConfiguredGuildSubset(configManager *ConfigManager, session *discordgo.Session, resolve func(*BotConfig) []GuildConfig) error {
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		slog.Warn("Basal threshold reached: Empty guild allocation vector in boot routine")
		return nil
	}

	guilds := cfg.Guilds
	if resolve != nil {
		guilds = resolve(cfg)
	}
	if len(guilds) == 0 {
		slog.Warn("Basal threshold reached: Empty guild allocation vector in boot routine")
		return nil
	}

	slog.Info("Load summary initialized",
		slog.Int("guilds_count", len(guilds)),
	)

	var errCount int
	for _, g := range guilds {
		guild, err := session.Guild(g.GuildID)
		if err == nil {
			slog.Info("Active interface confirmed",
				slog.String("guildName", guild.Name),
				slog.String("guildID", guild.ID),
			)
		} else {
			slog.Warn("Handshake failure with guild interface reported by central hub",
				slog.String("guildID", g.GuildID),
			)
			errCount++
		}
	}
	return nil
}
