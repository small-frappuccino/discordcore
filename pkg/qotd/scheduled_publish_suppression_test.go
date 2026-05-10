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

// TestParseSuppressedScheduledPublishDateHandlesEmptyAndWhitespace pins the
// "no suppression configured" outcome explicitly. Earlier tests only cover
// valid + malformed tokens, which leaves the most common empty-string and
// trim-only-whitespace cases dependent on incidental control flow rather than
// an asserted contract.
func TestParseSuppressedScheduledPublishDateHandlesEmptyAndWhitespace(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"", "   ", "\t\n"} {
		raw := raw
		t.Run("raw="+raw, func(t *testing.T) {
			t.Parallel()
			parsed, ok := parseSuppressedScheduledPublishDate(files.QOTDConfig{SuppressScheduledPublishDateUTC: raw})
			if ok || !parsed.IsZero() {
				t.Fatalf("expected blank suppression token to be treated as absent, got parsed=%s ok=%v", parsed.Format(time.RFC3339), ok)
			}
		})
	}
}

// TestSuppressionHelpersTreatZeroDateAsClearIntent guards the contract the
// service relies on when ResetDeckState (or similar flows) wants to wipe an
// existing suppression unconditionally — they call the helpers with
// time.Time{} and expect the suppression token to be cleared. Without this,
// a regression that returned the original config unchanged would leave a
// stale suppression token blocking the next eligible publish.
func TestSuppressionHelpersTreatZeroDateAsClearIntent(t *testing.T) {
	t.Parallel()

	starting := files.QOTDConfig{SuppressScheduledPublishDateUTC: "2026-04-03"}

	if got := suppressScheduledPublishDate(starting, time.Time{}); got.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected zero suppression input to clear the token, got %+v", got)
	}
	if got := clearSuppressedScheduledPublishDate(starting, time.Time{}); got.SuppressScheduledPublishDateUTC != "" {
		t.Fatalf("expected zero clear input to wipe the token, got %+v", got)
	}
	if isScheduledPublishSuppressed(starting, time.Time{}) {
		t.Fatal("expected zero candidate date to never match an existing suppression")
	}
}
