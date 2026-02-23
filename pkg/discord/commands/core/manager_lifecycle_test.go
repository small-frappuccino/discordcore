package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

	cfgMgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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

	cfgMgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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

	cfgMgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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

	cfgMgr := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
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
