package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestHasRoleID(t *testing.T) {
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
	if memberHasRole(nil, "r1") {
		t.Fatalf("expected false for nil member")
	}
	member := &discordgo.Member{Roles: []string{"a", "b"}}
	if !memberHasRole(member, "a") {
		t.Fatalf("expected true for existing role")
	}
	if memberHasRole(member, "z") {
		t.Fatalf("expected false for missing role")
	}
}
