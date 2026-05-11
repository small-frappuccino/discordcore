package qotd

import (
	"errors"
	"strings"
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

// FuzzNormalizeQuestionMutationBody pins the validator's contract under
// arbitrary user-controlled body bytes (this helper sits behind both the
// dashboard and the slash-command create paths, so its blast radius is wide).
// The seed corpus covers the obvious cases; the fuzzer extends to NUL bytes,
// invalid UTF-8, very long inputs, and edge-of-trim whitespace combinations.
// Both branches are pinned: success must produce a trimmed non-empty body
// with the documented default status; failure must wrap ErrInvalidQOTDInput
// and leave the returned body/status zero so callers cannot accidentally
// persist a half-validated record.
func FuzzNormalizeQuestionMutationBody(f *testing.F) {
	for _, seed := range []string{
		"",
		" ",
		"\t\n",
		"What now?",
		"  What now?  ",
		"single",
		"unicode 你好",
		string([]byte{0x00, 'a'}),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		gotBody, gotStatus, err := normalizeQuestionMutation(QuestionMutation{Body: body})
		if err != nil {
			if !errors.Is(err, files.ErrInvalidQOTDInput) {
				t.Fatalf("error must wrap ErrInvalidQOTDInput, got %v", err)
			}
			if gotBody != "" || gotStatus != "" {
				t.Fatalf("error path must return zero body/status, got body=%q status=%q", gotBody, gotStatus)
			}
			if strings.TrimSpace(body) != "" {
				t.Fatalf("non-blank body %q rejected — error path should only fire for blank bodies when status is empty", body)
			}
			return
		}
		if gotBody == "" {
			t.Fatalf("success path must return non-empty body, got empty from %q", body)
		}
		if gotBody != strings.TrimSpace(gotBody) {
			t.Fatalf("success path must return trimmed body, got %q", gotBody)
		}
		if gotStatus != QuestionStatusReady {
			t.Fatalf("default status must be ready, got %q", gotStatus)
		}
	})
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

func TestBuildOfficialThreadNameUsesZeroPaddedOrdinal(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		ordinal int64
		want    string
	}{
		{name: "first publish", ordinal: 1, want: "Question #001"},
		{name: "two-digit", ordinal: 42, want: "Question #042"},
		{name: "padding overflows past 999", ordinal: 1234, want: "Question #1234"},
		{name: "non-positive ordinal degrades gracefully", ordinal: 0, want: "Question"},
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
				ActiveDeckID:                     files.LegacyQOTDDefaultDeckID,
				Schedule:                         mkSchedule,
				Decks:                            []files.QOTDDeckConfig{enabledDeck},
				SuppressScheduledPublishDatesUTC: []string{"2026-05-10"},
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cm := files.NewMemoryConfigManager()
			if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g1", QOTD: tc.cfg}); err != nil {
				t.Fatalf("AddGuildConfig: %v", err)
			}
			service := NewService(cm, &storage.Store{}, nil)
			got, ok := service.NextScheduledPublishTime("g1", tc.now)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (got=%s)", ok, tc.wantOK, got.Format(time.RFC3339))
			}
			if ok && !got.Equal(tc.wantNext) {
				t.Fatalf("next = %s, want %s", got.Format(time.RFC3339), tc.wantNext.Format(time.RFC3339))
			}
		})
	}
}
