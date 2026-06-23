package tickets

import (
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
)

func TestPermissionsBitwise(t *testing.T) {
	tests := []struct {
		name          string
		initialAllow  discord.Permissions
		initialDeny   discord.Permissions
		opAllow       func(discord.Permissions) discord.Permissions
		opDeny        func(discord.Permissions) discord.Permissions
		expectedAllow discord.Permissions
		expectedDeny  discord.Permissions
	}{
		{
			name:          "ComputeCloseMember",
			initialAllow:  discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory,
			initialDeny:   0,
			opAllow:       ComputeCloseMemberAllow,
			opDeny:        ComputeCloseMemberDeny,
			expectedAllow: discord.PermissionViewChannel | discord.PermissionReadMessageHistory,
			expectedDeny:  discord.PermissionSendMessages,
		},
		{
			name:          "ComputeReopenMember",
			initialAllow:  discord.PermissionViewChannel | discord.PermissionReadMessageHistory,
			initialDeny:   discord.PermissionSendMessages,
			opAllow:       ComputeReopenMemberAllow,
			opDeny:        ComputeReopenMemberDeny,
			expectedAllow: discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory,
			expectedDeny:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAllow := tc.opAllow(tc.initialAllow)
			if gotAllow != tc.expectedAllow {
				t.Errorf("Allow mask mismatch: got %v, expected %v", gotAllow, tc.expectedAllow)
			}
			gotDeny := tc.opDeny(tc.initialDeny)
			if gotDeny != tc.expectedDeny {
				t.Errorf("Deny mask mismatch: got %v, expected %v", gotDeny, tc.expectedDeny)
			}
		})
	}
}

func TestNamingLogic(t *testing.T) {
	if GenerateTicketName(5) != "ticket-0005" {
		t.Errorf("GenerateTicketName(5) failed")
	}
	if OpenToClosedName("ticket-0005") != "closed-0005" {
		t.Errorf("OpenToClosedName failed")
	}
	if OpenToClosedName("not-a-ticket") != "not-a-ticket" {
		t.Errorf("OpenToClosedName on non-ticket failed")
	}
	if ClosedToOpenName("closed-0005") != "ticket-0005" {
		t.Errorf("ClosedToOpenName failed")
	}
	if ClosedToOpenName("not-a-ticket") != "not-a-ticket" {
		t.Errorf("ClosedToOpenName on non-ticket failed")
	}
	if !IsOpenTicket("ticket-0010") || IsOpenTicket("closed-0010") {
		t.Errorf("IsOpenTicket failed")
	}
	if !IsClosedTicket("closed-0010") || IsClosedTicket("ticket-0010") {
		t.Errorf("IsClosedTicket failed")
	}
}

func TestOpenPermissions(t *testing.T) {
	expected := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory
	if ComputeOpenMemberAllow() != expected {
		t.Errorf("ComputeOpenMemberAllow mismatch")
	}
	if ComputeOpenRoleAllow() != expected {
		t.Errorf("ComputeOpenRoleAllow mismatch")
	}
}
