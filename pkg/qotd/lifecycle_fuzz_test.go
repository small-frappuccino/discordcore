package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// FuzzCalculateNextPublishDelay fuzzes the delay calculation to ensure it never panics
// or returns a negative delay (which would cause a tight CPU loop in the daemon).
func FuzzCalculateNextPublishDelay(f *testing.F) {
	f.Add(int64(0), int64(0), int64(0), int64(0), int64(0))              // zeros
	f.Add(int64(1672531200), int64(12), int64(30), int64(0), int64(0))   // normal
	f.Add(int64(-62135596800), int64(23), int64(59), int64(0), int64(0)) // extreme past
	f.Add(int64(253402300799), int64(0), int64(0), int64(0), int64(0))   // extreme future

	f.Fuzz(func(t *testing.T, nowSec int64, hour int64, minute int64, locOffset int64, suppressSec int64) {
		// Constrain values to valid schedule ranges
		if hour < 0 || hour > 23 {
			hour = hour % 24
			if hour < 0 {
				hour += 24
			}
		}
		if minute < 0 || minute > 59 {
			minute = minute % 60
			if minute < 0 {
				minute += 60
			}
		}

		now := time.Unix(nowSec, 0)

		cfg := files.QOTDConfig{
			Schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(int(hour)),
				MinuteUTC: intPtr(int(minute)),
			},
		}

		// Execute the delay function
		delay := CalculateNextPublishDelay(cfg, now)

		// A negative delay means the timer loop would spin constantly
		if delay < 0 {
			t.Errorf("CalculateNextPublishDelay returned negative delay: %v", delay)
		}
	})
}

// FuzzDetermineOfficialPostLifecycle ensures the state machine boundaries
// never panic when provided with random boundary data.
func FuzzDetermineOfficialPostLifecycle(f *testing.F) {
	f.Add(int64(0), int64(0), int64(0), int64(0), "current")
	f.Add(int64(1672531200), int64(1672531200), int64(12), int64(30), "previous")

	f.Fuzz(func(t *testing.T, publishDateSec int64, nowSec int64, hour int64, min int64, state string) {
		if hour < 0 || hour > 23 {
			hour = 0
		}
		if min < 0 || min > 59 {
			min = 0
		}

		publishDate := time.Unix(publishDateSec, 0)
		now := time.Unix(nowSec, 0)

		post := OfficialPostRecord{
			PublishDateUTC: publishDate,
			State:          state,
		}

		schedule := files.QOTDPublishScheduleConfig{
			HourUTC:   intPtr(int(hour)),
			MinuteUTC: intPtr(int(min)),
		}

		lc := DetermineOfficialPostLifecycle(post, schedule, now)

		// Assertions
		if lc.PublishDateUTC.IsZero() && !publishDate.IsZero() {
			t.Errorf("PublishDateUTC normalized to zero unexpectedly")
		}
	})
}

func intPtr(i int) *int {
	return &i
}
