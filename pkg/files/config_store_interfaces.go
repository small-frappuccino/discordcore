package files

import (
	"context"
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

// ConfigSnapshot guarantees an O(1) read-only memory projection to prevent cross-goroutine write-panics.
type ConfigSnapshot interface {
	GuildID() string
	// TODO: legacy getters
}

// ConfigObserver dictates strict context preemption for reactive configuration sinks.
type ConfigObserver func(ctx context.Context, snapshot ConfigSnapshot)

// ConfigRegistry enforces Pub/Sub mapping. Implementations must utilize sync.RWMutex.
type ConfigRegistry interface {
	SubscribeToGuildChanges(guildID string, observer ConfigObserver)
}

// ConfigMutator encapsulates database commits and triggers asynchronous fan-out.
type ConfigMutator interface {
	Mutate(ctx context.Context, guildID string, mutationFn func() error) error
}

// Store is a topological aggregator embedding registry, mutator, and legacy interfaces.
type Store interface {
	ConfigLoader
	ConfigSaver
	ConfigDescriber
	ConfigRegistry
	ConfigMutator
}
