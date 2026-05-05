package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestMonitoringServiceHandleGuildCreatePersistsDormantGuild(t *testing.T) {
	t.Parallel()

	const guildID = "guild-any"

	cfgMgr := files.NewMemoryConfigManager()
	session := newLoggingLifecycleSession(t)
	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		botInstanceID: "companion",
	}

	ms.handleGuildCreate(session, &discordgo.GuildCreate{
		Guild: &discordgo.Guild{ID: guildID},
	})

	cfg := cfgMgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected one guild persisted after guild create, got %+v", cfg.Guilds)
	}
	if cfg.Guilds[0].GuildID != guildID || cfg.Guilds[0].BotInstanceID != "companion" {
		t.Fatalf("unexpected persisted guild after guild create: %+v", cfg.Guilds[0])
	}
	if cfg.Guilds[0].Channels != (files.ChannelsConfig{}) {
		t.Fatalf("expected no automatic channel bootstrap on guild create, got %+v", cfg.Guilds[0].Channels)
	}

	resolved := cfg.ResolveFeatures(guildID)
	if resolved.Services.Monitoring || resolved.Services.Commands || resolved.Logging.MemberJoin || resolved.StatsChannels || resolved.UserPrune {
		t.Fatalf("expected guild create to persist dormant feature defaults, got %+v", resolved)
	}
}
