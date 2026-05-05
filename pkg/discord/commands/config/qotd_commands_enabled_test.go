package config

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestQOTDConfigEnabledCommandTogglesEnabledState(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)
	mustSetGuildQOTDConfig(t, harness.cm, guildID, buildTestQOTDConfig(false, "123456789012345678", testCommandSchedule()))

	enableResp := harness.runSlash(t, qotdEnabledSubCommandName,
		boolOpt(qotdEnabledOptionName, true),
	)

	assertPublicContains(t, enableResp, "QOTD publishing is now enabled")
	assertActiveQOTDDeckState(t, harness.cm, guildID, "123456789012345678", true, testCommandSchedule())

	disableResp := harness.runSlash(t, qotdEnabledSubCommandName,
		boolOpt(qotdEnabledOptionName, false),
	)

	assertPublicContains(t, disableResp, "QOTD publishing is now disabled")
	assertActiveQOTDDeckState(t, harness.cm, guildID, "123456789012345678", false, testCommandSchedule())
}

func TestQOTDConfigEnabledCommandRejectsEnableWithoutChannel(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)

	resp := harness.runSlash(t, qotdEnabledSubCommandName,
		boolOpt(qotdEnabledOptionName, true),
	)

	assertEphemeralContains(t, resp, "this deck still has no channel")

	qotdConfig, err := harness.cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	if !qotdConfig.IsZero() {
		t.Fatalf("expected qotd config to remain empty, got %+v", qotdConfig)
	}
}

func TestQOTDConfigEnabledCommandRejectsEnableWithoutSchedule(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)
	mustSetGuildQOTDConfig(t, harness.cm, guildID, buildTestQOTDConfig(false, "123456789012345678", files.QOTDPublishScheduleConfig{}))

	resp := harness.runSlash(t, qotdEnabledSubCommandName,
		boolOpt(qotdEnabledOptionName, true),
	)

	assertEphemeralContains(t, resp, "the schedule is incomplete")
	assertActiveQOTDDeckState(t, harness.cm, guildID, "123456789012345678", false, files.QOTDPublishScheduleConfig{})
}

func TestQOTDConfigEnabledCommandSuppressesCurrentSlotWhenBecomingPublishable(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fixedNow := func() time.Time {
		return time.Date(2026, 5, 3, 6, 14, 0, 0, time.UTC)
	}
	harness := newConfigCommandTestHarnessWithClock(t, guildID, ownerID, fixedNow)
	mustSetGuildQOTDConfig(t, harness.cm, guildID, buildTestQOTDConfig(false, "123456789012345678", testCommandSchedule()))

	enableResp := harness.runSlash(t, qotdEnabledSubCommandName,
		boolOpt(qotdEnabledOptionName, true),
	)

	assertPublicContains(t, enableResp, "QOTD publishing is now enabled")
	assertActiveQOTDDeckState(t, harness.cm, guildID, "123456789012345678", true, testCommandSchedule())
	assertQOTDSuppressionDate(t, harness.cm, guildID, "2026-05-02")
}