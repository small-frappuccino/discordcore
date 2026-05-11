package qotd

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNopMetricsImplementsInterface(t *testing.T) {
	t.Parallel()

	// Pins that NopMetrics keeps satisfying the Metrics interface even if
	// new methods are added later. A new method on Metrics without a
	// matching NopMetrics method would break this assertion at compile
	// time on the next CI run, before it breaks every test that uses the
	// default Service constructor.
	var _ Metrics = NopMetrics{}
}

func TestInMemoryMetricsSnapshotZeroValuesAreSerializable(t *testing.T) {
	t.Parallel()

	m := NewInMemoryMetrics()
	snap := m.Snapshot()
	if got := len(snap.Publishes); got != 0 {
		t.Fatalf("expected empty publishes map on fresh metrics, got %d entries", got)
	}
	if snap.Reconcile.CyclesTotal != 0 {
		t.Fatalf("expected zero reconcile cycles on fresh metrics, got %d", snap.Reconcile.CyclesTotal)
	}

	// JSON marshal must succeed so /v1/health/qotd never panics on an
	// empty-state response. We don't assert structural shape here — the
	// next test does that with real data.
	if _, err := json.Marshal(snap); err != nil {
		t.Fatalf("expected fresh snapshot to marshal cleanly, got %v", err)
	}
}

func TestInMemoryMetricsRecordsPublishSuccessAndFailureSeparately(t *testing.T) {
	t.Parallel()

	m := NewInMemoryMetrics()
	m.RecordPublishAttempt(PublishModeScheduled)
	m.RecordPublishSuccess(PublishModeScheduled, 1500*time.Millisecond)
	m.RecordPublishAttempt(PublishModeScheduled)
	m.RecordPublishFailure(PublishModeScheduled, "no_questions", 200*time.Millisecond)
	m.RecordPublishAttempt(PublishModeManual)
	m.RecordPublishSuccess(PublishModeManual, 800*time.Millisecond)

	snap := m.Snapshot()
	scheduled, ok := snap.Publishes[string(PublishModeScheduled)]
	if !ok {
		t.Fatalf("expected scheduled bucket to exist, got %+v", snap.Publishes)
	}
	if scheduled.SuccessTotal != 1 || scheduled.FailureTotal != 1 {
		t.Fatalf("expected one success and one failure for scheduled mode, got %+v", scheduled)
	}
	if got := scheduled.FailureByCause["no_questions"]; got != 1 {
		t.Fatalf("expected no_questions cause to be counted once, got %d (full map=%+v)", got, scheduled.FailureByCause)
	}
	if scheduled.Duration.Count != 2 {
		t.Fatalf("expected duration count to include both attempts, got %+v", scheduled.Duration)
	}
	if scheduled.Duration.MaxSeconds < 1.0 {
		t.Fatalf("expected max duration to reflect the 1500ms attempt, got %+v", scheduled.Duration)
	}

	manual, ok := snap.Publishes[string(PublishModeManual)]
	if !ok || manual.SuccessTotal != 1 {
		t.Fatalf("expected one success for manual mode, got %+v", manual)
	}
}

func TestInMemoryMetricsRecordsReconcileCycleSuccessAndFailureSeparately(t *testing.T) {
	t.Parallel()

	m := NewInMemoryMetrics()
	m.RecordReconcileCycle(50*time.Millisecond, nil)
	m.RecordReconcileCycle(60*time.Millisecond, nil)
	m.RecordReconcileCycle(70*time.Millisecond, errors.New("boom"))

	snap := m.Snapshot()
	if snap.Reconcile.CyclesTotal != 3 {
		t.Fatalf("expected three reconcile cycles, got %d", snap.Reconcile.CyclesTotal)
	}
	if snap.Reconcile.FailuresTotal != 1 {
		t.Fatalf("expected one failed cycle, got %d", snap.Reconcile.FailuresTotal)
	}
	if snap.Reconcile.Duration.Count != 3 {
		t.Fatalf("expected duration to cover all three cycles, got %+v", snap.Reconcile.Duration)
	}
}

func TestInMemoryMetricsRecordsSideEvents(t *testing.T) {
	t.Parallel()

	m := NewInMemoryMetrics()
	m.RecordOfficialPostAbandoned()
	m.RecordOfficialPostAbandoned()
	m.RecordStateDivergence()
	m.RecordUnmanageableThread()
	m.RecordOrphanReclaim(3)
	m.RecordOrphanReclaim(0) // ignored: empty batches do not pollute the count
	m.RecordSuppressionCleared()

	snap := m.Snapshot()
	if snap.State.AbandonedTotal != 2 {
		t.Fatalf("expected two abandoned events, got %d", snap.State.AbandonedTotal)
	}
	if snap.State.DivergenceTotal != 1 {
		t.Fatalf("expected one divergence event, got %d", snap.State.DivergenceTotal)
	}
	if snap.State.UnmanageableThreadTotal != 1 {
		t.Fatalf("expected one unmanageable-thread event, got %d", snap.State.UnmanageableThreadTotal)
	}
	if snap.State.OrphanReservationsReclaimed != 3 {
		t.Fatalf("expected three reclaimed reservations, got %d", snap.State.OrphanReservationsReclaimed)
	}
	if snap.State.SuppressionsCleared != 1 {
		t.Fatalf("expected one suppression cleared, got %d", snap.State.SuppressionsCleared)
	}
}

func TestInMemoryMetricsTolerantToConcurrentWrites(t *testing.T) {
	t.Parallel()

	m := NewInMemoryMetrics()

	// 8 writers × 1000 records = 8000 events of each type. The atomic +
	// per-map-RWMutex design must surface exactly those numbers in the
	// snapshot — otherwise a race or lost update would be hiding.
	const workers = 8
	const perWorker = 1000
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				m.RecordPublishAttempt(PublishModeScheduled)
				m.RecordPublishSuccess(PublishModeScheduled, 10*time.Millisecond)
				m.RecordReconcileCycle(5*time.Millisecond, nil)
				m.RecordStateDivergence()
			}
		}()
	}
	wg.Wait()

	snap := m.Snapshot()
	total := int64(workers * perWorker)
	if got := snap.Publishes[string(PublishModeScheduled)].SuccessTotal; got != total {
		t.Fatalf("expected %d concurrent publishes, got %d", total, got)
	}
	if snap.Reconcile.CyclesTotal != total {
		t.Fatalf("expected %d concurrent reconcile cycles, got %d", total, snap.Reconcile.CyclesTotal)
	}
	if snap.State.DivergenceTotal != total {
		t.Fatalf("expected %d concurrent divergence events, got %d", total, snap.State.DivergenceTotal)
	}
}

func TestInMemoryMetricsSummaryTracksMaxIndependentOfSum(t *testing.T) {
	t.Parallel()

	m := NewInMemoryMetrics()
	// Two short observations and one long. Max must reflect the long one,
	// sum must be all three combined, count is three.
	m.RecordPublishSuccess(PublishModeScheduled, 50*time.Millisecond)
	m.RecordPublishSuccess(PublishModeScheduled, 30*time.Millisecond)
	m.RecordPublishSuccess(PublishModeScheduled, 5*time.Second)

	snap := m.Snapshot().Publishes[string(PublishModeScheduled)]
	if snap.Duration.Count != 3 {
		t.Fatalf("expected count=3, got %+v", snap.Duration)
	}
	if snap.Duration.MaxSeconds < 4.99 || snap.Duration.MaxSeconds > 5.01 {
		t.Fatalf("expected max ≈ 5s, got %+v", snap.Duration)
	}
	if snap.Duration.SumSeconds < 5.07 || snap.Duration.SumSeconds > 5.09 {
		t.Fatalf("expected sum ≈ 5.08s, got %+v", snap.Duration)
	}
}

func TestClassifyPublishFailureMapsKnownSentinels(t *testing.T) {
	t.Parallel()

	cases := map[error]string{
		nil:                            "none",
		ErrAlreadyPublished:            "already_published",
		ErrPublishInProgress:           "in_progress",
		ErrNoQuestionsAvailable:        "no_questions",
		ErrQOTDDisabled:                "qotd_disabled",
		ErrDiscordUnavailable:          "discord_unavailable",
		ErrOfficialPostStateDivergence: "state_divergence",
		errors.New("random"):           "other",
	}
	for err, want := range cases {
		got := ClassifyPublishFailure(err)
		if got != want {
			t.Fatalf("ClassifyPublishFailure(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestClassifyPublishFailureUnwrapsWrappedSentinels(t *testing.T) {
	t.Parallel()

	wrapped := wrappedErr{msg: "outer", inner: ErrNoQuestionsAvailable}
	if got := ClassifyPublishFailure(wrapped); got != "no_questions" {
		t.Fatalf("expected classifier to peek through wrapping, got %q", got)
	}
}

// SnapshotProvider must be satisfied by InMemoryMetrics so the route
// handler can detect a snapshottable implementation at runtime via type
// assertion. Pins the contract.
func TestInMemoryMetricsSatisfiesSnapshotProvider(t *testing.T) {
	t.Parallel()
	var _ SnapshotProvider = (*InMemoryMetrics)(nil)
}

type wrappedErr struct {
	msg   string
	inner error
}

func (e wrappedErr) Error() string { return e.msg + ": " + e.inner.Error() }
func (e wrappedErr) Unwrap() error { return e.inner }
