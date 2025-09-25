package errutil

import (
	"fmt"
	"sync"

	logging "github.com/alice-bnuy/discordcore/pkg/logging"
)

// Minimal replacement for the previously external errutil package.
// Provides:
// - InitializeGlobalErrorHandler(logger *logging.Logger) error
// - HandleDiscordError(operation string, fn func() error) error
// - HandleConfigError(operation, path string, fn func() error) error
//
// The implementations are intentionally minimal: they run the provided
// function, log the error with the provided global logger (if initialized),
// and return a wrapped/formatted error where appropriate.

var (
	mu     sync.RWMutex
	logger *logging.Logger
)

// InitializeGlobalErrorHandler sets the package-level logger used by the error helpers.
// It is safe to call multiple times; the last non-nil logger wins.
// Returns an error if the supplied logger is nil.
func InitializeGlobalErrorHandler(l *logging.Logger) error {
	if l == nil {
		return fmt.Errorf("nil logger provided")
	}
	mu.Lock()
	logger = l
	mu.Unlock()
	return nil
}

// HandleDiscordError executes fn and logs any error that occurs as a Discord-related error.
// It returns whatever error fn returns (unmodified), after logging it.
func HandleDiscordError(operation string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("nil function provided")
	}

	err := fn()
	if err == nil {
		return nil
	}

	mu.RLock()
	l := logger
	mu.RUnlock()

	if l != nil {
		l.WithField("operation", operation).ErrorWithErr("Discord operation failed", err)
	} else {
		// Best-effort fallback to package-level helper in logging (if available).
		// This ensures some logging even if InitializeGlobalErrorHandler wasn't called.
		logging.ErrorWithErr(fmt.Sprintf("Discord operation failed: %s", operation), err)
	}

	return err
}

// HandleConfigError executes fn and logs any error that occurs as a configuration-related error.
// It returns a wrapped error with context about the operation and path.
func HandleConfigError(operation, path string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("nil function provided")
	}

	err := fn()
	if err == nil {
		return nil
	}

	mu.RLock()
	l := logger
	mu.RUnlock()

	if l != nil {
		l.WithFields(map[string]any{
			"operation": operation,
			"path":      path,
		}).ErrorWithErr("Config operation failed", err)
	} else {
		logging.ErrorWithErr(fmt.Sprintf("Config operation failed: %s %s", operation, path), err)
	}

	return fmt.Errorf("config %s %s: %w", operation, path, err)
}
