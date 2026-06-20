package tickets

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"go.uber.org/goleak"
)

type rewriteTransport struct {
	Transport http.RoundTripper
	MockURL   *url.URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.MockURL.Scheme
	req.URL.Host = t.MockURL.Host
	return t.Transport.RoundTrip(req)
}

func newMockClient(t *testing.T, serverURL string) *state.State {
	s := state.New("Bot test")
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("parse mock url: %v", err)
	}

	oldTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		Transport: oldTransport,
		MockURL:   u,
	}
	t.Cleanup(func() {
		http.DefaultTransport = oldTransport
		if tr, ok := oldTransport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	})
	return s
}

func TestService_GenerateAndUploadTranscript_Success(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"))

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/messages") {
			before := r.URL.Query().Get("before")
			if before == "" {
				msgs := make([]discord.Message, 100)
				for i := 0; i < 100; i++ {
					msgs[i] = discord.Message{ID: discord.MessageID(200 - i), Content: "page1"}
				}
				json.NewEncoder(w).Encode(msgs)
				return
			}
			msgs := make([]discord.Message, 50)
			for i := 0; i < 50; i++ {
				msgs[i] = discord.Message{ID: discord.MessageID(100 - i), Content: "page2"}
			}
			json.NewEncoder(w).Encode(msgs)
			return
		}

		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/messages") {
			err := r.ParseMultipartForm(10 << 20)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			file, _, err := r.FormFile("file0")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			content, _ := io.ReadAll(file)

			var parsed []discord.Message
			if err := json.Unmarshal(content, &parsed); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if len(parsed) != 150 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			json.NewEncoder(w).Encode(discord.Message{ID: 999})
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	s := NewService(newMockClient(t, mockServer.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.GenerateAndUploadTranscript(ctx, 1, 2)
	if err != nil {
		t.Fatalf("GenerateAndUploadTranscript failed: %v", err)
	}
}

func TestService_GenerateAndUploadTranscript_Deadlock(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"), goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"))

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/messages") {
			before := r.URL.Query().Get("before")
			if before == "" {
				msgs := make([]discord.Message, 100)
				for i := 0; i < 100; i++ {
					msgs[i] = discord.Message{ID: discord.MessageID(200 - i), Content: "page1"}
				}
				json.NewEncoder(w).Encode(msgs)
				return
			}
			// Injeta falha na página N
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"message": "Internal Server Error"}`))
			return
		}

		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/messages") {
			r.ParseMultipartForm(10 << 20)
			file, _, err := r.FormFile("file0")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			_, _ = io.ReadAll(file)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer mockServer.Close()

	s := NewService(newMockClient(t, mockServer.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := s.GenerateAndUploadTranscript(ctx, 1, 2)
	if err == nil {
		t.Fatalf("expected error due to injected failure, got nil")
	}
	if !strings.Contains(err.Error(), "encode transcript") && !strings.Contains(err.Error(), "upload transcript") {
		t.Errorf("unexpected error: %v", err)
	}
}
