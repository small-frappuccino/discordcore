package files

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

func newTestConfigManager(guilds []GuildConfig) *ConfigManager {
	return &ConfigManager{
		config: &BotConfig{Guilds: guilds},
	}
}

func TestGuildConfigIndexHit(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestSnapshotConfigReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{
			GuildID: "g1",
			Channels: ChannelsConfig{
				MessageDelete: "c1",
			},
		},
	})

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) == 0 {
		t.Fatal("expected config snapshot")
	}

	cfg.Guilds[0].Channels.MessageDelete = "modified"

	fresh := mgr.Config()
	if got := fresh.Guilds[0].Channels.MessageDelete; got != "c1" {
		t.Fatalf("expected original channel to remain unchanged, got %q", got)
	}
}

func TestPublishedConfigReadsReuseSnapshot(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{
			GuildID: "g1",
			Channels: ChannelsConfig{
				MessageDelete: "c1",
			},
		},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	firstCfg := mgr.Config()
	secondCfg := mgr.Config()
	if firstCfg == nil || secondCfg == nil {
		t.Fatal("expected published config snapshot")
	}
	if firstCfg != secondCfg {
		t.Fatal("expected Config() to reuse the published snapshot")
	}

	firstGuild := mgr.GuildConfig("g1")
	secondGuild := mgr.GuildConfig("g1")
	if firstGuild == nil || secondGuild == nil {
		t.Fatal("expected published guild config snapshot")
	}
	if firstGuild != secondGuild {
		t.Fatal("expected GuildConfig() to reuse the published snapshot")
	}

	var memStats1, memStats2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats1)
	const runs = 10000
	for i := 0; i < runs; i++ {
		_ = mgr.Config()
		_ = mgr.GuildConfig("g1")
	}
	runtime.ReadMemStats(&memStats2)
	avgAllocs := float64(memStats2.Mallocs-memStats1.Mallocs) / float64(runs)
	if avgAllocs >= 0.5 {
		t.Fatalf("expected zero allocations for published config reads, got average %f (total mallocs: %d)", avgAllocs, memStats2.Mallocs-memStats1.Mallocs)
	}
}

func TestGuildConfigIndexDuplicateFix(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	store := &MemoryConfigStore{}
	raw := &BotConfig{
		Guilds: []GuildConfig{
			{GuildID: "g1"},
			{GuildID: "g1"},
			{GuildID: "g2"},
		},
	}
	if err := store.Save(raw); err != nil {
		t.Fatalf("seed config store: %v", err)
	}

	mgr := NewConfigManagerWithStore(store, nil)
	if err := mgr.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	updated, err := store.Load()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if got := len(updated.Guilds); got != 2 {
		t.Fatalf("expected 2 guilds after dedupe, got %d", got)
	}
	if stats := mgr.GuildIndexStats(); stats.Duplicates == 0 {
		t.Fatalf("expected duplicate counter to increment")
	}
}

func TestGuildConfigIndexConcurrency(t *testing.T) {
	t.Parallel()
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
				mgr.GuildConfig("g1")
				mgr.GuildConfig("missing")
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < writes; i++ {
			id := fmt.Sprintf("g%02d", i+2)
			mgr.AddGuildConfig(GuildConfig{GuildID: id})
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
