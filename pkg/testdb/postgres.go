package testdb

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
)

// EnvDatabaseURL defines env database url.
const EnvDatabaseURL = "DISCORDCORE_TEST_DATABASE_URL"

var dbNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)
var schemaCounter atomic.Uint64

// ErrDatabaseURLNotConfigured defines err database urlnot configured.
var ErrDatabaseURLNotConfigured = errors.New("postgres test database url not configured")

// BaseDatabaseURLFromEnv bases database urlfrom env.
func BaseDatabaseURLFromEnv() (string, error) {
	dsn := strings.TrimSpace(os.Getenv(EnvDatabaseURL))
	if dsn == "" {
		return "", fmt.Errorf("%w: %s is required for Postgres integration tests", ErrDatabaseURLNotConfigured, EnvDatabaseURL)
	}
	return dsn, nil
}

// IsDatabaseURLNotConfigured is database urlnot configured.
func IsDatabaseURLNotConfigured(err error) bool {
	return errors.Is(err, ErrDatabaseURLNotConfigured)
}

// OpenIsolatedDatabase creates a temporary schema and returns a DB handle scoped to it plus cleanup.
// Tests must set DISCORDCORE_TEST_DATABASE_URL to a writable PostgreSQL database URL.
func OpenIsolatedDatabase(ctx context.Context, baseDSN string) (*pgxpool.Pool, func() error, error) {
	db, _, cleanup, err := OpenIsolatedDatabaseWithDSN(ctx, baseDSN)
	return db, cleanup, err
}

// OpenIsolatedDatabaseWithDSN creates a temporary schema and returns a DB
// handle scoped to it, the DSN pointing at that schema, plus cleanup.
func OpenIsolatedDatabaseWithDSN(ctx context.Context, baseDSN string) (*pgxpool.Pool, string, func() error, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	baseDSN = strings.TrimSpace(baseDSN)
	if baseDSN == "" {
		return nil, "", nil, fmt.Errorf("base database url is empty")
	}

	_, err := url.Parse(baseDSN)
	if err != nil {
		return nil, "", nil, fmt.Errorf("parse base database url: %w", err)
	}

	admin, err := pgxpool.New(ctx, baseDSN)
	if err != nil {
		return nil, "", nil, fmt.Errorf("open base database connection: %w", err)
	}
	defer admin.Close()

	if err := admin.Ping(ctx); err != nil {
		return nil, "", nil, fmt.Errorf("ping base database connection: %w", err)
	}

	seq := schemaCounter.Add(1)
	schemaName := fmt.Sprintf("dc_test_%x_%x_%x", uint64(os.Getpid()), uint64(time.Now().UnixNano()), seq)
	if len(schemaName) > 63 {
		schemaName = schemaName[:63]
	}
	if !dbNamePattern.MatchString(schemaName) {
		return nil, "", nil, fmt.Errorf("generated invalid test schema name %q", schemaName)
	}

	if _, err := admin.Exec(ctx, fmt.Sprintf(`CREATE SCHEMA "%s"`, schemaName)); err != nil {
		return nil, "", nil, fmt.Errorf("create test schema %s: %w", schemaName, err)
	}

	testDSN, err := withSearchPath(baseDSN, schemaName)
	if err != nil {
		admin.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
		return nil, "", nil, fmt.Errorf("OpenIsolatedDatabaseWithDSN: %w", err)
	}

	testDB, err := pgxpool.New(ctx, testDSN)
	if err != nil {
		admin.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
		return nil, "", nil, fmt.Errorf("open test database handle for schema %s: %w", schemaName, err)
	}
	if err := testDB.Ping(ctx); err != nil {
		testDB.Close()
		admin.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
		return nil, "", nil, fmt.Errorf("ping test database handle for schema %s: %w", schemaName, err)
	}

	migrateCtx, migrateCancel := context.WithTimeout(ctx, 30*time.Second)
	defer migrateCancel()
	if err := persistence.NewPostgresMigrator(testDB).Up(migrateCtx); err != nil {
		testDB.Close()
		admin.Exec(ctx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName))
		return nil, "", nil, fmt.Errorf("apply postgres migrations for test schema %s: %w", schemaName, err)
	}

	cleanup := func() error {
		testDB.Close()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cleanupAdmin, err := pgxpool.New(cleanupCtx, baseDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "postgres test cleanup error: %v\n", err)
			return fmt.Errorf("open cleanup admin connection: %w", err)
		}
		defer cleanupAdmin.Close()

		if _, err := cleanupAdmin.Exec(cleanupCtx, fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName)); err != nil {
			fmt.Fprintf(os.Stderr, "postgres test cleanup error: %v\n", err)
			return fmt.Errorf("drop test schema %s: %w", schemaName, err)
		}
		return nil
	}

	return testDB, testDSN, cleanup, nil
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
