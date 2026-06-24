package members

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/handler"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/config"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/system"
)

type mockTransport struct{}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	if req.Method == "GET" && strings.Contains(path, "/members/") {
		memberJSON := `{
			"user": {
				"id": "999",
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
		meJSON := `{
			"id": "123456789",
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

type mockMembersRepo struct {
	members.Repository
	upsertJoinCalled atomic.Bool
	wg               *sync.WaitGroup
}

func (m *mockMembersRepo) UpsertMemberJoinContext(ctx context.Context, guildID, userID string, joinedAt time.Time) error {
	m.upsertJoinCalled.Store(true)
	if m.wg != nil {
		m.wg.Done()
	}
	return nil
}

func (m *mockMembersRepo) MemberJoin(ctx context.Context, guildID, userID string) (time.Time, bool, error) {
	return time.Time{}, false, nil
}

type mockSystemRepo struct {
	system.Repository
	joinIncrCalled  atomic.Bool
	leaveIncrCalled atomic.Bool
	joinWg          *sync.WaitGroup
	leaveWg         *sync.WaitGroup
}

func (m *mockSystemRepo) IncrementDailyMemberJoinContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	m.joinIncrCalled.Store(true)
	if m.joinWg != nil {
		m.joinWg.Done()
	}
	return nil
}

func (m *mockSystemRepo) IncrementDailyMemberLeaveContext(ctx context.Context, guildID, userID string, timestamp time.Time) error {
	m.leaveIncrCalled.Store(true)
	if m.leaveWg != nil {
		m.leaveWg.Done()
	}
	return nil
}

func (m *mockSystemRepo) SetLastEventForBot(ctx context.Context, instanceID string, t time.Time) error {
	return nil
}

func TestGatewayListener_Lifecycle(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stateVal := state.New("Bot Token")
	stateVal.PreHandler = handler.New()
	stateVal.Client.Client.Client = httpdriver.WrapClient(http.Client{Transport: &mockTransport{}})

	// Config manager setup
	storeConfig := &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "12345",
				Channels: files.ChannelsConfig{
					MemberJoin:  "67890",
					MemberLeave: "67890",
				},
			},
		},
	}
	store := &config.MemoryConfigStore{}
	_ = store.Save(storeConfig)
	configMgr := files.NewConfigManagerWithStore(store, logger)
	_ = configMgr.LoadConfig()

	// Member Event Service
	var wg sync.WaitGroup
	wg.Add(3)

	membersRepo := &mockMembersRepo{wg: &wg}
	systemRepo := &mockSystemRepo{joinWg: &wg, leaveWg: &wg}
	memberSvc := members.NewMemberEventServiceForBot(members.EventServiceDeps{
		ConfigManager:  configMgr,
		Sink:           members.NopMemberSink{},
		MembersRepo:    membersRepo,
		SystemRepo:     systemRepo,
		BotInstanceID:  "",
		Logger:         logger,
		DiscordAdapter: NewArikawaAdapter(stateVal),
	})

	_ = memberSvc.Start(context.Background())
	defer memberSvc.Stop(context.Background())

	listener := NewGatewayListener(stateVal, memberSvc)

	// Test service metadata implementation
	if listener.Name() != "discord_members_listener" {
		t.Errorf("unexpected name: %s", listener.Name())
	}
	if listener.Type() != service.ServiceType("gateway_listener") {
		t.Errorf("unexpected type: %s", listener.Type())
	}
	if listener.Priority() != service.PriorityNormal {
		t.Errorf("unexpected priority")
	}
	if len(listener.Dependencies()) != 1 || listener.Dependencies()[0] != "members" {
		t.Errorf("unexpected dependencies: %v", listener.Dependencies())
	}
	if listener.IsRunning() {
		t.Error("expected IsRunning to be false before Start")
	}

	health := listener.HealthCheck(context.Background())
	if !health.Healthy {
		t.Error("expected healthy listener")
	}
	_ = listener.Stats()

	// Start
	err := listener.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error starting listener: %v", err)
	}
	if !listener.IsRunning() {
		t.Error("expected IsRunning to be true after Start")
	}

	// Trigger GuildMemberAddEvent
	stateVal.Call(&gateway.GuildMemberAddEvent{
		GuildID: discord.GuildID(12345),
		Member: discord.Member{
			User: discord.User{
				ID:       discord.UserID(999),
				Username: "testuser",
				Bot:      false,
			},
			Joined: discord.Timestamp(time.Now()),
		},
	})

	// Trigger GuildMemberRemoveEvent
	stateVal.Call(&gateway.GuildMemberRemoveEvent{
		GuildID: discord.GuildID(12345),
		User: discord.User{
			ID:       discord.UserID(999),
			Username: "testuser",
			Bot:      false,
		},
	})

	// Wait for asynchronous event handlers to process
	wg.Wait()

	// Stop
	err = listener.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error stopping listener: %v", err)
	}
	if listener.IsRunning() {
		t.Error("expected IsRunning to be false after Stop")
	}

	// Check if mock repos were invoked
	if !membersRepo.upsertJoinCalled.Load() {
		t.Error("expected membersRepo.UpsertMemberJoinContext to be called")
	}
	if !systemRepo.joinIncrCalled.Load() {
		t.Error("expected systemRepo.IncrementDailyMemberJoinContext to be called")
	}
	if !systemRepo.leaveIncrCalled.Load() {
		t.Error("expected systemRepo.IncrementDailyMemberLeaveContext to be called")
	}
}
