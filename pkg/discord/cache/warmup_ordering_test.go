package cache_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"

	"github.com/small-frappuccino/discordgo"
)

// recordingWarmupSession replaced with inline closures

func TestIntelligentWarmupOrdering(t *testing.T) {
	g1 := &discordgo.Guild{ID: "g1", Name: "Guild One"}
	g2 := &discordgo.Guild{ID: "g2", Name: "Guild Two"}
	r1 := &discordgo.Role{ID: "r1", Name: "role"}
	c1 := &discordgo.Channel{ID: "c1", GuildID: "g1", Name: "chan"}
	m1 := &discordgo.Member{User: &discordgo.User{ID: "u1"}, JoinedAt: time.Now().UTC()}

	var mu sync.Mutex
	var calls []string
	var stateCalls int

	record := func(call string) {
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
	}

	session := cache.WarmupSession{
		StateGuilds: func() []*discordgo.Guild {
			mu.Lock()
			stateCalls++
			mu.Unlock()
			return nil
		},
		Guild: func(id string, options ...discordgo.RequestOption) (*discordgo.Guild, error) {
			record("guild:" + id)
			if id == "g1" {
				return g1, nil
			}
			if id == "g2" {
				return g2, nil
			}
			return nil, fmt.Errorf("missing guild %s", id)
		},
		GuildRoles: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Role, error) {
			record("roles:" + id)
			if id == "g1" || id == "g2" {
				return []*discordgo.Role{r1}, nil
			}
			return nil, fmt.Errorf("missing roles %s", id)
		},
		GuildChannels: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
			record("channels:" + id)
			if id == "g1" || id == "g2" {
				return []*discordgo.Channel{c1}, nil
			}
			return nil, fmt.Errorf("missing channels %s", id)
		},
		GuildMembers: func(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			record("members:" + guildID)
			if guildID == "g1" || guildID == "g2" {
				return []*discordgo.Member{m1}, nil
			}
			return nil, fmt.Errorf("missing members %s", guildID)
		},
	}

	old := cache.NewWarmupSession
	cache.NewWarmupSession = func(_ *discordgo.Session) cache.WarmupSession { return session }
	t.Cleanup(func() { cache.NewWarmupSession = old })

	uc := newTestCache(t)
	cfg := cache.WarmupConfig{
		FetchMissingMembers:  true,
		FetchMissingRoles:    true,
		FetchMissingGuilds:   true,
		FetchMissingChannels: true,
		MaxMembersPerGuild:   1,
		GuildIDs:             []string{"g1", "g2"},
	}

	if err := cache.IntelligentWarmup(&discordgo.Session{}, uc, nil, cfg); err != nil {
		t.Fatalf("IntelligentWarmup error: %v", err)
	}

	expected := []string{
		"guild:g1",
		"roles:g1",
		"channels:g1",
		"members:g1",
		"guild:g2",
		"roles:g2",
		"channels:g2",
		"members:g2",
	}

	mu.Lock()
	got := append([]string(nil), calls...)
	sCalls := stateCalls
	mu.Unlock()

	if sCalls != 0 {
		t.Fatalf("expected StateGuilds not to be called, got %d", sCalls)
	}
	if len(got) != len(expected) {
		t.Fatalf("unexpected call count: got %d want %d (%v)", len(got), len(expected), got)
	}
	for i, want := range expected {
		if got[i] != want {
			t.Fatalf("call order mismatch at %d: got %q want %q", i, got[i], want)
		}
	}
}
