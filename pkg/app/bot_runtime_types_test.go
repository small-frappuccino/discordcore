package app

import (
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestBotRuntimeResolverSessionForGuildUsesConfiguredBinding(t *testing.T) {
	t.Parallel()

	configManager := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "alice"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g2", BotInstanceID: "yuzuha"}); err != nil {
		t.Fatalf("add guild g2: %v", err)
	}

	aliceSession, err := discordgo.New("Bot alice-token")
	if err != nil {
		t.Fatalf("create alice session: %v", err)
	}
	yuzuhaSession, err := discordgo.New("Bot yuzuha-token")
	if err != nil {
		t.Fatalf("create yuzuha session: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"alice":  {instanceID: "alice", session: aliceSession},
		"yuzuha": {instanceID: "yuzuha", session: yuzuhaSession},
	}, "alice")

	if got := resolver.sessionForGuild("g1"); got != aliceSession {
		t.Fatalf("expected alice session for g1, got %p want %p", got, aliceSession)
	}
	if got := resolver.sessionForGuild("g2"); got != yuzuhaSession {
		t.Fatalf("expected yuzuha session for g2, got %p want %p", got, yuzuhaSession)
	}
	if got := resolver.sessionForGuild("missing"); got != aliceSession {
		t.Fatalf("expected default alice session for unknown guild, got %p want %p", got, aliceSession)
	}
}

func TestValidateConfiguredBotInstancesRejectsUnknownBinding(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: "missing",
		}},
	}

	err := validateConfiguredBotInstances(cfg, map[string]*botRuntime{
		"alice": {instanceID: "alice"},
	}, "alice")
	if err == nil {
		t.Fatal("expected validation error for unknown bot instance binding")
	}
}
