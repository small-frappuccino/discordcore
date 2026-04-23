package qotd

import (
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
