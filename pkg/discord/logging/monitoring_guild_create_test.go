package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestMonitoringServiceHandleGuildCreatePersistsDormantGuild(t *testing.T) {
	t.Parallel()

	cfgMgr := files.NewMemoryConfigManager()
	session := newLoggingLifecycleSession(t)
	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		botInstanceID: "yuzuha",
	}

	ms.handleGuildCreate(session, &discordgo.GuildCreate{
		Guild: &discordgo.Guild{ID: "1375650791251120179"},
	})

	cfg := cfgMgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected one guild persisted after guild create, got %+v", cfg.Guilds)
	}
	if cfg.Guilds[0].GuildID != "1375650791251120179" || cfg.Guilds[0].BotInstanceID != "yuzuha" {
		t.Fatalf("unexpected persisted guild after guild create: %+v", cfg.Guilds[0])
	}
	if cfg.Guilds[0].Channels != (files.ChannelsConfig{}) {
		t.Fatalf("expected no automatic channel bootstrap on guild create, got %+v", cfg.Guilds[0].Channels)
	}

	resolved := cfg.ResolveFeatures("1375650791251120179")
	if resolved.Services.Monitoring || resolved.Services.Commands || resolved.Logging.MemberJoin || resolved.StatsChannels || resolved.UserPrune {
		t.Fatalf("expected guild create to persist dormant feature defaults, got %+v", resolved)
	}
}

func TestMonitoringServiceHandleGuildCreateSkipsGuildOutsideAllowlist(t *testing.T) {
	t.Parallel()

	cfgMgr := files.NewMemoryConfigManager()
	session := newLoggingLifecycleSession(t)
	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		botInstanceID: "yuzuha",
	}

	ms.handleGuildCreate(session, &discordgo.GuildCreate{
		Guild: &discordgo.Guild{ID: "guild-denied"},
	})

	cfg := cfgMgr.SnapshotConfig()
	if len(cfg.Guilds) != 0 {
		t.Fatalf("expected guild outside allowlist to be ignored, got %+v", cfg.Guilds)
	}
}
