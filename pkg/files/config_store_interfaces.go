package files

import (
	"context"
	"sync"
)

// ConfigLoader defines the read paths for the bot configuration.
type ConfigLoader interface {
	Load() (*BotConfig, error)
	Exists() (bool, error)
}

// ConfigSaver defines the write path for the bot configuration.
type ConfigSaver interface {
	Save(*BotConfig) error
}

// ConfigDescriber provides human-readable context for the config storage mechanism.
type ConfigDescriber interface {
	Describe() string
}

// ConfigStore persists the canonical BotConfig by combining read, write, and descriptor capabilities.
type ConfigStore interface {
	ConfigLoader
	ConfigSaver
	ConfigDescriber
}

// ConfigMutationSubscriber defines the receiver for immutable configuration updates.
type ConfigMutationSubscriber interface {
	OnConfigMutated(ctx context.Context, snapshot ConfigSnapshot)
}

// ConfigObservable defines the mechanism for features to register for reactive updates.
type ConfigObservable interface {
	Subscribe(id string, sub ConfigMutationSubscriber)
}

// FeatureRegistry provides a thread-safe subscriber map for configuration mutations.
// It uses a sync.RWMutex to protect the internal subscriber map from race conditions
// when new features boot up and subscribe.
type FeatureRegistry struct {
	mu          sync.RWMutex
	subscribers map[string]ConfigMutationSubscriber
}

// NewFeatureRegistry initializes an empty thread-safe subscriber registry.
func NewFeatureRegistry() *FeatureRegistry {
	return &FeatureRegistry{
		subscribers: make(map[string]ConfigMutationSubscriber),
	}
}

// Subscribe registers a new listener into the thread-safe map.
func (r *FeatureRegistry) Subscribe(id string, sub ConfigMutationSubscriber) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.subscribers == nil {
		r.subscribers = make(map[string]ConfigMutationSubscriber)
	}
	r.subscribers[id] = sub
}

// Subscribers returns a shallow copy of the current listeners for safe iteration.
func (r *FeatureRegistry) Subscribers() map[string]ConfigMutationSubscriber {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[string]ConfigMutationSubscriber, len(r.subscribers))
	for k, v := range r.subscribers {
		out[k] = v
	}
	return out
}
