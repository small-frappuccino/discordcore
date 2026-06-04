package cache

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestCachedSessionGuildMemberUsesStateAndCaches(t *testing.T) {
	session := &discordgo.Session{State: discordgo.NewState()}
	member := &discordgo.Member{User: &discordgo.User{ID: "user"}}
	guild := &discordgo.Guild{ID: "guild", Members: []*discordgo.Member{member}}
	if err := session.State.GuildAdd(guild); err != nil {
		t.Fatalf("guild add: %v", err)
	}

	cache := NewUnifiedCache(CacheConfig{MemberTTL: time.Minute, GuildTTL: time.Minute, RolesTTL: time.Minute, ChannelTTL: time.Minute, CleanupInterval: time.Hour})
	cs := NewCachedSession(session, cache)

	got, err := cs.GuildMember("guild", "user")
	if err != nil {
		t.Fatalf("GuildMember returned error: %v", err)
	}
	if got == nil || got.User.ID != "user" {
		t.Fatalf("unexpected member: %+v", got)
	}
	if cached, ok := cache.GetMember("guild", "user"); !ok || cached.User.ID != "user" {
		t.Fatalf("expected member cached after state hit")
	}
}

func TestCachedSessionChannelUsesStateFallbackOrder(t *testing.T) {
	session := &discordgo.Session{State: discordgo.NewState()}
	ch := &discordgo.Channel{ID: "chan", GuildID: "g"}
	_ = session.State.ChannelAdd(ch)

	cache := NewUnifiedCache(CacheConfig{MemberTTL: time.Minute, GuildTTL: time.Minute, RolesTTL: time.Minute, ChannelTTL: time.Minute, CleanupInterval: time.Hour})
	// Prime cache to avoid hitting REST.
	cache.SetChannel("chan", ch)
	cs := NewCachedSession(session, cache)

	got, err := cs.Channel("chan")
	if err != nil {
		t.Fatalf("Channel returned error: %v", err)
	}
	if got == nil || got.ID != "chan" {
		t.Fatalf("unexpected channel: %+v", got)
	}
	if cached, ok := cache.GetChannel("chan"); !ok || cached.ID != "chan" {
		t.Fatalf("expected channel cached after state hit")
	}
}

type mockRoundTripper struct {
	calls atomic.Int32
	delay time.Duration
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.calls.Add(1)
	time.Sleep(m.delay)
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`{"user": {"id": "user123"}}`)),
		Header:     make(http.Header),
	}, nil
}

func TestCachedSessionSingleflightConcurrency(t *testing.T) {
	session, _ := discordgo.New("")

	rt := &mockRoundTripper{delay: 50 * time.Millisecond}
	session.Client = &http.Client{Transport: rt}

	cache := NewUnifiedCache(CacheConfig{MemberTTL: time.Minute})
	cs := NewCachedSession(session, cache)

	var wg sync.WaitGroup
	const concurrentCallers = 100

	// Create a barrier so all goroutines fire as simultaneously as possible
	start := make(chan struct{})

	// Pre-allocate to prevent race on t.Errorf
	var errs atomic.Int32

	for i := 0; i < concurrentCallers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			member, err := cs.GuildMember("guild123", "user123")
			if err != nil {
				t.Errorf("GuildMember error: %v", err)
				errs.Add(1)
			} else if member == nil || member.User == nil || member.User.ID != "user123" {
				t.Errorf("unexpected member result")
				errs.Add(1)
			}
		}()
	}

	// Release the barrier
	close(start)
	wg.Wait()

	if errs.Load() > 0 {
		t.Fatalf("encountered errors during concurrent fetches")
	}

	calls := rt.calls.Load()
	if calls != 1 {
		t.Fatalf("expected exactly 1 API call due to singleflight, got %d", calls)
	}
}
