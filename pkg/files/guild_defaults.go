package files

import (
	"fmt"
	"strings"
)

// NewMinimalGuildConfig returns a dormant guild record for automatic discovery.
// Newly listed guilds keep all feature overrides explicitly disabled until an
// operator configures them.
func NewMinimalGuildConfig(guildID, botInstanceID string) GuildConfig {
	disabled := false

	return GuildConfig{
		GuildID:       strings.TrimSpace(guildID),
		BotInstanceID: NormalizeBotInstanceID(botInstanceID),
		Features: FeatureToggles{
			Services: FeatureServiceToggles{
				Monitoring:    boolPtr(disabled),
				Automod:       boolPtr(disabled),
				Commands:      boolPtr(disabled),
				AdminCommands: boolPtr(disabled),
			},
			Logging: FeatureLoggingToggles{
				AvatarLogging:  boolPtr(disabled),
				RoleUpdate:     boolPtr(disabled),
				MemberJoin:     boolPtr(disabled),
				MemberLeave:    boolPtr(disabled),
				MessageProcess: boolPtr(disabled),
				MessageEdit:    boolPtr(disabled),
				MessageDelete:  boolPtr(disabled),
				ReactionMetric: boolPtr(disabled),
				AutomodAction:  boolPtr(disabled),
				ModerationCase: boolPtr(disabled),
				CleanAction:    boolPtr(disabled),
			},
			MessageCache: FeatureMessageCacheToggles{
				CleanupOnStartup: boolPtr(disabled),
				DeleteOnLog:      boolPtr(disabled),
			},
			PresenceWatch: FeaturePresenceWatchToggles{
				Bot:  boolPtr(disabled),
				User: boolPtr(disabled),
			},
			Maintenance: FeatureMaintenanceToggles{
				DBCleanup: boolPtr(disabled),
			},
			Safety: FeatureSafetyToggles{
				BotRolePermMirror: boolPtr(disabled),
			},
			Backfill: FeatureBackfillToggles{
				Enabled: boolPtr(disabled),
			},
			StatsChannels:  boolPtr(disabled),
			AutoRoleAssign: boolPtr(disabled),
			UserPrune:      boolPtr(disabled),
		},
	}
}

// EnsureMinimalGuildConfigForBot persists a dormant guild record if it does not
// exist yet. Existing guild settings are preserved.
func (mgr *ConfigManager) EnsureMinimalGuildConfigForBot(guildID, botInstanceID string) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return fmt.Errorf("guild id is required")
	}
	botInstanceID = NormalizeBotInstanceID(botInstanceID)

	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}
			if NormalizeBotInstanceID(cfg.Guilds[idx].BotInstanceID) == "" && botInstanceID != "" {
				cfg.Guilds[idx].BotInstanceID = botInstanceID
			}
			return nil
		}

		cfg.Guilds = append(cfg.Guilds, NewMinimalGuildConfig(guildID, botInstanceID))
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensure minimal guild config for %s: %w", guildID, err)
	}
	return nil
}
