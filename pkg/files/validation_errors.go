package files

import (
	"errors"
	"fmt"
)

var errValidationFailure = errors.New(ErrValidationFailed)

// IsValidationError reports whether err carries config validation context.
func IsValidationError(err error) bool {
	if err == nil {
		return false
	}

	var validationErr ValidationError
	if errors.As(err, &validationErr) {
		return true
	}

	var validationErrPtr *ValidationError
	return errors.Is(err, errValidationFailure) || errors.As(err, &validationErrPtr)
}

func wrapValidationError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", errValidationFailure, err)
}
