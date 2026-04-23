package qotd

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func TestOfficialPostJumpURLPrefersStarterMessage(t *testing.T) {
	t.Parallel()

	got := OfficialPostJumpURL(storage.QOTDOfficialPostRecord{
		GuildID:                 "guild-1",
		ChannelID:               "channel-1",
		DiscordStarterMessageID: "message-1",
		DiscordThreadID:         "thread-1",
	})
	if got != "https://discord.com/channels/guild-1/channel-1/message-1" {
		t.Fatalf("unexpected official post jump url: %q", got)
	}
}
