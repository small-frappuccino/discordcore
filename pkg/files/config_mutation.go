package files

import (
	"context"
	"fmt"
)

func (mgr *ConfigManager) updateGuildConfig(guildID string, fn func(*GuildConfig) error) error {
	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		guildConfig, err := GuildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.updateGuildConfig: %w", err)
		}
		if fn == nil {
			return nil
		}
		return fn(guildConfig)
	})

	return err
}

// UpdateGuildConfig provides an exported way to modify a guild's config
func (mgr *ConfigManager) UpdateGuildConfig(guildID string, fn func(*GuildConfig) error) error {
	return mgr.updateGuildConfig(guildID, fn)
}

func (mgr *ConfigManager) updateRuntimeConfigScope(scopeGuildID string, fn func(*RuntimeConfig) error) error {
	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		runtimeConfig, err := runtimeConfigForScope(cfg, scopeGuildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.updateRuntimeConfigScope: %w", err)
		}
		if runtimeConfig == nil || fn == nil {
			return nil
		}
		return fn(runtimeConfig)
	})

	return err
}

func runtimeConfigForScope(cfg *BotConfig, scopeGuildID string) (*RuntimeConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	if scopeGuildID == "" {
		return &cfg.RuntimeConfig, nil
	}

	guildConfig, err := GuildConfigByID(cfg, scopeGuildID)
	if err != nil {
		return nil, fmt.Errorf("guild config not found for %s", scopeGuildID)
	}
	return &guildConfig.RuntimeConfig, nil
}

// RevokeBotInstance removes the given instance from the configuration across all guilds,
// provided that its configured token exactly matches the revoked token.
func (mgr *ConfigManager) RevokeBotInstance(instanceID, token string) error {
	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		for i := range cfg.Guilds {
			guild := &cfg.Guilds[i]
			encToken, exists := guild.BotInstanceTokens[instanceID]
			if !exists {
				continue
			}
			if string(encToken) != token {
				continue
			}

			delete(guild.BotInstanceTokens, instanceID)

			if guild.BotInstanceStatuses != nil {
				delete(guild.BotInstanceStatuses, instanceID)
			}

			if guild.FeatureRouting != nil {
				for feature, route := range guild.FeatureRouting {
					if route == instanceID {
						delete(guild.FeatureRouting, feature)
					}
				}
			}
		}
		return nil
	})

	return err
}
