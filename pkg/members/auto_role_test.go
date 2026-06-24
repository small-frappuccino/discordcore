package members

import (
	"testing"
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
