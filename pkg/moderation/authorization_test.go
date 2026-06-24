package moderation

import "testing"

// TestHasPermission validates role hierarchies utilizing table-driven tests.
// It verifies standard permission evaluation, the Administrator flag override,
// and the scenario where a member lacks all roles.
func TestHasPermission(t *testing.T) {
	t.Parallel()
	const (
		guildID  = "guild_123"
		permKick = int64(0x00000002)
		permBan  = int64(0x00000004)
	)

	roles := map[string]Role{
		guildID:      {ID: guildID, Position: 0, Permissions: 0},
		"role_1":     {ID: "role_1", Position: 1, Permissions: permKick},
		"role_2":     {ID: "role_2", Position: 2, Permissions: permBan},
		"role_admin": {ID: "role_admin", Position: 10, Permissions: PermissionAdministrator},
	}

	tests := []struct {
		name         string
		member       *Member
		requiredPerm int64
		expected     bool
	}{
		{
			name:         "Member with specific permission",
			member:       &Member{UserID: "user1", RoleIDs: []string{"role_1"}},
			requiredPerm: permKick,
			expected:     true,
		},
		{
			name:         "Member without specific permission",
			member:       &Member{UserID: "user2", RoleIDs: []string{"role_1"}},
			requiredPerm: permBan,
			expected:     false,
		},
		{
			name:         "Member with Administrator flag override",
			member:       &Member{UserID: "user3", RoleIDs: []string{"role_admin"}},
			requiredPerm: permBan,
			expected:     true,
		},
		{
			name:         "Member with total omission of roles",
			member:       &Member{UserID: "user4", RoleIDs: []string{}},
			requiredPerm: permKick,
			expected:     false,
		},
		{
			name:         "Nil member",
			member:       nil,
			requiredPerm: permKick,
			expected:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := HasPermission(tc.member, guildID, roles, tc.requiredPerm)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

// TestCanModerate evaluates structural boundary anomalies such as actor and target
// possessing privileges on the exact same layer.
func TestCanModerate(t *testing.T) {
	t.Parallel()
	const guildID = "guild_123"

	roles := map[string]Role{
		guildID:  {ID: guildID, Position: 0, Permissions: 0},
		"role_1": {ID: "role_1", Position: 1, Permissions: 0},
		"role_2": {ID: "role_2", Position: 2, Permissions: 0},
		"role_3": {ID: "role_3", Position: 2, Permissions: 0},
	}

	tests := []struct {
		name     string
		actor    *Member
		target   *Member
		expected bool
	}{
		{
			name:     "Actor strictly higher",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_2"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_1"}},
			expected: true,
		},
		{
			name:     "Target strictly higher",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_1"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_2"}},
			expected: false,
		},
		{
			name:     "Actor and Target on the exact same layer (same role)",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_2"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_2"}},
			expected: false,
		},
		{
			name:     "Actor and Target on the exact same layer (different roles)",
			actor:    &Member{UserID: "user1", RoleIDs: []string{"role_2"}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_3"}},
			expected: false,
		},
		{
			name:     "Actor missing roles, target has roles",
			actor:    &Member{UserID: "user1", RoleIDs: []string{}},
			target:   &Member{UserID: "user2", RoleIDs: []string{"role_1"}},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := CanModerate(tc.actor, tc.target, guildID, roles)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}
