package qotd

import (
	"context"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type currentSlotState struct {
	ScheduleConfigured bool
	Schedule           PublishSchedule
	PublishDateUTC     time.Time
	PublishAtUTC       time.Time
	OfficialPost       *storage.QOTDOfficialPostRecord
}

func (st currentSlotState) BoundaryPassed(now time.Time) bool {
	if !st.ScheduleConfigured || st.PublishAtUTC.IsZero() {
		return false
	}
	now = normalizeClockInput(now)
	publishAt := st.PublishAtUTC.UTC()
	return !now.Before(publishAt)
}

func (st currentSlotState) HasOfficialPostRecord() bool {
	return st.OfficialPost != nil
}

func (st currentSlotState) HasPublishedOfficialPost() bool {
	return hasPublishedOfficialPostTarget(st.OfficialPost)
}

func (st currentSlotState) HasProvisioningOfficialPost() bool {
	if !st.HasOfficialPostRecord() {
		return false
	}
	if isOfficialPostAbandoned(*st.OfficialPost) {
		// The slot is permanently failed and needs human action. Treat it
		// like a published record from the runtime's perspective so the
		// publish loop neither resumes it nor inserts a sibling on top.
		return false
	}
	return !st.HasPublishedOfficialPost()
}

func (s *Service) loadCurrentSlotState(ctx context.Context, guildID string, cfg files.QOTDConfig, now time.Time) (currentSlotState, error) {
	return s.loadSlotState(ctx, guildID, cfg, now, CurrentPublishDateUTC)
}

func (s *Service) loadUpcomingSlotState(ctx context.Context, guildID string, cfg files.QOTDConfig, now time.Time) (currentSlotState, error) {
	return s.loadSlotState(ctx, guildID, cfg, now, CurrentPublishDateUTC)
}

// loadDueSlotState returns the state of TODAY's slot regardless of whether its
// publish time has already passed. Use this on the scheduled publish path:
// loadCurrentSlotState projects the next forward-looking slot (rolling to
// tomorrow as soon as today's publish time elapses by even one nanosecond),
// which makes BoundaryPassed almost impossible to satisfy from a 1-minute
// wall-clock-misaligned ticker. Here we keep PublishDateUTC anchored to today
// so BoundaryPassed reflects "today's slot is due".
//
// Unlike loadCurrentSlotState (which uses loadAutomaticSlotOfficialPost and
// therefore ignores non-consuming manual posts), this helper looks up ANY post
// recorded for today's date. The scheduler must back off whenever today
// already has a public post, regardless of mode — the unique index in
// qotd_official_posts is scoped to publish_mode='scheduled', so a non-consuming
// manual post does not block insertion at the DB level. Only by checking here
// do we avoid producing a duplicate post on top of a manual one.
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
	state.OfficialPost, err = s.loadSlotOfficialPost(ctx, guildID, state.PublishDateUTC)
	if err != nil {
		return currentSlotState{}, err
	}
	return state, nil
}

func (s *Service) loadSlotState(ctx context.Context, guildID string, cfg files.QOTDConfig, now time.Time, publishDate func(PublishSchedule, time.Time) time.Time) (currentSlotState, error) {
	state := currentSlotState{}
	schedule, err := resolvePublishSchedule(cfg)
	if err != nil {
		return state, nil
	}

	state.ScheduleConfigured = true
	state.Schedule = schedule
	state.PublishDateUTC = publishDate(schedule, now)
	state.PublishAtUTC = PublishTimeUTC(schedule, state.PublishDateUTC)
	state.OfficialPost, err = s.loadAutomaticSlotOfficialPost(ctx, guildID, state.PublishDateUTC)
	if err != nil {
		return currentSlotState{}, err
	}
	return state, nil
}

func (s *Service) loadSlotOfficialPost(ctx context.Context, guildID string, publishDate time.Time) (*storage.QOTDOfficialPostRecord, error) {
	publishDate = NormalizePublishDateUTC(publishDate)
	if publishDate.IsZero() {
		return nil, nil
	}
	return s.store.GetQOTDOfficialPostByDate(ctx, strings.TrimSpace(guildID), publishDate)
}

func (s *Service) loadAutomaticSlotOfficialPost(ctx context.Context, guildID string, publishDate time.Time) (*storage.QOTDOfficialPostRecord, error) {
	publishDate = NormalizePublishDateUTC(publishDate)
	if publishDate.IsZero() {
		return nil, nil
	}
	return s.store.GetAutomaticSlotQOTDOfficialPostByDate(ctx, strings.TrimSpace(guildID), publishDate)
}
