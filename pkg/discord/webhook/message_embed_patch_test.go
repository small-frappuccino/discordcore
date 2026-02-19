package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestParseWebhookURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		rawURL    string
		wantID    string
		wantToken string
		wantErr   bool
	}{
		{
			name:      "discord api path",
			rawURL:    "https://discord.com/api/webhooks/123456/token-abc",
			wantID:    "123456",
			wantToken: "token-abc",
		},
		{
			name:      "versioned api path",
			rawURL:    "https://discord.com/api/v10/webhooks/999/token_xyz?wait=true",
			wantID:    "999",
			wantToken: "token_xyz",
		},
		{
			name:    "invalid url",
			rawURL:  "://bad-url",
			wantErr: true,
		},
		{
			name:    "missing token",
			rawURL:  "https://discord.com/api/webhooks/123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotID, gotToken, err := parseWebhookURL(tt.rawURL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseWebhookURL error mismatch: got err=%v wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if gotID != tt.wantID {
				t.Fatalf("webhook id mismatch: got %q want %q", gotID, tt.wantID)
			}
			if gotToken != tt.wantToken {
				t.Fatalf("webhook token mismatch: got %q want %q", gotToken, tt.wantToken)
			}
		})
	}
}

func TestDecodeEmbeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		payload    string
		wantCount  int
		wantFirst  string
		expectFail bool
	}{
		{
			name:      "single embed object",
			payload:   `{"title":"single"}`,
			wantCount: 1,
			wantFirst: "single",
		},
		{
			name:      "embeds array",
			payload:   `[{"title":"first"},{"title":"second"}]`,
			wantCount: 2,
			wantFirst: "first",
		},
		{
			name:      "payload object with embeds",
			payload:   `{"embeds":[{"title":"wrapped"}]}`,
			wantCount: 1,
			wantFirst: "wrapped",
		},
		{
			name:       "invalid payload",
			payload:    `"oops"`,
			expectFail: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			embeds, err := decodeEmbeds(json.RawMessage(tt.payload))
			if (err != nil) != tt.expectFail {
				t.Fatalf("decodeEmbeds error mismatch: got err=%v expectFail=%v", err, tt.expectFail)
			}
			if tt.expectFail {
				return
			}
			if len(embeds) != tt.wantCount {
				t.Fatalf("embed count mismatch: got %d want %d", len(embeds), tt.wantCount)
			}
			if embeds[0] == nil || embeds[0].Title != tt.wantFirst {
				t.Fatalf("first embed title mismatch: got %+v want %q", embeds[0], tt.wantFirst)
			}
		})
	}
}

func TestPatchMessageEmbed(t *testing.T) {
	t.Parallel()

	var requestMethod string
	var requestPath string
	var requestBody struct {
		Embeds []discordgo.MessageEmbed `json:"embeds"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"456"}`))
	}))
	defer server.Close()

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	defer func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
	}()

	s, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("failed to create discord session: %v", err)
	}

	err = PatchMessageEmbed(s, MessageEmbedPatch{
		MessageID:  "456",
		WebhookURL: "https://discord.com/api/webhooks/123/token-abc",
		Embed:      json.RawMessage(`{"title":"updated title"}`),
	})
	if err != nil {
		t.Fatalf("PatchMessageEmbed returned error: %v", err)
	}

	if requestMethod != http.MethodPatch {
		t.Fatalf("unexpected method: got %s want %s", requestMethod, http.MethodPatch)
	}
	if !strings.HasSuffix(requestPath, "/webhooks/123/token-abc/messages/456") {
		t.Fatalf("unexpected request path: %s", requestPath)
	}
	if len(requestBody.Embeds) != 1 || requestBody.Embeds[0].Title != "updated title" {
		t.Fatalf("unexpected request embeds payload: %+v", requestBody.Embeds)
	}
}
