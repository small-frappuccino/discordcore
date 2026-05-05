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

func TestCurrentPublishDateUTCBeforeDailyBoundaryUsesPreviousDay(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	now := time.Date(2026, 4, 3, 12, 42, 59, 0, time.UTC)
	got := CurrentPublishDateUTC(schedule, now)
	want := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("expected current publish date %s, got %s", want.Format(time.RFC3339), got.Format(time.RFC3339))
	}
}

func TestUpcomingPublishDateUTCTracksNextBoundary(t *testing.T) {
	t.Parallel()

	schedule := testSchedule(t, 12, 43)
	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before boundary uses today",
			now:  time.Date(2026, 4, 3, 12, 42, 59, 0, time.UTC),
			want: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "exact boundary stays on today",
			now:  time.Date(2026, 4, 3, 12, 43, 0, 0, time.UTC),
			want: time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "after boundary moves to tomorrow",
			now:  time.Date(2026, 4, 3, 12, 43, 1, 0, time.UTC),
			want: time.Date(2026, 4, 4, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := UpcomingPublishDateUTC(schedule, tc.now)
			if !got.Equal(tc.want) {
				t.Fatalf("expected upcoming publish date %s, got %s", tc.want.Format(time.RFC3339), got.Format(time.RFC3339))
			}
		})
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
