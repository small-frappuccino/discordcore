package discordautomod

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func newTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()
	return files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
}

func testSessionWithChannel(guildID, channelID, botID string, perms int64) *discordgo.Session {
	state := discordgo.NewState()
	state.User = &discordgo.User{ID: botID}

	roleID := guildID
	guild := &discordgo.Guild{
		ID: guildID,
		Roles: []*discordgo.Role{
			{ID: roleID, Permissions: perms}}}
	_ = state.GuildAdd(guild)
	_ = state.ChannelAdd(&discordgo.Channel{
		ID:      channelID,
		GuildID: guildID,
		Type:    discordgo.ChannelTypeGuildText})
	_ = state.MemberAdd(&discordgo.Member{
		GuildID: guildID,
		User:    &discordgo.User{ID: botID},
		Roles:   []string{roleID}})

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
