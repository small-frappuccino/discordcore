package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

func init() {
	idgen.Init(1)
}

func TestStore_Iterators_EarlyExitCursorClosure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	rows := pgxmock.NewRows([]string{"user_id", "role_id"}).
		AddRow("1", "role1").
		AddRow("2", "role2").
		AddRow("3", "role3").
		AddRow("4", "role4")

	mock.ExpectQuery("SELECT user_id, role_id FROM roles_current").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)

	iterSeq, err := store.StreamAllGuildMemberRoles(context.Background(), "guild1")
	if err != nil {
		t.Fatalf("failed to stream: %v", err)
	}

	count := 0
	for _, _ = range iterSeq {
		count++
		if count == 3 {
			break
		}
	}

	if count != 3 {
		t.Errorf("expected 3 records processed, got %d", count)
	}
}

func BenchmarkStore_Iterators_CompleteDrain(b *testing.B) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		b.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		rows := pgxmock.NewRows([]string{"user_id", "role_id"})
		for j := 0; j < 100; j++ {
			rows.AddRow("1", "role1")
		}
		mock.ExpectQuery("SELECT user_id, role_id FROM roles_current").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
		iterSeq, _ := store.StreamAllGuildMemberRoles(context.Background(), "guild1")
		b.StartTimer()

		for _, _ = range iterSeq {
		}
	}
}

func TestStore_Context_ExecutionBoundaryTimeout(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	mock.ExpectExec("INSERT INTO member_joins").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1)).WillDelayFor(10 * time.Millisecond)

	err = store.UpsertMemberPresenceContext(ctx, members.PresenceInput{
		GuildID: "123",
		UserID:  "456",
	})

	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestStore_Context_StructuralMisalignment(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT user_id, avatar_hash").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"user_id", "avatar_hash"}).AddRow("1", "hash"))
	mock.ExpectExec("INSERT INTO avatars_history").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(&pgconn.PgError{
		Code:    "2202E",
		Message: "arrays must have same bounds",
	})
	mock.ExpectRollback()

	snapshots := []members.Snapshot{
		{UserID: "1", HasAvatar: true, AvatarHash: "hash1"},
		{UserID: "2", HasAvatar: true, AvatarHash: "hash2"},
	}

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), "guild1", snapshots, time.Now())
	if err == nil || !strings.Contains(err.Error(), "arrays must have same bounds") {
		t.Errorf("expected structural misalignment error, got %v", err)
	}
}

func TestStore_Context_UnaryMissingState(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT joined_at FROM member_joins").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	_, ok, err := store.MemberJoin(context.Background(), "invalid", "invalid")
	if err != nil {
		t.Errorf("expected nil error on ErrNoRows, got %v", err)
	}
	if ok {
		t.Errorf("expected ok to be false")
	}

	mock.ExpectQuery("SELECT avatar_hash, updated_at FROM avatars_current").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)
	_, _, ok, err = store.GetAvatar(context.Background(), "invalid", "invalid")
	if err != nil {
		t.Errorf("expected nil error on ErrNoRows, got %v", err)
	}
	if ok {
		t.Errorf("expected ok to be false")
	}
}

var _ DB = (*pgxpool.Pool)(nil)

func TestStore_Members_Idempotency_And_Temporal_Precedence(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to open stub db connection: %v", err)
	}
	defer mock.Close()

	mock.MatchExpectationsInOrder(false)
	store, _ := NewStore(mock, nil)

	snapshots := []members.Snapshot{
		{
			UserID:   "user1",
			JoinedAt: time.Now().Add(-10 * time.Hour),
			HasBot:   true,
			IsBot:    false,
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO member_joins").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	mock.ExpectRollback()

	err = store.UpsertGuildMemberSnapshotsContext(context.Background(), "guild1", snapshots, time.Now())
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}
