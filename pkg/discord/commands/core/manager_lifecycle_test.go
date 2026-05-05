package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func newCommandManagerSession(t *testing.T, handler http.HandlerFunc) *discordgo.Session {
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
	session.State = discordgo.NewState()
	session.State.User = &discordgo.User{ID: "app-id"}
	return session
}

func TestCommandManagerSetupAndShutdownHandlerLifecycle(t *testing.T) {
	session := newCommandManagerSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands") {
			_ = json.NewEncoder(w).Encode([]map[string]any{})
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	cfgMgr := files.NewMemoryConfigManager()
	cm := NewCommandManager(session, cfgMgr)

	if err := cm.SetupCommands(); err != nil {
		t.Fatalf("first setup: %v", err)
	}
	if cm.interactionHandlerCancel == nil {
		t.Fatalf("expected interaction handler cancel function to be set")
	}

	// Re-run setup to exercise the reinit branch that cancels an existing handler first.
	if err := cm.SetupCommands(); err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if cm.interactionHandlerCancel == nil {
		t.Fatalf("expected interaction handler cancel function to remain set after reinit")
	}

	if err := cm.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if cm.interactionHandlerCancel != nil {
		t.Fatalf("expected interaction handler cancel function to be cleared")
	}

	// Idempotent shutdown.
	if err := cm.Shutdown(); err != nil {
		t.Fatalf("second shutdown: %v", err)
	}
}

func TestCommandManagerSetupCommandsRollbackOnFetchError(t *testing.T) {
	session := newCommandManagerSession(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"forced failure"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})

	cfgMgr := files.NewMemoryConfigManager()
	cm := NewCommandManager(session, cfgMgr)

	err := cm.SetupCommands()
	if err == nil {
		t.Fatalf("expected setup error when command fetch fails")
	}
	if !strings.Contains(err.Error(), "failed to fetch registered commands") {
		t.Fatalf("unexpected setup error: %v", err)
	}
	if cm.interactionHandlerCancel != nil {
		t.Fatalf("expected interaction handler cancel to be cleared on rollback")
	}
}

func TestCommandManagerSetupCommandsRequiresSessionUser(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.State = discordgo.NewState()
	session.State.User = nil

	cfgMgr := files.NewMemoryConfigManager()
	cm := NewCommandManager(session, cfgMgr)

	err = cm.SetupCommands()
	if err == nil {
		t.Fatalf("expected setup error when session user is missing")
	}
	if got, want := err.Error(), "session not properly initialized"; got != want {
		t.Fatalf("unexpected setup error: got %q, want %q", got, want)
	}
	if cm.interactionHandlerCancel != nil {
		t.Fatalf("handler cancel should remain nil when setup precondition fails")
	}

	if err := cm.Shutdown(); err != nil {
		t.Fatalf("shutdown after precondition failure: %v", err)
	}
}

func TestCommandManagerSetupCommandsRollbackOnCreateError(t *testing.T) {
	session := newCommandManagerSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"forced create failure"}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewMemoryConfigManager()
	cm := NewCommandManager(session, cfgMgr)
	cm.GetRouter().RegisterCommand(testCommand{name: "ping"})

	err := cm.SetupCommands()
	if err == nil {
		t.Fatalf("expected setup error when command create fails")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("error creating command '%s'", "ping")) {
		t.Fatalf("unexpected setup error: %v", err)
	}
	if cm.interactionHandlerCancel != nil {
		t.Fatalf("expected interaction handler cancel to be cleared on rollback")
	}
}

func TestCommandManagerSetupCommandsUsesGlobalSyncWithoutDomainOverrides(t *testing.T) {
	requestPaths := make([]string, 0, 4)
	session := newCommandManagerSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		requestPaths = append(requestPaths, r.Method+" "+r.URL.Path)

		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
			var posted discordgo.ApplicationCommand
			if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
				t.Fatalf("decode posted global command: %v", err)
			}
			if posted.ID == "" {
				posted.ID = "created-id"
			}
			_ = json.NewEncoder(w).Encode(&posted)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	})

	cfgMgr := files.NewMemoryConfigManager()
	cm := NewCommandManager(session, cfgMgr)
	cm.GetRouter().RegisterCommand(testCommand{name: "ping"})

	if err := cm.SetupCommands(); err != nil {
		t.Fatalf("setup commands: %v", err)
	}

	if len(requestPaths) == 0 {
		t.Fatal("expected global sync requests")
	}
	for _, path := range requestPaths {
		if strings.Contains(path, "/guilds/") {
			t.Fatalf("expected legacy sync to avoid guild-scoped endpoints, got %q", path)
		}
	}
}

func TestCommandManagerSetupCommandsUsesGuildSyncWhenDomainOverridesExist(t *testing.T) {
	tests := []struct {
		name                  string
		allowRoute            func(routeKey InteractionRouteKey) bool
		wantConfigSubcommands []string
		wantTopLevelCommands  []string
	}{
		{
			name: "base app",
			allowRoute: func(routeKey InteractionRouteKey) bool {
				return routeKey.Path == "config commands_enabled"
			},
			wantConfigSubcommands: []string{"commands_enabled"},
			wantTopLevelCommands:  []string{"config"},
		},
		{
			name: "qotd app",
			allowRoute: func(routeKey InteractionRouteKey) bool {
				return routeKey.Path == "config qotd_channel" || routeKey.Path == "qotd"
			},
			wantConfigSubcommands: []string{"qotd_channel"},
			wantTopLevelCommands:  []string{"config", "qotd"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			globalDeletes := 0
			guildFetches := 0
			postedGuildCommands := make([]discordgo.ApplicationCommand, 0, 2)
			session := newCommandManagerSession(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/commands"):
					_ = json.NewEncoder(w).Encode([]map[string]any{{
						"id":          "legacy-global",
						"name":        "legacy-global",
						"description": "legacy global command",
					}})
				case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/applications/app-id/commands/legacy-global"):
					globalDeletes++
					w.WriteHeader(http.StatusNoContent)
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/applications/app-id/guilds/g1/commands"):
					guildFetches++
					_ = json.NewEncoder(w).Encode([]map[string]any{})
				case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/applications/app-id/guilds/g1/commands"):
					var posted discordgo.ApplicationCommand
					if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
						t.Fatalf("decode posted guild command: %v", err)
					}
					postedGuildCommands = append(postedGuildCommands, posted)
					if posted.ID == "" {
						posted.ID = posted.Name + "-id"
					}
					_ = json.NewEncoder(w).Encode(&posted)
				default:
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{}`))
				}
			})

			cfgMgr := files.NewMemoryConfigManager()
			if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
				cfg.Guilds = []files.GuildConfig{{
					GuildID:       "g1",
					BotInstanceID: "alice",
					DomainBotInstanceIDs: map[string]string{
						files.BotDomainQOTD: "companion",
					},
				}}
				return nil
			}); err != nil {
				t.Fatalf("seed config: %v", err)
			}

			cm := NewCommandManager(session, cfgMgr)
			router := cm.GetRouter()
			checker := NewPermissionChecker(session, cfgMgr)

			configGroup := NewGroupCommand("config", "config", checker)
			baseSubcommand := testSubCommand{name: "commands_enabled"}
			configGroup.AddSubCommand(baseSubcommand)
			router.RegisterSlashCommand(configGroup)

			qotdSubcommand := testSubCommand{name: "qotd_channel"}
			configGroup.AddSubCommand(qotdSubcommand)
			router.RegisterSlashSubCommandForDomain(files.BotDomainQOTD, "config", qotdSubcommand)
			router.RegisterSlashCommandForDomain(files.BotDomainQOTD, testCommand{name: "qotd"})
			router.SetGuildRouteFilter(func(guildID string, routeKey InteractionRouteKey) bool {
				return guildID == "g1" && tc.allowRoute(routeKey)
			})

			if err := cm.SetupCommands(); err != nil {
				t.Fatalf("setup commands: %v", err)
			}

			if globalDeletes != 1 {
				t.Fatalf("expected global command cleanup once, got %d", globalDeletes)
			}
			if guildFetches != 1 {
				t.Fatalf("expected exactly one guild command fetch, got %d", guildFetches)
			}

			gotNames := make([]string, 0, len(postedGuildCommands))
			configOptionNames := make([]string, 0, 2)
			for _, posted := range postedGuildCommands {
				gotNames = append(gotNames, posted.Name)
				if posted.Name == "config" {
					for _, option := range posted.Options {
						if option != nil {
							configOptionNames = append(configOptionNames, option.Name)
						}
					}
				}
			}
			sort.Strings(gotNames)
			sort.Strings(configOptionNames)

			if strings.Join(gotNames, ",") != strings.Join(tc.wantTopLevelCommands, ",") {
				t.Fatalf("unexpected posted guild commands: got=%v want=%v", gotNames, tc.wantTopLevelCommands)
			}
			if strings.Join(configOptionNames, ",") != strings.Join(tc.wantConfigSubcommands, ",") {
				t.Fatalf("unexpected config subcommands for guild sync: got=%v want=%v", configOptionNames, tc.wantConfigSubcommands)
			}
		})
	}
}
