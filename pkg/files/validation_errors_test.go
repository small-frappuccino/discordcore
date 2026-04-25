package files

import "testing"

func TestIsValidationErrorMatchesWrappedValidation(t *testing.T) {
	t.Parallel()

	err := wrapValidationError(NewValidationError(
		"guilds[0].roles.auto_assignment.required_roles",
		[]string{"stable-role"},
		"required_roles must contain exactly 2 role IDs",
	))
	if !IsValidationError(err) {
		t.Fatalf("expected wrapped validation error to be detected, got %v", err)
	}
}
