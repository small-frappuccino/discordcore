package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func newTestCache(t *testing.T) *UnifiedCache {
	t.Helper()
	cache := NewUnifiedCache(CacheConfig{
		MemberTTL:       time.Hour,
		GuildTTL:        time.Hour,
		RolesTTL:        time.Hour,
		ChannelTTL:      time.Hour,
		CleanupInterval: time.Hour,
	})
	t.Cleanup(cache.Stop)
	return cache
}

// Mock helpers have been replaced by inline closures on the warmupSession struct.

func TestIntelligentWarmupPopulatesCache(t *testing.T) {
	guild := &discordgo.Guild{ID: "g1", Name: "Guild One"}
	role := &discordgo.Role{ID: "r1", Name: "role"}
	channel := &discordgo.Channel{ID: "c1", GuildID: "g1", Name: "general"}
	member := &discordgo.Member{User: &discordgo.User{ID: "u1"}, JoinedAt: time.Now().UTC()}

	session := warmupSession{
		StateGuilds: func() []*discordgo.Guild { return []*discordgo.Guild{guild} },
		Guild: func(id string, options ...discordgo.RequestOption) (*discordgo.Guild, error) {
			if id == "g1" {
				return guild, nil
			}
			return nil, fmt.Errorf("missing guild %s", id)
		},
		GuildRoles: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Role, error) {
			if id == "g1" {
				return []*discordgo.Role{role}, nil
			}
			return nil, fmt.Errorf("missing roles %s", id)
		},
		GuildChannels: func(id string, options ...discordgo.RequestOption) ([]*discordgo.Channel, error) {
			if id == "g1" {
				return []*discordgo.Channel{channel}, nil
			}
			return nil, fmt.Errorf("missing channels %s", id)
		},
		GuildMembers: func(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			if after == "" {
				return []*discordgo.Member{member}, nil
			}
			return nil, nil
		},
	}

	old := newWarmupSession
	newWarmupSession = func(_ *discordgo.Session) warmupSession { return session }
	t.Cleanup(func() { newWarmupSession = old })

	cache := newTestCache(t)
	cfg := WarmupConfig{
		FetchMissingMembers:  true,
		FetchMissingRoles:    true,
		FetchMissingGuilds:   true,
		FetchMissingChannels: true,
		MaxMembersPerGuild:   1,
	}

	if err := IntelligentWarmup(&discordgo.Session{}, cache, nil, cfg); err != nil {
		t.Fatalf("IntelligentWarmup error: %v", err)
	}

	if got, ok := cache.GetGuild("g1"); !ok || got == nil || got.ID != "g1" {
		t.Fatalf("expected guild cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetRoles("g1"); !ok || len(got) != 1 || got[0].ID != "r1" {
		t.Fatalf("expected roles cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetChannel("c1"); !ok || got == nil || got.ID != "c1" {
		t.Fatalf("expected channel cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetMember("g1", "u1"); !ok || got == nil || got.User.ID != "u1" {
		t.Fatalf("expected member cached, got %v %v", got, ok)
	}
	if got, ok := cache.GetMember("g1", "missing"); ok || got != nil {
		t.Fatalf("expected cache miss for unknown member, got %v %v", got, ok)
	}
}

// blockingWarmupSession removed in favor of inline closures

func TestWarmupGuildMembersConcurrentCalls(t *testing.T) {
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	members := []*discordgo.Member{{User: &discordgo.User{ID: "u1"}}}

	session := warmupSession{
		GuildMembers: func(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
			ready <- struct{}{}
			<-release
			return members, nil
		},
	}
	cache := newTestCache(t)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := warmupGuildMembers(session, cache, nil, "g1", 1)
			errs <- err
		}()
	}

	<-ready
	<-ready
	close(release)

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("warmupGuildMembers error: %v", err)
		}
	}
	if got, ok := cache.GetMember("g1", "u1"); !ok || got == nil || got.User.ID != "u1" {
		t.Fatalf("expected member cached after concurrent warmup, got %v %v", got, ok)
	}
}
