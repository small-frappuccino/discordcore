package qotd

import (
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// PublishSchedule represents a QOTD publish schedule config.
type PublishSchedule = files.QOTDPublishScheduleConfig

// resolvePublishSchedule resolves the schedule from config.
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
	if now.After(PublishTimeUTC(schedule, today)) {
		return today.AddDate(0, 0, 1)
	}
	return today
}

// DuePublishDateUTC returns the most recent scheduled publish date at the given time.
func DuePublishDateUTC(schedule PublishSchedule, now time.Time) time.Time {
	now = normalizeClockInput(now)
	today := NormalizePublishDateUTC(now)
	if now.Before(PublishTimeUTC(schedule, today)) {
		return today.AddDate(0, 0, -1)
	}
	return today
}

// normalizeClockInput normalizes input to UTC.
func normalizeClockInput(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

// DetermineOfficialPostLifecycle derives the state and boundaries.
func DetermineOfficialPostLifecycle(post OfficialPostRecord, schedule files.QOTDPublishScheduleConfig, now time.Time) OfficialPostLifecycle {
	lc := OfficialPostLifecycle{
		PublishDateUTC: NormalizePublishDateUTC(post.PublishDateUTC),
		State:          OfficialPostState(post.State),
	}

	if schedule.IsComplete() {
		lc.PublishAt = PublishTimeUTC(schedule, lc.PublishDateUTC)
		// A post becomes previous the exact millisecond the next day's slot begins.
		lc.BecomesPreviousAt = PublishTimeUTC(schedule, lc.PublishDateUTC.AddDate(0, 0, 1))
		// It archives 24 hours after becoming previous.
		lc.ArchiveAt = lc.BecomesPreviousAt.AddDate(0, 0, 1)
	}

	lc.AnswerWindow = AnswerWindow{
		State:  lc.State,
		IsOpen: lc.State == OfficialPostStateCurrent,
	}
	if lc.AnswerWindow.IsOpen && !lc.BecomesPreviousAt.IsZero() {
		lc.AnswerWindow.ClosesAt = lc.BecomesPreviousAt
	}

	return lc
}

// CalculateNextPublishDelay returns the duration until the next schedule publish triggers.
func CalculateNextPublishDelay(cfg files.QOTDConfig, now time.Time) time.Duration {
	if !cfg.Schedule.IsComplete() {
		return 0
	}

	now = normalizeClockInput(now)
	publishDate := CurrentPublishDateUTC(cfg.Schedule, now)
	publishTime := PublishTimeUTC(cfg.Schedule, publishDate)

	delay := publishTime.Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}
