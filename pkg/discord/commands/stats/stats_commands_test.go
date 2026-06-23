package stats

import (
	"strings"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestStatsAddPersistsChannelConfig(t *testing.T) {
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, mockSvc, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "add", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"111111111"`)},
		{Name: "type", Type: discord.StringOptionType, Value: []byte(`"humans"`)},
	}))

	resp := rec.lastResponse(t)
	requireNonEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "111111111") {
		t.Fatalf("expected success mentioning the channel, got %q", resp.Data.Content.Val)
	}

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 stats channel config, got %d", len(cfg.Stats.Channels))
	}
	ch := cfg.Stats.Channels[0]
	if ch.ChannelID != "111111111" || ch.MemberType != "humans" || ch.NameTemplate != "" {
		t.Fatalf("unexpected persisted channel config: %+v", ch)
	}

	if !mockSvc.wasUpdateCalled() {
		t.Fatalf("expected UpdateStatsChannels to be called")
	}
}

func TestStatsAddUpdatesExistingChannelConfig(t *testing.T) {
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "111111111", MemberType: "all", NameTemplate: "Old: {count}"},
			},
		},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "add", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"111111111"`)},
		{Name: "type", Type: discord.StringOptionType, Value: []byte(`"bots"`)},
	}))

	resp := rec.lastResponse(t)
	requireNonEphemeralResponse(t, resp)

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected existing channel to be updated in-place, got %d channels", len(cfg.Stats.Channels))
	}
	ch := cfg.Stats.Channels[0]
	if ch.MemberType != "bots" || ch.NameTemplate != "" {
		t.Fatalf("expected channel config to be updated, got %+v", ch)
	}
}

func TestStatsAddWithRoleFilter(t *testing.T) {
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "add", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"222222222"`)},
		{Name: "role_filter", Type: discord.RoleOptionType, Value: []byte(`"333333333"`)},
	}))

	resp := rec.lastResponse(t)
	requireNonEphemeralResponse(t, resp)

	cfg := cm.GuildConfig(guildID)
	if len(cfg.Stats.Channels) != 1 {
		t.Fatalf("expected 1 stats channel, got %d", len(cfg.Stats.Channels))
	}
	if cfg.Stats.Channels[0].RoleID != "333333333" {
		t.Fatalf("expected role filter persisted, got %+v", cfg.Stats.Channels[0])
	}
}

func TestStatsRemoveDeletesChannelConfig(t *testing.T) {
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, mockSvc, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
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
	requireNonEphemeralResponse(t, resp)
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
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "remove", []discord.CommandInteractionOption{
		{Name: "channel", Type: discord.ChannelOptionType, Value: []byte(`"999999999"`)},
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content.Val, "configured") {
		t.Fatalf("expected configured error, got %q", resp.Data.Content.Val)
	}
}

func TestStatsListShowsConfiguredChannels(t *testing.T) {
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{ChannelID: "voice-total", MemberType: "all", Label: "Total: "},
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
	if embed.Footer == nil || !strings.Contains(embed.Footer.Text, "5 minutes") {
		t.Fatalf("expected footer to include update interval, got %+v", embed.Footer)
	}
}

func TestStatsListShowsEmptyStateWhenNoChannels(t *testing.T) {
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
	})

	handleRawStatsInteraction(t, router, cm, rec, newStatsSlashInteraction(guildID, ownerID, "list", nil))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if resp.Data.Content.Val == "" || !strings.Contains(resp.Data.Content.Val, "configured") {
		t.Fatalf("expected missing config message")
	}
}

func TestStatsListShowsRoleFilter(t *testing.T) {
	t.Parallel()
	const (
		guildID = "123456789"
		ownerID = "987654321"
	)

	router, cm, _, rec := newStatsCommandTestRouter(t, guildID, ownerID, files.GuildConfig{
		GuildID:  guildID,
		Features: files.FeatureToggles{},
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
