package qotd

import (
	"context"
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

// currentSlotState maintains the state of the current slot.
type currentSlotState struct {
	ScheduleConfigured bool
	Schedule           PublishSchedule
	PublishDateUTC     time.Time
	PublishAtUTC       time.Time
	OfficialPost       *OfficialPostRecord
}

// BoundaryPassed boundarys passed.
func (st currentSlotState) BoundaryPassed(now time.Time) bool {
	if !st.ScheduleConfigured || st.PublishAtUTC.IsZero() {
		return false
	}
	now = normalizeClockInput(now)
	publishAt := st.PublishAtUTC.UTC()
	return !now.Before(publishAt)
}

// HasOfficialPostRecord has official post record.
func (st currentSlotState) HasOfficialPostRecord() bool {
	return st.OfficialPost != nil
}

// HasPublishedOfficialPost has published official post.
func (st currentSlotState) HasPublishedOfficialPost() bool {
	if st.OfficialPost == nil {
		return false
	}
	return st.OfficialPost.State != string(OfficialPostStateProvisioning) && st.OfficialPost.State != string(OfficialPostStateAbandoned)
}

// HasProvisioningOfficialPost has provisioning official post.
func (st currentSlotState) HasProvisioningOfficialPost() bool {
	if !st.HasOfficialPostRecord() {
		return false
	}
	if st.OfficialPost.State == string(OfficialPostStateAbandoned) {
		return false
	}
	return !st.HasPublishedOfficialPost()
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

// loadDueSlotState loads the slot state.
func (s *Service) loadDueSlotState(ctx context.Context, guildID string, cfg files.QOTDConfig, now time.Time) (currentSlotState, error) {
	state := currentSlotState{}
	schedule, err := resolvePublishSchedule(cfg)
	if err != nil {
		return state, nil
	}

	state.ScheduleConfigured = true
	state.Schedule = schedule
	state.PublishDateUTC = NormalizePublishDateUTC(now)
	state.PublishAtUTC = PublishTimeUTC(schedule, state.PublishDateUTC)
	state.OfficialPost, err = s.repo.GetQOTDOfficialPostByDate(ctx, guildID, state.PublishDateUTC)
	if err != nil {
		return currentSlotState{}, fmt.Errorf("loadDueSlotState: %w", err)
	}
	return state, nil
}
