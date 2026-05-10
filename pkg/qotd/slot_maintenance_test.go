package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestStarterMessageDeleteChannelIDPrefersOriginalChannel(t *testing.T) {
	t.Parallel()

	post := storage.QOTDOfficialPostRecord{
		ChannelID:           "question-channel",
		DiscordThreadID:     "answer-thread",
		DiscordStarterMessageID: "starter-1",
	}

	if got := starterMessageDeleteChannelID(post); got != "question-channel" {
		t.Fatalf("starterMessageDeleteChannelID() = %q, want question-channel", got)
	}
}

func TestStarterMessageDeleteChannelIDFallsBackToThread(t *testing.T) {
	t.Parallel()

	post := storage.QOTDOfficialPostRecord{
		DiscordThreadID: "answer-thread",
	}

	if got := starterMessageDeleteChannelID(post); got != "answer-thread" {
		t.Fatalf("starterMessageDeleteChannelID() = %q, want answer-thread", got)
	}
}

// TestQuestionStillLinkedToOfficialPostBranches pins each branch of the
// questionStillLinkedToOfficialPost matcher: link via scheduled date, link via
// already-published timestamp, and the four "not linked" rejections. Every
// rejection has a dedicated row so a future refactor that accidentally accepts
// a stale link (e.g. trusting a published question whose post points to a
// different slot) fails its specific subtest instead of slipping through.
func TestQuestionStillLinkedToOfficialPostBranches(t *testing.T) {
	t.Parallel()

	postDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	matchingScheduled := time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC)
	matchingPublished := time.Date(2026, 5, 7, 13, 0, 0, 0, time.UTC)
	mismatchedDate := time.Date(2026, 5, 8, 12, 43, 0, 0, time.UTC)

	cases := []struct {
		name     string
		post     storage.QOTDOfficialPostRecord
		question storage.QOTDQuestionRecord
		want     bool
	}{
		{
			name:     "scheduled date matches post slot",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate},
			question: storage.QOTDQuestionRecord{ID: 10, ScheduledForDateUTC: &matchingScheduled},
			want:     true,
		},
		{
			name:     "published timestamp falls back to slot match when no scheduled date",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate},
			question: storage.QOTDQuestionRecord{ID: 10, PublishedOnceAt: &matchingPublished},
			want:     true,
		},
		{
			name:     "scheduled date overrides published timestamp when both exist",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate},
			question: storage.QOTDQuestionRecord{ID: 10, ScheduledForDateUTC: &matchingScheduled, PublishedOnceAt: &mismatchedDate},
			want:     true,
		},
		{
			name:     "scheduled date mismatch is not absorbed by a stale published timestamp",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate},
			question: storage.QOTDQuestionRecord{ID: 10, ScheduledForDateUTC: &mismatchedDate, PublishedOnceAt: &matchingPublished},
			want:     false,
		},
		{
			name:     "question id mismatch never links",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate},
			question: storage.QOTDQuestionRecord{ID: 99, ScheduledForDateUTC: &matchingScheduled},
			want:     false,
		},
		{
			name:     "non-positive question id never links",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 0, PublishDateUTC: postDate},
			question: storage.QOTDQuestionRecord{ID: 0, ScheduledForDateUTC: &matchingScheduled},
			want:     false,
		},
		{
			name:     "post with zero publish date never links",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 10},
			question: storage.QOTDQuestionRecord{ID: 10, ScheduledForDateUTC: &matchingScheduled},
			want:     false,
		},
		{
			name:     "question with no timestamps never links",
			post:     storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate},
			question: storage.QOTDQuestionRecord{ID: 10},
			want:     false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := questionStillLinkedToOfficialPost(tc.question, tc.post); got != tc.want {
				t.Fatalf("questionStillLinkedToOfficialPost() = %v, want %v", got, tc.want)
			}
		})
	}
}
