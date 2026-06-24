package config

import (
	"context"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Provider defines the interface for reading configuration states.
type Provider interface {
	Config() *files.BotConfig
	GuildConfig(guildID string) *files.GuildConfig
	UpdateConfig(ctx context.Context, fn func(*files.BotConfig) error) (files.BotConfig, error)
	LoadConfig() error
	UpdateRuntimeConfig(fn func(*files.RuntimeConfig) error) (files.RuntimeConfig, error)
	UpdateGuildConfig(guildID string, fn func(*files.GuildConfig) error) error
	RolePanels(guildID string) ([]files.RolePanelConfig, error)
	RolePanel(guildID, key string) (files.RolePanelConfig, error)
	SetRolePanelEmbed(guildID, key string, embed files.RolePanelConfig) error
	AddRolePanelField(guildID, key string, field files.RolePanelEmbedFieldConfig) error
	RemoveRolePanelField(guildID, key string, fieldIndex int) error
	UpsertRolePanelButton(guildID, key string, button files.RolePanelButtonConfig) error
	DeleteRolePanelButton(guildID, key, roleID string) error
	DeleteRolePanel(guildID, key string) error
	ListRolePanelPostings(guildID, key string) ([]files.RolePanelPostingConfig, error)
	AddRolePanelPosting(guildID, key string, posting files.RolePanelPostingConfig) error
	RemoveRolePanelPosting(guildID, key, messageID string) error
	RemoveRolePanelPostings(guildID, key string, messageIDs []string) error
	ClearRolePanelPostings(guildID, key string) error
	FindRolePanelPosting(guildID, messageID string) (string, files.RolePanelPostingConfig, error)
	RolePanelButtonByRoleID(guildID, roleID string) (files.RolePanelConfig, files.RolePanelButtonConfig, error)
}

// Loader defines the read paths for the bot configuration.
type Loader interface {
	Load() (*files.BotConfig, error)
	Exists() (bool, error)
}

// Saver defines the write path for the bot configuration.
type Saver interface {
	Save(*files.BotConfig) error
}

// Store persists the canonical BotConfig by combining read, write capabilities.
type Store interface {
	Loader
	Saver
}
