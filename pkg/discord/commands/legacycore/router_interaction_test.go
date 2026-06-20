package legacycore_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/legacycore"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

type mockRoundTripper struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.roundTripFunc != nil {
		return m.roundTripFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
		Header:     make(http.Header),
	}, nil
}

type testCommand struct {
	name                string
	requiresGuild       bool
	requiresPermissions bool
	handler             func(*legacycore.Context) error
	autocomplete        legacycore.AutocompleteHandler
	ackPolicy           legacycore.InteractionAckPolicy
}

func (tc testCommand) Name() string        { return tc.name }
func (tc testCommand) Description() string { return tc.name }
func (tc testCommand) Options() []*discordgo.ApplicationCommandOption {
	return nil
}
func (tc testCommand) Handle(ctx *legacycore.Context) error {
	if tc.handler != nil {
		return tc.handler(ctx)
	}
	return nil
}
func (tc testCommand) AutocompleteRouteHandler() legacycore.AutocompleteHandler {
	return tc.autocomplete
}
func (tc testCommand) InteractionAckPolicy() legacycore.InteractionAckPolicy { return tc.ackPolicy }
func (tc testCommand) RequiresGuild() bool                                   { return tc.requiresGuild }
func (tc testCommand) RequiresPermissions() bool                             { return tc.requiresPermissions }
func buildInteraction(command, guildID, userID string) *discordgo.InteractionCreate {
	data := discordgo.ApplicationCommandInteractionData{
		ID:      "cmd-" + command,
		Name:    command,
		Options: []*discordgo.ApplicationCommandInteractionDataOption{},
	}
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-" + command,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data:    data,
		},
	}
}
func TestHandleInteraction(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		delay      time.Duration
		cancelCtx  bool
		expectHung bool
	}{
		{name: "Unauthorized", status: http.StatusUnauthorized},
		{name: "Forbidden", status: http.StatusForbidden},
		{name: "BadGateway", status: http.StatusBadGateway},
		{name: "ContextCancel", status: http.StatusOK, delay: 1 * time.Second, cancelCtx: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session, err := discordgo.New("Bot test-token")
			if err != nil {
				t.Fatalf("failed to create session: %v", err)
			}
			rt := &mockRoundTripper{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					if tt.delay > 0 && strings.Contains(req.URL.Path, "/callback") {
						select {
						case <-time.After(tt.delay):
						case <-req.Context().Done():
							return nil, req.Context().Err()
						}
					}
					return &http.Response{
						StatusCode: tt.status,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
						Header:     make(http.Header),
					}, nil
				},
			}
			session.Client = &http.Client{Transport: rt}
			config := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
			router := legacycore.NewCommandRouter(session, config)
			router.RegisterCommand(testCommand{
				name:      "gatewaytest",
				ackPolicy: legacycore.InteractionAckPolicy{Mode: legacycore.InteractionAckModeDefer, Ephemeral: true},
				handler: func(ctx *legacycore.Context) error {
					return nil
				},
			})
			interaction := buildInteraction("gatewaytest", "guild", "user")
			done := make(chan struct{})
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.cancelCtx {
				ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()
			} else {
				ctx = context.Background()
			}
			start := time.Now()
			go func() {
				router.HandleInteractionWithContext(ctx, session, interaction)
				close(done)
			}()
			select {
			case <-done:
				duration := time.Since(start)
				if tt.cancelCtx && duration >= 500*time.Millisecond {
					t.Fatalf("HandleInteractionWithContext did not respect context cancellation, took %v", duration)
				}
			case <-time.After(2 * time.Second):
				if !tt.expectHung {
					t.Fatal("HandleInteractionWithContext hung on gateway failure")
				}
			}
		})
	}
}
