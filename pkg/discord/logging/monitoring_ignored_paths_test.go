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

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordgo"
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
	ms := &MonitoringService{store: store, statsService: NewStatsService(nil, nil, nil, nil, "", "", nil, nil, nil)}

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
		changeDebounce: changeDebouncer{
			entries: map[string]time.Time{
				guildID + ":" + userID + ":default": time.Now().UTC(),
			},
		},
		rolesCacheService: NewRolesCacheService(nil),
		statsService:      NewStatsService(nil, nil, nil, nil, "", "", nil, nil, nil),
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

	var roles []string
	for r, err := range store.GetMemberRoles(guildID, userID) {
		if err != nil {
			t.Fatalf("get role snapshot: %v", err)
		}
		roles = append(roles, r)
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
		changeDebounce: changeDebouncer{
			entries: map[string]time.Time{
				guildID + ":" + userID + ":default": time.Now().UTC(),
			},
		},
		rolesCacheService: NewRolesCacheService(nil),
		statsService:      NewStatsService(nil, nil, nil, nil, "", "", nil, nil, nil),
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

	var roles []string
	for r, err := range store.GetMemberRoles(guildID, userID) {
		if err != nil {
			t.Fatalf("get role snapshot: %v", err)
		}
		roles = append(roles, r)
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

	// Inject a monotonic clock so the "second.After(first)" invariant is
	// independent of the host wall clock's resolution. On Windows
	// `time.Now()` can have ~1ms granularity, and at a 5ms tick interval
	// under heavy parallel load (Postgres schema churn from sibling tests)
	// two consecutive samples can collide once truncated to the precision
	// the row carries through the round-trip. Each heartbeat attempt calls
	// ra.now() exactly once, so monotonically advancing the counter gives
	// the test a deterministic gap to assert against.
	base := time.Date(2026, time.January, 2, 7, 0, 0, 0, time.UTC)
	var calls atomic.Int32

	ticks := newTickRecorder(t, 2)
	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           runErrWithTimeoutContext,
		EventTimeout:     monitoringPersistenceTimeout,
		HeartbeatTimeout: monitoringPersistenceTimeout,
		Warn:             log.ApplicationLogger().Warn,
		Now: func() time.Time {
			return base.Add(time.Duration(calls.Add(1)) * time.Second)
		},
		OnHeartbeatTick: ticks.Hook,
	})

	ms := &MonitoringService{
		store:     store,
		controlCh: make(chan func()), stopChan: make(chan struct{}),
		activity:     activity,
		statsService: NewStatsService(nil, nil, nil, nil, "", "", nil, nil, nil),
	}

	origInterval := heartbeatTickInterval
	// 25ms (rather than the prior 5ms) keeps the test fast while reducing
	// CPU pressure when the package is run with high parallelism — under
	// load a 5ms ticker fires faster than the scheduler can drain the
	// hook + DB roundtrip, which inflated end-to-end wall time and pushed
	// tickRecorder.Next past its 2s safety timeout.
	heartbeatTickInterval = 25 * time.Millisecond
	t.Cleanup(func() {
		heartbeatTickInterval = origInterval
		close(ms.stopChan)
		if err := ms.stopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	ms.startHeartbeat(context.Background())

	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected initial heartbeat to succeed: %v", err)
	}
	first, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || first.IsZero() {
		t.Fatalf("expected initial heartbeat timestamp to be persisted: ok=%v err=%v", ok, err)
	}

	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected periodic heartbeat to succeed: %v", err)
	}
	second, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok {
		t.Fatalf("expected periodic heartbeat timestamp to be persisted: ok=%v err=%v", ok, err)
	}
	if !second.After(first) {
		t.Fatalf("expected periodic heartbeat to advance the timestamp: first=%s second=%s", first.UTC(), second.UTC())
	}
}
