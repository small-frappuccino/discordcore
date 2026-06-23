package postgres

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestStore_Init_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()
	mock.MatchExpectationsInOrder(false)

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Mock ensureMemberJoinColumns: columns exist
	for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", col).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
	}

	// Mock validateSchema: tables exist
	for _, table := range requiredSchemaTables {
		tblName := table
		mock.ExpectQuery(`SELECT to_regclass`).
			WithArgs(table).
			WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
	}

	// Mock validateSchema columns
	for table, columns := range requiredSchemaColumns {
		for _, col := range columns {
			mock.ExpectQuery(`SELECT data_type`).
				WithArgs(table, col.Name).
				WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow(col.DataType))
		}
	}

	// Mock resetQOTDQuestionSequenceWhenEmpty: has rows
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM qotd_questions`).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	err = store.Init()
	if err != nil {
		t.Errorf("expected Init to succeed, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_Init_MissingColumnsAndEmptyQOTD(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create pgxmock pool: %v", err)
	}
	defer mock.Close()
	mock.MatchExpectationsInOrder(false)

	store, err := NewStore(mock, nil)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Mock check: columns missing
	for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", col).
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	}

	mock.ExpectBegin()
	mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS is_bot BOOLEAN`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS left_at TIMESTAMPTZ`).
		WillReturnResult(pgxmock.NewResult("ALTER", 1))
	mock.ExpectExec(`UPDATE member_joins`).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec(`CREATE INDEX IF NOT EXISTS idx_member_joins_active`).
		WillReturnResult(pgxmock.NewResult("CREATE", 1))
	mock.ExpectCommit()
	mock.ExpectRollback()

	// Validate schema checks: tables exist
	for _, table := range requiredSchemaTables {
		tblName := table
		mock.ExpectQuery(`SELECT to_regclass`).
			WithArgs(table).
			WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
	}

	// Mock validateSchema columns
	for table, columns := range requiredSchemaColumns {
		for _, col := range columns {
			mock.ExpectQuery(`SELECT data_type`).
				WithArgs(table, col.Name).
				WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow(col.DataType))
		}
	}

	// Mock resetQOTDQuestionSequenceWhenEmpty: empty, so setval
	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM qotd_questions`).
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`SELECT setval`).
		WillReturnResult(pgxmock.NewResult("SELECT", 1))

	err = store.Init()
	if err != nil {
		t.Errorf("expected Init to succeed with migration, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStore_Init_Failures(t *testing.T) {
	t.Run("check columns error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnError(errors.New("query error"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on column check failure")
		}
		if !strings.Contains(err.Error(), "ensure member join state columns") {
			t.Errorf("expected ensure member join columns error, got: %v", err)
		}
	})

	t.Run("migration alter table error and rollback error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "is_bot").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		mock.ExpectQuery(`SELECT EXISTS`).
			WithArgs("member_joins", "left_at").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

		mock.ExpectBegin()
		mock.ExpectExec(`ALTER TABLE member_joins ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ`).
			WillReturnError(errors.New("alter table failure"))
		mock.ExpectRollback().WillReturnError(errors.New("rollback failure"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on migration failure")
		}
		if !strings.Contains(err.Error(), "alter table failure") || !strings.Contains(err.Error(), "rollback failure") {
			t.Errorf("expected combined alter table and rollback error, got: %v", err)
		}
	})

	t.Run("validate schema table missing", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: second table is missing
		for i, table := range requiredSchemaTables {
			if i == 1 {
				mock.ExpectQuery(`SELECT to_regclass`).
					WithArgs(table).
					WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(nil))
			} else {
				tblName := table
				mock.ExpectQuery(`SELECT to_regclass`).
					WithArgs(table).
					WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
			}
		}

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on missing table")
		}
		if !strings.Contains(err.Error(), "missing migrated tables") {
			t.Errorf("expected missing migrated tables error, got: %v", err)
		}
	})

	t.Run("validate schema column type mismatch", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()
		mock.MatchExpectationsInOrder(false)

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: tables exist
		for _, table := range requiredSchemaTables {
			tblName := table
			mock.ExpectQuery(`SELECT to_regclass`).
				WithArgs(table).
				WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
		}

		// validateSchema columns: first has error or mismatch
		// let's return a mismatch type
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("text")) // mismatch

		// other columns
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "is_bot").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("boolean"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "left_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "guild_id").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("text"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "updated_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on type mismatch")
		}
		if !strings.Contains(err.Error(), "type mismatch") {
			t.Errorf("expected type mismatch error, got: %v", err)
		}
	})

	t.Run("validate schema column missing", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()
		mock.MatchExpectationsInOrder(false)

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: tables exist
		for _, table := range requiredSchemaTables {
			tblName := table
			mock.ExpectQuery(`SELECT to_regclass`).
				WithArgs(table).
				WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
		}

		// validateSchema columns: first column is missing (pgx.ErrNoRows)
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "last_seen_at").
			WillReturnError(pgx.ErrNoRows)

		// other columns
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "is_bot").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("boolean"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("member_joins", "left_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "guild_id").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("text"))
		mock.ExpectQuery(`SELECT data_type`).
			WithArgs("avatars_current", "updated_at").
			WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow("timestamp with time zone"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on missing column")
		}
		if !strings.Contains(err.Error(), "missing") {
			t.Errorf("expected missing column error, got: %v", err)
		}
	})

	t.Run("reset qotd query failure", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		if err != nil {
			t.Fatalf("failed to create pgxmock pool: %v", err)
		}
		defer mock.Close()
		mock.MatchExpectationsInOrder(false)

		store, err := NewStore(mock, nil)
		if err != nil {
			t.Fatalf("failed to create store: %v", err)
		}

		// ensureMemberJoinColumns: exist
		for _, col := range []string{"last_seen_at", "is_bot", "left_at"} {
			mock.ExpectQuery(`SELECT EXISTS`).
				WithArgs("member_joins", col).
				WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		}

		// validateSchema: tables exist
		for _, table := range requiredSchemaTables {
			tblName := table
			mock.ExpectQuery(`SELECT to_regclass`).
				WithArgs(table).
				WillReturnRows(pgxmock.NewRows([]string{"to_regclass"}).AddRow(&tblName))
		}

		// validateSchema columns
		for table, columns := range requiredSchemaColumns {
			for _, col := range columns {
				mock.ExpectQuery(`SELECT data_type`).
					WithArgs(table, col.Name).
					WillReturnRows(pgxmock.NewRows([]string{"data_type"}).AddRow(col.DataType))
			}
		}

		// Mock resetQOTDQuestionSequenceWhenEmpty: query failure
		mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM qotd_questions`).
			WillReturnError(errors.New("qotd check failed"))

		err = store.Init()
		if err == nil {
			t.Fatal("expected Init to fail on QOTD check error")
		}
		if !strings.Contains(err.Error(), "reset qotd question sequence") {
			t.Errorf("expected reset qotd question sequence error, got: %v", err)
		}
	})
}
