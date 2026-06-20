package storage

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB defines the interface for PostgreSQL connection pooling.
// It is fully compatible with pgxpool.Pool and pgxmock.PgxPoolIface.
type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
	Ping(ctx context.Context) error
	Close()
}

// Store wraps a PostgreSQL database interface for durable caching and persistence.
//
// Concurrency: Safe for concurrent use by multiple goroutines.
// Lifecycle: Call Init() after creation before executing queries. Call Close() to release resources.
type Store struct {
	db     DB
	logger *slog.Logger
}

// NewStore creates a new Store using an existing SQL connection interface.
// Returns an error if the provided db is nil, avoiding runtime panics for invariant failures.
func NewStore(db DB, logger *slog.Logger) (*Store, error) {
	if db == nil {
		return nil, errors.New("storage: NewStore requires a non-nil DB interface")
	}
	return &Store{db: db, logger: logger}, nil
}

// log provides safe access to the configured logger or a default fallback.
func (s *Store) log() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// Close gracefully releases the underlying database connections.
func (s *Store) Close() error {
	s.db.Close()
	return nil
}

// internal query helpers abstract the receiver (db vs tx)

func txExecContext(ctx context.Context, tx pgx.Tx, query string, args ...any) (pgconn.CommandTag, error) {
	return tx.Exec(ctx, query, args...)
}

func txQueryContext(ctx context.Context, tx pgx.Tx, query string, args ...any) (pgx.Rows, error) {
	return tx.Query(ctx, query, args...)
}

func txQueryRowContext(ctx context.Context, tx pgx.Tx, query string, args ...any) pgx.Row {
	return tx.QueryRow(ctx, query, args...)
}
