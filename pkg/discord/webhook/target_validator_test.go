package webhook

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestValidateMessageTarget(t *testing.T) {
	tests := []struct {
		name          string
		webhookStatus int
		messageStatus int
		wantErr       bool
		wantClass     TargetValidationClass
		wantStatus    int
		wantTemporary bool
	}{
		{
			name:          "success",
			webhookStatus: http.StatusOK,
			messageStatus: http.StatusOK,
		},
		{
			name:          "webhook unauthorized",
			webhookStatus: http.StatusUnauthorized,
			messageStatus: http.StatusOK,
			wantErr:       true,
			wantClass:     TargetValidationClassAuthDenied,
			wantStatus:    http.StatusUnauthorized,
			wantTemporary: false,
		},
		{
			name:          "webhook forbidden",
			webhookStatus: http.StatusForbidden,
			messageStatus: http.StatusOK,
			wantErr:       true,
			wantClass:     TargetValidationClassAuthDenied,
			wantStatus:    http.StatusForbidden,
			wantTemporary: false,
		},
		{
			name:          "message not found",
			webhookStatus: http.StatusOK,
			messageStatus: http.StatusNotFound,
			wantErr:       true,
			wantClass:     TargetValidationClassNotFound,
			wantStatus:    http.StatusNotFound,
			wantTemporary: false,
		},
		{
			name:          "message rate limited",
			webhookStatus: http.StatusOK,
			messageStatus: http.StatusTooManyRequests,
			wantErr:       true,
			wantClass:     TargetValidationClassRateLimited,
			wantStatus:    http.StatusTooManyRequests,
			wantTemporary: true,
		},
		{
			name:          "webhook unavailable 500",
			webhookStatus: http.StatusInternalServerError,
			messageStatus: http.StatusOK,
			wantErr:       true,
			wantClass:     TargetValidationClassDiscordUnavailable,
			wantStatus:    http.StatusInternalServerError,
			wantTemporary: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch {
				case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/messages/"):
					w.Header().Set("Content-Type", "application/json")
					if tt.messageStatus != http.StatusOK {
						w.WriteHeader(tt.messageStatus)
						_, _ = w.Write([]byte(`{"message":"message error"}`))
						return
					}
					_, _ = w.Write([]byte(`{"id":"456","channel_id":"1","content":""}`))
					return
				case req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/webhooks/"):
					w.Header().Set("Content-Type", "application/json")
					if tt.webhookStatus != http.StatusOK {
						w.WriteHeader(tt.webhookStatus)
						_, _ = w.Write([]byte(`{"message":"webhook error"}`))
						return
					}
					_, _ = w.Write([]byte(`{"id":"123","type":1,"name":"test","token":"token-abc","channel_id":"1","guild_id":"1"}`))
					return
				default:
					w.WriteHeader(http.StatusOK)
				}
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

			session, err := discordgo.New("Bot test-token")
			if err != nil {
				t.Fatalf("failed to create session: %v", err)
			}

			err = ValidateMessageTarget(session, MessageTargetValidation{
				MessageID:  "456",
				WebhookURL: "https://discord.com/api/webhooks/123/token-abc",
				Timeout:    time.Second,
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMessageTarget error mismatch: got err=%v wantErr=%v", err, tt.wantErr)
			}
			if !tt.wantErr {
				return
			}

			var validationErr *TargetValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected TargetValidationError, got %T (%v)", err, err)
			}
			if validationErr.Class != tt.wantClass {
				t.Fatalf("unexpected class: got %q want %q", validationErr.Class, tt.wantClass)
			}
			if validationErr.StatusCode != tt.wantStatus {
				t.Fatalf("unexpected status code: got %d want %d", validationErr.StatusCode, tt.wantStatus)
			}
			if validationErr.Temporary != tt.wantTemporary {
				t.Fatalf("unexpected temporary flag: got %t want %t", validationErr.Temporary, tt.wantTemporary)
			}
		})
	}
}
