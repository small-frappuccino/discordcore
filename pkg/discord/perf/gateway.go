package perf

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

const (
	envGatewayPerfThresholdMs     = "ALICE_GATEWAY_PERF_THRESHOLD_MS"
	defaultGatewayPerfThresholdMs = int64(200)
)

var (
	gatewayThresholdOnce sync.Once
	gatewayThreshold     time.Duration
)

func gatewayPerfThreshold() time.Duration {
	gatewayThresholdOnce.Do(func() {
		ms := util.EnvInt64(envGatewayPerfThresholdMs, defaultGatewayPerfThresholdMs)
		if ms <= 0 {
			gatewayThreshold = 0
			return
		}
		gatewayThreshold = time.Duration(ms) * time.Millisecond
	})
	return gatewayThreshold
}

// StartGatewayEvent tracks how long a gateway handler takes and logs only when slow.
// Set ALICE_GATEWAY_PERF_THRESHOLD_MS to 0 to disable.
func StartGatewayEvent(event string, attrs ...slog.Attr) func() {
	threshold := gatewayPerfThreshold()
	if threshold <= 0 {
		return func() {}
	}

	start := time.Now()
	return func() {
		duration := time.Since(start)
		if duration < threshold {
			return
		}
		name := strings.TrimSpace(event)
		if name == "" {
			name = "unknown"
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
