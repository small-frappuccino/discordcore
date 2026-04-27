package qotd

import "time"

// BecomesPreviousAt returns when a published QOTD leaves the current-day slot.
func BecomesPreviousAt(schedule PublishSchedule, publishDate time.Time) time.Time {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return time.Time{}
	}
	return PublishTimeUTC(schedule, date.AddDate(0, 0, 1))
}

// ArchiveAt returns when a published QOTD ages out of the current+previous window.
func ArchiveAt(schedule PublishSchedule, publishDate time.Time) time.Time {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return time.Time{}
	}
	return PublishTimeUTC(schedule, date.AddDate(0, 0, 2))
}

// ManualBecomesPreviousAt returns when an independently published manual QOTD
// leaves its first 24-hour answer window.
func ManualBecomesPreviousAt(publishedAt time.Time) time.Time {
	publishedAt = normalizeClockInput(publishedAt)
	if publishedAt.IsZero() {
		return time.Time{}
	}
	return publishedAt.Add(24 * time.Hour)
}

// ManualArchiveAt returns when an independently published manual QOTD ages out
// of its two-day answer window.
func ManualArchiveAt(publishedAt time.Time) time.Time {
	publishedAt = normalizeClockInput(publishedAt)
	if publishedAt.IsZero() {
		return time.Time{}
	}
	return publishedAt.Add(48 * time.Hour)
}

// StateWithinWindow returns the lifecycle state for any official post whose
// current/previous/archive boundaries are already stored.
func StateWithinWindow(graceUntil, archiveAt, now time.Time) OfficialPostState {
	if graceUntil.IsZero() || archiveAt.IsZero() {
		return OfficialPostStateArchived
	}
	now = normalizeClockInput(now)
	graceUntil = normalizeClockInput(graceUntil)
	archiveAt = normalizeClockInput(archiveAt)

	switch {
	case now.Before(graceUntil):
		return OfficialPostStateCurrent
	case now.Before(archiveAt):
		return OfficialPostStatePrevious
	default:
		return OfficialPostStateArchived
	}
}

// StateAt returns the lifecycle state for an official QOTD post at a given time.
func StateAt(schedule PublishSchedule, publishDate, now time.Time) OfficialPostState {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return OfficialPostStateArchived
	}
	return StateWithinWindow(BecomesPreviousAt(schedule, date), ArchiveAt(schedule, date), now)
}

// EvaluateOfficialPostWindow returns the pure lifecycle view for any post whose
// current/previous/archive boundaries are already known.
func EvaluateOfficialPostWindow(publishDate, publishAt, graceUntil, archiveAt, now time.Time) OfficialPostLifecycle {
	publishDate = NormalizePublishDateUTC(publishDate)
	if publishDate.IsZero() {
		publishDate = NormalizePublishDateUTC(publishAt)
	}
	if publishDate.IsZero() || graceUntil.IsZero() || archiveAt.IsZero() {
		return OfficialPostLifecycle{}
	}
	now = normalizeClockInput(now)
	if publishAt.IsZero() {
	}
	publishAt = publishAt.UTC()
	graceUntil = graceUntil.UTC()
	archiveAt = archiveAt.UTC()

	state := StateWithinWindow(graceUntil, archiveAt, now)
	answerWindow := AnswerWindow{
		IsOpen:   state == OfficialPostStateCurrent || state == OfficialPostStatePrevious,
		State:    state,
		ClosesAt: archiveAt,
	}

	return OfficialPostLifecycle{
		PublishDateUTC:    publishDate,
		PublishAt:         publishAt,
		BecomesPreviousAt: graceUntil,
		ArchiveAt:         archiveAt,
		State:             state,
		AnswerWindow:      answerWindow,
	}
}

// EvaluateOfficialPost returns the pure lifecycle view for a published official post.
func EvaluateOfficialPost(schedule PublishSchedule, publishDate, now time.Time) OfficialPostLifecycle {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return OfficialPostLifecycle{}
	}
	return EvaluateOfficialPostWindow(date, PublishTimeUTC(schedule, date), BecomesPreviousAt(schedule, date), ArchiveAt(schedule, date), now)
}

// EvaluateManualOfficialPost returns the lifecycle view for a manually
// published official post whose window is anchored to its publish timestamp.
func EvaluateManualOfficialPost(publishedAt, now time.Time) OfficialPostLifecycle {
	publishedAt = normalizeClockInput(publishedAt)
	if publishedAt.IsZero() {
		return OfficialPostLifecycle{}
	}
	return EvaluateOfficialPostWindow(
		NormalizePublishDateUTC(publishedAt),
		publishedAt,
		ManualBecomesPreviousAt(publishedAt),
		ManualArchiveAt(publishedAt),
		now,
	)
}

// ShouldArchive reports whether the post should be archived now, ignoring already-archived rows.
func ShouldArchive(schedule PublishSchedule, publishDate, now time.Time, archivedAt *time.Time) bool {
	if archivedAt != nil && !archivedAt.IsZero() {
		return false
	}
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return false
	}
	now = normalizeClockInput(now)
	return !now.Before(ArchiveAt(schedule, date))
}
