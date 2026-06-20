package log

import "log/slog"

// SetErrorLoggerRawForTest overrides the raw error logger and returns a cleanup function.
// This is strictly for use in tests.
func SetErrorLoggerRawForTest(logger *slog.Logger) func() {
	if globalLogger == nil {
		globalLogger = &Logger{}
	}
	old := globalLogger.error
	globalLogger.error = logger
	return func() {
		globalLogger.error = old
	}
}
