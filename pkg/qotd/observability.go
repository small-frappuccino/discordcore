package qotd

import (
	"context"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/observability"
)

// Metrics is the narrow observability seam the QOTD service writes through.
// The interface intentionally hides whether the counters are in-memory,
// shipped to Prometheus, exported via OpenTelemetry, or thrown away — the
// service code stays the same in all three worlds. The default is a noop
// implementation so the bot can start without an explicit metrics wiring;
// the in-memory implementation (NewInMemoryMetrics) is what
// /v1/health/qotd reads from.
//
// Method shape is "Record<event>(labels, optional duration)" rather than
// "GetCounter('name').Inc()" because a typed surface catches event-naming
// drift at compile time. Adding a new event is one method on this
// interface plus the corresponding line in the snapshot; ad-hoc string
// keys are not supported on purpose.
// PublishMetrics tracks the lifecycle of a publish operation.
type PublishMetrics interface {
	// RecordPublishAttempt is called once per publish path entry, before
	// success/failure is known. Useful when the operator wants to see
	// "in flight" rate vs. completed rate.
	RecordPublishAttempt(mode PublishMode)

	// RecordPublishSuccess is called after the publish finalizes
	// successfully. The duration is measured from the publish path entry
	// (just before the lifecycle lock is acquired) to finalization.
	RecordPublishSuccess(mode PublishMode, duration time.Duration)

	// RecordPublishFailure is called when the publish path returns an
	// error or terminal state. cause is a short, stable token that the
	// classifier (classifyPublishFailure) maps from the underlying error
	// — operators read this as "why did publishes fail this hour".
	RecordPublishFailure(mode PublishMode, cause string, duration time.Duration)
}

// ReconcileMetrics tracks the background reconcile loop's behavior.
type ReconcileMetrics interface {
	// RecordReconcileCycle is called once per reconcile cycle that ran
	// to completion (successfully or not). err == nil distinguishes
	// success from failure inside the summary.
	RecordReconcileCycle(duration time.Duration, err error)

	// RecordOrphanReclaim records how many orphaned reservations the
	// reconcile loop returned to ready in one cycle.
	RecordOrphanReclaim(count int)
}

// StateMetrics tracks unusual side events and divergence.
type StateMetrics interface {
	// RecordOfficialPostAbandoned is called when a publish attempt is
	// marked terminally abandoned (channel deleted, bot kicked, missing
	// permission). Operators read this as "manual intervention queue
	// length" — a steady non-zero rate means broken guilds piling up.
	RecordOfficialPostAbandoned()

	// RecordSuppressionCleared is called each time an expired suppression
	// entry is purged from a guild's config.
	RecordSuppressionCleared()
}

// Metrics is the union of all observability seams the QOTD service writes through.
type Metrics interface {
	PublishMetrics
	ReconcileMetrics
	StateMetrics

	// Attach ensures the metrics pipeline is successfully bound prior to
	// the primary event loop. A failure to attach triggers a fatal abort.
	Attach(ctx context.Context) error
}

// SnapshotProvider is the optional capability the /v1/health/qotd handler
// looks for. The in-memory implementation satisfies it; the noop does
// not (it has nothing to snapshot). Routes use a type assertion so the
// metrics dependency stays Write-only on the hot path.
type SnapshotProvider interface {
	Snapshot() MetricsSnapshot
}

// PublishModeSnapshot bundles the success/failure totals and the duration
// summary for one publish mode (scheduled or manual). All fields are
// JSON-serializable.
type PublishModeSnapshot struct {
	SuccessTotal   int64                         `json:"success_total"`
	FailureTotal   int64                         `json:"failure_total"`
	FailureByCause map[string]int64              `json:"failure_by_cause,omitempty"`
	Duration       observability.SummarySnapshot `json:"duration_seconds"`
}

// ReconcileSnapshot is the reconcile loop's view.
type ReconcileSnapshot struct {
	CyclesTotal   int64                         `json:"cycles_total"`
	FailuresTotal int64                         `json:"failures_total"`
	Duration      observability.SummarySnapshot `json:"duration_seconds"`
}

// StateSnapshot is the bag of "side event" counters that don't fit the
// publish or reconcile groups. These are flat counts; operators read
// them as "how many of these unusual events happened recently".
type StateSnapshot struct {
	AbandonedTotal              int64 `json:"abandoned_total"`
	OrphanReservationsReclaimed int64 `json:"orphan_reservations_reclaimed_total"`
	SuppressionsCleared         int64 `json:"suppressions_cleared_total"`
}

// MetricsSnapshot is the JSON payload /v1/health/qotd returns. The order
// of fields here is the order operators see them; keep "publishes"
// first since it is the headline number.
type MetricsSnapshot struct {
	Publishes map[string]PublishModeSnapshot `json:"publishes"`
	Reconcile ReconcileSnapshot              `json:"reconcile"`
	State     StateSnapshot                  `json:"state"`
}

// NopMetrics is the default implementation when the bot is started
// without explicit metrics wiring. Every method is a no-op; this lets
// library code call s.metrics.RecordX(...) without nil checks.
type NopMetrics struct{}

// Attach returns nil for NopMetrics.
func (NopMetrics) Attach(context.Context) error { return nil }

// RecordPublishAttempt records publish attempt.
func (NopMetrics) RecordPublishAttempt(PublishMode) {}

// RecordPublishSuccess records publish success.
func (NopMetrics) RecordPublishSuccess(PublishMode, time.Duration) {}

// RecordPublishFailure records publish failure.
func (NopMetrics) RecordPublishFailure(PublishMode, string, time.Duration) {
}

// RecordReconcileCycle records reconcile cycle.
func (NopMetrics) RecordReconcileCycle(time.Duration, error) {}

// RecordOfficialPostAbandoned records official post abandoned.
func (NopMetrics) RecordOfficialPostAbandoned() {}

// RecordOrphanReclaim records orphan reclaim.
func (NopMetrics) RecordOrphanReclaim(int) {}

// RecordSuppressionCleared records suppression cleared.
func (NopMetrics) RecordSuppressionCleared() {}

// InMemoryMetrics is the lightweight implementation backing
// /v1/health/qotd. All counters are atomic int64; the labeled counters
// (failure-by-cause, per-mode publish duration) use a RWMutex around a
// small map because the cardinality is bounded by the source code (two
// modes × a handful of causes).
//
// Goroutine safety: every method is safe to call concurrently. Snapshot()
// takes a read lock briefly to copy the map; the atomic loads happen
// without locks.
type InMemoryMetrics struct {
	mu sync.RWMutex

	publishAttempts    map[PublishMode]*atomic.Int64
	publishSuccess     map[PublishMode]*atomic.Int64
	publishFailure     map[PublishMode]*atomic.Int64
	publishFailByCause map[PublishMode]map[string]*atomic.Int64
	publishDuration    map[PublishMode]*observability.Summary

	reconcileCycles   atomic.Int64
	reconcileFailures atomic.Int64
	reconcileDuration observability.Summary

	abandoned          atomic.Int64
	orphanReclaim      atomic.Int64
	suppressionCleared atomic.Int64
}

// Attach returns nil for InMemoryMetrics.
func (m *InMemoryMetrics) Attach(context.Context) error { return nil }

// RecordPublishAttempt records publish attempt.
func (m *InMemoryMetrics) RecordPublishAttempt(mode PublishMode) {
	m.counterFor(mode, m.publishAttemptsGetOrCreate).Add(1)
}

// RecordPublishSuccess records publish success.
func (m *InMemoryMetrics) RecordPublishSuccess(mode PublishMode, duration time.Duration) {
	m.counterFor(mode, m.publishSuccessGetOrCreate).Add(1)
	m.durationFor(mode).Observe(duration)
}

// RecordPublishFailure records publish failure.
func (m *InMemoryMetrics) RecordPublishFailure(mode PublishMode, cause string, duration time.Duration) {
	m.counterFor(mode, m.publishFailureGetOrCreate).Add(1)
	m.failureCauseFor(mode, cause).Add(1)
	m.durationFor(mode).Observe(duration)
}

// RecordReconcileCycle records reconcile cycle.
func (m *InMemoryMetrics) RecordReconcileCycle(duration time.Duration, err error) {
	m.reconcileCycles.Add(1)
	if err != nil {
		m.reconcileFailures.Add(1)
	}
	m.reconcileDuration.Observe(duration)
}

// RecordOfficialPostAbandoned records official post abandoned.
func (m *InMemoryMetrics) RecordOfficialPostAbandoned() { m.abandoned.Add(1) }

// RecordSuppressionCleared records suppression cleared.
func (m *InMemoryMetrics) RecordSuppressionCleared() { m.suppressionCleared.Add(1) }

// RecordOrphanReclaim records orphan reclaim.
func (m *InMemoryMetrics) RecordOrphanReclaim(count int) {
	if count <= 0 {
		return
	}
	m.orphanReclaim.Add(int64(count))
}

// Snapshot returns a JSON-friendly view of the current counter state. The
// returned MetricsSnapshot is a copy; callers can mutate it without
// affecting the live counters.
func (m *InMemoryMetrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	publishes := make(map[string]PublishModeSnapshot, len(m.publishSuccess))
	modes := collectPublishModes(m.publishAttempts, m.publishSuccess, m.publishFailure, m.publishFailByCause, m.publishDuration)
	for _, mode := range modes {
		modeKey := string(mode)
		causeMap := map[string]int64{}
		if causes, ok := m.publishFailByCause[mode]; ok {
			for cause, counter := range causes {
				causeMap[cause] = counter.Load()
			}
		}
		duration := observability.SummarySnapshot{}
		if s, ok := m.publishDuration[mode]; ok {
			duration = s.Snapshot()
		}
		publishes[modeKey] = PublishModeSnapshot{
			SuccessTotal:   atomicLoad(m.publishSuccess[mode]),
			FailureTotal:   atomicLoad(m.publishFailure[mode]),
			FailureByCause: causeMap,
			Duration:       duration,
		}
	}

	return MetricsSnapshot{
		Publishes: publishes,
		Reconcile: ReconcileSnapshot{
			CyclesTotal:   m.reconcileCycles.Load(),
			FailuresTotal: m.reconcileFailures.Load(),
			Duration:      m.reconcileDuration.Snapshot(),
		},
		State: StateSnapshot{
			AbandonedTotal:              m.abandoned.Load(),
			OrphanReservationsReclaimed: m.orphanReclaim.Load(),
			SuppressionsCleared:         m.suppressionCleared.Load(),
		},
	}
}

// counterFor abstracts the "lazy initialize per-mode atomic counter"
// pattern shared by attempts/success/failure. The getOrCreate closure
// captures the right map.
func (m *InMemoryMetrics) counterFor(mode PublishMode, getOrCreate func(PublishMode) *atomic.Int64) *atomic.Int64 {
	return getOrCreate(mode)
}

func (m *InMemoryMetrics) publishAttemptsGetOrCreate(mode PublishMode) *atomic.Int64 {
	return observability.GetOrCreateLabeledCounter(&m.mu, &m.publishAttempts, mode)
}
func (m *InMemoryMetrics) publishSuccessGetOrCreate(mode PublishMode) *atomic.Int64 {
	return observability.GetOrCreateLabeledCounter(&m.mu, &m.publishSuccess, mode)
}
func (m *InMemoryMetrics) publishFailureGetOrCreate(mode PublishMode) *atomic.Int64 {
	return observability.GetOrCreateLabeledCounter(&m.mu, &m.publishFailure, mode)
}

func (m *InMemoryMetrics) failureCauseFor(mode PublishMode, cause string) *atomic.Int64 {
	m.mu.RLock()
	if m.publishFailByCause != nil {
		if causes, ok := m.publishFailByCause[mode]; ok {
			if counter, ok := causes[cause]; ok {
				m.mu.RUnlock()
				return counter
			}
		}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishFailByCause == nil {
		m.publishFailByCause = make(map[PublishMode]map[string]*atomic.Int64)
	}
	causes, ok := m.publishFailByCause[mode]
	if !ok {
		causes = make(map[string]*atomic.Int64)
		m.publishFailByCause[mode] = causes
	}
	counter, ok := causes[cause]
	if !ok {
		counter = &atomic.Int64{}
		causes[cause] = counter
	}
	return counter
}

func (m *InMemoryMetrics) durationFor(mode PublishMode) *observability.Summary {
	m.mu.RLock()
	if m.publishDuration != nil {
		if s, ok := m.publishDuration[mode]; ok {
			m.mu.RUnlock()
			return s
		}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.publishDuration == nil {
		m.publishDuration = make(map[PublishMode]*observability.Summary)
	}
	s, ok := m.publishDuration[mode]
	if !ok {
		s = &observability.Summary{}
		m.publishDuration[mode] = s
	}
	return s
}

func atomicLoad(c *atomic.Int64) int64 {
	if c == nil {
		return 0
	}
	return c.Load()
}

func collectPublishModes(maps ...any) []PublishMode {
	seen := make(map[PublishMode]struct{})
	for _, m := range maps {
		switch v := m.(type) {
		case map[PublishMode]*atomic.Int64:
			for k := range v {
				seen[k] = struct{}{}
			}
		case map[PublishMode]map[string]*atomic.Int64:
			for k := range v {
				seen[k] = struct{}{}
			}
		case map[PublishMode]*observability.Summary:
			for k := range v {
				seen[k] = struct{}{}
			}
		}
	}
	out := make([]PublishMode, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}

// ClassifyPublishFailure maps a publish-path error into one of a small
// fixed set of cause tokens. Stable string set so /v1/health/qotd
// consumers can build dashboards without seeing new cause buckets pop
// up every release.
func ClassifyPublishFailure(err error) string {
	switch {
	case err == nil:
		return "none"
	case errorMatches(err, ErrAlreadyPublished):
		return "already_published"
	case errorMatches(err, ErrPublishInProgress):
		return "in_progress"
	case errorMatches(err, ErrNoQuestionsAvailable):
		return "no_questions"
	case errorMatches(err, ErrQOTDDisabled):
		return "qotd_disabled"
	case errorMatches(err, ErrDiscordUnavailable):
		return "discord_unavailable"

	default:
		return "other"
	}
}

// errorMatches is the tiny errors.Is wrapper that classifies via the
// public sentinels. Pulled out so ClassifyPublishFailure stays a flat
// switch — easier to grep for new cause tokens when reviewing diffs.
func errorMatches(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	return errors.Is(err, target)
}
