package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
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
	"daily_member_leaves",
	"ticket_sequences",
	"guild_configs",
	"user_preferences",
	"qotd_questions", // included since we need it in reset
}

// ColumnDef represents an expected schema column.
type ColumnDef struct {
	Name     string
	DataType string // e.g., "text", "timestamp with time zone", "boolean"
}

// requiredSchemaColumns maps tables to their expected columns and types to detect parametric deletion and type regressions.
var requiredSchemaColumns = map[string][]ColumnDef{
	"member_joins": {
		{Name: "last_seen_at", DataType: "timestamp with time zone"},
		{Name: "is_bot", DataType: "boolean"},
		{Name: "left_at", DataType: "timestamp with time zone"},
	},
	"avatars_current": {
		{Name: "guild_id", DataType: "text"},
		{Name: "updated_at", DataType: "timestamp with time zone"},
	},
}

// Init ensures the migrated schema is present and primes per-deployment state.
func (s *Store) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.log().Debug("Storage subsystem initializing: verifying schema and priming deployment state")

	if err := s.ensureMemberJoinColumns(ctx); err != nil {
		return fmt.Errorf("ensure member join state columns: %w", err)
	}
	if err := s.validateSchema(ctx); err != nil {
		return fmt.Errorf("validate schema: %w", err)
	}
	if err := s.resetQOTDQuestionSequenceWhenEmpty(ctx); err != nil {
		return fmt.Errorf("reset qotd question sequence: %w", err)
	}

	s.log().Debug("Storage subsystem initialized successfully")
	return nil
}

func (s *Store) ensureMemberJoinColumns(ctx context.Context) (err error) {
	missing, err := s.missingColumns(ctx, "member_joins", []string{"last_seen_at", "is_bot", "left_at"})
	if err != nil {
		return fmt.Errorf("Store.ensureMemberJoinColumns: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin member_joins bootstrap tx: %w", err)
	}
	defer func() {
		// Intercept rollback explicitly evaluating tx closed errors
		if rerr := tx.Rollback(ctx); rerr != nil && !errors.Is(rerr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback failed: %w", rerr))
		}
	}()

	if _, err := tx.Exec(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("add member_joins.last_seen_at column: %w", err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS is_bot BOOLEAN`); err != nil {
		return fmt.Errorf("add member_joins.is_bot column: %w", err)
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("add member_joins.left_at column: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE member_joins
		   SET last_seen_at = COALESCE(last_seen_at, joined_at)
		 WHERE last_seen_at IS NULL
	`); err != nil {
		return fmt.Errorf("backfill member_joins.last_seen_at: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE INDEX IF NOT EXISTS idx_member_joins_active ON member_joins(guild_id, left_at)`); err != nil {
		return fmt.Errorf("create member_joins active index: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit member_joins bootstrap: %w", err)
	}
	return nil
}

func (s *Store) missingColumns(ctx context.Context, table string, columns []string) ([]string, error) {
	missing := make([]string, 0)
	for _, column := range columns {
		var exists bool
		if err := s.db.QueryRow(
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

// validateSchema checks table presence, and strongly types columns against regressions.
func (s *Store) validateSchema(ctx context.Context) error {
	missingTables := make([]string, 0)
	for _, table := range requiredSchemaTables {
		var regclass *string
		if err := s.db.QueryRow(ctx, `SELECT to_regclass($1)`, table).Scan(&regclass); err != nil {
			return fmt.Errorf("check table %s existence: %w", table, err)
		}
		if regclass == nil || strings.TrimSpace(*regclass) == "" {
			missingTables = append(missingTables, table)
		}
	}
	if len(missingTables) > 0 {
		return fmt.Errorf("missing migrated tables (%s); apply postgres migrations before initializing store", strings.Join(missingTables, ", "))
	}

	var schemaErrors []string
	for table, columns := range requiredSchemaColumns {
		for _, col := range columns {
			var dataType string
			err := s.db.QueryRow(
				ctx,
				`SELECT data_type
				 FROM information_schema.columns
				 WHERE table_schema = current_schema()
				   AND table_name = $1
				   AND column_name = $2`,
				table,
				col.Name,
			).Scan(&dataType)

			if err != nil {
				if err == pgx.ErrNoRows {
					schemaErrors = append(schemaErrors, fmt.Sprintf("column %s.%s missing", table, col.Name))
					continue
				}
				return fmt.Errorf("check column %s.%s existence: %w", table, col.Name, err)
			}

			if dataType != col.DataType {
				schemaErrors = append(schemaErrors, fmt.Sprintf("column %s.%s type mismatch: expected %s, got %s", table, col.Name, col.DataType, dataType))
			}
		}
	}

	if len(schemaErrors) > 0 {
		return fmt.Errorf("schema validation failed: %s", strings.Join(schemaErrors, "; "))
	}
	return nil
}

func (s *Store) resetQOTDQuestionSequenceWhenEmpty(ctx context.Context) error {
	var hasRows bool
	if err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM qotd_questions LIMIT 1)`).Scan(&hasRows); err != nil {
		return fmt.Errorf("Store.resetQOTDQuestionSequenceWhenEmpty: %w", err)
	}
	if hasRows {
		return nil
	}

	if _, err := s.db.Exec(ctx, `
		SELECT setval(
			pg_get_serial_sequence(format('%I.%I', current_schema(), 'qotd_questions'), 'id'),
			1,
			false
		)
	`); err != nil {
		return err
	}
	return nil
}
