package stats

import (
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestStatsAddPersistsChannelConfig(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, cm, mockSvc := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "add", []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: "voice-channel-1"},
		{Name: "type", Type: discordgo.ApplicationCommandOptionString, Value: "humans"},
		{Name: "name_template", Type: discordgo.ApplicationCommandOptionString, Value: "Members: {count}"},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "voice-channel-1") {
		t.Fatalf("expected success mentioning the channel, got %q", resp.Data.Content)
	}

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 stats channel config, got %d", len(cfg.Stats.Channels))
	}
	ch := cfg.Stats.Channels[0]
	if ch.ChannelID != "voice-channel-1" || ch.MemberType != "humans" || ch.NameTemplate != "Members: {count}" {
		t.Fatalf("unexpected persisted channel config: %+v", ch)
	}

	if !mockSvc.wasUpdateCalled() {
		t.Fatalf("expected UpdateStatsChannels to be called")
	}
}

func TestStatsAddUpdatesExistingChannelConfig(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, cm, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "voice-channel-1", MemberType: "all", NameTemplate: "Old: {count}"},
			},
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "add", []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: "voice-channel-1"},
		{Name: "type", Type: discordgo.ApplicationCommandOptionString, Value: "bots"},
		{Name: "name_template", Type: discordgo.ApplicationCommandOptionString, Value: "Bots: {count}"},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected existing channel to be updated in-place, got %d channels", len(cfg.Stats.Channels))
	}
	ch := cfg.Stats.Channels[0]
	if ch.MemberType != "bots" || ch.NameTemplate != "Bots: {count}" {
		t.Fatalf("expected channel config to be updated, got %+v", ch)
	}
}

func TestStatsAddWithRoleFilter(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, cm, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "add", []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: "voice-channel-2"},
		{Name: "role_filter", Type: discordgo.ApplicationCommandOptionRole, Value: "role-vip"},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 stats channel, got %d", len(cfg.Stats.Channels))
	}
	if cfg.Stats.Channels[0].RoleID != "role-vip" {
		t.Fatalf("expected role filter persisted, got %+v", cfg.Stats.Channels[0])
	}
}

func TestStatsRemoveDeletesChannelConfig(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, cm, mockSvc := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "voice-channel-1", MemberType: "all"},
				{ChannelID: "voice-channel-2", MemberType: "bots"},
			},
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "remove", []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: "voice-channel-1"},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Removed") {
		t.Fatalf("expected removal confirmation, got %q", resp.Data.Content)
	}

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 remaining channel, got %d", len(cfg.Stats.Channels))
	}
	if cfg.Stats.Channels[0].ChannelID != "voice-channel-2" {
		t.Fatalf("expected voice-channel-2 to remain, got %+v", cfg.Stats.Channels[0])
	}

	if !mockSvc.wasUpdateCalled() {
		t.Fatalf("expected UpdateStatsChannels to be called")
	}
}

func TestStatsRemoveReportsErrorForUnknownChannel(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, _, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "remove", []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: "nonexistent-channel"},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "not configured") {
		t.Fatalf("expected not-configured error, got %q", resp.Data.Content)
	}
}

func TestStatsListShowsConfiguredChannels(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, _, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			UpdateIntervalMins: 60,
			Channels: []files.StatsChannelConfig{
				{ChannelID: "voice-total", MemberType: "all", NameTemplate: "Total: {count}"},
				{ChannelID: "voice-bots", MemberType: "bots"},
			},
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "list", nil))

	resp := rec.lastResponse(t)
	if len(resp.Data.Embeds) == 0 {
		t.Fatalf("expected embed response, got %+v", resp.Data)
	}
	embed := resp.Data.Embeds[0]
	if !strings.Contains(embed.Description, "voice-total") {
		t.Fatalf("expected embed to mention voice-total, got %q", embed.Description)
	}
	if !strings.Contains(embed.Description, "voice-bots") {
		t.Fatalf("expected embed to mention voice-bots, got %q", embed.Description)
	}
	if !strings.Contains(embed.Description, "Bots Only") {
		t.Fatalf("expected embed to show filter label for bots, got %q", embed.Description)
	}
	if embed.Footer == nil || !strings.Contains(embed.Footer.Text, "60") {
		t.Fatalf("expected footer to include update interval, got %+v", embed.Footer)
	}
}

func TestStatsListShowsEmptyStateWhenNoChannels(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, _, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "list", nil))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "no stats channels") {
		t.Fatalf("expected empty state message, got %q", resp.Data.Content)
	}
}

func TestStatsListShowsRoleFilter(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, _, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "voice-vip", MemberType: "all", RoleID: "role-vip"},
			},
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "list", nil))

	resp := rec.lastResponse(t)
	if len(resp.Data.Embeds) == 0 {
		t.Fatalf("expected embed response, got %+v", resp.Data)
	}
	if !strings.Contains(resp.Data.Embeds[0].Description, "role-vip") {
		t.Fatalf("expected embed to mention the role filter, got %q", resp.Data.Embeds[0].Description)
	}
}

func TestStatsSettingsShowsCurrentWhenNoOptionProvided(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, _, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Enabled:            true,
			UpdateIntervalMins: 45,
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "settings", nil))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Enabled") {
		t.Fatalf("expected current enabled status, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "45") {
		t.Fatalf("expected current interval value, got %q", resp.Data.Content)
	}
}

func TestStatsSettingsUpdatesInterval(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, cm, mockSvc := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			UpdateIntervalMins: 30,
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "settings", []*discordgo.ApplicationCommandInteractionDataOption{
		{Name: "update_interval_mins", Type: discordgo.ApplicationCommandOptionInteger, Value: float64(60)},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "updated") {
		t.Fatalf("expected success confirmation, got %q", resp.Data.Content)
	}

	cfg := cm.GuildConfig(guildID)
	if cfg.Stats.UpdateIntervalMins != 60 {
		t.Fatalf("expected interval persisted as 60, got %d", cfg.Stats.UpdateIntervalMins)
	}

	if !mockSvc.wasUpdateCalled() {
		t.Fatalf("expected UpdateStatsChannels to be called")
	}
}

func TestStatsCommandsRejectWhenFeatureDisabled(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, _, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(false),
		},
	})

	subcommands := []struct {
		name    string
		options []*discordgo.ApplicationCommandInteractionDataOption
	}{
		{"add", []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: "ch-1"},
		}},
		{"remove", []*discordgo.ApplicationCommandInteractionDataOption{
			{Name: "channel", Type: discordgo.ApplicationCommandOptionChannel, Value: "ch-1"},
		}},
		{"list", nil},
		{"settings", nil},
	}

	for _, sc := range subcommands {
		t.Run(sc.name, func(t *testing.T) {
			router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, sc.name, sc.options))
			resp := rec.lastResponse(t)
			requireEphemeralResponse(t, resp)
			if !strings.Contains(resp.Data.Content, "disabled") {
				t.Fatalf("expected disabled error for /%s %s, got %q", "stats", sc.name, resp.Data.Content)
			}
		})
	}
}

func TestStatsSettingsShowsDefaultIntervalWhenZero(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newStatsCommandTestSession(t)
	router, _, _ := newStatsCommandTestRouter(t, session, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Enabled:            false,
			UpdateIntervalMins: 0,
		},
	})

	router.HandleInteraction(session, newStatsSlashInteraction(guildID, ownerID, "settings", nil))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Disabled") {
		t.Fatalf("expected disabled status, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "30") {
		t.Fatalf("expected default 30 minute interval when zero, got %q", resp.Data.Content)
	}
}
