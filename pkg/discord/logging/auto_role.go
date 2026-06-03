package logging

import "github.com/bwmarrin/discordgo"

type autoRoleDecision int

const (
	autoRoleNoop autoRoleDecision = iota
	autoRoleAddTarget
	autoRoleRemoveTarget
)

// hasRoleID checks if a role ID is present in a member role list.
func hasRoleID(roles []string, roleID string) bool {
	if roleID == "" {
		return false
	}
	for _, rid := range roles {
		if rid == roleID {
			return true
		}
	}
	return false
}

// evaluateAutoRoleDecision centralizes the auto-assignment rule used by realtime
// member updates and periodic reconciliation.
//
// Ordering contract for requiredRoles:
// - requiredRoles[0] is roleA (stable level role, e.g. Arcane lvl 20).
// - requiredRoles[1] is roleB (volatile booster role, can be gained/lost).
//
// Business rule:
// - Add target role when member has both roleA and roleB and target is missing.
// - Remove target role when target exists but roleA is missing.
//
// Even if roleA is expected to be stable, we keep the removal rule as a safety
// self-heal for manual edits and stale states.
func evaluateAutoRoleDecision(memberRoles []string, targetRoleID string, requiredRoles []string) autoRoleDecision {
	if targetRoleID == "" || len(requiredRoles) < 2 {
		return autoRoleNoop
	}

	roleA := requiredRoles[0]
	roleB := requiredRoles[1]

	hasTarget := hasRoleID(memberRoles, targetRoleID)
	hasA := hasRoleID(memberRoles, roleA)
	hasB := hasRoleID(memberRoles, roleB)

	if hasTarget && !hasA {
		return autoRoleRemoveTarget
	}
	if !hasTarget && hasA && hasB {
		return autoRoleAddTarget
	}
	return autoRoleNoop
}

// memberHasRole checks if a member has a role ID.
func memberHasRole(member *discordgo.Member, roleID string) bool {
	if member == nil {
		return false
	}
	return hasRoleID(member.Roles, roleID)
}
