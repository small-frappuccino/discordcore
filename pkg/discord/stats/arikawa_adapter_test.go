package stats

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/httputil/httpdriver"
	domain "github.com/small-frappuccino/discordcore/pkg/stats"
)

type mockTransport struct {
	roundTrip func(*http.Request) (*http.Response, error)
}

func (m mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestArikawaGateway(t *testing.T) {
	s := state.New("Bot token")
	s.Client.Client.Client = httpdriver.WrapClient(http.Client{
		Transport: mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				if req.Method == "PATCH" && strings.Contains(req.URL.Path, "123") {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(`{"id":"123","name":"test","guild_id":"456"}`)),
					}, nil
				}
				if req.Method == "GET" && strings.Contains(req.URL.Path, "123") {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(`{"id":"123","name":"test","guild_id":"456"}`)),
					}, nil
				}
				if req.Method == "GET" && strings.Contains(req.URL.Path, "members") {
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewBufferString(`[{"user":{"id":"1","bot":true},"roles":["2"]}]`)),
					}, nil
				}
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
				}, nil
			},
		},
	})

	adapter := NewArikawaGateway(s, slog.Default())
	ctx := context.Background()

	t.Run("UpdateChannelName", func(t *testing.T) {
		err := adapter.UpdateChannelName(ctx, "invalid", "name")
		if err == nil {
			t.Errorf("expected error on invalid snowflake")
		}

		err = adapter.UpdateChannelName(ctx, "123", "name")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("GetChannel", func(t *testing.T) {
		_, err := adapter.GetChannel(ctx, "invalid")
		if err == nil {
			t.Errorf("expected error on invalid snowflake")
		}

		ch, err := adapter.GetChannel(ctx, "123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		} else if ch.Name != "test" {
			t.Errorf("expected test, got %s", ch.Name)
		}
	})

	t.Run("StreamGuildMembers", func(t *testing.T) {
		seq := adapter.StreamGuildMembers(ctx, "invalid")
		seq(func(snap domain.MemberSnapshot, err error) bool {
			if err == nil {
				t.Errorf("expected error on invalid snowflake")
			}
			return false
		})

		seq = adapter.StreamGuildMembers(ctx, "456")
		var count int
		seq(func(snap domain.MemberSnapshot, err error) bool {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return false
			}
			count++
			// Iterating over roles to ensure 100% coverage
			if snap.Roles != nil {
				snap.Roles(func(r string) bool { return true })
			}
			return true
		})
		if count != 1 {
			t.Errorf("expected 1 member, got %d", count)
		}
	})

	t.Run("StreamGuildMembers_ContextCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		seq := adapter.StreamGuildMembers(ctx, "456")
		seq(func(snap domain.MemberSnapshot, err error) bool {
			if err == nil {
				t.Errorf("expected context cancelled error")
			}
			return false
		})
	})
}
