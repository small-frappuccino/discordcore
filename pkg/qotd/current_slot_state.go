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
	publishAt := normalizeClockInput(st.PublishAtUTC)
	return !now.Before(publishAt)
}

func (st currentSlotState) HasOfficialPostRecord() bool {
	return st.OfficialPost != nil
}

func (st currentSlotState) HasPublishedOfficialPost() bool {
	return hasPublishedOfficialPostTarget(st.OfficialPost)
}

func (st currentSlotState) HasProvisioningOfficialPost() bool {
	return st.HasOfficialPostRecord() && !st.HasPublishedOfficialPost()
}

func (s *Service) loadCurrentSlotState(ctx context.Context, guildID string, cfg files.QOTDConfig, now time.Time) (currentSlotState, error) {
	state := currentSlotState{}
	schedule, err := resolvePublishSchedule(cfg)
	if err != nil {
		return state, nil
	}

	state.ScheduleConfigured = true
	state.Schedule = schedule
	state.PublishDateUTC = CurrentPublishDateUTC(schedule, now)
	state.PublishAtUTC = PublishTimeUTC(schedule, state.PublishDateUTC)
	state.OfficialPost, err = s.loadSlotOfficialPost(ctx, guildID, state.PublishDateUTC)
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