package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
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
	db         *sql.DB
	migrations []migration
}

func NewPostgresMigrator(db *sql.DB) Migrator {
	migs := append([]migration(nil), postgresMigrations...)
	sort.Slice(migs, func(i, j int) bool { return migs[i].Version < migs[j].Version })
	return &postgresMigrator{db: db, migrations: migs}
}

func (m *postgresMigrator) Up(ctx context.Context) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("postgres migrator database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.ensureVersionTable(ctx); err != nil {
		return err
	}
	current, err := m.Version(ctx)
	if err != nil {
		return err
	}

	for _, mig := range m.migrations {
		if mig.Version <= current {
			continue
		}
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration tx version %d: %w", mig.Version, err)
		}
		for _, sqlText := range mig.UpSQL {
			if _, err := tx.ExecContext(ctx, sqlText); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration version %d: %w", mig.Version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, mig.Version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration version %d: %w", mig.Version, err)
		}
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("commit migration version %d: %w", mig.Version, err)
		}
		current = mig.Version
	}

	return nil
}

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
		return err
	}
	current, err := m.Version(ctx)
	if err != nil {
		return err
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
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin rollback tx version %d: %w", mig.Version, err)
		}
		for _, sqlText := range mig.DownSQL {
			if _, err := tx.ExecContext(ctx, sqlText); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("rollback migration version %d: %w", mig.Version, err)
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, mig.Version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete migration record version %d: %w", mig.Version, err)
		}
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("commit rollback version %d: %w", mig.Version, err)
		}
		remaining--
	}
	return nil
}

func (m *postgresMigrator) Version(ctx context.Context) (int64, error) {
	if m == nil || m.db == nil {
		return 0, fmt.Errorf("postgres migrator database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.ensureVersionTable(ctx); err != nil {
		return 0, err
	}

	var version sql.NullInt64
	if err := m.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	if version.Valid {
		return version.Int64, nil
	}
	return 0, nil
}

func (m *postgresMigrator) ensureVersionTable(ctx context.Context) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("postgres migrator database handle is nil")
	}
	if _, err := m.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version    BIGINT PRIMARY KEY,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}
