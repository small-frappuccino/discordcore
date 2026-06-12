package testdb_test

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestOpenIsolatedDatabase(t *testing.T) {
	dsn, err := testdb.BaseDatabaseURLFromEnv()
	if testdb.IsDatabaseURLNotConfigured(err) {
		t.Skip("skipping test due to missing database url")
	}

	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to open isolated database: %v", err)
	}

	// Verify we can ping
	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("failed to ping test database: %v", err)
	}

	// Clean up
	if err := cleanup(); err != nil {
		t.Errorf("cleanup failed: %v", err)
	}

	// Verify the pool was closed
	if err := db.Ping(context.Background()); err == nil {
		t.Errorf("expected error pinging database after cleanup")
	}
}
