package cleanup

import (
	"errors"
	"net/http"

	"github.com/diamondburned/arikawa/v3/utils/httputil"
)

// FailureClass labels a Discord delete failure so callers can branch on the
// underlying cause (counters, log dedup, user-facing messages) instead of
// lumping every failure into one bucket.
//
// The classes mirror the load-bearing distinctions documented in the QOTD
// runtime classifiers: missing target, lost permission, age window, rate
// limit, and transient. Anything that does not match a known class falls
// back to FailureClassUnknown so the caller decides how to surface it.
type FailureClass int

// FailureClassUnknown defines failure class unknown.
const (
	FailureClassUnknown FailureClass = iota
	// FailureClassMissingMessage means the message no longer exists
	// (404 / Unknown Message). The cleanup goal is effectively achieved.
	FailureClassMissingMessage
	// FailureClassMissingChannel means the channel no longer exists
	// (404 / Unknown Channel). All subsequent deletes will fail the same way.
	FailureClassMissingChannel
	// FailureClassForbidden means the bot lost the permissions required
	// to delete here (403 / Missing Access / Missing Permissions).
	FailureClassForbidden
	// FailureClassBulkDeleteAge means Discord rejected a bulk-delete
	// chunk because at least one message is older than 14 days
	// (code 50034). Caller should fall back to per-message single delete.
	FailureClassBulkDeleteAge
	// FailureClassRateLimited means Discord throttled the request (429).
	FailureClassRateLimited
	// FailureClassTransient covers 5xx responses and bare network errors
	// where retrying later is the right move.
	FailureClassTransient
)

// ClassifyDeleteError maps a Discord REST error returned by message-delete
// flows to a FailureClass. Wrapped errors are unwrapped via errors.As.
func ClassifyDeleteError(err error) FailureClass {
	if err == nil {
		return FailureClassUnknown
	}
	var restErr *httputil.HTTPError
	if !errors.As(err, &restErr) || restErr == nil {
		return FailureClassTransient
	}

	switch restErr.Code {
	case 10008: // Unknown Message
		return FailureClassMissingMessage
	case 10003: // Unknown Channel
		return FailureClassMissingChannel
	case 50001, 50013: // Missing Access, Missing Permissions
		return FailureClassForbidden
	case 50034: // Message provided was too old to bulk delete
		return FailureClassBulkDeleteAge
	}

	switch restErr.Status {
	case http.StatusNotFound:
		return FailureClassMissingMessage
	case http.StatusForbidden, http.StatusUnauthorized:
		return FailureClassForbidden
	case http.StatusTooManyRequests:
		return FailureClassRateLimited
	}
	if restErr.Status >= 500 {
		return FailureClassTransient
	}

	return FailureClassUnknown
}

// ClassifyFetchError maps a Discord REST error returned by message-fetch
// flows (ChannelMessages and similar listing endpoints) to a FailureClass.
//
// Diverges from ClassifyDeleteError on the 404 axis: a 404 on a fetch
// targets the channel itself, not a single message, so it surfaces as
// FailureClassMissingChannel rather than FailureClassMissingMessage.
// Bulk-age (50034) and ErrCodeUnknownMessage do not occur on fetch and
// are intentionally absent. Wrapped errors are unwrapped via errors.As.
func ClassifyFetchError(err error) FailureClass {
	if err == nil {
		return FailureClassUnknown
	}
	var restErr *httputil.HTTPError
	if !errors.As(err, &restErr) || restErr == nil {
		return FailureClassTransient
	}

	switch restErr.Code {
	case 10003: // Unknown Channel
		return FailureClassMissingChannel
	case 50001, 50013: // Missing Access, Missing Permissions
		return FailureClassForbidden
	}

	switch restErr.Status {
	case http.StatusNotFound:
		return FailureClassMissingChannel
	case http.StatusForbidden, http.StatusUnauthorized:
		return FailureClassForbidden
	case http.StatusTooManyRequests:
		return FailureClassRateLimited
	}
	if restErr.Status >= 500 {
		return FailureClassTransient
	}

	return FailureClassUnknown
}
