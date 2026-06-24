package persistence_test

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestMigrator(t *testing.T) {
	t.Parallel()
	dsn, err := testdb.BaseDatabaseURLFromEnv()
	if testdb.IsDatabaseURLNotConfigured(err) {
		t.Skip("skipping test due to missing database url")
	}

	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to open isolated database: %v", err)
	}
	defer cleanup()

	migrator := persistence.NewPostgresMigrator(db)

	version, err := migrator.Version(context.Background())
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}
	if version <= 0 {
		t.Errorf("expected positive version after Up, got %d", version)
	}

	err = migrator.Down(context.Background(), 1)
	if err != nil {
		t.Fatalf("failed to run down migration: %v", err)
	}

	newVersion, err := migrator.Version(context.Background())
	if err != nil {
		t.Fatalf("failed to get version: %v", err)
	}
	if newVersion >= version {
		t.Errorf("expected version to decrease, got %d (was %d)", newVersion, version)
	}

	err = migrator.Up(context.Background())
	if err != nil {
		t.Fatalf("failed to run up migration: %v", err)
	}
}
