package logging

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestMonitoringService_HandleMemberUpdateSkipsAuditWhenLocalDiffEmpty(t *testing.T) {
	const (
		guildID   = "g-role-same"
		userID    = "u-role-same"
		channelID = "c-role-log"
	)

	store, _ := newLoggingStore(t, "monitoring-role-same.db")
	if err := store.UpsertMemberRoles(guildID, userID, []string{"role-same"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed role snapshot: %v", err)
	}

	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		RoleUpdate: channelID,
	})

	var auditGets int32
	var embedPosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			atomic.AddInt32(&auditGets, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"audit_log_entries": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&embedPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "unexpected"})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMembers

	ms := &MonitoringService{
		session:                 session,
		configManager:           cfgMgr,
		store:                   store,
		recentChanges:           map[string]time.Time{guildID + ":" + userID + ":default": time.Now().UTC()},
		rolesCache:              make(map[string]cachedRoles),
		rolesTTL:                time.Minute,
		roleUpdateAuditCache:    make(map[string]cachedRoleUpdateAudit),
		roleUpdateAuditDebounce: make(map[string]time.Time),
	}

	ms.handleMemberUpdate(session, &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userID,
				Username: "member-one",
				Avatar:   "",
			},
			Roles: []string{"role-same"},
		},
	})

	if got := atomic.LoadInt32(&auditGets); got != 0 {
		t.Fatalf("expected no audit log fetch when local diff is empty, got %d", got)
	}
	if got := atomic.LoadInt32(&embedPosts); got != 0 {
		t.Fatalf("expected no role update notification when local diff is empty, got %d", got)
	}
}

func TestMonitoringService_HandleMemberUpdateFallbackHandlesEmptyRoleSet(t *testing.T) {
	const (
		guildID   = "g-role-empty"
		userID    = "u-role-empty"
		channelID = "c-role-log"
	)

	store, _ := newLoggingStore(t, "monitoring-role-empty.db")
	if err := store.UpsertMemberRoles(guildID, userID, []string{"role-old"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed role snapshot: %v", err)
	}

	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		RoleUpdate: channelID,
	})

	var auditGets int32
	var embedPosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			atomic.AddInt32(&auditGets, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"audit_log_entries": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&embedPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "fallback"})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMembers

	ms := &MonitoringService{
		session:                 session,
		configManager:           cfgMgr,
		store:                   store,
		recentChanges:           map[string]time.Time{guildID + ":" + userID + ":default": time.Now().UTC()},
		rolesCache:              make(map[string]cachedRoles),
		rolesTTL:                time.Minute,
		roleUpdateAuditCache:    make(map[string]cachedRoleUpdateAudit),
		roleUpdateAuditDebounce: make(map[string]time.Time),
	}

	ms.handleMemberUpdate(session, &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userID,
				Username: "member-one",
				Avatar:   "",
			},
			Roles: []string{},
		},
	})

	roles, err := store.GetMemberRoles(guildID, userID)
	if err != nil {
		t.Fatalf("get role snapshot after empty update: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("expected role snapshot to be cleared after empty role update, got=%v", roles)
	}
	if _, ok := ms.cacheRolesGet(guildID, userID); ok {
		t.Fatalf("expected in-memory role cache to be cleared after empty role update")
	}
	if got := atomic.LoadInt32(&auditGets); got != 1 {
		t.Fatalf("expected one audit log fetch for non-empty local diff, got %d", got)
	}
	if got := atomic.LoadInt32(&embedPosts); got != 1 {
		t.Fatalf("expected one fallback role update notification, got %d", got)
	}
}

func TestMonitoringService_HandleMemberUpdateReusesGuildAuditCache(t *testing.T) {
	const (
		guildID   = "g-role-cache"
		channelID = "c-role-log"
		userOne   = "u-role-cache-1"
		userTwo   = "u-role-cache-2"
	)

	store, _ := newLoggingStore(t, "monitoring-role-cache.db")
	if err := store.UpsertMemberRoles(guildID, userOne, []string{"role-old-1"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed role snapshot user one: %v", err)
	}
	if err := store.UpsertMemberRoles(guildID, userTwo, []string{"role-old-2"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed role snapshot user two: %v", err)
	}

	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		RoleUpdate: channelID,
	})

	var auditGets int32
	var embedPosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			atomic.AddInt32(&auditGets, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"audit_log_entries": []map[string]any{
					{
						"id":          "role-cache-entry-1",
						"user_id":     "moderator-1",
						"target_id":   userOne,
						"action_type": int(discordgo.AuditLogActionMemberRoleUpdate),
						"changes": []map[string]any{
							{
								"key": discordgo.AuditLogChangeKeyRoleAdd,
								"new_value": []map[string]any{
									{"id": "role-new-1", "name": "Role New 1"},
								},
							},
						},
					},
					{
						"id":          "role-cache-entry-2",
						"user_id":     "moderator-2",
						"target_id":   userTwo,
						"action_type": int(discordgo.AuditLogActionMemberRoleUpdate),
						"changes": []map[string]any{
							{
								"key": discordgo.AuditLogChangeKeyRoleAdd,
								"new_value": []map[string]any{
									{"id": "role-new-2", "name": "Role New 2"},
								},
							},
						},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&embedPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": fmt.Sprintf("msg-%d", atomic.LoadInt32(&embedPosts))})
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
			guildID + ":" + userOne + ":default": time.Now().UTC(),
			guildID + ":" + userTwo + ":default": time.Now().UTC(),
		},
		rolesCache:              make(map[string]cachedRoles),
		rolesTTL:                time.Minute,
		roleUpdateAuditCache:    make(map[string]cachedRoleUpdateAudit),
		roleUpdateAuditDebounce: make(map[string]time.Time),
	}

	ms.handleMemberUpdate(session, &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userOne,
				Username: "member-one",
				Avatar:   "",
			},
			Roles: []string{"role-new-1"},
		},
	})
	ms.handleMemberUpdate(session, &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userTwo,
				Username: "member-two",
				Avatar:   "",
			},
			Roles: []string{"role-new-2"},
		},
	})

	if got := atomic.LoadInt32(&auditGets); got != 1 {
		t.Fatalf("expected one shared audit log fetch for both users, got %d", got)
	}
	if got := atomic.LoadInt32(&embedPosts); got != 2 {
		t.Fatalf("expected two role update notifications, got %d", got)
	}
	if got := atomic.LoadUint64(&ms.cacheRoleAuditHits); got == 0 {
		t.Fatalf("expected guild audit cache hit on second user update")
	}
}

func TestMonitoringService_HandleMemberUpdateDebouncesAuditRefreshByUser(t *testing.T) {
	const (
		guildID   = "g-role-debounce"
		channelID = "c-role-log"
		userID    = "u-role-debounce"
	)

	store, _ := newLoggingStore(t, "monitoring-role-debounce.db")
	if err := store.UpsertMemberRoles(guildID, userID, []string{"role-old"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed role snapshot: %v", err)
	}

	cfgMgr := newLoggingConfigManager(t, guildID, files.ChannelsConfig{
		RoleUpdate: channelID,
	})

	var auditGets int32
	var embedPosts int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/audit-logs", guildID):
			atomic.AddInt32(&auditGets, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"audit_log_entries": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&embedPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": fmt.Sprintf("msg-%d", atomic.LoadInt32(&embedPosts))})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})
	session.Identify.Intents = discordgo.IntentsGuildMembers

	ms := &MonitoringService{
		session:                 session,
		configManager:           cfgMgr,
		store:                   store,
		recentChanges:           map[string]time.Time{guildID + ":" + userID + ":default": time.Now().UTC()},
		rolesCache:              make(map[string]cachedRoles),
		rolesTTL:                time.Minute,
		roleUpdateAuditCache:    make(map[string]cachedRoleUpdateAudit),
		roleUpdateAuditDebounce: make(map[string]time.Time),
	}

	ms.handleMemberUpdate(session, &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userID,
				Username: "member-one",
				Avatar:   "",
			},
			Roles: []string{"role-mid"},
		},
	})
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
		t.Fatalf("get final role snapshot: %v", err)
	}
	if !sameStringSet(roles, []string{"role-new"}) {
		t.Fatalf("expected final role snapshot to reflect second event, got=%v", roles)
	}
	if got := atomic.LoadInt32(&auditGets); got != 1 {
		t.Fatalf("expected debounce to avoid a second audit log fetch, got %d", got)
	}
	if got := atomic.LoadInt32(&embedPosts); got != 2 {
		t.Fatalf("expected both role changes to be notified via fallback diff, got %d", got)
	}
}
