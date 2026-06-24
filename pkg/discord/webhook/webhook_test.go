package webhook_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/api/webhook"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/httputil"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
	"github.com/small-frappuccino/discordcore/pkg/app"
	webhookPkg "github.com/small-frappuccino/discordcore/pkg/discord/webhook"
)

type rewriteTransport struct {
	URL *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.URL.Scheme
	req.URL.Host = t.URL.Host
	return http.DefaultTransport.RoundTrip(req)
}

type MockAPI struct {
	WebhookMessageEditFn func(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID, data webhook.EditMessageData) (*discord.Message, error)
	WebhookWithTokenFn   func(ctx context.Context, webhookID discord.WebhookID, webhookToken string) (*discord.Webhook, error)
	WebhookMessageFn     func(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID) (*discord.Message, error)
}

func (m *MockAPI) WebhookMessageEdit(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID, data webhook.EditMessageData) (*discord.Message, error) {
	if m.WebhookMessageEditFn != nil {
		return m.WebhookMessageEditFn(ctx, webhookID, webhookToken, messageID, data)
	}
	return nil, nil
}

func (m *MockAPI) WebhookWithToken(ctx context.Context, webhookID discord.WebhookID, webhookToken string) (*discord.Webhook, error) {
	if m.WebhookWithTokenFn != nil {
		return m.WebhookWithTokenFn(ctx, webhookID, webhookToken)
	}
	return nil, nil
}

func (m *MockAPI) WebhookMessage(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID) (*discord.Message, error) {
	if m.WebhookMessageFn != nil {
		return m.WebhookMessageFn(ctx, webhookID, webhookToken, messageID)
	}
	return nil, nil
}

var _ webhookPkg.API = (*MockAPI)(nil)

func TestValidateMessageTarget_NetworkLifecycle(t *testing.T) {
	t.Parallel()
	mock := &MockAPI{
		WebhookWithTokenFn: func(ctx context.Context, webhookID discord.WebhookID, webhookToken string) (*discord.Webhook, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(1 * time.Second):
				return &discord.Webhook{}, nil
			}
		},
	}

	validation := webhookPkg.MessageTargetValidation{
		MessageID:  "456",
		WebhookURL: "https://discord.com/api/webhooks/123/token",
		Timeout:    0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	err := webhookPkg.ValidateMessageTarget(ctx, mock, validation)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var targetErr *webhookPkg.TargetValidationError
	if errors.As(err, &targetErr) {
		if !errors.Is(targetErr.Cause, context.DeadlineExceeded) {
			t.Fatalf("expected context.DeadlineExceeded cause, got: %v", targetErr.Cause)
		}
	} else {
		t.Fatalf("expected TargetValidationError, got %T", err)
	}
}

func TestValidateMessageTarget_ErrorAssertions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		httpStatus int
		wantClass  webhookPkg.TargetValidationClass
	}{
		{"Auth Denied 401", http.StatusUnauthorized, webhookPkg.TargetValidationClassAuthDenied},
		{"Not Found 404", http.StatusNotFound, webhookPkg.TargetValidationClassNotFound},
		{"Rate Limited 429", http.StatusTooManyRequests, webhookPkg.TargetValidationClassRateLimited},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpErr := &httputil.HTTPError{
				Status:  tt.httpStatus,
				Message: "forged error",
			}
			mock := &MockAPI{
				WebhookWithTokenFn: func(ctx context.Context, webhookID discord.WebhookID, webhookToken string) (*discord.Webhook, error) {
					return nil, httpErr
				},
			}

			validation := webhookPkg.MessageTargetValidation{
				MessageID:  "456",
				WebhookURL: "https://discord.com/api/webhooks/123/token",
			}

			err := webhookPkg.ValidateMessageTarget(context.Background(), mock, validation)
			var targetErr *webhookPkg.TargetValidationError
			if !errors.As(err, &targetErr) {
				t.Fatalf("expected TargetValidationError, got: %v", err)
			}
			if targetErr.Class != tt.wantClass {
				t.Fatalf("expected class %s, got %s", tt.wantClass, targetErr.Class)
			}
			if !errors.Is(targetErr.Cause, httpErr) {
				t.Fatalf("expected cause to be exactly our forged HTTPError")
			}
		})
	}
}

func TestDecodeEmbeds_Fuzzing(t *testing.T) {
	t.Parallel()
	payloads := []string{
		`{"title":"single"}`,
		`[{"title":"array_one"}]`,
		`{"embeds":[{"title":"object_nested"}]}`,
	}

	for _, p := range payloads {
		raw := json.RawMessage(p)
		embeds, err := webhookPkg.ExportDecodeEmbeds(raw)
		if err != nil {
			t.Fatalf("failed to decode %s: %v", p, err)
		}
		if len(embeds) == 0 {
			t.Fatal("expected at least 1 embed")
		}
	}
}

func BenchmarkDecodeEmbeds_Allocs(b *testing.B) {
	raw := json.RawMessage(`{"embeds":[{"title":"test","description":"allocations"}]}`)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := webhookPkg.ExportDecodeEmbeds(raw)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestArikawaAPI_ServerInjection_TableDriven(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_, _ = io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "messages/456") && r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"456","channel_id":"1","type":1,"content":"patched"}`))
			return
		}

		if strings.HasSuffix(r.URL.Path, "webhooks/123/token") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":"123","type":1,"name":"test","token":"token","channel_id":"1","guild_id":"1"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	srvURL, _ := url.Parse(srv.URL)
	customTransport := &rewriteTransport{URL: srvURL}

	httpClient := http.Client{Transport: customTransport}
	client := httputil.NewClient()
	client.Client = httpdriver.WrapClient(httpClient)
	client.Retries = 0

	tests := []struct {
		name       string
		messageID  string
		webhookURL string
		expectErr  bool
	}{
		{"Valid Target", "456", "https://discord.com/api/webhooks/123/token", false},
		{"Invalid Webhook ID", "456", "https://discord.com/api/webhooks/999/token", true},
		{"Invalid Message ID", "999", "https://discord.com/api/webhooks/123/token", true},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whID, whToken, _ := webhookPkg.ParseWebhookURL(tt.webhookURL)
			whClient := webhook.NewCustom(whID, whToken, client).WithContext(ctx)

			var err error
			if tt.messageID == "456" && tt.webhookURL == "https://discord.com/api/webhooks/123/token" {
				_, err = whClient.EditMessage(discord.MessageID(456), webhook.EditMessageData{
					Content: option.NewNullableString("patched"),
				})
			} else {
				_, err = whClient.Get()
				if err == nil && tt.messageID == "999" {
					_, err = whClient.Message(discord.MessageID(999))
				}
			}

			if (err != nil) != tt.expectErr {
				t.Fatalf("expected err %v, got %v", tt.expectErr, err)
			}
		})
	}
}

func TestWebhookConcurrentExecution(t *testing.T) {
	t.Parallel()
	orchestrator := app.NewStartupTaskOrchestrator(10)
	defer orchestrator.Shutdown(context.Background())

	mock := &MockAPI{
		WebhookWithTokenFn: func(ctx context.Context, webhookID discord.WebhookID, webhookToken string) (*discord.Webhook, error) {
			return &discord.Webhook{ID: webhookID}, nil
		},
		WebhookMessageFn: func(ctx context.Context, webhookID discord.WebhookID, webhookToken string, messageID discord.MessageID) (*discord.Message, error) {
			return &discord.Message{ID: messageID}, nil
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		orchestrator.GoLight("webhook_test", &mockTask{
			wg:   &wg,
			mock: mock,
		})
	}
	wg.Wait()
}

type mockTask struct {
	wg   *sync.WaitGroup
	mock *MockAPI
}

func (m *mockTask) Execute(ctx context.Context) error {
	defer m.wg.Done()
	validation := webhookPkg.MessageTargetValidation{
		MessageID:  "456",
		WebhookURL: "https://discord.com/api/webhooks/123/token",
	}
	return webhookPkg.ValidateMessageTarget(ctx, m.mock, validation)
}
