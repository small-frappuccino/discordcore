package messages

import (
	"sync/atomic"
)

// Metrics is the observability seam the messages service writes through.
type Metrics interface {
	RecordMessageSent()
}

// SnapshotProvider is the optional capability the /v1/health/messages handler looks for.
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is the JSON payload /v1/health/messages returns.
type MetricsSnapshot struct {
	MessagesSentTotal int64 `json:"messages_sent_total"`
}

// NopMetrics is the default implementation when the service is constructed without explicit metrics wiring.
type NopMetrics struct{}

func (NopMetrics) RecordMessageSent() {}

// InMemoryMetrics is the lightweight implementation backing /v1/health/messages.
type InMemoryMetrics struct {
	messagesSent atomic.Int64
}

// NewInMemoryMetrics constructs the production metrics implementation.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{}
}

func (m *InMemoryMetrics) RecordMessageSent() { m.messagesSent.Add(1) }

// Snapshot returns a JSON-friendly view of the current counter state.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	return MetricsSnapshot{
		MessagesSentTotal: m.messagesSent.Load(),
	}
}
