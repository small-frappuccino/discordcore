package stats

import (
	"strings"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestStatsAddPersistsChannelConfig(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, mockSvc, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "add", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"111111111"`)},
		{Name: "type", Type: discord.StringOptionType, Value: []byte(`"humans"`)},
		{Name: "name_template", Type: discord.StringOptionType, Value: []byte(`"Members: {count}"`)},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "111111111") {
		t.Fatalf("expected success mentioning the channel, got %q", resp.Data.Content.Val)
	}

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 stats channel config, got %d", len(cfg.Stats.Channels))
	}
	ch := cfg.Stats.Channels[0]
	if ch.ChannelID != "111111111" || ch.MemberType != "humans" || ch.NameTemplate != "Members: {count}" {
		t.Fatalf("unexpected persisted channel config: %+v", ch)
	}

	if !mockSvc.wasUpdateCalled() {
		t.Fatalf("expected UpdateStatsChannels to be called")
	}
}

func TestStatsAddUpdatesExistingChannelConfig(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "111111111", MemberType: "all", NameTemplate: "Old: {count}"},
			},
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "add", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"111111111"`)},
		{Name: "type", Type: discord.StringOptionType, Value: []byte(`"bots"`)},
		{Name: "name_template", Type: discord.StringOptionType, Value: []byte(`"Bots: {count}"`)},
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
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "add", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"222222222"`)},
		{Name: "role_filter", Type: discord.RoleOptionType, Value: []byte(`"333333333"`)},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 stats channel, got %d", len(cfg.Stats.Channels))
	}
	if cfg.Stats.Channels[0].RoleID != "333333333" {
		t.Fatalf("expected role filter persisted, got %+v", cfg.Stats.Channels[0])
	}
}

func TestStatsRemoveDeletesChannelConfig(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, mockSvc, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "111111111", MemberType: "all"},
				{ChannelID: "222222222", MemberType: "bots"},
			},
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "remove", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"111111111"`)},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "Removed") {
		t.Fatalf("expected removal confirmation, got %q", resp.Data.Content.Val)
	}

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 remaining channel, got %d", len(cfg.Stats.Channels))
	}
	if cfg.Stats.Channels[0].ChannelID != "222222222" {
		t.Fatalf("expected 222222222 to remain, got %+v", cfg.Stats.Channels[0])
	}

	if !mockSvc.wasUpdateCalled() {
		t.Fatalf("expected UpdateStatsChannels to be called")
	}
}

func TestStatsRemoveReportsErrorForUnknownChannel(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "remove", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"999999999"`)},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "not configured") {
		t.Fatalf("expected not-configured error, got %q", resp.Data.Content.Val)
	}
}

func TestStatsListShowsConfiguredChannels(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
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

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "list", nil))

	resp := rec.lastResponse(t)
	if resp.Data == nil || resp.Data.Embeds == nil || len(*resp.Data.Embeds) == 0 {
		t.Fatalf("expected embed response, got %+v", resp.Data)
	}
	embed := (*resp.Data.Embeds)[0]
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
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "list", nil))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "no stats channels") {
		t.Fatalf("expected empty state message, got %q", resp.Data.Content.Val)
	}
}

func TestStatsListShowsRoleFilter(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "voice-vip", MemberType: "all", RoleID: "333333333"},
			},
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "list", nil))

	resp := rec.lastResponse(t)
	if resp.Data == nil || resp.Data.Embeds == nil || len(*resp.Data.Embeds) == 0 {
		t.Fatalf("expected embed response, got %+v", resp.Data)
	}
	if !strings.Contains((*resp.Data.Embeds)[0].Description, "333333333") {
		t.Fatalf("expected embed to mention the role filter, got %q", (*resp.Data.Embeds)[0].Description)
	}
}

func TestStatsSettingsShowsCurrentWhenNoOptionProvided(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Enabled:            true,
			UpdateIntervalMins: 45,
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "settings", nil))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "Enabled") {
		t.Fatalf("expected current enabled status, got %q", resp.Data.Content.Val)
	}
	if !strings.Contains(resp.Data.Content.Val, "45") {
		t.Fatalf("expected current interval value, got %q", resp.Data.Content.Val)
	}
}

func TestStatsSettingsUpdatesInterval(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, mockSvc, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			UpdateIntervalMins: 30,
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "settings", []discord.CommandInteractionOption{
		{Name: "update_interval_mins", Type: discord.IntegerOptionType, Value: []byte(`60`)},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "Stats update interval set") {
		t.Fatalf("expected success confirmation, got %q", resp.Data.Content.Val)
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
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(false),
		},
	})

	subcommands := []struct {
		name    string
		options []discord.CommandInteractionOption
	}{
		{"add", []discord.CommandInteractionOption{
			{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"111111111"`)},
		}},
		{"remove", []discord.CommandInteractionOption{
			{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"111111111"`)},
		}},
		{"list", nil},
		{"settings", nil},
	}

	for _, sc := range subcommands {
		t.Run(sc.name, func(t *testing.T) {
			handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, sc.name, sc.options))
			resp := rec.lastResponse(t)
			requireEphemeralResponse(t, resp)
			if !strings.Contains(resp.Data.Content.Val, "disabled") {
				t.Fatalf("expected disabled error for /%s %s, got %q", "stats", sc.name, resp.Data.Content.Val)
			}
		})
	}
}

func TestStatsSettingsShowsDefaultIntervalWhenZero(t *testing.T) {
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Enabled:            false,
			UpdateIntervalMins: 0,
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "settings", nil))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "Disabled") {
		t.Fatalf("expected disabled status, got %q", resp.Data.Content.Val)
	}
	if !strings.Contains(resp.Data.Content.Val, "30") {
		t.Fatalf("expected default 30 minute interval when zero, got %q", resp.Data.Content.Val)
	}
}
