package errutil

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// Minimal replacement for the previously external errutil package.
// Provides:
// - InitializeGlobalErrorHandler(logger *log.Logger) error
// - HandleDiscordError(operation string, fn func() error) error
// - HandleConfigError(operation, path string, fn func() error) error
//
// The implementations are intentionally minimal: they run the provided
// function, log the error with the provided global logger (if initialized),
// and return a wrapped/formatted error where appropriate.

var (
	mu     sync.RWMutex
	logger *log.Logger
)

// InitializeGlobalErrorHandler sets the package-level logger used by the error helpers.
// It is safe to call multiple times; the last non-nil logger wins.
// Returns an error if the supplied logger is nil.
func InitializeGlobalErrorHandler(l *log.Logger) error {
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
		slog.Error("Discord operation failed", "operation", operation, "error", err)
	} else {
		// Best-effort fallback to package-level helper in logging (if available).
		// This ensures some logging even if InitializeGlobalErrorHandler wasn't called.
		slog.Error("Discord operation failed", "operation", operation, "error", err)
	}

	return err
}

// Wrap returns nil when err is nil; otherwise returns fmt.Errorf("%s: %w", prefix, err).
// It is intended for use in a deferred closure that mutates a named error return,
// so that every error path of a function picks up a single operation prefix without
// having to repeat it at each fmt.Errorf call site:
//
//	func Op() (err error) {
//	    defer func() { err = errutil.Wrap(err, "op") }()
//	    ...
//	}
func Wrap(err error, prefix string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// Wrapf is the formatting variant of [Wrap]: the prefix is built from format and args.
// It returns nil when err is nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
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
		slog.Error("Config operation failed", "operation", operation, "path", path, "error", err)
	} else {
		slog.Error("Config operation failed", "operation", operation, "path", path, "error", err)
	}

	return fmt.Errorf("config %s %s: %w", operation, path, err)
}
