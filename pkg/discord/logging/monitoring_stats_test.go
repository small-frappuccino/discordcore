package logging

import (
	"strings"
	"testing"

	svc "github.com/small-frappuccino/discordcore/pkg/service"
)

// TestMonitoringServiceMetricsRowsOrderAndLabels pins the display ordering
// of MonitoringService.metricsRows so /admin status stays predictable across
// builds. Renaming a label or reordering the slice here is a UX-visible
// breaking change for operators.
func TestMonitoringServiceMetricsRowsOrderAndLabels(t *testing.T) {
	t.Parallel()

	metrics := &InMemoryMetrics{}
	metrics.RecordAuditLogCall()
	metrics.RecordGuildMemberCall()
	metrics.RecordMessageSent()
	metrics.RecordStateMemberCacheHit()
	metrics.RecordRolesCacheMemoryHit()
	metrics.RecordRolesCacheStoreHit()
	metrics.RecordRolesAuditCacheHit()

	ms := &MonitoringService{
		rolesCacheService: NewRolesCacheService(nil),

		metrics: metrics}

	rows := ms.metricsRows()

	wantPrefixes := []string{
		"Running",
		"Roles cache size",
		"Roles cache default TTL",
		"Role update audit cache size",
		"Role update audit debounce size",
		"API audit log calls",
		"API guild member calls",
		"API messages sent",
		"State member cache hits",
		"Roles cache memory hits",
		"Roles cache store hits",
		"Roles audit cache hits"}

	if len(rows) < len(wantPrefixes) {
		t.Fatalf("expected at least %d rows, got %d (%+v)", len(wantPrefixes), len(rows), rows)
	}
	for i, wantLabel := range wantPrefixes {
		if rows[i].Label != wantLabel {
			t.Errorf("row %d label = %q, want %q", i, rows[i].Label, wantLabel)
		}
	}
}

// TestMonitoringServiceMetricsRowsMirrorObservability pins that the API/cache
// counter rows reflect the actual InMemoryMetrics snapshot — operators rely on
// the /admin status numbers matching /v1/health/monitoring.
func TestMonitoringServiceMetricsRowsMirrorObservability(t *testing.T) {
	t.Parallel()

	metrics := &InMemoryMetrics{}
	for i := 0; i < 3; i++ {
		metrics.RecordAuditLogCall()
	}
	for i := 0; i < 5; i++ {
		metrics.RecordMessageSent()
	}

	ms := &MonitoringService{
		rolesCacheService: NewRolesCacheService(nil),

		metrics: metrics}

	rows := ms.metricsRows()

	got := rowValueByLabel(rows, "API audit log calls")
	if got != "3" {
		t.Errorf("API audit log calls = %q, want %q", got, "3")
	}
	got = rowValueByLabel(rows, "API messages sent")
	if got != "5" {
		t.Errorf("API messages sent = %q, want %q", got, "5")
	}
	got = rowValueByLabel(rows, "API guild member calls")
	if got != "0" {
		t.Errorf("unset counter should still surface as %q, got %q", "0", got)
	}
}

// TestMonitoringServiceMetricsRowsWithoutObservability pins that NopMetrics
// (the test/default mode) leaves the API/cache counters off the display rows
// entirely rather than displaying noise. The local bookkeeping rows still
// surface because they read from the struct directly.
func TestMonitoringServiceMetricsRowsWithoutObservability(t *testing.T) {
	t.Parallel()

	ms := &MonitoringService{
		rolesCacheService: NewRolesCacheService(nil),

		// metrics intentionally nil — observability() yields NopMetrics.
	}

	rows := ms.metricsRows()

	for _, row := range rows {
		if strings.HasPrefix(row.Label, "API ") {
			t.Errorf("unexpected API row when no SnapshotProvider attached: %+v", row)
		}
		if strings.HasPrefix(row.Label, "State member") ||
			strings.HasPrefix(row.Label, "Roles cache memory") ||
			strings.HasPrefix(row.Label, "Roles cache store") ||
			strings.HasPrefix(row.Label, "Roles audit cache") {
			t.Errorf("unexpected cache hit row when no SnapshotProvider attached: %+v", row)
		}
	}

	// Local bookkeeping rows still appear.
	if rowValueByLabel(rows, "Running") == "" {
		t.Errorf("expected Running row even without observability, got rows=%+v", rows)
	}
}

// TestMonitoringServiceStatsReturnsTypedMetrics is the integration probe: the
// Service.Stats() contract surfaces the typed rows so /admin status reads
// from svc.ServiceStats.Metrics without resorting to map indexing.
func TestMonitoringServiceStatsReturnsTypedMetrics(t *testing.T) {
	t.Parallel()

	ms := &MonitoringService{
		rolesCacheService: NewRolesCacheService(nil),

		metrics: &InMemoryMetrics{}}

	stats := ms.Stats()
	if len(stats.Metrics) == 0 {
		t.Fatalf("expected Stats().Metrics to carry rows, got empty slice")
	}
	// Sanity-check the type — a compile-time assertion via the slice
	// element type would belong in the package types test, but the runtime
	// shape is what /admin status iterates.
	var _ svc.ServiceMetric = stats.Metrics[0]
}

func rowValueByLabel(rows []svc.ServiceMetric, label string) string {
	for _, row := range rows {
		if row.Label == label {
			return row.Value
		}
	}
	return ""
}
