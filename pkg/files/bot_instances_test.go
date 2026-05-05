package files

import (
	"reflect"
	"testing"
)

func TestGuildConfigEffectiveBotInstanceIDForDomainUsesOverrideAndFallback(t *testing.T) {
	t.Parallel()

	guild := GuildConfig{
		GuildID:       "guild-1",
		BotInstanceID: "alice",
		DomainBotInstanceIDs: map[string]string{
			" QOTD ": " yuzuha ",
			"tickets": "   ",
		},
	}

	if got := guild.BotInstanceIDOverrideForDomain(BotDomainQOTD); got != "yuzuha" {
		t.Fatalf("expected qotd override to resolve yuzuha, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain(BotDomainQOTD, "default"); got != "yuzuha" {
		t.Fatalf("expected qotd effective binding=yuzuha, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain("tickets", "default"); got != "alice" {
		t.Fatalf("expected empty tickets override to fall back to guild binding alice, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain("moderation", "default"); got != "alice" {
		t.Fatalf("expected unspecified domain to fall back to guild binding alice, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain("", "default"); got != "alice" {
		t.Fatalf("expected empty domain to use legacy guild binding alice, got %q", got)
	}
	if got := (GuildConfig{}).EffectiveBotInstanceIDForDomain(BotDomainQOTD, "default"); got != "default" {
		t.Fatalf("expected missing guild binding to fall back to runtime default, got %q", got)
	}
}

func TestBotConfigGuildsForBotInstanceForDomainUsesDomainAwareResolution(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{Guilds: []GuildConfig{
		{
			GuildID:       "g1",
			BotInstanceID: "alice",
			DomainBotInstanceIDs: map[string]string{
				BotDomainQOTD: "yuzuha",
			},
		},
		{GuildID: "g2", BotInstanceID: "alice"},
		{GuildID: "g3", BotInstanceID: "yuzuha"},
		{GuildID: "g4"},
	}}

	if got := guildIDsForTest(cfg.GuildsForBotInstance("alice", "alice")); !reflect.DeepEqual(got, []string{"g1", "g2", "g4"}) {
		t.Fatalf("expected legacy guild binding to stay unchanged, got %+v", got)
	}
	if got := guildIDsForTest(cfg.GuildsForBotInstanceForDomain(BotDomainQOTD, "alice", "alice")); !reflect.DeepEqual(got, []string{"g2", "g4"}) {
		t.Fatalf("expected qotd alice guilds [g2 g4], got %+v", got)
	}
	if got := guildIDsForTest(cfg.GuildsForBotInstanceForDomain(BotDomainQOTD, "yuzuha", "alice")); !reflect.DeepEqual(got, []string{"g1", "g3"}) {
		t.Fatalf("expected qotd yuzuha guilds [g1 g3], got %+v", got)
	}
	if got := guildIDsForTest(cfg.GuildsForBotInstanceForDomain("moderation", "alice", "alice")); !reflect.DeepEqual(got, []string{"g1", "g2", "g4"}) {
		t.Fatalf("expected unspecified domain to fall back to legacy guild binding, got %+v", got)
	}
}

func TestBotConfigHasDomainBotInstanceOverrides(t *testing.T) {
	t.Parallel()

	if (&BotConfig{}).HasDomainBotInstanceOverrides() {
		t.Fatal("expected empty config to report no domain overrides")
	}

	cfg := &BotConfig{Guilds: []GuildConfig{
		{GuildID: "g1", BotInstanceID: "alice"},
		{GuildID: "g2", BotInstanceID: "alice", DomainBotInstanceIDs: map[string]string{"tickets": "   "}},
	}}
	if cfg.HasDomainBotInstanceOverrides() {
		t.Fatal("expected blank domain override values to be ignored")
	}

	cfg.Guilds[1].DomainBotInstanceIDs[BotDomainQOTD] = "yuzuha"
	if !cfg.HasDomainBotInstanceOverrides() {
		t.Fatal("expected qotd override to trigger domain override detection")
	}
}

func guildIDsForTest(guilds []GuildConfig) []string {
	out := make([]string, 0, len(guilds))
	for _, guild := range guilds {
		out = append(out, guild.GuildID)
	}
	return out
}