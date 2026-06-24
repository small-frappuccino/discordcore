package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

func TestStore_TransactionalLifecycle_CommitValidation(t *testing.T) {
	t.Parallel()
	// Validação de Commit e Ignição de Rollback Silencioso
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	guildID := "12345"
	snapshots := []members.Snapshot{
		{UserID: "1", JoinedAt: time.Now(), HasBot: true, IsBot: false},
	}
	updatedAt := time.Now()

	mock.ExpectBegin()
	// Mock the single batch query inside UpsertGuildMemberSnapshotsContext (joinRows)
	mock.ExpectExec("INSERT INTO member_joins").
		WithArgs(guildID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	mock.ExpectCommit()
	mock.ExpectRollback()

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), guildID, snapshots, updatedAt)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestStore_TransactionalLifecycle_HybridRollbackFailures(t *testing.T) {
	t.Parallel()
	// Propagação Híbrida de Falhas de Rollback
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mock.Close()

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	guildID := "12345"
	snapshots := []members.Snapshot{
		{UserID: "1", JoinedAt: time.Now(), HasBot: true, IsBot: false},
	}
	updatedAt := time.Now()

	originalErr := errors.New("foreign key constraint violation")
	rollbackErr := errors.New("network interrupt during rollback")

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO member_joins").
		WithArgs(guildID, pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(originalErr)

	mock.ExpectRollback().WillReturnError(rollbackErr)

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), guildID, snapshots, updatedAt)

	// Assert using errors.As/errors.Is
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, originalErr) {
		t.Errorf("expected error tree to contain original error: %v, got: %v", originalErr, err)
	}
	if !errors.Is(err, rollbackErr) {
		t.Errorf("expected error tree to contain rollback error: %v, got: %v", rollbackErr, err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestNewStore_NilDB(t *testing.T) {
	t.Parallel()
	s, err := NewStore(nil, nil)
	if err == nil {
		t.Error("expected error when passing nil DB, got nil")
	}
	if s != nil {
		t.Error("expected store to be nil when DB is nil")
	}
}
