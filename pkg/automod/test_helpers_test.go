package automod

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func newTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()
	return files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
}

func testSessionWithChannel(t *testing.T, guildID, channelID, botID string, perms int64) *discordgo.Session {
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: botID}

	roleID := guildID
	guild := &discordgo.Guild{
		ID: guildID,
		Roles: []*discordgo.Role{
			{ID: roleID, Permissions: perms}}}
	if err := state.GuildAdd(guild); err != nil {
		t.Fatalf("GuildAdd failed: %v", err)
	}
	if err := state.ChannelAdd(&discordgo.Channel{
		ID:      channelID,
		GuildID: guildID,
		Type:    discordgo.ChannelTypeGuildText}); err != nil {
		t.Fatalf("ChannelAdd failed: %v", err)
	}
	if err := state.MemberAdd(&discordgo.Member{
		GuildID: guildID,
		User:    &discordgo.User{ID: botID},
		Roles:   []string{roleID}}); err != nil {
		t.Fatalf("MemberAdd failed: %v", err)
	}

	return &discordgo.Session{State: state}
}

func mustUpdateConfig(t *testing.T, cm *files.ConfigManager, fn func(*files.BotConfig)) {
	t.Helper()

	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		if fn != nil {
			fn(cfg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}
}
