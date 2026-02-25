package logging

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func newMonitoringRoleLookupTestSession(t *testing.T, handler http.HandlerFunc) *discordgo.Session {
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

func TestMonitoringRoleLookupFunctions(t *testing.T) {
	session := newMonitoringRoleLookupTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1/roles") {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": "r1", "name": "regular", "managed": false, "permissions": "0", "position": 1},
				{"id": "r2", "name": "managed", "managed": true, "permissions": "0", "position": 2},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})

	ms := &MonitoringService{session: session}

	if ms.isBotManagedRole("g1", "r1") {
		t.Fatalf("expected r1 to be non-managed")
	}
	if !ms.isBotManagedRole("g1", "r2") {
		t.Fatalf("expected r2 to be managed")
	}
	if ms.isBotManagedRole("g1", "") {
		t.Fatalf("expected empty role id to return false")
	}

	role, ok := ms.getRoleByID("g1", "r1")
	if !ok || role == nil || role.ID != "r1" {
		t.Fatalf("expected to find role r1, got ok=%v role=%v", ok, role)
	}
	if _, ok := ms.getRoleByID("g1", "missing"); ok {
		t.Fatalf("expected missing role to return ok=false")
	}

	managedRole, ok := ms.findBotManagedRole("g1")
	if !ok || managedRole == nil || managedRole.ID != "r2" {
		t.Fatalf("expected managed role r2, got ok=%v role=%v", ok, managedRole)
	}

	if _, ok := ms.findGuildRole("g1", nil); ok {
		t.Fatalf("expected nil matcher to return ok=false")
	}
}
