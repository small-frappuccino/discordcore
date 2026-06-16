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
	cm := files.NewConfigManagerWithStore(&mockConfigStore{}, nil)
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
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
					StatsChannels: testBoolPtr(true),
				},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c1"},
				},
				},
			},
			{
				GuildID: "guild-stats-custom",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
					StatsChannels: testBoolPtr(true),
				},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c2"},
				},
				},
			},
			{
				GuildID: "guild-stats-default",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
					StatsChannels: testBoolPtr(true),
				},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
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
	genericSvc := NewStatsService(nil, cm, nil, logger, "generic")
	defaultSvc := NewStatsService(nil, cm, nil, logger, "")

	if !genericSvc.handlesGuild("guild-stats-main") {
		t.Errorf("expected generic service to handle guild-stats-main")
	}

	if !genericSvc.handlesGuild("guild-stats-custom") {
		t.Errorf("expected generic service to handle guild-stats-custom")
	}

	if genericSvc.handlesGuild("guild-stats-default") {
		t.Errorf("expected generic service to NOT handle guild-stats-default (unrouted)")
	}
	if defaultSvc.handlesGuild("guild-stats-default") {
		t.Errorf("expected default service to NOT handle guild-stats-default (unrouted sentinel)")
	}
}

func testBoolPtr(v bool) *bool { return &v }
