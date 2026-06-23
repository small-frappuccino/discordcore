package log

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"runtime/debug"
)

// LazyStackTrace implements slog.LogValuer to defer debug.Stack() allocation
// until the log is actually emitted, preventing O(N) allocations in hot paths.
type LazyStackTrace struct{}

func (LazyStackTrace) LogValue() slog.Value {
	return slog.StringValue(string(debug.Stack()))
}

// GenerateRequestID produces a transient cryptographic identifier correlating logs and pages.
func GenerateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(bytes)
}

// EmitBlockingError injects structural metadata containing the stack trace and synthetic status 500.
func EmitBlockingError(msg string, err error, requestID string) {
	ErrorLoggerRaw().Error(msg,
		slog.String("request_id", requestID),
		slog.String("synthetic_code", "500"),
		slog.Any("stack_trace", LazyStackTrace{}),
		slog.Any("error", err),
	)
}
