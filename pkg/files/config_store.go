package files

// ConfigStore persists the canonical BotConfig.
//
// Missing state is normalized as an empty config instead of an error so the
// ConfigManager can keep one consistent lifecycle across backends.
type ConfigStore interface {
	Load() (*BotConfig, error)
	Save(*BotConfig) error
	Exists() (bool, error)
	Describe() string
}

const DefaultPostgresConfigStoreKey = "primary"
