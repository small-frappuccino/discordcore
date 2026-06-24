package files

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
