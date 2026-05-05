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
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "main"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g2", BotInstanceID: "companion"}); err != nil {
		t.Fatalf("add guild g2: %v", err)
	}
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g3", BotInstanceID: "main"}); err != nil {
		t.Fatalf("add guild g3: %v", err)
	}

	mainSession, err := discordgo.New("Bot main-token")
	if err != nil {
		t.Fatalf("create main session: %v", err)
	}
	companionSession, err := discordgo.New("Bot companion-token")
	if err != nil {
		t.Fatalf("create companion session: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"main":      {instanceID: "main", session: mainSession},
		"companion": {instanceID: "companion", session: companionSession},
	}, "main")

	if got, err := resolver.sessionForGuild("g1"); err != nil || got != mainSession {
		t.Fatalf("expected main session for g1, got %p err=%v want %p", got, err, mainSession)
	}
	if got, err := resolver.sessionForGuild("g2"); err != nil || got != companionSession {
		t.Fatalf("expected companion session for g2, got %p err=%v want %p", got, err, companionSession)
	}
	if got, err := resolver.sessionForGuild("g3"); err != nil || got != mainSession {
		t.Fatalf("expected main session for g3, got %p err=%v want %p", got, err, mainSession)
	}
	if got, err := resolver.sessionForGuild(""); err != nil || got != mainSession {
		t.Fatalf("expected default main session for empty guild, got %p err=%v want %p", got, err, mainSession)
	}
}

func TestBotRuntimeResolverSessionForGuildDomainUsesDomainOverride(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{
		GuildID:       "g1",
		BotInstanceID: "main",
		DomainBotInstanceIDs: map[string]string{
			files.BotDomainQOTD: "companion",
		},
	}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}

	mainSession, err := discordgo.New("Bot main-token")
	if err != nil {
		t.Fatalf("create main session: %v", err)
	}
	companionSession, err := discordgo.New("Bot companion-token")
	if err != nil {
		t.Fatalf("create companion session: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"main":      {instanceID: "main", session: mainSession},
		"companion": {instanceID: "companion", session: companionSession},
	}, "main")

	if got, err := resolver.sessionForGuild("g1"); err != nil || got != mainSession {
		t.Fatalf("expected legacy guild lookup to stay on main, got %p err=%v want %p", got, err, mainSession)
	}
	if got, err := resolver.sessionForGuildDomain("g1", files.BotDomainQOTD); err != nil || got != companionSession {
		t.Fatalf("expected qotd domain lookup to use companion, got %p err=%v want %p", got, err, companionSession)
	}
	if got, err := resolver.sessionForGuildDomain("g1", "moderation"); err != nil || got != mainSession {
		t.Fatalf("expected unspecified domain lookup to fall back to main, got %p err=%v want %p", got, err, mainSession)
	}
}

func TestBotRuntimeResolverSessionForGuildRejectsMissingGuild(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "main"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}

	mainSession, err := discordgo.New("Bot main-token")
	if err != nil {
		t.Fatalf("create main session: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"main": {instanceID: "main", session: mainSession},
	}, "main")

	if got, err := resolver.sessionForGuild("missing"); err == nil {
		t.Fatalf("expected missing guild lookup to fail, got session %p", got)
	} else if gotErr := err.Error(); gotErr != "guild missing is not configured" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBotRuntimeResolverSessionForGuildRejectsUnavailableRuntime(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "main"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{}, "main")

	if got, err := resolver.sessionForGuild("g1"); err == nil {
		t.Fatalf("expected unavailable runtime to fail, got session %p", got)
	} else if gotErr := err.Error(); gotErr != `bot instance "main" is unavailable for guild g1` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBotRuntimeResolverSessionForGuildRejectsMissingSession(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{GuildID: "g1", BotInstanceID: "main"}); err != nil {
		t.Fatalf("add guild g1: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"main": {instanceID: "main"},
	}, "main")

	if got, err := resolver.sessionForGuild("g1"); err == nil {
		t.Fatalf("expected missing session to fail, got session %p", got)
	} else if gotErr := err.Error(); gotErr != `discord session for guild g1 (bot instance "main") is unavailable` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBotRuntimeResolverRegisterGuildPersistsDormantConfig(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()

	mainSession, err := discordgo.New("Bot main-token")
	if err != nil {
		t.Fatalf("create main session: %v", err)
	}
	companionSession, err := discordgo.New("Bot companion-token")
	if err != nil {
		t.Fatalf("create companion session: %v", err)
	}

	resolver := newBotRuntimeResolver(configManager, map[string]*botRuntime{
		"main":      {instanceID: "main", session: mainSession},
		"companion": {instanceID: "companion", session: companionSession},
	}, "main")

	if err := resolver.registerGuild(context.Background(), "g-new", "companion"); err != nil {
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
	if guild.BotInstanceID != "companion" {
		t.Fatalf("expected bot instance binding companion, got %+v", guild)
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
	t.Setenv("MAIN_TOKEN", "main-token")
	t.Setenv("COMPANION_TOKEN", "")

	resolved, defaultOwnerBotInstanceID, err := resolveBotInstances("", RunOptions{
		DefaultOwnerBotInstanceID: "main",
		BotCatalog: []BotInstanceDefinition{
			{ID: "main", TokenEnv: "MAIN_TOKEN"},
			{ID: "companion", TokenEnv: "COMPANION_TOKEN", Optional: true},
		},
	})
	if err != nil {
		t.Fatalf("resolve bot instances: %v", err)
	}
	if defaultOwnerBotInstanceID != "main" {
		t.Fatalf("expected default owner main, got %q", defaultOwnerBotInstanceID)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected only main to resolve, got %+v", resolved)
	}
	if resolved[0].ID != "main" || resolved[0].TokenEnv != "MAIN_TOKEN" || resolved[0].Token != "main-token" {
		t.Fatalf("unexpected resolved instance: %+v", resolved[0])
	}
}

func TestResolveBotInstancesUsesFirstAvailableOptionalBotAsDefault(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("MAIN_TOKEN", "")
	t.Setenv("COMPANION_TOKEN", "companion-token")

	resolved, defaultOwnerBotInstanceID, err := resolveBotInstances("", RunOptions{
		BotCatalog: []BotInstanceDefinition{
			{ID: "main", TokenEnv: "MAIN_TOKEN", Optional: true},
			{ID: "companion", TokenEnv: "COMPANION_TOKEN", Optional: true},
		},
	})
	if err != nil {
		t.Fatalf("resolve bot instances: %v", err)
	}
	if defaultOwnerBotInstanceID != "companion" {
		t.Fatalf("expected companion to become the default owner, got %q", defaultOwnerBotInstanceID)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected only companion to resolve, got %+v", resolved)
	}
	if resolved[0].ID != "companion" || resolved[0].TokenEnv != "COMPANION_TOKEN" || resolved[0].Token != "companion-token" {
		t.Fatalf("unexpected resolved instance: %+v", resolved[0])
	}
}

func TestResolveBotInstancesRejectsMissingRequiredToken(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("MAIN_TOKEN", "")

	_, _, err := resolveBotInstances("", RunOptions{
		DefaultOwnerBotInstanceID: "main",
		BotCatalog: []BotInstanceDefinition{{
			ID:       "main",
			TokenEnv: "MAIN_TOKEN",
		}},
	})
	if err == nil {
		t.Fatal("expected missing required token to fail")
	}
	if got := err.Error(); got != "MAIN_TOKEN not set in environment or .env file" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBotInstancesReturnsSentinelWhenNoOptionalTokensAreConfigured(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("MAIN_TOKEN", "")
	t.Setenv("COMPANION_TOKEN", "")

	_, _, err := resolveBotInstances("", RunOptions{
		BotCatalog: []BotInstanceDefinition{
			{ID: "main", TokenEnv: "MAIN_TOKEN", Optional: true},
			{ID: "companion", TokenEnv: "COMPANION_TOKEN", Optional: true},
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
	t.Setenv("MAIN_TOKEN", "main-token")
	t.Setenv("COMPANION_TOKEN", "")

	_, _, err := resolveBotInstances("", RunOptions{
		DefaultOwnerBotInstanceID: "companion",
		BotCatalog: []BotInstanceDefinition{
			{ID: "main", TokenEnv: "MAIN_TOKEN"},
			{ID: "companion", TokenEnv: "COMPANION_TOKEN", Optional: true},
		},
	})
	if err == nil {
		t.Fatal("expected missing default optional bot instance to fail")
	}
	if got := err.Error(); got != `default bot instance "companion" is not present in the runtime catalog` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveBotInstancesAllowsRemoteDefaultWhenDefaultDomainIsUnsupported(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("COMPANION_TOKEN", "companion-token")

	resolved, defaultOwnerBotInstanceID, err := resolveBotInstances("", RunOptions{
		DefaultOwnerBotInstanceID: "main",
		SupportedDomains:     []string{files.BotDomainQOTD},
		BotCatalog: []BotInstanceDefinition{{
			ID:       "companion",
			TokenEnv: "COMPANION_TOKEN",
		}},
	})
	if err != nil {
		t.Fatalf("resolve bot instances: %v", err)
	}
	if defaultOwnerBotInstanceID != "main" {
		t.Fatalf("expected remote default owner main, got %q", defaultOwnerBotInstanceID)
	}
	if len(resolved) != 1 || resolved[0].ID != "companion" {
		t.Fatalf("expected only companion to resolve locally, got %+v", resolved)
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

	err := validateConfiguredBotInstances(cfg, knownBotInstanceCatalog(map[string]*botRuntime{
		"main": {instanceID: "main"},
	}, nil), "main")
	if err == nil {
		t.Fatal("expected validation error for unknown bot instance binding")
	}
}

func TestValidateConfiguredBotInstancesRejectsUnknownDomainBinding(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "missing",
			},
		}},
	}

	err := validateConfiguredBotInstances(cfg, knownBotInstanceCatalog(map[string]*botRuntime{
		"main": {instanceID: "main"},
	}, nil), "main")
	if err == nil {
		t.Fatal("expected validation error for unknown domain bot instance binding")
	}
	if got := err.Error(); got != `guild g1 domain "qotd" references unknown bot instance "missing"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfiguredBotInstancesAllowsKnownRemoteOwners(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
		}},
	}

	err := validateConfiguredBotInstances(cfg, knownBotInstanceCatalog(map[string]*botRuntime{
		"companion": {instanceID: "companion"},
	}, []string{"main"}), "main")
	if err != nil {
		t.Fatalf("expected known remote owners to validate, got %v", err)
	}
}
