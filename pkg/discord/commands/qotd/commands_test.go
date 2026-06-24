package qotd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"testing"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	"golang.org/x/sync/errgroup"
)

type MockService struct {
	mu            sync.Mutex
	inProgressMap map[string]bool
	panicOnRun    bool
}

func (s *MockService) ExecuteInGuildActorWithResult(guildID string, fn func() (any, error)) (any, error) {
	s.mu.Lock()
	if s.inProgressMap == nil {
		s.inProgressMap = make(map[string]bool)
	}

	if s.inProgressMap[guildID] {
		s.mu.Unlock()
		return nil, errors.New("concurrent access detected")
	}

	s.inProgressMap[guildID] = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.inProgressMap[guildID] = false
		s.mu.Unlock()
	}()

	if s.panicOnRun {
		panic("forced panic for test")
	}

	return fn()
}

func TestCommandHandler_ThunderingHerds(t *testing.T) {
	t.Parallel()
	svc := &MockService{}
	client := api.NewClient("token")
	client.Client.Client = httpdriver.WrapClient(http.Client{Transport: &mockTransport{}})

	handler := NewCommandHandler(svc, client)

	const numGoroutines = 1000
	eg, ctx := errgroup.WithContext(context.Background())

	for i := 0; i < numGoroutines; i++ {
		idx := i
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			ev := &gateway.InteractionCreateEvent{
				InteractionEvent: discord.InteractionEvent{
					ID:      discord.InteractionID(idx + 1),
					Token:   "token",
					GuildID: 12345,
					Data: &discord.CommandInteraction{
						Name: "qotd",
						Options: []discord.CommandInteractionOption{
							{
								Name: "publish",
							},
						},
					},
				},
			}
			// We only want to ensure it doesn't cause race conditions
			// or panic inside the handler concurrency map
			// Mock client will fail deferring, but the recover block handles it
			handler.HandleInteraction(ev)
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("thundering herds execution failed: %v", err)
	}
}

func TestCommandHandler_PanicRecovery(t *testing.T) {
	t.Parallel()
	svc := &MockService{panicOnRun: true}
	client := api.NewClient("token")
	client.Client.Client = httpdriver.WrapClient(http.Client{Transport: &mockTransport{}})

	handler := NewCommandHandler(svc, client).WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))

	ev := &gateway.InteractionCreateEvent{
		InteractionEvent: discord.InteractionEvent{
			ID:      discord.InteractionID(1),
			Token:   "token",
			GuildID: 12345,
			Data: &discord.CommandInteraction{
				Name: "qotd",
				Options: []discord.CommandInteractionOption{
					{
						Name: "publish",
					},
				},
			},
		},
	}

	handler.HandleInteraction(ev)
}

type mockTransport struct{}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		Header:     make(http.Header),
	}, nil
}
