//go:build !legacy
// +build !legacy

package task

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// mockDB implements storage.DB
type mockDB struct {
	execCount int
}

func (m *mockDB) Begin(ctx context.Context) (pgx.Tx, error) {
	return &mockTx{db: m}, nil
}
func (m *mockDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockDB) Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockDB) QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row {
	return nil
}
func (m *mockDB) Ping(ctx context.Context) error { return nil }
func (m *mockDB) Close()                         {}

type mockTx struct {
	db *mockDB
	pgx.Tx
}

func (tx *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if err := ctx.Err(); err != nil {
		return pgconn.CommandTag{}, err
	}
	tx.db.execCount++
	return pgconn.CommandTag{}, nil
}

type mockRows struct {
	pgx.Rows
}

func (m *mockRows) Err() error             { return nil }
func (m *mockRows) Close()                 {}
func (m *mockRows) Next() bool             { return false }
func (m *mockRows) Scan(dest ...any) error { return nil }

func (tx *mockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tx.db.execCount++
	return &mockRows{}, nil
}

type mockRow struct {
	pgx.Row
}

func (m *mockRow) Scan(dest ...any) error {
	return pgx.ErrNoRows
}

func (tx *mockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	tx.db.execCount++
	return &mockRow{}
}

type mockBatchResults struct {
	pgx.BatchResults
}

func (m *mockBatchResults) Close() error                     { return nil }
func (m *mockBatchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, nil }
func (m *mockBatchResults) Query() (pgx.Rows, error)         { return &mockRows{}, nil }
func (m *mockBatchResults) QueryRow() pgx.Row                { return &mockRow{} }

func (tx *mockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	tx.db.execCount++
	return &mockBatchResults{}
}

func (tx *mockTx) Commit(ctx context.Context) error   { return nil }
func (tx *mockTx) Rollback(ctx context.Context) error { return nil }

// TestAdapters_TransactionalFallback ensures that when the AvatarProcessor is suppressed,
// avatar updates safely execute a direct transactional fallback to Postgres storage.
func TestAdapters_TransactionalFallback(t *testing.T) {
	t.Parallel()

	db := &mockDB{}
	store, _ := storage.NewStore(db, nil)

	adapters := &NotificationAdapters{
		Router:          NewRouter(Defaults()),
		AvatarProcessor: nil, // Intentionally suppressed to trigger storage fallback
		Store:           store,
	}
	defer adapters.Router.Close()

	adapters.RegisterHandlers()

	payload := AvatarChangePayload{
		GuildID:   "123456789012345678",
		UserID:    "876543210987654321",
		Username:  "Alice",
		NewAvatar: "a_deadbeef1234567890",
	}

	// First pass: Valid context (Commit)
	ctx := context.Background()
	err := adapters.handleProcessAvatarChange(ctx, payload)
	if err != nil {
		t.Fatalf("Expected success for UpsertGuildMemberSnapshotsContext, got: %v", err)
	}

	if db.execCount == 0 {
		t.Fatalf("Expected fallback database insertion to be executed")
	}

	// Second pass: Invalid context (Rollback via simulated error)
	canceledCtx, cancelRollback := context.WithCancel(context.Background())
	cancelRollback() // Cancel immediately

	errRollback := adapters.handleProcessAvatarChange(canceledCtx, payload)
	if errRollback == nil {
		t.Fatalf("Expected context cancellation to force a database transaction rollback, got success")
	}
	if !strings.Contains(errRollback.Error(), "context canceled") {
		t.Fatalf("Expected 'context canceled' to trigger rollback, got: %v", errRollback)
	}
}
