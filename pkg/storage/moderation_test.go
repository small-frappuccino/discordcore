package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
)

func TestStore_Moderation_NextModerationCaseNumber(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("INSERT INTO moderation_cases").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_case_number"}).AddRow(int64(2)))

	store.NextModerationCaseNumber(context.Background(), "guild1")
}

func TestStore_Moderation_GetGuildOwnerID_ErrNoRows(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectQuery("SELECT owner_id").WithArgs(pgxmock.AnyArg()).WillReturnError(pgx.ErrNoRows)

	store.GetGuildOwnerID(context.Background(), "guild1")
}

func TestStore_Moderation_CreateModerationWarning(t *testing.T) {
	idgen.Init(1)
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store, _ := NewStore(mock, nil)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO moderation_cases").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_case_number"}).AddRow(int64(1)))
	mock.ExpectQuery("INSERT INTO moderation_warnings").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"id", "created_at"}).AddRow(int64(10), time.Now()))
	mock.ExpectCommit()
	mock.ExpectRollback()

	store.CreateModerationWarning(context.Background(), "guild1", "user1", "mod1", "spam", time.Now())
}
