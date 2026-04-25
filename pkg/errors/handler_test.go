package errors

import (
	"context"
	stderrors "errors"
	"io"
	"slices"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type fakeTimeoutError struct{}

func (fakeTimeoutError) Error() string   { return "deadline exceeded" }
func (fakeTimeoutError) Timeout() bool   { return true }
func (fakeTimeoutError) Temporary() bool { return true }

type fakeCommandError struct {
	message string
	code    string
}

func (e fakeCommandError) Error() string {
	return e.message
}

func (e fakeCommandError) CommandErrorCode() string {
	return e.code
}

type fakeValidationError struct {
	field   string
	message string
}

func (e fakeValidationError) Error() string {
	return e.message
}

func (e fakeValidationError) ValidationField() string {
	return e.field
}

func TestNormalizeErrorUsesStructuredErrorContracts(t *testing.T) {
	t.Parallel()

	handler := NewErrorHandler()
	tests := []struct {
		name        string
		err         error
		category    ErrorCategory
		recoverable bool
		severity    ErrorSeverity
		actions     []ErrorAction
	}{
		{
			name:        "files validation",
			err:         files.NewValidationError("guilds[0].roles.auto_assignment.required_roles", []string{"stable-role"}, "required_roles must contain exactly 2 role IDs"),
			category:    CategoryValidation,
			recoverable: false,
			severity:    SeverityLow,
			actions:     []ErrorAction{ActionLog},
		},
		{
			name:        "files config",
			err:         files.NewConfigError("load", "config.json", io.EOF),
			category:    CategoryConfig,
			recoverable: false,
			severity:    SeverityMedium,
			actions:     []ErrorAction{ActionLog, ActionNotify},
		},
		{
			name:        "cache error",
			err:         cache.NewCacheError("load", "guild:g1", io.EOF),
			category:    CategoryCache,
			recoverable: true,
			severity:    SeverityLow,
			actions:     []ErrorAction{ActionLog},
		},
		{
			name:        "command error",
			err:         fakeCommandError{message: "guild only"},
			category:    CategoryCommand,
			recoverable: false,
			severity:    SeverityMedium,
			actions:     []ErrorAction{ActionLog},
		},
		{
			name:        "command validation",
			err:         fakeValidationError{field: "scope", message: "invalid scope"},
			category:    CategoryValidation,
			recoverable: false,
			severity:    SeverityLow,
			actions:     []ErrorAction{ActionLog},
		},
		{
			name:        "typed discord rate limit",
			err:         files.NewDiscordError("send_message", 429, "rate limited", io.EOF),
			category:    CategoryDiscord,
			recoverable: true,
			severity:    SeverityMedium,
			actions:     []ErrorAction{ActionLog, ActionRetry},
		},
		{
			name:        "typed discord forbidden",
			err:         files.NewDiscordError("send_message", 403, "forbidden", io.EOF),
			category:    CategoryDiscord,
			recoverable: false,
			severity:    SeverityHigh,
			actions:     []ErrorAction{ActionLog, ActionNotify},
		},
		{
			name:        "context deadline",
			err:         context.DeadlineExceeded,
			category:    CategoryNetwork,
			recoverable: true,
			severity:    SeverityMedium,
			actions:     []ErrorAction{ActionLog},
		},
		{
			name:        "timeout interface",
			err:         fakeTimeoutError{},
			category:    CategoryNetwork,
			recoverable: true,
			severity:    SeverityMedium,
			actions:     []ErrorAction{ActionLog},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := handler.normalizeError(tc.err)
			if got.Category != tc.category {
				t.Fatalf("normalizeError().Category = %q, want %q", got.Category, tc.category)
			}
			if got.Recoverable != tc.recoverable {
				t.Fatalf("normalizeError().Recoverable = %v, want %v", got.Recoverable, tc.recoverable)
			}
			if got.Severity != tc.severity {
				t.Fatalf("normalizeError().Severity = %q, want %q", got.Severity, tc.severity)
			}
			if !slices.Equal(got.Actions, tc.actions) {
				t.Fatalf("normalizeError().Actions = %v, want %v", got.Actions, tc.actions)
			}
		})
	}
}

func TestNormalizeErrorUnwrapsServiceError(t *testing.T) {
	t.Parallel()

	handler := NewErrorHandler()
	want := NewServiceError(CategoryConfig, SeverityHigh, "control", "load", "configuration failed", io.EOF)
	want.Recoverable = false
	want.Actions = []ErrorAction{ActionLog, ActionNotify}

	got := handler.normalizeError(stderrors.Join(io.ErrUnexpectedEOF, want))
	if got != want {
		t.Fatalf("normalizeError() did not reuse wrapped ServiceError")
	}
}
