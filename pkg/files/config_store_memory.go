package files

import (
	"fmt"
	"sync"
)

const defaultMemoryConfigStoreDescription = "memory://bot_config_state"

// MemoryConfigStore persists BotConfig in memory.
// It is primarily intended for tests and lightweight local workflows that do
// not need cross-process persistence.
type MemoryConfigStore struct {
	mu          sync.RWMutex
	config      *BotConfig
	exists      bool
	description string
}

func NewMemoryConfigStore() *MemoryConfigStore {
	return &MemoryConfigStore{
		description: defaultMemoryConfigStoreDescription,
	}
}

// NewMemoryConfigManager creates a ConfigManager backed by an in-memory store.
func NewMemoryConfigManager() *ConfigManager {
	return NewConfigManagerWithStore(NewMemoryConfigStore())
}

func (s *MemoryConfigStore) Load() (*BotConfig, error) {
	cfg := &BotConfig{Guilds: []GuildConfig{}}
	if s == nil {
		return cfg, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.config == nil {
		return cfg, nil
	}

	out := cloneBotConfigPtr(s.config)
	if out == nil {
		return cfg, nil
	}
	if out.Guilds == nil {
		out.Guilds = []GuildConfig{}
	}
	return out, nil
}

func (s *MemoryConfigStore) Save(cfg *BotConfig) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil config")
	}
	if s == nil {
		return fmt.Errorf("memory config store is not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = cloneBotConfigPtr(cfg)
	if s.config == nil {
		s.config = &BotConfig{Guilds: []GuildConfig{}}
	}
	if s.config.Guilds == nil {
		s.config.Guilds = []GuildConfig{}
	}
	s.exists = true
	return nil
}

func (s *MemoryConfigStore) Exists() (bool, error) {
	if s == nil {
		return false, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.exists, nil
}

func (s *MemoryConfigStore) Describe() string {
	if s == nil || s.description == "" {
		return defaultMemoryConfigStoreDescription
	}
	return s.description
}
