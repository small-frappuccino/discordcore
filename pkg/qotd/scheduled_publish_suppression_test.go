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
	if got := suppressed.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-03" {
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

func TestSuppressScheduledPublishDateAccumulatesMultipleDates(t *testing.T) {
	t.Parallel()

	base := files.QOTDConfig{}
	one := suppressScheduledPublishDate(base, time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC))
	two := suppressScheduledPublishDate(one, time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC))

	if got := two.SuppressScheduledPublishDatesUTC; len(got) != 2 || got[0] != "2026-04-03" || got[1] != "2026-04-04" {
		t.Fatalf("expected the helper to accumulate both dates in sorted order, got %+v", got)
	}
	if !isScheduledPublishSuppressed(two, time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected first suppression to remain after adding a second, got %+v", two)
	}
	if !isScheduledPublishSuppressed(two, time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected second suppression to be active, got %+v", two)
	}

	// Re-suppressing the same date is idempotent and does not duplicate the
	// entry; this matters because manual publishes and reconcile may both
	// try to suppress the same day in quick succession.
	idempotent := suppressScheduledPublishDate(two, time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC))
	if got := idempotent.SuppressScheduledPublishDatesUTC; len(got) != 2 {
		t.Fatalf("expected duplicate suppression to be idempotent, got %+v", got)
	}
}

func TestClearSuppressedScheduledPublishDateClearsOnlyMatchingSlots(t *testing.T) {
	t.Parallel()

	current := files.QOTDConfig{SuppressScheduledPublishDatesUTC: []string{"2026-04-03", "2026-04-04"}}
	cleared := clearSuppressedScheduledPublishDate(current, time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC))
	if got := cleared.SuppressScheduledPublishDatesUTC; len(got) != 1 || got[0] != "2026-04-04" {
		t.Fatalf("expected matching slot clear to remove only that suppression, got %+v", cleared)
	}
	unchanged := clearSuppressedScheduledPublishDate(current, time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC))
	if got := unchanged.SuppressScheduledPublishDatesUTC; len(got) != 2 {
		t.Fatalf("expected non-matching slot clear to preserve suppression, got %+v", unchanged)
	}
	// Zero-time clear is a defensive no-op now that the multi-date model
	// makes "clear everything" expressible via direct slice assignment
	// (used in PrepareSettingsUpdate's ON→OFF branch).
	noop := clearSuppressedScheduledPublishDate(current, time.Time{})
	if got := noop.SuppressScheduledPublishDatesUTC; len(got) != 2 {
		t.Fatalf("expected zero-date clear to be a no-op under the multi-date model, got %+v", noop)
	}
}

func TestParseSuppressedScheduledPublishDatesParsesValidEntries(t *testing.T) {
	t.Parallel()

	dates, invalid := parseSuppressedScheduledPublishDates(files.QOTDConfig{
		SuppressScheduledPublishDatesUTC: []string{"2026-04-03", "2026-04-04"},
	})
	if len(invalid) != 0 {
		t.Fatalf("expected no invalid entries, got %+v", invalid)
	}
	wantA := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	wantB := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	if len(dates) != 2 || !dates[0].Equal(wantA) || !dates[1].Equal(wantB) {
		t.Fatalf("expected parsed dates %s and %s, got %+v", wantA, wantB, dates)
	}
}

func TestParseSuppressedScheduledPublishDatesReportsInvalidEntries(t *testing.T) {
	t.Parallel()

	dates, invalid := parseSuppressedScheduledPublishDates(files.QOTDConfig{
		SuppressScheduledPublishDatesUTC: []string{"2026-04-03", "not-a-date", ""},
	})
	wantValid := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if len(dates) != 1 || !dates[0].Equal(wantValid) {
		t.Fatalf("expected valid entry to survive, got %+v", dates)
	}
	if len(invalid) != 2 {
		t.Fatalf("expected both malformed entries to be reported, got %+v", invalid)
	}
}

// FuzzParseSuppressedScheduledPublishDates pins the parser's structural
// invariants on arbitrary bytes wrapped in a single-element slice — same
// guarantees the legacy single-string parser made: never panics, never
// returns a non-UTC time, never returns a non-day-boundary time, never
// returns both a valid date AND that date in the invalid slice.
func FuzzParseSuppressedScheduledPublishDates(f *testing.F) {
	for _, seed := range []string{
		"",
		"   ",
		"2026-04-03",
		"  2026-04-03  ",
		"not-a-date",
		"2026-04-03T12:43:00Z",
		"2026-13-40",
		"2026/04/03",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		dates, invalid := parseSuppressedScheduledPublishDates(files.QOTDConfig{
			SuppressScheduledPublishDatesUTC: []string{raw},
		})
		if len(dates)+len(invalid) > 1 {
			t.Fatalf("expected single-element input to map to one output bucket, got dates=%d invalid=%d (raw=%q)", len(dates), len(invalid), raw)
		}
		for _, parsed := range dates {
			if parsed.IsZero() {
				t.Fatalf("valid bucket must not contain zero time, raw=%q", raw)
			}
			if parsed.Location() != time.UTC {
				t.Fatalf("expected UTC parsed time, raw=%q got %s", raw, parsed.Location())
			}
			if parsed.Hour() != 0 || parsed.Minute() != 0 || parsed.Second() != 0 || parsed.Nanosecond() != 0 {
				t.Fatalf("expected normalized day boundary, raw=%q got %s", raw, parsed.Format(time.RFC3339))
			}
		}
	})
}
