package core

import "fmt"

type OperationalError struct {
	Op  string
	Err error
}

func (e *OperationalError) Error() string {
	return fmt.Sprintf("operation %s failed: %v", e.Op, e.Err)
}

func (e *OperationalError) Unwrap() error {
	return e.Err
}

type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation failed on %s: %s", e.Field, e.Reason)
}
