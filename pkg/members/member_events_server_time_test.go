package members

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
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
	joinEvents          []*gateway.GuildMemberAddEvent
	leaveEvents         []*gateway.GuildMemberRemoveEvent
	roleUpdateGuildID   string
	roleUpdateUser      discord.User
	addedRoles          []discord.RoleID
	removedRoles        []discord.RoleID
	avatarUpdateGuildID string
	avatarUpdateUser    discord.User
	oldAvatar           string
	newAvatar           string
}

func (m *mockMemberSink) OnMemberJoin(ctx context.Context, e *gateway.GuildMemberAddEvent, accountAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joinEvents = append(m.joinEvents, e)
}

func (m *mockMemberSink) OnMemberLeave(ctx context.Context, e *gateway.GuildMemberRemoveEvent, serverTime time.Duration, botTime time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leaveEvents = append(m.leaveEvents, e)
}

func (m *mockMemberSink) OnRoleUpdate(ctx context.Context, guildID string, user discord.User, addedRoles, removedRoles []discord.RoleID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roleUpdateGuildID = guildID
	m.roleUpdateUser = user
	m.addedRoles = addedRoles
	m.removedRoles = removedRoles
}

func (m *mockMemberSink) OnAvatarUpdate(ctx context.Context, guildID string, user discord.User, oldAvatarHash, newAvatarHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.avatarUpdateGuildID = guildID
	m.avatarUpdateUser = user
	m.oldAvatar = oldAvatarHash
	m.newAvatar = newAvatarHash
}

func (m *mockMemberSink) OnModerationAction(ctx context.Context, guildID string, actionType string, targetUser discord.User, reason string, moderator discord.User) {
}

// Mock RoundTripper
type mockTransport struct {
	mu              sync.Mutex
	addRoleCalls    int
	removeRoleCalls int
	memberCalls     int
	meCalls         int
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := req.URL.Path
	if req.Method == "PUT" && strings.Contains(path, "/roles/") {
		m.addRoleCalls++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}
	if req.Method == "DELETE" && strings.Contains(path, "/roles/") {
		m.removeRoleCalls++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}
	if req.Method == "GET" && strings.Contains(path, "/members/") {
		m.memberCalls++
		memberJSON := `{
			"user": {
				"id": "12345",
				"username": "testuser",
				"bot": false
			},
			"roles": [],
			"joined_at": "2026-06-23T00:00:00Z"
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(memberJSON)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}
	if req.Method == "GET" && strings.Contains(path, "/users/@me") {
		m.meCalls++
		meJSON := `{
			"id": "99999",
			"username": "botname",
			"bot": true
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(meJSON)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func setupTestService(t *testing.T) (*MemberEventService, *mockMembersRepo, *mockSystemRepo, *mockMemberSink, *mockTransport) {
	t.Helper()
	store := &files.MemoryConfigStore{}
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

	transport := &mockTransport{}
	st := state.New("Bot test")
	st.Client.Client.Client = httpdriver.WrapClient(http.Client{Transport: transport})

	deps := EventServiceDeps{
		ConfigManager: mgr,
		Sink:          sink,
		MembersRepo:   mRepo,
		SystemRepo:    sRepo,
		BotInstanceID: "",
		Logger:        logger,
		ArikawaState:  st,
	}

	svc := NewMemberEventServiceForBot(deps)
	// We also test the alternative constructor
	_ = NewMemberEventService(mgr, sink, mRepo, sRepo, logger)

	return svc, mRepo, sRepo, sink, transport
}

func TestMemberEventService_LifeCycle(t *testing.T) {
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
	svc, mRepo, sRepo, sink, transport := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// Test nil and bot filters
	svc.IngestGuildMemberAdd(context.Background(), nil)
	svc.IngestGuildMemberAdd(context.Background(), &gateway.GuildMemberAddEvent{
		Member: discord.Member{
			User: discord.User{Bot: true},
		},
	})
	if len(sink.joinEvents) != 0 {
		t.Errorf("expected no events forwarded for nil or bot joins")
	}

	// Canceled context
	ctxCancel, cancel := context.WithCancel(context.Background())
	cancel()
	svc.IngestGuildMemberAdd(ctxCancel, &gateway.GuildMemberAddEvent{
		Member: discord.Member{
			User: discord.User{ID: discord.UserID(12345), Bot: false},
		},
	})
	if len(sink.joinEvents) != 0 {
		t.Errorf("expected no events when context is canceled")
	}

	// Normal member join (should save in store and trigger role assignment evaluation)
	joinTime := time.Now().Add(-10 * time.Minute)
	e := &gateway.GuildMemberAddEvent{
		Member: discord.Member{
			User: discord.User{
				ID:  discord.UserID(12345),
				Bot: false,
			},
			RoleIDs: []discord.RoleID{discord.RoleID(333), discord.RoleID(443)}, // Both required roles present
			Joined:  discord.Timestamp(joinTime),
		},
	}
	e.GuildID = discord.GuildID(111)

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

	// Verify target role added via Arikawa Client
	transport.mu.Lock()
	if transport.addRoleCalls != 1 {
		t.Errorf("expected 1 call to AddRole, got %d", transport.addRoleCalls)
	}
	transport.mu.Unlock()
}

func TestMemberEventService_IngestGuildMemberRemove(t *testing.T) {
	svc, _, sRepo, sink, transport := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// Normal member leave (server time from memory)
	svc.joinMu.Lock()
	svc.joinTimes["111:12345"] = time.Now().Add(-2 * time.Hour)
	svc.joinMu.Unlock()

	e := &gateway.GuildMemberRemoveEvent{
		User: discord.User{
			ID:  discord.UserID(12345),
			Bot: false,
		},
	}
	e.GuildID = discord.GuildID(111)

	svc.IngestGuildMemberRemove(context.Background(), e)

	if len(sink.leaveEvents) != 1 {
		t.Fatalf("expected exactly one leave event, got %d", len(sink.leaveEvents))
	}

	sRepo.mu.Lock()
	if sRepo.leaveGuildID != "111" || sRepo.leaveUserID != "12345" {
		t.Errorf("expected daily member leave metric incremented")
	}
	sRepo.mu.Unlock()

	transport.mu.Lock()
	if transport.meCalls != 1 || transport.memberCalls != 1 {
		t.Errorf("expected bot time calls (meCalls=%d, memberCalls=%d)", transport.meCalls, transport.memberCalls)
	}
	transport.mu.Unlock()
}

func TestMemberEventService_IngestGuildMemberRemove_StoreFallback(t *testing.T) {
	svc, mRepo, sRepo, sink, _ := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// Server time from store
	mRepo.mu.Lock()
	mRepo.memberJoinAt = time.Now().Add(-5 * time.Hour)
	mRepo.memberJoinOk = true
	mRepo.mu.Unlock()

	e := &gateway.GuildMemberRemoveEvent{
		User: discord.User{
			ID:  discord.UserID(99999),
			Bot: false,
		},
	}
	e.GuildID = discord.GuildID(111)

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
	svc, _, _, sink, transport := setupTestService(t)
	_ = svc.Start(context.Background())
	defer func() { _ = svc.Stop(context.Background()) }()

	// Old member state
	oldMember := &discord.Member{
		User: discord.User{
			ID:     discord.UserID(12345),
			Bot:    false,
			Avatar: "old_avatar_hash",
		},
		RoleIDs: []discord.RoleID{discord.RoleID(333)}, // has roleA but not roleB
	}

	// New member state
	e := &gateway.GuildMemberUpdateEvent{
		User: discord.User{
			ID:     discord.UserID(12345),
			Bot:    false,
			Avatar: "new_avatar_hash",
		},
		RoleIDs: []discord.RoleID{discord.RoleID(333), discord.RoleID(443)}, // Gained roleB, should add target role
	}
	e.GuildID = discord.GuildID(111)

	svc.IngestGuildMemberUpdate(context.Background(), e, oldMember)

	// Verify avatar update and role update sinks called
	sink.mu.Lock()
	if sink.avatarUpdateGuildID != "111" || sink.oldAvatar != "old_avatar_hash" || sink.newAvatar != "new_avatar_hash" {
		t.Errorf("avatar update sink not called correctly")
	}
	if sink.roleUpdateGuildID != "111" || len(sink.addedRoles) != 1 || sink.addedRoles[0] != discord.RoleID(443) {
		t.Errorf("role update sink not called correctly")
	}
	sink.mu.Unlock()

	transport.mu.Lock()
	if transport.addRoleCalls != 1 {
		t.Errorf("expected 1 AddRole call, got %d", transport.addRoleCalls)
	}
	transport.mu.Unlock()

	// Now let's trigger role removal
	oldMember = &discord.Member{
		User: discord.User{
			ID:  discord.UserID(12345),
			Bot: false,
		},
		RoleIDs: []discord.RoleID{discord.RoleID(333), discord.RoleID(443), discord.RoleID(999)}, // has target role
	}

	e = &gateway.GuildMemberUpdateEvent{
		User: discord.User{
			ID:  discord.UserID(12345),
			Bot: false,
		},
		RoleIDs: []discord.RoleID{discord.RoleID(443), discord.RoleID(999)}, // Lost roleA, should remove target role
	}
	e.GuildID = discord.GuildID(111)

	svc.IngestGuildMemberUpdate(context.Background(), e, oldMember)

	transport.mu.Lock()
	if transport.removeRoleCalls != 1 {
		t.Errorf("expected 1 RemoveRole call, got %d", transport.removeRoleCalls)
	}
	transport.mu.Unlock()
}

func TestMemberEventService_CleanupJoinTimes(t *testing.T) {
	svc, _, _, _, _ := setupTestService(t)
	svc.joinTimes = make(map[string]time.Time)
	svc.joinTimes["k1"] = time.Now().Add(-8 * 24 * time.Hour) // older than 7 days
	svc.joinTimes["k2"] = time.Now().Add(-1 * time.Hour)      // recent

	svc.cleanupJoinTimes()

	svc.joinMu.RLock()
	defer svc.joinMu.RUnlock()
	if _, ok := svc.joinTimes["k1"]; ok {
		t.Errorf("expected k1 to be cleaned up")
	}
	if _, ok := svc.joinTimes["k2"]; !ok {
		t.Errorf("expected k2 to be preserved")
	}
}

func TestInMemoryMetrics(t *testing.T) {
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
	sink := NopMemberSink{}
	// None should panic
	sink.OnMemberJoin(context.Background(), nil, 0)
	sink.OnMemberLeave(context.Background(), nil, 0, 0)
	sink.OnRoleUpdate(context.Background(), "", discord.User{}, nil, nil)
	sink.OnAvatarUpdate(context.Background(), "", discord.User{}, "", "")
	sink.OnModerationAction(context.Background(), "", "", discord.User{}, "", discord.User{})
}

func TestNopMetrics(t *testing.T) {
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
	store := &files.MemoryConfigStore{}
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
