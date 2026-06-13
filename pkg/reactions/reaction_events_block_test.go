
package reactions

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)


func TestReactionEventServiceRemovesBlockedReactionWithoutMetricsStore(t *testing.T) {
	const (
		guildID       = "g-reaction-block"
		channelID     = "c-reaction-block"
		messageID     = "m-reaction-block"
		reactorUserID = "222222222222222222"
		targetUserID  = "111111111111111111"
		emojiID       = "987654321098765432"
		emojiName     = "skrunklyalice"
	)

	var messageLookups int32
	var reactionDeletes int32
	var mu sync.Mutex
	lastDeletePath := ""

	adapter := &mockReactionAdapter{
		getGuildIDForChannel: func(chID string) (string, error) {
			return guildID, nil
		},
		getMessageAuthorID: func(chID, msgID string) (string, bool, error) {
			atomic.AddInt32(&messageLookups, 1)
			if chID == channelID && msgID == messageID {
				return targetUserID, true, nil
			}
			return "", false, nil
		},
		removeReaction: func(chID, msgID, emID, usrID string) error {
			atomic.AddInt32(&reactionDeletes, 1)
			mu.Lock()
			lastDeletePath = fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/%s", chID, msgID, emID, usrID)
			mu.Unlock()
			return nil
		},
	}

	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{})
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds[0].ReactionBlocks = files.ReactionBlockConfig{Rules: []files.ReactionBlockRuleConfig{{
			ReactorUserID: reactorUserID,
			TargetUserID:  targetUserID,
			Emojis: []files.ReactionBlockEmojiConfig{{
				Kind:  files.ReactionBlockEmojiKindCustom,
				Value: emojiID,
				Name:  emojiName}}}}}
		return nil
	}); err != nil {
		t.Fatalf("update config: %v", err)
	}

	service := NewReactionEventService(adapter, cfgMgr, nil, slog.Default())
	service.HandleReactionAdd(context.Background(), &MessageReactionAdd{
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
		UserID:    reactorUserID,
		Emoji: Emoji{
			ID:   emojiID,
			Name: emojiName}})

	if got := atomic.LoadInt32(&messageLookups); got != 1 {
		t.Fatalf("expected one message lookup, got %d", got)
	}
	if got := atomic.LoadInt32(&reactionDeletes); got != 1 {
		t.Fatalf("expected one blocked reaction removal, got %d", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if lastDeletePath != fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/%s", channelID, messageID, emojiID, reactorUserID) {
		t.Fatalf("unexpected delete path: %q", lastDeletePath)
	}
}
