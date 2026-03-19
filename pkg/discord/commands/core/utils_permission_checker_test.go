package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func newPermissionCheckerTestSession(t *testing.T, handler http.HandlerFunc) *discordgo.Session {
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

func TestPermissionCheckerGetOwnerID_StoreWriteFailureKeepsRESTFallbackAndCache(t *testing.T) {
	var guildCalls int32
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1") {
			atomic.AddInt32(&guildCalls, 1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":       "g1",
				"name":     "Guild 1",
				"owner_id": "owner-1",
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})

	cfg := files.NewMemoryConfigManager()
	checker := NewPermissionChecker(session, cfg)

	// Intentionally uninitialized store to force Get/SetGuildOwnerID errors.
	checker.SetStore(storage.NewStore(nil))

	unifiedCache := cache.NewUnifiedCache(cache.DefaultCacheConfig())
	t.Cleanup(unifiedCache.Stop)
	checker.SetCache(unifiedCache)

	ownerID, ok, err := checker.ResolveOwnerID("g1")
	if err != nil {
		t.Fatalf("resolve owner id: %v", err)
	}
	if !ok {
		t.Fatalf("expected owner resolution success")
	}
	if ownerID != "owner-1" {
		t.Fatalf("expected owner-1, got %q", ownerID)
	}

	ownerID, ok, err = checker.ResolveOwnerID("g1")
	if err != nil {
		t.Fatalf("resolve owner id from cache: %v", err)
	}
	if !ok {
		t.Fatalf("expected cached owner resolution success")
	}
	if ownerID != "owner-1" {
		t.Fatalf("expected owner-1 on cached read, got %q", ownerID)
	}

	if got := atomic.LoadInt32(&guildCalls); got != 1 {
		t.Fatalf("expected one REST guild lookup, got %d", got)
	}
}
