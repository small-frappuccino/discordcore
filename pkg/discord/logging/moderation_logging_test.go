package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestResolveModerationLogChannelShared(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	channelID := "c1"

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			ModerationCase: channelID,
			AvatarLogging:  channelID,
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
			ModerationCase: channelID,
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
