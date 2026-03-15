package files

import (
	"fmt"
	"os"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/errutil"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// JSONConfigStore persists BotConfig in the legacy settings.json file.
type JSONConfigStore struct {
	path    string
	manager *util.JSONManager
}

func NewJSONConfigStore(path string) *JSONConfigStore {
	path = strings.TrimSpace(path)
	if path == "" {
		path = util.GetSettingsFilePath()
	}
	return &JSONConfigStore{
		path:    path,
		manager: util.NewJSONManager(path),
	}
}

func (s *JSONConfigStore) Load() (*BotConfig, error) {
	cfg := &BotConfig{Guilds: []GuildConfig{}}
	if s == nil || s.manager == nil {
		return cfg, nil
	}
	if err := s.manager.Load(cfg); err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, errutil.HandleConfigError("read", s.path, func() error { return err })
	}
	return cfg, nil
}

func (s *JSONConfigStore) Save(cfg *BotConfig) error {
	if cfg == nil {
		return fmt.Errorf("cannot save nil config")
	}
	if s == nil || s.manager == nil {
		return fmt.Errorf("json config store is not configured")
	}
	if err := s.manager.Save(cfg); err != nil {
		return errutil.HandleConfigError("write", s.path, func() error { return err })
	}
	return nil
}

func (s *JSONConfigStore) Exists() (bool, error) {
	if s == nil {
		return false, nil
	}
	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat legacy settings file %s: %w", s.path, err)
	}
	return !info.IsDir(), nil
}

func (s *JSONConfigStore) Describe() string {
	if s == nil || strings.TrimSpace(s.path) == "" {
		return "legacy-json"
	}
	return s.path
}
