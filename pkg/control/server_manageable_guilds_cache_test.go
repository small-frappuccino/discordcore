package control

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestResolveManageableGuildsCachesDiscordLookup(t *testing.T) {
	t.Parallel()

	var guildRequests atomic.Int32
	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/@me/guilds":
			guildRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	session, err := srv.discordOAuth.sessions.Create(
		discordOAuthUser{ID: "u1", Username: "alice"},
		[]string{discordOAuthScopeIdentify, discordOAuthScopeGuilds},
		"access-token",
		"refresh-token",
		"Bearer",
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create oauth session: %v", err)
	}

	ctx := context.Background()
	first, err := srv.resolveManageableGuilds(ctx, session)
	if err != nil {
		t.Fatalf("first manageable guild lookup: %v", err)
	}
	second, err := srv.resolveManageableGuilds(ctx, session)
	if err != nil {
		t.Fatalf("second manageable guild lookup: %v", err)
	}

	if len(first) != 1 || first[0].ID != "g1" {
		t.Fatalf("unexpected first manageable guild set: %+v", first)
	}
	if len(second) != 1 || second[0].ID != "g1" {
		t.Fatalf("unexpected second manageable guild set: %+v", second)
	}
	if got := guildRequests.Load(); got != 1 {
		t.Fatalf("expected discord guild endpoint to be called once, got %d", got)
	}
}

func TestResolveManageableGuildsCacheExpires(t *testing.T) {
	t.Parallel()

	var guildRequests atomic.Int32
	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/@me/guilds":
			guildRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.accessibleGuildCache.SetTTL(5 * time.Millisecond)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	session, err := srv.discordOAuth.sessions.Create(
		discordOAuthUser{ID: "u1", Username: "alice"},
		[]string{discordOAuthScopeIdentify, discordOAuthScopeGuilds},
		"access-token",
		"refresh-token",
		"Bearer",
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create oauth session: %v", err)
	}

	ctx := context.Background()
	if _, err := srv.resolveManageableGuilds(ctx, session); err != nil {
		t.Fatalf("first manageable guild lookup: %v", err)
	}

	time.Sleep(12 * time.Millisecond)

	if _, err := srv.resolveManageableGuilds(ctx, session); err != nil {
		t.Fatalf("second manageable guild lookup after ttl: %v", err)
	}

	if got := guildRequests.Load(); got != 2 {
		t.Fatalf("expected discord guild endpoint to be called twice after ttl expiry, got %d", got)
	}
}

func TestResolveAccessibleGuildsRecomputesDiscordRoleAccessOnCacheHit(t *testing.T) {
	t.Parallel()

	var guildRequests atomic.Int32
	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/users/@me/guilds":
			guildRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":false,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, cm := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})

	if _, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		for index := range cfg.Guilds {
			if cfg.Guilds[index].GuildID != "g1" {
				continue
			}
			cfg.Guilds[index].Roles.DashboardWrite = []string{"writer-role"}
			return nil
		}
		return nil
	}); err != nil {
		t.Fatalf("seed writable dashboard role config: %v", err)
	}

	currentSession := newTestDiscordSessionWithGuildMembers("g1",
		&discordgo.Member{
			User:  &discordgo.User{ID: "u1", Username: "alice"},
			Roles: nil,
		},
	)
	srv.SetDiscordSessionProvider(func() *discordgo.Session {
		return currentSession
	})

	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	session, err := srv.discordOAuth.sessions.Create(
		discordOAuthUser{ID: "u1", Username: "alice"},
		[]string{discordOAuthScopeIdentify, discordOAuthScopeGuilds},
		"access-token",
		"refresh-token",
		"Bearer",
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create oauth session: %v", err)
	}

	ctx := context.Background()
	first, err := srv.resolveAccessibleGuilds(ctx, session)
	if err != nil {
		t.Fatalf("first accessible guild lookup: %v", err)
	}
	if len(first) != 0 {
		t.Fatalf("expected no accessible guilds before discord role grant, got %+v", first)
	}

	currentSession = newTestDiscordSessionWithGuildMembers("g1",
		&discordgo.Member{
			User:  &discordgo.User{ID: "u1", Username: "alice"},
			Roles: []string{"writer-role"},
		},
	)

	second, err := srv.resolveAccessibleGuilds(ctx, session)
	if err != nil {
		t.Fatalf("second accessible guild lookup: %v", err)
	}
	if len(second) != 1 || second[0].ID != "g1" || second[0].AccessLevel != guildAccessLevelWrite {
		t.Fatalf("expected writable guild after discord role grant on cache hit, got %+v", second)
	}
	if got := guildRequests.Load(); got != 1 {
		t.Fatalf("expected discord guild endpoint to be called once across cache hit, got %d", got)
	}
}
