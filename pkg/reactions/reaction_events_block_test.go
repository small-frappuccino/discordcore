//go:build ignore

package reactions

import (
	"log/slog"

	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestReactionEventServiceRemovesBlockedReactionWithoutMetricsStore(t *testing.T) {
	const (
		guildID       = "g-reaction-block"
		channelID     = "c-reaction-block"
		messageID     = "m-reaction-block"
		reactorUserID = "222222222222222222"
		targetUserID  = "111111111111111111"
		emojiID       = "987654321098765432"
		emojiName     = "skrunklytest"
	)

	var messageLookups int32
	var reactionDeletes int32
	var mu sync.Mutex
	lastDeletePath := ""

	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID):
			atomic.AddInt32(&messageLookups, 1)
			json.NewEncoder(w).Encode(map[string]any{
				"id":         messageID,
				"channel_id": channelID,
				"author": map[string]any{
					"id":       targetUserID,
					"username": "target-user"}})
		case r.Method == http.MethodDelete && r.URL.Path == fmt.Sprintf("/channels/%s/messages/%s/reactions/%s/%s", channelID, messageID, emojiID, reactorUserID):
			atomic.AddInt32(&reactionDeletes, 1)
			mu.Lock()
			lastDeletePath = r.URL.Path
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Write([]byte(`{}`))
		}
	})

	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{})
	if _, err := cfgMgr.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
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

	service := NewReactionEventService(session, cfgMgr, nil, slog.Default())
	service.handleReactionAdd(context.Background(), session, &discordgo.MessageReactionAdd{
		MessageReaction: &discordgo.MessageReaction{
			GuildID:   guildID,
			ChannelID: channelID,
			MessageID: messageID,
			UserID:    reactorUserID,
			Emoji: discordgo.Emoji{
				ID:   emojiID,
				Name: emojiName}}})

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
