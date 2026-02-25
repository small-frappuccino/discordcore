package logging

import "github.com/bwmarrin/discordgo"

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

// memberHasRole checks if a member has a role ID.
func memberHasRole(member *discordgo.Member, roleID string) bool {
	if member == nil {
		return false
	}
	return hasRoleID(member.Roles, roleID)
}
