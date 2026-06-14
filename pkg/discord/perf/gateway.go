package perf

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/observability"
)

const (
	envGatewayPerfThresholdMs     = "DISCORDCORE_GATEWAY_PERF_THRESHOLD_MS"
	defaultGatewayPerfThresholdMs = int64(200)
)

var (
	gatewayThresholdOnce sync.Once
	gatewayThreshold     time.Duration

	gatewayMetricsMu sync.RWMutex
	gatewayMetrics   map[string]*observability.Summary
)

func gatewayPerfThreshold() time.Duration {
	gatewayThresholdOnce.Do(func() {
		ms := files.EnvInt64(envGatewayPerfThresholdMs, defaultGatewayPerfThresholdMs)
		if ms <= 0 {
			gatewayThreshold = 0
			return
		}
		gatewayThreshold = time.Duration(ms) * time.Millisecond
	})
	return gatewayThreshold
}

// StartGatewayEvent tracks how long a gateway handler takes. It aggregates
// all event execution latencies into an observability summary, and logs a
// warning if the event is slower than DISCORDCORE_GATEWAY_PERF_THRESHOLD_MS.
func StartGatewayEvent(event string, attrs ...slog.Attr) func() {
	start := time.Now()
	return func() {
		duration := time.Since(start)

		name := strings.TrimSpace(event)
		if name == "" {
			name = "unknown"
		}

		summary := observability.GetOrCreateLabeledSummary(&gatewayMetricsMu, &gatewayMetrics, name)
		summary.Observe(duration)

		threshold := gatewayPerfThreshold()
		if threshold <= 0 || duration < threshold {
			return
		}
		payload := make([]slog.Attr, 0, len(attrs)+3)
		payload = append(payload, slog.String("event", name))
		payload = append(payload, slog.Duration("duration", duration))
		payload = append(payload, slog.Int64("duration_ms", duration.Milliseconds()))
		payload = append(payload, attrs...)
		args := make([]any, 0, len(payload))
		for _, attr := range payload {
			args = append(args, attr)
		}
		log.DiscordLogger().Warn("slow gateway event handler", args...)
	}
}

// GatewayMetricsSnapshot is a snapshot of all gateway event latencies.
type GatewayMetricsSnapshot map[string]observability.SummarySnapshot

// SnapshotGatewayMetrics returns a snapshot of all gateway event latencies.
func SnapshotGatewayMetrics() GatewayMetricsSnapshot {
	gatewayMetricsMu.RLock()
	defer gatewayMetricsMu.RUnlock()
	snapshot := make(GatewayMetricsSnapshot, len(gatewayMetrics))
	for name, summary := range gatewayMetrics {
		snapshot[name] = summary.Snapshot()
	}
	return snapshot
}
