package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestShouldLogModerationEventModes(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	botID := "bot"
	otherID := "user"

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	cfg := cm.Config()
	if cfg == nil {
		t.Fatal("config is nil")
	}

	cfg.RuntimeConfig.ModerationLogMode = "off"
	if ShouldLogModerationEvent(cm, guildID, botID, botID, ModerationSourceCommand) {
		t.Fatal("expected off mode to block logs")
	}

	cfg.RuntimeConfig.ModerationLogMode = "alice_only"
	if !ShouldLogModerationEvent(cm, guildID, botID, botID, ModerationSourceCommand) {
		t.Fatal("expected alice_only to allow bot actor")
	}
	if ShouldLogModerationEvent(cm, guildID, otherID, botID, ModerationSourceCommand) {
		t.Fatal("expected alice_only to block non-bot actor")
	}

	cfg.RuntimeConfig.ModerationLogMode = "all"
	if !ShouldLogModerationEvent(cm, guildID, otherID, botID, ModerationSourceCommand) {
		t.Fatal("expected all to allow any actor")
	}

	cfg.RuntimeConfig.ModerationLogMode = ""
	if ShouldLogModerationEvent(cm, guildID, otherID, botID, ModerationSourceCommand) {
		t.Fatal("expected default alice_only to block non-bot actor")
	}
}

func TestResolveModerationLogChannelShared(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	channelID := "c1"

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			ModerationLog:   channelID,
			UserActivityLog: channelID,
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	if got, ok := ResolveModerationLogChannel(nil, cm, guildID); ok || got != "" {
		t.Fatalf("expected shared moderation channel to be rejected, got %q", got)
	}
}

func TestResolveModerationLogChannelValid(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	channelID := "c1"
	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			ModerationLog: channelID,
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(guildID, channelID, botID, perms)
	got, ok := ResolveModerationLogChannel(session, cm, guildID)
	if !ok {
		t.Fatal("expected moderation log channel to validate")
	}
	if got != channelID {
		t.Fatalf("expected channel %q, got %q", channelID, got)
	}
}

func testSessionWithChannel(guildID, channelID, botID string, perms int64) *discordgo.Session {
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: botID}

	roleID := guildID
	guild := &discordgo.Guild{
		ID: guildID,
		Roles: []*discordgo.Role{
			{ID: roleID, Permissions: perms},
		},
	}
	_ = state.GuildAdd(guild)
	_ = state.ChannelAdd(&discordgo.Channel{
		ID:      channelID,
		GuildID: guildID,
		Type:    discordgo.ChannelTypeGuildText,
	})
	_ = state.MemberAdd(&discordgo.Member{
		GuildID: guildID,
		User:    &discordgo.User{ID: botID},
		Roles:   []string{roleID},
	})

	return &discordgo.Session{State: state}
}
