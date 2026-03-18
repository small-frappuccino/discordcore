package files

import "fmt"

func (mgr *ConfigManager) updateGuildConfig(guildID string, fn func(*GuildConfig) error) error {
	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		guildConfig, err := guildConfigByID(cfg, guildID)
		if err != nil {
			return err
		}
		if fn == nil {
			return nil
		}
		return fn(guildConfig)
	})
	return err
}

func (mgr *ConfigManager) updateRuntimeConfigScope(scopeGuildID string, fn func(*RuntimeConfig) error) error {
	_, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		runtimeConfig, err := runtimeConfigForScope(cfg, scopeGuildID)
		if err != nil {
			return err
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

	guildConfig, err := guildConfigByID(cfg, scopeGuildID)
	if err != nil {
		return nil, fmt.Errorf("guild config not found for %s", scopeGuildID)
	}
	return &guildConfig.RuntimeConfig, nil
}
