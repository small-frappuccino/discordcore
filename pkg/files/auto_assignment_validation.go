package files

import (
	"fmt"
	"strings"
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
		cfg.Guilds[idx].BotInstanceID = NormalizeBotInstanceID(cfg.Guilds[idx].BotInstanceID)

		moderation, err := NormalizeGuildModerationConfig(
			cfg.Guilds[idx].Rulesets,
			cfg.Guilds[idx].LooseLists,
			cfg.Guilds[idx].Blocklist,
		)
		if err != nil {
			if validationErr, ok := err.(ValidationError); ok {
				validationErr.Field = fmt.Sprintf("guilds[%d].%s", idx, validationErr.Field)
				return validationErr
			}
			return err
		}
		cfg.Guilds[idx].Rulesets = moderation.Rulesets
		cfg.Guilds[idx].LooseLists = moderation.LooseRules
		cfg.Guilds[idx].Blocklist = moderation.Blocklist

		if err := validateGuildAutoAssignmentOrder(&cfg.Guilds[idx], idx); err != nil {
			return err
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
