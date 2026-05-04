package config

import (
	"testing"
)

func TestQOTDConfigScheduleCommandPersistsSchedule(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)

	resp := harness.runSlash(t, qotdScheduleSubCommandName,
		intOpt(qotdScheduleHourOptionName, 12),
		intOpt(qotdScheduleMinuteOptionName, 43),
	)

	assertPublicContains(t, resp, "QOTD for the active deck will now post at")
	assertActiveQOTDDeckState(t, harness.cm, guildID, "", false, testCommandSchedule())
}