package legacycore

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordgo"
)

type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestCleanGuildCommands_RateLimitFallback(t *testing.T) {
	session, err := discordgo.New("Bot dummy")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	deleteCallCount := 0
	var timeBeforeDelete time.Time
	var timeAfterDelete time.Time

	session.Client = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" && strings.Contains(req.URL.Path, "/commands") {
					jsonBody := `[{"id":"12345","application_id":"dummyApp","name":"testcmd","description":"test","version":"1"}]`
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(jsonBody)),
						Header:     make(http.Header),
					}, nil
				}
				if req.Method == "DELETE" {
					deleteCallCount++
					if deleteCallCount == 1 {
						timeBeforeDelete = time.Now()
						// Simulate 429 with stripped Retry-After
						resp := &http.Response{
							StatusCode: 429,
							Body:       io.NopCloser(bytes.NewBufferString(`{"message": "You are being rate limited", "retry_after": 0.0}`)),
							Header:     make(http.Header),
						}
						// No Retry-After header!
						return resp, nil
					}

					timeAfterDelete = time.Now()
					// Second call succeeds
					return &http.Response{
						StatusCode: 204,
						Body:       http.NoBody,
						Header:     make(http.Header),
					}, nil
				}
				return &http.Response{StatusCode: 404, Body: http.NoBody}, nil
			},
		},
	}

	// Use a 10s context so the 5s fallback delay has time to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = cleanGuildCommands(ctx, session, "dummyApp", "dummyGuild")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if deleteCallCount != 2 {
		t.Errorf("expected DELETE to be called 2 times (1 retry), got %d", deleteCallCount)
	}

	elapsed := timeAfterDelete.Sub(timeBeforeDelete)
	if elapsed < 5*time.Second {
		t.Errorf("expected fallback delay of at least 5s, got %v", elapsed)
	}
}

func TestCleanGuildCommands_ContextCancellation(t *testing.T) {
	session, err := discordgo.New("Bot dummy")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	deleteCallCount := 0
	session.Client = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" && strings.Contains(req.URL.Path, "/commands") {
					jsonBody := `[{"id":"12345","application_id":"dummyApp","name":"testcmd","description":"test","version":"1"}]`
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(jsonBody)),
						Header:     make(http.Header),
					}, nil
				}
				if req.Method == "DELETE" {
					deleteCallCount++
					// Simulate 429 with 5s backoff
					resp := &http.Response{
						StatusCode: 429,
						Body:       io.NopCloser(bytes.NewBufferString(`{"message": "Rate limited", "retry_after": 5.0}`)),
						Header:     make(http.Header),
					}
					return resp, nil
				}
				return &http.Response{StatusCode: 404, Body: http.NoBody}, nil
			},
		},
	}

	// Context with very short timeout so it cancels during the 5s sleep
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = cleanGuildCommands(ctx, session, "dummyApp", "dummyGuild")
	if err == nil {
		t.Fatalf("expected context cancellation error, got nil")
	}

	if err != context.DeadlineExceeded {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}

	if deleteCallCount != 1 {
		t.Errorf("expected DELETE to be called exactly 1 time, got %d", deleteCallCount)
	}
}

func TestCleanGuildCommands_500InternalServerErrorLoop(t *testing.T) {
	session, err := discordgo.New("Bot dummy")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	deleteCallCount := 0
	session.Client = &http.Client{
		Transport: &mockRoundTripper{
			roundTripFunc: func(req *http.Request) (*http.Response, error) {
				if req.Method == "GET" && strings.Contains(req.URL.Path, "/commands") {
					jsonBody := `[
						{"id":"1","application_id":"dummyApp","name":"cmd1","description":"test","version":"1"},
						{"id":"2","application_id":"dummyApp","name":"cmd2","description":"test","version":"1"}
					]`
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(jsonBody)),
						Header:     make(http.Header),
					}, nil
				}
				if req.Method == "DELETE" {
					deleteCallCount++
					// Simulate 500 error
					return &http.Response{
						StatusCode: 500,
						Body:       io.NopCloser(bytes.NewBufferString(`{"message": "Internal Server Error"}`)),
						Header:     make(http.Header),
					}, nil
				}
				return &http.Response{StatusCode: 404, Body: http.NoBody}, nil
			},
		},
	}

	ctx := context.Background()
	err = cleanGuildCommands(ctx, session, "dummyApp", "dummyGuild")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if deleteCallCount != 2 {
		t.Errorf("expected DELETE to be called exactly 2 times (once for each command), got %d", deleteCallCount)
	}
}
