package qotd

import (
	"context"
	"errors"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type stubSetThreadStatePublisher struct {
	calls []ThreadState
	err   error
}

func (p *stubSetThreadStatePublisher) PublishOfficialPost(context.Context, PublishOfficialPostParams) (*PublishedOfficialPost, error) {
	return nil, errors.New("unexpected PublishOfficialPost call")
}

func (p *stubSetThreadStatePublisher) SetThreadState(_ context.Context, _ string, _ string, state ThreadState) error {
	p.calls = append(p.calls, state)
	return p.err
}

func (p *stubSetThreadStatePublisher) DeleteOfficialPost(_ context.Context, _ DeleteOfficialPostParams) error {
	return errors.New("unexpected DeleteOfficialPost call")
}

func TestSetThreadStateDegradesGracefullyOnMissingAccess(t *testing.T) {
	t.Parallel()

	pub := &stubSetThreadStatePublisher{
		err: ErrDiscordMissingAccess,
	}
	service := &Service{publisher: pub}

	state := ThreadState{Pinned: false, Locked: false, Archived: false}
	missing, err := service.setThreadState(context.Background(), "guild-1", "thread-1", state)
	if err != nil {
		t.Fatalf("expected unmanageable thread to degrade silently, got err=%v", err)
	}
	if missing {
		t.Fatalf("expected unmanageable thread NOT to be classified as missing (404 path is for genuinely deleted threads), got missing=true")
	}

	for i := 0; i < 3; i++ {
		missing, err = service.setThreadState(context.Background(), "guild-1", "thread-1", state)
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
		err: ErrDiscordUnknownChannel,
	}
	service := &Service{publisher: pub}

	missing, err := service.setThreadState(context.Background(), "guild-1", "thread-1", ThreadState{})
	if err != nil {
		t.Fatalf("expected missing thread to be reported as missing without error, got err=%v", err)
	}
	if !missing {
		t.Fatalf("expected missing thread to flip the missing flag so callers can mark MissingDiscord, got missing=false")
	}
}

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

	if err := service.syncLiveOfficialPost(context.Background(), post, lifecycle); err != nil {
		t.Fatalf("expected short-circuit to succeed without invoking publisher or store, got %v", err)
	}
	if len(pub.calls) != 0 {
		t.Fatalf("expected zero publisher calls during short-circuit, got %d", len(pub.calls))
	}
}

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

	if err := service.syncLiveOfficialPost(context.Background(), post, lifecycle); err != nil {
		t.Fatalf("expected short-circuit to tolerate padded state values, got %v", err)
	}
	if len(pub.calls) != 0 {
		t.Fatalf("expected zero publisher calls during short-circuit, got %d", len(pub.calls))
	}
}

func TestSetThreadStateStillPropagatesUnknownErrors(t *testing.T) {
	t.Parallel()

	pub := &stubSetThreadStatePublisher{
		err: errors.New("boom"),
	}
	service := &Service{publisher: pub}

	missing, err := service.setThreadState(context.Background(), "guild-1", "thread-1", ThreadState{})
	if err == nil {
		t.Fatalf("expected 5xx errors to bubble up so the reconcile loop can retry, got err=nil")
	}
	if missing {
		t.Fatalf("expected 5xx not to be misclassified as missing thread, got missing=true")
	}
}
