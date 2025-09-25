package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Logger is a lightweight structured logger that uses the standard library under the hood.
// It provides a small API compatible with the previous external `logutil` usages in the codebase.
type Logger struct {
	mu     sync.Mutex
	fields map[string]any
	std    *log.Logger
}

var (
	// GlobalLogger is the package-level logger used by convenience functions and by other packages.
	GlobalLogger *Logger

	// setupOnce ensures the global logger is initialized only once in a thread-safe manner.
	setupOnce sync.Once
)

// SetupLogger initializes the global logger. It is idempotent and thread-safe.
func SetupLogger() error {
	setupOnce.Do(func() {
		l := log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
		GlobalLogger = &Logger{
			fields: make(map[string]any),
			std:    l,
		}
	})
	return nil
}

// CloseGlobalLogger flushes or closes any logger resources. For this implementation it's a no-op
// but kept for compatibility with previous behavior.
func CloseGlobalLogger() error { return nil }

// NewLogger creates a new logger instance (not used widely, but exported for completeness).
func NewLogger() *Logger {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	// Create a fresh logger that shares the same underlying std logger but has independent fields.
	return &Logger{fields: make(map[string]any), std: GlobalLogger.std}
}

// normalizeValue converts certain common types into forms that marshal well to JSON.
// - errors -> their Error() string
// - time.Time (and *time.Time) -> RFC3339Nano string
// - fmt.Stringer -> String()
// - nested maps and slices are normalized recursively
func normalizeValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case error:
		if x == nil {
			return nil
		}
		return x.Error()
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case *time.Time:
		if x == nil {
			return nil
		}
		return x.Format(time.RFC3339Nano)
	case fmt.Stringer:
		if x == nil {
			return nil
		}
		return x.String()
	case map[string]any:
		n := make(map[string]any, len(x))
		for k, vv := range x {
			n[k] = normalizeValue(vv)
		}
		return n
	case []any:
		s := make([]any, len(x))
		for i, vv := range x {
			s[i] = normalizeValue(vv)
		}
		return s
	default:
		return v
	}
}

func normalizeFields(fields map[string]any) map[string]any {
	n := make(map[string]any, len(fields))
	for k, v := range fields {
		n[k] = normalizeValue(v)
	}
	return n
}

// buildMessage composes the final log line including level and JSON-encoded fields when present.
func (l *Logger) buildMessage(level, msg string) string {
	// Copy fields under lock to avoid races when callers share the same Logger.
	l.mu.Lock()
	fieldsCopy := make(map[string]any, len(l.fields))
	for k, v := range l.fields {
		fieldsCopy[k] = v
	}
	l.mu.Unlock()

	if len(fieldsCopy) == 0 {
		return fmt.Sprintf("[%s] %s", level, msg)
	}

	normalized := normalizeFields(fieldsCopy)
	b, err := json.Marshal(normalized)
	if err != nil {
		// Fallback to Go-sprint of fields if JSON encoding fails
		return fmt.Sprintf("[%s] %s | fields=%v", level, msg, normalized)
	}
	return fmt.Sprintf("[%s] %s | %s", level, msg, string(b))
}

// WithField returns a new Logger with an additional field. It does not mutate the receiver.
func (l *Logger) WithField(key string, value any) *Logger {
	// Copy current fields under lock
	l.mu.Lock()
	newFields := make(map[string]any, len(l.fields)+1)
	for k, v := range l.fields {
		newFields[k] = v
	}
	l.mu.Unlock()

	newFields[key] = value
	return &Logger{fields: newFields, std: l.std}
}

// WithFields returns a new Logger with additional fields merged. It does not mutate the receiver.
func (l *Logger) WithFields(fields map[string]any) *Logger {
	l.mu.Lock()
	newFields := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	l.mu.Unlock()
	for k, v := range fields {
		newFields[k] = v
	}
	return &Logger{fields: newFields, std: l.std}
}

// WithError is a convenience that attaches an error as a string (nil-safe).
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return l.WithField("error", err.Error())
}

// ErrorWithErr logs a message along with an error on the Logger instance.
// It ensures nil-safety and records the error as a string for reliable JSON encoding.
func (l *Logger) ErrorWithErr(msg string, err error) {
	if err == nil {
		l.Error(msg)
		return
	}
	l.WithField("error", err.Error()).Error(msg)
}

// output writes the message to the underlying std logger.
func (l *Logger) output(level, msg string) {
	if l.std == nil {
		_ = SetupLogger()
	}
	l.std.Println(l.buildMessage(level, msg))
}

// Debug prints a debug-level message
func (l *Logger) Debug(msg string) { l.output("DEBUG", msg) }
func (l *Logger) Info(msg string)  { l.output("INFO", msg) }
func (l *Logger) Warn(msg string)  { l.output("WARN", msg) }
func (l *Logger) Error(msg string) { l.output("ERROR", msg) }

// Formatted variants
func (l *Logger) Debugf(format string, v ...any) { l.output("DEBUG", fmt.Sprintf(format, v...)) }
func (l *Logger) Infof(format string, v ...any)  { l.output("INFO", fmt.Sprintf(format, v...)) }
func (l *Logger) Warnf(format string, v ...any)  { l.output("WARN", fmt.Sprintf(format, v...)) }
func (l *Logger) Errorf(format string, v ...any) { l.output("ERROR", fmt.Sprintf(format, v...)) }

// Fatal logs and exits
func (l *Logger) Fatal(msg string) {
	l.output("FATAL", msg)
	// give a small grace for logs to be written
	time.Sleep(10 * time.Millisecond)
	os.Exit(1)
}
func (l *Logger) Fatalf(format string, v ...any) {
	l.output("FATAL", fmt.Sprintf(format, v...))
	time.Sleep(10 * time.Millisecond)
	os.Exit(1)
}

// Convenience top-level helpers that operate on the global logger.
// These match the previous external `logutil` API used across the codebase.
func Info(msg string) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Info(msg)
}
func Infof(f string, v ...any) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Infof(f, v...)
}
func Debug(msg string) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Debug(msg)
}
func Debugf(f string, v ...any) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Debugf(f, v...)
}
func Warn(msg string) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Warn(msg)
}
func Warnf(f string, v ...any) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Warnf(f, v...)
}
func Error(msg string) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Error(msg)
}
func Errorf(f string, v ...any) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Errorf(f, v...)
}
func Fatalf(f string, v ...any) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Fatalf(f, v...)
}
func Fatal(msg string) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	GlobalLogger.Fatal(msg)
}

// ErrorWithErr logs a message along with an error (keeps compatibility with previous code)
// It ensures the error is represented as a string for stable JSON serialization.
func ErrorWithErr(msg string, err error) {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	if err == nil {
		GlobalLogger.Error(msg)
		return
	}
	GlobalLogger.WithField("error", err.Error()).Error(msg)
}

// WithFieldsGlobal returns a logger built from the global logger with provided fields.
func WithFieldsGlobal(fields map[string]any) *Logger {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	return GlobalLogger.WithFields(fields)
}

// For backward compatibility this package also exposes WithFields as a top-level identifier.
var WithFields = WithFieldsGlobal

// WithField is a top-level helper to create a logger with a single field using the global logger.
func WithField(key string, value any) *Logger {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	return GlobalLogger.WithField(key, value)
}

// WithError is a top-level helper to attach an error (nil-safe) to the global logger.
// It ensures the error is represented as a string for reliable JSON encoding.
func WithError(err error) *Logger {
	if GlobalLogger == nil {
		_ = SetupLogger()
	}
	return GlobalLogger.WithError(err)
}

// flushWait is a small helper to let logs get flushed in short-lived programs or tests.
func flushWait() { time.Sleep(10 * time.Millisecond) }
