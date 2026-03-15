package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateLegacyJSONConfigToStore(t *testing.T) {
	store := openIsolatedPostgresConfigStore(t)

	root := t.TempDir()
	settingsDir := filepath.Join(root, "preferences")
	settingsPath := filepath.Join(settingsDir, "settings.json")

	legacyConfig := &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "guild-legacy",
				Features: FeatureToggles{
					Logging: FeatureLoggingToggles{
						MemberJoin: boolPtr(false),
					},
				},
				Channels: ChannelsConfig{
					MemberJoin: "channel-join",
				},
			},
		},
		RuntimeConfig: RuntimeConfig{
			BotTheme: "legacy",
		},
	}

	if err := SaveSettingsFileWithPath(settingsPath, legacyConfig); err != nil {
		t.Fatalf("write legacy settings file: %v", err)
	}

	result, err := MigrateLegacyJSONConfigToStore(settingsPath, store)
	if err != nil {
		t.Fatalf("migrate legacy settings file: %v", err)
	}
	if !result.Migrated {
		t.Fatalf("expected migration to occur")
	}
	if !result.RemovedFile {
		t.Fatalf("expected legacy settings file to be removed")
	}
	if !result.RemovedParentDir {
		t.Fatalf("expected empty legacy preferences directory to be removed")
	}

	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy settings file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(settingsDir); !os.IsNotExist(err) {
		t.Fatalf("expected legacy settings directory to be removed, stat err=%v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load migrated config from postgres: %v", err)
	}
	if got := loaded.RuntimeConfig.BotTheme; got != "legacy" {
		t.Fatalf("expected bot theme legacy, got %q", got)
	}
	if len(loaded.Guilds) != 1 {
		t.Fatalf("expected one migrated guild, got %d", len(loaded.Guilds))
	}
	if got := loaded.Guilds[0].Channels.MemberJoin; got != "channel-join" {
		t.Fatalf("expected member join channel channel-join, got %q", got)
	}
	if resolved := loaded.ResolveFeatures("guild-legacy"); resolved.Logging.MemberJoin {
		t.Fatalf("expected migrated member_join feature override to remain disabled")
	}
}

func TestMigrateLegacyJSONConfigToStoreRemovesRedundantLegacyFile(t *testing.T) {
	store := openIsolatedPostgresConfigStore(t)

	root := t.TempDir()
	settingsDir := filepath.Join(root, "preferences")
	settingsPath := filepath.Join(settingsDir, "settings.json")

	cfg := &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "guild-same",
				Channels: ChannelsConfig{
					Commands: "channel-commands",
				},
			},
		},
		RuntimeConfig: RuntimeConfig{
			BotTheme: "same",
		},
	}

	if err := store.Save(cfg); err != nil {
		t.Fatalf("seed postgres config store: %v", err)
	}
	if err := SaveSettingsFileWithPath(settingsPath, cfg); err != nil {
		t.Fatalf("write legacy settings file: %v", err)
	}

	result, err := MigrateLegacyJSONConfigToStore(settingsPath, store)
	if err != nil {
		t.Fatalf("remove redundant legacy settings file: %v", err)
	}
	if result.Migrated {
		t.Fatalf("expected redundant legacy cleanup without migration")
	}
	if !result.RemovedFile {
		t.Fatalf("expected redundant legacy settings file to be removed")
	}
	if !result.RemovedParentDir {
		t.Fatalf("expected redundant legacy settings directory to be removed")
	}
}
