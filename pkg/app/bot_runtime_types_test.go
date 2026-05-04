package app

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestBotRuntimeResolverSessionForGuildUsesConfiguredBinding(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "alice"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g2", BotInstanceID: "yuzuha"}); err != nil {
		t.Fatalf("add guild g2: %v", err)
	}
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g3", BotInstanceID: "alice"}); err != nil {
		t.Fatalf("add guild g3: %v", err)
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

	if got, err := resolver.sessionForGuild("g1"); err != nil || got != aliceSession {
		t.Fatalf("expected alice session for g1, got %p err=%v want %p", got, err, aliceSession)
	}
	if got, err := resolver.sessionForGuild("g2"); err != nil || got != yuzuhaSession {
		t.Fatalf("expected yuzuha session for g2, got %p err=%v want %p", got, err, yuzuhaSession)
	}
	if got, err := resolver.sessionForGuild("g3"); err != nil || got != aliceSession {
		t.Fatalf("expected alice session for g3, got %p err=%v want %p", got, err, aliceSession)
	}
	if got, err := resolver.sessionForGuild(""); err != nil || got != aliceSession {
		t.Fatalf("expected default alice session for empty guild, got %p err=%v want %p", got, err, aliceSession)
	}
}

func TestBotRuntimeResolverSessionForGuildRejectsMissingGuild(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "alice"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}

	aliceSession, err := discordgo.New("Bot alice-token")
	if err != nil {
		t.Fatalf("create alice session: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"alice": {instanceID: "alice", session: aliceSession},
	}, "alice")

	if got, err := resolver.sessionForGuild("missing"); err == nil {
		t.Fatalf("expected missing guild lookup to fail, got session %p", got)
	} else if gotErr := err.Error(); gotErr != "guild missing is not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBotRuntimeResolverSessionForGuildRejectsUnavailableRuntime(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "alice"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{}, "alice")

	if got, err := resolver.sessionForGuild("g1"); err == nil {
		t.Fatalf("expected unavailable runtime to fail, got session %p", got)
	} else if gotErr := err.Error(); gotErr != `bot instance "alice" is unavailable for guild g1` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBotRuntimeResolverSessionForGuildRejectsMissingSession(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "alice"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"alice": {instanceID: "alice"},
	}, "alice")

	if got, err := resolver.sessionForGuild("g1"); err == nil {
		t.Fatalf("expected missing session to fail, got session %p", got)
	} else if gotErr := err.Error(); gotErr != `discord session for guild g1 (bot instance "alice") is unavailable` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBotRuntimeResolverRegisterGuildPersistsDormantConfig(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()

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

	if err := resolver.registerGuild(context.Background(), "g-new", "yuzuha"); err != nil {
		t.Fatalf("register guild: %v", err)
	}

	snapshot := configManager.SnapshotConfig()
	var guild *files.GuildConfig
	for idx := range snapshot.Guilds {
		if snapshot.Guilds[idx].GuildID == "g-new" {
			guild = &snapshot.Guilds[idx]
			break
		}
	}
	if guild == nil {
		t.Fatal("expected dormant guild to be persisted")
	}
	if guild.BotInstanceID != "yuzuha" {
		t.Fatalf("expected bot instance binding yuzuha, got %+v", guild)
	}
	if guild.Channels != (files.ChannelsConfig{}) {
		t.Fatalf("expected no channel bootstrap during manual registration, got %+v", guild.Channels)
	}
	if len(guild.Roles.Allowed) != 0 ||
		guild.Roles.AutoAssignment.Enabled ||
		guild.Roles.AutoAssignment.TargetRoleID != "" ||
		len(guild.Roles.AutoAssignment.RequiredRoles) != 0 {
		t.Fatalf("expected no role bootstrap during manual registration, got %+v", guild.Roles)
	}

	resolved := snapshot.ResolveFeatures("g-new")
	if resolved.Services.Monitoring ||
		resolved.Services.Commands ||
		resolved.Logging.MemberJoin ||
		resolved.StatsChannels ||
		resolved.AutoRoleAssign ||
		resolved.UserPrune {
		t.Fatalf("expected dormant guild features to stay disabled, got %+v", resolved)
	}
}

func TestResolveBotInstancesSkipsOptionalInstancesWithoutToken(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ALICE_TOKEN", "alice-token")
	t.Setenv("YUZUHA_TOKEN", "")

	resolved, defaultBotInstanceID, err := resolveBotInstances("", RunOptions{
		DefaultBotInstanceID: "alice",
		BotCatalog: []BotInstanceDefinition{
			{ID: "alice", TokenEnv: "ALICE_TOKEN"},
			{ID: "yuzuha", TokenEnv: "YUZUHA_TOKEN", Optional: true},
		},
	})
	if err != nil {
		t.Fatalf("resolve bot instances: %v", err)
	}
	if defaultBotInstanceID != "alice" {
		t.Fatalf("expected default bot instance alice, got %q", defaultBotInstanceID)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected only alice to resolve, got %+v", resolved)
	}
	if resolved[0].ID != "alice" || resolved[0].TokenEnv != "ALICE_TOKEN" || resolved[0].Token != "alice-token" {
		t.Fatalf("unexpected resolved instance: %+v", resolved[0])
	}
}

func TestResolveBotInstancesUsesFirstAvailableOptionalBotAsDefault(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ALICE_TOKEN", "")
	t.Setenv("YUZUHA_TOKEN", "yuzuha-token")

	resolved, defaultBotInstanceID, err := resolveBotInstances("", RunOptions{
		BotCatalog: []BotInstanceDefinition{
			{ID: "alice", TokenEnv: "ALICE_TOKEN", Optional: true},
			{ID: "yuzuha", TokenEnv: "YUZUHA_TOKEN", Optional: true},
		},
	})
	if err != nil {
		t.Fatalf("resolve bot instances: %v", err)
	}
	if defaultBotInstanceID != "yuzuha" {
		t.Fatalf("expected yuzuha to become the default bot instance, got %q", defaultBotInstanceID)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected only yuzuha to resolve, got %+v", resolved)
	}
	if resolved[0].ID != "yuzuha" || resolved[0].TokenEnv != "YUZUHA_TOKEN" || resolved[0].Token != "yuzuha-token" {
		t.Fatalf("unexpected resolved instance: %+v", resolved[0])
	}
}

func TestResolveBotInstancesRejectsMissingRequiredToken(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ALICE_TOKEN", "")

	_, _, err := resolveBotInstances("", RunOptions{
		DefaultBotInstanceID: "alice",
		BotCatalog: []BotInstanceDefinition{{
			ID:       "alice",
			TokenEnv: "ALICE_TOKEN",
		}},
	})
	if err == nil {
		t.Fatal("expected missing required token to fail")
	}
	if got := err.Error(); got != "ALICE_TOKEN not set in environment or .env file" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBotInstancesReturnsSentinelWhenNoOptionalTokensAreConfigured(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ALICE_TOKEN", "")
	t.Setenv("YUZUHA_TOKEN", "")

	_, _, err := resolveBotInstances("", RunOptions{
		BotCatalog: []BotInstanceDefinition{
			{ID: "alice", TokenEnv: "ALICE_TOKEN", Optional: true},
			{ID: "yuzuha", TokenEnv: "YUZUHA_TOKEN", Optional: true},
		},
	})
	if err == nil {
		t.Fatal("expected missing optional token set to fail")
	}
	if !errors.Is(err, ErrNoBotTokensConfigured) {
		t.Fatalf("expected ErrNoBotTokensConfigured, got %v", err)
	}
	if got := err.Error(); got != ErrNoBotTokensConfigured.Error() {
		t.Fatalf("unexpected error message: got %q want %q", got, ErrNoBotTokensConfigured.Error())
	}
}

func TestResolveBotInstancesRejectsDefaultWhenOptionalInstanceIsUnavailable(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("ALICE_TOKEN", "alice-token")
	t.Setenv("YUZUHA_TOKEN", "")

	_, _, err := resolveBotInstances("", RunOptions{
		DefaultBotInstanceID: "yuzuha",
		BotCatalog: []BotInstanceDefinition{
			{ID: "alice", TokenEnv: "ALICE_TOKEN"},
			{ID: "yuzuha", TokenEnv: "YUZUHA_TOKEN", Optional: true},
		},
	})
	if err == nil {
		t.Fatal("expected missing default optional bot instance to fail")
	}
	if got := err.Error(); got != `default bot instance "yuzuha" is not present in the runtime catalog` {
		t.Fatalf("unexpected error: %v", err)
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

func TestValidateConfiguredBotInstancesRejectsUnknownDomainBinding(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: "alice",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "missing",
			},
		}},
	}

	err := validateConfiguredBotInstances(cfg, map[string]*botRuntime{
		"alice": {instanceID: "alice"},
	}, "alice")
	if err == nil {
		t.Fatal("expected validation error for unknown domain bot instance binding")
	}
	if got := err.Error(); got != `guild g1 domain "qotd" references unknown bot instance "missing"` {
		t.Fatalf("unexpected error: %v", err)
	}
}
