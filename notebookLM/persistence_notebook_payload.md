# Domain Architecture: persistence

## Layout Topology
```text
persistence/
├── config.go
├── health.go
├── health_test.go
├── migrator.go
├── migrator_test.go
├── open.go
├── open_test.go
├── postgres_migrations.go
├── qotd_legacy_schema_repair.go
├── qotd_legacy_schema_repair_test.go
└── tracer.go
```

## Source Stream Aggregation

// === FILE: pkg/persistence/config.go ===
```go
package persistence

import (
	"fmt"
	"strings"
	"time"
)

// Config defines database connectivity and pool options.
type Config struct {
	Driver              string
	DatabaseURL         string
	MaxOpenConns        int
	MaxIdleConns        int
	ConnMaxLifetimeSecs int
	ConnMaxIdleTimeSecs int
	PingTimeoutMS       int
}

// Normalized normalizeds.
func (c Config) Normalized() Config {
	out := c
	out.Driver = strings.ToLower(strings.TrimSpace(out.Driver))
	out.DatabaseURL = strings.TrimSpace(out.DatabaseURL)
	if out.MaxOpenConns <= 0 {
		out.MaxOpenConns = 20
	}
	if out.MaxIdleConns < 0 {
		out.MaxIdleConns = 0
	}
	if out.ConnMaxLifetimeSecs <= 0 {
		out.ConnMaxLifetimeSecs = int((30 * time.Minute).Seconds())
	}
	if out.ConnMaxIdleTimeSecs <= 0 {
		out.ConnMaxIdleTimeSecs = int((5 * time.Minute).Seconds())
	}
	if out.PingTimeoutMS <= 0 {
		out.PingTimeoutMS = int((5 * time.Second).Milliseconds())
	}
	return out
}

// Validate validates.
func (c Config) Validate() error {
	n := c.Normalized()
	if n.Driver == "" {
		return fmt.Errorf("database driver is required (expected: postgres)")
	}
	if n.Driver != "postgres" {
		return fmt.Errorf("unsupported database driver %q (supported: postgres)", n.Driver)
	}
	if n.DatabaseURL == "" {
		return fmt.Errorf("database_url is required in runtime_config.database")
	}
	return nil
}

```

// === FILE: pkg/persistence/health.go ===
```go
package persistence

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Ping checks database readiness.
func Ping(ctx context.Context, db *pgxpool.Pool) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

```

// === FILE: pkg/persistence/health_test.go ===
```go
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

```

// === FILE: pkg/persistence/migrator.go ===
```go
package persistence

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrator applies and rolls back versioned SQL migrations.
type Migrator interface {
	Up(ctx context.Context) error
	Down(ctx context.Context, steps int) error
	Version(ctx context.Context) (int64, error)
}

type migration struct {
	Version int64
	UpSQL   []string
	DownSQL []string
}

type postgresMigrator struct {
	db         *pgxpool.Pool
	migrations []migration
}

// NewPostgresMigrator news postgres migrator.
func NewPostgresMigrator(db *pgxpool.Pool) Migrator {
	migs := append([]migration(nil), postgresMigrations...)
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	return &postgresMigrator{db: db, migrations: migs}
}

// Up ups.
func (m *postgresMigrator) Up(ctx context.Context) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("postgres migrator database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.ensureVersionTable(ctx); err != nil {
		return fmt.Errorf("postgresMigrator.Up: %w", err)
	}
	current, err := m.Version(ctx)
	if err != nil {
		return fmt.Errorf("postgresMigrator.Up: %w", err)
	}

	for _, mig := range m.migrations {
		if mig.Version <= current {
			continue
		}
		tx, err := m.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin migration tx version %d: %w", mig.Version, err)
		}
		for _, sqlText := range mig.UpSQL {
			if _, execErr := tx.Exec(ctx, sqlText); execErr != nil {
				retErr := fmt.Errorf("apply migration version %d: %w", mig.Version, execErr)
				if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
					return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
				}
				return retErr
			}
		}
		if _, execErr := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, mig.Version); execErr != nil {
			retErr := fmt.Errorf("record migration version %d: %w", mig.Version, execErr)
			if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
				return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
			}
			return retErr
		}
		if execErr := tx.Commit(ctx); execErr != nil {
			retErr := fmt.Errorf("commit migration version %d: %w", mig.Version, execErr)
			if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
				return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
			}
			return retErr
		}
		current = mig.Version
	}

	if err := m.repairLegacySchemas(ctx); err != nil {
		return fmt.Errorf("postgresMigrator.Up: %w", err)
	}

	return nil
}

// Down downs.
func (m *postgresMigrator) Down(ctx context.Context, steps int) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("postgres migrator database handle is nil")
	}
	if steps <= 0 {
		return fmt.Errorf("down steps must be > 0")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.ensureVersionTable(ctx); err != nil {
		return fmt.Errorf("postgresMigrator.Down: %w", err)
	}
	current, err := m.Version(ctx)
	if err != nil {
		return fmt.Errorf("postgresMigrator.Down: %w", err)
	}
	if current == 0 {
		return nil
	}

	sorted := append([]migration(nil), m.migrations...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version > sorted[j].Version })

	remaining := steps
	for _, mig := range sorted {
		if remaining == 0 || mig.Version > current {
			continue
		}
		tx, err := m.db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin rollback tx version %d: %w", mig.Version, err)
		}
		for _, sqlText := range mig.DownSQL {
			if _, execErr := tx.Exec(ctx, sqlText); execErr != nil {
				retErr := fmt.Errorf("rollback migration version %d: %w", mig.Version, execErr)
				if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
					return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
				}
				return retErr
			}
		}
		if _, execErr := tx.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mig.Version); execErr != nil {
			retErr := fmt.Errorf("delete migration record version %d: %w", mig.Version, execErr)
			if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
				return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
			}
			return retErr
		}
		if execErr := tx.Commit(ctx); execErr != nil {
			retErr := fmt.Errorf("commit rollback version %d: %w", mig.Version, execErr)
			if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
				return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
			}
			return retErr
		}
		remaining--
	}
	return nil
}

// Version versions.
func (m *postgresMigrator) Version(ctx context.Context) (int64, error) {
	if m == nil || m.db == nil {
		return 0, fmt.Errorf("postgres migrator database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.ensureVersionTable(ctx); err != nil {
		return 0, fmt.Errorf("postgresMigrator.Version: %w", err)
	}

	var version *int64
	if err := m.db.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	if version != nil {
		return *version, nil
	}
	return 0, nil
}

func (m *postgresMigrator) ensureVersionTable(ctx context.Context) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("postgres migrator database handle is nil")
	}
	if _, err := m.db.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version    BIGINT PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func (m *postgresMigrator) repairLegacySchemas(ctx context.Context) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("postgres migrator database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin legacy schema repair tx: %w", err)
	}
	if execErr := repairQOTDLegacySchema(ctx, tx); execErr != nil {
		retErr := fmt.Errorf("postgresMigrator.repairLegacySchemas: %w", execErr)
		if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
		}
		return retErr
	}
	if execErr := tx.Commit(ctx); execErr != nil {
		retErr := fmt.Errorf("commit legacy schema repair: %w", execErr)
		if rerr := tx.Rollback(context.WithoutCancel(ctx)); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			return errors.Join(retErr, fmt.Errorf("rollback failed: %w", rerr))
		}
		return retErr
	}
	return nil
}

```

// === FILE: pkg/persistence/migrator_test.go ===
```go
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

```

// === FILE: pkg/persistence/open.go ===
```go
package persistence

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// maxPostgresMessageBodyBytes is the hard ceiling enforced on every Postgres
// protocol message received from the server. The wire protocol encodes the
// upcoming message length in a 4-byte big-endian prefix; if that prefix is
// corrupted (e.g. a load balancer cuts in with an HTML 503 page during
// connect), pgx will otherwise allocate exactly that many bytes for the
// receive buffer. We have seen this manifest in production as a 1.55 GiB
// allocation request leading to `fatal error: runtime: cannot allocate
// memory`, killing the process with no recoverable error.
//
// 64 MiB is roughly two orders of magnitude above any legitimate row
// payload this codebase sends or receives — QOTD question rows, moderation
// records, embeds — so it leaves abundant headroom while turning the
// corrupted-stream scenario into a normal `error` that bubbles up through
// database/sql and is logged like any other connection failure.
const maxPostgresMessageBodyBytes = 64 * 1024 * 1024

// Open creates a pgxpool handle configured for PostgreSQL.
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	normalized := cfg.Normalized()
	if err := normalized.Validate(); err != nil {
		return nil, fmt.Errorf("Open: %w", err)
	}

	config, err := pgxpool.ParseConfig(normalized.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres connection config: %w", err)
	}

	// Compose with any pre-existing BuildFrontend (today nil, but a future pgx
	// or middleware may install one) instead of clobbering it.
	previousBuildFrontend := config.ConnConfig.BuildFrontend
	config.ConnConfig.BuildFrontend = func(r io.Reader, w io.Writer) *pgproto3.Frontend {
		var frontend *pgproto3.Frontend
		if previousBuildFrontend != nil {
			frontend = previousBuildFrontend(r, w)
		} else {
			frontend = pgproto3.NewFrontend(r, w)
		}
		frontend.SetMaxBodyLen(maxPostgresMessageBodyBytes)
		return frontend
	}

	config.MaxConns = int32(normalized.MaxOpenConns)
	config.MinConns = int32(normalized.MaxIdleConns)
	config.MaxConnLifetime = time.Duration(normalized.ConnMaxLifetimeSecs) * time.Second
	config.MaxConnIdleTime = time.Duration(normalized.ConnMaxIdleTimeSecs) * time.Second
	config.ConnConfig.Tracer = newQueryTracer()

	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}

	pingCtx := ctx
	if pingCtx == nil {
		pingCtx = context.Background()
	}
	var cancel context.CancelFunc
	pingCtx, cancel = context.WithTimeout(pingCtx, time.Duration(normalized.PingTimeoutMS)*time.Millisecond)
	defer cancel()

	if err := Ping(pingCtx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres connection: %w", err)
	}
	return db, nil
}

// Metrics is the narrow observability seam for connection pool telemetry.
type Metrics interface {
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is the JSON-friendly view of pool saturation.
type MetricsSnapshot struct {
	AcquiredConns     int32 `json:"acquired_conns"`
	IdleConns         int32 `json:"idle_conns"`
	ConstructingConns int32 `json:"constructing_conns"`
	MaxConns          int32 `json:"max_conns"`
}

// InMemoryMetrics implements Metrics by delegating to a pgxpool.Pool.
type InMemoryMetrics struct {
	pool *pgxpool.Pool
}

// NewInMemoryMetrics constructs the pool telemetry accessor.
func NewInMemoryMetrics(pool *pgxpool.Pool) *InMemoryMetrics {
	return &InMemoryMetrics{pool: pool}
}

// Snapshot returns the real-time pool metrics.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	if m == nil || m.pool == nil {
		return MetricsSnapshot{}
	}
	stat := m.pool.Stat()
	return MetricsSnapshot{
		AcquiredConns:     stat.AcquiredConns(),
		IdleConns:         stat.IdleConns(),
		ConstructingConns: stat.ConstructingConns(),
		MaxConns:          stat.MaxConns(),
	}
}

// NopMetrics implements Metrics for environments without a database pool.
type NopMetrics struct{}

// Snapshot implements Metrics.
func (NopMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{}
}

```

// === FILE: pkg/persistence/open_test.go ===
```go
package persistence_test

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
)

func TestOpen_InvalidConfig(t *testing.T) {
	t.Parallel()
	_, err := persistence.Open(context.Background(), persistence.Config{
		DatabaseURL: "",
	})
	if err == nil {
		t.Errorf("expected error on empty database URL")
	}
}

func TestOpen_InvalidDSN(t *testing.T) {
	t.Parallel()
	_, err := persistence.Open(context.Background(), persistence.Config{
		DatabaseURL: "not_a_valid_dsn://",
	})
	if err == nil {
		t.Errorf("expected error on invalid DSN format")
	}
}

```

// === FILE: pkg/persistence/postgres_migrations.go ===
```go
package persistence

var postgresMigrations = []migration{
	{
		Version: 1,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS messages (
				guild_id        TEXT NOT NULL,
				message_id      TEXT NOT NULL,
				channel_id      TEXT NOT NULL,
				author_id       TEXT NOT NULL,
				author_username TEXT,
				author_avatar   TEXT,
				content         TEXT,
				cached_at       TIMESTAMPTZ NOT NULL,
				expires_at      TIMESTAMPTZ,
				PRIMARY KEY (guild_id, message_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_messages_expires ON messages(expires_at)`,
			`CREATE TABLE IF NOT EXISTS member_joins (
				guild_id   TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				joined_at  TIMESTAMPTZ NOT NULL,
				PRIMARY KEY (guild_id, user_id)
			)`,
			`CREATE TABLE IF NOT EXISTS avatars_current (
				guild_id    TEXT NOT NULL,
				user_id     TEXT NOT NULL,
				avatar_hash TEXT NOT NULL,
				updated_at  TIMESTAMPTZ NOT NULL,
				PRIMARY KEY (guild_id, user_id)
			)`,
			`CREATE TABLE IF NOT EXISTS avatars_history (
				id         BIGSERIAL PRIMARY KEY,
				guild_id   TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				old_hash   TEXT,
				new_hash   TEXT,
				changed_at TIMESTAMPTZ NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_avatars_hist_gid_uid ON avatars_history(guild_id, user_id)`,
			`CREATE INDEX IF NOT EXISTS idx_avatars_hist_changed ON avatars_history(changed_at)`,
			`CREATE TABLE IF NOT EXISTS messages_history (
				id            BIGSERIAL PRIMARY KEY,
				guild_id      TEXT NOT NULL,
				message_id    TEXT NOT NULL,
				channel_id    TEXT NOT NULL,
				author_id     TEXT NOT NULL,
				version       INTEGER NOT NULL,
				event_type    TEXT NOT NULL,
				content       TEXT,
				attachments   INTEGER NOT NULL DEFAULT 0,
				embeds_count  INTEGER NOT NULL DEFAULT 0,
				stickers      INTEGER NOT NULL DEFAULT 0,
				created_at    TIMESTAMPTZ NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_msg_hist_gid_mid ON messages_history(guild_id, message_id)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_msg_hist_gid_mid_ver ON messages_history(guild_id, message_id, version)`,
			`CREATE TABLE IF NOT EXISTS guild_meta (
				guild_id  TEXT PRIMARY KEY,
				bot_since TIMESTAMPTZ,
				owner_id  TEXT
			)`,
			`CREATE TABLE IF NOT EXISTS runtime_meta (
				key TEXT PRIMARY KEY,
				ts  TIMESTAMPTZ NOT NULL
			)`,
			`CREATE TABLE IF NOT EXISTS moderation_cases (
				guild_id         TEXT PRIMARY KEY,
				last_case_number BIGINT NOT NULL DEFAULT 0
			)`,
			`CREATE TABLE IF NOT EXISTS roles_current (
				guild_id   TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				role_id    TEXT NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL,
				PRIMARY KEY (guild_id, user_id, role_id)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_roles_current_member ON roles_current(guild_id, user_id)`,
			`CREATE TABLE IF NOT EXISTS persistent_cache (
				cache_key  TEXT PRIMARY KEY,
				cache_type TEXT NOT NULL,
				data       TEXT NOT NULL,
				expires_at TIMESTAMPTZ NOT NULL,
				cached_at  TIMESTAMPTZ NOT NULL
			)`,
			`CREATE INDEX IF NOT EXISTS idx_persistent_cache_type ON persistent_cache(cache_type)`,
			`CREATE INDEX IF NOT EXISTS idx_persistent_cache_expires ON persistent_cache(expires_at)`,
			`CREATE TABLE IF NOT EXISTS daily_message_metrics (
				guild_id   TEXT NOT NULL,
				channel_id TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				day        DATE NOT NULL,
				count      BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, channel_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_msg_by_guild_day ON daily_message_metrics(guild_id, day)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_msg_by_channel_day ON daily_message_metrics(channel_id, day)`,
			`CREATE TABLE IF NOT EXISTS daily_reaction_metrics (
				guild_id   TEXT NOT NULL,
				channel_id TEXT NOT NULL,
				user_id    TEXT NOT NULL,
				day        DATE NOT NULL,
				count      BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, channel_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_react_by_guild_day ON daily_reaction_metrics(guild_id, day)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_react_by_channel_day ON daily_reaction_metrics(channel_id, day)`,
			`CREATE TABLE IF NOT EXISTS daily_member_joins (
				guild_id TEXT NOT NULL,
				user_id  TEXT NOT NULL,
				day      DATE NOT NULL,
				count    BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_joins_by_guild_day ON daily_member_joins(guild_id, day)`,
			`CREATE TABLE IF NOT EXISTS daily_member_leaves (
				guild_id TEXT NOT NULL,
				user_id  TEXT NOT NULL,
				day      DATE NOT NULL,
				count    BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, user_id, day)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_daily_leaves_by_guild_day ON daily_member_leaves(guild_id, day)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS daily_member_leaves`,
			`DROP TABLE IF EXISTS daily_member_joins`,
			`DROP TABLE IF EXISTS daily_reaction_metrics`,
			`DROP TABLE IF EXISTS daily_message_metrics`,
			`DROP TABLE IF EXISTS persistent_cache`,
			`DROP TABLE IF EXISTS roles_current`,
			`DROP TABLE IF EXISTS moderation_cases`,
			`DROP TABLE IF EXISTS runtime_meta`,
			`DROP TABLE IF EXISTS guild_meta`,
			`DROP TABLE IF EXISTS messages_history`,
			`DROP TABLE IF EXISTS avatars_history`,
			`DROP TABLE IF EXISTS avatars_current`,
			`DROP TABLE IF EXISTS member_joins`,
			`DROP TABLE IF EXISTS messages`,
		},
	},
	{
		Version: 2,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS bot_config_state (
				config_key TEXT PRIMARY KEY,
				config_json JSONB NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS bot_config_state`,
		},
	},
	{
		Version: 3,
		UpSQL: []string{
			`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`,
			`UPDATE member_joins
			 SET last_seen_at = COALESCE(last_seen_at, joined_at)
			 WHERE last_seen_at IS NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE member_joins DROP COLUMN IF EXISTS last_seen_at`,
		},
	},
	{
		Version: 4,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS message_version_counters (
				guild_id     TEXT NOT NULL,
				message_id   TEXT NOT NULL,
				last_version BIGINT NOT NULL DEFAULT 0,
				PRIMARY KEY (guild_id, message_id)
			)`,
			`INSERT INTO message_version_counters (guild_id, message_id, last_version)
			 SELECT guild_id, message_id, MAX(version)::BIGINT
			 FROM messages_history
			 GROUP BY guild_id, message_id
			 ON CONFLICT (guild_id, message_id) DO UPDATE
			 SET last_version = GREATEST(message_version_counters.last_version, EXCLUDED.last_version)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS message_version_counters`,
		},
	},
	{
		Version: 5,
		UpSQL: []string{
			`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS is_bot BOOLEAN`,
			`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ`,
			`CREATE INDEX IF NOT EXISTS idx_member_joins_active ON member_joins(guild_id, left_at)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_member_joins_active`,
			`ALTER TABLE member_joins DROP COLUMN IF EXISTS left_at`,
			`ALTER TABLE member_joins DROP COLUMN IF EXISTS is_bot`,
		},
	},
	{
		Version: 6,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS moderation_warnings (
				id           BIGSERIAL PRIMARY KEY,
				guild_id     TEXT NOT NULL,
				user_id      TEXT NOT NULL,
				case_number  BIGINT NOT NULL,
				moderator_id TEXT NOT NULL,
				reason       TEXT NOT NULL,
				created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_moderation_warnings_case ON moderation_warnings(guild_id, case_number)`,
			`CREATE INDEX IF NOT EXISTS idx_moderation_warnings_user ON moderation_warnings(guild_id, user_id, created_at DESC)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_moderation_warnings_user`,
			`DROP INDEX IF EXISTS idx_moderation_warnings_case`,
			`DROP TABLE IF EXISTS moderation_warnings`,
		},
	},
	{
		Version: 7,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_questions (
				id                     BIGSERIAL PRIMARY KEY,
				guild_id               TEXT NOT NULL,
				body                   TEXT NOT NULL,
				status                 TEXT NOT NULL,
				queue_position         BIGINT NOT NULL,
				created_by             TEXT,
				scheduled_for_date_utc DATE,
				used_at                TIMESTAMPTZ,
				created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_queue ON qotd_questions(guild_id, queue_position)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_schedule ON qotd_questions(guild_id, scheduled_for_date_utc) WHERE scheduled_for_date_utc IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_questions_status ON qotd_questions(guild_id, status, queue_position)`,
			`CREATE TABLE IF NOT EXISTS qotd_official_posts (
				id                         BIGSERIAL PRIMARY KEY,
				guild_id                   TEXT NOT NULL,
				question_id                BIGINT NOT NULL REFERENCES qotd_questions(id) ON DELETE RESTRICT,
				publish_date_utc           DATE NOT NULL,
				state                      TEXT NOT NULL,
				channel_id                 TEXT NOT NULL,
				discord_thread_id          TEXT,
				discord_starter_message_id TEXT,
				question_text_snapshot     TEXT NOT NULL,
				published_at               TIMESTAMPTZ,
				grace_until                TIMESTAMPTZ NOT NULL,
				archive_at                 TIMESTAMPTZ NOT NULL,
				closed_at                  TIMESTAMPTZ,
				archived_at                TIMESTAMPTZ,
				last_reconciled_at         TIMESTAMPTZ,
				created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_publish_date ON qotd_official_posts(guild_id, publish_date_utc)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_thread ON qotd_official_posts(discord_thread_id) WHERE discord_thread_id IS NOT NULL`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_official_posts_archive ON qotd_official_posts(archived_at, archive_at)`,
			`CREATE TABLE IF NOT EXISTS qotd_thread_archives (
				id                BIGSERIAL PRIMARY KEY,
				guild_id          TEXT NOT NULL,
				official_post_id  BIGINT NOT NULL REFERENCES qotd_official_posts(id) ON DELETE CASCADE,
				source_kind       TEXT NOT NULL,
				discord_thread_id TEXT NOT NULL,
				archived_at       TIMESTAMPTZ NOT NULL,
				created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_thread_archives_thread ON qotd_thread_archives(discord_thread_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_thread_archives_post ON qotd_thread_archives(official_post_id, source_kind, archived_at DESC)`,
			`CREATE TABLE IF NOT EXISTS qotd_message_archives (
				id                   BIGSERIAL PRIMARY KEY,
				thread_archive_id    BIGINT NOT NULL REFERENCES qotd_thread_archives(id) ON DELETE CASCADE,
				discord_message_id   TEXT NOT NULL,
				author_id            TEXT,
				author_name_snapshot TEXT,
				author_is_bot        BOOLEAN NOT NULL DEFAULT FALSE,
				content              TEXT,
				embeds_json          JSONB,
				attachments_json     JSONB,
				created_at           TIMESTAMPTZ NOT NULL
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_message_archives_unique_message ON qotd_message_archives(thread_archive_id, discord_message_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_message_archives_created ON qotd_message_archives(thread_archive_id, created_at ASC)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_message_archives_created`,
			`DROP INDEX IF EXISTS idx_qotd_message_archives_unique_message`,
			`DROP TABLE IF EXISTS qotd_message_archives`,
			`DROP INDEX IF EXISTS idx_qotd_thread_archives_post`,
			`DROP INDEX IF EXISTS idx_qotd_thread_archives_thread`,
			`DROP TABLE IF EXISTS qotd_thread_archives`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_archive`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_thread`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_publish_date`,
			`DROP TABLE IF EXISTS qotd_official_posts`,
			`DROP INDEX IF EXISTS idx_qotd_questions_status`,
			`DROP INDEX IF EXISTS idx_qotd_questions_schedule`,
			`DROP INDEX IF EXISTS idx_qotd_questions_queue`,
			`DROP TABLE IF EXISTS qotd_questions`,
		},
	},
	{
		Version: 8,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts ADD COLUMN IF NOT EXISTS publish_mode TEXT`,
			`UPDATE qotd_official_posts
			 SET publish_mode = 'scheduled'
			 WHERE publish_mode IS NULL OR publish_mode = ''`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN publish_mode SET DEFAULT 'scheduled'`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN publish_mode SET NOT NULL`,
			`DROP INDEX IF EXISTS idx_qotd_official_posts_publish_date`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_scheduled_publish_date
			 ON qotd_official_posts(guild_id, publish_date_utc)
			 WHERE publish_mode = 'scheduled'`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_official_posts_scheduled_publish_date`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_publish_date ON qotd_official_posts(guild_id, publish_date_utc)`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS publish_mode`,
		},
	},
	{
		Version: 9,
		UpSQL:   []string{},
		DownSQL: []string{},
	},
	{
		Version: 10,
		UpSQL: []string{
			`ALTER TABLE qotd_questions ADD COLUMN IF NOT EXISTS deck_id TEXT`,
			`UPDATE qotd_questions
			 SET deck_id = 'default'
			 WHERE deck_id IS NULL OR deck_id = ''`,
			`ALTER TABLE qotd_questions ALTER COLUMN deck_id SET DEFAULT 'default'`,
			`ALTER TABLE qotd_questions ALTER COLUMN deck_id SET NOT NULL`,
			`DROP INDEX IF EXISTS idx_qotd_questions_queue`,
			`DROP INDEX IF EXISTS idx_qotd_questions_status`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_queue
			 ON qotd_questions(guild_id, deck_id, queue_position)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_questions_status
			 ON qotd_questions(guild_id, deck_id, status, queue_position)`,
			`ALTER TABLE qotd_official_posts ADD COLUMN IF NOT EXISTS deck_id TEXT`,
			`UPDATE qotd_official_posts
			 SET deck_id = 'default'
			 WHERE deck_id IS NULL OR deck_id = ''`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_id SET DEFAULT 'default'`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_id SET NOT NULL`,
			`ALTER TABLE qotd_official_posts ADD COLUMN IF NOT EXISTS deck_name_snapshot TEXT`,
			`UPDATE qotd_official_posts
			 SET deck_name_snapshot = 'Default'
			 WHERE deck_name_snapshot IS NULL OR deck_name_snapshot = ''`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_name_snapshot SET DEFAULT 'Default'`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN deck_name_snapshot SET NOT NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS deck_name_snapshot`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS deck_id`,
			`DROP INDEX IF EXISTS idx_qotd_questions_status`,
			`DROP INDEX IF EXISTS idx_qotd_questions_queue`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_queue ON qotd_questions(guild_id, queue_position)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_questions_status ON qotd_questions(guild_id, status, queue_position)`,
			`ALTER TABLE qotd_questions DROP COLUMN IF EXISTS deck_id`,
		},
	},
	{
		Version: 11,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts
			 DROP CONSTRAINT IF EXISTS qotd_official_posts_question_id_fkey`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_official_posts
			 ADD CONSTRAINT qotd_official_posts_question_id_fkey
			 FOREIGN KEY (question_id) REFERENCES qotd_questions(id) ON DELETE RESTRICT`,
		},
	},
	{
		Version: 12,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_collected_questions (
				id                         BIGSERIAL PRIMARY KEY,
				guild_id                   TEXT NOT NULL,
				source_channel_id          TEXT NOT NULL,
				source_message_id          TEXT NOT NULL,
				source_author_id           TEXT,
				source_author_name_snapshot TEXT,
				source_created_at          TIMESTAMPTZ NOT NULL,
				embed_title                TEXT NOT NULL DEFAULT '',
				question_text              TEXT NOT NULL,
				created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_collected_questions_message
			 ON qotd_collected_questions(guild_id, source_message_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_collected_questions_recent
			 ON qotd_collected_questions(guild_id, source_created_at DESC, id DESC)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_collected_questions_recent`,
			`DROP INDEX IF EXISTS idx_qotd_collected_questions_message`,
			`DROP TABLE IF EXISTS qotd_collected_questions`,
		},
	},
	{
		Version: 13,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_forum_surfaces (
				id                     BIGSERIAL PRIMARY KEY,
				guild_id               TEXT NOT NULL,
				deck_id                TEXT NOT NULL,
				channel_id             TEXT NOT NULL,
				question_list_thread_id TEXT,
				created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_forum_surfaces_deck
			 ON qotd_forum_surfaces(guild_id, deck_id)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_forum_surfaces_thread
			 ON qotd_forum_surfaces(question_list_thread_id)
			 WHERE question_list_thread_id IS NOT NULL AND question_list_thread_id <> ''`,
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS question_list_thread_id TEXT`,
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS question_list_entry_message_id TEXT`,
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS answer_channel_id TEXT`,
			`UPDATE qotd_official_posts
			 SET answer_channel_id = COALESCE(NULLIF(discord_thread_id, ''), channel_id)
			 WHERE answer_channel_id IS NULL OR answer_channel_id = ''`,
			`UPDATE qotd_official_posts
			 SET answer_channel_id = ''
			 WHERE answer_channel_id IS NULL`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN answer_channel_id SET DEFAULT ''`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN answer_channel_id SET NOT NULL`,
			`CREATE TABLE IF NOT EXISTS qotd_answer_messages (
				id                         BIGSERIAL PRIMARY KEY,
				guild_id                   TEXT NOT NULL,
				official_post_id           BIGINT NOT NULL REFERENCES qotd_official_posts(id) ON DELETE CASCADE,
				user_id                    TEXT NOT NULL,
				state                      TEXT NOT NULL,
				answer_channel_id          TEXT NOT NULL,
				discord_message_id         TEXT,
				created_via_interaction_id TEXT,
				created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				closed_at                  TIMESTAMPTZ,
				archived_at                TIMESTAMPTZ
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_answer_messages_unique_user
			 ON qotd_answer_messages(official_post_id, user_id)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_answer_messages_message
			 ON qotd_answer_messages(discord_message_id)
			 WHERE discord_message_id IS NOT NULL AND discord_message_id <> ''`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_answer_messages_state
			 ON qotd_answer_messages(official_post_id, state, created_at)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_answer_messages_state`,
			`DROP INDEX IF EXISTS idx_qotd_answer_messages_message`,
			`DROP INDEX IF EXISTS idx_qotd_answer_messages_unique_user`,
			`DROP TABLE IF EXISTS qotd_answer_messages`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS answer_channel_id`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS question_list_entry_message_id`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS question_list_thread_id`,
			`DROP INDEX IF EXISTS idx_qotd_forum_surfaces_thread`,
			`DROP INDEX IF EXISTS idx_qotd_forum_surfaces_deck`,
			`DROP TABLE IF EXISTS qotd_forum_surfaces`,
		},
	},
	{
		Version: 14,
		UpSQL:   []string{},
		DownSQL: []string{},
	},
	{
		Version: 15,
		UpSQL: []string{
			`ALTER TABLE qotd_questions
			 ADD COLUMN IF NOT EXISTS display_id BIGINT`,
			`WITH ordered AS (
				SELECT
					id,
					ROW_NUMBER() OVER (
						PARTITION BY guild_id, deck_id
						ORDER BY queue_position ASC, id ASC
					)::BIGINT AS next_display_id
				FROM qotd_questions
			)
			UPDATE qotd_questions AS questions
			SET display_id = ordered.next_display_id
			FROM ordered
			WHERE questions.id = ordered.id`,
			`UPDATE qotd_questions
			 SET display_id = 1
			 WHERE display_id IS NULL`,
			`ALTER TABLE qotd_questions
			 ALTER COLUMN display_id SET NOT NULL`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_questions_display_id
			 ON qotd_questions(guild_id, deck_id, display_id)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_questions_display_id`,
			`ALTER TABLE qotd_questions DROP COLUMN IF EXISTS display_id`,
		},
	},
	{
		Version: 16,
		UpSQL: []string{
			`ALTER TABLE qotd_questions
			 ADD COLUMN IF NOT EXISTS published_once_at TIMESTAMPTZ`,
			`UPDATE qotd_questions
			 SET published_once_at = used_at
			 WHERE published_once_at IS NULL
			   AND used_at IS NOT NULL`,
			`UPDATE qotd_questions AS questions
			 SET published_once_at = published.published_at
			 FROM (
				SELECT question_id, MAX(published_at) AS published_at
				FROM qotd_official_posts
				WHERE published_at IS NOT NULL
				GROUP BY question_id
			 ) AS published
			 WHERE questions.id = published.question_id
			   AND questions.published_once_at IS NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_questions DROP COLUMN IF EXISTS published_once_at`,
		},
	},
	{
		Version: 17,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS consume_automatic_slot BOOLEAN`,
			`UPDATE qotd_official_posts
			 SET consume_automatic_slot = TRUE
			 WHERE consume_automatic_slot IS NULL`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN consume_automatic_slot SET DEFAULT TRUE`,
			`ALTER TABLE qotd_official_posts
			 ALTER COLUMN consume_automatic_slot SET NOT NULL`,
		},
		DownSQL: []string{
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS consume_automatic_slot`,
		},
	},
	{
		// Adds an idempotency nonce to QOTD official posts so that a crash
		// between Discord's "message accepted" response and the DB write of
		// its message ID does not produce a duplicate post on resume. The
		// nonce is sent to Discord with enforce_nonce=true; if a publish was
		// already accepted with the same nonce, Discord returns the existing
		// message instead of creating a new one.
		Version: 18,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts
			 ADD COLUMN IF NOT EXISTS nonce TEXT`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_nonce
			 ON qotd_official_posts(nonce)
			 WHERE nonce IS NOT NULL`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_official_posts_nonce`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS nonce`,
		},
	},
	{
		// publish_ordinal is the monotonically-increasing publication sequence
		// number per (guild_id, deck_id), assigned on provisioning. It is what
		// the Discord thread title displays ("Question #001"), decoupled from
		// the question's queue position so that random question selection does
		// not perturb the visible numbering. Backfill chronologically by
		// published_at (falling back to created_at) so existing rows get a
		// stable ordering and the next allocation continues from MAX+1.
		Version: 19,
		UpSQL: []string{
			`ALTER TABLE qotd_official_posts ADD COLUMN IF NOT EXISTS publish_ordinal BIGINT`,
			`WITH ordered AS (
				SELECT id,
				       ROW_NUMBER() OVER (
				         PARTITION BY guild_id, deck_id
				         ORDER BY COALESCE(published_at, created_at) ASC, id ASC
				       )::BIGINT AS next_ordinal
				FROM qotd_official_posts
				WHERE publish_ordinal IS NULL
			)
			UPDATE qotd_official_posts AS posts
			SET publish_ordinal = ordered.next_ordinal
			FROM ordered
			WHERE posts.id = ordered.id`,
			`ALTER TABLE qotd_official_posts ALTER COLUMN publish_ordinal SET NOT NULL`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_official_posts_publish_ordinal
			 ON qotd_official_posts(guild_id, deck_id, publish_ordinal)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_official_posts_publish_ordinal`,
			`ALTER TABLE qotd_official_posts DROP COLUMN IF EXISTS publish_ordinal`,
		},
	},
	{
		// The QOTD collector subsystem (Publisher.FetchChannelMessages plus the
		// CollectArchivedQuestions / RemoveDeckDuplicatesFromCollector service
		// methods and their REST routes) was removed once the dashboard stopped
		// exercising it. The qotd_collected_questions table created in
		// migration 12 had no remaining readers or writers, so drop it here.
		// The DownSQL mirrors migration 12's UpSQL exactly so a rollback puts
		// the schema back in its previous shape, even though no Go code reads
		// from it anymore.
		Version: 20,
		UpSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_collected_questions_recent`,
			`DROP INDEX IF EXISTS idx_qotd_collected_questions_message`,
			`DROP TABLE IF EXISTS qotd_collected_questions`,
		},
		DownSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_collected_questions (
				id                         BIGSERIAL PRIMARY KEY,
				guild_id                   TEXT NOT NULL,
				source_channel_id          TEXT NOT NULL,
				source_message_id          TEXT NOT NULL,
				source_author_id           TEXT,
				source_author_name_snapshot TEXT,
				source_created_at          TIMESTAMPTZ NOT NULL,
				embed_title                TEXT NOT NULL DEFAULT '',
				question_text              TEXT NOT NULL,
				created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_collected_questions_message
			 ON qotd_collected_questions(guild_id, source_message_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_collected_questions_recent
			 ON qotd_collected_questions(guild_id, source_created_at DESC, id DESC)`,
		},
	},
	{
		// The QOTD thread-archive subsystem (Publisher.FetchThreadMessages
		// plus the Service.archiveThreadMessages / ensureThreadArchive flow
		// and their storage helpers) was removed once the archive on
		// retroactive QOTD threads stopped feeding any reader. The
		// qotd_thread_archives / qotd_message_archives tables created in
		// migration 7 had no remaining writers, so drop them here. The
		// DownSQL mirrors the original CREATE statements so a rollback
		// restores the empty schema shape even though no Go code reads
		// from it anymore.
		Version: 21,
		UpSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_message_archives_created`,
			`DROP INDEX IF EXISTS idx_qotd_message_archives_unique_message`,
			`DROP TABLE IF EXISTS qotd_message_archives`,
			`DROP INDEX IF EXISTS idx_qotd_thread_archives_post`,
			`DROP INDEX IF EXISTS idx_qotd_thread_archives_thread`,
			`DROP TABLE IF EXISTS qotd_thread_archives`,
		},
		DownSQL: []string{
			`CREATE TABLE IF NOT EXISTS qotd_thread_archives (
				id                BIGSERIAL PRIMARY KEY,
				guild_id          TEXT NOT NULL,
				official_post_id  BIGINT NOT NULL REFERENCES qotd_official_posts(id) ON DELETE CASCADE,
				source_kind       TEXT NOT NULL,
				discord_thread_id TEXT NOT NULL,
				archived_at       TIMESTAMPTZ NOT NULL,
				created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_thread_archives_thread ON qotd_thread_archives(discord_thread_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_thread_archives_post ON qotd_thread_archives(official_post_id, source_kind, archived_at DESC)`,
			`CREATE TABLE IF NOT EXISTS qotd_message_archives (
				id                   BIGSERIAL PRIMARY KEY,
				thread_archive_id    BIGINT NOT NULL REFERENCES qotd_thread_archives(id) ON DELETE CASCADE,
				discord_message_id   TEXT NOT NULL,
				author_id            TEXT,
				author_name_snapshot TEXT,
				author_is_bot        BOOLEAN NOT NULL DEFAULT FALSE,
				content              TEXT,
				embeds_json          JSONB,
				attachments_json     JSONB,
				created_at           TIMESTAMPTZ NOT NULL
			)`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_qotd_message_archives_unique_message ON qotd_message_archives(thread_archive_id, discord_message_id)`,
			`CREATE INDEX IF NOT EXISTS idx_qotd_message_archives_created ON qotd_message_archives(thread_archive_id, created_at ASC)`,
		},
	},
	{
		Version: 22,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS ticket_sequences (
				guild_id TEXT PRIMARY KEY,
				last_id  BIGINT NOT NULL DEFAULT 0
			)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS ticket_sequences`,
		},
	},
	{
		Version: 23,
		UpSQL: []string{
			`ALTER TABLE persistent_cache ADD COLUMN IF NOT EXISTS guild_id TEXT`,
			`ALTER TABLE persistent_cache
			 ADD CONSTRAINT fk_persistent_cache_guild_id
			 FOREIGN KEY (guild_id) REFERENCES guild_meta(guild_id) ON DELETE CASCADE`,
			`CREATE INDEX IF NOT EXISTS idx_persistent_cache_guild_id ON persistent_cache(guild_id) WHERE guild_id IS NOT NULL`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_persistent_cache_guild_id`,
			`ALTER TABLE persistent_cache DROP CONSTRAINT IF EXISTS fk_persistent_cache_guild_id`,
			`ALTER TABLE persistent_cache DROP COLUMN IF EXISTS guild_id`,
		},
	},
	{
		Version: 24,
		UpSQL: []string{
			`ALTER TABLE roles_current ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
			`CREATE INDEX IF NOT EXISTS idx_roles_current_deleted ON roles_current(deleted_at) WHERE deleted_at IS NOT NULL`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_roles_current_deleted`,
			`ALTER TABLE roles_current DROP COLUMN IF EXISTS deleted_at`,
		},
	},
	{
		Version: 25,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS guild_configs (
				guild_id TEXT PRIMARY KEY,
				config_version BIGINT NOT NULL DEFAULT 1,
				config_json JSONB NOT NULL,
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
			`INSERT INTO guild_configs (guild_id, config_version, config_json)
			 SELECT
			 	g->>'guild_id',
			 	COALESCE((g->>'config_version')::bigint, 1),
			 	g
			 FROM bot_config_state, jsonb_array_elements(config_json->'guilds') AS g
			 ON CONFLICT (guild_id) DO NOTHING`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS guild_configs`,
		},
	},
	{
		Version: 26,
		UpSQL: []string{
			`CREATE INDEX IF NOT EXISTS idx_qotd_official_posts_pending_recovery ON qotd_official_posts(guild_id, state, updated_at) WHERE archived_at IS NULL`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_official_posts_pending_recovery`,
		},
	},
	{
		Version: 27,
		UpSQL: []string{
			`CREATE INDEX IF NOT EXISTS idx_qotd_official_posts_guild_date ON qotd_official_posts(guild_id, publish_date_utc)`,
		},
		DownSQL: []string{
			`DROP INDEX IF EXISTS idx_qotd_official_posts_guild_date`,
		},
	},
	{
		Version: 28,
		UpSQL: []string{
			`CREATE TABLE IF NOT EXISTS user_preferences (
				user_id    TEXT PRIMARY KEY,
				theme      TEXT NOT NULL DEFAULT 'system',
				timezone   TEXT NOT NULL DEFAULT 'UTC',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)`,
		},
		DownSQL: []string{
			`DROP TABLE IF EXISTS user_preferences`,
		},
	},
}

```

// === FILE: pkg/persistence/qotd_legacy_schema_repair.go ===
```go
package persistence

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func repairQOTDLegacySchema(ctx context.Context, tx pgx.Tx) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("repair qotd legacy schema: %w", err)
		}
	}()
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	hasOfficialPosts, err := tableExists(ctx, tx, "qotd_official_posts")
	if err != nil {
		return fmt.Errorf("check official posts table: %w", err)
	}
	if !hasOfficialPosts {
		return nil
	}

	if err := repairOfficialPostsChannelColumns(ctx, tx); err != nil {
		return err
	}
	if err := repairForumSurfacesChannelColumn(ctx, tx); err != nil {
		return err
	}
	// Migrate legacy reply threads before dropping the table they live in.
	if err := migrateLegacyReplyThreads(ctx, tx); err != nil {
		return err
	}
	if err := dropLegacyReplyThreadArtifacts(ctx, tx); err != nil {
		return err
	}
	if err := dropOfficialPostsLegacyColumns(ctx, tx); err != nil {
		return err
	}

	return nil
}

// repairOfficialPostsChannelColumns backfills answer_channel_id on qotd_official_posts
// from the best available legacy source, then renames a lingering forum_channel_id
// column to channel_id.
func repairOfficialPostsChannelColumns(ctx context.Context, tx pgx.Tx) error {
	hasLegacyChannelColumn, err := columnExists(ctx, tx, "qotd_official_posts", "forum_channel_id")
	if err != nil {
		return fmt.Errorf("check official posts legacy channel column: %w", err)
	}
	hasChannelID, err := columnExists(ctx, tx, "qotd_official_posts", "channel_id")
	if err != nil {
		return fmt.Errorf("check official posts channel column: %w", err)
	}
	hasDiscordThreadID, err := columnExists(ctx, tx, "qotd_official_posts", "discord_thread_id")
	if err != nil {
		return fmt.Errorf("check official posts discord thread column: %w", err)
	}
	hasAnswerChannelID, err := columnExists(ctx, tx, "qotd_official_posts", "answer_channel_id")
	if err != nil {
		return fmt.Errorf("check official posts answer channel column: %w", err)
	}
	hasResponseChannelSnapshot, err := columnExists(ctx, tx, "qotd_official_posts", "response_channel_id_snapshot")
	if err != nil {
		return fmt.Errorf("check response channel snapshot column: %w", err)
	}
	channelColumn := "channel_id"
	if !hasChannelID {
		channelColumn = "forum_channel_id"
	}
	if (hasLegacyChannelColumn || hasChannelID) && hasDiscordThreadID && hasAnswerChannelID {
		if hasResponseChannelSnapshot {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
UPDATE qotd_official_posts
SET answer_channel_id = COALESCE(
	NULLIF(discord_thread_id, ''),
	NULLIF(response_channel_id_snapshot, ''),
	NULLIF(answer_channel_id, ''),
	%s
)
`, channelColumn)); err != nil {
				return fmt.Errorf("backfill answer channel from snapshot: %w", err)
			}
		} else {
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
UPDATE qotd_official_posts
SET answer_channel_id = COALESCE(
	NULLIF(discord_thread_id, ''),
	NULLIF(answer_channel_id, ''),
	%s
)
`, channelColumn)); err != nil {
				return fmt.Errorf("backfill answer channel: %w", err)
			}
		}
	}

	if hasLegacyChannelColumn && !hasChannelID {
		if _, err := tx.Exec(ctx, `ALTER TABLE qotd_official_posts RENAME COLUMN forum_channel_id TO channel_id`); err != nil {
			return fmt.Errorf("rename official posts legacy channel column: %w", err)
		}
	}

	return nil
}

// repairForumSurfacesChannelColumn reconciles the legacy forum_channel_id column on
// qotd_forum_surfaces into channel_id, backfilling then dropping it when both columns
// exist, or renaming it when only the legacy column remains.
func repairForumSurfacesChannelColumn(ctx context.Context, tx pgx.Tx) error {
	hasForumSurfaces, err := tableExists(ctx, tx, "qotd_forum_surfaces")
	if err != nil {
		return fmt.Errorf("check qotd surfaces table: %w", err)
	}
	if !hasForumSurfaces {
		return nil
	}

	hasLegacySurfaceChannelColumn, err := columnExists(ctx, tx, "qotd_forum_surfaces", "forum_channel_id")
	if err != nil {
		return fmt.Errorf("check qotd surfaces legacy channel column: %w", err)
	}
	hasSurfaceChannelID, err := columnExists(ctx, tx, "qotd_forum_surfaces", "channel_id")
	if err != nil {
		return fmt.Errorf("check qotd surfaces channel column: %w", err)
	}
	switch {
	case hasLegacySurfaceChannelColumn && hasSurfaceChannelID:
		if _, err := tx.Exec(ctx, `
UPDATE qotd_forum_surfaces
SET channel_id = COALESCE(NULLIF(channel_id, ''), forum_channel_id)
WHERE channel_id IS NULL OR channel_id = ''
`); err != nil {
			return fmt.Errorf("backfill qotd surfaces channel column: %w", err)
		}
		if _, err := tx.Exec(ctx, `
ALTER TABLE qotd_forum_surfaces
DROP COLUMN forum_channel_id
`); err != nil {
			return fmt.Errorf("drop qotd surfaces legacy channel column: %w", err)
		}
	case hasLegacySurfaceChannelColumn:
		if _, err := tx.Exec(ctx, `ALTER TABLE qotd_forum_surfaces RENAME COLUMN forum_channel_id TO channel_id`); err != nil {
			return fmt.Errorf("rename qotd surfaces legacy channel column: %w", err)
		}
	}

	return nil
}

// migrateLegacyReplyThreads copies rows from the legacy qotd_reply_threads table into
// qotd_answer_messages when both tables exist, skipping rows that already migrated.
func migrateLegacyReplyThreads(ctx context.Context, tx pgx.Tx) error {
	hasLegacyReplyThreads, err := tableExists(ctx, tx, "qotd_reply_threads")
	if err != nil {
		return fmt.Errorf("check legacy reply threads table: %w", err)
	}
	hasAnswerMessages, err := tableExists(ctx, tx, "qotd_answer_messages")
	if err != nil {
		return fmt.Errorf("check answer messages table: %w", err)
	}
	if !hasLegacyReplyThreads || !hasAnswerMessages {
		return nil
	}

	hasLegacyReplyThreadChannel, err := columnExists(ctx, tx, "qotd_reply_threads", "forum_channel_id")
	if err != nil {
		return fmt.Errorf("check legacy reply thread forum channel column: %w", err)
	}
	hasReplyThreadChannelID, err := columnExists(ctx, tx, "qotd_reply_threads", "channel_id")
	if err != nil {
		return fmt.Errorf("check legacy reply thread channel column: %w", err)
	}

	replyThreadChannelColumn := ""
	switch {
	case hasReplyThreadChannelID:
		replyThreadChannelColumn = "channel_id"
	case hasLegacyReplyThreadChannel:
		replyThreadChannelColumn = "forum_channel_id"
	default:
		return fmt.Errorf("qotd_reply_threads missing channel column")
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
INSERT INTO qotd_answer_messages (
	guild_id,
	official_post_id,
	user_id,
	state,
	answer_channel_id,
	discord_message_id,
	created_via_interaction_id,
	created_at,
	updated_at,
	closed_at,
	archived_at
)
SELECT
	guild_id,
	official_post_id,
	user_id,
	state,
	%s,
	discord_starter_message_id,
	created_via_interaction_id,
	created_at,
	updated_at,
	closed_at,
	archived_at
FROM qotd_reply_threads
ON CONFLICT (official_post_id, user_id) DO NOTHING
`, replyThreadChannelColumn)); err != nil {
		return fmt.Errorf("migrate legacy reply threads: %w", err)
	}

	return nil
}

// dropLegacyReplyThreadArtifacts removes the legacy qotd_reply_threads indexes and table.
func dropLegacyReplyThreadArtifacts(ctx context.Context, tx pgx.Tx) error {
	for _, indexName := range []string{
		"idx_qotd_reply_threads_provisioning_recovery",
		"idx_qotd_reply_threads_state",
		"idx_qotd_reply_threads_thread",
		"idx_qotd_reply_threads_unique_user",
	} {
		if _, err := tx.Exec(ctx, fmt.Sprintf(`DROP INDEX IF EXISTS %s`, indexName)); err != nil {
			return fmt.Errorf("drop legacy reply thread index %s: %w", indexName, err)
		}
	}
	if _, err := tx.Exec(ctx, `DROP TABLE IF EXISTS qotd_reply_threads`); err != nil {
		return fmt.Errorf("drop legacy reply threads table: %w", err)
	}
	return nil
}

// dropOfficialPostsLegacyColumns drops the obsolete response_channel_id_snapshot and
// is_pinned columns from qotd_official_posts when present.
func dropOfficialPostsLegacyColumns(ctx context.Context, tx pgx.Tx) error {
	hasResponseChannelSnapshot, err := columnExists(ctx, tx, "qotd_official_posts", "response_channel_id_snapshot")
	if err != nil {
		return fmt.Errorf("check response channel snapshot column: %w", err)
	}
	if hasResponseChannelSnapshot {
		if _, err := tx.Exec(ctx, `
ALTER TABLE qotd_official_posts
DROP COLUMN response_channel_id_snapshot
`); err != nil {
			return fmt.Errorf("drop response channel snapshot column: %w", err)
		}
	}

	hasPinned, err := columnExists(ctx, tx, "qotd_official_posts", "is_pinned")
	if err != nil {
		return fmt.Errorf("check is_pinned column: %w", err)
	}
	if hasPinned {
		if _, err := tx.Exec(ctx, `
ALTER TABLE qotd_official_posts
DROP COLUMN is_pinned
`); err != nil {
			return fmt.Errorf("drop is_pinned column: %w", err)
		}
	}

	return nil
}

func tableExists(ctx context.Context, tx pgx.Tx, tableName string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.tables
	WHERE table_schema = current_schema()
	  AND table_name = $1
)`, tableName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func columnExists(ctx context.Context, tx pgx.Tx, tableName, columnName string) (bool, error) {
	var exists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = current_schema()
	  AND table_name = $1
	  AND column_name = $2
)`, tableName, columnName).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

```

// === FILE: pkg/persistence/qotd_legacy_schema_repair_test.go ===
```go
//go:build integration

package persistence_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func TestPostgresMigratorUpRepairsLegacyQOTDSchemaDrift(t *testing.T) {
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

	migrator := persistence.NewPostgresMigrator(db)
	latestVersion, err := migrator.Version(context.Background())
	if err != nil {
		t.Fatalf("read schema version before drift: %v", err)
	}
	if latestVersion == 0 {
		t.Fatal("expected migrated test schema before applying legacy drift")
	}

	if _, err := db.Exec(context.Background(), `
ALTER TABLE qotd_official_posts RENAME COLUMN channel_id TO forum_channel_id;
ALTER TABLE qotd_official_posts ADD COLUMN response_channel_id_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE qotd_official_posts ADD COLUMN is_pinned BOOLEAN NOT NULL DEFAULT FALSE;
CREATE TABLE qotd_reply_threads (
	id                         BIGSERIAL PRIMARY KEY,
	guild_id                   TEXT NOT NULL,
	official_post_id           BIGINT NOT NULL REFERENCES qotd_official_posts(id) ON DELETE CASCADE,
	user_id                    TEXT NOT NULL,
	state                      TEXT NOT NULL,
	forum_channel_id           TEXT NOT NULL,
	discord_thread_id          TEXT,
	discord_starter_message_id TEXT,
	created_via_interaction_id TEXT,
	provisioning_nonce         TEXT,
	created_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	closed_at                  TIMESTAMPTZ,
	archived_at                TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_qotd_reply_threads_unique_user ON qotd_reply_threads(official_post_id, user_id);
CREATE UNIQUE INDEX idx_qotd_reply_threads_thread ON qotd_reply_threads(discord_thread_id) WHERE discord_thread_id IS NOT NULL;
CREATE INDEX idx_qotd_reply_threads_state ON qotd_reply_threads(official_post_id, state, created_at);
CREATE INDEX idx_qotd_reply_threads_provisioning_recovery ON qotd_reply_threads(guild_id, state, updated_at) WHERE discord_thread_id IS NULL;
`); err != nil {
		t.Fatalf("recreate legacy qotd artifacts: %v", err)
	}

	var questionID int64
	if err := db.QueryRow(context.Background(), `
INSERT INTO qotd_questions (
	guild_id,
	deck_id,
	body,
	status,
	queue_position,
	display_id
) VALUES ('g1', 'default', 'Legacy question', 'used', 1, 1)
RETURNING id
`).Scan(&questionID); err != nil {
		t.Fatalf("insert question: %v", err)
	}

	// publish_ordinal must be set explicitly: migration 19 made it NOT NULL,
	// and this fixture represents a row that has already been migrated past
	// 19 but still carries other legacy drift the repair must clean up.
	var officialPostID int64
	if err := db.QueryRow(context.Background(), `
INSERT INTO qotd_official_posts (
	guild_id,
	question_id,
	publish_date_utc,
	state,
	forum_channel_id,
	discord_thread_id,
	discord_starter_message_id,
	question_text_snapshot,
	published_at,
	grace_until,
	archive_at,
	deck_id,
	deck_name_snapshot,
	publish_mode,
	publish_ordinal,
	response_channel_id_snapshot,
	is_pinned
) VALUES (
	'g1',
	$1,
	DATE '2026-04-03',
	'current',
	'forum-legacy',
	'',
	'starter-legacy',
	'Legacy question',
	NOW(),
	NOW(),
	NOW(),
	'default',
	'Default',
	'scheduled',
	1,
	'response-legacy',
	TRUE
)
RETURNING id
`, questionID).Scan(&officialPostID); err != nil {
		t.Fatalf("insert official post: %v", err)
	}

	if _, err := db.Exec(context.Background(), `
INSERT INTO qotd_reply_threads (
	guild_id,
	official_post_id,
	user_id,
	state,
	forum_channel_id,
	discord_thread_id,
	discord_starter_message_id,
	created_via_interaction_id
) VALUES (
	'g1',
	$1,
	'user-1',
	'active',
	'legacy-answer-channel',
	'',
	'legacy-answer-message',
	'interaction-1'
)
`, officialPostID); err != nil {
		t.Fatalf("insert legacy reply thread: %v", err)
	}

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("upgrade with legacy repair: %v", err)
	}

	version, err := migrator.Version(context.Background())
	if err != nil {
		t.Fatalf("read schema version after upgrade: %v", err)
	}
	if version != latestVersion {
		t.Fatalf("expected schema version %d after repair, got %d", latestVersion, version)
	}

	var answerChannelID string
	if err := db.QueryRow(context.Background(), `
SELECT answer_channel_id
FROM qotd_official_posts
WHERE id = $1
`, officialPostID).Scan(&answerChannelID); err != nil {
		t.Fatalf("read repaired official post answer channel: %v", err)
	}
	if answerChannelID != "response-legacy" {
		t.Fatalf("expected answer_channel_id to prefer legacy response snapshot, got %q", answerChannelID)
	}

	var migratedMessages int
	if err := db.QueryRow(context.Background(), `
SELECT COUNT(*)
FROM qotd_answer_messages
WHERE official_post_id = $1
  AND user_id = 'user-1'
  AND answer_channel_id = 'legacy-answer-channel'
  AND discord_message_id = 'legacy-answer-message'
`, officialPostID).Scan(&migratedMessages); err != nil {
		t.Fatalf("count migrated answer messages: %v", err)
	}
	if migratedMessages != 1 {
		t.Fatalf("expected one migrated answer message, got %d", migratedMessages)
	}

	if err := tableAbsent(db, "qotd_reply_threads"); err != nil {
		t.Fatal(err)
	}
	if err := columnAbsent(db, "qotd_official_posts", "response_channel_id_snapshot"); err != nil {
		t.Fatal(err)
	}
	if err := columnAbsent(db, "qotd_official_posts", "is_pinned"); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresMigratorUpRepairsLegacyQOTDSurfaceChannelColumn(t *testing.T) {
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

	migrator := persistence.NewPostgresMigrator(db)
	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("apply baseline schema: %v", err)
	}

	if _, err := db.Exec(context.Background(), `
INSERT INTO qotd_forum_surfaces (
	guild_id,
	deck_id,
	channel_id,
	question_list_thread_id
) VALUES (
	'g1',
	'default',
	'channel-1',
	'thread-1'
)
`); err != nil {
		t.Fatalf("insert qotd surface: %v", err)
	}

	if _, err := db.Exec(context.Background(), `
ALTER TABLE qotd_forum_surfaces
RENAME COLUMN channel_id TO forum_channel_id
`); err != nil {
		t.Fatalf("drift qotd surfaces schema: %v", err)
	}

	if err := migrator.Up(context.Background()); err != nil {
		t.Fatalf("repair drifted qotd surfaces schema: %v", err)
	}

	var repairedChannelID string
	if err := db.QueryRow(context.Background(), `
SELECT channel_id
FROM qotd_forum_surfaces
WHERE guild_id = 'g1' AND deck_id = 'default'
`).Scan(&repairedChannelID); err != nil {
		t.Fatalf("read repaired qotd surface: %v", err)
	}
	if repairedChannelID != "channel-1" {
		t.Fatalf("expected repaired qotd surface channel_id to be preserved, got %q", repairedChannelID)
	}

	if err := columnAbsent(db, "qotd_forum_surfaces", "forum_channel_id"); err != nil {
		t.Fatal(err)
	}
}

func tableAbsent(db *pgxpool.Pool, tableName string) error {
	var exists bool
	if err := db.QueryRow(context.Background(), `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.tables
	WHERE table_schema = current_schema()
	  AND table_name = $1
)`, tableName).Scan(&exists); err != nil {
		return fmt.Errorf("query table %s existence: %w", tableName, err)
	}
	if exists {
		return fmt.Errorf("expected table %s to be absent", tableName)
	}
	return nil
}

func columnAbsent(db *pgxpool.Pool, tableName, columnName string) error {
	var exists bool
	if err := db.QueryRow(context.Background(), `
SELECT EXISTS(
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = current_schema()
	  AND table_name = $1
	  AND column_name = $2
)`, tableName, columnName).Scan(&exists); err != nil {
		return fmt.Errorf("query column %s.%s existence: %w", tableName, columnName, err)
	}
	if exists {
		return fmt.Errorf("expected column %s.%s to be absent", tableName, columnName)
	}
	return nil
}

```

// === FILE: pkg/persistence/tracer.go ===
```go
package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/observability"
)

// queryTracer records query latencies and logs excessively slow or failing queries.
type queryTracer struct {
	summary *observability.Summary
}

// newQueryTracer creates a new configured queryTracer.
func newQueryTracer() *queryTracer {
	return &queryTracer{
		summary: &observability.Summary{},
	}
}

type queryTracerCtxKey struct{}

type queryTraceData struct {
	start time.Time
	sql   string
}

// TraceQueryStart attaches the start time to the context.
func (t *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryTracerCtxKey{}, queryTraceData{
		start: time.Now(),
		sql:   data.SQL,
	})
}

// TraceQueryEnd records the duration and emits warnings if necessary.
func (t *queryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	qData, ok := ctx.Value(queryTracerCtxKey{}).(queryTraceData)
	if !ok {
		return
	}
	duration := time.Since(qData.start)
	t.summary.Observe(duration)

	logger := log.DatabaseLogger()
	if duration > 500*time.Millisecond {
		logger.WarnContext(ctx, "slow database query",
			"duration_ms", duration.Milliseconds(),
			"sql", qData.sql,
			"err", data.Err,
		)
	} else if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		logger.WarnContext(ctx, "database query error",
			"duration_ms", duration.Milliseconds(),
			"sql", qData.sql,
			"err", data.Err,
		)
	}
}

```

