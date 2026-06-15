package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
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

func TestPermissionCheckerHasPermissionAllowsAdministratorRoleWithoutAllowedRoles(t *testing.T) {
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	session.State = discordgo.NewState()
	if err := session.State.GuildAdd(&discordgo.Guild{
		ID:      "g1",
		OwnerID: "owner-1",
		Roles: []*discordgo.Role{
			{ID: "g1", Permissions: 0},
			{ID: "admin-role", Permissions: discordgo.PermissionAdministrator},
		},
	}); err != nil {
		t.Fatalf("guild add: %v", err)
	}
	if err := session.State.MemberAdd(&discordgo.Member{
		GuildID: "g1",
		User:    &discordgo.User{ID: "user-1"},
		Roles:   []string{"admin-role"},
	}); err != nil {
		t.Fatalf("member add: %v", err)
	}

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if err := cfg.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	checker := NewPermissionChecker(session, cfg)
	if !checker.HasPermission("g1", "user-1") {
		t.Fatal("expected administrator role to grant command permission without configured allowed roles")
	}
}

func TestPermissionCheckerHasPermissionAllowsManageGuildWithoutAllowedRoles(t *testing.T) {
	session := newPermissionCheckerTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	session.State = discordgo.NewState()
	if err := session.State.GuildAdd(&discordgo.Guild{
		ID:      "g1",
		OwnerID: "owner-1",
		Roles: []*discordgo.Role{
			{ID: "g1", Permissions: 0},
			{ID: "manage-role", Permissions: discordgo.PermissionManageGuild},
		},
	}); err != nil {
		t.Fatalf("guild add: %v", err)
	}
	if err := session.State.MemberAdd(&discordgo.Member{
		GuildID: "g1",
		User:    &discordgo.User{ID: "user-2"},
		Roles:   []string{"manage-role"},
	}); err != nil {
		t.Fatalf("member add: %v", err)
	}

	cfg := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if err := cfg.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	checker := NewPermissionChecker(session, cfg)
	if !checker.HasPermission("g1", "user-2") {
		t.Fatal("expected manage guild role to grant command permission without configured allowed roles")
	}
}
