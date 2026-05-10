package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// publishedOfficialPostFixture returns a record that satisfies
// isOfficialPostProvisioningComplete + has a PublishedAt timestamp, so the
// helpers under test recognize it as a finished publish. Each field is the
// minimum required by the helper contract; if a contract gains another
// required field the test file fails loudly instead of silently skipping it.
func publishedOfficialPostFixture(state OfficialPostState) storage.QOTDOfficialPostRecord {
	publishedAt := time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC)
	return storage.QOTDOfficialPostRecord{
		State:                   string(state),
		DiscordThreadID:         "thread-1",
		DiscordStarterMessageID: "starter-1",
		AnswerChannelID:         "answers-1",
		PublishedAt:             &publishedAt,
	}
}

func TestCurrentSlotStateBoundaryPassedIgnoresUnconfiguredAndCompares(t *testing.T) {
	t.Parallel()

	publishAt := time.Date(2026, 5, 7, 12, 43, 0, 0, time.UTC)

	cases := []struct {
		name string
		st   currentSlotState
		now  time.Time
		want bool
	}{
		{
			name: "schedule not configured never passes",
			st:   currentSlotState{ScheduleConfigured: false, PublishAtUTC: publishAt},
			now:  publishAt.Add(time.Hour),
			want: false,
		},
		{
			name: "zero publish time never passes",
			st:   currentSlotState{ScheduleConfigured: true},
			now:  publishAt,
			want: false,
		},
		{
			name: "before publish stays unpassed",
			st:   currentSlotState{ScheduleConfigured: true, PublishAtUTC: publishAt},
			now:  publishAt.Add(-time.Nanosecond),
			want: false,
		},
		{
			name: "exact boundary is passed",
			st:   currentSlotState{ScheduleConfigured: true, PublishAtUTC: publishAt},
			now:  publishAt,
			want: true,
		},
		{
			name: "after boundary is passed",
			st:   currentSlotState{ScheduleConfigured: true, PublishAtUTC: publishAt},
			now:  publishAt.Add(time.Hour),
			want: true,
		},
		{
			name: "non-utc now still compared in utc",
			st:   currentSlotState{ScheduleConfigured: true, PublishAtUTC: publishAt},
			now:  publishAt.In(time.FixedZone("ahead", 3*60*60)),
			want: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.st.BoundaryPassed(tc.now); got != tc.want {
				t.Fatalf("BoundaryPassed() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCurrentSlotStateHasOfficialPostRecordReflectsPointer(t *testing.T) {
	t.Parallel()

	if (currentSlotState{}).HasOfficialPostRecord() {
		t.Fatal("expected nil official post pointer to report no record")
	}
	post := publishedOfficialPostFixture(OfficialPostStateCurrent)
	state := currentSlotState{OfficialPost: &post}
	if !state.HasOfficialPostRecord() {
		t.Fatal("expected non-nil official post pointer to report a record")
	}
}

// TestCurrentSlotStateClassifiesProvisioningVsPublishedVsAbandoned pins the
// classification the publish loop relies on:
//
//   - a published row blocks both new inserts and resume work;
//   - a provisioning row triggers resume work;
//   - an abandoned row blocks resume work entirely (admin must intervene).
//
// A regression in any one of the three classifications either spams Discord
// (resuming an abandoned post forever) or silently double-publishes (treating
// a provisioning row as published).
func TestCurrentSlotStateClassifiesProvisioningVsPublishedVsAbandoned(t *testing.T) {
	t.Parallel()

	provisioning := storage.QOTDOfficialPostRecord{
		State: string(OfficialPostStateProvisioning),
	}
	published := publishedOfficialPostFixture(OfficialPostStateCurrent)
	abandoned := storage.QOTDOfficialPostRecord{
		State: string(OfficialPostStateAbandoned),
	}

	cases := []struct {
		name             string
		state            currentSlotState
		wantPublished    bool
		wantProvisioning bool
	}{
		{name: "no record at all", state: currentSlotState{}, wantPublished: false, wantProvisioning: false},
		{name: "provisioning record", state: currentSlotState{OfficialPost: &provisioning}, wantPublished: false, wantProvisioning: true},
		{name: "published record", state: currentSlotState{OfficialPost: &published}, wantPublished: true, wantProvisioning: false},
		{name: "abandoned record stays terminal", state: currentSlotState{OfficialPost: &abandoned}, wantPublished: false, wantProvisioning: false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.state.HasPublishedOfficialPost(); got != tc.wantPublished {
				t.Fatalf("HasPublishedOfficialPost() = %v, want %v", got, tc.wantPublished)
			}
			if got := tc.state.HasProvisioningOfficialPost(); got != tc.wantProvisioning {
				t.Fatalf("HasProvisioningOfficialPost() = %v, want %v", got, tc.wantProvisioning)
			}
		})
	}
}
