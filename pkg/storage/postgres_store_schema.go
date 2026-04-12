package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

var requiredSchemaTables = []string{
	"messages",
	"member_joins",
	"avatars_current",
	"avatars_history",
	"messages_history",
	"message_version_counters",
	"guild_meta",
	"runtime_meta",
	"moderation_cases",
	"moderation_warnings",
	"roles_current",
	"persistent_cache",
	"daily_message_metrics",
	"daily_reaction_metrics",
	"daily_member_joins",
	"daily_member_leaves",
}

var requiredSchemaColumns = map[string][]string{
	"member_joins": []string{"last_seen_at", "is_bot", "left_at"},
}

func (s *Store) ensureMemberJoinColumns(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	missingColumns, err := s.missingColumns(ctx, "member_joins", requiredSchemaColumns["member_joins"])
	if err != nil {
		return err
	}
	if len(missingColumns) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin member_joins bootstrap tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("add member_joins.last_seen_at column: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS is_bot BOOLEAN`); err != nil {
		return fmt.Errorf("add member_joins.is_bot column: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("add member_joins.left_at column: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE member_joins
		   SET last_seen_at = COALESCE(last_seen_at, joined_at)
		 WHERE last_seen_at IS NULL
	`); err != nil {
		return fmt.Errorf("backfill member_joins.last_seen_at: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_member_joins_active ON member_joins(guild_id, left_at)`); err != nil {
		return fmt.Errorf("create member_joins active index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit member_joins bootstrap: %w", err)
	}
	return nil
}

func (s *Store) missingColumns(ctx context.Context, table string, columns []string) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	missing := make([]string, 0)
	for _, column := range columns {
		var exists bool
		if err := s.db.QueryRowContext(
			ctx,
			`SELECT EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = $1
				  AND column_name = $2
			)`,
			table,
			column,
		).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check %s.%s existence: %w", table, column, err)
		}
		if !exists {
			missing = append(missing, column)
		}
	}
	return missing, nil
}

func validateSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	missing := make([]string, 0)
	for _, table := range requiredSchemaTables {
		var regclass sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT to_regclass($1)`, table).Scan(&regclass); err != nil {
			return fmt.Errorf("check table %s existence: %w", table, err)
		}
		if !regclass.Valid || strings.TrimSpace(regclass.String) == "" {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing migrated tables (%s); apply postgres migrations before initializing store", strings.Join(missing, ", "))
	}

	missingColumns := make([]string, 0)
	for table, columns := range requiredSchemaColumns {
		for _, column := range columns {
			var exists bool
			if err := db.QueryRowContext(
				ctx,
				`SELECT EXISTS (
					SELECT 1
					FROM information_schema.columns
					WHERE table_schema = current_schema()
					  AND table_name = $1
					  AND column_name = $2
				)`,
				table,
				column,
			).Scan(&exists); err != nil {
				return fmt.Errorf("check column %s.%s existence: %w", table, column, err)
			}
			if !exists {
				missingColumns = append(missingColumns, table+"."+column)
			}
		}
	}
	if len(missingColumns) > 0 {
		return fmt.Errorf("missing migrated columns (%s); apply postgres migrations before initializing store", strings.Join(missingColumns, ", "))
	}
	return nil
}
