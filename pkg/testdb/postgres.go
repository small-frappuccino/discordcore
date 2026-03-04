package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const EnvDatabaseURL = "DISCORDCORE_TEST_DATABASE_URL"

var dbNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)
var schemaCounter atomic.Uint64

func BaseDatabaseURLFromEnv() (string, error) {
	dsn := strings.TrimSpace(os.Getenv(EnvDatabaseURL))
	if dsn == "" {
		return "", fmt.Errorf("%s is required for Postgres integration tests", EnvDatabaseURL)
	}
	return dsn, nil
}

// OpenIsolatedDatabase creates a temporary schema and returns a DB handle scoped to it plus cleanup.
// Tests must set DISCORDCORE_TEST_DATABASE_URL to a writable PostgreSQL database URL.
func OpenIsolatedDatabase(ctx context.Context, baseDSN string) (*sql.DB, func() error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	baseDSN = strings.TrimSpace(baseDSN)
	if baseDSN == "" {
		return nil, nil, fmt.Errorf("base database url is empty")
	}

	_, err := url.Parse(baseDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse base database url: %w", err)
	}

	admin, err := sql.Open("pgx", baseDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("open base database connection: %w", err)
	}
	defer admin.Close()

	if err := admin.PingContext(ctx); err != nil {
		return nil, nil, fmt.Errorf("ping base database connection: %w", err)
	}

	seq := schemaCounter.Add(1)
	schemaName := fmt.Sprintf("dc_test_%x_%x_%x", uint64(os.Getpid()), uint64(time.Now().UnixNano()), seq)
	if len(schemaName) > 63 {
		schemaName = schemaName[:63]
	}
	if !dbNamePattern.MatchString(schemaName) {
		return nil, nil, fmt.Errorf("generated invalid test schema name %q", schemaName)
	}

	if _, err := admin.ExecContext(ctx, fmt.Sprintf(`CREATE SCHEMA "%s"`, schemaName)); err != nil {
		return nil, nil, fmt.Errorf("create test schema %s: %w", schemaName, err)
	}

	testDSN, err := withSearchPath(baseDSN, schemaName)
	if err != nil {
		_, _ = admin.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
		return nil, nil, err
	}

	testDB, err := sql.Open("pgx", testDSN)
	if err != nil {
		_, _ = admin.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
		return nil, nil, fmt.Errorf("open test database handle for schema %s: %w", schemaName, err)
	}
	if err := testDB.PingContext(ctx); err != nil {
		_ = testDB.Close()
		_, _ = admin.ExecContext(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
		return nil, nil, fmt.Errorf("ping test database handle for schema %s: %w", schemaName, err)
	}

	cleanup := func() error {
		_ = testDB.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cleanupAdmin, err := sql.Open("pgx", baseDSN)
		if err != nil {
			return fmt.Errorf("open cleanup admin connection: %w", err)
		}
		defer cleanupAdmin.Close()

		if _, err := cleanupAdmin.ExecContext(cleanupCtx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName)); err != nil {
			return fmt.Errorf("drop test schema %s: %w", schemaName, err)
		}
		return nil
	}

	return testDB, cleanup, nil
}

func withSearchPath(baseDSN, schemaName string) (string, error) {
	u, err := url.Parse(baseDSN)
	if err != nil {
		return "", fmt.Errorf("parse base database url: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schemaName+",public")
	u.RawQuery = q.Encode()
	return u.String(), nil
}
