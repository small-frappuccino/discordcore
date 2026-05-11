package qotd

import (
	"errors"
	"strings"
	"testing"
)

func TestOfficialPostStateDivergenceErrorErrorIncludesIDAndIntent(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial postgres: connection refused")
	err := &OfficialPostStateDivergenceError{
		OfficialPostID: 42,
		IntendedState:  OfficialPostStatePrevious,
		Cause:          cause,
	}

	msg := err.Error()
	for _, snippet := range []string{"42", string(OfficialPostStatePrevious), cause.Error()} {
		if !strings.Contains(msg, snippet) {
			t.Fatalf("expected divergence error to mention %q, got %q", snippet, msg)
		}
	}
}

// TestOfficialPostStateDivergenceErrorErrorWithoutCauseStillSurfacesContext
// pins that the structured-log payload remains readable when the underlying
// cause is nil (which can happen if a caller constructs the error
// directly for a test or fault-injection scenario).
func TestOfficialPostStateDivergenceErrorErrorWithoutCauseStillSurfacesContext(t *testing.T) {
	t.Parallel()

	err := &OfficialPostStateDivergenceError{
		OfficialPostID: 7,
		IntendedState:  OfficialPostStateArchived,
	}

	msg := err.Error()
	if !strings.Contains(msg, "7") {
		t.Fatalf("expected error to mention post id even without cause, got %q", msg)
	}
	if !strings.Contains(msg, string(OfficialPostStateArchived)) {
		t.Fatalf("expected error to mention intended state even without cause, got %q", msg)
	}
	if strings.HasSuffix(msg, ": ") {
		t.Fatalf("expected error message to not trail on an empty cause, got %q", msg)
	}
}

func TestOfficialPostStateDivergenceErrorErrorRendersUnknownStateSentinel(t *testing.T) {
	t.Parallel()

	err := &OfficialPostStateDivergenceError{OfficialPostID: 1}
	if !strings.Contains(err.Error(), "<unknown>") {
		t.Fatalf("expected empty intended state to render as <unknown>, got %q", err.Error())
	}
}

func TestOfficialPostStateDivergenceErrorErrorOnNilReceiverReturnsEmpty(t *testing.T) {
	t.Parallel()

	var err *OfficialPostStateDivergenceError
	if got := err.Error(); got != "" {
		t.Fatalf("expected nil receiver Error() to return empty string, got %q", got)
	}
	if got := err.Unwrap(); got != nil {
		t.Fatalf("expected nil receiver Unwrap() to return nil, got %v", got)
	}
}

func TestOfficialPostStateDivergenceErrorMatchesSentinelViaErrorsIs(t *testing.T) {
	t.Parallel()

	cause := errors.New("write conflict")
	err := &OfficialPostStateDivergenceError{
		OfficialPostID: 99,
		IntendedState:  OfficialPostStateCurrent,
		Cause:          cause,
	}

	if !errors.Is(err, ErrOfficialPostStateDivergence) {
		t.Fatal("expected divergence error to match ErrOfficialPostStateDivergence sentinel")
	}
	if !errors.Is(err, cause) {
		t.Fatal("expected divergence error to expose the underlying cause via errors.Is")
	}
}

func TestOfficialPostStateDivergenceErrorMatchesSentinelEvenWithoutCause(t *testing.T) {
	t.Parallel()

	err := &OfficialPostStateDivergenceError{OfficialPostID: 1, IntendedState: OfficialPostStatePrevious}
	if !errors.Is(err, ErrOfficialPostStateDivergence) {
		t.Fatal("expected divergence error without cause to still match the sentinel")
	}
}
