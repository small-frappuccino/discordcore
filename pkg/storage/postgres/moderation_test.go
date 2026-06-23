package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/small-frappuccino/discordcore/pkg/idgen"
	"github.com/small-frappuccino/discordcore/pkg/moderation"
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

func TestStore_Moderation_CreateWarning(t *testing.T) {
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

func TestStore_Moderation_NextModerationCaseNumber_Errors(t *testing.T) {
	t.Run("empty guildID", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		_, err := store.NextModerationCaseNumber(context.Background(), "")
		if err == nil {
			t.Error("expected error on empty guildID")
		}
	})

	t.Run("db error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery("INSERT INTO moderation_cases").
			WillReturnError(errors.New("db error"))

		_, err := store.NextModerationCaseNumber(context.Background(), "guild1")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Moderation_CreateWarning_Errors(t *testing.T) {
	t.Run("missing fields", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		_, err := store.CreateModerationWarning(context.Background(), "", "", "", "", time.Now())
		if err == nil {
			t.Error("expected validation error")
		}
	})

	t.Run("begin tx error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin().WillReturnError(errors.New("begin error"))

		_, err := store.CreateModerationWarning(context.Background(), "g", "u", "m", "reason", time.Now())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("insert warning error", func(t *testing.T) {
		idgen.Init(1)
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectBegin()
		mock.ExpectQuery("INSERT INTO moderation_cases").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"last_case_number"}).AddRow(int64(1)))
		mock.ExpectQuery("INSERT INTO moderation_warnings").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(errors.New("warning error"))
		mock.ExpectRollback()

		_, err := store.CreateModerationWarning(context.Background(), "g", "u", "m", "reason", time.Now())
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func TestStore_Moderation_ListModerationWarnings(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		seq := store.ListModerationWarnings(context.Background(), "", "", 5)
		for range seq {
			t.Error("expected zero iteration")
		}
	})

	t.Run("success", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		now := time.Now()
		rows := pgxmock.NewRows([]string{"id", "guild_id", "user_id", "case_number", "moderator_id", "reason", "created_at"}).
			AddRow(int64(1), "g1", "u1", int64(10), "mod1", "spam", now)

		mock.ExpectQuery(`SELECT id, guild_id, user_id, case_number, moderator_id, reason, created_at`).
			WithArgs("g1", "u1", 5).
			WillReturnRows(rows)

		seq := store.ListModerationWarnings(context.Background(), "g1", "u1", 5)
		var list []moderation.Warning
		for w, err := range seq {
			if err != nil {
				t.Fatalf("unexpected iterator error: %v", err)
			}
			list = append(list, w)
		}

		if len(list) != 1 || list[0].Reason != "spam" {
			t.Errorf("unexpected results: %+v", list)
		}
	})

	t.Run("query error", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT id, guild_id, user_id, case_number, moderator_id, reason, created_at`).
			WillReturnError(errors.New("query error"))

		seq := store.ListModerationWarnings(context.Background(), "g1", "u1", 5)
		for _, err := range seq {
			if err == nil {
				t.Error("expected error from iterator")
			}
		}
	})
}

func TestStore_Moderation_GuildOwner(t *testing.T) {
	t.Run("empty inputs", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		err := store.SetGuildOwnerID(context.Background(), "", "")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("success Set and Get", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectExec(`INSERT INTO guild_meta`).
			WithArgs("g1", "owner1").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := store.SetGuildOwnerID(context.Background(), "g1", "owner1")
		if err != nil {
			t.Errorf("SetGuildOwnerID failed: %v", err)
		}

		ownerStr := "owner1"
		mock.ExpectQuery(`SELECT owner_id`).
			WithArgs("g1").
			WillReturnRows(pgxmock.NewRows([]string{"owner_id"}).AddRow(&ownerStr))

		owner, ok, err := store.GetGuildOwnerID(context.Background(), "g1")
		if err != nil || !ok || owner != "owner1" {
			t.Errorf("GetGuildOwnerID failed: owner=%s, ok=%t, err=%v", owner, ok, err)
		}
	})

	t.Run("owner is null", func(t *testing.T) {
		mock, _ := pgxmock.NewPool()
		defer mock.Close()
		store, _ := NewStore(mock, nil)

		mock.ExpectQuery(`SELECT owner_id`).
			WithArgs("g1").
			WillReturnRows(pgxmock.NewRows([]string{"owner_id"}).AddRow(nil))

		owner, ok, err := store.GetGuildOwnerID(context.Background(), "g1")
		if err != nil || ok || owner != "" {
			t.Errorf("expected empty owner, ok=false; got: owner=%s, ok=%t, err=%v", owner, ok, err)
		}
	})
}
