package qotd

import (
	"errors"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestNormalizeQuestionMutationDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	body, status, err := normalizeQuestionMutation(QuestionMutation{Body: "  What now?  "})
	if err != nil {
		t.Fatalf("normalizeQuestionMutation(default) failed: %v", err)
	}
	if body != "What now?" || status != QuestionStatusReady {
		t.Fatalf("unexpected default mutation normalization: body=%q status=%q", body, status)
	}

	if _, _, err := normalizeQuestionMutation(QuestionMutation{Body: " ", Status: QuestionStatusReady}); !errors.Is(err, files.ErrInvalidQOTDInput) {
		t.Fatalf("expected blank body to fail with invalid qotd input, got %v", err)
	}
	if _, _, err := normalizeQuestionMutation(QuestionMutation{Body: "Question", Status: QuestionStatusUsed}); !errors.Is(err, files.ErrInvalidQOTDInput) {
		t.Fatalf("expected immutable publish-only status to fail normalization, got %v", err)
	}
}

func TestIsImmutableQuestionRecognizesPublishedReservedAndUsedStates(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		question storage.QOTDQuestionRecord
		want     bool
	}{
		{name: "ready question stays mutable", question: storage.QOTDQuestionRecord{Status: string(QuestionStatusReady)}, want: false},
		{name: "published once is immutable", question: storage.QOTDQuestionRecord{Status: string(QuestionStatusReady), PublishedOnceAt: &publishedAt}, want: true},
		{name: "scheduled date is immutable", question: storage.QOTDQuestionRecord{Status: string(QuestionStatusReady), ScheduledForDateUTC: &publishDate}, want: true},
		{name: "reserved status is immutable", question: storage.QOTDQuestionRecord{Status: string(QuestionStatusReserved)}, want: true},
		{name: "used status is immutable", question: storage.QOTDQuestionRecord{Status: string(QuestionStatusUsed)}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isImmutableQuestion(tt.question); got != tt.want {
				t.Fatalf("isImmutableQuestion() = %v, want %v for %+v", got, tt.want, tt.question)
			}
		})
	}
}

func TestNormalizeReorderInputValidatesAndPreservesRemainingQuestions(t *testing.T) {
	t.Parallel()

	questions := []storage.QOTDQuestionRecord{{ID: 10}, {ID: 20}, {ID: 30}}
	normalized, err := normalizeReorderInput(questions, []int64{30, 10})
	if err != nil {
		t.Fatalf("normalizeReorderInput() failed: %v", err)
	}
	if len(normalized) != 3 || normalized[0] != 30 || normalized[1] != 10 || normalized[2] != 20 {
		t.Fatalf("unexpected normalized reorder ids: %+v", normalized)
	}
	if _, err := normalizeReorderInput(questions, []int64{999}); !errors.Is(err, files.ErrInvalidQOTDInput) {
		t.Fatalf("expected unknown question id to fail with invalid qotd input, got %v", err)
	}
	if _, err := normalizeReorderInput(questions, []int64{10, 10}); !errors.Is(err, files.ErrInvalidQOTDInput) {
		t.Fatalf("expected duplicate question ids to fail with invalid qotd input, got %v", err)
	}
	if _, err := normalizeReorderInput(questions, nil); !errors.Is(err, files.ErrInvalidQOTDInput) {
		t.Fatalf("expected missing reorder ids to fail with invalid qotd input, got %v", err)
	}
}

func TestQuestionSelectionHelpersIgnorePublishedAndScheduledQuestions(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	questions := []storage.QOTDQuestionRecord{
		{ID: 1, Status: string(QuestionStatusReady), PublishedOnceAt: &publishedAt},
		{ID: 2, Status: string(QuestionStatusReserved), ScheduledForDateUTC: &publishDate},
		{ID: 3, Status: string(QuestionStatusReady)},
	}
	ready := firstReadyUnscheduledQuestion(questions)
	if ready == nil || ready.ID != 3 {
		t.Fatalf("expected firstReadyUnscheduledQuestion() to skip published/reserved questions, got %+v", ready)
	}
	reserved := reservedQuestionForDate(questions, time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC))
	if reserved == nil || reserved.ID != 2 {
		t.Fatalf("expected reservedQuestionForDate() to normalize slot dates, got %+v", reserved)
	}
	if reservedQuestionForDate(questions, time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)) != nil {
		t.Fatal("expected reservedQuestionForDate() to return nil for a different slot date")
	}
}
