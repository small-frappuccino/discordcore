package qotd

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestUncoveredLifecycleAndService(t *testing.T) {
	t.Parallel()
	// 1. Test resolvePublishSchedule
	cfgEmpty := files.QOTDConfig{}
	_, err := resolvePublishSchedule(cfgEmpty)
	if err == nil {
		t.Error("expected error for empty schedule config")
	}

	h := 12
	m := 30
	cfgValid := files.QOTDConfig{
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &h,
			MinuteUTC: &m,
		},
	}
	sched, err := resolvePublishSchedule(cfgValid)
	if err != nil {
		t.Errorf("expected no error for valid schedule config, got %v", err)
	}
	if *sched.HourUTC != 12 || *sched.MinuteUTC != 30 {
		t.Errorf("resolved schedule values mismatch: hour=%d, minute=%d", *sched.HourUTC, *sched.MinuteUTC)
	}

	// 2. Test DuePublishDateUTC
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	due := DuePublishDateUTC(sched, now)
	expectedDue := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	if !due.Equal(expectedDue) {
		t.Errorf("expected due publish date %v, got %v", expectedDue, due)
	}

	nowAfter := time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
	dueAfter := DuePublishDateUTC(sched, nowAfter)
	expectedDueAfter := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	if !dueAfter.Equal(expectedDueAfter) {
		t.Errorf("expected due publish date %v, got %v", expectedDueAfter, dueAfter)
	}

	// 4. Test ShouldConsumeAutomaticSlot
	pDefault := PublishNowParams{}
	if !pDefault.ShouldConsumeAutomaticSlot() {
		t.Error("expected ShouldConsumeAutomaticSlot to return true by default")
	}
	tTrue := true
	pTrue := PublishNowParams{ConsumeAutomaticSlot: &tTrue}
	if !pTrue.ShouldConsumeAutomaticSlot() {
		t.Error("expected ShouldConsumeAutomaticSlot to return true")
	}
	tFalse := false
	pFalse := PublishNowParams{ConsumeAutomaticSlot: &tFalse}
	if pFalse.ShouldConsumeAutomaticSlot() {
		t.Error("expected ShouldConsumeAutomaticSlot to return false")
	}

	// 5. Test service publisher/clock setters
	mgr := files.NewConfigManagerWithStore(nil, nil)
	mgr.ApplyConfig(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				QOTD: files.QOTDConfig{
					Schedule: files.QOTDPublishScheduleConfig{
						HourUTC:   &h,
						MinuteUTC: &m,
					},
				},
			},
		},
	})

	svc := NewService(mgr)
	if svc.GetPublisher() != nil {
		t.Error("expected publisher to be nil initially")
	}
	var dummyPub dummyPublisher
	svc.SetPublisher(&dummyPub)
	if svc.GetPublisher() != &dummyPub {
		t.Error("expected publisher to be injected")
	}

	svc.SetClock(nil) // should not panic and default to real clock
	tFixed := time.Date(2026, 6, 23, 15, 0, 0, 0, time.UTC)
	svc.SetClock(clock.NewMockClock(tFixed))
	if !svc.now().Equal(tFixed) {
		t.Errorf("expected clock to be fake clock with time %v, got %v", tFixed, svc.now())
	}

	// Test NextScheduledPublishTime
	nextSchedTime := svc.NextScheduledPublishTime("g1")
	expectedNextTime := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	if !nextSchedTime.Equal(expectedNextTime) {
		t.Errorf("expected next scheduled publish time %v, got %v", expectedNextTime, nextSchedTime)
	}

	nextSchedTimeInvalid := svc.NextScheduledPublishTime("g_invalid")
	if !nextSchedTimeInvalid.IsZero() {
		t.Errorf("expected zero time for invalid guild config, got %v", nextSchedTimeInvalid)
	}

	// Test NopMetrics to hit 100% coverage
	var nop NopMetrics
	nop.RecordOfficialPostAbandoned()
	nop.RecordSuppressionCleared()
}

type dummyPublisher struct{}

func (d *dummyPublisher) PublishOfficialPost(ctx context.Context, params PublishOfficialPostParams) (*PublishedOfficialPost, error) {
	return nil, nil
}
func (d *dummyPublisher) DeleteOfficialPost(ctx context.Context, params DeleteOfficialPostParams) error {
	return nil
}
