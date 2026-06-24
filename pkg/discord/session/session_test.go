package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordgo"
)

func TestNewDiscordSessionEmptyToken(t *testing.T) {
	t.Parallel()
	if _, err := NewDiscordSession(""); err == nil {
		t.Fatalf("expected error for empty token")
	}
}

func TestNewDiscordSessionCreateError(t *testing.T) {
	t.Parallel()
	token := "token-create-error-" + t.Name()
	newSessionOverrides.Store("Bot "+token, func(t string) (*discordgo.Session, error) {
		return nil, errors.New("boom")
	})
	defer newSessionOverrides.Delete("Bot " + token)

	if _, err := NewDiscordSession(token); err == nil || !strings.Contains(err.Error(), "failed to create") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewDiscordSessionSuccess(t *testing.T) {
	t.Parallel()
	session := &discordgo.Session{}
	token := "token-success-" + t.Name()
	newSessionOverrides.Store("Bot "+token, func(t string) (*discordgo.Session, error) {
		return session, nil
	})
	defer newSessionOverrides.Delete("Bot " + token)

	got, err := NewDiscordSession(token)
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
	t.Parallel()
	session := &discordgo.Session{}
	token := "token-mask-" + t.Name()
	newSessionOverrides.Store("Bot "+token, func(t string) (*discordgo.Session, error) {
		return session, nil
	})
	defer newSessionOverrides.Delete("Bot " + token)

	mask := discordgo.IntentsGuilds | discordgo.IntentsGuildMessageReactions
	got, err := NewDiscordSessionWithIntents(token, mask)
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
	t.Parallel()
	tests := []struct {
		name     string
		setupCtx func(t *testing.T, session *discordgo.Session) (context.Context, context.CancelFunc)
		wantErr  string
	}{
		{
			name: "Success path",
			setupCtx: func(t *testing.T, session *discordgo.Session) (context.Context, context.CancelFunc) {
				var capturedHandler func(*discordgo.Session, *discordgo.Ready)
				ctx := context.WithValue(context.Background(), openSessionKey, func(s *discordgo.Session) error {
					if capturedHandler == nil {
						t.Fatalf("handler not registered")
					}
					capturedHandler(s, &discordgo.Ready{})
					return nil
				})
				ctx = context.WithValue(ctx, closeSessionKey, func(s *discordgo.Session) error {
					t.Fatalf("closeSession should not be called on success")
					return nil
				})
				ctx = context.WithValue(ctx, addHandlerOnceKey, func(s *discordgo.Session, h interface{}) func() {
					capturedHandler = h.(func(*discordgo.Session, *discordgo.Ready))
					return func() {}
				})
				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				return ctx, cancel
			},
			wantErr: "",
		},
		{
			name: "OpenSession failure",
			setupCtx: func(t *testing.T, session *discordgo.Session) (context.Context, context.CancelFunc) {
				ctx := context.WithValue(context.Background(), openSessionKey, func(s *discordgo.Session) error {
					return errors.New("open error")
				})
				ctx = context.WithValue(ctx, addHandlerOnceKey, func(s *discordgo.Session, h interface{}) func() {
					return func() {}
				})
				ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
				return ctx, cancel
			},
			wantErr: "open error",
		},
		{
			name: "Context timeout/cancellation",
			setupCtx: func(t *testing.T, session *discordgo.Session) (context.Context, context.CancelFunc) {
				ctx := context.WithValue(context.Background(), openSessionKey, func(s *discordgo.Session) error {
					return nil
				})
				ctx = context.WithValue(ctx, closeSessionKey, func(s *discordgo.Session) error {
					return nil
				})
				ctx = context.WithValue(ctx, addHandlerOnceKey, func(s *discordgo.Session, h interface{}) func() {
					return func() {}
				})
				ctx, cancel := context.WithCancel(ctx)
				cancel()
				return ctx, cancel
			},
			wantErr: "handshake timed out or canceled",
		},
		{
			name: "Nil session",
			setupCtx: func(t *testing.T, session *discordgo.Session) (context.Context, context.CancelFunc) {
				return context.Background(), func() {}
			},
			wantErr: "session is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var session *discordgo.Session
			if tt.name != "Nil session" {
				session = &discordgo.Session{}
			}

			ctx, cancel := tt.setupCtx(t, session)
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
