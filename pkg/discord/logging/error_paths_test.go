package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

func newDiscordSessionWithAPI(t *testing.T, handler http.HandlerFunc) *discordgo.Session {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	oldApplications := discordgo.EndpointApplications
	oldGuilds := discordgo.EndpointGuilds
	oldChannels := discordgo.EndpointChannels
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	discordgo.EndpointApplications = discordgo.EndpointAPI + "applications"
	discordgo.EndpointGuilds = discordgo.EndpointAPI + "guilds/"
	discordgo.EndpointChannels = discordgo.EndpointAPI + "channels/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
		discordgo.EndpointApplications = oldApplications
		discordgo.EndpointGuilds = oldGuilds
		discordgo.EndpointChannels = oldChannels
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	return session
}

func TestEventTimestampPersistenceErrorBranches(t *testing.T) {
	failingStore := storage.NewStore(nil)

	memberService := NewMemberEventService(nil, nil, nil, failingStore)
	messageService := NewMessageEventService(nil, nil, nil, failingStore)
	monitoringService := &MonitoringService{
		store:    failingStore,
		activity: newMonitoringRuntimeActivity(failingStore),
	}

	memberService.markEvent(context.Background())
	messageService.markEvent(context.Background())
	monitoringService.markEvent(context.Background())
}

func TestMonitoringServiceStartHeartbeatPersistenceErrorBranch(t *testing.T) {
	failingStore := storage.NewStore(nil)
	ms := &MonitoringService{
		store:    failingStore,
		stopChan: make(chan struct{}),
		activity: newMonitoringRuntimeActivity(failingStore),
	}

	ms.startHeartbeat(context.Background())
	if ms.activity == nil {
		t.Fatalf("expected runtime activity to be initialized")
	}
	if ms.activity.hbCancel == nil {
		t.Fatalf("expected heartbeat cancel function to be initialized")
	}
	if ms.activity.hbDone == nil {
		t.Fatalf("expected heartbeat done channel to be initialized")
	}
	close(ms.stopChan)
	if err := ms.stopHeartbeat(context.Background()); err != nil {
		t.Fatalf("stop heartbeat: %v", err)
	}
}

func TestMaybeRestoreBotRolePermissionsLogsEditError(t *testing.T) {
	const guildID = "g1"
	const roleID = "r1"

	var roleEditCalls int32
	session := newDiscordSessionWithAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		roleListPath := fmt.Sprintf("/guilds/%s/roles", guildID)
		roleEditPath := fmt.Sprintf("/guilds/%s/roles/%s", guildID, roleID)

		switch {
		case r.Method == http.MethodGet && r.URL.Path == roleListPath:
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"id":          roleID,
					"name":        "managed-role",
					"managed":     true,
					"permissions": "8",
				},
			})
		case r.Method == http.MethodPatch && r.URL.Path == roleEditPath:
			atomic.AddInt32(&roleEditCalls, 1)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"forced role edit failure"}`))
		default:
			// Keep handler permissive for other paths used by discordgo internals.
			if strings.Contains(r.URL.Path, "/roles") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[]`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}
	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	ms := &MonitoringService{
		session: session,
		store:   store,
	}

	ms.saveBotRolePermSnapshot(guildID, roleID, 8, "tester")
	ms.maybeRestoreBotRolePermissions(guildID, roleID, 4)

	if got := atomic.LoadInt32(&roleEditCalls); got != 1 {
		t.Fatalf("expected one role edit attempt, got %d", got)
	}
}
