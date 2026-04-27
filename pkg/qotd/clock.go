package qotd

import (
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type PublishSchedule = files.QOTDPublishScheduleConfig

func resolvePublishSchedule(cfg files.QOTDConfig) (PublishSchedule, error) {
	if !cfg.Schedule.IsComplete() {
		return PublishSchedule{}, fmt.Errorf("qotd publish schedule is incomplete")
	}
	return cfg.Schedule, nil
}

// NormalizePublishDateUTC returns the canonical UTC calendar date for a QOTD slot.
func NormalizePublishDateUTC(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// PublishTimeUTC returns the exact publish time for a QOTD date.
func PublishTimeUTC(schedule PublishSchedule, publishDate time.Time) time.Time {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return time.Time{}
	}
	hourUTC, minuteUTC, ok := schedule.Values()
	if !ok {
		return time.Time{}
	}
	return time.Date(date.Year(), date.Month(), date.Day(), hourUTC, minuteUTC, 0, 0, time.UTC)
}

// CurrentPublishDateUTC returns the active publish date at the given time.
func CurrentPublishDateUTC(schedule PublishSchedule, now time.Time) time.Time {
	now = normalizeClockInput(now)
	today := NormalizePublishDateUTC(now)
	if now.Before(PublishTimeUTC(schedule, today)) {
		return today.AddDate(0, 0, -1)
	}
	return today
}

// PreviousPublishDateUTC returns the immediately previous publish date.
func PreviousPublishDateUTC(schedule PublishSchedule, now time.Time) time.Time {
	return CurrentPublishDateUTC(schedule, now).AddDate(0, 0, -1)
}

// NextPublishTimeUTC returns the next publish boundary after now.
func NextPublishTimeUTC(schedule PublishSchedule, now time.Time) time.Time {
	current := CurrentPublishDateUTC(schedule, now)
	return PublishTimeUTC(schedule, current.AddDate(0, 0, 1))
}

func normalizeClockInput(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
