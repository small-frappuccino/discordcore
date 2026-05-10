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

// TestSlotMaintenancePartialErrorErrorEdgeCases pins behavior the
// failure-summary test does not exercise: the no-failures branch (used when a
// partial flag is raised but no individual post failed), the unknown-date
// branch (so a partial bubbled up with a never-set PublishDateUTC does not
// silently render as "0001-01-01"), the default action label when the caller
// forgets to set Action, and the nil-receiver guards on Error/Unwrap that
// callers downstream rely on.
func TestSlotMaintenancePartialErrorErrorEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("no failures still labels the partial result", func(t *testing.T) {
		t.Parallel()
		err := &SlotMaintenancePartialError{
			Action: "clear_day",
			Result: SlotMaintenanceResult{
				PublishDateUTC:       time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC),
				OfficialPostsCleared: 2,
				QuestionsReleased:    1,
			},
		}
		message := err.Error()
		for _, snippet := range []string{"clear_day", "2026-05-07", "cleared=2", "released=1"} {
			if !strings.Contains(message, snippet) {
				t.Fatalf("expected error message %q to contain %q", message, snippet)
			}
		}
		if strings.Contains(message, "failed=") || strings.Contains(message, "post_ids=") {
			t.Fatalf("expected no failure section in the no-failures branch, got %q", message)
		}
	})

	t.Run("unknown date and missing action use safe defaults", func(t *testing.T) {
		t.Parallel()
		err := &SlotMaintenancePartialError{}
		message := err.Error()
		if !strings.Contains(message, "maintenance") {
			t.Fatalf("expected default action label %q, got %q", "maintenance", message)
		}
		if !strings.Contains(message, "unknown-date") {
			t.Fatalf("expected unknown-date label, got %q", message)
		}
		if strings.Contains(message, "0001-01-01") {
			t.Fatalf("expected zero-value publish date to render as unknown-date, got %q", message)
		}
	})

	t.Run("nil receiver returns empty error and nil unwrap", func(t *testing.T) {
		t.Parallel()
		var err *SlotMaintenancePartialError
		if got := err.Error(); got != "" {
			t.Fatalf("expected nil receiver Error() to return empty string, got %q", got)
		}
		if got := err.Unwrap(); got != nil {
			t.Fatalf("expected nil receiver Unwrap() to return nil, got %v", got)
		}
	})
}

func TestPublishNowParamsShouldConsumeAutomaticSlotDefaultsToTrue(t *testing.T) {
	t.Parallel()

	truthy := true
	falsy := false

	cases := []struct {
		name string
		in   PublishNowParams
		want bool
	}{
		{name: "nil pointer defaults to consuming the slot", in: PublishNowParams{}, want: true},
		{name: "explicit true keeps slot consumption", in: PublishNowParams{ConsumeAutomaticSlot: &truthy}, want: true},
		{name: "explicit false suppresses slot consumption", in: PublishNowParams{ConsumeAutomaticSlot: &falsy}, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.in.ShouldConsumeAutomaticSlot(); got != tc.want {
				t.Fatalf("ShouldConsumeAutomaticSlot() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestJoinInt64CSVHandlesEmptyAndMultipleValues(t *testing.T) {
	t.Parallel()

	if got := joinInt64CSV(nil); got != "" {
		t.Fatalf("expected empty string for nil slice, got %q", got)
	}
	if got := joinInt64CSV([]int64{}); got != "" {
		t.Fatalf("expected empty string for empty slice, got %q", got)
	}
	if got := joinInt64CSV([]int64{42}); got != "42" {
		t.Fatalf("expected single-element CSV %q, got %q", "42", got)
	}
	if got := joinInt64CSV([]int64{-1, 0, 1}); got != "-1,0,1" {
		t.Fatalf("expected CSV preserving negative and zero values, got %q", got)
	}
}
