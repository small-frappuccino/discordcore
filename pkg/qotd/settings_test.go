package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestSuppressedPublishDateOnEnableSuppressesCurrentSlotWhenBecomingPublishable(t *testing.T) {
	t.Parallel()

	hourUTC := 12
	minuteUTC := 43
	now := time.Date(2026, 5, 3, 6, 14, 0, 0, time.UTC)
	current := files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		},
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   false,
			ChannelID: "123456789012345678",
		}},
	}
	next := files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		},
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	}

	publishDate, suppress := suppressedPublishDateOnEnable(current, next, now)
	if !suppress {
		t.Fatal("expected enabling automatic publish to suppress the active due slot")
	}

	want := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	if !publishDate.Equal(want) {
		t.Fatalf("expected suppression for active slot %s, got %s", want.Format(time.RFC3339), publishDate.Format(time.RFC3339))
	}
}

func TestSuppressedPublishDateOnEnableDoesNotSuppressWhenAlreadyPublishable(t *testing.T) {
	t.Parallel()

	hourUTC := 12
	minuteUTC := 43
	now := time.Date(2026, 5, 3, 13, 0, 0, 0, time.UTC)
	current := files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &hourUTC,
			MinuteUTC: &minuteUTC,
		},
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "123456789012345678",
		}},
	}
	next := current

	if _, suppress := suppressedPublishDateOnEnable(current, next, now); suppress {
		t.Fatal("expected no suppression when automatic publish is already configured")
	}
}
