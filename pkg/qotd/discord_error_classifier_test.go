package qotd

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/small-frappuccino/discordgo"
)

func makeRESTError(statusCode int, code int, message string) *discordgo.RESTError {
	restErr := &discordgo.RESTError{}
	if statusCode != 0 {
		restErr.Response = &http.Response{StatusCode: statusCode}
	}
	if code != 0 || message != "" {
		restErr.Message = &discordgo.APIErrorMessage{Code: code, Message: message}
	}
	return restErr
}

func TestIsUnrecoverableDiscordPublishErrorTreatsClientErrorsAsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{name: "404 from response", err: makeRESTError(http.StatusNotFound, 0, "not found")},
		{name: "403 from response", err: makeRESTError(http.StatusForbidden, 0, "forbidden")},
		{name: "401 from response", err: makeRESTError(http.StatusUnauthorized, 0, "unauthorized")},
		{name: "unknown channel code", err: makeRESTError(0, discordgo.ErrCodeUnknownChannel, "unknown channel")},
		{name: "missing access code", err: makeRESTError(0, discordgo.ErrCodeMissingAccess, "missing access")},
		{name: "missing permissions code", err: makeRESTError(0, discordgo.ErrCodeMissingPermissions, "missing permissions")},
		{name: "unknown guild code", err: makeRESTError(0, discordgo.ErrCodeUnknownGuild, "unknown guild")},
		{name: "wrapped 404", err: fmt.Errorf("publish failed: %w", makeRESTError(http.StatusNotFound, 0, "gone"))},
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
		{name: "403 from response", err: makeRESTError(http.StatusForbidden, 0, "forbidden")},
		{name: "missing access code", err: makeRESTError(0, discordgo.ErrCodeMissingAccess, "missing access")},
		{name: "missing permissions code", err: makeRESTError(0, discordgo.ErrCodeMissingPermissions, "missing permissions")},
		{name: "wrapped 403", err: fmt.Errorf("set qotd thread state: %w", makeRESTError(http.StatusForbidden, 0, "forbidden"))},
	}
	for _, tt := range matchTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !isUnmanageableDiscordThreadError(tt.err) {
				t.Fatalf("expected error to classify as unmanageable thread: %v", tt.err)
			}
		})
	}

	// Unrelated errors must NOT degrade silently — those should still bubble
	// up so the publish/reconcile path retries or surfaces them. In
	// particular, 404/Unknown Channel is "thread missing", a separate branch
	// that flips the post to OfficialPostStateMissingDiscord.
	skipTests := []struct {
		name string
		err  error
	}{
		{name: "nil", err: nil},
		{name: "plain network error", err: errors.New("dial tcp: timeout")},
		{name: "404 missing thread", err: makeRESTError(http.StatusNotFound, 0, "not found")},
		{name: "unknown channel code", err: makeRESTError(0, discordgo.ErrCodeUnknownChannel, "unknown channel")},
		{name: "5xx response", err: makeRESTError(http.StatusInternalServerError, 0, "boom")},
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
		{name: "5xx response", err: makeRESTError(http.StatusInternalServerError, 0, "internal error")},
		{name: "rate limited", err: makeRESTError(http.StatusTooManyRequests, 0, "rate limited")},
		{name: "unmapped client error code", err: makeRESTError(0, 50034, "bulk delete out of range")},
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
