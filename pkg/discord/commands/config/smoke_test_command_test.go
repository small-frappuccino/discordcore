package config

import (
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestSmokeTestSubCommandReportsDormantBootstrapAndQOTDActions(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)
	mustUpdateConfig(t, harness.cm, func(cfg *files.BotConfig) {
		falseValue := false
		cfg.Guilds[0].Features.Services.Commands = &falseValue
	})

	resp := harness.runSlash(t, smokeTestSubCommandName)
	assertEphemeralContains(t, resp, "General / Initial Setup")
	if !strings.Contains(resp.Data.Content, "/config list remains available while this guild is still dormant") {
		t.Fatalf("expected dormant /config list readiness in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Set the QOTD channel and schedule first") {
		t.Fatalf("expected qotd enable readiness guidance in response, got %q", resp.Data.Content)
	}
}

func TestSmokeTestSubCommandReportsReadyQOTDAndUnlockedCommands(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)
	mustUpdateConfig(t, harness.cm, func(cfg *files.BotConfig) {
		cfg.Guilds[0].Channels.Commands = "command-123"
	})
	mustSetGuildQOTDConfig(t, harness.cm, guildID, buildTestQOTDConfig(false, "qotd-123", testCommandSchedule()))

	resp := harness.runSlash(t, smokeTestSubCommandName)
	assertEphemeralContains(t, resp, "QOTD")
	if !strings.Contains(resp.Data.Content, "Full slash command surface is enabled") {
		t.Fatalf("expected full slash enabled line in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "QOTD is ready to enable. Run /config qotd_enabled true") {
		t.Fatalf("expected qotd enable readiness line in response, got %q", resp.Data.Content)
	}
}