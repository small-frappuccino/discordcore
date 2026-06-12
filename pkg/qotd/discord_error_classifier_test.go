package qotd

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsUnrecoverableDiscordPublishErrorTreatsClientErrorsAsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "unknown channel code", err: ErrDiscordUnknownChannel},
		{name: "missing access code", err: ErrDiscordMissingAccess},
		{name: "missing permissions code", err: ErrDiscordMissingPermissions},
		{name: "unknown guild code", err: ErrDiscordUnknownGuild},
		{name: "wrapped error", err: fmt.Errorf("publish failed: %w", ErrDiscordUnknownChannel)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !isUnrecoverableDiscordPublishError(tt.err) {
				t.Fatalf("expected error to be classified as unrecoverable: %v", tt.err)
			}
		})
	}
}

func TestIsUnmanageableDiscordThreadErrorMatchesPermissionRejections(t *testing.T) {
	t.Parallel()

	matchTests := []struct {
		name string
		err  error
	}{
		{name: "missing access code", err: ErrDiscordMissingAccess},
		{name: "missing permissions code", err: ErrDiscordMissingPermissions},
		{name: "wrapped error", err: fmt.Errorf("set qotd thread state: %w", ErrDiscordMissingAccess)},
	}
	for _, tt := range matchTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !isUnmanageableDiscordThreadError(tt.err) {
				t.Fatalf("expected error to classify as unmanageable thread: %v", tt.err)
			}
		})
	}

	skipTests := []struct {
		name string
		err  error
	}{
		{name: "nil", err: nil},
		{name: "plain network error", err: errors.New("dial tcp: timeout")},
		{name: "unknown channel code", err: ErrDiscordUnknownChannel},
	}
	for _, tt := range skipTests {
		t.Run("not_"+tt.name, func(t *testing.T) {
			t.Parallel()
			if isUnmanageableDiscordThreadError(tt.err) {
				t.Fatalf("expected error to NOT classify as unmanageable thread: %v", tt.err)
			}
		})
	}
}

func TestIsUnrecoverableDiscordPublishErrorLeavesTransientFailuresRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "nil", err: nil},
		{name: "plain error", err: errors.New("dial tcp: lookup discord.com: no such host")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if isUnrecoverableDiscordPublishError(tt.err) {
				t.Fatalf("expected error to stay retryable, got terminal: %v", tt.err)
			}
		})
	}
}
