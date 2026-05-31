package core

import (
	"fmt"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// ConfigPersister manages configuration persistence
type ConfigPersister struct {
	configManager *files.ConfigManager
}

// NewConfigPersister creates a new configuration persister
func NewConfigPersister(cm *files.ConfigManager) *ConfigPersister {
	return &ConfigPersister{configManager: cm}
}

// Save saves the guild configuration
func (cp *ConfigPersister) Save(config *files.GuildConfig) error {
	if err := cp.configManager.AddGuildConfig(*config); err != nil {
		return fmt.Errorf("failed to update config in memory: %w", err)
	}
	if err := cp.configManager.SaveConfig(); err != nil {
		return fmt.Errorf("failed to persist config: %w", err)
	}
	return nil
}

// SaveWithBackup saves the configuration with a backup
func (cp *ConfigPersister) SaveWithBackup(config *files.GuildConfig) error {
	// Implement backup if needed
	return cp.Save(config)
}

// EnsureGuildConfig ensures there is a configuration for the server
func EnsureGuildConfig(configManager *files.ConfigManager, guildID string) *files.GuildConfig {
	config := configManager.GuildConfig(guildID)
	if config == nil {
		config = &files.GuildConfig{
			GuildID: guildID,
			Roles:   files.RolesConfig{Allowed: []string{}},
		}
	}
	return config
}
