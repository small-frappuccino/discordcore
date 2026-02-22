package messageupdate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
)

type patchRecorder struct {
	mu sync.Mutex

	webhookPatchCount int
	channelPatchCount int
	lastWebhookPath   string
	lastChannelPath   string
}

func (r *patchRecorder) recordWebhook(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhookPatchCount++
	r.lastWebhookPath = path
}

func (r *patchRecorder) recordChannel(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channelPatchCount++
	r.lastChannelPath = path
}

func (r *patchRecorder) counts() (int, int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.webhookPatchCount, r.channelPatchCount
}

func (r *patchRecorder) webhookPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastWebhookPath
}

func (r *patchRecorder) channelPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastChannelPath
}

func newMessageUpdateTestSession(t *testing.T) (*discordgo.Session, *patchRecorder) {
	t.Helper()

	rec := &patchRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.Contains(req.URL.Path, "/webhooks/") && req.Method == http.MethodPatch:
			rec.recordWebhook(req.URL.Path)
			var payload map[string]any
			_ = json.NewDecoder(req.Body).Decode(&payload)
			if _, ok := payload["embeds"]; !ok {
				t.Errorf("expected embeds in webhook patch payload: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"ok"}`))
			return
		case strings.Contains(req.URL.Path, "/channels/") && req.Method == http.MethodPatch:
			rec.recordChannel(req.URL.Path)
			var payload map[string]any
			_ = json.NewDecoder(req.Body).Decode(&payload)
			if _, ok := payload["embeds"]; !ok {
				t.Errorf("expected embeds in channel patch payload: %#v", payload)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"ok"}`))
			return
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"noop"}`))
			return
		}
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldChannels := discordgo.EndpointChannels
	oldWebhooks := discordgo.EndpointWebhooks
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointChannels = server.URL + "/channels/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointChannels = oldChannels
		discordgo.EndpointWebhooks = oldWebhooks
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	return session, rec
}

func TestUpdateEmbeds_WebhookTarget(t *testing.T) {
	session, rec := newMessageUpdateTestSession(t)
	err := UpdateEmbeds(session, EmbedUpdateTarget{
		Type:       TargetTypeWebhookMessage,
		MessageID:  "456",
		WebhookURL: "https://discord.com/api/webhooks/123/token",
	}, []*discordgo.MessageEmbed{
		{Title: "Partners"},
	})
	if err != nil {
		t.Fatalf("UpdateEmbeds returned error: %v", err)
	}

	webhookCount, channelCount := rec.counts()
	if webhookCount != 1 {
		t.Fatalf("expected 1 webhook patch call, got %d", webhookCount)
	}
	if channelCount != 0 {
		t.Fatalf("expected 0 channel patch calls, got %d", channelCount)
	}
	if path := rec.webhookPath(); !strings.Contains(path, "/webhooks/123/token/messages/456") {
		t.Fatalf("unexpected webhook patch path: %q", path)
	}
}

func TestUpdateEmbeds_ChannelTarget(t *testing.T) {
	session, rec := newMessageUpdateTestSession(t)
	err := UpdateEmbeds(session, EmbedUpdateTarget{
		Type:      TargetTypeChannelMessage,
		MessageID: "456",
		ChannelID: "789",
	}, []*discordgo.MessageEmbed{
		{Title: "Partners"},
		{Title: "Partners 2"},
	})
	if err != nil {
		t.Fatalf("UpdateEmbeds returned error: %v", err)
	}

	webhookCount, channelCount := rec.counts()
	if webhookCount != 0 {
		t.Fatalf("expected 0 webhook patch calls, got %d", webhookCount)
	}
	if channelCount != 1 {
		t.Fatalf("expected 1 channel patch call, got %d", channelCount)
	}
	if path := rec.channelPath(); !strings.Contains(path, "/channels/789/messages/456") {
		t.Fatalf("unexpected channel patch path: %q", path)
	}
}

func TestUpdateEmbeds_InvalidTarget(t *testing.T) {
	session, _ := newMessageUpdateTestSession(t)
	err := UpdateEmbeds(session, EmbedUpdateTarget{
		Type:      TargetTypeChannelMessage,
		MessageID: "456",
		ChannelID: "not-numeric",
	}, []*discordgo.MessageEmbed{
		{Title: "Partners"},
	})
	if err == nil {
		t.Fatal("expected validation error for invalid channel target")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "invalid embed update target") {
		t.Fatalf("expected invalid target error, got %v", err)
	}
}
