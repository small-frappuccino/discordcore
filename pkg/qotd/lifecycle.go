package qotd

import "time"

// BecomesPreviousAt returns when a published QOTD leaves the current-day slot.
func BecomesPreviousAt(publishDate time.Time) time.Time {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return time.Time{}
	}
	return PublishTimeUTC(date.AddDate(0, 0, 1))
}

// ArchiveAt returns when a published QOTD ages out of the current+previous window.
func ArchiveAt(publishDate time.Time) time.Time {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return time.Time{}
	}
	return PublishTimeUTC(date.AddDate(0, 0, 2))
}

// StateAt returns the lifecycle state for an official QOTD post at a given time.
func StateAt(publishDate, now time.Time) OfficialPostState {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return OfficialPostStateArchived
	}
	now = normalizeClockInput(now)

	switch {
	case now.Before(BecomesPreviousAt(date)):
		return OfficialPostStateCurrent
	case now.Before(ArchiveAt(date)):
		return OfficialPostStatePrevious
	default:
		return OfficialPostStateArchived
	}
}

// EvaluateOfficialPost returns the pure lifecycle view for a published official post.
func EvaluateOfficialPost(publishDate, now time.Time) OfficialPostLifecycle {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return OfficialPostLifecycle{}
	}
	now = normalizeClockInput(now)

	state := StateAt(date, now)
	archiveAt := ArchiveAt(date)
	answerWindow := AnswerWindow{
		IsOpen:   state == OfficialPostStateCurrent || state == OfficialPostStatePrevious,
		State:    state,
		ClosesAt: archiveAt,
	}

	return OfficialPostLifecycle{
		PublishDateUTC:    date,
		PublishAt:         PublishTimeUTC(date),
		BecomesPreviousAt: BecomesPreviousAt(date),
		ArchiveAt:         archiveAt,
		State:             state,
		AnswerWindow:      answerWindow,
	}
}

// CanAnswer reports whether new reply threads may still be created for a post.
func CanAnswer(publishDate, now time.Time) bool {
	return EvaluateOfficialPost(publishDate, now).AnswerWindow.IsOpen
}

// ShouldArchive reports whether the post should be archived now, ignoring already-archived rows.
func ShouldArchive(publishDate, now time.Time, archivedAt *time.Time) bool {
	if archivedAt != nil && !archivedAt.IsZero() {
		return false
	}
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return false
	}
	now = normalizeClockInput(now)
	return !now.Before(ArchiveAt(date))
}
