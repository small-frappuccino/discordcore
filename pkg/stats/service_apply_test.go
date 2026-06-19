package stats

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestApplyMemberAdd(t *testing.T) {
	// test store

	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "guild-stats-main",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{},
				Stats: files.StatsConfig{
					Channels: []files.StatsChannelConfig{
						{ChannelID: "c1"},
					},
				},
			},
		}
		return nil
	})

	// Test early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.ApplyMemberAdd("guild-stats-main", "user1", time.Now(), false, func(yield func(string) bool) {
		yield("role1")
		yield("role2")
	})

	store := newMockStateStore()

	svc := NewStatsService(nil, cm, store, slog.Default(), "generic")
	svc.ApplyMemberAdd("guild-stats-main", "user1", time.Now(), false, func(yield func(string) bool) {
		yield("role1")
		yield("role2")
	})
}

func TestApplyMemberRemove(t *testing.T) {
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: "guild-stats-main", BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"}, FeatureRouting: map[string]string{"stats": "generic"}, Features: files.FeatureToggles{}, Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}},
		}
		return nil
	})

	// Testing early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.ApplyMemberRemove("guild-stats-main", "user1")

	store := newMockStateStore()

	svc := NewStatsService(nil, cm, store, slog.Default(), "generic")
	svc.ApplyMemberRemove("guild-stats-main", "user1")
}

func TestApplyStatsMemberUpdate(t *testing.T) {
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: "guild-stats-main", BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"}, FeatureRouting: map[string]string{"stats": "generic"}, Features: files.FeatureToggles{}, Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}},
		}
		return nil
	})

	// Testing early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.ApplyStatsMemberUpdate("guild-stats-main", "user1", true, func(yield func(string) bool) {
		yield("role1")
	})

	store := newMockStateStore()

	svc := NewStatsService(nil, cm, store, slog.Default(), "generic")
	svc.ApplyStatsMemberUpdate("guild-stats-main", "user1", true, func(yield func(string) bool) {
		yield("role1")
	})
}
