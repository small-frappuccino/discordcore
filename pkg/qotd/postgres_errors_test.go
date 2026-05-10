package qotd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsQOTDScheduledPublishConflictMatchesRelevantConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		constraintName string
		want           bool
	}{
		{name: "current scheduled constraint", constraintName: qotdScheduledPublishConstraint, want: true},
		{name: "legacy scheduled constraint", constraintName: qotdLegacyPublishDateConstraint, want: true},
		{name: "wrong qotd constraint", constraintName: qotdThreadArchiveConstraint, want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := fmt.Errorf("wrapped: %w", &pgconn.PgError{
				Code:           postgresUniqueViolationCode,
				ConstraintName: tt.constraintName,
			})
			if got := isQOTDScheduledPublishConflict(err); got != tt.want {
				t.Fatalf("isQOTDScheduledPublishConflict() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsQOTDThreadArchiveConflictMatchesThreadConstraint(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("wrapped: %w", &pgconn.PgError{
		Code:           postgresUniqueViolationCode,
		ConstraintName: qotdThreadArchiveConstraint,
	})
	if !isQOTDThreadArchiveConflict(err) {
		t.Fatal("expected thread archive conflict to match typed postgres error")
	}
}

func TestIsQOTDAnswerMessageConflictMatchesUniqueUserConstraint(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("wrapped: %w", &pgconn.PgError{
		Code:           postgresUniqueViolationCode,
		ConstraintName: qotdAnswerMessagesUniqueUserConstraint,
	})
	if !isQOTDAnswerMessageConflict(err) {
		t.Fatal("expected answer message conflict to match typed postgres error")
	}
}

func TestQOTDUniqueConstraintHelpersIgnoreNonMatchingErrors(t *testing.T) {
	t.Parallel()

	nonUnique := fmt.Errorf("wrapped: %w", &pgconn.PgError{
		Code:           "23503",
		ConstraintName: qotdScheduledPublishConstraint,
	})
	if isQOTDScheduledPublishConflict(nonUnique) {
		t.Fatal("expected non-unique postgres error to be ignored")
	}

	wrongConstraint := fmt.Errorf("wrapped: %w", &pgconn.PgError{
		Code:           postgresUniqueViolationCode,
		ConstraintName: "idx_other_unique_constraint",
	})
	if isQOTDAnswerMessageConflict(wrongConstraint) || isQOTDThreadArchiveConflict(wrongConstraint) {
		t.Fatal("expected unrelated unique constraint to be ignored")
	}
}

// TestQOTDUniqueConstraintHelpersHandleNilAndNonPostgresErrors guards the
// callers that wrap a generic error in fmt.Errorf and then ask "is this a
// constraint conflict?" — they must always get a clean false back, not a
// crash from the type assertion or a misleading positive on a non-postgres
// error type. The original suite only exercises pgconn.PgError inputs.
func TestQOTDUniqueConstraintHelpersHandleNilAndNonPostgresErrors(t *testing.T) {
	t.Parallel()

	classifiers := map[string]func(error) bool{
		"scheduled_publish": isQOTDScheduledPublishConflict,
		"thread_archive":    isQOTDThreadArchiveConflict,
		"answer_message":    isQOTDAnswerMessageConflict,
	}
	inputs := []struct {
		name string
		err  error
	}{
		{name: "nil error", err: nil},
		{name: "plain error", err: errors.New("dial tcp: lookup db: no such host")},
		{name: "wrapped plain error", err: fmt.Errorf("wrapped: %w", errors.New("io timeout"))},
		{name: "typed pg error pointer set to nil", err: (*pgconn.PgError)(nil)},
	}

	for label, classify := range classifiers {
		for _, in := range inputs {
			classify, in, label := classify, in, label
			t.Run(label+"/"+in.name, func(t *testing.T) {
				t.Parallel()
				if classify(in.err) {
					t.Fatalf("expected %s classifier to return false for %s, got true (err=%v)", label, in.name, in.err)
				}
			})
		}
	}
}
