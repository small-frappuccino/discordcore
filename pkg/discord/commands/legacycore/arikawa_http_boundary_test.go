package legacycore

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type rewriteTransport struct {
	URL *url.URL
	T   http.RoundTripper
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = r.URL.Scheme
	req.URL.Host = r.URL.Host
	return r.T.RoundTrip(req)
}

type MockArikawaCommand struct {
	execCount int
}

func (m *MockArikawaCommand) Name() string                     { return "mock_cmd" }
func (m *MockArikawaCommand) Description() string              { return "mock" }
func (m *MockArikawaCommand) Options() []discord.CommandOption { return nil }
func (m *MockArikawaCommand) RequiresGuild() bool              { return false }
func (m *MockArikawaCommand) RequiresPermissions() bool        { return false }
func (m *MockArikawaCommand) Handle(ctx *ArikawaContext) error {
	m.execCount++
	// Execute an HTTP call via the client to simulate hitting the boundary
	_, err := ctx.Client.Me()
	return err
}

func TestArikawaCommandRouter_HTTPBoundary(t *testing.T) {
	requestCount := 0

	// Create httptest.Server to simulate Discord API
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		fmt.Printf("Requested Path: %s\n", r.URL.Path)

		if r.URL.Path == "/api/v9/users/@me" || r.URL.Path == "/users/@me" {
			if requestCount == 1 {
				// Induce 429 Rate Limit on first try
				w.Header().Set("X-RateLimit-Reset-After", "0.1")
				w.Header().Set("X-RateLimit-Bucket", "test_bucket")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"message": "You are being rate limited.", "retry_after": 0.1, "global": false}`))
				return
			}
			if requestCount == 2 {
				// Induce 403 Missing Access
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"message": "Missing Access", "code": 50001}`))
				return
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": "123", "username": "mock_bot"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// Inject the mock transport into Arikawa Client
	client := api.NewClient("Bot fake-token")
	client.Client = httputil.NewClient()
	client.Client.Client = httpdriver.WrapClient(*ts.Client())

	// Arikawa allows overriding the API endpoint via the global httputil,
	// but v3 uses client.Client (which is a *httputil.Client).
	// To override URL prefix we might need to intercept it, but wait:
	tsURL, _ := url.Parse(ts.URL)
	client.Client.Client = httpdriver.WrapClient(http.Client{
		Transport: &rewriteTransport{
			URL: tsURL,
			T:   http.DefaultTransport,
		},
	})

	router := &ArikawaCommandRouter{
		commands:   make(map[string]ArikawaCommand),
		components: make(map[string]ArikawaComponentHandler),
		client:     client,
		config:     files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil),
	}

	mockCmd := &MockArikawaCommand{}
	router.Register(mockCmd)

	interaction := &discord.InteractionEvent{
		ID: discord.InteractionID(111),
		Data: &discord.CommandInteraction{
			Name: "mock_cmd",
		},
	}

	// This should hit 429, retry, then hit 403 and return the error.
	router.HandleInteractionEvent(interaction)

	if requestCount < 2 {
		t.Errorf("expected at least 2 requests (1 rate limit retry, 1 failure), got %d", requestCount)
	}
	if mockCmd.execCount != 1 {
		t.Errorf("expected 1 execution, got %d", mockCmd.execCount)
	}
}

// Validation of Webhook Impersonation
func TestWebhookImpersonation_Boundary(t *testing.T) {
	// Webhook tests usually mock the WebhookExecute API call
	requestCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Simulate WebhookExecute endpoint
		if strings.Contains(r.URL.Path, "/webhooks/") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id": "999", "channel_id": "888"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := api.NewClient("Bot fake")
	client.Client = httputil.NewClient()
	tsURL, _ := url.Parse(ts.URL)
	client.Client.Client = httpdriver.WrapClient(http.Client{
		Transport: &rewriteTransport{
			URL: tsURL,
			T:   http.DefaultTransport,
		},
	})

	whClient := webhook.FromAPI(123, "token", client)
	err := whClient.Execute(
		webhook.ExecuteData{
			Content:   "Test content",
			Username:  "ImpersonatedUser",
			AvatarURL: "https://avatar.url/123.png",
		},
	)

	if err != nil {
		t.Errorf("expected webhook execution to succeed, got %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected 1 request, got %d", requestCount)
	}
}
