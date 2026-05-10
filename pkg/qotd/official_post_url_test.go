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

// TestOfficialPostJumpURLFallbacksAndTrimming pins each fallback branch with
// its own row so a regression in one branch (e.g. preferring the question-list
// pointer over the live thread, dropping whitespace trimming, returning the
// wrong empty value) fails its own subtest instead of being absorbed into the
// happy path. The starter-message branch is exercised separately above; this
// table covers the remaining four branches.
func TestOfficialPostJumpURLFallbacksAndTrimming(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		post storage.QOTDOfficialPostRecord
		want string
	}{
		{
			name: "thread fallback when starter message missing",
			post: storage.QOTDOfficialPostRecord{
				GuildID:         "guild-1",
				ChannelID:       "channel-1",
				DiscordThreadID: "thread-1",
			},
			want: "https://discord.com/channels/guild-1/thread-1",
		},
		{
			name: "thread fallback when channel missing keeps starter message ignored",
			post: storage.QOTDOfficialPostRecord{
				GuildID:                 "guild-1",
				DiscordStarterMessageID: "message-1",
				DiscordThreadID:         "thread-1",
			},
			want: "https://discord.com/channels/guild-1/thread-1",
		},
		{
			name: "question-list fallback when no live discord pointers remain",
			post: storage.QOTDOfficialPostRecord{
				GuildID:                    "guild-1",
				QuestionListThreadID:       "list-thread",
				QuestionListEntryMessageID: "list-message",
			},
			want: "https://discord.com/channels/guild-1/list-thread/list-message",
		},
		{
			name: "whitespace-only pointers degrade to empty url",
			post: storage.QOTDOfficialPostRecord{
				GuildID:                    "guild-1",
				ChannelID:                  "   ",
				DiscordStarterMessageID:    "   ",
				DiscordThreadID:            " \t ",
				QuestionListThreadID:       " ",
				QuestionListEntryMessageID: " ",
			},
			want: "",
		},
		{
			name: "no pointers at all returns empty url",
			post: storage.QOTDOfficialPostRecord{GuildID: "guild-1"},
			want: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := OfficialPostJumpURL(tc.post); got != tc.want {
				t.Fatalf("OfficialPostJumpURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
