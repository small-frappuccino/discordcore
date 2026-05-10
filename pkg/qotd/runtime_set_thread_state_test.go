package qotd

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
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
