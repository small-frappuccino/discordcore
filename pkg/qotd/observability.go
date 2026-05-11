package qotd

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"
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
type Metrics interface {
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

	// RecordReconcileCycle is called once per reconcile cycle that ran
	// to completion (successfully or not). err == nil distinguishes
	// success from failure inside the summary.
	RecordReconcileCycle(duration time.Duration, err error)

	// RecordOfficialPostAbandoned is called when a publish attempt is
	// marked terminally abandoned (channel deleted, bot kicked, missing
	// permission). Operators read this as "manual intervention queue
	// length" — a steady non-zero rate means broken guilds piling up.
	RecordOfficialPostAbandoned()

	// RecordStateDivergence is called every time the asymmetric "Discord
	// OK, DB failed" branch fires inside
	// applyOfficialPostThreadTransition. Transient spikes are expected
	// (Postgres blips); a persistent rate is a real outage.
	RecordStateDivergence()

	// RecordUnmanageableThread is called the first time a (thread,
	// target-state) pair is rejected with 403; the existing log-once
	// dedup means subsequent calls are silenced, so this metric counts
	// distinct rejections, not the per-cycle retries.
	RecordUnmanageableThread()

	// RecordOrphanReclaim records how many orphaned reservations the
	// reconcile loop returned to ready in one cycle.
	RecordOrphanReclaim(count int)

	// RecordSuppressionCleared is called each time an expired suppression
	// entry is purged from a guild's config.
	RecordSuppressionCleared()
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
	SuccessTotal  int64            `json:"success_total"`
	FailureTotal  int64            `json:"failure_total"`
	FailureByCause map[string]int64 `json:"failure_by_cause,omitempty"`
	Duration      SummarySnapshot  `json:"duration_seconds"`
}

// ReconcileSnapshot is the reconcile loop's view.
type ReconcileSnapshot struct {
	CyclesTotal   int64           `json:"cycles_total"`
	FailuresTotal int64           `json:"failures_total"`
	Duration      SummarySnapshot `json:"duration_seconds"`
}

// StateSnapshot is the bag of "side event" counters that don't fit the
// publish or reconcile groups. These are flat counts; operators read
// them as "how many of these unusual events happened recently".
type StateSnapshot struct {
	AbandonedTotal              int64 `json:"abandoned_total"`
	DivergenceTotal             int64 `json:"divergence_total"`
	UnmanageableThreadTotal     int64 `json:"unmanageable_thread_total"`
	OrphanReservationsReclaimed int64 `json:"orphan_reservations_reclaimed_total"`
	SuppressionsCleared         int64 `json:"suppressions_cleared_total"`
}

// SummarySnapshot is the count/sum/max shape that mirrors a Prometheus
// summary minus quantiles. Operators get average via sum/count and tail
// behavior via max. Designed so a Prometheus migration is one transform
// per field, not a redesign.
type SummarySnapshot struct {
	Count      int64   `json:"count"`
	SumSeconds float64 `json:"sum_seconds"`
	MaxSeconds float64 `json:"max_seconds"`
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

func (NopMetrics) RecordPublishAttempt(PublishMode)                  {}
func (NopMetrics) RecordPublishSuccess(PublishMode, time.Duration)   {}
func (NopMetrics) RecordPublishFailure(PublishMode, string, time.Duration) {
}
func (NopMetrics) RecordReconcileCycle(time.Duration, error) {}
func (NopMetrics) RecordOfficialPostAbandoned()              {}
func (NopMetrics) RecordStateDivergence()                    {}
func (NopMetrics) RecordUnmanageableThread()                 {}
func (NopMetrics) RecordOrphanReclaim(int)                   {}
func (NopMetrics) RecordSuppressionCleared()                 {}

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

	publishAttempts map[PublishMode]*atomic.Int64
	publishSuccess  map[PublishMode]*atomic.Int64
	publishFailure  map[PublishMode]*atomic.Int64
	publishFailByCause map[PublishMode]map[string]*atomic.Int64
	publishDuration map[PublishMode]*summary

	reconcileCycles   atomic.Int64
	reconcileFailures atomic.Int64
	reconcileDuration summary

	abandoned          atomic.Int64
	divergence         atomic.Int64
	unmanageableThread atomic.Int64
	orphanReclaim      atomic.Int64
	suppressionCleared atomic.Int64
}

// NewInMemoryMetrics constructs the production metrics implementation.
// Use this in pkg/app wiring and pass into qotd.NewService.
func NewInMemoryMetrics() *InMemoryMetrics {
	return &InMemoryMetrics{
		publishAttempts:    make(map[PublishMode]*atomic.Int64),
		publishSuccess:     make(map[PublishMode]*atomic.Int64),
		publishFailure:     make(map[PublishMode]*atomic.Int64),
		publishFailByCause: make(map[PublishMode]map[string]*atomic.Int64),
		publishDuration:    make(map[PublishMode]*summary),
	}
}

func (m *InMemoryMetrics) RecordPublishAttempt(mode PublishMode) {
	m.counterFor(mode, m.publishAttemptsGetOrCreate).Add(1)
}

func (m *InMemoryMetrics) RecordPublishSuccess(mode PublishMode, duration time.Duration) {
	m.counterFor(mode, m.publishSuccessGetOrCreate).Add(1)
	m.durationFor(mode).observe(duration)
}

func (m *InMemoryMetrics) RecordPublishFailure(mode PublishMode, cause string, duration time.Duration) {
	m.counterFor(mode, m.publishFailureGetOrCreate).Add(1)
	m.failureCauseFor(mode, cause).Add(1)
	m.durationFor(mode).observe(duration)
}

func (m *InMemoryMetrics) RecordReconcileCycle(duration time.Duration, err error) {
	m.reconcileCycles.Add(1)
	if err != nil {
		m.reconcileFailures.Add(1)
	}
	m.reconcileDuration.observe(duration)
}

func (m *InMemoryMetrics) RecordOfficialPostAbandoned() { m.abandoned.Add(1) }
func (m *InMemoryMetrics) RecordStateDivergence()        { m.divergence.Add(1) }
func (m *InMemoryMetrics) RecordUnmanageableThread()     { m.unmanageableThread.Add(1) }
func (m *InMemoryMetrics) RecordSuppressionCleared()     { m.suppressionCleared.Add(1) }

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
		duration := SummarySnapshot{}
		if s, ok := m.publishDuration[mode]; ok {
			duration = s.snapshot()
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
			Duration:      m.reconcileDuration.snapshot(),
		},
		State: StateSnapshot{
			AbandonedTotal:              m.abandoned.Load(),
			DivergenceTotal:             m.divergence.Load(),
			UnmanageableThreadTotal:     m.unmanageableThread.Load(),
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
	return getOrCreateModeCounter(&m.mu, m.publishAttempts, mode)
}
func (m *InMemoryMetrics) publishSuccessGetOrCreate(mode PublishMode) *atomic.Int64 {
	return getOrCreateModeCounter(&m.mu, m.publishSuccess, mode)
}
func (m *InMemoryMetrics) publishFailureGetOrCreate(mode PublishMode) *atomic.Int64 {
	return getOrCreateModeCounter(&m.mu, m.publishFailure, mode)
}

func (m *InMemoryMetrics) failureCauseFor(mode PublishMode, cause string) *atomic.Int64 {
	m.mu.RLock()
	if causes, ok := m.publishFailByCause[mode]; ok {
		if counter, ok := causes[cause]; ok {
			m.mu.RUnlock()
			return counter
		}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *InMemoryMetrics) durationFor(mode PublishMode) *summary {
	m.mu.RLock()
	if s, ok := m.publishDuration[mode]; ok {
		m.mu.RUnlock()
		return s
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.publishDuration[mode]
	if !ok {
		s = &summary{}
		m.publishDuration[mode] = s
	}
	return s
}

func getOrCreateModeCounter(mu *sync.RWMutex, m map[PublishMode]*atomic.Int64, mode PublishMode) *atomic.Int64 {
	mu.RLock()
	if c, ok := m[mode]; ok {
		mu.RUnlock()
		return c
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if c, ok := m[mode]; ok {
		return c
	}
	c := &atomic.Int64{}
	m[mode] = c
	return c
}

func atomicLoad(c *atomic.Int64) int64 {
	if c == nil {
		return 0
	}
	return c.Load()
}

func collectPublishModes(maps ...interface{}) []PublishMode {
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
		case map[PublishMode]*summary:
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

// summary is the count + sum + max tracker behind SummarySnapshot. It
// uses three atomics — Observe is lock-free, snapshot is one load each.
type summary struct {
	count    atomic.Int64
	sumNanos atomic.Int64
	maxNanos atomic.Int64
}

func (s *summary) observe(d time.Duration) {
	if d < 0 {
		d = 0
	}
	s.count.Add(1)
	s.sumNanos.Add(d.Nanoseconds())
	candidate := d.Nanoseconds()
	for {
		cur := s.maxNanos.Load()
		if candidate <= cur {
			return
		}
		if s.maxNanos.CompareAndSwap(cur, candidate) {
			return
		}
	}
}

func (s *summary) snapshot() SummarySnapshot {
	if s == nil {
		return SummarySnapshot{}
	}
	return SummarySnapshot{
		Count:      s.count.Load(),
		SumSeconds: time.Duration(s.sumNanos.Load()).Seconds(),
		MaxSeconds: time.Duration(s.maxNanos.Load()).Seconds(),
	}
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
	case errorMatches(err, ErrOfficialPostStateDivergence):
		return "state_divergence"
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
