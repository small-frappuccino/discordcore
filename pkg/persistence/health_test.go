package persistence_test

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestPing(t *testing.T) {
	t.Parallel()
	if err := persistence.Ping(context.Background(), nil); err == nil {
		t.Errorf("expected error when pinging nil database handle")
	}

	dsn, err := testdb.BaseDatabaseURLFromEnv()
	if testdb.IsDatabaseURLNotConfigured(err) {
		t.Skip("skipping test due to missing database url")
	}

	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), dsn)
	if err != nil {
		t.Fatalf("failed to open isolated database: %v", err)
	}
	defer cleanup()

	if err := persistence.Ping(context.Background(), db); err != nil {
		t.Errorf("expected nil error on valid ping, got: %v", err)
	}

	db.Close()
	if err := persistence.Ping(context.Background(), db); err == nil {
		t.Errorf("expected error when pinging closed database handle")
	}
}
