package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordgo"
)

func restoreSessionStubs(t *testing.T, newFn func(string) (*discordgo.Session, error), openFn func(*discordgo.Session) error, closeFn func(*discordgo.Session) error, addHandlerFn func(*discordgo.Session, interface{}) func()) {
	t.Helper()
	originalNew := newSession
	originalOpen := openSession
	originalClose := closeSession
	originalAddHandler := addHandlerOnce
	t.Cleanup(func() {
		newSession = originalNew
		openSession = originalOpen
		closeSession = originalClose
		addHandlerOnce = originalAddHandler
	})
	if newFn != nil {
		newSession = newFn
	}
	if openFn != nil {
		openSession = openFn
	}
	if closeFn != nil {
		closeSession = closeFn
	}
	if addHandlerFn != nil {
		addHandlerOnce = addHandlerFn
	}
}

func TestNewDiscordSessionEmptyToken(t *testing.T) {
	called := false
	restoreSessionStubs(t, func(token string) (*discordgo.Session, error) {
		called = true
		return nil, nil
	}, func(*discordgo.Session) error { return nil }, func(*discordgo.Session) error { return nil }, nil)

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
	}, func(*discordgo.Session) error { t.Fatalf("openSession should not run on create error"); return nil }, func(*discordgo.Session) error { return nil }, nil)

	if _, err := NewDiscordSession("token"); err == nil || !strings.Contains(err.Error(), "failed to create") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewDiscordSessionSuccess(t *testing.T) {
	session := &discordgo.Session{}
	restoreSessionStubs(t, func(token string) (*discordgo.Session, error) {
		return session, nil
	}, func(*discordgo.Session) error {
		t.Fatalf("openSession should not be called in constructor")
		return nil
	}, func(*discordgo.Session) error { t.Fatalf("closeSession should not be called on success"); return nil }, nil)

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

func TestNewDiscordSessionWithIntentsUsesProvidedMask(t *testing.T) {
	session := &discordgo.Session{}
	restoreSessionStubs(t, func(token string) (*discordgo.Session, error) {
		return session, nil
	}, func(*discordgo.Session) error {
		t.Fatalf("openSession should not be called in constructor")
		return nil
	}, func(*discordgo.Session) error { t.Fatalf("closeSession should not be called on success"); return nil }, nil)

	mask := discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions
	got, err := NewDiscordSessionWithIntents("token", mask)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != session {
		t.Fatalf("expected returned session pointer")
	}
	if session.Identify.Intents != mask {
		t.Fatalf("expected intents mask %d, got %d", mask, session.Identify.Intents)
	}
}

func TestOpenSession(t *testing.T) {
	tests := []struct {
		name       string
		setupStubs func(t *testing.T, session *discordgo.Session)
		setupCtx   func() (context.Context, context.CancelFunc)
		wantErr    string
	}{
		{
			name: "Success path",
			setupStubs: func(t *testing.T, session *discordgo.Session) {
				var capturedHandler func(*discordgo.Session, *discordgo.Ready)
				restoreSessionStubs(t, nil, func(s *discordgo.Session) error {
					if capturedHandler == nil {
						t.Fatalf("handler not registered")
					}
					// Simulate the gateway READY event
					capturedHandler(s, &discordgo.Ready{})
					return nil
				}, func(s *discordgo.Session) error {
					t.Fatalf("closeSession should not be called on success")
					return nil
				}, func(s *discordgo.Session, h interface{}) func() {
					capturedHandler = h.(func(*discordgo.Session, *discordgo.Ready))
					return func() {}
				})
			},
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 2*time.Second)
			},
			wantErr: "",
		},
		{
			name: "OpenSession failure",
			setupStubs: func(t *testing.T, session *discordgo.Session) {
				restoreSessionStubs(t, nil, func(s *discordgo.Session) error {
					return errors.New("open error")
				}, nil, func(s *discordgo.Session, h interface{}) func() {
					return func() {}
				})
			},
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 2*time.Second)
			},
			wantErr: "open error",
		},
		{
			name: "Context timeout/cancellation",
			setupStubs: func(t *testing.T, session *discordgo.Session) {
				restoreSessionStubs(t, nil, func(s *discordgo.Session) error {
					// We just wait and don't trigger the handler
					return nil
				}, func(s *discordgo.Session) error {
					return nil
				}, func(s *discordgo.Session, h interface{}) func() {
					return func() {}
				})
			},
			setupCtx: func() (context.Context, context.CancelFunc) {
				// Immediately cancel context
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			wantErr: "handshake timed out or canceled",
		},
		{
			name:       "Nil session",
			setupStubs: func(t *testing.T, session *discordgo.Session) {},
			setupCtx: func() (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			wantErr: "session is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var session *discordgo.Session
			if tt.name != "Nil session" {
				session = &discordgo.Session{}
			}

			tt.setupStubs(t, session)
			ctx, cancel := tt.setupCtx()
			defer cancel()

			err := OpenSession(ctx, session)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
			}
		})
	}
}
