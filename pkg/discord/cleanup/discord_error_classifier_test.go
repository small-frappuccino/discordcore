package cleanup

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

func TestClassifyFetchError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want FailureClass
	}{
		{name: "nil", err: nil, want: FailureClassUnknown},
		{name: "plain network error", err: errors.New("dial tcp: timeout"), want: FailureClassTransient},
		// 404 on a fetch targets the channel, not a single message.
		{name: "404 response", err: makeRESTError(http.StatusNotFound, 0, "not found"), want: FailureClassMissingChannel},
		{name: "unknown channel code", err: makeRESTError(0, discordgo.ErrCodeUnknownChannel, "unknown channel"), want: FailureClassMissingChannel},
		{name: "403 response", err: makeRESTError(http.StatusForbidden, 0, "forbidden"), want: FailureClassForbidden},
		{name: "401 response", err: makeRESTError(http.StatusUnauthorized, 0, "unauthorized"), want: FailureClassForbidden},
		{name: "missing access code", err: makeRESTError(0, discordgo.ErrCodeMissingAccess, "missing access"), want: FailureClassForbidden},
		{name: "missing permissions code", err: makeRESTError(0, discordgo.ErrCodeMissingPermissions, "missing permissions"), want: FailureClassForbidden},
		{name: "rate limited", err: makeRESTError(http.StatusTooManyRequests, 0, "rate limited"), want: FailureClassRateLimited},
		{name: "5xx response", err: makeRESTError(http.StatusInternalServerError, 0, "boom"), want: FailureClassTransient},
		{name: "wrapped 404", err: fmt.Errorf("fetch failed: %w", makeRESTError(http.StatusNotFound, 0, "gone")), want: FailureClassMissingChannel},
		// Unknown-Message and bulk-delete-age never come back from a fetch
		// endpoint and should fall to the generic bucket so callers do not
		// silently treat them as "channel ok, message gone".
		{name: "unknown message code unrelated to fetch", err: makeRESTError(0, discordgo.ErrCodeUnknownMessage, "unknown message"), want: FailureClassUnknown},
		{name: "bulk delete age unrelated to fetch", err: makeRESTError(http.StatusBadRequest, discordgo.ErrCodeMessageProvidedTooOldForBulkDelete, "older than 14 days"), want: FailureClassUnknown},
		{name: "unmapped client error", err: makeRESTError(http.StatusBadRequest, 99999, "unknown"), want: FailureClassUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyFetchError(tt.err); got != tt.want {
				t.Fatalf("ClassifyFetchError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

func TestClassifyDeleteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want FailureClass
	}{
		{name: "nil", err: nil, want: FailureClassUnknown},
		{name: "plain network error", err: errors.New("dial tcp: timeout"), want: FailureClassTransient},
		{name: "404 response", err: makeRESTError(http.StatusNotFound, 0, "not found"), want: FailureClassMissingMessage},
		{name: "unknown message code", err: makeRESTError(0, discordgo.ErrCodeUnknownMessage, "unknown message"), want: FailureClassMissingMessage},
		{name: "unknown channel code", err: makeRESTError(0, discordgo.ErrCodeUnknownChannel, "unknown channel"), want: FailureClassMissingChannel},
		{name: "403 response", err: makeRESTError(http.StatusForbidden, 0, "forbidden"), want: FailureClassForbidden},
		{name: "401 response", err: makeRESTError(http.StatusUnauthorized, 0, "unauthorized"), want: FailureClassForbidden},
		{name: "missing access code", err: makeRESTError(0, discordgo.ErrCodeMissingAccess, "missing access"), want: FailureClassForbidden},
		{name: "missing permissions code", err: makeRESTError(0, discordgo.ErrCodeMissingPermissions, "missing permissions"), want: FailureClassForbidden},
		{name: "bulk delete age code", err: makeRESTError(http.StatusBadRequest, discordgo.ErrCodeMessageProvidedTooOldForBulkDelete, "older than 14 days"), want: FailureClassBulkDeleteAge},
		{name: "rate limited", err: makeRESTError(http.StatusTooManyRequests, 0, "rate limited"), want: FailureClassRateLimited},
		{name: "5xx response", err: makeRESTError(http.StatusInternalServerError, 0, "boom"), want: FailureClassTransient},
		{name: "wrapped 404", err: fmt.Errorf("delete failed: %w", makeRESTError(http.StatusNotFound, 0, "gone")), want: FailureClassMissingMessage},
		{name: "wrapped age", err: fmt.Errorf("bulk failed: %w", makeRESTError(0, discordgo.ErrCodeMessageProvidedTooOldForBulkDelete, "older than 14 days")), want: FailureClassBulkDeleteAge},
		{name: "unmapped client error", err: makeRESTError(http.StatusBadRequest, 99999, "unknown client error"), want: FailureClassUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClassifyDeleteError(tt.err); got != tt.want {
				t.Fatalf("ClassifyDeleteError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
