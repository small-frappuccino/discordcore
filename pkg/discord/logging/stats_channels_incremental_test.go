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

func testBoolPtr(value bool) *bool {
	return &value
}

func TestStatsGuildStateAppliesIncrementalMemberChanges(t *testing.T) {
	state := newStatsGuildState("role-a,role-b", nil)

	if !state.applyAdd("u1", statsMemberSnapshot{isBot: false, trackedRoles: []string{"role-a"}}) {
		t.Fatalf("expected first add to succeed")
	}
	if !state.applyAdd("u2", statsMemberSnapshot{isBot: true, trackedRoles: []string{"role-a", "role-b"}}) {
		t.Fatalf("expected second add to succeed")
	}

	if got := state.totals.total("all"); got != 2 {
		t.Fatalf("unexpected total members after add: got %d want 2", got)
	}
	if got := state.roleTotals["role-a"].total("all"); got != 2 {
		t.Fatalf("unexpected role-a count after add: got %d want 2", got)
	}
	if got := state.roleTotals["role-b"].total("bots"); got != 1 {
		t.Fatalf("unexpected role-b bot count after add: got %d want 1", got)
	}

	if !state.applyUpdate("u1", statsMemberSnapshot{isBot: false, trackedRoles: []string{"role-b"}}) {
		t.Fatalf("expected update to succeed")
	}
	if got := state.roleTotals["role-a"].total("all"); got != 1 {
		t.Fatalf("unexpected role-a count after update: got %d want 1", got)
	}
	if got := state.roleTotals["role-b"].total("all"); got != 2 {
		t.Fatalf("unexpected role-b count after update: got %d want 2", got)
	}

	if !state.applyRemove("u2") {
		t.Fatalf("expected remove to succeed")
	}
	if got := state.totals.total("all"); got != 1 {
		t.Fatalf("unexpected total members after remove: got %d want 1", got)
	}
	if got := state.totals.total("bots"); got != 0 {
		t.Fatalf("unexpected bot total after remove: got %d want 0", got)
	}
	if got := state.roleTotals["role-b"].total("all"); got != 1 {
		t.Fatalf("unexpected role-b count after remove: got %d want 1", got)
	}
	if _, exists := state.roleTotals["role-a"]; exists {
		t.Fatalf("expected role-a bucket to be removed after last member removal")
	}
}

func TestMonitoringServiceUpdateStatsChannelsUsesIncrementalState(t *testing.T) {
	const (
		guildID   = "g-stats"
		channelID = "c-stats"
	)

	var memberFetches int32
	var channelGets int32
	var channelEdits int32

	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/members", guildID):
			atomic.AddInt32(&memberFetches, 1)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"user": map[string]any{
						"id":  "u1",
						"bot": false,
					},
					"roles": []string{},
				},
				{
					"user": map[string]any{
						"id":  "u2",
						"bot": false,
					},
					"roles": []string{},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/channels/%s", channelID):
			atomic.AddInt32(&channelGets, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       channelID,
				"guild_id": guildID,
				"name":     "Members",
			})
		case r.Method == http.MethodPatch && r.URL.Path == fmt.Sprintf("/channels/%s", channelID):
			atomic.AddInt32(&channelEdits, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       channelID,
				"guild_id": guildID,
				"name":     "Members | 2",
			})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewMemoryConfigManager()
	if err := cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: testBoolPtr(true),
			},
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Enabled:            true,
			UpdateIntervalMins: 1,
			Channels: []files.StatsChannelConfig{
				{
					ChannelID:    channelID,
					Label:        "Members",
					NameTemplate: "{label} | {count}",
					MemberType:   "humans",
				},
			},
		},
	}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		statsLastRun:  make(map[string]time.Time),
		statsGuilds:   make(map[string]*statsGuildState),
	}

	if err := ms.updateStatsChannels(context.Background()); err != nil {
		t.Fatalf("first updateStatsChannels error: %v", err)
	}
	if got := atomic.LoadInt32(&memberFetches); got != 1 {
		t.Fatalf("expected one member pagination on first publish, got %d", got)
	}
	if got := atomic.LoadInt32(&channelGets); got != 1 {
		t.Fatalf("expected one channel lookup on first publish, got %d", got)
	}
	if got := atomic.LoadInt32(&channelEdits); got != 1 {
		t.Fatalf("expected one channel edit on first publish, got %d", got)
	}

	ms.statsLastRun[guildID] = time.Time{}
	if err := ms.updateStatsChannels(context.Background()); err != nil {
		t.Fatalf("second updateStatsChannels error: %v", err)
	}
	if got := atomic.LoadInt32(&memberFetches); got != 1 {
		t.Fatalf("expected no extra member pagination when counters are warm, got %d", got)
	}
	if got := atomic.LoadInt32(&channelGets); got != 1 {
		t.Fatalf("expected no extra channel lookup when published value is unchanged, got %d", got)
	}
	if got := atomic.LoadInt32(&channelEdits); got != 1 {
		t.Fatalf("expected no extra channel edit when published value is unchanged, got %d", got)
	}
}

func TestMonitoringServiceHandleMemberUpdateUpdatesStatsWhenRoleLogSuppressed(t *testing.T) {
	const (
		guildID = "g-stats-update"
		userID  = "u-stats-update"
		roleID  = "role-a"
	)

	cfgMgr := files.NewMemoryConfigManager()
	if err := cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: testBoolPtr(true),
			},
			Logging: files.FeatureLoggingToggles{
				RoleUpdate: testBoolPtr(false),
			},
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Enabled: true,
			Channels: []files.StatsChannelConfig{
				{
					ChannelID: channelIDForTest("stats-update"),
					RoleID:    roleID,
				},
			},
		},
	}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	_, trackedRolesKey := statsTrackedRoles([]files.StatsChannelConfig{{RoleID: roleID}})
	state := newStatsGuildState(trackedRolesKey, nil)
	state.initialized = true
	if !state.applyAdd(userID, statsMemberSnapshot{isBot: false}) {
		t.Fatalf("expected initial member seed to succeed")
	}

	ms := &MonitoringService{
		session:       newLoggingLifecycleSession(t),
		configManager: cfgMgr,
		recentChanges: map[string]time.Time{
			guildID + ":" + userID + ":default": time.Now().UTC(),
		},
		statsGuilds: map[string]*statsGuildState{
			guildID: state,
		},
	}

	ms.handleMemberUpdate(ms.session, &discordgo.GuildMemberUpdate{
		Member: &discordgo.Member{
			GuildID: guildID,
			User: &discordgo.User{
				ID:       userID,
				Username: "stats-user",
			},
			Roles: []string{roleID},
		},
	})

	bucket := ms.statsGuilds[guildID].roleTotals[roleID]
	if got := bucket.total("all"); got != 1 {
		t.Fatalf("expected stats role count to update even when role log emission is suppressed, got %d", got)
	}
	if ms.statsGuilds[guildID].dirty {
		t.Fatalf("expected stats state to remain clean after in-band member update")
	}
}

func TestMonitoringServiceUpdateStatsChannelsHydratesFromStore(t *testing.T) {
	const (
		guildID   = "g-stats-store"
		channelID = "c-stats-store"
		userID    = "u-stats-store"
		roleID    = "role-store"
	)

	store, _ := newLoggingStore(t, "stats-store.db")
	joinedAt := time.Now().UTC().Add(-time.Hour)
	seenAt := joinedAt.Add(30 * time.Minute)
	if err := store.UpsertMemberPresenceContext(context.Background(), guildID, userID, joinedAt, seenAt, false); err != nil {
		t.Fatalf("seed member presence: %v", err)
	}
	if err := store.UpsertMemberRoles(guildID, userID, []string{roleID}, seenAt); err != nil {
		t.Fatalf("seed member roles: %v", err)
	}
	if err := store.SetMetadataContext(context.Background(), statsSeedMetadataKey(guildID), seenAt); err != nil {
		t.Fatalf("seed stats metadata: %v", err)
	}

	var memberFetches int32
	var channelGets int32
	var channelEdits int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/guilds/%s/members", guildID):
			atomic.AddInt32(&memberFetches, 1)
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodGet && r.URL.Path == fmt.Sprintf("/channels/%s", channelID):
			atomic.AddInt32(&channelGets, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       channelID,
				"guild_id": guildID,
				"name":     "Tracked",
			})
		case r.Method == http.MethodPatch && r.URL.Path == fmt.Sprintf("/channels/%s", channelID):
			atomic.AddInt32(&channelEdits, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       channelID,
				"guild_id": guildID,
				"name":     "Tracked | 1",
			})
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewMemoryConfigManager()
	if err := cfgMgr.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: testBoolPtr(true),
			},
			StatsChannels: testBoolPtr(true),
		},
		Stats: files.StatsConfig{
			Enabled:            true,
			UpdateIntervalMins: 30,
			Channels: []files.StatsChannelConfig{
				{
					ChannelID:    channelID,
					Label:        "Tracked",
					NameTemplate: "{label} | {count}",
					RoleID:       roleID,
				},
			},
		},
	}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	ms := &MonitoringService{
		session:       session,
		configManager: cfgMgr,
		store:         store,
		statsLastRun:  make(map[string]time.Time),
		statsGuilds:   make(map[string]*statsGuildState),
	}
	now := time.Now().UTC()
	if err := store.SetHeartbeatContext(context.Background(), now); err != nil {
		t.Fatalf("set heartbeat: %v", err)
	}

	if err := ms.updateStatsChannels(context.Background()); err != nil {
		t.Fatalf("updateStatsChannels error: %v", err)
	}
	if got := atomic.LoadInt32(&memberFetches); got != 0 {
		t.Fatalf("expected store hydration to avoid guild member fetches, got %d", got)
	}
	if got := atomic.LoadInt32(&channelGets); got != 1 {
		t.Fatalf("expected one channel lookup for first publish, got %d", got)
	}
	if got := atomic.LoadInt32(&channelEdits); got != 1 {
		t.Fatalf("expected one channel edit from hydrated store state, got %d", got)
	}
}

func channelIDForTest(suffix string) string {
	return "c-" + suffix
}
