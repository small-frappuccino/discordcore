package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestInteractionGatewayFailures(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
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
			session.Client = server.Client() // Inject the test client explicitly if necessary, though URL usually suffices

			config := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
			router := NewCommandRouter(session, config)

			router.RegisterCommand(testCommand{
				name:      "gatewaytest",
				ackPolicy: InteractionAckPolicy{Mode: InteractionAckModeDefer, Ephemeral: true},
				handler: func(ctx *Context) error {
					return nil
				},
			})

			interaction := buildInteraction("gatewaytest", "guild", "user")

			done := make(chan struct{})
			go func() {
				router.HandleInteractionWithContext(context.Background(), session, interaction)
				close(done)
			}()

			select {
			case <-done:
				// success, didn't hang and didn't crash
			case <-time.After(2 * time.Second):
				t.Fatal("HandleInteractionWithContext hung on gateway failure")
			}
		})
	}
}

func TestInteractionContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sleep longer than the context deadline
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(discordgo.Message{})
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

	config := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	router := NewCommandRouter(session, config)

	router.RegisterCommand(testCommand{
		name:      "contexttest",
		ackPolicy: InteractionAckPolicy{Mode: InteractionAckModeDefer, Ephemeral: true},
		handler: func(ctx *Context) error {
			return nil
		},
	})

	interaction := buildInteraction("contexttest", "guild", "user")

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	router.HandleInteractionWithContext(ctx, session, interaction)
	duration := time.Since(start)

	// The router should abandon the request when context cancels, taking ~50ms
	// instead of the 1s sleep in the server.
	if duration >= 500*time.Millisecond {
		t.Fatalf("HandleInteractionWithContext did not respect context cancellation, took %v", duration)
	}
}
