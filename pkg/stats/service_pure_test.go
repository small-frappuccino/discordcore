package stats

import (
	"context"
	"iter"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

type mockStateStore struct {
	members      map[string]map[string]members.CurrentState
	metadata     map[string]time.Time
	botHeartbeat map[string]time.Time
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		members: map[string]map[string]members.CurrentState{
			"guild-stats-main": {
				"user1": {
					UserID:   "user1",
					HasBot:   true,
					IsBot:    false,
					Roles:    []string{"role1"},
					JoinedAt: time.Now().UTC(),
				},
			},
		},
		metadata:     map[string]time.Time{"stats_channels.seeded:guild-stats-main": time.Now().UTC()},
		botHeartbeat: map[string]time.Time{"generic": time.Now().UTC()},
	}
}

func (m *mockStateStore) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error] {
	return func(yield func(members.CurrentState, error) bool) {
		for _, v := range m.members[guildID] {
			if !yield(v, nil) {
				return
			}
		}
	}
}

func (m *mockStateStore) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	t, ok := m.metadata[key]
	return t, ok, nil
}

func (m *mockStateStore) SetMetadata(ctx context.Context, key string, at time.Time) error {
	m.metadata[key] = at
	return nil
}

func (m *mockStateStore) UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error {
	if m.members[input.GuildID] == nil {
		m.members[input.GuildID] = make(map[string]members.CurrentState)
	}
	v := m.members[input.GuildID][input.UserID]
	v.UserID = input.UserID
	v.IsBot = input.IsBot
	m.members[input.GuildID][input.UserID] = v
	return nil
}

func (m *mockStateStore) UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error {
	if m.members[guildID] == nil {
		m.members[guildID] = make(map[string]members.CurrentState)
	}
	v := m.members[guildID][userID]
	v.UserID = userID
	v.Roles = roles
	m.members[guildID][userID] = v
	return nil
}

func (m *mockStateStore) MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error {
	if m.members[guildID] == nil {
		m.members[guildID] = make(map[string]members.CurrentState)
	}
	v := m.members[guildID][userID]
	v.UserID = userID
	v.LeftAt = at
	m.members[guildID][userID] = v
	return nil
}

func (m *mockStateStore) HeartbeatForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error) {
	t, ok := m.botHeartbeat[botInstanceID]
	return t, ok, nil
}

func TestHandlesGuild(t *testing.T) {
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: "g1", BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"}, FeatureRouting: map[string]string{"stats": "generic"}},
			{GuildID: "g2", BotInstanceTokens: map[string]files.EncryptedString{"other": "token"}, FeatureRouting: map[string]string{"stats": "other"}},
		}
		return nil
	})

	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	if !svc.handlesGuild("g1") {
		t.Errorf("expected to handle g1")
	}
	if svc.handlesGuild("g2") {
		t.Errorf("expected not to handle g2")
	}
	if svc.handlesGuild("g3") {
		t.Errorf("expected not to handle g3")
	}
}

func TestStatsServiceMethods(t *testing.T) {
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")

	if svc.Name() != "stats" {
		t.Errorf("unexpected name")
	}
	// svc.Type() returns svc.ServiceType, which is an integer. Let's just call it.
	svc.Type()
	svc.Priority()
	svc.Dependencies()
	svc.Stats()

	ctx := context.Background()
	svc.HealthCheck(ctx)
}

func TestShouldRunStatsUpdate(t *testing.T) {
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")

	interval := time.Minute
	// first run
	if !svc.shouldRunStatsUpdate("g1", interval) {
		t.Errorf("expected true on first run")
	}
	// second run immediately after
	if svc.shouldRunStatsUpdate("g1", interval) {
		t.Errorf("expected false on second run")
	}

	// Force update
	svc.ForceGuildUpdate("g1")
	if !svc.shouldRunStatsUpdate("g1", interval) {
		t.Errorf("expected true after force")
	}
}

func TestStatsTrackedRoles(t *testing.T) {
	channels := []files.StatsChannelConfig{
		{ChannelID: "1", RoleID: "r1"},
		{ChannelID: "2"},
		{ChannelID: "3", RoleID: "r2"},
		{ChannelID: "4", RoleID: "r1"},
	}
	trackedRoles, key := statsTrackedRoles(channels)
	if len(trackedRoles) != 2 {
		t.Errorf("expected 2 tracked roles, got %d", len(trackedRoles))
	}
	_, hasR1 := trackedRoles["r1"]
	_, hasR2 := trackedRoles["r2"]
	if !hasR1 || !hasR2 {
		t.Errorf("missing tracked roles")
	}
	// "r1,r2" or "r2,r1"
	if key != "r1,r2" && key != "r2,r1" {
		t.Errorf("unexpected key: %s", key)
	}
}

func TestStatsRequiresBotClassification(t *testing.T) {
	if statsRequiresBotClassification([]files.StatsChannelConfig{{MemberType: "all"}}) {
		t.Errorf("expected false")
	}
	if !statsRequiresBotClassification([]files.StatsChannelConfig{{MemberType: "humans"}}) {
		t.Errorf("expected true")
	}
	if !statsRequiresBotClassification([]files.StatsChannelConfig{{MemberType: "bots"}}) {
		t.Errorf("expected true")
	}
}

func TestFilterTrackedRoles(t *testing.T) {
	trackedRoles := map[string]struct{}{
		"r1": {},
		"r3": {},
	}
	roles := []string{"r1", "r2", "r3", "r4"}
	filtered := filterTrackedRoles(slices.Values(roles), trackedRoles)
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered roles, got %d", len(filtered))
	}
	if filtered[0] != "r1" && filtered[1] != "r3" {
		t.Errorf("unexpected filtered roles")
	}
}

func TestStatsCountForChannel(t *testing.T) {
	state := newStatsGuildState("r1", nil)
	state.applyAdd("user1", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("user2", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("bot1", statsMemberSnapshot{isBot: true, trackedRoles: []string{}})

	snapshot := statsGuildSnapshot{
		totals:     state.totals,
		roleTotals: state.roleTotals,
	}

	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all"}); count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "humans"}); count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "bots"}); count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all", RoleID: "r1"}); count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all", RoleID: "r2"}); count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestStatsGuildStateMethods(t *testing.T) {
	state := newStatsGuildState("r1", map[string]statsPublishedChannel{
		"c1": {count: 10, name: "test", label: "test"},
	})

	state.applyAdd("user1", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("user2", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("bot1", statsMemberSnapshot{isBot: true, trackedRoles: []string{}})

	state.applyUpdate("user1", statsMemberSnapshot{isBot: false, trackedRoles: []string{}})
	state.applyRemove("user2")

	// user1: lost r1
	// user2: removed completely
	// bot1: unchanged

	snapshot := statsGuildSnapshot{
		totals:     state.totals,
		roleTotals: state.roleTotals,
	}

	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all"}); count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all", RoleID: "r1"}); count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	state.applyDelta("user3", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}}, true, false) // isAdd=true, isRemove=false
	if state.totals.humans != 2 {
		t.Errorf("expected 2 humans, got %d", state.totals.humans)
	}
}

func TestStatsSnapshotHelpers(t *testing.T) {
	gwMem := MemberSnapshot{
		UserID: "u1",
		IsBot:  true,
		Roles: func(yield func(string) bool) {
			yield("r1")
			yield("r2")
		},
	}
	trackedRoles := map[string]struct{}{"r1": {}}

	_, snap, active := statsSnapshotFromGatewayMember(gwMem, trackedRoles)
	if !active {
		t.Errorf("expected active")
	}
	if !snap.isBot {
		t.Errorf("expected isBot to be true")
	}
	if len(snap.trackedRoles) != 1 || snap.trackedRoles[0] != "r1" {
		t.Errorf("unexpected tracked roles: %v", snap.trackedRoles)
	}

	storedState := members.CurrentState{
		UserID: "u2",
		IsBot:  false,
		Roles:  []string{"r1", "r2"},
		Active: true,
	}
	_, snap2, active2 := statsSnapshotFromStoredState(storedState, trackedRoles)
	if !active2 {
		t.Errorf("expected active")
	}
	if snap2.isBot {
		t.Errorf("expected isBot to be false")
	}
	if len(snap2.trackedRoles) != 1 || snap2.trackedRoles[0] != "r1" {
		t.Errorf("unexpected tracked roles: %v", snap2.trackedRoles)
	}
}

func TestStatsIntervalHelpers(t *testing.T) {
	// interval logic testing
	if statsInterval() != 5*time.Minute {
		t.Errorf("expected 5m default")
	}

	if statsReconcileInterval() != 6*time.Hour {
		t.Errorf("expected 6 hour reconcile interval")
	}
	statsStoreFreshnessLimit() // Just execute for coverage
	if statsSeedMetadataKey("g1") == "" {
		t.Errorf("unexpected seed key")
	}
}

func TestStatsStateAndStoreHelpers(t *testing.T) {
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	ctx := context.Background()

	// These depend on DB, so skip them when we don't have a DB mock.
	// We will only call publishStatsForGuild and streamGuildMembers since they have nil checks.

	// publishStatsForGuild should exit quickly if store/gw is nil
	err := svc.publishStatsForGuild(ctx, files.GuildConfig{GuildID: "g1"})
	if err == nil {
		t.Errorf("expected err for nil store/gw")
	}

	// streamGuildMembers should just return nil err since gw is nil
	svc.streamGuildMembers(ctx, "g1")
}

func TestStatsGuildStateMemoryHelpers(t *testing.T) {
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")

	// test replace and prune
	state := newStatsGuildState("r1", nil)
	state.initialized = true
	svc.replaceStatsGuildState("g1", state)

	channels := svc.statsPublishedChannels("g1")
	if channels == nil {
		t.Errorf("expected empty map, got nil")
	}

	ch, ok := svc.statsPublishedChannel("g1", "c1")
	if ok || ch.name != "" {
		t.Errorf("expected empty channel")
	}

	svc.recordStatsPublishedChannel("g1", "c1", statsPublishedChannel{count: 10, name: "test", label: "label"})
	ch2, ok2 := svc.statsPublishedChannel("g1", "c1")
	if !ok2 || ch2.count != 10 || ch2.name != "test" || ch2.label != "label" {
		t.Errorf("unexpected channel properties")
	}

	snap, okSnap := svc.statsSnapshot("g1")
	if !okSnap || snap.totals.all != 0 {
		t.Errorf("expected 0 totals")
	}
}

func TestStatsReconcileInterval(t *testing.T) {
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "g1",
				FeatureRouting:    map[string]string{"stats": "generic"},
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"},
				Stats:             files.StatsConfig{},
			},
		}
		return nil
	})

	NewStatsService(nil, cm, newMockStateStore(), slog.Default(), "generic")

	if statsReconcileInterval() != defaultStatsReconcileInterval {
		t.Errorf("expected default")
	}
}
