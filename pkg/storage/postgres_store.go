package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Store wraps a PostgreSQL database for durable caching of messages,
// avatar hashes (current and history), guild metadata (e.g., bot_since) and member joins.
type Store struct {
	db *sql.DB
}

const storeBulkInsertMaxRows = 4000

// GuildMemberSnapshot represents the persisted snapshot for one guild member.
// Fields are opt-in so callers can batch only the parts they need to refresh.
type GuildMemberSnapshot struct {
	UserID     string
	AvatarHash string
	HasAvatar  bool
	Roles      []string
	HasRoles   bool
	JoinedAt   time.Time
	IsBot      bool
	HasBot     bool
}

type GuildMemberCurrentState struct {
	UserID     string
	JoinedAt   time.Time
	LastSeenAt time.Time
	LeftAt     time.Time
	Active     bool
	IsBot      bool
	HasBot     bool
	Roles      []string
}

type CacheEntryRecord struct {
	Key       string
	CacheType string
	Data      string
	ExpiresAt time.Time
}

type ModerationWarning struct {
	ID          int64
	GuildID     string
	UserID      string
	CaseNumber  int64
	ModeratorID string
	Reason      string
	CreatedAt   time.Time
}

// NewStore creates a new Store using an existing SQL connection. Call Init() before using it.
// Panics when db is nil: callers construct the store from a verified *sql.DB at
// startup, so a nil here is a programmer error, not a runtime condition.
func NewStore(db *sql.DB) *Store {
	if db == nil {
		panic("storage: NewStore requires a non-nil *sql.DB")
	}
	return &Store{db: db}
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
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM qotd_questions LIMIT 1)`).Scan(&hasRows); err != nil {
		return err
	}
	if hasRows {
		return nil
	}

	if _, err := s.db.ExecContext(ctx, `
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

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(query, args...)
}

func (s *Store) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.db.ExecContext(ctx, query, args...)
}

func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

func (s *Store) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

func (s *Store) queryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(query, args...)
}

func (s *Store) queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, query, args...)
}

func txExec(tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	return tx.Exec(query, args...)
}

func txExecContext(ctx context.Context, tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	return tx.ExecContext(ctx, query, args...)
}

func txQueryRow(tx *sql.Tx, query string, args ...any) *sql.Row {
	return tx.QueryRow(query, args...)
}

func txQueryContext(ctx context.Context, tx *sql.Tx, query string, args ...any) (*sql.Rows, error) {
	return tx.QueryContext(ctx, query, args...)
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}
