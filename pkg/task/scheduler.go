package task

import (
	"sync"
	"time"
)

// Cancel is a function that cancels a scheduled job.
type Cancel func()

// ScheduleDailyAtUTC schedules a task to start at the next occurrence of hour:minute:00 (UTC)
// and then repeats it every 24 hours.
//
// It returns a Cancel function that stops the pending first run (if it hasn't executed yet)
// and cancels the repeating job once it is registered.
func (tr *TaskRouter) ScheduleDailyAtUTC(hour, minute int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(1, hour, minute, 0, t)
}

// ScheduleDailyAtUTCWithSeconds schedules a task to start at the next occurrence of hour:minute:second (UTC)
// and then repeats it every 24 hours.
//
// It returns a Cancel function that stops the pending first run (if it hasn't executed yet)
// and cancels the repeating job once it is registered.
func (tr *TaskRouter) ScheduleDailyAtUTCWithSeconds(hour, minute, second int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(1, hour, minute, second, t)
}

// ScheduleEveryNDaysAtUTC schedules a task to start at the next occurrence of hour:minute:00 (UTC)
// and then repeats it every N days. If n <= 0, it defaults to 1 (daily).
//
// It returns a Cancel function that stops the pending first run (if it hasn't executed yet)
// and cancels the repeating job once it is registered.
func (tr *TaskRouter) ScheduleEveryNDaysAtUTC(n int, hour, minute int, t Task) Cancel {
	return tr.ScheduleEveryNDaysAtUTCWithSeconds(n, hour, minute, 0, t)
}

// ScheduleEveryNDaysAtUTCWithSeconds schedules a task to start at the next occurrence of hour:minute:second (UTC)
// and then repeats it every N days. If n <= 0, it defaults to 1 (daily).
//
// It returns a Cancel function that stops the pending first run (if it hasn't executed yet)
// and cancels the repeating job once it is registered.
func (tr *TaskRouter) ScheduleEveryNDaysAtUTCWithSeconds(n int, hour, minute, second int, t Task) Cancel {
	if n <= 0 {
		n = 1
	}
	// Normalize inputs to sane ranges
	hour = clampInt(hour, 0, 23)
	minute = clampInt(minute, 0, 59)
	second = clampInt(second, 0, 59)

	interval := time.Duration(n) * 24 * time.Hour

	cancelCh := make(chan struct{})
	var once sync.Once

	var innerMu sync.Mutex
	var innerCancel Cancel

	// Helper to cancel both the pending timer (by closing cancelCh) and the repeating job (if any)
	cancel := func() {
		once.Do(func() {
			close(cancelCh)
			innerMu.Lock()
			if innerCancel != nil {
				innerCancel()
				innerCancel = nil
			}
			innerMu.Unlock()
		})
	}

	go func() {
		// Compute next target at UTC
		now := time.Now().UTC()
		target := nextUTCTimestamp(now, hour, minute, second)
		if !now.Before(target) {
			// If now is equal or after target, schedule for the next N-day boundary anchored on today's target
			target = target.Add(interval)
		}
		delay := time.Until(target)

		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			// Register repeating job every N days from the first target run
			repeater := tr.ScheduleEvery(interval, t)
			innerMu.Lock()
			innerCancel = repeater
			innerMu.Unlock()

			// Wait for external cancellation
			<-cancelCh
			innerMu.Lock()
			if innerCancel != nil {
				innerCancel()
				innerCancel = nil
			}
			innerMu.Unlock()
			return

		case <-cancelCh:
			// Cancel before first run; nothing else to do
			return
		}
	}()

	return cancel
}

// nextUTCTimestamp returns the timestamp (UTC) for today at hour:minute:second
// relative to the provided "from" time (which is interpreted as UTC).
func nextUTCTimestamp(from time.Time, hour, minute, second int) time.Time {
	return time.Date(from.Year(), from.Month(), from.Day(), hour, minute, second, 0, time.UTC)
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
