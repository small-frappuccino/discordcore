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

	if _, err := normalizeDomainBotInstanceBindings(cfg); err != nil {
		return err
	}

	for idx := range cfg.Guilds {
		cfg.Guilds[idx].BotInstanceID = NormalizeBotInstanceID(cfg.Guilds[idx].BotInstanceID)

		if err := validateGuildAutoAssignmentOrder(&cfg.Guilds[idx], idx); err != nil {
			return err
		}
	}

	return nil
}

func normalizeDomainBotInstanceBindings(cfg *BotConfig) (bool, error) {
	if cfg == nil {
		return false, nil
	}

	changed := false
	for idx := range cfg.Guilds {
		normalized, guildChanged, err := normalizeGuildDomainBotInstanceBindings(cfg.Guilds[idx].DomainBotInstanceIDs, idx)
		if err != nil {
			return changed, err
		}
		cfg.Guilds[idx].DomainBotInstanceIDs = normalized
		if guildChanged {
			changed = true
		}
	}

	return changed, nil
}

func normalizeGuildDomainBotInstanceBindings(in map[string]string, guildIndex int) (map[string]string, bool, error) {
	if len(in) == 0 {
		return nil, in != nil, nil
	}

	out := make(map[string]string, len(in))
	changed := false
	fieldBase := fmt.Sprintf("guilds[%d].domain_bot_instance_ids", guildIndex)

	for rawDomain, rawBotInstanceID := range in {
		domain := NormalizeBotDomain(rawDomain)
		if domain == "" {
			return nil, changed, NewValidationError(fieldBase, in, "domain override keys must be non-empty")
		}
		switch domain {
		case botDomainCore, botDomainDefault:
			return nil, changed, NewValidationError(
				fmt.Sprintf("%s.%s", fieldBase, domain),
				rawBotInstanceID,
				"use bot_instance_id for the implicit default domain; domain_bot_instance_ids only accepts specialized domains",
			)
		}

		botInstanceID := NormalizeBotInstanceID(rawBotInstanceID)
		if botInstanceID == "" {
			return nil, changed, NewValidationError(
				fmt.Sprintf("%s.%s", fieldBase, domain),
				rawBotInstanceID,
				"domain override must reference a non-empty bot instance id",
			)
		}

		if existing, ok := out[domain]; ok {
			if existing != botInstanceID {
				return nil, changed, NewValidationError(
					fmt.Sprintf("%s.%s", fieldBase, domain),
					rawBotInstanceID,
					fmt.Sprintf("domain override resolves to conflicting bot instance ids (%s, %s)", existing, botInstanceID),
				)
			}
			changed = true
			continue
		}

		if rawDomain != domain || rawBotInstanceID != botInstanceID {
			changed = true
		}
		out[domain] = botInstanceID
	}

	if len(out) != len(in) {
		changed = true
	}

	return out, changed, nil
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
