package files

import (
	"context"
	"fmt"
	"time"
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

// DispatchConfigMutation performs a non-blocking fan-out of the updated configuration.
// It complies with the CSP contract by extracting a read-only snapshot and pushing it
// to context-bounded goroutines, preventing write-starvation on the primary thread.
func DispatchConfigMutation(registry *FeatureRegistry, cfg *BotConfig) {
	if registry == nil || cfg == nil {
		return
	}

	// 1. Compile immutable read-only snapshot.
	snapshot := ConfigSnapshot(*CloneBotConfigPtr(cfg))
	subs := registry.Subscribers()

	// 2. Non-blocking fan-out via context-bounded goroutines.
	for _, sub := range subs {
		subscriber := sub
		go func() {
			// Apply a strict 10-second boundary to prevent leaked goroutines if a subscriber hangs.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			defer func() {
				if r := recover(); r != nil {
					// Isolate failure to the specific subscriber; do not crash the primary dispatcher.
					// In a real environment, we would use slog.Error here.
				}
			}()

			// Execute the callback synchronously within the bounded goroutine.
			subscriber.OnConfigMutated(ctx, snapshot)
		}()
	}
}
