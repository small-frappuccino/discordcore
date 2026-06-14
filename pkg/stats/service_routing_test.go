package stats

import (
	"log/slog"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type mockConfigStore struct {
	cfg *files.BotConfig
}

func (m *mockConfigStore) Load() (*files.BotConfig, error) {
	if m.cfg == nil {
		return &files.BotConfig{}, nil
	}
	return m.cfg, nil
}

func (m *mockConfigStore) Save(cfg *files.BotConfig) error {
	m.cfg = cfg
	return nil
}

func (m *mockConfigStore) Transaction(fn func(cfg *files.BotConfig) error) (bool, error) {
	if m.cfg == nil {
		m.cfg = &files.BotConfig{}
	}
	if err := fn(m.cfg); err != nil {
		return false, err
	}
	return true, nil
}

func (m *mockConfigStore) Describe() string {
	return "mock"
}

func (m *mockConfigStore) Exists() (bool, error) {
	return m.cfg != nil, nil
}

func newTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()
	cm := files.NewConfigManagerWithStore(&mockConfigStore{})
	cfg, _, err := cm.LoadConfigFromStore()
	if err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}
	cm.ApplyConfig(cfg)
	return cm
}

func TestStatsServiceHandlesGuild(t *testing.T) {
	cm := newTestConfigManager(t)

	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "guild-stats-main",
				BotInstanceTokens: map[string]files.EncryptedString{
					"main": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "main",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
					StatsChannels: testBoolPtr(true),
				},
				Stats: files.StatsConfig{
					Enabled: true,
					Channels: []files.StatsChannelConfig{
						{ChannelID: "c1"},
					},
				},
			},
			{
				GuildID: "guild-stats-companion",
				BotInstanceTokens: map[string]files.EncryptedString{
					"companion": "token",
					"main":      "token",
				},
				FeatureRouting: map[string]string{
					"stats": "companion",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
					StatsChannels: testBoolPtr(true),
				},
				Stats: files.StatsConfig{
					Enabled: true,
					Channels: []files.StatsChannelConfig{
						{ChannelID: "c2"},
					},
				},
			},
			{
				GuildID: "guild-stats-default",
				BotInstanceTokens: map[string]files.EncryptedString{
					"main": "token",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
					StatsChannels: testBoolPtr(true),
				},
				Stats: files.StatsConfig{
					Enabled: true,
					Channels: []files.StatsChannelConfig{
						{ChannelID: "c3"},
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	logger := slog.Default()
	mainSvc := NewStatsService(nil, cm, nil, logger, "main", "main")
	companionSvc := NewStatsService(nil, cm, nil, logger, "companion", "main")

	if !mainSvc.handlesGuild("guild-stats-main") {
		t.Errorf("expected main service to handle guild-stats-main")
	}
	if companionSvc.handlesGuild("guild-stats-main") {
		t.Errorf("expected companion service to NOT handle guild-stats-main")
	}

	if mainSvc.handlesGuild("guild-stats-companion") {
		t.Errorf("expected main service to NOT handle guild-stats-companion")
	}
	if !companionSvc.handlesGuild("guild-stats-companion") {
		t.Errorf("expected companion service to handle guild-stats-companion")
	}

	if !mainSvc.handlesGuild("guild-stats-default") {
		t.Errorf("expected main service to handle guild-stats-default (default bot instance)")
	}
	if companionSvc.handlesGuild("guild-stats-default") {
		t.Errorf("expected companion service to NOT handle guild-stats-default")
	}
}

func testBoolPtr(v bool) *bool { return &v }
