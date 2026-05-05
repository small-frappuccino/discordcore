package files

import (
	"strings"
	"testing"
)

func TestNormalizeAutoAssignmentRoleOrderBackfillsBoosterRole(t *testing.T) {
	cfg := &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				Roles: RolesConfig{
					AutoAssignment: AutoAssignmentConfig{
						Enabled:       true,
						TargetRoleID:  "target-role",
						RequiredRoles: []string{"level-role", "booster-role"},
					},
				},
			},
		},
	}

	changed := normalizeAutoAssignmentRoleOrder(cfg)
	if !changed {
		t.Fatalf("expected migration to backfill booster_role")
	}
	if got := cfg.Guilds[0].Roles.BoosterRole; got != "booster-role" {
		t.Fatalf("unexpected booster_role after migration: got=%q", got)
	}
	if err := validateBotConfig(cfg); err != nil {
		t.Fatalf("expected migrated config to validate, got error: %v", err)
	}
}

func TestValidateBotConfigRejectsAutoAssignmentOrderMismatch(t *testing.T) {
	cfg := &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				Roles: RolesConfig{
					BoosterRole: "booster-role",
					AutoAssignment: AutoAssignmentConfig{
						Enabled:       true,
						TargetRoleID:  "target-role",
						RequiredRoles: []string{"level-role", "other-role"},
					},
				},
			},
		},
	}

	err := validateBotConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for required_roles order mismatch")
	}
	if !strings.Contains(err.Error(), "required_roles[1] must match roles.booster_role") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidateBotConfigRejectsInvalidRequiredRolesLength(t *testing.T) {
	cfg := &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				Roles: RolesConfig{
					BoosterRole: "booster-role",
					AutoAssignment: AutoAssignmentConfig{
						Enabled:       true,
						TargetRoleID:  "target-role",
						RequiredRoles: []string{"level-role", "booster-role", "extra-role"},
					},
				},
			},
		},
	}

	err := validateBotConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for invalid required_roles length")
	}
	if !strings.Contains(err.Error(), "required_roles must contain exactly 2 role IDs") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestConfigManagerLoadConfigMigratesAutoAssignmentBoosterRole(t *testing.T) {
	store := NewMemoryConfigStore()
	input := BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				Roles: RolesConfig{
					AutoAssignment: AutoAssignmentConfig{
						Enabled:       true,
						TargetRoleID:  "target-role",
						RequiredRoles: []string{"level-role", "booster-role"},
					},
				},
			},
		},
	}
	if err := store.Save(&input); err != nil {
		t.Fatalf("seed config store: %v", err)
	}

	mgr := NewConfigManagerWithStore(store)
	if err := mgr.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	gcfg := mgr.GuildConfig("g1")
	if gcfg == nil {
		t.Fatalf("expected guild g1 after load")
	}
	if got := gcfg.Roles.BoosterRole; got != "booster-role" {
		t.Fatalf("expected booster_role migration, got=%q", got)
	}

	persisted, err := store.Load()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if len(persisted.Guilds) != 1 {
		t.Fatalf("expected one guild persisted in store, got %d", len(persisted.Guilds))
	}
	if got := persisted.Guilds[0].Roles.BoosterRole; got != "booster-role" {
		t.Fatalf("expected booster_role persisted after migration, got=%q", got)
	}
}

func TestConfigManagerSaveConfigRejectsInvalidAutoAssignmentOrder(t *testing.T) {
	mgr := NewMemoryConfigManager()
	mgr.config = &BotConfig{
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				Roles: RolesConfig{
					BoosterRole: "booster-role",
					AutoAssignment: AutoAssignmentConfig{
						Enabled:       true,
						TargetRoleID:  "target-role",
						RequiredRoles: []string{"level-role", "different-role"},
					},
				},
			},
		},
	}

	err := mgr.SaveConfig()
	if err == nil {
		t.Fatalf("expected save to fail on invalid auto-assignment order")
	}
	if !strings.Contains(err.Error(), ErrValidationFailed) {
		t.Fatalf("expected validation error, got: %v", err)
	}
}

func TestValidateBotConfigNormalizesDomainBotInstanceBindings(t *testing.T) {
	cfg := &BotConfig{
		Guilds: []GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: " alice ",
			DomainBotInstanceIDs: map[string]string{
				" QOTD ": " companion ",
			},
		}},
	}

	if err := validateBotConfig(cfg); err != nil {
		t.Fatalf("expected domain bot bindings to validate, got: %v", err)
	}
	if got := cfg.Guilds[0].BotInstanceID; got != "alice" {
		t.Fatalf("expected guild bot instance to normalize to alice, got %q", got)
	}
	if got := cfg.Guilds[0].DomainBotInstanceIDs[BotDomainQOTD]; got != "companion" {
		t.Fatalf("expected qotd override to normalize to companion, got %q", got)
	}
}

func TestValidateBotConfigRejectsReservedDomainBotInstanceBinding(t *testing.T) {
	cfg := &BotConfig{
		Guilds: []GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: "alice",
			DomainBotInstanceIDs: map[string]string{
				"default": "companion",
			},
		}},
	}

	err := validateBotConfig(cfg)
	if err == nil {
		t.Fatal("expected validation error for reserved domain binding")
	}
	if !strings.Contains(err.Error(), "use bot_instance_id for the implicit default domain") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestConfigManagerLoadConfigMigratesDomainBotInstanceBindings(t *testing.T) {
	store := NewMemoryConfigStore()
	input := BotConfig{
		Guilds: []GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: " alice ",
			DomainBotInstanceIDs: map[string]string{
				" QOTD ": " companion ",
			},
		}},
	}
	if err := store.Save(&input); err != nil {
		t.Fatalf("seed config store: %v", err)
	}

	mgr := NewConfigManagerWithStore(store)
	if err := mgr.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	gcfg := mgr.GuildConfig("g1")
	if gcfg == nil {
		t.Fatalf("expected guild g1 after load")
	}
	if got := gcfg.BotInstanceID; got != "alice" {
		t.Fatalf("expected guild bot instance normalized to alice, got %q", got)
	}
	if got := gcfg.DomainBotInstanceIDs[BotDomainQOTD]; got != "companion" {
		t.Fatalf("expected qotd override persisted as companion, got %q", got)
	}

	persisted, err := store.Load()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if got := persisted.Guilds[0].BotInstanceID; got != "alice" {
		t.Fatalf("expected persisted guild bot instance normalized to alice, got %q", got)
	}
	if got := persisted.Guilds[0].DomainBotInstanceIDs[BotDomainQOTD]; got != "companion" {
		t.Fatalf("expected persisted qotd override normalized to companion, got %q", got)
	}
}
