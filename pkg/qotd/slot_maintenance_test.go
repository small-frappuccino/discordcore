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

func TestQuestionStillLinkedToOfficialPostMatchesScheduledDate(t *testing.T) {
	t.Parallel()

	postDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	questionDate := time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC)

	post := storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate}
	question := storage.QOTDQuestionRecord{ID: 10, ScheduledForDateUTC: &questionDate}

	if !questionStillLinkedToOfficialPost(question, post) {
		t.Fatal("expected scheduled question date to link to the official post slot")
	}
}

func TestQuestionStillLinkedToOfficialPostRejectsMismatchedDates(t *testing.T) {
	t.Parallel()

	postDate := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	laterPublish := time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)

	post := storage.QOTDOfficialPostRecord{QuestionID: 10, PublishDateUTC: postDate}
	question := storage.QOTDQuestionRecord{ID: 10, PublishedOnceAt: &laterPublish}

	if questionStillLinkedToOfficialPost(question, post) {
		t.Fatal("expected mismatched publish date not to link question to the cleared official post")
	}
}
