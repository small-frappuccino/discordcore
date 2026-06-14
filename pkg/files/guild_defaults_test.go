package files

import (
	"testing"
)

func TestNewMinimalGuildConfigDisablesAllFeatures(t *testing.T) {
	t.Parallel()

	guild := NewMinimalGuildConfig("guild-new")
	cfg := &BotConfig{
		Guilds: []GuildConfig{guild},
	}

	if guild.GuildID != "guild-new" {
		t.Fatalf("expected guild id to be preserved, got %+v", guild)
	}
	if guild.Channels != (ChannelsConfig{}) {
		t.Fatalf("expected minimal guild to avoid channel bootstrap, got %+v", guild.Channels)
	}
	if len(guild.Roles.Allowed) != 0 ||
		guild.Roles.AutoAssignment.Enabled ||
		guild.Roles.AutoAssignment.TargetRoleID != "" ||
		len(guild.Roles.AutoAssignment.RequiredRoles) != 0 ||
		guild.Roles.BoosterRole != "" ||
		guild.Roles.MuteRole != "" {
		t.Fatalf("expected minimal guild to avoid role bootstrap, got %+v", guild.Roles)
	}

	resolved := cfg.ResolveFeatures("guild-new")
	if resolved.Services.Monitoring ||
		resolved.Services.Automod ||
		resolved.Services.Commands ||
		resolved.Logging.AvatarLogging ||
		resolved.Logging.RoleUpdate ||
		resolved.Logging.MemberJoin ||
		resolved.Logging.MemberLeave ||
		resolved.Logging.MessageProcess ||
		resolved.Logging.MessageEdit ||
		resolved.Logging.MessageDelete ||
		resolved.Logging.ReactionMetric ||
		resolved.Logging.AutomodAction ||
		resolved.Logging.ModerationCase ||
		resolved.Logging.CleanAction ||
		resolved.Moderation.Ban ||
		resolved.Moderation.MassBan ||
		resolved.Moderation.Kick ||
		resolved.Moderation.Timeout ||
		resolved.Moderation.Warn ||
		resolved.Moderation.Warnings ||
		resolved.MessageCache.CleanupOnStartup ||
		resolved.MessageCache.DeleteOnLog ||
		resolved.PresenceWatch.Bot ||
		resolved.PresenceWatch.User ||
		resolved.Maintenance.DBCleanup ||
		resolved.Safety.BotRolePermMirror ||
		resolved.Backfill.Enabled ||
		resolved.MuteRole ||
		resolved.StatsChannels ||
		resolved.AutoRoleAssign ||
		resolved.UserPrune {
		t.Fatalf("expected all resolved feature defaults to be disabled, got %+v", resolved)
	}
}

func TestEnsureMinimalGuildConfigPersistsDormantGuild(t *testing.T) {
	t.Parallel()

	store := &MemoryConfigStore{}
	mgr := NewConfigManagerWithStore(store)

	if err := mgr.EnsureMinimalGuildConfig("guild-new"); err != nil {
		t.Fatalf("ensure minimal guild config: %v", err)
	}

	snapshot := mgr.SnapshotConfig()
	if len(snapshot.Guilds) != 1 {
		t.Fatalf("expected one guild in snapshot, got %+v", snapshot.Guilds)
	}
	if resolved := snapshot.ResolveFeatures("guild-new"); resolved.Services.Commands {
		t.Fatalf("expected dormant guild commands feature to be disabled, got %+v", resolved.Services)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load persisted settings: %v", err)
	}
	if len(loaded.Guilds) != 1 {
		t.Fatalf("expected one persisted guild, got %+v", loaded.Guilds)
	}
	if loaded.Guilds[0].GuildID != "guild-new" {
		t.Fatalf("unexpected persisted guild config: %+v", loaded.Guilds[0])
	}
	if resolved := loaded.ResolveFeatures("guild-new"); resolved.Logging.MemberJoin {
		t.Fatalf("expected persisted dormant guild member_join feature to be disabled, got %+v", resolved.Logging)
	}
}

func TestEnsureMinimalGuildConfigPreservesDomainOverridesOnExistingGuild(t *testing.T) {
	t.Parallel()

	store := &MemoryConfigStore{}
	mgr := NewConfigManagerWithStore(store)
	if _, err := mgr.UpdateConfig(func(cfg *BotConfig) error {
		cfg.Guilds = []GuildConfig{{
			GuildID: "guild-existing",
		}}
		return nil
	}); err != nil {
		t.Fatalf("seed existing guild config: %v", err)
	}

	if err := mgr.EnsureMinimalGuildConfig("guild-existing"); err != nil {
		t.Fatalf("ensure minimal guild config: %v", err)
	}

	snapshot := mgr.SnapshotConfig()
	if len(snapshot.Guilds) != 1 {
		t.Fatalf("expected one guild in snapshot, got %+v", snapshot.Guilds)
	}
}

func TestEnsureMinimalGuildConfigPersistsDormantGuildToPostgres(t *testing.T) {
	store := openIsolatedPostgresConfigStore(t)
	mgr := NewConfigManagerWithStore(store)

	if err := mgr.EnsureMinimalGuildConfig("guild-pg"); err != nil {
		t.Fatalf("ensure minimal guild config in postgres: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load postgres-backed config: %v", err)
	}
	if len(loaded.Guilds) != 1 {
		t.Fatalf("expected one persisted guild in postgres, got %+v", loaded.Guilds)
	}
	if loaded.Guilds[0].GuildID != "guild-pg" {
		t.Fatalf("unexpected postgres-backed guild config: %+v", loaded.Guilds[0])
	}
	if resolved := loaded.ResolveFeatures("guild-pg"); resolved.Services.Monitoring || resolved.Services.Commands || resolved.Logging.MemberJoin {
		t.Fatalf("expected postgres-backed dormant guild features to stay disabled, got %+v", resolved)
	}
}

func TestResolveFeaturesDefaultsModerationCommandsEnabledWhenUnset(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{
		Guilds: []GuildConfig{{GuildID: "guild-unset"}},
	}

	resolved := cfg.ResolveFeatures("guild-unset")
	if !resolved.Moderation.Ban ||
		!resolved.Moderation.MassBan ||
		!resolved.Moderation.Kick ||
		!resolved.Moderation.Timeout ||
		!resolved.Moderation.Warn ||
		!resolved.Moderation.Warnings {
		t.Fatalf("expected unset moderation command toggles to default to enabled, got %+v", resolved.Moderation)
	}
}
