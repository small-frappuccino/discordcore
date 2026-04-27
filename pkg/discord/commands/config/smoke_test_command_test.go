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

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)
	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		falseValue := false
		cfg.Guilds[0].Features.Services.Commands = &falseValue
	})

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, smokeTestSubCommandName, nil))
	resp := rec.lastResponse(t)
	if err := ephemeralError(resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Data.Content, "/config list remains available while this guild is still dormant") {
		t.Fatalf("expected dormant /config list readiness in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Non-bootstrap routes remain blocked until /config commands_enabled true") {
		t.Fatalf("expected blocked non-bootstrap line in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Run /config command_channel <channel>") {
		t.Fatalf("expected command channel action in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Run /config qotd_channel <channel>") {
		t.Fatalf("expected qotd channel action in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Run /config qotd_schedule <hour> <minute>") {
		t.Fatalf("expected qotd schedule action in response, got %q", resp.Data.Content)
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

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)
	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		cfg.Guilds[0].Channels.Commands = "command-123"
		cfg.Guilds[0].QOTD = files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Schedule:     testCommandSchedule(),
			Decks: []files.QOTDDeckConfig{{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				ChannelID: "qotd-123",
			}},
		}
	})

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, smokeTestSubCommandName, nil))
	resp := rec.lastResponse(t)
	if err := ephemeralError(resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Data.Content, "Command channel configured: <#command-123>") {
		t.Fatalf("expected command channel pass in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "Full slash command surface is enabled") {
		t.Fatalf("expected full slash enabled line in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "QOTD channel configured: <#qotd-123>") {
		t.Fatalf("expected qotd channel pass in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "QOTD publish schedule configured: 12:43 UTC") {
		t.Fatalf("expected qotd schedule pass in response, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "QOTD is ready to enable. Run /config qotd_enabled true") {
		t.Fatalf("expected qotd enable readiness line in response, got %q", resp.Data.Content)
	}
}