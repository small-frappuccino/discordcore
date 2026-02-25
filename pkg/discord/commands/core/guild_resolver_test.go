package core

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestPermissionCheckerResolveRoles_UsesStateAndCache(t *testing.T) {
	session := &discordgo.Session{State: discordgo.NewState()}
	guild := &discordgo.Guild{
		ID:      "g1",
		OwnerID: "owner-1",
		Roles: []*discordgo.Role{
			{ID: "r1", Name: "Role 1", Position: 1},
		},
	}
	if err := session.State.GuildAdd(guild); err != nil {
		t.Fatalf("guild add: %v", err)
	}

	cfg := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	checker := NewPermissionChecker(session, cfg)

	unifiedCache := cache.NewUnifiedCache(cache.DefaultCacheConfig())
	t.Cleanup(unifiedCache.Stop)
	checker.SetCache(unifiedCache)

	roles, err := checker.ResolveRoles("g1")
	if err != nil {
		t.Fatalf("resolve roles from state: %v", err)
	}
	if len(roles) != 1 || roles[0] == nil || roles[0].ID != "r1" {
		t.Fatalf("unexpected roles from state: %+v", roles)
	}

	if cached, ok := unifiedCache.GetRoles("g1"); !ok || len(cached) != 1 || cached[0] == nil || cached[0].ID != "r1" {
		t.Fatalf("expected roles cached after state hit")
	}

	// Force second lookup to rely on cache by removing roles from state.
	guild.Roles = nil

	roles, err = checker.ResolveRoles("g1")
	if err != nil {
		t.Fatalf("resolve roles from cache: %v", err)
	}
	if len(roles) != 1 || roles[0] == nil || roles[0].ID != "r1" {
		t.Fatalf("unexpected roles from cache: %+v", roles)
	}
}

func TestPermissionCheckerResolveOwnerID_ReturnsErrorOnRESTFailure(t *testing.T) {
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom","code":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})

	cfg := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	checker := NewPermissionChecker(session, cfg)

	ownerID, ok, err := checker.ResolveOwnerID("g1")
	if err == nil {
		t.Fatalf("expected REST failure error")
	}
	if ok {
		t.Fatalf("expected owner not found when REST fails")
	}
	if ownerID != "" {
		t.Fatalf("expected empty owner ID, got %q", ownerID)
	}
}
