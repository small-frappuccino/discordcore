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

func TestParseSuppressedScheduledPublishDateParsesValidDate(t *testing.T) {
	t.Parallel()

	parsed, ok := parseSuppressedScheduledPublishDate(files.QOTDConfig{SuppressScheduledPublishDateUTC: "2026-04-03"})
	if !ok {
		t.Fatal("expected valid suppression token to parse")
	}
	want := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) {
		t.Fatalf("expected parsed suppression date %s, got %s", want.Format(time.RFC3339), parsed.Format(time.RFC3339))
	}
}

func TestParseSuppressedScheduledPublishDateRejectsInvalidDate(t *testing.T) {
	t.Parallel()

	if parsed, ok := parseSuppressedScheduledPublishDate(files.QOTDConfig{SuppressScheduledPublishDateUTC: "not-a-date"}); ok || !parsed.IsZero() {
		t.Fatalf("expected invalid suppression token to fail parse, got parsed=%s ok=%v", parsed.Format(time.RFC3339), ok)
	}
}

// FuzzParseSuppressedScheduledPublishDate pins the parser's two structural
// invariants for arbitrary input bytes — (a) it never panics, and (b) the
// (parsed, ok) return pair stays internally consistent (ok=true ⇔ non-zero
// UTC time). Table tests cover the obvious shapes; the seed corpus runs on
// every `go test` and `-fuzz=` extends coverage to NUL bytes, partial dates,
// trailing junk, and other inputs a maintainer would not think to enumerate.
func FuzzParseSuppressedScheduledPublishDate(f *testing.F) {
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
		parsed, ok := parseSuppressedScheduledPublishDate(files.QOTDConfig{SuppressScheduledPublishDateUTC: raw})
		switch {
		case ok && parsed.IsZero():
			t.Fatalf("ok=true must imply non-zero parsed time, raw=%q", raw)
		case !ok && !parsed.IsZero():
			t.Fatalf("ok=false must imply zero parsed time, raw=%q parsed=%s", raw, parsed.Format(time.RFC3339))
		case ok && parsed.Location() != time.UTC:
			t.Fatalf("expected UTC parsed time, raw=%q got %s", raw, parsed.Location())
		case ok && (parsed.Hour() != 0 || parsed.Minute() != 0 || parsed.Second() != 0 || parsed.Nanosecond() != 0):
			t.Fatalf("expected normalized day boundary, raw=%q got %s", raw, parsed.Format(time.RFC3339))
		}
	})
}
