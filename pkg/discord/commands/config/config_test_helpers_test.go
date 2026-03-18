package config

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func mustUpdateConfig(t *testing.T, cm *files.ConfigManager, fn func(*files.BotConfig)) {
	t.Helper()

	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		if fn != nil {
			fn(cfg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}
}
