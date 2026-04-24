package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func recentAuditSnowflake(now time.Time) string {
	const discordEpochMS = int64(1420070400000)
	ms := now.UTC().UnixMilli()
	if ms < discordEpochMS {
		ms = discordEpochMS
	}
	id := uint64(ms-discordEpochMS) << 22
	return strconv.FormatUint(id, 10)
}

func TestMonitoringService_HandleGuildUpdatePersistsOwnerID(t *testing.T) {
	const (
		guildID = "g-owner"
		ownerID = "owner-new"
	)

	store, _ := newLoggingStore(t, "monitoring-owner.db")
	if err := store.SetGuildOwnerID(guildID, "owner-old"); err != nil {
		t.Fatalf("seed old owner id: %v", err)
	}
	ms := &MonitoringService{store: store}

	ms.handleGuildUpdate(nil, &discordgo.GuildUpdate{
		Guild: &discordgo.Guild{
			ID:      guildID,
			OwnerID: ownerID,
		},
	})

	gotOwnerID, ok, err := store.GetGuildOwnerID(guildID)
	if err != nil {
		t.Fatalf("get guild owner id: %v", err)
	}
	if !ok || gotOwnerID != ownerID {
		t.Fatalf("unexpected owner id: got=%q ok=%v want=%q", gotOwnerID, ok, ownerID)
	}
}

func TestMonitoringService_HandleMemberUpdate_AuditPathUpdatesRoleSnapshot(t *testing.T) {
	const (
		guildID   = "g-audit-update"
		userID    = "u-audit-update"
		channelID = "c-role-log"
	)

	store, _ := newLoggingStore(t, "monitoring-role-audit.db")
	if err := store.UpsertMemberRoles(guildID, userID, []string{"role-old"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed previous roles: %v", err)
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
						"id":          recentAuditSnowflake(time.Now().UTC()),
						"user_id":     "moderator-1",
						"target_id":   userID,
						"action_type": int(discordgo.AuditLogActionMemberRoleUpdate),
						"changes": []map[string]any{
							{
								"key": discordgo.AuditLogChangeKeyRoleAdd,
								"new_value": []map[string]any{
									{"id": "role-new", "name": "Role New"},
								},
							},
							{
								"key": discordgo.AuditLogChangeKeyRoleRemove,
								"new_value": []map[string]any{
									{"id": "role-old", "name": "Role Old"},
								},
							},
						},
					},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&embedPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "embed-1"})
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
				Username: "member-audit",
				Avatar:   "",
			},
			Roles: []string{"role-new"},
		},
	})

	roles, err := store.GetMemberRoles(guildID, userID)
	if err != nil {
		t.Fatalf("get role snapshot: %v", err)
	}
	if !sameStringSet(roles, []string{"role-new"}) {
		t.Fatalf("expected updated role snapshot, got %v", roles)
	}

	if got := atomic.LoadInt32(&embedPosts); got != 1 {
		t.Fatalf("expected one role update embed send, got %d", got)
	}
}

func TestMonitoringService_HandleMemberUpdate_FallbackPathUpdatesRoleSnapshot(t *testing.T) {
	const (
		guildID   = "g-fallback-update"
		userID    = "u-fallback-update"
		channelID = "c-role-log"
	)

	store, _ := newLoggingStore(t, "monitoring-role-fallback.db")
	if err := store.UpsertMemberRoles(guildID, userID, []string{"role-old"}, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("seed previous roles: %v", err)
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
				"audit_log_entries": []any{},
			})
		case r.Method == http.MethodPost && r.URL.Path == fmt.Sprintf("/channels/%s/messages", channelID):
			atomic.AddInt32(&embedPosts, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "embed-2"})
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
				Username: "member-fallback",
				Avatar:   "",
			},
			Roles: []string{"role-new"},
		},
	})

	roles, err := store.GetMemberRoles(guildID, userID)
	if err != nil {
		t.Fatalf("get role snapshot: %v", err)
	}
	if !sameStringSet(roles, []string{"role-new"}) {
		t.Fatalf("expected updated role snapshot in fallback path, got %v", roles)
	}

	if got := atomic.LoadInt32(&embedPosts); got != 1 {
		t.Fatalf("expected one fallback role update embed send, got %d", got)
	}
}

func TestMonitoringService_StartHeartbeatTickerPersistsPeriodicUpdates(t *testing.T) {
	store, _ := newLoggingStore(t, "monitoring-heartbeat.db")

	ms := &MonitoringService{
		store:    store,
		stopChan: make(chan struct{}),
		activity: newMonitoringRuntimeActivity(store),
	}

	origInterval := heartbeatTickInterval
	heartbeatTickInterval = 5 * time.Millisecond
	t.Cleanup(func() {
		heartbeatTickInterval = origInterval
		close(ms.stopChan)
		time.Sleep(10 * time.Millisecond)
		if err := ms.stopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	ms.startHeartbeat(context.Background())

	firstDeadline := time.Now().Add(100 * time.Millisecond)
	var first time.Time
	for time.Now().Before(firstDeadline) {
		if ts, ok, err := store.Heartbeat(context.Background()); err == nil && ok {
			first = ts
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if first.IsZero() {
		t.Fatalf("expected initial heartbeat timestamp to be persisted")
	}

	updated := false
	updateDeadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(updateDeadline) {
		if ts, ok, err := store.Heartbeat(context.Background()); err == nil && ok && ts.After(first) {
			updated = true
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if !updated {
		t.Fatalf("expected periodic heartbeat persistence update after ticker tick")
	}
}
