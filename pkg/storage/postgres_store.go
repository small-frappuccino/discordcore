package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store wraps a PostgreSQL database for durable caching of messages,
// avatar hashes (current and history), guild metadata (e.g., bot_since) and member joins.
//
// Concurrency: Store is safe for concurrent use by multiple goroutines.
// Lifecycle: Call Init() after creation before executing any queries. Call Close() to release resources.
type Store struct {
	db *pgxpool.Pool
}

const storeBulkInsertMaxRows = 4000

// NewStore creates a new Store using an existing SQL connection. Call Init() before using it.
// Returns an error if the provided db is nil, avoiding runtime panics for invariant failures.
func NewStore(db *pgxpool.Pool) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("storage: NewStore requires a non-nil *pgxpool.Pool")
	}
	return &Store{db: db}, nil
}

// Init ensures the migrated schema is present and primes per-deployment state.
func (s *Store) Init() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.ensureMemberJoinColumns(ctx); err != nil {
		return fmt.Errorf("ensure member join state columns: %w", err)
	}
	if err := validateSchema(ctx, s.db); err != nil {
		return fmt.Errorf("validate schema: %w", err)
	}
	if err := s.resetQOTDQuestionSequenceWhenEmpty(ctx); err != nil {
		return fmt.Errorf("reset qotd question sequence: %w", err)
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

func (s *Store) exec(query string, args ...any) (pgconn.CommandTag, error) {
	return s.db.Exec(context.Background(), query, args...)
}

func (s *Store) execContext(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return s.db.Exec(ctx, query, args...)
}

func (s *Store) query(query string, args ...any) (pgx.Rows, error) {
	return s.db.Query(context.Background(), query, args...)
}

func (s *Store) queryContext(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return s.db.Query(ctx, query, args...)
}

func (s *Store) queryRow(query string, args ...any) pgx.Row {
	return s.db.QueryRow(context.Background(), query, args...)
}

func (s *Store) queryRowContext(ctx context.Context, query string, args ...any) pgx.Row {
	return s.db.QueryRow(ctx, query, args...)
}

func txExec(tx pgx.Tx, query string, args ...any) (pgconn.CommandTag, error) {
	return tx.Exec(context.Background(), query, args...)
}

func txExecContext(ctx context.Context, tx pgx.Tx, query string, args ...any) (pgconn.CommandTag, error) {
	return tx.Exec(ctx, query, args...)
}

func txQueryRow(tx pgx.Tx, query string, args ...any) pgx.Row {
	return tx.QueryRow(context.Background(), query, args...)
}

func txQueryRowContext(ctx context.Context, tx pgx.Tx, query string, args ...any) pgx.Row {
	return tx.QueryRow(ctx, query, args...)
}

func txQueryContext(ctx context.Context, tx pgx.Tx, query string, args ...any) (pgx.Rows, error) {
	return tx.Query(ctx, query, args...)
}

// Close closes the underlying database.
func (s *Store) Close() error {
	s.db.Close()
	return nil
}
