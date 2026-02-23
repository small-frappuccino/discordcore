package logging

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage"
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
	failingStore := storage.NewStore(filepath.Join(t.TempDir(), "not-initialized.db"))

	memberService := NewMemberEventService(nil, nil, nil, failingStore)
	messageService := NewMessageEventService(nil, nil, nil, failingStore)
	monitoringService := &MonitoringService{store: failingStore}

	memberService.markEvent()
	messageService.markEvent()
	monitoringService.markEvent()
}

func TestMonitoringServiceStartHeartbeatPersistenceErrorBranch(t *testing.T) {
	failingStore := storage.NewStore(filepath.Join(t.TempDir(), "heartbeat-not-initialized.db"))
	ms := &MonitoringService{
		store:    failingStore,
		stopChan: make(chan struct{}),
	}

	ms.startHeartbeat()
	if ms.heartbeatTicker == nil {
		t.Fatalf("expected heartbeat ticker to be initialized")
	}
	if ms.heartbeatStop == nil {
		t.Fatalf("expected heartbeat stop channel to be initialized")
	}
	close(ms.stopChan)
	if ms.heartbeatTicker != nil {
		ms.heartbeatTicker.Stop()
	}
	if ms.heartbeatStop != nil {
		close(ms.heartbeatStop)
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

	store := storage.NewStore(filepath.Join(t.TempDir(), "monitoring.db"))
	if err := store.Init(); err != nil {
		t.Fatalf("init sqlite store: %v", err)
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
