package moderation

import (
	"sync"
	"testing"
	"time"
)

func TestInMemoryMetricsSnapshotReflectsRecordings(t *testing.T) {
	t.Parallel()

	m := &InMemoryMetrics{}
	m.RecordCleanAttempt()
	m.RecordCleanAttempt()
	m.RecordCleanSuccess(1200*time.Millisecond, 5)
	m.RecordCleanFailure(CleanFailureCausePermissionDenied, 80*time.Millisecond)
	m.RecordCleanFailure(CleanFailureCauseFetchRateLimited, 90*time.Millisecond)
	m.RecordCleanFailure(CleanFailureCauseFetchRateLimited, 100*time.Millisecond)
	m.RecordCleanDeleteFailure(CleanFailureClassForbidden)
	m.RecordCleanDeleteFailure(CleanFailureClassForbidden)
	m.RecordCleanDeleteFailure(CleanFailureClassRateLimited)
	m.RecordCleanAuditLogFailure()
	m.RecordCleanAuditLogFailure()

	snap := m.Snapshot().Clean

	if snap.AttemptsTotal != 2 {
		t.Fatalf("AttemptsTotal=%d want 2", snap.AttemptsTotal)
	}
	if snap.SuccessTotal != 1 {
		t.Fatalf("SuccessTotal=%d want 1", snap.SuccessTotal)
	}
	if snap.FailureTotal != 3 {
		t.Fatalf("FailureTotal=%d want 3", snap.FailureTotal)
	}
	if snap.DeletedMessagesTotal != 5 {
		t.Fatalf("DeletedMessagesTotal=%d want 5", snap.DeletedMessagesTotal)
	}
	if got := snap.FailureByCause[CleanFailureCauseFetchRateLimited]; got != 2 {
		t.Fatalf("FailureByCause[fetch_rate_limited]=%d want 2", got)
	}
	if got := snap.FailureByCause[CleanFailureCausePermissionDenied]; got != 1 {
		t.Fatalf("FailureByCause[permission_denied]=%d want 1", got)
	}
	if got := snap.DeleteFailureByClass[FailureClassToken(CleanFailureClassForbidden)]; got != 2 {
		t.Fatalf("DeleteFailureByClass[forbidden]=%d want 2", got)
	}
	if got := snap.DeleteFailureByClass[FailureClassToken(CleanFailureClassRateLimited)]; got != 1 {
		t.Fatalf("DeleteFailureByClass[rate_limited]=%d want 1", got)
	}
	if snap.Duration.Count != 4 {
		t.Fatalf("Duration.Count=%d want 4 (1 success + 3 failures)", snap.Duration.Count)
	}
	if snap.AuditLogFailureTotal != 2 {
		t.Fatalf("AuditLogFailureTotal=%d want 2", snap.AuditLogFailureTotal)
	}
	if snap.Duration.MaxSeconds < 1.199 || snap.Duration.MaxSeconds > 1.201 {
		t.Fatalf("Duration.MaxSeconds=%f want ~1.2", snap.Duration.MaxSeconds)
	}
}

func TestInMemoryMetricsIsConcurrencySafe(t *testing.T) {
	t.Parallel()

	m := &InMemoryMetrics{}
	const goroutines = 16
	const perGoroutine = 200

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				m.RecordCleanAttempt()
				m.RecordCleanSuccess(10*time.Millisecond, 1)
				m.RecordCleanFailure(CleanFailureCauseFetchTransient, 5*time.Millisecond)
				m.RecordCleanDeleteFailure(CleanFailureClassTransient)
			}
		}()
	}
	wg.Wait()

	snap := m.Snapshot().Clean
	total := int64(goroutines * perGoroutine)
	if snap.AttemptsTotal != total {
		t.Fatalf("AttemptsTotal=%d want %d", snap.AttemptsTotal, total)
	}
	if snap.SuccessTotal != total {
		t.Fatalf("SuccessTotal=%d want %d", snap.SuccessTotal, total)
	}
	if snap.FailureTotal != total {
		t.Fatalf("FailureTotal=%d want %d", snap.FailureTotal, total)
	}
}

func TestClassifyCleanFetchFailureCovers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		class CleanFailureClass
		want  string
	}{
		{CleanFailureClassForbidden, CleanFailureCauseFetchForbidden},
		{CleanFailureClassMissingChannel, CleanFailureCauseFetchMissing},
		{CleanFailureClassRateLimited, CleanFailureCauseFetchRateLimited},
		{CleanFailureClassTransient, CleanFailureCauseFetchTransient},
		{CleanFailureClassUnknown, CleanFailureCauseFetchUnknown},
		{CleanFailureClassBulkDeleteAge, CleanFailureCauseFetchUnknown},
	}

	for _, tc := range cases {
		if got := ClassifyCleanFetchFailure(tc.class); got != tc.want {
			t.Errorf("ClassifyCleanFetchFailure(%v)=%q want %q", tc.class, got, tc.want)
		}
	}
}

func TestNopMetricsSatisfiesInterfaceWithoutSnapshotProvider(t *testing.T) {
	t.Parallel()

	var m Metrics = NopMetrics{}
	m.RecordCleanAttempt()
	m.RecordCleanSuccess(time.Second, 3)
	m.RecordCleanFailure("anything", time.Millisecond)
	m.RecordCleanDeleteFailure(CleanFailureClassUnknown)
	m.RecordCleanAuditLogFailure()

	if _, ok := m.(SnapshotProvider); ok {
		t.Fatal("NopMetrics must NOT satisfy SnapshotProvider; the route uses the type assertion to detect missing observability")
	}
}

func TestInMemoryMetricsSatisfiesSnapshotProvider(t *testing.T) {
	t.Parallel()

	var m Metrics = &InMemoryMetrics{}
	if _, ok := m.(SnapshotProvider); !ok {
		t.Fatal("InMemoryMetrics must satisfy SnapshotProvider for /v1/health/moderation")
	}
}
