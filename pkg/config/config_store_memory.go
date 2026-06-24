package config

import (
	"fmt"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const defaultMemoryConfigStoreDescription = "memory://bot_config_state"

// MemoryConfigStore persists files.BotConfig in memory.
// It is primarily intended for tests and lightweight local workflows that do
// not need cross-process persistence.
type MemoryConfigStore struct {
	mu          sync.Mutex
	config      *files.BotConfig
	exists      bool
	description string
}

// Load loads.
func (s *MemoryConfigStore) Load() (*files.BotConfig, error) {
	cfg := &files.BotConfig{Guilds: []files.GuildConfig{}}
	if s == nil {
		return cfg, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.config == nil {
		return cfg, nil
	}

	out := files.CloneBotConfigPtr(s.config)
	if out == nil {
		return cfg, nil
	}
	if out.Guilds == nil {
		out.Guilds = []files.GuildConfig{}
	}
	return out, nil
}

// Save saves.
func (s *MemoryConfigStore) Save(cfg *files.BotConfig) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil config")
	}
	if s == nil {
		return fmt.Errorf("memory config store is not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = files.CloneBotConfigPtr(cfg)
	if s.config == nil {
		s.config = &files.BotConfig{Guilds: []files.GuildConfig{}}
	}
	if s.config.Guilds == nil {
		s.config.Guilds = []files.GuildConfig{}
	}
	s.exists = true
	return nil
}

// Exists exists.
func (s *MemoryConfigStore) Exists() (bool, error) {
	if s == nil {
		return false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exists, nil
}

// Describe describes.
func (s *MemoryConfigStore) Describe() string {
	if s == nil || s.description == "" {
		return defaultMemoryConfigStoreDescription
	}
	return s.description
}
