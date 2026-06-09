package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// TestNotifyLifecycleEventDoesNothingWithoutWebhookURL pins that the
// lifecycle path is opt-in: with the env var unset, notifyLifecycleEvent
// must be a no-op (no panic, no log spam). Forgetting this means every
// dev environment ships startup/shutdown warnings.
func TestNotifyLifecycleEventDoesNothingWithoutWebhookURL(t *testing.T) {
	t.Setenv(lifecycleWebhookEnv, "")
	notifyLifecycleEvent("starting", "")
}

// TestNotifyLifecycleEventPostsToConfiguredURL exercises the happy path
// end-to-end via httptest: confirm that the body is JSON with the
// expected content field, and that the helper actually executes the POST
// (not just builds the request). Without this test, a refactor could
// silently disable the entire notification path and only be caught when
// a real outage went unannounced.
func TestNotifyLifecycleEventPostsToConfiguredURL(t *testing.T) {
	origAppName := files.ConfiguredAppName
	origAppVersion := files.AppVersion
	origBotName := files.DiscordBotName
	t.Cleanup(func() {
		files.ConfiguredAppName = origAppName
		files.AppVersion = origAppVersion
		files.DiscordBotName = origBotName
	})
	files.ConfiguredAppName = "discordmain"
	files.AppVersion = "v0.test"
	files.DiscordBotName = "TestBot"

	var (
		mu       sync.Mutex
		received []map[string]string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("expected application/json content type, got %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		var payload map[string]string
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Errorf("decode body: %v raw=%q", err, string(body))
			return
		}
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	notifyLifecycleEvent("starting", "")
	notifyLifecycleEvent("fatal", "nil pointer dereference")

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 webhook posts, got %d", len(received))
	}
	if got := received[0]["content"]; !strings.Contains(got, "discordmain") || !strings.Contains(got, "starting") {
		t.Fatalf("first content missing app/reason: %q", got)
	}
	if got := received[1]["content"]; !strings.Contains(got, "fatal") || !strings.Contains(got, "nil pointer") {
		t.Fatalf("second content missing reason/detail: %q", got)
	}
}

// TestBuildLifecycleContentFormat pins the human-readable shape an
// operator sees in chat. Order matters (app first, then reason, then
// detail) because alert channels are skim-read; reshuffling the layout
// is a real UX regression for the on-call human.
func TestBuildLifecycleContentFormat(t *testing.T) {
	origAppName := files.ConfiguredAppName
	origAppVersion := files.AppVersion
	origBotName := files.DiscordBotName
	t.Cleanup(func() {
		files.ConfiguredAppName = origAppName
		files.AppVersion = origAppVersion
		files.DiscordBotName = origBotName
	})
	files.ConfiguredAppName = "discordqotd"
	files.AppVersion = "v0.42.0"
	files.DiscordBotName = "QOTD"

	got := buildLifecycleContent("stopping", "")
	want := "**discordqotd** (v0.42.0) as `QOTD` → stopping"
	if got != want {
		t.Fatalf("buildLifecycleContent(stopping, ''): got %q want %q", got, want)
	}

	got = buildLifecycleContent("fatal", "runtime panic: nil map write")
	if !strings.HasPrefix(got, "**discordqotd** (v0.42.0)") {
		t.Fatalf("fatal content lost app prefix: %q", got)
	}
	if !strings.Contains(got, "→ fatal — runtime panic") {
		t.Fatalf("fatal content lost reason/detail separator: %q", got)
	}
}

// TestBuildLifecycleContentFallsBackWhenIdentityUnset confirms the
// content stays readable even when called before the bot has resolved
// its Discord identity (e.g. very early startup failure). Empty fields
// would print as awkward empty parens / dangling backticks; the
// fallbacks here mean an operator can still tell which deployment died.
func TestBuildLifecycleContentFallsBackWhenIdentityUnset(t *testing.T) {
	origAppName := files.ConfiguredAppName
	origAppVersion := files.AppVersion
	origBotName := files.DiscordBotName
	t.Cleanup(func() {
		files.ConfiguredAppName = origAppName
		files.AppVersion = origAppVersion
		files.DiscordBotName = origBotName
	})
	files.ConfiguredAppName = ""
	files.AppVersion = ""
	files.DiscordBotName = ""

	got := buildLifecycleContent("starting", "")
	if !strings.Contains(got, "discordcore") {
		t.Fatalf("expected fallback app name 'discordcore', got %q", got)
	}
	if !strings.Contains(got, files.DiscordCoreVersion) {
		t.Fatalf("expected fallback to core version %q, got %q", files.DiscordCoreVersion, got)
	}
	if strings.Contains(got, "``") {
		t.Fatalf("empty bot name produced empty backticks: %q", got)
	}
}

// TestNotifyLifecycleEventHandles5xx proves that HTTP 500 errors from the webhook
// endpoint do not bubble up or panic, and are instead cleanly handled.
func TestNotifyLifecycleEventHandles5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	// This should not panic or hang
	notifyLifecycleEvent("fatal", "simulated 500 error")
}

// TestNotifyLifecycleEventTimeoutContext proves that latency spikes from the
// discord webhook API are bounded by the context timeout, preventing the shutdown
// process from hanging indefinitely.
func TestNotifyLifecycleEventTimeoutContext(t *testing.T) {
	var handlerCalled sync.WaitGroup
	handlerCalled.Add(1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled.Done()
		// Sleep longer than the 3s timeout defined in lifecycleWebhookTimeout
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	start := time.Now()
	// This will block until context cancellation if the server hangs
	notifyLifecycleEvent("stopping", "simulated timeout")
	elapsed := time.Since(start)

	// Ensure the handler was actually hit
	handlerCalled.Wait()

	// Given lifecycleWebhookTimeout is 3 seconds, elapsed should be around 3s, not 5s.
	if elapsed >= 5*time.Second {
		t.Fatalf("expected timeout near 3s, but request took %v", elapsed)
	}
}
