package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// TestIsOfficialPostProvisioningCompletePinsRequiredFields covers the three
// fields the helper requires together. A regression that drops one of them
// (e.g. accepting a row whose AnswerChannelID is empty) silently lets a
// half-provisioned post be classified as published downstream.
func TestIsOfficialPostProvisioningCompletePinsRequiredFields(t *testing.T) {
	t.Parallel()

	complete := storage.QOTDOfficialPostRecord{
		DiscordThreadID:         "thread-1",
		DiscordStarterMessageID: "starter-1",
		AnswerChannelID:         "answers-1",
	}

	cases := []struct {
		name string
		mut  func(*storage.QOTDOfficialPostRecord)
		want bool
	}{
		{name: "all three fields set", mut: func(*storage.QOTDOfficialPostRecord) {}, want: true},
		{name: "missing thread id", mut: func(p *storage.QOTDOfficialPostRecord) { p.DiscordThreadID = "" }, want: false},
		{name: "missing starter message id", mut: func(p *storage.QOTDOfficialPostRecord) { p.DiscordStarterMessageID = "" }, want: false},
		{name: "missing answer channel id", mut: func(p *storage.QOTDOfficialPostRecord) { p.AnswerChannelID = "" }, want: false},
		{name: "whitespace-only thread id", mut: func(p *storage.QOTDOfficialPostRecord) { p.DiscordThreadID = "   " }, want: false},
		{name: "whitespace-only starter message id", mut: func(p *storage.QOTDOfficialPostRecord) { p.DiscordStarterMessageID = "\t" }, want: false},
		{name: "whitespace-only answer channel id", mut: func(p *storage.QOTDOfficialPostRecord) { p.AnswerChannelID = " \n " }, want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			post := complete
			tc.mut(&post)
			if got := isOfficialPostProvisioningComplete(post); got != tc.want {
				t.Fatalf("isOfficialPostProvisioningComplete() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestIsOfficialPostPublishedRequiresTimestampAndCompleteFields pins the
// composition contract: PublishedAt alone is not enough — the row must also
// have all provisioning fields. Otherwise the publish loop would treat a
// half-finished row whose nonce was inserted but whose Discord IDs failed to
// land as already-published and refuse to retry.
func TestIsOfficialPostPublishedRequiresTimestampAndCompleteFields(t *testing.T) {
	t.Parallel()

	published := time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC)
	complete := storage.QOTDOfficialPostRecord{
		DiscordThreadID:         "thread-1",
		DiscordStarterMessageID: "starter-1",
		AnswerChannelID:         "answers-1",
		PublishedAt:             &published,
	}

	if !isOfficialPostPublished(complete) {
		t.Fatal("expected complete row with timestamp to be classified as published")
	}

	noTimestamp := complete
	noTimestamp.PublishedAt = nil
	if isOfficialPostPublished(noTimestamp) {
		t.Fatal("expected row without PublishedAt to stay unpublished")
	}

	missingThread := complete
	missingThread.DiscordThreadID = ""
	if isOfficialPostPublished(missingThread) {
		t.Fatal("expected row missing thread id to stay unpublished even with PublishedAt set")
	}
}

// TestIsOfficialPostAbandonedHandlesWhitespaceAndUnknownStates pins the state
// classification the runtime uses to decide whether to abandon resume work.
// A regression that string-matches without trimming, or that accidentally
// accepts other terminal-looking states, would either keep retrying an
// abandoned post (spamming Discord) or silently abandon a recoverable one.
func TestIsOfficialPostAbandonedHandlesWhitespaceAndUnknownStates(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		state string
		want  bool
	}{
		{name: "exact abandoned state", state: string(OfficialPostStateAbandoned), want: true},
		{name: "abandoned state with surrounding whitespace", state: "  " + string(OfficialPostStateAbandoned) + "\n", want: true},
		{name: "failed state is retryable, not abandoned", state: string(OfficialPostStateFailed), want: false},
		{name: "missing_discord state is recoverable", state: string(OfficialPostStateMissingDiscord), want: false},
		{name: "empty state stays recoverable", state: "", want: false},
		{name: "unknown state stays recoverable", state: "ghost-mode", want: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			post := storage.QOTDOfficialPostRecord{State: tc.state}
			if got := isOfficialPostAbandoned(post); got != tc.want {
				t.Fatalf("isOfficialPostAbandoned() = %v, want %v", got, tc.want)
			}
		})
	}
}
