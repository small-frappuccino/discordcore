package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestScheduledPublishSuppressionHelpersMatchNormalizedSlotDates(t *testing.T) {
	t.Parallel()

	base := files.QOTDConfig{}
	suppressed := suppressScheduledPublishDate(base, time.Date(2026, 4, 3, 13, 5, 0, 0, time.UTC))
	if suppressed.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected helper to persist a date-only suppression token, got %+v", suppressed)
	}
	if !isScheduledPublishSuppressed(suppressed, time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected matching slot suppression, got %+v", suppressed)
	}
	if !isScheduledPublishSuppressed(suppressed, time.Date(2026, 4, 3, 23, 59, 0, 0, time.UTC)) {
		t.Fatalf("expected slot suppression to ignore the wall-clock time, got %+v", suppressed)
	}
	if isScheduledPublishSuppressed(suppressed, time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected suppression to stop at the configured slot date, got %+v", suppressed)
	}
}

func TestClearSuppressedScheduledPublishDateClearsOnlyMatchingSlots(t *testing.T) {
	t.Parallel()

	current := files.QOTDConfig{SuppressScheduledPublishDateUTC: "2026-04-03"}
	cleared := clearSuppressedScheduledPublishDate(current, time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC))
	if cleared.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected matching slot clear to remove suppression, got %+v", cleared)
	}
	unchanged := clearSuppressedScheduledPublishDate(current, time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC))
	if unchanged.SuppressScheduledPublishDateUTC != "2026-04-03" {
		t.Fatalf("expected non-matching slot clear to preserve suppression, got %+v", unchanged)
	}
	fullyCleared := clearSuppressedScheduledPublishDate(current, time.Time{})
	if fullyCleared.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected zero-date clear to remove suppression unconditionally, got %+v", fullyCleared)
	}
}
