package files

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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

// ConfigEvent contains the GuildID and the new/mutated configuration state.
type ConfigEvent struct {
	GuildID string
	State   ConfigSnapshot
}

// ConfigEventObserver defines the callback for configuration mutations.
type ConfigEventObserver func(ctx context.Context, event ConfigEvent)

// EventBus implements a thread-safe Pub/Sub observer pattern for configuration mutations.
// It leverages atomic.Pointer to guarantee a zero-allocation, wait-free read path on the critical hot path of event dispatch.
type EventBus struct {
	subscribers atomic.Pointer[map[string][]ConfigEventObserver]
	mu          sync.Mutex // Serializes subscription mutations
}

// NewEventBus creates and initializes a new EventBus.
func NewEventBus() *EventBus {
	eb := &EventBus{}
	initial := make(map[string][]ConfigEventObserver)
	eb.subscribers.Store(&initial)
	return eb
}

// Subscribe registers an observer for a specific guild's configuration mutations.
// It uses copy-on-write semantics to avoid blocking readers during updates.
func (eb *EventBus) Subscribe(guildID string, observer ConfigEventObserver) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	oldMap := *eb.subscribers.Load()
	newMap := make(map[string][]ConfigEventObserver, len(oldMap)+1)

	for k, v := range oldMap {
		// Allocate a new slice for the existing observers to avoid cross-contamination
		// when mutating the list of observers for a guild.
		newSlice := make([]ConfigEventObserver, len(v))
		copy(newSlice, v)
		newMap[k] = newSlice
	}

	newMap[guildID] = append(newMap[guildID], observer)
	eb.subscribers.Store(&newMap)
}

// Publish broadcasts a ConfigEvent to all registered subscribers for the guild.
// PERFORMANCE INVARIANT: Wait-free execution. No locks acquired. Zero allocations.
func (eb *EventBus) Publish(ctx context.Context, event ConfigEvent) {
	subsMap := *eb.subscribers.Load()
	if observers, ok := subsMap[event.GuildID]; ok {
		for i := 0; i < len(observers); i++ {
			// Executed synchronously per subscriber to prevent unbounded goroutine spawning.
			observers[i](ctx, event)
		}
	}
}
