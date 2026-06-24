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
	"github.com/small-frappuccino/discordcore/pkg/config"
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
	t.Parallel()

	deferralReceived := make(chan bool, 1)
	editReceived := make(chan bool, 1)
	blockGetChannel := make(chan struct{})

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
			// Wait for the test to signal deferral completion
			<-blockGetChannel
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

	cm := files.NewConfigManagerWithStore(&config.MemoryConfigStore{}, nil)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := discordtickets.NewService(st, logger)
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

	go r.HandleInteraction(event)

	select {
	case <-deferralReceived:
		// Success: deferral was sent before GET /channels/ returned
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for deferred response")
	}

	// Unblock the GET /channels/ handler
	close(blockGetChannel)

	select {
	case <-editReceived:
		// Success: edit webhook completed after unblocking GET /channels/
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for edit response")
	}
}
