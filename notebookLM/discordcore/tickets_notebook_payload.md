# Domain Architecture: tickets

## Layout Topology
```text
tickets/
├── ticket.go
├── ticket_test.go
└── ticket_unit_test.go
```

## Source Stream Aggregation

// === FILE: pkg/tickets/ticket.go ===
```go
package tickets

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
)

// GenerateTicketName creates a canonical text channel name for a new ticket.
func GenerateTicketName(id int64) string {
	return fmt.Sprintf("ticket-%04d", id)
}

// IsOpenTicket checks if the given channel name indicates an active ticket.
func IsOpenTicket(name string) bool {
	return len(name) >= 7 && name[:7] == "ticket-"
}

// IsClosedTicket checks if the given channel name indicates a closed ticket.
func IsClosedTicket(name string) bool {
	return len(name) >= 7 && name[:7] == "closed-"
}

// OpenToClosedName converts an open ticket channel name to a closed one.
func OpenToClosedName(name string) string {
	if IsOpenTicket(name) {
		return "closed-" + name[7:]
	}
	return name
}

// ClosedToOpenName converts a closed ticket channel name back to an open one.
func ClosedToOpenName(name string) string {
	if IsClosedTicket(name) {
		return "ticket-" + name[7:]
	}
	return name
}

// ComputeOpenMemberAllow returns the permission bits allowed for the ticket creator upon opening.
func ComputeOpenMemberAllow() discord.Permissions {
	// Standard viewing, sending, and history permissions.
	return discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory
}

// ComputeOpenRoleAllow returns the permission bits allowed for the designated staff role upon opening.
func ComputeOpenRoleAllow() discord.Permissions {
	return discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionReadMessageHistory
}

// ComputeCloseMemberAllow takes the current allow mask and removes the ability to send messages.
func ComputeCloseMemberAllow(current discord.Permissions) discord.Permissions {
	// Bitwise clearing of the SendMessages flag
	return current &^ discord.PermissionSendMessages
}

// ComputeCloseMemberDeny takes the current deny mask and explicitly denies the ability to send messages.
func ComputeCloseMemberDeny(current discord.Permissions) discord.Permissions {
	// Bitwise setting of the SendMessages flag
	return current | discord.PermissionSendMessages
}

// ComputeReopenMemberAllow takes the current allow mask and restores the ability to send messages.
func ComputeReopenMemberAllow(current discord.Permissions) discord.Permissions {
	return current | discord.PermissionSendMessages
}

// ComputeReopenMemberDeny takes the current deny mask and removes explicit denial to send messages.
func ComputeReopenMemberDeny(current discord.Permissions) discord.Permissions {
	return current &^ discord.PermissionSendMessages
}

// Manager orchestrates domain logic for tickets avoiding direct Discord integrations.
type Manager struct {
	store  *postgres.Store
	logger *slog.Logger
}

// NewManager constructs a ticket manager.
func NewManager(store *postgres.Store, logger *slog.Logger) *Manager {
	return &Manager{
		store:  store,
		logger: logger,
	}
}

// NextID retrieves the next sequential ticket identifier for a given guild in a thread-safe manner.
func (m *Manager) NextID(ctx context.Context, guildID string) (int64, error) {
	// Debug: Granular tracking of transactional ID retrieval.
	m.logger.Debug("requesting next sequential ticket ID from storage",
		slog.String("guildID", guildID),
	)

	id, err := m.store.NextTicketID(ctx, guildID)
	if err != nil {
		// Error: Blocking structural failure restricted to the scope of the transaction.
		m.logger.Error("failed to retrieve next sequential ticket ID",
			slog.String("guildID", guildID),
			slog.String("synthetic_fault_code", "500"),
			slog.String("error", err.Error()),
		)
		return 0, err
	}

	return id, nil
}

```

// === FILE: pkg/tickets/ticket_test.go ===
```go
//go:build integration

package tickets

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
	"golang.org/x/sync/errgroup"
)

func TestNextID_ACID(t *testing.T) {
	t.Parallel()
	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}
	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store, err := postgres.NewStore(db, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := NewManager(store, logger)
	guildID := "concurrent-test-guild"

	const workers = 20
	const iterations = 50

	var mu sync.Mutex
	results := make(map[int64]bool)
	errCh := make(chan error, workers*iterations)

	eg, ctx := errgroup.WithContext(context.Background())
	for i := 0; i < workers; i++ {
		eg.Go(func() error {
			for j := 0; j < iterations; j++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				id, err := mgr.NextID(ctxTimeout, guildID)
				cancel()
				if err != nil {
					errCh <- err
					continue
				}
				mu.Lock()
				if results[id] {
					errCh <- fmt.Errorf("duplicate ticket ID detected: %d", id)
				}
				results[id] = true
				mu.Unlock()
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent workers failed: %v", err)
	}
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}

	if len(results) != workers*iterations {
		t.Errorf("expected %d unique IDs, got %d", workers*iterations, len(results))
	}
}

```

// === FILE: pkg/tickets/ticket_unit_test.go ===
```go
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

```

