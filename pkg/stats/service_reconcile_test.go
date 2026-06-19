package stats

import (
	"context"
	"iter"
	"log/slog"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type mockGateway struct {
	members []MemberSnapshot
	channel *Channel
}

func (m *mockGateway) UpdateChannelName(ctx context.Context, channelID, newName string) error {
	return nil
}

func (m *mockGateway) GetChannel(ctx context.Context, channelID string) (*Channel, error) {
	if m.channel != nil {
		return m.channel, nil
	}
	return &Channel{ID: channelID, Name: "test", GuildID: "guild-stats-main"}, nil
}

func (m *mockGateway) StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[MemberSnapshot, error] {
	return func(yield func(MemberSnapshot, error) bool) {
		for _, mem := range m.members {
			if !yield(mem, nil) {
				return
			}
		}
	}
}

func TestReconcileGuild(t *testing.T) {

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
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c1"},
				},
				},
			},
		}
		return nil
	})

	gateway := &mockGateway{
		members: []MemberSnapshot{
			{
				UserID: "u1",
				IsBot:  false,
				Roles: func(yield func(string) bool) {
					yield("role1")
				},
			},
			{
				UserID: "u2",
				IsBot:  true,
				Roles: func(yield func(string) bool) {
					yield("role2")
				},
			},
		},
	}

	// Test early return with nil store
	svcNil := NewStatsService(gateway, cm, nil, slog.Default(), "generic")
	svcNil.reconcileStatsForGuild(context.Background(), files.GuildConfig{GuildID: "guild-stats-main"})

	store := newMockStateStore()
	svc := NewStatsService(gateway, cm, store, slog.Default(), "generic")
	ctx := context.Background()

	gcfg, _, _, ok := svc.statsGuildConfig("guild-stats-main")
	if !ok {
		t.Fatalf("statsGuildConfig returned ok=false")
	}
	if gcfg.GuildID == "" {
		t.Fatalf("gcfg is empty")
	}
	err := svc.reconcileStatsForGuild(ctx, gcfg)
	if err != nil {
		t.Fatalf("reconcileStatsForGuild failed: %v", err)
	}
}

func TestReconcileAllGuilds(t *testing.T) {

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
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c1"},
				},
				},
			},
		}
		return nil
	})

	gateway := &mockGateway{}

	store := newMockStateStore()
	svc := NewStatsService(gateway, cm, store, slog.Default(), "generic")
	ctx := context.Background()

	svc.UpdateStatsChannels(ctx)
}

func TestStatsServiceLifecycle(t *testing.T) {
	cm := newTestConfigManager(t)

	// Test early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.reconcileStatsForGuild(context.Background(), files.GuildConfig{GuildID: "guild-stats-main"})

	svc := NewStatsService(nil, cm, newMockStateStore(), slog.Default(), "generic")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !svc.IsRunning() {
		t.Errorf("expected IsRunning to be true")
	}

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if svc.IsRunning() {
		t.Errorf("expected IsRunning to be false")
	}
}
