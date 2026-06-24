package testdb_test

import (
	"context"
	"os"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestOpenIsolatedDatabase(t *testing.T) {
	t.Parallel()
	dsn, err := testdb.BaseDatabaseURLFromEnv()
	if testdb.IsDatabaseURLNotConfigured(err) {
		t.Skip("skipping test due to missing database url")
	}

	// Test with nil context to cover lines 51-53 of postgres.go
	db, cleanup, err := testdb.OpenIsolatedDatabase(nil, dsn)
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

func TestBaseDatabaseURLFromEnv_NotConfigured(t *testing.T) {
	t.Parallel()
	oldVal := os.Getenv(testdb.EnvDatabaseURL)
	os.Setenv(testdb.EnvDatabaseURL, "")
	defer os.Setenv(testdb.EnvDatabaseURL, oldVal)

	_, err := testdb.BaseDatabaseURLFromEnv()
	if !testdb.IsDatabaseURLNotConfigured(err) {
		t.Errorf("expected ErrDatabaseURLNotConfigured when environment variable is empty")
	}
}

func TestOpenIsolatedDatabase_Errors(t *testing.T) {
	t.Parallel()
	// Test empty DSN
	_, _, _, err := testdb.OpenIsolatedDatabaseWithDSN(context.Background(), "")
	if err == nil {
		t.Errorf("expected error with empty DSN")
	}

	// Test invalid DSN URL
	_, _, _, err = testdb.OpenIsolatedDatabaseWithDSN(context.Background(), "::invalid-dsn::")
	if err == nil {
		t.Errorf("expected error with invalid URL")
	}

	// Test invalid connection parameter (wrong host/port)
	_, _, _, err = testdb.OpenIsolatedDatabaseWithDSN(context.Background(), "postgres://127.0.0.1:9999/nonexistent?connect_timeout=1")
	if err == nil {
		t.Errorf("expected error with invalid connection parameters")
	}
}
