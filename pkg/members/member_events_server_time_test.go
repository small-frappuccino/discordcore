package members

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/system"
)

// Mock Repositories
type mockMembersRepo struct {
	Repository
	mu           sync.Mutex
	joinedAt     time.Time
	upsertErr    error
	joinErr      error
	memberJoinAt time.Time
	memberJoinOk bool
}

func (m *mockMembersRepo) UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joinedAt = joinedAt
	return m.upsertErr
}

func (m *mockMembersRepo) MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.memberJoinAt, m.memberJoinOk, m.joinErr
}

type mockSystemRepo struct {
	system.Repository
	mu           sync.Mutex
	joinGuildID  string
	joinUserID   string
	joinAt       time.Time
	leaveGuildID string
	leaveUserID  string
	leaveAt      time.Time
	joinErr      error
	leaveErr     error
}

func (m *mockSystemRepo) IncrementDailyMemberJoinContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joinGuildID = guildID
	m.joinUserID = userID
	m.joinAt = timestamp
	return m.joinErr
}

func (m *mockSystemRepo) IncrementDailyMemberLeaveContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leaveGuildID = guildID
	m.leaveUserID = userID
	m.leaveAt = timestamp
	return m.leaveErr
}

func (m *mockSystemRepo) SetLastEventForBot(ctx context.Context, instanceID string, t time.Time) error {
	return nil
}

func (m *mockSystemRepo) SetHeartbeatForBot(ctx context.Context, instanceID string, t time.Time) error {
	return nil
}

type mockMemberSink struct {
	mu                  sync.Mutex
	joinEvents          []MemberJoinIntent
	leaveEvents         []MemberLeaveIntent
	roleUpdateGuildID   string
	roleUpdateUser      string
	addedRoles          []string
	removedRoles        []string
	avatarUpdateGuildID string
	avatarUpdateUser    string
	oldAvatar           string
	newAvatar           string
}

func (m *mockMemberSink) OnMemberJoin(ctx context.Context, intent MemberJoinIntent, accountAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joinEvents = append(m.joinEvents, intent)
}

func (m *mockMemberSink) OnMemberLeave(ctx context.Context, intent MemberLeaveIntent, serverTime time.Duration, botTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leaveEvents = append(m.leaveEvents, intent)
}

func (m *mockMemberSink) OnRoleUpdate(ctx context.Context, intent RoleUpdateIntent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roleUpdateGuildID = intent.GuildID
	m.roleUpdateUser = intent.UserID
	m.addedRoles = intent.AddedRoles
	m.removedRoles = intent.RemovedRoles
}

func (m *mockMemberSink) OnAvatarUpdate(ctx context.Context, intent AvatarUpdateIntent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.avatarUpdateGuildID = intent.GuildID
	m.avatarUpdateUser = intent.UserID
	m.oldAvatar = intent.OldAvatarHash
	m.newAvatar = intent.NewAvatarHash
}

func (m *mockMemberSink) OnModerationAction(ctx context.Context, intent ModerationActionIntent) {
}

type mockDiscordAdapter struct {
	mu              sync.Mutex
	addRoleCalls    int
	removeRoleCalls int
	memberCalls     int
	meCalls         int
}

func (m *mockDiscordAdapter) Me() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.meCalls++
	return "99999", nil
}

func (m *mockDiscordAdapter) MemberJoinedAt(ctx context.Context, guildID, userID string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memberCalls++
	return time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC), nil
}

func (m *mockDiscordAdapter) AddRole(ctx context.Context, guildID, userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addRoleCalls++
	return nil
}

func (m *mockDiscordAdapter) RemoveRole(ctx context.Context, guildID, userID, roleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeRoleCalls++
	return nil
}

func setupTestService(t *testing.T) (*MemberEventService, *mockMembersRepo, *mockSystemRepo, *mockMemberSink, *mockDiscordAdapter) {
	t.Helper()
	store := &config.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111",
				Channels: files.ChannelsConfig{
					MemberJoin:    "222",
					MemberLeave:   "222",
					AvatarLogging: "222",
					RoleUpdate:    "222",
				},
				Roles: files.RolesConfig{
					AutoAssignment: files.AutoAssignmentConfig{
						Enabled:       true,
						TargetRoleID:  "999",
						RequiredRoles: []string{"333", "443"},
					},
				},
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()

	mRepo := &mockMembersRepo{}
	sRepo := &mockSystemRepo{}
	sink := &mockMemberSink{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	adapter := &mockDiscordAdapter{}

	deps := EventServiceDeps{
		ConfigManager:  mgr,
		Sink:           sink,
		MembersRepo:    mRepo,
		SystemRepo:     sRepo,
		BotInstanceID:  "",
		Logger:         logger,
		DiscordAdapter: adapter,
	}

	svc := NewMemberEventServiceForBot(deps)
	_ = NewMemberEventService(mgr, sink, mRepo, sRepo, logger)

	return svc, mRepo, sRepo, sink, adapter
}

func TestMemberEventService_LifeCycle(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := setupTestService(t)
	ctx := context.Background()

	if svc.IsRunning() {
		t.Errorf("service should not be running before start")
	}

	err := svc.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if !svc.IsRunning() {
		t.Errorf("service should be running after start")
	}

	if svc.Name() != "member_events_" {
		t.Errorf("unexpected name: %s", svc.Name())
	}

	if svc.Type() != service.TypeMonitoring {
		t.Errorf("unexpected type")
	}

	if svc.Priority() != service.PriorityNormal {
		t.Errorf("unexpected priority")
	}

	if len(svc.Dependencies()) != 0 {
		t.Errorf("unexpected dependencies")
	}

	hs := svc.HealthCheck(ctx)
	if !hs.Healthy {
		t.Errorf("expected healthy")
	}

	_ = svc.Stats()

	err = svc.Stop(ctx)
	if err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	if svc.IsRunning() {
		t.Errorf("service should not be running after stop")
	}
}

func TestMemberEventService_IngestGuildMemberAdd(t *testing.T) {
	t.Parallel()
	svc, mRepo, sRepo, sink, adapter := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// Test nil and bot filters
	svc.IngestGuildMemberAdd(context.Background(), MemberJoinIntent{})
	svc.IngestGuildMemberAdd(context.Background(), MemberJoinIntent{
		Bot: true,
	})
	if len(sink.joinEvents) != 0 {
		t.Errorf("expected no events forwarded for nil or bot joins")
	}

	// Canceled context
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()
	svc.IngestGuildMemberAdd(ctxCancel, MemberJoinIntent{
		UserID: "12345",
		Bot:    false,
	})
	if len(sink.joinEvents) != 0 {
		t.Errorf("expected no events when context is canceled")
	}

	// Normal member join
	joinTime := time.Now().Add(-10 * time.Minute)
	e := MemberJoinIntent{
		GuildID:  "111",
		UserID:   "12345",
		Bot:      false,
		RoleIDs:  []string{"333", "443"},
		JoinedAt: joinTime,
	}

	svc.IngestGuildMemberAdd(context.Background(), e)

	if len(sink.joinEvents) != 1 {
		t.Errorf("expected exactly one join event, got %d", len(sink.joinEvents))
	}

	// Verify persistence in repository
	mRepo.mu.Lock()
	if mRepo.joinedAt.Unix() != joinTime.Unix() {
		t.Errorf("expected joinedAt to be persisted, got %v", mRepo.joinedAt)
	}
	mRepo.mu.Unlock()

	sRepo.mu.Lock()
	if sRepo.joinGuildID != "111" || sRepo.joinUserID != "12345" {
		t.Errorf("expected daily member join metric incremented")
	}
	sRepo.mu.Unlock()

	// Verify target role added
	adapter.mu.Lock()
	if adapter.addRoleCalls != 1 {
		t.Errorf("expected 1 call to AddRole, got %d", adapter.addRoleCalls)
	}
	adapter.mu.Unlock()
}

func TestMemberEventService_IngestGuildMemberRemove(t *testing.T) {
	t.Parallel()
	svc, _, sRepo, sink, adapter := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// Normal member leave (server time from memory)
	svc.joinMu.Lock()
	svc.joinTimes["111:12345"] = time.Now().Add(-2 * time.Hour)
	svc.joinMu.Unlock()

	e := MemberLeaveIntent{
		GuildID: "111",
		UserID:  "12345",
		Bot:     false,
	}

	svc.IngestGuildMemberRemove(context.Background(), e)

	if len(sink.leaveEvents) != 1 {
		t.Fatalf("expected exactly one leave event, got %d", len(sink.leaveEvents))
	}

	sRepo.mu.Lock()
	if sRepo.leaveGuildID != "111" || sRepo.leaveUserID != "12345" {
		t.Errorf("expected daily member leave metric incremented")
	}
	sRepo.mu.Unlock()

	adapter.mu.Lock()
	if adapter.meCalls != 1 || adapter.memberCalls != 1 {
		t.Errorf("expected bot time calls (meCalls=%d, memberCalls=%d)", adapter.meCalls, adapter.memberCalls)
	}
	adapter.mu.Unlock()
}

func TestMemberEventService_IngestGuildMemberRemove_StoreFallback(t *testing.T) {
	t.Parallel()
	svc, mRepo, sRepo, sink, _ := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// Server time from store
	mRepo.mu.Lock()
	mRepo.memberJoinAt = time.Now().Add(-5 * time.Hour)
	mRepo.memberJoinOk = true
	mRepo.mu.Unlock()

	e := MemberLeaveIntent{
		GuildID: "111",
		UserID:  "99999",
		Bot:     false,
	}

	svc.IngestGuildMemberRemove(context.Background(), e)

	if len(sink.leaveEvents) != 1 {
		t.Fatalf("expected exactly one leave event, got %d", len(sink.leaveEvents))
	}

	sRepo.mu.Lock()
	if sRepo.leaveGuildID != "111" || sRepo.leaveUserID != "99999" {
		t.Errorf("expected daily member leave metric incremented")
	}
	sRepo.mu.Unlock()
}

func TestMemberEventService_IngestGuildMemberUpdate(t *testing.T) {
	t.Parallel()
	svc, _, _, sink, adapter := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// New member state
	e := MemberUpdateIntent{
		GuildID:    "111",
		UserID:     "12345",
		Bot:        false,
		AvatarHash: "new_avatar_hash",
		RoleIDs:    []string{"333", "443"}, // Gained roleB, should add target role
		OldRoleIDs: []string{"333"},        // has roleA but not roleB
		OldAvatar:  "old_avatar_hash",
	}

	svc.IngestGuildMemberUpdate(context.Background(), e)

	// Verify avatar update and role update sinks called
	sink.mu.Lock()
	if sink.avatarUpdateGuildID != "111" || sink.oldAvatar != "old_avatar_hash" || sink.newAvatar != "new_avatar_hash" {
		t.Errorf("avatar update sink not called correctly")
	}
	if sink.roleUpdateGuildID != "111" || len(sink.addedRoles) != 1 || sink.addedRoles[0] != "443" {
		t.Errorf("role update sink not called correctly")
	}
	sink.mu.Unlock()

	adapter.mu.Lock()
	if adapter.addRoleCalls != 1 {
		t.Errorf("expected 1 AddRole call, got %d", adapter.addRoleCalls)
	}
	adapter.mu.Unlock()

	// Now let's trigger role removal
	e = MemberUpdateIntent{
		GuildID:    "111",
		UserID:     "12345",
		Bot:        false,
		RoleIDs:    []string{"443", "999"},        // Lost roleA, should remove target role
		OldRoleIDs: []string{"333", "443", "999"}, // has target role
	}

	svc.IngestGuildMemberUpdate(context.Background(), e)

	adapter.mu.Lock()
	if adapter.removeRoleCalls != 1 {
		t.Errorf("expected 1 RemoveRole call, got %d", adapter.removeRoleCalls)
	}
	adapter.mu.Unlock()
}

func TestMemberEventService_CleanupJoinTimes(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _ := setupTestService(t)
	svc.joinTimes = make(map[string]time.Time)
	svc.joinTimes["k1"] = time.Now().Add(-8 * 24 * time.Hour) // older than 7 days
	svc.joinTimes["k2"] = time.Now().Add(-1 * time.Hour)      // recent

	svc.cleanupJoinTimes()

	svc.joinMu.Lock()
	defer svc.joinMu.Unlock()
	if _, ok := svc.joinTimes["k1"]; ok {
		t.Errorf("expected k1 to be cleaned up")
	}
	if _, ok := svc.joinTimes["k2"]; !ok {
		t.Errorf("expected k2 to be preserved")
	}
}

func TestInMemoryMetrics(t *testing.T) {
	t.Parallel()
	metrics := NewInMemoryMetrics()
	metrics.RecordGuildMemberCall()
	metrics.RecordStateMemberCacheHit()
	metrics.RecordRolesCacheMemoryHit()
	metrics.RecordRolesCacheStoreHit()
	metrics.RecordRolesAuditCacheHit()
	metrics.RecordAuditLogCall()

	snap := metrics.Snapshot()
	if snap.StateMemberHitsTotal != 1 {
		t.Errorf("expected StateMemberHitsTotal=1, got %d", snap.StateMemberHitsTotal)
	}
}

func TestNopMemberSink(t *testing.T) {
	t.Parallel()
	sink := NopMemberSink{}
	// None should panic
	sink.OnMemberJoin(context.Background(), MemberJoinIntent{}, 0)
	sink.OnMemberLeave(context.Background(), MemberLeaveIntent{}, 0, 0)
	sink.OnRoleUpdate(context.Background(), RoleUpdateIntent{})
	sink.OnAvatarUpdate(context.Background(), AvatarUpdateIntent{})
	sink.OnModerationAction(context.Background(), ModerationActionIntent{})
}

func TestNopMetrics(t *testing.T) {
	t.Parallel()
	metrics := NopMetrics{}
	// None should panic
	metrics.RecordGuildMemberCall()
	metrics.RecordStateMemberCacheHit()
	metrics.RecordRolesCacheMemoryHit()
	metrics.RecordRolesCacheStoreHit()
	metrics.RecordRolesAuditCacheHit()
	metrics.RecordAuditLogCall()
}

func TestMemberEventService_HandlesGuild(t *testing.T) {
	t.Parallel()
	// 1. nil service or nil config manager
	var nilSvc *MemberEventService
	if nilSvc.handlesGuild("111") {
		t.Errorf("expected false for nil service")
	}

	svc, _, _, _, _ := setupTestService(t)
	// ConfigManager is nil
	svc.configManager = nil
	if svc.handlesGuild("111") {
		t.Errorf("expected false for nil config manager")
	}

	// Restore config manager
	svc, _, _, _, _ = setupTestService(t)

	// 2. empty botInstanceID handles everything
	svc.botInstanceID = ""
	if !svc.handlesGuild("111") {
		t.Errorf("expected true for empty botInstanceID")
	}

	// 3. non-empty botInstanceID
	svc.botInstanceID = "instance1"

	// empty guild ID
	if svc.handlesGuild("") {
		t.Errorf("expected false for empty guildID")
	}

	// guild config not found
	if svc.handlesGuild("999") {
		t.Errorf("expected false for non-existent guild")
	}

	// Setup config store with a guild that doesn't belong to instance1
	store := &config.MemoryConfigStore{}
	_ = store.Save(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "111",
				BotInstanceTokens: map[string]files.EncryptedString{
					"instance2": "token2",
				},
				FeatureRouting: map[string]string{
					"roles": "instance2",
				},
			},
			{
				GuildID: "222",
				BotInstanceTokens: map[string]files.EncryptedString{
					"instance1": "token1",
				},
				FeatureRouting: map[string]string{
					"roles":   "instance1",
					"logging": "instance2",
				},
			},
			{
				GuildID: "333",
				BotInstanceTokens: map[string]files.EncryptedString{
					"instance1": "token1",
				},
				FeatureRouting: map[string]string{
					"roles":   "instance2",
					"logging": "instance1",
				},
			},
			{
				GuildID: "444",
				BotInstanceTokens: map[string]files.EncryptedString{
					"instance1": "token1",
				},
				FeatureRouting: map[string]string{
					"roles":   "instance2",
					"logging": "instance2",
				},
			},
		},
	})
	mgr := files.NewConfigManagerWithStore(store, nil)
	_ = mgr.LoadConfig()
	svc.configManager = mgr

	// 111 belongs to instance2, not instance1
	if svc.handlesGuild("111") {
		t.Errorf("expected false for guild 111")
	}

	// 222 belongs to instance1, and roles resolves to instance1
	if !svc.handlesGuild("222") {
		t.Errorf("expected true for guild 222 (roles routed)")
	}

	// 333 belongs to instance1, and logging resolves to instance1
	if !svc.handlesGuild("333") {
		t.Errorf("expected true for guild 333 (logging routed)")
	}

	// 444 belongs to instance1, but neither roles nor logging resolves to instance1
	if svc.handlesGuild("444") {
		t.Errorf("expected false for guild 444 (unrouted)")
	}
}
