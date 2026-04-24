package cache

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

type memberResponse struct {
	User     *discordgo.User `json:"user"`
	Roles    []string        `json:"roles,omitempty"`
	JoinedAt time.Time       `json:"joined_at,omitempty"`
}

func setupMemberServer(t *testing.T, handler func(guildID, userID string) (memberResponse, int)) (*httptest.Server, func()) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/guilds/") {
			http.NotFound(w, r)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/guilds/"), "/")
		if len(parts) != 3 || parts[1] != "members" {
			http.NotFound(w, r)
			return
		}
		guildID := parts[0]
		userID := parts[2]

		resp, code := handler(guildID, userID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		if code >= 200 && code < 300 {
			_ = json.NewEncoder(w).Encode(resp)
		}
	}))

	oldGuilds := discordgo.EndpointGuilds
	discordgo.EndpointGuilds = srv.URL + path.Clean("/guilds/") + "/"
	restore := func() {
		discordgo.EndpointGuilds = oldGuilds
		srv.Close()
	}
	return srv, restore
}

func newTestStore(t *testing.T) *storage.Store {
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

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("store init: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestRefreshMemberDataUpdatesCacheAndStore(t *testing.T) {
	joinedAt := time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)
	srv, restore := setupMemberServer(t, func(guildID, userID string) (memberResponse, int) {
		return memberResponse{
			User:     &discordgo.User{ID: userID},
			Roles:    []string{"r1", "r2"},
			JoinedAt: joinedAt,
		}, http.StatusOK
	})
	defer restore()

	session, err := discordgo.New("")
	if err != nil {
		t.Fatalf("session init: %v", err)
	}
	session.Client = srv.Client()

	cache := newTestCache(t)
	store := newTestStore(t)

	if err := RefreshMemberData(session, cache, store, "g1", []string{"u1"}); err != nil {
		t.Fatalf("RefreshMemberData error: %v", err)
	}

	if got, ok := cache.GetMember("g1", "u1"); !ok || got == nil || got.User.ID != "u1" {
		t.Fatalf("expected cached member, got %v %v", got, ok)
	}

	gotJoin, ok, err := store.MemberJoin(context.Background(), "g1", "u1")
	if err != nil {
		t.Fatalf("GetMemberJoin error: %v", err)
	}
	if !ok || !gotJoin.Equal(joinedAt) {
		t.Fatalf("expected join time %v, got %v (ok=%v)", joinedAt, gotJoin, ok)
	}

	gotRoles, err := store.GetMemberRoles("g1", "u1")
	if err != nil {
		t.Fatalf("GetMemberRoles error: %v", err)
	}
	if len(gotRoles) != 2 {
		t.Fatalf("expected 2 roles persisted, got %v", gotRoles)
	}
	roleSet := map[string]struct{}{}
	for _, roleID := range gotRoles {
		roleSet[roleID] = struct{}{}
	}
	if _, ok := roleSet["r1"]; !ok {
		t.Fatalf("expected r1 persisted, got %v", gotRoles)
	}
	if _, ok := roleSet["r2"]; !ok {
		t.Fatalf("expected r2 persisted, got %v", gotRoles)
	}
}

func TestRefreshMemberDataSkipsFailures(t *testing.T) {
	srv, restore := setupMemberServer(t, func(guildID, userID string) (memberResponse, int) {
		if userID == "bad" {
			return memberResponse{}, http.StatusInternalServerError
		}
		return memberResponse{
			User:  &discordgo.User{ID: userID},
			Roles: []string{"r1"},
		}, http.StatusOK
	})
	defer restore()

	session, err := discordgo.New("")
	if err != nil {
		t.Fatalf("session init: %v", err)
	}
	session.Client = srv.Client()

	cache := newTestCache(t)
	store := newTestStore(t)

	if err := RefreshMemberData(session, cache, store, "g1", []string{"bad", "good"}); err != nil {
		t.Fatalf("RefreshMemberData error: %v", err)
	}

	if got, ok := cache.GetMember("g1", "bad"); ok || got != nil {
		t.Fatalf("expected bad member not cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetMember("g1", "good"); !ok || got == nil || got.User.ID != "good" {
		t.Fatalf("expected good member cached, got %v %v", got, ok)
	}
}

func TestWarmupGuildMembersPreservesHistoricalJoin(t *testing.T) {
	store := newTestStore(t)
	cache := newTestCache(t)

	historicalJoin := time.Date(2024, 6, 12, 15, 0, 0, 0, time.UTC)
	if err := store.UpsertMemberJoin("g1", "u1", historicalJoin); err != nil {
		t.Fatalf("seed historical join: %v", err)
	}

	session := &funcWarmupSession{
		membersFunc: func(string, string, int, ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			return []*discordgo.Member{
				{
					User:     &discordgo.User{ID: "u1"},
					JoinedAt: historicalJoin.Add(48 * time.Hour),
				},
			}, nil
		},
	}

	gotCount, err := warmupGuildMembers(session, cache, store, "g1", 1)
	if err != nil {
		t.Fatalf("warmupGuildMembers error: %v", err)
	}
	if gotCount != 1 {
		t.Fatalf("expected 1 cached member, got %d", gotCount)
	}

	gotJoin, ok, err := store.MemberJoin(context.Background(), "g1", "u1")
	if err != nil {
		t.Fatalf("GetMemberJoin error: %v", err)
	}
	if !ok || !gotJoin.Equal(historicalJoin) {
		t.Fatalf("expected historical join %v, got %v (ok=%v)", historicalJoin, gotJoin, ok)
	}
}

func TestKeepMemberDataFreshPreservesHistoricalJoin(t *testing.T) {
	store := newTestStore(t)

	historicalJoin := time.Date(2023, 10, 3, 11, 0, 0, 0, time.UTC)
	if err := store.UpsertMemberJoin("g1", "u1", historicalJoin); err != nil {
		t.Fatalf("seed historical join: %v", err)
	}

	if err := KeepMemberDataFresh(store, "g1", []string{"u1"}); err != nil {
		t.Fatalf("KeepMemberDataFresh error: %v", err)
	}

	gotJoin, ok, err := store.MemberJoin(context.Background(), "g1", "u1")
	if err != nil {
		t.Fatalf("GetMemberJoin error: %v", err)
	}
	if !ok || !gotJoin.Equal(historicalJoin) {
		t.Fatalf("expected historical join %v, got %v (ok=%v)", historicalJoin, gotJoin, ok)
	}
}
