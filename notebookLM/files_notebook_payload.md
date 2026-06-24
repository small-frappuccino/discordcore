# Domain Architecture: files

## Layout Topology
```text
files/
├── auto_assignment_validation.go
├── auto_assignment_validation_test.go
├── bot_instances.go
├── channels_config_test.go
├── config_manager_index_test.go
├── config_mutation.go
├── config_snapshot.go
├── config_snapshot_test.go
├── config_store_interfaces.go
├── config_transaction_test.go
├── consts.go
├── custom_rpc.go
├── encryption.go
├── encryption_test.go
├── env.go
├── env_test.go
├── feature_registry.go
├── feature_registry_test.go
├── features.go
├── guild_defaults.go
├── guild_defaults_test.go
├── guild_registration_errors.go
├── json_manager.go
├── json_manager_test.go
├── legacy_guild_config_test.go
├── legacy_moderation_migration_test.go
├── mock_store_test.go
├── notify_subscribers_test.go
├── partner_board.go
├── partner_board_test.go
├── paths.go
├── preferences.go
├── qotd.go
├── qotd_test.go
├── reaction_blocks.go
├── reaction_blocks_test.go
├── role_panel.go
├── role_panel_test.go
├── runtime_config_test.go
├── runtime_webhook_embed_updates.go
├── runtime_webhook_embed_updates_test.go
├── settings_normalization.go
├── types.go
├── types_embeds.go
├── validation_errors.go
├── validation_errors_test.go
└── version.go
```

## Source Stream Aggregation

// === FILE: pkg/files/auto_assignment_validation.go ===
```go
package files

import (
	"fmt"
	"strings"
)

const (
	botDomainCore    = "core"
	botDomainDefault = "default"
)

// normalizeAutoAssignmentRoleOrder backfills explicit ordering anchors for
// legacy configs. The canonical ordering is:
// - required_roles[0] => roleA (stable level role)
// - required_roles[1] => roleB (booster role)
//
// If auto-assignment is enabled and booster_role is empty, we backfill it from
// required_roles[1] when available.
func normalizeAutoAssignmentRoleOrder(cfg *BotConfig) bool {
	if cfg == nil {
		return false
	}

	changed := false
	for i := range cfg.Guilds {
		guild := &cfg.Guilds[i]
		auto := &guild.Roles.AutoAssignment
		if !auto.Enabled || len(auto.RequiredRoles) < 2 {
			continue
		}
		roleB := strings.TrimSpace(auto.RequiredRoles[1])
		if roleB == "" {
			continue
		}
		if strings.TrimSpace(guild.Roles.BoosterRole) == "" {
			guild.Roles.BoosterRole = roleB
			changed = true
		}
	}
	return changed
}

func validateBotConfig(cfg *BotConfig) error {
	if cfg == nil {
		return nil
	}

	for idx := range cfg.Guilds {
		if err := validateGuildAutoAssignmentOrder(&cfg.Guilds[idx], idx); err != nil {
			return fmt.Errorf("validateBotConfig: %w", err)
		}
	}

	return nil
}

func validateGuildAutoAssignmentOrder(guild *GuildConfig, guildIndex int) error {
	if guild == nil {
		return nil
	}

	auto := guild.Roles.AutoAssignment
	if !auto.Enabled {
		return nil
	}

	fieldBase := fmt.Sprintf("guilds[%d].roles.auto_assignment", guildIndex)
	targetRoleID := strings.TrimSpace(auto.TargetRoleID)
	if targetRoleID == "" {
		return NewValidationError(fieldBase+".target_role", auto.TargetRoleID, "target role is required when auto-assignment is enabled")
	}

	if len(auto.RequiredRoles) != 2 {
		return NewValidationError(
			fieldBase+".required_roles",
			auto.RequiredRoles,
			"required_roles must contain exactly 2 role IDs ordered as [roleA(level), roleB(booster)]",
		)
	}

	roleA := strings.TrimSpace(auto.RequiredRoles[0])
	roleB := strings.TrimSpace(auto.RequiredRoles[1])

	if roleA == "" || roleB == "" {
		return NewValidationError(fieldBase+".required_roles", auto.RequiredRoles, "required_roles entries must be non-empty role IDs")
	}
	if roleA == roleB {
		return NewValidationError(fieldBase+".required_roles", auto.RequiredRoles, "required_roles[0] and required_roles[1] must be different roles")
	}
	if roleA == targetRoleID || roleB == targetRoleID {
		return NewValidationError(fieldBase+".required_roles", auto.RequiredRoles, "required_roles cannot include target_role")
	}

	boosterRole := strings.TrimSpace(guild.Roles.BoosterRole)
	if boosterRole == "" {
		return NewValidationError(
			fmt.Sprintf("guilds[%d].roles.booster_role", guildIndex),
			guild.Roles.BoosterRole,
			"booster_role is required when auto-assignment is enabled to enforce required_roles ordering",
		)
	}
	if roleB != boosterRole {
		return NewValidationError(
			fieldBase+".required_roles",
			auto.RequiredRoles,
			fmt.Sprintf("required_roles[1] must match roles.booster_role (%s)", boosterRole),
		)
	}
	if roleA == boosterRole {
		return NewValidationError(fieldBase+".required_roles", auto.RequiredRoles, "required_roles[0] must be the stable level role, not booster_role")
	}

	return nil
}

```

// === FILE: pkg/files/auto_assignment_validation_test.go ===
```go
package files

import (
	"strings"
	"testing"
)

func TestNormalizeAutoAssignmentRoleOrderBackfillsBoosterRole(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	store := &mockConfigStore{}
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

	mgr := NewConfigManagerWithStore(store, nil)
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
	t.Parallel()
	mgr := NewConfigManagerWithStore(&mockConfigStore{}, nil)
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

```

// === FILE: pkg/files/bot_instances.go ===
```go
package files

import (
	"strings"
)

// NormalizeBotInstanceID trims a persisted bot instance identifier.
func NormalizeBotInstanceID(botInstanceID string) string {
	return strings.TrimSpace(botInstanceID)
}

// BelongsToBotInstance reports whether the guild should be handled by the
// provided runtime, which is true if the guild has a configured token for it.
func BelongsToBotInstance(gc GuildConfig, botInstanceID string) bool {
	botInstanceID = NormalizeBotInstanceID(botInstanceID)

	// If the guild has gracefully fallen back due to having NO bot tokens,
	// the magic blank instance handles it.
	if len(gc.BotInstanceTokens) == 0 {
		return botInstanceID == ""
	}

	token, ok := gc.BotInstanceTokens[botInstanceID]
	match := ok && len(token) > 0

	return match
}

// GuildsForBotInstance returns the guild subset assigned to the provided bot instance,
// preserving config order.
func GuildsForBotInstance(cfg *BotConfig, botInstanceID string) []GuildConfig {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	target := NormalizeBotInstanceID(botInstanceID)

	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if BelongsToBotInstance(guild, target) {
			out = append(out, guild)
		}
	}

	return out
}

// GuildsForBotInstanceFeature returns the guild subset assigned to the provided bot instance for a specific feature,
// preserving config order.
func GuildsForBotInstanceFeature(cfg *BotConfig, botInstanceID string, feature string) []GuildConfig {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}

	target := NormalizeBotInstanceID(botInstanceID)

	out := make([]GuildConfig, 0, len(cfg.Guilds))
	for _, guild := range cfg.Guilds {
		if !BelongsToBotInstance(guild, target) {
			continue
		}
		resolvedID, _ := ResolveFeatureBotInstanceID(guild, feature)
		if resolvedID == target {
			out = append(out, guild)
		}
	}

	return out
}

// ResolveFeatureBotInstanceID returns the designated bot instance for a given feature.
// It explicitly parses FeatureRouting and falls back to "".
// It returns the resolved instance ID and a boolean fallbackFlag
// indicating if the designated bot token was revoked, invalid, or missing, necessitating
// a degradation to the default fallback bot.
func ResolveFeatureBotInstanceID(gc GuildConfig, feature string) (resolvedID string, fallback bool) {
	// If the guild has gracefully fallen back due to having NO bot tokens,
	// the magic blank instance handles ALL features.
	if len(gc.BotInstanceTokens) == 0 {
		return "", false
	}

	route, exists := gc.FeatureRouting[feature]
	if !exists || route == "" {
		// If unrouted, return a sentinel so it does not accidentally match
		// the magic blank instance ("").
		return "<unrouted>", false
	}

	token, ok := gc.BotInstanceTokens[route]
	if !ok || len(token) == 0 {
		return "<unrouted>", true
	}

	return route, false
}

```

// === FILE: pkg/files/channels_config_test.go ===
```go
package files

import (
	"encoding/json"
	"testing"
)

func TestChannelsConfigUnmarshalStrictSchema(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"avatar_logging": "c-avatar",
		"role_update": "c-role",
		"member_join": "c-join",
		"member_leave": "c-leave",
		"message_edit": "c-edit",
		"message_delete": "c-delete",
		"automod_action": "c-automod",
		"moderation_case": "c-mod",
		"clean_action": "c-clean",
		"entry_backfill": "c-backfill"
	}`)

	var channels ChannelsConfig
	if err := json.Unmarshal(raw, &channels); err != nil {
		t.Fatalf("unmarshal channels: %v", err)
	}

	if channels.AvatarLogging != "c-avatar" || channels.RoleUpdate != "c-role" {
		t.Fatalf("unexpected user channel mapping: avatar=%q role=%q", channels.AvatarLogging, channels.RoleUpdate)
	}
	if channels.MemberJoin != "c-join" || channels.MemberLeave != "c-leave" {
		t.Fatalf("unexpected member channel mapping: join=%q leave=%q", channels.MemberJoin, channels.MemberLeave)
	}
	if channels.MessageEdit != "c-edit" || channels.MessageDelete != "c-delete" {
		t.Fatalf("unexpected message channel mapping: edit=%q delete=%q", channels.MessageEdit, channels.MessageDelete)
	}
	if channels.AutomodAction != "c-automod" || channels.ModerationCase != "c-mod" || channels.CleanAction != "c-clean" {
		t.Fatalf("unexpected moderation channel mapping: automod=%q moderation=%q clean=%q", channels.AutomodAction, channels.ModerationCase, channels.CleanAction)
	}
	if channels.EntryBackfill != "c-backfill" {
		t.Fatalf("unexpected utility channels: backfill=%q", channels.EntryBackfill)
	}
}

func TestChannelsConfigHelpersStrict(t *testing.T) {
	t.Parallel()

	channels := ChannelsConfig{
		EntryBackfill: "c-backfill",
	}

	if got := channels.BackfillChannelID(); got != "c-backfill" {
		t.Fatalf("expected strict backfill channel, got %q", got)
	}
}

```

// === FILE: pkg/files/config_manager_index_test.go ===
```go
package files

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"golang.org/x/sync/errgroup"
)

func newTestConfigManager(guilds []GuildConfig) *ConfigManager {
	return &ConfigManager{
		config: &BotConfig{Guilds: guilds},
	}
}

func TestGuildConfigIndexHit(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
		{GuildID: "g2"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	cfg := mgr.GuildConfig("g2")
	if cfg == nil || cfg.GuildID != "g2" {
		t.Fatalf("expected guild g2, got %+v", cfg)
	}
}

func TestGuildConfigIndexMiss(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	if cfg := mgr.GuildConfig("missing"); cfg != nil {
		t.Fatalf("expected nil for missing guild, got %+v", cfg)
	}
}

func TestGuildConfigIndexUpdate(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	if err := mgr.AddGuildConfig(GuildConfig{GuildID: "g2"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	if cfg := mgr.GuildConfig("g2"); cfg == nil || cfg.GuildID != "g2" {
		t.Fatalf("expected guild g2 after add, got %+v", cfg)
	}

	mgr.RemoveGuildConfig("g1")
	if cfg := mgr.GuildConfig("g1"); cfg != nil {
		t.Fatalf("expected g1 removed, got %+v", cfg)
	}
}

func TestSnapshotConfigReturnsDefensiveCopy(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{
			GuildID: "g1",
			Channels: ChannelsConfig{
				MessageDelete: "c1",
			},
		},
	})

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) == 0 {
		t.Fatal("expected config snapshot")
	}

	cfg.Guilds[0].Channels.MessageDelete = "modified"

	fresh := mgr.Config()
	if got := fresh.Guilds[0].Channels.MessageDelete; got != "c1" {
		t.Fatalf("expected original channel to remain unchanged, got %q", got)
	}
}

func TestPublishedConfigReadsReuseSnapshot(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{
			GuildID: "g1",
			Channels: ChannelsConfig{
				MessageDelete: "c1",
			},
		},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	firstCfg := mgr.Config()
	secondCfg := mgr.Config()
	if firstCfg == nil || secondCfg == nil {
		t.Fatal("expected published config snapshot")
	}
	if firstCfg != secondCfg {
		t.Fatal("expected Config() to reuse the published snapshot")
	}

	firstGuild := mgr.GuildConfig("g1")
	secondGuild := mgr.GuildConfig("g1")
	if firstGuild == nil || secondGuild == nil {
		t.Fatal("expected published guild config snapshot")
	}
	if firstGuild != secondGuild {
		t.Fatal("expected GuildConfig() to reuse the published snapshot")
	}

	var memStats1, memStats2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memStats1)
	const runs = 10000
	for i := 0; i < runs; i++ {
		_ = mgr.Config()
		_ = mgr.GuildConfig("g1")
	}
	runtime.ReadMemStats(&memStats2)
	avgAllocs := float64(memStats2.Mallocs-memStats1.Mallocs) / float64(runs)
	if avgAllocs >= 0.5 {
		t.Fatalf("expected zero allocations for published config reads, got average %f (total mallocs: %d)", avgAllocs, memStats2.Mallocs-memStats1.Mallocs)
	}
}

func TestGuildConfigIndexDuplicateFix(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
		{GuildID: "g1"},
		{GuildID: "g2"},
	})

	if _, err := mgr.rebuildGuildIndexLocked("test"); err == nil {
		t.Fatalf("expected duplicate error")
	}

	if got := len(mgr.config.Guilds); got != 2 {
		t.Fatalf("expected 2 guilds after dedupe, got %d", got)
	}
	if cfg := mgr.GuildConfig("g1"); cfg == nil {
		t.Fatalf("expected g1 to remain after dedupe")
	}
	if stats := mgr.GuildIndexStats(); stats.Duplicates == 0 {
		t.Fatalf("expected duplicate counter to increment")
	}
}

func TestGuildConfigIndexDedupePersistsOnLoad(t *testing.T) {
	t.Parallel()
	store := &mockConfigStore{}
	raw := &BotConfig{
		Guilds: []GuildConfig{
			{GuildID: "g1"},
			{GuildID: "g1"},
			{GuildID: "g2"},
		},
	}
	if err := store.Save(raw); err != nil {
		t.Fatalf("seed config store: %v", err)
	}

	mgr := NewConfigManagerWithStore(store, nil)
	if err := mgr.LoadConfig(); err != nil {
		t.Fatalf("load config: %v", err)
	}

	updated, err := store.Load()
	if err != nil {
		t.Fatalf("load persisted config: %v", err)
	}
	if got := len(updated.Guilds); got != 2 {
		t.Fatalf("expected 2 guilds after dedupe, got %d", got)
	}
	if stats := mgr.GuildIndexStats(); stats.Duplicates == 0 {
		t.Fatalf("expected duplicate counter to increment")
	}
}

func TestGuildConfigIndexConcurrency(t *testing.T) {
	t.Parallel()
	mgr := newTestConfigManager([]GuildConfig{
		{GuildID: "g1"},
	})
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	eg, ctx := errgroup.WithContext(context.Background())
	readers := 10
	writes := 20

	for i := 0; i < readers; i++ {
		eg.Go(func() error {
			for j := 0; j < 200; j++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				mgr.GuildConfig("g1")
				mgr.GuildConfig("missing")
			}
			return nil
		})
	}

	eg.Go(func() error {
		for i := 0; i < writes; i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			id := fmt.Sprintf("g%02d", i+2)
			if err := mgr.AddGuildConfig(GuildConfig{GuildID: id}); err != nil {
				return err
			}
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrency execution failed: %v", err)
	}

	if cfg := mgr.GuildConfig("g1"); cfg == nil {
		t.Fatalf("expected g1 to remain accessible")
	}
	if stats := mgr.GuildIndexStats(); stats.Rebuilds == 0 {
		t.Fatalf("expected rebuild counter to be non-zero")
	}
}

```

// === FILE: pkg/files/config_mutation.go ===
```go
package files

import (
	"context"
	"fmt"
)

func (mgr *ConfigManager) updateGuildConfig(guildID string, fn func(*GuildConfig) error) error {
	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		guildConfig, err := GuildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.updateGuildConfig: %w", err)
		}
		if fn == nil {
			return nil
		}
		return fn(guildConfig)
	})

	return err
}

// UpdateGuildConfig provides an exported way to modify a guild's config
func (mgr *ConfigManager) UpdateGuildConfig(guildID string, fn func(*GuildConfig) error) error {
	return mgr.updateGuildConfig(guildID, fn)
}

func (mgr *ConfigManager) updateRuntimeConfigScope(scopeGuildID string, fn func(*RuntimeConfig) error) error {
	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		runtimeConfig, err := runtimeConfigForScope(cfg, scopeGuildID)
		if err != nil {
			return fmt.Errorf("ConfigManager.updateRuntimeConfigScope: %w", err)
		}
		if runtimeConfig == nil || fn == nil {
			return nil
		}
		return fn(runtimeConfig)
	})

	return err
}

func runtimeConfigForScope(cfg *BotConfig, scopeGuildID string) (*RuntimeConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	if scopeGuildID == "" {
		return &cfg.RuntimeConfig, nil
	}

	guildConfig, err := GuildConfigByID(cfg, scopeGuildID)
	if err != nil {
		return nil, fmt.Errorf("guild config not found for %s", scopeGuildID)
	}
	return &guildConfig.RuntimeConfig, nil
}

// RevokeBotInstance removes the given instance from the configuration across all guilds,
// provided that its configured token exactly matches the revoked token.
func (mgr *ConfigManager) RevokeBotInstance(instanceID, token string) error {
	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		for i := range cfg.Guilds {
			guild := &cfg.Guilds[i]
			encToken, exists := guild.BotInstanceTokens[instanceID]
			if !exists {
				continue
			}
			if string(encToken) != token {
				continue
			}

			delete(guild.BotInstanceTokens, instanceID)

			if guild.BotInstanceStatuses != nil {
				delete(guild.BotInstanceStatuses, instanceID)
			}

			if guild.FeatureRouting != nil {
				for feature, route := range guild.FeatureRouting {
					if route == instanceID {
						delete(guild.FeatureRouting, feature)
					}
				}
			}
		}
		return nil
	})

	return err
}

```

// === FILE: pkg/files/config_snapshot.go ===
```go
package files

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/errgroup"
)

func (mgr *ConfigManager) currentPublishedSnapshot() *publishedConfigSnapshot {
	if mgr == nil {
		return nil
	}
	if snap := mgr.published.Load(); snap != nil {
		return snap
	}

	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.publishSnapshotLocked()
}

func (mgr *ConfigManager) publishSnapshotLocked() *publishedConfigSnapshot {
	if mgr == nil || mgr.config == nil {
		if mgr != nil {
			mgr.published.Store(nil)
		}
		return nil
	}

	snap := &publishedConfigSnapshot{
		config:     CloneBotConfigPtr(mgr.config),
		guildIndex: cloneGuildIndex(mgr.guildIndex),
	}
	if snap.guildIndex == nil {
		snap.guildIndex = buildReadonlyGuildIndex(snap.config)
	}
	mgr.published.Store(snap)
	return snap
}

func cloneGuildIndex(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for guildID, idx := range in {
		out[guildID] = idx
	}
	return out
}

func buildReadonlyGuildIndex(cfg *BotConfig) map[string]int {
	if cfg == nil || len(cfg.Guilds) == 0 {
		return nil
	}
	index := make(map[string]int, len(cfg.Guilds))
	for i := range cfg.Guilds {
		guildID := cfg.Guilds[i].GuildID
		if guildID == "" {
			continue
		}
		if _, exists := index[guildID]; exists {
			continue
		}
		index[guildID] = i
	}
	return index
}

// SnapshotConfig returns a deep copy of the current bot config for read-only use
// outside the ConfigManager lock.
func (mgr *ConfigManager) SnapshotConfig() BotConfig {
	snap := mgr.currentPublishedSnapshot()
	if snap == nil || snap.config == nil {
		return BotConfig{Guilds: []GuildConfig{}}
	}

	out := cloneBotConfig(*snap.config)
	if out.Guilds == nil {
		out.Guilds = []GuildConfig{}
	}
	return out
}

// ConfigSnapshot is an immutable, read-only representation of the system configuration.
type ConfigSnapshot = BotConfig

// UpdateConfig applies a full-config mutation transactionally and persists the
// result. On error, in-memory state is restored to the previous snapshot.
func (mgr *ConfigManager) UpdateConfig(ctx context.Context, fn func(*BotConfig) error) (BotConfig, error) {
	if mgr == nil {
		return BotConfig{}, fmt.Errorf("config manager is nil")
	}
	mgr.mu.Lock()

	if mgr.config == nil {
		mgr.config = &BotConfig{Guilds: []GuildConfig{}}
	}

	previous := mgr.config
	previousIndex := cloneGuildIndex(mgr.guildIndex)
	next := CloneBotConfigPtr(mgr.config)

	if fn != nil {
		if err := fn(next); err != nil {
			mgr.mu.Unlock()
			return BotConfig{}, fmt.Errorf("ConfigManager.UpdateConfig: %w", err)
		}
	}

	mgr.config = next
	if _, err := mgr.rebuildGuildIndexLocked("update"); err != nil {
		// rebuildGuildIndexLocked already normalizes duplicate guild IDs in memory
		// and emits context-rich logs. The updated config remains canonical.
	}

	if err := mgr.saveConfigLocked(); err != nil {
		mgr.config = previous
		mgr.guildIndex = previousIndex
		mgr.publishSnapshotLocked()
		mgr.mu.Unlock()
		return BotConfig{}, fmt.Errorf("ConfigManager.UpdateConfig: %w", err)
	}

	snapshot := mgr.publishSnapshotLocked()
	mgr.mu.Unlock() // Release lock before notifying subscribers

	// Notify subscribers asynchronously with context propagation
	if err := mgr.notifySubscribers(ctx, previous, snapshot.config); err != nil {
		// We do not rollback the persistence since it was already saved,
		// but we return the propagation error to inform the caller
		return cloneBotConfig(*snapshot.config), fmt.Errorf("ConfigManager.UpdateConfig (propagation error): %w", err)
	}

	if snapshot == nil || snapshot.config == nil {
		return BotConfig{Guilds: []GuildConfig{}}, nil
	}
	return cloneBotConfig(*snapshot.config), nil
}

// AddSubscriber registers a callback to be invoked when the configuration changes.
func (mgr *ConfigManager) AddSubscriber(sub ConfigSubscriber) {
	if mgr == nil {
		return
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.subscribers = append(mgr.subscribers, sub)
}

func (mgr *ConfigManager) notifySubscribers(ctx context.Context, oldCfg, newCfg *BotConfig) error {
	mgr.mu.Lock()
	if len(mgr.subscribers) == 0 {
		mgr.mu.Unlock()
		return nil
	}
	subs := make([]ConfigSubscriber, len(mgr.subscribers))
	copy(subs, mgr.subscribers)
	mgr.mu.Unlock()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(10)

	for _, subscriber := range subs {
		sub := subscriber
		eg.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("subscriber panic intercepted: %v", r)
				}
			}()

			if subErr := sub(egCtx, oldCfg, newCfg); subErr != nil {
				return fmt.Errorf("subscriber execution failed: %w", subErr)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		// Return error so UpdateConfig caller knows synchronization failed
		return err
	}
	return nil
}

func CloneBotConfigPtr(in *BotConfig) *BotConfig {
	if in == nil {
		return nil
	}
	out := cloneBotConfig(*in)
	return &out
}

func cloneGuildConfigPtr(in *GuildConfig) *GuildConfig {
	if in == nil {
		return nil
	}
	out := cloneGuildConfig(*in)
	return &out
}

func cloneBotConfig(in BotConfig) BotConfig {
	return BotConfig{
		ConfigVersion: in.ConfigVersion,
		Guilds:        cloneGuildConfigs(in.Guilds),
		Features:      cloneFeatureToggles(in.Features),
		RuntimeConfig: cloneRuntimeConfig(in.RuntimeConfig),
	}
}

func cloneGuildConfigs(in []GuildConfig) []GuildConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]GuildConfig, 0, len(in))
	for _, guild := range in {
		out = append(out, cloneGuildConfig(guild))
	}
	return out
}

func cloneGuildConfig(in GuildConfig) GuildConfig {
	return GuildConfig{
		GuildID:             in.GuildID,
		ConfigVersion:       in.ConfigVersion,
		FeatureRouting:      cloneStringMap(in.FeatureRouting),
		BotInstanceTokens:   cloneEncryptedStringMap(in.BotInstanceTokens),
		BotInstanceStatuses: cloneStringMap(in.BotInstanceStatuses),
		Features:            cloneFeatureToggles(in.Features),
		Channels:            in.Channels,
		Roles:               cloneRolesConfig(in.Roles),
		Stats:               cloneStatsConfig(in.Stats),
		RolesCacheTTL:       in.RolesCacheTTL,
		MemberCacheTTL:      in.MemberCacheTTL,
		GuildCacheTTL:       in.GuildCacheTTL,
		ChannelCacheTTL:     in.ChannelCacheTTL,
		UserPrune:           cloneUserPruneConfig(in.UserPrune),
		PartnerBoard:        clonePartnerBoardConfig(in.PartnerBoard),
		ReactionBlocks:      cloneReactionBlockConfig(in.ReactionBlocks),
		QOTD:                cloneQOTDConfig(in.QOTD),
		Tickets:             cloneTicketsConfig(in.Tickets),
		RolePanels:          cloneRolePanels(in.RolePanels),
		CustomEmbeds:        cloneCustomEmbeds(in.CustomEmbeds),
		RuntimeConfig:       cloneRuntimeConfig(in.RuntimeConfig),
		LogModerationScope:  in.LogModerationScope,
	}
}

func cloneReactionBlockConfig(in ReactionBlockConfig) ReactionBlockConfig {
	if len(in.Rules) == 0 {
		return ReactionBlockConfig{}
	}
	out := ReactionBlockConfig{Rules: make([]ReactionBlockRuleConfig, 0, len(in.Rules))}
	for _, rule := range in.Rules {
		next := ReactionBlockRuleConfig{
			ReactorUserID: rule.ReactorUserID,
			TargetUserID:  rule.TargetUserID,
		}
		if len(rule.Emojis) > 0 {
			next.Emojis = make([]ReactionBlockEmojiConfig, 0, len(rule.Emojis))
			for _, emoji := range rule.Emojis {
				next.Emojis = append(next.Emojis, emoji)
			}
		}
		out.Rules = append(out.Rules, next)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneEncryptedStringMap(in map[string]EncryptedString) map[string]EncryptedString {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]EncryptedString, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneRuntimeConfig(in RuntimeConfig) RuntimeConfig {
	return RuntimeConfig{
		Database:                     in.Database,
		BotTheme:                     in.BotTheme,
		DisableDBCleanup:             in.DisableDBCleanup,
		DisableMessageLogs:           in.DisableMessageLogs,
		DisableEntryExitLogs:         in.DisableEntryExitLogs,
		DisableReactionLogs:          in.DisableReactionLogs,
		DisableUserLogs:              in.DisableUserLogs,
		DisableCleanLog:              in.DisableCleanLog,
		ModerationLogging:            cloneBoolPtr(in.ModerationLogging),
		PresenceWatchUserID:          in.PresenceWatchUserID,
		PresenceWatchBot:             in.PresenceWatchBot,
		MessageCacheTTLHours:         in.MessageCacheTTLHours,
		MessageDeleteOnLog:           in.MessageDeleteOnLog,
		MessageCacheCleanup:          in.MessageCacheCleanup,
		GlobalMaxWorkers:             in.GlobalMaxWorkers,
		BackfillChannelID:            in.BackfillChannelID,
		BackfillStartDay:             in.BackfillStartDay,
		BackfillInitialDate:          in.BackfillInitialDate,
		DisableBotRolePermMirror:     in.DisableBotRolePermMirror,
		BotRolePermMirrorActorRoleID: in.BotRolePermMirrorActorRoleID,
		WebhookEmbedUpdates:          cloneWebhookEmbedUpdateList(in.WebhookEmbedUpdates),
		WebhookEmbedValidation:       in.WebhookEmbedValidation,
		DisableInteractiveEphemeral:  in.DisableInteractiveEphemeral,
		LogModerationScope:           in.LogModerationScope,
	}
}

func cloneFeatureToggles(in FeatureToggles) FeatureToggles {
	var out FeatureToggles
	for _, spec := range featureRegistry {
		out.SetToggle(spec.ID, cloneBoolPtr(in.LookupToggle(spec.ID)))
	}
	return out
}

func cloneRolesConfig(in RolesConfig) RolesConfig {
	return RolesConfig{
		Allowed:        cloneStringSlice(in.Allowed),
		DashboardRead:  cloneStringSlice(in.DashboardRead),
		DashboardWrite: cloneStringSlice(in.DashboardWrite),
		AutoAssignment: cloneAutoAssignmentConfig(in.AutoAssignment),
		BoosterRole:    in.BoosterRole,
		MuteRole:       in.MuteRole,
	}
}

func cloneAutoAssignmentConfig(in AutoAssignmentConfig) AutoAssignmentConfig {
	return AutoAssignmentConfig{
		Enabled:       in.Enabled,
		TargetRoleID:  in.TargetRoleID,
		RequiredRoles: cloneStringSlice(in.RequiredRoles),
	}
}

func cloneStatsConfig(in StatsConfig) StatsConfig {
	return StatsConfig{
		Channels: cloneStatsChannelConfigs(in.Channels),
	}
}

func cloneStatsChannelConfigs(in []StatsChannelConfig) []StatsChannelConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]StatsChannelConfig, len(in))
	copy(out, in)
	return out
}

func cloneUserPruneConfig(in UserPruneConfig) UserPruneConfig {
	return UserPruneConfig{
		Enabled: in.Enabled,
	}
}

func clonePartnerBoardConfig(in PartnerBoardConfig) PartnerBoardConfig {
	return PartnerBoardConfig{
		Postings: cloneCustomEmbedPostings(in.Postings),
		Template: in.Template,
		Partners: clonePartnerEntries(in.Partners),
	}
}

func cloneCustomEmbedPostings(in []CustomEmbedPostingConfig) []CustomEmbedPostingConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]CustomEmbedPostingConfig, len(in))
	copy(out, in)
	return out
}

func cloneQOTDConfig(in QOTDConfig) QOTDConfig {
	var suppressed []string
	if len(in.SuppressScheduledPublishDatesUTC) > 0 {
		suppressed = append([]string(nil), in.SuppressScheduledPublishDatesUTC...)
	}
	return QOTDConfig{
		VerifiedRoleID:                   in.VerifiedRoleID,
		ActiveDeckID:                     in.ActiveDeckID,
		Decks:                            cloneQOTDDeckConfigs(in.Decks),
		Schedule:                         cloneQOTDPublishScheduleConfig(in.Schedule),
		SuppressScheduledPublishDatesUTC: suppressed,
	}
}

// CloneQOTDConfig deep-copies a QOTDConfig so callers can mutate the result
// without aliasing the source's slices or pointer-valued schedule fields.
func CloneQOTDConfig(in QOTDConfig) QOTDConfig {
	return cloneQOTDConfig(in)
}

func cloneQOTDPublishScheduleConfig(in QOTDPublishScheduleConfig) QOTDPublishScheduleConfig {
	return QOTDPublishScheduleConfig{
		HourUTC:   cloneOptionalInt(in.HourUTC),
		MinuteUTC: cloneOptionalInt(in.MinuteUTC),
	}
}

func cloneQOTDDeckConfigs(in []QOTDDeckConfig) []QOTDDeckConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]QOTDDeckConfig, len(in))
	copy(out, in)
	return out
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneOptionalInt(in *int) *int {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneBoolPtr(in *bool) *bool {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneJSONRawMessage(in json.RawMessage) json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(in))
	copy(out, in)
	return out
}

func cloneTicketsConfig(in TicketsConfig) TicketsConfig {
	var categories []TicketsCategoryConfig
	if len(in.Categories) > 0 {
		categories = make([]TicketsCategoryConfig, len(in.Categories))
		copy(categories, in.Categories)
	}
	return TicketsConfig{
		Enabled:             in.Enabled,
		TranscriptChannelID: in.TranscriptChannelID,
		Categories:          categories,
	}
}

```

// === FILE: pkg/files/config_snapshot_test.go ===
```go
package files

import (
	"reflect"
	"testing"
)

func TestCloneFeatureTogglesRoundtripAllTrue(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	for _, spec := range featureRegistry {
		v := true
		src.SetToggle(spec.ID, &v)
	}

	got := cloneFeatureToggles(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("clone (all true) mismatch:\n  src=%#v\n  got=%#v", src, got)
	}
}

func TestCloneFeatureTogglesRoundtripAllFalse(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	for _, spec := range featureRegistry {
		v := false
		src.SetToggle(spec.ID, &v)
	}

	got := cloneFeatureToggles(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("clone (all false) mismatch:\n  src=%#v\n  got=%#v", src, got)
	}
}

func TestCloneFeatureTogglesRoundtripAllNil(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	got := cloneFeatureToggles(src)
	if !reflect.DeepEqual(got, src) {
		t.Fatalf("clone (all nil) mismatch:\n  src=%#v\n  got=%#v", src, got)
	}
}

func TestCloneFeatureTogglesIsolatesMutation(t *testing.T) {
	t.Parallel()

	var src FeatureToggles
	v := true
	src.SetToggle("moderation.clean", &v)

	clone := cloneFeatureToggles(src)
	falseVal := false
	src.SetToggle("moderation.clean", &falseVal)

	if ptr := clone.LookupToggle("moderation.clean"); ptr == nil || *ptr != true {
		t.Fatalf("expected clone to retain true after src mutation, got %v", ptr)
	}
}

```

// === FILE: pkg/files/config_store_interfaces.go ===
```go
package files

// ConfigLoader defines the read paths for the bot configuration.
type ConfigLoader interface {
	Load() (*BotConfig, error)
	Exists() (bool, error)
}

// ConfigSaver defines the write path for the bot configuration.
type ConfigSaver interface {
	Save(*BotConfig) error
}

// ConfigDescriber provides human-readable context for the config storage mechanism.
type ConfigDescriber interface {
	Describe() string
}

// ConfigStore persists the canonical BotConfig by combining read, write, and descriptor capabilities.
type ConfigStore interface {
	ConfigLoader
	ConfigSaver
	ConfigDescriber
}

```

// === FILE: pkg/files/config_transaction_test.go ===
```go
package files

import (
	"encoding/json"
	"errors"
	"testing"
)

type transactionalTestStore struct {
	cfg     *BotConfig
	saveErr error
}

func (s *transactionalTestStore) Load() (*BotConfig, error) {
	if s == nil || s.cfg == nil {
		return &BotConfig{Guilds: []GuildConfig{}}, nil
	}
	return CloneBotConfigPtr(s.cfg), nil
}

func (s *transactionalTestStore) Save(cfg *BotConfig) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.cfg = CloneBotConfigPtr(cfg)
	return nil
}

func (s *transactionalTestStore) Exists() (bool, error) {
	return s != nil && s.cfg != nil, nil
}

func (s *transactionalTestStore) Describe() string {
	return "transactional-test-store"
}

func newTransactionalTestManager(t *testing.T, cfg *BotConfig, saveErr error) (*ConfigManager, *transactionalTestStore) {
	t.Helper()

	if cfg == nil {
		cfg = &BotConfig{Guilds: []GuildConfig{}}
	}
	if cfg.Guilds == nil {
		cfg.Guilds = []GuildConfig{}
	}

	store := &transactionalTestStore{
		cfg:     CloneBotConfigPtr(cfg),
		saveErr: saveErr,
	}
	mgr := NewConfigManagerWithStore(store, nil)
	mgr.config = CloneBotConfigPtr(cfg)
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	return mgr, store
}

func TestUpdateRuntimeConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		RuntimeConfig: RuntimeConfig{BotTheme: "halloween"},
	}, saveErr)

	_, err := mgr.UpdateRuntimeConfig(func(rc *RuntimeConfig) error {
		rc.BotTheme = "winter"
		return nil
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	if got := mgr.SnapshotConfig().RuntimeConfig.BotTheme; got != "halloween" {
		t.Fatalf("expected runtime config rollback, got %q", got)
	}
}

func TestCreateWebhookEmbedUpdateRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{}, saveErr)

	err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "100",
		WebhookURL: "https://discord.com/api/webhooks/1/token",
		Embed:      json.RawMessage(`{"title":"initial"}`),
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if updates := cfg.RuntimeConfig.NormalizedWebhookEmbedUpdates(); len(updates) != 0 {
		t.Fatalf("expected webhook updates rollback, got %+v", updates)
	}
}

```

// === FILE: pkg/files/consts.go ===
```go
package files

// ## Constants
// This section consolidates all constants into a single declaration for better organization and readability.
const (
	// ## Error Constants
	// Avatar cache errors
	ErrReadCacheFile        = "error reading cache file: %w"
	ErrUnmarshalCache       = "error unmarshalling cache: %w"
	ErrCreateCacheDirectory = "error creating cache directory: %w"
	ErrMarshalCache         = "error marshalling cache: %w"
	ErrSaveCacheFile        = "error saving cache file: %w"
	WarnNoGuildCache        = "ClearForGuild called, but guild has no cache"

	// Configuration and File System errors
	ErrFailedLoadConfig           = "failed to load config: %w"
	ErrCreateConfigDir            = "error creating config directory: %w"
	ErrCreateLogsDir              = "error creating logs directory: %w"
	ErrFailedCheckPerms           = "failed to check permissions: %w"
	ErrCreateConfigFile           = "error creating config file: %w"
	ErrCreateCacheFile            = "error creating cache file: %w"
	ErrFailedResolveConfigPath    = "failed to resolve config path: %w"
	ErrCannotSaveNilConfig        = "cannot save nil config"
	ErrFailedMarshalConfig        = "failed to marshal config: %w"
	ErrProjectRootPathNotFoundMsg = "project root path not found"
	ErrInvalidPath                = "invalid path: %w"
	ErrCreateCacheDir             = "error creating cache directory: %w"

	// Discord API and Guild-related errors
	ErrGuildsNotAccessible  = "%d configured guild(s) could not be accessed"
	ErrGuildInfoFetchMsg    = "error fetching guild info %s: %w"
	ErrNoSuitableChannelMsg = "no suitable channel found in guild %s"
	ErrChannelNotFound      = "channel not found"
	ErrChannelWrongGuild    = "channel does not belong to this guild"
	ErrChannelWrongType     = "channel must be a text channel"
	ErrChannelNoPermissions = "bot lacks permissions to send messages in channel"

	// General errors
	ErrValidationFailed           = "validation failed"
	ErrConfigOperationFailed      = "configuration operation failed"
	ErrDiscordOperationFailed     = "discord operation failed"
	ErrNonRetryable               = "non-retryable error encountered"
	ErrOperationFailed            = "operation failed"
	ErrGlobalLoggerNotInitialized = "global logger not initialized for error handler"
	ErrOnAttempt                  = "error on attempt %d for %s"
	ErrOperationAttemptsFailed    = "operation %s failed after %d attempts. Last error: %w"

	// Error format strings
	ErrFmtNonRetryable                 = "non-retryable error in %s: %w"
	ErrFmtOperationCancelled           = "operation %s cancelled: %w"
	ErrFmtOperationFailedAfterRetries  = "operation %s failed after %d attempts: %w"
	ErrFmtOperationFailed              = "%s failed: %w"
	ErrFmtPanicCriticalOperationFailed = "critical operation %s failed: %v"

	// ## Log Constants
	// Configuration and startup logs
	LogCouldNotFetchGuild     = "Could not fetch guild details: %v"
	LogNoSuitableChannel      = "No suitable channel found in guild %s"
	LogGuildAdded             = "Guild added"
	LogGuildAlreadyConfigured = "Guild already configured, skipping"
	LogMonitorGuild           = "Will monitor this guild"
	LogConfigFileNotFound     = "Config file not found, creating: %s"

	LogNoConfiguredGuilds    = "No configured guilds are assigned to this runtime."
	LogGuildNotAccessible    = "Guild not accessible; skipping"
	LogFoundConfiguredGuilds = "%d configured guild(s) found"

	// Specific loading and saving logs
	LogLoadConfigFailedJoinPaths   = "Failed to join paths: %s, error: %v"
	LogLoadConfigFileNotFound      = "Config file not found at path: %s, initializing default config"
	LogLoadConfigFailedReadFile    = "Failed to read config file at path: %s, error: %v"
	LogLoadConfigFailedUnmarshal   = "Failed to unmarshal config data from path: %s, error: %v"
	LogLoadConfigNoGuilds          = "Loaded config has no guilds defined, path: %s"
	LogLoadConfigSuccess           = "Successfully loaded config from path: %s"
	LogSaveConfigNilConfig         = "Attempted to save nil config"
	LogSaveConfigFailedResolvePath = "Failed to resolve config path: %s, error: %v"
	LogSaveConfigFailedMarshal     = "Failed to marshal config, error: %v"
	LogSaveConfigFailedWriteFile   = "Failed to write config to path: %s, error: %v"
	LogSaveConfigSuccess           = "Successfully saved config to path: %s"

	// General log messages
	MsgOperationRetrying            = "operation failed, retrying"
	MsgOperationSucceededAfterRetry = "operation succeeded after retry"
	MsgOperationFailedAllRetries    = "operation failed after all retries"
	MsgOperationFailedCleanup       = "operation failed, running cleanup"
	MsgCriticalOperationFailed      = "critical operation failed"
)

```

// === FILE: pkg/files/custom_rpc.go ===
```go
package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EnsureCustomRPCFile ensures custom-rpc.json exists and has a valid shape.
func EnsureCustomRPCFile() error {
	return EnsureCustomRPCFileAtPath(GetCustomRPCFilePath())
}

// EnsureCustomRPCFileAtPath ensures custom-rpc.json exists at a custom location.
func EnsureCustomRPCFileAtPath(path string) error {
	if path == "" {
		return fmt.Errorf("custom rpc path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create preferences directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return writeDefaultCustomRPC(path)
		}
		return fmt.Errorf("failed to read custom rpc config: %w", err)
	}

	var tmp CustomRPCConfig
	if json.Unmarshal(data, &tmp) == nil {
		return nil
	}

	return writeDefaultCustomRPC(path)
}

// LoadCustomRPCFile loads custom-rpc.json from the default path.
func LoadCustomRPCFile() (*CustomRPCConfig, error) {
	return LoadCustomRPCFileFromPath(GetCustomRPCFilePath())
}

// LoadCustomRPCFileFromPath loads custom-rpc.json from a custom path.
func LoadCustomRPCFileFromPath(path string) (*CustomRPCConfig, error) {
	cfg := &CustomRPCConfig{Profiles: []CustomRPCProfile{}}
	if path == "" {
		return cfg, fmt.Errorf("custom rpc path is empty")
	}

	jsonManager := &JSONManager{FilePath: path}
	if err := jsonManager.Load(cfg); err != nil {
		return nil, fmt.Errorf("failed to load custom rpc config from %s: %w", path, err)
	}
	return cfg, nil
}

// SaveCustomRPCFile saves custom-rpc.json to the default path.
func SaveCustomRPCFile(config *CustomRPCConfig) error {
	return SaveCustomRPCFileToPath(GetCustomRPCFilePath(), config)
}

// SaveCustomRPCFileToPath saves custom-rpc.json to a custom path.
func SaveCustomRPCFileToPath(path string, config *CustomRPCConfig) error {
	if config == nil {
		return fmt.Errorf("cannot save nil custom rpc config")
	}
	if path == "" {
		return fmt.Errorf("custom rpc path is empty")
	}
	jsonManager := &JSONManager{FilePath: path}
	if err := jsonManager.Save(config); err != nil {
		return fmt.Errorf("failed to save custom rpc config to %s: %w", path, err)
	}
	return nil
}

func writeDefaultCustomRPC(path string) error {
	defaultConfig := CustomRPCConfig{Profiles: []CustomRPCProfile{}}
	configData, err := json.MarshalIndent(defaultConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal custom rpc config: %w", err)
	}
	if err := os.WriteFile(path, configData, 0644); err != nil {
		return fmt.Errorf("failed to write custom rpc config: %w", err)
	}
	return nil
}

```

// === FILE: pkg/files/encryption.go ===
```go
package files

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// TokenHash returns a 16-character SHA-256 hash of the token for deduplication.
func TokenHash(token string) string {
	if token == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:16])
}

// getEncryptionKey derives a 32-byte key from environment variables.
func getEncryptionKey() []byte {
	keys := []string{
		"PASTEBIN_ENCRYPTION_KEY",
		"DISCORDCORE_TOKEN",
		"DISCORD_TOKEN",
		"BOT_TOKEN",
	}
	var secret string
	for _, k := range keys {
		if val := os.Getenv(k); val != "" {
			secret = val
			break
		}
	}
	if secret == "" {
		// Fallback for testing/dev environments.
		secret = "discordcore-default-fallback-salt-super-secret-key-12345"
	}
	hash := sha256.Sum256([]byte(secret))
	return hash[:]
}

// Encrypt encrypts plainText using AES-GCM and returns a base64 encoded ciphertext.
func Encrypt(plainText string) (string, error) {
	if plainText == "" {
		return "", nil
	}
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("Encrypt: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("Encrypt: %w", err)
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("Encrypt: %w", err)
	}
	cipherText := aesGCM.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt decrypts a base64 encoded ciphertext using AES-GCM.
func Decrypt(cipherText string) (string, error) {
	if cipherText == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	key := getEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, actualCipherText := data[:nonceSize], data[nonceSize:]
	plainText, err := aesGCM.Open(nil, nonce, actualCipherText, nil)
	if err != nil {
		return "", fmt.Errorf("Decrypt: %w", err)
	}
	return string(plainText), nil
}

// EncryptedString represents a string that is transparently encrypted/decrypted
// when marshaling/unmarshaling JSON.
type EncryptedString string

// MarshalJSON encrypts the value before marshaling.
func (es EncryptedString) MarshalJSON() ([]byte, error) {
	enc, err := Encrypt(string(es))
	if err != nil {
		return nil, fmt.Errorf("EncryptedString.MarshalJSON: %w", err)
	}
	return json.Marshal(enc)
}

// UnmarshalJSON decrypts the base64 ciphertext during unmarshaling.
// If decryption fails, it falls back to storing the raw string, ensuring backwards
// compatibility and resilience against missing keys.
func (es *EncryptedString) UnmarshalJSON(data []byte) error {
	var val string
	if err := json.Unmarshal(data, &val); err != nil {
		return fmt.Errorf("EncryptedString.UnmarshalJSON: %w", err)
	}
	dec, err := Decrypt(val)
	if err != nil {
		// If the fallback value doesn't contain a dot, it's not a valid Discord
		// token and is likely an encrypted payload that failed to decrypt.
		// Dropping it prevents 4004 Auth Failures from passing ciphertext to the gateway.
		if !strings.Contains(val, ".") {
			*es = ""
			return nil
		}
		// Decryption failed. Fallback to raw string value.
		*es = EncryptedString(val)
		return nil
	}
	*es = EncryptedString(dec)
	return nil
}

```

// === FILE: pkg/files/encryption_test.go ===
```go
package files

import (
	"encoding/json"
	"testing"
)

func TestEncryptionSymmetric(t *testing.T) {
	t.Parallel()

	original := "hello secret credentials"
	cipher, err := Encrypt(original)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	if cipher == original {
		t.Fatalf("cipher matches original, encryption failed to obfuscate")
	}

	decrypted, err := Decrypt(cipher)
	if err != nil {
		t.Fatalf("failed to decrypt: %v", err)
	}

	if decrypted != original {
		t.Errorf("decrypted string mismatch: got %q, want %q", decrypted, original)
	}
}

type testConfigContainer struct {
	Secret EncryptedString `json:"secret"`
}

func TestEncryptedStringJSON(t *testing.T) {
	t.Parallel()

	original := "my-secret-password-xyz"
	container := testConfigContainer{
		Secret: EncryptedString(original),
	}

	data, err := json.Marshal(container)
	if err != nil {
		t.Fatalf("failed to marshal container: %v", err)
	}

	// Verify it does not contain the raw secret in the JSON output
	if jsonStr := string(data); len(jsonStr) == 0 || jsonStr == original {
		t.Fatalf("marshalled JSON contains raw secret: %s", jsonStr)
	}

	var decoded testConfigContainer
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if string(decoded.Secret) != original {
		t.Errorf("unmarshalled secret mismatch: got %q, want %q", decoded.Secret, original)
	}
}

func TestEncryptedStringUnmarshalFallback(t *testing.T) {
	t.Parallel()
	// If unmarshalling raw, unencrypted json, it should fallback to raw value.
	rawJSON := `{"secret": "plain-text-legacy.key"}`
	var decoded testConfigContainer
	if err := json.Unmarshal([]byte(rawJSON), &decoded); err != nil {
		t.Fatalf("failed to unmarshal unencrypted: %v", err)
	}

	if string(decoded.Secret) != "plain-text-legacy.key" {
		t.Errorf("unmarshalled plaintext fallback mismatch: got %q, want %q", decoded.Secret, "plain-text-legacy.key")
	}
}

```

// === FILE: pkg/files/env.go ===
```go
package files

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

// Declared as a var (not const) so tests can override it.
var (
	fallbackEnvPath  = `D:\Users\alice\.local\bin\.env`
	getenvFunc       = os.Getenv
	testEnvOverrides sync.Map
)

func getEnv(key string) string {
	var pcs [16]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, ".Test") {
			var foundVal string
			var found bool
			testEnvOverrides.Range(func(k, v any) bool {
				name := k.(string)
				mainTestName := name
				if idx := strings.Index(name, "/"); idx != -1 {
					mainTestName = name[:idx]
				}
				if strings.HasSuffix(frame.Function, "."+mainTestName) {
					if envMap, ok := v.(map[string]string); ok {
						if val, ok := envMap[key]; ok {
							foundVal = val
							found = true
							return false
						}
					}
				}
				return true
			})
			if found {
				return foundVal
			}
		}
		if !more {
			break
		}
	}
	return getenvFunc(key)
}

// LoadEnvWithLocalBinFallback ensures the specified environment variable is present.
// It always attempts to load a single fallback file located at $HOME/.local/bin/.env
// to populate any variables that are currently missing from the environment (without
// overwriting already-set variables). Then it reads and returns the requested variable.
//
// Behavior:
//   - Does NOT load .env from the current working directory.
//   - Always tries to load "$HOME/.local/bin/.env" if it exists, using non-overwriting semantics.
//   - After attempting the fallback load, returns the value of tokenEnvName if present.
//
// Returns the value of the environment variable when found, or a non-nil error if the
// variable remains unset after the fallback attempt. Errors are descriptive to help callers
// decide how to log or handle the situation.
//
// Callers should pass the exact environment variable name they expect (for example
// "ALICE_BOT_DEVELOPMENT_TOKEN" or a repo-specific token name).
func LoadEnvWithLocalBinFallback(tokenEnvName string) (string, error) {
	envPath := fallbackEnvPath
	if info, statErr := os.Stat(envPath); statErr == nil && !info.IsDir() {
		// godotenv.Load will NOT override variables that are already set.
		godotenv.Load(envPath)
	}

	if v := getEnv(tokenEnvName); v != "" {
		return v, nil
	}

	return "", fmt.Errorf("environment variable %q not set; attempted to load fallback file %s", tokenEnvName, envPath)
}

// EnvBool returns true if the named environment variable is set to a truthy value.
// Accepted truthy values (case-insensitive, trimmed):
// "1", "true", "yes", "y", "on"
func EnvBool(name string) bool {
	v := strings.ToLower(strings.TrimSpace(getEnv(name)))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

// EnvString returns the trimmed value of the environment variable, or def if empty/unset.
func EnvString(name, def string) string {
	v := strings.TrimSpace(getEnv(name))
	if v == "" {
		return def
	}
	return v
}

// EnvInt64 returns the parsed int64 value of the environment variable, or def if empty/unset/invalid.
func EnvInt64(name string, def int64) int64 {
	v := strings.TrimSpace(getEnv(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

```

// === FILE: pkg/files/env_test.go ===
```go
package files

import (
	"os"
	"path/filepath"
	"testing"
)

func setTestEnv(t *testing.T, env map[string]string) {
	testEnvOverrides.Store(t.Name(), env)
	t.Cleanup(func() {
		testEnvOverrides.Delete(t.Name())
	})
}

func TestLoadEnvWithLocalBinFallbackUsesHomeFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	fakeHome := filepath.Join(tmp, "home")
	if err := os.MkdirAll(filepath.Join(fakeHome, ".local", "bin"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	envPath := filepath.Join(fakeHome, ".local", "bin", ".env")
	if err := os.WriteFile(envPath, []byte("TOKEN=fromfile"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	orig := fallbackEnvPath
	fallbackEnvPath = envPath
	t.Cleanup(func() { fallbackEnvPath = orig })

	setTestEnv(t, map[string]string{})

	got, err := LoadEnvWithLocalBinFallback("TOKEN")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got != "fromfile" {
		t.Fatalf("expected value from file, got %q", got)
	}

	// When env already set, file should not override.
	setTestEnv(t, map[string]string{
		"TOKEN": "envwins",
	})
	got, err = LoadEnvWithLocalBinFallback("TOKEN")
	if err != nil || got != "envwins" {
		t.Fatalf("expected existing env to win, got %q err=%v", got, err)
	}
}

func TestEnvHelpers(t *testing.T) {
	t.Parallel()
	setTestEnv(t, map[string]string{
		"BOOL_TRUE":  "YeS",
		"BOOL_FALSE": "0",
		"STR_EMPTY":  "  ",
		"INT_OK":     "42",
		"INT_BAD":    "oops",
	})

	if !EnvBool("BOOL_TRUE") {
		t.Fatalf("expected truthy value")
	}
	if EnvBool("BOOL_FALSE") {
		t.Fatalf("expected falsy value")
	}

	if got := EnvString("STR_EMPTY", "default"); got != "default" {
		t.Fatalf("expected default, got %q", got)
	}

	if got := EnvInt64("INT_OK", 1); got != 42 {
		t.Fatalf("expected 42, got %d", got)
	}
	if got := EnvInt64("INT_BAD", 7); got != 7 {
		t.Fatalf("expected fallback, got %d", got)
	}
}

```

// === FILE: pkg/files/feature_registry.go ===
```go
// Package files owns the canonical feature-toggle registry.
//
// This file intentionally carries only the schema-level data each
// toggle needs (ID, struct Path, Default). Product-facing metadata —
// human label, description, area, tags, editable fields, and the
// associated discord/logging LogEvent — lives in
// pkg/control/features_catalog.go (`featureDefinitions`). The split
// is deliberate: pkg/files is the lowest layer in the dependency
// graph and must not import pkg/control or pkg/discord/logging.
// Pulling UI metadata down would invert layering; introducing a
// third joining layer would just rebuild featureDefinitions under a
// different name. featureDefinitions consumes registry IDs and the
// bijection between the two is locked by a contract test in
// pkg/control/feature_contract_test.go.
package files

// toggleSpec describes one boolean feature toggle. It is the single
// source of truth that drives resolve, clone, defaults, dashboard
// binding, override detection and the per-command enabled check.
//
// Accessor functions replace reflection to ensure compile-time safety
// when interacting with FeatureToggles and ResolvedFeatureToggles.
type toggleSpec struct {
	ID          string
	Default     bool
	Get         func(ft *FeatureToggles) *bool
	Set         func(ft *FeatureToggles, val *bool)
	GetResolved func(rft *ResolvedFeatureToggles) bool
	SetResolved func(rft *ResolvedFeatureToggles, val bool)
}

var featureRegistry = []toggleSpec{
	{
		ID: "services.monitoring", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Services.Monitoring },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Services.Monitoring = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Services.Monitoring },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Services.Monitoring = val },
	},
	{
		ID: "services.commands", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Services.Commands },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Services.Commands = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Services.Commands },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Services.Commands = val },
	},

	// --------------------------------------------------------------------
	// DEPRECATED LOGGING TOGGLES
	// Logging features are implicitly enabled by the presence of a target
	// channel configuration. These registry entries are retained solely to
	// preserve JSON schema deserialization backwards-compatibility.
	// --------------------------------------------------------------------
	{
		ID: "logging.avatar_logging", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.AvatarLogging },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.AvatarLogging = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.AvatarLogging },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.AvatarLogging = val },
	},
	{
		ID: "logging.role_update", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.RoleUpdate },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.RoleUpdate = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.RoleUpdate },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.RoleUpdate = val },
	},
	{
		ID: "logging.member_join", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MemberJoin },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MemberJoin = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MemberJoin },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MemberJoin = val },
	},
	{
		ID: "logging.member_leave", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MemberLeave },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MemberLeave = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MemberLeave },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MemberLeave = val },
	},
	{
		ID: "logging.message_process", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MessageProcess },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MessageProcess = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MessageProcess },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MessageProcess = val },
	},
	{
		ID: "logging.message_edit", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MessageEdit },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MessageEdit = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MessageEdit },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MessageEdit = val },
	},
	{
		ID: "logging.message_delete", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.MessageDelete },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.MessageDelete = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.MessageDelete },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.MessageDelete = val },
	},
	{
		ID: "logging.reaction_metric", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.ReactionMetric },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.ReactionMetric = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.ReactionMetric },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.ReactionMetric = val },
	},
	{
		ID: "logging.automod_action", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.AutomodAction },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.AutomodAction = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.AutomodAction },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.AutomodAction = val },
	},
	{
		ID: "logging.moderation_case", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.ModerationCase },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.ModerationCase = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.ModerationCase },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.ModerationCase = val },
	},
	{
		ID: "logging.clean_action", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Logging.CleanAction },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Logging.CleanAction = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Logging.CleanAction },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Logging.CleanAction = val },
	},
	// --------------------------------------------------------------------
	{
		ID: "moderation.ban", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Ban },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Ban = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Ban },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Ban = val },
	},
	{
		ID: "moderation.massban", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.MassBan },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.MassBan = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.MassBan },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.MassBan = val },
	},
	{
		ID: "moderation.kick", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Kick },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Kick = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Kick },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Kick = val },
	},
	{
		ID: "moderation.timeout", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Timeout },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Timeout = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Timeout },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Timeout = val },
	},
	{
		ID: "moderation.warn", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Warn },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Warn = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Warn },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Warn = val },
	},
	{
		ID: "moderation.warnings", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Warnings },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Warnings = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Warnings },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Warnings = val },
	},
	{
		ID: "moderation.clean", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Moderation.Clean },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Moderation.Clean = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Moderation.Clean },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Moderation.Clean = val },
	},
	{
		ID: "message_cache.cleanup_on_startup", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.MessageCache.CleanupOnStartup },
		Set:         func(ft *FeatureToggles, val *bool) { ft.MessageCache.CleanupOnStartup = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.MessageCache.CleanupOnStartup },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.MessageCache.CleanupOnStartup = val },
	},
	{
		ID: "message_cache.delete_on_log", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.MessageCache.DeleteOnLog },
		Set:         func(ft *FeatureToggles, val *bool) { ft.MessageCache.DeleteOnLog = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.MessageCache.DeleteOnLog },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.MessageCache.DeleteOnLog = val },
	},
	{
		ID: "presence_watch.bot", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.PresenceWatch.Bot },
		Set:         func(ft *FeatureToggles, val *bool) { ft.PresenceWatch.Bot = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.PresenceWatch.Bot },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.PresenceWatch.Bot = val },
	},
	{
		ID: "presence_watch.user", Default: false,
		Get:         func(ft *FeatureToggles) *bool { return ft.PresenceWatch.User },
		Set:         func(ft *FeatureToggles, val *bool) { ft.PresenceWatch.User = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.PresenceWatch.User },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.PresenceWatch.User = val },
	},
	{
		ID: "maintenance.db_cleanup", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Maintenance.DBCleanup },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Maintenance.DBCleanup = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Maintenance.DBCleanup },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Maintenance.DBCleanup = val },
	},
	{
		ID: "safety.bot_role_perm_mirror", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.Safety.BotRolePermMirror },
		Set:         func(ft *FeatureToggles, val *bool) { ft.Safety.BotRolePermMirror = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.Safety.BotRolePermMirror },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.Safety.BotRolePermMirror = val },
	},

	{
		ID: "moderation.mute_role", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.MuteRole },
		Set:         func(ft *FeatureToggles, val *bool) { ft.MuteRole = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.MuteRole },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.MuteRole = val },
	},

	{
		ID: "role_panels", Default: true,
		Get:         func(ft *FeatureToggles) *bool { return ft.RolePanels },
		Set:         func(ft *FeatureToggles, val *bool) { ft.RolePanels = cloneBoolPtr(val) },
		GetResolved: func(rft *ResolvedFeatureToggles) bool { return rft.RolePanels },
		SetResolved: func(rft *ResolvedFeatureToggles, val bool) { rft.RolePanels = val },
	},
}

var featureSpecByID = func() map[string]toggleSpec {
	out := make(map[string]toggleSpec, len(featureRegistry))
	for _, spec := range featureRegistry {
		out[spec.ID] = spec
	}
	return out
}()

// FeatureToggleIDs returns the list of registered toggle IDs in
// declaration order.
func FeatureToggleIDs() []string {
	out := make([]string, len(featureRegistry))
	for i, spec := range featureRegistry {
		out[i] = spec.ID
	}
	return out
}

// FeatureToggleSpec looks up a registered toggle by ID.
func FeatureToggleSpec(id string) (toggleSpec, bool) {
	spec, ok := featureSpecByID[id]
	return spec, ok
}

// LookupToggle returns the *bool stored under the given toggle ID,
// or nil when the toggle is unset or the ID is not registered.
func (ft FeatureToggles) LookupToggle(id string) *bool {
	spec, ok := featureSpecByID[id]
	if !ok {
		return nil
	}
	return cloneBoolPtr(spec.Get(&ft))
}

// SetToggle writes value into the registered toggle. Unknown IDs are
// ignored. The value pointer is copied; callers may reuse it.
func (ft *FeatureToggles) SetToggle(id string, value *bool) {
	spec, ok := featureSpecByID[id]
	if !ok {
		return
	}
	spec.Set(ft, value)
}

// HasAnyOverride reports whether any registered toggle field is set.
// Non-toggle fields on FeatureToggles are not considered.
func (ft FeatureToggles) HasAnyOverride() bool {
	for _, spec := range featureRegistry {
		ptr := spec.Get(&ft)
		if ptr != nil {
			return true
		}
	}
	return false
}

// Lookup returns the resolved bool for the given toggle ID and a
// flag indicating whether the ID is registered.
func (rft ResolvedFeatureToggles) Lookup(id string) (bool, bool) {
	spec, ok := featureSpecByID[id]
	if !ok {
		return false, false
	}
	return spec.GetResolved(&rft), true
}

```

// === FILE: pkg/files/feature_registry_test.go ===
```go
package files

import (
	"encoding/json"

	"reflect"
	"testing"
)

func TestFeatureRegistryIDsAreUnique(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, len(featureRegistry))
	for _, spec := range featureRegistry {
		if _, ok := seen[spec.ID]; ok {
			t.Fatalf("feature registry: duplicate id %q", spec.ID)
		}
		seen[spec.ID] = struct{}{}
	}
}

func TestFeatureRegistryDefaultsMatchLegacyResolveFeatures(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{}
	resolved := cfg.ResolveFeatures("")

	for _, spec := range featureRegistry {
		got, ok := resolved.Lookup(spec.ID)
		if !ok {
			t.Fatalf("feature registry: spec %q not found on resolved toggles", spec.ID)
		}
		if got != spec.Default {
			t.Errorf("feature registry: spec %q default mismatch: got=%v want=%v", spec.ID, got, spec.Default)
		}
	}
}

func TestLookupToggleRoundTrip(t *testing.T) {
	t.Parallel()

	for _, spec := range featureRegistry {
		var ft FeatureToggles
		if ft.LookupToggle(spec.ID) != nil {
			t.Fatalf("spec %q: expected nil before set", spec.ID)
		}

		trueVal := true
		ft.SetToggle(spec.ID, &trueVal)
		if ptr := ft.LookupToggle(spec.ID); ptr == nil || *ptr != true {
			t.Fatalf("spec %q: expected true after set, got %v", spec.ID, ptr)
		}

		falseVal := false
		ft.SetToggle(spec.ID, &falseVal)
		if ptr := ft.LookupToggle(spec.ID); ptr == nil || *ptr != false {
			t.Fatalf("spec %q: expected false after set, got %v", spec.ID, ptr)
		}

		ft.SetToggle(spec.ID, nil)
		if ptr := ft.LookupToggle(spec.ID); ptr != nil {
			t.Fatalf("spec %q: expected nil after clear, got %v", spec.ID, *ptr)
		}
	}
}

func TestHasAnyOverrideDetectsEachToggle(t *testing.T) {
	t.Parallel()

	if (FeatureToggles{}).HasAnyOverride() {
		t.Fatalf("expected HasAnyOverride to be false for zero toggles")
	}

	for _, spec := range featureRegistry {
		var ft FeatureToggles
		v := false
		ft.SetToggle(spec.ID, &v)
		if !ft.HasAnyOverride() {
			t.Fatalf("HasAnyOverride did not detect override for %q", spec.ID)
		}
	}
}

func TestFeatureTogglesJSONRoundTrip(t *testing.T) {
	t.Parallel()

	src := FeatureToggles{}
	for _, spec := range featureRegistry {
		v := true
		src.SetToggle(spec.ID, &v)
	}

	data, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var round FeatureToggles
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !reflect.DeepEqual(src, round) {
		t.Fatalf("json round-trip mismatch:\n  src=%#v\n  got=%#v", src, round)
	}
}

```

// === FILE: pkg/files/features.go ===
```go
package files

import (
	"encoding/json"
	"fmt"
)

// FeatureServiceToggles holds optional overrides for runtime behavior.
// When unset, defaults preserve current behavior.
type FeatureServiceToggles struct {
	Monitoring *bool `json:"monitoring,omitempty"`
	Commands   *bool `json:"commands,omitempty"`
}

// FeatureLoggingToggles overrides individual log-event categories.
//
// Deprecated: Logging features are implicitly enabled when their respective
// channel targets are populated. These boolean toggles remain in the struct
// to preserve Config Schema Evolution JSON parsing compatibility, but they
// are ignored by the runtime logging policy and bot capability resolver.
type FeatureLoggingToggles struct {
	AvatarLogging  *bool `json:"avatar_logging,omitempty"`
	RoleUpdate     *bool `json:"role_update,omitempty"`
	MemberJoin     *bool `json:"member_join,omitempty"`
	MemberLeave    *bool `json:"member_leave,omitempty"`
	MessageProcess *bool `json:"message_process,omitempty"`
	MessageEdit    *bool `json:"message_edit,omitempty"`
	MessageDelete  *bool `json:"message_delete,omitempty"`
	ReactionMetric *bool `json:"reaction_metric,omitempty"`
	AutomodAction  *bool `json:"automod_action,omitempty"`
	ModerationCase *bool `json:"moderation_case,omitempty"`
	CleanAction    *bool `json:"clean_action,omitempty"`
}

// FeatureModerationToggles enables or disables individual moderation commands.
// A nil field leaves that command at its default availability.
type FeatureModerationToggles struct {
	Ban      *bool `json:"ban,omitempty"`
	MassBan  *bool `json:"massban,omitempty"`
	Kick     *bool `json:"kick,omitempty"`
	Timeout  *bool `json:"timeout,omitempty"`
	Warn     *bool `json:"warn,omitempty"`
	Warnings *bool `json:"warnings,omitempty"`
	Clean    *bool `json:"clean,omitempty"`
}

// FeatureMessageCacheToggles controls message-cache maintenance behavior. A nil
// field leaves that behavior at its default.
type FeatureMessageCacheToggles struct {
	CleanupOnStartup *bool `json:"cleanup_on_startup,omitempty"`
	DeleteOnLog      *bool `json:"delete_on_log,omitempty"`
}

// FeaturePresenceWatchToggles selects which presences are watched. A nil field
// leaves that target at its default.
type FeaturePresenceWatchToggles struct {
	Bot  *bool `json:"bot,omitempty"`
	User *bool `json:"user,omitempty"`
}

// FeatureMaintenanceToggles controls background maintenance jobs. A nil field
// leaves the job at its default.
type FeatureMaintenanceToggles struct {
	DBCleanup *bool `json:"db_cleanup,omitempty"`
}

// FeatureSafetyToggles controls safety mechanisms such as mirroring bot role
// permissions. A nil field leaves the mechanism at its default.
type FeatureSafetyToggles struct {
	BotRolePermMirror *bool `json:"bot_role_perm_mirror,omitempty"`
}

// FeatureToggles is the per-guild override surface for optional behavior,
// grouped by domain. Pointer fields are tri-state: nil means inherit the
// default, while a non-nil value forces the feature on or off. Resolve to
// concrete booleans via ResolvedFeatureToggles.
type FeatureToggles struct {
	Services      FeatureServiceToggles       `json:"services,omitempty"`
	Logging       FeatureLoggingToggles       `json:"logging,omitempty"`
	Moderation    FeatureModerationToggles    `json:"moderation,omitempty"`
	MessageCache  FeatureMessageCacheToggles  `json:"message_cache,omitempty"`
	PresenceWatch FeaturePresenceWatchToggles `json:"presence_watch,omitempty"`
	Maintenance   FeatureMaintenanceToggles   `json:"maintenance,omitempty"`
	Safety        FeatureSafetyToggles        `json:"safety,omitempty"`
	MuteRole      *bool                       `json:"mute_role,omitempty"`
	RolePanels    *bool                       `json:"role_panels,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (ft *FeatureToggles) UnmarshalJSON(data []byte) error {
	type alias FeatureToggles
	var parsed alias

	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("FeatureToggles.UnmarshalJSON: %w", err)
	}
	*ft = FeatureToggles(parsed)
	return nil
}

// ResolvedFeatureToggles is FeatureToggles with every tri-state pointer
// collapsed to a concrete boolean by applying defaults. It is the form consumed
// by runtime code that must not deal with nil-means-default semantics.
type ResolvedFeatureToggles struct {
	Services struct {
		Monitoring bool
		Commands   bool
	}
	Logging struct {
		AvatarLogging  bool
		RoleUpdate     bool
		MemberJoin     bool
		MemberLeave    bool
		MessageProcess bool
		MessageEdit    bool
		MessageDelete  bool
		ReactionMetric bool
		AutomodAction  bool
		ModerationCase bool
		CleanAction    bool
	}
	Moderation struct {
		Ban      bool
		MassBan  bool
		Kick     bool
		Timeout  bool
		Warn     bool
		Warnings bool
		Clean    bool
	}
	MessageCache struct {
		CleanupOnStartup bool
		DeleteOnLog      bool
	}
	PresenceWatch struct {
		Bot  bool
		User bool
	}
	Maintenance struct {
		DBCleanup bool
	}
	Safety struct {
		BotRolePermMirror bool
	}
	MuteRole   bool
	RolePanels bool
}

func boolPtr(v bool) *bool {
	return &v
}

func resolveFeatureBool(guildVal *bool, globalVal *bool, def bool) bool {
	if guildVal != nil {
		return *guildVal
	}
	if globalVal != nil {
		return *globalVal
	}
	return def
}

// ResolveFeatures merges global and guild feature toggles with defaults.
func (cfg *BotConfig) ResolveFeatures(guildID string) ResolvedFeatureToggles {
	global := FeatureToggles{}
	if cfg != nil {
		global = cfg.Features
	}

	var guild FeatureToggles
	if cfg != nil && guildID != "" {
		for _, g := range cfg.Guilds {
			if g.GuildID == guildID {
				guild = g.Features
				break
			}
		}
	}

	var out ResolvedFeatureToggles
	for _, spec := range featureRegistry {
		guildPtr := guild.LookupToggle(spec.ID)
		globalPtr := global.LookupToggle(spec.ID)
		resolved := resolveFeatureBool(guildPtr, globalPtr, spec.Default)
		spec.SetResolved(&out, resolved)
	}

	return out
}

```

// === FILE: pkg/files/guild_defaults.go ===
```go
package files

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// NewMinimalGuildConfig returns a dormant guild record for automatic discovery.
// Newly listed guilds keep all feature overrides explicitly disabled until an
// operator configures them.
func NewMinimalGuildConfig(guildID string) GuildConfig {
	disabled := false

	features := FeatureToggles{}
	for _, spec := range featureRegistry {
		// Do not forcefully disable the core command router. If we disable it, the bot
		// strips its own command list out of Discord entirely when joining a new guild.
		if spec.ID == "services.commands" {
			continue
		}
		features.SetToggle(spec.ID, boolPtr(disabled))
	}

	slog.Debug("Granular inspection: Dormant guild configuration structure materialized in memory",
		slog.String("guild_id", guildID),
	)

	return GuildConfig{
		GuildID:  strings.TrimSpace(guildID),
		Features: features,
	}
}

// EnsureMinimalGuildConfigForBot persists a dormant guild record if it does not
// exist yet. Existing guild settings are preserved.
func (mgr *ConfigManager) EnsureMinimalGuildConfig(guildID string) error {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		err := fmt.Errorf("guild id is required")
		log.EmitBlockingError("Blocking structural failure: Guild configuration enforcement aborted due to null identifier", err, log.GenerateRequestID())
		return err
	}

	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}

			slog.Debug("Granular inspection: Guild configuration already resident in operational matrix",
				slog.String("guild_id", guildID),
				slog.Int("matrix_index", idx),
			)
			return nil
		}

		cfg.Guilds = append(cfg.Guilds, NewMinimalGuildConfig(guildID))

		slog.Info("Architectural state transition: Dormant guild node appended to global configuration tree",
			slog.String("guild_id", guildID),
		)

		return nil
	})

	if err != nil {
		errWrap := fmt.Errorf("ensure minimal guild config for %s: %w", guildID, err)
		log.EmitBlockingError("Blocking structural failure: State mutation transaction rejected during guild enforcement", errWrap, log.GenerateRequestID())
		return errWrap
	}
	return nil
}

```

// === FILE: pkg/files/guild_defaults_test.go ===
```go
package files

import (
	"context"
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
		resolved.Safety.BotRolePermMirror {
		t.Fatalf("expected all resolved feature defaults to be disabled, got %+v", resolved)
	}
}

func TestEnsureMinimalGuildConfigPersistsDormantGuild(t *testing.T) {
	t.Parallel()

	store := &mockConfigStore{}
	mgr := NewConfigManagerWithStore(store, nil)

	if err := mgr.EnsureMinimalGuildConfig("guild-new"); err != nil {
		t.Fatalf("ensure minimal guild config: %v", err)
	}

	snapshot := mgr.SnapshotConfig()
	if len(snapshot.Guilds) != 1 {
		t.Fatalf("expected one guild in snapshot, got %+v", snapshot.Guilds)
	}
	if resolved := snapshot.ResolveFeatures("guild-new"); !resolved.Services.Commands {
		t.Fatalf("expected dormant guild commands feature to be enabled by default, got %+v", resolved.Services)
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

	store := &mockConfigStore{}
	mgr := NewConfigManagerWithStore(store, nil)
	if _, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
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
	t.Parallel()
	store := &mockConfigStore{}
	mgr := NewConfigManagerWithStore(store, nil)

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
	if resolved := loaded.ResolveFeatures("guild-pg"); resolved.Services.Monitoring || resolved.Logging.MemberJoin {
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

```

// === FILE: pkg/files/guild_registration_errors.go ===
```go
package files

import "errors"

var (
	// ErrGuildBootstrapPrerequisite indicates bootstrap could not proceed because
	// the guild is missing a required local precondition, such as a writable
	// target channel.
	ErrGuildBootstrapPrerequisite = errors.New("guild bootstrap prerequisite failed")
	// ErrGuildBootstrapDiscordFetch indicates bootstrap could not fetch Discord
	// state required to create a guild config.
	ErrGuildBootstrapDiscordFetch = errors.New("guild bootstrap discord fetch failed")
)

```

// === FILE: pkg/files/json_manager.go ===
```go
package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sync"

	"github.com/small-frappuccino/discordcore/pkg/sys"
)

// JSONManager handles reading and writing JSON data to a file.
type JSONManager struct {
	FilePath    string
	ProjectRoot string // Optional: for safe saving
	mu          sync.Mutex
}

// WithProjectRoot sets the project root for safe saving.
func (m *JSONManager) WithProjectRoot(projectRoot string) *JSONManager {
	m.ProjectRoot = projectRoot
	return m
}

// Load reads the JSON file and unmarshals it into the provided data structure.
func (m *JSONManager) Load(data any) error {
	m.mu.Lock()
	filePath := m.FilePath
	m.mu.Unlock()

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(fileData, data); err != nil {
		return fmt.Errorf("failed to unmarshal json: %w", err)
	}

	return nil
}

// Save marshals the provided data structure and writes it to the JSON file.
func (m *JSONManager) Save(data any) (err error) {
	fileData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}

	m.mu.Lock()
	targetPath := m.FilePath
	projectRoot := m.ProjectRoot
	m.mu.Unlock()

	if projectRoot != "" {
		safePath, err := safeJoin(projectRoot, targetPath)
		if err != nil {
			return fmt.Errorf("failed to resolve safe file path: %w", err)
		}
		targetPath = safePath
	}
	dir := filepath.Dir(targetPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	fileMode := os.FileMode(0o644)
	if info, err := os.Stat(targetPath); err == nil {
		fileMode = info.Mode().Perm()
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to stat target file: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(targetPath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			if rmErr := os.Remove(tmpPath); rmErr != nil && !os.IsNotExist(rmErr) {
				if err != nil {
					err = errors.Join(err, fmt.Errorf("remove stale temp file %q: %w", tmpPath, rmErr))
				} else {
					err = fmt.Errorf("remove stale temp file %q: %w", tmpPath, rmErr)
				}
			}
		}
	}()

	if err := tmpFile.Chmod(fileMode); err != nil {
		retErr := fmt.Errorf("failed to set temp file permissions: %w", err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close temp file after chmod failure: %w", closeErr))
		}
		return retErr
	}
	if _, err := tmpFile.Write(fileData); err != nil {
		retErr := fmt.Errorf("failed to write temp file: %w", err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close temp file after write failure: %w", closeErr))
		}
		return retErr
	}
	if err := tmpFile.Sync(); err != nil {
		retErr := fmt.Errorf("failed to sync temp file: %w", err)
		if closeErr := tmpFile.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close temp file after sync failure: %w", closeErr))
		}
		return retErr
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := sys.ReplaceFile(tmpPath, targetPath); err != nil {
		return fmt.Errorf("failed to replace file atomically: %w", err)
	}
	if err := sys.SyncDir(dir); err != nil {
		return fmt.Errorf("failed to sync parent directory: %w", err)
	}
	cleanupTmp = false

	return nil
}

// safeJoin ensures that the joined path is within the base directory.
func safeJoin(baseDir, relPath string) (string, error) {
	cleanBase := filepath.Clean(baseDir)
	cleanPath := filepath.Join(cleanBase, relPath)
	rel, err := filepath.Rel(cleanBase, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid path: %s", relPath)
	}
	return cleanPath, nil
}

```

// === FILE: pkg/files/json_manager_test.go ===
```go
package files

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONManagerSaveWritesAtomically(t *testing.T) {
	t.Parallel()

	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	path := filepath.Join(t.TempDir(), "payload.json")
	manager := &JSONManager{FilePath: path}

	if err := manager.Save(payload{Name: "first", Count: 1}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := manager.Save(payload{Name: "second", Count: 2}); err != nil {
		t.Fatalf("second save: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}

	var got payload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal saved file: %v", err)
	}
	if got != (payload{Name: "second", Count: 2}) {
		t.Fatalf("unexpected saved payload: %+v", got)
	}

	tmpMatches, err := filepath.Glob(path + ".*.tmp")
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(tmpMatches) != 0 {
		t.Fatalf("expected no temp files left behind, got %v", tmpMatches)
	}
}

```

// === FILE: pkg/files/legacy_guild_config_test.go ===
```go
package files

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGuildConfigLegacyMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		jsonInput  string
		wantTokens []string
	}{
		{
			name:       "migrates bot_instance_id",
			jsonInput:  `{"guild_id": "g1", "bot_instance_id": "generic"}`,
			wantTokens: []string{"generic"},
		},
		{
			name:       "migrates domain_bot_instance_ids",
			jsonInput:  `{"guild_id": "g2", "domain_bot_instance_ids": {"qotd": "generic", "moderation": "admin"}}`,
			wantTokens: []string{"generic", "admin"},
		},
		{
			name:       "combines both legacy fields",
			jsonInput:  `{"guild_id": "g3", "bot_instance_id": "generic", "domain_bot_instance_ids": {"qotd": "generic"}}`,
			wantTokens: []string{"generic"},
		},
		{
			name:       "normalizes legacy names",
			jsonInput:  `{"guild_id": "g4", "bot_instance_id": " Alice ", "domain_bot_instance_ids": {"qotd": "Bob"}}`,
			wantTokens: []string{"Alice", "Bob"},
		},
		{
			name:       "ignores empty fields",
			jsonInput:  `{"guild_id": "g5"}`,
			wantTokens: nil,
		},
		{
			name:       "does not overwrite existing canonical tokens",
			jsonInput:  `{"guild_id": "g6", "bot_instance_id": "generic", "bot_instance_tokens": {"generic": "existing.token"}}`,
			wantTokens: []string{"generic"}, // we should check that "generic" has "existing.token"
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gc GuildConfig
			if err := json.Unmarshal([]byte(tc.jsonInput), &gc); err != nil {
				t.Fatalf("UnmarshalJSON failed: %v", err)
			}

			if len(tc.wantTokens) == 0 {
				if len(gc.BotInstanceTokens) > 0 {
					t.Fatalf("expected empty BotInstanceTokens, got %+v", gc.BotInstanceTokens)
				}
				return
			}

			if len(gc.BotInstanceTokens) != len(tc.wantTokens) {
				t.Fatalf("expected %d tokens, got %d: %+v", len(tc.wantTokens), len(gc.BotInstanceTokens), gc.BotInstanceTokens)
			}

			for _, want := range tc.wantTokens {
				val, exists := gc.BotInstanceTokens[want]
				if !exists {
					t.Errorf("expected BotInstanceTokens to contain key %q", want)
				}
				if tc.name == "does not overwrite existing canonical tokens" && want == "generic" {
					if string(val) != "existing.token" {
						t.Errorf("expected token to remain 'existing.token', got %q", val)
					}
				} else if strings.HasPrefix(tc.name, "migrates legacy") || tc.name == "migrates mixed legacy tokens" {
					// We injected actual tokens in these tests
				} else if string(val) != "" {
					t.Errorf("expected empty token for migrated key %q, got %q", want, val)
				}
			}

			// Validate that legacy fields are NOT marshaled back at the top level
			marshaled, err := json.Marshal(gc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var m map[string]interface{}
			if err := json.Unmarshal(marshaled, &m); err != nil {
				t.Fatalf("Unmarshal of marshaled json failed: %v", err)
			}

			if _, ok := m["bot_instance_id"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'bot_instance_id'")
			}
			if _, ok := m["domain_bot_instance_ids"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'domain_bot_instance_ids'")
			}
			if _, ok := m["main"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'main'")
			}
			if _, ok := m["custom"]; ok {
				t.Errorf("Marshaled JSON should not contain top-level 'custom'")
			}
		})
	}
}

```

// === FILE: pkg/files/legacy_moderation_migration_test.go ===
```go
package files

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBotConfigRoundTripDropsLegacyModerationFields(t *testing.T) {
	t.Parallel()

	const legacyConfig = `{
		"guilds": [
			{
				"guild_id": "g1",
				"rulesets": [
					{
						"id": "ruleset-a",
						"name": "Ruleset A",
						"enabled": true,
						"rules": [
							{
								"id": "rule-a",
								"name": "Rule A",
								"enabled": true,
								"lists": [
									{
										"id": "list-a",
										"name": "List A",
										"type": "generic",
										"blocked_keywords": ["spam"]
									}
								]
							}
						]
					}
				],
				"loose_rules": [
					{
						"id": "rule-b",
						"name": "Rule B",
						"enabled": true,
						"lists": [
							{
								"id": "list-b",
								"name": "List B",
								"type": "generic",
								"blocked_keywords": ["eggs"]
							}
						]
					}
				],
				"blocklist": ["legacy-word"]
			}
		]
	}`

	var cfg BotConfig
	if err := json.Unmarshal([]byte(legacyConfig), &cfg); err != nil {
		t.Fatalf("unmarshal legacy config: %v", err)
	}
	if len(cfg.Guilds) != 1 || cfg.Guilds[0].GuildID != "g1" {
		t.Fatalf("unexpected guilds after unmarshal: %+v", cfg.Guilds)
	}

	encoded, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal migrated config: %v", err)
	}

	output := string(encoded)
	for _, legacyField := range []string{`"rulesets"`, `"loose_rules"`, `"blocklist"`} {
		if strings.Contains(output, legacyField) {
			t.Fatalf("expected migrated config to drop %s, got %s", legacyField, output)
		}
	}
}

```

// === FILE: pkg/files/mock_store_test.go ===
```go
package files

import "sync"

type mockConfigStore struct {
	mu  sync.Mutex
	cfg *BotConfig
}

func (m *mockConfigStore) Load() (*BotConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg == nil {
		return &BotConfig{Guilds: []GuildConfig{}}, nil
	}
	return m.cfg, nil
}

func (m *mockConfigStore) Save(cfg *BotConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cfg = cfg
	return nil
}

func (m *mockConfigStore) Exists() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg != nil, nil
}

func (m *mockConfigStore) Describe() string {
	return "mock://config"
}

```

// === FILE: pkg/files/notify_subscribers_test.go ===
```go
package files

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// Injeção de $N$ assinantes de bloqueio simultâneo onde $N$ supera o limite de errgroup.SetLimit.
func TestNotifySubscribers_ConcurrencyLimitExceeded(t *testing.T) {
	t.Parallel()

	cfgManager := NewConfigManagerWithStore(nil, nil)

	const numSubscribers = 25 // Exceeds errgroup limit of 10
	var activeWorkers int32
	var maxWorkers int32
	var wg sync.WaitGroup

	wg.Add(numSubscribers)

	// Barrier to hold workers until all are ready
	startBarrier := make(chan struct{})

	// Give errgroup tasks a way to notify they have started running
	workerStarted := make(chan struct{}, numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		cfgManager.AddSubscriber(func(ctx context.Context, oldConfig, newConfig *BotConfig) error {
			defer wg.Done()

			current := atomic.AddInt32(&activeWorkers, 1)

			for {
				max := atomic.LoadInt32(&maxWorkers)
				if current > max {
					if atomic.CompareAndSwapInt32(&maxWorkers, max, current) {
						break
					}
				} else {
					break
				}
			}

			workerStarted <- struct{}{}
			<-startBarrier
			atomic.AddInt32(&activeWorkers, -1)
			return nil
		})
	}

	cfg := &BotConfig{}

	// Launch notifySubscribers in background so we can unblock the barrier
	notifyDone := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		defer close(notifyDone)
		return cfgManager.notifySubscribers(egCtx, cfg, cfg)
	})

	// Wait exactly for the errgroup limit (10) of workers to start
	for i := 0; i < 10; i++ {
		<-workerStarted
	}

	// Unblock all workers
	close(startBarrier)

	// Wait for notifications to finish
	<-notifyDone
	if err := eg.Wait(); err != nil {
		t.Fatalf("notifySubscribers failed: %v", err)
	}
	wg.Wait()

	limit := atomic.LoadInt32(&maxWorkers)
	if limit > 10 {
		t.Errorf("expected max concurrency <= 10, got %d", limit)
	}
}

// Disparo de exceções forçadas via panic("simulated") no interior das rotinas de processamento de múltiplos assinantes.
func TestNotifySubscribers_PanicRecovery(t *testing.T) {
	t.Parallel()

	cfgManager := NewConfigManagerWithStore(nil, nil)

	var successCount int32

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		atomic.AddInt32(&successCount, 1)
		return nil
	})

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		panic("simulated")
	})

	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		atomic.AddInt32(&successCount, 1)
		return nil
	})

	cfg := &BotConfig{}
	err := cfgManager.notifySubscribers(context.Background(), cfg, cfg)

	if err == nil || !strings.Contains(err.Error(), "simulated") {
		t.Fatalf("expected panic error, got %v", err)
	}

	// We can't guarantee how many succeeded due to errgroup short-circuiting on first error
	// But it shouldn't crash the test.
}

// Injeção de time.Sleep ou laços infinitos em assinantes combinada com uma janela milissegunda restrita via context.WithTimeout.
func TestNotifySubscribers_ContextTimeoutPreemption(t *testing.T) {
	t.Parallel()

	cfgManager := NewConfigManagerWithStore(nil, nil)

	neverChan := make(chan struct{})
	cfgManager.AddSubscriber(func(ctx context.Context, oldCfg, newCfg *BotConfig) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-neverChan:
			return nil
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := &BotConfig{}
	err := cfgManager.notifySubscribers(ctx, cfg, cfg)

	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}

```

// === FILE: pkg/files/partner_board.go ===
```go
package files

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
)

var (
	// ErrPartnerNotFound indicates no partner matched the requested key.
	ErrPartnerNotFound = errors.New("partner not found")
	// ErrPartnerAlreadyExists indicates a duplicate partner key (name/link).
	ErrPartnerAlreadyExists = errors.New("partner already exists")
	// ErrGuildConfigNotFound indicates requested guild config was not found.
	ErrGuildConfigNotFound = errors.New("guild config not found")
	// ErrInvalidPartnerBoardInput indicates invalid partner board input payload.
	ErrInvalidPartnerBoardInput = errors.New("invalid partner board input")
)

// AddPartnerBoardPosting adds a new posting to the partner board config.
func (mgr *ConfigManager) AddPartnerBoardPosting(guildID string, posting CustomEmbedPostingConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("add partner board posting: %w", invalidPartnerBoardInput("guild_id is required"))
	}
	if posting.IsZero() {
		return invalidPartnerBoardInput("posting cannot be empty")
	}

	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		for _, p := range guildConfig.PartnerBoard.Postings {
			if p.MessageID == posting.MessageID {
				return nil
			}
		}

		if len(guildConfig.PartnerBoard.Postings) >= 50 {
			guildConfig.PartnerBoard.Postings = guildConfig.PartnerBoard.Postings[1:]
		}
		guildConfig.PartnerBoard.Postings = append(guildConfig.PartnerBoard.Postings, CustomEmbedPostingConfig{
			ChannelID:    strings.TrimSpace(posting.ChannelID),
			MessageID:    strings.TrimSpace(posting.MessageID),
			WebhookID:    strings.TrimSpace(posting.WebhookID),
			WebhookToken: strings.TrimSpace(posting.WebhookToken),
		})
		return nil
	})
}

// RemovePartnerBoardPosting removes a posting from the partner board config.
func (mgr *ConfigManager) RemovePartnerBoardPosting(guildID, messageID string) error {
	if guildID == "" {
		return invalidPartnerBoardInput("guild_id is required")
	}
	msgID := strings.TrimSpace(messageID)
	if msgID == "" {
		return invalidPartnerBoardInput("message_id is required")
	}

	return mgr.updateGuildConfig(guildID, func(guildConfig *GuildConfig) error {
		for i, p := range guildConfig.PartnerBoard.Postings {
			if p.MessageID == msgID {
				guildConfig.PartnerBoard.Postings = append(guildConfig.PartnerBoard.Postings[:i], guildConfig.PartnerBoard.Postings[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("%w: message_id=%s", ErrCustomEmbedPostingNotFound, msgID)
	})
}

// RemovePartnerBoardPostings removes multiple postings from the partner board config.
func (mgr *ConfigManager) RemovePartnerBoardPostings(guildID string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	if guildID == "" {
		return invalidPartnerBoardInput("guild_id is required")
	}

	idsToRemove := make(map[string]bool, len(messageIDs))
	for _, id := range messageIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			idsToRemove[trimmed] = true
		}
	}
	if len(idsToRemove) == 0 {
		return nil
	}

	return mgr.updateGuildConfig(guildID, func(guildConfig *GuildConfig) error {
		var kept []CustomEmbedPostingConfig
		for _, p := range guildConfig.PartnerBoard.Postings {
			if !idsToRemove[p.MessageID] {
				kept = append(kept, p)
			}
		}
		guildConfig.PartnerBoard.Postings = kept
		return nil
	})
}

// PartnerBoardTemplate returns the configured board template for a guild.
func (mgr *ConfigManager) PartnerBoardTemplate(guildID string) (PartnerBoardTemplateConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return PartnerBoardTemplateConfig{}, fmt.Errorf("get partner board template: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return PartnerBoardTemplateConfig{}, fmt.Errorf("ConfigManager.PartnerBoardTemplate: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}
	return normalizePartnerBoardTemplate(guildConfig.PartnerBoard.Template), nil
}

// GetPartnerBoardTemplate returns the configured board template for a guild.
func (mgr *ConfigManager) GetPartnerBoardTemplate(guildID string) (PartnerBoardTemplateConfig, error) {
	return mgr.PartnerBoardTemplate(guildID)
}

// SetPartnerBoardTemplate sets the board template for a guild.
func (mgr *ConfigManager) SetPartnerBoardTemplate(guildID string, template PartnerBoardTemplateConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return fmt.Errorf("set partner board template: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	normalized := normalizePartnerBoardTemplate(template)
	if err := mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		guildConfig.PartnerBoard.Template = normalized
		return nil
	}); err != nil {
		return fmt.Errorf("set partner board template: %w", err)
	}
	return nil
}

// PartnerBoard returns target/template/partners using canonical partner ordering.
func (mgr *ConfigManager) PartnerBoard(guildID string) (PartnerBoardConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return PartnerBoardConfig{}, fmt.Errorf("get partner board: %w", invalidPartnerBoardInput("guild_id is required"))
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return PartnerBoardConfig{}, fmt.Errorf("ConfigManager.PartnerBoard: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}

	var postings []CustomEmbedPostingConfig
	if len(guildConfig.PartnerBoard.Postings) > 0 {
		postings = make([]CustomEmbedPostingConfig, len(guildConfig.PartnerBoard.Postings))
		copy(postings, guildConfig.PartnerBoard.Postings)
	}

	partners, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("get partner board: %w", err)
	}

	return PartnerBoardConfig{
		Postings: postings,
		Template: normalizePartnerBoardTemplate(guildConfig.PartnerBoard.Template),
		Partners: clonePartnerEntries(partners),
	}, nil
}

// GetPartnerBoard returns target/template/partners using canonical partner ordering.
func (mgr *ConfigManager) GetPartnerBoard(guildID string) (PartnerBoardConfig, error) {
	return mgr.PartnerBoard(guildID)
}

// ListPartners lists partner records for a guild in canonical deterministic order.
func (mgr *ConfigManager) ListPartners(guildID string) (_ []PartnerEntryConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("list partners: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return nil, invalidPartnerBoardInput("guild_id is required")
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return nil, fmt.Errorf("ConfigManager.ListPartners: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}

	partners, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return nil, fmt.Errorf("ConfigManager.ListPartners: %w", err)
	}
	return clonePartnerEntries(partners), nil
}

// Partner retrieves one partner by name (case-insensitive).
func (mgr *ConfigManager) Partner(guildID, name string) (_ PartnerEntryConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get partner: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return PartnerEntryConfig{}, invalidPartnerBoardInput("guild_id is required")
	}

	targetName := normalizeNameKey(name)
	if targetName == "" {
		return PartnerEntryConfig{}, invalidPartnerBoardInput("name is required")
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return PartnerEntryConfig{}, fmt.Errorf("ConfigManager.Partner: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}

	partners, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
	if err != nil {
		return PartnerEntryConfig{}, fmt.Errorf("ConfigManager.Partner: %w", err)
	}

	idx := findPartnerIndexByNameKey(partners, targetName)
	if idx < 0 {
		return PartnerEntryConfig{}, fmt.Errorf("%w: name=%s", ErrPartnerNotFound, strings.TrimSpace(name))
	}
	return clonePartnerEntry(partners[idx]), nil
}

// GetPartner retrieves one partner by name (case-insensitive).
func (mgr *ConfigManager) GetPartner(guildID, name string) (PartnerEntryConfig, error) {
	return mgr.Partner(guildID, name)
}

// CreatePartner creates a new partner record (dedupe by name/link).
func (mgr *ConfigManager) CreatePartner(guildID string, partner PartnerEntryConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create partner: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidPartnerBoardInput("guild_id is required")
	}

	normalized, err := normalizePartnerEntry(partner)
	if err != nil {
		return fmt.Errorf("ConfigManager.CreatePartner: %w", err)
	}
	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		current, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
		if err != nil {
			return fmt.Errorf("ConfigManager.CreatePartner: %w", err)
		}

		nameKey := normalizeNameKey(normalized.Name)
		if findPartnerIndexByNameKey(current, nameKey) >= 0 {
			return fmt.Errorf("%w: name=%s", ErrPartnerAlreadyExists, normalized.Name)
		}
		linkKey := normalizeLinkKey(normalized.Link)
		if findPartnerIndexByLinkKey(current, linkKey) >= 0 {
			return fmt.Errorf("%w: link=%s", ErrPartnerAlreadyExists, normalized.Link)
		}

		current = append(current, normalized)
		sortPartnersDeterministically(current)
		guildConfig.PartnerBoard.Partners = current
		return nil
	})
}

// UpdatePartner updates one existing partner selected by current name (case-insensitive).
func (mgr *ConfigManager) UpdatePartner(guildID, currentName string, partner PartnerEntryConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update partner: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidPartnerBoardInput("guild_id is required")
	}

	targetNameKey := normalizeNameKey(currentName)
	if targetNameKey == "" {
		return invalidPartnerBoardInput("current_name is required")
	}

	normalized, err := normalizePartnerEntry(partner)
	if err != nil {
		return fmt.Errorf("ConfigManager.UpdatePartner: %w", err)
	}
	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		current, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
		if err != nil {
			return fmt.Errorf("ConfigManager.UpdatePartner: %w", err)
		}

		idx := findPartnerIndexByNameKey(current, targetNameKey)
		if idx < 0 {
			return fmt.Errorf("%w: name=%s", ErrPartnerNotFound, strings.TrimSpace(currentName))
		}

		newNameKey := normalizeNameKey(normalized.Name)
		if dup := findPartnerIndexByNameKey(current, newNameKey); dup >= 0 && dup != idx {
			return fmt.Errorf("%w: name=%s", ErrPartnerAlreadyExists, normalized.Name)
		}
		newLinkKey := normalizeLinkKey(normalized.Link)
		if dup := findPartnerIndexByLinkKey(current, newLinkKey); dup >= 0 && dup != idx {
			return fmt.Errorf("%w: link=%s", ErrPartnerAlreadyExists, normalized.Link)
		}

		current[idx] = normalized
		sortPartnersDeterministically(current)
		guildConfig.PartnerBoard.Partners = current
		return nil
	})
}

// DeletePartner deletes one partner by name (case-insensitive).
func (mgr *ConfigManager) DeletePartner(guildID, name string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete partner: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidPartnerBoardInput("guild_id is required")
	}

	targetNameKey := normalizeNameKey(name)
	if targetNameKey == "" {
		return invalidPartnerBoardInput("name is required")
	}

	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		current, err := canonicalizePartnerEntries(guildConfig.PartnerBoard.Partners)
		if err != nil {
			return fmt.Errorf("ConfigManager.DeletePartner: %w", err)
		}

		idx := findPartnerIndexByNameKey(current, targetNameKey)
		if idx < 0 {
			return fmt.Errorf("%w: name=%s", ErrPartnerNotFound, strings.TrimSpace(name))
		}

		current = slices.Delete(current, idx, idx+1)
		sortPartnersDeterministically(current)
		guildConfig.PartnerBoard.Partners = current
		return nil
	})
}

func normalizeEmbedUpdateTargetConfig(in EmbedUpdateTargetConfig) (EmbedUpdateTargetConfig, error) {
	out := EmbedUpdateTargetConfig{
		Type:       strings.ToLower(strings.TrimSpace(in.Type)),
		MessageID:  strings.TrimSpace(in.MessageID),
		ChannelID:  strings.TrimSpace(in.ChannelID),
		WebhookURL: strings.TrimSpace(in.WebhookURL),
	}

	if out.Type == "" && out.MessageID == "" && out.ChannelID == "" && out.WebhookURL == "" {
		return EmbedUpdateTargetConfig{}, nil
	}
	if out.Type == "" {
		if out.WebhookURL != "" {
			out.Type = EmbedUpdateTargetTypeWebhookMessage
		} else if out.ChannelID != "" {
			out.Type = EmbedUpdateTargetTypeChannelMessage
		}
	}

	if out.MessageID == "" {
		return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("message_id is required")
	}
	if !isAllDigits(out.MessageID) {
		return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("message_id must be numeric")
	}

	switch out.Type {
	case EmbedUpdateTargetTypeWebhookMessage:
		if out.WebhookURL == "" {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("webhook_url is required for type=%s", out.Type)
		}
		if err := validateDiscordWebhookURL(out.WebhookURL); err != nil {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("webhook_url is invalid: %v", err)
		}
		out.ChannelID = ""
	case EmbedUpdateTargetTypeChannelMessage:
		if out.ChannelID == "" {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("channel_id is required for type=%s", out.Type)
		}
		if !isAllDigits(out.ChannelID) {
			return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput("channel_id must be numeric")
		}
		out.WebhookURL = ""
	default:
		return EmbedUpdateTargetConfig{}, invalidPartnerBoardInput(
			"type is invalid (use %s or %s)",
			EmbedUpdateTargetTypeWebhookMessage,
			EmbedUpdateTargetTypeChannelMessage,
		)
	}

	return out, nil
}

func canonicalizePartnerEntries(entries []PartnerEntryConfig) ([]PartnerEntryConfig, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	normalized := make([]PartnerEntryConfig, 0, len(entries))
	for i, entry := range entries {
		n, err := normalizePartnerEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("partner[%d]: %w", i, err)
		}
		normalized = append(normalized, n)
	}

	sortPartnersDeterministically(normalized)

	seenNames := make(map[string]struct{}, len(normalized))
	seenLinks := make(map[string]struct{}, len(normalized))
	deduped := make([]PartnerEntryConfig, 0, len(normalized))
	for _, item := range normalized {
		nameKey := normalizeNameKey(item.Name)
		if _, exists := seenNames[nameKey]; exists {
			continue
		}
		linkKey := normalizeLinkKey(item.Link)
		if _, exists := seenLinks[linkKey]; exists {
			continue
		}
		seenNames[nameKey] = struct{}{}
		seenLinks[linkKey] = struct{}{}
		deduped = append(deduped, item)
	}

	return deduped, nil
}

func normalizePartnerEntry(in PartnerEntryConfig) (PartnerEntryConfig, error) {
	out := PartnerEntryConfig{
		Fandom: sanitizeSingleLine(in.Fandom),
		Name:   sanitizeSingleLine(in.Name),
	}
	if out.Name == "" {
		return PartnerEntryConfig{}, invalidPartnerBoardInput("name is required")
	}

	link, err := normalizeDiscordInviteURL(in.Link)
	if err != nil {
		return PartnerEntryConfig{}, fmt.Errorf("link: %w", err)
	}
	out.Link = link
	return out, nil
}

func normalizeDiscordInviteURL(raw string) (string, error) {
	raw = sanitizeSingleLine(raw)
	if raw == "" {
		return "", invalidPartnerBoardInput("invite URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", invalidPartnerBoardInput("invalid URL")
	}
	if u == nil {
		return "", invalidPartnerBoardInput("invalid URL")
	}
	if scheme := strings.ToLower(strings.TrimSpace(u.Scheme)); scheme != "http" && scheme != "https" {
		return "", invalidPartnerBoardInput("URL scheme must be http or https")
	}

	code, err := extractDiscordInviteCode(u)
	if err != nil {
		return "", fmt.Errorf("normalizeDiscordInviteURL: %w", err)
	}

	// Canonical persisted format for deterministic comparison/output.
	return "https://discord.gg/" + strings.ToLower(code), nil
}

func extractDiscordInviteCode(u *url.URL) (string, error) {
	if u == nil {
		return "", invalidPartnerBoardInput("invalid URL")
	}

	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return "", invalidPartnerBoardInput("URL host is required")
	}

	pathParts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(pathParts) == 0 || strings.TrimSpace(pathParts[0]) == "" {
		return "", invalidPartnerBoardInput("invite code is required")
	}

	var code string
	switch host {
	case "discord.gg", "www.discord.gg":
		code = pathParts[0]
	case "discord.com", "www.discord.com", "ptb.discord.com", "canary.discord.com":
		if len(pathParts) < 2 || pathParts[0] != "invite" {
			return "", invalidPartnerBoardInput("Discord invite URL must match /invite/{code}")
		}
		code = pathParts[1]
	default:
		return "", invalidPartnerBoardInput("URL host must be a Discord invite domain")
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return "", invalidPartnerBoardInput("invite code is required")
	}
	for _, r := range code {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-' || r == '_' {
			continue
		}
		return "", invalidPartnerBoardInput("invite code has invalid characters")
	}

	return code, nil
}

func sortPartnersDeterministically(entries []PartnerEntryConfig) {
	sort.SliceStable(entries, func(i, j int) bool {
		leftFandom := strings.ToLower(entries[i].Fandom)
		rightFandom := strings.ToLower(entries[j].Fandom)
		if leftFandom != rightFandom {
			return leftFandom < rightFandom
		}

		leftName := strings.ToLower(entries[i].Name)
		rightName := strings.ToLower(entries[j].Name)
		if leftName != rightName {
			return leftName < rightName
		}

		leftLink := strings.ToLower(entries[i].Link)
		rightLink := strings.ToLower(entries[j].Link)
		if leftLink != rightLink {
			return leftLink < rightLink
		}

		if entries[i].Fandom != entries[j].Fandom {
			return entries[i].Fandom < entries[j].Fandom
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Link < entries[j].Link
	})
}

func findPartnerIndexByNameKey(entries []PartnerEntryConfig, nameKey string) int {
	if nameKey == "" {
		return -1
	}
	for i, entry := range entries {
		if normalizeNameKey(entry.Name) == nameKey {
			return i
		}
	}
	return -1
}

func findPartnerIndexByLinkKey(entries []PartnerEntryConfig, linkKey string) int {
	if linkKey == "" {
		return -1
	}
	for i, entry := range entries {
		if normalizeLinkKey(entry.Link) == linkKey {
			return i
		}
	}
	return -1
}

func normalizeNameKey(name string) string {
	return strings.ToLower(sanitizeSingleLine(name))
}

func normalizeLinkKey(link string) string {
	return strings.ToLower(strings.TrimSpace(link))
}

func sanitizeSingleLine(in string) string {
	out := strings.TrimSpace(in)
	out = strings.ReplaceAll(out, "\r\n", " ")
	out = strings.ReplaceAll(out, "\n", " ")
	out = strings.ReplaceAll(out, "\r", " ")
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func clonePartnerEntry(in PartnerEntryConfig) PartnerEntryConfig {
	return PartnerEntryConfig{
		Fandom: strings.TrimSpace(in.Fandom),
		Name:   strings.TrimSpace(in.Name),
		Link:   strings.TrimSpace(in.Link),
	}
}

func clonePartnerEntries(in []PartnerEntryConfig) []PartnerEntryConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]PartnerEntryConfig, 0, len(in))
	for _, item := range in {
		out = append(out, clonePartnerEntry(item))
	}
	return out
}

func normalizePartnerBoardTemplate(in PartnerBoardTemplateConfig) PartnerBoardTemplateConfig {
	return PartnerBoardTemplateConfig{
		Title:                      sanitizeSingleLine(in.Title),
		ContinuationTitle:          sanitizeSingleLine(in.ContinuationTitle),
		Intro:                      strings.TrimSpace(in.Intro),
		SectionHeaderTemplate:      strings.TrimSpace(in.SectionHeaderTemplate),
		SectionContinuationSuffix:  strings.TrimSpace(in.SectionContinuationSuffix),
		SectionContinuationPattern: strings.TrimSpace(in.SectionContinuationPattern),
		LineTemplate:               strings.TrimSpace(in.LineTemplate),
		EmptyStateText:             strings.TrimSpace(in.EmptyStateText),
		FooterTemplate:             strings.TrimSpace(in.FooterTemplate),
		OtherFandomLabel:           sanitizeSingleLine(in.OtherFandomLabel),
		Color:                      in.Color,
		DisableFandomSorting:       in.DisableFandomSorting,
		DisablePartnerSorting:      in.DisablePartnerSorting,
	}
}

func invalidPartnerBoardInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidPartnerBoardInput, fmt.Sprintf(format, args...))
}

```

// === FILE: pkg/files/partner_board_test.go ===
```go
package files

import (
	"errors"
	"testing"
)

func newPartnerBoardTestManager(t *testing.T, cfg *BotConfig) *ConfigManager {
	t.Helper()

	if cfg == nil {
		cfg = &BotConfig{Guilds: []GuildConfig{}}
	}
	if cfg.Guilds == nil {
		cfg.Guilds = []GuildConfig{}
	}

	mgr := NewConfigManagerWithStore(&mockConfigStore{}, nil)
	mgr.config = cfg
	if _, err := mgr.rebuildGuildIndexLocked("test"); err != nil {
		t.Fatalf("rebuild index: %v", err)
	}
	return mgr
}

func TestPartnersCRUDAndDeterministicOrder(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Mains",
		Link:   "https://discord.com/invite/JaneMains",
	}); err != nil {
		t.Fatalf("create partner jane: %v", err)
	}
	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/Citlali",
	}); err != nil {
		t.Fatalf("create partner citlali: %v", err)
	}

	list, err := mgr.ListPartners("g1")
	if err != nil {
		t.Fatalf("list partners: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 partners, got %d", len(list))
	}

	// Deterministic order: fandom (asc), then name.
	if list[0].Fandom != "Genshin Impact" || list[0].Name != "Citlali Mains" {
		t.Fatalf("unexpected first partner order: %+v", list[0])
	}
	if list[1].Fandom != "ZZZ" || list[1].Name != "Jane Mains" {
		t.Fatalf("unexpected second partner order: %+v", list[1])
	}

	// Canonical invite format.
	if list[0].Link != "https://discord.gg/citlali" {
		t.Fatalf("unexpected canonical invite for citlali: %q", list[0].Link)
	}
	if list[1].Link != "https://discord.gg/janemains" {
		t.Fatalf("unexpected canonical invite for jane: %q", list[1].Link)
	}

	if err := mgr.UpdatePartner("g1", "Jane Mains", PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Doe Mains",
		Link:   "https://discord.gg/janedoe",
	}); err != nil {
		t.Fatalf("update partner: %v", err)
	}

	got, err := mgr.Partner("g1", "jane doe mains")
	if err != nil {
		t.Fatalf("Partner() failed: %v", err)
	}
	if got.Name != "Jane Doe Mains" {
		t.Fatalf("unexpected updated name: %+v", got)
	}

	if err := mgr.DeletePartner("g1", "Citlali Mains"); err != nil {
		t.Fatalf("delete partner: %v", err)
	}
	afterDelete, err := mgr.ListPartners("g1")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(afterDelete) != 1 || afterDelete[0].Name != "Jane Doe Mains" {
		t.Fatalf("unexpected list after delete: %+v", afterDelete)
	}
}

func TestPartnersValidationAndDedup(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "https://discord.gg/citlali",
	}); err != nil {
		t.Fatalf("create baseline partner: %v", err)
	}

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "citlali mains",
		Link:   "https://discord.gg/citlali2",
	}); !errors.Is(err, ErrPartnerAlreadyExists) {
		t.Fatalf("expected name duplicate error, got %v", err)
	}

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains 2",
		Link:   "https://discord.com/invite/CITLALI",
	}); !errors.Is(err, ErrPartnerAlreadyExists) {
		t.Fatalf("expected link duplicate error, got %v", err)
	}

	if err := mgr.CreatePartner("g1", PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Invalid Invite",
		Link:   "https://example.com/invite/not-discord",
	}); err == nil {
		t.Fatal("expected invalid invite host error")
	}
}

func TestPartnersUpdateDeleteNotFound(t *testing.T) {
	t.Parallel()

	mgr := newPartnerBoardTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	})

	if err := mgr.UpdatePartner("g1", "missing", PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Mains",
		Link:   "https://discord.gg/jane",
	}); !errors.Is(err, ErrPartnerNotFound) {
		t.Fatalf("expected ErrPartnerNotFound on update, got %v", err)
	}

	if err := mgr.DeletePartner("g1", "missing"); !errors.Is(err, ErrPartnerNotFound) {
		t.Fatalf("expected ErrPartnerNotFound on delete, got %v", err)
	}
}

```

// === FILE: pkg/files/paths.go ===
```go
package files

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"strings"

	"github.com/small-frappuccino/discordcore/pkg/sys"
	"github.com/small-frappuccino/discordcore/pkg/theme"
)

// CurrentGitBranch defines current git branch.
// ApplicationCachesPath defines application caches path.
var (
	// ConfiguredAppName can be set by host before Discord auth; when non-empty, EffectiveBotName() uses it.
	ConfiguredAppName string

	// DiscordBotName is set at runtime via SetBotName using the Discord API username.
	// It has no hardcoded default to avoid stale paths; when empty, EffectiveBotName() provides a fallback.
	DiscordBotName string

	// Paths are recalculated when SetBotName or SetAppName is called.
	ApplicationSupportPath string
	ApplicationCachesPath  string

	CurrentGitBranch string
)

func init() {
	// Detect current git branch (best-effort; used for token selection).
	CurrentGitBranch = getCurrentGitBranch()

	// Initialize base paths with a fallback bot name; SetBotName will recompute them once the session is available.
	ApplicationSupportPath = GetApplicationSupportPath(CurrentGitBranch)
	ApplicationCachesPath = GetApplicationCachesPath()
}

func getCurrentGitBranch() string {
	data, err := os.ReadFile(".git/HEAD")
	if err != nil {
		return "unknown"
	}
	line := strings.TrimSpace(string(data))
	if strings.HasPrefix(line, "ref: ") {
		if i := strings.LastIndex(line, "/"); i >= 0 && i+1 < len(line) {
			return line[i+1:]
		}
	}
	return line
}

// GetDiscordBotToken removed.
//
// Token selection by branch and automatic environment lookups were intentionally removed
// from this package to avoid implicit behavior shared across projects. Use
// `LoadEnvWithLocalBinFallback(tokenEnvName)` from this package to load a token from
// environment with the fallback to `$HOME/.local/bin/.env` when needed.

// SetBotName sets the bot name (from Discord API) and recomputes base paths.
// It also attempts a one-time migration of legacy cache files to the new caches location.
func SetBotName(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	DiscordBotName = sanitizeName(name)

	// Recompute base paths now that we have a proper bot name.
	ApplicationSupportPath = GetApplicationSupportPath(CurrentGitBranch)
	ApplicationCachesPath = GetApplicationCachesPath()

}

// SetAppName sets a configured application name and recomputes base paths.
// This allows host applications to control folder names before Discord auth.
func SetAppName(name string) {
	if strings.TrimSpace(name) == "" {
		return
	}
	ConfiguredAppName = sanitizeName(name)

	// Recompute base paths to use configured name.
	ApplicationSupportPath = GetApplicationSupportPath(CurrentGitBranch)
	ApplicationCachesPath = GetApplicationCachesPath()
}

// SetTheme sets the active theme by name. Empty name resets to default.
func SetTheme(name string) error {
	if strings.TrimSpace(name) == "" {
		return theme.SetCurrent("")
	}
	return theme.SetCurrent(name)
}

// ConfigureThemeFromConfig loads theme from the persisted runtime config (bot_theme), if set.
// This replaces the previous environment-variable based theme selection.
func ConfigureThemeFromConfig(themeName string) error {
	if strings.TrimSpace(themeName) != "" {
		return theme.SetCurrent(themeName)
	}
	return nil
}

// EffectiveBotName returns the current application/bot name, preferring a configured
// name when available, otherwise falling back to the Discord username, then a default.
func EffectiveBotName() string {
	// Prefer explicitly configured app name.
	if n := strings.TrimSpace(ConfiguredAppName); n != "" {
		return n
	}
	// Fallback to Discord-provided bot username.
	if n := strings.TrimSpace(DiscordBotName); n != "" {
		return n
	}
	// Final fallback.
	return "discordmain"
}

// GetApplicationSupportPath returns the base path for configuration files using the unified OS rules:
//   - Linux/Unix:  ~/.config/<AppName>
//   - macOS:       ~/Library/Preferences/<AppName>
//   - Windows:     %APPDATA%/<AppName>
func GetApplicationSupportPath(_ string) string {
	app := EffectiveBotName()
	if dir := strings.TrimSpace(sys.PlatformConfigDir(app)); dir != "" {
		return dir
	}
	// Last-resort fallback if platform resolution fails unexpectedly.
	return filepath.Join(".", "config", app)
}

// GetApplicationCachesPath returns the base path for cache files using the unified OS rules:
//   - Linux/Unix:  ~/.cache/<AppName>
//   - macOS:       ~/Library/Caches/<AppName>
//   - Windows:     %APPDATA%/<AppName>/Cache
func GetApplicationCachesPath() string {
	app := EffectiveBotName()
	if dir := strings.TrimSpace(sys.PlatformCacheDir(app)); dir != "" {
		return dir
	}
	// Last-resort fallback if platform resolution fails unexpectedly.
	return filepath.Join(".", "cache", app)
}

// Deprecated: MigrationCacheFilePath returns the path to the avatar cache JSON used only for migration.
func MigrationCacheFilePath() string {
	return filepath.Join(ApplicationCachesPath, "avatar", "avatar_cache.json")
}

// Deprecated: LegacyMigrationCacheFilePath returns the previous JSON cache path, used only for migration.
func LegacyMigrationCacheFilePath() string {
	return filepath.Join(ApplicationSupportPath, "data", "application_cache.json")
}

// GetCustomRPCFilePath returns the path for the custom Discord RPC JSON.
// Layout: <ConfigBase>/preferences/custom-rpc.json
func GetCustomRPCFilePath() string {
	return filepath.Join(ApplicationSupportPath, "preferences", "custom-rpc.json")
}

// GetLogFilePath returns the path to the main log file using the unified OS rules:
//   - Linux/Unix:  ~/.log/<AppName>/discordcore.log
//   - macOS:       ~/Library/Logs/<AppName>/discordcore.log
//   - Windows:     %APPDATA%/<AppName>/Logs/discordcore.log
func GetLogFilePath() string {
	app := EffectiveBotName()
	base := strings.TrimSpace(sys.PlatformLogDir(app))
	if base == "" {
		base = filepath.Join(".", "logs", app)
	}
	return filepath.Join(base, "discordcore.log")
}

// EnsureCacheDirs creates base cache directories as needed.
// Safe to call multiple times.
func EnsureCacheDirs() error {
	dirs := []string{
		filepath.Join(ApplicationCachesPath, "avatar"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create cache directory %s: %w", d, err)
		}
	}
	return nil
}

func removeDirIfEmpty(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("removeDirIfEmpty: %w", err)
	}
	defer f.Close()
	entries, err := f.ReadDir(1)
	if err != nil && err != io.EOF {
		return fmt.Errorf("removeDirIfEmpty: %w", err)
	}
	// If we got at least one entry, it's not empty.
	if len(entries) > 0 {
		return nil
	}
	return os.Remove(dir)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func homeDir() string {
	// Deprecated for base path resolution: kept for any legacy callers.
	// Prefer OS-specific resolution via platformConfigDir/platformCacheDir/platformLogDir.
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	// Prefer UserHomeDir when HOME isn't set (notably on Windows).
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	// Fallback to current working directory if no better option is available.
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return "."
}

func sanitizeName(s string) string {
	// Keep it simple: trim spaces and replace slashes to avoid path issues.
	out := strings.TrimSpace(s)
	out = strings.ReplaceAll(out, "/", "-")
	out = strings.ReplaceAll(out, string(filepath.Separator), "-")
	if out == "" {
		return "DiscordBot"
	}
	return out
}

// EnsureCacheInitialized creates the minimal cache structure if it is not present.
// It is safe to call multiple times.
func EnsureCacheInitialized() error {
	dirs := []string{
		filepath.Join(ApplicationCachesPath, "avatar"), // avatar cache (kept for additional runtime artifacts)
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("failed to create cache directory %s: %w", d, err)
		}
	}

	// Best-effort marker file so we can detect initialization later (ignore errors).
	os.WriteFile(filepath.Join(ApplicationCachesPath, "avatar", ".keep"), []byte{}, 0o644)

	return nil
}

```

// === FILE: pkg/files/preferences.go ===
```go
package files

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/small-frappuccino/discordgo"
)

// log.GenerateRequestID creates a unique transient identifier for error correlation.
func GenerateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(bytes)
}

// log.EmitBlockingError encapsulates the emission of structural failures with mandatory metadata.
func EmitBlockingError(logger *slog.Logger, msg string, err error, requestID string) {
	if logger == nil {
		logger = slog.Default()
	}
	logger.Error(msg,
		slog.String("request_id", requestID),
		slog.String("synthetic_code", "500"),
		slog.Any("error", err),
	)
}

// --- Initialization and Persistence ---

// NewConfigManagerWithStore instantiates a new config manager backed by the
// provided persistence layer.
func NewConfigManagerWithStore(store ConfigStore, logger *slog.Logger) *ConfigManager {
	if logger == nil {
		logger = slog.Default()
	}
	description := ""
	if store != nil {
		description = store.Describe()
	}
	return &ConfigManager{
		configFilePath: description,
		store:          store,
		logger:         logger,
	}
}

// log returns the configured logger or a default logger.
func (mgr *ConfigManager) log() *slog.Logger {
	if mgr == nil || mgr.logger == nil {
		return slog.Default()
	}
	return mgr.logger
}

// LoadConfigFromStore performs an atomic read and validation of the configuration
// on the persistence layer without mutating the active manager state.
func (mgr *ConfigManager) LoadConfigFromStore() (*BotConfig, bool, error) {
	if mgr.store == nil {
		err := fmt.Errorf("config store is not configured")
		EmitBlockingError(mgr.log(), "Failed to initialize configuration read", err, GenerateRequestID())
		return nil, false, err
	}
	cfg, err := mgr.store.Load()
	if err != nil {
		errWrap := fmt.Errorf("load configuration from %s: %w", mgr.ConfigPath(), err)
		EmitBlockingError(mgr.log(), "Structural failure in file loading", errWrap, GenerateRequestID())
		return nil, false, errWrap
	}

	orderMigrated := normalizeAutoAssignmentRoleOrder(cfg)

	if validationErr := validateBotConfig(cfg); validationErr != nil {
		errWrap := wrapValidationError(validationErr)
		EmitBlockingError(mgr.log(), "Validation failure of loaded configuration", errWrap, GenerateRequestID())
		return nil, false, errWrap
	}
	return cfg, orderMigrated, nil
}

// ApplyConfig atomically rotates the global configuration pointer and rebuilds indexes.
func (mgr *ConfigManager) ApplyConfig(cfg *BotConfig) int {
	if cfg == nil {
		return 0
	}
	mgr.mu.Lock()

	mgr.log().Debug("Starting atomic transition of configuration state",
		slog.Int("guilds_payload_size", len(cfg.Guilds)),
	)

	oldCfg := mgr.config
	mgr.config = cfg

	if len(mgr.config.Guilds) == 0 {
		mgr.log().Warn("Applied configuration does not contain active guilds. Running in basal mode.",
			slog.String("path", mgr.ConfigPath()),
		)
	}

	dupCount, err := mgr.rebuildGuildIndexLocked("apply")
	if err != nil {
		mgr.log().Warn("Mitigated degradation in index rebuilding",
			slog.String("error", err.Error()),
			slog.String("path", mgr.ConfigPath()),
		)
	}

	mgr.publishSnapshotLocked()
	mgr.mu.Unlock()

	mgr.notifySubscribers(context.Background(), oldCfg, cfg)

	mgr.log().Info("Configuration state transition completed",
		slog.Int("duplicates_removed", dupCount),
	)
	return dupCount
}

// LoadConfig loads the configuration directly from the filesystem.
func (mgr *ConfigManager) LoadConfig() error {
	cfg, orderMigrated, err := mgr.LoadConfigFromStore()
	if err != nil {
		return err
	}

	dupCount := mgr.ApplyConfig(cfg)

	if dupCount > 0 || orderMigrated {
		mgr.log().Debug("Structural anomaly resolved in memory, forcing compensatory persistence",
			slog.Bool("order_migrated", orderMigrated),
			slog.Int("duplicates", dupCount),
		)
		if saveErr := mgr.SaveConfig(); saveErr != nil {
			errWrap := fmt.Errorf("save configuration after normalization: %w", saveErr)
			EmitBlockingError(mgr.log(), "Failed to write structural corrections to configuration", errWrap, GenerateRequestID())
			return errWrap
		}
		mgr.log().Info("Configuration re-persisted after runtime normalization",
			slog.String("path", mgr.ConfigPath()),
			slog.Int("duplicates", dupCount),
			slog.Bool("autoRoleOrderMigrated", orderMigrated),
		)
	} else if exists, err := mgr.store.Exists(); err == nil && !exists {
		mgr.log().Info("Initialized in clean state: primary file not detected",
			slog.String("path", mgr.ConfigPath()),
		)
	}
	return nil
}

// SaveConfig persists the active configuration to the filesystem.
func (mgr *ConfigManager) SaveConfig() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if err := mgr.saveConfigLocked(); err != nil {
		errWrap := fmt.Errorf("ConfigManager.SaveConfig: %w", err)
		EmitBlockingError(mgr.log(), "Blocking global persistence failure", errWrap, GenerateRequestID())
		return errWrap
	}
	mgr.publishSnapshotLocked()
	return nil
}

// SaveGuildConfig updates a specific guild configuration and persists the change immediately.
func (mgr *ConfigManager) SaveGuildConfig(cfg GuildConfig) error {
	mgr.log().Debug("Updating granular guild state",
		slog.String("guildID", cfg.GuildID),
	)
	if err := mgr.AddGuildConfig(cfg); err != nil {
		return fmt.Errorf("failed to update in-memory configuration: %w", err)
	}
	if err := mgr.SaveConfig(); err != nil {
		return fmt.Errorf("failed to persist guild configuration: %w", err)
	}
	return nil
}

func (mgr *ConfigManager) saveConfigLocked() error {
	if mgr.config == nil {
		return errors.New(ErrCannotSaveNilConfig)
	}
	if mgr.store == nil {
		return fmt.Errorf("config store is not configured")
	}
	if validationErr := validateBotConfig(mgr.config); validationErr != nil {
		return wrapValidationError(validationErr)
	}

	if err := mgr.store.Save(mgr.config); err != nil {
		return fmt.Errorf("save configuration for %s: %w", mgr.ConfigPath(), err)
	}

	mgr.log().Info("I/O state transition: Configuration successfully persisted",
		slog.String("path", mgr.ConfigPath()),
	)

	return nil
}

// UpdateRuntimeConfig mutates runtime_config and persists the change to disk.
func (mgr *ConfigManager) UpdateRuntimeConfig(fn func(*RuntimeConfig) error) (RuntimeConfig, error) {
	snapshot, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		if fn == nil {
			return nil
		}
		return fn(&cfg.RuntimeConfig)
	})

	if err != nil {
		errWrap := fmt.Errorf("ConfigManager.UpdateRuntimeConfig: %w", err)
		EmitBlockingError(mgr.log(), "Mutational failure in runtime configuration", errWrap, GenerateRequestID())
		return RuntimeConfig{}, errWrap
	}
	return snapshot.RuntimeConfig, nil
}

// --- Getters ---

// ConfigPath returns a text description of the active configuration backend.
func (mgr *ConfigManager) ConfigPath() string {
	if mgr == nil {
		return ""
	}
	if strings.TrimSpace(mgr.configFilePath) != "" {
		return mgr.configFilePath
	}
	if mgr.store != nil {
		return mgr.store.Describe()
	}
	return ""
}

// Config returns the current read-only published snapshot of the configuration.
func (mgr *ConfigManager) Config() *BotConfig {
	snap := mgr.currentPublishedSnapshot()
	if snap == nil {
		return nil
	}
	return snap.config
}

// HasAnyGuilds evaluates the existence of configured guilds.
func (mgr *ConfigManager) HasAnyGuilds() bool {
	snap := mgr.currentPublishedSnapshot()
	return snap != nil && snap.config != nil && len(snap.config.Guilds) > 0
}

// --- Guild Config Management ---

// GuildConfig returns the current read-only published snapshot of the configuration for a guild.
func (mgr *ConfigManager) GuildConfig(guildID string) *GuildConfig {
	if mgr == nil || guildID == "" {
		return nil
	}
	snap := mgr.currentPublishedSnapshot()
	if snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
		return nil
	}
	mgr.indexMisses.Add(1)
	return mgr.guildConfigWithPublish(guildID)
}

func (mgr *ConfigManager) guildConfigWithPublish(guildID string) *GuildConfig {
	if mgr == nil {
		return nil
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil || guildID == "" {
		return nil
	}
	if snap := mgr.publishSnapshotLocked(); snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
	}
	if _, err := mgr.rebuildGuildIndexLocked("lookup_miss"); err != nil {
		mgr.log().Warn("Index rebuilding triggered via mitigated cache miss",
			slog.String("guildID", guildID),
			slog.String("error", err.Error()),
		)
	}
	if snap := mgr.publishSnapshotLocked(); snap != nil && snap.config != nil && snap.guildIndex != nil {
		if idx, ok := snap.guildIndex[guildID]; ok {
			if idx >= 0 && idx < len(snap.config.Guilds) && snap.config.Guilds[idx].GuildID == guildID {
				return &snap.config.Guilds[idx]
			}
		}
	}
	mgr.log().Debug("Guild mapping does not exist in consolidated index",
		slog.String("guildID", guildID),
	)
	return nil
}

func (mgr *ConfigManager) rebuildGuildIndexLocked(reason string) (int, error) {
	mgr.indexRebuilds.Add(1)
	if mgr.config == nil {
		mgr.guildIndex = nil
		mgr.log().Info("Guild index cleared due to nil configuration",
			slog.String("reason", reason),
		)
		return 0, nil
	}

	mgr.log().Debug("Iterating guild structures for hash index rebuilding",
		slog.String("reason", reason),
	)

	index := make(map[string]int, len(mgr.config.Guilds))
	deduped := make([]GuildConfig, 0, len(mgr.config.Guilds))
	dupCount := 0

	for _, g := range mgr.config.Guilds {
		gid := g.GuildID
		if gid == "" {
			deduped = append(deduped, g)
			continue
		}
		if _, exists := index[gid]; exists {
			mgr.log().Debug("Key collision avoided during index construction",
				slog.String("guildID", gid),
			)
			dupCount++
			continue
		}
		index[gid] = len(deduped)
		deduped = append(deduped, g)
	}

	if dupCount > 0 {
		mgr.indexDuplicates.Add(uint64(dupCount))
		mgr.log().Warn("Structural integrity corrected locally: duplicate guilds purged from vector",
			slog.String("reason", reason),
			slog.Int("duplicates", dupCount),
			slog.Int("remaining", len(deduped)),
		)
		mgr.config.Guilds = deduped
	}

	mgr.guildIndex = index
	mgr.log().Info("Structural state transition completed: Guild index rebuilt",
		slog.String("reason", reason),
		slog.Int("guilds_count", len(mgr.config.Guilds)),
	)

	if dupCount > 0 {
		return dupCount, fmt.Errorf("removed %d duplicate guild configurations", dupCount)
	}
	return dupCount, nil
}

// GuildIndexStats returns operational counters for index metrics.
func (mgr *ConfigManager) GuildIndexStats() GuildIndexStats {
	if mgr == nil {
		return GuildIndexStats{}
	}
	return GuildIndexStats{
		Rebuilds:   mgr.indexRebuilds.Load(),
		Misses:     mgr.indexMisses.Load(),
		Duplicates: mgr.indexDuplicates.Load(),
	}
}

// AddGuildConfig injects or replaces the mapped configuration of a guild.
func (mgr *ConfigManager) AddGuildConfig(guildCfg GuildConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	next := CloneBotConfigPtr(mgr.config)
	if next == nil {
		next = &BotConfig{Guilds: []GuildConfig{}}
	}

	mgr.log().Debug("Granular guild injection into configuration tree",
		slog.String("guildID", guildCfg.GuildID),
	)

	next.Guilds = append(slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildCfg.GuildID
	}), guildCfg)

	mgr.config = next
	if _, err := mgr.rebuildGuildIndexLocked("add"); err != nil {
		errWrap := fmt.Errorf("add guild configuration: %w", err)
		EmitBlockingError(mgr.log(), "Critical failure attaching configuration to state tree", errWrap, GenerateRequestID())
		return errWrap
	}
	mgr.publishSnapshotLocked()
	return nil
}

// RemoveGuildConfig purges a guild configuration.
func (mgr *ConfigManager) RemoveGuildConfig(guildID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.config == nil {
		return
	}

	mgr.log().Debug("Atomic removal of guild node in configuration",
		slog.String("guildID", guildID),
	)

	next := CloneBotConfigPtr(mgr.config)
	next.Guilds = slices.DeleteFunc(next.Guilds, func(g GuildConfig) bool {
		return g.GuildID == guildID
	})
	mgr.config = next

	if _, err := mgr.rebuildGuildIndexLocked("remove"); err != nil {
		mgr.log().Warn("Collision mitigated during post-removal rebuild",
			slog.String("guildID", guildID),
			slog.String("error", err.Error()),
		)
	}
	mgr.publishSnapshotLocked()
}

// --- Guild Detection & Addition ---

// DetectGuilds automatically detects guilds where the bot is active.
func (mgr *ConfigManager) DetectGuilds(session *discordgo.Session) error {
	return mgr.DetectGuildsForBot(session, "")
}

// DetectGuildsForBot automates guild discovery and binds it to the
// corresponding bot identifier.
func (mgr *ConfigManager) DetectGuildsForBot(session *discordgo.Session, botInstanceID string) error {
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	detected := make([]GuildConfig, 0, len(session.State.Guilds))

	for _, g := range session.State.Guilds {
		fullGuild, err := session.Guild(g.ID)
		if err != nil {
			mgr.log().Warn("Degradation in fetching guild architectural data; main operation will continue idly",
				slog.String("guildID", g.ID),
				slog.String("error", err.Error()),
			)
			continue
		}

		channelID := FindSuitableChannel(session, g.ID)
		if channelID == "" {
			mgr.log().Warn("Mitigated failure: primary operational channel missing in target guild",
				slog.String("guildName", fullGuild.Name),
				slog.String("guildID", g.ID),
			)
			continue
		}

		roles := FindAdminRoles(session, g.ID)

		entryLeaveID := FindEntryLeaveChannel(session, g.ID)
		if entryLeaveID == "" {
			mgr.log().Debug("Dynamic routing: using main channel as fallback for entry_leave",
				slog.String("guildID", g.ID),
			)
			entryLeaveID = channelID
		}

		guildCfg := GuildConfig{
			GuildID: g.ID,
			Channels: ChannelsConfig{
				Commands:      channelID,
				AvatarLogging: channelID,
				RoleUpdate:    channelID,
				MemberJoin:    entryLeaveID,
				MemberLeave:   entryLeaveID,
				MessageEdit:   channelID,
				MessageDelete: channelID,
			},
			Roles: RolesConfig{
				Allowed: roles,
			},
		}
		detected = append(detected, guildCfg)
		mgr.log().Info("Network transition: Guild linked to discovery matrix",
			slog.String("guildName", fullGuild.Name),
			slog.String("guildID", g.ID),
			slog.String("channelID", channelID),
		)
	}

	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		cfg.Guilds = detected
		return nil
	})

	if err != nil {
		EmitBlockingError(mgr.log(), "Block update failure during heuristic detection phase", err, GenerateRequestID())
	}
	return err
}

// RegisterGuild explicitly injects a new guild.
func (mgr *ConfigManager) RegisterGuild(session *discordgo.Session, guildID string) error {
	return mgr.RegisterGuildForBot(session, guildID, "")
}

// RegisterGuildForBot injects and binds the guild to the parameterized bot instance.
func (mgr *ConfigManager) RegisterGuildForBot(session *discordgo.Session, guildID, botInstanceID string) error {
	if session == nil {
		err := fmt.Errorf("%w: discord session is not available", ErrGuildBootstrapDiscordFetch)
		EmitBlockingError(mgr.log(), "Corrupted state in register routine: Null session", err, GenerateRequestID())
		return err
	}
	botInstanceID = NormalizeBotInstanceID(botInstanceID)
	if mgr.GuildConfig(guildID) != nil {
		mgr.log().Info("Pre-existing condition silently resolved: guild already injected",
			slog.String("guildID", guildID),
		)
		return nil
	}
	guild, err := session.Guild(guildID)
	if err != nil {
		return fmt.Errorf("%w: "+ErrGuildInfoFetchMsg, ErrGuildBootstrapDiscordFetch, guildID, err)
	}
	channelID := FindSuitableChannel(session, guildID)
	if channelID == "" {
		return fmt.Errorf("%w: "+ErrNoSuitableChannelMsg, ErrGuildBootstrapPrerequisite, guild.Name)
	}
	roles := FindAdminRoles(session, guildID)
	entryLeaveID := FindEntryLeaveChannel(session, guildID)
	if entryLeaveID == "" {
		entryLeaveID = channelID
	}

	guildCfg := GuildConfig{
		GuildID: guildID,
		Channels: ChannelsConfig{
			Commands:      channelID,
			AvatarLogging: channelID,
			RoleUpdate:    channelID,
			MemberJoin:    entryLeaveID,
			MemberLeave:   entryLeaveID,
			MessageEdit:   channelID,
			MessageDelete: channelID,
		},
		Roles: RolesConfig{
			Allowed: roles,
		},
	}

	if _, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		cfg.Guilds = append(cfg.Guilds, guildCfg)
		return nil
	}); err != nil {
		errWrap := fmt.Errorf("register guild: save configuration: %w", err)
		EmitBlockingError(mgr.log(), "Blocking failure in primary injection routine", errWrap, GenerateRequestID())
		return errWrap
	}

	channelName := channelID
	if ch, err := session.Channel(channelID); err == nil {
		channelName = ch.Name
	}
	mgr.log().Info("Architectural state transition: Guild registration completed and coupled to serial port",
		slog.String("guildName", guild.Name),
		slog.String("guildID", guildID),
		slog.String("channel", channelName),
	)
	return nil
}

// --- Utility & Logging ---

// ShowConfiguredGuilds emits summary logs of the indexed instances.
func ShowConfiguredGuilds(s *discordgo.Session, configManager *ConfigManager) {
	configuration := configManager.Config()
	if configuration == nil || len(configuration.Guilds) == 0 {
		return
	}
	for _, guildConfig := range configuration.Guilds {
		if guild, err := s.Guild(guildConfig.GuildID); err == nil {
			configManager.log().Info("Compliant procedure: Active monitoring on guild telemetry channel",
				slog.String("guildName", guild.Name),
				slog.String("guildID", guild.ID),
			)
		} else {
			configManager.log().Warn("Obstruction in communication network: Registered guild inaccessible to telemetry inspection",
				slog.String("guildID", guildConfig.GuildID),
			)
		}
	}
}

// FindSuitableChannel searches for the suitable primary channel for pipe allocation.
func FindSuitableChannel(session *discordgo.Session, guildID string) string {
	if session == nil || session.State == nil || session.State.User == nil {
		return ""
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil || channels == nil {
		return ""
	}
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			permissions, err := session.UserChannelPermissions(session.State.User.ID, channel.ID)
			if err == nil && (permissions&discordgo.PermissionSendMessages) != 0 {
				if channel.Name == "general" || channel.Name == "geral" || channel.Name == "bot-logs" || channel.Name == "logs" {
					return channel.ID
				}
				if channel.ID != "" {
					return channel.ID
				}
			}
		}
	}
	return ""
}

// FindEntryLeaveChannel searches for the primary channel for logging user I/O events.
func FindEntryLeaveChannel(session *discordgo.Session, guildID string) string {
	if session == nil || session.State == nil || session.State.User == nil {
		return ""
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return ""
	}
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			name := strings.ToLower(channel.Name)
			if name == "user-entry-leave" {
				if HasSendPermission(session, channel.ID) {
					return channel.ID
				}
			}
		}
	}
	return ""
}

// HasSendPermission validates authorization vectors against the target bitmask.
func HasSendPermission(session *discordgo.Session, channelID string) bool {
	if session == nil || session.State == nil || session.State.User == nil || channelID == "" {
		return false
	}
	if perms, err := session.UserChannelPermissions(session.State.User.ID, channelID); err == nil {
		return (perms & discordgo.PermissionSendMessages) != 0
	}
	return false
}

// FindAdminRoles extracts roles containing the administrator bitmask from the vector.
func FindAdminRoles(session *discordgo.Session, guildID string) []string {
	var allowedRoles []string
	roles, err := session.GuildRoles(guildID)
	if err == nil {
		for _, role := range roles {
			if role.Name != "@everyone" && (role.Permissions&discordgo.PermissionAdministrator) != 0 {
				allowedRoles = append(allowedRoles, role.ID)
			}
		}
	}
	return allowedRoles
}

// TextChannels converts and extracts channels suitable for text transmission from the multiplexer.
func TextChannels(session *discordgo.Session, guildID string) ([]*discordgo.Channel, error) {
	if session == nil || session.State == nil || session.State.User == nil {
		return nil, fmt.Errorf("session not initialized")
	}
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return nil, fmt.Errorf("TextChannels: %w", err)
	}
	var textChannels []*discordgo.Channel
	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			permissions, err := session.UserChannelPermissions(session.State.User.ID, channel.ID)
			if err == nil && (permissions&discordgo.PermissionSendMessages) != 0 {
				textChannels = append(textChannels, channel)
			}
		}
	}
	return textChannels, nil
}

// ValidateChannel validates node properties, hierarchical structure, and constraint integrity.
func ValidateChannel(session *discordgo.Session, guildID, channelID string) error {
	if session == nil || session.State == nil || session.State.User == nil {
		return errors.New("session not initialized")
	}
	channel, err := session.Channel(channelID)
	if err != nil {
		return fmt.Errorf("%s: %w", ErrChannelNotFound, err)
	}
	if channel.GuildID != guildID {
		return errors.New(ErrChannelWrongGuild)
	}
	if channel.Type != discordgo.ChannelTypeGuildText {
		return errors.New(ErrChannelWrongType)
	}
	permissions, err := session.UserChannelPermissions(session.State.User.ID, channelID)
	if err != nil {
		return fmt.Errorf(ErrFailedCheckPerms, err)
	}
	if (permissions & discordgo.PermissionSendMessages) == 0 {
		return errors.New(ErrChannelNoPermissions)
	}
	return nil
}

// LogConfiguredGuilds logs a summary of the mapped node tree.
func LogConfiguredGuilds(configManager *ConfigManager, session *discordgo.Session) error {
	return LogConfiguredGuildsForBot(configManager, session, "")
}

// LogConfiguredGuildsForBot summarizes the mapped subset designated for routing of explicit bot instance.
func LogConfiguredGuildsForBot(configManager *ConfigManager, session *discordgo.Session, botInstanceID string) error {
	return logConfiguredGuildSubset(configManager, session, func(cfg *BotConfig) []GuildConfig {
		guilds := cfg.Guilds
		if normalizedBotInstanceID := NormalizeBotInstanceID(botInstanceID); normalizedBotInstanceID != "" {
			guilds = GuildsForBotInstance(cfg, normalizedBotInstanceID)
		}
		return guilds
	})
}

func logConfiguredGuildSubset(configManager *ConfigManager, session *discordgo.Session, resolve func(*BotConfig) []GuildConfig) error {
	cfg := configManager.Config()
	if cfg == nil || len(cfg.Guilds) == 0 {
		configManager.log().Warn("Basal threshold reached: Empty guild allocation vector in boot routine")
		return nil
	}

	guilds := cfg.Guilds
	if resolve != nil {
		guilds = resolve(cfg)
	}
	if len(guilds) == 0 {
		configManager.log().Warn("Basal threshold reached: Empty guild allocation vector in boot routine")
		return nil
	}

	configManager.log().Info("Load summary initialized",
		slog.Int("guilds_count", len(guilds)),
	)

	var errCount int
	for _, g := range guilds {
		guild, err := session.Guild(g.GuildID)
		if err == nil {
			configManager.logger.Info("Active interface confirmed",
				slog.String("guildName", guild.Name),
				slog.String("guildID", guild.ID),
			)
		} else {
			configManager.logger.Warn("Handshake failure with guild interface reported by central hub",
				slog.String("guildID", g.GuildID),
			)
			errCount++
		}
	}
	return nil
}

```

// === FILE: pkg/files/qotd.go ===
```go
package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	// ErrInvalidQOTDInput indicates invalid QOTD configuration payloads.
	ErrInvalidQOTDInput = errors.New("invalid qotd input")
)

// LegacyQOTDDefaultDeckName defines legacy qotddefault deck name.
// LegacyQOTDDefaultDeckID defines legacy qotddefault deck id.
const (
	LegacyQOTDDefaultDeckID   = "default"
	LegacyQOTDDefaultDeckName = "Default"
	qotdPublishDateLayout     = "2006-01-02"
)

// QOTDSelectionStrategy enumerates the supported question-selection strategies
// for automatic publish. The string values are persisted in the deck config
// and mirror the QOTDQuestionSelector vocabulary used by the storage layer.
type QOTDSelectionStrategy string

// QOTDSelectionStrategyRandom defines qotdselection strategy random.
// QOTDSelectionStrategyQueue defines qotdselection strategy queue.
const (
	QOTDSelectionStrategyQueue  QOTDSelectionStrategy = "queue"
	QOTDSelectionStrategyRandom QOTDSelectionStrategy = "random"
)

// IsZero reports whether all QOTD deck fields are unset.
func (cfg QOTDDeckConfig) IsZero() bool {
	return strings.TrimSpace(cfg.ID) == "" &&
		strings.TrimSpace(cfg.Name) == "" &&
		!cfg.Enabled &&
		strings.TrimSpace(cfg.ChannelID) == "" &&
		strings.TrimSpace(cfg.SelectionStrategy) == ""
}

// EffectiveSelectionStrategy returns the deck's configured strategy, falling
// back to "queue" when unset or unrecognized.
func (cfg QOTDDeckConfig) EffectiveSelectionStrategy() QOTDSelectionStrategy {
	switch strings.ToLower(strings.TrimSpace(cfg.SelectionStrategy)) {
	case string(QOTDSelectionStrategyRandom):
		return QOTDSelectionStrategyRandom
	default:
		return QOTDSelectionStrategyQueue
	}
}

// IsZero reports whether both schedule components are unset.
func (cfg QOTDPublishScheduleConfig) IsZero() bool {
	return cfg.HourUTC == nil && cfg.MinuteUTC == nil
}

// IsComplete reports whether both schedule components are present.
func (cfg QOTDPublishScheduleConfig) IsComplete() bool {
	return cfg.HourUTC != nil && cfg.MinuteUTC != nil
}

// Values returns the configured UTC schedule when both components are present.
func (cfg QOTDPublishScheduleConfig) Values() (hour int, minute int, ok bool) {
	if !cfg.IsComplete() {
		return 0, 0, false
	}
	return *cfg.HourUTC, *cfg.MinuteUTC, true
}

// IsZero reports whether all QOTD fields are unset.
func (cfg QOTDConfig) IsZero() bool {
	if len(cfg.deckConfigs()) > 0 {
		if len(cfg.deckConfigs()) == 1 &&
			isImplicitDefaultQOTDDeck(cfg.deckConfigs()[0], strings.TrimSpace(cfg.ActiveDeckID)) &&
			cfg.Schedule.IsZero() &&
			len(cfg.SuppressScheduledPublishDatesUTC) == 0 {
			return true
		}
		return false
	}
	if !cfg.Schedule.IsZero() || len(cfg.SuppressScheduledPublishDatesUTC) != 0 {
		return false
	}
	return true
}

// SuppressesScheduledPublishDate reports whether the given UTC publish date
// is in the suppression set. The set membership semantic replaces the old
// single-string field, so callers can suppress today AND tomorrow at the
// same time (e.g. a manual publish that occupies tomorrow's slot while a
// maintenance flow pauses today's automatic publish).
func (cfg QOTDConfig) SuppressesScheduledPublishDate(publishDate time.Time) bool {
	publishDate = publishDate.UTC()
	if publishDate.IsZero() {
		return false
	}
	target := publishDate.Format(qotdPublishDateLayout)
	for _, raw := range cfg.SuppressScheduledPublishDatesUTC {
		if strings.TrimSpace(raw) == target {
			return true
		}
	}
	return false
}

// WithSuppressedScheduledPublishDate returns a config with the publish date
// added to the suppression set. Idempotent: passing a date that is already
// suppressed returns the config unchanged.
func (cfg QOTDConfig) WithSuppressedScheduledPublishDate(publishDate time.Time) QOTDConfig {
	publishDate = publishDate.UTC()
	if publishDate.IsZero() {
		return cfg
	}
	if cfg.SuppressesScheduledPublishDate(publishDate) {
		return cfg
	}
	formatted := publishDate.Format(qotdPublishDateLayout)
	cfg.SuppressScheduledPublishDatesUTC = append(append([]string(nil), cfg.SuppressScheduledPublishDatesUTC...), formatted)
	sortSuppressedPublishDates(cfg.SuppressScheduledPublishDatesUTC)
	return cfg
}

// ClearSuppressedScheduledPublishDate returns a config with the publish
// date removed from the suppression set. Idempotent: passing a date that
// is not in the set returns the config unchanged.
func (cfg QOTDConfig) ClearSuppressedScheduledPublishDate(publishDate time.Time) QOTDConfig {
	publishDate = publishDate.UTC()
	if publishDate.IsZero() || !cfg.SuppressesScheduledPublishDate(publishDate) {
		return cfg
	}
	target := publishDate.Format(qotdPublishDateLayout)
	next := make([]string, 0, len(cfg.SuppressScheduledPublishDatesUTC))
	for _, raw := range cfg.SuppressScheduledPublishDatesUTC {
		if strings.TrimSpace(raw) == target {
			continue
		}
		next = append(next, raw)
	}
	if len(next) == 0 {
		cfg.SuppressScheduledPublishDatesUTC = nil
	} else {
		cfg.SuppressScheduledPublishDatesUTC = next
	}
	return cfg
}

// SuppressedScheduledPublishDates returns the canonical sorted set of
// suppressed UTC publish dates as time.Time values. Convenience for
// callers that want to iterate the set without re-parsing the strings.
func (cfg QOTDConfig) SuppressedScheduledPublishDates() []time.Time {
	if len(cfg.SuppressScheduledPublishDatesUTC) == 0 {
		return nil
	}
	out := make([]time.Time, 0, len(cfg.SuppressScheduledPublishDatesUTC))
	for _, raw := range cfg.SuppressScheduledPublishDatesUTC {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(qotdPublishDateLayout, raw)
		if err != nil {
			continue
		}
		out = append(out, parsed.UTC())
	}
	return out
}

func sortSuppressedPublishDates(dates []string) {
	sort.SliceStable(dates, func(i, j int) bool {
		return strings.TrimSpace(dates[i]) < strings.TrimSpace(dates[j])
	})
}

// DashboardQOTDConfig returns a stable deck-aware settings payload for the
// dashboard, even when the persisted config is still empty.
func DashboardQOTDConfig(cfg QOTDConfig) QOTDConfig {
	out := cloneQOTDConfig(cfg)
	decks := out.deckConfigs()
	if len(decks) == 0 {
		out.ActiveDeckID = LegacyQOTDDefaultDeckID
		out.Decks = []QOTDDeckConfig{{
			ID:   LegacyQOTDDefaultDeckID,
			Name: LegacyQOTDDefaultDeckName,
		}}
		return out
	}

	out.Decks = decks
	if strings.TrimSpace(out.ActiveDeckID) == "" {
		if activeDeck, ok := (QOTDConfig{
			ActiveDeckID: out.ActiveDeckID,
			Decks:        decks,
		}).ActiveDeck(); ok {
			out.ActiveDeckID = activeDeck.ID
		}
	}
	return out
}

// DeckByID resolves one configured deck by ID.
func (cfg QOTDConfig) DeckByID(deckID string) (QOTDDeckConfig, bool) {
	deckID = strings.TrimSpace(deckID)
	for _, deck := range cfg.deckConfigs() {
		if strings.TrimSpace(deck.ID) == deckID {
			return deck, true
		}
	}
	return QOTDDeckConfig{}, false
}

// ActiveDeck resolves the active configured deck, if any.
func (cfg QOTDConfig) ActiveDeck() (QOTDDeckConfig, bool) {
	decks := cfg.deckConfigs()
	if len(decks) == 0 {
		return QOTDDeckConfig{}, false
	}
	activeDeckID := strings.TrimSpace(cfg.ActiveDeckID)
	if activeDeckID != "" {
		for _, deck := range decks {
			if strings.TrimSpace(deck.ID) == activeDeckID {
				return deck, true
			}
		}
	}
	for _, deck := range decks {
		if deck.Enabled {
			return deck, true
		}
	}
	return decks[0], true
}

func (cfg QOTDConfig) deckConfigs() []QOTDDeckConfig {
	if len(cfg.Decks) > 0 {
		return cloneQOTDDeckConfigs(cfg.Decks)
	}
	return nil
}

func isImplicitDefaultQOTDDeck(deck QOTDDeckConfig, activeDeckID string) bool {
	return strings.TrimSpace(deck.ID) == LegacyQOTDDefaultDeckID &&
		strings.TrimSpace(deck.Name) == LegacyQOTDDefaultDeckName &&
		!deck.Enabled &&
		strings.TrimSpace(deck.ChannelID) == "" &&
		(activeDeckID == "" || activeDeckID == LegacyQOTDDefaultDeckID)
}

// UnmarshalJSON unmarshals json.
func (cfg *QOTDDeckConfig) UnmarshalJSON(data []byte) error {
	type rawQOTDDeckConfig struct {
		ID        string `json:"id,omitempty"`
		Name      string `json:"name,omitempty"`
		Enabled   bool   `json:"enabled,omitempty"`
		ChannelID string `json:"channel_id,omitempty"`
		// Deprecated: migrated to ChannelID
		ForumChannelID string `json:"forum_channel_id,omitempty"`
		// Deprecated: migrated to ChannelID
		QuestionChannelID string `json:"question_channel_id,omitempty"`
		// Deprecated: migrated to ChannelID
		ResponseChannelID string `json:"response_channel_id,omitempty"`
		SelectionStrategy string `json:"selection_strategy,omitempty"`
	}

	var raw rawQOTDDeckConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("QOTDDeckConfig.UnmarshalJSON: %w", err)
	}

	channelID := strings.TrimSpace(raw.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ForumChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.QuestionChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ResponseChannelID)
	}

	*cfg = QOTDDeckConfig{
		ID:                raw.ID,
		Name:              raw.Name,
		Enabled:           raw.Enabled,
		ChannelID:         channelID,
		SelectionStrategy: strings.TrimSpace(raw.SelectionStrategy),
	}
	return nil
}

// UnmarshalJSON unmarshals json.
func (cfg *QOTDConfig) UnmarshalJSON(data []byte) error {
	type rawQOTDPublishScheduleConfig struct {
		HourUTC   *int `json:"hour_utc,omitempty"`
		MinuteUTC *int `json:"minute_utc,omitempty"`
		// Deprecated: migrated to Schedule
		PublishHourUTC *int `json:"publish_hour_utc,omitempty"`
		// Deprecated: migrated to Schedule
		PublishMinuteUTC *int `json:"publish_minute_utc,omitempty"`
		// Deprecated: migrated to HourUTC
		QOTDTimeHourUTC *int `json:"qotd_time_hour_utc,omitempty"`
		// Deprecated: migrated to MinuteUTC
		QOTDTimeMinuteUTC *int `json:"qotd_time_minute_utc,omitempty"`
	}

	type rawQOTDConfig struct {
		VerifiedRoleID string                       `json:"verified_role_id,omitempty"`
		ActiveDeckID   string                       `json:"active_deck_id,omitempty"`
		Decks          []QOTDDeckConfig             `json:"decks,omitempty"`
		Schedule       rawQOTDPublishScheduleConfig `json:"schedule,omitempty"`
		// SuppressScheduledPublishDatesUTC is the new list form. Older configs
		// persisted only LegacySuppressDateUTC; the unmarshal migrates the
		// legacy value into the list when the new field is absent.
		SuppressScheduledPublishDatesUTC []string `json:"suppress_scheduled_publish_dates_utc,omitempty"`
		// Deprecated: migrated to SuppressScheduledPublishDatesUTC
		SuppressScheduledPublishDateUTC string `json:"suppress_scheduled_publish_date_utc,omitempty"`
		// Deprecated: migrated to Decks
		Enabled bool `json:"enabled,omitempty"`
		// Deprecated: migrated to Decks
		ChannelID string `json:"channel_id,omitempty"`
		// Deprecated: migrated to Decks
		ForumChannelID string `json:"forum_channel_id,omitempty"`
		// Deprecated: migrated to Decks
		QuestionChannelID string `json:"question_channel_id,omitempty"`
		// Deprecated: migrated to Decks
		ResponseChannelID string `json:"response_channel_id,omitempty"`
		// Deprecated: migrated to Schedule
		PublishHourUTC *int `json:"publish_hour_utc,omitempty"`
		// Deprecated: migrated to Schedule
		PublishMinuteUTC *int `json:"publish_minute_utc,omitempty"`
		// Deprecated: migrated to PublishHourUTC
		QOTDTimeHourUTC *int `json:"qotd_time_hour_utc,omitempty"`
		// Deprecated: migrated to PublishMinuteUTC
		QOTDTimeMinuteUTC *int `json:"qotd_time_minute_utc,omitempty"`
	}

	var raw rawQOTDConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("QOTDConfig.UnmarshalJSON: %w", err)
	}

	schedule := QOTDPublishScheduleConfig{
		HourUTC:   cloneOptionalInt(raw.Schedule.HourUTC),
		MinuteUTC: cloneOptionalInt(raw.Schedule.MinuteUTC),
	}
	if schedule.HourUTC == nil {
		switch {
		case raw.Schedule.PublishHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.Schedule.PublishHourUTC)
		case raw.PublishHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.PublishHourUTC)
		case raw.Schedule.QOTDTimeHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.Schedule.QOTDTimeHourUTC)
		case raw.QOTDTimeHourUTC != nil:
			schedule.HourUTC = cloneOptionalInt(raw.QOTDTimeHourUTC)
		}
	}
	if schedule.MinuteUTC == nil {
		switch {
		case raw.Schedule.PublishMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.Schedule.PublishMinuteUTC)
		case raw.PublishMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.PublishMinuteUTC)
		case raw.Schedule.QOTDTimeMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.Schedule.QOTDTimeMinuteUTC)
		case raw.QOTDTimeMinuteUTC != nil:
			schedule.MinuteUTC = cloneOptionalInt(raw.QOTDTimeMinuteUTC)
		}
	}

	suppressedDates := raw.SuppressScheduledPublishDatesUTC
	if len(suppressedDates) == 0 && strings.TrimSpace(raw.SuppressScheduledPublishDateUTC) != "" {
		// Legacy single-string field migration: keep old persisted configs
		// loading until the next write replaces the legacy key with the
		// canonical list form.
		suppressedDates = []string{raw.SuppressScheduledPublishDateUTC}
	}

	*cfg = QOTDConfig{
		VerifiedRoleID:                   raw.VerifiedRoleID,
		ActiveDeckID:                     raw.ActiveDeckID,
		Decks:                            raw.Decks,
		Schedule:                         schedule,
		SuppressScheduledPublishDatesUTC: suppressedDates,
	}
	if len(raw.Decks) > 0 {
		return nil
	}

	channelID := strings.TrimSpace(raw.ChannelID)
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ForumChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.QuestionChannelID)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(raw.ResponseChannelID)
	}
	if !raw.Enabled && channelID == "" {
		return nil
	}

	cfg.Decks = []QOTDDeckConfig{{
		ID:        LegacyQOTDDefaultDeckID,
		Name:      LegacyQOTDDefaultDeckName,
		Enabled:   raw.Enabled,
		ChannelID: channelID,
	}}
	return nil
}

// QOTDConfig returns the canonical QOTD config for one guild.
func (mgr *ConfigManager) QOTDConfig(guildID string) (_ QOTDConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get qotd config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return QOTDConfig{}, invalidQOTDInput("guild_id is required")
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return QOTDConfig{}, fmt.Errorf("ConfigManager.QOTDConfig: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}

	normalized, err := NormalizeQOTDConfig(guildConfig.QOTD)
	if err != nil {
		return QOTDConfig{}, fmt.Errorf("ConfigManager.QOTDConfig: %w", err)
	}
	return normalized, nil
}

// GetQOTDConfig returns the canonical QOTD config for one guild.
func (mgr *ConfigManager) GetQOTDConfig(guildID string) (QOTDConfig, error) {
	return mgr.QOTDConfig(guildID)
}

// SetQOTDConfig validates and persists the QOTD config for one guild.
func (mgr *ConfigManager) SetQOTDConfig(guildID string, cfg QOTDConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("set qotd config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidQOTDInput("guild_id is required")
	}

	normalized, err := NormalizeQOTDConfig(cfg)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetQOTDConfig: %w", err)
	}
	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		guildConfig.QOTD = normalized
		return nil
	})
}

func invalidQOTDInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidQOTDInput, fmt.Sprintf(format, args...))
}

```

// === FILE: pkg/files/qotd_test.go ===
```go
package files

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func testQOTDSchedule(hour, minute int) QOTDPublishScheduleConfig {
	return QOTDPublishScheduleConfig{
		HourUTC:   &hour,
		MinuteUTC: &minute,
	}
}

func TestQOTDDeckConfigEffectiveSelectionStrategyDefaultsToQueue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  string
		want QOTDSelectionStrategy
	}{
		{name: "empty falls back to queue", raw: "", want: QOTDSelectionStrategyQueue},
		{name: "explicit queue stays queue", raw: "queue", want: QOTDSelectionStrategyQueue},
		{name: "random is honored", raw: "random", want: QOTDSelectionStrategyRandom},
		{name: "case-insensitive random", raw: "RANDOM", want: QOTDSelectionStrategyRandom},
		{name: "whitespace tolerated", raw: "  random  ", want: QOTDSelectionStrategyRandom},
		{name: "unknown values fall back to queue", raw: "shuffle", want: QOTDSelectionStrategyQueue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := QOTDDeckConfig{SelectionStrategy: tc.raw}.EffectiveSelectionStrategy()
			if got != tc.want {
				t.Fatalf("EffectiveSelectionStrategy(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestNormalizeQOTDConfigPreservesSelectionStrategy(t *testing.T) {
	t.Parallel()

	hourUTC, minuteUTC := 12, 0
	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		Schedule: QOTDPublishScheduleConfig{HourUTC: &hourUTC, MinuteUTC: &minuteUTC},
		Decks: []QOTDDeckConfig{{
			ID:                LegacyQOTDDefaultDeckID,
			Name:              LegacyQOTDDefaultDeckName,
			Enabled:           true,
			ChannelID:         "123456789012345678",
			SelectionStrategy: "random",
		}},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if len(normalized.Decks) != 1 || normalized.Decks[0].SelectionStrategy != string(QOTDSelectionStrategyRandom) {
		t.Fatalf("expected normalized deck to keep selection_strategy=random, got %+v", normalized.Decks)
	}
}

func TestNormalizeQOTDConfigDropsUnknownSelectionStrategy(t *testing.T) {
	t.Parallel()

	hourUTC, minuteUTC := 12, 0
	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		Schedule: QOTDPublishScheduleConfig{HourUTC: &hourUTC, MinuteUTC: &minuteUTC},
		Decks: []QOTDDeckConfig{{
			ID:                LegacyQOTDDefaultDeckID,
			Name:              LegacyQOTDDefaultDeckName,
			Enabled:           true,
			ChannelID:         "123456789012345678",
			SelectionStrategy: "shuffle",
		}},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if len(normalized.Decks) != 1 || normalized.Decks[0].SelectionStrategy != "" {
		t.Fatalf("expected unknown selection_strategy to be dropped to empty, got %+v", normalized.Decks)
	}
}

func TestNormalizeQOTDConfigRequiresDeliveryTargetsWhenEnabled(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeQOTDConfig(QOTDConfig{
		Decks: []QOTDDeckConfig{{
			ID:      LegacyQOTDDefaultDeckID,
			Name:    LegacyQOTDDefaultDeckName,
			Enabled: true,
		}},
	}); err == nil {
		t.Fatal("expected enabled qotd config without delivery targets to fail")
	}
}

func TestNormalizeQOTDConfigRequiresScheduleWhenEnabled(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeQOTDConfig(QOTDConfig{
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	}); err == nil {
		t.Fatal("expected enabled qotd config without schedule to fail")
	}
}

func TestNormalizeQOTDConfigAllowsPartialScheduleWhileDisabled(t *testing.T) {
	t.Parallel()

	hourUTC := 7
	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		Schedule: QOTDPublishScheduleConfig{HourUTC: &hourUTC},
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			ChannelID: "123456789012345678",
		}},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if normalized.Schedule.HourUTC == nil || *normalized.Schedule.HourUTC != 7 {
		t.Fatalf("expected partial schedule to persist, got %+v", normalized.Schedule)
	}
	if normalized.Schedule.MinuteUTC != nil {
		t.Fatalf("expected minute to remain unset, got %+v", normalized.Schedule)
	}
}

func TestNormalizeQOTDConfigNormalizesSuppressedScheduledPublishDate(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		ActiveDeckID: LegacyQOTDDefaultDeckID,
		Schedule:     testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
		SuppressScheduledPublishDatesUTC: []string{" 2026-04-03 "},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	if got := normalized.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-03" {
		t.Fatalf("expected canonical suppressed publish date, got %+v", normalized)
	}
	if !normalized.SuppressesScheduledPublishDate(time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)) {
		t.Fatalf("expected normalized config to suppress the matching slot date, got %+v", normalized)
	}
	cleared := normalized.ClearSuppressedScheduledPublishDate(time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if len(cleared.SuppressScheduledPublishDatesUTC) != 0 {
		t.Fatalf("expected matching clear to remove suppressed slot date, got %+v", cleared)
	}
	unchanged := normalized.ClearSuppressedScheduledPublishDate(time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC))
	if got := unchanged.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-03" {
		t.Fatalf("expected non-matching clear to preserve suppressed slot date, got %+v", unchanged)
	}
	shifted := normalized.WithSuppressedScheduledPublishDate(time.Date(2026, 4, 5, 13, 15, 0, 0, time.UTC))
	if got := shifted.SuppressScheduledPublishDatesUTC; len(got) != 2 || got[0] != "2026-04-03" || got[1] != "2026-04-05" {
		t.Fatalf("expected suppression helper to add a second canonical date, got %+v", shifted)
	}
}

func TestNormalizeQOTDConfigDedupesAndSortsSuppressionList(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeQOTDConfig(QOTDConfig{
		ActiveDeckID: LegacyQOTDDefaultDeckID,
		Schedule:     testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
		SuppressScheduledPublishDatesUTC: []string{
			"2026-04-05",
			" 2026-04-03 ",
			"2026-04-03",
			"",
			"2026-04-05",
		},
	})
	if err != nil {
		t.Fatalf("NormalizeQOTDConfig() failed: %v", err)
	}
	got := normalized.SuppressScheduledPublishDatesUTC
	want := []string{"2026-04-03", "2026-04-05"}
	if len(got) != len(want) {
		t.Fatalf("expected canonical sorted dedupe, got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected dates[%d]=%q, got %q (full=%+v)", i, want[i], got[i], got)
		}
	}
}

func TestQOTDConfigUnmarshalMigratesLegacySingleSuppressionField(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"suppress_scheduled_publish_date_utc":"2026-04-04"}`)
	var cfg QOTDConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("Unmarshal(legacy field) failed: %v", err)
	}
	if got := cfg.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-04" {
		t.Fatalf("expected legacy single-string suppression to migrate into the list form, got %+v", got)
	}
}

func TestQOTDConfigUnmarshalPrefersNewListWhenBothFieldsPresent(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"suppress_scheduled_publish_date_utc":"2026-04-04","suppress_scheduled_publish_dates_utc":["2026-05-01","2026-05-02"]}`)
	var cfg QOTDConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("Unmarshal(both fields) failed: %v", err)
	}
	got := cfg.SuppressScheduledPublishDatesUTC
	want := []string{"2026-05-01", "2026-05-02"}
	if len(got) != len(want) {
		t.Fatalf("expected the new list to take precedence over the legacy single value, got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected dates[%d]=%q, got %q", i, want[i], got[i])
		}
	}
}

func TestNormalizeQOTDConfigRejectsInvalidSuppressedScheduledPublishDate(t *testing.T) {
	t.Parallel()

	_, err := NormalizeQOTDConfig(QOTDConfig{
		ActiveDeckID: LegacyQOTDDefaultDeckID,
		Schedule:     testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
		SuppressScheduledPublishDatesUTC: []string{"04/03/2026"},
	})
	if err == nil {
		t.Fatal("expected invalid suppressed scheduled publish date to fail normalization")
	}
}

func TestSetQOTDConfigCanonicalizesMessageChannelFields(t *testing.T) {
	t.Parallel()

	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, nil)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		VerifiedRoleID: " 987654321098765432 ",
		ActiveDeckID:   LegacyQOTDDefaultDeckID,
		Schedule:       testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: " 123456789012345678 ",
		}},
	})
	if err != nil {
		t.Fatalf("SetQOTDConfig() failed: %v", err)
	}

	cfg, err := mgr.QOTDConfig("g1")
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	deck, ok := cfg.ActiveDeck()
	if !ok {
		t.Fatal("expected qotd config to expose an active deck")
	}
	if !deck.Enabled {
		t.Fatal("expected qotd deck to remain enabled")
	}
	if deck.ChannelID != "123456789012345678" {
		t.Fatalf("expected canonical channel id, got %q", deck.ChannelID)
	}
	if cfg.VerifiedRoleID != "987654321098765432" {
		t.Fatalf("expected canonical verified role id, got %q", cfg.VerifiedRoleID)
	}
	if cfg.ActiveDeckID != LegacyQOTDDefaultDeckID {
		t.Fatalf("expected default deck to become active, got %q", cfg.ActiveDeckID)
	}
	if cfg.Schedule.HourUTC == nil || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.HourUTC != 12 || *cfg.Schedule.MinuteUTC != 43 {
		t.Fatalf("expected canonical schedule, got %+v", cfg.Schedule)
	}
}

func TestSetQOTDConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, saveErr)

	err := mgr.SetQOTDConfig("g1", QOTDConfig{
		ActiveDeckID: LegacyQOTDDefaultDeckID,
		Schedule:     testQOTDSchedule(12, 43),
		Decks: []QOTDDeckConfig{{
			ID:        LegacyQOTDDefaultDeckID,
			Name:      LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected guild config to remain intact, got %+v", cfg.Guilds)
	}
	if !cfg.Guilds[0].QOTD.IsZero() {
		t.Fatalf("expected qotd config rollback, got %+v", cfg.Guilds[0].QOTD)
	}
}

func TestQOTDConfigUnmarshalMigratesLegacyChannelFields(t *testing.T) {
	t.Parallel()

	var cfg QOTDConfig
	if err := json.Unmarshal([]byte(`{
		"enabled": true,
		"question_channel_id": "123456789012345678",
		"qotd_time_hour_utc": 7,
		"qotd_time_minute_utc": 5
	}`), &cfg); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	deck, ok := cfg.ActiveDeck()
	if !ok {
		t.Fatal("expected legacy payload to produce a default deck")
	}
	if !deck.Enabled {
		t.Fatalf("expected legacy enabled flag to carry over, got %+v", deck)
	}
	if deck.ChannelID != "123456789012345678" {
		t.Fatalf("expected legacy question channel to map to channel_id, got %+v", deck)
	}
	if cfg.Schedule.HourUTC == nil || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.HourUTC != 7 || *cfg.Schedule.MinuteUTC != 5 {
		t.Fatalf("expected legacy schedule fields to map to canonical schedule, got %+v", cfg.Schedule)
	}
}

// TestQOTDConfigLegacyJSONTableMappings ensures that legacy JSON keys (such as
// forum_channel_id and old QOTD schedules) correctly map to canonical config fields.
func TestQOTDConfigLegacyJSONTableMappings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload string
		check   func(t *testing.T, cfg QOTDConfig)
	}{
		{
			name:    "legacy question_channel_id maps to default deck channel_id",
			payload: `{"enabled": true, "question_channel_id": "111"}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if len(cfg.Decks) != 1 || cfg.Decks[0].ChannelID != "111" {
					t.Fatalf("expected channel_id mapped to 111, got %+v", cfg.Decks)
				}
			},
		},
		{
			name:    "legacy forum_channel_id maps to default deck forum_channel_id",
			payload: `{"enabled": true, "forum_channel_id": "222"}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if len(cfg.Decks) != 1 || cfg.Decks[0].ChannelID != "222" {
					t.Fatalf("expected forum_channel_id mapped to ChannelID 222, got %+v", cfg.Decks)
				}
			},
		},
		{
			name:    "legacy qotd_time_hour_utc and minute maps to Schedule",
			payload: `{"qotd_time_hour_utc": 15, "qotd_time_minute_utc": 30}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if cfg.Schedule.HourUTC == nil || *cfg.Schedule.HourUTC != 15 || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.MinuteUTC != 30 {
					t.Fatalf("expected schedule 15:30, got %+v", cfg.Schedule)
				}
			},
		},
		{
			name:    "legacy publish_hour_utc and minute maps to Schedule",
			payload: `{"publish_hour_utc": 10, "publish_minute_utc": 45}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if cfg.Schedule.HourUTC == nil || *cfg.Schedule.HourUTC != 10 || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.MinuteUTC != 45 {
					t.Fatalf("expected schedule 10:45, got %+v", cfg.Schedule)
				}
			},
		},
		{
			name:    "legacy suppress_scheduled_publish_date_utc maps to list",
			payload: `{"suppress_scheduled_publish_date_utc": "2026-06-04"}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if len(cfg.SuppressScheduledPublishDatesUTC) != 1 || cfg.SuppressScheduledPublishDatesUTC[0] != "2026-06-04" {
					t.Fatalf("expected suppressed dates list length 1, got %+v", cfg.SuppressScheduledPublishDatesUTC)
				}
			},
		},
		{
			name:    "canonical schedule shadows legacy",
			payload: `{"publish_hour_utc": 10, "publish_minute_utc": 45, "schedule": {"hour_utc": 11, "minute_utc": 50}}`,
			check: func(t *testing.T, cfg QOTDConfig) {
				if cfg.Schedule.HourUTC == nil || *cfg.Schedule.HourUTC != 11 || cfg.Schedule.MinuteUTC == nil || *cfg.Schedule.MinuteUTC != 50 {
					t.Fatalf("expected canonical schedule 11:50 to win, got %+v", cfg.Schedule)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var cfg QOTDConfig
			if err := json.Unmarshal([]byte(tc.payload), &cfg); err != nil {
				t.Fatalf("unmarshal failed: %v", err)
			}
			tc.check(t, cfg)
		})
	}
}

```

// === FILE: pkg/files/reaction_blocks.go ===
```go
package files

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ReactionBlockEmojiKindUnicode defines reaction block emoji kind unicode.
// ReactionBlockEmojiKindCustom defines reaction block emoji kind custom.
const (
	ReactionBlockEmojiKindCustom  = "custom"
	ReactionBlockEmojiKindUnicode = "unicode"
)

// ErrInvalidReactionBlockInput defines err invalid reaction block input.
var ErrInvalidReactionBlockInput = errors.New("invalid reaction block input")

// CloneReactionBlockConfig clones reaction block config.
func CloneReactionBlockConfig(in ReactionBlockConfig) ReactionBlockConfig {
	return cloneReactionBlockConfig(in)
}

// IsZero is zero.
func (cfg ReactionBlockConfig) IsZero() bool {
	return len(cfg.Rules) == 0
}

// IsZero is zero.
func (rule ReactionBlockRuleConfig) IsZero() bool {
	return strings.TrimSpace(rule.ReactorUserID) == "" && strings.TrimSpace(rule.TargetUserID) == "" && len(rule.Emojis) == 0
}

// IsZero is zero.
func (emoji ReactionBlockEmojiConfig) IsZero() bool {
	return reactionBlockEmojiKey(emoji) == ""
}

// Display displays.
func (emoji ReactionBlockEmojiConfig) Display() string {
	switch reactionBlockEmojiKind(emoji.Kind) {
	case ReactionBlockEmojiKindCustom:
		name := strings.TrimSpace(emoji.Name)
		if name == "" {
			name = "emoji"
		}
		prefix := ":"
		if emoji.Animated {
			prefix = "a:"
		}
		if value := strings.TrimSpace(emoji.Value); value != "" {
			return "<" + prefix + name + ":" + value + ">"
		}
	case ReactionBlockEmojiKindUnicode:
		if alias := normalizeReactionBlockAlias(emoji.Alias); alias != "" {
			return alias
		}
		if value := strings.TrimSpace(emoji.Value); value != "" {
			return value
		}
	}
	return ""
}

// EmojisForPair emojis for pair.
func (cfg ReactionBlockConfig) EmojisForPair(reactorUserID, targetUserID string) []ReactionBlockEmojiConfig {
	reactorUserID = strings.TrimSpace(reactorUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	if reactorUserID == "" || targetUserID == "" {
		return nil
	}
	for _, rule := range cfg.Rules {
		if strings.TrimSpace(rule.ReactorUserID) != reactorUserID || strings.TrimSpace(rule.TargetUserID) != targetUserID {
			continue
		}
		if len(rule.Emojis) == 0 {
			return nil
		}
		out := make([]ReactionBlockEmojiConfig, 0, len(rule.Emojis))
		for _, emoji := range rule.Emojis {
			if emoji.IsZero() {
				continue
			}
			out = append(out, emoji)
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

// BlocksEmojiForPair blocks emoji for pair.
func (cfg ReactionBlockConfig) BlocksEmojiForPair(reactorUserID, targetUserID string, emoji ReactionBlockEmojiConfig) bool {
	key := reactionBlockEmojiKey(emoji)
	if key == "" {
		return false
	}
	for _, candidate := range cfg.EmojisForPair(reactorUserID, targetUserID) {
		if reactionBlockEmojiKey(candidate) == key {
			return true
		}
	}
	return false
}

// NormalizeReactionBlockConfig normalizes reaction block config.
func NormalizeReactionBlockConfig(in ReactionBlockConfig) (ReactionBlockConfig, error) {
	if len(in.Rules) == 0 {
		return ReactionBlockConfig{}, nil
	}

	out := make([]ReactionBlockRuleConfig, 0, len(in.Rules))
	indexByPair := make(map[string]int, len(in.Rules))
	for idx, rule := range in.Rules {
		normalized, err := normalizeReactionBlockRuleConfig(rule)
		if err != nil {
			return ReactionBlockConfig{}, invalidReactionBlockInput("rules[%d]: %v", idx, err)
		}
		pairKey := reactionBlockPairKey(normalized.ReactorUserID, normalized.TargetUserID)
		if existingIdx, ok := indexByPair[pairKey]; ok {
			merged := append(cloneReactionBlockRuleConfig(out[existingIdx]).Emojis, normalized.Emojis...)
			normalizedEmojis, err := normalizeReactionBlockEmojiConfigs(merged)
			if err != nil {
				return ReactionBlockConfig{}, invalidReactionBlockInput("rules[%d]: %v", idx, err)
			}
			out[existingIdx].Emojis = normalizedEmojis
			continue
		}
		indexByPair[pairKey] = len(out)
		out = append(out, normalized)
	}

	if len(out) == 0 {
		return ReactionBlockConfig{}, nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReactorUserID != out[j].ReactorUserID {
			return out[i].ReactorUserID < out[j].ReactorUserID
		}
		return out[i].TargetUserID < out[j].TargetUserID
	})
	return ReactionBlockConfig{Rules: out}, nil
}

// ReactionBlockConfig reactions block config.
func (mgr *ConfigManager) ReactionBlockConfig(guildID string) (_ ReactionBlockConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get reaction block config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return ReactionBlockConfig{}, invalidReactionBlockInput("guild_id is required")
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return ReactionBlockConfig{}, fmt.Errorf("ConfigManager.ReactionBlockConfig: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}

	normalized, err := NormalizeReactionBlockConfig(guildConfig.ReactionBlocks)
	if err != nil {
		return ReactionBlockConfig{}, fmt.Errorf("ConfigManager.ReactionBlockConfig: %w", err)
	}
	return normalized, nil
}

// GetReactionBlockConfig gets reaction block config.
func (mgr *ConfigManager) GetReactionBlockConfig(guildID string) (ReactionBlockConfig, error) {
	return mgr.ReactionBlockConfig(guildID)
}

// SetReactionBlockConfig sets reaction block config.
func (mgr *ConfigManager) SetReactionBlockConfig(guildID string, cfg ReactionBlockConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("set reaction block config: %w", err)
		}
	}()
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidReactionBlockInput("guild_id is required")
	}

	normalized, err := NormalizeReactionBlockConfig(cfg)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetReactionBlockConfig: %w", err)
	}
	return mgr.updateGuildConfig(scope, func(guildConfig *GuildConfig) error {
		guildConfig.ReactionBlocks = normalized
		return nil
	})
}

func invalidReactionBlockInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidReactionBlockInput, fmt.Sprintf(format, args...))
}

func normalizeReactionBlockRuleConfig(in ReactionBlockRuleConfig) (ReactionBlockRuleConfig, error) {
	reactorUserID := strings.TrimSpace(in.ReactorUserID)
	if reactorUserID == "" {
		return ReactionBlockRuleConfig{}, fmt.Errorf("reactor_user_id is required")
	}
	if !isAllDigits(reactorUserID) {
		return ReactionBlockRuleConfig{}, fmt.Errorf("reactor_user_id must be numeric")
	}

	targetUserID := strings.TrimSpace(in.TargetUserID)
	if targetUserID == "" {
		return ReactionBlockRuleConfig{}, fmt.Errorf("target_user_id is required")
	}
	if !isAllDigits(targetUserID) {
		return ReactionBlockRuleConfig{}, fmt.Errorf("target_user_id must be numeric")
	}

	emojis, err := normalizeReactionBlockEmojiConfigs(in.Emojis)
	if err != nil {
		return ReactionBlockRuleConfig{}, fmt.Errorf("normalizeReactionBlockRuleConfig: %w", err)
	}
	if len(emojis) == 0 {
		return ReactionBlockRuleConfig{}, fmt.Errorf("emojis must contain at least one blocked emoji")
	}

	return ReactionBlockRuleConfig{
		ReactorUserID: reactorUserID,
		TargetUserID:  targetUserID,
		Emojis:        emojis,
	}, nil
}

func normalizeReactionBlockEmojiConfigs(in []ReactionBlockEmojiConfig) ([]ReactionBlockEmojiConfig, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]ReactionBlockEmojiConfig, 0, len(in))
	indexByKey := make(map[string]int, len(in))
	for idx, emoji := range in {
		normalized, err := normalizeReactionBlockEmojiConfig(emoji)
		if err != nil {
			return nil, fmt.Errorf("emojis[%d]: %w", idx, err)
		}
		key := reactionBlockEmojiKey(normalized)
		if existingIdx, ok := indexByKey[key]; ok {
			out[existingIdx] = mergeReactionBlockEmojiConfig(out[existingIdx], normalized)
			continue
		}
		indexByKey[key] = len(out)
		out = append(out, normalized)
	}
	if len(out) == 0 {
		return nil, nil
	}
	sort.Slice(out, func(i, j int) bool {
		return reactionBlockEmojiKey(out[i]) < reactionBlockEmojiKey(out[j])
	})
	return out, nil
}

func normalizeReactionBlockEmojiConfig(in ReactionBlockEmojiConfig) (ReactionBlockEmojiConfig, error) {
	kind := reactionBlockEmojiKind(in.Kind)
	value := strings.TrimSpace(in.Value)
	name := strings.TrimSpace(in.Name)
	alias := normalizeReactionBlockAlias(in.Alias)

	switch kind {
	case ReactionBlockEmojiKindCustom:
		if value == "" {
			return ReactionBlockEmojiConfig{}, fmt.Errorf("custom emoji value is required")
		}
		if !isAllDigits(value) {
			return ReactionBlockEmojiConfig{}, fmt.Errorf("custom emoji value must be numeric")
		}
		return ReactionBlockEmojiConfig{
			Kind:     kind,
			Value:    value,
			Name:     name,
			Animated: in.Animated,
		}, nil
	case ReactionBlockEmojiKindUnicode:
		if value == "" {
			return ReactionBlockEmojiConfig{}, fmt.Errorf("unicode emoji value is required")
		}
		return ReactionBlockEmojiConfig{
			Kind:  kind,
			Value: value,
			Alias: alias,
		}, nil
	default:
		return ReactionBlockEmojiConfig{}, fmt.Errorf("emoji kind must be %q or %q", ReactionBlockEmojiKindCustom, ReactionBlockEmojiKindUnicode)
	}
}

func normalizeReactionBlockAlias(alias string) string {
	alias = strings.ToLower(strings.TrimSpace(alias))
	if alias == "" {
		return ""
	}
	if strings.Count(alias, ":") < 2 || !strings.HasPrefix(alias, ":") || !strings.HasSuffix(alias, ":") {
		return ""
	}
	return alias
}

func reactionBlockEmojiKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case ReactionBlockEmojiKindCustom:
		return ReactionBlockEmojiKindCustom
	case ReactionBlockEmojiKindUnicode:
		return ReactionBlockEmojiKindUnicode
	default:
		return ""
	}
}

func reactionBlockPairKey(reactorUserID, targetUserID string) string {
	reactorUserID = strings.TrimSpace(reactorUserID)
	targetUserID = strings.TrimSpace(targetUserID)
	if reactorUserID == "" || targetUserID == "" {
		return ""
	}
	return reactorUserID + ":" + targetUserID
}

func reactionBlockEmojiKey(emoji ReactionBlockEmojiConfig) string {
	kind := reactionBlockEmojiKind(emoji.Kind)
	value := strings.TrimSpace(emoji.Value)
	if kind == "" || value == "" {
		return ""
	}
	return kind + ":" + value
}

func mergeReactionBlockEmojiConfig(current, incoming ReactionBlockEmojiConfig) ReactionBlockEmojiConfig {
	if current.Name == "" {
		current.Name = incoming.Name
	}
	if current.Alias == "" {
		current.Alias = incoming.Alias
	}
	current.Animated = current.Animated || incoming.Animated
	return current
}

func cloneReactionBlockRuleConfig(in ReactionBlockRuleConfig) ReactionBlockRuleConfig {
	out := ReactionBlockRuleConfig{
		ReactorUserID: in.ReactorUserID,
		TargetUserID:  in.TargetUserID,
	}
	if len(in.Emojis) > 0 {
		out.Emojis = make([]ReactionBlockEmojiConfig, 0, len(in.Emojis))
		out.Emojis = append(out.Emojis, in.Emojis...)
	}
	return out
}

```

// === FILE: pkg/files/reaction_blocks_test.go ===
```go
package files

import (
	"errors"
	"testing"
)

func TestNormalizeReactionBlockConfigMergesPairsAndDedupesEmojis(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeReactionBlockConfig(ReactionBlockConfig{Rules: []ReactionBlockRuleConfig{
		{
			ReactorUserID: " 222222222222222222 ",
			TargetUserID:  " 111111111111111111 ",
			Emojis: []ReactionBlockEmojiConfig{
				{Kind: ReactionBlockEmojiKindCustom, Value: " 987654321098765432 ", Name: "skrunklytest"},
				{Kind: ReactionBlockEmojiKindUnicode, Value: "❌", Alias: " :X: "},
			},
		},
		{
			ReactorUserID: "222222222222222222",
			TargetUserID:  "111111111111111111",
			Emojis: []ReactionBlockEmojiConfig{
				{Kind: ReactionBlockEmojiKindUnicode, Value: "❌"},
				{Kind: ReactionBlockEmojiKindCustom, Value: "123456789012345678", Name: "blobwave"},
			},
		},
	}})
	if err != nil {
		t.Fatalf("NormalizeReactionBlockConfig() failed: %v", err)
	}
	if len(normalized.Rules) != 1 {
		t.Fatalf("expected one merged rule, got %+v", normalized.Rules)
	}

	rule := normalized.Rules[0]
	if rule.ReactorUserID != "222222222222222222" {
		t.Fatalf("expected canonical reactor user id, got %q", rule.ReactorUserID)
	}
	if rule.TargetUserID != "111111111111111111" {
		t.Fatalf("expected canonical target user id, got %q", rule.TargetUserID)
	}
	if len(rule.Emojis) != 3 {
		t.Fatalf("expected three unique blocked emojis, got %+v", rule.Emojis)
	}
	if rule.Emojis[2].Alias != ":x:" {
		t.Fatalf("expected unicode alias to be normalized, got %+v", rule.Emojis[2])
	}
	if !normalized.BlocksEmojiForPair(
		"222222222222222222",
		"111111111111111111",
		ReactionBlockEmojiConfig{Kind: ReactionBlockEmojiKindCustom, Value: "987654321098765432"},
	) {
		t.Fatalf("expected custom emoji to match normalized rule, got %+v", normalized)
	}
	if !normalized.BlocksEmojiForPair(
		"222222222222222222",
		"111111111111111111",
		ReactionBlockEmojiConfig{Kind: ReactionBlockEmojiKindUnicode, Value: "❌"},
	) {
		t.Fatalf("expected unicode emoji to match normalized rule, got %+v", normalized)
	}
}

func TestSetReactionBlockConfigCanonicalizesAndReadsBack(t *testing.T) {
	t.Parallel()

	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, nil)

	err := mgr.SetReactionBlockConfig("g1", ReactionBlockConfig{Rules: []ReactionBlockRuleConfig{{
		ReactorUserID: " 222222222222222222 ",
		TargetUserID:  " 111111111111111111 ",
		Emojis: []ReactionBlockEmojiConfig{{
			Kind:  ReactionBlockEmojiKindCustom,
			Value: " 987654321098765432 ",
			Name:  "skrunklytest",
		}},
	}}})
	if err != nil {
		t.Fatalf("SetReactionBlockConfig() failed: %v", err)
	}

	cfg, err := mgr.ReactionBlockConfig("g1")
	if err != nil {
		t.Fatalf("ReactionBlockConfig() failed: %v", err)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("expected one persisted rule, got %+v", cfg)
	}
	if cfg.Rules[0].ReactorUserID != "222222222222222222" || cfg.Rules[0].TargetUserID != "111111111111111111" {
		t.Fatalf("expected canonical persisted pair ids, got %+v", cfg.Rules[0])
	}
	if len(cfg.Rules[0].Emojis) != 1 {
		t.Fatalf("expected one persisted emoji, got %+v", cfg.Rules[0].Emojis)
	}
	if cfg.Rules[0].Emojis[0].Display() != "<:skrunklytest:987654321098765432>" {
		t.Fatalf("expected custom emoji display label, got %+v", cfg.Rules[0].Emojis[0])
	}
}

func TestSetReactionBlockConfigRollsBackOnSaveError(t *testing.T) {
	t.Parallel()

	saveErr := errors.New("save failed")
	mgr, _ := newTransactionalTestManager(t, &BotConfig{
		Guilds: []GuildConfig{{GuildID: "g1"}},
	}, saveErr)

	err := mgr.SetReactionBlockConfig("g1", ReactionBlockConfig{Rules: []ReactionBlockRuleConfig{{
		ReactorUserID: "222222222222222222",
		TargetUserID:  "111111111111111111",
		Emojis: []ReactionBlockEmojiConfig{{
			Kind:  ReactionBlockEmojiKindUnicode,
			Value: "❌",
			Alias: ":x:",
		}},
	}}})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}

	cfg := mgr.SnapshotConfig()
	if len(cfg.Guilds) != 1 {
		t.Fatalf("expected guild config to remain intact, got %+v", cfg.Guilds)
	}
	if !cfg.Guilds[0].ReactionBlocks.IsZero() {
		t.Fatalf("expected reaction block config rollback, got %+v", cfg.Guilds[0].ReactionBlocks)
	}
}

```

// === FILE: pkg/files/role_panel.go ===
```go
package files

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

var (
	// ErrRolePanelNotFound indicates no role panel matched the requested key.
	ErrRolePanelNotFound = errors.New("role panel not found")
	// ErrRolePanelButtonNotFound indicates no panel button matched the requested role ID.
	ErrRolePanelButtonNotFound = errors.New("role panel button not found")
	// ErrRolePanelPostingNotFound indicates no posting matched the requested message ID.
	ErrRolePanelPostingNotFound = errors.New("role panel posting not found")
	// ErrInvalidRolePanelInput indicates invalid role panel input payload.
	ErrInvalidRolePanelInput = errors.New("invalid role panel input")
)

const (
	// RolePanelMaxButtons mirrors Discord's hard cap of 25 components per
	// message (5 ActionRows × 5 buttons each).
	RolePanelMaxButtons = 25
	// RolePanelKeyMaxLen bounds the per-guild panel key so command custom
	// IDs and config keys stay readable.
	RolePanelKeyMaxLen = 32
	// RolePanelLabelMaxLen mirrors Discord's button label limit.
	RolePanelLabelMaxLen = 80
	// RolePanelTitleMaxLen mirrors Discord's embed title limit.
	RolePanelTitleMaxLen = 256
	// RolePanelDescriptionMaxLen is the embed description limit. Discord
	// caps at 4096; the slightly smaller bound here leaves a margin for
	// the renderer to add suffixes if needed without re-validating.
	RolePanelDescriptionMaxLen = 4000
	// RolePanelColorMax is the maximum allowed 24-bit RGB color value.
	RolePanelColorMax = 0xFFFFFF
	// RolePanelAuthorMaxLen mirrors Discord's embed author name limit.
	RolePanelAuthorMaxLen = 256
	// RolePanelFooterMaxLen mirrors Discord's embed footer text limit.
	RolePanelFooterMaxLen = 2048
	// RolePanelFieldNameMaxLen mirrors Discord's embed field name limit.
	RolePanelFieldNameMaxLen = 256
	// RolePanelFieldValueMaxLen mirrors Discord's embed field value limit.
	RolePanelFieldValueMaxLen = 1024
	// RolePanelMaxFields mirrors Discord's embed fields limit.
	RolePanelMaxFields = 25
	// RolePanelMaxTotalLen mirrors Discord's embed character limit.
	RolePanelMaxTotalLen = 6000
)

// RolePanelEmbedFieldConfig captures one field in a role panel embed.
type RolePanelEmbedFieldConfig struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// RolePanelButtonConfig captures one toggleable role button on a panel.
//
// EmojiID is set for custom Discord emojis; EmojiName carries either the
// custom emoji name (when EmojiID is set) or the unicode glyph (when
// EmojiID is empty). EmojiAnimated is only meaningful for custom emojis.
type RolePanelButtonConfig struct {
	RoleID        string `json:"role_id"`
	Label         string `json:"label"`
	EmojiName     string `json:"emoji_name,omitempty"`
	EmojiID       string `json:"emoji_id,omitempty"`
	EmojiAnimated bool   `json:"emoji_animated,omitempty"`
}

// RolePanelPostingConfig identifies one Discord message authored by
// the bot that materializes a role panel. Postings are recorded after
// /roles post succeeds so /roles delete and /roles refresh can edit
// the original messages (strip components on delete, re-render
// embed+buttons on refresh) instead of leaving them frozen and
// half-functional.
type RolePanelPostingConfig struct {
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
	WebhookID    string `json:"webhook_id,omitempty"`
	WebhookToken string `json:"webhook_token,omitempty"`
}

// IsZero reports whether the posting carries no meaningful data.
func (p RolePanelPostingConfig) IsZero() bool {
	return strings.TrimSpace(p.ChannelID) == "" && strings.TrimSpace(p.MessageID) == "" && strings.TrimSpace(p.WebhookID) == "" && strings.TrimSpace(p.WebhookToken) == ""
}

// RolePanelConfig captures one keyed role panel for a guild.
type RolePanelConfig struct {
	Key           string                      `json:"key"`
	Title         string                      `json:"title,omitempty"`
	Description   string                      `json:"description,omitempty"`
	Color         int                         `json:"color,omitempty"`
	AuthorName    string                      `json:"author_name,omitempty"`
	AuthorIconURL string                      `json:"author_icon_url,omitempty"`
	FooterText    string                      `json:"footer_text,omitempty"`
	FooterIconURL string                      `json:"footer_icon_url,omitempty"`
	ImageURL      string                      `json:"image_url,omitempty"`
	ThumbnailURL  string                      `json:"thumbnail_url,omitempty"`
	Fields        []RolePanelEmbedFieldConfig `json:"fields,omitempty"`
	Buttons       []RolePanelButtonConfig     `json:"buttons,omitempty"`
	Postings      []RolePanelPostingConfig    `json:"postings,omitempty"`
}

// IsZero reports whether the button carries no meaningful data.
func (b RolePanelButtonConfig) IsZero() bool {
	return strings.TrimSpace(b.RoleID) == "" &&
		strings.TrimSpace(b.Label) == "" &&
		strings.TrimSpace(b.EmojiName) == "" &&
		strings.TrimSpace(b.EmojiID) == ""
}

// HasEmoji reports whether the button carries either a custom or unicode emoji.
func (b RolePanelButtonConfig) HasEmoji() bool {
	return strings.TrimSpace(b.EmojiName) != "" || strings.TrimSpace(b.EmojiID) != ""
}

// IsZero reports whether the panel carries no meaningful data.
func (cfg RolePanelConfig) IsZero() bool {
	return strings.TrimSpace(cfg.Key) == "" &&
		strings.TrimSpace(cfg.Title) == "" &&
		strings.TrimSpace(cfg.Description) == "" &&
		cfg.Color == 0 &&
		strings.TrimSpace(cfg.AuthorName) == "" &&
		strings.TrimSpace(cfg.AuthorIconURL) == "" &&
		strings.TrimSpace(cfg.FooterText) == "" &&
		strings.TrimSpace(cfg.FooterIconURL) == "" &&
		strings.TrimSpace(cfg.ImageURL) == "" &&
		strings.TrimSpace(cfg.ThumbnailURL) == "" &&
		len(cfg.Fields) == 0 &&
		len(cfg.Buttons) == 0 &&
		len(cfg.Postings) == 0
}

// CloneRolePanelConfig returns a deep copy safe for callers to mutate.
func CloneRolePanelConfig(in RolePanelConfig) RolePanelConfig {
	return cloneRolePanel(in)
}

// CloneRolePanelConfigs returns a deep copy of the panel slice.
func CloneRolePanelConfigs(in []RolePanelConfig) []RolePanelConfig {
	return cloneRolePanels(in)
}

// --- Normalization ---

// NormalizeRolePanelKey lower-cases and trims a key in the canonical form
// used for lookup and storage. Returns an empty string when the input is
// blank after normalization.
func NormalizeRolePanelKey(raw string) string {
	out := strings.TrimSpace(raw)
	out = strings.ToLower(out)
	return out
}

func validateRolePanelKey(raw string) (string, error) {
	out := NormalizeRolePanelKey(raw)
	if out == "" {
		return "", invalidRolePanelInput("key is required")
	}
	if utf8.RuneCountInString(out) > RolePanelKeyMaxLen {
		return "", invalidRolePanelInput("key must be at most %d characters", RolePanelKeyMaxLen)
	}
	for _, r := range out {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return "", invalidRolePanelInput("key may only contain lowercase letters, digits, '-' and '_'")
		}
	}
	return out, nil
}

func normalizeRolePanelButton(in RolePanelButtonConfig) (RolePanelButtonConfig, error) {
	out := RolePanelButtonConfig{
		RoleID:        strings.TrimSpace(in.RoleID),
		Label:         sanitizeSingleLine(in.Label),
		EmojiName:     strings.TrimSpace(in.EmojiName),
		EmojiID:       strings.TrimSpace(in.EmojiID),
		EmojiAnimated: in.EmojiAnimated,
	}
	if out.RoleID == "" {
		return RolePanelButtonConfig{}, invalidRolePanelInput("role_id is required")
	}
	if !isAllDigits(out.RoleID) {
		return RolePanelButtonConfig{}, invalidRolePanelInput("role_id must be numeric")
	}
	if out.Label == "" {
		return RolePanelButtonConfig{}, invalidRolePanelInput("label is required")
	}
	if utf8.RuneCountInString(out.Label) > RolePanelLabelMaxLen {
		return RolePanelButtonConfig{}, invalidRolePanelInput("label must be at most %d characters", RolePanelLabelMaxLen)
	}
	if out.EmojiID != "" {
		if !isAllDigits(out.EmojiID) {
			return RolePanelButtonConfig{}, invalidRolePanelInput("emoji_id must be numeric")
		}
		if out.EmojiName == "" {
			return RolePanelButtonConfig{}, invalidRolePanelInput("emoji_name is required when emoji_id is set")
		}
	} else {
		out.EmojiAnimated = false
	}
	return out, nil
}

func validateRolePanelEmbedFields(in RolePanelConfig) (RolePanelConfig, error) {
	out := in
	out.Title = strings.TrimSpace(in.Title)
	out.Description = strings.TrimSpace(in.Description)
	out.AuthorName = strings.TrimSpace(in.AuthorName)
	out.AuthorIconURL = strings.TrimSpace(in.AuthorIconURL)
	out.FooterText = strings.TrimSpace(in.FooterText)
	out.FooterIconURL = strings.TrimSpace(in.FooterIconURL)
	out.ImageURL = strings.TrimSpace(in.ImageURL)
	out.ThumbnailURL = strings.TrimSpace(in.ThumbnailURL)

	if utf8.RuneCountInString(out.Title) > RolePanelTitleMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("title must be at most %d characters", RolePanelTitleMaxLen)
	}
	if utf8.RuneCountInString(out.Description) > RolePanelDescriptionMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("description must be at most %d characters", RolePanelDescriptionMaxLen)
	}
	if out.Color < 0 || out.Color > RolePanelColorMax {
		return RolePanelConfig{}, invalidRolePanelInput("color must be in range [0, %d]", RolePanelColorMax)
	}
	if utf8.RuneCountInString(out.AuthorName) > RolePanelAuthorMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("author_name must be at most %d characters", RolePanelAuthorMaxLen)
	}
	if utf8.RuneCountInString(out.FooterText) > RolePanelFooterMaxLen {
		return RolePanelConfig{}, invalidRolePanelInput("footer_text must be at most %d characters", RolePanelFooterMaxLen)
	}
	return out, nil
}

func rolePanelTotalLen(embed RolePanelConfig) int {
	count := utf8.RuneCountInString(embed.Title) +
		utf8.RuneCountInString(embed.Description) +
		utf8.RuneCountInString(embed.AuthorName) +
		utf8.RuneCountInString(embed.FooterText)
	for _, f := range embed.Fields {
		count += utf8.RuneCountInString(f.Name) + utf8.RuneCountInString(f.Value)
	}
	return count
}

func normalizeRolePanelEmbedField(in RolePanelEmbedFieldConfig) (RolePanelEmbedFieldConfig, error) {
	out := RolePanelEmbedFieldConfig{
		Name:   strings.TrimSpace(in.Name),
		Value:  strings.TrimSpace(in.Value),
		Inline: in.Inline,
	}
	if out.Name == "" {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field name is required")
	}
	if out.Value == "" {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field value is required")
	}
	if utf8.RuneCountInString(out.Name) > RolePanelFieldNameMaxLen {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field name must be at most %d characters", RolePanelFieldNameMaxLen)
	}
	if utf8.RuneCountInString(out.Value) > RolePanelFieldValueMaxLen {
		return RolePanelEmbedFieldConfig{}, invalidRolePanelInput("field value must be at most %d characters", RolePanelFieldValueMaxLen)
	}
	return out, nil
}

func normalizeRolePanelPosting(in RolePanelPostingConfig) (RolePanelPostingConfig, error) {
	out := RolePanelPostingConfig{
		ChannelID:    strings.TrimSpace(in.ChannelID),
		MessageID:    strings.TrimSpace(in.MessageID),
		WebhookID:    strings.TrimSpace(in.WebhookID),
		WebhookToken: strings.TrimSpace(in.WebhookToken),
	}
	if out.ChannelID == "" {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.channel_id is required")
	}
	if !isAllDigits(out.ChannelID) {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.channel_id must be numeric")
	}
	if out.MessageID == "" {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.message_id is required")
	}
	if !isAllDigits(out.MessageID) {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.message_id must be numeric")
	}
	if out.WebhookID != "" && !isAllDigits(out.WebhookID) {
		return RolePanelPostingConfig{}, invalidRolePanelInput("posting.webhook_id must be numeric")
	}
	return out, nil
}

func normalizeRolePanel(in RolePanelConfig) (RolePanelConfig, error) {
	key, err := validateRolePanelKey(in.Key)
	if err != nil {
		return RolePanelConfig{}, fmt.Errorf("normalizeRolePanel: %w", err)
	}
	embedFields, err := validateRolePanelEmbedFields(in)
	if err != nil {
		return RolePanelConfig{}, fmt.Errorf("normalizeRolePanel: %w", err)
	}

	fields := make([]RolePanelEmbedFieldConfig, 0, len(in.Fields))
	for i, f := range in.Fields {
		nf, err := normalizeRolePanelEmbedField(f)
		if err != nil {
			return RolePanelConfig{}, fmt.Errorf("fields[%d]: %w", i, err)
		}
		fields = append(fields, nf)
	}
	if len(fields) > RolePanelMaxFields {
		return RolePanelConfig{}, invalidRolePanelInput("panel must have at most %d fields", RolePanelMaxFields)
	}

	seen := make(map[string]struct{}, len(in.Buttons))
	buttons := make([]RolePanelButtonConfig, 0, len(in.Buttons))
	for i, b := range in.Buttons {
		nb, err := normalizeRolePanelButton(b)
		if err != nil {
			return RolePanelConfig{}, fmt.Errorf("buttons[%d]: %w", i, err)
		}
		if _, dup := seen[nb.RoleID]; dup {
			continue
		}
		seen[nb.RoleID] = struct{}{}
		buttons = append(buttons, nb)
	}
	if len(buttons) > RolePanelMaxButtons {
		return RolePanelConfig{}, invalidRolePanelInput("panel must have at most %d buttons", RolePanelMaxButtons)
	}

	postings, err := normalizeRolePanelPostingList(in.Postings)
	if err != nil {
		return RolePanelConfig{}, fmt.Errorf("normalizeRolePanel: %w", err)
	}

	return RolePanelConfig{
		Key:           key,
		Title:         embedFields.Title,
		Description:   embedFields.Description,
		Color:         embedFields.Color,
		AuthorName:    embedFields.AuthorName,
		AuthorIconURL: embedFields.AuthorIconURL,
		FooterText:    embedFields.FooterText,
		FooterIconURL: embedFields.FooterIconURL,
		ImageURL:      embedFields.ImageURL,
		ThumbnailURL:  embedFields.ThumbnailURL,
		Fields:        fields,
		Buttons:       buttons,
		Postings:      postings,
	}, nil
}

func normalizeRolePanelPostingList(in []RolePanelPostingConfig) ([]RolePanelPostingConfig, error) {
	if len(in) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]RolePanelPostingConfig, 0, len(in))
	for i, p := range in {
		np, err := normalizeRolePanelPosting(p)
		if err != nil {
			return nil, fmt.Errorf("postings[%d]: %w", i, err)
		}
		key := np.MessageID
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, np)
	}
	return out, nil
}

func cloneRolePanelButton(in RolePanelButtonConfig) RolePanelButtonConfig {
	return RolePanelButtonConfig{
		RoleID:        in.RoleID,
		Label:         in.Label,
		EmojiName:     in.EmojiName,
		EmojiID:       in.EmojiID,
		EmojiAnimated: in.EmojiAnimated,
	}
}

func cloneRolePanel(in RolePanelConfig) RolePanelConfig {
	out := RolePanelConfig{
		Key:           in.Key,
		Title:         in.Title,
		Description:   in.Description,
		Color:         in.Color,
		AuthorName:    in.AuthorName,
		AuthorIconURL: in.AuthorIconURL,
		FooterText:    in.FooterText,
		FooterIconURL: in.FooterIconURL,
		ImageURL:      in.ImageURL,
		ThumbnailURL:  in.ThumbnailURL,
	}
	if len(in.Fields) > 0 {
		out.Fields = make([]RolePanelEmbedFieldConfig, 0, len(in.Fields))
		for _, f := range in.Fields {
			out.Fields = append(out.Fields, RolePanelEmbedFieldConfig{
				Name:   f.Name,
				Value:  f.Value,
				Inline: f.Inline,
			})
		}
	}
	if len(in.Buttons) > 0 {
		out.Buttons = make([]RolePanelButtonConfig, 0, len(in.Buttons))
		for _, b := range in.Buttons {
			out.Buttons = append(out.Buttons, cloneRolePanelButton(b))
		}
	}
	if len(in.Postings) > 0 {
		out.Postings = make([]RolePanelPostingConfig, 0, len(in.Postings))
		for _, p := range in.Postings {
			out.Postings = append(out.Postings, p)
		}
	}
	return out
}

func cloneRolePanels(in []RolePanelConfig) []RolePanelConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]RolePanelConfig, 0, len(in))
	for _, p := range in {
		out = append(out, cloneRolePanel(p))
	}
	return out
}

func sortRolePanels(panels []RolePanelConfig) {
	sort.SliceStable(panels, func(i, j int) bool {
		return panels[i].Key < panels[j].Key
	})
}

func findRolePanelIndex(panels []RolePanelConfig, key string) int {
	target := NormalizeRolePanelKey(key)
	if target == "" {
		return -1
	}
	for i, p := range panels {
		if NormalizeRolePanelKey(p.Key) == target {
			return i
		}
	}
	return -1
}

func findRolePanelButtonIndex(buttons []RolePanelButtonConfig, roleID string) int {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return -1
	}
	for i, b := range buttons {
		if strings.TrimSpace(b.RoleID) == roleID {
			return i
		}
	}
	return -1
}

func findRolePanelPostingIndex(postings []RolePanelPostingConfig, messageID string) int {
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		return -1
	}
	for i, p := range postings {
		if strings.TrimSpace(p.MessageID) == messageID {
			return i
		}
	}
	return -1
}

func invalidRolePanelInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRolePanelInput, fmt.Sprintf(format, args...))
}

// --- ConfigManager API ---

// RolePanels returns the role panels configured for a guild in
// deterministic key order. Callers receive a deep copy and may mutate
// freely.
func (mgr *ConfigManager) RolePanels(guildID string) ([]RolePanelConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return nil, invalidRolePanelInput("guild_id is required")
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return nil, fmt.Errorf("ConfigManager.RolePanels: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}
	out := cloneRolePanels(guildConfig.RolePanels)
	sortRolePanels(out)
	return out, nil
}

// RolePanel returns one panel by key. Returns ErrRolePanelNotFound when
// the panel does not exist.
func (mgr *ConfigManager) RolePanel(guildID, key string) (RolePanelConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return RolePanelConfig{}, invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return RolePanelConfig{}, fmt.Errorf("ConfigManager.RolePanel: %w", err)
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return RolePanelConfig{}, fmt.Errorf("ConfigManager.RolePanel: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}
	idx := findRolePanelIndex(guildConfig.RolePanels, target)
	if idx < 0 {
		return RolePanelConfig{}, fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
	}
	return cloneRolePanel(guildConfig.RolePanels[idx]), nil
}

// SetRolePanelEmbed sets the embed-level fields for one panel,
// creating the panel when missing. Buttons, fields, and postings on an
// existing panel are preserved.
func (mgr *ConfigManager) SetRolePanelEmbed(guildID, key string, embed RolePanelConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetRolePanelEmbed: %w", err)
	}

	validated, err := validateRolePanelEmbedFields(embed)
	if err != nil {
		return fmt.Errorf("ConfigManager.SetRolePanelEmbed: %w", err)
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			validated.Key = target
			if rolePanelTotalLen(validated) > RolePanelMaxTotalLen {
				return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
			}
			gc.RolePanels = append(gc.RolePanels, validated)
			sortRolePanels(gc.RolePanels)
			return nil
		}

		copyEmbed := gc.RolePanels[idx]
		copyEmbed.Title = validated.Title
		copyEmbed.Description = validated.Description
		copyEmbed.Color = validated.Color
		copyEmbed.AuthorName = validated.AuthorName
		copyEmbed.AuthorIconURL = validated.AuthorIconURL
		copyEmbed.FooterText = validated.FooterText
		copyEmbed.FooterIconURL = validated.FooterIconURL
		copyEmbed.ImageURL = validated.ImageURL
		copyEmbed.ThumbnailURL = validated.ThumbnailURL

		if rolePanelTotalLen(copyEmbed) > RolePanelMaxTotalLen {
			return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
		}

		gc.RolePanels[idx] = copyEmbed
		return nil
	})
}

// AddRolePanelField appends a field to the panel's embed.
func (mgr *ConfigManager) AddRolePanelField(guildID, key string, field RolePanelEmbedFieldConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.AddRolePanelField: %w", err)
	}
	validated, err := normalizeRolePanelEmbedField(field)
	if err != nil {
		return fmt.Errorf("ConfigManager.AddRolePanelField: %w", err)
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		if len(gc.RolePanels[idx].Fields) >= RolePanelMaxFields {
			return invalidRolePanelInput("panel must have at most %d fields", RolePanelMaxFields)
		}

		copyEmbed := gc.RolePanels[idx]
		copyEmbed.Fields = append(copyEmbed.Fields, validated)

		if rolePanelTotalLen(copyEmbed) > RolePanelMaxTotalLen {
			return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
		}

		gc.RolePanels[idx] = copyEmbed
		return nil
	})
}

// RemoveRolePanelField removes a field from the panel's embed by its index (0-based).
func (mgr *ConfigManager) RemoveRolePanelField(guildID, key string, fieldIndex int) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.RemoveRolePanelField: %w", err)
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		fields := gc.RolePanels[idx].Fields
		if fieldIndex < 0 || fieldIndex >= len(fields) {
			return invalidRolePanelInput("invalid field index")
		}

		normalized := slices.Delete(fields, fieldIndex, fieldIndex+1)

		copyEmbed := gc.RolePanels[idx]
		copyEmbed.Fields = normalized

		if rolePanelTotalLen(copyEmbed) > RolePanelMaxTotalLen {
			return invalidRolePanelInput("panel total character count must be at most %d", RolePanelMaxTotalLen)
		}

		gc.RolePanels[idx] = copyEmbed
		return nil
	})
}

// UpsertRolePanelButton inserts a new button or replaces the existing
// one matching the same role ID, creating the panel when missing.
func (mgr *ConfigManager) UpsertRolePanelButton(guildID, key string, button RolePanelButtonConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.UpsertRolePanelButton: %w", err)
	}
	normalized, err := normalizeRolePanelButton(button)
	if err != nil {
		return fmt.Errorf("ConfigManager.UpsertRolePanelButton: %w", err)
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			gc.RolePanels = append(gc.RolePanels, RolePanelConfig{
				Key:     target,
				Buttons: []RolePanelButtonConfig{normalized},
			})
			sortRolePanels(gc.RolePanels)
			return nil
		}
		buttons := gc.RolePanels[idx].Buttons
		btnIdx := findRolePanelButtonIndex(buttons, normalized.RoleID)
		if btnIdx >= 0 {
			buttons[btnIdx] = normalized
			gc.RolePanels[idx].Buttons = buttons
			return nil
		}
		if len(buttons) >= RolePanelMaxButtons {
			return invalidRolePanelInput("panel must have at most %d buttons", RolePanelMaxButtons)
		}
		gc.RolePanels[idx].Buttons = append(buttons, normalized)
		return nil
	})
}

// DeleteRolePanelButton removes the button matching the given role ID
// from a panel. Returns ErrRolePanelNotFound or
// ErrRolePanelButtonNotFound when the targets do not exist.
func (mgr *ConfigManager) DeleteRolePanelButton(guildID, key, roleID string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.DeleteRolePanelButton: %w", err)
	}
	rid := strings.TrimSpace(roleID)
	if rid == "" {
		return invalidRolePanelInput("role_id is required")
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		btnIdx := findRolePanelButtonIndex(gc.RolePanels[idx].Buttons, rid)
		if btnIdx < 0 {
			return fmt.Errorf("%w: role_id=%s", ErrRolePanelButtonNotFound, rid)
		}
		gc.RolePanels[idx].Buttons = slices.Delete(gc.RolePanels[idx].Buttons, btnIdx, btnIdx+1)
		return nil
	})
}

// DeleteRolePanel removes the entire panel for a guild. Returns
// ErrRolePanelNotFound when the panel does not exist.
func (mgr *ConfigManager) DeleteRolePanel(guildID, key string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.DeleteRolePanel: %w", err)
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		gc.RolePanels = slices.Delete(gc.RolePanels, idx, idx+1)
		return nil
	})
}

// ListRolePanelPostings returns the persisted (channel_id, message_id)
// pairs for one panel in insertion order.
func (mgr *ConfigManager) ListRolePanelPostings(guildID, key string) ([]RolePanelPostingConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return nil, invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return nil, fmt.Errorf("ConfigManager.ListRolePanelPostings: %w", err)
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return nil, fmt.Errorf("ConfigManager.ListRolePanelPostings: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}
	idx := findRolePanelIndex(guildConfig.RolePanels, target)
	if idx < 0 {
		return nil, fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
	}
	postings := guildConfig.RolePanels[idx].Postings
	if len(postings) == 0 {
		return nil, nil
	}
	out := make([]RolePanelPostingConfig, len(postings))
	copy(out, postings)
	return out, nil
}

// AddRolePanelPosting records a (channel_id, message_id) pair on a
// panel. Duplicates by message ID are silently coalesced. The panel
// must already exist.
func (mgr *ConfigManager) AddRolePanelPosting(guildID, key string, posting RolePanelPostingConfig) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.AddRolePanelPosting: %w", err)
	}
	normalized, err := normalizeRolePanelPosting(posting)
	if err != nil {
		return fmt.Errorf("ConfigManager.AddRolePanelPosting: %w", err)
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		if findRolePanelPostingIndex(gc.RolePanels[idx].Postings, normalized.MessageID) >= 0 {
			return nil
		}
		gc.RolePanels[idx].Postings = append(gc.RolePanels[idx].Postings, normalized)
		return nil
	})
}

// RemoveRolePanelPosting drops a (channel_id, message_id) pair from a
// panel. Returns ErrRolePanelPostingNotFound when the message is not
// tracked.
func (mgr *ConfigManager) RemoveRolePanelPosting(guildID, key, messageID string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.RemoveRolePanelPosting: %w", err)
	}
	mid := strings.TrimSpace(messageID)
	if mid == "" {
		return invalidRolePanelInput("message_id is required")
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		pIdx := findRolePanelPostingIndex(gc.RolePanels[idx].Postings, mid)
		if pIdx < 0 {
			return fmt.Errorf("%w: message_id=%s", ErrRolePanelPostingNotFound, mid)
		}
		gc.RolePanels[idx].Postings = slices.Delete(gc.RolePanels[idx].Postings, pIdx, pIdx+1)
		return nil
	})
}

// RemoveRolePanelPostings drops multiple (channel_id, message_id) pairs from a
// panel. Message IDs that are not tracked are safely ignored.
func (mgr *ConfigManager) RemoveRolePanelPostings(guildID, key string, messageIDs []string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.RemoveRolePanelPostings: %w", err)
	}

	idsToRemove := make(map[string]bool, len(messageIDs))
	for _, id := range messageIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			idsToRemove[trimmed] = true
		}
	}
	if len(idsToRemove) == 0 {
		return nil
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}

		var kept []RolePanelPostingConfig
		for _, p := range gc.RolePanels[idx].Postings {
			if !idsToRemove[p.MessageID] {
				kept = append(kept, p)
			}
		}
		gc.RolePanels[idx].Postings = kept
		return nil
	})
}

// ClearRolePanelPostings drops every recorded posting for a panel.
// Used by /roles delete after the postings have been edited; the
// caller is responsible for the message-edit pass.
func (mgr *ConfigManager) ClearRolePanelPostings(guildID, key string) error {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return invalidRolePanelInput("guild_id is required")
	}
	target, err := validateRolePanelKey(key)
	if err != nil {
		return fmt.Errorf("ConfigManager.ClearRolePanelPostings: %w", err)
	}

	return mgr.updateGuildConfig(scope, func(gc *GuildConfig) error {
		idx := findRolePanelIndex(gc.RolePanels, target)
		if idx < 0 {
			return fmt.Errorf("%w: key=%s", ErrRolePanelNotFound, target)
		}
		gc.RolePanels[idx].Postings = nil
		return nil
	})
}

// FindRolePanelPosting searches all panels in a guild for a posting
// matching the message ID. Returns the panel key plus the posting on
// hit, or ErrRolePanelPostingNotFound when no panel tracks the
// message.
func (mgr *ConfigManager) FindRolePanelPosting(guildID, messageID string) (string, RolePanelPostingConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return "", RolePanelPostingConfig{}, invalidRolePanelInput("guild_id is required")
	}
	mid := strings.TrimSpace(messageID)
	if mid == "" {
		return "", RolePanelPostingConfig{}, invalidRolePanelInput("message_id is required")
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return "", RolePanelPostingConfig{}, fmt.Errorf("ConfigManager.FindRolePanelPosting: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}
	for _, panel := range guildConfig.RolePanels {
		pIdx := findRolePanelPostingIndex(panel.Postings, mid)
		if pIdx >= 0 {
			return panel.Key, panel.Postings[pIdx], nil
		}
	}
	return "", RolePanelPostingConfig{}, fmt.Errorf("%w: message_id=%s", ErrRolePanelPostingNotFound, mid)
}

// RolePanelButtonByRoleID searches all panels in a guild for a button
// matching the role ID. Used by the component handler to validate
// toggle requests against the current persisted configuration. Returns
// ErrRolePanelButtonNotFound when no panel button references the role.
func (mgr *ConfigManager) RolePanelButtonByRoleID(guildID, roleID string) (RolePanelConfig, RolePanelButtonConfig, error) {
	scope := strings.TrimSpace(guildID)
	if scope == "" {
		return RolePanelConfig{}, RolePanelButtonConfig{}, invalidRolePanelInput("guild_id is required")
	}
	rid := strings.TrimSpace(roleID)
	if rid == "" {
		return RolePanelConfig{}, RolePanelButtonConfig{}, invalidRolePanelInput("role_id is required")
	}

	guildConfig := mgr.GuildConfig(scope)
	if guildConfig == nil {
		return RolePanelConfig{}, RolePanelButtonConfig{}, fmt.Errorf("ConfigManager.RolePanelButtonByRoleID: %w: guild_id=%s", ErrGuildConfigNotFound, scope)
	}
	for _, panel := range guildConfig.RolePanels {
		btnIdx := findRolePanelButtonIndex(panel.Buttons, rid)
		if btnIdx >= 0 {
			return cloneRolePanel(panel), cloneRolePanelButton(panel.Buttons[btnIdx]), nil
		}
	}
	return RolePanelConfig{}, RolePanelButtonConfig{}, fmt.Errorf("%w: role_id=%s", ErrRolePanelButtonNotFound, rid)
}

```

// === FILE: pkg/files/role_panel_test.go ===
```go
package files

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func newRolePanelTestManager(t *testing.T, guildID string) *ConfigManager {
	t.Helper()
	mgr := NewConfigManagerWithStore(&mockConfigStore{}, nil)
	mgr.config = &BotConfig{Guilds: []GuildConfig{{GuildID: guildID}}}
	return mgr
}

func TestRolePanelKeyValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"trims and lowercases", "  Pings  ", "pings", false},
		{"keeps digits and dashes", "Welcome-Roles_2", "welcome-roles_2", false},
		{"rejects empty", "  ", "", true},
		{"rejects whitespace inside", "two words", "", true},
		{"rejects punctuation", "with.dot", "", true},
		{"rejects unicode letters", "rôles", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := validateRolePanelKey(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("normalized key = %q want %q", got, tc.want)
			}
		})
	}
}

func TestRolePanelButtonValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      RolePanelButtonConfig
		wantErr string
	}{
		{
			name: "valid custom emoji",
			in: RolePanelButtonConfig{
				RoleID:        "1380646673482518639",
				Label:         "Announcements",
				EmojiName:     "clouud",
				EmojiID:       "1378934415186464808",
				EmojiAnimated: false,
			},
		},
		{
			name: "valid unicode emoji",
			in: RolePanelButtonConfig{
				RoleID:    "1380646673482518639",
				Label:     "Hello",
				EmojiName: "👋",
			},
		},
		{
			name: "valid no emoji",
			in: RolePanelButtonConfig{
				RoleID: "1380646673482518639",
				Label:  "Click me",
			},
		},
		{
			name:    "missing role",
			in:      RolePanelButtonConfig{Label: "Click"},
			wantErr: "role_id is required",
		},
		{
			name:    "non-numeric role",
			in:      RolePanelButtonConfig{RoleID: "not-a-snowflake", Label: "Click"},
			wantErr: "role_id must be numeric",
		},
		{
			name:    "missing label",
			in:      RolePanelButtonConfig{RoleID: "100"},
			wantErr: "label is required",
		},
		{
			name:    "emoji id without name",
			in:      RolePanelButtonConfig{RoleID: "100", Label: "L", EmojiID: "200"},
			wantErr: "emoji_name is required when emoji_id is set",
		},
		{
			name:    "label too long",
			in:      RolePanelButtonConfig{RoleID: "100", Label: strings.Repeat("a", RolePanelLabelMaxLen+1)},
			wantErr: "label must be at most",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := normalizeRolePanelButton(tc.in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRolePanelEmbedFieldValidation(t *testing.T) {
	t.Parallel()
	validCfg := RolePanelConfig{
		Title:       "  Title  ",
		Description: "  Desc  ",
		Color:       16753104,
	}
	if _, err := validateRolePanelEmbedFields(validCfg); err != nil {
		t.Fatalf("unexpected error on valid input: %v", err)
	}
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{Color: -1}); err == nil {
		t.Fatalf("expected error for negative color")
	}
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{Color: RolePanelColorMax + 1}); err == nil {
		t.Fatalf("expected error for color overflow")
	}
	bigTitle := strings.Repeat("a", RolePanelTitleMaxLen+1)
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{Title: bigTitle}); err == nil {
		t.Fatalf("expected error for oversized title")
	}
	bigAuthor := strings.Repeat("a", RolePanelAuthorMaxLen+1)
	if _, err := validateRolePanelEmbedFields(RolePanelConfig{AuthorName: bigAuthor}); err == nil {
		t.Fatalf("expected error for oversized author name")
	}
}

func TestRolePanelFieldCRUD(t *testing.T) {
	t.Parallel()
	guildID := "guild-fields"
	mgr := newRolePanelTestManager(t, guildID)

	if err := mgr.SetRolePanelEmbed(guildID, "test-panel", RolePanelConfig{
		Title: "Panel with fields",
	}); err != nil {
		t.Fatalf("set embed: %v", err)
	}

	f1 := RolePanelEmbedFieldConfig{Name: "Field 1", Value: "Value 1"}
	if err := mgr.AddRolePanelField(guildID, "test-panel", f1); err != nil {
		t.Fatalf("add field 1: %v", err)
	}

	f2 := RolePanelEmbedFieldConfig{Name: "Field 2", Value: "Value 2", Inline: true}
	if err := mgr.AddRolePanelField(guildID, "test-panel", f2); err != nil {
		t.Fatalf("add field 2: %v", err)
	}

	panel, err := mgr.RolePanel(guildID, "test-panel")
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if len(panel.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(panel.Fields))
	}
	if panel.Fields[0].Name != "Field 1" || panel.Fields[1].Name != "Field 2" {
		t.Fatalf("fields did not match: %+v", panel.Fields)
	}

	if err := mgr.RemoveRolePanelField(guildID, "test-panel", 0); err != nil {
		t.Fatalf("remove field 0: %v", err)
	}

	panel, err = mgr.RolePanel(guildID, "test-panel")
	if err != nil {
		t.Fatalf("get panel after remove: %v", err)
	}
	if len(panel.Fields) != 1 || panel.Fields[0].Name != "Field 2" {
		t.Fatalf("expected 1 field named 'Field 2', got %+v", panel.Fields)
	}

	// Test out of bounds remove
	if err := mgr.RemoveRolePanelField(guildID, "test-panel", 5); err == nil {
		t.Fatalf("expected error for out of bounds field removal")
	}
}

func TestRolePanelCRUDLifecycle(t *testing.T) {
	t.Parallel()
	guildID := "guild-1"
	mgr := newRolePanelTestManager(t, guildID)

	if err := mgr.SetRolePanelEmbed(guildID, "pings", RolePanelConfig{
		Title:       "⋆｡°✩ ! ✩°｡⋆ Pings! ⋆｡°✩ ! ✩°｡⋆",
		Description: "Please select some of our optional roles below!",
		Color:       16753104,
	}); err != nil {
		t.Fatalf("set embed: %v", err)
	}

	if err := mgr.UpsertRolePanelButton(guildID, "pings", RolePanelButtonConfig{
		RoleID:    "1380646673482518639",
		Label:     "Announcements",
		EmojiName: "clouud",
		EmojiID:   "1378934415186464808",
	}); err != nil {
		t.Fatalf("upsert button: %v", err)
	}

	panel, err := mgr.RolePanel(guildID, "PINGS")
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	if panel.Key != "pings" {
		t.Fatalf("expected normalized key, got %q", panel.Key)
	}
	if panel.Color != 16753104 {
		t.Fatalf("color persisted incorrectly: %d", panel.Color)
	}
	if len(panel.Buttons) != 1 || panel.Buttons[0].RoleID != "1380646673482518639" {
		t.Fatalf("unexpected buttons after upsert: %+v", panel.Buttons)
	}

	if err := mgr.UpsertRolePanelButton(guildID, "pings", RolePanelButtonConfig{
		RoleID:    "1380646673482518639",
		Label:     "Announcements (renamed)",
		EmojiName: "clouud",
		EmojiID:   "1378934415186464808",
	}); err != nil {
		t.Fatalf("re-upsert button: %v", err)
	}
	panel, err = mgr.RolePanel(guildID, "pings")
	if err != nil {
		t.Fatalf("re-fetch panel: %v", err)
	}
	if len(panel.Buttons) != 1 || panel.Buttons[0].Label != "Announcements (renamed)" {
		t.Fatalf("re-upsert did not replace existing button: %+v", panel.Buttons)
	}

	panels, err := mgr.RolePanels(guildID)
	if err != nil {
		t.Fatalf("list panels: %v", err)
	}
	if len(panels) != 1 {
		t.Fatalf("expected one panel, got %d", len(panels))
	}

	if err := mgr.DeleteRolePanelButton(guildID, "pings", "1380646673482518639"); err != nil {
		t.Fatalf("delete button: %v", err)
	}
	panel, err = mgr.RolePanel(guildID, "pings")
	if err != nil {
		t.Fatalf("re-fetch panel after delete: %v", err)
	}
	if len(panel.Buttons) != 0 {
		t.Fatalf("expected zero buttons after delete, got %+v", panel.Buttons)
	}

	if err := mgr.DeleteRolePanel(guildID, "pings"); err != nil {
		t.Fatalf("delete panel: %v", err)
	}
	if _, err := mgr.RolePanel(guildID, "pings"); !errors.Is(err, ErrRolePanelNotFound) {
		t.Fatalf("expected ErrRolePanelNotFound after delete, got %v", err)
	}
}

func TestRolePanelButtonByRoleIDFindsAcrossPanels(t *testing.T) {
	t.Parallel()
	guildID := "guild-2"
	mgr := newRolePanelTestManager(t, guildID)

	if err := mgr.UpsertRolePanelButton(guildID, "alpha", RolePanelButtonConfig{
		RoleID: "111",
		Label:  "Alpha button",
	}); err != nil {
		t.Fatalf("upsert alpha: %v", err)
	}
	if err := mgr.UpsertRolePanelButton(guildID, "beta", RolePanelButtonConfig{
		RoleID: "222",
		Label:  "Beta button",
	}); err != nil {
		t.Fatalf("upsert beta: %v", err)
	}

	panel, btn, err := mgr.RolePanelButtonByRoleID(guildID, "222")
	if err != nil {
		t.Fatalf("find by role: %v", err)
	}
	if panel.Key != "beta" || btn.Label != "Beta button" {
		t.Fatalf("unexpected match: %+v / %+v", panel, btn)
	}

	if _, _, err := mgr.RolePanelButtonByRoleID(guildID, "999"); !errors.Is(err, ErrRolePanelButtonNotFound) {
		t.Fatalf("expected ErrRolePanelButtonNotFound, got %v", err)
	}
}

func TestRolePanelButtonCapEnforced(t *testing.T) {
	t.Parallel()
	guildID := "guild-cap"
	mgr := newRolePanelTestManager(t, guildID)
	for i := 0; i < RolePanelMaxButtons; i++ {
		btn := RolePanelButtonConfig{
			RoleID: pad19DigitID(i + 1),
			Label:  "Button",
		}
		if err := mgr.UpsertRolePanelButton(guildID, "cap", btn); err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
	}
	overflow := RolePanelButtonConfig{
		RoleID: pad19DigitID(RolePanelMaxButtons + 1),
		Label:  "Overflow",
	}
	if err := mgr.UpsertRolePanelButton(guildID, "cap", overflow); !errors.Is(err, ErrInvalidRolePanelInput) {
		t.Fatalf("expected ErrInvalidRolePanelInput at cap, got %v", err)
	}
}

func TestRolePanelMutationsAreSnapshotIsolated(t *testing.T) {
	t.Parallel()
	guildID := "guild-snap"
	mgr := newRolePanelTestManager(t, guildID)
	if err := mgr.UpsertRolePanelButton(guildID, "iso", RolePanelButtonConfig{
		RoleID: "100",
		Label:  "Original",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	panel, err := mgr.RolePanel(guildID, "iso")
	if err != nil {
		t.Fatalf("get panel: %v", err)
	}
	panel.Buttons[0].Label = "Mutated outside"

	again, err := mgr.RolePanel(guildID, "iso")
	if err != nil {
		t.Fatalf("re-fetch panel: %v", err)
	}
	if !reflect.DeepEqual(again.Buttons[0].Label, "Original") {
		t.Fatalf("snapshot leaked mutation: %q", again.Buttons[0].Label)
	}
}

func TestRolePanelPostingValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		in      RolePanelPostingConfig
		wantErr string
	}{
		{name: "valid", in: RolePanelPostingConfig{ChannelID: "100", MessageID: "200"}},
		{name: "missing channel", in: RolePanelPostingConfig{MessageID: "200"}, wantErr: "channel_id is required"},
		{name: "non-numeric channel", in: RolePanelPostingConfig{ChannelID: "abc", MessageID: "200"}, wantErr: "channel_id must be numeric"},
		{name: "missing message", in: RolePanelPostingConfig{ChannelID: "100"}, wantErr: "message_id is required"},
		{name: "non-numeric message", in: RolePanelPostingConfig{ChannelID: "100", MessageID: "abc"}, wantErr: "message_id must be numeric"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := normalizeRolePanelPosting(tc.in)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRolePanelPostingsCRUD(t *testing.T) {
	t.Parallel()
	guildID := "guild-postings"
	mgr := newRolePanelTestManager(t, guildID)
	if err := mgr.UpsertRolePanelButton(guildID, "pings", RolePanelButtonConfig{RoleID: "100", Label: "Click"}); err != nil {
		t.Fatalf("seed button: %v", err)
	}

	const (
		firstID  = "601"
		secondID = "602"
	)
	posting := RolePanelPostingConfig{ChannelID: "111", MessageID: firstID}
	if err := mgr.AddRolePanelPosting(guildID, "pings", posting); err != nil {
		t.Fatalf("add posting: %v", err)
	}

	if err := mgr.AddRolePanelPosting(guildID, "pings", posting); err != nil {
		t.Fatalf("re-add same posting must be idempotent, got: %v", err)
	}
	listed, err := mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list postings: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected dedupe to keep one entry, got %d", len(listed))
	}

	another := RolePanelPostingConfig{ChannelID: "111", MessageID: secondID}
	if err := mgr.AddRolePanelPosting(guildID, "pings", another); err != nil {
		t.Fatalf("add second posting: %v", err)
	}
	listed, err = mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list after second add: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 postings, got %d", len(listed))
	}

	if err := mgr.RemoveRolePanelPosting(guildID, "pings", firstID); err != nil {
		t.Fatalf("remove posting: %v", err)
	}
	listed, err = mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list after remove: %v", err)
	}
	if len(listed) != 1 || listed[0].MessageID != secondID {
		t.Fatalf("unexpected postings after remove: %+v", listed)
	}

	if err := mgr.RemoveRolePanelPosting(guildID, "pings", firstID); !errors.Is(err, ErrRolePanelPostingNotFound) {
		t.Fatalf("expected ErrRolePanelPostingNotFound for repeat remove, got %v", err)
	}

	if err := mgr.ClearRolePanelPostings(guildID, "pings"); err != nil {
		t.Fatalf("clear postings: %v", err)
	}
	listed, err = mgr.ListRolePanelPostings(guildID, "pings")
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected zero postings after clear, got %d", len(listed))
	}
}

func TestFindRolePanelPostingSearchesAcrossPanels(t *testing.T) {
	t.Parallel()
	guildID := "guild-find-posting"
	mgr := newRolePanelTestManager(t, guildID)
	if err := mgr.UpsertRolePanelButton(guildID, "alpha", RolePanelButtonConfig{RoleID: "100", Label: "A"}); err != nil {
		t.Fatalf("seed alpha: %v", err)
	}
	if err := mgr.UpsertRolePanelButton(guildID, "beta", RolePanelButtonConfig{RoleID: "200", Label: "B"}); err != nil {
		t.Fatalf("seed beta: %v", err)
	}
	if err := mgr.AddRolePanelPosting(guildID, "alpha", RolePanelPostingConfig{ChannelID: "11", MessageID: "501"}); err != nil {
		t.Fatalf("add alpha posting: %v", err)
	}
	if err := mgr.AddRolePanelPosting(guildID, "beta", RolePanelPostingConfig{ChannelID: "22", MessageID: "502"}); err != nil {
		t.Fatalf("add beta posting: %v", err)
	}

	key, posting, err := mgr.FindRolePanelPosting(guildID, "502")
	if err != nil {
		t.Fatalf("find posting: %v", err)
	}
	if key != "beta" || posting.ChannelID != "22" {
		t.Fatalf("unexpected match: %s / %+v", key, posting)
	}

	if _, _, err := mgr.FindRolePanelPosting(guildID, "999"); !errors.Is(err, ErrRolePanelPostingNotFound) {
		t.Fatalf("expected ErrRolePanelPostingNotFound, got %v", err)
	}
}

func pad19DigitID(n int) string {
	const base = "1000000000000000000"
	if n <= 0 {
		return base
	}
	out := []byte(base)
	for i := len(out) - 1; i >= 0 && n > 0; i-- {
		digit := byte(n%10) + '0'
		out[i] = digit
		n /= 10
	}
	return string(out)
}

```

// === FILE: pkg/files/runtime_config_test.go ===
```go
package files

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRuntimeConfigModerationLoggingEnabled(t *testing.T) {
	t.Parallel()

	if got := (RuntimeConfig{}).ModerationLoggingEnabled(); !got {
		t.Fatal("expected default moderation logging to be enabled")
	}

	enabled := true
	if got := (RuntimeConfig{ModerationLogging: &enabled}).ModerationLoggingEnabled(); !got {
		t.Fatal("expected moderation_logging=true to enable logs")
	}

	disabled := false
	if got := (RuntimeConfig{ModerationLogging: &disabled}).ModerationLoggingEnabled(); got {
		t.Fatal("expected moderation_logging=false to disable logs")
	}
}

func TestRuntimeConfigUnmarshalMigratesLegacyModerationLogMode(t *testing.T) {
	t.Parallel()

	var off RuntimeConfig
	if err := json.Unmarshal([]byte(`{"moderation_log_mode":"off"}`), &off); err != nil {
		t.Fatalf("unmarshal legacy off: %v", err)
	}
	if off.ModerationLogging == nil || *off.ModerationLogging {
		t.Fatalf("expected legacy moderation_log_mode=off to migrate to moderation_logging=false, got %+v", off.ModerationLogging)
	}

	var botOnly RuntimeConfig
	if err := json.Unmarshal([]byte(`{"moderation_log_mode":"bot_only"}`), &botOnly); err != nil {
		t.Fatalf("unmarshal legacy bot_only: %v", err)
	}
	if botOnly.ModerationLogging == nil || !*botOnly.ModerationLogging {
		t.Fatalf("expected legacy moderation_log_mode=bot_only to migrate to moderation_logging=true, got %+v", botOnly.ModerationLogging)
	}

	// Canonical value wins over the legacy key when both are present.
	var both RuntimeConfig
	if err := json.Unmarshal([]byte(`{"moderation_logging":true,"moderation_log_mode":"off"}`), &both); err != nil {
		t.Fatalf("unmarshal both: %v", err)
	}
	if both.ModerationLogging == nil || !*both.ModerationLogging {
		t.Fatalf("expected canonical moderation_logging=true to win, got %+v", both.ModerationLogging)
	}
}

func TestResolveRuntimeConfigModerationLoggingMerge(t *testing.T) {
	t.Parallel()

	globalEnabled := true
	guildDisabled := false

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			ModerationLogging: &globalEnabled,
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					ModerationLogging: &guildDisabled,
				},
			},
		},
	}

	rc := cfg.ResolveRuntimeConfig("g1")
	if rc.ModerationLogging == nil || *rc.ModerationLogging {
		t.Fatal("expected guild moderation_logging=false to override global true")
	}

	disabled := false
	legacyCfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			ModerationLogging: &disabled,
		},
		Guilds: []GuildConfig{{GuildID: "g2"}},
	}
	legacyRC := legacyCfg.ResolveRuntimeConfig("g2")
	if legacyRC.ModerationLogging == nil || *legacyRC.ModerationLogging {
		t.Fatal("expected global moderation_logging=false to resolve as disabled")
	}
}

func TestResolveRuntimeConfigGlobalMaxWorkersMerge(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			GlobalMaxWorkers: 8,
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					GlobalMaxWorkers: 3,
				},
			},
		},
	}

	if got := cfg.ResolveRuntimeConfig("g1").GlobalMaxWorkers; got != 3 {
		t.Fatalf("expected guild override to win for global_max_workers, got %d", got)
	}
	if got := cfg.ResolveRuntimeConfig("g2").GlobalMaxWorkers; got != 8 {
		t.Fatalf("expected global fallback for unknown guild, got %d", got)
	}
}

func TestRuntimeConfigNormalizedWebhookEmbedUpdates(t *testing.T) {
	t.Parallel()

	list := RuntimeConfig{
		WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
			{},
			{
				MessageID:  "m2",
				WebhookURL: "https://discord.com/api/webhooks/2/token",
				Embed:      json.RawMessage(`{"title":"list"}`),
			},
		},
	}
	listUpdates := list.NormalizedWebhookEmbedUpdates()
	if len(listUpdates) != 1 || listUpdates[0].MessageID != "m2" {
		t.Fatalf("expected normalized list to drop empty placeholders, got %+v", listUpdates)
	}
}

func TestRuntimeConfigUnmarshalMigratesLegacyWebhookEmbedUpdate(t *testing.T) {
	t.Parallel()

	// Sole legacy single-entry key — migrates into the canonical list.
	var legacyOnly RuntimeConfig
	legacyJSON := `{"webhook_embed_update":{"message_id":"m1","webhook_url":"https://discord.com/api/webhooks/1/token","embed":{"title":"legacy"}}}`
	if err := json.Unmarshal([]byte(legacyJSON), &legacyOnly); err != nil {
		t.Fatalf("unmarshal legacy-only: %v", err)
	}
	if len(legacyOnly.WebhookEmbedUpdates) != 1 || legacyOnly.WebhookEmbedUpdates[0].MessageID != "m1" {
		t.Fatalf("expected legacy single key migrated into canonical list, got %+v", legacyOnly.WebhookEmbedUpdates)
	}

	// Canonical list with a non-empty entry shadows the legacy key.
	var both RuntimeConfig
	bothJSON := `{"webhook_embed_updates":[{"message_id":"m2","webhook_url":"https://discord.com/api/webhooks/2/token","embed":{"title":"list"}}],"webhook_embed_update":{"message_id":"legacy-ignored","webhook_url":"https://discord.com/api/webhooks/9/token","embed":{"title":"legacy"}}}`
	if err := json.Unmarshal([]byte(bothJSON), &both); err != nil {
		t.Fatalf("unmarshal both: %v", err)
	}
	if len(both.WebhookEmbedUpdates) != 1 || both.WebhookEmbedUpdates[0].MessageID != "m2" {
		t.Fatalf("expected canonical list to shadow legacy key, got %+v", both.WebhookEmbedUpdates)
	}
}

func TestResolveRuntimeConfigWebhookEmbedUpdatesMerge(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
				{
					MessageID:  "global",
					WebhookURL: "https://discord.com/api/webhooks/1/token",
					Embed:      json.RawMessage(`{"title":"global"}`),
				},
			},
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
						{
							MessageID:  "guild",
							WebhookURL: "https://discord.com/api/webhooks/2/token",
							Embed:      json.RawMessage(`{"title":"guild"}`),
						},
					},
				},
			},
		},
	}

	resolved := cfg.ResolveRuntimeConfig("g1")
	updates := resolved.NormalizedWebhookEmbedUpdates()
	if len(updates) != 1 || updates[0].MessageID != "guild" {
		t.Fatalf("expected guild list to override global list, got %+v", updates)
	}
}

func TestRuntimeConfigWebhookEmbedValidationDefaultsAndNormalization(t *testing.T) {
	t.Parallel()

	effective := (RuntimeConfig{}).EffectiveWebhookEmbedValidation()
	if effective.Mode != WebhookEmbedValidationModeOff {
		t.Fatalf("expected default mode=off, got %q", effective.Mode)
	}
	if effective.TimeoutMS != DefaultWebhookEmbedValidationTimeoutMS {
		t.Fatalf("expected default timeout=%d, got %d", DefaultWebhookEmbedValidationTimeoutMS, effective.TimeoutMS)
	}

	invalid := RuntimeConfig{
		WebhookEmbedValidation: WebhookEmbedValidationConfig{
			Mode:      "unknown",
			TimeoutMS: -10,
		},
	}.EffectiveWebhookEmbedValidation()
	if invalid.Mode != WebhookEmbedValidationModeOff {
		t.Fatalf("expected invalid mode to normalize to off, got %q", invalid.Mode)
	}
	if invalid.TimeoutMS != DefaultWebhookEmbedValidationTimeoutMS {
		t.Fatalf("expected invalid timeout to normalize to default, got %d", invalid.TimeoutMS)
	}

	strict := RuntimeConfig{
		WebhookEmbedValidation: WebhookEmbedValidationConfig{
			Mode:      "STRICT",
			TimeoutMS: 4500,
		},
	}.EffectiveWebhookEmbedValidation()
	if strict.Mode != WebhookEmbedValidationModeStrict {
		t.Fatalf("expected mode strict, got %q", strict.Mode)
	}
	if strict.TimeoutMS != 4500 {
		t.Fatalf("expected timeout 4500, got %d", strict.TimeoutMS)
	}
}

func TestResolveRuntimeConfigWebhookEmbedValidationMerge(t *testing.T) {
	t.Parallel()

	cfg := &BotConfig{
		RuntimeConfig: RuntimeConfig{
			WebhookEmbedValidation: WebhookEmbedValidationConfig{
				Mode:      WebhookEmbedValidationModeSoft,
				TimeoutMS: 3000,
			},
		},
		Guilds: []GuildConfig{
			{
				GuildID: "g1",
				RuntimeConfig: RuntimeConfig{
					WebhookEmbedValidation: WebhookEmbedValidationConfig{
						Mode: WebhookEmbedValidationModeStrict,
					},
				},
			},
			{
				GuildID: "g2",
				RuntimeConfig: RuntimeConfig{
					WebhookEmbedValidation: WebhookEmbedValidationConfig{
						TimeoutMS: 9000,
					},
				},
			},
		},
	}

	g1 := cfg.ResolveRuntimeConfig("g1").EffectiveWebhookEmbedValidation()
	if g1.Mode != WebhookEmbedValidationModeStrict {
		t.Fatalf("expected g1 mode strict, got %q", g1.Mode)
	}
	if g1.TimeoutMS != 3000 {
		t.Fatalf("expected g1 timeout fallback to global 3000, got %d", g1.TimeoutMS)
	}

	g2 := cfg.ResolveRuntimeConfig("g2").EffectiveWebhookEmbedValidation()
	if g2.Mode != WebhookEmbedValidationModeSoft {
		t.Fatalf("expected g2 mode fallback to global soft, got %q", g2.Mode)
	}
	if g2.TimeoutMS != 9000 {
		t.Fatalf("expected g2 timeout override 9000, got %d", g2.TimeoutMS)
	}
}

func TestNormalizeRuntimeConfigRejectsNegativeGlobalMaxWorkers(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeRuntimeConfig(RuntimeConfig{GlobalMaxWorkers: -1}); err == nil {
		t.Fatal("expected negative global_max_workers to be rejected")
	}
}

// TestResolveRuntimeConfigAdoptsEveryGuildField guards against the silent-merge-gap
// failure mode of ResolveRuntimeConfig. That merge is ~30 hand-written
// "if guildRC.X != zero { resolved.X = guildRC.X }" blocks, so adding a field to
// RuntimeConfig without its matching merge line compiles cleanly and breaks no other
// test — the per-guild override is simply dropped at runtime. The merge stays explicit
// on purpose ("clear is better than clever"); this test, not reflection in production
// code, is the guard.
//
// Default rule: a non-zero guild sentinel must survive into the resolved config. Fields
// whose semantics differ are listed in exceptions and covered by dedicated assertions.
//
// Adding a new RuntimeConfig field? Either (a) merge it in ResolveRuntimeConfig so it
// satisfies the default adoption rule, or (b) add it to exceptions with its own
// assertion here. A field that is neither merged nor classified fails this test.
func TestResolveRuntimeConfigAdoptsEveryGuildField(t *testing.T) {
	t.Parallel()

	// Fields not covered by the default "non-zero guild sentinel is adopted" rule;
	// each has a dedicated assertion below.
	exceptions := map[string]string{
		"ModerationLogging":    "*bool with normalization (global defaults to non-nil)",
		"BackfillInitialDate":  "GuildOnly: adopts the guild value even when zero, no global fallback",
		"WebhookEmbedUpdates":  "slice merged via NormalizedWebhookEmbedUpdates (empty entries filtered)",
		"PastebinDevKey":       "global-only credential, intentionally not per-guild overridable",
		"PastebinUserName":     "global-only credential, intentionally not per-guild overridable",
		"PastebinUserPassword": "global-only credential, intentionally not per-guild overridable",
	}

	recurse := map[reflect.Type]bool{
		reflect.TypeOf(DatabaseRuntimeConfig{}):        true,
		reflect.TypeOf(WebhookEmbedValidationConfig{}): true,
	}
	leaves := runtimeConfigLeaves(reflect.TypeOf(RuntimeConfig{}), "", nil, recurse)

	// A stale exception (e.g. a renamed field) would let a new field slip past the
	// default loop unguarded, so fail if an exception no longer matches a real field.
	leafNames := make(map[string]bool, len(leaves))
	for _, lf := range leaves {
		leafNames[lf.name] = true
	}
	for name := range exceptions {
		if !leafNames[name] {
			t.Errorf("exceptions lists %q, which is not a RuntimeConfig field; remove or rename the stale entry", name)
		}
	}

	const testGuildID = "guild-under-test"

	for _, lf := range leaves {
		if _, skip := exceptions[lf.name]; skip {
			continue
		}
		t.Run(lf.name, func(t *testing.T) {
			var guildRC RuntimeConfig
			lf.setSentinel(t, reflect.ValueOf(&guildRC).Elem().FieldByIndex(lf.index))

			cfg := &BotConfig{
				Guilds: []GuildConfig{{GuildID: testGuildID, RuntimeConfig: guildRC}},
			}
			resolved := cfg.ResolveRuntimeConfig(testGuildID)
			lf.assertAdopted(t, reflect.ValueOf(resolved).FieldByIndex(lf.index))
		})
	}

	// Dedicated assertions for the exception fields.

	t.Run("ModerationLogging", func(t *testing.T) {
		cfg := &BotConfig{
			RuntimeConfig: RuntimeConfig{ModerationLogging: boolPtr(true)},
			Guilds: []GuildConfig{{
				GuildID:       testGuildID,
				RuntimeConfig: RuntimeConfig{ModerationLogging: boolPtr(false)},
			}},
		}
		resolved := cfg.ResolveRuntimeConfig(testGuildID)
		if resolved.ModerationLogging == nil || *resolved.ModerationLogging {
			t.Fatalf("expected guild moderation_logging=false to override global true, got %v", resolved.ModerationLogging)
		}
	})

	t.Run("BackfillInitialDate", func(t *testing.T) {
		adopted := &BotConfig{
			RuntimeConfig: RuntimeConfig{BackfillInitialDate: "2020-01-01"},
			Guilds: []GuildConfig{{
				GuildID:       testGuildID,
				RuntimeConfig: RuntimeConfig{BackfillInitialDate: "2024-12-31"},
			}},
		}
		if got := adopted.ResolveRuntimeConfig(testGuildID).BackfillInitialDate; got != "2024-12-31" {
			t.Fatalf("expected guild backfill_initial_date to be adopted, got %q", got)
		}
		// GuildOnly: an empty guild value clears the global instead of falling back.
		cleared := &BotConfig{
			RuntimeConfig: RuntimeConfig{BackfillInitialDate: "2020-01-01"},
			Guilds:        []GuildConfig{{GuildID: testGuildID}},
		}
		if got := cleared.ResolveRuntimeConfig(testGuildID).BackfillInitialDate; got != "" {
			t.Fatalf("expected GuildOnly backfill_initial_date to ignore the global fallback, got %q", got)
		}
	})

	t.Run("WebhookEmbedUpdates", func(t *testing.T) {
		cfg := &BotConfig{
			RuntimeConfig: RuntimeConfig{WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
				{MessageID: "global", WebhookURL: "https://discord.com/api/webhooks/1/token"},
			}},
			Guilds: []GuildConfig{{
				GuildID: testGuildID,
				RuntimeConfig: RuntimeConfig{WebhookEmbedUpdates: []WebhookEmbedUpdateConfig{
					{MessageID: "guild", WebhookURL: "https://discord.com/api/webhooks/2/token"},
				}},
			}},
		}
		updates := cfg.ResolveRuntimeConfig(testGuildID).NormalizedWebhookEmbedUpdates()
		if len(updates) != 1 || updates[0].MessageID != "guild" {
			t.Fatalf("expected guild webhook_embed_updates to override global, got %+v", updates)
		}
	})

	t.Run("PastebinGlobalOnly", func(t *testing.T) {
		cfg := &BotConfig{
			RuntimeConfig: RuntimeConfig{
				PastebinDevKey:       EncryptedString("global-dev-key"),
				PastebinUserName:     EncryptedString("global-user"),
				PastebinUserPassword: EncryptedString("global-pass"),
			},
			Guilds: []GuildConfig{{
				GuildID: testGuildID,
				RuntimeConfig: RuntimeConfig{
					PastebinDevKey:       EncryptedString("guild-dev-key"),
					PastebinUserName:     EncryptedString("guild-user"),
					PastebinUserPassword: EncryptedString("guild-pass"),
				},
			}},
		}
		resolved := cfg.ResolveRuntimeConfig(testGuildID)
		if resolved.PastebinDevKey != "global-dev-key" ||
			resolved.PastebinUserName != "global-user" ||
			resolved.PastebinUserPassword != "global-pass" {
			t.Fatalf("expected pastebin credentials to remain global-only, got dev=%q user=%q pass=%q",
				resolved.PastebinDevKey, resolved.PastebinUserName, resolved.PastebinUserPassword)
		}
	})
}

const (
	runtimeConfigSentinelString = "runtime-config-guard-sentinel"
	runtimeConfigSentinelInt    = int64(424242)
)

// runtimeConfigLeaf identifies one mergeable RuntimeConfig field by dotted name
// (for messages and exception lookup) and by reflect index path (for set/get).
type runtimeConfigLeaf struct {
	name  string
	index []int
}

// runtimeConfigLeaves flattens typ into its mergeable leaf fields, descending only
// into the nested struct types in recurse (Database, WebhookEmbedValidation) so the
// guard checks each concrete merged field rather than a parent struct.
func runtimeConfigLeaves(typ reflect.Type, prefix string, base []int, recurse map[reflect.Type]bool) []runtimeConfigLeaf {
	var leaves []runtimeConfigLeaf
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		index := append(append([]int(nil), base...), i)
		name := prefix + field.Name
		if recurse[field.Type] {
			leaves = append(leaves, runtimeConfigLeaves(field.Type, name+".", index, recurse)...)
			continue
		}
		leaves = append(leaves, runtimeConfigLeaf{name: name, index: index})
	}
	return leaves
}

// setSentinel writes a non-zero, type-appropriate sentinel into the guild-side field.
func (lf runtimeConfigLeaf) setSentinel(t *testing.T, field reflect.Value) {
	t.Helper()
	switch field.Kind() {
	case reflect.String:
		field.SetString(runtimeConfigSentinelString)
	case reflect.Bool:
		field.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		field.SetInt(runtimeConfigSentinelInt)
	default:
		t.Fatalf("RuntimeConfig field %s has kind %s with no sentinel rule; merge it in ResolveRuntimeConfig and rely on the default adoption rule, or classify it in the exceptions map", lf.name, field.Kind())
	}
}

// assertAdopted fails when the resolved field did not take the guild sentinel, which
// is the signature of a missing merge line in ResolveRuntimeConfig.
func (lf runtimeConfigLeaf) assertAdopted(t *testing.T, got reflect.Value) {
	t.Helper()
	switch got.Kind() {
	case reflect.String:
		if got.String() != runtimeConfigSentinelString {
			t.Fatalf("ResolveRuntimeConfig dropped the guild override for %s: got %q, want %q; add the merge line or classify the field in the exceptions map", lf.name, got.String(), runtimeConfigSentinelString)
		}
	case reflect.Bool:
		if !got.Bool() {
			t.Fatalf("ResolveRuntimeConfig dropped the guild override for %s: got false, want true; add the merge line or classify the field in the exceptions map", lf.name)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if got.Int() != runtimeConfigSentinelInt {
			t.Fatalf("ResolveRuntimeConfig dropped the guild override for %s: got %d, want %d; add the merge line or classify the field in the exceptions map", lf.name, got.Int(), runtimeConfigSentinelInt)
		}
	default:
		t.Fatalf("RuntimeConfig field %s has kind %s with no sentinel rule", lf.name, got.Kind())
	}
}

```

// === FILE: pkg/files/runtime_webhook_embed_updates.go ===
```go
package files

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

var (
	// ErrWebhookEmbedUpdateNotFound indicates no entry matched the provided message_id.
	ErrWebhookEmbedUpdateNotFound = errors.New("webhook embed update not found")
	// ErrWebhookEmbedUpdateAlreadyExists indicates message_id already exists in the target scope.
	ErrWebhookEmbedUpdateAlreadyExists = errors.New("webhook embed update already exists")
)

// ListWebhookEmbedUpdates returns webhook embed update entries for the given scope.
// Scope behavior:
// - guildID empty or "global": bot-level runtime_config
// - guildID set: guild-level runtime_config for that guild ID
func (mgr *ConfigManager) ListWebhookEmbedUpdates(guildID string) ([]WebhookEmbedUpdateConfig, error) {
	scope := normalizeWebhookScope(guildID)

	cfg := mgr.Config()
	if cfg == nil {
		return nil, nil
	}
	rc, err := runtimeConfigForScope(cfg, scope)
	if err != nil {
		return nil, fmt.Errorf("ConfigManager.ListWebhookEmbedUpdates: %w", err)
	}
	if rc == nil {
		return nil, nil
	}
	return cloneWebhookEmbedUpdateList(rc.NormalizedWebhookEmbedUpdates()), nil
}

// GetWebhookEmbedUpdate fetches one entry by message_id from the target scope.
func (mgr *ConfigManager) GetWebhookEmbedUpdate(guildID, messageID string) (_ WebhookEmbedUpdateConfig, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("get webhook embed update: %w", err)
		}
	}()
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("message_id is required")
	}

	scope := normalizeWebhookScope(guildID)

	cfg := mgr.Config()
	if cfg == nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
	}
	rc, err := runtimeConfigForScope(cfg, scope)
	if err != nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("ConfigManager.GetWebhookEmbedUpdate: %w", err)
	}
	if rc == nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
	}

	updates := rc.NormalizedWebhookEmbedUpdates()
	idx := findWebhookEmbedUpdateIndexByMessageID(updates, targetID)
	if idx < 0 {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
	}
	return cloneWebhookEmbedUpdateConfig(updates[idx]), nil
}

// CreateWebhookEmbedUpdate appends a new entry to the target scope.
func (mgr *ConfigManager) CreateWebhookEmbedUpdate(guildID string, update WebhookEmbedUpdateConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("create webhook embed update: %w", err)
		}
	}()
	scope := normalizeWebhookScope(guildID)

	normalized, err := normalizeWebhookEmbedUpdateConfig(update)
	if err != nil {
		return fmt.Errorf("ConfigManager.CreateWebhookEmbedUpdate: %w", err)
	}
	return mgr.updateRuntimeConfigScope(scope, func(rc *RuntimeConfig) error {
		updates := rc.NormalizedWebhookEmbedUpdates()
		if findWebhookEmbedUpdateIndexByMessageID(updates, normalized.MessageID) >= 0 {
			return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateAlreadyExists, normalized.MessageID)
		}

		updates = append(updates, normalized)
		setWebhookEmbedUpdatesCanonical(rc, updates)
		return nil
	})
}

// UpdateWebhookEmbedUpdate replaces an existing entry selected by message_id.
func (mgr *ConfigManager) UpdateWebhookEmbedUpdate(guildID, messageID string, update WebhookEmbedUpdateConfig) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("update webhook embed update: %w", err)
		}
	}()
	scope := normalizeWebhookScope(guildID)
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return fmt.Errorf("message_id is required")
	}

	normalized, err := normalizeWebhookEmbedUpdateConfig(update)
	if err != nil {
		return fmt.Errorf("ConfigManager.UpdateWebhookEmbedUpdate: %w", err)
	}
	return mgr.updateRuntimeConfigScope(scope, func(rc *RuntimeConfig) error {
		updates := rc.NormalizedWebhookEmbedUpdates()
		idx := findWebhookEmbedUpdateIndexByMessageID(updates, targetID)
		if idx < 0 {
			return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
		}

		if normalized.MessageID != targetID {
			dupIdx := findWebhookEmbedUpdateIndexByMessageID(updates, normalized.MessageID)
			if dupIdx >= 0 && dupIdx != idx {
				return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateAlreadyExists, normalized.MessageID)
			}
		}

		updates[idx] = normalized
		setWebhookEmbedUpdatesCanonical(rc, updates)
		return nil
	})
}

// DeleteWebhookEmbedUpdate removes an entry from the target scope.
func (mgr *ConfigManager) DeleteWebhookEmbedUpdate(guildID, messageID string) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("delete webhook embed update: %w", err)
		}
	}()
	scope := normalizeWebhookScope(guildID)
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return fmt.Errorf("message_id is required")
	}

	return mgr.updateRuntimeConfigScope(scope, func(rc *RuntimeConfig) error {
		updates := rc.NormalizedWebhookEmbedUpdates()
		idx := findWebhookEmbedUpdateIndexByMessageID(updates, targetID)
		if idx < 0 {
			return fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateNotFound, targetID)
		}

		updates = slices.Delete(updates, idx, idx+1)
		setWebhookEmbedUpdatesCanonical(rc, updates)
		return nil
	})
}

func normalizeWebhookScope(guildID string) string {
	scope := strings.TrimSpace(guildID)
	if strings.EqualFold(scope, "global") {
		return ""
	}
	return scope
}

func normalizeWebhookEmbedUpdateConfig(in WebhookEmbedUpdateConfig) (WebhookEmbedUpdateConfig, error) {
	out := WebhookEmbedUpdateConfig{
		MessageID:  strings.TrimSpace(in.MessageID),
		WebhookURL: strings.TrimSpace(in.WebhookURL),
	}

	if out.MessageID == "" {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("message_id is required")
	}
	if out.WebhookURL == "" {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("webhook_url is required")
	}
	if err := validateDiscordWebhookURL(out.WebhookURL); err != nil {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("webhook_url is invalid: %w", err)
	}

	raw := bytes.TrimSpace(in.Embed)
	if len(raw) == 0 {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("embed payload is required")
	}
	if !json.Valid(raw) {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("embed payload must be valid JSON")
	}
	if raw[0] != '{' && raw[0] != '[' {
		return WebhookEmbedUpdateConfig{}, fmt.Errorf("embed payload must be a JSON object or array")
	}

	out.Embed = append(json.RawMessage(nil), raw...)
	return out, nil
}

func setWebhookEmbedUpdatesCanonical(rc *RuntimeConfig, updates []WebhookEmbedUpdateConfig) {
	if rc == nil {
		return
	}
	rc.WebhookEmbedUpdates = cloneWebhookEmbedUpdateList(updates)
}

func cloneWebhookEmbedUpdateConfig(in WebhookEmbedUpdateConfig) WebhookEmbedUpdateConfig {
	out := WebhookEmbedUpdateConfig{
		MessageID:  strings.TrimSpace(in.MessageID),
		WebhookURL: strings.TrimSpace(in.WebhookURL),
	}
	if len(in.Embed) > 0 {
		out.Embed = append(json.RawMessage(nil), in.Embed...)
	}
	return out
}

func cloneWebhookEmbedUpdateList(in []WebhookEmbedUpdateConfig) []WebhookEmbedUpdateConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]WebhookEmbedUpdateConfig, 0, len(in))
	for _, item := range in {
		out = append(out, cloneWebhookEmbedUpdateConfig(item))
	}
	return out
}

func findWebhookEmbedUpdateIndexByMessageID(updates []WebhookEmbedUpdateConfig, messageID string) int {
	targetID := strings.TrimSpace(messageID)
	if targetID == "" {
		return -1
	}
	for i, item := range updates {
		if strings.TrimSpace(item.MessageID) == targetID {
			return i
		}
	}
	return -1
}

func validateDiscordWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be a valid URL")
	}

	if strings.TrimSpace(u.Scheme) == "" || strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("must include scheme and host")
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "webhooks" {
			continue
		}
		if i+2 >= len(parts) {
			return fmt.Errorf("must match /webhooks/{id}/{token}")
		}

		webhookID := strings.TrimSpace(parts[i+1])
		webhookToken := strings.TrimSpace(parts[i+2])
		if webhookID == "" || webhookToken == "" {
			return fmt.Errorf("must include non-empty webhook id and token")
		}
		if !isAllDigits(webhookID) {
			return fmt.Errorf("webhook id must be numeric")
		}
		return nil
	}

	return fmt.Errorf("must match /webhooks/{id}/{token}")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

```

// === FILE: pkg/files/runtime_webhook_embed_updates_test.go ===
```go
package files

import (
	"encoding/json"
	"errors"
	"testing"
)

func newWebhookUpdatesTestManager(t *testing.T, cfg *BotConfig) *ConfigManager {
	t.Helper()

	if cfg == nil {
		cfg = &BotConfig{Guilds: []GuildConfig{}}
	}
	if cfg.Guilds == nil {
		cfg.Guilds = []GuildConfig{}
	}

	mgr := NewConfigManagerWithStore(&mockConfigStore{}, nil)
	mgr.config = cfg
	return mgr
}

func TestWebhookEmbedUpdatesCRUDGlobal(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, nil)
	initial := WebhookEmbedUpdateConfig{
		MessageID:  "100",
		WebhookURL: "https://discord.com/api/webhooks/1/token",
		Embed:      json.RawMessage(`{"title":"initial"}`),
	}

	if err := mgr.CreateWebhookEmbedUpdate("", initial); err != nil {
		t.Fatalf("create global webhook update: %v", err)
	}

	list, err := mgr.ListWebhookEmbedUpdates("global")
	if err != nil {
		t.Fatalf("list global webhook updates: %v", err)
	}
	if len(list) != 1 || list[0].MessageID != "100" {
		t.Fatalf("unexpected list after create: %+v", list)
	}

	got, err := mgr.GetWebhookEmbedUpdate("", "100")
	if err != nil {
		t.Fatalf("get global webhook update: %v", err)
	}
	if got.WebhookURL != initial.WebhookURL {
		t.Fatalf("unexpected webhook_url: got %q want %q", got.WebhookURL, initial.WebhookURL)
	}

	update := WebhookEmbedUpdateConfig{
		MessageID:  "101",
		WebhookURL: "https://discord.com/api/webhooks/1/token-updated",
		Embed:      json.RawMessage(`{"title":"updated"}`),
	}
	if err := mgr.UpdateWebhookEmbedUpdate("", "100", update); err != nil {
		t.Fatalf("update global webhook update: %v", err)
	}

	got, err = mgr.GetWebhookEmbedUpdate("", "101")
	if err != nil {
		t.Fatalf("get updated webhook update: %v", err)
	}
	if got.WebhookURL != update.WebhookURL {
		t.Fatalf("unexpected updated webhook_url: got %q want %q", got.WebhookURL, update.WebhookURL)
	}

	if err := mgr.DeleteWebhookEmbedUpdate("", "101"); err != nil {
		t.Fatalf("delete global webhook update: %v", err)
	}

	list, err = mgr.ListWebhookEmbedUpdates("")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected empty list after delete, got %+v", list)
	}
}

func TestWebhookEmbedUpdatesCRUDGuildScope(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, &BotConfig{
		Guilds: []GuildConfig{
			{GuildID: "g1"},
		},
	})

	item := WebhookEmbedUpdateConfig{
		MessageID:  "200",
		WebhookURL: "https://discord.com/api/webhooks/2/token",
		Embed:      json.RawMessage(`{"title":"guild"}`),
	}
	if err := mgr.CreateWebhookEmbedUpdate("g1", item); err != nil {
		t.Fatalf("create guild webhook update: %v", err)
	}

	guildList, err := mgr.ListWebhookEmbedUpdates("g1")
	if err != nil {
		t.Fatalf("list guild webhook updates: %v", err)
	}
	if len(guildList) != 1 || guildList[0].MessageID != "200" {
		t.Fatalf("unexpected guild list: %+v", guildList)
	}

	globalList, err := mgr.ListWebhookEmbedUpdates("")
	if err != nil {
		t.Fatalf("list global webhook updates: %v", err)
	}
	if len(globalList) != 0 {
		t.Fatalf("expected no global updates, got %+v", globalList)
	}

	if err := mgr.CreateWebhookEmbedUpdate("g-missing", item); err == nil {
		t.Fatal("expected error for missing guild scope")
	}
}

func TestWebhookEmbedUpdatesCreateValidationAndDuplicates(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, nil)

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{}); err == nil {
		t.Fatal("expected validation error for empty payload")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "not-a-url",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}); err == nil {
		t.Fatal("expected validation error for invalid webhook_url format")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/webhooks/3",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}); err == nil {
		t.Fatal("expected validation error for webhook_url without token")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/webhooks/abc/token",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}); err == nil {
		t.Fatal("expected validation error for non-numeric webhook id")
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/v10/webhooks/3/token",
		Embed:      json.RawMessage(`"not-object"`),
	}); err == nil {
		t.Fatal("expected validation error for non-object/non-array embed payload")
	}

	valid := WebhookEmbedUpdateConfig{
		MessageID:  "300",
		WebhookURL: "https://discord.com/api/v10/webhooks/3/token",
		Embed:      json.RawMessage(`{"title":"ok"}`),
	}
	if err := mgr.CreateWebhookEmbedUpdate("", valid); err != nil {
		t.Fatalf("create valid webhook update: %v", err)
	}
	if err := mgr.CreateWebhookEmbedUpdate("", valid); !errors.Is(err, ErrWebhookEmbedUpdateAlreadyExists) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

func TestWebhookEmbedUpdatesLegacyFallbackMigration(t *testing.T) {
	t.Parallel()

	var bot BotConfig
	legacyJSON := `{"guilds":[],"runtime_config":{"webhook_embed_update":{"message_id":"legacy","webhook_url":"https://discord.com/api/webhooks/9/token","embed":{"title":"legacy"}}}}`
	if err := json.Unmarshal([]byte(legacyJSON), &bot); err != nil {
		t.Fatalf("unmarshal legacy bot config: %v", err)
	}
	if len(bot.RuntimeConfig.WebhookEmbedUpdates) != 1 || bot.RuntimeConfig.WebhookEmbedUpdates[0].MessageID != "legacy" {
		t.Fatalf("expected legacy single key migrated into canonical list at decode time, got %+v", bot.RuntimeConfig.WebhookEmbedUpdates)
	}

	mgr := newWebhookUpdatesTestManager(t, &bot)
	list, err := mgr.ListWebhookEmbedUpdates("")
	if err != nil {
		t.Fatalf("list after legacy migration: %v", err)
	}
	if len(list) != 1 || list[0].MessageID != "legacy" {
		t.Fatalf("expected legacy item to remain visible after migration, got %+v", list)
	}

	if err := mgr.CreateWebhookEmbedUpdate("", WebhookEmbedUpdateConfig{
		MessageID:  "new",
		WebhookURL: "https://discord.com/api/webhooks/10/token",
		Embed:      json.RawMessage(`{"title":"new"}`),
	}); err != nil {
		t.Fatalf("create new item alongside migrated legacy: %v", err)
	}

	cfg := mgr.Config()
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.RuntimeConfig.WebhookEmbedUpdates) != 2 {
		t.Fatalf("expected canonical list with 2 items, got %+v", cfg.RuntimeConfig.WebhookEmbedUpdates)
	}
}

func TestWebhookEmbedUpdatesUpdateDeleteNotFound(t *testing.T) {
	t.Parallel()

	mgr := newWebhookUpdatesTestManager(t, nil)
	update := WebhookEmbedUpdateConfig{
		MessageID:  "401",
		WebhookURL: "https://discord.com/api/webhooks/4/token",
		Embed:      json.RawMessage(`{"title":"u"}`),
	}

	if err := mgr.UpdateWebhookEmbedUpdate("", "missing", update); !errors.Is(err, ErrWebhookEmbedUpdateNotFound) {
		t.Fatalf("expected not found on update, got %v", err)
	}
	if err := mgr.DeleteWebhookEmbedUpdate("", "missing"); !errors.Is(err, ErrWebhookEmbedUpdateNotFound) {
		t.Fatalf("expected not found on delete, got %v", err)
	}
}

```

// === FILE: pkg/files/settings_normalization.go ===
```go
package files

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
)

// NormalizeRuntimeConfig canonicalizes runtime config sections used by the
// control dashboard before they are persisted as part of broader config writes.
func NormalizeRuntimeConfig(in RuntimeConfig) (RuntimeConfig, error) {
	out := cloneRuntimeConfig(in)

	if normalizedDB, ok, err := normalizeRuntimeDatabaseConfig(out.Database); err != nil {
		return RuntimeConfig{}, fmt.Errorf("database: %w", err)
	} else if ok {
		out.Database = normalizedDB
	}
	if out.GlobalMaxWorkers < 0 {
		return RuntimeConfig{}, fmt.Errorf("global_max_workers must be >= 0")
	}

	if updates := out.NormalizedWebhookEmbedUpdates(); len(updates) > 0 {
		normalized := make([]WebhookEmbedUpdateConfig, 0, len(updates))
		seen := make(map[string]struct{}, len(updates))
		for idx, item := range updates {
			next, err := normalizeWebhookEmbedUpdateConfig(item)
			if err != nil {
				return RuntimeConfig{}, fmt.Errorf("webhook_embed_updates[%d]: %w", idx, err)
			}
			if _, exists := seen[next.MessageID]; exists {
				return RuntimeConfig{}, fmt.Errorf("%w: message_id=%s", ErrWebhookEmbedUpdateAlreadyExists, next.MessageID)
			}
			seen[next.MessageID] = struct{}{}
			normalized = append(normalized, next)
		}
		out.WebhookEmbedUpdates = normalized
	} else {
		out.WebhookEmbedUpdates = nil
	}

	out.WebhookEmbedValidation = out.WebhookEmbedValidation.Normalized()

	return out, nil
}

// NormalizePartnerBoardConfig canonicalizes the partner board config so broad
// config writes share the same validation rules as the dedicated board service.
func NormalizePartnerBoardConfig(in PartnerBoardConfig) (PartnerBoardConfig, error) {
	partners, err := canonicalizePartnerEntries(in.Partners)
	if err != nil {
		return PartnerBoardConfig{}, fmt.Errorf("partners: %w", err)
	}

	return PartnerBoardConfig{
		Postings: cloneCustomEmbedPostings(in.Postings),
		Template: normalizePartnerBoardTemplate(in.Template),
		Partners: clonePartnerEntries(partners),
	}, nil
}

// NormalizeQOTDConfig canonicalizes guild QOTD settings so dedicated routes and
// broad config writes can share the same validation behavior.
func NormalizeQOTDConfig(in QOTDConfig) (QOTDConfig, error) {
	verifiedRoleID := strings.TrimSpace(in.VerifiedRoleID)
	activeDeckID := strings.TrimSpace(in.ActiveDeckID)
	decks := cloneQOTDDeckConfigs(in.Decks)
	suppressedPublishDatesUTC, err := normalizeQOTDSuppressedPublishDates(in.SuppressScheduledPublishDatesUTC)
	if err != nil {
		return QOTDConfig{}, invalidQOTDInput("suppress_scheduled_publish_dates_utc: %v", err)
	}
	schedule, err := normalizeQOTDPublishScheduleConfig(in.Schedule)
	if err != nil {
		return QOTDConfig{}, invalidQOTDInput("schedule: %v", err)
	}
	if verifiedRoleID != "" && !isAllDigits(verifiedRoleID) {
		return QOTDConfig{}, invalidQOTDInput("verified_role_id must be numeric")
	}

	if len(decks) == 0 {
		// suppressedPublishDateUTC must keep the config non-zero on the
		// no-deck path: a suppression-only config still carries meaningful
		// state (the scheduler reads it back to skip the suppressed slot).
		// QOTDConfig.IsZero handles the symmetric case on the read side.
		if verifiedRoleID == "" && schedule.IsZero() && len(suppressedPublishDatesUTC) == 0 {
			return QOTDConfig{}, nil
		}
		return QOTDConfig{
			VerifiedRoleID:                   verifiedRoleID,
			Schedule:                         schedule,
			SuppressScheduledPublishDatesUTC: suppressedPublishDatesUTC,
		}, nil
	}

	normalizedDecks := make([]QOTDDeckConfig, 0, len(decks))
	seenIDs := make(map[string]struct{}, len(decks))
	seenNames := make(map[string]struct{}, len(decks))
	for idx, deck := range decks {
		normalized, err := normalizeQOTDDeckConfig(deck)
		if err != nil {
			return QOTDConfig{}, invalidQOTDInput("decks[%d]: %v", idx, err)
		}
		if _, exists := seenIDs[normalized.ID]; exists {
			return QOTDConfig{}, invalidQOTDInput("deck ids must be unique")
		}
		seenIDs[normalized.ID] = struct{}{}
		nameKey := strings.ToLower(normalized.Name)
		if _, exists := seenNames[nameKey]; exists {
			return QOTDConfig{}, invalidQOTDInput("deck names must be unique")
		}
		seenNames[nameKey] = struct{}{}
		normalizedDecks = append(normalizedDecks, normalized)
	}

	if activeDeckID == "" {
		activeDeckID = firstEnabledQOTDDeckID(normalizedDecks)
	}
	if activeDeckID == "" && len(normalizedDecks) > 0 {
		activeDeckID = normalizedDecks[0].ID
	}
	if activeDeckID != "" {
		if _, ok := seenIDs[activeDeckID]; !ok {
			return QOTDConfig{}, invalidQOTDInput("active_deck_id must refer to a configured deck")
		}
	}

	if firstEnabledQOTDDeckID(normalizedDecks) != "" && !schedule.IsComplete() {
		return QOTDConfig{}, invalidQOTDInput("schedule.hour_utc and schedule.minute_utc are required when enabled")
	}

	if len(normalizedDecks) == 1 &&
		isImplicitDefaultQOTDDeck(normalizedDecks[0], activeDeckID) &&
		verifiedRoleID == "" &&
		schedule.IsZero() &&
		len(suppressedPublishDatesUTC) == 0 {
		return QOTDConfig{}, nil
	}

	return QOTDConfig{
		VerifiedRoleID:                   verifiedRoleID,
		ActiveDeckID:                     activeDeckID,
		Decks:                            normalizedDecks,
		Schedule:                         schedule,
		SuppressScheduledPublishDatesUTC: suppressedPublishDatesUTC,
	}, nil
}

// normalizeQOTDSuppressedPublishDates validates each entry, dedupes (case
// insensitive whitespace), and returns the canonical sorted list. Empty
// entries are silently dropped; a malformed entry fails the whole config so
// callers learn about the typo at write time instead of at runtime when the
// scheduler tries to compare against a corrupt date.
func normalizeQOTDSuppressedPublishDates(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := time.Parse(qotdPublishDateLayout, raw)
		if err != nil {
			return nil, fmt.Errorf("must be UTC publish dates in YYYY-MM-DD format")
		}
		canonical := parsed.UTC().Format(qotdPublishDateLayout)
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	if len(out) == 0 {
		return nil, nil
	}
	sort.Strings(out)
	return out, nil
}

func normalizeQOTDPublishScheduleConfig(in QOTDPublishScheduleConfig) (QOTDPublishScheduleConfig, error) {
	out := QOTDPublishScheduleConfig{
		HourUTC:   cloneOptionalInt(in.HourUTC),
		MinuteUTC: cloneOptionalInt(in.MinuteUTC),
	}
	if out.HourUTC != nil {
		if *out.HourUTC < 0 || *out.HourUTC > 23 {
			return QOTDPublishScheduleConfig{}, fmt.Errorf("hour_utc must be between 0 and 23")
		}
	}
	if out.MinuteUTC != nil {
		if *out.MinuteUTC < 0 || *out.MinuteUTC > 59 {
			return QOTDPublishScheduleConfig{}, fmt.Errorf("minute_utc must be between 0 and 59")
		}
	}
	return out, nil
}

func normalizeQOTDDeckConfig(in QOTDDeckConfig) (QOTDDeckConfig, error) {
	out := QOTDDeckConfig{
		ID:                strings.TrimSpace(in.ID),
		Name:              strings.TrimSpace(in.Name),
		Enabled:           in.Enabled,
		ChannelID:         strings.TrimSpace(in.ChannelID),
		SelectionStrategy: normalizeQOTDSelectionStrategy(in.SelectionStrategy),
	}

	if out.ID == "" {
		out.ID = idgen.GenerateString()
	}
	if out.Name == "" {
		return QOTDDeckConfig{}, fmt.Errorf("name is required")
	}
	if out.ChannelID != "" && !isAllDigits(out.ChannelID) {
		return QOTDDeckConfig{}, fmt.Errorf("channel_id must be numeric")
	}
	if out.Enabled {
		if out.ChannelID == "" {
			return QOTDDeckConfig{}, fmt.Errorf("channel_id is required when enabled")
		}
	}
	return out, nil
}

// normalizeQOTDSelectionStrategy coerces persisted values into the supported
// vocabulary. Empty / unknown values fall back to "" so the consumer can
// apply its own default; only "random" is propagated as a non-default
// selection.
func normalizeQOTDSelectionStrategy(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(QOTDSelectionStrategyRandom):
		return string(QOTDSelectionStrategyRandom)
	case string(QOTDSelectionStrategyQueue):
		return string(QOTDSelectionStrategyQueue)
	default:
		return ""
	}
}

func firstEnabledQOTDDeckID(decks []QOTDDeckConfig) string {
	for _, deck := range decks {
		if deck.Enabled {
			return deck.ID
		}
	}
	return ""
}

func normalizeRuntimeDatabaseConfig(in DatabaseRuntimeConfig) (DatabaseRuntimeConfig, bool, error) {
	cfg := persistence.Config{
		Driver:              in.Driver,
		DatabaseURL:         in.DatabaseURL,
		MaxOpenConns:        in.MaxOpenConns,
		MaxIdleConns:        in.MaxIdleConns,
		ConnMaxLifetimeSecs: in.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: in.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       in.PingTimeoutMS,
	}

	if cfg == (persistence.Config{}) {
		return DatabaseRuntimeConfig{}, false, nil
	}

	normalized := cfg.Normalized()
	if err := normalized.Validate(); err != nil {
		return DatabaseRuntimeConfig{}, false, fmt.Errorf("normalizeRuntimeDatabaseConfig: %w", err)
	}

	return DatabaseRuntimeConfig{
		Driver:              normalized.Driver,
		DatabaseURL:         normalized.DatabaseURL,
		MaxOpenConns:        normalized.MaxOpenConns,
		MaxIdleConns:        normalized.MaxIdleConns,
		ConnMaxLifetimeSecs: normalized.ConnMaxLifetimeSecs,
		ConnMaxIdleTimeSecs: normalized.ConnMaxIdleTimeSecs,
		PingTimeoutMS:       normalized.PingTimeoutMS,
	}, true, nil
}

```

// === FILE: pkg/files/types.go ===
```go
package files

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RuntimeConfig centralizes operational toggles/parameters that were previously
// controlled via environment variables. These values are meant to be edited
// from Discord via an interactive embed and persisted in the active config store.
//
// Keep names in CAPS to mirror the previous env var names and make auditing easy.
type RuntimeConfig struct {
	Database DatabaseRuntimeConfig `json:"database,omitempty"`

	// THEME
	BotTheme string `json:"bot_theme,omitempty"`

	// SERVICES (LOGGING)
	DisableDBCleanup     bool `json:"disable_db_cleanup,omitempty"`
	DisableMessageLogs   bool `json:"disable_message_logs,omitempty"`
	DisableEntryExitLogs bool `json:"disable_entry_exit_logs,omitempty"`
	DisableReactionLogs  bool `json:"disable_reaction_logs,omitempty"`
	DisableUserLogs      bool `json:"disable_user_logs,omitempty"`
	DisableCleanLog      bool `json:"disable_clean_log,omitempty"`
	// MODERATION LOGS
	// true/nil: send moderation logs automatically
	// false: do not send moderation logs
	ModerationLogging  *bool  `json:"moderation_logging,omitempty"`
	LogModerationScope string `json:"log_moderation_scope,omitempty"` // discordcore, all_bots, all

	// PRESENCE WATCH
	PresenceWatchUserID string `json:"presence_watch_user_id,omitempty"`
	PresenceWatchBot    bool   `json:"presence_watch_bot,omitempty"`

	// MESSAGE CACHE
	MessageCacheTTLHours int  `json:"message_cache_ttl_hours,omitempty"`
	MessageDeleteOnLog   bool `json:"message_delete_on_log,omitempty"`
	MessageCacheCleanup  bool `json:"message_cache_cleanup,omitempty"`

	// TASK ROUTER
	// 0 means "use the runtime default budget".
	GlobalMaxWorkers int `json:"global_max_workers,omitempty"`

	// BACKFILL (ENTRY/EXIT)
	BackfillChannelID   string `json:"backfill_channel_id,omitempty"`
	BackfillStartDay    string `json:"backfill_start_day,omitempty"` // YYYY-MM-DD, default: today UTC when empty
	BackfillInitialDate string `json:"backfill_initial_date,omitempty"`
	MimuWelcomeString   string `json:"mimu_welcome_string,omitempty"`
	MimuGoodbyeString   string `json:"mimu_goodbye_string,omitempty"`

	// BOT ROLE PERMISSION MIRRORING (SAFETY)
	// Previously controllable via env vars:
	//   - ALICE_DISABLE_BOT_ROLE_PERM_MIRROR
	//   - ALICE_BOT_ROLE_PERM_MIRROR_ACTOR_ROLE_ID
	DisableBotRolePermMirror     bool   `json:"disable_bot_role_perm_mirror,omitempty"`
	BotRolePermMirrorActorRoleID string `json:"bot_role_perm_mirror_actor_role_id,omitempty"`

	// Webhook embed message patch (global or per-guild override).
	// Intended for editing an existing webhook message embed by ID.
	WebhookEmbedUpdates []WebhookEmbedUpdateConfig `json:"webhook_embed_updates,omitempty"`
	// Remote validation behavior for webhook embed targets used by CRUD commands.
	WebhookEmbedValidation WebhookEmbedValidationConfig `json:"webhook_embed_validation,omitempty"`

	// Toggle to disable ephemeral messages for interactive embeds per guild.
	DisableInteractiveEphemeral bool `json:"disable_interactive_ephemeral,omitempty"`

	// Global Pastebin Credentials (safely encrypted)
	PastebinDevKey       EncryptedString `json:"pastebin_dev_key,omitempty"`
	PastebinUserName     EncryptedString `json:"pastebin_user_name,omitempty"`
	PastebinUserPassword EncryptedString `json:"pastebin_user_password,omitempty"`
}

// UnmarshalJSON decodes a RuntimeConfig and absorbs legacy persisted keys into
// their canonical successors so older settings files continue to load:
//   - "moderation_log_mode" (off/non-off string) migrates into ModerationLogging
//     when ModerationLogging is unset
//   - "webhook_embed_update" (single-entry legacy form) is appended to
//     WebhookEmbedUpdates when no non-empty canonical entry shadows it
//
// The legacy keys never round-trip into the public type; the marshalled form
// only emits the canonical fields.
func (rc *RuntimeConfig) UnmarshalJSON(data []byte) error {
	type alias RuntimeConfig
	type rawRuntimeConfig struct {
		alias
		// Deprecated: migrated to ModerationLogging
		ModerationLogMode string `json:"moderation_log_mode,omitempty"`
		// Deprecated: migrated to WebhookEmbedUpdates
		WebhookEmbedUpdate WebhookEmbedUpdateConfig `json:"webhook_embed_update,omitempty"`
	}

	var raw rawRuntimeConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("RuntimeConfig.UnmarshalJSON: %w", err)
	}

	*rc = RuntimeConfig(raw.alias)

	if rc.ModerationLogging == nil && strings.TrimSpace(raw.ModerationLogMode) != "" {
		rc.ModerationLogging = boolPtr(strings.ToLower(strings.TrimSpace(raw.ModerationLogMode)) != "off")
	}

	if !raw.WebhookEmbedUpdate.IsZero() {
		hasCanonical := false
		for _, item := range rc.WebhookEmbedUpdates {
			if !item.IsZero() {
				hasCanonical = true
				break
			}
		}
		if !hasCanonical {
			rc.WebhookEmbedUpdates = append(rc.WebhookEmbedUpdates, raw.WebhookEmbedUpdate)
		}
	}

	return nil
}

// DatabaseRuntimeConfig defines runtime database configuration.
// The only supported driver is postgres.
type DatabaseRuntimeConfig struct {
	Driver              string `json:"driver,omitempty"`
	DatabaseURL         string `json:"database_url,omitempty"`
	MaxOpenConns        int    `json:"max_open_conns,omitempty"`
	MaxIdleConns        int    `json:"max_idle_conns,omitempty"`
	ConnMaxLifetimeSecs int    `json:"conn_max_lifetime_secs,omitempty"`
	ConnMaxIdleTimeSecs int    `json:"conn_max_idle_time_secs,omitempty"`
	PingTimeoutMS       int    `json:"ping_timeout_ms,omitempty"`
}

// WebhookEmbedUpdateConfig defines how to patch an existing webhook message embed.
type WebhookEmbedUpdateConfig struct {
	MessageID  string          `json:"message_id,omitempty"`
	WebhookURL string          `json:"webhook_url,omitempty"`
	Embed      json.RawMessage `json:"embed,omitempty"`
}

// WebhookEmbedValidationModeSoft defines webhook embed validation mode soft.
// WebhookEmbedValidationModeStrict defines webhook embed validation mode strict.
// DefaultWebhookEmbedValidationTimeoutMS defines default webhook embed validation timeout ms.
// WebhookEmbedValidationModeOff defines webhook embed validation mode off.
const (
	WebhookEmbedValidationModeOff    = "off"
	WebhookEmbedValidationModeSoft   = "soft"
	WebhookEmbedValidationModeStrict = "strict"

	DefaultWebhookEmbedValidationTimeoutMS = 3000
)

// WebhookEmbedValidationConfig controls remote validation behavior for webhook targets.
// mode:
// - off: no remote validation
// - soft: validate remotely, but persist even on failure
// - strict: validate remotely and fail before persisting when validation fails
type WebhookEmbedValidationConfig struct {
	Mode      string `json:"mode,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

// Normalized returns a canonical config with safe defaults.
func (c WebhookEmbedValidationConfig) Normalized() WebhookEmbedValidationConfig {
	mode := normalizeWebhookEmbedValidationMode(c.Mode)
	timeout := c.TimeoutMS
	if timeout <= 0 {
		timeout = DefaultWebhookEmbedValidationTimeoutMS
	}
	return WebhookEmbedValidationConfig{
		Mode:      mode,
		TimeoutMS: timeout,
	}
}

func normalizeWebhookEmbedValidationMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case WebhookEmbedValidationModeSoft:
		return WebhookEmbedValidationModeSoft
	case WebhookEmbedValidationModeStrict:
		return WebhookEmbedValidationModeStrict
	default:
		return WebhookEmbedValidationModeOff
	}
}

// IsZero reports whether all fields are unset.
func (c WebhookEmbedUpdateConfig) IsZero() bool {
	return strings.TrimSpace(c.MessageID) == "" &&
		strings.TrimSpace(c.WebhookURL) == "" &&
		len(bytes.TrimSpace(c.Embed)) == 0
}

// NormalizedWebhookEmbedUpdates returns the canonical webhook_embed_updates list
// with empty placeholder entries filtered out. The legacy single-entry
// "webhook_embed_update" key is migrated into this slice at JSON decode time by
// RuntimeConfig.UnmarshalJSON, so callers no longer need a fallback branch.
func (rc RuntimeConfig) NormalizedWebhookEmbedUpdates() []WebhookEmbedUpdateConfig {
	updates := make([]WebhookEmbedUpdateConfig, 0, len(rc.WebhookEmbedUpdates))
	for _, item := range rc.WebhookEmbedUpdates {
		if item.IsZero() {
			continue
		}
		updates = append(updates, item)
	}
	if len(updates) == 0 {
		return nil
	}
	return updates
}

// EffectiveWebhookEmbedValidation resolves webhook_embed_validation defaults.
func (rc RuntimeConfig) EffectiveWebhookEmbedValidation() WebhookEmbedValidationConfig {
	return rc.WebhookEmbedValidation.Normalized()
}

// ## Config Types

// ChannelsConfig groups channel IDs per guild.
type ChannelsConfig struct {
	Commands string `json:"commands,omitempty"`

	// Event/feature-scoped channels (canonical settings schema).
	AvatarLogging  string `json:"avatar_logging,omitempty"`
	RoleUpdate     string `json:"role_update,omitempty"`
	MemberJoin     string `json:"member_join,omitempty"`
	MemberLeave    string `json:"member_leave,omitempty"`
	MessageEdit    string `json:"message_edit,omitempty"`
	MessageDelete  string `json:"message_delete,omitempty"`
	AutomodAction  string `json:"automod_action,omitempty"`
	ModerationCase string `json:"moderation_case,omitempty"`
	CleanAction    string `json:"clean_action,omitempty"`
	EntryBackfill  string `json:"entry_backfill,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (cc *ChannelsConfig) UnmarshalJSON(data []byte) error {
	type alias ChannelsConfig
	type rawChannelsConfig struct {
		alias
		// Deprecated: removed in favor of native features
		VerificationCleanup string `json:"verification_cleanup,omitempty"`
	}

	var raw rawChannelsConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("ChannelsConfig.UnmarshalJSON: %w", err)
	}

	*cc = ChannelsConfig(raw.alias)
	return nil
}

// BackfillChannelID backfills channel id.
func (cc ChannelsConfig) BackfillChannelID() string {
	return strings.TrimSpace(cc.EntryBackfill)
}

// StatsChannelConfig defines a channel that should reflect a member count.
type StatsChannelConfig struct {
	ChannelID    string `json:"channel_id,omitempty"`
	Label        string `json:"label,omitempty"`
	NameTemplate string `json:"name_template,omitempty"` // Supports {label} and {count}
	MemberType   string `json:"member_type,omitempty"`   // all|humans|bots (default: all)
	RoleID       string `json:"role_id,omitempty"`       // Optional role filter
}

// StatsConfig groups the periodic stats channel updates for a guild.
type StatsConfig struct {
	Channels []StatsChannelConfig `json:"channels,omitempty"`
}

// AutoAssignmentConfig defines automatic role assignment rules.
type AutoAssignmentConfig struct {
	Enabled       bool     `json:"enabled,omitempty"`
	TargetRoleID  string   `json:"target_role,omitempty"`
	RequiredRoles []string `json:"required_roles,omitempty"`
}

// RolesConfig groups role-related settings per guild.
type RolesConfig struct {
	Allowed        []string             `json:"allowed,omitempty"`
	DashboardRead  []string             `json:"dashboard_read,omitempty"`
	DashboardWrite []string             `json:"dashboard_write,omitempty"`
	AutoAssignment AutoAssignmentConfig `json:"auto_assignment,omitempty"`
	BoosterRole    string               `json:"booster_role,omitempty"`
	MuteRole       string               `json:"mute_role,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (rc *RolesConfig) UnmarshalJSON(data []byte) error {
	type alias RolesConfig
	type rawRolesConfig struct {
		alias
		// Deprecated: removed in favor of native features
		VerificationRole string `json:"verification_role,omitempty"`
	}

	var raw rawRolesConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("RolesConfig.UnmarshalJSON: %w", err)
	}

	*rc = RolesConfig(raw.alias)
	return nil
}

// EmbedUpdateTargetTypeWebhookMessage defines embed update target type webhook message.
// EmbedUpdateTargetTypeChannelMessage defines embed update target type channel message.
const (
	EmbedUpdateTargetTypeWebhookMessage = "webhook_message"
	EmbedUpdateTargetTypeChannelMessage = "channel_message"
)

// EmbedUpdateTargetConfig defines the target used to update one existing message embed.
// Supported target types:
// - webhook_message: requires message_id + webhook_url
// - channel_message: requires message_id + channel_id
type EmbedUpdateTargetConfig struct {
	Type       string `json:"type,omitempty"`
	MessageID  string `json:"message_id,omitempty"`
	ChannelID  string `json:"channel_id,omitempty"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// IsZero reports whether all fields are empty.
func (c EmbedUpdateTargetConfig) IsZero() bool {
	return strings.TrimSpace(c.Type) == "" &&
		strings.TrimSpace(c.MessageID) == "" &&
		strings.TrimSpace(c.ChannelID) == "" &&
		strings.TrimSpace(c.WebhookURL) == ""
}

// PartnerEntryConfig defines one partner record for a board.
type PartnerEntryConfig struct {
	Fandom string `json:"fandom,omitempty"`
	Name   string `json:"name,omitempty"`
	Link   string `json:"link,omitempty"`
}

// PartnerBoardTemplateConfig controls board rendering behavior.
type PartnerBoardTemplateConfig struct {
	Title                      string `json:"title,omitempty"`
	ContinuationTitle          string `json:"continuation_title,omitempty"`
	Intro                      string `json:"intro,omitempty"`
	SectionHeaderTemplate      string `json:"section_header_template,omitempty"`
	SectionContinuationSuffix  string `json:"section_continuation_suffix,omitempty"`
	SectionContinuationPattern string `json:"section_continuation_template,omitempty"`
	LineTemplate               string `json:"line_template,omitempty"`
	EmptyStateText             string `json:"empty_state_text,omitempty"`
	FooterTemplate             string `json:"footer_template,omitempty"`
	OtherFandomLabel           string `json:"other_fandom_label,omitempty"`
	Color                      int    `json:"color,omitempty"`
	DisableFandomSorting       bool   `json:"disable_fandom_sorting,omitempty"`
	DisablePartnerSorting      bool   `json:"disable_partner_sorting,omitempty"`
}

// PartnerBoardConfig stores target, template, and partner records.
type PartnerBoardConfig struct {
	Template PartnerBoardTemplateConfig `json:"template,omitempty"`
	Partners []PartnerEntryConfig       `json:"partners,omitempty"`
	Postings []CustomEmbedPostingConfig `json:"postings,omitempty"`
}

// QOTDDeckConfig stores one named QOTD deck plus its target delivery channel.
type QOTDDeckConfig struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Enabled   bool   `json:"enabled,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
	// SelectionStrategy controls how the next ready question is picked at
	// automatic publish time: "queue" (default — head of the queue, the
	// historical behavior) or "random" (uniformly random eligible question).
	// The visible thread numbering ("Question #001"...) is independent of
	// this strategy because each post carries its own publish ordinal.
	SelectionStrategy string `json:"selection_strategy,omitempty"`
}

// QOTDPublishScheduleConfig stores the UTC publish boundary for one guild.
type QOTDPublishScheduleConfig struct {
	HourUTC   *int `json:"hour_utc,omitempty"`
	MinuteUTC *int `json:"minute_utc,omitempty"`
}

// QOTDConfig stores per-guild question-of-the-day deck settings.
type QOTDConfig struct {
	VerifiedRoleID string                    `json:"verified_role_id,omitempty"`
	ActiveDeckID   string                    `json:"active_deck_id,omitempty"`
	Decks          []QOTDDeckConfig          `json:"decks,omitempty"`
	Schedule       QOTDPublishScheduleConfig `json:"schedule,omitempty"`
	// SuppressScheduledPublishDatesUTC is the canonical set of UTC publish
	// dates (YYYY-MM-DD) for which the scheduler must skip its automatic
	// publish. Manual publishes that consume a slot, or maintenance flows
	// that pause one specific day, add entries here; the runtime trims
	// expired dates on each cycle. Replaces the legacy single-string field
	// "suppress_scheduled_publish_date_utc" — JSON unmarshal still accepts
	// the legacy form and migrates it into this slice so old persisted
	// configs continue to load.
	SuppressScheduledPublishDatesUTC []string `json:"suppress_scheduled_publish_dates_utc,omitempty"`
}

// UserPruneConfig controls periodic user pruning per guild.
type UserPruneConfig struct {
	// Enabled toggles the automatic monthly prune.
	// true: execute native Discord prune automatically on day 28 (30-day inactivity window)
	// false: do not execute automatically
	Enabled bool `json:"enabled,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (upc *UserPruneConfig) UnmarshalJSON(data []byte) error {
	type alias UserPruneConfig
	type rawUserPruneConfig struct {
		alias
		// Deprecated: removed in favor of native Discord prune (Enabled toggle)
		GraceDays int `json:"grace_days,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		ScanIntervalMins int `json:"scan_interval_mins,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		InitialDelaySecs int `json:"initial_delay_secs,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		KicksPerSecond int `json:"kps,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		MaxKicksPerRun int `json:"max_kicks_per_run,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		ExemptRoleIDs []string `json:"exempt_role_ids,omitempty"`
		// Deprecated: removed in favor of native Discord prune
		DryRun bool `json:"dry_run,omitempty"`
	}

	var raw rawUserPruneConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("UserPruneConfig.UnmarshalJSON: %w", err)
	}

	*upc = UserPruneConfig(raw.alias)
	return nil
}

// ReactionBlockEmojiConfig stores one blocked emoji selector.
//
// Kind is one of:
// - "custom": Value is the custom emoji ID, Name is the display name
// - "unicode": Value is the Unicode emoji, Alias is an optional :shortcode:
type ReactionBlockEmojiConfig struct {
	Kind     string `json:"kind,omitempty"`
	Value    string `json:"value,omitempty"`
	Name     string `json:"name,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Animated bool   `json:"animated,omitempty"`
}

// ReactionBlockRuleConfig stores the blocked emoji list for one reactor/target pair.
type ReactionBlockRuleConfig struct {
	ReactorUserID string                     `json:"reactor_user_id,omitempty"`
	TargetUserID  string                     `json:"target_user_id,omitempty"`
	Emojis        []ReactionBlockEmojiConfig `json:"emojis,omitempty"`
}

// ReactionBlockConfig stores per-guild emoji reaction restrictions.
type ReactionBlockConfig struct {
	Rules []ReactionBlockRuleConfig `json:"rules,omitempty"`
}

// TicketsCategoryConfig maps a ticket category name to its assigned Role ID.
type TicketsCategoryConfig struct {
	Name   string `json:"name,omitempty"`
	RoleID string `json:"role_id,omitempty"`
}

// TicketsConfig stores ticket system configuration per guild.
type TicketsConfig struct {
	Enabled             bool                    `json:"enabled,omitempty"`
	TranscriptChannelID string                  `json:"transcript_channel_id,omitempty"`
	Categories          []TicketsCategoryConfig `json:"categories,omitempty"`
}

// GuildConfig holds the configuration for a specific guild.
type GuildConfig struct {
	GuildID             string                     `json:"guild_id"`
	ConfigVersion       int64                      `json:"config_version"`
	BotInstanceTokens   map[string]EncryptedString `json:"bot_instance_tokens,omitempty"`
	BotInstanceStatuses map[string]string          `json:"bot_instance_statuses,omitempty"`
	FeatureRouting      map[string]string          `json:"feature_routing,omitempty"`
	Features            FeatureToggles             `json:"features,omitempty"`
	Channels            ChannelsConfig             `json:"channels,omitempty"`
	Roles               RolesConfig                `json:"roles,omitempty"`
	Stats               StatsConfig                `json:"stats,omitempty"`

	// Cache TTL configuration (per-guild tuning)
	RolesCacheTTL   string `json:"roles_cache_ttl,omitempty"`   // e.g.: "5m", "1h" (default: "5m")
	MemberCacheTTL  string `json:"member_cache_ttl,omitempty"`  // e.g.: "5m", "10m" (default: "5m")
	GuildCacheTTL   string `json:"guild_cache_ttl,omitempty"`   // e.g.: "15m", "30m" (default: "15m")
	ChannelCacheTTL string `json:"channel_cache_ttl,omitempty"` // e.g.: "15m", "30m" (default: "15m")

	UserPrune UserPruneConfig `json:"user_prune,omitempty"`

	PartnerBoard   PartnerBoardConfig  `json:"partner_board,omitempty"`
	ReactionBlocks ReactionBlockConfig `json:"reaction_blocks,omitempty"`
	QOTD           QOTDConfig          `json:"qotd,omitempty"`
	Tickets        TicketsConfig       `json:"tickets,omitempty"`
	RolePanels     []RolePanelConfig   `json:"role_panels,omitempty"`
	CustomEmbeds   []CustomEmbedConfig `json:"custom_embeds,omitempty"`

	// RuntimeConfig allows per-guild overrides for certain settings.
	RuntimeConfig RuntimeConfig `json:"runtime_config,omitempty"`

	// LogModerationScope determines what moderation events are logged.
	LogModerationScope string `json:"log_moderation_scope,omitempty"`
}

// UnmarshalJSON unmarshals json.
func (gc *GuildConfig) UnmarshalJSON(data []byte) error {
	type alias GuildConfig
	type rawGuildConfig struct {
		alias
		BotInstanceID        string            `json:"bot_instance_id,omitempty"`
		DomainBotInstanceIDs map[string]string `json:"domain_bot_instance_ids,omitempty"`
	}

	var raw rawGuildConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("GuildConfig.UnmarshalJSON: %w", err)
	}

	if raw.BotInstanceID != "" || len(raw.DomainBotInstanceIDs) > 0 {
		if raw.BotInstanceTokens == nil {
			raw.BotInstanceTokens = make(map[string]EncryptedString)
		}

		if raw.BotInstanceID != "" {
			normalized := NormalizeBotInstanceID(raw.BotInstanceID)
			if normalized != "" {
				if _, exists := raw.BotInstanceTokens[normalized]; !exists {
					raw.BotInstanceTokens[normalized] = ""
				}
			}
		}

		for _, instanceID := range raw.DomainBotInstanceIDs {
			normalized := NormalizeBotInstanceID(instanceID)
			if normalized != "" {
				if _, exists := raw.BotInstanceTokens[normalized]; !exists {
					raw.BotInstanceTokens[normalized] = ""
				}
			}
		}
	}

	*gc = GuildConfig(raw.alias)
	return nil
}

// BotConfig holds the configuration for the bot.
type BotConfig struct {
	ConfigVersion int64         `json:"config_version"`
	Guilds        []GuildConfig `json:"guilds"`

	// Features holds optional toggles for runtime behavior overrides.
	Features FeatureToggles `json:"features,omitempty"`

	// RuntimeConfig holds bot-level runtime overrides editable from Discord.
	// This intentionally replaces the previous "env var toggles" for operational
	// behavior (except for token), so settings can be managed in-app.
	//
	// NOTE: These are NOT environment variables. They are persisted in the active config store.
	RuntimeConfig RuntimeConfig `json:"runtime_config,omitempty"`
}

// CustomRPCConfig holds profiles for local Discord Rich Presence.
type CustomRPCConfig struct {
	DefaultProfile string             `json:"default_profile,omitempty"`
	UserProfiles   map[string]string  `json:"user_profiles,omitempty"`
	Profiles       []CustomRPCProfile `json:"profiles,omitempty"`
}

// CustomRPCProfile defines a single Rich Presence profile.
type CustomRPCProfile struct {
	Name                  string             `json:"name"`
	Disabled              bool               `json:"disabled,omitempty"`
	User                  string             `json:"user,omitempty"`
	ApplicationID         string             `json:"application_id"`
	Type                  string             `json:"type,omitempty"`
	URL                   string             `json:"url,omitempty"`
	Details               string             `json:"details,omitempty"`
	State                 string             `json:"state,omitempty"`
	Party                 RPCPartyConfig     `json:"party,omitempty"`
	Timestamp             RPCTimestampConfig `json:"timestamp,omitempty"`
	Assets                RPCAssetsConfig    `json:"assets,omitempty"`
	Buttons               []RPCButtonConfig  `json:"buttons,omitempty"`
	UpdateIntervalSeconds int                `json:"update_interval_seconds,omitempty"`
}

// RPCPartyConfig controls the optional party size display.
type RPCPartyConfig struct {
	ID      string `json:"id,omitempty"`
	Current int    `json:"current,omitempty"`
	Max     int    `json:"max,omitempty"`
}

// RPCTimestampConfig controls timestamp behavior.
type RPCTimestampConfig struct {
	Mode      string `json:"mode,omitempty"`
	StartUnix int64  `json:"start_unix,omitempty"`
	EndUnix   int64  `json:"end_unix,omitempty"`
	Start     string `json:"start,omitempty"`
	End       string `json:"end,omitempty"`
}

// RPCAssetsConfig controls asset keys and hover text.
type RPCAssetsConfig struct {
	LargeImageKey string `json:"large_image_key,omitempty"`
	LargeText     string `json:"large_text,omitempty"`
	SmallImageKey string `json:"small_image_key,omitempty"`
	SmallText     string `json:"small_text,omitempty"`
}

// RPCButtonConfig defines a label + URL button.
type RPCButtonConfig struct {
	Label string `json:"label,omitempty"`
	URL   string `json:"url,omitempty"`
}

// ResolveRuntimeConfig returns the runtime configuration for a guild,
// falling back to the global one if the field is not defined (zero-value).
func (cfg *BotConfig) ResolveRuntimeConfig(guildID string) RuntimeConfig {
	global := cfg.RuntimeConfig
	if global.ModerationLogging == nil {
		global.ModerationLogging = boolPtr(global.ModerationLoggingEnabled())
	}
	if guildID == "" {
		return global
	}

	var guildRC RuntimeConfig
	found := false
	for _, g := range cfg.Guilds {
		if g.GuildID == guildID {
			guildRC = g.RuntimeConfig
			found = true
			break
		}
	}

	if !found {
		return global
	}

	// Manual merging logic. Fields that are zero-value in guildRC will use global values.
	// This is better than a generic library for such a small struct and specific rules.
	resolved := global

	if guildRC.Database.Driver != "" {
		resolved.Database.Driver = guildRC.Database.Driver
	}
	if guildRC.Database.DatabaseURL != "" {
		resolved.Database.DatabaseURL = guildRC.Database.DatabaseURL
	}
	if guildRC.Database.MaxOpenConns != 0 {
		resolved.Database.MaxOpenConns = guildRC.Database.MaxOpenConns
	}
	if guildRC.Database.MaxIdleConns != 0 {
		resolved.Database.MaxIdleConns = guildRC.Database.MaxIdleConns
	}
	if guildRC.Database.ConnMaxLifetimeSecs != 0 {
		resolved.Database.ConnMaxLifetimeSecs = guildRC.Database.ConnMaxLifetimeSecs
	}
	if guildRC.Database.ConnMaxIdleTimeSecs != 0 {
		resolved.Database.ConnMaxIdleTimeSecs = guildRC.Database.ConnMaxIdleTimeSecs
	}
	if guildRC.Database.PingTimeoutMS != 0 {
		resolved.Database.PingTimeoutMS = guildRC.Database.PingTimeoutMS
	}

	if guildRC.BotTheme != "" {
		resolved.BotTheme = guildRC.BotTheme
	}

	if guildRC.DisableDBCleanup {
		resolved.DisableDBCleanup = true
	}
	if guildRC.DisableMessageLogs {
		resolved.DisableMessageLogs = true
	}
	if guildRC.DisableEntryExitLogs {
		resolved.DisableEntryExitLogs = true
	}
	if guildRC.DisableReactionLogs {
		resolved.DisableReactionLogs = true
	}
	if guildRC.DisableUserLogs {
		resolved.DisableUserLogs = true
	}
	if guildRC.DisableCleanLog {
		resolved.DisableCleanLog = true
	}
	if guildRC.ModerationLogging != nil {
		resolved.ModerationLogging = boolPtr(*guildRC.ModerationLogging)
	}
	if guildRC.LogModerationScope != "" {
		resolved.LogModerationScope = guildRC.LogModerationScope
	}
	if guildRC.PresenceWatchUserID != "" {
		resolved.PresenceWatchUserID = guildRC.PresenceWatchUserID
	}
	if guildRC.PresenceWatchBot {
		resolved.PresenceWatchBot = true
	}

	if guildRC.MessageCacheTTLHours != 0 {
		resolved.MessageCacheTTLHours = guildRC.MessageCacheTTLHours
	}
	if guildRC.MessageDeleteOnLog {
		resolved.MessageDeleteOnLog = true
	}
	if guildRC.MessageCacheCleanup {
		resolved.MessageCacheCleanup = true
	}
	if guildRC.GlobalMaxWorkers != 0 {
		resolved.GlobalMaxWorkers = guildRC.GlobalMaxWorkers
	}

	if guildRC.BackfillChannelID != "" {
		resolved.BackfillChannelID = guildRC.BackfillChannelID
	}
	if guildRC.BackfillStartDay != "" {
		resolved.BackfillStartDay = guildRC.BackfillStartDay
	}

	// BackfillInitialDate is GuildOnly: it must be set in the guild config
	// and does not fall back to the global config.
	resolved.BackfillInitialDate = guildRC.BackfillInitialDate

	if guildRC.MimuWelcomeString != "" {
		resolved.MimuWelcomeString = guildRC.MimuWelcomeString
	}
	if guildRC.MimuGoodbyeString != "" {
		resolved.MimuGoodbyeString = guildRC.MimuGoodbyeString
	}

	if guildRC.DisableBotRolePermMirror {
		resolved.DisableBotRolePermMirror = true
	}
	if guildRC.BotRolePermMirrorActorRoleID != "" {
		resolved.BotRolePermMirrorActorRoleID = guildRC.BotRolePermMirrorActorRoleID
	}
	if mode := strings.TrimSpace(guildRC.WebhookEmbedValidation.Mode); mode != "" {
		resolved.WebhookEmbedValidation.Mode = mode
	}
	if guildRC.WebhookEmbedValidation.TimeoutMS > 0 {
		resolved.WebhookEmbedValidation.TimeoutMS = guildRC.WebhookEmbedValidation.TimeoutMS
	}
	if guildUpdates := guildRC.NormalizedWebhookEmbedUpdates(); len(guildUpdates) > 0 {
		resolved.WebhookEmbedUpdates = append([]WebhookEmbedUpdateConfig(nil), guildUpdates...)
	}
	if guildRC.DisableInteractiveEphemeral {
		resolved.DisableInteractiveEphemeral = true
	}
	return resolved
}

// ModerationLoggingEnabled resolves whether moderation logs should be sent.
// Defaults to true when runtime_config.moderation_logging is unset; the legacy
// "moderation_log_mode" key is migrated into ModerationLogging at JSON decode
// time by RuntimeConfig.UnmarshalJSON.
func (rc RuntimeConfig) ModerationLoggingEnabled() bool {
	if rc.ModerationLogging != nil {
		return *rc.ModerationLogging
	}
	return true
}

// ConfigSubscriber receives notifications when the bot configuration changes.
type ConfigSubscriber func(ctx context.Context, oldCfg, newCfg *BotConfig) error

// ConfigManager handles bot configuration management.
//
// Concurrency: ConfigManager is safe for concurrent use by multiple goroutines.
// Readers should treat Config() and GuildConfig() results as read-only snapshots;
// persist changes through the existing update helpers.
type ConfigManager struct {
	configFilePath  string
	logsDirPath     string
	store           ConfigStore
	logger          *slog.Logger
	config          *BotConfig
	guildIndex      map[string]int
	published       atomic.Pointer[publishedConfigSnapshot]
	indexRebuilds   atomic.Uint64
	indexMisses     atomic.Uint64
	indexDuplicates atomic.Uint64
	subscribers     []ConfigSubscriber
	mu              sync.RWMutex
}

type publishedConfigSnapshot struct {
	config     *BotConfig
	guildIndex map[string]int
}

// GuildIndexStats exposes counters for the guild config index.
type GuildIndexStats struct {
	Rebuilds   uint64
	Misses     uint64
	Duplicates uint64
}

// AvatarChange holds information about a user's avatar change.
type AvatarChange struct {
	UserID    string
	Username  string
	OldAvatar string
	NewAvatar string
	Timestamp time.Time
}

func GuildConfigByID(cfg *BotConfig, guildID string) (*GuildConfig, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, strings.TrimSpace(guildID))
	}

	target := strings.TrimSpace(guildID)
	if target == "" {
		return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, target)
	}

	for idx := range cfg.Guilds {
		if cfg.Guilds[idx].GuildID == target {
			return &cfg.Guilds[idx], nil
		}
	}
	return nil, fmt.Errorf("%w: guild_id=%s", ErrGuildConfigNotFound, target)
}

// RolesCacheTTLDuration returns the configured TTL for the roles cache or a default of 5m.
func (gc *GuildConfig) RolesCacheTTLDuration() time.Duration {
	const def = 5 * time.Minute
	if gc == nil || gc.RolesCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.RolesCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// MemberCacheTTLDuration returns the configured TTL for the members cache or a default of 5m.
func (gc *GuildConfig) MemberCacheTTLDuration() time.Duration {
	const def = 5 * time.Minute
	if gc == nil || gc.MemberCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.MemberCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// GuildCacheTTLDuration returns the configured TTL for the guilds cache or a default of 15m.
func (gc *GuildConfig) GuildCacheTTLDuration() time.Duration {
	const def = 15 * time.Minute
	if gc == nil || gc.GuildCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.GuildCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// ChannelCacheTTLDuration returns the configured TTL for the channels cache or a default of 15m.
func (gc *GuildConfig) ChannelCacheTTLDuration() time.Duration {
	const def = 15 * time.Minute
	if gc == nil || gc.ChannelCacheTTL == "" {
		return def
	}
	d, err := time.ParseDuration(gc.ChannelCacheTTL)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

// SetRolesCacheTTL sets the roles cache TTL per guild (e.g., "5m", "1h") and persists the setting.
func (mgr *ConfigManager) SetRolesCacheTTL(guildID string, ttl string) error {
	if guildID == "" {
		return fmt.Errorf("guild not found")
	}
	// Validate format (allow empty to reset to default)
	if ttl != "" {
		if _, err := time.ParseDuration(ttl); err != nil {
			return fmt.Errorf("invalid ttl: %w", err)
		}
	}
	_, err := mgr.UpdateConfig(context.Background(), func(cfg *BotConfig) error {
		gcfg, err := GuildConfigByID(cfg, guildID)
		if err != nil {
			return fmt.Errorf("guild not found")
		}
		gcfg.RolesCacheTTL = ttl
		return nil
	})

	return err
}

// GetRolesCacheTTL gets the configured roles cache TTL (original string, e.g., "5m").
func (mgr *ConfigManager) GetRolesCacheTTL(guildID string) string {
	gcfg := mgr.GuildConfig(guildID)
	if gcfg == nil {
		return ""
	}
	return gcfg.RolesCacheTTL
}

// ## Error Types

// ValidationError represents a validation error with field context.
type ValidationError struct {
	Field   string
	Value   any
	Message string
}

// ValidationField validations field.
func (e ValidationError) ValidationField() string {
	return e.Field
}

// Error errors.
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(field string, value any, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// ConfigError represents configuration-related errors.
type ConfigError struct {
	Operation string
	Path      string
	Cause     error
}

// ConfigErrorPath configs error path.
func (e ConfigError) ConfigErrorPath() string {
	return e.Path
}

// Error errors.
func (e ConfigError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("config %s failed for %s: %v", e.Operation, e.Path, e.Cause)
	}
	return fmt.Sprintf("config %s failed for %s", e.Operation, e.Path)
}

// Unwrap unwraps.
func (e ConfigError) Unwrap() error {
	return e.Cause
}

// NewConfigError creates a new configuration error.
func NewConfigError(operation, path string, cause error) ConfigError {
	return ConfigError{
		Operation: operation,
		Path:      path,
		Cause:     cause,
	}
}

// DiscordError represents Discord API related errors.
type DiscordError struct {
	Operation string
	Code      int
	Message   string
	Cause     error
}

// DiscordErrorCode discords error code.
func (e DiscordError) DiscordErrorCode() int {
	return e.Code
}

// Error errors.
func (e DiscordError) Error() string {
	if e.Code > 0 {
		return fmt.Sprintf("Discord API error during %s (code %d): %s", e.Operation, e.Code, e.Message)
	}
	return fmt.Sprintf("Discord API error during %s: %s", e.Operation, e.Message)
}

// Unwrap unwraps.
func (e DiscordError) Unwrap() error {
	return e.Cause
}

// NewDiscordError creates a new Discord API error.
func NewDiscordError(operation string, code int, message string, cause error) DiscordError {
	return DiscordError{
		Operation: operation,
		Code:      code,
		Message:   message,
		Cause:     cause,
	}
}

// ## Utility Functions

// IsRetryableError determines if an error can be retried.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific retryable errors.
	if errors.Is(err, ErrRateLimited) {
		return true
	}

	// Check for Discord errors that might be retryable.
	var discordErr DiscordError
	if errors.As(err, &discordErr) {
		// 5xx errors are typically retryable.
		return discordErr.Code >= 500 && discordErr.Code < 600
	}

	return false
}

// ## General Errors

// ErrRateLimited defines err rate limited.
var ErrRateLimited = errors.New("rate limited")

```

// === FILE: pkg/files/types_embeds.go ===
```go
package files

import (
	"errors"
	"strings"
)

type CustomEmbedFieldConfig struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type CustomEmbedPostingConfig struct {
	ChannelID    string `json:"channel_id"`
	MessageID    string `json:"message_id"`
	WebhookID    string `json:"webhook_id,omitempty"`
	WebhookToken string `json:"webhook_token,omitempty"`
}

func (p CustomEmbedPostingConfig) IsZero() bool {
	return strings.TrimSpace(p.ChannelID) == "" &&
		strings.TrimSpace(p.MessageID) == "" &&
		strings.TrimSpace(p.WebhookID) == "" &&
		strings.TrimSpace(p.WebhookToken) == ""
}

type CustomEmbedConfig struct {
	Key           string                     `json:"key"`
	Title         string                     `json:"title,omitempty"`
	Description   string                     `json:"description,omitempty"`
	Color         int                        `json:"color,omitempty"`
	AuthorName    string                     `json:"author_name,omitempty"`
	AuthorIconURL string                     `json:"author_icon_url,omitempty"`
	FooterText    string                     `json:"footer_text,omitempty"`
	FooterIconURL string                     `json:"footer_icon_url,omitempty"`
	ImageURL      string                     `json:"image_url,omitempty"`
	ThumbnailURL  string                     `json:"thumbnail_url,omitempty"`
	Fields        []CustomEmbedFieldConfig   `json:"fields,omitempty"`
	Postings      []CustomEmbedPostingConfig `json:"postings,omitempty"`
}

func (cfg CustomEmbedConfig) IsZero() bool {
	return strings.TrimSpace(cfg.Key) == "" &&
		strings.TrimSpace(cfg.Title) == "" &&
		strings.TrimSpace(cfg.Description) == "" &&
		cfg.Color == 0 &&
		strings.TrimSpace(cfg.AuthorName) == "" &&
		strings.TrimSpace(cfg.AuthorIconURL) == "" &&
		strings.TrimSpace(cfg.FooterText) == "" &&
		strings.TrimSpace(cfg.FooterIconURL) == "" &&
		strings.TrimSpace(cfg.ImageURL) == "" &&
		strings.TrimSpace(cfg.ThumbnailURL) == "" &&
		len(cfg.Fields) == 0 &&
		len(cfg.Postings) == 0
}

var ErrCustomEmbedPostingNotFound = errors.New("custom embed posting not found")

func cloneCustomEmbeds(in []CustomEmbedConfig) []CustomEmbedConfig {
	if len(in) == 0 {
		return nil
	}
	out := make([]CustomEmbedConfig, 0, len(in))
	for _, ce := range in {
		out = append(out, cloneCustomEmbed(ce))
	}
	return out
}

func cloneCustomEmbed(in CustomEmbedConfig) CustomEmbedConfig {
	out := CustomEmbedConfig{
		Key:           in.Key,
		Title:         in.Title,
		Description:   in.Description,
		Color:         in.Color,
		AuthorName:    in.AuthorName,
		AuthorIconURL: in.AuthorIconURL,
		FooterText:    in.FooterText,
		FooterIconURL: in.FooterIconURL,
		ImageURL:      in.ImageURL,
		ThumbnailURL:  in.ThumbnailURL,
	}

	if len(in.Fields) > 0 {
		out.Fields = make([]CustomEmbedFieldConfig, len(in.Fields))
		copy(out.Fields, in.Fields)
	}

	if len(in.Postings) > 0 {
		out.Postings = make([]CustomEmbedPostingConfig, len(in.Postings))
		copy(out.Postings, in.Postings)
	}

	return out
}

```

// === FILE: pkg/files/validation_errors.go ===
```go
package files

import (
	"errors"
	"fmt"
)

var errValidationFailure = errors.New(ErrValidationFailed)

// IsValidationError reports whether err carries config validation context.
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}

	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		return true
	}

	var validationErrPtr *ValidationError
	return errors.Is(err, errValidationFailure) || errors.As(err, &validationErrPtr)
}

func wrapValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errValidationFailure, err)
}

```

// === FILE: pkg/files/validation_errors_test.go ===
```go
package files

import "testing"

func TestIsValidationErrorMatchesWrappedValidation(t *testing.T) {
	t.Parallel()

	err := wrapValidationError(NewValidationError(
		"guilds[0].roles.auto_assignment.required_roles",
		[]string{"stable-role"},
		"required_roles must contain exactly 2 role IDs",
	))
	if !IsValidationError(err) {
		t.Fatalf("expected wrapped validation error to be detected, got %v", err)
	}
}

```

// === FILE: pkg/files/version.go ===
```go
package files

// DiscordCoreVersion is the current version of the discordcore package.
// This value is automatically updated by the release CLI tool.
const DiscordCoreVersion = "v0.857.0-rc.6"

// AppVersion is the version of the application using discordcore.
var AppVersion string

// SetAppVersion sets the version of the application using discordcore.
func SetAppVersion(v string) {
	AppVersion = v
}

```

