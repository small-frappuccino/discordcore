package tickets

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	discordtickets "github.com/small-frappuccino/discordcore/pkg/discord/tickets"
	"github.com/small-frappuccino/discordcore/pkg/files"
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

func TestRouter_DeferBeforeIO(t *testing.T) {
	deferralReceived := make(chan bool, 1)
	editReceived := make(chan bool, 1)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/interactions/") && strings.Contains(r.URL.Path, "/callback") {
			var data api.InteractionResponse
			json.NewDecoder(r.Body).Decode(&data)
			if data.Type == api.DeferredMessageInteractionWithSource {
				deferralReceived <- true
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/webhooks/") {
			editReceived <- true
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/channels/") {
			// Sleep to simulate latência
			time.Sleep(4 * time.Second)
			json.NewEncoder(w).Encode(discord.Channel{
				ID:   discord.ChannelID(2),
				Name: "ticket-0001",
			})
			return
		}

		if r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/channels/") {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(mockServer.Close)

	st := state.New("Bot test")
	u, _ := url.Parse(mockServer.URL)
	oldTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		Transport: oldTransport,
		MockURL:   u,
	}
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := discordtickets.NewService(st)
	r := NewTicketRouter(st, svc, nil, cm, logger)

	event := &gateway.InteractionCreateEvent{
		InteractionEvent: discord.InteractionEvent{
			ID:    discord.InteractionID(1),
			AppID: discord.AppID(1),
			Token: "token",
			Data: &discord.ButtonInteraction{
				CustomID: "ticket_close",
			},
		},
	}

	start := time.Now()
	go r.HandleInteraction(event)

	select {
	case <-deferralReceived:
		if time.Since(start) > 2*time.Second {
			t.Errorf("deferral took too long: %v", time.Since(start))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for deferred response")
	}

	select {
	case <-editReceived:
		if time.Since(start) < 4*time.Second {
			t.Errorf("edit received too early: %v", time.Since(start))
		}
	case <-time.After(6 * time.Second):
		t.Fatal("timeout waiting for edit response")
	}
}
