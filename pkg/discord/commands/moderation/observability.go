package moderation

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cleanup"
	"github.com/small-frappuccino/discordcore/pkg/observability"
)

// Metrics is the narrow observability seam moderation commands write through.
// The interface intentionally hides whether the counters are in-memory,
// shipped to Prometheus, or thrown away — recording code stays the same in
// all three worlds. NopMetrics is the default so commands can call
// m.RecordX(...) without nil checks; the in-memory implementation
// (NewInMemoryMetrics) is what /v1/health/moderation reads from.
//
// Mod actions are operationally as sensitive as QOTD publishes (audit,
// abuse, accidental wipes), so the recorded event set mirrors the QOTD
// shape: a typed surface that catches event-naming drift at compile time
// instead of free-form string keys. Right now only /clean records here;
// future moderation commands plug in by adding methods, not by introducing
// a parallel metrics package.
type Metrics interface {
	// RecordCleanAttempt is called once per /clean invocation, before
	// success/failure is known. Useful as the "in flight" denominator
	// against RecordCleanSuccess/RecordCleanFailure.
	RecordCleanAttempt()

	// RecordCleanSuccess is called after /clean completes the deletion
	// pass without an early-return error. deletedMessages is the count
	// actually removed from Discord (bulk + single combined); the
	// duration is measured from RecordCleanAttempt to completion.
	RecordCleanSuccess(duration time.Duration, deletedMessages int)

	// RecordCleanFailure is called when /clean returns an error before
	// finishing the deletion pass (feature disabled, invalid input, lost
	// permission, session unavailable, fetch failure). cause is a short,
	// stable token operators read as "why did /clean refuse this hour".
	RecordCleanFailure(cause string, duration time.Duration)

	// RecordCleanDeleteFailure is called once per message that Discord
	// rejected during the deletion pass. The cleanup.FailureClass is
	// preserved (forbidden, missing_channel, rate_limited, transient,
	// unknown) so operators can distinguish "lost permission mid-flight"
	// from "Discord 5xx spike" without grepping logs.
	RecordCleanDeleteFailure(class cleanup.FailureClass)

	// RecordCleanAuditLogFailure is called when the audit-log channel
	// embed POST fails (the secondary audit consumer). The primary audit
	// trail is the structured application log line emitted on every
	// /clean run, so this metric exists purely to surface the silent
	// loss: a steady non-zero rate means "audit channel is broken and
	// nobody noticed because the slash command still replied success".
	RecordCleanAuditLogFailure()
}

// SnapshotProvider is the optional capability the /v1/health/moderation
// handler looks for. The in-memory implementation satisfies it; NopMetrics
// does not (it has nothing to snapshot). Routes use a type assertion so the
// metrics dependency stays write-only on the hot path.
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
}

// MetricsSnapshot is the JSON payload /v1/health/moderation returns. The
// outer struct exists so future moderation commands can add their own
// snapshot sub-types without breaking the top-level shape.
type MetricsSnapshot struct {
	Clean CleanSnapshot `json:"clean"`
}

// CleanSnapshot is the /clean command's view: invocation counters, the
// deletion sub-totals, the per-cause failure map, and a count/sum/max
// duration summary. Operators read SuccessTotal/AttemptsTotal as the
// success ratio and FailureByCause as the qualitative "why".
type CleanSnapshot struct {
	AttemptsTotal        int64            `json:"attempts_total"`
	SuccessTotal         int64            `json:"success_total"`
	FailureTotal         int64            `json:"failure_total"`
	FailureByCause       map[string]int64 `json:"failure_by_cause,omitempty"`
	DeleteFailureByClass map[string]int64 `json:"delete_failure_by_class,omitempty"`
	DeletedMessagesTotal int64            `json:"deleted_messages_total"`
	// AuditLogFailureTotal counts how many /clean runs successfully
	// deleted messages but failed to post the audit-log channel embed.
	// Operators read a non-zero rate as "audit channel is broken" —
	// the structured application log still has the primary record.
	AuditLogFailureTotal int64                         `json:"audit_log_failure_total"`
	Duration             observability.SummarySnapshot `json:"duration_seconds"`
}

// Stable cause tokens recorded by RecordCleanFailure. Keep this list in
// sync with the call sites in clean_command.go — operators build alerts
// against these strings, so renames are a breaking change for them.
const (
	CleanFailureCauseFeatureDisabled  = "feature_disabled"
	CleanFailureCauseInvalidRequest   = "invalid_request"
	CleanFailureCausePermissionDenied = "permission_denied"
	CleanFailureCauseFetchForbidden   = "fetch_forbidden"
	CleanFailureCauseFetchMissing     = "fetch_missing_channel"
	CleanFailureCauseFetchRateLimited = "fetch_rate_limited"
	CleanFailureCauseFetchTransient   = "fetch_transient"
	CleanFailureCauseFetchUnknown     = "fetch_unknown"
)

// ClassifyCleanFetchFailure maps a fetch-side cleanup.FailureClass into
// the stable CleanFailureCauseFetch* token. Separate function (not just
// a string concat) so the cause vocabulary stays grep-able when adding
// new fetch errors.
func ClassifyCleanFetchFailure(class cleanup.FailureClass) string {
	switch class {
	case cleanup.FailureClassForbidden:
		return CleanFailureCauseFetchForbidden
	case cleanup.FailureClassMissingChannel:
		return CleanFailureCauseFetchMissing
	case cleanup.FailureClassRateLimited:
		return CleanFailureCauseFetchRateLimited
	case cleanup.FailureClassTransient:
		return CleanFailureCauseFetchTransient
	default:
		return CleanFailureCauseFetchUnknown
	}
}

// FailureClassToken maps a cleanup.FailureClass to the canonical short
// token used in logs, /v1/health/moderation snapshots, and dashboards.
// Single source of truth: both the clean command's structured logs and
// the per-class delete failure counters route through here.
func FailureClassToken(class cleanup.FailureClass) string {
	switch class {
	case cleanup.FailureClassMissingMessage:
		return "missing_message"
	case cleanup.FailureClassMissingChannel:
		return "missing_channel"
	case cleanup.FailureClassForbidden:
		return "forbidden"
	case cleanup.FailureClassBulkDeleteAge:
		return "bulk_delete_age"
	case cleanup.FailureClassRateLimited:
		return "rate_limited"
	case cleanup.FailureClassTransient:
		return "transient"
	default:
		return "unknown"
	}
}

// NopMetrics is the default implementation when moderation commands are
// registered without explicit metrics wiring. Every method is a no-op;
// this lets command code call m.RecordX(...) without nil checks.
type NopMetrics struct{}

func (NopMetrics) RecordCleanAttempt()                           {}
func (NopMetrics) RecordCleanSuccess(time.Duration, int)         {}
func (NopMetrics) RecordCleanFailure(string, time.Duration)      {}
func (NopMetrics) RecordCleanDeleteFailure(cleanup.FailureClass) {}
func (NopMetrics) RecordCleanAuditLogFailure()                   {}

// InMemoryMetrics is the lightweight implementation backing
// /v1/health/moderation. Atomic int64 counters; the labeled maps
// (failure-by-cause, delete-failure-by-class) sit behind a RWMutex
// because their cardinality is bounded by the source code (a handful of
// cause tokens, a handful of failure classes).
//
// Goroutine safety: every method is safe to call concurrently. Snapshot()
// briefly takes a read lock to copy the maps; the atomic loads happen
// without locks.
type InMemoryMetrics struct {
	mu sync.RWMutex

	attempts atomic.Int64
	success  atomic.Int64
	failure  atomic.Int64

	failureByCause       map[string]*atomic.Int64
	deleteFailureByClass map[string]*atomic.Int64

	deletedMessages atomic.Int64
	auditLogFailure atomic.Int64
	duration        observability.Summary
}

// NewInMemoryMetrics constructs the production metrics implementation.
// Use this in pkg/app wiring and pass into the moderation command
// registration so /v1/health/moderation has counters to expose.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{
		failureByCause:       make(map[string]*atomic.Int64),
		deleteFailureByClass: make(map[string]*atomic.Int64),
	}
}

func (m *InMemoryMetrics) RecordCleanAttempt() {
	m.attempts.Add(1)
}

func (m *InMemoryMetrics) RecordCleanSuccess(duration time.Duration, deletedMessages int) {
	m.success.Add(1)
	if deletedMessages > 0 {
		m.deletedMessages.Add(int64(deletedMessages))
	}
	m.duration.Observe(duration)
}

func (m *InMemoryMetrics) RecordCleanFailure(cause string, duration time.Duration) {
	m.failure.Add(1)
	m.causeCounter(cause).Add(1)
	m.duration.Observe(duration)
}

func (m *InMemoryMetrics) RecordCleanDeleteFailure(class cleanup.FailureClass) {
	m.classCounter(FailureClassToken(class)).Add(1)
}

func (m *InMemoryMetrics) RecordCleanAuditLogFailure() {
	m.auditLogFailure.Add(1)
}

// Snapshot returns a JSON-friendly view of the current counter state. The
// returned MetricsSnapshot is a copy; callers can mutate it without
// affecting the live counters.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	failureByCause := make(map[string]int64, len(m.failureByCause))
	for cause, counter := range m.failureByCause {
		failureByCause[cause] = counter.Load()
	}
	deleteFailureByClass := make(map[string]int64, len(m.deleteFailureByClass))
	for class, counter := range m.deleteFailureByClass {
		deleteFailureByClass[class] = counter.Load()
	}

	return MetricsSnapshot{
		Clean: CleanSnapshot{
			AttemptsTotal:        m.attempts.Load(),
			SuccessTotal:         m.success.Load(),
			FailureTotal:         m.failure.Load(),
			FailureByCause:       failureByCause,
			DeleteFailureByClass: deleteFailureByClass,
			DeletedMessagesTotal: m.deletedMessages.Load(),
			AuditLogFailureTotal: m.auditLogFailure.Load(),
			Duration:             m.duration.Snapshot(),
		},
	}
}

func (m *InMemoryMetrics) causeCounter(cause string) *atomic.Int64 {
	return observability.GetOrCreateLabeledCounter(&m.mu, m.failureByCause, cause)
}

func (m *InMemoryMetrics) classCounter(label string) *atomic.Int64 {
	return observability.GetOrCreateLabeledCounter(&m.mu, m.deleteFailureByClass, label)
}
