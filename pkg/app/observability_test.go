package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
)

// The arikawa_qotd_publisher_test.go logic:
func TestArikawaQOTDPublisher_GetArikawaPublisher(t *testing.T) {
	t.Parallel()
	resolver := newBotRuntimeResolver(nil, nil)
	publisher := NewArikawaQOTDPublisher(resolver)

	// Will error because guild has no runtime
	_, err := publisher.getArikawaPublisher("missing_guild")
	if err == nil {
		t.Fatal("expected structural isolation error for missing guild runtime")
	}
}

func TestArikawaQOTDPublisher_PublishOfficialPost(t *testing.T) {
	t.Parallel()
	resolver := newBotRuntimeResolver(nil, nil)
	publisher := NewArikawaQOTDPublisher(resolver)

	_, err := publisher.PublishOfficialPost(context.Background(), qotd.PublishOfficialPostParams{
		GuildID:                  "guild1",
		ChannelID:                "channel1",
		OfficialThreadID:         "author1",
		OfficialStarterMessageID: "content",
	})
	if err == nil {
		t.Fatal("expected early failure due to missing guild gateway binding")
	}
}

func TestArikawaQOTDPublisher_DeleteOfficialPost(t *testing.T) {
	t.Parallel()
	resolver := newBotRuntimeResolver(nil, nil)
	publisher := NewArikawaQOTDPublisher(resolver)

	err := publisher.DeleteOfficialPost(context.Background(), qotd.DeleteOfficialPostParams{
		GuildID:                 "guild1",
		ChannelID:               "channel1",
		DiscordStarterMessageID: "message1",
	})
	if err == nil {
		t.Fatal("expected early failure due to missing guild gateway binding")
	}
}

// The lifecycle_webhook_test.go logic:
func TestNotifyLifecycleEventSendsWebhook(t *testing.T) {
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

func TestNotifyLifecycleEventHandles5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	// Should not panic
	notifyLifecycleEvent("fatal", "simulated 500 error")
}

func TestNotifyLifecycleEventTimeoutContext(t *testing.T) {
	var handlerCalled sync.WaitGroup
	handlerCalled.Add(1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled.Done()
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	t.Setenv(lifecycleWebhookEnv, server.URL)

	start := time.Now()
	notifyLifecycleEvent("stopping", "simulated timeout")
	elapsed := time.Since(start)

	handlerCalled.Wait()

	if elapsed >= 5*time.Second {
		t.Fatalf("expected timeout near 3s, but request took %v", elapsed)
	}
}

// The runner_webhook_updates_test.go logic:
func TestCollectStartupWebhookEmbedUpdatesGlobalAndGuild(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
				{
					MessageID:  "global-1",
					WebhookURL: "https://discord.com/api/webhooks/1/token",
					Embed:      json.RawMessage(`{"title":"g1"}`),
				},
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID: "guild-a",
				RuntimeConfig: files.RuntimeConfig{
					WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
						{
							MessageID:  "guild-a-1",
							WebhookURL: "https://discord.com/api/webhooks/2/token",
							Embed:      json.RawMessage(`{"title":"a1"}`),
						},
						{
							MessageID:  "guild-a-2",
							WebhookURL: "https://discord.com/api/webhooks/3/token",
							Embed:      json.RawMessage(`{"title":"a2"}`),
						},
					},
				},
			},
			{
				GuildID: "guild-b",
				RuntimeConfig: files.RuntimeConfig{
					WebhookEmbedUpdates: []files.WebhookEmbedUpdateConfig{
						{
							MessageID:  "guild-b-1",
							WebhookURL: "https://discord.com/api/webhooks/4/token",
							Embed:      json.RawMessage(`{"title":"b1"}`),
						},
					},
				},
			},
		},
	}

	got := collectStartupWebhookEmbedUpdates(cfg)
	if len(got) != 4 {
		t.Fatalf("expected 4 startup updates, got %d", len(got))
	}

	if got[0].scope != "global" || got[0].index != 0 || got[0].update.MessageID != "global-1" {
		t.Fatalf("unexpected first item: %+v", got[0])
	}
	if got[1].scope != "guild:guild-a" || got[1].index != 0 || got[1].update.MessageID != "guild-a-1" {
		t.Fatalf("unexpected second item: %+v", got[1])
	}
	if got[2].scope != "guild:guild-a" || got[2].index != 1 || got[2].update.MessageID != "guild-a-2" {
		t.Fatalf("unexpected third item: %+v", got[2])
	}
	if got[3].scope != "guild:guild-b" || got[3].index != 0 || got[3].update.MessageID != "guild-b-1" {
		t.Fatalf("unexpected fourth item: %+v", got[3])
	}
}

func TestCollectStartupWebhookEmbedUpdatesNilConfig(t *testing.T) {
	t.Parallel()

	if got := collectStartupWebhookEmbedUpdates(nil); len(got) != 0 {
		t.Fatalf("expected nil/empty list for nil config, got %+v", got)
	}
}
