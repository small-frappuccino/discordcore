package config

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestQOTDConfigCommandsSetChannelAndToggleEnabled(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, qotdChannelSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		channelOpt(qotdChannelOptionName, "123456789012345678"),
	}))
	channelResp := rec.lastResponse(t)
	if err := ephemeralError(channelResp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(channelResp.Data.Content, "<#123456789012345678>") {
		t.Fatalf("expected channel mention in response, got %q", channelResp.Data.Content)
	}

	qotdConfig, err := cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	deck, ok := qotdConfig.ActiveDeck()
	if !ok {
		t.Fatalf("expected active deck after channel update: %+v", qotdConfig)
	}
	if deck.ChannelID != "123456789012345678" || deck.Enabled {
		t.Fatalf("unexpected qotd config after channel update: %+v", deck)
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, qotdEnabledSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		boolOpt(qotdEnabledOptionName, true),
	}))
	enableResp := rec.lastResponse(t)
	if err := ephemeralError(enableResp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(enableResp.Data.Content, "QOTD is now enabled") {
		t.Fatalf("unexpected enable response: %q", enableResp.Data.Content)
	}

	qotdConfig, err = cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() after enable failed: %v", err)
	}
	deck, ok = qotdConfig.ActiveDeck()
	if !ok || !deck.Enabled || deck.ChannelID != "123456789012345678" {
		t.Fatalf("unexpected qotd config after enabling: %+v", qotdConfig)
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, qotdEnabledSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		boolOpt(qotdEnabledOptionName, false),
	}))
	disableResp := rec.lastResponse(t)
	if err := ephemeralError(disableResp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(disableResp.Data.Content, "QOTD is now disabled") {
		t.Fatalf("unexpected disable response: %q", disableResp.Data.Content)
	}

	qotdConfig, err = cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() after disable failed: %v", err)
	}
	deck, ok = qotdConfig.ActiveDeck()
	if !ok || deck.Enabled || deck.ChannelID != "123456789012345678" {
		t.Fatalf("unexpected qotd config after disabling: %+v", qotdConfig)
	}
}

func TestQOTDConfigCommandsRejectEnableWithoutChannel(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, qotdEnabledSubCommandName, []*discordgo.ApplicationCommandInteractionDataOption{
		boolOpt(qotdEnabledOptionName, true),
	}))

	resp := rec.lastResponse(t)
	if err := ephemeralError(resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Data.Content, "Set a QOTD channel before enabling publishing") {
		t.Fatalf("unexpected validation response: %q", resp.Data.Content)
	}

	qotdConfig, err := cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	if !qotdConfig.IsZero() {
		t.Fatalf("expected qotd config to remain empty, got %+v", qotdConfig)
	}
}

func channelOpt(name, channelID string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionChannel,
		Value: channelID,
	}
}

func TestQOTDConfigGetReportsCurrentState(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		for idx := range cfg.Guilds {
			if cfg.Guilds[idx].GuildID != guildID {
				continue
			}
			cfg.Guilds[idx].QOTD = files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "channel-555",
				}},
			}
		}
	})

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "get", nil))
	resp := rec.lastResponse(t)
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected config get response to remain non-ephemeral, got flags=%v", resp.Data.Flags)
	}
	if len(resp.Data.Embeds) != 1 {
		t.Fatalf("expected config get response to include one embed, got %+v", resp.Data.Embeds)
	}
	description := resp.Data.Embeds[0].Description
	if !strings.Contains(description, "QOTD Enabled: true") {
		t.Fatalf("expected qotd enabled line in config output, got %q", description)
	}
	if !strings.Contains(description, "QOTD Channel: channel-555") {
		t.Fatalf("expected qotd channel line in config output, got %q", description)
	}
}