package core

import "fmt"

// OperationalError signifies a structural failure scoped to a specific runtime operation.
// It wraps an underlying error, preserving context while exposing the exact operational
// boundary that collapsed (e.g. "handler_help", "dispatch.parse").
type OperationalError struct {
	Op  string
	Err error
}

// Error implements the standard error interface, yielding the composed error chain.
func (e *OperationalError) Error() string {
	return fmt.Sprintf("operation %s failed: %v", e.Op, e.Err)
}

// Unwrap supports the errors.Is and errors.As traversal mechanisms, exposing the base failure.
func (e *OperationalError) Unwrap() error {
	return e.Err
}

// ValidationError flags an invalid internal or external state preventing execution.
// It specifies the exact field and the reasoning to aid immediate failure resolution
// without triggering broader infrastructure alerts.
type ValidationError struct {
	Field  string
	Reason string
}

// Error implements the standard error interface, formatting the validation constraint.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Reason)
}
