package config

import (
	"strings"
	"testing"
)

func TestQOTDConfigGetReportsCurrentState(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)
	mustSetGuildQOTDConfig(t, harness.cm, guildID, buildTestQOTDConfig(true, "channel-555", testCommandSchedule()))

	resp := harness.runSlash(t, "get")
	assertPublicResponse(t, resp)
	if len(resp.Data.Embeds) != 1 {
		t.Fatalf("expected config get response to include one embed, got %+v", resp.Data.Embeds)
	}
	description := resp.Data.Embeds[0].Description
	if !strings.Contains(description, "QOTD Channel: channel-555") {
		t.Fatalf("expected qotd channel line in config output, got %q", description)
	}
}