package reactions

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
	"testing"
)

func newLoggingConfigManager(t *testing.T, guildID string, channels files.ChannelsConfig) *files.ConfigManager {
	mgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if err := mgr.AddGuildConfig(files.GuildConfig{
		GuildID:  guildID,
		Channels: channels}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}
	return mgr
}

type mockReactionAdapter struct {
	getGuildIDForChannel func(chID string) (string, error)
	getMessageAuthorID   func(chID, msgID string) (string, bool, error)
	removeReaction       func(chID, msgID, emID, usrID string) error
}

func (m *mockReactionAdapter) GetGuildIDForChannel(chID string) (string, error) {
	if m.getGuildIDForChannel != nil {
		return m.getGuildIDForChannel(chID)
	}
	return "", nil
}

func (m *mockReactionAdapter) GetMessageAuthorID(chID, msgID string) (string, bool, error) {
	if m.getMessageAuthorID != nil {
		return m.getMessageAuthorID(chID, msgID)
	}
	return "", false, nil
}

func (m *mockReactionAdapter) RemoveReaction(chID, msgID, emID, usrID string) error {
	if m.removeReaction != nil {
		return m.removeReaction(chID, msgID, emID, usrID)
	}
	return nil
}
