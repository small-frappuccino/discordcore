//go:build integration

package tickets

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
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
	if ClosedToOpenName("closed-0005") != "ticket-0005" {
		t.Errorf("ClosedToOpenName failed")
	}
	if !IsOpenTicket("ticket-0010") || IsOpenTicket("closed-0010") {
		t.Errorf("IsOpenTicket failed")
	}
	if !IsClosedTicket("closed-0010") || IsClosedTicket("ticket-0010") {
		t.Errorf("IsClosedTicket failed")
	}
}

func TestNextID_ACID(t *testing.T) {
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

	store, err := storage.NewStore(db, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("failed to init store: %v", err)
	}

	mgr := NewManager(store)
	guildID := "concurrent-test-guild"

	const workers = 20
	const iterations = 50

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make(map[int64]bool)

	errCh := make(chan error, workers*iterations)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				id, err := mgr.NextID(ctx, guildID)
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
		}()
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}

	if len(results) != workers*iterations {
		t.Errorf("expected %d unique IDs, got %d", workers*iterations, len(results))
	}
}
