package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Init validates the database handle and ensures the migrated schema is present.
func (s *Store) Init() error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store database handle is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.ensureMemberJoinColumns(ctx); err != nil {
		return fmt.Errorf("ensure member join state columns: %w", err)
	}
	if err := validateSchema(ctx, s.db); err != nil {
		return fmt.Errorf("validate schema: %w", err)
	}
	return nil
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(rebind(query), args...)
}

func (s *Store) execContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.db.ExecContext(ctx, rebind(query), args...)
}

func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(rebind(query), args...)
}

func (s *Store) queryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.db.QueryContext(ctx, rebind(query), args...)
}

func (s *Store) queryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(rebind(query), args...)
}

func (s *Store) queryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.db.QueryRowContext(ctx, rebind(query), args...)
}

func txExec(tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	return tx.Exec(rebind(query), args...)
}

func txExecContext(ctx context.Context, tx *sql.Tx, query string, args ...any) (sql.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return tx.ExecContext(ctx, rebind(query), args...)
}

func txQueryRow(tx *sql.Tx, query string, args ...any) *sql.Row {
	return tx.QueryRow(rebind(query), args...)
}

func txQueryContext(ctx context.Context, tx *sql.Tx, query string, args ...any) (*sql.Rows, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return tx.QueryContext(ctx, rebind(query), args...)
}

// rebind converts question-mark placeholders to PostgreSQL-style numbered placeholders.
func rebind(query string) string {
	if query == "" {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 8)
	index := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			b.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func execValuesInChunks(ctx context.Context, tx *sql.Tx, prefix, suffix string, rowCount, columnCount int, appendArgs func([]any, int) []any) error {
	if rowCount == 0 {
		return nil
	}
	if columnCount <= 0 {
		return fmt.Errorf("column count must be positive")
	}
	if appendArgs == nil {
		return fmt.Errorf("append args callback is nil")
	}

	for start := 0; start < rowCount; start += storeBulkInsertMaxRows {
		end := start + storeBulkInsertMaxRows
		if end > rowCount {
			end = rowCount
		}

		var b strings.Builder
		b.Grow(len(prefix) + len(suffix) + (end-start)*(columnCount*2+4))
		b.WriteString(prefix)

		args := make([]any, 0, (end-start)*columnCount)
		for rowIndex := start; rowIndex < end; rowIndex++ {
			if rowIndex > start {
				b.WriteByte(',')
			}
			b.WriteByte('(')
			for columnIndex := 0; columnIndex < columnCount; columnIndex++ {
				if columnIndex > 0 {
					b.WriteByte(',')
				}
				b.WriteByte('?')
			}
			b.WriteByte(')')
			args = appendArgs(args, rowIndex)
		}
		b.WriteString(suffix)

		if _, err := txExecContext(ctx, tx, b.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

func execValuesContext(ctx context.Context, db *sql.DB, prefix, suffix string, rowCount, columnCount int, appendArgs func([]any, int) []any) error {
	if db == nil {
		return fmt.Errorf("database handle is nil")
	}
	if rowCount == 0 {
		return nil
	}
	if columnCount <= 0 {
		return fmt.Errorf("column count must be positive")
	}
	if appendArgs == nil {
		return fmt.Errorf("append args callback is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for start := 0; start < rowCount; start += storeBulkInsertMaxRows {
		end := start + storeBulkInsertMaxRows
		if end > rowCount {
			end = rowCount
		}

		var b strings.Builder
		b.Grow(len(prefix) + len(suffix) + (end-start)*(columnCount*2+4))
		b.WriteString(prefix)

		args := make([]any, 0, (end-start)*columnCount)
		for rowIndex := start; rowIndex < end; rowIndex++ {
			if rowIndex > start {
				b.WriteByte(',')
			}
			b.WriteByte('(')
			for columnIndex := 0; columnIndex < columnCount; columnIndex++ {
				if columnIndex > 0 {
					b.WriteByte(',')
				}
				b.WriteByte('?')
			}
			b.WriteByte(')')
			args = appendArgs(args, rowIndex)
		}
		b.WriteString(suffix)

		if _, err := db.ExecContext(ctx, rebind(b.String()), args...); err != nil {
			return err
		}
	}
	return nil
}
