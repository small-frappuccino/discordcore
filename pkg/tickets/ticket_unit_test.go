package tickets

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
)

func TestPermissionsBitwise(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	expected := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory
	if ComputeOpenMemberAllow() != expected {
		t.Errorf("ComputeOpenMemberAllow mismatch")
	}
	if ComputeOpenRoleAllow() != expected {
		t.Errorf("ComputeOpenRoleAllow mismatch")
	}
}

func TestManager_NewAndNextID(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := postgres.NewStore(mock, logger)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	mgr := NewManager(store, logger)
	if mgr == nil {
		t.Fatal("expected NewManager to return non-nil manager")
	}
	if mgr.store != store {
		t.Error("manager store mismatch")
	}
	if mgr.logger != logger {
		t.Error("manager logger mismatch")
	}

	guildID := "test-guild"

	// 1. NextID Happy Path
	columns := []string{"last_id"}
	mock.ExpectQuery(`INSERT INTO ticket_sequences`).
		WithArgs(guildID).
		WillReturnRows(pgxmock.NewRows(columns).AddRow(int64(42)))

	id, err := mgr.NextID(context.Background(), guildID)
	if err != nil {
		t.Errorf("NextID failed: %v", err)
	}
	if id != 42 {
		t.Errorf("expected NextID to return 42, got %d", id)
	}

	// 2. NextID Error Path
	dbErr := errors.New("db connection failure")
	mock.ExpectQuery(`INSERT INTO ticket_sequences`).
		WithArgs(guildID).
		WillReturnError(dbErr)

	id, err = mgr.NextID(context.Background(), guildID)
	if err == nil {
		t.Error("expected NextID to return an error, got nil")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("expected error to wrap '%v', got '%v'", dbErr, err)
	}
	if id != 0 {
		t.Errorf("expected NextID to return 0 on error, got %d", id)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled database expectations: %v", err)
	}
}
