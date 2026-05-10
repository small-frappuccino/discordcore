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

func TestReorderQuestionIDsToIndexMovesQuestionInBothDirections(t *testing.T) {
	t.Parallel()

	questions := []storage.QOTDQuestionRecord{{ID: 10}, {ID: 20}, {ID: 30}, {ID: 40}}

	if got := reorderQuestionIDsToIndex(questions, 3, 1); len(got) != 4 || got[0] != 10 || got[1] != 40 || got[2] != 20 || got[3] != 30 {
		t.Fatalf("expected later question to move earlier, got %+v", got)
	}
	if got := reorderQuestionIDsToIndex(questions, 0, 2); len(got) != 4 || got[0] != 20 || got[1] != 30 || got[2] != 10 || got[3] != 40 {
		t.Fatalf("expected earlier question to move later, got %+v", got)
	}
}

// TestReorderQuestionIDsToIndexEdgeCases covers the silent-no-op and
// invalid-index branches the happy-path test does not exercise. Without
// these, a regression that returns nil for a noop or accepts an out-of-range
// target index would not surface until a frontend-driven reorder corrupted
// the queue.
func TestReorderQuestionIDsToIndexEdgeCases(t *testing.T) {
	t.Parallel()

	questions := []storage.QOTDQuestionRecord{{ID: 10}, {ID: 20}, {ID: 30}}

	if got := reorderQuestionIDsToIndex(questions, 1, 1); len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("expected no-op reorder to preserve order, got %+v", got)
	}
	if got := reorderQuestionIDsToIndex(nil, 0, 0); got != nil {
		t.Fatalf("expected empty input to return nil, got %+v", got)
	}
	for _, badPair := range [][2]int{{-1, 0}, {0, -1}, {3, 0}, {0, 3}} {
		if got := reorderQuestionIDsToIndex(questions, badPair[0], badPair[1]); got != nil {
			t.Fatalf("expected out-of-range indices %v to return nil, got %+v", badPair, got)
		}
	}
}

// TestReorderQuestionIDsRespectsQueuePositionAndDirection pins the exported
// helper that callers use directly when an operator nudges a question up or
// down by one slot. It covers the four branches that matter operationally:
// unsorted input is sorted by QueuePosition before the swap so the storage
// order wins over the input slice order, the move is bounded at the head and
// tail without erroring out, and bad inputs degrade to nil instead of
// returning a partially-populated slice that the caller would then persist.
func TestReorderQuestionIDsRespectsQueuePositionAndDirection(t *testing.T) {
	t.Parallel()

	unsorted := []storage.QOTDQuestionRecord{
		{ID: 30, QueuePosition: 3},
		{ID: 10, QueuePosition: 1},
		{ID: 20, QueuePosition: 2},
	}

	if got := ReorderQuestionIDs(unsorted, 30, -1); len(got) != 3 || got[0] != 10 || got[1] != 30 || got[2] != 20 {
		t.Fatalf("expected last question to swap with the middle one, got %+v", got)
	}
	if got := ReorderQuestionIDs(unsorted, 10, 1); len(got) != 3 || got[0] != 20 || got[1] != 10 || got[2] != 30 {
		t.Fatalf("expected first question to swap with the middle one, got %+v", got)
	}
	if got := ReorderQuestionIDs(unsorted, 10, -1); len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("expected head clamp to return the sorted order unchanged, got %+v", got)
	}
	if got := ReorderQuestionIDs(unsorted, 30, 1); len(got) != 3 || got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Fatalf("expected tail clamp to return the sorted order unchanged, got %+v", got)
	}
	if got := ReorderQuestionIDs(unsorted, 30, 0); got != nil {
		t.Fatalf("expected zero direction to return nil, got %+v", got)
	}
	if got := ReorderQuestionIDs(unsorted, 999, -1); got != nil {
		t.Fatalf("expected unknown id to return nil, got %+v", got)
	}
	if got := ReorderQuestionIDs(nil, 10, -1); got != nil {
		t.Fatalf("expected empty input to return nil, got %+v", got)
	}
}

// TestFirstReadyUnscheduledQuestionSkipsNonEligibleStates extends the existing
// firstReadyUnscheduled test to cover every reason the helper rejects a row:
// non-ready status (draft/disabled/used/reserved), already-published rows, and
// rows reserved by the scheduler. Without this, a regression that accepts
// drafts or disabled questions would silently expose a half-edited row to the
// publish path.
func TestFirstReadyUnscheduledQuestionSkipsNonEligibleStates(t *testing.T) {
	t.Parallel()

	publishedAt := time.Date(2026, 4, 2, 13, 0, 0, 0, time.UTC)
	scheduledFor := time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)
	skip := []storage.QOTDQuestionRecord{
		{ID: 1, Status: string(QuestionStatusDraft)},
		{ID: 2, Status: string(QuestionStatusDisabled)},
		{ID: 3, Status: string(QuestionStatusReserved)},
		{ID: 4, Status: string(QuestionStatusUsed)},
		{ID: 5, Status: string(QuestionStatusReady), PublishedOnceAt: &publishedAt},
		{ID: 6, Status: string(QuestionStatusReady), ScheduledForDateUTC: &scheduledFor},
	}
	if got := firstReadyUnscheduledQuestion(skip); got != nil {
		t.Fatalf("expected no eligible question, got %+v", got)
	}

	withReady := append(append([]storage.QOTDQuestionRecord(nil), skip...), storage.QOTDQuestionRecord{ID: 99, Status: string(QuestionStatusReady)})
	got := firstReadyUnscheduledQuestion(withReady)
	if got == nil || got.ID != 99 {
		t.Fatalf("expected helper to return the trailing ready row, got %+v", got)
	}
}

// TestReservedQuestionForDateRejectsZeroAndUnscheduledRows keeps the helper
// from drifting into matching rows it should not — the original test only
// covers the positive case and "different slot date". Zero input previously
// would normalize to zero and silently match every row whose
// ScheduledForDateUTC was nil, so explicitly pinning both refusals is cheap
// insurance.
func TestReservedQuestionForDateRejectsZeroAndUnscheduledRows(t *testing.T) {
	t.Parallel()

	scheduledFor := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	zeroDate := time.Time{}

	rows := []storage.QOTDQuestionRecord{
		{ID: 1, Status: string(QuestionStatusReserved)},
		{ID: 2, Status: string(QuestionStatusReserved), ScheduledForDateUTC: &zeroDate},
		{ID: 3, Status: string(QuestionStatusReady), ScheduledForDateUTC: &scheduledFor},
	}
	if got := reservedQuestionForDate(rows, time.Time{}); got != nil {
		t.Fatalf("expected zero slot date to return nil, got %+v", got)
	}
	if got := reservedQuestionForDate(rows, scheduledFor); got != nil {
		t.Fatalf("expected reserved-but-unscheduled and ready rows to be skipped, got %+v", got)
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

// TestNormalizeActorIDFallsBackToControlAPIWhenBlank pins the audit-log
// fallback used when a control-plane caller forgets to attach an actor (the
// audit row would otherwise read as an empty string and erase provenance).
// The helper is small but its output is permanently recorded in storage, so
// keeping the contract pinned is cheap insurance.
func TestNormalizeActorIDFallsBackToControlAPIWhenBlank(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty falls back", in: "", want: "control_api"},
		{name: "whitespace falls back", in: "   \t\n", want: "control_api"},
		{name: "preserves explicit actor", in: "user-42", want: "user-42"},
		{name: "trims surrounding whitespace", in: "  user-42  ", want: "user-42"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeActorID(tc.in); got != tc.want {
				t.Fatalf("normalizeActorID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestDerefTimeReturnsZeroForNilAndUTCForValue pins the small helper that
// callers use when threading optional time pointers through the pure layer
// (e.g. PublishedOnceAt, ArchivedAt). A regression that returned the local
// representation would silently shift schedule comparisons by the test
// machine's offset.
func TestDerefTimeReturnsZeroForNilAndUTCForValue(t *testing.T) {
	t.Parallel()

	if got := derefTime(nil); !got.IsZero() {
		t.Fatalf("expected nil pointer to deref to zero, got %s", got.Format(time.RFC3339))
	}

	value := time.Date(2026, 5, 7, 12, 43, 0, 0, time.FixedZone("ahead", 3*60*60))
	got := derefTime(&value)
	if !got.Equal(value) || got.Location() != time.UTC {
		t.Fatalf("expected derefTime to normalize to UTC, got %s in %s", got.Format(time.RFC3339), got.Location())
	}
}

func TestBuildOfficialThreadNameUsesZeroPaddedOrdinal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		ordinal int64
		want    string
	}{
		{name: "first publish", ordinal: 1, want: "Pergunta #001"},
		{name: "two-digit", ordinal: 42, want: "Pergunta #042"},
		{name: "padding overflows past 999", ordinal: 1234, want: "Pergunta #1234"},
		{name: "non-positive ordinal degrades gracefully", ordinal: 0, want: "Pergunta"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := buildOfficialThreadName(tc.ordinal); got != tc.want {
				t.Fatalf("buildOfficialThreadName(%d) = %q, want %q", tc.ordinal, got, tc.want)
			}
		})
	}
}

// TestNextScheduledPublishTimeProjectsTodayOrTomorrow exercises the wake-up
// projection used by the runtime loop. It's table-driven because each branch
// (before/after the slot, suppressed today, deck disabled, no schedule, no
// channel) is a distinct correctness condition: a regression in any single
// branch could either silently delay a publish (returning false when a slot
// is actually due soon) or wake the loop on a slot that PublishScheduledIfDue
// would refuse (suppression).
func TestNextScheduledPublishTimeProjectsTodayOrTomorrow(t *testing.T) {
	t.Parallel()

	hour, minute := 12, 43
	mkSchedule := files.QOTDPublishScheduleConfig{HourUTC: &hour, MinuteUTC: &minute}
	enabledDeck := files.QOTDDeckConfig{
		ID:        files.LegacyQOTDDefaultDeckID,
		Name:      files.LegacyQOTDDefaultDeckName,
		Enabled:   true,
		ChannelID: "100000000000000001",
	}
	disabledDeck := files.QOTDDeckConfig{
		ID:        files.LegacyQOTDDefaultDeckID,
		Name:      files.LegacyQOTDDefaultDeckName,
		Enabled:   false,
		ChannelID: "100000000000000001",
	}
	enabledNoChannel := files.QOTDDeckConfig{
		ID:      files.LegacyQOTDDefaultDeckID,
		Name:    files.LegacyQOTDDefaultDeckName,
		Enabled: true,
	}

	cases := []struct {
		name      string
		cfg       files.QOTDConfig
		now       time.Time
		wantOK    bool
		wantNext  time.Time
	}{
		{
			name: "before today's slot returns today",
			cfg: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Schedule:     mkSchedule,
				Decks:        []files.QOTDDeckConfig{enabledDeck},
			},
			now:      time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			wantOK:   true,
			wantNext: time.Date(2026, 5, 10, 12, 43, 0, 0, time.UTC),
		},
		{
			name: "after today's slot rolls to tomorrow",
			cfg: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Schedule:     mkSchedule,
				Decks:        []files.QOTDDeckConfig{enabledDeck},
			},
			now:      time.Date(2026, 5, 10, 13, 0, 0, 0, time.UTC),
			wantOK:   true,
			wantNext: time.Date(2026, 5, 11, 12, 43, 0, 0, time.UTC),
		},
		{
			name: "today's slot suppressed advances one day",
			cfg: files.QOTDConfig{
				ActiveDeckID:                   files.LegacyQOTDDefaultDeckID,
				Schedule:                        mkSchedule,
				Decks:                           []files.QOTDDeckConfig{enabledDeck},
				SuppressScheduledPublishDateUTC: "2026-05-10",
			},
			now:      time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			wantOK:   true,
			wantNext: time.Date(2026, 5, 11, 12, 43, 0, 0, time.UTC),
		},
		{
			name: "incomplete schedule reports no eligible moment",
			cfg: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Schedule:     files.QOTDPublishScheduleConfig{},
				Decks:        []files.QOTDDeckConfig{enabledDeck},
			},
			now:    time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			wantOK: false,
		},
		{
			name: "disabled deck reports no eligible moment",
			cfg: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Schedule:     mkSchedule,
				Decks:        []files.QOTDDeckConfig{disabledDeck},
			},
			now:    time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			wantOK: false,
		},
		{
			name: "missing channel reports no eligible moment",
			cfg: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Schedule:     mkSchedule,
				Decks:        []files.QOTDDeckConfig{enabledNoChannel},
			},
			now:    time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC),
			wantOK: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := nextScheduledPublishTimeFromConfig(tc.cfg, tc.now)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got=%s)", ok, tc.wantOK, got.Format(time.RFC3339))
			}
			if ok && !got.Equal(tc.wantNext) {
				t.Fatalf("next = %s, want %s", got.Format(time.RFC3339), tc.wantNext.Format(time.RFC3339))
			}
		})
	}
}

// TestNextScheduledPublishTimeRejectsBlankAndNilService pins the early-return
// guards. The runtime loop calls this on every wake-up; a regression that
// crashed on a nil receiver or accepted a whitespace guild ID would either
// panic the loop or fan out spurious wake-ups for an unconfigured guild.
func TestNextScheduledPublishTimeRejectsBlankAndNilService(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)

	var nilService *Service
	if got, ok := nilService.NextScheduledPublishTime("g1", now); ok || !got.IsZero() {
		t.Fatalf("expected nil service to return zero/false, got %s ok=%v", got.Format(time.RFC3339), ok)
	}

	cm := files.NewMemoryConfigManager()
	service := NewService(cm, &storage.Store{}, nil)

	if got, ok := service.NextScheduledPublishTime("", now); ok || !got.IsZero() {
		t.Fatalf("expected empty guild id to return zero/false, got %s ok=%v", got.Format(time.RFC3339), ok)
	}
	if got, ok := service.NextScheduledPublishTime("   \t", now); ok || !got.IsZero() {
		t.Fatalf("expected whitespace guild id to return zero/false, got %s ok=%v", got.Format(time.RFC3339), ok)
	}
}
