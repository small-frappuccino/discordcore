package config

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func configStringOpt(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func TestConfigSetSubCommandUsesPublicConfirmation(t *testing.T) {
	const (
		guildID = "guild-set"
		ownerID = "owner-set"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)

	resp := harness.runSlash(t, "set",
		configStringOpt("key", "channels.commands"),
		configStringOpt("value", "channel-987"),
	)

	assertPublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "channels.commands") {
		t.Fatalf("unexpected config set response: %q", resp.Data.Content)
	}

	guild := harness.cm.GuildConfig(guildID)
	if guild == nil || guild.Channels.Commands != "channel-987" {
		t.Fatalf("expected commands channel persisted, got %+v", guild)
	}
}

func TestConfigGetSubCommandUsesPrivateReadPolicy(t *testing.T) {
	const (
		guildID = "guild-get"
		ownerID = "owner-get"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)
	mustSetGuildQOTDConfig(t, harness.cm, guildID, buildTestQOTDConfig(true, "channel-555", testCommandSchedule()))
	mustUpdateConfig(t, harness.cm, func(cfg *files.BotConfig) {
		cfg.Guilds[0].Channels.Commands = "command-321"
	})

	resp := harness.runSlash(t, "get")
	assertEphemeralResponse(t, resp)
	if len(resp.Data.Embeds) != 1 {
		t.Fatalf("expected config get response to include one embed, got %+v", resp.Data.Embeds)
	}
	description := resp.Data.Embeds[0].Description
	if !strings.Contains(description, "QOTD Channel: channel-555") || !strings.Contains(description, "Command Channel: command-321") {
		t.Fatalf("expected current config details in response, got %q", description)
	}
}

func TestConfigListSubCommandUsesPrivateListPolicy(t *testing.T) {
	const (
		guildID = "guild-list"
		ownerID = "owner-list"
	)

	harness := newConfigCommandTestHarness(t, guildID, ownerID)

	resp := harness.runSlash(t, "list")
	assertEphemeralResponse(t, resp)
	if len(resp.Data.Embeds) != 1 {
		t.Fatalf("expected config list response to include one embed, got %+v", resp.Data.Embeds)
	}
	description := resp.Data.Embeds[0].Description
	if !strings.Contains(description, "/config allowed_role_list") || !strings.Contains(description, "/config webhook_embed_list") {
		t.Fatalf("expected config option catalog in response, got %q", description)
	}
}