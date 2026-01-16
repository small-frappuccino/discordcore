package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func newTestConfigManager(guilds []GuildConfig) *ConfigManager {
	return &ConfigManager{
		config: &BotConfig{Guilds: guilds},
	}
}

func TestGuildConfigIndexHit(t *testing.T) {
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
		{GuildID: "g2"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	cfg := mgr.GuildConfig("g2")
	if cfg == nil || cfg.GuildID != "g2" {
		t.Fatalf("expected guild g2, got %+v", cfg)
	}
}

func TestGuildConfigIndexMiss(t *testing.T) {
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	if cfg := mgr.GuildConfig("missing"); cfg != nil {
		t.Fatalf("expected nil for missing guild, got %+v", cfg)
	}
}

func TestGuildConfigIndexUpdate(t *testing.T) {
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	if err := mgr.AddGuildConfig(GuildConfig{GuildID: "g2"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	if cfg := mgr.GuildConfig("g2"); cfg == nil || cfg.GuildID != "g2" {
		t.Fatalf("expected guild g2 after add, got %+v", cfg)
	}

	mgr.RemoveGuildConfig("g1")
	if cfg := mgr.GuildConfig("g1"); cfg != nil {
		t.Fatalf("expected g1 removed, got %+v", cfg)
	}
}

func TestGuildConfigIndexDuplicateFix(t *testing.T) {
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
		{GuildID: "g1"},
		{GuildID: "g2"},
	})

	if _, err := mgr.rebuildGuildIndexLocked("test"); err == nil {
		t.Fatalf("expected duplicate error")
	}

	if got := len(mgr.config.Guilds); got != 2 {
		t.Fatalf("expected 2 guilds after dedupe, got %d", got)
	}
	if cfg := mgr.GuildConfig("g1"); cfg == nil {
		t.Fatalf("expected g1 to remain after dedupe")
	}
	if stats := mgr.GuildIndexStats(); stats.Duplicates == 0 {
		t.Fatalf("expected duplicate counter to increment")
	}
}

func TestGuildConfigIndexDedupePersistsOnLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")
	raw := BotConfig{
		Guilds: []GuildConfig{
			{GuildID: "g1"},
			{GuildID: "g1"},
			{GuildID: "g2"},
		},
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	mgr := NewConfigManagerWithPath(path)
	if err := mgr.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var cfg BotConfig
	if err := json.Unmarshal(updated, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if got := len(cfg.Guilds); got != 2 {
		t.Fatalf("expected 2 guilds after dedupe, got %d", got)
	}
	if stats := mgr.GuildIndexStats(); stats.Duplicates == 0 {
		t.Fatalf("expected duplicate counter to increment")
	}
}

func TestGuildConfigIndexConcurrency(t *testing.T) {
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	var wg sync.WaitGroup
	readers := 10
	writes := 20

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = mgr.GuildConfig("g1")
				_ = mgr.GuildConfig("missing")
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < writes; i++ {
			id := fmt.Sprintf("g%02d", i+2)
			_ = mgr.AddGuildConfig(GuildConfig{GuildID: id})
		}
	}()

	wg.Wait()

	if cfg := mgr.GuildConfig("g1"); cfg == nil {
		t.Fatalf("expected g1 to remain accessible")
	}
	if stats := mgr.GuildIndexStats(); stats.Rebuilds == 0 {
		t.Fatalf("expected rebuild counter to be non-zero")
	}
}
