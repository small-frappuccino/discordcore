package session

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func restoreSessionStubs(t *testing.T, newFn func(string) (*discordgo.Session, error), openFn func(*discordgo.Session) error, closeFn func(*discordgo.Session) error) {
	t.Helper()
	originalNew := newSession
	originalOpen := openSession
	originalClose := closeSession
	t.Cleanup(func() {
		newSession = originalNew
		openSession = originalOpen
		closeSession = originalClose
	})
	newSession = newFn
	openSession = openFn
	closeSession = closeFn
}

func TestNewDiscordSessionEmptyToken(t *testing.T) {
	called := false
	restoreSessionStubs(t, func(token string) (*discordgo.Session, error) {
		called = true
		return nil, nil
	}, func(*discordgo.Session) error { return nil }, func(*discordgo.Session) error { return nil })

	if _, err := NewDiscordSession(""); err == nil {
		t.Fatalf("expected error for empty token")
	}
	if called {
		t.Fatalf("newSession should not be called for empty token")
	}
}

func TestNewDiscordSessionCreateError(t *testing.T) {
	restoreSessionStubs(t, func(token string) (*discordgo.Session, error) {
		return nil, errors.New("boom")
	}, func(*discordgo.Session) error { t.Fatalf("openSession should not run on create error"); return nil }, func(*discordgo.Session) error { return nil })

	if _, err := NewDiscordSession("token"); err == nil || !strings.Contains(err.Error(), "failed to create") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewDiscordSessionConnectionErrorCloses(t *testing.T) {
	session := &discordgo.Session{}
	closed := false
	restoreSessionStubs(t, func(token string) (*discordgo.Session, error) {
		return session, nil
	}, func(*discordgo.Session) error { return errors.New("connect-fail") }, func(*discordgo.Session) error {
		closed = true
		return nil
	})

	if _, err := NewDiscordSession("token"); err == nil || !strings.Contains(err.Error(), "failed to connect") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !closed {
		t.Fatalf("expected closeSession to be called on connect failure")
	}
}

func TestNewDiscordSessionSuccess(t *testing.T) {
	session := &discordgo.Session{}
	restoreSessionStubs(t, func(token string) (*discordgo.Session, error) {
		return session, nil
	}, func(*discordgo.Session) error { return nil }, func(*discordgo.Session) error { t.Fatalf("closeSession should not be called on success"); return nil })

	got, err := NewDiscordSession("token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != session {
		t.Fatalf("expected returned session pointer")
	}
	if session.Identify.Intents&discordgo.IntentMessageContent == 0 {
		t.Fatalf("expected intents to be set on session")
	}
}
