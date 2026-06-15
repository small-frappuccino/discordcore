package files

import (
	"fmt"
	"strings"
)

// NewMinimalGuildConfig returns a dormant guild record for automatic discovery.
// Newly listed guilds keep all feature overrides explicitly disabled until an
// operator configures them.
func NewMinimalGuildConfig(guildID string) GuildConfig {
	disabled := false

	features := FeatureToggles{}
	for _, spec := range featureRegistry {
		// Do not forcefully disable the core command router. If we disable it, the bot
		// strips its own command list out of Discord entirely when joining a new guild.
		if spec.ID == "services.commands" {
			continue
		}
		features.SetToggle(spec.ID, boolPtr(disabled))
	}

	return GuildConfig{
		GuildID:  strings.TrimSpace(guildID),
		Features: features,
	}
}

// EnsureMinimalGuildConfigForBot persists a dormant guild record if it does not
// exist yet. Existing guild settings are preserved.
func (mgr *ConfigManager) EnsureMinimalGuildConfig(guildID string) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return fmt.Errorf("guild id is required")
	}

	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}
			return nil
		}

		cfg.Guilds = append(cfg.Guilds, NewMinimalGuildConfig(guildID))
		return nil
	})
	if err != nil {
		return fmt.Errorf("ensure minimal guild config for %s: %w", guildID, err)
	}
	return nil
}
