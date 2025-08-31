package discordcore

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrorHandler provides utilities for consistent error handling
type ErrorHandler struct {
	logger *Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *Logger) *ErrorHandler {
	return &ErrorHandler{
		logger: logger,
	}
}

// HandleValidationError wraps a function with validation error handling
func (eh *ErrorHandler) HandleValidationError(field string, fn func() error) error {
	if err := fn(); err != nil {
		eh.WithFields(map[string]interface{}{
			"field": field,
			"error": err,
		}).Error(ErrValidationFailed)
		return NewValidationError(field, nil, err.Error())
	}
	return nil
}

// HandleConfigError wraps configuration operations with consistent error handling
func (eh *ErrorHandler) HandleConfigError(operation, path string, fn func() error) error {
	if err := fn(); err != nil {
		configErr := NewConfigError(operation, path, err)
		eh.WithFields(map[string]interface{}{
			"operation": operation,
			"path":      path,
			"error":     err,
		}).Error(ErrConfigOperationFailed)
		return configErr
	}
	return nil
}

// HandleDiscordError wraps Discord API operations with error handling
func (eh *ErrorHandler) HandleDiscordError(operation string, fn func() error) error {
	if err := fn(); err != nil {
		discordErr := NewDiscordError(operation, 0, err.Error(), err)
		eh.WithFields(map[string]interface{}{
			"operation": operation,
			"error":     err,
		}).Error(ErrDiscordOperationFailed)
		return discordErr
	}
	return nil
}

// RetryOperation executes an operation with retry logic
func (eh *ErrorHandler) RetryOperation(ctx context.Context, operation string, maxAttempts int, fn func() error) error {
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err

			// Check if error is retryable
			if !IsRetryableError(err) {
				eh.WithFields(map[string]interface{}{
					"operation": operation,
					"attempt":   attempt,
					"error":     err,
				}).Error(ErrNonRetryable)
				return fmt.Errorf(ErrFmtNonRetryable, operation, err)
			}

			if attempt < maxAttempts {
				backoff := time.Duration(attempt) * time.Second
				eh.WithFields(map[string]interface{}{
					"operation":   operation,
					"attempt":     attempt,
					"maxAttempts": maxAttempts,
					"backoff":     backoff,
					"error":       err,
				}).Warn(MsgOperationRetrying)

				select {
				case <-ctx.Done():
					return fmt.Errorf(ErrFmtOperationCancelled, operation, ctx.Err())
				case <-time.After(backoff):
					continue
				}
			}
		} else {
			if attempt > 1 {
				eh.WithFields(map[string]interface{}{
					"operation": operation,
					"attempts":  attempt,
				}).Info(MsgOperationSucceededAfterRetry)
			}
			return nil
		}
	}

	eh.WithFields(map[string]interface{}{
		"operation": operation,
		"attempts":  maxAttempts,
		"lastError": lastErr,
	}).Error(MsgOperationFailedAllRetries)
	return fmt.Errorf(ErrFmtOperationFailedAfterRetries, operation, maxAttempts, lastErr)
}

// HandleWithCleanup executes a function with cleanup on error
func (eh *ErrorHandler) HandleWithCleanup(operation string, fn func() error, cleanup func()) error {
	if err := fn(); err != nil {
		eh.WithFields(map[string]interface{}{
			"operation": operation,
			"error":     err,
		}).Debug(MsgOperationFailedCleanup)
		cleanup()
		return err
	}
	return nil
}

// LogAndWrapError logs an error with context and wraps it with additional message
func (eh *ErrorHandler) LogAndWrapError(err error, operation string, fields map[string]interface{}) error {
	if err == nil {
		return nil
	}

	logFields := map[string]interface{}{
		"operation": operation,
		"error":     err,
	}

	// Merge additional fields
	for k, v := range fields {
		logFields[k] = v
	}

	eh.WithFields(logFields).Error(ErrOperationFailed)
	return fmt.Errorf(ErrFmtOperationFailed, operation, err)
}

// EnsureSuccess panics if the error is not nil - use only for critical operations
func (eh *ErrorHandler) EnsureSuccess(err error, operation string) {
	if err != nil {
		eh.WithFields(map[string]interface{}{
			"operation": operation,
			"error":     err,
		}).Fatal(MsgCriticalOperationFailed)
	}
}

// Add the WithFields method to the ErrorHandler struct
func (eh *ErrorHandler) WithFields(fields map[string]interface{}) *Logger {
	if eh.logger != nil {
		return eh.logger.WithFields(fields)
	}
	return &Logger{} // Return a no-op logger if eh.logger is nil
}

// Global error handler instance
var GlobalErrorHandler *ErrorHandler

// InitializeGlobalErrorHandler initializes the global error handler
func InitializeGlobalErrorHandler(logger *Logger) error {
	if logger == nil {
		return errors.New(ErrGlobalLoggerNotInitialized)
	}
	GlobalErrorHandler = NewErrorHandler(logger)
	return nil
}

// Convenience functions using global error handler
func HandleValidationError(field string, fn func() error) error {
	if GlobalErrorHandler != nil {
		return GlobalErrorHandler.HandleValidationError(field, fn)
	}
	return fn()
}

func HandleConfigError(operation, path string, fn func() error) error {
	if GlobalErrorHandler != nil {
		return GlobalErrorHandler.HandleConfigError(operation, path, fn)
	}
	return fn()
}

func HandleDiscordError(operation string, fn func() error) error {
	if GlobalErrorHandler != nil {
		return GlobalErrorHandler.HandleDiscordError(operation, fn)
	}
	return fn()
}

func RetryOperation(ctx context.Context, operation string, maxAttempts int, fn func() error) error {
	if GlobalErrorHandler != nil {
		return GlobalErrorHandler.RetryOperation(ctx, operation, maxAttempts, fn)
	}
	return fn()
}

func LogAndWrapError(err error, operation string, fields map[string]interface{}) error {
	if GlobalErrorHandler != nil {
		return GlobalErrorHandler.LogAndWrapError(err, operation, fields)
	}
	return err
}

func EnsureSuccess(err error, operation string) {
	if GlobalErrorHandler != nil {
		GlobalErrorHandler.EnsureSuccess(err, operation)
	} else if err != nil {
		panic(fmt.Sprintf(ErrFmtPanicCriticalOperationFailed, operation, err))
	}
}

// ----- Retry -----

func (rm *RetryManager) ExecuteWithRetry(operation func() error, operationName string) error {
	var lastErr error

	for attempt := 0; attempt <= rm.maxRetries; attempt++ {
		if attempt > 0 {
			delay := rm.calculateDelay(attempt)
			rm.logger.WithField("attempt", attempt+1).
				WithField("max_attempts", rm.maxRetries+1).
				WithField("operation", operationName).
				WithField("delay", delay).
				Infof("Attempt %d/%d for %s in %v", attempt+1, rm.maxRetries+1, operationName, delay)
			time.Sleep(delay)
		}

		err := operation()
		if err == nil {
			if attempt > 0 {
				rm.logger.WithField("operation", operationName).
					WithField("attempts_used", attempt+1).
					Infof("Operation %s succeeded after %d attempts", operationName, attempt+1)
			}
			return nil
		}

		lastErr = err
		rm.logger.WithField("operation", operationName).
			WithField("attempt", attempt+1).
			ErrorWithErr(fmt.Sprintf(ErrOnAttempt, attempt+1, operationName), err)
	}

	return fmt.Errorf(ErrOperationAttemptsFailed, operationName, rm.maxRetries+1, lastErr)
}

func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	delay := time.Duration(float64(rm.baseDelay) * float64(attempt) * rm.backoffFactor)
	if delay > rm.maxDelay {
		delay = rm.maxDelay
	}
	return delay
}

func (rm *RetryManager) SetMaxRetries(retries int) {
	rm.maxRetries = retries
}

func (rm *RetryManager) SetBaseDelay(delay time.Duration) {
	rm.baseDelay = delay
}

func (rm *RetryManager) SetMaxDelay(delay time.Duration) {
	rm.maxDelay = delay
}

// Modify the existing RetryManager struct to include a logger field
type RetryManager struct {
	maxRetries    int
	baseDelay     time.Duration
	maxDelay      time.Duration
	backoffFactor float64
	logger        *Logger
}

// Update the existing NewRetryManager function to initialize the logger
func NewRetryManager(maxRetries int, baseDelay, maxDelay time.Duration, backoffFactor float64, logger *Logger) *RetryManager {
	return &RetryManager{
		maxRetries:    maxRetries,
		baseDelay:     baseDelay,
		maxDelay:      maxDelay,
		backoffFactor: backoffFactor,
		logger:        logger,
	}
}
