package core

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
	"github.com/small-frappuccino/discordgo"
)

func newPermissionCheckerWithCache(t *testing.T, session *discordgo.Session) (*PermissionChecker, *cache.UnifiedCache) {
	t.Helper()

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	checker := NewPermissionChecker(session, cfg)
	unifiedCache := cache.NewUnifiedCache(cache.DefaultCacheConfig())
	t.Cleanup(unifiedCache.Stop)
	checker.SetCache(unifiedCache)
	return checker, unifiedCache
}

func newPermissionCheckerStore(t *testing.T) *storage.Store {
	t.Helper()

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

	store, err := storage.NewStore(db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	if err := store.Init(); err != nil {
		t.Fatalf("store init: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestPermissionCheckerResolveMember_UsesCacheBeforeStateAndREST(t *testing.T) {
	var memberCalls int32
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1/members/u1") {
			atomic.AddInt32(&memberCalls, 1)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})
	session.State = discordgo.NewState()
	if err := session.State.GuildAdd(&discordgo.Guild{ID: "g1"}); err != nil {
		t.Fatalf("guild add: %v", err)
	}
	if err := session.State.MemberAdd(&discordgo.Member{
		GuildID: "g1",
		User:    &discordgo.User{ID: "u1", Username: "state-user"},
	}); err != nil {
		t.Fatalf("member add: %v", err)
	}

	checker, unifiedCache := newPermissionCheckerWithCache(t, session)
	unifiedCache.SetMember("g1", "u1", &discordgo.Member{
		GuildID: "g1",
		User:    &discordgo.User{ID: "u1", Username: "cache-user"},
	})

	member, ok, err := checker.ResolveMember("g1", "u1")
	if err != nil {
		t.Fatalf("resolve member: %v", err)
	}
	if !ok || member == nil || member.User == nil || member.User.Username != "cache-user" {
		t.Fatalf("expected cache member, got member=%+v ok=%v", member, ok)
	}
	if got := atomic.LoadInt32(&memberCalls); got != 0 {
		t.Fatalf("expected no REST call on cache hit, got %d", got)
	}
}

func TestPermissionCheckerResolveMember_UsesStateBeforeREST(t *testing.T) {
	var memberCalls int32
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1/members/u1") {
			atomic.AddInt32(&memberCalls, 1)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})
	session.State = discordgo.NewState()
	if err := session.State.GuildAdd(&discordgo.Guild{ID: "g1"}); err != nil {
		t.Fatalf("guild add: %v", err)
	}
	if err := session.State.MemberAdd(&discordgo.Member{
		GuildID: "g1",
		User:    &discordgo.User{ID: "u1", Username: "state-user"},
	}); err != nil {
		t.Fatalf("member add: %v", err)
	}

	checker, unifiedCache := newPermissionCheckerWithCache(t, session)

	member, ok, err := checker.ResolveMember("g1", "u1")
	if err != nil {
		t.Fatalf("resolve member: %v", err)
	}
	if !ok || member == nil || member.User == nil || member.User.Username != "state-user" {
		t.Fatalf("expected state member, got member=%+v ok=%v", member, ok)
	}
	if got := atomic.LoadInt32(&memberCalls); got != 0 {
		t.Fatalf("expected no REST call on state hit, got %d", got)
	}

	if cached, cachedOK := unifiedCache.GetMember("g1", "u1"); !cachedOK || cached == nil || cached.User == nil || cached.User.Username != "state-user" {
		t.Fatalf("expected member cached after state hit")
	}
}

func TestPermissionCheckerResolveMember_ReturnsErrorOnRESTFailure(t *testing.T) {
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1/members/u1") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom","code":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	checker := NewPermissionChecker(session, cfg)

	member, ok, err := checker.ResolveMember("g1", "u1")
	if err == nil {
		t.Fatalf("expected REST failure error")
	}
	if ok || member != nil {
		t.Fatalf("expected no member resolution on REST failure, member=%+v ok=%v", member, ok)
	}
}

func TestPermissionCheckerResolveMember_ReturnsNotFoundOnREST404(t *testing.T) {
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	checker := NewPermissionChecker(session, cfg)

	member, ok, err := checker.ResolveMember("g1", "u1")
	if err != nil {
		t.Fatalf("expected nil error on REST 404, got %v", err)
	}
	if ok || member != nil {
		t.Fatalf("expected member unresolved on REST 404, member=%+v ok=%v", member, ok)
	}
}

func TestPermissionCheckerResolveRoles_UsesCacheBeforeStateAndREST(t *testing.T) {
	var roleCalls int32
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1/roles") {
			atomic.AddInt32(&roleCalls, 1)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})
	session.State = discordgo.NewState()
	if err := session.State.GuildAdd(&discordgo.Guild{
		ID:    "g1",
		Roles: []*discordgo.Role{{ID: "r-state", Name: "State Role"}},
	}); err != nil {
		t.Fatalf("guild add: %v", err)
	}

	checker, unifiedCache := newPermissionCheckerWithCache(t, session)
	unifiedCache.SetRoles("g1", []*discordgo.Role{{ID: "r-cache", Name: "Cache Role"}})

	roles, err := checker.ResolveRoles("g1")
	if err != nil {
		t.Fatalf("resolve roles: %v", err)
	}
	if len(roles) != 1 || roles[0] == nil || roles[0].ID != "r-cache" {
		t.Fatalf("expected cache roles, got %+v", roles)
	}
	if got := atomic.LoadInt32(&roleCalls); got != 0 {
		t.Fatalf("expected no REST call on cache hit, got %d", got)
	}
}

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

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
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

func TestPermissionCheckerResolveRoles_ReturnsErrorOnRESTFailure(t *testing.T) {
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/guilds/g1/roles") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"boom","code":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	checker := NewPermissionChecker(session, cfg)

	roles, err := checker.ResolveRoles("g1")
	if err == nil {
		t.Fatalf("expected REST failure error")
	}
	if roles != nil {
		t.Fatalf("expected nil roles on REST failure, got %+v", roles)
	}
}

func TestPermissionCheckerResolveRoles_ReturnsNotFoundOnREST404(t *testing.T) {
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found","code":0}`))
	})

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	checker := NewPermissionChecker(session, cfg)

	roles, err := checker.ResolveRoles("g1")
	if err != nil {
		t.Fatalf("expected nil error on REST 404, got %v", err)
	}
	if roles != nil {
		t.Fatalf("expected nil roles on REST 404, got %+v", roles)
	}
}
