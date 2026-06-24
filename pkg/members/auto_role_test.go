package members

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
)

func TestHasRoleID(t *testing.T) {
	t.Parallel()
	if hasRoleID(nil, "r1") {
		t.Fatalf("expected false for nil roles")
	}
	if hasRoleID([]string{"r1", "r2"}, "") {
		t.Fatalf("expected false for empty role id")
	}
	if !hasRoleID([]string{"r1", "r2"}, "r2") {
		t.Fatalf("expected true when role exists")
	}
	if hasRoleID([]string{"r1", "r2"}, "r3") {
		t.Fatalf("expected false when role does not exist")
	}
}

func TestMemberHasRole(t *testing.T) {
	t.Parallel()
	if memberHasRole(nil, "r1") {
		t.Fatalf("expected false for nil member")
	}
	member := &discord.Member{RoleIDs: []discord.RoleID{discord.RoleID(1), discord.RoleID(2)}}
	if !memberHasRole(member, "1") {
		t.Fatalf("expected true for existing role")
	}
	if memberHasRole(member, "z") {
		t.Fatalf("expected false for missing role")
	}
}

func TestEvaluateAutoRoleDecision(t *testing.T) {
	t.Parallel()
	required := []string{"role-a", "role-b"}
	target := "role-target"

	tests := []struct {
		name  string
		roles []string
		want  autoRoleDecision
	}{
		{
			name:  "add target when member has role A and role B",
			roles: []string{"role-a", "role-b"},
			want:  AutoRoleAddTarget},
		{
			name:  "remove target when role A is missing",
			roles: []string{"role-target", "role-b"},
			want:  AutoRoleRemoveTarget},
		{
			name:  "noop when member already has target and still has role A",
			roles: []string{"role-a", "role-target"},
			want:  autoRoleNoop},
		{
			name:  "noop when only role A is present",
			roles: []string{"role-a"},
			want:  autoRoleNoop}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateAutoRoleDecision(tt.roles, target, required)
			if got != tt.want {
				t.Fatalf("EvaluateAutoRoleDecision()=%v, want %v", got, tt.want)
			}
		})
	}
}
