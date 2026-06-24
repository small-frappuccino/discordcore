package qotd

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
)

func TestArikawaPublisher_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedError error
		isAbandoned   bool
	}{
		{
			name:          "404 Unknown Channel",
			statusCode:    http.StatusNotFound,
			responseBody:  `{"message": "Unknown Channel", "code": 10003}`,
			expectedError: domain.ErrDiscordUnknownChannel,
			isAbandoned:   true,
		},
		{
			name:          "403 Missing Access",
			statusCode:    http.StatusForbidden,
			responseBody:  `{"message": "Missing Access", "code": 50001}`,
			expectedError: domain.ErrDiscordMissingAccess,
			isAbandoned:   true,
		},
		{
			name:          "429 Too Many Requests",
			statusCode:    http.StatusTooManyRequests,
			responseBody:  `{"message": "You are being rate limited", "retry_after": 0.5}`,
			expectedError: nil, // Note: It shouldn't match an unrecoverable error. It will map to the underlying httputil.HTTPError
			isAbandoned:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.responseBody))
			}))
			defer ts.Close()

			client := api.NewClient("Bot token")
			httpClient := http.Client{
				Transport: &rewriteTransport{
					Transport: http.DefaultTransport,
					BaseURL:   ts.URL,
				},
			}
			client.Client.Client = httpdriver.WrapClient(httpClient)

			pub := NewArikawaPublisher(client)

			_, err := pub.PublishOfficialPost(context.Background(), domain.PublishOfficialPostParams{
				GuildID:      "123",
				ChannelID:    "456",
				QuestionText: "Test",
			})

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if tc.expectedError != nil && !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error %v, got %v", tc.expectedError, err)
			}

			abandoned := isUnrecoverableDiscordPublishError(err)
			if abandoned != tc.isAbandoned {
				t.Errorf("expected isAbandoned=%v, got %v", tc.isAbandoned, abandoned)
			}
		})
	}
}

type rewriteTransport struct {
	Transport http.RoundTripper
	BaseURL   string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = t.BaseURL[7:] // strip "http://"
	return t.Transport.RoundTrip(req)
}
