package files

// ConfigStore persists the canonical BotConfig.
//
// Missing state is normalized as an empty config instead of an error so the
// ConfigManager can keep one consistent lifecycle across backends.
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

// DefaultPostgresConfigStoreKey defines default postgres config store key.
const DefaultPostgresConfigStoreKey = "primary"
