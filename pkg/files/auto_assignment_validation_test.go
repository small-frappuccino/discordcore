package files

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")

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

	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input config: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write input config: %v", err)
	}

	mgr := NewConfigManagerWithPath(path)
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

	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	var cfgOnDisk BotConfig
	if err := json.Unmarshal(persisted, &cfgOnDisk); err != nil {
		t.Fatalf("unmarshal persisted config: %v", err)
	}
	if len(cfgOnDisk.Guilds) != 1 {
		t.Fatalf("expected one guild on disk, got %d", len(cfgOnDisk.Guilds))
	}
	if got := cfgOnDisk.Guilds[0].Roles.BoosterRole; got != "booster-role" {
		t.Fatalf("expected booster_role persisted after migration, got=%q", got)
	}
}

func TestConfigManagerSaveConfigRejectsInvalidAutoAssignmentOrder(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "settings.json")
	mgr := NewConfigManagerWithPath(path)
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
