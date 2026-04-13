package files

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrInvalidQOTDInput indicates invalid QOTD configuration payloads.
	ErrInvalidQOTDInput = errors.New("invalid qotd input")
)

// IsZero reports whether all QOTD fields are unset.
func (cfg QOTDConfig) IsZero() bool {
	return !cfg.Enabled &&
		strings.TrimSpace(cfg.QuestionChannelID) == "" &&
		strings.TrimSpace(cfg.ResponseChannelID) == ""
}

// GetQOTDConfig returns the canonical QOTD config for one guild.
func (mgr *ConfigManager) GetQOTDConfig(guildID string) (QOTDConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return QOTDConfig{}, fmt.Errorf("get qotd config: %w", invalidQOTDInput("guild_id is required"))
	}

	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	guildConfig, err := mgr.guildConfigByIDLocked(scope)
	if err != nil {
		return QOTDConfig{}, err
	}

	normalized, err := NormalizeQOTDConfig(guildConfig.QOTD)
	if err != nil {
		return QOTDConfig{}, fmt.Errorf("get qotd config: %w", err)
	}
	return normalized, nil
}

// SetQOTDConfig validates and persists the QOTD config for one guild.
func (mgr *ConfigManager) SetQOTDConfig(guildID string, cfg QOTDConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("set qotd config: %w", invalidQOTDInput("guild_id is required"))
	}

	normalized, err := NormalizeQOTDConfig(cfg)
	if err != nil {
		return fmt.Errorf("set qotd config: %w", err)
	}
	if err := mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		guildConfig.QOTD = normalized
		return nil
	}); err != nil {
		return fmt.Errorf("set qotd config: %w", err)
	}
	return nil
}

func invalidQOTDInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidQOTDInput, fmt.Sprintf(format, args...))
}
