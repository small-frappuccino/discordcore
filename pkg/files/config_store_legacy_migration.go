package files

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

type LegacyConfigMigrationResult struct {
	Migrated         bool
	RemovedFile      bool
	RemovedParentDir bool
}

// MigrateLegacyJSONConfigToStore copies the legacy settings.json payload into the
// active config store on first boot, then removes the old file and its parent
// directory when they are no longer needed.
func MigrateLegacyJSONConfigToStore(settingsPath string, store ConfigStore) (LegacyConfigMigrationResult, error) {
	result := LegacyConfigMigrationResult{}
	settingsPath = strings.TrimSpace(settingsPath)
	if settingsPath == "" || store == nil {
		return result, nil
	}

	legacyStore := NewJSONConfigStore(settingsPath)
	legacyExists, err := legacyStore.Exists()
	if err != nil {
		return result, err
	}
	if !legacyExists {
		return result, nil
	}

	storeExists, err := store.Exists()
	if err != nil {
		return result, err
	}
	if storeExists {
		equivalent, eqErr := legacyConfigMatchesStore(legacyStore, store)
		if eqErr != nil {
			return result, eqErr
		}
		if equivalent {
			if err := os.Remove(settingsPath); err != nil {
				log.ApplicationLogger().Warn("Failed to remove redundant legacy settings.json after postgres migration", "path", settingsPath, "err", err)
			} else {
				result.RemovedFile = true
			}
			if removed, err := removeDirIfEmptyAndOwned(filepath.Dir(settingsPath)); err != nil {
				log.ApplicationLogger().Warn("Failed to remove redundant legacy settings parent directory after postgres migration", "dir", filepath.Dir(settingsPath), "err", err)
			} else {
				result.RemovedParentDir = removed
			}
			return result, nil
		}
		log.ApplicationLogger().Warn(
			"Legacy settings.json still exists but differs from postgres config; leaving file untouched",
			"path", settingsPath,
			"store", store.Describe(),
		)
		return result, nil
	}

	cfg, err := legacyStore.Load()
	if err != nil {
		return result, fmt.Errorf("load legacy settings for migration: %w", err)
	}
	_ = normalizeAutoAssignmentRoleOrder(cfg)
	if validationErr := validateBotConfig(cfg); validationErr != nil {
		return result, fmt.Errorf("%s: %w", ErrValidationFailed, validationErr)
	}
	if err := store.Save(cfg); err != nil {
		return result, fmt.Errorf("save migrated config to %s: %w", store.Describe(), err)
	}
	result.Migrated = true

	if err := os.Remove(settingsPath); err != nil {
		log.ApplicationLogger().Warn("Failed to remove legacy settings.json after migration", "path", settingsPath, "err", err)
	} else {
		result.RemovedFile = true
	}

	if removed, err := removeDirIfEmptyAndOwned(filepath.Dir(settingsPath)); err != nil {
		log.ApplicationLogger().Warn("Failed to remove legacy settings parent directory after migration", "dir", filepath.Dir(settingsPath), "err", err)
	} else {
		result.RemovedParentDir = removed
	}

	return result, nil
}

func legacyConfigMatchesStore(legacyStore ConfigStore, store ConfigStore) (bool, error) {
	legacyCfg, err := legacyStore.Load()
	if err != nil {
		return false, fmt.Errorf("load legacy settings for comparison: %w", err)
	}
	storeCfg, err := store.Load()
	if err != nil {
		return false, fmt.Errorf("load postgres config for comparison: %w", err)
	}
	return configsEquivalentForMigration(legacyCfg, storeCfg)
}

func configsEquivalentForMigration(left *BotConfig, right *BotConfig) (bool, error) {
	leftCopy := cloneOrDefaultConfig(left)
	rightCopy := cloneOrDefaultConfig(right)

	_ = normalizeAutoAssignmentRoleOrder(&leftCopy)
	_ = normalizeAutoAssignmentRoleOrder(&rightCopy)
	if err := validateBotConfig(&leftCopy); err != nil {
		return false, fmt.Errorf("%s: %w", ErrValidationFailed, err)
	}
	if err := validateBotConfig(&rightCopy); err != nil {
		return false, fmt.Errorf("%s: %w", ErrValidationFailed, err)
	}

	leftRaw, err := json.Marshal(leftCopy)
	if err != nil {
		return false, fmt.Errorf("marshal legacy config for comparison: %w", err)
	}
	rightRaw, err := json.Marshal(rightCopy)
	if err != nil {
		return false, fmt.Errorf("marshal postgres config for comparison: %w", err)
	}
	return bytes.Equal(leftRaw, rightRaw), nil
}

func cloneOrDefaultConfig(cfg *BotConfig) BotConfig {
	if cfg == nil {
		return BotConfig{Guilds: []GuildConfig{}}
	}
	return cloneBotConfig(*cfg)
}

func removeDirIfEmptyAndOwned(dir string) (bool, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false, nil
	}

	f, err := os.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer f.Close()

	entries, err := f.ReadDir(1)
	if err != nil && err != io.EOF {
		return false, err
	}
	if len(entries) > 0 {
		return false, nil
	}
	if err := os.Remove(dir); err != nil {
		return false, err
	}
	return true, nil
}
