package qotd

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSlotMaintenancePartialErrorUnwrapIncludesSentinelWithoutCause(t *testing.T) {
	t.Parallel()

	err := &SlotMaintenancePartialError{Action: "clear_day"}
	if !errors.Is(err, ErrSlotMaintenancePartial) {
		t.Fatal("expected partial error to unwrap to ErrSlotMaintenancePartial")
	}
}

func TestSlotMaintenancePartialErrorUnwrapIncludesCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("discord cleanup failed")
	err := &SlotMaintenancePartialError{Action: "clear_day", Cause: cause}
	if !errors.Is(err, ErrSlotMaintenancePartial) {
		t.Fatal("expected partial error to unwrap to ErrSlotMaintenancePartial")
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected partial error to unwrap to the original cause")
	}
}

func TestSlotMaintenancePartialErrorErrorIncludesFailureSummary(t *testing.T) {
	t.Parallel()

	err := &SlotMaintenancePartialError{
		Action: "clear_day",
		Result: SlotMaintenanceResult{
			PublishDateUTC:       time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
			OfficialPostsCleared: 1,
			QuestionsReleased:    1,
		},
		FailedOfficialPostIDs: []int64{11, 29},
	}

	message := err.Error()
	for _, snippet := range []string{"clear_day", "2026-05-07", "cleared=1", "released=1", "failed=2", "11,29"} {
		if !strings.Contains(message, snippet) {
			t.Fatalf("expected error message %q to contain %q", message, snippet)
		}
	}
}
