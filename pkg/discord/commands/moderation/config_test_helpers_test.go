package moderation

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func newTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()
	return files.NewMemoryConfigManager()
}

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
