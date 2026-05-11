package qotd

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type stubSetThreadStatePublisher struct {
	calls []discordqotd.ThreadState
	err   error
}

func (p *stubSetThreadStatePublisher) PublishOfficialPost(context.Context, *discordgo.Session, discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	return nil, errors.New("unexpected PublishOfficialPost call")
}

func (p *stubSetThreadStatePublisher) SetThreadState(_ context.Context, _ *discordgo.Session, _ string, state discordqotd.ThreadState) error {
	p.calls = append(p.calls, state)
	return p.err
}

func (p *stubSetThreadStatePublisher) FetchThreadMessages(context.Context, *discordgo.Session, string) ([]discordqotd.ArchivedMessage, error) {
	return nil, errors.New("unexpected FetchThreadMessages call")
}

func (p *stubSetThreadStatePublisher) FetchChannelMessages(context.Context, *discordgo.Session, string, string, int) ([]discordqotd.ArchivedMessage, error) {
	return nil, errors.New("unexpected FetchChannelMessages call")
}

func TestSetThreadStateDegradesGracefullyOnMissingAccess(t *testing.T) {
	t.Parallel()

	pub := &stubSetThreadStatePublisher{
		err: makeRESTError(http.StatusForbidden, discordgo.ErrCodeMissingAccess, "missing access"),
	}
	service := &Service{publisher: pub}

	state := discordqotd.ThreadState{Pinned: false, Locked: false, Archived: false}
	missing, err := service.setThreadState(context.Background(), nil, "thread-1", state)
	if err != nil {
		t.Fatalf("expected unmanageable thread to degrade silently, got err=%v", err)
	}
	if missing {
		t.Fatalf("expected unmanageable thread NOT to be classified as missing (404 path is for genuinely deleted threads), got missing=true")
	}

	// Repeated calls with the same target state should NOT keep calling
	// Discord-side logic differently, but they MUST stay silent at the
	// public surface (the caller relies on err==nil to advance lifecycle).
	for i := 0; i < 3; i++ {
		missing, err = service.setThreadState(context.Background(), nil, "thread-1", state)
		if err != nil {
			t.Fatalf("expected repeated unmanageable calls to stay silent, got err=%v on call %d", err, i+2)
		}
		if missing {
			t.Fatalf("expected unmanageable thread to stay non-missing on repeated calls, got missing=true")
		}
	}
	if len(pub.calls) != 4 {
		t.Fatalf("expected publisher to be invoked once per call, got %d", len(pub.calls))
	}
}

func TestSetThreadStateStillPropagatesGenuineMissingThread(t *testing.T) {
	t.Parallel()

	pub := &stubSetThreadStatePublisher{
		err: makeRESTError(http.StatusNotFound, 0, "unknown channel"),
	}
	service := &Service{publisher: pub}

	missing, err := service.setThreadState(context.Background(), nil, "thread-1", discordqotd.ThreadState{})
	if err != nil {
		t.Fatalf("expected missing thread to be reported as missing without error, got err=%v", err)
	}
	if !missing {
		t.Fatalf("expected missing thread to flip the missing flag so callers can mark MissingDiscord, got missing=false")
	}
}

// TestSyncLiveOfficialPostShortCircuitsWhenStateMatchesTarget pins the
// reconcile-loop optimization that skips the SetThreadState API call (and
// the redundant DB update) when the stored post state already matches the
// lifecycle target. Constructing the Service with a nil store proves the
// short-circuit never reaches the store; the publisher stub returns an
// error on any invocation, so an accidental API call would fail the test.
func TestSyncLiveOfficialPostShortCircuitsWhenStateMatchesTarget(t *testing.T) {
	t.Parallel()

	pub := &stubSetThreadStatePublisher{
		err: errors.New("publisher must not be called when DB state already matches lifecycle target"),
	}
	service := &Service{publisher: pub}

	post := storage.QOTDOfficialPostRecord{
		ID:              1,
		DiscordThreadID: "thread-1",
		State:           string(OfficialPostStateCurrent),
	}
	lifecycle := OfficialPostLifecycle{State: OfficialPostStateCurrent}

	if err := service.syncLiveOfficialPost(context.Background(), nil, post, lifecycle); err != nil {
		t.Fatalf("expected short-circuit to succeed without invoking publisher or store, got %v", err)
	}
	if len(pub.calls) != 0 {
		t.Fatalf("expected zero publisher calls during short-circuit, got %d", len(pub.calls))
	}
}

// TestSyncLiveOfficialPostShortCircuitIgnoresStateWhitespace pins that
// whitespace around the persisted state (which postgres can return for
// historical rows) does not defeat the short-circuit comparison.
func TestSyncLiveOfficialPostShortCircuitIgnoresStateWhitespace(t *testing.T) {
	t.Parallel()

	pub := &stubSetThreadStatePublisher{
		err: errors.New("publisher must not be called when DB state already matches lifecycle target"),
	}
	service := &Service{publisher: pub}

	post := storage.QOTDOfficialPostRecord{
		ID:              1,
		DiscordThreadID: "thread-1",
		State:           "  " + string(OfficialPostStatePrevious) + "  ",
	}
	lifecycle := OfficialPostLifecycle{State: OfficialPostStatePrevious}

	if err := service.syncLiveOfficialPost(context.Background(), nil, post, lifecycle); err != nil {
		t.Fatalf("expected short-circuit to tolerate padded state values, got %v", err)
	}
	if len(pub.calls) != 0 {
		t.Fatalf("expected zero publisher calls during short-circuit, got %d", len(pub.calls))
	}
}

func TestSetThreadStateStillPropagatesUnknownErrors(t *testing.T) {
	t.Parallel()

	pub := &stubSetThreadStatePublisher{
		err: makeRESTError(http.StatusInternalServerError, 0, "boom"),
	}
	service := &Service{publisher: pub}

	missing, err := service.setThreadState(context.Background(), nil, "thread-1", discordqotd.ThreadState{})
	if err == nil {
		t.Fatalf("expected 5xx errors to bubble up so the reconcile loop can retry, got err=nil")
	}
	if missing {
		t.Fatalf("expected 5xx not to be misclassified as missing thread, got missing=true")
	}
}
