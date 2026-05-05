package files

import (
	"reflect"
	"testing"
)

func TestGuildConfigEffectiveBotInstanceIDForDomainUsesOverrideAndFallback(t *testing.T) {
	t.Parallel()

	guild := GuildConfig{
		GuildID:       "guild-1",
		BotInstanceID: "main",
		DomainBotInstanceIDs: map[string]string{
			" QOTD ":  " companion ",
			"tickets": "   ",
		},
	}

	if got := guild.BotInstanceIDOverrideForDomain(BotDomainQOTD); got != "companion" {
		t.Fatalf("expected qotd override to resolve companion, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain(BotDomainQOTD, "default"); got != "companion" {
		t.Fatalf("expected qotd effective binding=companion, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain("tickets", "default"); got != "main" {
		t.Fatalf("expected empty tickets override to fall back to guild binding main, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain("moderation", "default"); got != "main" {
		t.Fatalf("expected unspecified domain to fall back to guild binding main, got %q", got)
	}
	if got := guild.EffectiveBotInstanceIDForDomain("", "default"); got != "main" {
		t.Fatalf("expected empty domain to use legacy guild binding main, got %q", got)
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
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				BotDomainQOTD: "companion",
			},
		},
		{GuildID: "g2", BotInstanceID: "main"},
		{GuildID: "g3", BotInstanceID: "companion"},
		{GuildID: "g4"},
	}}

	if got := guildIDsForTest(cfg.GuildsForBotInstance("main", "main")); !reflect.DeepEqual(got, []string{"g1", "g2", "g4"}) {
		t.Fatalf("expected legacy guild binding to stay unchanged, got %+v", got)
	}
	if got := guildIDsForTest(cfg.GuildsForBotInstanceForDomain(BotDomainQOTD, "main", "main")); !reflect.DeepEqual(got, []string{"g2", "g4"}) {
		t.Fatalf("expected qotd main guilds [g2 g4], got %+v", got)
	}
	if got := guildIDsForTest(cfg.GuildsForBotInstanceForDomain(BotDomainQOTD, "companion", "main")); !reflect.DeepEqual(got, []string{"g1", "g3"}) {
		t.Fatalf("expected qotd companion guilds [g1 g3], got %+v", got)
	}
	if got := guildIDsForTest(cfg.GuildsForBotInstanceForDomain("moderation", "main", "main")); !reflect.DeepEqual(got, []string{"g1", "g2", "g4"}) {
		t.Fatalf("expected unspecified domain to fall back to legacy guild binding, got %+v", got)
	}
}

func TestBotConfigHasDomainBotInstanceOverrides(t *testing.T) {
	t.Parallel()

	if (&BotConfig{}).HasDomainBotInstanceOverrides() {
		t.Fatal("expected empty config to report no domain overrides")
	}

	cfg := &BotConfig{Guilds: []GuildConfig{
		{GuildID: "g1", BotInstanceID: "main"},
		{GuildID: "g2", BotInstanceID: "main", DomainBotInstanceIDs: map[string]string{"tickets": "   "}},
	}}
	if cfg.HasDomainBotInstanceOverrides() {
		t.Fatal("expected blank domain override values to be ignored")
	}

	cfg.Guilds[1].DomainBotInstanceIDs[BotDomainQOTD] = "companion"
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
