package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func newLoggingLifecycleSession(t *testing.T) *discordgo.Session {
	t.Helper()

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	return session
}

func TestMemberEventService_StartStopDoesNotLeakHandlers(t *testing.T) {
	const (
		guildID      = "g-lifecycle-member"
		channelID    = "c-lifecycle-member"
		firstUserID  = "111111111111111111"
		secondUserID = "222222222222222222"
	)

	store, dbPath := newLoggingStore(t, "lifecycle-member.db")
	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		MemberJoin:  channelID,
		MemberLeave: channelID,
	})

	var notificationPosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&notificationPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "lifecycle-msg"})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMembers
	service := NewMemberEventService(session, cfgMgr, NewNotificationSender(session), store)

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start member event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 3 {
		t.Fatalf("expected 3 registered handlers after start, got %d", got)
	}

	dispatchDiscordEvent(session, "GUILD_MEMBER_ADD", &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       firstUserID,
				Username: "first-user",
			},
			JoinedAt: time.Now().UTC().Add(-30 * time.Minute),
		},
	})
	if got := atomic.LoadInt32(&notificationPosts); got != 1 {
		t.Fatalf("expected one notification after first dispatch, got %d", got)
	}

	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("stop member event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 0 {
		t.Fatalf("expected 0 registered handlers after stop, got %d", got)
	}

	dispatchDiscordEvent(session, "GUILD_MEMBER_ADD", &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       "333333333333333333",
				Username: "stopped-user",
			},
			JoinedAt: time.Now().UTC().Add(-10 * time.Minute),
		},
	})
	if got := atomic.LoadInt32(&notificationPosts); got != 1 {
		t.Fatalf("expected no event processing while service is stopped, got %d notification sends", got)
	}

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("restart member event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 3 {
		t.Fatalf("expected 3 registered handlers after restart, got %d", got)
	}

	dispatchDiscordEvent(session, "GUILD_MEMBER_ADD", &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       secondUserID,
				Username: "second-user",
			},
			JoinedAt: time.Now().UTC().Add(-20 * time.Minute),
		},
	})

	if got := atomic.LoadInt32(&notificationPosts); got != 2 {
		t.Fatalf("expected exactly one notification per dispatched event, got %d", got)
	}

	if got := dailyMemberMetricCount(t, dbPath, "daily_member_joins", guildID, firstUserID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected first user to be counted once, got %d", got)
	}
	if got := dailyMemberMetricCount(t, dbPath, "daily_member_joins", guildID, secondUserID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected second user to be counted once, got %d", got)
	}

	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("final stop member event service: %v", err)
	}
}

func TestMessageEventService_StartStopDoesNotLeakHandlers(t *testing.T) {
	const (
		guildID   = "g-lifecycle-message"
		channelID = "c-lifecycle-message"
		userID    = "lifecycle-user"
	)

	store, dbPath := newLoggingStore(t, "lifecycle-message.db")
	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{})

	session := newLoggingLifecycleSession(t)
	session.Identify.Intents = discordgo.IntentsGuildMessages
	service := NewMessageEventService(session, cfgMgr, nil, store)

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start message event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 4 {
		t.Fatalf("expected 4 registered handlers after start, got %d", got)
	}

	dispatchDiscordEvent(session, "MESSAGE_CREATE", &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "m-first",
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "first",
			Author: &discordgo.User{
				ID:       userID,
				Username: "lifecycle-user",
			},
		},
	})
	if got := dailyMessageMetricCount(t, dbPath, guildID, channelID, userID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected one processed message after first dispatch, got %d", got)
	}

	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("stop message event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 0 {
		t.Fatalf("expected 0 registered handlers after stop, got %d", got)
	}

	dispatchDiscordEvent(session, "MESSAGE_CREATE", &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "m-stopped",
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "should-not-be-processed",
			Author: &discordgo.User{
				ID:       userID,
				Username: "lifecycle-user",
			},
		},
	})
	if got := dailyMessageMetricCount(t, dbPath, guildID, channelID, userID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected no extra processing while stopped, daily metric=%d", got)
	}

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("restart message event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 4 {
		t.Fatalf("expected 4 registered handlers after restart, got %d", got)
	}

	dispatchDiscordEvent(session, "MESSAGE_CREATE", &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "m-second",
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "second",
			Author: &discordgo.User{
				ID:       userID,
				Username: "lifecycle-user",
			},
		},
	})

	if got := dailyMessageMetricCount(t, dbPath, guildID, channelID, userID, time.Now().UTC()); got != 2 {
		t.Fatalf("expected one processed message per dispatch after restart, got daily metric=%d", got)
	}

	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("final stop message event service: %v", err)
	}
}

func TestReactionEventService_StartStopDoesNotLeakHandlers(t *testing.T) {
	const (
		guildID   = "g-lifecycle-reaction"
		channelID = "c-lifecycle-reaction"
		userID    = "lifecycle-user"
	)

	store, dbPath := newLoggingStore(t, "lifecycle-reaction.db")
	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{})

	session := newLoggingLifecycleSession(t)
	service := NewReactionEventService(session, cfgMgr, store)

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("start reaction event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 1 {
		t.Fatalf("expected 1 registered handler after start, got %d", got)
	}

	dispatchDiscordEvent(session, "MESSAGE_REACTION_ADD", &discordgo.MessageReactionAdd{
		MessageReaction: &discordgo.MessageReaction{
			GuildID:   guildID,
			ChannelID: channelID,
			UserID:    userID,
			Emoji: discordgo.Emoji{
				Name: "thumbsup",
			},
		},
	})
	if got := dailyReactionMetricCount(t, dbPath, guildID, channelID, userID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected one reaction metric after first dispatch, got %d", got)
	}

	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("stop reaction event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 0 {
		t.Fatalf("expected 0 registered handlers after stop, got %d", got)
	}

	dispatchDiscordEvent(session, "MESSAGE_REACTION_ADD", &discordgo.MessageReactionAdd{
		MessageReaction: &discordgo.MessageReaction{
			GuildID:   guildID,
			ChannelID: channelID,
			UserID:    userID,
			Emoji: discordgo.Emoji{
				Name: "thumbsup",
			},
		},
	})
	if got := dailyReactionMetricCount(t, dbPath, guildID, channelID, userID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected no extra processing while stopped, reaction metric=%d", got)
	}

	if err := service.Start(context.Background()); err != nil {
		t.Fatalf("restart reaction event service: %v", err)
	}
	if got := len(service.handlerCancels); got != 1 {
		t.Fatalf("expected 1 registered handler after restart, got %d", got)
	}

	dispatchDiscordEvent(session, "MESSAGE_REACTION_ADD", &discordgo.MessageReactionAdd{
		MessageReaction: &discordgo.MessageReaction{
			GuildID:   guildID,
			ChannelID: channelID,
			UserID:    userID,
			Emoji: discordgo.Emoji{
				Name: "thumbsup",
			},
		},
	})
	if got := dailyReactionMetricCount(t, dbPath, guildID, channelID, userID, time.Now().UTC()); got != 2 {
		t.Fatalf("expected one processed reaction per dispatch after restart, got metric=%d", got)
	}

	if err := service.Stop(context.Background()); err != nil {
		t.Fatalf("final stop reaction event service: %v", err)
	}
}
