package qotd

import "time"

const (
	PublishHourUTC   = 12
	PublishMinuteUTC = 43
)

// NormalizePublishDateUTC returns the canonical UTC calendar date for a QOTD slot.
func NormalizePublishDateUTC(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// PublishTimeUTC returns the exact publish time for a QOTD date.
func PublishTimeUTC(publishDate time.Time) time.Time {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return time.Time{}
	}
	return time.Date(date.Year(), date.Month(), date.Day(), PublishHourUTC, PublishMinuteUTC, 0, 0, time.UTC)
}

// CurrentPublishDateUTC returns the active publish date at the given time.
func CurrentPublishDateUTC(now time.Time) time.Time {
	now = normalizeClockInput(now)
	today := NormalizePublishDateUTC(now)
	if now.Before(PublishTimeUTC(today)) {
		return today.AddDate(0, 0, -1)
	}
	return today
}

// PreviousPublishDateUTC returns the immediately previous publish date.
func PreviousPublishDateUTC(now time.Time) time.Time {
	return CurrentPublishDateUTC(now).AddDate(0, 0, -1)
}

// NextPublishTimeUTC returns the next publish boundary after now.
func NextPublishTimeUTC(now time.Time) time.Time {
	current := CurrentPublishDateUTC(now)
	return PublishTimeUTC(current.AddDate(0, 0, 1))
}

func normalizeClockInput(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
