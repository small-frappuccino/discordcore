package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func testSchedule(t *testing.T, hour, minute int) PublishSchedule {
	t.Helper()
	hourUTC := hour
	minuteUTC := minute
	schedule, err := resolvePublishSchedule(files.QOTDConfig{
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		},
	})
	if err != nil {
		t.Fatalf("resolvePublishSchedule() failed: %v", err)
	}
	return schedule
}

func TestCurrentPublishDateUTCBeforeDailyBoundaryUsesToday(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	now := time.Date(2026, 4, 3, 12, 42, 59, 0, time.UTC)
	got := CurrentPublishDateUTC(schedule, now)
	want := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected current publish date %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestDuePublishDateUTCBeforeDailyBoundaryUsesPreviousDay(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	now := time.Date(2026, 4, 3, 12, 42, 59, 0, time.UTC)
	got := DuePublishDateUTC(schedule, now)
	want := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected due publish date %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestCurrentPublishDateUTCExactBoundaryUsesToday(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	now := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	got := CurrentPublishDateUTC(schedule, now)
	want := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected current publish date %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestCurrentPublishDateUTCAfterBoundaryUsesTomorrow(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	now := time.Date(2026, 4, 3, 12, 43, 1, 0, time.UTC)
	got := CurrentPublishDateUTC(schedule, now)
	want := time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected current publish date %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestDuePublishDateUTCExactBoundaryUsesToday(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	now := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	got := DuePublishDateUTC(schedule, now)
	want := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected due publish date %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestDuePublishDateUTCAfterBoundaryUsesToday(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	now := time.Date(2026, 4, 3, 12, 43, 1, 0, time.UTC)
	got := DuePublishDateUTC(schedule, now)
	want := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected due publish date %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestEvaluateOfficialPostTransitionsCurrentPreviousArchived(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	current := EvaluateOfficialPost(schedule, publishDate, time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC))
	if current.State != OfficialPostStateCurrent || !current.AnswerWindow.IsOpen {
		t.Fatalf("expected current/open lifecycle, got %+v", current)
	}

	previous := EvaluateOfficialPost(schedule, publishDate, time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC))
	if previous.State != OfficialPostStatePrevious || !previous.AnswerWindow.IsOpen {
		t.Fatalf("expected previous/open lifecycle, got %+v", previous)
	}

	archived := EvaluateOfficialPost(schedule, publishDate, time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC))
	if archived.State != OfficialPostStateArchived || archived.AnswerWindow.IsOpen {
		t.Fatalf("expected archived/closed lifecycle, got %+v", archived)
	}
}

func TestShouldArchiveHonorsExistingArchivedTimestamp(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	archivedAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	if ShouldArchive(schedule, publishDate, time.Date(2026, 4, 6, 12, 43, 0, 0, time.UTC), &archivedAt) {
		t.Fatal("expected already-archived post to skip archive work")
	}
}

func TestEvaluateManualOfficialPostUsesIndependentWindow(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, 4, 3, 9, 15, 0, 0, time.UTC)

	current := EvaluateManualOfficialPost(publishedAt, time.Date(2026, 4, 4, 9, 14, 59, 0, time.UTC))
	if current.State != OfficialPostStateCurrent || !current.AnswerWindow.IsOpen {
		t.Fatalf("expected manual lifecycle to stay current for the first 24 hours, got %+v", current)
	}

	previous := EvaluateManualOfficialPost(publishedAt, time.Date(2026, 4, 4, 9, 15, 1, 0, time.UTC))
	if previous.State != OfficialPostStatePrevious || !previous.AnswerWindow.IsOpen {
		t.Fatalf("expected manual lifecycle to move to previous after 24 hours, got %+v", previous)
	}

	archived := EvaluateManualOfficialPost(publishedAt, time.Date(2026, 4, 5, 9, 15, 1, 0, time.UTC))
	if archived.State != OfficialPostStateArchived || archived.AnswerWindow.IsOpen {
		t.Fatalf("expected manual lifecycle to archive after 48 hours, got %+v", archived)
	}
}

// TestBecomesPreviousAtAndArchiveAtProjectFromPublishDate pins the +1/+2 day
// projection that downstream code uses to populate GraceUntil and ArchiveAt
// when inserting a new official post row. A regression that drifts those
// boundaries by even one day silently shortens or extends the answer window
// without affecting any other helper, so the projection deserves its own
// pinned test rather than relying on the EvaluateOfficialPost composition.
func TestBecomesPreviousAtAndArchiveAtProjectFromPublishDate(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	wantPrevious := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	if got := BecomesPreviousAt(schedule, publishDate); !got.Equal(wantPrevious) {
		t.Fatalf("BecomesPreviousAt() = %s, want %s", got.Format(time.RFC3339), wantPrevious.Format(time.RFC3339))
	}
	wantArchive := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	if got := ArchiveAt(schedule, publishDate); !got.Equal(wantArchive) {
		t.Fatalf("ArchiveAt() = %s, want %s", got.Format(time.RFC3339), wantArchive.Format(time.RFC3339))
	}
	if got := BecomesPreviousAt(schedule, time.Time{}); !got.IsZero() {
		t.Fatalf("expected zero publish date to project zero time, got %s", got.Format(time.RFC3339))
	}
	if got := ArchiveAt(schedule, time.Time{}); !got.IsZero() {
		t.Fatalf("expected zero publish date to project zero time, got %s", got.Format(time.RFC3339))
	}
}

// TestManualBecomesPreviousAtAndArchiveAtAddFixedHours pins the manual
// lifecycle's +24h/+48h projection independently of EvaluateManualOfficialPost.
// The composition test exercises the boundaries via state transitions; this
// helper-level test catches a unit drift (e.g. accidentally using days instead
// of hours) the composition could mask if the state machine were rewritten.
// Zero publishedAt is asserted explicitly to keep the contract aligned with
// BecomesPreviousAt/ArchiveAt — both families now return zero for zero input.
func TestManualBecomesPreviousAtAndArchiveAtAddFixedHours(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, 4, 3, 9, 15, 0, 0, time.UTC)
	wantPrevious := publishedAt.Add(24 * time.Hour)
	wantArchive := publishedAt.Add(48 * time.Hour)

	if got := ManualBecomesPreviousAt(publishedAt); !got.Equal(wantPrevious) {
		t.Fatalf("ManualBecomesPreviousAt() = %s, want %s", got.Format(time.RFC3339), wantPrevious.Format(time.RFC3339))
	}
	if got := ManualArchiveAt(publishedAt); !got.Equal(wantArchive) {
		t.Fatalf("ManualArchiveAt() = %s, want %s", got.Format(time.RFC3339), wantArchive.Format(time.RFC3339))
	}
	if got := ManualBecomesPreviousAt(time.Time{}); !got.IsZero() {
		t.Fatalf("expected zero publishedAt to project zero, got %s", got.Format(time.RFC3339))
	}
	if got := ManualArchiveAt(time.Time{}); !got.IsZero() {
		t.Fatalf("expected zero publishedAt to project zero, got %s", got.Format(time.RFC3339))
	}

	nonUTC := time.Date(2026, 4, 3, 9, 15, 0, 0, time.FixedZone("ahead", 3*60*60))
	if got := ManualBecomesPreviousAt(nonUTC); got.Location() != time.UTC || !got.Equal(nonUTC.Add(24*time.Hour)) {
		t.Fatalf("expected UTC normalization, got %s in %s", got.Format(time.RFC3339), got.Location())
	}
}

// TestStateWithinWindowBoundariesAndDegenerateCases targets the pure state
// machine that downstream callers reuse with already-stored boundaries (so
// they do not have to recompute from the schedule). The runtime has at least
// two callers that pass these boundaries directly; without an isolated test
// a regression in the boundary classification would only surface through the
// integration suite. The "zero boundaries" row pins the documented contract
// that a degenerate row reports as Archived rather than Current.
func TestStateWithinWindowBoundariesAndDegenerateCases(t *testing.T) {
	t.Parallel()

	graceUntil := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)

	cases := []struct {
		name       string
		graceUntil time.Time
		archiveAt  time.Time
		now        time.Time
		want       OfficialPostState
	}{
		{name: "before grace stays current", graceUntil: graceUntil, archiveAt: archiveAt, now: graceUntil.Add(-time.Nanosecond), want: OfficialPostStateCurrent},
		{name: "exact grace boundary flips to previous", graceUntil: graceUntil, archiveAt: archiveAt, now: graceUntil, want: OfficialPostStatePrevious},
		{name: "between grace and archive stays previous", graceUntil: graceUntil, archiveAt: archiveAt, now: archiveAt.Add(-time.Nanosecond), want: OfficialPostStatePrevious},
		{name: "exact archive boundary flips to archived", graceUntil: graceUntil, archiveAt: archiveAt, now: archiveAt, want: OfficialPostStateArchived},
		{name: "zero boundaries always archive", graceUntil: time.Time{}, archiveAt: time.Time{}, now: graceUntil, want: OfficialPostStateArchived},
		{name: "missing grace alone treated as degenerate", graceUntil: time.Time{}, archiveAt: archiveAt, now: graceUntil.Add(-time.Hour), want: OfficialPostStateArchived},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := StateWithinWindow(tc.graceUntil, tc.archiveAt, tc.now); got != tc.want {
				t.Fatalf("StateWithinWindow() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestEvaluateOfficialPostWindowDegenerateInputsReturnEmptyLifecycle pins the
// "give me back nothing" contract — this is what callers use to detect a row
// that has no useful lifecycle yet (e.g. abandoned posts whose boundaries
// were never persisted). Without this, a refactor that fabricated default
// boundaries would silently mark abandoned rows as current/previous.
func TestEvaluateOfficialPostWindowDegenerateInputsReturnEmptyLifecycle(t *testing.T) {
	t.Parallel()

	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	publishAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	now := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		publishDate time.Time
		publishAt   time.Time
		graceUntil  time.Time
		archiveAt   time.Time
	}{
		{name: "missing dates entirely", publishDate: time.Time{}, publishAt: time.Time{}, graceUntil: graceUntil, archiveAt: archiveAt},
		{name: "missing grace boundary", publishDate: publishDate, publishAt: publishAt, graceUntil: time.Time{}, archiveAt: archiveAt},
		{name: "missing archive boundary", publishDate: publishDate, publishAt: publishAt, graceUntil: graceUntil, archiveAt: time.Time{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := EvaluateOfficialPostWindow(tc.publishDate, tc.publishAt, tc.graceUntil, tc.archiveAt, now)
			if !got.PublishDateUTC.IsZero() || !got.PublishAt.IsZero() || got.AnswerWindow.IsOpen || got.State != "" {
				t.Fatalf("expected empty lifecycle for degenerate input, got %+v", got)
			}
		})
	}
}

// TestEvaluateOfficialPostWindowBackfillsPublishDateFromPublishAt pins the
// fallback that lets a caller hand in a row whose PublishDateUTC is zero but
// whose PublishAt is known — the helper normalizes the publish-at into a
// calendar date so the lifecycle stays well-formed.
func TestEvaluateOfficialPostWindowBackfillsPublishDateFromPublishAt(t *testing.T) {
	t.Parallel()

	publishAt := time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC)
	graceUntil := time.Date(2026, 4, 4, 12, 43, 0, 0, time.UTC)
	archiveAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	now := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)

	got := EvaluateOfficialPostWindow(time.Time{}, publishAt, graceUntil, archiveAt, now)
	wantDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	if !got.PublishDateUTC.Equal(wantDate) {
		t.Fatalf("expected PublishDateUTC backfill from publishAt, got %s", got.PublishDateUTC.Format(time.RFC3339))
	}
	if got.State != OfficialPostStateCurrent || !got.AnswerWindow.IsOpen || !got.AnswerWindow.ClosesAt.Equal(archiveAt) {
		t.Fatalf("expected current/open lifecycle anchored to archive boundary, got %+v", got)
	}
}

// TestStateAtUsesScheduleProjection covers the convenience wrapper that
// composes BecomesPreviousAt/ArchiveAt with StateWithinWindow. The composing
// test (TestEvaluateOfficialPostTransitions...) only exercises the happy
// publishDate path, leaving the documented "zero publishDate degrades to
// archived" contract uncovered.
func TestStateAtUsesScheduleProjection(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	if got := StateAt(schedule, publishDate, time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)); got != OfficialPostStateCurrent {
		t.Fatalf("expected current state, got %q", got)
	}
	if got := StateAt(schedule, publishDate, time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)); got != OfficialPostStateArchived {
		t.Fatalf("expected archived state at exact archive boundary, got %q", got)
	}
	if got := StateAt(schedule, time.Time{}, time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)); got != OfficialPostStateArchived {
		t.Fatalf("expected zero publish date to degrade to archived, got %q", got)
	}
}

// TestShouldArchiveBoundaries pins the archive readiness check the reconcile
// loop uses to decide whether to push an old post into the archive flow.
// Each row is a distinct correctness condition: pre-boundary must wait,
// at-and-after boundary must trigger, zero publish date must back off
// silently, and an existing archivedAt must never re-trigger work.
func TestShouldArchiveBoundaries(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	publishDate := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	archiveBoundary := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)
	archivedAt := time.Date(2026, 4, 5, 12, 43, 0, 0, time.UTC)

	cases := []struct {
		name        string
		publishDate time.Time
		now         time.Time
		archivedAt  *time.Time
		want        bool
	}{
		{name: "before boundary returns false", publishDate: publishDate, now: archiveBoundary.Add(-time.Nanosecond), archivedAt: nil, want: false},
		{name: "exact boundary returns true", publishDate: publishDate, now: archiveBoundary, archivedAt: nil, want: true},
		{name: "after boundary returns true", publishDate: publishDate, now: archiveBoundary.Add(time.Hour), archivedAt: nil, want: true},
		{name: "already archived returns false even after boundary", publishDate: publishDate, now: archiveBoundary.Add(time.Hour), archivedAt: &archivedAt, want: false},
		{name: "zero publish date never archives", publishDate: time.Time{}, now: archiveBoundary, archivedAt: nil, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ShouldArchive(schedule, tc.publishDate, tc.now, tc.archivedAt); got != tc.want {
				t.Fatalf("ShouldArchive() = %v, want %v", got, tc.want)
			}
		})
	}
}
