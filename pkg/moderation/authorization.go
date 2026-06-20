package moderation

// Role defines the properties of a guild role necessary for evaluating hierarchy
// and permissions in a Discord-agnostic manner.
type Role struct {
	ID          string
	Position    int
	Permissions int64
}

// Member defines the properties of a guild member necessary for evaluating permissions.
type Member struct {
	UserID  string
	RoleIDs []string
}

const (
	// PermissionAdministrator is the equivalent of the Discord Administrator flag (0x00000008).
	PermissionAdministrator int64 = 0x00000008
)

// HasPermission evaluates if a member possesses a specific bitwise permission.
// It checks all roles the member has, including the implicit @everyone role (guildID).
// If the member has a role with the Administrator flag (0x00000008), this function
// will always return true, short-circuiting other evaluations.
func HasPermission(member *Member, guildID string, rolesByID map[string]Role, requiredPerm int64) bool {
	if member == nil {
		return false
	}

	var accumulatedPerms int64

	// Accumulate permissions from the @everyone role.
	if everyoneRole, ok := rolesByID[guildID]; ok {
		accumulatedPerms |= everyoneRole.Permissions
	}

	// Accumulate permissions from all assigned roles.
	for _, roleID := range member.RoleIDs {
		if role, ok := rolesByID[roleID]; ok {
			accumulatedPerms |= role.Permissions
		}
	}

	// Administrator flag overrides any other permission checks.
	if (accumulatedPerms & PermissionAdministrator) != 0 {
		return true
	}

	return (accumulatedPerms & requiredPerm) != 0
}

// HighestRolePosition calculates the highest position of any role a member has.
// This is used for hierarchy evaluations (e.g., actor cannot moderate a target
// with an equal or higher role position).
func HighestRolePosition(member *Member, guildID string, rolesByID map[string]Role) int {
	if member == nil {
		return -1
	}

	pos := -1

	// Check @everyone role position.
	if everyoneRole, ok := rolesByID[guildID]; ok {
		pos = everyoneRole.Position
	}

	// Find the maximum position across all assigned roles.
	for _, roleID := range member.RoleIDs {
		if role, ok := rolesByID[roleID]; ok && role.Position > pos {
			pos = role.Position
		}
	}

	return pos
}

// CanModerate checks if the actor can take moderation action against the target based
// strictly on role hierarchy. The actor's highest role must be strictly greater than
// the target's highest role.
func CanModerate(actor, target *Member, guildID string, rolesByID map[string]Role) bool {
	actorPos := HighestRolePosition(actor, guildID, rolesByID)
	targetPos := HighestRolePosition(target, guildID, rolesByID)

	return actorPos > targetPos
}
