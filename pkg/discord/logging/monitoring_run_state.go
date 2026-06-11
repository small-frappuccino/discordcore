package logging

import (
	"context"
	"time"
)

// monitoringRunState groups the run-state of a MonitoringService: the running
// flag, lifecycle timestamps and counters, the lifecycle context and its
// cancel func, the stop channel guarded by stopOnce, the owned-worker wait
// group, and the cancel handles for the scheduled monitor jobs.
//
// Every field is guarded by MonitoringService.runMu, the lock that serializes
// Start and Stop, with one exception: wg is internally synchronized, so its
// Add/Done/Wait are called without holding runMu. The zero value is the
// not-running state.
type monitoringRunState struct {
	running      bool
	startTime    *time.Time
	stopTime     *time.Time
	restartCount int
	errorCount   int
	lastErrorAt  *time.Time
	ctx          context.Context
}
