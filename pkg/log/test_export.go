package log

import (
	"log/slog"
	"runtime"
	"strings"
)

// SetErrorLoggerRawForTest overrides the raw error logger and returns a cleanup function.
// This is strictly for use in tests.
func SetErrorLoggerRawForTest(logger *slog.Logger) func() {
	var pcs [16]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	var testName string
	for {
		frame, more := frames.Next()
		if strings.Contains(frame.Function, ".Test") {
			idx := strings.LastIndex(frame.Function, ".")
			if idx != -1 {
				testName = frame.Function[idx+1:]
				if idxSlash := strings.Index(testName, "/"); idxSlash != -1 {
					testName = testName[:idxSlash]
				}
				break
			}
		}
		if !more {
			break
		}
	}

	if testName != "" {
		testRawErrorLogger.Store(testName, logger)
		return func() {
			testRawErrorLogger.Delete(testName)
		}
	}

	if globalLogger == nil {
		globalLogger = &Logger{}
	}
	old := globalLogger.error
	globalLogger.error = logger
	return func() {
		globalLogger.error = old
	}
}
