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

func TestMemberEventService_HandleGuildMemberAddRemovePersistsData(t *testing.T) {
	const (
		guildID      = "g-member"
		logChannelID = "c-member"
		userID       = "123456789012345678"
		botID        = "999999999999999999"
	)

	store, dbPath := newLoggingStore(t, "member-events.db")
	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		MemberJoin:  logChannelID,
		MemberLeave: logChannelID,
	})

	var messagePosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", logChannelID):
			atomic.AddInt32(&messagePosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "msg"})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/members/%s", guildID, botID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]any{
					"id":       botID,
					"username": "alice-bot",
					"bot":      true,
				},
				"joined_at": time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339),
				"roles":     []string{},
			})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMembers
	session.State.User = &discordgo.User{ID: botID, Username: "alice-bot", Bot: true}

	service := NewMemberEventService(session, cfgMgr, NewNotificationSender(session), store)

	joinedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	service.handleGuildMemberAdd(context.Background(), session, &discordgo.GuildMemberAdd{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userID,
				Username: "member-one",
			},
			JoinedAt: joinedAt,
		},
	})

	gotJoin, ok, err := store.GetMemberJoin(guildID, userID)
	if err != nil {
		t.Fatalf("get member join: %v", err)
	}
	if !ok {
		t.Fatalf("expected persisted join time for %s", userID)
	}
	if gotJoin.Unix() != joinedAt.Unix() {
		t.Fatalf("unexpected join timestamp: got=%s want=%s", gotJoin.UTC(), joinedAt.UTC())
	}
	if got := dailyMemberMetricCount(t, dbPath, "daily_member_joins", guildID, userID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected one daily join metric, got %d", got)
	}

	service.handleGuildMemberRemove(context.Background(), session, &discordgo.GuildMemberRemove{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userID,
				Username: "member-one",
			},
		},
	})

	if got := dailyMemberMetricCount(t, dbPath, "daily_member_leaves", guildID, userID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected one daily leave metric, got %d", got)
	}
	if got := atomic.LoadInt32(&messagePosts); got != 2 {
		t.Fatalf("expected two member notifications (join+leave), got %d", got)
	}
}

func TestMessageEventService_PersistsCreateUpdateDeleteFlows(t *testing.T) {
	const (
		guildID      = "g-message"
		channelID    = "c-source"
		logChannelID = "c-log"
		messageID    = "m-1"
		userID       = "u-1"
	)

	store, dbPath := newLoggingStore(t, "message-events.db")
	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		MessageEdit:   logChannelID,
		MessageDelete: logChannelID,
	})

	var notificationPosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", logChannelID):
			atomic.AddInt32(&notificationPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "log-msg"})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"audit_log_entries": []any{},
			})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMessages

	service := NewMessageEventService(session, cfgMgr, NewNotificationSender(session), store)
	service.cacheEnabled = true
	service.versioningEnabled = true
	service.cacheTTL = 24 * time.Hour

	service.handleMessageCreate(context.Background(), session, &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "before",
			Author: &discordgo.User{
				ID:       userID,
				Username: "member-one",
			},
		},
	})

	cachedBefore, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get created message: %v", err)
	}
	if cachedBefore == nil || cachedBefore.Content != "before" {
		t.Fatalf("expected cached message with original content, got %+v", cachedBefore)
	}

	if err := service.processMessageUpdate(session, &discordgo.MessageUpdate{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
			Content:   "after",
			Author: &discordgo.User{
				ID:       userID,
				Username: "member-one",
			},
		},
	}, false); err != nil {
		t.Fatalf("process update: %v", err)
	}

	cachedAfter, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get updated message: %v", err)
	}
	if cachedAfter == nil || cachedAfter.Content != "after" {
		t.Fatalf("expected updated cached content, got %+v", cachedAfter)
	}

	if err := service.processMessageDelete(session, &discordgo.MessageDelete{
		Message: &discordgo.Message{
			ID:        messageID,
			GuildID:   guildID,
			ChannelID: channelID,
		},
	}, false); err != nil {
		t.Fatalf("process delete: %v", err)
	}

	cachedAfterDelete, err := store.GetMessage(guildID, messageID)
	if err != nil {
		t.Fatalf("get message after delete flow: %v", err)
	}
	if cachedAfterDelete == nil {
		t.Fatalf("expected cache record to remain when delete_on_log is disabled")
	}

	if got := dailyMessageMetricCount(t, dbPath, guildID, channelID, userID, time.Now().UTC()); got != 1 {
		t.Fatalf("expected one daily message metric, got %d", got)
	}
	if got := messageHistoryCount(t, dbPath, guildID, messageID, "create"); got != 1 {
		t.Fatalf("expected one create history row, got %d", got)
	}
	if got := messageHistoryCount(t, dbPath, guildID, messageID, "edit"); got != 1 {
		t.Fatalf("expected one edit history row, got %d", got)
	}
	if got := messageHistoryCount(t, dbPath, guildID, messageID, "delete"); got != 1 {
		t.Fatalf("expected one delete history row, got %d", got)
	}
	if got := atomic.LoadInt32(&notificationPosts); got != 2 {
		t.Fatalf("expected two notification posts (edit+delete), got %d", got)
	}
}

func TestMonitoringService_InitializeGuildCachePersistsOwnerBotAndRoles(t *testing.T) {
	const (
		guildID = "g-cache"
		ownerID = "owner-1"
		botID   = "bot-1"
		userID  = "user-1"
	)

	store, _ := newLoggingStore(t, "monitoring-cache.db")
	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{})

	botJoinedAt := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	memberJoinedAt := time.Now().UTC().Add(-12 * time.Hour).Truncate(time.Second)

	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/members", guildID):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"user": map[string]any{
						"id":       userID,
						"username": "member-one",
						"avatar":   "",
					},
					"joined_at": memberJoinedAt.Format(time.RFC3339),
					"roles":     []string{"role-a", "role-b"},
				},
			})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})

	session.State.User = &discordgo.User{ID: botID, Username: "alice-bot", Bot: true}
	if err := session.State.GuildAdd(&discordgo.Guild{
		ID:      guildID,
		Name:    "Guild Cache",
		OwnerID: ownerID,
	}); err != nil {
		t.Fatalf("add guild to state: %v", err)
	}
	if err := session.State.MemberAdd(&discordgo.Member{
		GuildID: guildID,
		User: &discordgo.User{
			ID:       botID,
			Username: "alice-bot",
			Bot:      true,
		},
		JoinedAt: botJoinedAt,
	}); err != nil {
		t.Fatalf("add bot member to state: %v", err)
	}

	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		store:         store,
		recentChanges: make(map[string]time.Time),
		rolesCache:    make(map[string]cachedRoles),
		rolesTTL:      time.Minute,
	}

	ms.initializeGuildCache(guildID)

	gotOwnerID, ok, err := store.GetGuildOwnerID(guildID)
	if err != nil {
		t.Fatalf("get guild owner id: %v", err)
	}
	if !ok || gotOwnerID != ownerID {
		t.Fatalf("unexpected owner id: got=%q ok=%v want=%q", gotOwnerID, ok, ownerID)
	}

	gotBotSince, ok, err := store.GetBotSince(guildID)
	if err != nil {
		t.Fatalf("get bot since: %v", err)
	}
	if !ok {
		t.Fatalf("expected bot since to be persisted")
	}
	if gotBotSince.Unix() != botJoinedAt.Unix() {
		t.Fatalf("unexpected bot since: got=%s want=%s", gotBotSince.UTC(), botJoinedAt.UTC())
	}

	avatarHash, _, ok, err := store.GetAvatar(guildID, userID)
	if err != nil {
		t.Fatalf("get avatar snapshot: %v", err)
	}
	if !ok || avatarHash != "default" {
		t.Fatalf("unexpected avatar snapshot: hash=%q ok=%v", avatarHash, ok)
	}

	roles, err := store.GetMemberRoles(guildID, userID)
	if err != nil {
		t.Fatalf("get roles snapshot: %v", err)
	}
	if !sameStringSet(roles, []string{"role-a", "role-b"}) {
		t.Fatalf("unexpected persisted roles: got=%v", roles)
	}

	gotJoin, ok, err := store.GetMemberJoin(guildID, userID)
	if err != nil {
		t.Fatalf("get member join snapshot: %v", err)
	}
	if !ok || gotJoin.Unix() != memberJoinedAt.Unix() {
		t.Fatalf("unexpected member join snapshot: got=%s ok=%v want=%s", gotJoin.UTC(), ok, memberJoinedAt.UTC())
	}

	if cachedRoles, ok := ms.cacheRolesGet(guildID, userID); !ok || !sameStringSet(cachedRoles, []string{"role-a", "role-b"}) {
		t.Fatalf("expected in-memory role snapshot to be populated, got=%v ok=%v", cachedRoles, ok)
	}
}

func TestMonitoringService_HandleMemberUpdateUpdatesSnapshotWhenAuditDeltaFiltersOut(t *testing.T) {
	const (
		guildID   = "g-role-update"
		userID    = "u-role-update"
		channelID = "c-role-log"
	)

	store, _ := newLoggingStore(t, "monitoring-role-update.db")
	if err := store.UpsertMemberRoles(guildID, userID, []string{"role-old"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed old role snapshot: %v", err)
	}

	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		RoleUpdate: channelID,
	})

	var embedPosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"audit_log_entries": []map[string]any{
					{
						"id":          "not-a-snowflake",
						"user_id":     "moderator-1",
						"target_id":   userID,
						"action_type": int(discordgo.AuditLogActionMemberRoleUpdate),
						"changes": []map[string]any{
							{
								"key": discordgo.AuditLogChangeKeyRoleAdd,
								"new_value": []map[string]any{
									{"id": "role-from-audit", "name": "Audit Role"},
								},
							},
						},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&embedPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "unexpected-send"})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMembers

	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		store:         store,
		recentChanges: map[string]time.Time{
			guildID + ":" + userID + ":default": time.Now().UTC(),
		},
		rolesCache: make(map[string]cachedRoles),
		rolesTTL:   time.Minute,
	}

	ms.handleMemberUpdate(session, &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userID,
				Username: "member-one",
				Avatar:   "",
			},
			Roles: []string{"role-new"},
		},
	})

	roles, err := store.GetMemberRoles(guildID, userID)
	if err != nil {
		t.Fatalf("get updated role snapshot: %v", err)
	}
	if !sameStringSet(roles, []string{"role-new"}) {
		t.Fatalf("expected role snapshot to be updated to current state, got=%v", roles)
	}

	if cachedRoles, ok := ms.cacheRolesGet(guildID, userID); !ok || !sameStringSet(cachedRoles, []string{"role-new"}) {
		t.Fatalf("expected updated in-memory role cache, got=%v ok=%v", cachedRoles, ok)
	}

	if got := atomic.LoadInt32(&embedPosts); got != 0 {
		t.Fatalf("expected no role update embed when verified delta is empty, got %d sends", got)
	}
}
