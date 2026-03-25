package files

import (
	"encoding/json"
	"fmt"
)

func (mgr *ConfigManager) currentPublishedSnapshot() *publishedConfigSnapshot {
	if mgr == nil {
		return nil
	}
	if snap := mgr.published.Load(); snap != nil {
		return snap
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.publishSnapshotLocked()
}

func (mgr *ConfigManager) publishSnapshotLocked() *publishedConfigSnapshot {
	if mgr == nil || mgr.config == nil {
		if mgr != nil {
			mgr.published.Store(nil)
		}
		return nil
	}

	snap := &publishedConfigSnapshot{
		config:     cloneBotConfigPtr(mgr.config),
		guildIndex: cloneGuildIndex(mgr.guildIndex),
	}
	if snap.guildIndex == nil {
		snap.guildIndex = buildReadonlyGuildIndex(snap.config)
	}
	mgr.published.Store(snap)
	return snap
}

func cloneGuildIndex(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for guildID, idx := range in {
		out[guildID] = idx
	}
	return out
}

func buildReadonlyGuildIndex(cfg *BotConfig) map[string]int {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}
	index := make(map[string]int, len(cfg.Guilds))
	for i := range cfg.Guilds {
		guildID := cfg.Guilds[i].GuildID
		if guildID == "" {
			continue
		}
		if _, exists := index[guildID]; exists {
			continue
		}
		index[guildID] = i
	}
	return index
}

// SnapshotConfig returns a deep copy of the current bot config for read-only use
// outside the ConfigManager lock.
func (mgr *ConfigManager) SnapshotConfig() BotConfig {
	snap := mgr.currentPublishedSnapshot()
	if snap == nil || snap.config == nil {
		return BotConfig{Guilds: []GuildConfig{}}
	}

	out := cloneBotConfig(*snap.config)
	if out.Guilds == nil {
		out.Guilds = []GuildConfig{}
	}
	return out
}

// UpdateConfig applies a full-config mutation transactionally and persists the
// result. On error, in-memory state is restored to the previous snapshot.
func (mgr *ConfigManager) UpdateConfig(fn func(*BotConfig) error) (BotConfig, error) {
	if mgr == nil {
		return BotConfig{}, fmt.Errorf("config manager is nil")
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		mgr.config = &BotConfig{Guilds: []GuildConfig{}}
	}

	previous := mgr.config
	previousIndex := cloneGuildIndex(mgr.guildIndex)
	next := cloneBotConfigPtr(mgr.config)

	if fn != nil {
		if err := fn(next); err != nil {
			return BotConfig{}, err
		}
	}

	mgr.config = next
	if _, err := mgr.rebuildGuildIndexLocked("update"); err != nil {
		// rebuildGuildIndexLocked already normalizes duplicate guild IDs in memory
		// and emits context-rich logs. The updated config remains canonical.
	}

	if err := mgr.saveConfigLocked(); err != nil {
		mgr.config = previous
		mgr.guildIndex = previousIndex
		mgr.publishSnapshotLocked()
		return BotConfig{}, err
	}

	snapshot := mgr.publishSnapshotLocked()
	if snapshot == nil || snapshot.config == nil {
		return BotConfig{Guilds: []GuildConfig{}}, nil
	}
	return cloneBotConfig(*snapshot.config), nil
}

func cloneBotConfigPtr(in *BotConfig) *BotConfig {
	if in == nil {
		return nil
	}
	out := cloneBotConfig(*in)
	return &out
}

func cloneGuildConfigPtr(in *GuildConfig) *GuildConfig {
	if in == nil {
		return nil
	}
	out := cloneGuildConfig(*in)
	return &out
}

func cloneBotConfig(in BotConfig) BotConfig {
	return BotConfig{
		Guilds:        cloneGuildConfigs(in.Guilds),
		Features:      cloneFeatureToggles(in.Features),
		RuntimeConfig: cloneRuntimeConfig(in.RuntimeConfig),
	}
}

func cloneGuildConfigs(in []GuildConfig) []GuildConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]GuildConfig, 0, len(in))
	for _, guild := range in {
		out = append(out, cloneGuildConfig(guild))
	}
	return out
}

func cloneGuildConfig(in GuildConfig) GuildConfig {
	return GuildConfig{
		GuildID:         in.GuildID,
		BotInstanceID:   in.BotInstanceID,
		Features:        cloneFeatureToggles(in.Features),
		Channels:        in.Channels,
		Roles:           cloneRolesConfig(in.Roles),
		Stats:           cloneStatsConfig(in.Stats),
		RolesCacheTTL:   in.RolesCacheTTL,
		MemberCacheTTL:  in.MemberCacheTTL,
		GuildCacheTTL:   in.GuildCacheTTL,
		ChannelCacheTTL: in.ChannelCacheTTL,
		UserPrune:       cloneUserPruneConfig(in.UserPrune),
		PartnerBoard:    clonePartnerBoardConfig(in.PartnerBoard),
		RuntimeConfig:   cloneRuntimeConfig(in.RuntimeConfig),
	}
}

func cloneRuntimeConfig(in RuntimeConfig) RuntimeConfig {
	return RuntimeConfig{
		Database:                     in.Database,
		BotTheme:                     in.BotTheme,
		DisableDBCleanup:             in.DisableDBCleanup,
		DisableAutomodLogs:           in.DisableAutomodLogs,
		DisableMessageLogs:           in.DisableMessageLogs,
		DisableEntryExitLogs:         in.DisableEntryExitLogs,
		DisableReactionLogs:          in.DisableReactionLogs,
		DisableUserLogs:              in.DisableUserLogs,
		DisableCleanLog:              in.DisableCleanLog,
		ModerationLogging:            cloneBoolPtr(in.ModerationLogging),
		ModerationLogMode:            in.ModerationLogMode,
		PresenceWatchUserID:          in.PresenceWatchUserID,
		PresenceWatchBot:             in.PresenceWatchBot,
		MessageCacheTTLHours:         in.MessageCacheTTLHours,
		MessageDeleteOnLog:           in.MessageDeleteOnLog,
		MessageCacheCleanup:          in.MessageCacheCleanup,
		GlobalMaxWorkers:             in.GlobalMaxWorkers,
		BackfillChannelID:            in.BackfillChannelID,
		BackfillStartDay:             in.BackfillStartDay,
		BackfillInitialDate:          in.BackfillInitialDate,
		DisableBotRolePermMirror:     in.DisableBotRolePermMirror,
		BotRolePermMirrorActorRoleID: in.BotRolePermMirrorActorRoleID,
		WebhookEmbedUpdates:          cloneWebhookEmbedUpdateList(in.WebhookEmbedUpdates),
		WebhookEmbedUpdate:           cloneWebhookEmbedUpdateConfig(in.WebhookEmbedUpdate),
		WebhookEmbedValidation:       in.WebhookEmbedValidation,
	}
}

func cloneFeatureToggles(in FeatureToggles) FeatureToggles {
	return FeatureToggles{
		Services: FeatureServiceToggles{
			Monitoring:    cloneBoolPtr(in.Services.Monitoring),
			Automod:       cloneBoolPtr(in.Services.Automod),
			Commands:      cloneBoolPtr(in.Services.Commands),
			AdminCommands: cloneBoolPtr(in.Services.AdminCommands),
		},
		Logging: FeatureLoggingToggles{
			AvatarLogging:  cloneBoolPtr(in.Logging.AvatarLogging),
			RoleUpdate:     cloneBoolPtr(in.Logging.RoleUpdate),
			MemberJoin:     cloneBoolPtr(in.Logging.MemberJoin),
			MemberLeave:    cloneBoolPtr(in.Logging.MemberLeave),
			MessageProcess: cloneBoolPtr(in.Logging.MessageProcess),
			MessageEdit:    cloneBoolPtr(in.Logging.MessageEdit),
			MessageDelete:  cloneBoolPtr(in.Logging.MessageDelete),
			ReactionMetric: cloneBoolPtr(in.Logging.ReactionMetric),
			AutomodAction:  cloneBoolPtr(in.Logging.AutomodAction),
			ModerationCase: cloneBoolPtr(in.Logging.ModerationCase),
			CleanAction:    cloneBoolPtr(in.Logging.CleanAction),
		},
		MessageCache: FeatureMessageCacheToggles{
			CleanupOnStartup: cloneBoolPtr(in.MessageCache.CleanupOnStartup),
			DeleteOnLog:      cloneBoolPtr(in.MessageCache.DeleteOnLog),
		},
		PresenceWatch: FeaturePresenceWatchToggles{
			Bot:  cloneBoolPtr(in.PresenceWatch.Bot),
			User: cloneBoolPtr(in.PresenceWatch.User),
		},
		Maintenance: FeatureMaintenanceToggles{
			DBCleanup: cloneBoolPtr(in.Maintenance.DBCleanup),
		},
		Safety: FeatureSafetyToggles{
			BotRolePermMirror: cloneBoolPtr(in.Safety.BotRolePermMirror),
		},
		Backfill: FeatureBackfillToggles{
			Enabled: cloneBoolPtr(in.Backfill.Enabled),
		},
		MuteRole:       cloneBoolPtr(in.MuteRole),
		StatsChannels:  cloneBoolPtr(in.StatsChannels),
		AutoRoleAssign: cloneBoolPtr(in.AutoRoleAssign),
		UserPrune:      cloneBoolPtr(in.UserPrune),
	}
}

func cloneRolesConfig(in RolesConfig) RolesConfig {
	return RolesConfig{
		Allowed:          cloneStringSlice(in.Allowed),
		AutoAssignment:   cloneAutoAssignmentConfig(in.AutoAssignment),
		VerificationRole: in.VerificationRole,
		BoosterRole:      in.BoosterRole,
		MuteRole:         in.MuteRole,
	}
}

func cloneAutoAssignmentConfig(in AutoAssignmentConfig) AutoAssignmentConfig {
	return AutoAssignmentConfig{
		Enabled:       in.Enabled,
		TargetRoleID:  in.TargetRoleID,
		RequiredRoles: cloneStringSlice(in.RequiredRoles),
	}
}

func cloneStatsConfig(in StatsConfig) StatsConfig {
	return StatsConfig{
		Enabled:            in.Enabled,
		UpdateIntervalMins: in.UpdateIntervalMins,
		Channels:           cloneStatsChannelConfigs(in.Channels),
	}
}

func cloneStatsChannelConfigs(in []StatsChannelConfig) []StatsChannelConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]StatsChannelConfig, len(in))
	copy(out, in)
	return out
}

func cloneUserPruneConfig(in UserPruneConfig) UserPruneConfig {
	return UserPruneConfig{
		Enabled:          in.Enabled,
		GraceDays:        in.GraceDays,
		ScanIntervalMins: in.ScanIntervalMins,
		InitialDelaySecs: in.InitialDelaySecs,
		KicksPerSecond:   in.KicksPerSecond,
		MaxKicksPerRun:   in.MaxKicksPerRun,
		ExemptRoleIDs:    cloneStringSlice(in.ExemptRoleIDs),
		DryRun:           in.DryRun,
	}
}

func clonePartnerBoardConfig(in PartnerBoardConfig) PartnerBoardConfig {
	return PartnerBoardConfig{
		Target:   in.Target,
		Template: in.Template,
		Partners: clonePartnerEntries(in.Partners),
	}
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneJSONRawMessage(in json.RawMessage) json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(in))
	copy(out, in)
	return out
}
